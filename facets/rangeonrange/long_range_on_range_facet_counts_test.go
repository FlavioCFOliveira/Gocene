package rangeonrange

import "testing"

func TestLongRangeOnRangeFacetCounts(t *testing.T) {
	c := NewLongRangeOnRangeFacetCounts("range", IntersectsRelation,
		NewLongRange("low", 0, 5),
		NewLongRange("mid", 4, 8))
	c.Accept(3, 6)
	got := c.GetCounts()
	if got[0] != 1 || got[1] != 1 {
		t.Errorf("counts = %v", got)
	}
}
