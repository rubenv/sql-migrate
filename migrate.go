package migrate

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-gorp/gorp/v3"

	"github.com/rubenv/sql-migrate/sqlparse"
)

type MigrationDirection int

const (
	Up MigrationDirection = iota
	Down
)

// MigrationSet provides database parameters for a migration execution
type MigrationSet struct {
	// TableName name of the table used to store migration info.
	TableName string
	// SchemaName schema that the migration table be referenced.
	SchemaName string
	// IgnoreUnknown skips the check to see if there is a migration
	// ran in the database that is not in MigrationSource.
	//
	// This should be used sparingly as it is removing a safety check.
	IgnoreUnknown bool
	// DisableCreateTable disable the creation of the migration table
	DisableCreateTable bool
	// LazyLoad enable migration file to be loaded only when needed.
	LazyLoad bool
}

var migSet = MigrationSet{}

// NewMigrationSet returns a parametrized Migration object
func (ms MigrationSet) getTableName() string {
	if ms.TableName == "" {
		return "gorp_migrations"
	}
	return ms.TableName
}

var numberPrefixRegex = regexp.MustCompile(`^(\d+).*$`)

// PlanError happens where no migration plan could be created between the sets
// of already applied migrations and the currently found. For example, when the database
// contains a migration which is not among the migrations list found for an operation.
type PlanError struct {
	Migration    *Migration
	ErrorMessage string
}

func newPlanError(migration *Migration, errorMessage string) error {
	return &PlanError{
		Migration:    migration,
		ErrorMessage: errorMessage,
	}
}

func (p *PlanError) Error() string {
	return fmt.Sprintf("Unable to create migration plan because of %s: %s",
		p.Migration.Id, p.ErrorMessage)
}

// TxError is returned when any error is encountered during a database
// transaction. It contains the relevant *Migration and notes it's Id in the
// Error function output.
type TxError struct {
	Migration *Migration
	Err       error
}

func newTxError(migration *PlannedMigration, err error) error {
	return &TxError{
		Migration: migration.Migration,
		Err:       err,
	}
}

func (e *TxError) Error() string {
	return e.Err.Error() + " handling " + e.Migration.Id
}

// Set the name of the table used to store migration info.
//
// Should be called before any other call such as (Exec, ExecMax, ...).
func SetTable(name string) {
	if name != "" {
		migSet.TableName = name
	}
}

// SetSchema sets the name of a schema that the migration table be referenced.
func SetSchema(name string) {
	if name != "" {
		migSet.SchemaName = name
	}
}

// SetDisableCreateTable sets the boolean to disable the creation of the migration table
func SetDisableCreateTable(disable bool) {
	migSet.DisableCreateTable = disable
}

// SetIgnoreUnknown sets the flag that skips database check to see if there is a
// migration in the database that is not in migration source.
//
// This should be used sparingly as it is removing a safety check.
func SetIgnoreUnknown(v bool) {
	migSet.IgnoreUnknown = v
}

// SetLazyLoad sets the boolean to enable migration file to be loaded only when needed.
func SetLazyLoad(v bool) {
	migSet.LazyLoad = v
}

type migrationFile struct {
	dir      http.FileSystem
	root     string
	baseName string
}

type Migration struct {
	Id   string
	Up   []string
	Down []string

	DisableTransactionUp   bool
	DisableTransactionDown bool

	// lazyLoadFile is information of migration file, which is used to load migration file later if not nil.
	lazyLoadFile *migrationFile
}

func (m Migration) Less(other *Migration) bool {
	switch {
	case m.isNumeric() && other.isNumeric() && m.VersionInt() != other.VersionInt():
		return m.VersionInt() < other.VersionInt()
	case m.isNumeric() && !other.isNumeric():
		return true
	case !m.isNumeric() && other.isNumeric():
		return false
	default:
		return m.Id < other.Id
	}
}

func (m Migration) isNumeric() bool {
	return len(m.NumberPrefixMatches()) > 0
}

func (m Migration) NumberPrefixMatches() []string {
	return numberPrefixRegex.FindStringSubmatch(m.Id)
}

func (m Migration) VersionInt() int64 {
	v := m.NumberPrefixMatches()[1]
	value, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("Could not parse %q into int64: %s", v, err))
	}
	return value
}

// Load parses migration file if not yet
func (m *Migration) Load() error {
	if m.lazyLoadFile == nil {
		return nil
	}
	root := m.lazyLoadFile.root
	name := m.lazyLoadFile.baseName
	file, err := m.lazyLoadFile.dir.Open(path.Join(root, name))
	if err != nil {
		return fmt.Errorf("Error while opening %s: %s", name, err)
	}
	defer func() { _ = file.Close() }()

	parsed, err := sqlparse.ParseMigration(file)
	if err != nil {
		return fmt.Errorf("Error parsing migration (%s): %s", m.Id, err)
	}
	m.Up = parsed.UpStatements
	m.Down = parsed.DownStatements
	m.DisableTransactionUp = parsed.DisableTransactionUp
	m.DisableTransactionDown = parsed.DisableTransactionDown
	m.lazyLoadFile = nil
	return nil
}

type PlannedMigration struct {
	*Migration

	DisableTransaction bool
	Queries            []string
}

type byId []*Migration

func (b byId) Len() int           { return len(b) }
func (b byId) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byId) Less(i, j int) bool { return b[i].Less(b[j]) }

type MigrationRecord struct {
	Id        string    `db:"id"`
	AppliedAt time.Time `db:"applied_at"`
}

type OracleDialect struct {
	gorp.OracleDialect
}

func (d OracleDialect) IfTableNotExists(command, schema, table string) string {
	return command
}

func (d OracleDialect) IfSchemaNotExists(command, schema string) string {
	return command
}

func (d OracleDialect) IfTableExists(command, schema, table string) string {
	return command
}

var MigrationDialects = map[string]gorp.Dialect{
	"sqlite3":   gorp.SqliteDialect{},
	"postgres":  gorp.PostgresDialect{},
	"mysql":     gorp.MySQLDialect{Engine: "InnoDB", Encoding: "UTF8"},
	"mssql":     gorp.SqlServerDialect{},
	"oci8":      OracleDialect{},
	"godror":    OracleDialect{},
	"snowflake": gorp.SnowflakeDialect{},
}

type MigrationSource interface {
	// Finds the migrations.
	//
	// The resulting slice of migrations should be sorted by Id.
	FindMigrations() ([]*Migration, error)
}

// A hardcoded set of migrations, in-memory.
type MemoryMigrationSource struct {
	Migrations []*Migration
}

var _ MigrationSource = (*MemoryMigrationSource)(nil)

func (m MemoryMigrationSource) FindMigrations() ([]*Migration, error) {
	// Make sure migrations are sorted. In order to make the MemoryMigrationSource safe for
	// concurrent use we should not mutate it in place. So `FindMigrations` would sort a copy
	// of the m.Migrations.
	migrations := make([]*Migration, len(m.Migrations))
	copy(migrations, m.Migrations)
	sort.Sort(byId(migrations))
	return migrations, nil
}

// A set of migrations loaded from an http.FileServer

type HttpFileSystemMigrationSource struct {
	FileSystem http.FileSystem
}

var _ MigrationSource = (*HttpFileSystemMigrationSource)(nil)

func (f HttpFileSystemMigrationSource) FindMigrations() ([]*Migration, error) {
	return findMigrations(f.FileSystem, "/")
}

// A set of migrations loaded from a directory.
type FileMigrationSource struct {
	Dir string
}

var _ MigrationSource = (*FileMigrationSource)(nil)

func (f FileMigrationSource) FindMigrations() ([]*Migration, error) {
	filesystem := http.Dir(f.Dir)
	return findMigrations(filesystem, "/")
}

func findMigrations(dir http.FileSystem, root string) ([]*Migration, error) {
	migrations := make([]*Migration, 0)

	file, err := dir.Open(root)
	if err != nil {
		return nil, err
	}

	files, err := file.Readdir(0)
	if err != nil {
		return nil, err
	}

	for _, info := range files {
		if strings.HasSuffix(info.Name(), ".sql") {
			if migSet.LazyLoad {
				migration := &Migration{
					Id: info.Name(),
					lazyLoadFile: &migrationFile{
						dir:      dir,
						root:     root,
						baseName: info.Name(),
					},
				}
				migrations = append(migrations, migration)
			} else {
				migration, err := migrationFromFile(dir, root, info)
				if err != nil {
					return nil, err
				}
				migrations = append(migrations, migration)
			}
		}
	}

	// Make sure migrations are sorted
	sort.Sort(byId(migrations))

	return migrations, nil
}

func migrationFromFile(dir http.FileSystem, root string, info os.FileInfo) (*Migration, error) {
	path := path.Join(root, info.Name())
	file, err := dir.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Error while opening %s: %s", info.Name(), err)
	}
	defer func() { _ = file.Close() }()

	migration, err := ParseMigration(info.Name(), file)
	if err != nil {
		return nil, fmt.Errorf("Error while parsing %s: %s", info.Name(), err)
	}
	return migration, nil
}

// Migrations from a bindata asset set.
type AssetMigrationSource struct {
	// Asset should return content of file in path if exists
	Asset func(path string) ([]byte, error)

	// AssetDir should return list of files in the path
	AssetDir func(path string) ([]string, error)

	// Path in the bindata to use.
	Dir string
}

var _ MigrationSource = (*AssetMigrationSource)(nil)

func (a AssetMigrationSource) FindMigrations() ([]*Migration, error) {
	migrations := make([]*Migration, 0)

	files, err := a.AssetDir(a.Dir)
	if err != nil {
		return nil, err
	}

	for _, name := range files {
		if strings.HasSuffix(name, ".sql") {
			file, err := a.Asset(path.Join(a.Dir, name))
			if err != nil {
				return nil, err
			}

			migration, err := ParseMigration(name, bytes.NewReader(file))
			if err != nil {
				return nil, err
			}

			migrations = append(migrations, migration)
		}
	}

	// Make sure migrations are sorted
	sort.Sort(byId(migrations))

	return migrations, nil
}

// Avoids pulling in the packr library for everyone, mimicks the bits of
// packr.Box that we need.
type PackrBox interface {
	List() []string
	Find(name string) ([]byte, error)
}

// Migrations from a packr box.
type PackrMigrationSource struct {
	Box PackrBox

	// Path in the box to use.
	Dir string
}

var _ MigrationSource = (*PackrMigrationSource)(nil)

func (p PackrMigrationSource) FindMigrations() ([]*Migration, error) {
	migrations := make([]*Migration, 0)
	items := p.Box.List()

	prefix := ""
	dir := path.Clean(p.Dir)
	if dir != "." {
		prefix = fmt.Sprintf("%s/", dir)
	}

	for _, item := range items {
		if !strings.HasPrefix(item, prefix) {
			continue
		}
		name := strings.TrimPrefix(item, prefix)
		if strings.Contains(name, "/") {
			continue
		}

		if strings.HasSuffix(name, ".sql") {
			file, err := p.Box.Find(item)
			if err != nil {
				return nil, err
			}

			migration, err := ParseMigration(name, bytes.NewReader(file))
			if err != nil {
				return nil, err
			}

			migrations = append(migrations, migration)
		}
	}

	// Make sure migrations are sorted
	sort.Sort(byId(migrations))

	return migrations, nil
}

// Migration parsing
func ParseMigration(id string, r io.ReadSeeker) (*Migration, error) {
	m := &Migration{
		Id: id,
	}

	parsed, err := sqlparse.ParseMigration(r)
	if err != nil {
		return nil, fmt.Errorf("Error parsing migration (%s): %s", id, err)
	}

	m.Up = parsed.UpStatements
	m.Down = parsed.DownStatements

	m.DisableTransactionUp = parsed.DisableTransactionUp
	m.DisableTransactionDown = parsed.DisableTransactionDown

	return m, nil
}

type SqlExecutor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Insert(list ...interface{}) error
	Delete(list ...interface{}) (int64, error)
}

// Execute a set of migrations
//
// Returns the number of applied migrations.
func Exec(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection) (int, error) {
	return ExecMax(db, dialect, m, dir, 0)
}

// Returns the number of applied migrations.
func (ms MigrationSet) Exec(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection) (int, error) {
	return ms.ExecMax(db, dialect, m, dir, 0)
}

// Execute a set of migrations
//
// Will apply at most `max` migrations. Pass 0 for no limit (or use Exec).
//
// Returns the number of applied migrations.
func ExecMax(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, max int) (int, error) {
	return migSet.ExecMax(db, dialect, m, dir, max)
}

// Execute a set of migrations
//
// Will apply at the target `version` of migration. Cannot be a negative value.
//
// Returns the number of applied migrations.
func ExecVersion(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, version int64) (int, error) {
	if version < 0 {
		return 0, fmt.Errorf("target version %d should not be negative", version)
	}
	return migSet.ExecVersion(db, dialect, m, dir, version)
}

// Returns the number of applied migrations.
func (ms MigrationSet) ExecMax(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, max int) (int, error) {
	migrations, dbMap, err := ms.PlanMigration(db, dialect, m, dir, max)
	if err != nil {
		return 0, err
	}
	return ms.applyMigrations(dir, migrations, dbMap)
}

// Returns the number of applied migrations.
func (ms MigrationSet) ExecVersion(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, version int64) (int, error) {
	migrations, dbMap, err := ms.PlanMigrationToVersion(db, dialect, m, dir, version)
	if err != nil {
		return 0, err
	}
	return ms.applyMigrations(dir, migrations, dbMap)
}

// Applies the planned migrations and returns the number of applied migrations.
func (ms MigrationSet) applyMigrations(dir MigrationDirection, migrations []*PlannedMigration, dbMap *gorp.DbMap) (int, error) {
	applied := 0
	for _, migration := range migrations {
		var executor SqlExecutor
		var err error

		if migration.DisableTransaction {
			executor = dbMap
		} else {
			executor, err = dbMap.Begin()
			if err != nil {
				return applied, newTxError(migration, err)
			}
		}

		for _, stmt := range migration.Queries {
			// remove the semicolon from stmt, fix ORA-00922 issue in database oracle
			stmt = strings.TrimSuffix(stmt, "\n")
			stmt = strings.TrimSuffix(stmt, " ")
			stmt = strings.TrimSuffix(stmt, ";")
			if _, err := executor.Exec(stmt); err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					_ = trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		}

		switch dir {
		case Up:
			err = executor.Insert(&MigrationRecord{
				Id:        migration.Id,
				AppliedAt: time.Now(),
			})
			if err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					_ = trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		case Down:
			_, err := executor.Delete(&MigrationRecord{
				Id: migration.Id,
			})
			if err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					_ = trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		default:
			panic("Not possible")
		}

		if trans, ok := executor.(*gorp.Transaction); ok {
			if err := trans.Commit(); err != nil {
				return applied, newTxError(migration, err)
			}
		}

		applied++
	}

	return applied, nil
}

// Plan a migration.
func PlanMigration(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, max int) ([]*PlannedMigration, *gorp.DbMap, error) {
	return migSet.PlanMigration(db, dialect, m, dir, max)
}

// Plan a migration to version.
func PlanMigrationToVersion(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, version int64) ([]*PlannedMigration, *gorp.DbMap, error) {
	return migSet.PlanMigrationToVersion(db, dialect, m, dir, version)
}

// Plan a migration.
func (ms MigrationSet) PlanMigration(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, max int) ([]*PlannedMigration, *gorp.DbMap, error) {
	return ms.planMigrationCommon(db, dialect, m, dir, max, -1)
}

// Plan a migration to version.
func (ms MigrationSet) PlanMigrationToVersion(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, version int64) ([]*PlannedMigration, *gorp.DbMap, error) {
	return ms.planMigrationCommon(db, dialect, m, dir, 0, version)
}

// A common method to plan a migration.
func (ms MigrationSet) planMigrationCommon(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, max int, version int64) ([]*PlannedMigration, *gorp.DbMap, error) {
	dbMap, err := ms.getMigrationDbMap(db, dialect)
	if err != nil {
		return nil, nil, err
	}

	migrations, err := m.FindMigrations()
	if err != nil {
		return nil, nil, err
	}

	var migrationRecords []MigrationRecord
	_, err = dbMap.Select(&migrationRecords, fmt.Sprintf("SELECT * FROM %s", dbMap.Dialect.QuotedTableForQuery(ms.SchemaName, ms.getTableName())))
	if err != nil {
		return nil, nil, err
	}

	// Sort migrations that have been run by Id.
	var existingMigrations []*Migration
	for _, migrationRecord := range migrationRecords {
		existingMigrations = append(existingMigrations, &Migration{
			Id: migrationRecord.Id,
		})
	}
	sort.Sort(byId(existingMigrations))

	// Make sure all migrations in the database are among the found migrations which
	// are to be applied.
	if !ms.IgnoreUnknown {
		migrationsSearch := make(map[string]struct{})
		for _, migration := range migrations {
			migrationsSearch[migration.Id] = struct{}{}
		}
		for _, existingMigration := range existingMigrations {
			if _, ok := migrationsSearch[existingMigration.Id]; !ok {
				return nil, nil, newPlanError(existingMigration, "unknown migration in database")
			}
		}
	}

	// Get last migration that was run
	record := &Migration{}
	if len(existingMigrations) > 0 {
		record = existingMigrations[len(existingMigrations)-1]
	}

	result := make([]*PlannedMigration, 0)

	// Add missing migrations up to the last run migration.
	// This can happen for example when merges happened.
	if len(existingMigrations) > 0 {
		catchUp, err := ToCatchup(migrations, existingMigrations, record)
		if err != nil {
			return nil, nil, err
		}
		result = append(result, catchUp...)
	}

	// Figure out which migrations to apply
	toApply := ToApply(migrations, record.Id, dir)
	toApplyCount := len(toApply)

	if version >= 0 {
		targetIndex := 0
		for targetIndex < len(toApply) {
			tempVersion := toApply[targetIndex].VersionInt()
			if dir == Up && tempVersion > version || dir == Down && tempVersion < version {
				return nil, nil, newPlanError(&Migration{}, fmt.Errorf("unknown migration with version id %d in database", version).Error())
			}
			if tempVersion == version {
				toApplyCount = targetIndex + 1
				break
			}
			targetIndex++
		}
		if targetIndex == len(toApply) {
			return nil, nil, newPlanError(&Migration{}, fmt.Errorf("unknown migration with version id %d in database", version).Error())
		}
	} else if max > 0 && max < toApplyCount {
		toApplyCount = max
	}
	for _, v := range toApply[0:toApplyCount] {
		err = v.Load()
		if err != nil {
			return nil, nil, err
		}

		if dir == Up {
			result = append(result, &PlannedMigration{
				Migration:          v,
				Queries:            v.Up,
				DisableTransaction: v.DisableTransactionUp,
			})
		} else if dir == Down {
			result = append(result, &PlannedMigration{
				Migration:          v,
				Queries:            v.Down,
				DisableTransaction: v.DisableTransactionDown,
			})
		}
	}

	return result, dbMap, nil
}

// Skip a set of migrations
//
// Will skip at most `max` migrations. Pass 0 for no limit.
//
// Returns the number of skipped migrations.
func SkipMax(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, max int) (int, error) {
	migrations, dbMap, err := PlanMigration(db, dialect, m, dir, max)
	if err != nil {
		return 0, err
	}

	// Skip migrations
	applied := 0
	for _, migration := range migrations {
		var executor SqlExecutor

		if migration.DisableTransaction {
			executor = dbMap
		} else {
			executor, err = dbMap.Begin()
			if err != nil {
				return applied, newTxError(migration, err)
			}
		}

		err = executor.Insert(&MigrationRecord{
			Id:        migration.Id,
			AppliedAt: time.Now(),
		})
		if err != nil {
			if trans, ok := executor.(*gorp.Transaction); ok {
				_ = trans.Rollback()
			}

			return applied, newTxError(migration, err)
		}

		if trans, ok := executor.(*gorp.Transaction); ok {
			if err := trans.Commit(); err != nil {
				return applied, newTxError(migration, err)
			}
		}

		applied++
	}

	return applied, nil
}

// Filter a slice of migrations into ones that should be applied.
func ToApply(migrations []*Migration, current string, direction MigrationDirection) []*Migration {
	var index = -1
	if current != "" {
		for index < len(migrations)-1 {
			index++
			if migrations[index].Id == current {
				break
			}
		}
	}

	if direction == Up {
		return migrations[index+1:]
	} else if direction == Down {
		if index == -1 {
			return []*Migration{}
		}

		// Add in reverse order
		toApply := make([]*Migration, index+1)
		for i := 0; i < index+1; i++ {
			toApply[index-i] = migrations[i]
		}
		return toApply
	}

	panic("Not possible")
}

func ToCatchup(migrations, existingMigrations []*Migration, lastRun *Migration) ([]*PlannedMigration, error) {
	missing := make([]*PlannedMigration, 0)
	for _, migration := range migrations {
		found := false
		for _, existing := range existingMigrations {
			if existing.Id == migration.Id {
				found = true
				break
			}
		}
		if !found && migration.Less(lastRun) {
			err := migration.Load()
			if err != nil {
				return nil, err
			}
			missing = append(missing, &PlannedMigration{
				Migration:          migration,
				Queries:            migration.Up,
				DisableTransaction: migration.DisableTransactionUp,
			})
		}
	}
	return missing, nil
}

func GetMigrationRecords(db *sql.DB, dialect string) ([]*MigrationRecord, error) {
	return migSet.GetMigrationRecords(db, dialect)
}

func (ms MigrationSet) GetMigrationRecords(db *sql.DB, dialect string) ([]*MigrationRecord, error) {
	dbMap, err := ms.getMigrationDbMap(db, dialect)
	if err != nil {
		return nil, err
	}

	var records []*MigrationRecord
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY %s ASC", dbMap.Dialect.QuotedTableForQuery(ms.SchemaName, ms.getTableName()), dbMap.Dialect.QuoteField("id"))
	_, err = dbMap.Select(&records, query)
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (ms MigrationSet) getMigrationDbMap(db *sql.DB, dialect string) (*gorp.DbMap, error) {
	d, ok := MigrationDialects[dialect]
	if !ok {
		return nil, fmt.Errorf("Unknown dialect: %s", dialect)
	}

	// When using the mysql driver, make sure that the parseTime option is
	// configured, otherwise it won't map time columns to time.Time. See
	// https://github.com/rubenv/sql-migrate/issues/2
	if dialect == "mysql" {
		var out *time.Time
		err := db.QueryRow("SELECT NOW()").Scan(&out)
		if err != nil {
			if err.Error() == "sql: Scan error on column index 0: unsupported driver -> Scan pair: []uint8 -> *time.Time" ||
				err.Error() == "sql: Scan error on column index 0: unsupported Scan, storing driver.Value type []uint8 into type *time.Time" ||
				err.Error() == "sql: Scan error on column index 0, name \"NOW()\": unsupported Scan, storing driver.Value type []uint8 into type *time.Time" {
				return nil, errors.New(`Cannot parse dates.

Make sure that the parseTime option is supplied to your database connection.
Check https://github.com/go-sql-driver/mysql#parsetime for more info.`)
			} else {
				return nil, err
			}
		}
	}

	// Create migration database map
	dbMap := &gorp.DbMap{Db: db, Dialect: d}
	table := dbMap.AddTableWithNameAndSchema(MigrationRecord{}, ms.SchemaName, ms.getTableName()).SetKeys(false, "Id")
	//dbMap.TraceOn("", log.New(os.Stdout, "migrate: ", log.Lmicroseconds))

	if dialect == "oci8" || dialect == "godror" {
		table.ColMap("Id").SetMaxSize(4000)
	}

	if migSet.DisableCreateTable {
		return dbMap, nil
	}

	err := dbMap.CreateTablesIfNotExists()
	if err != nil {
		// Oracle database does not support `if not exists`, so use `ORA-00955:` error code
		// to check if the table exists.
		if (dialect == "oci8" || dialect == "godror") && strings.Contains(err.Error(), "ORA-00955:") {
			return dbMap, nil
		}
		return nil, err
	}

	return dbMap, nil
}

// TODO: Run migration + record insert in transaction.
