package migrate

import (
	"database/sql"
	"sort"
	"time"

	"github.com/coopernurse/gorp"
)

type Migration struct {
	Id        string
	Up        string
	Down      string
	AppliedAt time.Time
}

type ById []*Migration

func (b ById) Len() int           { return len(b) }
func (b ById) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b ById) Less(i, j int) bool { return b[i].Id < b[j].Id }

type MigrationRecord struct {
	Id        string    `db:"id"`
	Up        string    `db:"-"`
	Down      string    `db:"-"`
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

// Execute a set of migrations
func Exec(db *gorp.DbMap, m MigrationSource) error {
	dbMap := &gorp.DbMap{Db: db.Db, Dialect: db.Dialect}
	dbMap.AddTableWithName(MigrationRecord{}, "gorp_migrations").SetKeys(false, "Id")

	// Make sure we have the migrations table
	err := dbMap.CreateTablesIfNotExists()
	if err != nil {
		return err
	}

	migrations, err := m.FindMigrations()
	if err != nil {
		return err
	}

	// Make sure migrations are sorted
	sort.Sort(ById(migrations))

	// Find the newest applied migration
	var record MigrationRecord
	err = dbMap.SelectOne(&record, "SELECT * FROM gorp_migrations ORDER BY id DESC LIMIT 1")
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	var index = -1
	for index < len(migrations) && migrations[index].Id <= record.Id {
		index++
	}

	return nil
}
