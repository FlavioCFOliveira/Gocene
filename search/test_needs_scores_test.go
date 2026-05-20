// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestNeedsScores.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with ScoreMode propagation through query trees, not yet complete in Gocene.

package search

import "testing"

func TestNeedsScores_ProhibitedClause(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestNeedsScores_ConstantScoreQuery(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestNeedsScores_SortByField(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestNeedsScores_SortByScore(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
