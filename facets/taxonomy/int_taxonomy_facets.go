// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import "github.com/FlavioCFOliveira/Gocene/facets"

// IntTaxonomyFacets is the base struct for taxonomy-based facets that
// aggregate to int32 per ordinal. Concrete types (TaxonomyFacetCounts,
// TaxonomyFacetIntAssociations) embed this. Mirrors
// org.apache.lucene.facet.taxonomy.IntTaxonomyFacets.
type IntTaxonomyFacets struct {
	*TaxonomyFacets

	// AggregationFunction combines per-document int values.
	AggregationFunction AssociationAggregationFunction

	// values is the dense per-ordinal aggregate store (int32 for
	// association values; TaxonomyFacets.counts covers raw hit counts).
	values []int32
}

// NewIntTaxonomyFacets allocates IntTaxonomyFacets wrapping base.
// Pass aggregationFunction = SUM for count-based impls.
func NewIntTaxonomyFacets(
	indexFieldName string,
	taxoReader TaxonomyReaderI,
	config *facets.FacetsConfig,
	aggregationFunction AssociationAggregationFunction,
) *IntTaxonomyFacets {
	if aggregationFunction == nil {
		aggregationFunction = SUM
	}
	base := NewTaxonomyFacets(indexFieldName, taxoReader, config)
	itf := &IntTaxonomyFacets{
		TaxonomyFacets:      base,
		AggregationFunction: aggregationFunction,
	}

	// Override the aggregation hooks on the base to use int32 values.
	base.GetAggregationValueFn = func(ord int) float64 { return float64(itf.GetValue(ord)) }
	base.AggregateFn = func(a, b float64) float64 {
		return float64(aggregationFunction.AggregateInt(int32(a), int32(b)))
	}
	return itf
}

// initValues lazily allocates the values slice.
func (itf *IntTaxonomyFacets) initValues() {
	if itf.values == nil {
		itf.values = make([]int32, itf.TaxoReader.GetSize())
	}
}

// SetValue sets the accumulated int value for ordinal.
func (itf *IntTaxonomyFacets) SetValue(ord int, v int32) {
	itf.initValues()
	itf.values[ord] = v
}

// GetValue returns the accumulated int value for ordinal.
func (itf *IntTaxonomyFacets) GetValue(ord int) int32 {
	if itf.values == nil || ord < 0 || ord >= len(itf.values) {
		return 0
	}
	return itf.values[ord]
}

// AccumulateIntValue aggregates incoming into the running value for ordinal.
func (itf *IntTaxonomyFacets) AccumulateIntValue(ord int, incoming int32) {
	itf.initValues()
	itf.values[ord] = itf.AggregationFunction.AggregateInt(itf.values[ord], incoming)
}
