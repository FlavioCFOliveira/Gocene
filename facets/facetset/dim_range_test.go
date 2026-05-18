package facetset

import "testing"

func TestDimRange(t *testing.T) {
	r := NewDimRange(10, 20)
	if !r.Contains(10) || !r.Contains(15) || !r.Contains(20) {
		t.Error("inclusive bounds")
	}
	if r.Contains(9) || r.Contains(21) {
		t.Error("outside bounds")
	}
	swap := NewDimRange(30, 10)
	if swap.Min != 10 || swap.Max != 30 {
		t.Error("min/max swap")
	}
}
