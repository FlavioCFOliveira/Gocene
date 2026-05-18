package facetset

import "testing"

func TestExactFacetSetMatcher(t *testing.T) {
	m := NewExactFacetSetMatcher("cell", 1, 2, 3)
	if m.Label() != "cell" || m.Dims() != 3 {
		t.Error("metadata")
	}
	if !m.Matches([]int64{1, 2, 3}) {
		t.Error("equal should match")
	}
	if m.Matches([]int64{1, 2, 4}) {
		t.Error("different should not match")
	}
	if m.Matches([]int64{1, 2}) {
		t.Error("dimensionality mismatch")
	}
}
