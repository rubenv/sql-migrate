package migrate

import (
	"database/sql"
	"testing"

	"github.com/coopernurse/gorp"
	_ "github.com/mattn/go-sqlite3"
	. "gopkg.in/check.v1"
)

type SqliteMigrateSuite struct {
	DbMap *gorp.DbMap
}

var _ = Suite(&SqliteMigrateSuite{})

func Test(t *testing.T) { TestingT(t) }

func (s *SqliteMigrateSuite) SetUpTest(c *C) {
	db, err := sql.Open("sqlite3", "/tmp/gorp-migrate-sqlite.db")
	c.Assert(err, IsNil)

	s.DbMap = &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}
}

func (s *SqliteMigrateSuite) TearDownTest(c *C) {
	//err := os.Remove("/tmp/gorp-migrate-sqlite.db")
	//c.Assert(err, IsNil)
}

func (s *SqliteMigrateSuite) TestRunMigration(c *C) {
	migrations := &MemoryMigrationSource{
		Migrations: []*Migration{
			&Migration{
				Id:   "123",
				Up:   "CREATE TABLE people (id int)",
				Down: "DROP TABLE people",
			},
		},
	}

	err := Exec(s.DbMap, migrations)
	c.Assert(err, IsNil)

	_, err = s.DbMap.Exec("SELECT * FROM people")
	c.Assert(err, IsNil)
}
