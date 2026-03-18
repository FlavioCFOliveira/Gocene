// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewDrillSideways(t *testing.T) {
	// Create a mock searcher (would need a real index in practice)
	var searcher *search.IndexSearcher

	config := NewFacetsConfig()
	config.SetMultiValued("category", true)

	// Test with taxonomy reader
	taxoReader := NewTaxonomyReader()
	ds := NewDrillSideways(searcher, config, taxoReader)

	if ds == nil {
		t.Fatal("expected DrillSideways to be created")
	}
	if ds.searcher != searcher {
		t.Error("expected searcher to be set")
	}
	if ds.config != config {
		t.Error("expected config to be set")
	}
	if ds.taxoReader != taxoReader {
		t.Error("expected taxoReader to be set")
	}
}

func TestNewDrillSidewaysWithoutTaxonomy(t *testing.T) {
	var searcher *search.IndexSearcher
	config := NewFacetsConfig()

	ds := NewDrillSidewaysWithoutTaxonomy(searcher, config)

	if ds == nil {
		t.Fatal("expected DrillSideways to be created")
	}
	if ds.taxoReader != nil {
		t.Error("expected taxoReader to be nil")
	}
}

func TestDrillSidewaysSearchResult(t *testing.T) {
	result := &DrillSidewaysSearchResult{
		FacetResults: make(map[string]*FacetResult),
		HitsCount:    100,
	}

	// Add a facet result
	facetResult := &FacetResult{
		Dim:        "category",
		ChildCount: 5,
		LabelValues: []*LabelAndValue{
			{Label: "electronics", Value: 50},
			{Label: "books", Value: 30},
		},
	}
	result.FacetResults["category"] = facetResult

	// Test GetFacets
	facets := result.GetFacets()
	if len(facets) != 1 {
		t.Errorf("expected 1 facet result, got %d", len(facets))
	}

	// Test GetFacetResult
	fr := result.GetFacetResult("category")
	if fr == nil {
		t.Error("expected facet result for category")
	}
	if fr.Dim != "category" {
		t.Errorf("expected dim 'category', got %s", fr.Dim)
	}

	// Test non-existent dimension
	fr = result.GetFacetResult("nonexistent")
	if fr != nil {
		t.Error("expected nil for non-existent dimension")
	}

	// Test GetHitsCount
	if result.GetHitsCount() != 100 {
		t.Errorf("expected hits count 100, got %d", result.GetHitsCount())
	}
}

func TestNewDrillSidewaysQuery(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	if dsq == nil {
		t.Fatal("expected DrillSidewaysQuery to be created")
	}
	if dsq.BaseQuery == nil {
		t.Error("expected base query to be set")
	}
	if len(dsq.DrillDownDimensions) != 0 {
		t.Error("expected empty drill down dimensions")
	}
	if len(dsq.DrillDownQueries) != 0 {
		t.Error("expected empty drill down queries")
	}
}

func TestDrillSidewaysQueryAddDrillDown(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	// Add drill down
	termQuery := search.NewTermQuery(nil)
	dsq.AddDrillDown("category", termQuery)

	if len(dsq.DrillDownDimensions) != 1 {
		t.Errorf("expected 1 drill down dimension, got %d", len(dsq.DrillDownDimensions))
	}
	if dsq.DrillDownDimensions[0] != "category" {
		t.Errorf("expected dimension 'category', got %s", dsq.DrillDownDimensions[0])
	}

	// Test GetDrillDownQuery
	q := dsq.GetDrillDownQuery("category")
	if q == nil {
		t.Error("expected drill down query for category")
	}

	// Test GetDrillDownDimensions
	dims := dsq.GetDrillDownDimensions()
	if len(dims) != 1 {
		t.Errorf("expected 1 dimension, got %d", len(dims))
	}

	// Test HasDrillDown
	if !dsq.HasDrillDown() {
		t.Error("expected HasDrillDown to be true")
	}
}

func TestDrillSidewaysQueryClone(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	termQuery := search.NewTermQuery(nil)
	dsq.AddDrillDown("category", termQuery)

	cloned := dsq.Clone()
	if cloned == nil {
		t.Fatal("expected cloned query")
	}

	clonedDSQ, ok := cloned.(*DrillSidewaysQuery)
	if !ok {
		t.Fatal("expected cloned to be DrillSidewaysQuery")
	}

	if len(clonedDSQ.DrillDownDimensions) != 1 {
		t.Error("expected cloned to have drill down dimensions")
	}
}

func TestDrillSidewaysQueryEquals(t *testing.T) {
	baseQuery1 := search.NewMatchAllDocsQuery()
	dsq1 := NewDrillSidewaysQuery(baseQuery1)

	baseQuery2 := search.NewMatchAllDocsQuery()
	dsq2 := NewDrillSidewaysQuery(baseQuery2)

	// Same base query, no drill downs - should be equal
	if !dsq1.Equals(dsq2) {
		t.Error("expected queries to be equal")
	}

	// Add drill down to one
	termQuery := search.NewTermQuery(nil)
	dsq1.AddDrillDown("category", termQuery)

	if dsq1.Equals(dsq2) {
		t.Error("expected queries to not be equal after adding drill down")
	}
}

func TestDrillSidewaysQueryHashCode(t *testing.T) {
	baseQuery := search.NewMatchAllDocsQuery()
	dsq := NewDrillSidewaysQuery(baseQuery)

	hc1 := dsq.HashCode()

	// Add drill down
	termQuery := search.NewTermQuery(nil)
	dsq.AddDrillDown("category", termQuery)

	hc2 := dsq.HashCode()

	// Hash codes should be different
	if hc1 == hc2 {
		t.Error("expected hash codes to be different after adding drill down")
	}
}

func TestPathMatches(t *testing.T) {
	tests := []struct {
		path1    []string
		path2    []string
		expected bool
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a", "b"}, []string{"a", "c"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{}, []string{}, true},
		{[]string{"a"}, []string{"b"}, false},
	}

	for _, test := range tests {
		result := pathMatches(test.path1, test.path2)
		if result != test.expected {
			t.Errorf("pathMatches(%v, %v) = %v, expected %v", test.path1, test.path2, result, test.expected)
		}
	}
}
