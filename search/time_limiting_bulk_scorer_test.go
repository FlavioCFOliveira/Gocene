// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestTimeLimitingBulkScorer.java

package search_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// countingQueryTimeout fires after a fixed number of ShouldExit calls.
type countingQueryTimeout struct {
	limit   int
	counter int
}

func newCountingQueryTimeout(limit int) *countingQueryTimeout {
	return &countingQueryTimeout{limit: limit}
}

func (q *countingQueryTimeout) ShouldExit() bool {
	q.counter++
	return q.counter >= q.limit
}

// alwaysExitTimeout returns true on every call.
type alwaysExitTimeout struct{}

func (a alwaysExitTimeout) ShouldExit() bool { return true }

// neverExitTimeout never requests exit.
type neverExitTimeout struct{}

func (n neverExitTimeout) ShouldExit() bool { return false }

// noopBulkScorer is a BulkScorer that does nothing and reports cost 42.
type noopBulkScorer struct{}

func (s *noopBulkScorer) Score(_ search.Collector, _ search.DocIdSetIterator) error { return nil }
func (s *noopBulkScorer) Cost() int64                                               { return 42 }

var _ search.BulkScorer = (*noopBulkScorer)(nil)

// errorBulkScorer returns a specific error from Score.
type errorBulkScorer struct{ err error }

func (s *errorBulkScorer) Score(_ search.Collector, _ search.DocIdSetIterator) error {
	return s.err
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestTimeLimitingBulkScorer_TimeLimitingBulkScorer mirrors
// TestTimeLimitingBulkScorer.testTimeLimitingBulkScorer.
// The Java test requires a full index + searcher; that infrastructure is not
// yet available in Gocene. Ported as a degraded stub.
func TestTimeLimitingBulkScorer_TimeLimitingBulkScorer(t *testing.T) {
	t.Skip("requires full IndexWriter/IndexSearcher/TermQuery — deferred until index round-trip is wired")
}

// TestTimeLimitingBulkScorer_ExponentialRate mirrors
// TestTimeLimitingBulkScorer.testExponentialRate.
// The Java test verifies the growing-interval arithmetic of the inner
// score(min,max) loop. Since Gocene's BulkScorer contract does not expose
// min/max range parameters the exponential-rate path is not implemented;
// this test is recorded as a degraded stub.
func TestTimeLimitingBulkScorer_ExponentialRate(t *testing.T) {
	t.Skip("exponential-rate interval arithmetic requires min/max BulkScorer API — deferred")
}

// TestTimeLimitingBulkScorer_TimeoutOnEntry verifies ErrTimeExceeded is
// returned immediately when the timeout fires before the first delegation.
func TestTimeLimitingBulkScorer_TimeoutOnEntry(t *testing.T) {
	s := search.NewTimeLimitingBulkScorer(&noopBulkScorer{}, alwaysExitTimeout{})
	err := s.Score(nil, nil)
	if !errors.Is(err, search.ErrTimeExceeded) {
		t.Errorf("Score() error = %v, want ErrTimeExceeded", err)
	}
}

// TestTimeLimitingBulkScorer_NoTimeout verifies the inner scorer is called
// when the timeout has not fired.
func TestTimeLimitingBulkScorer_NoTimeout(t *testing.T) {
	inner := &noopBulkScorer{}
	s := search.NewTimeLimitingBulkScorer(inner, neverExitTimeout{})
	if err := s.Score(nil, nil); err != nil {
		t.Errorf("Score() error = %v, want nil", err)
	}
}

// TestTimeLimitingBulkScorer_InnerErrorPropagated verifies that errors from
// the inner scorer are returned unchanged.
func TestTimeLimitingBulkScorer_InnerErrorPropagated(t *testing.T) {
	sentinel := errors.New("inner error")
	s := search.NewTimeLimitingBulkScorer(&errorBulkScorer{err: sentinel}, neverExitTimeout{})
	err := s.Score(nil, nil)
	if !errors.Is(err, sentinel) {
		t.Errorf("Score() error = %v, want sentinel", err)
	}
}

// TestTimeLimitingBulkScorer_CountingTimeout fires after N calls; verifies
// the first call succeeds (counter not yet at limit) and the second times out.
func TestTimeLimitingBulkScorer_CountingTimeout(t *testing.T) {
	qt := newCountingQueryTimeout(2) // fires on call 2
	s := search.NewTimeLimitingBulkScorer(&noopBulkScorer{}, qt)

	// First call: counter becomes 1 < limit → no exit.
	if err := s.Score(nil, nil); err != nil {
		t.Errorf("first Score() error = %v, want nil", err)
	}
	// Second call: counter becomes 2 == limit → exit.
	err := s.Score(nil, nil)
	if !errors.Is(err, search.ErrTimeExceeded) {
		t.Errorf("second Score() error = %v, want ErrTimeExceeded", err)
	}
}

// TestTimeLimitingBulkScorer_CostDelegated verifies Cost delegates to inner.
func TestTimeLimitingBulkScorer_CostDelegated(t *testing.T) {
	bs := &noopBulkScorer{}
	s := search.NewTimeLimitingBulkScorer(bs, neverExitTimeout{})
	if s.Cost() != 42 {
		t.Errorf("Cost() = %d, want 42", s.Cost())
	}
}

// TestTimeLimitingBulkScorer_NilBulkScorerPanics verifies the constructor
// rejects a nil inner scorer.
func TestTimeLimitingBulkScorer_NilBulkScorerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil bulkScorer, got none")
		}
	}()
	search.NewTimeLimitingBulkScorer(nil, neverExitTimeout{})
}

// TestTimeLimitingBulkScorer_NilTimeoutPanics verifies the constructor
// rejects a nil queryTimeout.
func TestTimeLimitingBulkScorer_NilTimeoutPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil queryTimeout, got none")
		}
	}()
	search.NewTimeLimitingBulkScorer(&noopBulkScorer{}, nil)
}

// TestTimeLimitingBulkScorer_ImplementsBulkScorer checks the interface
// satisfaction at compile time.
func TestTimeLimitingBulkScorer_ImplementsBulkScorer(t *testing.T) {
	var _ search.BulkScorer = search.NewTimeLimitingBulkScorer(&noopBulkScorer{}, neverExitTimeout{})
}
