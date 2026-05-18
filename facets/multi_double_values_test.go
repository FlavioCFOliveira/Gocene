package facets

import "testing"

func TestEmptyMultiDoubleValues(t *testing.T) {
	m := NewEmptyMultiDoubleValues()
	ok, err := m.AdvanceExact(0)
	if err != nil || ok {
		t.Errorf("AdvanceExact = %v %v", ok, err)
	}
	if m.DocValueCount() != 0 {
		t.Error("count")
	}
	v, err := m.NextValue()
	if err != nil || v != 0 {
		t.Errorf("NextValue = %v %v", v, err)
	}
}
