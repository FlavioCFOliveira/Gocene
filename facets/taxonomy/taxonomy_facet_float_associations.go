package taxonomy

// TaxonomyFacetFloatAssociations aggregates float32 association payloads by
// ordinal using a configurable AssociationAggregationFunction. Mirrors
// org.apache.lucene.facet.taxonomy.TaxonomyFacetFloatAssociations.
type TaxonomyFacetFloatAssociations struct {
	dim        string
	aggregator AssociationAggregationFunction
	values     map[int]float32
}

// NewTaxonomyFacetFloatAssociations builds an aggregator on the supplied
// dimension label and aggregation function.
func NewTaxonomyFacetFloatAssociations(dim string, aggregator AssociationAggregationFunction) *TaxonomyFacetFloatAssociations {
	if aggregator == nil {
		aggregator = SUM
	}
	return &TaxonomyFacetFloatAssociations{
		dim:        dim,
		aggregator: aggregator,
		values:     make(map[int]float32),
	}
}

// Aggregate folds incoming into the running value for ordinal ord.
func (t *TaxonomyFacetFloatAssociations) Aggregate(ord int, incoming float32) {
	cur, ok := t.values[ord]
	if !ok {
		t.values[ord] = incoming
		return
	}
	t.values[ord] = t.aggregator.Aggregate(cur, incoming)
}

// ValueForOrd returns the accumulated value for ordinal ord.
func (t *TaxonomyFacetFloatAssociations) ValueForOrd(ord int) float32 {
	return t.values[ord]
}

// GetDim returns the dimension label.
func (t *TaxonomyFacetFloatAssociations) GetDim() string { return t.dim }
