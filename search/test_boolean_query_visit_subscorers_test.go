// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestBooleanQueryVisitSubscorers.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with scorer visitor traversal, not yet complete in Gocene.

package search

import "testing"

func TestBooleanQueryVisitSubscorers_Disjunctions(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestBooleanQueryVisitSubscorers_NestedDisjunctions(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestBooleanQueryVisitSubscorers_Conjunctions(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestBooleanQueryVisitSubscorers_DisjunctionMatches(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestBooleanQueryVisitSubscorers_MinShouldMatchMatches(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestBooleanQueryVisitSubscorers_GetChildrenMinShouldMatchSumScorer(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestBooleanQueryVisitSubscorers_GetChildrenBoosterScorer(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
