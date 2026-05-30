// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestBlockMaxConjunction.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with block-max WAND conjunction scoring, not yet complete in Gocene.

package search

import "testing"

// TestBlockMaxConjunction_Random mirrors testRandom.
// It verifies that block-max conjunction produces the same results as a naive
// conjunction across random queries and documents.
func TestBlockMaxConjunction_Random(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
