package rangefacets

import "testing"

func TestLongRangeFacetCounts(t *testing.T) {
	c := NewLongRangeFacetCounts("year",
		NewLongRange("old", 1990, true, 2000, false),
		NewLongRange("new", 2000, true, 2026, true))
	for _, v := range []int64{1990, 1995, 2000, 2020, 2026} {
		c.Accept(v)
	}
	counts := c.GetCounts()
	if counts[0] != 2 || counts[1] != 3 {
		t.Errorf("counts = %v", counts)
	}
}
