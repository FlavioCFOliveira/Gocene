package join

import "testing"

func TestSeekingTermSetTermsEnum(t *testing.T) {
	e := NewSeekingTermSetTermsEnum([]string{"banana", "apple", "cherry"})
	if e.Size() != 3 {
		t.Error("size")
	}
	if !e.SeekCeil("apricot") {
		t.Error("seekCeil")
	}
	if e.Term() != "banana" {
		t.Errorf("got %q", e.Term())
	}
	if !e.SeekExact("cherry") || e.Term() != "cherry" {
		t.Error("seekExact")
	}
	if _, ok := e.Next(); ok {
		t.Error("should be exhausted")
	}
	if e.SeekExact("missing") {
		t.Error("missing should be false")
	}
}
