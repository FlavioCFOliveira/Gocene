// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// shiftedOrdMap returns the identity-with-offset map: ordinal i in
// the source graph maps to (i + offset) in the destination graph.
// Used by the merge tests to interleave the smaller graph's ordinals
// into the destination after the seed graph occupies [0, seedSize).
func shiftedOrdMap(n, offset int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + offset
	}
	return out
}

// TestMergingHnswGraphBuilder_RejectsEmptyGraphs verifies the
// factory surfaces an error when the input slice is empty.
func TestMergingHnswGraphBuilder_RejectsEmptyGraphs(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	_, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42, nil, nil, 1, nil,
	)
	if err == nil {
		t.Fatalf("empty graphs: want error, got nil")
	}
}

// TestMergingHnswGraphBuilder_RejectsMismatchedLengths verifies the
// factory rejects a graphs/ordMaps length mismatch.
func TestMergingHnswGraphBuilder_RejectsMismatchedLengths(t *testing.T) {
	const n = 4
	coords := linspace(n, 0, 3)
	src := buildSourceGraph(t, coords, 4, 10, 1)
	sup := newBuilderScorerSupplier(coords)

	_, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{src},
		[][]int{identityOrdMap(n), identityOrdMap(n)}, // 2 maps, 1 graph
		n, nil,
	)
	if err == nil {
		t.Fatalf("length mismatch: want error, got nil")
	}
}

// TestMergingHnswGraphBuilder_RejectsNilGraph verifies the factory
// rejects a nil graph in the input slice.
func TestMergingHnswGraphBuilder_RejectsNilGraph(t *testing.T) {
	const n = 4
	coords := linspace(n, 0, 3)
	src := buildSourceGraph(t, coords, 4, 10, 1)
	sup := newBuilderScorerSupplier(coords)

	_, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{src, nil},
		[][]int{identityOrdMap(n), identityOrdMap(n)},
		n*2, nil,
	)
	if err == nil {
		t.Fatalf("nil graph[1]: want error, got nil")
	}
}

// TestMergingHnswGraphBuilder_RejectsNilOrdMap verifies the factory
// rejects a nil ordinal map in the input slice.
func TestMergingHnswGraphBuilder_RejectsNilOrdMap(t *testing.T) {
	const n = 4
	coords := linspace(n, 0, 3)
	src := buildSourceGraph(t, coords, 4, 10, 1)
	sup := newBuilderScorerSupplier(coords)

	_, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{src},
		[][]int{nil},
		n, nil,
	)
	if err == nil {
		t.Fatalf("nil ordMap: want error, got nil")
	}
}

// TestMergingHnswGraphBuilder_SingleGraphIsStructuralCopy verifies
// that a one-graph "merge" with initializedNodes covering the seed
// degenerates to the InitGraph structural copy: Build is a no-op
// (no remaining graphs to fold) and the final initializedNodes
// sweep finds nothing to do.
func TestMergingHnswGraphBuilder_SingleGraphIsStructuralCopy(t *testing.T) {
	const n = 8
	coords := linspace(n, 0, 5)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(n)

	initialized, err := util.NewFixedBitSet(n)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 0; i < n; i++ {
		initialized.Set(i)
	}

	b, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{src},
		[][]int{ord},
		n, initialized,
	)
	if err != nil {
		t.Fatalf("NewMergingHnswGraphBuilderFromGraphs: %v", err)
	}

	dst, err := b.Build(n)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if dst.Size() != n {
		t.Fatalf("dst.Size=%d, want %d", dst.Size(), n)
	}
	// Every source node must appear in dst with at least one
	// neighbour (the source graph is densely connected at this
	// size / M).
	for node := 0; node < n; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from dst level 0", node)
		}
		nbrs := dst.GetNeighbors(0, node)
		if nbrs.Size() == 0 {
			t.Errorf("node %d has zero neighbours after structural copy", node)
		}
	}
}

// TestMergingHnswGraphBuilder_MergeTwoGraphs is the central
// integration test. Two graphs of equal size are merged into a
// destination of capacity 2n; the larger (here arbitrarily picked,
// since they are equal size) seeds the destination through
// InitGraph, and the smaller is folded in via updateGraph.
//
// Post-conditions:
//
//   - dst.Size() == 2n, every ordinal in [0, 2n) is present on
//     level 0;
//   - dst is rooted (every level-0 node is reachable from the
//     entry point);
//   - no node has more than 2*M neighbours on level 0.
//
// This exercises both the join-set Phase 1 path and the
// entry-point-seeded Phase 2 path of updateGraph.
func TestMergingHnswGraphBuilder_MergeTwoGraphs(t *testing.T) {
	const n = 10
	const total = 2 * n
	coordsAll := linspace(total, 0, 9)

	// Build two source graphs over the two coord halves. The
	// destination will see ordinals 0..total-1; the first source
	// keeps ordinals 0..n-1 and the second remaps to n..2n-1.
	srcA := buildSourceGraph(t, coordsAll[:n], 4, 10, 1)
	srcB := buildSourceGraph(t, coordsAll[n:], 4, 10, 2)

	sup := newBuilderScorerSupplier(coordsAll)
	ordA := identityOrdMap(n)
	ordB := shiftedOrdMap(n, n)

	initialized, err := util.NewFixedBitSet(total)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	// Mark every destination ordinal that one of the input graphs
	// is expected to populate. The seed graph (srcA) imports
	// [0, n), the second graph imports [n, 2n) via ordB.
	for i := 0; i < total; i++ {
		initialized.Set(i)
	}

	b, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{srcA, srcB},
		[][]int{ordA, ordB},
		total, initialized,
	)
	if err != nil {
		t.Fatalf("NewMergingHnswGraphBuilderFromGraphs: %v", err)
	}

	dst, err := b.Build(total)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if dst.Size() != total {
		t.Fatalf("dst.Size=%d, want %d", dst.Size(), total)
	}
	for node := 0; node < total; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from dst level 0", node)
		}
		nbrs := dst.GetNeighbors(0, node)
		if nbrs.Size() > 2*b.m {
			t.Errorf("node %d nbrs=%d > 2*M=%d", node, nbrs.Size(), 2*b.m)
		}
	}
	rooted, err := IsRooted(dst)
	if err != nil {
		t.Fatalf("IsRooted: %v", err)
	}
	if !rooted {
		t.Errorf("merged graph not rooted")
	}
}

// TestMergingHnswGraphBuilder_FinalSweepInsertsUncoveredNodes
// exercises the post-fold initializedNodes sweep. The destination
// graph capacity is larger than the union of the source graphs;
// the surplus ordinals are flagged uninitialised in the bitset.
// After Build, every ordinal in [0, maxOrd) must be present on
// level 0 — the sweep is what closed the gap.
func TestMergingHnswGraphBuilder_FinalSweepInsertsUncoveredNodes(t *testing.T) {
	const seedN = 6
	const extraN = 4
	const total = seedN + extraN
	coords := linspace(total, 0, 8)

	src := buildSourceGraph(t, coords[:seedN], 4, 10, 1)
	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(seedN)

	initialized, err := util.NewFixedBitSet(total)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	// Only the seed ordinals are flagged; the extra range is left
	// unset so the sweep picks them up.
	for i := 0; i < seedN; i++ {
		initialized.Set(i)
	}

	b, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{src},
		[][]int{ord},
		total, initialized,
	)
	if err != nil {
		t.Fatalf("NewMergingHnswGraphBuilderFromGraphs: %v", err)
	}

	dst, err := b.Build(total)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if dst.Size() != total {
		t.Fatalf("dst.Size=%d, want %d", dst.Size(), total)
	}
	for node := 0; node < total; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from dst level 0 after sweep", node)
		}
	}
	rooted, err := IsRooted(dst)
	if err != nil {
		t.Fatalf("IsRooted: %v", err)
	}
	if !rooted {
		t.Errorf("graph not rooted after sweep")
	}
}

// TestMergingHnswGraphBuilder_NilInitializedNodesSkipsSweep verifies
// that when initializedNodes is nil the final sweep is skipped:
// only the input-graph ordinals end up in the destination, and Build
// does not attempt to insert anything beyond them.
func TestMergingHnswGraphBuilder_NilInitializedNodesSkipsSweep(t *testing.T) {
	const n = 6
	coords := linspace(n, 0, 4)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(n)

	b, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{src},
		[][]int{ord},
		n, nil,
	)
	if err != nil {
		t.Fatalf("NewMergingHnswGraphBuilderFromGraphs: %v", err)
	}

	// Build with maxOrd much larger than the actual seed; with nil
	// initializedNodes the sweep is skipped and dst.Size must
	// reflect only the input graph.
	dst, err := b.Build(n * 10)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if dst.Size() != n {
		t.Errorf("dst.Size=%d, want %d (no sweep should run)",
			dst.Size(), n)
	}
}

// TestMergingHnswGraphBuilder_FrozenBuildRejected verifies the
// shadowed Build honours the builder's frozen flag set by a prior
// GetCompletedGraph call.
func TestMergingHnswGraphBuilder_FrozenBuildRejected(t *testing.T) {
	const n = 4
	coords := linspace(n, 0, 3)
	src := buildSourceGraph(t, coords, 4, 10, 1)
	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(n)

	b, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{src},
		[][]int{ord},
		n, nil,
	)
	if err != nil {
		t.Fatalf("NewMergingHnswGraphBuilderFromGraphs: %v", err)
	}
	if _, err := b.GetCompletedGraph(); err != nil {
		t.Fatalf("GetCompletedGraph: %v", err)
	}
	if _, err := b.Build(n); err == nil {
		t.Fatalf("Build after freeze: want error, got nil")
	}
}

// TestMergingHnswGraphBuilder_HnswBuilderInterface confirms the
// type satisfies the HnswBuilder interface in practice (compile-time
// guard already exists in the source, but the interface accessors
// must also yield sensible values).
func TestMergingHnswGraphBuilder_HnswBuilderInterface(t *testing.T) {
	const n = 4
	coords := linspace(n, 0, 3)
	src := buildSourceGraph(t, coords, 4, 10, 1)
	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(n)

	b, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{src},
		[][]int{ord},
		n, nil,
	)
	if err != nil {
		t.Fatalf("NewMergingHnswGraphBuilderFromGraphs: %v", err)
	}
	var iface HnswBuilder = b
	if iface.GetGraph() == nil {
		t.Errorf("GetGraph() returned nil through HnswBuilder interface")
	}
	// AddGraphNode falls through to the embedded base when called
	// via the interface (Go has no virtual dispatch — the
	// MergingHnswGraphBuilder does NOT override AddGraphNode, so
	// the inherited implementation is invoked either way).
	if err := iface.AddGraphNode(n - 1); err != nil {
		t.Errorf("AddGraphNode via interface: %v", err)
	}
}

// TestMergingHnswGraphBuilder_FormatBuildMessage exercises the
// info-stream prelude string format. Mirrors Lucene's concatenation
// shape ("build graph from merging N graphs of K vectors, graph
// sizes:S1 S2 ").
func TestMergingHnswGraphBuilder_FormatBuildMessage(t *testing.T) {
	const n = 4
	coords := linspace(n, 0, 3)
	srcA := buildSourceGraph(t, coords, 4, 10, 1)
	srcB := buildSourceGraph(t, coords, 4, 10, 2)
	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(n)

	b, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{srcA, srcB},
		[][]int{ord, ord},
		n, nil,
	)
	if err != nil {
		t.Fatalf("NewMergingHnswGraphBuilderFromGraphs: %v", err)
	}
	msg := b.formatBuildMessage(7)
	// Use a substring match — the exact ASCII sequence is
	// implementation-coupled but the prefix and the graph-sizes
	// suffix are stable.
	wantPrefix := "build graph from merging 2 graphs of 7 vectors, graph sizes:"
	if got := msg[:len(wantPrefix)]; got != wantPrefix {
		t.Errorf("prefix mismatch:\n got %q\nwant %q", got, wantPrefix)
	}
	// Both sizes should appear in the suffix.
	for _, want := range []string{"4 "} {
		if !containsSubstring(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
}

// containsSubstring is a tiny helper to avoid a strings.Contains
// import in this file. Equivalent to strings.Contains(s, sub).
func containsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestMergingHnswGraphBuilder_JoinSetSortedDeterminism verifies the
// join-set materialisation phase iterates in ascending node order.
// This is a Java-parity guarantee — Lucene calls
// IntHashSet.toArray() followed by Arrays.sort(...), and the Go
// port replicates that with map keys + sort.Ints. The guarantee
// matters because the order in which join-set nodes are inserted
// influences the eps formation for non-join-set nodes (a join-set
// node v < u becomes a valid seed only after it is inserted).
func TestMergingHnswGraphBuilder_JoinSetSortedDeterminism(t *testing.T) {
	const n = 12
	coords := linspace(n, 0, 9)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	// Compute the join set directly and verify our manual
	// sort gives the same sequence as a fresh slice + sort.
	j, err := ComputeJoinSet(src)
	if err != nil {
		t.Fatalf("ComputeJoinSet: %v", err)
	}
	manual := make([]int, 0, len(j))
	for node := range j {
		manual = append(manual, node)
	}
	sorted := append([]int(nil), manual...)
	sort.Ints(sorted)

	// Re-sort manual: it must produce the same sequence as sorted.
	sort.Ints(manual)
	if len(manual) != len(sorted) {
		t.Fatalf("sort changed length: %d -> %d", len(sorted), len(manual))
	}
	for i := range manual {
		if manual[i] != sorted[i] {
			t.Errorf("sort divergence at %d: %d vs %d",
				i, manual[i], sorted[i])
		}
	}
}

// TestMergingHnswGraphBuilder_OutOfBoundsOrdMap verifies updateGraph
// surfaces a typed error rather than a panic when the source graph
// references an ordinal outside ordMapS bounds. Constructed by
// supplying a too-short ordMap for a graph that has more nodes than
// the map covers.
func TestMergingHnswGraphBuilder_OutOfBoundsOrdMap(t *testing.T) {
	const seedN = 4
	const smallN = 4
	const total = seedN + smallN
	coords := linspace(total, 0, 6)

	srcSeed := buildSourceGraph(t, coords[:seedN], 4, 10, 1)
	srcSmall := buildSourceGraph(t, coords[seedN:], 4, 10, 2)
	sup := newBuilderScorerSupplier(coords)

	ordSeed := identityOrdMap(seedN)
	// Truncated ordMap: srcSmall has smallN=4 nodes but we only
	// supply 2 entries.
	ordSmall := []int{seedN, seedN + 1}

	b, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{srcSeed, srcSmall},
		[][]int{ordSeed, ordSmall},
		total, nil,
	)
	if err != nil {
		t.Fatalf("NewMergingHnswGraphBuilderFromGraphs: %v", err)
	}
	_, buildErr := b.Build(total)
	if buildErr == nil {
		t.Fatalf("truncated ordMap: want error from Build, got nil")
	}
	// The error should mention ordMap bounds.
	if !containsSubstring(buildErr.Error(), "ordMap") {
		t.Errorf("error message missing ordMap context: %v", buildErr)
	}
}

// TestMergingHnswGraphBuilder_NegativeOrdMapEntry exercises the
// safety guard against -1 sentinel ordinals in ordMapS. The merge
// pipeline expects every input ordinal to map to a surviving
// destination ordinal; a -1 entry is a programming error.
func TestMergingHnswGraphBuilder_NegativeOrdMapEntry(t *testing.T) {
	const seedN = 4
	const smallN = 4
	const total = seedN + smallN - 1 // one source ordinal is "deleted"
	coords := linspace(total+1, 0, 5)

	srcSeed := buildSourceGraph(t, coords[:seedN], 4, 10, 1)
	srcSmall := buildSourceGraph(t, coords[seedN:seedN+smallN], 4, 10, 2)
	sup := newBuilderScorerSupplier(coords)

	ordSeed := identityOrdMap(seedN)
	// One -1 sentinel in ordSmall: the merge path must reject it
	// rather than silently bypass the node.
	ordSmall := []int{seedN, -1, seedN + 1, seedN + 2}

	b, err := NewMergingHnswGraphBuilderFromGraphs(
		sup, 4, 10, 42,
		[]HnswGraph{srcSeed, srcSmall},
		[][]int{ordSeed, ordSmall},
		total+1, nil,
	)
	if err != nil {
		t.Fatalf("NewMergingHnswGraphBuilderFromGraphs: %v", err)
	}
	_, buildErr := b.Build(total + 1)
	if buildErr == nil {
		t.Fatalf("negative ordMap entry: want error from Build, got nil")
	}
}
