// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

// FastTaxonomyFacetCounts computes facet counts using the taxonomy index.
// It reads per-document ordinals from SortedNumericDocValues indexed by
// FacetsConfig.BuildWithTaxonomy, counts them, and rolls up the hierarchy.
//
// This is the Go port of Lucene's
// org.apache.lucene.facet.taxonomy.FastTaxonomyFacetCounts.
type FastTaxonomyFacetCounts struct {
	*IntTaxonomyFacets
}

// NewFastTaxonomyFacetCounts creates a new FastTaxonomyFacetCounts for the
// given index field, taxonomy reader, and facets configuration.
//
// When indexFieldName is empty, the default field name "$facets" is used.
func NewFastTaxonomyFacetCounts(
	indexFieldName string,
	taxoReader TaxonomyReaderI,
	config *facets.FacetsConfig,
) *FastTaxonomyFacetCounts {
	if indexFieldName == "" {
		indexFieldName = "$facets"
	}
	itf := NewIntTaxonomyFacets(indexFieldName, taxoReader, config, SUM)
	ftfc := &FastTaxonomyFacetCounts{IntTaxonomyFacets: itf}

	// Override GetAggregationValueFn to use the count buffer rather than the
	// int32 values buffer. FastTaxonomyFacetCounts counts occurrences via
	// IncrementCount (which populates the base counts[]), not via the
	// IntTaxonomyFacets.values[] which is designed for association aggregation.
	itf.TaxonomyFacets.GetAggregationValueFn = func(ord int) float64 {
		return float64(itf.TaxonomyFacets.getCount(ord))
	}
	return ftfc
}

// Accumulate iterates the SortedNumericDocValues of every matching-docs
// segment and increments the count for each ordinal found, then rolls up
// the hierarchy. This is the main entry point after collecting search hits
// via FacetsCollector.
//
// Mirrors FastTaxonomyFacetCounts.count(FacetsCollector).
func (ftfc *FastTaxonomyFacetCounts) Accumulate(matchingDocs []*facets.MatchingDocs) error {
	ftfc.initCounters()
	for _, md := range matchingDocs {
		if err := ftfc.accumulateSegment(md); err != nil {
			return err
		}
	}
	return ftfc.Rollup()
}

// accumulateSegment counts ordinals from one segment's SortedNumericDocValues.
func (ftfc *FastTaxonomyFacetCounts) accumulateSegment(md *facets.MatchingDocs) error {
	field := ftfc.IndexFieldName
	return facets.ForEachTaxonomyOrdinal(md, field, func(docID, ord int) {
		ftfc.IncrementCount(ord, 1)
	})
}

// GetTopChildren returns the top N child counts for dim+path, sorted
// descending by count. Returns nil when the dimension is unknown or has
// no matching children.
func (ftfc *FastTaxonomyFacetCounts) GetTopChildren(topN int, dim string, path ...string) (*facets.FacetResult, error) {
	return ftfc.TaxonomyFacets.GetTopChildren(topN, dim, path...)
}

// GetAllChildren returns all children with positive count for dim+path.
func (ftfc *FastTaxonomyFacetCounts) GetAllChildren(dim string, path ...string) (*facets.FacetResult, error) {
	return ftfc.TaxonomyFacets.GetAllChildren(dim, path...)
}

// GetSpecificValue returns the count for the exact path dim+path, or -1
// when the path does not exist.
func (ftfc *FastTaxonomyFacetCounts) GetSpecificValue(dim string, path ...string) (*facets.FacetResult, error) {
	components := append([]string{dim}, path...)
	ord := ftfc.TaxoReader.GetOrdinal(components...)
	if ord < 0 {
		return nil, fmt.Errorf("path '%s' not found", dim)
	}
	if !ftfc.HasValues() {
		return nil, nil
	}
	count := ftfc.getCount(ord)
	result := facets.NewFacetResultWithPath(dim, path)
	result.Value = int64(count)
	if len(path) > 0 {
		result.AddLabelValue(facets.NewLabelAndValue(path[len(path)-1], int64(count)))
	} else {
		result.AddLabelValue(facets.NewLabelAndValue(dim, int64(count)))
	}
	return result, nil
}

// GetAllDims returns one FacetResult per dimension, sorted by count desc.
func (ftfc *FastTaxonomyFacetCounts) GetAllDims(topN int) ([]*facets.FacetResult, error) {
	return ftfc.TaxonomyFacets.GetAllDims(topN)
}
