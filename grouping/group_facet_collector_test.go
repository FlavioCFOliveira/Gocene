package grouping

import "testing"

func TestMapGroupFacetCollector(t *testing.T) {
	c := NewMapGroupFacetCollector()
	c.Collect("g1", "red")
	c.Collect("g2", "red") // distinct group for "red"
	c.Collect("g1", "red") // duplicate (g1, red) — dropped
	c.Collect("g1", "blue")
	if c.TotalCount() != 3 {
		t.Errorf("total = %d", c.TotalCount())
	}
	got := map[string]int{}
	for _, e := range c.GetCounts() {
		got[e.Facet] = e.Count
	}
	if got["red"] != 2 || got["blue"] != 1 {
		t.Errorf("counts = %v", got)
	}
}
