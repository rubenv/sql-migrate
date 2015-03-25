package migrate

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rubenv/gorp"
	. "gopkg.in/check.v1"
)

var filename = "/tmp/sql-migrate-sqlite.db"
var sqliteMigrations = []*Migration{
	&Migration{
		Id:   "123",
		Up:   []string{"CREATE TABLE people (id int)"},
		Down: []string{"DROP TABLE people"},
	},
	&Migration{
		Id:   "124",
		Up:   []string{"ALTER TABLE people ADD COLUMN first_name text"},
		Down: []string{"SELECT 0"}, // Not really supported
	},
}

type SqliteMigrateSuite struct {
	Db    *sql.DB
	DbMap *gorp.DbMap
}

var _ = Suite(&SqliteMigrateSuite{})

func (s *SqliteMigrateSuite) SetUpTest(c *C) {
	db, err := sql.Open("sqlite3", filename)
	c.Assert(err, IsNil)

	s.Db = db
	s.DbMap = &gorp.DbMap{Db: db, Dialect: &gorp.SqliteDialect{}}
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
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Can use table now
	_, err = s.DbMap.Exec("SELECT * FROM people")
	c.Assert(err, IsNil)

	// Shouldn't apply migration again
	n, err = Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func (s *SqliteMigrateSuite) TestMigrateMultiple(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: sqliteMigrations[:2],
	}

	// Executes two migrations
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
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
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Execute a new migration
	migrations = &MemoryMigrationSource{
		Migrations: sqliteMigrations[:2],
	}
	n, err = Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Can use column now
	_, err = s.DbMap.Exec("SELECT first_name FROM people")
	c.Assert(err, IsNil)
}

func (s *SqliteMigrateSuite) TestFileMigrate(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	// Executes two migrations
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Has data
	id, err := s.DbMap.SelectInt("SELECT id FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(1))
}

func (s *SqliteMigrateSuite) TestAssetMigrate(c *C) {
	migrations := &AssetMigrationSource{
		Asset:    Asset,
		AssetDir: AssetDir,
		Dir:      "test-migrations",
	}

	// Executes two migrations
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Has data
	id, err := s.DbMap.SelectInt("SELECT id FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(1))
}

func (s *SqliteMigrateSuite) TestMigrateMax(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	// Executes one migration
	n, err := ExecMax(s.Db, "sqlite3", migrations, Up, 1)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	id, err := s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(0))
}

func (s *SqliteMigrateSuite) TestMigrateDown(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Has data
	id, err := s.DbMap.SelectInt("SELECT id FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(1))

	// Undo the last one
	n, err = ExecMax(s.Db, "sqlite3", migrations, Down, 1)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// No more data
	id, err = s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(0))

	// Remove the table.
	n, err = ExecMax(s.Db, "sqlite3", migrations, Down, 1)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Cannot query it anymore
	_, err = s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, Not(IsNil))

	// Nothing left to do.
	n, err = ExecMax(s.Db, "sqlite3", migrations, Down, 1)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func (s *SqliteMigrateSuite) TestMigrateDownFull(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Has data
	id, err := s.DbMap.SelectInt("SELECT id FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(1))

	// Undo the last one
	n, err = Exec(s.Db, "sqlite3", migrations, Down)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Cannot query it anymore
	_, err = s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, Not(IsNil))

	// Nothing left to do.
	n, err = Exec(s.Db, "sqlite3", migrations, Down)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func (s *SqliteMigrateSuite) TestMigrateTransaction(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			sqliteMigrations[0],
			sqliteMigrations[1],
			&Migration{
				Id:   "125",
				Up:   []string{"INSERT INTO people (id, first_name) VALUES (1, 'Test')", "SELECT fail"},
				Down: []string{}, // Not important here
			},
		},
	}

	// Should fail, transaction should roll back the INSERT.
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, Not(IsNil))
	c.Assert(n, Equals, 2)

	// INSERT should be rolled back
	count, err := s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, IsNil)
	c.Assert(count, Equals, int64(0))
}

func (s *SqliteMigrateSuite) TestPlanMigration(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			&Migration{
				Id:   "1_create_table.sql",
				Up:   []string{"CREATE TABLE people (id int)"},
				Down: []string{"DROP TABLE people"},
			},
			&Migration{
				Id:   "2_alter_table.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN first_name text"},
				Down: []string{"SELECT 0"}, // Not really supported
			},
			&Migration{
				Id:   "10_add_last_name.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN last_name text"},
				Down: []string{"ALTER TABLE people DROP COLUMN last_name"},
			},
		},
	}
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 3)

	migrations.Migrations = append(migrations.Migrations, &Migration{
		Id: "11_add_middle_name.sql",
		Up: []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{
			"CREATE TEMPORARY TABLE people_backup(id int, first_name text, last_name text);",
			"INSERT INTO people_backup SELECT id, first_name, last_name  FROM people;",
			"DROP TABLE people;",
			"ALTER TABLE people_backup RENAME TO people;",
		},
	})

	plannedMigrations, _, err := PlanMigration(s.Db, "sqlite3", migrations, Up, 0)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 1)
	c.Assert(plannedMigrations[0].Migration, Equals, migrations.Migrations[3])

	_, err = Exec(s.Db, "sqlite3", migrations, Up)

	migrations.Migrations = migrations.Migrations[0:3] //simulate redployig app without last sql file

	plannedMigrations, _, err = PlanMigration(s.Db, "sqlite3", migrations, Up, 0)
	c.Assert(plannedMigrations, HasLen, 1)
	n, err = Exec(s.Db, "sqlite3", migrations, Up)

	plannedMigrations, _, err = PlanMigration(s.Db, "sqlite3", migrations, Up, 0)
	c.Assert(plannedMigrations, HasLen, 0)

	plannedMigrations, _, err = PlanMigration(s.Db, "sqlite3", migrations, Down, 0)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 3)
	c.Assert(plannedMigrations[0].Migration, Equals, migrations.Migrations[2])
	c.Assert(plannedMigrations[1].Migration, Equals, migrations.Migrations[1])
	c.Assert(plannedMigrations[2].Migration, Equals, migrations.Migrations[0])

}
