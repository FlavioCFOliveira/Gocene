package rangefacets

import "testing"

func TestLongRange(t *testing.T) {
	r := NewLongRange("ok", 0, true, 10, false)
	if !r.Accept(0) {
		t.Error("min")
	}
	if r.Accept(10) {
		t.Error("max exclusive")
	}
	if r.String() != "ok[0,10)" {
		t.Errorf("string=%q", r.String())
	}
}
