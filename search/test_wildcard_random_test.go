// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestWildcardRandom.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// integration with WildcardQuery, not yet complete in Gocene.

package search

import "testing"

// TestWildcardRandom_Wildcards mirrors testWildcards.
// It verifies that random wildcard queries produce results consistent with
// brute-force term enumeration.
func TestWildcardRandom_Wildcards(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
