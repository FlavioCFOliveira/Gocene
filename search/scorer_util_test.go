// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestScorerUtil.java

package search_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestScorerUtil_MinRequiredScore mirrors TestScorerUtil.testMinRequiredScore.
func TestScorerUtil_MinRequiredScore(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	const iters = 10000
	for i := 0; i < iters; i++ {
		maxRemainingScore := rng.Float64()
		minCompetitiveScore := rng.Float32()
		numScorers := rng.Intn(999) + 1 // [1, 999]

		minRequiredScore := search.MinRequiredScore(maxRemainingScore, minCompetitiveScore, numScorers)

		if minCompetitiveScore < float32(maxRemainingScore) {
			if minRequiredScore > 0 {
				t.Errorf("iter %d: minCompetitiveScore(%v) < maxRemainingScore(%v): minRequiredScore=%v, want ≤ 0",
					i, minCompetitiveScore, maxRemainingScore, minRequiredScore)
			}
		} else {
			// The value just below minRequiredScore must NOT produce a sum ≥ minCompetitiveScore.
			below := math.Nextafter(minRequiredScore, math.Inf(-1))
			sumBelow := float32(util.MathSumUpperBound(below+maxRemainingScore, numScorers))
			if sumBelow >= minCompetitiveScore {
				t.Errorf("iter %d: sumUpperBound(nextDown(minRequiredScore)+maxRemaining)=%v >= minCompetitiveScore=%v",
					i, sumBelow, minCompetitiveScore)
			}
		}
	}
}

// TestScorerUtil_CostWithMinShouldMatch covers the CostWithMinShouldMatch
// helper ported from ScorerUtil.costWithMinShouldMatch.
func TestScorerUtil_CostWithMinShouldMatch(t *testing.T) {
	// 3 scorers cost 10,20,30; minShouldMatch=0 → keep 4 cheapest (all 3) → 60.
	got := search.CostWithMinShouldMatch([]int64{10, 20, 30}, 3, 0)
	if got != 60 {
		t.Errorf("CostWithMinShouldMatch(msm=0)=%d, want 60", got)
	}

	// minShouldMatch=1 → keep 3 cheapest → 10+20+30=60.
	got = search.CostWithMinShouldMatch([]int64{10, 20, 30}, 3, 1)
	if got != 60 {
		t.Errorf("CostWithMinShouldMatch(msm=1)=%d, want 60", got)
	}

	// minShouldMatch=2 → keep 2 cheapest → 10+20=30.
	got = search.CostWithMinShouldMatch([]int64{10, 20, 30}, 3, 2)
	if got != 30 {
		t.Errorf("CostWithMinShouldMatch(msm=2)=%d, want 30", got)
	}

	// minShouldMatch=3 would make keep=1 → only cheapest = 10.
	got = search.CostWithMinShouldMatch([]int64{10, 20, 30}, 3, 3)
	if got != 10 {
		t.Errorf("CostWithMinShouldMatch(msm=3)=%d, want 10", got)
	}
}

// TestScorerUtil_MinRequiredScoreNegativeMinCompetitive verifies behaviour
// when minCompetitiveScore is close to zero.
func TestScorerUtil_MinRequiredScoreNegativeMinCompetitive(t *testing.T) {
	// When minCompetitiveScore == 0, minRequiredScore should be ≤ 0.
	got := search.MinRequiredScore(0.5, 0, 10)
	if got > 0 {
		t.Errorf("minRequiredScore(minComp=0)=%v, want ≤ 0", got)
	}
}

// TestScorerUtil_EdgeCases covers additional edge cases for MinRequiredScore.
func TestScorerUtil_EdgeCases(t *testing.T) {
	// Zero numScorers should not panic.
	got := search.MinRequiredScore(1.0, 0.5, 0)
	_ = got

	// maxRemainingScore of 0 with positive minCompetitiveScore.
	got = search.MinRequiredScore(0, 0.5, 1)
	if got > 0.5 {
		t.Errorf("MinRequiredScore(0, 0.5, 1) = %v, want ≤ 0.5", got)
	}

	// maxRemainingScore equals minCompetitiveScore exactly.
	got = search.MinRequiredScore(0.5, 0.5, 1)
	if got > 0 {
		t.Errorf("MinRequiredScore(0.5, 0.5, 1) = %v, want ≤ 0", got)
	}

	// Very large numScorers with same score values.
	got = search.MinRequiredScore(1.0, 100.0, 1000)
	if got <= 0 {
		t.Errorf("MinRequiredScore(1.0, 100.0, 1000) = %v, want > 0", got)
	}

// TestScorerUtil_CostWithMinShouldMatchEdgeCases covers additional edge
// cases for CostWithMinShouldMatch.
func TestScorerUtil_CostWithMinShouldMatchEdgeCases(t *testing.T) {
	// Single scorer.
	got := search.CostWithMinShouldMatch([]int64{42}, 1, 0)
	if got != 42 {
		t.Errorf("single scorer msm=0: want 42, got %d", got)
	}
	got = search.CostWithMinShouldMatch([]int64{42}, 1, 1)
	if got != 42 {
		t.Errorf("single scorer msm=1: want 42, got %d", got)
	}

	// All equal costs.
	got = search.CostWithMinShouldMatch([]int64{10, 10, 10}, 3, 2)
	if got != 20 {
		t.Errorf("equal costs msm=2: want 20, got %d", got)
	}

	// Zero costs.
	got = search.CostWithMinShouldMatch([]int64{0, 0, 0}, 3, 0)
	if got != 0 {
		t.Errorf("zero costs msm=0: want 0, got %d", got)
	}

	// minShouldMatch larger than numScorers — keep ≤ 0 → return 0 (mirrors Lucene).
	got = search.CostWithMinShouldMatch([]int64{10, 20, 30}, 3, 10)
	if got != 0 {
		t.Errorf("msm=10 larger than numScorers: want 0, got %d", got)
	}

	// Unsorted input.
	got = search.CostWithMinShouldMatch([]int64{30, 10, 20}, 3, 2)
	if got != 30 {
		t.Errorf("unsorted input msm=2: want 30 (10+20), got %d", got)
	}
}