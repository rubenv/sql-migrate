package migrate

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rubenv/sql-migrate/sqlparse"
	"gopkg.in/gorp.v1"
)

type MigrationDirection int

const (
	Up MigrationDirection = iota
	Down
)

var tableName = "gorp_migrations"
var schemaName = ""
var numberPrefixRegex = regexp.MustCompile(`^(\d+).*$`)

// Lock related bits
var (
	DefaultLockWaitTime = time.Duration(1 * time.Minute)

	lockTableName     = "gorp_lock"
	lockName          = "sql-migrate"
	lockWatchInterval = time.Duration(1 * time.Second)
	lockMaxStaleAge   = time.Duration(1 * time.Minute)
)

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
		tableName = name
	}
}

// SetSchema sets the name of a schema that the migration table be referenced.
func SetSchema(name string) {
	if name != "" {
		schemaName = name
	}
}

type Migration struct {
	Id   string
	Up   []string
	Down []string

	DisableTransactionUp   bool
	DisableTransactionDown bool
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

type LockRecord struct {
	Lock       string    `db:"lock"`
	AcquiredAt time.Time `db:"acquired_at"`
}

var MigrationDialects = map[string]gorp.Dialect{
	"sqlite3":  gorp.SqliteDialect{},
	"postgres": gorp.PostgresDialect{},
	"mysql":    gorp.MySQLDialect{Engine: "InnoDB", Encoding: "UTF8"},
	"mssql":    gorp.SqlServerDialect{},
	"oci8":     gorp.OracleDialect{},
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
	return findMigrations(f.FileSystem)
}

// A set of migrations loaded from a directory.
type FileMigrationSource struct {
	Dir string
}

var _ MigrationSource = (*FileMigrationSource)(nil)

func (f FileMigrationSource) FindMigrations() ([]*Migration, error) {
	filesystem := http.Dir(f.Dir)
	return findMigrations(filesystem)
}

func findMigrations(dir http.FileSystem) ([]*Migration, error) {
	migrations := make([]*Migration, 0)

	file, err := dir.Open("/")
	if err != nil {
		return nil, err
	}

	files, err := file.Readdir(0)
	if err != nil {
		return nil, err
	}

	for _, info := range files {
		if strings.HasSuffix(info.Name(), ".sql") {
			file, err := dir.Open(info.Name())
			if err != nil {
				return nil, fmt.Errorf("Error while opening %s: %s", info.Name(), err)
			}

			migration, err := ParseMigration(info.Name(), file)
			if err != nil {
				return nil, fmt.Errorf("Error while parsing %s: %s", info.Name(), err)
			}

			migrations = append(migrations, migration)
		}
	}

	// Make sure migrations are sorted
	sort.Sort(byId(migrations))

	return migrations, nil
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
	Bytes(name string) []byte
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
			file := p.Box.Bytes(item)

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

// Wrapper for ExecMaxWithLock(); same behavior except max migrations is set to 0 (no limit)
func ExecWithLock(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, waitTime time.Duration) (int, error) {
	return ExecMaxWithLock(db, dialect, m, dir, 0, waitTime)
}

// Perform a migration while utilizing a simple db-based mutex.
//
// This functionality is useful if you are running more than 1 instance of your
// app that performs in-app migrations. This will make sure the migrations do
// not collide with eachother.
//
// When using this functionality, a single `sql-migrate` instance will be designated
// as the 'master migrator'; other instances will stay in 'waitState' and will
// wait until the lock is either:
//
// * Released (lock record is removed from the `gorp_lock` table)
//     * At which point, the 'waitState' migrators will exit cleanly (and not
//       perform any migrations)
//
// OR
//
// * The 'waitTime' is exceeded, in which case, `sql-migrate` instances in `waitState`
//   will return an error saying that they've exceeded the wait time.
//
// Finally, if for some reason your app crashes/gets killed before the lock was
// able to get cleaned up - the stale lock will be cleaned up on next start up.
//
//  Note: If you are running into the latter case, considering bumping up the `waitTime`.
func ExecMaxWithLock(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, max int, waitTime time.Duration) (int, error) {
	if dialect == "sqlite3" {
		return 0, errors.New("ExecWithLock does not support sqlite3 dialect")
	}

	dbMap, err := getMigrationDbMap(db, dialect)
	if err != nil {
		return 0, fmt.Errorf("Unable to instantiate dbmap: %v", err)
	}

	mlock, err := newMigrationLock(dbMap, waitTime)
	// Skip ExecMax if we encountered an error during newMigrationLock()
	if err != nil {
		return 0, err
	}

	// We are the master migrator so we must clean up our lock
	if !mlock.waitState {
		defer mlock.end()
	}

	return ExecMax(db, dialect, m, dir, max)
}

// Execute a set of migrations
//
// Returns the number of applied migrations.
func Exec(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection) (int, error) {
	return ExecMax(db, dialect, m, dir, 0)
}

type migrationLock struct {
	id        int
	dbMap     *gorp.DbMap
	waitState bool
	waitTime  time.Duration
}

// * check for (and delete) outdated lock
// * insert lock in db
// * if insert fails, means an existing lock is in place;
//     * set 'wait' to 'true'
//     * periodically check for lock existance ("master migrator" should remove lock when done)
//     * stop waiting if lock doesn't disappear
// * if insert succeeds, means we are the "master migrator"
//     * return from beginLock() -> let ExecMax do its thing;
//     * once ExecMax finishes, clean up our lock
func newMigrationLock(dbMap *gorp.DbMap, waitTime time.Duration) (*migrationLock, error) {
	mlock := &migrationLock{
		id:        time.Now().Nanosecond(),
		dbMap:     dbMap,
		waitState: false,
		waitTime:  waitTime,
	}

	// Remove potentially stale lock
	if err := mlock.removeStaleLock(); err != nil {
		return nil, err
	}

	insertErr := mlock.dbMap.Insert(&LockRecord{
		Lock:       lockName,
		AcquiredAt: time.Now(),
	})

	if insertErr != nil {
		// Insert failed, we are in 'wait' state; begin watching existing lock
		mlock.waitState = true

		if err := mlock.beginWatch(); err != nil {
			// We have exceeded 'waitTime', bail out
			return nil, err
		}

		// Lock was released; nothing to do
		return mlock, nil
	}

	// lock insertion succeeded, good to go
	return mlock, nil
}

// Remove a (potentially) stale lock
//
// Delete lock record if the lock is older than "now() - lockMaxStaleAge".
func (m *migrationLock) removeStaleLock() error {
	maxDate := time.Now().Add(-lockMaxStaleAge)

	_, err := m.dbMap.Exec("DELETE FROM gorp_lock WHERE acquired_at <= ?", maxDate)
	if err != nil {
		return fmt.Errorf("Unable to remove stale lock: %v", err)
	}

	return nil
}

// Periodically check for the existence of a 'lock' record
//
// If the lock record disappears before the 'waitTime' is up, return no error.
// If 'waitTime' is exceeded, return a 'wait time exceeded' error.
func (m *migrationLock) beginWatch() error {
	ticker := time.NewTicker(lockWatchInterval)
	defer ticker.Stop()

	beginTime := time.Now()

	for {
		<-ticker.C

		// Time waiting for lock clearance has elapsed
		if time.Since(beginTime) > m.waitTime {
			return fmt.Errorf("Exceeded lock clearance wait time (%v)", time.Since(beginTime))
		}

		var lockRecord LockRecord

		err := m.dbMap.SelectOne(&lockRecord, fmt.Sprintf("SELECT * FROM %v", lockTableName))
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}
	}

	return nil
}

// Remove lock record (if we are the 'master migrator')
func (m *migrationLock) end() {
	// Nothing to do if we were in 'waitState'
	if m.waitState {
		return
	}

	// perform lock clean up
	_, err := m.dbMap.Delete(&LockRecord{Lock: lockName})
	if err != nil {
		fmt.Printf("Ran into an error during lock cleanup: %v\n", err)
	}
}

// Execute a set of migrations
//
// Will apply at most `max` migrations. Pass 0 for no limit (or use Exec).
//
// Returns the number of applied migrations.
func ExecMax(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection, max int) (int, error) {
	migrations, dbMap, err := PlanMigration(db, dialect, m, dir, max)
	if err != nil {
		return 0, err
	}

	// Apply migrations
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

		for _, stmt := range migration.Queries {
			if _, err := executor.Exec(stmt); err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					trans.Rollback()
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
					trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		case Down:
			_, err := executor.Delete(&MigrationRecord{
				Id: migration.Id,
			})
			if err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					trans.Rollback()
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
	dbMap, err := getMigrationDbMap(db, dialect)
	if err != nil {
		return nil, nil, err
	}

	migrations, err := m.FindMigrations()
	if err != nil {
		return nil, nil, err
	}

	var migrationRecords []MigrationRecord
	_, err = dbMap.Select(&migrationRecords, fmt.Sprintf("SELECT * FROM %s", dbMap.Dialect.QuotedTableForQuery(schemaName, tableName)))
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

	// Get last migration that was run
	record := &Migration{}
	if len(existingMigrations) > 0 {
		record = existingMigrations[len(existingMigrations)-1]
	}

	result := make([]*PlannedMigration, 0)

	// Add missing migrations up to the last run migration.
	// This can happen for example when merges happened.
	if len(existingMigrations) > 0 {
		result = append(result, ToCatchup(migrations, existingMigrations, record)...)
	}

	// Figure out which migrations to apply
	toApply := ToApply(migrations, record.Id, dir)
	toApplyCount := len(toApply)
	if max > 0 && max < toApplyCount {
		toApplyCount = max
	}
	for _, v := range toApply[0:toApplyCount] {

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
				trans.Rollback()
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

func ToCatchup(migrations, existingMigrations []*Migration, lastRun *Migration) []*PlannedMigration {
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
			missing = append(missing, &PlannedMigration{
				Migration:          migration,
				Queries:            migration.Up,
				DisableTransaction: migration.DisableTransactionUp,
			})
		}
	}
	return missing
}

func GetMigrationRecords(db *sql.DB, dialect string) ([]*MigrationRecord, error) {
	dbMap, err := getMigrationDbMap(db, dialect)
	if err != nil {
		return nil, err
	}

	var records []*MigrationRecord
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", dbMap.Dialect.QuotedTableForQuery(schemaName, tableName))
	_, err = dbMap.Select(&records, query)
	if err != nil {
		return nil, err
	}

	return records, nil
}

func getMigrationDbMap(db *sql.DB, dialect string) (*gorp.DbMap, error) {
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
				err.Error() == "sql: Scan error on column index 0: unsupported Scan, storing driver.Value type []uint8 into type *time.Time" {
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
	dbMap.AddTableWithNameAndSchema(MigrationRecord{}, schemaName, tableName).SetKeys(false, "Id")
	//dbMap.TraceOn("", log.New(os.Stdout, "migrate: ", log.Lmicroseconds))

	// Create lock table
	dbMap.AddTableWithNameAndSchema(LockRecord{}, schemaName, lockTableName).SetKeys(false, "Lock").ColMap("Lock").SetUnique(true)

	err := dbMap.CreateTablesIfNotExists()
	if err != nil {
		return nil, err
	}

	return dbMap, nil
}

// TODO: Run migration + record insert in transaction.
