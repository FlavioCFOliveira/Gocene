// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestNot.java
//
// Deviation: all test methods skipped — TestNot tests boolean NOT (MUST_NOT)
// queries against an IndexWriter+IndexSearcher pipeline not yet complete in Gocene.

package search

import "testing"

// TestNot_TestNot mirrors testNot.
// It indexes a document containing "all" and "one" and verifies that a boolean
// NOT query on "one" does NOT match the document.
func TestNot_TestNot(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
