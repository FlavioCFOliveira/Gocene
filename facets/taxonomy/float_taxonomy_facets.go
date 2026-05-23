// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import "github.com/FlavioCFOliveira/Gocene/facets"

// FloatTaxonomyFacets is the base struct for taxonomy-based facets that
// aggregate to float32 per ordinal. Concrete types (TaxonomyFacetFloatAssociations)
// embed this. Mirrors org.apache.lucene.facet.taxonomy.FloatTaxonomyFacets.
type FloatTaxonomyFacets struct {
	*TaxonomyFacets

	// AggregationFunction combines per-document float values.
	AggregationFunction AssociationAggregationFunction

	// values is the dense per-ordinal float32 aggregate store.
	values []float32
}

// NewFloatTaxonomyFacets allocates FloatTaxonomyFacets wrapping base.
// Pass aggregationFunction = SUM for sum-based impls.
func NewFloatTaxonomyFacets(
	indexFieldName string,
	taxoReader TaxonomyReaderI,
	config *facets.FacetsConfig,
	aggregationFunction AssociationAggregationFunction,
) *FloatTaxonomyFacets {
	if aggregationFunction == nil {
		aggregationFunction = SUM
	}
	base := NewTaxonomyFacets(indexFieldName, taxoReader, config)
	ftf := &FloatTaxonomyFacets{
		TaxonomyFacets:      base,
		AggregationFunction: aggregationFunction,
	}

	// Override aggregation hooks on the base to use float32 values.
	base.GetAggregationValueFn = func(ord int) float64 { return float64(ftf.GetValue(ord)) }
	base.AggregateFn = func(a, b float64) float64 {
		return float64(aggregationFunction.Aggregate(float32(a), float32(b)))
	}
	return ftf
}

// initValues lazily allocates the values slice.
func (ftf *FloatTaxonomyFacets) initValues() {
	if ftf.values == nil {
		ftf.values = make([]float32, ftf.TaxoReader.GetSize())
	}
}

// SetValue sets the accumulated float value for ordinal.
func (ftf *FloatTaxonomyFacets) SetValue(ord int, v float32) {
	ftf.initValues()
	ftf.values[ord] = v
}

// GetValue returns the accumulated float value for ordinal.
func (ftf *FloatTaxonomyFacets) GetValue(ord int) float32 {
	if ftf.values == nil || ord < 0 || ord >= len(ftf.values) {
		return 0
	}
	return ftf.values[ord]
}

// AccumulateFloatValue aggregates incoming into the running value for ordinal.
func (ftf *FloatTaxonomyFacets) AccumulateFloatValue(ord int, incoming float32) {
	ftf.initValues()
	ftf.values[ord] = ftf.AggregationFunction.Aggregate(ftf.values[ord], incoming)
}
