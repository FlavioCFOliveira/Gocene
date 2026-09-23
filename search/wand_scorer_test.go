// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestWANDScorer.java
// (Apache Lucene 10.5.0).
//
// Every test except testScalingFactor and testScaleMaxScore drives a
// WANDScorer through an IndexSearcher (the private WANDScorerQuery and
// MaxScoreWrapperQuery build `new WANDScorer(scorers, minShouldMatch,
// scoreMode, leadCost)` and hand it to the search loop as a Scorer). Gocene's
// WANDScorer is a partial port: it does not satisfy the Scorer contract (no
// advanceShallow(int), twoPhaseIterator(), getChildren(); setMinCompetitiveScore
// returns no error), so those tests fail naming it.

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// wandScorerBlocker names the partial production class.
const wandScorerBlocker = "requires org.apache.lucene.search.WANDScorer as a Scorer (partial port: advanceShallow, " +
	"twoPhaseIterator, getChildren not ported) (not ported)"

func TestWANDScorerScalingFactor(t *testing.T) {
	doTestScalingFactor(t, 1)
	doTestScalingFactor(t, 2)
	doTestScalingFactor(t, math.Nextafter32(1, 0))
	doTestScalingFactor(t, math.Nextafter32(1, 2))
	doTestScalingFactor(t, math.SmallestNonzeroFloat32)
	doTestScalingFactor(t, math.Nextafter32(math.SmallestNonzeroFloat32, 1))
	doTestScalingFactor(t, math.MaxFloat32)
	doTestScalingFactor(t, math.Nextafter32(math.MaxFloat32, 0))
	if got, want := search.WANDScoreScalingFactor(0), search.WANDScoreScalingFactor(math.SmallestNonzeroFloat32)+1; got != want {
		t.Fatalf("scalingFactor(0): expected %d, got %d", want, got)
	}
	if got, want := search.WANDScoreScalingFactor(float32(math.Inf(1))), search.WANDScoreScalingFactor(math.MaxFloat32)-1; got != want {
		t.Fatalf("scalingFactor(+Inf): expected %d, got %d", want, got)
	}

	// Greater scores produce lower scaling factors
	if !(search.WANDScoreScalingFactor(1) > search.WANDScoreScalingFactor(10)) {
		t.Fatal("scalingFactor(1f) > scalingFactor(10f)")
	}
	if !(search.WANDScoreScalingFactor(math.MaxFloat32) > search.WANDScoreScalingFactor(float32(math.Inf(1)))) {
		t.Fatal("scalingFactor(Float.MAX_VALUE) > scalingFactor(Float.POSITIVE_INFINITY)")
	}
	if !(search.WANDScoreScalingFactor(0) > search.WANDScoreScalingFactor(math.SmallestNonzeroFloat32)) {
		t.Fatal("scalingFactor(0f) > scalingFactor(Float.MIN_VALUE)")
	}
}

// doTestScalingFactor renders the private doTestScalingFactor(float).
func doTestScalingFactor(t *testing.T, f float32) {
	t.Helper()
	scalingFactor := search.WANDScoreScalingFactor(f)
	scaled := float32(math.Ldexp(float64(f), scalingFactor)) // Math.scalb(f, scalingFactor)
	if !(scaled >= float32(int64(1)<<(search.FloatMantissaBits-1))) {
		t.Fatalf("%v", scaled)
	}
	if !(scaled < float32(int64(1)<<search.FloatMantissaBits)) {
		t.Fatalf("%v", scaled)
	}
}

func TestWANDScorerScaleMaxScore(t *testing.T) {
	if got, want := search.WANDScaleMaxScore(32, search.WANDScoreScalingFactor(32)), int64(1)<<(search.FloatMantissaBits-1); got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
	if got := search.WANDScaleMaxScore(32, search.WANDScoreScalingFactor(float32(math.Ldexp(1, 60)))); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := search.WANDScaleMaxScore(32, search.WANDScoreScalingFactor(float32(math.Inf(1)))); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

func TestWANDScorerBasics(t *testing.T) { t.Fatal(wandScorerBlocker) }

func TestWANDScorerBasicsWithDisjunctionAndMinShouldMatch(t *testing.T) { t.Fatal(wandScorerBlocker) }

func TestWANDScorerBasicsWithDisjunctionAndMinShouldMatchAndTailSizeCondition(t *testing.T) {
	t.Fatal(wandScorerBlocker)
}

func TestWANDScorerBasicsWithDisjunctionAndMinShouldMatchAndNonScoringMode(t *testing.T) {
	t.Fatal(wandScorerBlocker)
}

func TestWANDScorerBasicsWithFilteredDisjunctionAndMinShouldMatch(t *testing.T) {
	t.Fatal(wandScorerBlocker)
}

func TestWANDScorerBasicsWithFilteredDisjunctionAndMinShouldMatchAndNonScoringMode(t *testing.T) {
	t.Fatal(wandScorerBlocker)
}

func TestWANDScorerBasicsWithFilteredDisjunctionAndMustNotAndMinShouldMatch(t *testing.T) {
	t.Fatal(wandScorerBlocker)
}

func TestWANDScorerBasicsWithFilteredDisjunctionAndMustNotAndMinShouldMatchAndNonScoringMode(t *testing.T) {
	t.Fatal(wandScorerBlocker)
}

func TestWANDScorerRandom(t *testing.T) { t.Fatal(wandScorerBlocker) }

func TestWANDScorerRandomWithZeroScores(t *testing.T) { t.Fatal(wandScorerBlocker) }

func TestWANDScorerRandomWithInfiniteMaxScore(t *testing.T) { t.Fatal(wandScorerBlocker) }

func TestWANDScorerRandomWithMaxScoreOverflow(t *testing.T) { t.Fatal(wandScorerBlocker) }
