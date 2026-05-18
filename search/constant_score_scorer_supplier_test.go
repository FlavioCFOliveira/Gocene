// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"testing"
)

// TestConstantScoreScorerSupplier_Get_BuildsConstantScoreScorer
// confirms Get builds a ConstantScoreScorer over the iterator
// produced by the factory closure and that the scorer carries the
// constant score and ScoreMode the supplier was built with.
func TestConstantScoreScorerSupplier_Get_BuildsConstantScoreScorer(t *testing.T) {
	t.Parallel()
	calls := 0
	supplier := NewConstantScoreScorerSupplier(
		0.75,
		COMPLETE,
		42,
		func(leadCost int64) (DocIdSetIterator, error) {
			calls++
			return NewRangeDocIdSetIterator(0, 3), nil
		},
	)

	if got, want := supplier.Cost(), int64(42); got != want {
		t.Fatalf("Cost before Get: got %d, want %d", got, want)
	}

	scorer, err := supplier.Get(7)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if scorer == nil {
		t.Fatalf("Get: scorer is nil")
	}
	if calls != 1 {
		t.Fatalf("iterator factory: got %d calls, want 1", calls)
	}

	css, ok := scorer.(*ConstantScoreScorer)
	if !ok {
		t.Fatalf("scorer type: got %T, want *ConstantScoreScorer", scorer)
	}
	if got, want := css.Score(), float32(0.75); got != want {
		t.Fatalf("scorer Score: got %v, want %v", got, want)
	}
	if got, want := css.GetScoreMode(), COMPLETE; got != want {
		t.Fatalf("scorer ScoreMode: got %v, want %v", got, want)
	}
}

// TestConstantScoreScorerSupplier_NilFactoryFallsBackToEmpty
// confirms the supplier installs a safe fallback when callers do
// not supply a factory.
func TestConstantScoreScorerSupplier_NilFactoryFallsBackToEmpty(t *testing.T) {
	t.Parallel()
	supplier := NewConstantScoreScorerSupplier(1.0, COMPLETE_NO_SCORES, 5, nil)
	scorer, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got, err := scorer.NextDoc(); err != nil || got != NO_MORE_DOCS {
		t.Fatalf("scorer NextDoc on fallback: got doc=%d err=%v, want doc=%d", got, err, NO_MORE_DOCS)
	}
}

// TestConstantScoreScorerSupplier_NegativeCostClampsToZero locks
// the invariant that a negative cost passed at construction is
// reported as zero (matching the Java reference's documented
// behaviour).
func TestConstantScoreScorerSupplier_NegativeCostClampsToZero(t *testing.T) {
	t.Parallel()
	supplier := NewConstantScoreScorerSupplier(1.0, COMPLETE, -5, nil)
	if got, want := supplier.Cost(), int64(0); got != want {
		t.Fatalf("Cost: got %d, want %d", got, want)
	}
}

// TestConstantScoreScorerSupplier_TopLevelScoringFlag flips the
// flag and confirms IsTopLevelScoringClause reflects it.
func TestConstantScoreScorerSupplier_TopLevelScoringFlag(t *testing.T) {
	t.Parallel()
	supplier := NewConstantScoreScorerSupplier(1.0, COMPLETE, 1, nil)
	if supplier.IsTopLevelScoringClause() {
		t.Fatalf("IsTopLevelScoringClause before set: got true, want false")
	}
	supplier.SetTopLevelScoringClause()
	if !supplier.IsTopLevelScoringClause() {
		t.Fatalf("IsTopLevelScoringClause after set: got false, want true")
	}
}

// TestConstantScoreScorerSupplier_GetPropagatesFactoryError
// confirms that a factory returning an error propagates through
// Get without being silently swallowed.
func TestConstantScoreScorerSupplier_GetPropagatesFactoryError(t *testing.T) {
	t.Parallel()
	target := errors.New("backing store missing")
	supplier := NewConstantScoreScorerSupplier(
		1.0,
		COMPLETE,
		0,
		func(_ int64) (DocIdSetIterator, error) { return nil, target },
	)
	if _, err := supplier.Get(0); !errors.Is(err, target) {
		t.Fatalf("Get: err=%v, want wrapped %v", err, target)
	}
}

// TestConstantScoreScorerSupplierFromIterator_WrapsIterator confirms
// the convenience constructor wraps a pre-built iterator and that
// Cost is propagated from the iterator.
func TestConstantScoreScorerSupplierFromIterator_WrapsIterator(t *testing.T) {
	t.Parallel()
	iter := NewRangeDocIdSetIterator(0, 10)
	supplier := NewConstantScoreScorerSupplierFromIterator(0.25, TOP_SCORES, iter)
	if got, want := supplier.Cost(), int64(10); got != want {
		t.Fatalf("Cost: got %d, want %d", got, want)
	}
	scorer, err := supplier.Get(0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got, want := scorer.Score(), float32(0.25); got != want {
		t.Fatalf("scorer Score: got %v, want %v", got, want)
	}
}
