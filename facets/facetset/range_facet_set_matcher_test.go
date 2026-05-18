package facetset

import "testing"

func TestRangeFacetSetMatcher(t *testing.T) {
	m := NewRangeFacetSetMatcher("box", NewDimRange(0, 10), NewDimRange(0, 10))
	if m.Label() != "box" || m.Dims() != 2 {
		t.Error("metadata")
	}
	if !m.Matches([]int64{5, 5}) {
		t.Error("inside")
	}
	if m.Matches([]int64{11, 5}) {
		t.Error("outside first dim")
	}
	if m.Matches([]int64{5, 11}) {
		t.Error("outside second dim")
	}
	if m.Matches([]int64{5}) {
		t.Error("dim mismatch")
	}
}
