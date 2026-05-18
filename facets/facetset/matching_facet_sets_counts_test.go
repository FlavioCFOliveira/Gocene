package facetset

import "testing"

func TestMatchingFacetSetsCounts(t *testing.T) {
	field := NewFacetSetsField("dimset",
		NewIntFacetSet(5, 5),
		NewIntFacetSet(15, 15))
	bv := field.BinaryValue()

	matchers := []FacetSetMatcher{
		NewRangeFacetSetMatcher("low", NewDimRange(0, 10), NewDimRange(0, 10)),
		NewRangeFacetSetMatcher("high", NewDimRange(11, 20), NewDimRange(11, 20)),
		NewExactFacetSetMatcher("origin", 0, 0),
	}
	counts := NewMatchingFacetSetsCounts("dim", matchers, IntDecoder)
	if err := counts.Accumulate(bv); err != nil {
		t.Fatal(err)
	}
	got := counts.GetCounts()
	if got[0] != 1 || got[1] != 1 || got[2] != 0 {
		t.Errorf("counts = %v", got)
	}
	if counts.GetTotalHits() != 1 {
		t.Errorf("hits = %d", counts.GetTotalHits())
	}
}
