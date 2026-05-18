package facets

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestConstantMultiDoubleValuesSource(t *testing.T) {
	src := NewConstantMultiDoubleValuesSource(1.5, 2.5, 3.5)
	if src.NeedsScores() {
		t.Error("NeedsScores")
	}
	if !src.IsCacheable(index.LeafReaderContext{}) {
		t.Error("IsCacheable")
	}
	mv, err := src.GetValues(index.LeafReaderContext{})
	if err != nil {
		t.Fatal(err)
	}
	ok, _ := mv.AdvanceExact(0)
	if !ok {
		t.Fatal("expected values")
	}
	if mv.DocValueCount() != 3 {
		t.Errorf("count = %d", mv.DocValueCount())
	}
	v, _ := mv.NextValue()
	if v != 1.5 {
		t.Errorf("first = %v", v)
	}
}
