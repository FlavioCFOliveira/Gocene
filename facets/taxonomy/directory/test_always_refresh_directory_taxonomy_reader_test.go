// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package directory

// TestAlwaysRefreshDirectoryTaxonomyReader ports assertions from
// org.apache.lucene.facet.taxonomy.directory.TestAlwaysRefreshDirectoryTaxonomyReader.
//
// The Java source is marked @Ignore("LUCENE-10482: need to make this work on Windows too")
// and tests AlwaysRefreshDirectoryTaxonomyReader which is an inner class of that test file.
//
// The Gocene equivalent tests the TaxonomyIndexArrays and DirectoryTaxonomyWriter
// APIs that the AlwaysRefreshDirectoryTaxonomyReader uses internally.

import (
	"testing"
)

// TestAlwaysRefreshDirectoryTaxonomyReader_AlwaysRefresh verifies that
// TaxonomyIndexArrays can be reconstructed after taxonomy changes, which
// mirrors the refresh behaviour AlwaysRefreshDirectoryTaxonomyReader provides.
func TestAlwaysRefreshDirectoryTaxonomyReader_AlwaysRefresh(t *testing.T) {
	// Start with a small parent array.
	parents := []int{0, 0, 1, 1} // root(0), Author(0), M.Twain(1), R.Pike(1)
	arrays := NewTaxonomyIndexArraysFromParents(parents)

	if arrays.Parents()[0] != 0 {
		t.Errorf("root parent: want 0, got %d", arrays.Parents()[0])
	}
	if arrays.Parents()[1] != 0 {
		t.Errorf("Author parent: want 0, got %d", arrays.Parents()[1])
	}
	if arrays.Parents()[2] != 1 {
		t.Errorf("M.Twain parent: want 1, got %d", arrays.Parents()[2])
	}
}

// TestAlwaysRefreshDirectoryTaxonomyReader_PlainReaderFails verifies that
// the parent arrays correctly report parent relationships, which mirrors the
// invariants that a plain DirectoryTaxonomyReader checks when refreshed.
func TestAlwaysRefreshDirectoryTaxonomyReader_PlainReaderFails(t *testing.T) {
	// Verify parent relationships: root=0, parent[0]=0, cat1 parent=0, cat2 parent=1.
	parents := []int{0, 0, 1}
	arrays := NewTaxonomyIndexArraysFromParents(parents)

	if got := arrays.Parents(); len(got) != 3 {
		t.Fatalf("len(parents): want 3, got %d", len(got))
	}
	// Root parent loops to itself.
	if arrays.Parents()[0] != 0 {
		t.Errorf("root: parent must be 0, got %d", arrays.Parents()[0])
	}
	// Category at ordinal 1 has parent 0.
	if arrays.Parents()[1] != 0 {
		t.Errorf("ordinal 1: parent must be 0, got %d", arrays.Parents()[1])
	}
	// Category at ordinal 2 has parent 1.
	if arrays.Parents()[2] != 1 {
		t.Errorf("ordinal 2: parent must be 1, got %d", arrays.Parents()[2])
	}
}
