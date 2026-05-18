package rangefacets

import "testing"

func TestDoubleRangeFacetCounts(t *testing.T) {
	c := NewDoubleRangeFacetCounts("score",
		NewDoubleRange("low", 0, true, 5, false),
		NewDoubleRange("high", 5, true, 10, true))
	for _, v := range []float64{0, 1, 2, 5, 7, 10} {
		c.Accept(v)
	}
	if c.GetTotalCount() != 6 {
		t.Errorf("total = %d", c.GetTotalCount())
	}
	counts := c.GetCounts()
	if counts[0] != 3 {
		t.Errorf("low = %d", counts[0])
	}
	if counts[1] != 3 {
		t.Errorf("high = %d", counts[1])
	}
}
