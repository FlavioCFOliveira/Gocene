// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestTopKnnResults.java
//
// Deviation: ported as a fully executable Type C test (pure data structure —
// no index needed). Gocene's API differs from Java only in capitalisation
// (TopDocs() vs topDocs()).

package search

import (
	"testing"
)

// TestTopKnnResults_CollectAndProvideResults mirrors testCollectAndProvideResults.
// It verifies that TopKnnCollector keeps the top-K results and returns them
// sorted by score descending.
func TestTopKnnResults_CollectAndProvideResults(t *testing.T) {
	results := NewTopKnnCollector(5, 0) // visitLimit=0 → MaxInt32

	nodes := []int{4, 1, 5, 7, 8, 10, 2}
	scores := []float32{1, 0.5, 0.6, 2, 2, 1.2, 4}

	for i, node := range nodes {
		results.Collect(node, scores[i])
	}

	topDocs := results.TopDocs()
	if len(topDocs.ScoreDocs) != 5 {
		t.Fatalf("expected 5 results, got %d", len(topDocs.ScoreDocs))
	}

	wantNodes := []int{2, 7, 8, 10, 4}
	wantScores := []float32{4, 2, 2, 1.2, 1}

	for i, sd := range topDocs.ScoreDocs {
		if sd.Doc != wantNodes[i] {
			t.Errorf("result[%d]: got doc %d, want %d", i, sd.Doc, wantNodes[i])
		}
		if sd.Score != wantScores[i] {
			t.Errorf("result[%d]: got score %f, want %f", i, sd.Score, wantScores[i])
		}
	}
}
