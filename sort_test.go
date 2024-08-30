package migrate

import (
	"sort"

	//revive:disable-next-line:dot-imports
	. "gopkg.in/check.v1"
)

type SortSuite struct{}

var _ = Suite(&SortSuite{})

func (*SortSuite) TestSortMigrations(c *C) {
	migrations := byId([]*Migration{
		{Id: "10_abc", Up: nil, Down: nil},
		{Id: "120_cde", Up: nil, Down: nil},
		{Id: "1_abc", Up: nil, Down: nil},
		{Id: "efg", Up: nil, Down: nil},
		{Id: "2_cde", Up: nil, Down: nil},
		{Id: "35_cde", Up: nil, Down: nil},
		{Id: "3_efg", Up: nil, Down: nil},
		{Id: "4_abc", Up: nil, Down: nil},
	})

	sort.Sort(migrations)
	c.Assert(migrations, HasLen, 8)
	c.Assert(migrations[0].Id, Equals, "1_abc")
	c.Assert(migrations[1].Id, Equals, "2_cde")
	c.Assert(migrations[2].Id, Equals, "3_efg")
	c.Assert(migrations[3].Id, Equals, "4_abc")
	c.Assert(migrations[4].Id, Equals, "10_abc")
	c.Assert(migrations[5].Id, Equals, "35_cde")
	c.Assert(migrations[6].Id, Equals, "120_cde")
	c.Assert(migrations[7].Id, Equals, "efg")
}
