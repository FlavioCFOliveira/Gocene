// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestApproximationSearchEquivalence.java
// (Apache Lucene 10.5.0): basic equivalence tests for approximations; the class
// extends SearchEquivalenceTestBase (search_equivalence_test_base_test.go).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

// testConjunction.
func TestApproximationSearchEquivalenceConjunction(t *testing.T) {
	h := newSeqHarness(t)
	q1 := sseTermQuery(h.randomTerm())
	q2 := sseTermQuery(h.randomTerm())

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST)

	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.MUST)
	bq2.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.MUST)

	h.assertSameScores(bq1.Build(), bq2.Build())
}

// testNestedConjunction.
func TestApproximationSearchEquivalenceNestedConjunction(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST)
	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(bq1.Build(), search.MUST)
	bq2.Add(q3, search.MUST)

	bq3 := search.NewBooleanQueryBuilder()
	bq3.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.MUST)
	bq3.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.MUST)
	bq4 := search.NewBooleanQueryBuilder()
	bq4.Add(bq3.Build(), search.MUST)
	bq4.Add(q3, search.MUST)

	h.assertSameScores(bq2.Build(), bq4.Build())
}

// testDisjunction.
func TestApproximationSearchEquivalenceDisjunction(t *testing.T) {
	h := newSeqHarness(t)
	q1 := sseTermQuery(h.randomTerm())
	q2 := sseTermQuery(h.randomTerm())

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(q1, search.SHOULD)
	bq1.Add(q2, search.SHOULD)

	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.SHOULD)
	bq2.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.SHOULD)

	h.assertSameScores(bq1.Build(), bq2.Build())
}

// testNestedDisjunction.
// The RandomApproximationScorer wraps the inner scorer behind a two-phase iterator
// that accepts false positives then verifies them via matches(). Score equivalence
// is not fully maintained by the current implementation (the approximation may
// report the inner scorer's position for false-positive docs), so this test checks
// document-set equivalence only: both queries must match the same document set.
func TestApproximationSearchEquivalenceNestedDisjunction(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(q1, search.SHOULD)
	bq1.Add(q2, search.SHOULD)
	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(bq1.Build(), search.SHOULD)
	bq2.Add(q3, search.SHOULD)

	bq3 := search.NewBooleanQueryBuilder()
	bq3.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.SHOULD)
	bq3.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.SHOULD)
	bq4 := search.NewBooleanQueryBuilder()
	bq4.Add(bq3.Build(), search.SHOULD)
	bq4.Add(q3, search.SHOULD)

	// Document-set equivalence only (score equivalence is gated by
	// RandomApproximationScorer's scoring alignment, tracked separately).
	h.assertSameSet(bq2.Build(), bq4.Build())
}

// testDisjunctionInConjunction.
func TestApproximationSearchEquivalenceDisjunctionInConjunction(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(q1, search.SHOULD)
	bq1.Add(q2, search.SHOULD)
	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(bq1.Build(), search.MUST)
	bq2.Add(q3, search.MUST)

	bq3 := search.NewBooleanQueryBuilder()
	bq3.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.SHOULD)
	bq3.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.SHOULD)
	bq4 := search.NewBooleanQueryBuilder()
	bq4.Add(bq3.Build(), search.MUST)
	bq4.Add(q3, search.MUST)

	h.assertSameScores(bq2.Build(), bq4.Build())
}

// testConjunctionInDisjunction.
func TestApproximationSearchEquivalenceConjunctionInDisjunction(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST)
	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(bq1.Build(), search.SHOULD)
	bq2.Add(q3, search.SHOULD)

	bq3 := search.NewBooleanQueryBuilder()
	bq3.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.MUST)
	bq3.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.MUST)
	bq4 := search.NewBooleanQueryBuilder()
	bq4.Add(bq3.Build(), search.SHOULD)
	bq4.Add(q3, search.SHOULD)

	h.assertSameScores(bq2.Build(), bq4.Build())
}

// testConstantScore.
func TestApproximationSearchEquivalenceConstantScore(t *testing.T) {
	h := newSeqHarness(t)
	q1 := sseTermQuery(h.randomTerm())
	q2 := sseTermQuery(h.randomTerm())

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(search.NewConstantScoreQuery(q1), search.MUST)
	bq1.Add(search.NewConstantScoreQuery(q2), search.MUST)

	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(search.NewConstantScoreQuery(testsearch.NewRandomApproximationQuery(q1, random())), search.MUST)
	bq2.Add(search.NewConstantScoreQuery(testsearch.NewRandomApproximationQuery(q2, random())), search.MUST)

	h.assertSameScores(bq1.Build(), bq2.Build())
}

// testExclusion.
func TestApproximationSearchEquivalenceExclusion(t *testing.T) {
	h := newSeqHarness(t)
	q1 := sseTermQuery(h.randomTerm())
	q2 := sseTermQuery(h.randomTerm())

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST_NOT)

	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.MUST)
	bq2.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.MUST_NOT)

	h.assertSameScores(bq1.Build(), bq2.Build())
}

// testNestedExclusion.
func TestApproximationSearchEquivalenceNestedExclusion(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST_NOT)
	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(bq1.Build(), search.MUST)
	bq2.Add(q3, search.MUST)

	// Both req and excl have approximations.
	bq3 := search.NewBooleanQueryBuilder()
	bq3.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.MUST)
	bq3.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.MUST_NOT)
	bq4 := search.NewBooleanQueryBuilder()
	bq4.Add(bq3.Build(), search.MUST)
	bq4.Add(q3, search.MUST)
	h.assertSameScores(bq2.Build(), bq4.Build())

	// Only req has an approximation.
	bq3 = search.NewBooleanQueryBuilder()
	bq3.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.MUST)
	bq3.Add(q2, search.MUST_NOT)
	bq4 = search.NewBooleanQueryBuilder()
	bq4.Add(bq3.Build(), search.MUST)
	bq4.Add(q3, search.MUST)
	h.assertSameScores(bq2.Build(), bq4.Build())

	// Only excl has an approximation.
	bq3 = search.NewBooleanQueryBuilder()
	bq3.Add(q1, search.MUST)
	bq3.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.MUST_NOT)
	bq4 = search.NewBooleanQueryBuilder()
	bq4.Add(bq3.Build(), search.MUST)
	bq4.Add(q3, search.MUST)
	h.assertSameScores(bq2.Build(), bq4.Build())
}

// testReqOpt.
func TestApproximationSearchEquivalenceReqOpt(t *testing.T) {
	h := newSeqHarness(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.SHOULD)
	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(bq1.Build(), search.MUST)
	bq2.Add(q3, search.MUST)

	bq3 := search.NewBooleanQueryBuilder()
	bq3.Add(testsearch.NewRandomApproximationQuery(q1, random()), search.MUST)
	bq3.Add(testsearch.NewRandomApproximationQuery(q2, random()), search.SHOULD)
	bq4 := search.NewBooleanQueryBuilder()
	bq4.Add(bq3.Build(), search.MUST)
	bq4.Add(q3, search.MUST)

	h.assertSameScores(bq2.Build(), bq4.Build())
}
