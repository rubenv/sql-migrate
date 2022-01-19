package migrate

import (
	"sort"

	. "gopkg.in/check.v1"
)

type SortSuite struct{}

var _ = Suite(&SortSuite{})

func (s *SortSuite) TestSortMigrations(c *C) {
	var migrations = byId([]*Migration{
		{Id: "10_abc"},
		{Id: "120_cde"},
		{Id: "1_abc"},
		{Id: "efg"},
		{Id: "2_cde"},
		{Id: "35_cde"},
		{Id: "3_efg"},
		{Id: "4_abc"},
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
