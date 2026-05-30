// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSimilarityProvider.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with per-field similarity configuration, not yet complete in Gocene.

package search

import "testing"

// TestSimilarityProvider_Basics mirrors testBasics.
// It verifies that a per-field SimilarityProvider is honoured when scoring
// queries across multiple fields with different similarity models.
func TestSimilarityProvider_Basics(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
