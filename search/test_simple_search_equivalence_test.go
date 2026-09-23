// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestSimpleSearchEquivalence.java
// (Apache Lucene 10.5.0): basic equivalence tests for core queries; the class
// extends SearchEquivalenceTestBase (search_equivalence_test_base_test.go).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func sseTermQuery(t *index.Term) *search.TermQuery { return search.NewTermQuery(t) }

// testTermVersusBooleanOr: A ⊆ (A B).
func TestSimpleSearchEquivalenceTermVersusBooleanOr(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(sseTermQuery(t1), search.SHOULD)
	q2.Add(sseTermQuery(t2), search.SHOULD)
	h.assertSubsetOf(q1, q2.Build())
}

// testTermVersusBooleanReqOpt: A ⊆ (+A B).
func TestSimpleSearchEquivalenceTermVersusBooleanReqOpt(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(sseTermQuery(t1), search.MUST)
	q2.Add(sseTermQuery(t2), search.SHOULD)
	h.assertSubsetOf(q1, q2.Build())
}

// testBooleanReqExclVersusTerm: (A -B) ⊆ A.
func TestSimpleSearchEquivalenceBooleanReqExclVersusTerm(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewBooleanQueryBuilder()
	q1.Add(sseTermQuery(t1), search.MUST)
	q1.Add(sseTermQuery(t2), search.MUST_NOT)
	q2 := sseTermQuery(t1)
	h.assertSubsetOf(q1.Build(), q2)
}

// testBooleanAndVersusBooleanOr: (A B) ⊆ (A B).
func TestSimpleSearchEquivalenceBooleanAndVersusBooleanOr(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewBooleanQueryBuilder()
	q1.Add(sseTermQuery(t1), search.SHOULD)
	q1.Add(sseTermQuery(t2), search.SHOULD)
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(sseTermQuery(t1), search.SHOULD)
	q2.Add(sseTermQuery(t2), search.SHOULD)
	h.assertSubsetOf(q1.Build(), q2.Build())
}

// testDisjunctionSumVersusDisjunctionMax: (A B) = (A | B).
func TestSimpleSearchEquivalenceDisjunctionSumVersusDisjunctionMax(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewBooleanQueryBuilder()
	q1.Add(sseTermQuery(t1), search.SHOULD)
	q1.Add(sseTermQuery(t2), search.SHOULD)
	q2 := search.NewDisjunctionMaxQuery([]search.Query{sseTermQuery(t1), sseTermQuery(t2)}, 0.5)
	h.assertSameSet(q1.Build(), q2)
}

// testExactPhraseVersusBooleanAnd: "A B" ⊆ (+A +B).
func TestSimpleSearchEquivalenceExactPhraseVersusBooleanAnd(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewPhraseQueryWithTerms(0, t1.Field, t1, t2)
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(sseTermQuery(t1), search.MUST)
	q2.Add(sseTermQuery(t2), search.MUST)
	h.assertSubsetOf(q1, q2.Build())
}

// testExactPhraseVersusBooleanAndWithHoles.
func TestSimpleSearchEquivalenceExactPhraseVersusBooleanAndWithHoles(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	b := search.NewPhraseQueryBuilder()
	b.AddWithPosition(t1, 0)
	b.AddWithPosition(t2, 2)
	q1 := b.Build()
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(sseTermQuery(t1), search.MUST)
	q2.Add(sseTermQuery(t2), search.MUST)
	h.assertSubsetOf(q1, q2.Build())
}

// testPhraseVersusSloppyPhrase: "A B" ⊆ "A B"~1.
func TestSimpleSearchEquivalencePhraseVersusSloppyPhrase(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewPhraseQueryWithTerms(0, t1.Field, t1, t2)
	q2 := search.NewPhraseQueryWithTerms(1, t1.Field, t1, t2)
	h.assertSubsetOf(q1, q2)
}

// testPhraseVersusSloppyPhraseWithHoles.
func TestSimpleSearchEquivalencePhraseVersusSloppyPhraseWithHoles(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	b := search.NewPhraseQueryBuilder()
	b.AddWithPosition(t1, 0)
	b.AddWithPosition(t2, 2)
	q1 := b.Build()
	b2 := search.NewPhraseQueryBuilder()
	b2.SetSlop(2)
	b2.AddWithPosition(t1, 0)
	b2.AddWithPosition(t2, 2)
	q2 := b2.Build()
	h.assertSubsetOf(q1, q2)
}

// testExactPhraseVersusMultiPhrase: "A B" ⊆ "A (B C)".
func TestSimpleSearchEquivalenceExactPhraseVersusMultiPhrase(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	t3 := h.randomTerm()
	q1 := search.NewPhraseQueryWithTerms(0, t1.Field, t1, t2)
	q2b := search.NewMultiPhraseQueryBuilder()
	q2b.Add(t1)
	q2b.AddTerms([]*index.Term{t2, t3})
	h.assertSubsetOf(q1, q2b.Build())
}

// testExactPhraseVersusMultiPhraseWithHoles.
func TestSimpleSearchEquivalenceExactPhraseVersusMultiPhraseWithHoles(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	t3 := h.randomTerm()
	b := search.NewPhraseQueryBuilder()
	b.AddWithPosition(t1, 0)
	b.AddWithPosition(t2, 2)
	q1 := b.Build()
	q2b := search.NewMultiPhraseQueryBuilder()
	q2b.Add(t1)
	q2b.AddTermsAtPosition([]*index.Term{t2, t3}, 2)
	h.assertSubsetOf(q1, q2b.Build())
}

// testSloppyPhraseVersusBooleanAnd: "A B"~∞ = +A +B if A != B.
func TestSimpleSearchEquivalenceSloppyPhraseVersusBooleanAnd(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	q1 := search.NewPhraseQueryWithTerms(int(^uint(0)>>1), t1.Field, t1, t2)
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(sseTermQuery(t1), search.MUST)
	q2.Add(sseTermQuery(t2), search.MUST)
	h.assertSameSet(q1, q2.Build())
}

// testPhraseRelativePositions: phrase positions are relative.
func TestSimpleSearchEquivalencePhraseRelativePositions(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewPhraseQueryWithTerms(0, t1.Field, t1, t2)
	b := search.NewPhraseQueryBuilder()
	b.AddWithPosition(t1, 10000)
	b.AddWithPosition(t2, 10001)
	q2 := b.Build()
	h.assertSameScores(q1, q2)
}

// testSloppyPhraseRelativePositions.
func TestSimpleSearchEquivalenceSloppyPhraseRelativePositions(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewPhraseQueryWithTerms(2, t1.Field, t1, t2)
	b := search.NewPhraseQueryBuilder()
	b.SetSlop(2)
	b.AddWithPosition(t1, 10000)
	b.AddWithPosition(t2, 10001)
	q2 := b.Build()
	h.assertSameScores(q1, q2)
}

// testBoostQuerySimplification. The AssertingQuery wrapper keeps BoostQuery from
// merging the inner and outer boosts, so the two boost stacks must still score
// identically.
func TestSimpleSearchEquivalenceBoostQuerySimplification(t *testing.T) {
	h := newSeqHarness(t)
	b1 := random().Float32() * 10
	b2 := random().Float32() * 10
	term := h.randomTerm()

	q1 := search.NewBoostQuery(search.NewBoostQuery(sseTermQuery(term), b2), b1)
	// Use AssertingQuery to prevent BoostQuery from merging inner and outer boosts
	// Query q2 = new BoostQuery(new AssertingQuery(random(), new BoostQuery(new TermQuery(term), b2)), b1);
	t.Fatal(assertingQueryBlocker)
	var q2 search.Query
	h.assertSameScores(q1, q2)
}

// testBooleanBoostPropagation.
func TestSimpleSearchEquivalenceBooleanBoostPropagation(t *testing.T) {
	h := newSeqHarness(t)
	boost1 := random().Float32()
	tq := search.NewBoostQuery(sseTermQuery(h.randomTerm()), boost1)

	boost2 := random().Float32()
	q1 := search.NewBoostQuery(tq, boost2)
	inner := search.NewBooleanQueryBuilder()
	inner.Add(tq, search.MUST)
	inner.Add(tq, search.FILTER)
	q2 := search.NewBoostQuery(inner.Build(), boost2)
	h.assertSameScores(q1, q2)
}

// testBooleanOrVsSynonym.
func TestSimpleSearchEquivalenceBooleanOrVsSynonym(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewSynonymQueryBuilder(t1.Field).AddTerm(t1).AddTerm(t2).Build()
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(sseTermQuery(t1), search.SHOULD)
	q2.Add(sseTermQuery(t2), search.SHOULD)
	h.assertSameSet(q1, q2.Build())
}
