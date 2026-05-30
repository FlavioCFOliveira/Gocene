// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// TestSegmentToThreadMapping is the Go port of Lucene's
// org.apache.lucene.index.TestSegmentToThreadMapping.
//
// The original test exercises IndexSearcher.slices(...), which maps index
// segments (LeafReaderContexts) onto search slices/partitions. Gocene's
// search.IndexSearcher does not yet expose Slices, LeafSlice, or
// LeafReaderContextPartition (search/index_searcher.go has no slicing API),
// so a full roundtrip cannot compile. Per Sprint 55 option c, the test is
// recorded as a skip-stub keyed to the missing production type and should be
// fleshed out once IndexSearcher gains a slicing implementation.
func TestSegmentToThreadMapping(t *testing.T) {
	t.Fatal("blocked: search.IndexSearcher exposes no Slices/LeafSlice/LeafReaderContextPartition API (search/index_searcher.go); segment-to-slice mapping not yet ported")
}
