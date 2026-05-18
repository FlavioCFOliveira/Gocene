package rangeonrange

import "testing"

func TestDoubleRangeContainsWithinOverlaps(t *testing.T) {
	r := NewDoubleRange("ok", 0, 10)
	if !r.Contains(2, 8) {
		t.Error("contains inside")
	}
	if r.Contains(-1, 8) {
		t.Error("contains crosses lo")
	}
	if !r.Within(-1, 11) {
		t.Error("within superset")
	}
	if !r.Overlaps(5, 15) {
		t.Error("overlaps cross")
	}
	if r.Overlaps(11, 20) {
		t.Error("disjoint")
	}
}
