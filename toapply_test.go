package migrate

import (
	"sort"
	. "gopkg.in/check.v1"
)

var toapplyMigrations = []*Migration{
	&Migration{Id: "abc", Up: nil, Down: nil},
	&Migration{Id: "cde", Up: nil, Down: nil},
	&Migration{Id: "efg", Up: nil, Down: nil},
}

type ToApplyMigrateSuite struct {
}

var _ = Suite(&ToApplyMigrateSuite{})

func (s *ToApplyMigrateSuite) TestGetAll(c *C) {
	toApply := ToApply(toapplyMigrations, toapplyMigrations[0:0], Up)
	c.Assert(toApply, HasLen, 3)
	c.Assert(toApply[0], Equals, toapplyMigrations[0])
	c.Assert(toApply[1], Equals, toapplyMigrations[1])
	c.Assert(toApply[2], Equals, toapplyMigrations[2])
}

func (s *ToApplyMigrateSuite) TestGetAbc(c *C) {
	toApply := ToApply(toapplyMigrations, toapplyMigrations[0:1], Up)
	c.Assert(toApply, HasLen, 2)
	c.Assert(toApply[0], Equals, toapplyMigrations[1])
	c.Assert(toApply[1], Equals, toapplyMigrations[2])
}

func (s *ToApplyMigrateSuite) TestGetCde(c *C) {
	toApply := ToApply(toapplyMigrations, toapplyMigrations[0:2], Up)
	c.Assert(toApply, HasLen, 1)
	c.Assert(toApply[0], Equals, toapplyMigrations[2])
}

func (s *ToApplyMigrateSuite) TestGetDone(c *C) {
	toApply := ToApply(toapplyMigrations, toapplyMigrations[0:3], Up)
	c.Assert(toApply, HasLen, 0)
}

func (s *ToApplyMigrateSuite) TestDownDone(c *C) {
	toApply := ToApply(toapplyMigrations, toapplyMigrations[0:0], Down)
	c.Assert(toApply, HasLen, 0)
}

func (s *ToApplyMigrateSuite) TestDownCde(c *C) {
	toApply := ToApply(toapplyMigrations, toapplyMigrations[0:2], Down)
	c.Assert(toApply, HasLen, 2)
	c.Assert(toApply[0], Equals, toapplyMigrations[1])
	c.Assert(toApply[1], Equals, toapplyMigrations[0])
}

func (s *ToApplyMigrateSuite) TestDownAbc(c *C) {
	toApply := ToApply(toapplyMigrations, toapplyMigrations[0:1], Down)
	c.Assert(toApply, HasLen, 1)
	c.Assert(toApply[0], Equals, toapplyMigrations[0])
}

func (s *ToApplyMigrateSuite) TestDownAll(c *C) {
	toApply := ToApply(toapplyMigrations, toapplyMigrations, Down)
	c.Assert(toApply, HasLen, 3)
	c.Assert(toApply[0], Equals, toapplyMigrations[2])
	c.Assert(toApply[1], Equals, toapplyMigrations[1])
	c.Assert(toApply[2], Equals, toapplyMigrations[0])
}

func (s *ToApplyMigrateSuite) TestAlphaNumericMigrations(c *C) {
	var migrations = byId([]*Migration{
		&Migration{Id: "10_abc", Up: nil, Down: nil},
		&Migration{Id: "1_abc", Up: nil, Down: nil},
		&Migration{Id: "efg", Up: nil, Down: nil},
		&Migration{Id: "2_cde", Up: nil, Down: nil},
		&Migration{Id: "35_cde", Up: nil, Down: nil},
	})

	sort.Sort(migrations)

	toApplyUp := ToApply(migrations, migrations[0:2], Up)
	c.Assert(toApplyUp, HasLen, 3)
	c.Assert(toApplyUp[0].Id, Equals, "10_abc")
	c.Assert(toApplyUp[1].Id, Equals, "35_cde")
	c.Assert(toApplyUp[2].Id, Equals, "efg")

	toApplyDown := ToApply(migrations, migrations[0:2], Down)
	c.Assert(toApplyDown, HasLen, 2)
	c.Assert(toApplyDown[0].Id, Equals, "2_cde")
	c.Assert(toApplyDown[1].Id, Equals, "1_abc")
}
