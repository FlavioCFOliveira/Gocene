// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPointQueries.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with BKD-indexed point field queries (54 tests), not yet complete in Gocene.

package search

import "testing"

// TestPointQueries_Basics is a placeholder — TestPointQueries has 54 test
// methods that all require IndexWriter+IndexSearcher+BKD integration.
func TestPointQueries_Basics(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+BKD integration (pre-existing failure in Gocene; 54 test methods)")
}
