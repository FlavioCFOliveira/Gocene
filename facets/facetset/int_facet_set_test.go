package facetset

import "testing"

func TestIntFacetSet(t *testing.T) {
	s := NewIntFacetSet(1, 2, -3)
	if s.Dims() != 3 {
		t.Errorf("dims = %d", s.Dims())
	}
	if s.SizeInBytes() != 12 {
		t.Errorf("size = %d", s.SizeInBytes())
	}
	buf := make([]byte, s.SizeInBytes())
	s.PackValues(buf)
	if buf[0] != 0 || buf[3] != 1 {
		t.Errorf("encoding of 1: %v", buf[:4])
	}
	cv := s.GetComparableValues()
	if len(cv) != 3 || cv[0] != 1 || cv[2] != -3 {
		t.Errorf("comparable values: %v", cv)
	}
	if vs := s.Values(); len(vs) != 3 || vs[0] != 1 {
		t.Errorf("values: %v", vs)
	}
}
