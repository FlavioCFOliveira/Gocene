// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"testing"
)

func TestNewTaxonomyFacetLabels(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, err := NewTaxonomyFacetLabels(reader)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tfl == nil {
		t.Fatal("expected TaxonomyFacetLabels to be created")
	}
	if tfl.reader != reader {
		t.Error("expected reader to be set")
	}
}

func TestNewTaxonomyFacetLabelsNilReader(t *testing.T) {
	tfl, err := NewTaxonomyFacetLabels(nil)
	if err == nil {
		t.Error("expected error for nil reader")
	}
	if tfl != nil {
		t.Error("expected nil TaxonomyFacetLabels for nil reader")
	}
}

func TestTaxonomyFacetLabelsGetAllLabels(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Initially should be empty
	labels := tfl.GetAllLabels()
	if len(labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(labels))
	}
}

func TestTaxonomyFacetLabelsGetLabel(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Non-existent ordinal should return nil
	label := tfl.GetLabel(1)
	if label != nil {
		t.Error("expected nil for non-existent ordinal")
	}
}

func TestTaxonomyFacetLabelsGetLabels(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Empty ordinals should return empty labels
	labels := tfl.GetLabels([]int{})
	if len(labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(labels))
	}

	// Non-existent ordinals should return empty labels
	labels = tfl.GetLabels([]int{1, 2, 3})
	if len(labels) != 0 {
		t.Errorf("expected 0 labels for non-existent ordinals, got %d", len(labels))
	}
}

func TestTaxonomyFacetLabelsGetLabelsByDimension(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Initially should be empty
	labels := tfl.GetLabelsByDimension("electronics")
	if len(labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(labels))
	}
}

func TestTaxonomyFacetLabelsGetDimensions(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Initially should be empty
	dims := tfl.GetDimensions()
	if len(dims) != 0 {
		t.Errorf("expected 0 dimensions, got %d", len(dims))
	}
}

func TestTaxonomyFacetLabelsGetChildLabels(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Non-existent parent should return empty
	labels := tfl.GetChildLabels(1)
	if len(labels) != 0 {
		t.Errorf("expected 0 child labels, got %d", len(labels))
	}
}

func TestTaxonomyFacetLabelsGetSiblingLabels(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Non-existent ordinal should return empty
	labels := tfl.GetSiblingLabels(1)
	if len(labels) != 0 {
		t.Errorf("expected 0 sibling labels, got %d", len(labels))
	}
}

func TestTaxonomyFacetLabelsGetAncestorLabels(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Non-existent ordinal should return empty
	labels := tfl.GetAncestorLabels(1)
	if len(labels) != 0 {
		t.Errorf("expected 0 ancestor labels, got %d", len(labels))
	}
}

func TestTaxonomyFacetLabelsGetDescendantLabels(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Non-existent ordinal should return empty
	labels := tfl.GetDescendantLabels(1)
	if len(labels) != 0 {
		t.Errorf("expected 0 descendant labels, got %d", len(labels))
	}
}

func TestTaxonomyFacetLabelsGetLabelCount(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	// Initially should be 0
	count := tfl.GetLabelCount()
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestTaxonomyFacetLabelsGetReader(t *testing.T) {
	reader := NewTaxonomyReader()
	tfl, _ := NewTaxonomyFacetLabels(reader)

	gotReader := tfl.GetReader()
	if gotReader != reader {
		t.Error("expected GetReader to return the same reader")
	}
}

func TestNewTaxonomyFacetLabelsBuilder(t *testing.T) {
	reader := NewTaxonomyReader()
	builder := NewTaxonomyFacetLabelsBuilder(reader)

	if builder == nil {
		t.Fatal("expected builder to be created")
	}
	if builder.reader != reader {
		t.Error("expected reader to be set")
	}
	if builder.maxDepth != -1 {
		t.Errorf("expected maxDepth -1, got %d", builder.maxDepth)
	}
}

func TestTaxonomyFacetLabelsBuilderSetDimensions(t *testing.T) {
	reader := NewTaxonomyReader()
	builder := NewTaxonomyFacetLabelsBuilder(reader)

	dims := []string{"electronics", "books"}
	builder.SetDimensions(dims)

	if len(builder.dimensions) != 2 {
		t.Errorf("expected 2 dimensions, got %d", len(builder.dimensions))
	}
}

func TestTaxonomyFacetLabelsBuilderSetMaxDepth(t *testing.T) {
	reader := NewTaxonomyReader()
	builder := NewTaxonomyFacetLabelsBuilder(reader)

	builder.SetMaxDepth(3)

	if builder.maxDepth != 3 {
		t.Errorf("expected maxDepth 3, got %d", builder.maxDepth)
	}
}

func TestTaxonomyFacetLabelsBuilderBuild(t *testing.T) {
	reader := NewTaxonomyReader()
	builder := NewTaxonomyFacetLabelsBuilder(reader)

	tfl, err := builder.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tfl == nil {
		t.Fatal("expected TaxonomyFacetLabels to be built")
	}
}

func TestTaxonomyFacetLabelsBuilderGetFilteredLabels(t *testing.T) {
	reader := NewTaxonomyReader()
	builder := NewTaxonomyFacetLabelsBuilder(reader)

	// Empty reader should return empty labels
	labels := builder.GetFilteredLabels()
	if len(labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(labels))
	}
}

func TestTaxonomyFacetLabelsBuilderGetFilteredLabelsWithDimensions(t *testing.T) {
	reader := NewTaxonomyReader()
	builder := NewTaxonomyFacetLabelsBuilder(reader)
	builder.SetDimensions([]string{"electronics"})

	// Empty reader should return empty labels
	labels := builder.GetFilteredLabels()
	if len(labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(labels))
	}
}

func TestTaxonomyFacetLabelsBuilderGetFilteredLabelsWithMaxDepth(t *testing.T) {
	reader := NewTaxonomyReader()
	builder := NewTaxonomyFacetLabelsBuilder(reader)
	builder.SetMaxDepth(2)

	// Empty reader should return empty labels
	labels := builder.GetFilteredLabels()
	if len(labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(labels))
	}
}
