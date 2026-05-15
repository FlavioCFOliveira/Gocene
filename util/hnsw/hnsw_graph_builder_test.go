// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// builderScorer is a 1-D, symmetric, updateable scorer used to exercise
// the builder. The "query" ordinal is set by SetScoringOrdinal; the
// similarity between query Q and another ordinal O is
// 1 / (1 + |coords[Q] - coords[O]|). The score is in (0, 1], maximised
// when both ordinals point at the same coordinate, and monotonically
// decreasing in distance. The metric is symmetric — Score(A → B) ==
// Score(B → A) — which makes diversity checks well-defined.
type builderScorer struct {
	coords []float32
	query  int
}

func newBuilderScorer(coords []float32) *builderScorer {
	return &builderScorer{coords: coords, query: -1}
}

func (s *builderScorer) Score(node int) (float32, error) {
	if s.query < 0 || s.query >= len(s.coords) {
		return 0, errors.New("builderScorer: query ordinal not set")
	}
	if node < 0 || node >= len(s.coords) {
		return float32(math.Inf(-1)), nil
	}
	d := s.coords[s.query] - s.coords[node]
	if d < 0 {
		d = -d
	}
	return 1.0 / (1.0 + d), nil
}

func (s *builderScorer) BulkScore(nodes []int, out []float32, numNodes int) (float32, error) {
	return BulkScoreDefault(s, nodes, out, numNodes)
}

func (s *builderScorer) MaxOrd() int                                  { return len(s.coords) }
func (s *builderScorer) OrdToDoc(ord int) int                         { return ord }
func (s *builderScorer) GetAcceptOrds(acceptDocs util.Bits) util.Bits { return acceptDocs }
func (s *builderScorer) SetScoringOrdinal(node int) error {
	if node < 0 || node >= len(s.coords) {
		return errors.New("builderScorer: ordinal out of range")
	}
	s.query = node
	return nil
}

// builderScorerSupplier hands out builderScorer instances sharing the
// same coords slice. Each Scorer call returns a fresh receiver so the
// builder may hold its own scratch ordinal.
type builderScorerSupplier struct {
	coords []float32
}

func newBuilderScorerSupplier(coords []float32) *builderScorerSupplier {
	return &builderScorerSupplier{coords: coords}
}

func (s *builderScorerSupplier) Scorer() (UpdateableRandomVectorScorer, error) {
	return newBuilderScorer(s.coords), nil
}

func (s *builderScorerSupplier) Copy() (RandomVectorScorerSupplier, error) {
	return &builderScorerSupplier{coords: s.coords}, nil
}

// recordingInfoStream collects messages so tests can assert that the
// builder routed its output through the configured stream.
type recordingInfoStream struct {
	enabled  bool
	messages []string
}

func (r *recordingInfoStream) Message(component, message string) {
	if r.enabled {
		r.messages = append(r.messages, component+": "+message)
	}
}
func (r *recordingInfoStream) IsEnabled(string) bool { return r.enabled }
func (r *recordingInfoStream) Close() error          { return nil }

// erroringScorer returns an error from Score / BulkScore on demand;
// useful for verifying the builder surfaces errors instead of swallowing
// them.
type erroringScorer struct {
	*builderScorer
	failOn int
}

func (e *erroringScorer) Score(node int) (float32, error) {
	if node == e.failOn {
		return 0, errors.New("forced scorer failure")
	}
	return e.builderScorer.Score(node)
}

func (e *erroringScorer) BulkScore(nodes []int, out []float32, numNodes int) (float32, error) {
	return BulkScoreDefault(e, nodes, out, numNodes)
}

// linspace returns n evenly-spaced coordinates in [lo, hi].
func linspace(n int, lo, hi float32) []float32 {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []float32{lo}
	}
	out := make([]float32, n)
	step := (hi - lo) / float32(n-1)
	for i := 0; i < n; i++ {
		out[i] = lo + step*float32(i)
	}
	return out
}

// TestHnswGraphBuilder_NewRejectsBadM verifies that the constructor
// rejects M <= 0 with an error that does not panic.
func TestHnswGraphBuilder_NewRejectsBadM(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	if _, err := NewHnswGraphBuilder(sup, 0, 10, 42); err == nil {
		t.Fatalf("M==0: want error, got nil")
	}
	if _, err := NewHnswGraphBuilder(sup, -1, 10, 42); err == nil {
		t.Fatalf("M==-1: want error, got nil")
	}
}

// TestHnswGraphBuilder_NewRejectsBadBeamWidth verifies that the
// constructor rejects beamWidth <= 0.
func TestHnswGraphBuilder_NewRejectsBadBeamWidth(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	if _, err := NewHnswGraphBuilder(sup, 16, 0, 42); err == nil {
		t.Fatalf("beamWidth==0: want error, got nil")
	}
}

// TestHnswGraphBuilder_NewRejectsNilSupplier verifies that the
// constructor rejects a nil scorer supplier.
func TestHnswGraphBuilder_NewRejectsNilSupplier(t *testing.T) {
	if _, err := NewHnswGraphBuilder(nil, 16, 10, 42); err == nil {
		t.Fatalf("nil supplier: want error, got nil")
	}
}

// TestHnswGraphBuilder_BuildEmpty verifies Build(0) on an empty
// supplier produces an empty graph and freezes the builder.
func TestHnswGraphBuilder_BuildEmpty(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{})
	b, err := NewHnswGraphBuilder(sup, 16, 10, 42)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilder: %v", err)
	}
	g, err := b.Build(0)
	if err != nil {
		t.Fatalf("Build(0): %v", err)
	}
	if g == nil {
		t.Fatalf("Build(0): graph is nil")
	}
	if g.Size() != 0 {
		t.Errorf("size=%d, want 0", g.Size())
	}
	if entry, _ := g.EntryNode(); entry != -1 {
		t.Errorf("entry=%d, want -1", entry)
	}
	// Frozen state: further adds must be rejected.
	if err := b.AddGraphNode(0); err == nil {
		t.Errorf("AddGraphNode after Build: want error, got nil")
	}
}

// TestHnswGraphBuilder_SingleNode verifies that inserting a single
// node lands it as the entry point with no neighbours.
func TestHnswGraphBuilder_SingleNode(t *testing.T) {
	coords := []float32{0.0}
	sup := newBuilderScorerSupplier(coords)
	b, err := NewHnswGraphBuilder(sup, 16, 10, 42)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilder: %v", err)
	}
	if err := b.AddGraphNode(0); err != nil {
		t.Fatalf("AddGraphNode(0): %v", err)
	}
	g, err := b.GetCompletedGraph()
	if err != nil {
		t.Fatalf("GetCompletedGraph: %v", err)
	}
	if g.Size() != 1 {
		t.Errorf("size=%d, want 1", g.Size())
	}
	if entry, _ := g.EntryNode(); entry != 0 {
		t.Errorf("entry=%d, want 0", entry)
	}
	// The single node has no neighbours on level 0.
	if g.GetNeighbors(0, 0).Size() != 0 {
		t.Errorf("neighbours of node 0: size=%d, want 0",
			g.GetNeighbors(0, 0).Size())
	}
}

// TestHnswGraphBuilder_BuildSequentialLinear builds a graph from 20
// nodes spaced linearly along a 1-D line and verifies that:
//   - every node ends up at level 0;
//   - the graph is rooted (every node reachable from the entry);
//   - the neighbour list of each node is bounded by 2*M;
//   - the graph has exactly one connected component on level 0.
func TestHnswGraphBuilder_BuildSequentialLinear(t *testing.T) {
	const n = 20
	coords := linspace(n, 0, 10)
	sup := newBuilderScorerSupplier(coords)
	b, err := NewHnswGraphBuilder(sup, 4, 10, 1)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilder: %v", err)
	}
	g, err := b.Build(n)
	if err != nil {
		t.Fatalf("Build(%d): %v", n, err)
	}
	if g.Size() != n {
		t.Fatalf("size=%d, want %d", g.Size(), n)
	}
	entry, _ := g.EntryNode()
	if entry < 0 || entry >= n {
		t.Fatalf("entry=%d out of range [0,%d)", entry, n)
	}
	levels, _ := g.NumLevels()
	if levels < 1 {
		t.Fatalf("levels=%d, want >=1", levels)
	}
	for node := 0; node < n; node++ {
		if !g.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from level 0", node)
		}
		nbrs := g.GetNeighbors(0, node)
		if nbrs.Size() > 2*b.m {
			t.Errorf("node %d level 0: neighbours=%d > 2*M=%d",
				node, nbrs.Size(), 2*b.m)
		}
	}
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted: %v", err)
	}
	if !rooted {
		t.Errorf("graph not rooted after build")
	}
	sizes, err := ComponentSizes(g, 0)
	if err != nil {
		t.Fatalf("ComponentSizes: %v", err)
	}
	if len(sizes) != 1 || sizes[0] != n {
		t.Errorf("ComponentSizes(level 0)=%v, want [%d]", sizes, n)
	}
}

// TestHnswGraphBuilder_BuildClusterRandom builds a graph from a cluster
// of randomly-placed 1-D points and checks structural invariants only:
// every node present, neighbour count bounded, graph rooted. The exact
// neighbours depend on the level distribution (which we do not pin to
// byte-match Lucene), so the test stays property-based.
func TestHnswGraphBuilder_BuildClusterRandom(t *testing.T) {
	const n = 30
	// Deterministic source so the test is reproducible without
	// chasing Java's SplittableRandom byte stream.
	r := rand.New(rand.NewPCG(7, 13))
	coords := make([]float32, n)
	for i := range coords {
		coords[i] = float32(r.Float64() * 20.0)
	}
	sup := newBuilderScorerSupplier(coords)
	b, err := NewHnswGraphBuilder(sup, 8, 16, 99)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilder: %v", err)
	}
	g, err := b.Build(n)
	if err != nil {
		t.Fatalf("Build(%d): %v", n, err)
	}
	if g.Size() != n {
		t.Fatalf("size=%d, want %d", g.Size(), n)
	}
	for node := 0; node < n; node++ {
		nbrs := g.GetNeighbors(0, node)
		if nbrs.Size() > 2*b.m {
			t.Errorf("node %d level 0: neighbours=%d > 2*M=%d",
				node, nbrs.Size(), 2*b.m)
		}
	}
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted: %v", err)
	}
	if !rooted {
		t.Errorf("graph not rooted after build")
	}
}

// TestHnswGraphBuilder_BuildIsReproducibleWithSameSeed verifies that
// two builders constructed with the same seed produce identical graph
// topologies. This is the stronger correctness property for
// deterministic level selection — even though the level stream itself
// is not Java-byte-compatible, the Go port must be self-consistent.
func TestHnswGraphBuilder_BuildIsReproducibleWithSameSeed(t *testing.T) {
	const n = 15
	coords := linspace(n, 0, 5)
	graphFor := func(seed int64) *OnHeapHnswGraph {
		sup := newBuilderScorerSupplier(coords)
		b, err := NewHnswGraphBuilder(sup, 4, 8, seed)
		if err != nil {
			t.Fatalf("NewHnswGraphBuilder: %v", err)
		}
		g, err := b.Build(n)
		if err != nil {
			t.Fatalf("Build(%d): %v", n, err)
		}
		return g
	}
	a := graphFor(42)
	bGraph := graphFor(42)

	if a.Size() != bGraph.Size() {
		t.Fatalf("size mismatch: %d vs %d", a.Size(), bGraph.Size())
	}
	la, _ := a.NumLevels()
	lb, _ := bGraph.NumLevels()
	if la != lb {
		t.Fatalf("numLevels mismatch: %d vs %d", la, lb)
	}
	for node := 0; node < n; node++ {
		na := a.GetNeighbors(0, node).Nodes()
		nb := bGraph.GetNeighbors(0, node).Nodes()
		if len(na) != len(nb) {
			t.Fatalf("node %d: nbr count %d vs %d", node, len(na), len(nb))
		}
		ca := append([]int(nil), na[:a.GetNeighbors(0, node).Size()]...)
		cb := append([]int(nil), nb[:bGraph.GetNeighbors(0, node).Size()]...)
		sort.Ints(ca)
		sort.Ints(cb)
		for i := range ca {
			if ca[i] != cb[i] {
				t.Fatalf("node %d: nbr[%d] %d vs %d", node, i, ca[i], cb[i])
			}
		}
	}
}

// TestHnswGraphBuilder_RejectAfterCompleted verifies that AddGraphNode
// and AddGraphNodeWithEntryPoints both fail after GetCompletedGraph.
func TestHnswGraphBuilder_RejectAfterCompleted(t *testing.T) {
	coords := linspace(5, 0, 1)
	sup := newBuilderScorerSupplier(coords)
	b, err := NewHnswGraphBuilder(sup, 4, 10, 42)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilder: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := b.AddGraphNode(i); err != nil {
			t.Fatalf("AddGraphNode(%d): %v", i, err)
		}
	}
	if _, err := b.GetCompletedGraph(); err != nil {
		t.Fatalf("GetCompletedGraph: %v", err)
	}
	if err := b.AddGraphNode(3); err == nil {
		t.Errorf("AddGraphNode after completed: want error")
	}
	if err := b.AddGraphNodeWithEntryPoints(4, map[int]struct{}{0: {}}); err == nil {
		t.Errorf("AddGraphNodeWithEntryPoints after completed: want error")
	}
	if _, err := b.Build(5); err == nil {
		t.Errorf("Build after completed: want error")
	}
}

// TestHnswGraphBuilder_CompletedIsIdempotent verifies that calling
// GetCompletedGraph twice returns the same graph reference and does
// not error.
func TestHnswGraphBuilder_CompletedIsIdempotent(t *testing.T) {
	coords := linspace(3, 0, 1)
	sup := newBuilderScorerSupplier(coords)
	b, _ := NewHnswGraphBuilder(sup, 4, 10, 42)
	for i := 0; i < 3; i++ {
		if err := b.AddGraphNode(i); err != nil {
			t.Fatalf("AddGraphNode(%d): %v", i, err)
		}
	}
	g1, err := b.GetCompletedGraph()
	if err != nil {
		t.Fatalf("GetCompletedGraph (1): %v", err)
	}
	g2, err := b.GetCompletedGraph()
	if err != nil {
		t.Fatalf("GetCompletedGraph (2): %v", err)
	}
	if g1 != g2 {
		t.Errorf("GetCompletedGraph not idempotent: g1=%p g2=%p", g1, g2)
	}
}

// TestHnswGraphBuilder_GetGraphReturnsLiveGraph verifies that GetGraph
// returns the same instance that Build / GetCompletedGraph returns.
// This is the contract HnswBuilder advertises.
func TestHnswGraphBuilder_GetGraphReturnsLiveGraph(t *testing.T) {
	sup := newBuilderScorerSupplier(linspace(3, 0, 1))
	b, _ := NewHnswGraphBuilder(sup, 4, 10, 42)
	g1 := b.GetGraph()
	if g1 == nil {
		t.Fatalf("GetGraph(): nil before build")
	}
	for i := 0; i < 3; i++ {
		if err := b.AddGraphNode(i); err != nil {
			t.Fatalf("AddGraphNode(%d): %v", i, err)
		}
	}
	g2, _ := b.GetCompletedGraph()
	if g1 != g2 {
		t.Errorf("GetGraph/Completed mismatch: g1=%p g2=%p", g1, g2)
	}
}

// TestHnswGraphBuilder_SetInfoStream verifies that the builder routes
// progress messages through the configured info stream once it is
// installed.
func TestHnswGraphBuilder_SetInfoStream(t *testing.T) {
	const n = 5
	sup := newBuilderScorerSupplier(linspace(n, 0, 1))
	b, _ := NewHnswGraphBuilder(sup, 4, 10, 42)
	rec := &recordingInfoStream{enabled: true}
	b.SetInfoStream(rec)
	if _, err := b.Build(n); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(rec.messages) == 0 {
		t.Errorf("info stream received no messages")
	}
	foundBuild := false
	foundAdd := false
	for _, m := range rec.messages {
		if containsString(m, "HNSW: build graph") {
			foundBuild = true
		}
		if containsString(m, "HNSW: addVectors") {
			foundAdd = true
		}
	}
	if !foundBuild {
		t.Errorf("expected 'build graph' message; got %v", rec.messages)
	}
	if !foundAdd {
		t.Errorf("expected 'addVectors' message; got %v", rec.messages)
	}
}

// TestHnswGraphBuilder_SetInfoStreamNilFallsBack verifies that passing
// nil to SetInfoStream restores the package default rather than
// nil-panicking on the next Message call.
func TestHnswGraphBuilder_SetInfoStreamNilFallsBack(t *testing.T) {
	sup := newBuilderScorerSupplier(linspace(3, 0, 1))
	b, _ := NewHnswGraphBuilder(sup, 4, 10, 42)
	b.SetInfoStream(nil)
	// Must not panic when the next add fires through Build.
	if _, err := b.Build(3); err != nil {
		t.Fatalf("Build after SetInfoStream(nil): %v", err)
	}
}

// TestHnswGraphBuilder_DisabledStreamDoesNotEmit verifies the
// IsEnabled guard: a stream that reports disabled receives no
// messages even when build runs.
func TestHnswGraphBuilder_DisabledStreamDoesNotEmit(t *testing.T) {
	sup := newBuilderScorerSupplier(linspace(3, 0, 1))
	b, _ := NewHnswGraphBuilder(sup, 4, 10, 42)
	rec := &recordingInfoStream{enabled: false}
	b.SetInfoStream(rec)
	if _, err := b.Build(3); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(rec.messages) != 0 {
		t.Errorf("disabled stream received %d messages", len(rec.messages))
	}
}

// TestHnswGraphBuilder_FromExistingGraph builds atop an OnHeapHnswGraph
// the caller already owns. Mirrors the merge-time entry point.
func TestHnswGraphBuilder_FromExistingGraph(t *testing.T) {
	const n = 8
	coords := linspace(n, 0, 4)
	sup := newBuilderScorerSupplier(coords)
	g := NewOnHeapHnswGraph(4, n)
	b, err := NewHnswGraphBuilderFromGraph(sup, 10, 42, g)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilderFromGraph: %v", err)
	}
	if b.GetGraph() != g {
		t.Errorf("FromGraph: builder did not adopt the supplied graph")
	}
	got, err := b.Build(n)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got != g {
		t.Errorf("Build: returned graph differs from the seeded one")
	}
	if got.Size() != n {
		t.Errorf("size=%d, want %d", got.Size(), n)
	}
}

// TestHnswGraphBuilder_FromExistingGraphRejectsNil verifies a defensive
// nil check on the FromGraph entry point.
func TestHnswGraphBuilder_FromExistingGraphRejectsNil(t *testing.T) {
	sup := newBuilderScorerSupplier(linspace(2, 0, 1))
	if _, err := NewHnswGraphBuilderFromGraph(sup, 10, 42, nil); err == nil {
		t.Errorf("FromGraph(nil): want error")
	}
}

// TestHnswGraphBuilder_AddWithEntryPoints validates the
// AddGraphNodeWithEntryPoints path: when the caller pre-populates the
// level-0 seeds (mirroring MergingHnswGraphBuilder's behaviour) the
// node still lands in the graph and remains reachable.
func TestHnswGraphBuilder_AddWithEntryPoints(t *testing.T) {
	const n = 6
	coords := linspace(n, 0, 5)
	sup := newBuilderScorerSupplier(coords)
	b, err := NewHnswGraphBuilder(sup, 4, 8, 42)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilder: %v", err)
	}
	// First seed the graph with a couple of regular adds, then drop
	// in node 2 with a pinned entry-point set.
	for i := 0; i < 2; i++ {
		if err := b.AddGraphNode(i); err != nil {
			t.Fatalf("AddGraphNode(%d): %v", i, err)
		}
	}
	eps0 := map[int]struct{}{0: {}, 1: {}}
	if err := b.AddGraphNodeWithEntryPoints(2, eps0); err != nil {
		t.Fatalf("AddGraphNodeWithEntryPoints: %v", err)
	}
	// Continue with the remaining regular adds; we just want a
	// well-formed multi-node graph.
	for i := 3; i < n; i++ {
		if err := b.AddGraphNode(i); err != nil {
			t.Fatalf("AddGraphNode(%d): %v", i, err)
		}
	}
	g, err := b.GetCompletedGraph()
	if err != nil {
		t.Fatalf("GetCompletedGraph: %v", err)
	}
	if g.Size() != n {
		t.Fatalf("size=%d, want %d", g.Size(), n)
	}
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted: %v", err)
	}
	if !rooted {
		t.Errorf("graph not rooted after build")
	}
}

// TestHnswGraphBuilder_ScorerErrorPropagates verifies that an error
// produced by the scorer during a build propagates upward instead of
// being swallowed.
func TestHnswGraphBuilder_ScorerErrorPropagates(t *testing.T) {
	const n = 5
	coords := linspace(n, 0, 4)
	sup := &erroringSupplier{coords: coords, failOn: 2}
	b, err := NewHnswGraphBuilder(sup, 4, 10, 42)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilder: %v", err)
	}
	if _, err := b.Build(n); err == nil {
		t.Errorf("Build: expected scorer error, got nil")
	}
}

// erroringSupplier supplies erroringScorer instances configured to
// fail when scoring a specific ordinal.
type erroringSupplier struct {
	coords []float32
	failOn int
}

func (s *erroringSupplier) Scorer() (UpdateableRandomVectorScorer, error) {
	return &erroringScorer{builderScorer: newBuilderScorer(s.coords), failOn: s.failOn}, nil
}
func (s *erroringSupplier) Copy() (RandomVectorScorerSupplier, error) { return s, nil }

// TestPopToScratch verifies that the helper drains a candidate
// collector into a NeighborArray in worst-first ascending order.
func TestPopToScratch(t *testing.T) {
	c := NewGraphBuilderKnnCollector(5)
	c.Collect(0, 0.9)
	c.Collect(1, 0.7)
	c.Collect(2, 0.5)
	c.Collect(3, 0.3)
	c.Collect(4, 0.1)

	scratch := NewNeighborArray(8, false)
	popToScratch(c, scratch)
	if scratch.Size() != 5 {
		t.Fatalf("scratch size=%d, want 5", scratch.Size())
	}
	// The smallest score (0.1) must be at index 0; the largest (0.9)
	// at index 4. NeighborArray with descOrder=false enforces strictly
	// non-decreasing scores.
	want := []float32{0.1, 0.3, 0.5, 0.7, 0.9}
	for i, w := range want {
		if got := scratch.GetScore(i); got != w {
			t.Errorf("scratch[%d].score=%v, want %v", i, got, w)
		}
	}
}

// TestGetRandomGraphLevel sanity-checks the geometric sampler: with
// ml = 1 / ln(M), the expected level is finite, non-negative, and the
// distribution is heavy-tailed. We sample many draws and assert level
// 0 dominates while the maximum stays bounded by what the inverse log
// can produce.
func TestGetRandomGraphLevel(t *testing.T) {
	ml := 1.0 / math.Log(16.0)
	r := rand.New(rand.NewPCG(1, 2))
	const samples = 10000
	levelCounts := make(map[int]int)
	maxLevel := 0
	for i := 0; i < samples; i++ {
		level := getRandomGraphLevel(ml, r)
		if level < 0 {
			t.Fatalf("level %d is negative", level)
		}
		levelCounts[level]++
		if level > maxLevel {
			maxLevel = level
		}
	}
	// Level 0 should dominate: roughly 1 - 1/M = ~93% for M=16.
	level0 := levelCounts[0]
	if level0 < samples*70/100 {
		t.Errorf("level 0 count=%d, want at least 70%% of %d", level0, samples)
	}
	// Sanity ceiling: with 10k draws and ml ~ 0.36 the max level should
	// stay under, say, 10.
	if maxLevel > 10 {
		t.Errorf("maxLevel=%d, want <=10 over %d samples", maxLevel, samples)
	}
}

// TestGraphBuilderKnnCollector covers the inner collector contract:
// Collect, MinimumScore, PopNode, PopUntilNearestKNodes, Clear,
// TopDocs panic.
func TestGraphBuilderKnnCollector(t *testing.T) {
	c := NewGraphBuilderKnnCollector(3)
	if c.K() != 3 {
		t.Errorf("K=%d, want 3", c.K())
	}
	if c.EarlyTerminated() {
		t.Errorf("EarlyTerminated must always be false")
	}
	if c.VisitLimit() != math.MaxInt64 {
		t.Errorf("VisitLimit=%d, want %d", c.VisitLimit(), int64(math.MaxInt64))
	}
	c.IncVisitedCount(5)
	if c.VisitedCount() != 5 {
		t.Errorf("VisitedCount=%d, want 5", c.VisitedCount())
	}

	// Collect items beyond k; the queue must overflow keeping the top
	// k scoring entries.
	c.Collect(0, 0.1)
	c.Collect(1, 0.5)
	c.Collect(2, 0.9)
	c.Collect(3, 0.7)
	c.Collect(4, 0.2)
	if c.Size() != 3 {
		t.Fatalf("size after 5 inserts=%d, want 3", c.Size())
	}
	if c.MinCompetitiveSimilarity() != c.MinimumScore() {
		t.Errorf("MinCompetitiveSimilarity / MinimumScore disagree")
	}
	if c.GetSearchStrategy() != nil {
		t.Errorf("GetSearchStrategy must be nil for builder collector")
	}

	// TopDocs must panic.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("TopDocs: want panic, got none")
			}
		}()
		_ = c.TopDocs()
	}()

	// Clear resets size and visited count.
	c.Clear()
	if c.Size() != 0 {
		t.Errorf("size after Clear=%d, want 0", c.Size())
	}
	if c.VisitedCount() != 0 {
		t.Errorf("VisitedCount after Clear=%d, want 0", c.VisitedCount())
	}
}

// TestGraphBuilderKnnCollector_MinCompetitive verifies that the
// collector reports -Inf before reaching k items and switches to the
// heap top thereafter.
func TestGraphBuilderKnnCollector_MinCompetitive(t *testing.T) {
	c := NewGraphBuilderKnnCollector(3)
	if got := c.MinCompetitiveSimilarity(); got != float32(math.Inf(-1)) {
		t.Errorf("empty: MinCompetitiveSimilarity=%v, want -Inf", got)
	}
	c.Collect(0, 0.5)
	c.Collect(1, 0.8)
	if got := c.MinCompetitiveSimilarity(); got != float32(math.Inf(-1)) {
		t.Errorf("size=2: MinCompetitiveSimilarity=%v, want -Inf", got)
	}
	c.Collect(2, 0.2)
	if got := c.MinCompetitiveSimilarity(); got != 0.2 {
		t.Errorf("size=3: MinCompetitiveSimilarity=%v, want 0.2", got)
	}
}

// TestHnswGraphBuilder_BuildLargerCluster builds a 100-node graph and
// confirms structural invariants. This is the upper end of the
// "small" cluster bracket suggested by the task brief; anything
// larger would push run time beyond the unit-test budget.
func TestHnswGraphBuilder_BuildLargerCluster(t *testing.T) {
	const n = 100
	r := rand.New(rand.NewPCG(11, 17))
	coords := make([]float32, n)
	for i := range coords {
		coords[i] = float32(r.Float64() * 50.0)
	}
	sup := newBuilderScorerSupplier(coords)
	b, err := NewHnswGraphBuilder(sup, 8, 16, 1234)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilder: %v", err)
	}
	g, err := b.Build(n)
	if err != nil {
		t.Fatalf("Build(%d): %v", n, err)
	}
	if g.Size() != n {
		t.Fatalf("size=%d, want %d", g.Size(), n)
	}
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted: %v", err)
	}
	if !rooted {
		t.Errorf("100-node graph not rooted")
	}
}

// containsString is a tiny helper to avoid pulling in strings for a
// single Contains call in the info-stream test.
func containsString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
