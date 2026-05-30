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
	"github.com/FlavioCFOliveira/Gocene/util"
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

// noopBulkScorer is a BulkScorer that does nothing, reports cost 42, and
// returns max so the windowing loop terminates.
type noopBulkScorer struct{}

func (s *noopBulkScorer) Score(_ search.LeafCollector, _ util.Bits, _, max int) (int, error) {
	return max, nil
}
func (s *noopBulkScorer) Cost() int64 { return 42 }

var _ search.BulkScorer = (*noopBulkScorer)(nil)

// errorBulkScorer returns a specific error from Score.
type errorBulkScorer struct{ err error }

func (s *errorBulkScorer) Score(_ search.LeafCollector, _ util.Bits, _, max int) (int, error) {
	return max, s.err
}
func (s *errorBulkScorer) Cost() int64 { return 1 }

// dummyLeafCollector is a no-op LeafCollector used to drive a BulkScorer.
type dummyLeafCollector struct{}

func (dummyLeafCollector) SetScorer(_ search.Scorer) error { return nil }
func (dummyLeafCollector) Collect(_ int) error             { return nil }

// scoreFull drives a BulkScorer over the whole document space, the analogue of
// score(collector, acceptDocs, 0, NO_MORE_DOCS) in the Lucene tests.
func scoreFull(s search.BulkScorer) (int, error) {
	return s.Score(dummyLeafCollector{}, nil, 0, search.NO_MORE_DOCS)
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestTimeLimitingBulkScorer_TimeLimitingBulkScorer mirrors
// TestTimeLimitingBulkScorer.testTimeLimitingBulkScorer.
// The Java test requires a full index + searcher; that infrastructure is not
// yet available in Gocene. Ported as a degraded stub.
func TestTimeLimitingBulkScorer_TimeLimitingBulkScorer(t *testing.T) {
	t.Fatal("requires full IndexWriter/IndexSearcher/TermQuery — deferred until index round-trip is wired")
}

// TestTimeLimitingBulkScorer_ExponentialRate is a faithful port of
// TestTimeLimitingBulkScorer.testExponentialRate. It verifies the
// growing-interval arithmetic of the inner score(min, max) windowing loop now
// that Gocene's BulkScorer exposes the min/max range contract (rmp #4777).
func TestTimeLimitingBulkScorer_ExponentialRate(t *testing.T) {
	const maxDocs = search.NO_MORE_DOCS - 1

	inner := &exponentialRateBulkScorer{t: t, expectedInterval: search.Interval}
	scorer := search.NewTimeLimitingBulkScorer(inner, neverExitTimeout{})
	if _, err := scorer.Score(dummyLeafCollector{}, util.NewMatchAllBits(search.NO_MORE_DOCS), 0, maxDocs); err != nil {
		t.Fatalf("Score: %v", err)
	}
}

// exponentialRateBulkScorer asserts that the windows handed to it by
// TimeLimitingBulkScorer grow by 50% each call without skipping documents,
// mirroring the anonymous BulkScorer in Lucene's testExponentialRate.
type exponentialRateBulkScorer struct {
	t                *testing.T
	expectedInterval int
	lastMax          int
	lastInterval     int
}

func (s *exponentialRateBulkScorer) Score(_ search.LeafCollector, _ util.Bits, min, max int) (int, error) {
	const maxDocs = search.NO_MORE_DOCS - 1
	difference := max - min

	// The rate should only increase or remain equal, never overflow.
	if difference < s.lastInterval {
		s.t.Fatalf("rate should only go up: difference=%d lastInterval=%d", difference, s.lastInterval)
	}
	// No documents should be skipped between windows.
	if s.lastMax != min {
		s.t.Fatalf("documents skipped: lastMax=%d min=%d", s.lastMax, min)
	}
	// The difference equals the step, except on the last window where fewer
	// docs remain.
	if max == maxDocs {
		if s.expectedInterval < difference {
			s.t.Fatalf("incorrect rate (final window): expectedInterval=%d difference=%d", s.expectedInterval, difference)
		}
	} else if s.expectedInterval != difference {
		s.t.Fatalf("incorrect rate: expectedInterval=%d difference=%d", s.expectedInterval, difference)
	}

	s.lastMax = max
	s.lastInterval = difference

	// Integer sum mirrors the exponential-growth formula used by the scorer.
	s.expectedInterval = s.expectedInterval + s.expectedInterval/2
	if s.expectedInterval < 0 { // overflow guard
		s.expectedInterval = s.lastInterval
	}
	return max, nil
}

func (s *exponentialRateBulkScorer) Cost() int64 { return 1 }

// TestTimeLimitingBulkScorer_TimeoutOnEntry verifies ErrTimeExceeded is
// returned when the timeout fires before the first delegation.
func TestTimeLimitingBulkScorer_TimeoutOnEntry(t *testing.T) {
	s := search.NewTimeLimitingBulkScorer(&noopBulkScorer{}, alwaysExitTimeout{})
	_, err := scoreFull(s)
	if !errors.Is(err, search.ErrTimeExceeded) {
		t.Errorf("Score() error = %v, want ErrTimeExceeded", err)
	}
}

// TestTimeLimitingBulkScorer_NoTimeout verifies the inner scorer is called
// when the timeout has not fired.
func TestTimeLimitingBulkScorer_NoTimeout(t *testing.T) {
	inner := &noopBulkScorer{}
	s := search.NewTimeLimitingBulkScorer(inner, neverExitTimeout{})
	if _, err := scoreFull(s); err != nil {
		t.Errorf("Score() error = %v, want nil", err)
	}
}

// TestTimeLimitingBulkScorer_InnerErrorPropagated verifies that errors from
// the inner scorer are returned unchanged.
func TestTimeLimitingBulkScorer_InnerErrorPropagated(t *testing.T) {
	sentinel := errors.New("inner error")
	s := search.NewTimeLimitingBulkScorer(&errorBulkScorer{err: sentinel}, neverExitTimeout{})
	_, err := scoreFull(s)
	if !errors.Is(err, sentinel) {
		t.Errorf("Score() error = %v, want sentinel", err)
	}
}

// TestTimeLimitingBulkScorer_CountingTimeout fires after N timeout checks;
// the windowed loop makes one check per sub-window, so a small index space
// scored in a single window exits after the first check when limit is 1.
func TestTimeLimitingBulkScorer_CountingTimeout(t *testing.T) {
	qt := newCountingQueryTimeout(1) // fires on the first check
	s := search.NewTimeLimitingBulkScorer(&noopBulkScorer{}, qt)

	_, err := scoreFull(s)
	if !errors.Is(err, search.ErrTimeExceeded) {
		t.Errorf("Score() error = %v, want ErrTimeExceeded", err)
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
