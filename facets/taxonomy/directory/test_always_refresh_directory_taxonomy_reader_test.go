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
// All tests require:
//   - DirectoryTaxonomyWriter + commit + snapshot copy
//   - SearcherTaxonomyManager with maybeRefresh and backward-time rollback
//   - AlwaysRefreshDirectoryTaxonomyReader (overrides doOpenIfChanged to always reconstruct)
//
// None of these are implemented in the Gocene directory package yet.
// All tests are deferred with t.Skip until the full pipeline is available.

import "testing"

// TestAlwaysRefreshDirectoryTaxonomyReader_AlwaysRefresh verifies that
// AlwaysRefreshDirectoryTaxonomyReader can refresh after a taxonomy rollback to
// an older checkpoint, while plain DirectoryTaxonomyReader panics on stale arrays.
func TestAlwaysRefreshDirectoryTaxonomyReader_AlwaysRefresh(t *testing.T) {
	t.Fatal("requires AlwaysRefreshDirectoryTaxonomyReader + SearcherTaxonomyManager + DirectoryTaxonomyWriter pipeline")
}

// TestAlwaysRefreshDirectoryTaxonomyReader_PlainReaderFails verifies that a plain
// DirectoryTaxonomyReader raises an error when refreshed after backward rollback.
func TestAlwaysRefreshDirectoryTaxonomyReader_PlainReaderFails(t *testing.T) {
	t.Fatal("requires DirectoryTaxonomyWriter snapshot/rollback + SearcherTaxonomyManager.maybeRefresh pipeline")
}
