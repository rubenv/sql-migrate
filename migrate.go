package migrate

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-gorp/gorp"
	"github.com/rubenv/sql-migrate/sqlparse"
)

type MigrationDirection int

const (
	Up MigrationDirection = iota
	Down
)

var tableName = "gorp_migrations"
var numberPrefixRegex = regexp.MustCompile(`^(\d+).*$`)

// Set the name of the table used to store migration info.
//
// Should be called before any other call such as (Exec, ExecMax, ...).
func SetTable(name string) {
	if name != "" {
		tableName = name
	}
}

type Migration struct {
	Id   string
	Up   []string
	Down []string
}

func (m Migration) Less(other *Migration) bool {
	switch {
	case m.isNumeric() && other.isNumeric():
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
	Queries []string
}

type byId []*Migration

func (b byId) Len() int           { return len(b) }
func (b byId) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byId) Less(i, j int) bool { return b[i].Less(b[j]) }

type MigrationRecord struct {
	Id        string    `db:"id"`
	AppliedAt time.Time `db:"applied_at"`
	DownSql   string    `db:"down_sql"`
}

var MigrationDialects = map[string]gorp.Dialect{
	"sqlite3":  gorp.SqliteDialect{},
	"postgres": gorp.PostgresDialect{},
	"mysql":    gorp.MySQLDialect{"InnoDB", "UTF8"},
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
	// Make sure migrations are sorted
	sort.Sort(byId(m.Migrations))

	return m.Migrations, nil
}

// A set of migrations loaded from a directory.
type FileMigrationSource struct {
	Dir string
}

var _ MigrationSource = (*FileMigrationSource)(nil)

func (f FileMigrationSource) FindMigrations() ([]*Migration, error) {
	migrations := make([]*Migration, 0)

	file, err := os.Open(f.Dir)
	if err != nil {
		return nil, err
	}

	files, err := file.Readdir(0)
	if err != nil {
		return nil, err
	}

	for _, info := range files {
		if strings.HasSuffix(info.Name(), ".sql") {
			file, err := os.Open(path.Join(f.Dir, info.Name()))
			if err != nil {
				return nil, err
			}

			migration, err := ParseMigration(info.Name(), file)
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

// Migration parsing
func ParseMigration(id string, r io.ReadSeeker) (*Migration, error) {
	m := &Migration{
		Id: id,
	}

	up, err := sqlparse.SplitSQLStatements(r, true)
	if err != nil {
		return nil, err
	}

	down, err := sqlparse.SplitSQLStatements(r, false)
	if err != nil {
		return nil, err
	}

	m.Up = up
	m.Down = down

	return m, nil
}

// Execute a set of migrations
//
// Returns the number of applied migrations.
func Exec(db *sql.DB, dialect string, m MigrationSource, dir MigrationDirection) (int, error) {
	return ExecMax(db, dialect, m, dir, 0)
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
		trans, err := dbMap.Begin()
		if err != nil {
			return applied, err
		}

		for _, stmt := range migration.Queries {
			_, err := trans.Exec(stmt)
			if err != nil {
				trans.Rollback()
				return applied, err
			}
		}

		if dir == Up {
			if len(migration.Migration.Up) != 0 {
				//this is a real up - insert the record
				err = trans.Insert(&MigrationRecord{
					Id:        migration.Id,
					AppliedAt: time.Now(),
					DownSql:   strings.Join(migration.Down, "\n"),
				})
			} else {
				//no up query means this is supposed ot prune thi smigration from db
				_, err = trans.Delete(&MigrationRecord{
					Id: migration.Id,
				})
			}
			if err != nil {
				return applied, err
			}
		} else if dir == Down {
			_, err := trans.Delete(&MigrationRecord{
				Id: migration.Id,
			})
			if err != nil {
				return applied, err
			}
		} else {
			panic("Not possible")
		}

		err = trans.Commit()
		if err != nil {
			return applied, err
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
	_, err = dbMap.Select(&migrationRecords, fmt.Sprintf("SELECT * FROM %s", tableName))
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

	// Figure out which of the supplied migrations has been applied.
	toApply := ToApply(migrations, record.Id, dir)
	toApplyCount := len(toApply)
	if max > 0 && max < toApplyCount {
		toApplyCount = max
	}

	result := make([]*PlannedMigration, toApplyCount)
	for k, v := range toApply[0:toApplyCount] {
		result[k] = &PlannedMigration{
			Migration: v,
		}

		if dir == Up {
			result[k].Queries = v.Up
		} else if dir == Down {
			result[k].Queries = v.Down
		}
	}

	// if we are downgrading our main app, apply the downs of the
	// new migrations that have been run. We know a migration is no
	// longer relevant if the file that it ran from no longer exists
	migrationsToRemove := ToRemove(migrations, migrationRecords)
	for _, v := range migrationsToRemove {
		result = append(result, &PlannedMigration{
			Migration: v,
			Queries:   v.Down,
		})

	}

	return result, dbMap, nil
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

// Filter a slice of migrations into ones that should be removed.
// A migrations should be removed if the .sql file no longer exists or
// the  MigrationSource no longer has the migration but it still exists in the DB.
// This can happen if an older version of the Application is deployed in a rollback scenario
func ToRemove(migrations []*Migration, migrationRecords []MigrationRecord) []*Migration {
	missingMigrations := make([]*Migration, 0)

	for _, mr := range migrationRecords {
		var migrationExists = false
		for _, m := range migrations {
			if m.Id == mr.Id {
				migrationExists = true
				break
			}
		}
		//fmt.Println(mr, ": ", migrationExists)
		if !migrationExists {
			var m Migration
			m.Id = mr.Id
			m.Down = strings.Split(mr.DownSql, "\n")
			missingMigrations = append(missingMigrations, &m)
		}
	}

	// Add in reverse order
	index := len(missingMigrations) - 1
	toRemove := make([]*Migration, index+1)
	for i := 0; i <= index; i++ {
		toRemove[index-i] = missingMigrations[i]
	}
	return toRemove

	panic("Not possible")
}

func GetMigrationRecords(db *sql.DB, dialect string) ([]*MigrationRecord, error) {
	dbMap, err := getMigrationDbMap(db, dialect)
	if err != nil {
		return nil, err
	}

	var records []*MigrationRecord
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", tableName)
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
			if err.Error() == "sql: Scan error on column index 0: unsupported driver -> Scan pair: []uint8 -> *time.Time" {
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
	dbMap.AddTableWithName(MigrationRecord{}, tableName).SetKeys(false, "Id")
	//dbMap.TraceOn("", log.New(os.Stdout, "migrate: ", log.Lmicroseconds))

	err := dbMap.CreateTablesIfNotExists()
	if err != nil {
		return nil, err
	}

	return dbMap, nil
}

// TODO: Run migration + record insert in transaction.
