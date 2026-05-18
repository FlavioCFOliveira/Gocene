package facets

import "testing"

func TestStringDocValuesReaderState(t *testing.T) {
	s := NewStringDocValuesReaderState("brand", []string{"acme", "globex"})
	if s.UniqueOrds != 2 {
		t.Errorf("UniqueOrds = %d", s.UniqueOrds)
	}
	if s.TermForOrd(0) != "acme" {
		t.Errorf("ord 0")
	}
	if s.TermForOrd(2) != "" {
		t.Errorf("out of range")
	}
}
