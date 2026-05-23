// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package directory

// TestConcurrentFacetedIndexing ports assertions from
// org.apache.lucene.facet.taxonomy.directory.TestConcurrentFacetedIndexing.
//
// All tests require:
//   - IndexWriter + DirectoryTaxonomyWriter with concurrent goroutines
//   - FacetsConfig.Build + FacetField index pipeline
//   - DirectoryTaxonomyReader + ParallelTaxonomyArrays.parents()
//   - LruTaxonomyWriterCache and NO_OP cache variants
//
// These components are not yet fully wired in Gocene.
// All tests are deferred with t.Skip until the full pipeline is available.

import "testing"

// TestConcurrentFacetedIndexing_Concurrency verifies that concurrent goroutines
// can index faceted documents (using DirectoryTaxonomyWriter) without races,
// and that the resulting taxonomy contains all expected categories.
func TestConcurrentFacetedIndexing_Concurrency(t *testing.T) {
	t.Skip("requires IndexWriter + DirectoryTaxonomyWriter concurrent indexing + ParallelTaxonomyArrays pipeline")
}
