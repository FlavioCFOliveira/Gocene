package facets

import "testing"

func TestEmptyMultiLongValues(t *testing.T) {
	m := NewEmptyMultiLongValues()
	ok, _ := m.AdvanceExact(0)
	if ok {
		t.Error("AdvanceExact")
	}
	if m.DocValueCount() != 0 {
		t.Error("count")
	}
}
