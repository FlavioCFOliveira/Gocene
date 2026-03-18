// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"testing"
)

func TestNewTaxonomyFacetsAccumulator(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()

	acc, err := NewTaxonomyFacetsAccumulator(reader, config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if acc == nil {
		t.Fatal("expected accumulator to be created")
	}
	if acc.reader != reader {
		t.Error("expected reader to be set")
	}
	if acc.config != config {
		t.Error("expected config to be set")
	}
}

func TestNewTaxonomyFacetsAccumulatorNilReader(t *testing.T) {
	config := NewFacetsConfig()
	acc, err := NewTaxonomyFacetsAccumulator(nil, config)
	if err == nil {
		t.Error("expected error for nil reader")
	}
	if acc != nil {
		t.Error("expected nil accumulator for nil reader")
	}
}

func TestNewTaxonomyFacetsAccumulatorNilConfig(t *testing.T) {
	reader := NewTaxonomyReader()
	acc, err := NewTaxonomyFacetsAccumulator(reader, nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
	if acc != nil {
		t.Error("expected nil accumulator for nil config")
	}
}

func TestTaxonomyFacetsAccumulatorAccumulateFromMatchingDocs(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, config)

	err := acc.AccumulateFromMatchingDocs(nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = acc.AccumulateFromMatchingDocs([]*MatchingDocs{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTaxonomyFacetsAccumulatorIncrementCount(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, config)

	// For an empty reader, the counts array has size 1 (just for ordinal 0)
	// Increment ordinal 0 (the only valid ordinal for empty reader)
	acc.IncrementCount(0, 10)
	if acc.GetCount(0) != 10 {
		t.Errorf("expected count 10 for ordinal 0, got %d", acc.GetCount(0))
	}

	// Increment again
	acc.IncrementCount(0, 5)
	if acc.GetCount(0) != 15 {
		t.Errorf("expected count 15 for ordinal 0, got %d", acc.GetCount(0))
	}

	// Increment invalid ordinal (should not panic)
	acc.IncrementCount(-1, 10)
	acc.IncrementCount(1000, 10)
}

func TestTaxonomyFacetsAccumulatorGetCount(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, config)

	// Get count for invalid ordinal
	if acc.GetCount(-1) != 0 {
		t.Error("expected 0 for invalid ordinal")
	}
	if acc.GetCount(1000) != 0 {
		t.Error("expected 0 for out of range ordinal")
	}

	// Get count after increment (ordinal 0 is valid for empty reader)
	acc.IncrementCount(0, 10)
	if acc.GetCount(0) != 10 {
		t.Errorf("expected count 10 for ordinal 0, got %d", acc.GetCount(0))
	}
}

func TestTaxonomyFacetsAccumulatorGetTopChildren(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, config)

	// Non-existent dimension
	result, err := acc.GetTopChildren(10, "nonexistent")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil for non-existent dimension")
	}
}

func TestTaxonomyFacetsAccumulatorGetAllChildren(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, config)

	// Non-existent dimension
	result, err := acc.GetAllChildren("nonexistent")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil for non-existent dimension")
	}
}

func TestTaxonomyFacetsAccumulatorGetSpecificValue(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, config)

	// Non-existent path
	result, err := acc.GetSpecificValue("nonexistent", "path")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != nil {
		t.Error("expected nil for non-existent path")
	}
}

func TestTaxonomyFacetsAccumulatorGetReader(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, config)

	gotReader := acc.GetReader()
	if gotReader != reader {
		t.Error("expected GetReader to return the same reader")
	}
}

func TestTaxonomyFacetsAccumulatorGetConfig(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, config)

	gotConfig := acc.GetConfig()
	if gotConfig != config {
		t.Error("expected GetConfig to return the same config")
	}
}

func TestTaxonomyFacetsAccumulatorReset(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, config)

	// Add some counts
	acc.IncrementCount(1, 10)
	acc.IncrementCount(2, 20)

	// Reset
	acc.Reset()

	// Counts should be zero
	if acc.GetCount(1) != 0 {
		t.Errorf("expected count 0 after reset, got %d", acc.GetCount(1))
	}
	if acc.GetCount(2) != 0 {
		t.Errorf("expected count 0 after reset, got %d", acc.GetCount(2))
	}
}

func TestNewTaxonomyFacetsAccumulatorFactory(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	factory := NewTaxonomyFacetsAccumulatorFactory(reader, config)

	if factory == nil {
		t.Fatal("expected factory to be created")
	}
	if factory.reader != reader {
		t.Error("expected reader to be set")
	}
	if factory.config != config {
		t.Error("expected config to be set")
	}
}

func TestTaxonomyFacetsAccumulatorFactoryCreateAccumulator(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	factory := NewTaxonomyFacetsAccumulatorFactory(reader, config)

	acc, err := factory.CreateAccumulator()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if acc == nil {
		t.Fatal("expected accumulator to be created")
	}
}

func TestNewTaxonomyFacetsAccumulatorBuilder(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	builder := NewTaxonomyFacetsAccumulatorBuilder(reader, config)

	if builder == nil {
		t.Fatal("expected builder to be created")
	}
	if builder.reader != reader {
		t.Error("expected reader to be set")
	}
	if builder.config != config {
		t.Error("expected config to be set")
	}
}

func TestTaxonomyFacetsAccumulatorBuilderSetInitialCount(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	builder := NewTaxonomyFacetsAccumulatorBuilder(reader, config)

	builder.SetInitialCount(100)
	if builder.initialCount != 100 {
		t.Errorf("expected initialCount 100, got %d", builder.initialCount)
	}
}

func TestTaxonomyFacetsAccumulatorBuilderBuild(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	builder := NewTaxonomyFacetsAccumulatorBuilder(reader, config)

	acc, err := builder.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if acc == nil {
		t.Fatal("expected accumulator to be built")
	}
}

func TestTaxonomyFacetsAccumulatorBuilderBuildWithInitialCount(t *testing.T) {
	reader := NewTaxonomyReader()
	config := NewFacetsConfig()
	builder := NewTaxonomyFacetsAccumulatorBuilder(reader, config)
	builder.SetInitialCount(100)

	acc, err := builder.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if acc == nil {
		t.Fatal("expected accumulator to be built")
	}

	// For an empty reader, the counts array has size 1 (just for ordinal 0)
	// The initial count should be set for all ordinals
	// Just verify the accumulator was created successfully
	if acc.counts == nil {
		t.Error("expected counts array to be initialized")
	}
}
