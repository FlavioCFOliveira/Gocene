// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"math"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// trackingScorer wraps a fakeScorer and counts every Score / BulkScore
// invocation. Used by the filtered-path dispatch test to confirm
// FilteredHnswGraphSearcher scored only accepted ordinals.
type trackingScorer struct {
	*fakeScorer
	scoredOrds map[int]int
}

func newTrackingScorer(scores []float32) *trackingScorer {
	return &trackingScorer{
		fakeScorer: &fakeScorer{scores: scores},
		scoredOrds: make(map[int]int, len(scores)),
	}
}

func (t *trackingScorer) Score(node int) (float32, error) {
	t.scoredOrds[node]++
	return t.fakeScorer.Score(node)
}

func (t *trackingScorer) BulkScore(nodes []int, out []float32, n int) (float32, error) {
	for i := 0; i < n; i++ {
		t.scoredOrds[nodes[i]]++
	}
	return t.fakeScorer.BulkScore(nodes, out, n)
}

// densePathGraph builds a graph where every node is connected to a
// large neighbourhood (`degree` neighbours each, wrapping by index).
// Combined with a sparse accept-ords filter this exposes the fan-out
// behaviour of the filtered searcher: a candidate's local neighbours
// will mostly be rejected, forcing expansion into neighbours-of-
// neighbours. degree must be even.
func densePathGraph(t *testing.T, size, degree int) *OnHeapHnswGraph {
	t.Helper()
	if degree%2 != 0 {
		t.Fatalf("densePathGraph: degree must be even, got %d", degree)
	}
	g := NewOnHeapHnswGraph(degree, size)
	for i := 0; i < size; i++ {
		g.AddNode(0, i)
	}
	for i := 0; i < size; i++ {
		neigh := g.GetNeighbors(0, i)
		for d := 1; d <= degree/2; d++ {
			if i-d >= 0 {
				neigh.AddOutOfOrder(i-d, 0)
			}
			if i+d < size {
				neigh.AddOutOfOrder(i+d, 0)
			}
		}
	}
	g.TrySetNewEntryNode(0, 0)
	return g
}

// runFilteredSearch builds a FilteredHnswGraphSearcher and runs it
// against a TopKnnCollector; returns the collected TopDocs. Used by
// the dispatch tests and the correctness tests below.
func runFilteredSearch(
	t *testing.T,
	scorer RandomVectorScorer,
	graph HnswGraph,
	k int,
	acceptOrds util.Bits,
	filterSize int,
	visitLimit int,
	strategy KnnSearchStrategy,
) *TopDocs {
	t.Helper()
	collector := NewTopKnnCollector(k, visitLimit, strategy)
	s := NewFilteredHnswGraphSearcher(k, graph, filterSize, acceptOrds)
	if err := Search(s, collector, scorer, graph, acceptOrds); err != nil {
		t.Fatalf("Search: %v", err)
	}
	return collector.TopDocs()
}

// TestFilteredHnswGraphSearcher_BasicAcceptFilter exercises the
// happy path on a small graph with a moderately sparse filter:
// only odd ords are accepted; the result must contain only odd ords
// and must be score-descending. This is the canonical correctness
// check for the filtered path.
func TestFilteredHnswGraphSearcher_BasicAcceptFilter(t *testing.T) {
	const size = 12
	g := densePathGraph(t, size, 6)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}

	acceptOrds, err := util.NewFixedBitSet(size)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	filterSize := 0
	for i := 1; i < size; i += 2 {
		acceptOrds.Set(i)
		filterSize++
	}

	docs := runFilteredSearch(t, scorer, g, 3, acceptOrds, filterSize, math.MaxInt32, nil)
	if len(docs.ScoreDocs) != 3 {
		t.Fatalf("got %d results, want 3", len(docs.ScoreDocs))
	}

	// Every returned ord must be accepted.
	for i, sd := range docs.ScoreDocs {
		if !acceptOrds.Get(sd.Doc) {
			t.Errorf("result[%d]: ord %d is not accepted", i, sd.Doc)
		}
	}
	// And the order must be score-descending.
	sorted := sort.SliceIsSorted(docs.ScoreDocs, func(i, j int) bool {
		return docs.ScoreDocs[i].Score > docs.ScoreDocs[j].Score
	})
	if !sorted {
		t.Errorf("results are not score-descending: %+v", docs.ScoreDocs)
	}
	// Top accepted ords are 11, 9, 7.
	want := []int{11, 9, 7}
	for i, sd := range docs.ScoreDocs {
		if sd.Doc != want[i] {
			t.Errorf("result[%d]: got %d, want %d", i, sd.Doc, want[i])
		}
	}
}

// TestFilteredHnswGraphSearcher_DenseFilter exercises the regime in
// which most ordinals pass the filter: the expansion path should
// rarely fire because filteredAmount stays low. We still expect the
// search to return the top-k by score.
func TestFilteredHnswGraphSearcher_DenseFilter(t *testing.T) {
	const size = 10
	g := densePathGraph(t, size, 4)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}

	acceptOrds, err := util.NewFixedBitSet(size)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	// Reject only ord 5; accept the other 9.
	filterSize := 0
	for i := 0; i < size; i++ {
		if i == 5 {
			continue
		}
		acceptOrds.Set(i)
		filterSize++
	}

	docs := runFilteredSearch(t, scorer, g, 5, acceptOrds, filterSize, math.MaxInt32, nil)
	if len(docs.ScoreDocs) != 5 {
		t.Fatalf("got %d results, want 5", len(docs.ScoreDocs))
	}
	for i, sd := range docs.ScoreDocs {
		if sd.Doc == 5 {
			t.Errorf("result[%d]: rejected ord 5 leaked into the result set", i)
		}
		if !acceptOrds.Get(sd.Doc) {
			t.Errorf("result[%d]: ord %d not accepted", i, sd.Doc)
		}
	}
	// Top-5 accepted ords are 9, 8, 7, 6, 4 (5 is filtered out).
	want := []int{9, 8, 7, 6, 4}
	for i, sd := range docs.ScoreDocs {
		if sd.Doc != want[i] {
			t.Errorf("result[%d]: got %d, want %d", i, sd.Doc, want[i])
		}
	}
}

// TestFilteredHnswGraphSearcher_SparseFilter covers the regime
// where only a small fraction of ordinals pass: the filtered searcher
// must still be able to find an accepted neighbour by expanding into
// the rejected neighbourhood (the ACORN-1 path).
func TestFilteredHnswGraphSearcher_SparseFilter(t *testing.T) {
	const size = 16
	g := densePathGraph(t, size, 4)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}

	acceptOrds, err := util.NewFixedBitSet(size)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	// Accept ords {7, 14}: two non-adjacent accepted points. With
	// degree 4 the searcher must hop through the rejected
	// neighbourhood to reach them from entry 0.
	acceptOrds.Set(7)
	acceptOrds.Set(14)
	filterSize := 2

	docs := runFilteredSearch(t, scorer, g, 2, acceptOrds, filterSize, math.MaxInt32, nil)
	if len(docs.ScoreDocs) > 2 {
		t.Fatalf("got %d results, want at most 2", len(docs.ScoreDocs))
	}
	for _, sd := range docs.ScoreDocs {
		if !acceptOrds.Get(sd.Doc) {
			t.Errorf("ord %d in result is not accepted", sd.Doc)
		}
	}
	// The exact reachability of ords 7 and 14 from entry 0 depends
	// on the graph topology; we only assert that whatever the
	// searcher collected is in the accepted set and score-descending.
	sorted := sort.SliceIsSorted(docs.ScoreDocs, func(i, j int) bool {
		return docs.ScoreDocs[i].Score > docs.ScoreDocs[j].Score
	})
	if !sorted {
		t.Errorf("results are not score-descending: %+v", docs.ScoreDocs)
	}
}

// TestFilteredHnswGraphSearcher_OnlyScoresAccepted asserts the
// filtered searcher never invokes the scorer on a rejected ord (the
// whole point of the optimisation). The scorer is wrapped in a
// trackingScorer that records every Score / BulkScore call.
func TestFilteredHnswGraphSearcher_OnlyScoresAccepted(t *testing.T) {
	const size = 12
	g := densePathGraph(t, size, 4)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}

	acceptOrds, err := util.NewFixedBitSet(size)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	// Accept only ords {2, 5, 9}: a mix of low- and high-scoring
	// accepted points scattered through the graph.
	acceptOrds.Set(2)
	acceptOrds.Set(5)
	acceptOrds.Set(9)
	filterSize := 3

	scorer := newTrackingScorer(scores)
	collector := NewTopKnnCollector(3, math.MaxInt32, nil)
	s := NewFilteredHnswGraphSearcher(3, g, filterSize, acceptOrds)
	if err := Search(s, collector, scorer, g, acceptOrds); err != nil {
		t.Fatalf("Search: %v", err)
	}

	// FindBestEntryPoint may legally score the entry node (ord 0),
	// even if it is not accepted: the descent is the regular
	// (inherited) path and uses the scorer on each candidate it
	// walks past on the upper levels. On this single-level graph,
	// however, the descent loop is degenerate and only scores the
	// entry node. After that, the filtered SearchLevel must score
	// only accepted ords. We assert exactly that: every scored ord
	// is either the entry node (0) or is accepted.
	for ord := range scorer.scoredOrds {
		if ord == 0 {
			continue
		}
		if !acceptOrds.Get(ord) {
			t.Errorf("scorer was called on rejected ord %d", ord)
		}
	}
}

// TestFilteredHnswGraphSearcher_ConstructorValidation covers the
// constructor's input contract: nil acceptOrds, filterSize <= 0, and
// filterSize >= graph.Size() must each panic.
func TestFilteredHnswGraphSearcher_ConstructorValidation(t *testing.T) {
	g := densePathGraph(t, 10, 4)
	acceptOrds, err := util.NewFixedBitSet(10)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 0; i < 10; i += 2 {
		acceptOrds.Set(i)
	}

	tests := []struct {
		name       string
		acceptOrds util.Bits
		filterSize int
	}{
		{"nil acceptOrds", nil, 5},
		{"filterSize zero", acceptOrds, 0},
		{"filterSize negative", acceptOrds, -1},
		{"filterSize == graph size", acceptOrds, 10},
		{"filterSize > graph size", acceptOrds, 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("NewFilteredHnswGraphSearcher did not panic")
				}
			}()
			_ = NewFilteredHnswGraphSearcher(3, g, tc.filterSize, tc.acceptOrds)
		})
	}
}

// TestFilteredHnswGraphSearcher_UnknownMaxConn covers the case where
// the graph cannot expose MaxConn: filtered search depends on it for
// sizing the per-iteration scratch queues, so the constructor must
// reject the graph.
func TestFilteredHnswGraphSearcher_UnknownMaxConn(t *testing.T) {
	// emptyHnswGraph in this package reports UnknownMaxConn. Any
	// other implementation with MaxConn() == UnknownMaxConn would
	// trigger the same rejection.
	g := Empty()
	// The empty graph's size is 0; filterSize must therefore be in
	// (0, 0) which is already impossible, so we'd panic on the size
	// check before reaching MaxConn. Build a tiny non-empty graph
	// whose MaxConn is overridden via the unknownMaxConnGraph
	// helper below.
	_ = g

	wrap := &unknownMaxConnGraph{inner: densePathGraph(t, 6, 4)}
	acceptOrds, err := util.NewFixedBitSet(6)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	acceptOrds.Set(0)
	acceptOrds.Set(2)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewFilteredHnswGraphSearcher did not panic on UnknownMaxConn")
		}
	}()
	_ = NewFilteredHnswGraphSearcher(2, wrap, 2, acceptOrds)
}

// unknownMaxConnGraph forwards every HnswGraph method to its inner
// graph except MaxConn, which it overrides to return UnknownMaxConn.
type unknownMaxConnGraph struct{ inner HnswGraph }

func (u *unknownMaxConnGraph) SeekLevel(level, target int) error {
	return u.inner.SeekLevel(level, target)
}
func (u *unknownMaxConnGraph) Size() int                  { return u.inner.Size() }
func (u *unknownMaxConnGraph) NextNeighbor() (int, error) { return u.inner.NextNeighbor() }
func (u *unknownMaxConnGraph) NumLevels() (int, error)    { return u.inner.NumLevels() }
func (u *unknownMaxConnGraph) MaxConn() int               { return UnknownMaxConn }
func (u *unknownMaxConnGraph) EntryNode() (int, error)    { return u.inner.EntryNode() }
func (u *unknownMaxConnGraph) NeighborCount() int         { return u.inner.NeighborCount() }
func (u *unknownMaxConnGraph) GetNodesOnLevel(level int) (NodesIterator, error) {
	return u.inner.GetNodesOnLevel(level)
}
func (u *unknownMaxConnGraph) MaxNodeID() int { return u.inner.MaxNodeID() }

// TestFilteredHnswGraphSearcher_VisitLimit verifies the filtered
// searcher honours the collector's visit budget and reports an
// approximate total when early-terminated.
func TestFilteredHnswGraphSearcher_VisitLimit(t *testing.T) {
	const size = 16
	g := densePathGraph(t, size, 4)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.05
	}
	scorer := &fakeScorer{scores: scores}

	acceptOrds, err := util.NewFixedBitSet(size)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 1; i < size; i += 2 {
		acceptOrds.Set(i)
	}
	filterSize := size / 2

	docs := runFilteredSearch(t, scorer, g, 5, acceptOrds, filterSize, 3, nil)
	if docs.TotalHits.Relation != GreaterThanOrEqualTo {
		t.Errorf("relation = %v, want GreaterThanOrEqualTo", docs.TotalHits.Relation)
	}
}

// TestFilteredHnswGraphSearcher_DispatchThreshold covers the dispatch
// integration: SearchWithCollectorAndFilter must choose the filtered
// path when the HnswStrategy says so for the given filter ratio, and
// must fall back to the regular path otherwise.
//
// We can't easily observe which path was taken without instrumenting
// the searchers, so we compare results between the two thresholds:
// the regular path uses the inherited SearchLevel which checks
// acceptOrds at COLLECTION time, and may therefore visit (and waste
// scoring effort on) rejected ords. The filtered path checks
// acceptOrds at TRAVERSAL time, so the visited count reported by the
// collector should be different in the two regimes.
//
// To keep the assertion robust we just check that both regimes
// return the same top-k by score, which is the contract: the two
// paths must agree on the result set (their performance differs, not
// their correctness).
func TestFilteredHnswGraphSearcher_DispatchThreshold(t *testing.T) {
	const size = 16
	g := densePathGraph(t, size, 4)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}

	acceptOrds, err := util.NewFixedBitSet(size)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 1; i < size; i += 2 {
		acceptOrds.Set(i)
	}
	filterSize := size / 2

	// Threshold 0 → never use filtered (regular path).
	regularCollector := NewTopKnnCollector(3, math.MaxInt32, NewHnswStrategy(0))
	if err := SearchWithCollectorAndFilter(scorer, regularCollector, g, acceptOrds, filterSize); err != nil {
		t.Fatalf("SearchWithCollectorAndFilter (regular): %v", err)
	}
	regular := regularCollector.TopDocs()

	// Threshold 100 → always use filtered.
	filteredCollector := NewTopKnnCollector(3, math.MaxInt32, NewHnswStrategy(100))
	if err := SearchWithCollectorAndFilter(scorer, filteredCollector, g, acceptOrds, filterSize); err != nil {
		t.Fatalf("SearchWithCollectorAndFilter (filtered): %v", err)
	}
	filtered := filteredCollector.TopDocs()

	if len(regular.ScoreDocs) != len(filtered.ScoreDocs) {
		t.Fatalf("regular and filtered returned different sizes: %d vs %d",
			len(regular.ScoreDocs), len(filtered.ScoreDocs))
	}
	for i := range regular.ScoreDocs {
		if regular.ScoreDocs[i].Doc != filtered.ScoreDocs[i].Doc {
			t.Errorf("result[%d]: regular=%d, filtered=%d",
				i, regular.ScoreDocs[i].Doc, filtered.ScoreDocs[i].Doc)
		}
	}
}

// TestFilteredHnswGraphSearcher_DispatchNoStrategy verifies that when
// the collector exposes no HnswStrategy, the dispatch falls back to
// the regular path even with non-nil acceptOrds and a positive
// filteredDocCount. The default strategy has filteredSearchThreshold ==
// 0, so the filtered branch is never taken.
func TestFilteredHnswGraphSearcher_DispatchNoStrategy(t *testing.T) {
	const size = 10
	g := linearGraph(t, size)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}

	acceptOrds, err := util.NewFixedBitSet(size)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 0; i < size; i += 2 {
		acceptOrds.Set(i)
	}
	filterSize := size / 2

	collector := NewTopKnnCollector(3, math.MaxInt32, nil)
	if err := SearchWithCollectorAndFilter(scorer, collector, g, acceptOrds, filterSize); err != nil {
		t.Fatalf("SearchWithCollectorAndFilter: %v", err)
	}
	docs := collector.TopDocs()
	if len(docs.ScoreDocs) != 3 {
		t.Fatalf("got %d results, want 3", len(docs.ScoreDocs))
	}
	// Top accepted ords: 8, 6, 4.
	want := []int{8, 6, 4}
	for i, sd := range docs.ScoreDocs {
		if sd.Doc != want[i] {
			t.Errorf("result[%d]: got doc %d, want %d", i, sd.Doc, want[i])
		}
	}
}

// TestIntArrayQueue covers the small fixed-capacity FIFO used by the
// filtered search loop: it must hand out elements in insertion order,
// report capacity/count accurately, panic on add-past-capacity, and
// return NO_MORE_DOCS on poll-from-empty.
func TestIntArrayQueue(t *testing.T) {
	q := newIntArrayQueue(3)
	if q.capacity() != 3 {
		t.Errorf("capacity() = %d, want 3", q.capacity())
	}
	if q.count() != 0 {
		t.Errorf("count() = %d, want 0", q.count())
	}
	if q.isFull() {
		t.Errorf("isFull() = true on empty queue")
	}
	if got := q.poll(); got != util.NO_MORE_DOCS {
		t.Errorf("poll() on empty queue = %d, want NO_MORE_DOCS", got)
	}

	q.add(10)
	q.add(20)
	q.add(30)
	if !q.isFull() {
		t.Errorf("isFull() = false on full queue")
	}
	if q.count() != 3 {
		t.Errorf("count() = %d, want 3", q.count())
	}

	// add past capacity panics.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("add past capacity did not panic")
			}
		}()
		q.add(40)
	}()

	if got := q.poll(); got != 10 {
		t.Errorf("poll() = %d, want 10", got)
	}
	if got := q.poll(); got != 20 {
		t.Errorf("poll() = %d, want 20", got)
	}
	if q.count() != 1 {
		t.Errorf("count() = %d, want 1 after 2 polls", q.count())
	}
	if got := q.poll(); got != 30 {
		t.Errorf("poll() = %d, want 30", got)
	}
	if got := q.poll(); got != util.NO_MORE_DOCS {
		t.Errorf("poll() on drained queue = %d, want NO_MORE_DOCS", got)
	}

	q.clear()
	if q.count() != 0 || q.isFull() {
		t.Errorf("after clear: count=%d isFull=%v", q.count(), q.isFull())
	}
	q.add(99)
	if got := q.poll(); got != 99 {
		t.Errorf("poll() after clear+add = %d, want 99", got)
	}
}

// TestIntArrayQueue_BadCapacity covers the constructor's contract:
// non-positive capacity must panic.
func TestIntArrayQueue_BadCapacity(t *testing.T) {
	for _, c := range []int{0, -1, -100} {
		c := c
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("newIntArrayQueue(%d) did not panic", c)
				}
			}()
			_ = newIntArrayQueue(c)
		})
	}
}

// TestFilteredBitSet_PicksSparse covers the bitset picker: a low
// expected visitation count should yield a SparseFixedBitSet, a high
// count should yield a FixedBitSet. We use a graph size of 1024 so
// the threshold (size >> 7 == 8) is comfortably above 1 and below 100.
func TestFilteredBitSet_PicksSparse(t *testing.T) {
	bs := filteredBitSetPick(1, 1024)
	if _, ok := bs.(*util.SparseFixedBitSet); !ok {
		t.Errorf("filteredBitSetPick(1, 1024) returned %T, want *SparseFixedBitSet", bs)
	}
	bs = filteredBitSetPick(100, 1024)
	if _, ok := bs.(*util.FixedBitSet); !ok {
		t.Errorf("filteredBitSetPick(100, 1024) returned %T, want *FixedBitSet", bs)
	}
}
