// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"testing"
)

func TestNewRandomSamplingFacetsAccumulator(t *testing.T) {
	tests := []struct {
		name        string
		config      *FacetsConfig
		sampleRate  float64
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid config and sample rate",
			config:     NewFacetsConfig(),
			sampleRate: 0.1,
			wantErr:    false,
		},
		{
			name:        "nil config",
			config:      nil,
			sampleRate:  0.1,
			wantErr:     true,
			errContains: "facets config cannot be nil",
		},
		{
			name:        "zero sample rate",
			config:      NewFacetsConfig(),
			sampleRate:  0,
			wantErr:     true,
			errContains: "sample rate must be between 0.0 and 1.0",
		},
		{
			name:        "sample rate too high",
			config:      NewFacetsConfig(),
			sampleRate:  1.5,
			wantErr:     true,
			errContains: "sample rate must be between 0.0 and 1.0",
		},
		{
			name:        "negative sample rate",
			config:      NewFacetsConfig(),
			sampleRate:  -0.1,
			wantErr:     true,
			errContains: "sample rate must be between 0.0 and 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, err := NewRandomSamplingFacetsAccumulator(tt.config, tt.sampleRate)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
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
			if acc.GetSampleRate() != tt.sampleRate {
				t.Errorf("expected sample rate %f, got %f", tt.sampleRate, acc.GetSampleRate())
			}
			if acc.GetConfig() != tt.config {
				t.Errorf("expected config to match")
			}
			if acc.GetMinSampleSize() != 100 {
				t.Errorf("expected min sample size 100, got %d", acc.GetMinSampleSize())
			}
			if acc.GetMaxSampleSize() != 100000 {
				t.Errorf("expected max sample size 100000, got %d", acc.GetMaxSampleSize())
			}
			if acc.GetConfidenceLevel() != 0.95 {
				t.Errorf("expected confidence level 0.95, got %f", acc.GetConfidenceLevel())
			}
		})
	}
}

func TestRandomSamplingFacetsAccumulator_Accumulate(t *testing.T) {
	config := NewFacetsConfig()
	acc, err := NewRandomSamplingFacetsAccumulator(config, 0.1)
	if err != nil {
		t.Fatalf("failed to create accumulator: %v", err)
	}

	// Create some facet results
	result1 := NewFacetResult("category")
	result1.AddLabelValue(NewLabelAndValue("category/electronics", 10))
	result1.AddLabelValue(NewLabelAndValue("category/books", 5))

	result2 := NewFacetResult("category")
	result2.AddLabelValue(NewLabelAndValue("category/electronics", 5))
	result2.AddLabelValue(NewLabelAndValue("category/clothing", 8))

	// Accumulate results
	err = acc.Accumulate([]*FacetResult{result1, result2})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify counts
	electronicsOrd := acc.getOrCreateOrdinal("category/electronics")
	booksOrd := acc.getOrCreateOrdinal("category/books")
	clothingOrd := acc.getOrCreateOrdinal("category/clothing")

	if acc.GetCount(electronicsOrd) != 15 {
		t.Errorf("expected electronics count 15, got %d", acc.GetCount(electronicsOrd))
	}
	if acc.GetCount(booksOrd) != 5 {
		t.Errorf("expected books count 5, got %d", acc.GetCount(booksOrd))
	}
	if acc.GetCount(clothingOrd) != 8 {
		t.Errorf("expected clothing count 8, got %d", acc.GetCount(clothingOrd))
	}
}

func TestRandomSamplingFacetsAccumulator_CalculateSampleSize(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	tests := []struct {
		totalDocs   int
		expectedMin int
		expectedMax int
	}{
		{1000, 100, 100},        // 10% of 1000 = 100
		{50, 100, 100},          // Below min, should use min
		{10000000, 100, 100000}, // Above max, should use max
	}

	for _, tt := range tests {
		sampleSize := acc.calculateSampleSize(tt.totalDocs)
		if sampleSize < tt.expectedMin || sampleSize > tt.expectedMax {
			t.Errorf("for totalDocs=%d, expected sample size between %d and %d, got %d",
				tt.totalDocs, tt.expectedMin, tt.expectedMax, sampleSize)
		}
	}
}

func TestRandomSamplingFacetsAccumulator_ReservoirSample(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Create test data
	var docs []*MatchingDocs
	for i := 0; i < 100; i++ {
		docs = append(docs, &MatchingDocs{TotalHits: 1})
	}

	// Test sampling
	sample := acc.reservoirSample(docs, 10)
	if len(sample) != 10 {
		t.Errorf("expected sample size 10, got %d", len(sample))
	}

	// Test with sample size larger than population
	sample = acc.reservoirSample(docs, 200)
	if len(sample) != 100 {
		t.Errorf("expected sample size 100 (full population), got %d", len(sample))
	}

	// Test with zero sample size
	sample = acc.reservoirSample(docs, 0)
	if sample != nil {
		t.Error("expected nil for zero sample size")
	}
}

func TestRandomSamplingFacetsAccumulator_GetEstimatedCount(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Set up test data
	acc.totalDocs = 1000
	acc.totalSampledDocs = 100

	ord := acc.getOrCreateOrdinal("category/electronics")
	acc.incrementCount(ord, 10) // 10 out of 100 sampled

	// Estimated count should be 10 * (1000/100) = 100
	estimated := acc.GetEstimatedCount(ord)
	if estimated != 100 {
		t.Errorf("expected estimated count 100, got %d", estimated)
	}

	// Test with zero sampled docs
	acc.totalSampledDocs = 0
	estimated = acc.GetEstimatedCount(ord)
	if estimated != 0 {
		t.Errorf("expected estimated count 0 when no docs sampled, got %d", estimated)
	}
}

func TestRandomSamplingFacetsAccumulator_GetConfidenceInterval(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Set up test data
	acc.totalDocs = 1000
	acc.totalSampledDocs = 100

	ord := acc.getOrCreateOrdinal("category/electronics")
	acc.incrementCount(ord, 50) // 50 out of 100 sampled

	lower, upper := acc.GetConfidenceInterval(ord)

	if lower >= upper {
		t.Errorf("expected lower < upper, got lower=%d, upper=%d", lower, upper)
	}

	if lower < 0 {
		t.Errorf("expected lower >= 0, got %d", lower)
	}

	if upper > 1000 {
		t.Errorf("expected upper <= 1000, got %d", upper)
	}
}

func TestRandomSamplingFacetsAccumulator_GetTopChildren(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Set up test data
	acc.totalDocs = 1000
	acc.totalSampledDocs = 100

	// Add some counts
	acc.incrementCount(acc.getOrCreateOrdinal("category/electronics"), 50)
	acc.incrementCount(acc.getOrCreateOrdinal("category/books"), 30)
	acc.incrementCount(acc.getOrCreateOrdinal("category/clothing"), 20)

	result, err := acc.GetTopChildren(2, "category")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if result.ChildCount != 2 {
		t.Errorf("expected 2 children, got %d", result.ChildCount)
	}

	// Test invalid topN
	_, err = acc.GetTopChildren(0, "category")
	if err == nil {
		t.Error("expected error for invalid topN")
	}
}

func TestRandomSamplingFacetsAccumulator_GetSpecificValue(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	acc.totalDocs = 1000
	acc.totalSampledDocs = 100

	acc.incrementCount(acc.getOrCreateOrdinal("category/electronics"), 42)

	result, err := acc.GetSpecificValue("category", "electronics")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// Estimated count should be 42 * (1000/100) = 420
	if result.Value != 420 {
		t.Errorf("expected value 420, got %d", result.Value)
	}
}

func TestRandomSamplingFacetsAccumulator_GetDimensions(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

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

func TestRandomSamplingFacetsAccumulator_Reset(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Add some counts
	acc.incrementCount(acc.getOrCreateOrdinal("category/electronics"), 100)
	acc.totalSampledDocs = 100
	acc.totalDocs = 1000

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

	if acc.GetTotalSampledDocs() != 0 {
		t.Errorf("expected total sampled docs to be 0 after reset")
	}

	if acc.GetTotalDocs() != 0 {
		t.Errorf("expected total docs to be 0 after reset")
	}
}

func TestRandomSamplingFacetsAccumulator_IsEmpty(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Initially should be empty
	if !acc.IsEmpty() {
		t.Errorf("expected accumulator to be empty initially")
	}

	// Add a count
	acc.incrementCount(acc.getOrCreateOrdinal("category/electronics"), 1)

	// Should not be empty
	if acc.IsEmpty() {
		t.Errorf("expected accumulator to not be empty after adding count")
	}
}

func TestRandomSamplingFacetsAccumulator_SetMinSampleSize(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Default should be 100
	if acc.GetMinSampleSize() != 100 {
		t.Errorf("expected min sample size 100 by default")
	}

	// Set to 50
	acc.SetMinSampleSize(50)
	if acc.GetMinSampleSize() != 50 {
		t.Errorf("expected min sample size 50")
	}

	// Set to 0 (should not change)
	acc.SetMinSampleSize(0)
	if acc.GetMinSampleSize() != 50 {
		t.Errorf("expected min sample size to remain 50")
	}
}

func TestRandomSamplingFacetsAccumulator_SetMaxSampleSize(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Default should be 100000
	if acc.GetMaxSampleSize() != 100000 {
		t.Errorf("expected max sample size 100000 by default")
	}

	// Set to 50000
	acc.SetMaxSampleSize(50000)
	if acc.GetMaxSampleSize() != 50000 {
		t.Errorf("expected max sample size 50000")
	}

	// Set to 0 (should not change)
	acc.SetMaxSampleSize(0)
	if acc.GetMaxSampleSize() != 50000 {
		t.Errorf("expected max sample size to remain 50000")
	}
}

func TestRandomSamplingFacetsAccumulator_SetConfidenceLevel(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Default should be 0.95
	if acc.GetConfidenceLevel() != 0.95 {
		t.Errorf("expected confidence level 0.95 by default")
	}

	// Set to 0.99
	acc.SetConfidenceLevel(0.99)
	if acc.GetConfidenceLevel() != 0.99 {
		t.Errorf("expected confidence level 0.99")
	}

	// Set invalid value (should not change)
	acc.SetConfidenceLevel(1.5)
	if acc.GetConfidenceLevel() != 0.99 {
		t.Errorf("expected confidence level to remain 0.99")
	}

	acc.SetConfidenceLevel(-0.1)
	if acc.GetConfidenceLevel() != 0.99 {
		t.Errorf("expected confidence level to remain 0.99")
	}
}

func TestRandomSamplingFacetsAccumulator_SetSeed(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Set seed
	acc.SetSeed(12345)
	if acc.GetSeed() != 12345 {
		t.Errorf("expected seed 12345, got %d", acc.GetSeed())
	}
}

func TestNewRandomSamplingFacetsAccumulatorBuilder(t *testing.T) {
	config := NewFacetsConfig()

	acc, err := NewRandomSamplingFacetsAccumulatorBuilder(config, 0.1).
		SetMinSampleSize(50).
		SetMaxSampleSize(50000).
		SetConfidenceLevel(0.99).
		SetSeed(12345).
		Build()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if acc.GetMinSampleSize() != 50 {
		t.Errorf("expected min sample size 50, got %d", acc.GetMinSampleSize())
	}

	if acc.GetMaxSampleSize() != 50000 {
		t.Errorf("expected max sample size 50000, got %d", acc.GetMaxSampleSize())
	}

	if acc.GetConfidenceLevel() != 0.99 {
		t.Errorf("expected confidence level 0.99, got %f", acc.GetConfidenceLevel())
	}

	if acc.GetSeed() != 12345 {
		t.Errorf("expected seed 12345, got %d", acc.GetSeed())
	}
}

func TestRandomSamplingFacetsAccumulatorBuilder_BuildError(t *testing.T) {
	// Test with nil config
	_, err := NewRandomSamplingFacetsAccumulatorBuilder(nil, 0.1).Build()
	if err == nil {
		t.Error("expected error with nil config")
	}

	// Test with invalid sample rate
	_, err = NewRandomSamplingFacetsAccumulatorBuilder(NewFacetsConfig(), 0).Build()
	if err == nil {
		t.Error("expected error with zero sample rate")
	}
}

func TestRandomSamplingFacetsAccumulator_AccumulateFromMatchingDocs(t *testing.T) {
	config := NewFacetsConfig()
	acc, _ := NewRandomSamplingFacetsAccumulator(config, 0.1)

	// Test with empty matching docs
	err := acc.AccumulateFromMatchingDocs([]*MatchingDocs{})
	if err != nil {
		t.Errorf("unexpected error with empty matching docs: %v", err)
	}

	// Test with nil matching docs
	err = acc.AccumulateFromMatchingDocs(nil)
	if err != nil {
		t.Errorf("unexpected error with nil matching docs: %v", err)
	}
}
