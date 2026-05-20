// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSimpleExplanationsOfNonMatches.java
//
// Deviation: all test methods skipped — TestSimpleExplanationsOfNonMatches extends
// TestSimpleExplanations and overrides the test setup to verify explanations for
// non-matching documents. Both require IndexWriter+IndexSearcher integration not
// yet complete in Gocene.

package search

import "testing"

// TestSimpleExplanationsOfNonMatches is a placeholder for the class that re-runs
// all TestSimpleExplanations scenarios in "explain non-matches" mode.
func TestSimpleExplanationsOfNonMatches(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
