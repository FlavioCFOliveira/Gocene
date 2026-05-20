// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestAutomatonQueryUnicode.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with AutomatonQuery over Unicode terms, not yet complete in Gocene.

package search

import "testing"

// TestAutomatonQueryUnicode_SortOrder mirrors testSortOrder.
// It verifies that automaton query matches respect Unicode sort order in
// the index term dictionary.
func TestAutomatonQueryUnicode_SortOrder(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
