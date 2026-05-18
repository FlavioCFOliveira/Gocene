package taxonomy

import "testing"

func TestTaxonomyFacetFloatAssociationsSum(t *testing.T) {
	a := NewTaxonomyFacetFloatAssociations("score", SUM)
	a.Aggregate(7, 1.5)
	a.Aggregate(7, 2.5)
	if a.ValueForOrd(7) != 4.0 {
		t.Errorf("sum = %v", a.ValueForOrd(7))
	}
}

func TestTaxonomyFacetFloatAssociationsMax(t *testing.T) {
	a := NewTaxonomyFacetFloatAssociations("score", MAX)
	a.Aggregate(7, 1.0)
	a.Aggregate(7, 9.0)
	a.Aggregate(7, 4.0)
	if a.ValueForOrd(7) != 9.0 {
		t.Errorf("max = %v", a.ValueForOrd(7))
	}
}
