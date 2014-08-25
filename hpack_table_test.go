package http2

import (
	gc "gopkg.in/check.v1"
)

type HpackTableTest struct {
}

func (t *HpackTableTest) TestNewHpackTable(c *gc.C) {
	table := NewHpackTable()
	c.Check(table.Index.Len(), gc.Equals, len(kHpackStaticTable))
	c.Check(table.TotalInsertions, gc.Equals, len(kHpackStaticTable) + 1)
}

var _ = gc.Suite(&HpackTableTest{})
