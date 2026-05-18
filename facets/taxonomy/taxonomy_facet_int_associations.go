package taxonomy

// TaxonomyFacetIntAssociations is the int32 counterpart of
// TaxonomyFacetFloatAssociations. Mirrors
// org.apache.lucene.facet.taxonomy.TaxonomyFacetIntAssociations.
type TaxonomyFacetIntAssociations struct {
	dim        string
	aggregator AssociationAggregationFunction
	values     map[int]int32
}

// NewTaxonomyFacetIntAssociations builds an aggregator.
func NewTaxonomyFacetIntAssociations(dim string, aggregator AssociationAggregationFunction) *TaxonomyFacetIntAssociations {
	if aggregator == nil {
		aggregator = SUM
	}
	return &TaxonomyFacetIntAssociations{
		dim:        dim,
		aggregator: aggregator,
		values:     make(map[int]int32),
	}
}

// Aggregate folds incoming into the running value for ordinal ord.
func (t *TaxonomyFacetIntAssociations) Aggregate(ord int, incoming int32) {
	cur, ok := t.values[ord]
	if !ok {
		t.values[ord] = incoming
		return
	}
	t.values[ord] = t.aggregator.AggregateInt(cur, incoming)
}

// ValueForOrd returns the accumulated value for ordinal ord.
func (t *TaxonomyFacetIntAssociations) ValueForOrd(ord int) int32 {
	return t.values[ord]
}

// GetDim returns the dimension label.
func (t *TaxonomyFacetIntAssociations) GetDim() string { return t.dim }
