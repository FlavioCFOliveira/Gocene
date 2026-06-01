// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSimpleSearchEquivalence.java
//
// Basic equivalence tests for core queries, run against the shared random a-z
// corpus from the search-equivalence harness (search_equivalence_test_base_test.go).
// Each test asserts a subset or same-set/same-scores relationship between two
// logically related query shapes, e.g. A ⊆ (A B), "A B" ⊆ (+A +B),
// (A B) = (A | B), exactly as the Lucene assertions do.

package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func sseTermQuery(t *index.Term) *search.TermQuery { return search.NewTermQuery(t) }

// TestSimpleSearchEquivalence_TermVersusBooleanOr ports testTermVersusBooleanOr: A ⊆ (A B).
func TestSimpleSearchEquivalence_TermVersusBooleanOr(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := search.NewBooleanQuery()
	q2.Add(sseTermQuery(t1), search.SHOULD)
	q2.Add(sseTermQuery(t2), search.SHOULD)
	h.seqAssertSubsetOf(q1, q2)
}

// TestSimpleSearchEquivalence_TermVersusBooleanReqOpt ports testTermVersusBooleanReqOpt: A ⊆ (+A B).
func TestSimpleSearchEquivalence_TermVersusBooleanReqOpt(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := search.NewBooleanQuery()
	q2.Add(sseTermQuery(t1), search.MUST)
	q2.Add(sseTermQuery(t2), search.SHOULD)
	h.seqAssertSubsetOf(q1, q2)
}

// TestSimpleSearchEquivalence_BooleanReqExclVersusTerm ports testBooleanReqExclVersusTerm: (A -B) ⊆ A.
func TestSimpleSearchEquivalence_BooleanReqExclVersusTerm(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewBooleanQuery()
	q1.Add(sseTermQuery(t1), search.MUST)
	q1.Add(sseTermQuery(t2), search.MUST_NOT)
	q2 := sseTermQuery(t1)
	h.seqAssertSubsetOf(q1, q2)
}

// TestSimpleSearchEquivalence_BooleanAndVersusBooleanOr ports testBooleanAndVersusBooleanOr: (A B) ⊆ (A B).
func TestSimpleSearchEquivalence_BooleanAndVersusBooleanOr(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewBooleanQuery()
	q1.Add(sseTermQuery(t1), search.SHOULD)
	q1.Add(sseTermQuery(t2), search.SHOULD)
	q2 := search.NewBooleanQuery()
	q2.Add(sseTermQuery(t1), search.SHOULD)
	q2.Add(sseTermQuery(t2), search.SHOULD)
	h.seqAssertSubsetOf(q1, q2)
}

// TestSimpleSearchEquivalence_DisjunctionSumVersusDisjunctionMax ports
// testDisjunctionSumVersusDisjunctionMax: (A B) = (A | B).
func TestSimpleSearchEquivalence_DisjunctionSumVersusDisjunctionMax(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewBooleanQuery()
	q1.Add(sseTermQuery(t1), search.SHOULD)
	q1.Add(sseTermQuery(t2), search.SHOULD)
	q2 := search.NewDisjunctionMaxQueryWithTieBreaker([]search.Query{sseTermQuery(t1), sseTermQuery(t2)}, 0.5)
	h.seqAssertSameSet(q1, q2)
}

// TestSimpleSearchEquivalence_ExactPhraseVersusBooleanAnd ports
// testExactPhraseVersusBooleanAnd: "A B" ⊆ (+A +B).
func TestSimpleSearchEquivalence_ExactPhraseVersusBooleanAnd(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewPhraseQuery(t1.Field, t1, t2)
	q2 := search.NewBooleanQuery()
	q2.Add(sseTermQuery(t1), search.MUST)
	q2.Add(sseTermQuery(t2), search.MUST)
	h.seqAssertSubsetOf(q1, q2)
}

// TestSimpleSearchEquivalence_ExactPhraseVersusBooleanAndWithHoles ports
// testExactPhraseVersusBooleanAndWithHoles.
func TestSimpleSearchEquivalence_ExactPhraseVersusBooleanAndWithHoles(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	b := search.NewPhraseQueryBuilder()
	b.AddTermAtPosition(t1, 0)
	b.AddTermAtPosition(t2, 2)
	q1 := b.Build()
	q2 := search.NewBooleanQuery()
	q2.Add(sseTermQuery(t1), search.MUST)
	q2.Add(sseTermQuery(t2), search.MUST)
	h.seqAssertSubsetOf(q1, q2)
}

// TestSimpleSearchEquivalence_PhraseVersusSloppyPhrase ports
// testPhraseVersusSloppyPhrase: "A B" ⊆ "A B"~1.
func TestSimpleSearchEquivalence_PhraseVersusSloppyPhrase(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewPhraseQuery(t1.Field, t1, t2)
	q2 := search.NewPhraseQueryWithSlop(1, t1.Field, t1, t2)
	h.seqAssertSubsetOf(q1, q2)
}

// TestSimpleSearchEquivalence_PhraseVersusSloppyPhraseWithHoles ports
// testPhraseVersusSloppyPhraseWithHoles.
func TestSimpleSearchEquivalence_PhraseVersusSloppyPhraseWithHoles(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	b := search.NewPhraseQueryBuilder()
	b.AddTermAtPosition(t1, 0)
	b.AddTermAtPosition(t2, 2)
	q1 := b.Build()
	b2 := search.NewPhraseQueryBuilder()
	b2.SetSlop(2)
	b2.AddTermAtPosition(t1, 0)
	b2.AddTermAtPosition(t2, 2)
	q2 := b2.Build()
	h.seqAssertSubsetOf(q1, q2)
}

// TestSimpleSearchEquivalence_ExactPhraseVersusMultiPhrase ports
// testExactPhraseVersusMultiPhrase: "A B" ⊆ "A (B C)".
func TestSimpleSearchEquivalence_ExactPhraseVersusMultiPhrase(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	t3 := h.randomTerm()
	q1 := search.NewPhraseQuery(t1.Field, t1, t2)
	q2b := search.NewMultiPhraseQueryBuilder()
	q2b.Add(t1)
	q2b.AddTerms([]*index.Term{t2, t3})
	h.seqAssertSubsetOf(q1, q2b.Build())
}

// TestSimpleSearchEquivalence_ExactPhraseVersusMultiPhraseWithHoles ports
// testExactPhraseVersusMultiPhraseWithHoles.
func TestSimpleSearchEquivalence_ExactPhraseVersusMultiPhraseWithHoles(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	t3 := h.randomTerm()
	b := search.NewPhraseQueryBuilder()
	b.AddTermAtPosition(t1, 0)
	b.AddTermAtPosition(t2, 2)
	q1 := b.Build()
	q2b := search.NewMultiPhraseQueryBuilder()
	q2b.Add(t1)
	q2b.AddTermsAtPosition([]*index.Term{t2, t3}, 2)
	h.seqAssertSubsetOf(q1, q2b.Build())
}

// TestSimpleSearchEquivalence_SloppyPhraseVersusBooleanAnd ports
// testSloppyPhraseVersusBooleanAnd: "A B"~∞ = +A +B if A != B.
func TestSimpleSearchEquivalence_SloppyPhraseVersusBooleanAnd(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	q1 := search.NewPhraseQueryWithSlop(int(^uint(0)>>1), t1.Field, t1, t2)
	q2 := search.NewBooleanQuery()
	q2.Add(sseTermQuery(t1), search.MUST)
	q2.Add(sseTermQuery(t2), search.MUST)
	h.seqAssertSameSet(q1, q2)
}

// TestSimpleSearchEquivalence_PhraseRelativePositions ports
// testPhraseRelativePositions: phrase positions are relative.
func TestSimpleSearchEquivalence_PhraseRelativePositions(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewPhraseQuery(t1.Field, t1, t2)
	b := search.NewPhraseQueryBuilder()
	b.AddTermAtPosition(t1, 10000)
	b.AddTermAtPosition(t2, 10001)
	q2 := b.Build()
	h.seqAssertSameScores(q1, q2)
}

// TestSimpleSearchEquivalence_SloppyPhraseRelativePositions ports
// testSloppyPhraseRelativePositions.
func TestSimpleSearchEquivalence_SloppyPhraseRelativePositions(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewPhraseQueryWithSlop(2, t1.Field, t1, t2)
	b := search.NewPhraseQueryBuilder()
	b.SetSlop(2)
	b.AddTermAtPosition(t1, 10000)
	b.AddTermAtPosition(t2, 10001)
	q2 := b.Build()
	h.seqAssertSameScores(q1, q2)
}

// TestSimpleSearchEquivalence_BoostQuerySimplification ports
// testBoostQuerySimplification. The AssertingQuery wrapper keeps BoostQuery from
// merging the inner and outer boosts, so the two boost stacks must still score
// identically.
func TestSimpleSearchEquivalence_BoostQuerySimplification(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()) ^ 0xB005)) //nolint:gosec // deterministic test seed
	b1 := rng.Float32() * 10
	b2 := rng.Float32() * 10
	term := h.randomTerm()

	q1 := search.NewBoostQuery(search.NewBoostQuery(sseTermQuery(term), b2), b1)
	q2 := search.NewBoostQuery(newAssertingQuery(search.NewBoostQuery(sseTermQuery(term), b2)), b1)
	h.seqAssertSameScores(q1, q2)
}

// TestSimpleSearchEquivalence_BooleanBoostPropagation ports
// testBooleanBoostPropagation.
func TestSimpleSearchEquivalence_BooleanBoostPropagation(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()) ^ 0xB006)) //nolint:gosec // deterministic test seed
	boost1 := rng.Float32()
	tq := search.NewBoostQuery(sseTermQuery(h.randomTerm()), boost1)

	boost2 := rng.Float32()
	q1 := search.NewBoostQuery(tq, boost2)
	inner := search.NewBooleanQuery()
	inner.Add(tq, search.MUST)
	inner.Add(tq, search.FILTER)
	q2 := search.NewBoostQuery(inner, boost2)
	h.seqAssertSameScores(q1, q2)
}

// TestSimpleSearchEquivalence_BooleanOrVsSynonym ports testBooleanOrVsSynonym.
func TestSimpleSearchEquivalence_BooleanOrVsSynonym(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	t1 := h.randomTerm()
	t2 := h.randomTerm()
	q1 := search.NewSynonymQueryBuilder(t1.Field).AddTerm(t1).AddTerm(t2).Build()
	q2 := search.NewBooleanQuery()
	q2.Add(sseTermQuery(t1), search.SHOULD)
	q2.Add(sseTermQuery(t2), search.SHOULD)
	h.seqAssertSameSet(q1, q2)
}
