// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPositionIncrement.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with custom position increment in the token stream, not yet complete in Gocene.

package search

import "testing"

// TestPositionIncrement_TestCrazy mirrors testCrazy.
// It verifies that PhraseQuery and SpanQuery correctly handle gaps
// introduced by non-unit position increments.
func TestPositionIncrement_TestCrazy(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
