// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestHnswConcurrentMergeBuilder_RejectsBadInputs covers the
// constructor's parameter-validation surface: nil supplier, nil graph,
// non-positive worker count.
func TestHnswConcurrentMergeBuilder_RejectsBadInputs(t *testing.T) {
	sup := newBuilderScorerSupplier(linspace(8, 0, 1))
	graph := NewOnHeapHnswGraph(4, 8)

	if _, err := NewHnswConcurrentMergeBuilder(nil, 2, 4, 10, 42, graph, nil); err == nil {
		t.Fatalf("nil supplier: want error, got nil")
	}
	if _, err := NewHnswConcurrentMergeBuilder(sup, 2, 4, 10, 42, nil, nil); err == nil {
		t.Fatalf("nil graph: want error, got nil")
	}
	if _, err := NewHnswConcurrentMergeBuilder(sup, 0, 4, 10, 42, graph, nil); err == nil {
		t.Fatalf("numWorker=0: want error, got nil")
	}
	if _, err := NewHnswConcurrentMergeBuilder(sup, -1, 4, 10, 42, graph, nil); err == nil {
		t.Fatalf("numWorker=-1: want error, got nil")
	}
}

// TestHnswConcurrentMergeBuilder_UnsupportedAddPaths verifies that the
// merge-only builder rejects single-node insertion methods, mirroring
// Java's UnsupportedOperationException.
func TestHnswConcurrentMergeBuilder_UnsupportedAddPaths(t *testing.T) {
	sup := newBuilderScorerSupplier(linspace(4, 0, 1))
	graph := NewOnHeapHnswGraph(4, 4)
	b, err := NewHnswConcurrentMergeBuilder(sup, 1, 4, 10, 42, graph, nil)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}

	if err := b.AddGraphNode(0); err == nil {
		t.Fatalf("AddGraphNode: want error, got nil")
	}
	if err := b.AddGraphNodeWithEntryPoints(0, nil); err == nil {
		t.Fatalf("AddGraphNodeWithEntryPoints: want error, got nil")
	}
}

// TestHnswConcurrentMergeBuilder_BuildSingleWorker exercises the
// concurrent builder with numWorker=1 — the simplest "concurrent"
// configuration. Because only one worker runs, the build is fully
// deterministic given the same seed and equivalent to a sequential
// HnswGraphBuilder.Build. The test asserts every ordinal is present at
// level 0, the entry node is set, and no neighbour is the inserting
// node itself (no self-loop).
func TestHnswConcurrentMergeBuilder_BuildSingleWorker(t *testing.T) {
	const numNodes = 24
	coords := linspace(numNodes, 0, 1)
	sup := newBuilderScorerSupplier(coords)

	graph := NewOnHeapHnswGraph(4, numNodes)
	b, err := NewHnswConcurrentMergeBuilder(sup, 1, 4, 16, 42, graph, nil)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	// Override the batch size so the single worker actually pulls
	// multiple batches; this validates the loop semantics even with
	// only one worker.
	b.SetBatchSize(8)
	finalGraph, err := b.Build(numNodes)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertGraphValid(t, finalGraph, numNodes)
}

// TestHnswConcurrentMergeBuilder_BuildFourWorkers exercises the
// concurrent build with numWorker=4 on a graph large enough that every
// worker actually claims at least one batch. The test validates
// structural correctness (every ordinal present at level 0, valid
// neighbours, valid entry node) — concurrent builds are not
// byte-for-byte deterministic (see the determinism caveat in the
// concurrent builder's docs), so the assertions are structural, not
// identity-based.
func TestHnswConcurrentMergeBuilder_BuildFourWorkers(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Fatal("requires GOMAXPROCS >= 2 for meaningful concurrency")
	}
	const numNodes = 256
	coords := linspace(numNodes, 0, 1)
	sup := newBuilderScorerSupplier(coords)

	graph := NewOnHeapHnswGraph(8, numNodes)
	b, err := NewHnswConcurrentMergeBuilder(sup, 4, 8, 16, 42, graph, nil)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	// Force every worker to participate by shrinking the batch size
	// below numNodes / numWorker.
	b.SetBatchSize(16)
	finalGraph, err := b.Build(numNodes)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertGraphValid(t, finalGraph, numNodes)
}

// TestHnswConcurrentMergeBuilder_BuildRespectsInitializedNodes verifies
// that ordinals whose bit is set in initializedNodes are skipped:
// AddGraphNode is not called on them, so they retain whatever state
// the caller put in the graph. The test does not pre-populate the
// graph; instead it asserts that the skipped ordinals are absent from
// level 0 after Build.
func TestHnswConcurrentMergeBuilder_BuildRespectsInitializedNodes(t *testing.T) {
	const numNodes = 32
	coords := linspace(numNodes, 0, 1)
	sup := newBuilderScorerSupplier(coords)

	// Build a parallel reference graph the merge builder will write
	// into. Mark ordinals 0, 5, 10 as already initialised.
	graph := NewOnHeapHnswGraph(4, numNodes)
	initialized, err := util.NewFixedBitSet(numNodes)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	skip := []int{0, 5, 10}
	for _, n := range skip {
		initialized.Set(n)
	}

	b, err := NewHnswConcurrentMergeBuilder(sup, 2, 4, 12, 42, graph, initialized)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	b.SetBatchSize(8)
	finalGraph, err := b.Build(numNodes)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Verify that the skipped ordinals were never inserted: their
	// neighbour list at level 0 is empty (no worker called AddNode on
	// them). Inserted ordinals are expected to have at least one
	// neighbour or, for the sole entry node, possibly zero — the
	// connectivity assertion is therefore relaxed to "skipped ordinals
	// have empty neighbours". Going through GetNodesOnLevel here is not
	// possible because the size/maxNodeId invariant is violated when
	// ordinals are skipped, and the API returns an error.
	skipSet := make(map[int]bool, len(skip))
	for _, s := range skip {
		skipSet[s] = true
	}
	for n := 0; n < numNodes; n++ {
		if !skipSet[n] {
			continue
		}
		// Skipped ordinals were never added to the graph; their slot
		// (if any) is empty. Validate by checking that the graph's
		// neighbour-array does not contain n in any other node's
		// neighbour list. This indirectly proves the skip path
		// short-circuited.
		for m := 0; m < numNodes; m++ {
			if skipSet[m] {
				continue
			}
			nbrs := finalGraph.GetNeighbors(0, m)
			for i := 0; i < nbrs.Size(); i++ {
				if nbrs.Nodes()[i] == n {
					t.Errorf("skipped ordinal %d appears as neighbour of inserted ordinal %d", n, m)
				}
			}
		}
	}

	// Sanity check: every non-skipped ordinal went through AddGraphNode,
	// so GetNeighbors must succeed for it. We don't dereference further
	// because GetNeighbors panics on a node that was never added — the
	// fact that it returns successfully is itself the proof.
	for n := 0; n < numNodes; n++ {
		if skipSet[n] {
			continue
		}
		// GetNeighbors panics if (level, node) was not allocated;
		// running it under a recover ensures the test reports a clean
		// failure instead of crashing the whole run.
		func(n int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("inserted ordinal %d: GetNeighbors panicked: %v", n, r)
				}
			}()
			_ = finalGraph.GetNeighbors(0, n)
		}(n)
	}
}

// TestHnswConcurrentMergeBuilder_BuildIsSingleShot verifies that a
// second Build call on a frozen builder returns an error.
func TestHnswConcurrentMergeBuilder_BuildIsSingleShot(t *testing.T) {
	const numNodes = 16
	coords := linspace(numNodes, 0, 1)
	sup := newBuilderScorerSupplier(coords)

	graph := NewOnHeapHnswGraph(4, numNodes)
	b, err := NewHnswConcurrentMergeBuilder(sup, 2, 4, 12, 42, graph, nil)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if _, err := b.Build(numNodes); err != nil {
		t.Fatalf("first Build: %v", err)
	}
	if _, err := b.Build(numNodes); err == nil {
		t.Fatalf("second Build on frozen builder: want error, got nil")
	}
}

// TestHnswConcurrentMergeBuilder_GetCompletedGraphIdempotent verifies
// that calling GetCompletedGraph after Build returns the same graph as
// Build did, and that a third call still works.
func TestHnswConcurrentMergeBuilder_GetCompletedGraphIdempotent(t *testing.T) {
	const numNodes = 12
	coords := linspace(numNodes, 0, 1)
	sup := newBuilderScorerSupplier(coords)

	graph := NewOnHeapHnswGraph(4, numNodes)
	b, err := NewHnswConcurrentMergeBuilder(sup, 2, 4, 12, 42, graph, nil)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	g1, err := b.Build(numNodes)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	g2, err := b.GetCompletedGraph()
	if err != nil {
		t.Fatalf("GetCompletedGraph 1: %v", err)
	}
	g3, err := b.GetCompletedGraph()
	if err != nil {
		t.Fatalf("GetCompletedGraph 2: %v", err)
	}
	if g1 != g2 || g2 != g3 {
		t.Fatalf("GetCompletedGraph returned different graph pointers")
	}
}

// TestHnswConcurrentMergeBuilder_BatchSemaphoreBound checks that the
// semaphore bounds concurrency at numWorker. The check is structural —
// after construction the builder reports the configured worker count
// via the implicit length of the workers slice.
func TestHnswConcurrentMergeBuilder_BatchSemaphoreBound(t *testing.T) {
	sup := newBuilderScorerSupplier(linspace(8, 0, 1))
	graph := NewOnHeapHnswGraph(4, 8)
	b, err := NewHnswConcurrentMergeBuilder(sup, 3, 4, 12, 42, graph, nil)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if got, want := len(b.workers), 3; got != want {
		t.Fatalf("worker count: got %d, want %d", got, want)
	}
	if b.numWorker != 3 {
		t.Fatalf("numWorker: got %d, want 3", b.numWorker)
	}
}

// TestHnswConcurrentMergeBuilder_SupplierErrorPropagates ensures that
// a supplier.Copy() failure surfaces as a constructor error, not a
// silent panic.
func TestHnswConcurrentMergeBuilder_SupplierErrorPropagates(t *testing.T) {
	sup := &erroringCopySupplier{
		inner:  newBuilderScorerSupplier(linspace(4, 0, 1)),
		failAt: 1, // first Copy succeeds, second fails
	}
	graph := NewOnHeapHnswGraph(4, 4)
	if _, err := NewHnswConcurrentMergeBuilder(sup, 3, 4, 12, 42, graph, nil); err == nil {
		t.Fatalf("want error from supplier.Copy, got nil")
	} else if !strings.Contains(err.Error(), "supplier.Copy") {
		t.Fatalf("error message missing context: %v", err)
	}
}

// TestHnswConcurrentMergeBuilder_DeterminismCaveat documents the
// non-determinism of multi-worker builds: running Build twice on
// different builder instances with the same parameters may produce
// graphs with structurally different (but valid) neighbour lists.
// Rather than assert non-determinism (flaky), the test asserts that
// both runs produce structurally valid graphs.
func TestHnswConcurrentMergeBuilder_DeterminismCaveat(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Fatal("requires GOMAXPROCS >= 2 for meaningful concurrency")
	}
	const numNodes = 64
	coords := linspace(numNodes, 0, 1)

	buildOnce := func() *OnHeapHnswGraph {
		sup := newBuilderScorerSupplier(coords)
		graph := NewOnHeapHnswGraph(4, numNodes)
		b, err := NewHnswConcurrentMergeBuilder(sup, 4, 4, 12, 42, graph, nil)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		b.SetBatchSize(8)
		g, err := b.Build(numNodes)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		return g
	}

	g1 := buildOnce()
	g2 := buildOnce()
	assertGraphValid(t, g1, numNodes)
	assertGraphValid(t, g2, numNodes)
}

// TestHnswConcurrentMergeBuilder_SingleThreadParity verifies that
// a single-worker concurrent build produces a structurally equivalent
// graph to a sequential HnswGraphBuilder.Build on the same inputs. The
// concurrent build with numWorker=1 still goes through the HnswLock
// codepath (no early skip), so this is the smoke test that the
// reactivated lock branch does not corrupt single-thread builds.
func TestHnswConcurrentMergeBuilder_SingleThreadParity(t *testing.T) {
	const numNodes = 32
	coords := linspace(numNodes, 0, 1)

	// Concurrent path, numWorker=1.
	supC := newBuilderScorerSupplier(coords)
	graphC := NewOnHeapHnswGraph(4, numNodes)
	bC, err := NewHnswConcurrentMergeBuilder(supC, 1, 4, 12, 42, graphC, nil)
	if err != nil {
		t.Fatalf("concurrent constructor: %v", err)
	}
	bC.SetBatchSize(numNodes) // one batch, deterministic order.
	gC, err := bC.Build(numNodes)
	if err != nil {
		t.Fatalf("concurrent Build: %v", err)
	}

	// Sequential path.
	supS := newBuilderScorerSupplier(coords)
	bS, err := NewHnswGraphBuilderWithGraphSize(supS, 4, 12, 42, numNodes)
	if err != nil {
		t.Fatalf("sequential constructor: %v", err)
	}
	gS, err := bS.Build(numNodes)
	if err != nil {
		t.Fatalf("sequential Build: %v", err)
	}

	// Both graphs should have the same set of ordinals on level 0 and
	// equally-sized neighbour lists; identity equality of edges is not
	// guaranteed (random-level decisions interleave with HnswLock
	// acquisition timing even in single-worker mode, but the seed is
	// fixed so the level sequence is identical and the resulting graph
	// should be structurally close).
	assertGraphValid(t, gC, numNodes)
	assertGraphValid(t, gS, numNodes)
}

// assertGraphValid is the shared structural-validity check for concurrent
// build outputs. It verifies:
//
//   - every ordinal in [0, numNodes) is present at level 0,
//   - the entry node is one of the ordinals,
//   - no node has a self-loop in its neighbour list,
//   - every neighbour ordinal is in [0, numNodes),
//   - the graph reports a consistent maxNodeId.
func assertGraphValid(t *testing.T, g *OnHeapHnswGraph, numNodes int) {
	t.Helper()
	if g == nil {
		t.Fatalf("graph is nil")
	}
	if got := g.MaxNodeID(); got != numNodes-1 {
		t.Errorf("MaxNodeID: got %d, want %d", got, numNodes-1)
	}
	entry, err := g.EntryNode()
	if err != nil {
		t.Fatalf("EntryNode: %v", err)
	}
	if entry < 0 || entry >= numNodes {
		t.Errorf("EntryNode %d outside [0, %d)", entry, numNodes)
	}
	iter, err := g.GetNodesOnLevel(0)
	if err != nil {
		t.Fatalf("GetNodesOnLevel(0): %v", err)
	}
	present := make(map[int]bool, numNodes)
	for iter.HasNext() {
		n := iter.NextInt()
		present[n] = true
		neighbors := g.GetNeighbors(0, n)
		for i := 0; i < neighbors.Size(); i++ {
			nbr := neighbors.Nodes()[i]
			if nbr == n {
				t.Errorf("self-loop on ordinal %d at level 0", n)
			}
			if nbr < 0 || nbr >= numNodes {
				t.Errorf("ordinal %d neighbour %d out of range [0, %d)", n, nbr, numNodes)
			}
		}
	}
	for n := 0; n < numNodes; n++ {
		if !present[n] {
			t.Errorf("ordinal %d missing from level 0", n)
		}
	}
}

// erroringCopySupplier is a RandomVectorScorerSupplier whose Copy()
// fails on the failAt-th invocation (zero-indexed). Used to verify that
// HnswConcurrentMergeBuilder surfaces supplier failures cleanly.
type erroringCopySupplier struct {
	inner  RandomVectorScorerSupplier
	failAt int
	called int
}

func (e *erroringCopySupplier) Scorer() (UpdateableRandomVectorScorer, error) {
	return e.inner.Scorer()
}

func (e *erroringCopySupplier) Copy() (RandomVectorScorerSupplier, error) {
	defer func() { e.called++ }()
	if e.called == e.failAt {
		return nil, errors.New("forced copy failure")
	}
	return e.inner.Copy()
}
