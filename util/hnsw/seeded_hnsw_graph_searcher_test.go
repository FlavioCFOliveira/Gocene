// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// recordingSearcher captures the (level, eps) it is invoked with so
// the tests can assert seed handoff without running a real beam
// search. FindBestEntryPoint must never be reached on the seeded
// path; calling it fails the test loudly.
type recordingSearcher struct {
	t           *testing.T
	lastLevel   int
	lastEps     []int
	searchCalls int
	bestCalls   int
}

func (r *recordingSearcher) SearchLevel(
	_ KnnCollector,
	_ RandomVectorScorer,
	level int,
	eps []int,
	_ HnswGraph,
	_ util.Bits,
) error {
	r.searchCalls++
	r.lastLevel = level
	r.lastEps = append(r.lastEps[:0], eps...)
	return nil
}

func (r *recordingSearcher) FindBestEntryPoint(
	_ RandomVectorScorer,
	_ HnswGraph,
	_ KnnCollector,
) ([]int, error) {
	r.bestCalls++
	r.t.Fatalf("seeded searcher must not delegate FindBestEntryPoint")
	return nil, nil
}

// stubIterator is the minimum DocIdSetIterator implementation needed
// to feed seededFromEntryPoints: NextDoc walks a slice of docs and
// returns NO_MORE_DOCS once it is drained. Unused operations panic.
type stubIterator struct {
	docs []int
	pos  int
}

func newStubIterator(docs ...int) *stubIterator { return &stubIterator{docs: docs} }

func (s *stubIterator) DocID() int { panic("unused") }
func (s *stubIterator) NextDoc() (int, error) {
	if s.pos >= len(s.docs) {
		return util.NO_MORE_DOCS, nil
	}
	v := s.docs[s.pos]
	s.pos++
	return v, nil
}
func (s *stubIterator) Advance(int) (int, error) { panic("unused") }
func (s *stubIterator) Cost() int64              { return int64(len(s.docs)) }
func (s *stubIterator) DocIDRunEnd() int         { panic("unused") }

// errIterator is a DocIdSetIterator whose NextDoc always errors;
// used to confirm seededFromEntryPoints surfaces iterator failures
// wrapped, not swallowed.
type errIterator struct{ err error }

func (e *errIterator) DocID() int               { panic("unused") }
func (e *errIterator) NextDoc() (int, error)    { return 0, e.err }
func (e *errIterator) Advance(int) (int, error) { panic("unused") }
func (e *errIterator) Cost() int64              { return 0 }
func (e *errIterator) DocIDRunEnd() int         { panic("unused") }

// TestSeededHnswGraphSearcher_FromEntryPointsDrainsIterator asserts
// the factory consumes exactly numEps ordinals and preserves their
// order in the seedOrds slice.
func TestSeededHnswGraphSearcher_FromEntryPointsDrainsIterator(t *testing.T) {
	delegate := &recordingSearcher{t: t}
	it := newStubIterator(3, 7, 1, 9) // factory must only read the first 3.

	s, err := seededFromEntryPoints(delegate, 3, it, 100)
	if err != nil {
		t.Fatalf("seededFromEntryPoints: %v", err)
	}
	want := []int{3, 7, 1}
	if len(s.seedOrds) != len(want) {
		t.Fatalf("seedOrds len=%d, want %d", len(s.seedOrds), len(want))
	}
	for i, v := range want {
		if s.seedOrds[i] != v {
			t.Errorf("seedOrds[%d]=%d, want %d", i, s.seedOrds[i], v)
		}
	}
	if it.pos != 3 {
		t.Errorf("iterator drained %d times, want 3", it.pos)
	}
}

// TestSeededHnswGraphSearcher_FindBestReturnsSeeds asserts the
// seeded searcher's FindBestEntryPoint short-circuits the upper-
// level descent and returns the stored seeds untouched. It must
// also never call the delegate's FindBestEntryPoint.
func TestSeededHnswGraphSearcher_FindBestReturnsSeeds(t *testing.T) {
	delegate := &recordingSearcher{t: t}
	seeds := []int{4, 0, 2}
	s := newSeededHnswGraphSearcher(delegate, seeds)

	got, err := s.FindBestEntryPoint(nil, nil, nil)
	if err != nil {
		t.Fatalf("FindBestEntryPoint: %v", err)
	}
	if len(got) != len(seeds) {
		t.Fatalf("entry points len=%d, want %d", len(got), len(seeds))
	}
	for i, v := range seeds {
		if got[i] != v {
			t.Errorf("entry[%d]=%d, want %d", i, got[i], v)
		}
	}
	if delegate.bestCalls != 0 {
		t.Errorf("delegate.FindBestEntryPoint called %d times, want 0", delegate.bestCalls)
	}
}

// TestSeededHnswGraphSearcher_SearchLevelForwards asserts SearchLevel
// is a transparent pass-through to the delegate.
func TestSeededHnswGraphSearcher_SearchLevelForwards(t *testing.T) {
	delegate := &recordingSearcher{t: t}
	eps := []int{5, 6}
	s := newSeededHnswGraphSearcher(delegate, eps)

	if err := s.SearchLevel(nil, nil, 0, eps, nil, nil); err != nil {
		t.Fatalf("SearchLevel: %v", err)
	}
	if delegate.searchCalls != 1 {
		t.Fatalf("delegate.SearchLevel called %d times, want 1", delegate.searchCalls)
	}
	if delegate.lastLevel != 0 {
		t.Errorf("delegate received level %d, want 0", delegate.lastLevel)
	}
	if len(delegate.lastEps) != len(eps) || delegate.lastEps[0] != 5 || delegate.lastEps[1] != 6 {
		t.Errorf("delegate received eps %v, want %v", delegate.lastEps, eps)
	}
}

// TestSeededHnswGraphSearcher_FromEntryPointsRejectsNonPositive
// covers the numEps <= 0 guard.
func TestSeededHnswGraphSearcher_FromEntryPointsRejectsNonPositive(t *testing.T) {
	delegate := &recordingSearcher{t: t}
	for _, n := range []int{0, -1, -42} {
		if _, err := seededFromEntryPoints(delegate, n, newStubIterator(1), 10); !errors.Is(err, errSeededNoEntryPoints) {
			t.Errorf("numEps=%d: err=%v, want errSeededNoEntryPoints", n, err)
		}
	}
}

// TestSeededHnswGraphSearcher_FromEntryPointsErrorsOnExhaustion
// covers the case where the iterator yields fewer ordinals than
// requested.
func TestSeededHnswGraphSearcher_FromEntryPointsErrorsOnExhaustion(t *testing.T) {
	delegate := &recordingSearcher{t: t}
	_, err := seededFromEntryPoints(delegate, 3, newStubIterator(1, 2), 10)
	if !errors.Is(err, errSeededTooFewEntryPoints) {
		t.Fatalf("err=%v, want errSeededTooFewEntryPoints", err)
	}
}

// TestSeededHnswGraphSearcher_FromEntryPointsWrapsIteratorError
// confirms iterator failures bubble up wrapped (errors.Is must reach
// the original).
func TestSeededHnswGraphSearcher_FromEntryPointsWrapsIteratorError(t *testing.T) {
	delegate := &recordingSearcher{t: t}
	sentinel := errors.New("boom")
	_, err := seededFromEntryPoints(delegate, 1, &errIterator{err: sentinel}, 10)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v, want chain containing %v", err, sentinel)
	}
}

// TestSeededHnswGraphSearcher_FromEntryPointsPanicsOnOutOfRange
// confirms the assert-equivalent fires when an entry-point ordinal is
// >= graphSize.
func TestSeededHnswGraphSearcher_FromEntryPointsPanicsOnOutOfRange(t *testing.T) {
	delegate := &recordingSearcher{t: t}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on out-of-range entry point")
		}
	}()
	_, _ = seededFromEntryPoints(delegate, 1, newStubIterator(99), 10)
}

// TestSeededHnswGraphSearcher_SearchUsesSeeds is the end-to-end
// check: invoking the package-level Search helper on a seeded
// searcher must call the delegate's SearchLevel with the supplied
// seeds and never its FindBestEntryPoint.
func TestSeededHnswGraphSearcher_SearchUsesSeeds(t *testing.T) {
	delegate := &recordingSearcher{t: t}
	seeds := []int{8, 1, 4}
	s := newSeededHnswGraphSearcher(delegate, seeds)

	if err := Search(s, nil, nil, nil, nil); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if delegate.searchCalls != 1 {
		t.Fatalf("delegate.SearchLevel calls=%d, want 1", delegate.searchCalls)
	}
	if delegate.bestCalls != 0 {
		t.Fatalf("delegate.FindBestEntryPoint calls=%d, want 0", delegate.bestCalls)
	}
	if len(delegate.lastEps) != len(seeds) {
		t.Fatalf("eps len=%d, want %d", len(delegate.lastEps), len(seeds))
	}
	for i, v := range seeds {
		if delegate.lastEps[i] != v {
			t.Errorf("eps[%d]=%d, want %d", i, delegate.lastEps[i], v)
		}
	}
}
