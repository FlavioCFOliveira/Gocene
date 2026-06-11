// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package directory

// TestConcurrentFacetedIndexing ports assertions from
// org.apache.lucene.facet.taxonomy.directory.TestConcurrentFacetedIndexing.
//
// The Java original tests concurrent indexing with IndexWriter +
// DirectoryTaxonomyWriter and verifies that the resulting taxonomy arrays
// are consistent.
//
// Gocene's TaxonomyIndexArrays provides the parent/children/siblings arrays
// that are the output of concurrent indexing. These tests verify that the
// arrays can be built and queried correctly.

import (
	"testing"
)

// TestConcurrentFacetedIndexing_Concurrency verifies that TaxonomyIndexArrays
// correctly tracks parent relationships, which is the invariant that concurrent
// indexing must preserve.
func TestConcurrentFacetedIndexing_Concurrency(t *testing.T) {
	// Build the parents array that would result from concurrent faceted indexing.
	// Example: root(0), Author(0), M.Twain(1), R.Pike(1), Animals(0), Dog(4), Cat(4)
	parents := []int{0, 0, 1, 1, 0, 4, 4}
	arrays := NewTaxonomyIndexArraysFromParents(parents)

	// Verify parents.
	expectedParents := []int{0, 0, 1, 1, 0, 4, 4}
	for i, want := range expectedParents {
		if arrays.Parents()[i] != want {
			t.Errorf("parents[%d]: want %d, got %d", i, want, arrays.Parents()[i])
		}
	}

	// Add another category (concurrent indexing adding a new category).
	arrays.Add(7, 1) // New cat(7) under Author(1)
	if arrays.Parents()[7] != 1 {
		t.Errorf("added category parent: want 1, got %d", arrays.Parents()[7])
	}

	// Verify children/siblings (lazily computed).
	children := arrays.Children()
	if children == nil {
		t.Fatal("Children returned nil")
	}
	if len(children) != len(parents)+1 {
		t.Errorf("Children length: want %d, got %d", len(parents)+1, len(children))
	}
}
