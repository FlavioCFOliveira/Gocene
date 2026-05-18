package facetset

import "testing"

func TestDoubleFacetSet(t *testing.T) {
	s := NewDoubleFacetSet(1.5, -2.5, 1e18)
	if s.Dims() != 3 || s.SizeInBytes() != 24 {
		t.Errorf("dims/size: %d %d", s.Dims(), s.SizeInBytes())
	}
	cv := s.GetComparableValues()
	if !(cv[1] < cv[0] && cv[0] < cv[2]) {
		t.Errorf("sortable encoding broken: %v", cv)
	}
}
