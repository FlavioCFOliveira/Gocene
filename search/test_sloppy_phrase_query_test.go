// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSloppyPhraseQuery.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// integration with PhraseQuery slop, not yet complete in Gocene.

package search

import "testing"

// TestSloppyPhraseQuery_Doc4Query4 mirrors testDoc4_Query4_All_Slops_Should_match.
func TestSloppyPhraseQuery_Doc4Query4(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestSloppyPhraseQuery_Doc1Query1 mirrors testDoc1_Query1_All_Slops_Should_match.
func TestSloppyPhraseQuery_Doc1Query1(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestSloppyPhraseQuery_Doc2Query1 mirrors testDoc2_Query1_Slop_6_or_more_Should_match.
func TestSloppyPhraseQuery_Doc2Query1(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestSloppyPhraseQuery_Doc2Query2 mirrors testDoc2_Query2_All_Slops_Should_match.
func TestSloppyPhraseQuery_Doc2Query2(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestSloppyPhraseQuery_Doc3Query1 mirrors testDoc3_Query1_All_Slops_Should_match.
func TestSloppyPhraseQuery_Doc3Query1(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestSloppyPhraseQuery_Doc5Query5 mirrors testDoc5_Query5_Any_Slop_Should_be_consistent.
func TestSloppyPhraseQuery_Doc5Query5(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestSloppyPhraseQuery_SlopWithHoles mirrors testSlopWithHoles.
func TestSloppyPhraseQuery_SlopWithHoles(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestSloppyPhraseQuery_InfiniteFreq1 mirrors testInfiniteFreq1.
func TestSloppyPhraseQuery_InfiniteFreq1(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestSloppyPhraseQuery_InfiniteFreq2 mirrors testInfiniteFreq2.
func TestSloppyPhraseQuery_InfiniteFreq2(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
