package migrate

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/coopernurse/gorp"
	"github.com/rubenv/gorp-migrate/sqlparse"
)

type MigrationDirection int

const (
	Up MigrationDirection = iota
	Down
)

type Migration struct {
	Id   string
	Up   []string
	Down []string
}

type PlannedMigration struct {
	Id      string
	Queries []string
}

type byId []*Migration

func (b byId) Len() int           { return len(b) }
func (b byId) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byId) Less(i, j int) bool { return b[i].Id < b[j].Id }

type MigrationRecord struct {
	Id        string    `db:"id"`
	AppliedAt time.Time `db:"applied_at"`
}

type MigrationSource interface {
	FindMigrations() ([]*Migration, error)
}

// A hardcoded set of migrations, in-memory.
type MemoryMigrationSource struct {
	Migrations []*Migration
}

var _ MigrationSource = (*MemoryMigrationSource)(nil)

func (m MemoryMigrationSource) FindMigrations() ([]*Migration, error) {
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
func Exec(db *gorp.DbMap, m MigrationSource, dir MigrationDirection) (int, error) {
	return ExecMax(db, m, dir, 0)
}

// Execute a set of migrations
//
// Will apply at most `max` migrations. Pass 0 for no limit (or use Exec).
//
// Returns the number of applied migrations.
func ExecMax(db *gorp.DbMap, m MigrationSource, dir MigrationDirection, max int) (int, error) {
	migrations, dbMap, err := PlanMigration(db, m, dir, max)
	if err != nil {
		return 0, err
	}

	// Apply migrations
	applied := 0
	for _, migration := range migrations {
		for _, stmt := range migration.Queries {
			_, err := dbMap.Exec(stmt)
			if err != nil {
				return applied, err
			}
		}

		err = dbMap.Insert(&MigrationRecord{
			Id:        migration.Id,
			AppliedAt: time.Now(),
		})
		if err != nil {
			return applied, err
		}

		applied++
	}

	return applied, nil
}

// Plan a migration.
func PlanMigration(db *gorp.DbMap, m MigrationSource, dir MigrationDirection, max int) ([]*PlannedMigration, *gorp.DbMap, error) {
	dbMap := &gorp.DbMap{Db: db.Db, Dialect: db.Dialect}
	dbMap.AddTableWithName(MigrationRecord{}, "gorp_migrations").SetKeys(false, "Id")
	//dbMap.TraceOn("", log.New(os.Stdout, "migrate: ", log.Lmicroseconds))

	// Make sure we have the migrations table
	err := dbMap.CreateTablesIfNotExists()
	if err != nil {
		return nil, nil, err
	}

	migrations, err := m.FindMigrations()
	if err != nil {
		return nil, nil, err
	}

	// Make sure migrations are sorted
	sort.Sort(byId(migrations))

	// Find the newest applied migration
	var record MigrationRecord
	err = dbMap.SelectOne(&record, "SELECT * FROM gorp_migrations ORDER BY id DESC LIMIT 1")
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, err
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
			Id: v.Id,
		}

		if dir == Up {
			result[k].Queries = v.Up
		} else if dir == Down {
			result[k].Queries = v.Down
		}
	}

	return result, dbMap, nil
}

// Filter a slice of migrations into ones that should be applied.
func ToApply(migrations []*Migration, current string, direction MigrationDirection) []*Migration {
	var index = -1
	for index < len(migrations)-1 && migrations[index+1].Id <= current {
		index++
	}
	toApply := migrations[index+1:]
	return toApply
}

// TODO: Run migration + record insert in transaction.
