package migrate

import (
	"database/sql"
	"os"

	"github.com/coopernurse/gorp"
	_ "github.com/mattn/go-sqlite3"
	. "gopkg.in/check.v1"
)

var filename = "/tmp/gorp-migrate-sqlite.db"
var sqliteMigrations = []*Migration{
	&Migration{
		Id:   "123",
		Up:   "CREATE TABLE people (id int)",
		Down: "DROP TABLE people",
	},
	&Migration{
		Id:   "124",
		Up:   "ALTER TABLE people ADD COLUMN first_name text",
		Down: "SELECT 0", // Not really supported
	},
}

type SqliteMigrateSuite struct {
	DbMap *gorp.DbMap
}

var _ = Suite(&SqliteMigrateSuite{})

func (s *SqliteMigrateSuite) SetUpTest(c *C) {
	db, err := sql.Open("sqlite3", filename)
	c.Assert(err, IsNil)

	s.DbMap = &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}
}

func (s *SqliteMigrateSuite) TearDownTest(c *C) {
	err := os.Remove(filename)
	c.Assert(err, IsNil)
}

func (s *SqliteMigrateSuite) TestRunMigration(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: sqliteMigrations[:1],
	}

	// Executes one migration
	n, err := Exec(s.DbMap, migrations)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Can use table now
	_, err = s.DbMap.Exec("SELECT * FROM people")
	c.Assert(err, IsNil)

	// Shouldn't apply migration again
	n, err = Exec(s.DbMap, migrations)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func (s *SqliteMigrateSuite) TestMigrateMultiple(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: sqliteMigrations[:2],
	}

	// Executes one migration
	n, err := Exec(s.DbMap, migrations)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Can use column now
	_, err = s.DbMap.Exec("SELECT first_name FROM people")
	c.Assert(err, IsNil)
}

func (s *SqliteMigrateSuite) TestMigrateIncremental(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: sqliteMigrations[:1],
	}

	// Executes one migration
	n, err := Exec(s.DbMap, migrations)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Execute a new migration
	migrations = &MemoryMigrationSource{
		Migrations: sqliteMigrations[:2],
	}
	n, err = Exec(s.DbMap, migrations)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Can use column now
	_, err = s.DbMap.Exec("SELECT first_name FROM people")
	c.Assert(err, IsNil)
}
