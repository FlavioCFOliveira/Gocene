package sortedset

import "testing"

func TestDefaultSortedSetDocValuesReaderState(t *testing.T) {
	s := NewDefaultSortedSetDocValuesReaderState("field",
		5,
		map[string][2]int{
			"color": {0, 3},
			"size":  {3, 5},
		})
	if s.GetField() != "field" || s.GetSize() != 5 {
		t.Error("basic")
	}
	start, end := s.GetOrdRange("color")
	if start != 0 || end != 3 {
		t.Errorf("color = %d,%d", start, end)
	}
	start, end = s.GetOrdRange("missing")
	if start != -1 || end != -1 {
		t.Error("missing dim")
	}
	dims := s.GetDims()
	if len(dims) != 2 || dims[0] != "color" || dims[1] != "size" {
		t.Errorf("dims = %v", dims)
	}
}
