// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy_test

// TestTaxonomyFacetCounts ports selected assertions from
// org.apache.lucene.facet.taxonomy.TestTaxonomyFacetCounts.
//
// Integration tests (testBasic, testMultiValuedHierarchy, testRandom, etc.)
// require IndexWriter + FacetsCollector + DirectoryTaxonomyReader pipeline
// and are deferred with t.Skip.
//
// Unit tests cover TaxonomyFacets rollup logic and count increments using
// InMemoryParallelTaxonomyArrays.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/taxonomy"
)

// stubReaderFor creates a stubTaxonomyReader with the given size.
// (stubTaxonomyReader is defined in test_taxonomy_facet_value_source_test.go)
func stubReaderFor(size int) *stubTaxonomyReader {
	return &stubTaxonomyReader{size: size}
}

// TestTaxonomyFacets_ParallelArrays verifies that InMemoryParallelTaxonomyArrays
// correctly stores and returns the three parallel arrays used during rollup.
func TestTaxonomyFacets_ParallelArrays(t *testing.T) {
	parents := []int{-1, 0, 1, 1, 1}
	children := []int{1, 2, -1, -1, -1}
	siblings := []int{-1, -1, 3, 4, -1}
	arrays := taxonomy.NewInMemoryParallelTaxonomyArrays(parents, children, siblings)

	if len(arrays.Parents()) != 5 {
		t.Errorf("Parents len: want 5, got %d", len(arrays.Parents()))
	}
	if arrays.Parents()[0] != -1 {
		t.Errorf("root parent: want -1, got %d", arrays.Parents()[0])
	}
	if arrays.Children()[0] != 1 {
		t.Errorf("root first-child: want 1, got %d", arrays.Children()[0])
	}
	if arrays.Siblings()[2] != 3 {
		t.Errorf("ord2 sibling: want 3, got %d", arrays.Siblings()[2])
	}
}

// TestTaxonomyFacets_IncrCount exercises the base count increment method.
func TestTaxonomyFacets_IncrCount(t *testing.T) {
	reader := stubReaderFor(10)
	cfg := facets.NewFacetsConfig()
	itf := taxonomy.NewIntTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	itf.AccumulateIntValue(3, 5)
	itf.AccumulateIntValue(3, 3)
	if got := itf.GetValue(3); got != 8 {
		t.Errorf("accumulated: want 8, got %d", got)
	}
}

// parallelReader implements TaxonomyReaderI with configurable arrays.
type parallelReader struct {
	size   int
	arrays taxonomy.ParallelTaxonomyArrays
}

func (r *parallelReader) GetSize() int                                    { return r.size }
func (r *parallelReader) GetPath(_ int) []string                          { return nil }
func (r *parallelReader) GetOrdinal(_ ...string) int                      { return -1 }
func (r *parallelReader) GetParallelTaxonomyArrays() taxonomy.ParallelTaxonomyArrays {
	return r.arrays
}

// -- Integration stubs -------------------------------------------------------

func TestTaxonomyFacetCounts_Basic(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + DirectoryTaxonomyWriter/Reader pipeline")
}

func TestTaxonomyFacetCounts_MultiValuedHierarchy(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + DirectoryTaxonomyWriter/Reader pipeline")
}

func TestTaxonomyFacetCounts_Random(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + DirectoryTaxonomyWriter/Reader pipeline")
}

func TestTaxonomyFacetCounts_DrillDown(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + DirectoryTaxonomyWriter/Reader pipeline")
}
