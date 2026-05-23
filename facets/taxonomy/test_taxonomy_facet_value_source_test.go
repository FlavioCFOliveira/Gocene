// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy_test

// TestTaxonomyFacetValueSource ports selected assertions from
// org.apache.lucene.facet.taxonomy.TestTaxonomyFacetValueSource.
//
// Full integration tests (testBasic, testWithScore, etc.) require
// IndexWriter + FacetsCollector + DocValues pipeline — deferred with t.Skip.
// Unit tests cover IntTaxonomyFacets and FloatTaxonomyFacets accumulation.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/taxonomy"
)

// stubTaxonomyReader is a minimal TaxonomyReaderI for unit tests.
type stubTaxonomyReader struct {
	size int
}

func (r *stubTaxonomyReader) GetSize() int                                    { return r.size }
func (r *stubTaxonomyReader) GetPath(_ int) []string                          { return nil }
func (r *stubTaxonomyReader) GetOrdinal(_ ...string) int                      { return -1 }
func (r *stubTaxonomyReader) GetParallelTaxonomyArrays() taxonomy.ParallelTaxonomyArrays {
	parents := make([]int, r.size)
	for i := range parents {
		parents[i] = -1
	}
	return taxonomy.NewInMemoryParallelTaxonomyArrays(parents, make([]int, r.size), make([]int, r.size))
}

// TestIntTaxonomyFacets_SetGetValue verifies SetValue/GetValue round-trip.
func TestIntTaxonomyFacets_SetGetValue(t *testing.T) {
	reader := &stubTaxonomyReader{size: 10}
	cfg := facets.NewFacetsConfig()
	itf := taxonomy.NewIntTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	itf.SetValue(3, 42)
	if got := itf.GetValue(3); got != 42 {
		t.Errorf("GetValue(3): want 42, got %d", got)
	}
	if got := itf.GetValue(5); got != 0 {
		t.Errorf("GetValue(5): want 0, got %d", got)
	}
}

// TestIntTaxonomyFacets_AccumulateIntValue verifies SUM aggregation.
func TestIntTaxonomyFacets_AccumulateIntValue(t *testing.T) {
	reader := &stubTaxonomyReader{size: 10}
	cfg := facets.NewFacetsConfig()
	itf := taxonomy.NewIntTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	itf.AccumulateIntValue(2, 10)
	itf.AccumulateIntValue(2, 5)
	if got := itf.GetValue(2); got != 15 {
		t.Errorf("accumulated SUM: want 15, got %d", got)
	}
}

// TestFloatTaxonomyFacets_SetGetValue verifies FloatTaxonomyFacets.
func TestFloatTaxonomyFacets_SetGetValue(t *testing.T) {
	reader := &stubTaxonomyReader{size: 10}
	cfg := facets.NewFacetsConfig()
	ftf := taxonomy.NewFloatTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	ftf.SetValue(1, 3.14)
	got := ftf.GetValue(1)
	if got < 3.13 || got > 3.15 {
		t.Errorf("GetValue(1): want ~3.14, got %v", got)
	}
	if ftf.GetValue(0) != 0.0 {
		t.Errorf("GetValue(0): want 0.0, got %v", ftf.GetValue(0))
	}
}

// TestFloatTaxonomyFacets_AccumulateFloat verifies float SUM accumulation.
func TestFloatTaxonomyFacets_AccumulateFloat(t *testing.T) {
	reader := &stubTaxonomyReader{size: 10}
	cfg := facets.NewFacetsConfig()
	ftf := taxonomy.NewFloatTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	ftf.AccumulateFloatValue(4, 1.5)
	ftf.AccumulateFloatValue(4, 2.5)
	got := ftf.GetValue(4)
	if got < 3.99 || got > 4.01 {
		t.Errorf("accumulated float SUM: want ~4.0, got %v", got)
	}
}

// -- Integration stubs -------------------------------------------------------

func TestTaxonomyFacetValueSource_Basic(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + DocValues + TaxonomyFacets pipeline")
}

func TestTaxonomyFacetValueSource_WithScore(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + DocValues + TaxonomyFacets pipeline")
}
