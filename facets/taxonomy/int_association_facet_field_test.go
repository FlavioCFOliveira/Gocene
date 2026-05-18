package taxonomy

import "testing"

func TestIntAssociationFacetField(t *testing.T) {
	f := NewIntAssociationFacetField(42, "rank", "primary")
	if f.GetDim() != "rank" {
		t.Error("dim")
	}
	if got := IntAssociationFromBytes(f.Association); got != 42 {
		t.Errorf("decoded = %d", got)
	}
}
