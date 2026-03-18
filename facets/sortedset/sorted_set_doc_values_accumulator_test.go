// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

func TestNewSortedSetDocValuesAccumulator(t *testing.T) {
	tests := []struct {
		name        string
		config      *facets.FacetsConfig
		field       string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid config and field",
			config:  facets.NewFacetsConfig(),
			field:   "category",
			wantErr: false,
		},
		{
			name:        "nil config",
			config:      nil,
			field:       "category",
			wantErr:     true,
			errContains: "facets config cannot be nil",
		},
		{
			name:        "empty field",
			config:      facets.NewFacetsConfig(),
			field:       "",
			wantErr:     true,
			errContains: "field name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, err := NewSortedSetDocValuesAccumulator(tt.config, tt.field)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if acc == nil {
				t.Errorf("expected accumulator, got nil")
				return
			}

			// Verify initial state
			if acc.GetField() != tt.field {
				t.Errorf("expected field %q, got %q", tt.field, acc.GetField())
			}
			if acc.GetConfig() != tt.config {
				t.Errorf("expected config to match")
			}
			if !acc.IsHierarchical() {
				t.Errorf("expected hierarchical to be true by default")
			}
			if acc.GetMaxCategories() != 10000 {
				t.Errorf("expected max categories 10000, got %d", acc.GetMaxCategories())
			}
		})
	}
}

func TestSortedSetDocValuesAccumulator_Accumulate(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Create some facet results
	result1 := facets.NewFacetResult("category")
	result1.AddLabelValue(facets.NewLabelAndValue("category/electronics", 10))
	result1.AddLabelValue(facets.NewLabelAndValue("category/books", 5))

	result2 := facets.NewFacetResult("category")
	result2.AddLabelValue(facets.NewLabelAndValue("category/electronics", 5))
	result2.AddLabelValue(facets.NewLabelAndValue("category/clothing", 8))

	// Accumulate results
	err = acc.Accumulate([]*facets.FacetResult{result1, result2})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify counts
	electronicsOrd := acc.labelToOrd["category/electronics"]
	booksOrd := acc.labelToOrd["category/books"]
	clothingOrd := acc.labelToOrd["category/clothing"]

	if acc.GetCount(electronicsOrd) != 15 {
		t.Errorf("expected electronics count 15, got %d", acc.GetCount(electronicsOrd))
	}
	if acc.GetCount(booksOrd) != 5 {
		t.Errorf("expected books count 5, got %d", acc.GetCount(booksOrd))
	}
	if acc.GetCount(clothingOrd) != 8 {
		t.Errorf("expected clothing count 8, got %d", acc.GetCount(clothingOrd))
	}

	// Test with nil results
	err = acc.Accumulate(nil)
	if err != nil {
		t.Errorf("unexpected error with nil results: %v", err)
	}

	// Test with empty results
	err = acc.Accumulate([]*facets.FacetResult{})
	if err != nil {
		t.Errorf("unexpected error with empty results: %v", err)
	}
}

func TestSortedSetDocValuesAccumulator_GetTopChildren(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Add some counts
	acc.IncrementCount(acc.getOrCreateOrdinal("category/electronics"), 100)
	acc.IncrementCount(acc.getOrCreateOrdinal("category/books"), 50)
	acc.IncrementCount(acc.getOrCreateOrdinal("category/clothing"), 75)
	acc.IncrementCount(acc.getOrCreateOrdinal("category/food"), 25)

	tests := []struct {
		name          string
		topN          int
		dim           string
		path          []string
		wantErr       bool
		expectedCount int
		expectedTotal int64
	}{
		{
			name:          "top 2 children",
			topN:          2,
			dim:           "category",
			expectedCount: 2,
			expectedTotal: 250, // Sum of all matching (implementation sums all, not just top N)
		},
		{
			name:          "top 10 children (more than available)",
			topN:          10,
			dim:           "category",
			expectedCount: 4,
			expectedTotal: 250,
		},
		{
			name:          "top 1 child",
			topN:          1,
			dim:           "category",
			expectedCount: 1,
			expectedTotal: 250, // Sum of all matching (implementation sums all, not just top N)
		},
		{
			name:    "invalid topN",
			topN:    0,
			dim:     "category",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := acc.GetTopChildren(tt.topN, tt.dim, tt.path...)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result == nil {
				t.Errorf("expected result, got nil")
				return
			}

			if result.ChildCount != tt.expectedCount {
				t.Errorf("expected %d children, got %d", tt.expectedCount, result.ChildCount)
			}
			if result.Value != tt.expectedTotal {
				t.Errorf("expected total %d, got %d", tt.expectedTotal, result.Value)
			}
		})
	}
}

func TestSortedSetDocValuesAccumulator_GetAllChildren(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Add some counts
	acc.IncrementCount(acc.getOrCreateOrdinal("category/electronics"), 100)
	acc.IncrementCount(acc.getOrCreateOrdinal("category/books"), 50)

	result, err := acc.GetAllChildren("category")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if result.ChildCount != 2 {
		t.Errorf("expected 2 children, got %d", result.ChildCount)
	}
	if result.Value != 150 {
		t.Errorf("expected total 150, got %d", result.Value)
	}
}

func TestSortedSetDocValuesAccumulator_GetSpecificValue(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Add a count
	acc.IncrementCount(acc.getOrCreateOrdinal("category/electronics"), 42)

	result, err := acc.GetSpecificValue("category", "electronics")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if result.Value != 42 {
		t.Errorf("expected value 42, got %d", result.Value)
	}
	if len(result.Path) != 1 || result.Path[0] != "electronics" {
		t.Errorf("expected path [electronics], got %v", result.Path)
	}
}

func TestSortedSetDocValuesAccumulator_GetDimensions(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Initially should be empty
	dims := acc.GetDimensions()
	if len(dims) != 0 {
		t.Errorf("expected 0 dimensions initially, got %d", len(dims))
	}

	// Add some labels
	acc.getOrCreateOrdinal("category/electronics")
	acc.getOrCreateOrdinal("category/books")
	acc.getOrCreateOrdinal("color/red")

	dims = acc.GetDimensions()
	if len(dims) != 2 {
		t.Errorf("expected 2 dimensions, got %d", len(dims))
	}

	// Check that dimensions are sorted
	if dims[0] != "category" || dims[1] != "color" {
		t.Errorf("expected dimensions [category, color], got %v", dims)
	}
}

func TestSortedSetDocValuesAccumulator_Reset(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Add some counts
	acc.IncrementCount(acc.getOrCreateOrdinal("category/electronics"), 100)

	// Verify not empty
	if acc.IsEmpty() {
		t.Errorf("expected accumulator to not be empty")
	}

	// Reset
	acc.Reset()

	// Verify empty
	if !acc.IsEmpty() {
		t.Errorf("expected accumulator to be empty after reset")
	}

	// Verify counts cleared
	dims := acc.GetDimensions()
	if len(dims) != 0 {
		t.Errorf("expected 0 dimensions after reset, got %d", len(dims))
	}
}

func TestSortedSetDocValuesAccumulator_IsEmpty(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Initially should be empty
	if !acc.IsEmpty() {
		t.Errorf("expected accumulator to be empty initially")
	}

	// Add a count
	acc.IncrementCount(acc.getOrCreateOrdinal("category/electronics"), 1)

	// Should not be empty
	if acc.IsEmpty() {
		t.Errorf("expected accumulator to not be empty after adding count")
	}
}

func TestSortedSetDocValuesAccumulator_GetCount(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Get count for non-existent ordinal
	if acc.GetCount(999) != 0 {
		t.Errorf("expected 0 for non-existent ordinal")
	}

	// Get count for invalid ordinal
	if acc.GetCount(-1) != 0 {
		t.Errorf("expected 0 for invalid ordinal")
	}

	// Add a count and verify
	ord := acc.getOrCreateOrdinal("category/electronics")
	acc.IncrementCount(ord, 42)
	if acc.GetCount(ord) != 42 {
		t.Errorf("expected count 42, got %d", acc.GetCount(ord))
	}
}

func TestSortedSetDocValuesAccumulator_SetHierarchical(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Default should be true
	if !acc.IsHierarchical() {
		t.Errorf("expected hierarchical to be true by default")
	}

	// Set to false
	acc.SetHierarchical(false)
	if acc.IsHierarchical() {
		t.Errorf("expected hierarchical to be false")
	}
}

func TestSortedSetDocValuesAccumulator_SetMaxCategories(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Default should be 10000
	if acc.GetMaxCategories() != 10000 {
		t.Errorf("expected max categories 10000 by default")
	}

	// Set to 100
	acc.SetMaxCategories(100)
	if acc.GetMaxCategories() != 100 {
		t.Errorf("expected max categories 100")
	}
}

func TestSortedSetDocValuesAccumulatorFactory(t *testing.T) {
	config := facets.NewFacetsConfig()
	factory := NewSortedSetDocValuesAccumulatorFactory(config, "category")

	acc, err := factory.CreateAccumulator()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if acc == nil {
		t.Errorf("expected accumulator, got nil")
		return
	}

	if acc.GetField() != "category" {
		t.Errorf("expected field category, got %s", acc.GetField())
	}
}

func TestSortedSetDocValuesAccumulatorBuilder(t *testing.T) {
	config := facets.NewFacetsConfig()

	acc, err := NewSortedSetDocValuesAccumulatorBuilder(config, "category").
		SetHierarchical(false).
		SetMaxCategories(5000).
		Build()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if acc.IsHierarchical() {
		t.Errorf("expected hierarchical to be false")
	}

	if acc.GetMaxCategories() != 5000 {
		t.Errorf("expected max categories 5000, got %d", acc.GetMaxCategories())
	}
}

func TestSortedSetDocValuesAccumulatorBuilder_BuildError(t *testing.T) {
	// Test with nil config
	_, err := NewSortedSetDocValuesAccumulatorBuilder(nil, "category").Build()
	if err == nil {
		t.Errorf("expected error with nil config")
	}

	// Test with empty field
	_, err = NewSortedSetDocValuesAccumulatorBuilder(facets.NewFacetsConfig(), "").Build()
	if err == nil {
		t.Errorf("expected error with empty field")
	}
}

func TestSortedSetDocValuesAccumulator_MaxCategoriesLimit(t *testing.T) {
	config := facets.NewFacetsConfig()
	acc, err := NewSortedSetDocValuesAccumulator(config, "category")
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Set a small max categories limit
	acc.SetMaxCategories(5)

	// Add more categories than the limit
	for i := 0; i < 10; i++ {
		ord := acc.getOrCreateOrdinal(fmt.Sprintf("category/item%d", i))
		if i < 5 {
			if ord == 0 {
				t.Errorf("expected valid ordinal for item %d", i)
			}
		} else {
			if ord != 0 {
				t.Errorf("expected invalid ordinal (0) for item %d beyond limit", i)
			}
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
