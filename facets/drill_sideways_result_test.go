// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewDrillSidewaysResult(t *testing.T) {
	result := NewDrillSidewaysResult("category")

	if result == nil {
		t.Fatal("expected result to be created")
	}
	if result.Dim != "category" {
		t.Errorf("expected dim 'category', got %s", result.Dim)
	}
	if len(result.Path) != 0 {
		t.Error("expected empty path")
	}
	if len(result.LabelValues) != 0 {
		t.Error("expected empty label values")
	}
}

func TestNewDrillSidewaysResultWithPath(t *testing.T) {
	path := []string{"electronics", "phones"}
	result := NewDrillSidewaysResultWithPath("category", path)

	if len(result.Path) != 2 {
		t.Errorf("expected path length 2, got %d", len(result.Path))
	}
	if result.Path[0] != "electronics" {
		t.Errorf("expected path[0] 'electronics', got %s", result.Path[0])
	}
}

func TestDrillSidewaysResultAddLabelValue(t *testing.T) {
	result := NewDrillSidewaysResult("category")
	result.AddLabelValue(&LabelAndValue{Label: "electronics", Value: 10})
	result.AddLabelValue(&LabelAndValue{Label: "books", Value: 5})

	if len(result.LabelValues) != 2 {
		t.Errorf("expected 2 label values, got %d", len(result.LabelValues))
	}
}

func TestDrillSidewaysResultGetTotalCount(t *testing.T) {
	result := NewDrillSidewaysResult("category")
	result.AddLabelValue(&LabelAndValue{Label: "electronics", Value: 10})
	result.AddLabelValue(&LabelAndValue{Label: "books", Value: 5})

	total := result.GetTotalCount()
	if total != 15 {
		t.Errorf("expected total 15, got %d", total)
	}
}

func TestDrillSidewaysResultSize(t *testing.T) {
	result := NewDrillSidewaysResult("category")
	if result.Size() != 0 {
		t.Errorf("expected size 0, got %d", result.Size())
	}

	result.AddLabelValue(&LabelAndValue{Label: "electronics", Value: 10})
	if result.Size() != 1 {
		t.Errorf("expected size 1, got %d", result.Size())
	}
}

func TestDrillSidewaysResultIsEmpty(t *testing.T) {
	result := NewDrillSidewaysResult("category")
	if !result.IsEmpty() {
		t.Error("expected IsEmpty to be true")
	}

	result.AddLabelValue(&LabelAndValue{Label: "electronics", Value: 10})
	if result.IsEmpty() {
		t.Error("expected IsEmpty to be false")
	}
}

func TestDrillSidewaysResultSortByValue(t *testing.T) {
	result := NewDrillSidewaysResult("category")
	result.AddLabelValue(&LabelAndValue{Label: "books", Value: 5})
	result.AddLabelValue(&LabelAndValue{Label: "electronics", Value: 10})
	result.AddLabelValue(&LabelAndValue{Label: "clothing", Value: 3})

	result.SortByValue()

	if result.LabelValues[0].Value != 10 {
		t.Errorf("expected first value 10, got %d", result.LabelValues[0].Value)
	}
	if result.LabelValues[1].Value != 5 {
		t.Errorf("expected second value 5, got %d", result.LabelValues[1].Value)
	}
	if result.LabelValues[2].Value != 3 {
		t.Errorf("expected third value 3, got %d", result.LabelValues[2].Value)
	}
}

func TestDrillSidewaysResultSortByLabel(t *testing.T) {
	result := NewDrillSidewaysResult("category")
	result.AddLabelValue(&LabelAndValue{Label: "electronics", Value: 10})
	result.AddLabelValue(&LabelAndValue{Label: "books", Value: 5})
	result.AddLabelValue(&LabelAndValue{Label: "clothing", Value: 3})

	result.SortByLabel()

	if result.LabelValues[0].Label != "books" {
		t.Errorf("expected first label 'books', got %s", result.LabelValues[0].Label)
	}
	if result.LabelValues[1].Label != "clothing" {
		t.Errorf("expected second label 'clothing', got %s", result.LabelValues[1].Label)
	}
	if result.LabelValues[2].Label != "electronics" {
		t.Errorf("expected third label 'electronics', got %s", result.LabelValues[2].Label)
	}
}

func TestDrillSidewaysResultGetTopN(t *testing.T) {
	result := NewDrillSidewaysResult("category")
	result.AddLabelValue(&LabelAndValue{Label: "electronics", Value: 10})
	result.AddLabelValue(&LabelAndValue{Label: "books", Value: 5})
	result.AddLabelValue(&LabelAndValue{Label: "clothing", Value: 3})

	top2 := result.GetTopN(2)
	if len(top2) != 2 {
		t.Errorf("expected 2 results, got %d", len(top2))
	}

	top10 := result.GetTopN(10)
	if len(top10) != 3 {
		t.Errorf("expected 3 results, got %d", len(top10))
	}
}

func TestDrillSidewaysResultString(t *testing.T) {
	result := NewDrillSidewaysResult("category")
	result.Value = 100
	result.ChildCount = 5
	result.AddLabelValue(&LabelAndValue{Label: "electronics", Value: 10})

	s := result.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
}

func TestNewDrillSidewaysResults(t *testing.T) {
	results := NewDrillSidewaysResults()

	if results == nil {
		t.Fatal("expected results to be created")
	}
	if len(results.Results) != 0 {
		t.Error("expected empty results")
	}
}

func TestDrillSidewaysResultsAddResult(t *testing.T) {
	results := NewDrillSidewaysResults()
	result := NewDrillSidewaysResult("category")
	results.AddResult("category", result)

	if len(results.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results.Results))
	}
}

func TestDrillSidewaysResultsGetResult(t *testing.T) {
	results := NewDrillSidewaysResults()
	result := NewDrillSidewaysResult("category")
	results.AddResult("category", result)

	retrieved := results.GetResult("category")
	if retrieved == nil {
		t.Error("expected to retrieve result")
	}
	if retrieved.Dim != "category" {
		t.Errorf("expected dim 'category', got %s", retrieved.Dim)
	}

	// Non-existent dimension
	if results.GetResult("nonexistent") != nil {
		t.Error("expected nil for non-existent dimension")
	}
}

func TestDrillSidewaysResultsGetDimensions(t *testing.T) {
	results := NewDrillSidewaysResults()
	results.AddResult("category", NewDrillSidewaysResult("category"))
	results.AddResult("brand", NewDrillSidewaysResult("brand"))

	dims := results.GetDimensions()
	if len(dims) != 2 {
		t.Errorf("expected 2 dimensions, got %d", len(dims))
	}

	// Should be sorted
	if dims[0] != "brand" {
		t.Errorf("expected first dim 'brand', got %s", dims[0])
	}
	if dims[1] != "category" {
		t.Errorf("expected second dim 'category', got %s", dims[1])
	}
}

func TestDrillSidewaysResultsIsEmpty(t *testing.T) {
	results := NewDrillSidewaysResults()
	if !results.IsEmpty() {
		t.Error("expected IsEmpty to be true")
	}

	results.AddResult("category", NewDrillSidewaysResult("category"))
	if results.IsEmpty() {
		t.Error("expected IsEmpty to be false")
	}
}

func TestDrillSidewaysResultsSize(t *testing.T) {
	results := NewDrillSidewaysResults()
	if results.Size() != 0 {
		t.Errorf("expected size 0, got %d", results.Size())
	}

	results.AddResult("category", NewDrillSidewaysResult("category"))
	results.AddResult("brand", NewDrillSidewaysResult("brand"))
	if results.Size() != 2 {
		t.Errorf("expected size 2, got %d", results.Size())
	}
}

func TestDrillSidewaysResultsToSearchResult(t *testing.T) {
	results := NewDrillSidewaysResults()
	results.TotalHits = 100

	result := NewDrillSidewaysResult("category")
	result.Value = 50
	result.ChildCount = 3
	result.AddLabelValue(&LabelAndValue{Label: "electronics", Value: 10})
	results.AddResult("category", result)

	hits := &search.TopDocs{}
	searchResult := results.ToSearchResult(hits)

	if searchResult == nil {
		t.Fatal("expected search result")
	}
	if searchResult.HitsCount != 100 {
		t.Errorf("expected hits count 100, got %d", searchResult.HitsCount)
	}
	if len(searchResult.FacetResults) != 1 {
		t.Errorf("expected 1 facet result, got %d", len(searchResult.FacetResults))
	}
}

func TestNewDrillSidewaysResultBuilder(t *testing.T) {
	builder := NewDrillSidewaysResultBuilder("category")
	if builder == nil {
		t.Fatal("expected builder to be created")
	}
	if builder.result == nil {
		t.Fatal("expected result to be initialized")
	}
}

func TestDrillSidewaysResultBuilder(t *testing.T) {
	result := NewDrillSidewaysResultBuilder("category").
		SetPath([]string{"electronics"}).
		SetValue(100).
		SetChildCount(5).
		AddLabelValue("phones", 30).
		AddLabelValue("laptops", 20).
		Build()

	if result.Dim != "category" {
		t.Errorf("expected dim 'category', got %s", result.Dim)
	}
	if len(result.Path) != 1 || result.Path[0] != "electronics" {
		t.Error("expected path to be set")
	}
	if result.Value != 100 {
		t.Errorf("expected value 100, got %d", result.Value)
	}
	if result.ChildCount != 5 {
		t.Errorf("expected child count 5, got %d", result.ChildCount)
	}
	if len(result.LabelValues) != 2 {
		t.Errorf("expected 2 label values, got %d", len(result.LabelValues))
	}
}

func TestNewDrillSidewaysFacetResult(t *testing.T) {
	result := NewDrillSidewaysFacetResult("category")

	if result == nil {
		t.Fatal("expected result to be created")
	}
	if result.Dim != "category" {
		t.Errorf("expected dim 'category', got %s", result.Dim)
	}
	if result.DrillSidewaysCount != 0 {
		t.Error("expected drill sideways count to be 0")
	}
	if result.IsDrillDown {
		t.Error("expected IsDrillDown to be false")
	}
}

func TestDrillSidewaysFacetResultSetters(t *testing.T) {
	result := NewDrillSidewaysFacetResult("category")
	result.SetDrillSidewaysCount(50)
	result.SetIsDrillDown(true)

	if result.GetDrillSidewaysCount() != 50 {
		t.Errorf("expected drill sideways count 50, got %d", result.GetDrillSidewaysCount())
	}
	if !result.GetIsDrillDown() {
		t.Error("expected IsDrillDown to be true")
	}
}
