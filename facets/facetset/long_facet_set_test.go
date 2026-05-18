package facetset

import "testing"

func TestLongFacetSet(t *testing.T) {
	s := NewLongFacetSet(1, 2, -3)
	if s.Dims() != 3 || s.SizeInBytes() != 24 {
		t.Errorf("dims/size: %d %d", s.Dims(), s.SizeInBytes())
	}
	buf := make([]byte, s.SizeInBytes())
	s.PackValues(buf)
	if buf[7] != 1 {
		t.Errorf("first encoded byte run: %v", buf[:8])
	}
	cv := s.GetComparableValues()
	if cv[2] != -3 {
		t.Errorf("comparable values: %v", cv)
	}
}
