// +build go1.16

package migrate

import (
	"embed"

	. "gopkg.in/check.v1"
)

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
