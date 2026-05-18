package rangefacets

import "testing"

func TestDoubleRangeInclusive(t *testing.T) {
	r := NewDoubleRange("ok", 0, true, 10, true)
	if !r.Accept(0) || !r.Accept(10) || !r.Accept(5) {
		t.Error("inclusive accept")
	}
	if r.Accept(-0.1) || r.Accept(10.1) {
		t.Error("outside")
	}
}

func TestDoubleRangeExclusive(t *testing.T) {
	r := NewDoubleRange("ok", 0, false, 10, false)
	if r.Accept(0) || r.Accept(10) {
		t.Error("exclusive bounds")
	}
	if !r.Accept(5) {
		t.Error("inside")
	}
}

func TestDoubleRangeString(t *testing.T) {
	r := NewDoubleRange("ok", 0, true, 10, false)
	if r.String() != "ok[0,10)" {
		t.Errorf("String=%q", r.String())
	}
}
