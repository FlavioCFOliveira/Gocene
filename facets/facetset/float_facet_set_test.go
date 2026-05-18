package facetset

import "testing"

func TestFloatFacetSet(t *testing.T) {
	s := NewFloatFacetSet(0.5, -0.5, 1e6)
	if s.Dims() != 3 || s.SizeInBytes() != 12 {
		t.Errorf("dims/size: %d %d", s.Dims(), s.SizeInBytes())
	}
	cv := s.GetComparableValues()
	// Lucene's sortable encoding must preserve numeric ordering under signed
	// comparison: encoded(-0.5) < encoded(0.5) < encoded(1e6).
	if !(cv[1] < cv[0] && cv[0] < cv[2]) {
		t.Errorf("sortable encoding broken: got %v", cv)
	}
}
