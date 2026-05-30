// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestNearest.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with nearest-neighbor geo search, not yet complete in Gocene.

package search

import "testing"

func TestNearest_NearestNeighborWithDeletedDocs(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+geo integration (pre-existing failure in Gocene)")
}
func TestNearest_NearestNeighborWithAllDeletedDocs(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+geo integration (pre-existing failure in Gocene)")
}
func TestNearest_TieBreakByDocID(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+geo integration (pre-existing failure in Gocene)")
}
func TestNearest_NearestNeighborWithNoDocs(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+geo integration (pre-existing failure in Gocene)")
}
func TestNearest_NearestNeighborRandom(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+geo integration (pre-existing failure in Gocene)")
}
