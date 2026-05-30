// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestTopDocsRRF.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with Reciprocal Rank Fusion (RRF) TopDocs merging, not yet complete in Gocene.

package search

import "testing"

func TestTopDocsRRF_Basics(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestTopDocsRRF_ShardIndex(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestTopDocsRRF_InconsistentShardIndex(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestTopDocsRRF_InvalidTopN(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestTopDocsRRF_InvalidK(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
