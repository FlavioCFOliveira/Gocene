package facets

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestConstantMultiLongValuesSource(t *testing.T) {
	src := NewConstantMultiLongValuesSource(1, 2, 3)
	mv, _ := src.GetValues(index.LeafReaderContext{})
	ok, _ := mv.AdvanceExact(0)
	if !ok {
		t.Fatal("expected values")
	}
	if mv.DocValueCount() != 3 {
		t.Error("count")
	}
	v, _ := mv.NextValue()
	if v != 1 {
		t.Errorf("first = %d", v)
	}
}
