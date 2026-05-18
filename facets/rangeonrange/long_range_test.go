package rangeonrange

import "testing"

func TestLongRangePredicates(t *testing.T) {
	r := NewLongRange("ok", 0, 10)
	if !r.Contains(2, 8) || r.Contains(-1, 8) {
		t.Error("Contains")
	}
	if !r.Within(-1, 11) {
		t.Error("Within")
	}
	if !r.Overlaps(5, 15) || r.Overlaps(11, 20) {
		t.Error("Overlaps")
	}
}
