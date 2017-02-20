package migrate

import (
	"database/sql"
	"flag"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	. "gopkg.in/check.v1"
	"gopkg.in/gorp.v1"
)

var enableMySQLFlag = flag.Bool("enable-mysql", false, "Perform mysql tests (default=false)")

var (
	testDBName = "test_db"
	testDBHost = "127.0.0.1"
	testDBPort = "3306"
	testDBUser = "root"
	testDBPass = ""

	testDBDSN = fmt.Sprintf("%v:%v@tcp(%v:%v)/?parseTime=true&timeout=10s",
		testDBUser, testDBPass, testDBHost, testDBPort)

	testDBFullDSN = fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?parseTime=true&timeout=10s",
		testDBUser, testDBPass, testDBHost, testDBPort, testDBName)
)

var mysqlMigrations = []*Migration{
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

type MySQLMigrateSuite struct {
	Db    *sql.DB
	DbMap *gorp.DbMap
}

var _ = Suite(&MySQLMigrateSuite{})

// Drop initial DB (if found)
func (s *MySQLMigrateSuite) SetUpSuite(c *C) {
	if !*enableMySQLFlag {
		c.Skip("Skipping mysql tests due to -enable-mysql flag not being set")
	}

	db, err := sql.Open("mysql", testDBDSN)
	c.Assert(err, IsNil)

	db.Exec(fmt.Sprintf("DROP DATABASE `%v`", testDBName))
	c.Assert(db.Close(), IsNil)
}

func (s *MySQLMigrateSuite) SetUpTest(c *C) {
	// Initial connection without DB
	initialDB, err := sql.Open("mysql", testDBDSN)
	c.Assert(err, IsNil)

	_, dbCreateErr := initialDB.Exec(fmt.Sprintf("CREATE DATABASE `%v`", testDBName))
	c.Assert(dbCreateErr, IsNil)

	initialDB.Close()

	// final connect
	db, err := sql.Open("mysql", testDBFullDSN)
	c.Assert(err, IsNil)

	s.Db = db
	s.DbMap = &gorp.DbMap{Db: db, Dialect: &gorp.MySQLDialect{}}
}

func (s *MySQLMigrateSuite) TearDownTest(c *C) {
	_, err := s.Db.Exec(fmt.Sprintf("DROP DATABASE `%v`", testDBName))
	c.Assert(err, IsNil)
}

func (s *MySQLMigrateSuite) TestRunMigration(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: mysqlMigrations[:1],
	}

	// Executes one migration
	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Can use table now
	_, err = s.DbMap.Exec("SELECT * FROM people")
	c.Assert(err, IsNil)

	// Shouldn't apply migration again
	n, err = Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func (s *MySQLMigrateSuite) TestRunMigrationEscapeTable(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: mysqlMigrations[:1],
	}

	SetTable(`my migrations`)

	// Executes one migration
	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)
}

func (s *MySQLMigrateSuite) TestMigrateMultiple(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: mysqlMigrations[:2],
	}

	// Executes two migrations
	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Can use column now
	_, err = s.DbMap.Exec("SELECT first_name FROM people")
	c.Assert(err, IsNil)
}

type execResult struct {
	n   int
	err error
}

func (s *MySQLMigrateSuite) concurrentMigrate(useLock bool, waitTime time.Duration) []*execResult {
	migrations := &MemoryMigrationSource{
		Migrations: mysqlMigrations[:2],
	}

	numMigrate := 10
	errChannel := make(chan *execResult, numMigrate)

	for i := 1; i <= numMigrate; i++ {
		go func() {
			var n int
			var err error

			if useLock {
				n, err = ExecWithLock(s.Db, "mysql", migrations, Up, time.Duration(waitTime))
			} else {
				n, err = Exec(s.Db, "mysql", migrations, Up)
			}

			errChannel <- &execResult{
				n:   n,
				err: err,
			}
		}()
	}

	var execResults []*execResult

	for i := 1; i <= numMigrate; i++ {
		result := <-errChannel
		execResults = append(execResults, result)
	}

	return execResults
}

func (s *MySQLMigrateSuite) TestConcurrentMigrateWithoutLock(c *C) {
	results := s.concurrentMigrate(false, time.Duration(1*time.Second))

	var errorFound bool
	var badIndex int

	for i, v := range results {
		if v.err != nil {
			errorFound = true
			badIndex = i
			break
		}
	}

	// Concurrent migrates with Exec() should run into at least 1 failure
	c.Assert(errorFound, Equals, true)
	c.Assert(results[badIndex].err, NotNil)
}

func (s *MySQLMigrateSuite) TestConcurrentMigrateWithLock(c *C) {
	results := s.concurrentMigrate(true, time.Duration(5*time.Second))

	var errorFound bool

	for _, v := range results {
		if v.err != nil {
			errorFound = true
		}
	}

	// Concurrent migrates with ExecWithLock() should NOT run into any errors
	c.Assert(errorFound, Equals, false)
}

func (s *MySQLMigrateSuite) TestConcurrentMigrateWithLockShortWaitTime(c *C) {
	results := s.concurrentMigrate(true, time.Duration(500*time.Nanosecond))

	var errorFound bool
	var badIndex int

	for i, v := range results {
		if v.err != nil {
			errorFound = true
			badIndex = i
		}
	}

	// Concurrent migrates with ExecWithLock but too low of a waittime should
	// result in at least 1 failure
	c.Assert(errorFound, Equals, true)
	c.Assert(results[badIndex].err, ErrorMatches, "Exceeded lock clearance wait time.+")
}

func (s *MySQLMigrateSuite) TestMigrateIncremental(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: mysqlMigrations[:1],
	}

	// Executes one migration
	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Execute a new migration
	migrations = &MemoryMigrationSource{
		Migrations: mysqlMigrations[:2],
	}
	n, err = Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Can use column now
	_, err = s.DbMap.Exec("SELECT first_name FROM people")
	c.Assert(err, IsNil)
}

func (s *MySQLMigrateSuite) TestFileMigrate(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	// Executes two migrations
	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Has data
	id, err := s.DbMap.SelectInt("SELECT id FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(1))
}

func (s *MySQLMigrateSuite) TestAssetMigrate(c *C) {
	migrations := &AssetMigrationSource{
		Asset:    Asset,
		AssetDir: AssetDir,
		Dir:      "test-migrations",
	}

	// Executes two migrations
	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Has data
	id, err := s.DbMap.SelectInt("SELECT id FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(1))
}

func (s *MySQLMigrateSuite) TestMigrateMax(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	// Executes one migration
	n, err := ExecMax(s.Db, "mysql", migrations, Up, 1)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	id, err := s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(0))
}

func (s *MySQLMigrateSuite) TestMigrateDown(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Has data
	id, err := s.DbMap.SelectInt("SELECT id FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(1))

	// Undo the last one
	n, err = ExecMax(s.Db, "mysql", migrations, Down, 1)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// No more data
	id, err = s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(0))

	// Remove the table.
	n, err = ExecMax(s.Db, "mysql", migrations, Down, 1)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 1)

	// Cannot query it anymore
	_, err = s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, Not(IsNil))

	// Nothing left to do.
	n, err = ExecMax(s.Db, "mysql", migrations, Down, 1)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func (s *MySQLMigrateSuite) TestMigrateDownFull(c *C) {
	migrations := &FileMigrationSource{
		Dir: "test-migrations",
	}

	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Has data
	id, err := s.DbMap.SelectInt("SELECT id FROM people")
	c.Assert(err, IsNil)
	c.Assert(id, Equals, int64(1))

	// Undo the last one
	n, err = Exec(s.Db, "mysql", migrations, Down)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)

	// Cannot query it anymore
	_, err = s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, Not(IsNil))

	// Nothing left to do.
	n, err = Exec(s.Db, "mysql", migrations, Down)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func (s *MySQLMigrateSuite) TestMigrateTransaction(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			mysqlMigrations[0],
			mysqlMigrations[1],
			&Migration{
				Id:   "125",
				Up:   []string{"INSERT INTO people (id, first_name) VALUES (1, 'Test')", "SELECT fail"},
				Down: []string{}, // Not important here
			},
		},
	}

	// Should fail, transaction should roll back the INSERT.
	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, Not(IsNil))
	c.Assert(n, Equals, 2)

	// INSERT should be rolled back
	count, err := s.DbMap.SelectInt("SELECT COUNT(*) FROM people")
	c.Assert(err, IsNil)
	c.Assert(count, Equals, int64(0))
}

func (s *MySQLMigrateSuite) TestPlanMigration(c *C) {
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
	n, err := Exec(s.Db, "mysql", migrations, Up)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 3)

	migrations.Migrations = append(migrations.Migrations, &Migration{
		Id:   "11_add_middle_name.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})

	plannedMigrations, _, err := PlanMigration(s.Db, "mysql", migrations, Up, 0)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 1)
	c.Assert(plannedMigrations[0].Migration, Equals, migrations.Migrations[3])

	plannedMigrations, _, err = PlanMigration(s.Db, "mysql", migrations, Down, 0)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 3)
	c.Assert(plannedMigrations[0].Migration, Equals, migrations.Migrations[2])
	c.Assert(plannedMigrations[1].Migration, Equals, migrations.Migrations[1])
	c.Assert(plannedMigrations[2].Migration, Equals, migrations.Migrations[0])
}

func (s *MySQLMigrateSuite) TestPlanMigrationWithHoles(c *C) {
	up := "SELECT 0"
	down := "SELECT 1"
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			&Migration{
				Id:   "1",
				Up:   []string{up},
				Down: []string{down},
			},
			&Migration{
				Id:   "3",
				Up:   []string{up},
				Down: []string{down},
			},
		},
	}
	n, err := Exec(s.Db, "mysql", migrations, Up)
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
	plannedMigrations, _, err := PlanMigration(s.Db, "mysql", migrations, Up, 0)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 3)
	c.Assert(plannedMigrations[0].Migration.Id, Equals, "2")
	c.Assert(plannedMigrations[0].Queries[0], Equals, up)
	c.Assert(plannedMigrations[1].Migration.Id, Equals, "4")
	c.Assert(plannedMigrations[1].Queries[0], Equals, up)
	c.Assert(plannedMigrations[2].Migration.Id, Equals, "5")
	c.Assert(plannedMigrations[2].Queries[0], Equals, up)

	// first catch up to current target state 123, then migrate down 1 step to 12
	plannedMigrations, _, err = PlanMigration(s.Db, "mysql", migrations, Down, 1)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 2)
	c.Assert(plannedMigrations[0].Migration.Id, Equals, "2")
	c.Assert(plannedMigrations[0].Queries[0], Equals, up)
	c.Assert(plannedMigrations[1].Migration.Id, Equals, "3")
	c.Assert(plannedMigrations[1].Queries[0], Equals, down)

	// first catch up to current target state 123, then migrate down 2 steps to 1
	plannedMigrations, _, err = PlanMigration(s.Db, "mysql", migrations, Down, 2)
	c.Assert(err, IsNil)
	c.Assert(plannedMigrations, HasLen, 3)
	c.Assert(plannedMigrations[0].Migration.Id, Equals, "2")
	c.Assert(plannedMigrations[0].Queries[0], Equals, up)
	c.Assert(plannedMigrations[1].Migration.Id, Equals, "3")
	c.Assert(plannedMigrations[1].Queries[0], Equals, down)
	c.Assert(plannedMigrations[2].Migration.Id, Equals, "2")
	c.Assert(plannedMigrations[2].Queries[0], Equals, down)
}

func (s *MySQLMigrateSuite) TestLess(c *C) {
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
