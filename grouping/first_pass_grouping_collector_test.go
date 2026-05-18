package grouping

import "testing"

// compareAsc orders by the first sortValue interpreted as int.
func compareAsc(a, b []any) int {
	ai := a[0].(int)
	bi := b[0].(int)
	switch {
	case ai < bi:
		return -1
	case ai > bi:
		return 1
	default:
		return 0
	}
}

func TestFirstPassGroupingCollectorKeepsTopN(t *testing.T) {
	c := NewFirstPassGroupingCollector[string](2, compareAsc)
	c.Collect("red", []any{5}, 0)
	c.Collect("green", []any{1}, 1)
	c.Collect("blue", []any{3}, 2)
	groups := c.GetTopGroups()
	if len(groups) != 2 {
		t.Fatalf("groups = %d", len(groups))
	}
	have := map[string]bool{}
	for _, g := range groups {
		have[g.GroupValue] = true
	}
	if !have["green"] || !have["blue"] {
		t.Errorf("expected green+blue, got %v", have)
	}
}

func TestFirstPassGroupingCollectorUpdatesExisting(t *testing.T) {
	c := NewFirstPassGroupingCollector[string](2, compareAsc)
	c.Collect("red", []any{10}, 0)
	c.Collect("red", []any{1}, 1)
	groups := c.GetTopGroups()
	if len(groups) != 1 {
		t.Fatalf("groups = %d", len(groups))
	}
	if groups[0].SortValues[0].(int) != 1 {
		t.Errorf("sortValue = %v", groups[0].SortValues[0])
	}
}
