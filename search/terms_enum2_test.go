// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: terms_enum2_test.go
// Source: lucene/core/src/test/org/apache/lucene/index/TestTermsEnum2.java
// Purpose: Randomized tests that compare TermsEnum seeking, nexting and
// intersection against automata-derived expectations over a Unicode index.
// Task: GOC-4148
//
// Port status: STRUCTURED / SKIPPED.
//
// Every @Test method in the Java reference has a 1:1 Go counterpart below,
// but each is gated with t.Skip because the supporting test infrastructure
// does not yet exist in Gocene. The skip reasons are intentionally specific
// so the bodies can be unskipped incrementally as the dependencies land:
//
//   - RandomIndexWriter: Gocene has no RandomIndexWriter equivalent; sibling
//     index tests substitute a plain index.IndexWriter and flag it as a
//     deviation. The shared random-doc fixture (setUp) needs it to vary
//     maxBufferedDocs and flush behavior.
//   - MockAnalyzer / MockTokenizer.KEYWORD: no keyword MockAnalyzer is wired
//     for search_test, so the "field" StringField cannot be tokenized as the
//     Java fixture expects.
//   - AutomatonTestUtil.randomRegexp / sameLanguage: there is no real
//     AutomatonTestUtil in Gocene. search/automaton_query_test.go only defines
//     a local stub struct with Minus/RandomAutomaton; randomRegexp and
//     sameLanguage are absent.
//   - CheckHits.checkEqual: no CheckHits port exists for search_test.
//   - index.MultiTerms.Iterator / Intersect: MultiTerms.Iterator currently
//     returns ErrMultiTermsEnumNotImplemented, so seeking, nexting and
//     intersect cannot be exercised end to end.
//
// When the dependencies above are available, replace each t.Skip with the
// ported body sketched in the accompanying comments.

package search_test

import (
	"testing"
)

// setUpTermsEnum2Fixture mirrors TestTermsEnum2.setUp.
//
// Java reference builds:
//   - numIterations = atLeast(50)
//   - a RandomIndexWriter over newDirectory() with a KEYWORD MockAnalyzer and
//     a randomized maxBufferedDocs in [50, 1000]
//   - atLeast(200) documents, each a single Store.YES StringField "field" set
//     to TestUtil.randomUnicodeString(random())
//   - terms: a sorted set (TreeSet) of the indexed BytesRef terms
//   - termsAutomaton = Automata.makeStringUnion(terms)
//   - reader/searcher derived from the writer
//
// Not yet portable: see the package-level skip reasons (RandomIndexWriter,
// MockAnalyzer, searcher construction over a multi-segment reader).
func setUpTermsEnum2Fixture(t *testing.T) {
	t.Helper()
	t.Fatal("TestTermsEnum2 fixture needs RandomIndexWriter + KEYWORD MockAnalyzer; see file header")
}

// TestTermsEnum2_FiniteVersusInfinite ports testFiniteVersusInfinite:
// tests a pre-intersected automaton against the original.
//
// For each iteration: build a random regexp automaton, collect the indexed
// terms it accepts, build an alternate automaton as the string union of those
// matched terms, run AutomatonQuery for both automata and assert the score
// docs are equal via CheckHits.checkEqual.
//
// Blocked by: AutomatonTestUtil.randomRegexp, CheckHits.checkEqual, and the
// shared fixture (RandomIndexWriter / searcher).
func TestTermsEnum2_FiniteVersusInfinite(t *testing.T) {
	t.Skip("TestTermsEnum2 fixture needs RandomIndexWriter + KEYWORD MockAnalyzer — not yet ported")
}
}

// TestTermsEnum2_Seeking ports testSeeking:
// seeks to every term accepted by some automaton.
//
// For each iteration: build a random regexp automaton, shuffle the indexed
// terms, and for every term the automaton accepts either seekExact (expect
// true) or seekCeil (expect SeekStatus.FOUND and matching term()), chosen at
// random.
//
// Blocked by: AutomatonTestUtil.randomRegexp and index.MultiTerms.Iterator
// (currently returns ErrMultiTermsEnumNotImplemented).
func TestTermsEnum2_Seeking(t *testing.T) {
	setUpTermsEnum2Fixture(t)
	t.Fatal("blocked: AutomatonTestUtil.randomRegexp and MultiTerms.Iterator not available")
}

// TestTermsEnum2_SeekingAndNexting ports testSeekingAndNexting:
// mixes up seek and next for all terms.
//
// For each iteration: iterate the sorted terms in order; for each term pick
// one of three actions at random — next() (expect the term), seekCeil()
// (expect SeekStatus.FOUND and matching term()), or seekExact() (expect true).
//
// Blocked by: index.MultiTerms.Iterator (currently returns
// ErrMultiTermsEnumNotImplemented) and the shared fixture.
func TestTermsEnum2_SeekingAndNexting(t *testing.T) {
	setUpTermsEnum2Fixture(t)
	t.Fatal("blocked: MultiTerms.Iterator not available")
}

// TestTermsEnum2_Intersect ports testIntersect.
//
// For each iteration: build a random regexp automaton, wrap it in a
// CompiledAutomaton, intersect the field's MultiTerms against it, collect the
// produced terms, and assert (via AutomatonTestUtil.sameLanguage) that their
// string-union automaton accepts the same language as the determinized
// intersection of termsAutomaton with the regexp automaton.
//
// Blocked by: AutomatonTestUtil.randomRegexp, AutomatonTestUtil.sameLanguage,
// and index.MultiTerms.Intersect / Iterator.
func TestTermsEnum2_Intersect(t *testing.T) {
	setUpTermsEnum2Fixture(t)
	t.Fatal("blocked: AutomatonTestUtil.{randomRegexp,sameLanguage} and MultiTerms.Intersect not available")
}
