package taxonomy

import "testing"

func TestFloatAssociationFacetField(t *testing.T) {
	f := NewFloatAssociationFacetField(3.14, "score", "weight")
	if f.GetDim() != "score" {
		t.Error("dim")
	}
	if len(f.Association) != 4 {
		t.Error("payload size")
	}
	if got := FloatAssociationFromBytes(f.Association); got != 3.14 {
		t.Errorf("decoded = %v", got)
	}
}
