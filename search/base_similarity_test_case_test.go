// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Shared harness ported from Apache Lucene 10.4.0:
//   lucene/test-framework/src/java/org/apache/lucene/tests/search/similarities/BaseSimilarityTestCase.java
//
// BaseSimilarityTestCase is the abstract scoring-invariant harness that the
// AxiomaticTestCase / BasicModelTestCase / DistributionTestCase abstract bases
// (and their concrete subclasses) extend. It exercises a Similarity's
// SimScorer over a range of collection/term statistics, freqs and norms and
// asserts the core scoring invariants:
//   - maxScore (score at freq=Float.MAX_VALUE, norm=1) is not NaN
//   - the score is finite and (for non-Indri sims) non-negative
//   - the score never exceeds maxScore
//   - the explanation value equals the score
//   - the score is non-decreasing in freq and non-increasing in encoded norm
//
// This is the Scorer104 (Lucene-faithful) similarity surface.

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// simStatsScenario is one (corpus, term) statistics pairing used to drive the
// scoring-invariant checks.
type simStatsScenario struct {
	name string
	cs   *search.CollectionStatistics
	ts   *search.TermStatistics
}

// baseSimilarityStatsScenarios returns a representative spread of collection /
// term statistics, mirroring the randomized corpora BaseSimilarityTestCase
// generates (tiny, typical and large collections).
func baseSimilarityStatsScenarios() []simStatsScenario {
	return []simStatsScenario{
		{
			name: "tiny",
			cs:   search.NewCollectionStatistics("body", 1, 1, 1, 1),
			ts:   search.NewTermStatistics(index.NewTerm("body", "x"), 1, 1),
		},
		{
			name: "typical",
			cs:   search.NewCollectionStatistics("body", 1000, 1000, 50000, 1000),
			ts:   search.NewTermStatistics(index.NewTerm("body", "go"), 100, 300),
		},
		{
			name: "large",
			cs:   search.NewCollectionStatistics("body", 1_000_000, 1_000_000, 100_000_000, 10_000_000),
			ts:   search.NewTermStatistics(index.NewTerm("body", "common"), 500_000, 5_000_000),
		},
	}
}

// checkSimilarityScoring runs the BaseSimilarityTestCase scoring invariants for
// one Similarity. allowNegative relaxes the non-negative checks for sims (like
// Indri) whose scores may legitimately be negative.
func checkSimilarityScoring(t *testing.T, name string, sim search.LuceneSimilarity, allowNegative bool) {
	t.Helper()
	boosts := []float32{1, 2.5}
	freqs := []float32{1, 2, 4, 15}
	norms := []int64{1, 2, 16, 64, 255}

	for _, sc := range baseSimilarityStatsScenarios() {
		for _, boost := range boosts {
			scorer := sim.Scorer104(boost, sc.cs, sc.ts)
			if scorer == nil {
				t.Errorf("%s/%s boost=%v: nil scorer", name, sc.name, boost)
				continue
			}
			maxScore := scorer.Score104(math.MaxFloat32, 1)
			if math.IsNaN(float64(maxScore)) {
				t.Errorf("%s/%s boost=%v: maxScore is NaN", name, sc.name, boost)
			}
			for _, norm := range norms {
				for _, freq := range freqs {
					score := scorer.Score104(freq, norm)
					if !isFiniteFloat32(score) {
						t.Errorf("%s/%s boost=%v freq=%v norm=%d: infinite/NaN score %v",
							name, sc.name, boost, freq, norm, score)
						continue
					}
					if !allowNegative && score < 0 {
						t.Errorf("%s/%s boost=%v freq=%v norm=%d: negative score %v",
							name, sc.name, boost, freq, norm, score)
					}
					if score > maxScore {
						t.Errorf("%s/%s boost=%v freq=%v norm=%d: score %v > maxScore %v",
							name, sc.name, boost, freq, norm, score, maxScore)
					}
					// Explanation value must equal the score.
					expl := scorer.Explain104(
						search.MatchExplanation(freq, "freq, occurrences of term within document"), norm)
					if expl == nil {
						t.Errorf("%s/%s boost=%v freq=%v norm=%d: nil explanation",
							name, sc.name, boost, freq, norm)
					} else if expl.GetValue() != score {
						t.Errorf("%s/%s boost=%v freq=%v norm=%d: explanation value %v != score %v",
							name, sc.name, boost, freq, norm, expl.GetValue(), score)
					}
				}
				// Monotonic in freq: score is non-decreasing as freq grows.
				prev := scorer.Score104(freqs[0], norm)
				for _, freq := range freqs[1:] {
					cur := scorer.Score104(freq, norm)
					if isFiniteFloat32(prev) && isFiniteFloat32(cur) && cur < prev {
						t.Errorf("%s/%s boost=%v norm=%d: score(%v)=%v < score(prev)=%v (not monotonic in freq)",
							name, sc.name, boost, norm, freq, cur, prev)
					}
					prev = cur
				}
				// Monotonic in norm: a shorter doc (smaller encoded norm) scores
				// at least as high as a longer one for the same freq.
				if norm > 1 {
					shorter := scorer.Score104(freqs[0], norm-1)
					longer := scorer.Score104(freqs[0], norm)
					if isFiniteFloat32(shorter) && isFiniteFloat32(longer) && shorter < longer {
						t.Errorf("%s/%s boost=%v norm=%d: score(norm-1)=%v < score(norm)=%v (not monotonic in norm)",
							name, sc.name, boost, norm, shorter, longer)
					}
				}
			}
		}
	}
}

// isFiniteFloat32 reports whether f is neither NaN nor infinite.
func isFiniteFloat32(f float32) bool {
	return !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0)
}
