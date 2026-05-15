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

// fakeScorer is a deterministic RandomVectorScorer keyed on a slice of
// per-ord scores. Score(ord) returns scores[ord]; the scorer never
// errors. Used in every search test below in lieu of a real vector
// scorer (which would require a port of VectorSimilarityFunction).
type fakeScorer struct {
	scores []float32
}

func (f *fakeScorer) Score(node int) (float32, error) {
	if node < 0 || node >= len(f.scores) {
		return float32(math.Inf(-1)), nil
	}
	return f.scores[node], nil
}

func (f *fakeScorer) BulkScore(nodes []int, out []float32, numNodes int) (float32, error) {
	return BulkScoreDefault(f, nodes, out, numNodes)
}

func (f *fakeScorer) MaxOrd() int                                  { return len(f.scores) }
func (f *fakeScorer) OrdToDoc(ord int) int                         { return ord }
func (f *fakeScorer) GetAcceptOrds(acceptDocs util.Bits) util.Bits { return acceptDocs }

// linearGraph constructs an OnHeapHnswGraph with size nodes, a single
// level (level 0), and an adjacency list where every node is connected
// to its two immediate neighbours (the previous and the next index,
// when they exist). This is the simplest non-trivial topology that
// still requires multi-hop traversal to reach the optimum from a
// non-trivial entry point.
func linearGraph(t *testing.T, size int) *OnHeapHnswGraph {
	t.Helper()
	g := NewOnHeapHnswGraph(2, size)
	for i := 0; i < size; i++ {
		g.AddNode(0, i)
	}
	for i := 0; i < size; i++ {
		neigh := g.GetNeighbors(0, i)
		if i > 0 {
			neigh.AddOutOfOrder(i-1, 0)
		}
		if i < size-1 {
			neigh.AddOutOfOrder(i+1, 0)
		}
	}
	g.TrySetNewEntryNode(0, 0)
	return g
}

// twoLevelGraph constructs a two-level OnHeapHnswGraph: nodes 0..size-1
// on level 0 connected in a star around node 0, and only node 0 on
// level 1 (the apex). The apex points back to every other node on
// level 0 in its level-0 neighbour list — exercising the multi-level
// descent code path even though the upper level is degenerate.
func twoLevelGraph(t *testing.T, size int) *OnHeapHnswGraph {
	t.Helper()
	g := NewOnHeapHnswGraph(size, size)
	// Top-level (1) is added before level 0 (Java contract: top first).
	g.AddNode(1, 0)
	g.AddNode(0, 0)
	// Other nodes are level-0 only.
	for i := 1; i < size; i++ {
		g.AddNode(0, i)
	}
	// Level 0: node 0 is connected to every other node; every other
	// node is connected back to node 0 and to its index-neighbours.
	hub := g.GetNeighbors(0, 0)
	for i := 1; i < size; i++ {
		hub.AddOutOfOrder(i, 0)
		neigh := g.GetNeighbors(0, i)
		neigh.AddOutOfOrder(0, 0)
	}
	// Level 1: only node 0 is present; its neighbour list is empty
	// (it is the apex). FindBestEntryPoint will skip the loop body
	// when no friends exist on the upper level.
	g.TrySetNewEntryNode(0, 1)
	return g
}

// disconnectedGraph constructs a level-0-only graph that has two
// connected components: nodes 0..mid-1 form a star around 0, and
// nodes mid..size-1 form a separate star around mid. The two stars
// are NOT connected — exercising the searcher's behaviour when the
// optimum lies in a component the entry point cannot reach.
func disconnectedGraph(t *testing.T, size, mid int) *OnHeapHnswGraph {
	t.Helper()
	if mid <= 0 || mid >= size {
		t.Fatalf("disconnectedGraph: mid=%d size=%d invalid", mid, size)
	}
	g := NewOnHeapHnswGraph(size, size)
	for i := 0; i < size; i++ {
		g.AddNode(0, i)
	}
	// Star A: 0 -> {1..mid-1}.
	starA := g.GetNeighbors(0, 0)
	for i := 1; i < mid; i++ {
		starA.AddOutOfOrder(i, 0)
		g.GetNeighbors(0, i).AddOutOfOrder(0, 0)
	}
	// Star B: mid -> {mid+1..size-1}.
	starB := g.GetNeighbors(0, mid)
	for i := mid + 1; i < size; i++ {
		starB.AddOutOfOrder(i, 0)
		g.GetNeighbors(0, i).AddOutOfOrder(mid, 0)
	}
	g.TrySetNewEntryNode(0, 0)
	return g
}

// TestHnswGraphSearcher_EmptyGraph covers the empty-graph short
// circuit: an OnHeapHnswGraph with no entry node must return no
// results without panicking.
func TestHnswGraphSearcher_EmptyGraph(t *testing.T) {
	g := NewOnHeapHnswGraph(4, -1)
	scorer := &fakeScorer{scores: []float32{}}
	got, err := SearchWithOnHeapGraph(scorer, 5, g, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if len(topDocs.ScoreDocs) != 0 {
		t.Fatalf("got %d results, want 0", len(topDocs.ScoreDocs))
	}
	if topDocs.TotalHits.Value != 0 {
		t.Fatalf("got %d visited, want 0", topDocs.TotalHits.Value)
	}
}

// TestHnswGraphSearcher_SingleNode covers the smallest non-empty
// graph: one node, no neighbours. The searcher must collect it and
// return it as the sole result with the scorer's value.
func TestHnswGraphSearcher_SingleNode(t *testing.T) {
	g := NewOnHeapHnswGraph(4, 1)
	g.AddNode(0, 0)
	g.TrySetNewEntryNode(0, 0)

	scorer := &fakeScorer{scores: []float32{0.5}}
	got, err := SearchWithOnHeapGraph(scorer, 5, g, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if len(topDocs.ScoreDocs) != 1 {
		t.Fatalf("got %d results, want 1", len(topDocs.ScoreDocs))
	}
	if topDocs.ScoreDocs[0].Doc != 0 {
		t.Errorf("got doc %d, want 0", topDocs.ScoreDocs[0].Doc)
	}
	if topDocs.ScoreDocs[0].Score != 0.5 {
		t.Errorf("got score %v, want 0.5", topDocs.ScoreDocs[0].Score)
	}
}

// TestHnswGraphSearcher_BasicK1 verifies k=1 on a linear graph: the
// search must traverse from the entry node (0) and converge on the
// highest-scoring node (size-1 in this test setup).
func TestHnswGraphSearcher_BasicK1(t *testing.T) {
	const size = 10
	g := linearGraph(t, size)
	// Increasing scores: 0.0, 0.1, ..., 0.9. The optimum is node 9.
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}
	got, err := SearchWithOnHeapGraph(scorer, 1, g, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if len(topDocs.ScoreDocs) != 1 {
		t.Fatalf("got %d results, want 1", len(topDocs.ScoreDocs))
	}
	if topDocs.ScoreDocs[0].Doc != size-1 {
		t.Errorf("got doc %d, want %d", topDocs.ScoreDocs[0].Doc, size-1)
	}
}

// TestHnswGraphSearcher_BasicK5 verifies k=5 returns the five
// highest-scoring nodes in score-descending order on a linear graph
// of 10 ascending scores.
func TestHnswGraphSearcher_BasicK5(t *testing.T) {
	const size = 10
	g := linearGraph(t, size)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}
	got, err := SearchWithOnHeapGraph(scorer, 5, g, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if len(topDocs.ScoreDocs) != 5 {
		t.Fatalf("got %d results, want 5", len(topDocs.ScoreDocs))
	}
	// score-descending order = nodes 9, 8, 7, 6, 5.
	want := []int{9, 8, 7, 6, 5}
	for i, sd := range topDocs.ScoreDocs {
		if sd.Doc != want[i] {
			t.Errorf("result[%d]: got doc %d, want %d", i, sd.Doc, want[i])
		}
	}
}

// TestHnswGraphSearcher_MultiLevelDescent runs a search on a
// two-level graph where the entry node lives on level 1 and must
// be descended to level 0 before the beam search begins. The
// scoring is set so the optimum is at the far end of the star;
// the descent itself does not change the entry point but the code
// path is exercised.
func TestHnswGraphSearcher_MultiLevelDescent(t *testing.T) {
	const size = 6
	g := twoLevelGraph(t, size)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}
	got, err := SearchWithOnHeapGraph(scorer, 3, g, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if len(topDocs.ScoreDocs) != 3 {
		t.Fatalf("got %d results, want 3", len(topDocs.ScoreDocs))
	}
	want := []int{5, 4, 3}
	for i, sd := range topDocs.ScoreDocs {
		if sd.Doc != want[i] {
			t.Errorf("result[%d]: got doc %d, want %d", i, sd.Doc, want[i])
		}
	}
}

// TestHnswGraphSearcher_DisconnectedComponent verifies the searcher
// is well-behaved when the optimum lies in a component the entry
// point cannot reach: the search returns the best result reachable
// from the entry node, not the global optimum.
func TestHnswGraphSearcher_DisconnectedComponent(t *testing.T) {
	const size = 10
	const mid = 5
	g := disconnectedGraph(t, size, mid)
	// The global optimum is node 9 (in the second component); the
	// reachable optimum from node 0 is node mid-1 == 4 (within
	// component A).
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}
	got, err := SearchWithOnHeapGraph(scorer, 1, g, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if len(topDocs.ScoreDocs) != 1 {
		t.Fatalf("got %d results, want 1", len(topDocs.ScoreDocs))
	}
	// The best reachable node from entry 0 is mid-1 (= 4): the search
	// cannot cross to component B.
	if topDocs.ScoreDocs[0].Doc != mid-1 {
		t.Errorf("got doc %d, want %d (best in reachable component)",
			topDocs.ScoreDocs[0].Doc, mid-1)
	}
}

// TestHnswGraphSearcher_AcceptOrds verifies the acceptOrds filter is
// honoured at collection time: only ordinals whose bit is set may be
// returned, even if other ordinals score higher.
func TestHnswGraphSearcher_AcceptOrds(t *testing.T) {
	const size = 10
	g := linearGraph(t, size)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}

	// Accept only even ordinals.
	acceptOrds, err := util.NewFixedBitSet(size)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 0; i < size; i += 2 {
		acceptOrds.Set(i)
	}

	got, err := SearchWithOnHeapGraph(scorer, 3, g, acceptOrds, math.MaxInt32)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if len(topDocs.ScoreDocs) != 3 {
		t.Fatalf("got %d results, want 3", len(topDocs.ScoreDocs))
	}
	// Top-3 accepted ords are 8, 6, 4.
	want := []int{8, 6, 4}
	for i, sd := range topDocs.ScoreDocs {
		if sd.Doc != want[i] {
			t.Errorf("result[%d]: got doc %d, want %d", i, sd.Doc, want[i])
		}
		if !acceptOrds.Get(sd.Doc) {
			t.Errorf("result[%d] doc %d is not accepted", i, sd.Doc)
		}
	}
}

// TestHnswGraphSearcher_VisitLimit verifies that a tight visit limit
// triggers early termination: the result set is marked as a lower
// bound (GreaterThanOrEqualTo) when EarlyTerminated() fires.
func TestHnswGraphSearcher_VisitLimit(t *testing.T) {
	const size = 20
	g := linearGraph(t, size)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.05
	}
	scorer := &fakeScorer{scores: scores}

	// Visit budget below the graph size forces an early stop.
	got, err := SearchWithOnHeapGraph(scorer, 5, g, nil, 3)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if topDocs.TotalHits.Relation != GreaterThanOrEqualTo {
		t.Errorf("relation = %v, want GreaterThanOrEqualTo (early termination expected)",
			topDocs.TotalHits.Relation)
	}
}

// TestHnswGraphSearcher_MinCompetitiveSimilarity exercises the early
// exit triggered by minCompetitiveSimilarity. After the top-k heap is
// full, candidates whose score is strictly below the heap's worst
// must not displace any kept result.
func TestHnswGraphSearcher_MinCompetitiveSimilarity(t *testing.T) {
	const size = 10
	g := linearGraph(t, size)
	scores := make([]float32, size)
	// Make node 0 a strong local maximum and have scores decrease
	// monotonically: the searcher should converge on the top-3
	// (0, 1, 2) and never explore deeper than necessary.
	for i := range scores {
		scores[i] = 1.0 - float32(i)*0.1
	}
	scorer := &fakeScorer{scores: scores}
	got, err := SearchWithOnHeapGraph(scorer, 3, g, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if len(topDocs.ScoreDocs) != 3 {
		t.Fatalf("got %d results, want 3", len(topDocs.ScoreDocs))
	}
	// Top-3 by score-descending are 0, 1, 2.
	want := []int{0, 1, 2}
	for i, sd := range topDocs.ScoreDocs {
		if sd.Doc != want[i] {
			t.Errorf("result[%d]: got doc %d, want %d", i, sd.Doc, want[i])
		}
	}
}

// TestExpectedVisitedNodes spot-checks the heuristic at boundary
// values: k=0 → 0, graphSize=0 → 0, and a regular case where the
// returned bound is positive and proportional to k * log(graphSize).
func TestExpectedVisitedNodes(t *testing.T) {
	if got := ExpectedVisitedNodes(0, 100); got != 0 {
		t.Errorf("ExpectedVisitedNodes(0, 100) = %d, want 0", got)
	}
	if got := ExpectedVisitedNodes(10, 0); got != 0 {
		t.Errorf("ExpectedVisitedNodes(10, 0) = %d, want 0", got)
	}
	// log(1000) ~= 6.9, k=10 → ~69.
	if got := ExpectedVisitedNodes(10, 1000); got < 60 || got > 80 {
		t.Errorf("ExpectedVisitedNodes(10, 1000) = %d, want in [60, 80]", got)
	}
}

// TestHnswStrategy_UseFilteredSearch covers the threshold predicate
// at the two boundaries Java exercises.
func TestHnswStrategy_UseFilteredSearch(t *testing.T) {
	s := NewHnswStrategy(50) // 50%
	if !s.UseFilteredSearch(0.3) {
		t.Errorf("UseFilteredSearch(0.3) = false at threshold 50, want true")
	}
	if s.UseFilteredSearch(0.5) {
		t.Errorf("UseFilteredSearch(0.5) = true at threshold 50, want false")
	}
}

// TestHnswStrategy_DefaultNeverFiltered ensures the default
// configuration (threshold == 0) never enables the filtered path.
func TestHnswStrategy_DefaultNeverFiltered(t *testing.T) {
	if DefaultHnswStrategy.UseFilteredSearch(0) {
		t.Errorf("DefaultHnswStrategy.UseFilteredSearch(0) = true, want false")
	}
	if DefaultHnswStrategy.UseFilteredSearch(1) {
		t.Errorf("DefaultHnswStrategy.UseFilteredSearch(1) = true, want false")
	}
}

// TestSearchOrderIsScoreDescending sanity-checks that the TopDocs
// drained by TopKnnCollector is monotonically score-descending.
func TestSearchOrderIsScoreDescending(t *testing.T) {
	const size = 10
	g := linearGraph(t, size)
	scores := make([]float32, size)
	for i := range scores {
		scores[i] = float32(i) * 0.1
	}
	scorer := &fakeScorer{scores: scores}
	got, err := SearchWithOnHeapGraph(scorer, size, g, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("SearchWithOnHeapGraph: %v", err)
	}
	topDocs := got.TopDocs()
	if len(topDocs.ScoreDocs) != size {
		t.Fatalf("got %d results, want %d", len(topDocs.ScoreDocs), size)
	}
	sorted := sort.SliceIsSorted(topDocs.ScoreDocs, func(i, j int) bool {
		return topDocs.ScoreDocs[i].Score > topDocs.ScoreDocs[j].Score
	})
	if !sorted {
		t.Errorf("TopDocs are not score-descending: %+v", topDocs.ScoreDocs)
	}
}
