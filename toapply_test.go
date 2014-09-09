package migrate

import (
	_ "github.com/mattn/go-sqlite3"
	. "gopkg.in/check.v1"
)

var toapplyMigrations = []*Migration{
	&Migration{Id: "abc", Up: "", Down: ""},
	&Migration{Id: "cde", Up: "", Down: ""},
	&Migration{Id: "efg", Up: "", Down: ""},
}

type ToApplyMigrateSuite struct {
}

var _ = Suite(&ToApplyMigrateSuite{})

func (s *ToApplyMigrateSuite) TestGetAll(c *C) {
	toApply := ToApply(toapplyMigrations, "", Up)
	c.Assert(toApply, HasLen, 3)
	c.Assert(toApply[0], Equals, toapplyMigrations[0])
	c.Assert(toApply[1], Equals, toapplyMigrations[1])
	c.Assert(toApply[2], Equals, toapplyMigrations[2])
}

func (s *ToApplyMigrateSuite) TestGetAbc(c *C) {
	toApply := ToApply(toapplyMigrations, "abc", Up)
	c.Assert(toApply, HasLen, 2)
	c.Assert(toApply[0], Equals, toapplyMigrations[1])
	c.Assert(toApply[1], Equals, toapplyMigrations[2])
}

func (s *ToApplyMigrateSuite) TestGetCde(c *C) {
	toApply := ToApply(toapplyMigrations, "cde", Up)
	c.Assert(toApply, HasLen, 1)
	c.Assert(toApply[0], Equals, toapplyMigrations[2])
}

func (s *ToApplyMigrateSuite) TestGetDone(c *C) {
	toApply := ToApply(toapplyMigrations, "efg", Up)
	c.Assert(toApply, HasLen, 0)

	toApply = ToApply(toapplyMigrations, "zzz", Up)
	c.Assert(toApply, HasLen, 0)
}
