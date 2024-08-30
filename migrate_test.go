package migrate

import (
	"context"
	"database/sql"
	"embed"
	"net/http"
	"time"

	"github.com/go-gorp/gorp/v3"
	//revive:disable-next-line:dot-imports
	. "gopkg.in/check.v1"

	_ "github.com/mattn/go-sqlite3"
)

var sqliteMigrations = []*Migration{
	{
		Id:   "123",
		Up:   []string{"CREATE TABLE people (id int)"},
		Down: []string{"DROP TABLE people"},
	},
	{
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
	var err error
	db, err := sql.Open("sqlite3", ":memory:")
	c.Assert(err, IsNil)

	s.Db = db
	s.DbMap = &gorp.DbMap{Db: db, Dialect: &gorp.SqliteDialect{}}
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

func (s *SqliteMigrateSuite) TestRunMigrationEscapeTable(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: sqliteMigrations[:1],
	}

	SetTable(`my migrations`)

	// Executes one migration
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)
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

	LimitTimePrecision(true)
	defer func() {
		LimitTimePrecision(false)
	}()
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

func (s *SqliteMigrateSuite) TestHttpFileSystemMigrate(c *C) {
	migrations := &HttpFileSystemMigrationSource{
		FileSystem: http.Dir("test-migrations"),
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

func (s *SqliteMigrateSuite) TestMigrateVersionInt(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	// Executes migration with target version 1
	n, err := ExecVersion(s.Db, "sqlite3", migrations, Up, 1)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	id, err := s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(0))
}

func (s *SqliteMigrateSuite) TestMigrateVersionInt2(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	// Executes migration with target version 2
	n, err := ExecVersion(s.Db, "sqlite3", migrations, Up, 2)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	id, err := s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(1))
}

func (s *SqliteMigrateSuite) TestMigrateVersionIntFailedWithNotExistingVerion(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	// Executes migration with not existing version 3
	_, err := ExecVersion(s.Db, "sqlite3", migrations, Up, 3)
	c.Assert(err, NotNil)
}

func (s *SqliteMigrateSuite) TestMigrateVersionIntFailedWithInvalidVerion(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	// Executes migration with invalid version -1
	_, err := ExecVersion(s.Db, "sqlite3", migrations, Up, -1)
	c.Assert(err, NotNil)
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
			{
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
			{
				Id:   "1_create_table.sql",
				Up:   []string{"CREATE TABLE people (id int)"},
				Down: []string{"DROP TABLE people"},
			},
			{
				Id:   "2_alter_table.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN first_name text"},
				Down: []string{"SELECT 0"}, // Not really supported
			},
			{
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
		Id:   "11_add_middle_name.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})

	plannedMigrations, _, err := PlanMigration(s.Db, "sqlite3", migrations, Up, 0)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 1)
	c.Assert(plannedMigrations[0].Migration, Equals, migrations.Migrations[3])

	plannedMigrations, _, err = PlanMigration(s.Db, "sqlite3", migrations, Down, 0)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 3)
	c.Assert(plannedMigrations[0].Migration, Equals, migrations.Migrations[2])
	c.Assert(plannedMigrations[1].Migration, Equals, migrations.Migrations[1])
	c.Assert(plannedMigrations[2].Migration, Equals, migrations.Migrations[0])
}

func (s *SqliteMigrateSuite) TestSkipMigration(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			{
				Id:   "1_create_table.sql",
				Up:   []string{"CREATE TABLE people (id int)"},
				Down: []string{"DROP TABLE people"},
			},
			{
				Id:   "2_alter_table.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN first_name text"},
				Down: []string{"SELECT 0"}, // Not really supported
			},
			{
				Id:   "10_add_last_name.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN last_name text"},
				Down: []string{"ALTER TABLE people DROP COLUMN last_name"},
			},
		},
	}
	LimitTimePrecision(true)
	defer func() {
		LimitTimePrecision(false)
	}()
	n, err := SkipMax(s.Db, "sqlite3", migrations, Up, 0)
	// there should be no errors
	c.Assert(err, IsNil)
	// we should have detected and skipped 3 migrations
	c.Assert(n, Equals, 3)
	// should not actually have the tables now since it was skipped
	// so this query should fail
	_, err = s.DbMap.Exec("SELECT * FROM people")
	c.Assert(err, NotNil)
	// run the migrations again, should execute none of them since we pegged the db level
	// in the skip command
	n2, err2 := Exec(s.Db, "sqlite3", migrations, Up)
	// there should be no errors
	c.Assert(err2, IsNil)
	// we should not have executed any migrations
	c.Assert(n2, Equals, 0)
}

func (s *SqliteMigrateSuite) TestPlanMigrationWithHoles(c *C) {
	up := "SELECT 0"
	down := "SELECT 1"
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			{
				Id:   "1",
				Up:   []string{up},
				Down: []string{down},
			},
			{
				Id:   "3",
				Up:   []string{up},
				Down: []string{down},
			},
		},
	}
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	migrations.Migrations = append(migrations.Migrations, &Migration{
		Id:   "2",
		Up:   []string{up},
		Down: []string{down},
	})

	migrations.Migrations = append(migrations.Migrations, &Migration{
		Id:   "4",
		Up:   []string{up},
		Down: []string{down},
	})

	migrations.Migrations = append(migrations.Migrations, &Migration{
		Id:   "5",
		Up:   []string{up},
		Down: []string{down},
	})

	// apply all the missing migrations
	plannedMigrations, _, err := PlanMigration(s.Db, "sqlite3", migrations, Up, 0)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 3)
	c.Assert(plannedMigrations[0].Migration.Id, Equals, "2")
	c.Assert(plannedMigrations[0].Queries[0], Equals, up)
	c.Assert(plannedMigrations[1].Migration.Id, Equals, "4")
	c.Assert(plannedMigrations[1].Queries[0], Equals, up)
	c.Assert(plannedMigrations[2].Migration.Id, Equals, "5")
	c.Assert(plannedMigrations[2].Queries[0], Equals, up)

	// first catch up to current target state 123, then migrate down 1 step to 12
	plannedMigrations, _, err = PlanMigration(s.Db, "sqlite3", migrations, Down, 1)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 2)
	c.Assert(plannedMigrations[0].Migration.Id, Equals, "2")
	c.Assert(plannedMigrations[0].Queries[0], Equals, up)
	c.Assert(plannedMigrations[1].Migration.Id, Equals, "3")
	c.Assert(plannedMigrations[1].Queries[0], Equals, down)

	// first catch up to current target state 123, then migrate down 2 steps to 1
	plannedMigrations, _, err = PlanMigration(s.Db, "sqlite3", migrations, Down, 2)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 3)
	c.Assert(plannedMigrations[0].Migration.Id, Equals, "2")
	c.Assert(plannedMigrations[0].Queries[0], Equals, up)
	c.Assert(plannedMigrations[1].Migration.Id, Equals, "3")
	c.Assert(plannedMigrations[1].Queries[0], Equals, down)
	c.Assert(plannedMigrations[2].Migration.Id, Equals, "2")
	c.Assert(plannedMigrations[2].Queries[0], Equals, down)
}

func (*SqliteMigrateSuite) TestLess(c *C) {
	c.Assert((Migration{Id: "1"}).Less(&Migration{Id: "2"}), Equals, true)           // 1 less than 2
	c.Assert((Migration{Id: "2"}).Less(&Migration{Id: "1"}), Equals, false)          // 2 not less than 1
	c.Assert((Migration{Id: "1"}).Less(&Migration{Id: "a"}), Equals, true)           // 1 less than a
	c.Assert((Migration{Id: "a"}).Less(&Migration{Id: "1"}), Equals, false)          // a not less than 1
	c.Assert((Migration{Id: "a"}).Less(&Migration{Id: "a"}), Equals, false)          // a not less than a
	c.Assert((Migration{Id: "1-a"}).Less(&Migration{Id: "1-b"}), Equals, true)       // 1-a less than 1-b
	c.Assert((Migration{Id: "1-b"}).Less(&Migration{Id: "1-a"}), Equals, false)      // 1-b not less than 1-a
	c.Assert((Migration{Id: "1"}).Less(&Migration{Id: "10"}), Equals, true)          // 1 less than 10
	c.Assert((Migration{Id: "10"}).Less(&Migration{Id: "1"}), Equals, false)         // 10 not less than 1
	c.Assert((Migration{Id: "1_foo"}).Less(&Migration{Id: "10_bar"}), Equals, true)  // 1_foo not less than 1
	c.Assert((Migration{Id: "10_bar"}).Less(&Migration{Id: "1_foo"}), Equals, false) // 10 not less than 1
	// 20160126_1100 less than 20160126_1200
	c.Assert((Migration{Id: "20160126_1100"}).
		Less(&Migration{Id: "20160126_1200"}), Equals, true)
	// 20160126_1200 not less than 20160126_1100
	c.Assert((Migration{Id: "20160126_1200"}).
		Less(&Migration{Id: "20160126_1100"}), Equals, false)
}

func (s *SqliteMigrateSuite) TestPlanMigrationWithUnknownDatabaseMigrationApplied(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			{
				Id:   "1_create_table.sql",
				Up:   []string{"CREATE TABLE people (id int)"},
				Down: []string{"DROP TABLE people"},
			},
			{
				Id:   "2_alter_table.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN first_name text"},
				Down: []string{"SELECT 0"}, // Not really supported
			},
			{
				Id:   "10_add_last_name.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN last_name text"},
				Down: []string{"ALTER TABLE people DROP COLUMN last_name"},
			},
		},
	}
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 3)

	// Note that migration 10_add_last_name.sql is missing from the new migrations source
	// so it is considered an "unknown" migration for the planner.
	migrations.Migrations = append(migrations.Migrations[:2], &Migration{
		Id:   "10_add_middle_name.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})

	_, _, err = PlanMigration(s.Db, "sqlite3", migrations, Up, 0)
	c.Assert(err, NotNil, Commentf("Up migrations should not have been applied when there "+
		"is an unknown migration in the database"))
	c.Assert(err, FitsTypeOf, &PlanError{})

	_, _, err = PlanMigration(s.Db, "sqlite3", migrations, Down, 0)
	c.Assert(err, NotNil, Commentf("Down migrations should not have been applied when there "+
		"is an unknown migration in the database"))
	c.Assert(err, FitsTypeOf, &PlanError{})
}

func (s *SqliteMigrateSuite) TestPlanMigrationWithIgnoredUnknownDatabaseMigrationApplied(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			{
				Id:   "1_create_table.sql",
				Up:   []string{"CREATE TABLE people (id int)"},
				Down: []string{"DROP TABLE people"},
			},
			{
				Id:   "2_alter_table.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN first_name text"},
				Down: []string{"SELECT 0"}, // Not really supported
			},
			{
				Id:   "10_add_last_name.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN last_name text"},
				Down: []string{"ALTER TABLE people DROP COLUMN last_name"},
			},
		},
	}
	SetIgnoreUnknown(true)
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 3)

	// Note that migration 10_add_last_name.sql is missing from the new migrations source
	// so it is considered an "unknown" migration for the planner.
	migrations.Migrations = append(migrations.Migrations[:2], &Migration{
		Id:   "10_add_middle_name.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})

	_, _, err = PlanMigration(s.Db, "sqlite3", migrations, Up, 0)
	c.Assert(err, IsNil)

	_, _, err = PlanMigration(s.Db, "sqlite3", migrations, Down, 0)
	c.Assert(err, IsNil)
	SetIgnoreUnknown(false) // Make sure we are not breaking other tests as this is globaly set
}

func (s *SqliteMigrateSuite) TestPlanMigrationToVersion(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			{
				Id:   "1_create_table.sql",
				Up:   []string{"CREATE TABLE people (id int)"},
				Down: []string{"DROP TABLE people"},
			},
			{
				Id:   "2_alter_table.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN first_name text"},
				Down: []string{"SELECT 0"}, // Not really supported
			},
			{
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
		Id:   "11_add_middle_name.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})

	plannedMigrations, _, err := PlanMigrationToVersion(s.Db, "sqlite3", migrations, Up, 11)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 1)
	c.Assert(plannedMigrations[0].Migration, Equals, migrations.Migrations[3])

	plannedMigrations, _, err = PlanMigrationToVersion(s.Db, "sqlite3", migrations, Down, 1)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 3)
	c.Assert(plannedMigrations[0].Migration, Equals, migrations.Migrations[2])
	c.Assert(plannedMigrations[1].Migration, Equals, migrations.Migrations[1])
	c.Assert(plannedMigrations[2].Migration, Equals, migrations.Migrations[0])
}

// TestExecWithUnknownMigrationInDatabase makes sure that problems found with planning the
// migrations are propagated and returned by Exec.
func (s *SqliteMigrateSuite) TestExecWithUnknownMigrationInDatabase(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: sqliteMigrations[:2],
	}

	// Executes two migrations
	n, err := Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Then create a new migration source with one of the migrations missing
	newSqliteMigrations := []*Migration{
		{
			Id:   "124_other",
			Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
			Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
		},
		{
			Id:   "125",
			Up:   []string{"ALTER TABLE people ADD COLUMN age int"},
			Down: []string{"ALTER TABLE people DROP COLUMN age"},
		},
	}
	migrations = &MemoryMigrationSource{
		Migrations: append(sqliteMigrations[:1], newSqliteMigrations...),
	}

	n, err = Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, NotNil, Commentf("Migrations should not have been applied when there "+
		"is an unknown migration in the database"))
	c.Assert(err, FitsTypeOf, &PlanError{})
	c.Assert(n, Equals, 0)

	// Make sure the new columns are not actually created
	_, err = s.DbMap.Exec("SELECT middle_name FROM people")
	c.Assert(err, NotNil)
	_, err = s.DbMap.Exec("SELECT age FROM people")
	c.Assert(err, NotNil)
}

func (s *SqliteMigrateSuite) TestRunMigrationObjDefaultTable(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: sqliteMigrations[:1],
	}

	ms := MigrationSet{}
	// Executes one migration
	n, err := ms.Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Can use table now
	_, err = s.DbMap.Exec("SELECT * FROM people")
	c.Assert(err, IsNil)

	// Uses default tableName
	_, err = s.DbMap.Exec("SELECT * FROM gorp_migrations")
	c.Assert(err, IsNil)

	// Shouldn't apply migration again
	n, err = ms.Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func (s *SqliteMigrateSuite) TestRunMigrationObjOtherTable(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: sqliteMigrations[:1],
	}

	ms := MigrationSet{TableName: "other_migrations"}
	// Executes one migration
	n, err := ms.Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Can use table now
	_, err = s.DbMap.Exec("SELECT * FROM people")
	c.Assert(err, IsNil)

	// Uses default tableName
	_, err = s.DbMap.Exec("SELECT * FROM other_migrations")
	c.Assert(err, IsNil)

	// Shouldn't apply migration again
	n, err = ms.Exec(s.Db, "sqlite3", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func (*SqliteMigrateSuite) TestSetDisableCreateTable(c *C) {
	c.Assert(migSet.DisableCreateTable, Equals, false)

	SetDisableCreateTable(true)
	c.Assert(migSet.DisableCreateTable, Equals, true)

	SetDisableCreateTable(false)
	c.Assert(migSet.DisableCreateTable, Equals, false)
}

func (s *SqliteMigrateSuite) TestGetMigrationDbMapWithDisableCreateTable(c *C) {
	SetDisableCreateTable(false)

	_, err := migSet.getMigrationDbMap(s.Db, "postgres")
	c.Assert(err, IsNil)
}

// If ms.DisableCreateTable == true, then the the migrations table should not be
// created, regardless of the global migSet.DisableCreateTable setting.
func (s *SqliteMigrateSuite) TestGetMigrationObjDbMapWithDisableCreateTableTrue(c *C) {
	SetDisableCreateTable(false)
	ms := MigrationSet{
		DisableCreateTable: true,
		TableName:          "silly_example_table",
	}
	c.Assert(migSet.DisableCreateTable, Equals, false)
	c.Assert(ms.DisableCreateTable, Equals, true)

	dbMap, err := ms.getMigrationDbMap(s.Db, "sqlite3")
	c.Assert(err, IsNil)
	c.Assert(dbMap, NotNil)

	tableNameIfExists, err := s.DbMap.SelectNullStr(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=$1",
		ms.TableName,
	)
	c.Assert(err, IsNil)
	c.Assert(tableNameIfExists.Valid, Equals, false)
}

// If ms.DisableCreateTable == false, then the the migrations table should not be
// created, regardless of the global migSet.DisableCreateTable setting.
func (s *SqliteMigrateSuite) TestGetMigrationObjDbMapWithDisableCreateTableFalse(c *C) {
	SetDisableCreateTable(true)
	defer SetDisableCreateTable(false) // reset the global state when the test ends.
	ms := MigrationSet{
		DisableCreateTable: false,
		TableName:          "silly_example_table",
	}
	c.Assert(migSet.DisableCreateTable, Equals, true)
	c.Assert(ms.DisableCreateTable, Equals, false)

	dbMap, err := ms.getMigrationDbMap(s.Db, "sqlite3")
	c.Assert(err, IsNil)
	c.Assert(dbMap, NotNil)

	tableNameIfExists, err := s.DbMap.SelectNullStr(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=$1",
		ms.TableName,
	)
	c.Assert(err, IsNil)
	c.Assert(tableNameIfExists.Valid, Equals, true)
	c.Assert(tableNameIfExists.String, Equals, ms.TableName)
}

func (s *SqliteMigrateSuite) TestContextTimeout(c *C) {
	// This statement will run for a long time: 1,000,000 iterations of the fibonacci sequence
	fibonacciLoopStmt := `WITH RECURSIVE
	   fibo (curr, next)
	 AS
	   ( SELECT 1,1
	     UNION ALL
	     SELECT next, curr+next FROM fibo
	     LIMIT 1000000 )
	 SELECT group_concat(curr) FROM fibo;
	`
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			sqliteMigrations[0],
			sqliteMigrations[1],
			{
				Id:   "125",
				Up:   []string{fibonacciLoopStmt},
				Down: []string{}, // Not important here
			},
			{
				Id:   "125",
				Up:   []string{"INSERT INTO people (id, first_name) VALUES (1, 'Test')", "SELECT fail"},
				Down: []string{}, // Not important here
			},
		},
	}

	// Should never run the insert
	ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelFunc()
	n, err := ExecContext(ctx, s.Db, "sqlite3", migrations, Up)
	c.Assert(err, Not(IsNil))
	c.Assert(n, Equals, 2)
}

//go:embed test-migrations/*
var testEmbedFS embed.FS

func (s *SqliteMigrateSuite) TestEmbedSource(c *C) {
	migrations := EmbedFileSystemMigrationSource{
		FileSystem: testEmbedFS,
		Root:       "test-migrations",
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
