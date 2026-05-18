package facetset

import "testing"

func TestFacetSetsField(t *testing.T) {
	field := NewFacetSetsField("dimset",
		NewIntFacetSet(1, 2, 3),
		NewIntFacetSet(4, 5, 6))
	if field.Name != "dimset" {
		t.Error("name")
	}
	bv := field.BinaryValue()
	if len(bv) == 0 {
		t.Fatal("empty value")
	}
	// header: 2 sets, 3 dims, then 24 bytes of int32 (2 sets * 3 dims * 4 bytes)
	expectedBodyLen := 2 * 3 * 4
	if len(bv) <= expectedBodyLen {
		t.Errorf("body too short: %d", len(bv))
	}
}

func TestFacetSetsFieldEnforcesUniformDims(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic on mismatched dims")
		}
	}()
	NewFacetSetsField("x", NewIntFacetSet(1, 2), NewIntFacetSet(1, 2, 3))
}
