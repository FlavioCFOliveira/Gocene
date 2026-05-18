package rangeonrange

import "testing"

func TestDoubleRangeOnRangeFacetCountsIntersect(t *testing.T) {
	c := NewDoubleRangeOnRangeFacetCounts("score", IntersectsRelation,
		NewDoubleRange("low", 0, 5),
		NewDoubleRange("mid", 4, 8))
	c.Accept(3, 6)
	got := c.GetCounts()
	if got[0] != 1 || got[1] != 1 {
		t.Errorf("counts = %v", got)
	}
}

func TestDoubleRangeOnRangeFacetCountsWithin(t *testing.T) {
	c := NewDoubleRangeOnRangeFacetCounts("score", WithinRelation,
		NewDoubleRange("box", 0, 10))
	c.Accept(2, 8)
	c.Accept(-1, 11)
	got := c.GetCounts()
	if got[0] != 1 {
		t.Errorf("counts = %v (only first interval is within)", got)
	}
}
