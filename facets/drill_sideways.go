// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// DrillSideways performs search with drill-sideways faceting.
// Unlike drill-down which only shows counts for the current selection,
// drill-sideways computes facet counts for all dimensions independently,
// allowing users to see counts for other facet values even when drilling down.
//
// This is the Go port of Lucene's org.apache.lucene.facet.DrillSideways.
type DrillSideways struct {
	// searcher is the index searcher used for queries
	searcher *search.IndexSearcher

	// config holds the facets configuration
	config *FacetsConfig

	// taxoReader is the taxonomy reader (may be nil for non-taxonomy facets)
	taxoReader *TaxonomyReader

	// facetsCollector is the collector used for facet counting
	facetsCollector *FacetsCollector
}

// DrillSidewaysSearchResult holds the results of a drill-sideways search.
type DrillSidewaysSearchResult struct {
	// Hits are the matching documents
	Hits *search.TopDocs

	// FacetResults contains the facet results for each dimension
	FacetResults map[string]*FacetResult

	// HitsCount is the total number of hits
	HitsCount int64
}

// NewDrillSideways creates a new DrillSideways instance.
func NewDrillSideways(searcher *search.IndexSearcher, config *FacetsConfig, taxoReader *TaxonomyReader) *DrillSideways {
	return &DrillSideways{
		searcher:   searcher,
		config:     config,
		taxoReader: taxoReader,
	}
}

// NewDrillSidewaysWithoutTaxonomy creates a new DrillSideways instance without taxonomy.
// This is used for SortedSetDocValues facets.
func NewDrillSidewaysWithoutTaxonomy(searcher *search.IndexSearcher, config *FacetsConfig) *DrillSideways {
	return &DrillSideways{
		searcher: searcher,
		config:   config,
	}
}

// Search performs a drill-sideways search with the given query and topN.
// Returns the search hits and facet counts for all dimensions.
func (ds *DrillSideways) Search(query search.Query, topN int) (*DrillSidewaysSearchResult, error) {
	return ds.SearchWithQuery(query, topN)
}

// SearchWithQuery performs a drill-sideways search with the given query.
func (ds *DrillSideways) SearchWithQuery(query search.Query, topN int) (*DrillSidewaysSearchResult, error) {
	// Create a collector for the main search
	collector := search.NewTopDocsCollector(topN)

	// Execute the search
	err := ds.searcher.SearchWithCollector(query, collector)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	hits := collector.TopDocs()

	// Get dimensions from config
	dimensions := ds.config.GetDims()

	// Compute facet results for each dimension
	facetResults := make(map[string]*FacetResult)

	for _, dim := range dimensions {
		result, err := ds.getFacetResult(query, dim)
		if err != nil {
			return nil, fmt.Errorf("failed to get facet result for dimension %s: %w", dim, err)
		}
		if result != nil {
			facetResults[dim] = result
		}
	}

	return &DrillSidewaysSearchResult{
		Hits:         hits,
		FacetResults: facetResults,
		HitsCount:    hits.TotalHits.Value,
	}, nil
}

// SearchWithCollector performs a drill-sideways search with a custom collector.
func (ds *DrillSideways) SearchWithCollector(query search.Query, collector search.Collector) (*DrillSidewaysSearchResult, error) {
	// Execute the search with the custom collector
	err := ds.searcher.SearchWithCollector(query, collector)
	if err != nil {
		return nil, fmt.Errorf("search with collector failed: %w", err)
	}

	// Get dimensions from config
	dimensions := ds.config.GetDims()

	// Compute facet results for each dimension
	facetResults := make(map[string]*FacetResult)

	for _, dim := range dimensions {
		result, err := ds.getFacetResult(query, dim)
		if err != nil {
			return nil, fmt.Errorf("failed to get facet result for dimension %s: %w", dim, err)
		}
		if result != nil {
			facetResults[dim] = result
		}
	}

	return &DrillSidewaysSearchResult{
		FacetResults: facetResults,
		HitsCount:    0, // Unknown with custom collector
	}, nil
}

// getFacetResult computes the facet result for a single dimension.
func (ds *DrillSideways) getFacetResult(query search.Query, dim string) (*FacetResult, error) {
	// Create a facets instance based on available components
	var facets Facets

	if ds.taxoReader != nil {
		// Use taxonomy-based facets
		// This would typically use FastTaxonomyFacetCounts
		// For now, return a placeholder result
		facets = ds.createTaxonomyFacets(dim)
	} else {
		// Use SortedSetDocValues facets
		facets = ds.createSortedSetFacets(dim)
	}

	if facets == nil {
		return nil, nil
	}

	// Get top children for this dimension
	return facets.GetTopChildren(10, dim)
}

// createTaxonomyFacets creates a taxonomy-based facets instance.
func (ds *DrillSideways) createTaxonomyFacets(dim string) Facets {
	if ds.taxoReader == nil {
		return nil
	}
	// This would create a FastTaxonomyFacetCounts instance
	// Implementation depends on GC-427
	return nil
}

// createSortedSetFacets creates a SortedSetDocValues-based facets instance.
func (ds *DrillSideways) createSortedSetFacets(dim string) Facets {
	// This would create a SortedSetDocValuesFacetCounts instance
	// Implementation depends on GC-428
	return nil
}

// GetFacets returns the facets for each dimension after a search.
func (dsr *DrillSidewaysSearchResult) GetFacets() map[string]*FacetResult {
	return dsr.FacetResults
}

// GetFacetResult returns the facet result for a specific dimension.
func (dsr *DrillSidewaysSearchResult) GetFacetResult(dim string) *FacetResult {
	if dsr.FacetResults == nil {
		return nil
	}
	return dsr.FacetResults[dim]
}

// GetHits returns the search hits.
func (dsr *DrillSidewaysSearchResult) GetHits() *search.TopDocs {
	return dsr.Hits
}

// GetHitsCount returns the total number of hits.
func (dsr *DrillSidewaysSearchResult) GetHitsCount() int64 {
	return dsr.HitsCount
}

// GetOrds returns the ordinals for a dimension.
// This is used internally for taxonomy-based facets.
func (ds *DrillSideways) GetOrds(dim string) ([]int, error) {
	if ds.taxoReader == nil {
		return nil, fmt.Errorf("taxonomy reader not available")
	}
	// Implementation would query the taxonomy reader
	return nil, nil
}

// GetCardinality returns the cardinality (number of unique values) for a dimension.
func (ds *DrillSideways) GetCardinality(dim string) (int, error) {
	if ds.taxoReader == nil {
		return 0, fmt.Errorf("taxonomy reader not available")
	}
	// Implementation would query the taxonomy reader
	return 0, nil
}

// GetChildCount returns the number of child facets for a given dimension and path.
func (ds *DrillSideways) GetChildCount(dim string, path []string) (int, error) {
	result, err := ds.getFacetResult(nil, dim)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.ChildCount, nil
}

// GetValue returns the value/count for a specific facet label.
func (ds *DrillSideways) GetValue(dim string, path []string) (int64, error) {
	result, err := ds.getFacetResult(nil, dim)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	// Find the specific value in the result
	for _, lv := range result.LabelValues {
		// Compare path components with label
		labelPath := []string{lv.Label}
		if pathMatches(labelPath, path) {
			return lv.Value, nil
		}
	}
	return 0, nil
}

// pathMatches checks if two paths match.
func pathMatches(path1, path2 []string) bool {
	if len(path1) != len(path2) {
		return false
	}
	for i, p := range path1 {
		if p != path2[i] {
			return false
		}
	}
	return true
}

// GetPath returns the path for a given ordinal.
func (ds *DrillSideways) GetPath(ordinal int) ([]string, error) {
	if ds.taxoReader == nil {
		return nil, fmt.Errorf("taxonomy reader not available")
	}
	// Implementation would query the taxonomy reader
	return nil, nil
}

// GetLabel returns the label for a given ordinal.
func (ds *DrillSideways) GetLabel(ordinal int) (*FacetLabel, error) {
	if ds.taxoReader == nil {
		return nil, fmt.Errorf("taxonomy reader not available")
	}
	// Implementation would query the taxonomy reader
	return nil, nil
}

// GetTopChildren returns the top N children for a dimension.
func (ds *DrillSideways) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
	// Create a facets instance
	var facets Facets
	if ds.taxoReader != nil {
		facets = ds.createTaxonomyFacets(dim)
	} else {
		facets = ds.createSortedSetFacets(dim)
	}

	if facets == nil {
		return nil, fmt.Errorf("could not create facets for dimension %s", dim)
	}

	return facets.GetTopChildren(topN, dim, path...)
}

// DrillSidewaysQuery wraps a query for drill-sideways processing.
// This is used internally to track which dimensions are being drilled into.
type DrillSidewaysQuery struct {
	// BaseQuery is the underlying query
	BaseQuery search.Query

	// DrillDownDimensions are the dimensions being drilled down
	DrillDownDimensions []string

	// DrillDownQueries are the queries for each drill-down dimension
	DrillDownQueries map[string]search.Query
}

// NewDrillSidewaysQuery creates a new DrillSidewaysQuery.
func NewDrillSidewaysQuery(baseQuery search.Query) *DrillSidewaysQuery {
	return &DrillSidewaysQuery{
		BaseQuery:           baseQuery,
		DrillDownDimensions: make([]string, 0),
		DrillDownQueries:    make(map[string]search.Query),
	}
}

// AddDrillDown adds a drill-down clause for the specified dimension.
func (dsq *DrillSidewaysQuery) AddDrillDown(dim string, query search.Query) {
	dsq.DrillDownDimensions = append(dsq.DrillDownDimensions, dim)
	dsq.DrillDownQueries[dim] = query
}

// GetDrillDownQuery returns the drill-down query for a dimension.
func (dsq *DrillSidewaysQuery) GetDrillDownQuery(dim string) search.Query {
	return dsq.DrillDownQueries[dim]
}

// GetDrillDownDimensions returns the dimensions being drilled down.
func (dsq *DrillSidewaysQuery) GetDrillDownDimensions() []string {
	result := make([]string, len(dsq.DrillDownDimensions))
	copy(result, dsq.DrillDownDimensions)
	return result
}

// HasDrillDown returns true if there are any drill-down clauses.
func (dsq *DrillSidewaysQuery) HasDrillDown() bool {
	return len(dsq.DrillDownDimensions) > 0
}

// Rewrite rewrites the query.
func (dsq *DrillSidewaysQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	if dsq.BaseQuery == nil {
		return dsq, nil
	}
	rewritten, err := dsq.BaseQuery.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	dsq.BaseQuery = rewritten
	return dsq, nil
}

// Clone creates a copy of this query.
func (dsq *DrillSidewaysQuery) Clone() search.Query {
	cloned := &DrillSidewaysQuery{
		BaseQuery:           dsq.BaseQuery.Clone(),
		DrillDownDimensions: make([]string, len(dsq.DrillDownDimensions)),
		DrillDownQueries:    make(map[string]search.Query),
	}
	copy(cloned.DrillDownDimensions, dsq.DrillDownDimensions)
	for k, v := range dsq.DrillDownQueries {
		cloned.DrillDownQueries[k] = v.Clone()
	}
	return cloned
}

// Equals checks if this query equals another.
func (dsq *DrillSidewaysQuery) Equals(other search.Query) bool {
	otherDSQ, ok := other.(*DrillSidewaysQuery)
	if !ok {
		return false
	}
	if !dsq.BaseQuery.Equals(otherDSQ.BaseQuery) {
		return false
	}
	if len(dsq.DrillDownDimensions) != len(otherDSQ.DrillDownDimensions) {
		return false
	}
	for i, dim := range dsq.DrillDownDimensions {
		if dim != otherDSQ.DrillDownDimensions[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (dsq *DrillSidewaysQuery) HashCode() int {
	h := dsq.BaseQuery.HashCode()
	for _, dim := range dsq.DrillDownDimensions {
		for _, c := range dim {
			h = 31*h + int(c)
		}
	}
	return h
}

// CreateWeight creates a Weight for this query.
func (dsq *DrillSidewaysQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	// This would create a DrillSidewaysWeight
	// For now, delegate to base query
	if dsq.BaseQuery != nil {
		return dsq.BaseQuery.CreateWeight(searcher, needsScores, boost)
	}
	return nil, fmt.Errorf("no base query")
}
