// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestApproximationSearchEquivalence.java
//
// Basic equivalence tests for approximations: every test compares a BooleanQuery
// built from plain TermQueries against the same shape built from
// RandomApproximationQuery-wrapped TermQueries, asserting the two produce the same
// documents and the same scores (assertSameScores). Because the random
// approximation introduces two-phase false positives that matches() rejects, the
// wrapped query is score-equivalent to the plain query — which is exactly the
// invariant under test. Runs against the shared random a-z corpus from
// search_equivalence_test_base_test.go.

package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// aseRandom returns a deterministic rng seeded from the test name, used to seed
// the RandomApproximationQuery wrappers.
func aseRandom(t *testing.T) *rand.Rand {
	return rand.New(rand.NewSource(hashStringSeed(t.Name()) ^ 0xA9CE)) //nolint:gosec // deterministic test seed
}

func aseApprox(q search.Query, rng *rand.Rand) search.Query {
	return newRandomApproximationQuery(q, rng)
}

// TestApproximationSearchEquivalence_Conjunction ports testConjunction.
func TestApproximationSearchEquivalence_Conjunction(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	q1 := sseTermQuery(h.randomTerm())
	q2 := sseTermQuery(h.randomTerm())

	bq1 := search.NewBooleanQuery()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST)

	bq2 := search.NewBooleanQuery()
	bq2.Add(aseApprox(q1, rng), search.MUST)
	bq2.Add(aseApprox(q2, rng), search.MUST)

	h.seqAssertSameScores(bq1, bq2)
}

// TestApproximationSearchEquivalence_NestedConjunction ports testNestedConjunction.
func TestApproximationSearchEquivalence_NestedConjunction(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQuery()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST)
	bq2 := search.NewBooleanQuery()
	bq2.Add(bq1, search.MUST)
	bq2.Add(q3, search.MUST)

	bq3 := search.NewBooleanQuery()
	bq3.Add(aseApprox(q1, rng), search.MUST)
	bq3.Add(aseApprox(q2, rng), search.MUST)
	bq4 := search.NewBooleanQuery()
	bq4.Add(bq3, search.MUST)
	bq4.Add(q3, search.MUST)

	h.seqAssertSameScores(bq2, bq4)
}

// TestApproximationSearchEquivalence_Disjunction ports testDisjunction.
func TestApproximationSearchEquivalence_Disjunction(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	q1 := sseTermQuery(h.randomTerm())
	q2 := sseTermQuery(h.randomTerm())

	bq1 := search.NewBooleanQuery()
	bq1.Add(q1, search.SHOULD)
	bq1.Add(q2, search.SHOULD)

	bq2 := search.NewBooleanQuery()
	bq2.Add(aseApprox(q1, rng), search.SHOULD)
	bq2.Add(aseApprox(q2, rng), search.SHOULD)

	h.seqAssertSameScores(bq1, bq2)
}

// TestApproximationSearchEquivalence_NestedDisjunction ports testNestedDisjunction.
func TestApproximationSearchEquivalence_NestedDisjunction(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQuery()
	bq1.Add(q1, search.SHOULD)
	bq1.Add(q2, search.SHOULD)
	bq2 := search.NewBooleanQuery()
	bq2.Add(bq1, search.SHOULD)
	bq2.Add(q3, search.SHOULD)

	bq3 := search.NewBooleanQuery()
	bq3.Add(aseApprox(q1, rng), search.SHOULD)
	bq3.Add(aseApprox(q2, rng), search.SHOULD)
	bq4 := search.NewBooleanQuery()
	bq4.Add(bq3, search.SHOULD)
	bq4.Add(q3, search.SHOULD)

	h.seqAssertSameScores(bq2, bq4)
}

// TestApproximationSearchEquivalence_DisjunctionInConjunction ports testDisjunctionInConjunction.
func TestApproximationSearchEquivalence_DisjunctionInConjunction(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQuery()
	bq1.Add(q1, search.SHOULD)
	bq1.Add(q2, search.SHOULD)
	bq2 := search.NewBooleanQuery()
	bq2.Add(bq1, search.MUST)
	bq2.Add(q3, search.MUST)

	bq3 := search.NewBooleanQuery()
	bq3.Add(aseApprox(q1, rng), search.SHOULD)
	bq3.Add(aseApprox(q2, rng), search.SHOULD)
	bq4 := search.NewBooleanQuery()
	bq4.Add(bq3, search.MUST)
	bq4.Add(q3, search.MUST)

	h.seqAssertSameScores(bq2, bq4)
}

// TestApproximationSearchEquivalence_ConjunctionInDisjunction ports testConjunctionInDisjunction.
func TestApproximationSearchEquivalence_ConjunctionInDisjunction(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQuery()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST)
	bq2 := search.NewBooleanQuery()
	bq2.Add(bq1, search.SHOULD)
	bq2.Add(q3, search.SHOULD)

	bq3 := search.NewBooleanQuery()
	bq3.Add(aseApprox(q1, rng), search.MUST)
	bq3.Add(aseApprox(q2, rng), search.MUST)
	bq4 := search.NewBooleanQuery()
	bq4.Add(bq3, search.SHOULD)
	bq4.Add(q3, search.SHOULD)

	h.seqAssertSameScores(bq2, bq4)
}

// TestApproximationSearchEquivalence_ConstantScore ports testConstantScore.
func TestApproximationSearchEquivalence_ConstantScore(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	q1 := sseTermQuery(h.randomTerm())
	q2 := sseTermQuery(h.randomTerm())

	bq1 := search.NewBooleanQuery()
	bq1.Add(search.NewConstantScoreQuery(q1), search.MUST)
	bq1.Add(search.NewConstantScoreQuery(q2), search.MUST)

	bq2 := search.NewBooleanQuery()
	bq2.Add(search.NewConstantScoreQuery(aseApprox(q1, rng)), search.MUST)
	bq2.Add(search.NewConstantScoreQuery(aseApprox(q2, rng)), search.MUST)

	h.seqAssertSameScores(bq1, bq2)
}

// TestApproximationSearchEquivalence_Exclusion ports testExclusion.
func TestApproximationSearchEquivalence_Exclusion(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	q1 := sseTermQuery(h.randomTerm())
	q2 := sseTermQuery(h.randomTerm())

	bq1 := search.NewBooleanQuery()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST_NOT)

	bq2 := search.NewBooleanQuery()
	bq2.Add(aseApprox(q1, rng), search.MUST)
	bq2.Add(aseApprox(q2, rng), search.MUST_NOT)

	h.seqAssertSameScores(bq1, bq2)
}

// TestApproximationSearchEquivalence_NestedExclusion ports testNestedExclusion.
func TestApproximationSearchEquivalence_NestedExclusion(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQuery()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.MUST_NOT)
	bq2 := search.NewBooleanQuery()
	bq2.Add(bq1, search.MUST)
	bq2.Add(q3, search.MUST)

	// Both req and excl have approximations.
	bq3 := search.NewBooleanQuery()
	bq3.Add(aseApprox(q1, rng), search.MUST)
	bq3.Add(aseApprox(q2, rng), search.MUST_NOT)
	bq4 := search.NewBooleanQuery()
	bq4.Add(bq3, search.MUST)
	bq4.Add(q3, search.MUST)
	h.seqAssertSameScores(bq2, bq4)

	// Only req has an approximation.
	bq3 = search.NewBooleanQuery()
	bq3.Add(aseApprox(q1, rng), search.MUST)
	bq3.Add(q2, search.MUST_NOT)
	bq4 = search.NewBooleanQuery()
	bq4.Add(bq3, search.MUST)
	bq4.Add(q3, search.MUST)
	h.seqAssertSameScores(bq2, bq4)

	// Only excl has an approximation.
	bq3 = search.NewBooleanQuery()
	bq3.Add(q1, search.MUST)
	bq3.Add(aseApprox(q2, rng), search.MUST_NOT)
	bq4 = search.NewBooleanQuery()
	bq4.Add(bq3, search.MUST)
	bq4.Add(q3, search.MUST)
	h.seqAssertSameScores(bq2, bq4)
}

// TestApproximationSearchEquivalence_ReqOpt ports testReqOpt.
func TestApproximationSearchEquivalence_ReqOpt(t *testing.T) {
	h := newSeqHarness(t)
	defer h.close()
	rng := aseRandom(t)
	t1 := h.randomTerm()
	t2 := h.randomTermDistinct(t1)
	t3 := h.randomTerm()
	q1 := sseTermQuery(t1)
	q2 := sseTermQuery(t2)
	q3 := sseTermQuery(t3)

	bq1 := search.NewBooleanQuery()
	bq1.Add(q1, search.MUST)
	bq1.Add(q2, search.SHOULD)
	bq2 := search.NewBooleanQuery()
	bq2.Add(bq1, search.MUST)
	bq2.Add(q3, search.MUST)

	bq3 := search.NewBooleanQuery()
	bq3.Add(aseApprox(q1, rng), search.MUST)
	bq3.Add(aseApprox(q2, rng), search.SHOULD)
	bq4 := search.NewBooleanQuery()
	bq4.Add(bq3, search.MUST)
	bq4.Add(q3, search.MUST)

	h.seqAssertSameScores(bq2, bq4)
}
