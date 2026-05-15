// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// buildSourceGraph constructs a small Hnsw graph from the supplied
// coordinates using the standard builder. It is the seed graph that
// the InitializedHnswGraphBuilder tests copy structure from.
func buildSourceGraph(t *testing.T, coords []float32, m, beam int, seed int64) *OnHeapHnswGraph {
	t.Helper()
	sup := newBuilderScorerSupplier(coords)
	b, err := NewHnswGraphBuilder(sup, m, beam, seed)
	if err != nil {
		t.Fatalf("NewHnswGraphBuilder: %v", err)
	}
	g, err := b.Build(len(coords))
	if err != nil {
		t.Fatalf("Build(%d): %v", len(coords), err)
	}
	return g
}

// identityOrdMap returns a slice of length n where entry i maps to i.
// Used by the tests that do not exercise deletions.
func identityOrdMap(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// TestInitializedHnswGraphBuilder_RejectsNilInitializer verifies the
// factory surfaces an error when the source graph is nil.
func TestInitializedHnswGraphBuilder_RejectsNilInitializer(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	if _, err := NewInitializedHnswGraphBuilderFromGraph(
		sup, 10, 42, nil, []int{0}, nil, 1,
	); err == nil {
		t.Fatalf("nil initializer graph: want error, got nil")
	}
}

// TestInitializedHnswGraphBuilder_RejectsNilOrdMap verifies the
// factory rejects a nil newOrdMap.
func TestInitializedHnswGraphBuilder_RejectsNilOrdMap(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	src := buildSourceGraph(t, []float32{0.0}, 4, 10, 1)
	if _, err := NewInitializedHnswGraphBuilderFromGraph(
		sup, 10, 42, src, nil, nil, 1,
	); err == nil {
		t.Fatalf("nil newOrdMap: want error, got nil")
	}
}

// TestInitializedHnswGraphBuilder_InitFromSmallGraph builds a small
// source graph, copies it through InitializedHnswGraphBuilder, then
// verifies the destination contains every source node and matches
// the source neighbour set on level 0. No deletions, no new nodes —
// this is the structural-copy baseline.
func TestInitializedHnswGraphBuilder_InitFromSmallGraph(t *testing.T) {
	const n = 8
	coords := linspace(n, 0, 5)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(n)
	b, err := NewInitializedHnswGraphBuilderFromGraph(
		sup, 10, 42, src, ord, nil, n,
	)
	if err != nil {
		t.Fatalf("NewInitializedHnswGraphBuilderFromGraph: %v", err)
	}
	dst := b.GetGraph()
	if dst.Size() != n {
		t.Fatalf("dst.Size=%d, want %d", dst.Size(), n)
	}
	srcLevels, _ := src.NumLevels()
	dstLevels, _ := dst.NumLevels()
	if dstLevels != srcLevels {
		t.Errorf("levels: dst=%d src=%d", dstLevels, srcLevels)
	}
	// Every node must be present at level 0 and carry the same
	// neighbour set as the source.
	for node := 0; node < n; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from dst level 0", node)
		}
		srcNbrs := src.GetNeighbors(0, node)
		dstNbrs := dst.GetNeighbors(0, node)
		if srcNbrs.Size() != dstNbrs.Size() {
			t.Errorf("node %d: src nbrs=%d dst nbrs=%d",
				node, srcNbrs.Size(), dstNbrs.Size())
			continue
		}
		srcSet := make(map[int]struct{}, srcNbrs.Size())
		for i := 0; i < srcNbrs.Size(); i++ {
			srcSet[srcNbrs.Nodes()[i]] = struct{}{}
		}
		for i := 0; i < dstNbrs.Size(); i++ {
			nb := dstNbrs.Nodes()[i]
			if _, ok := srcSet[nb]; !ok {
				t.Errorf("node %d dst nbr %d not in src nbrs", node, nb)
			}
		}
	}
}

// TestInitializedHnswGraphBuilder_AddNewNodes builds a source graph
// of size m, initialises a larger destination graph of capacity 2m,
// then incrementally adds m new ordinals. The post-condition is that
// every node — initialised plus newly added — must be present on
// level 0 with a bounded neighbour count, and the graph must be
// rooted.
func TestInitializedHnswGraphBuilder_AddNewNodes(t *testing.T) {
	const initial = 8
	const total = 16
	coords := linspace(total, 0, 10)
	src := buildSourceGraph(t, coords[:initial], 4, 10, 1)

	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(initial)

	// Track initialised ordinals so the shadowed AddGraphNode skips
	// them on incremental add. The bitset must accommodate every
	// destination ordinal (initial + new).
	initialized, err := util.NewFixedBitSet(total)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 0; i < initial; i++ {
		initialized.Set(i)
	}

	b, err := NewInitializedHnswGraphBuilderFromGraph(
		sup, 10, 42, src, ord, initialized, total,
	)
	if err != nil {
		t.Fatalf("NewInitializedHnswGraphBuilderFromGraph: %v", err)
	}

	// Re-running AddGraphNode on an initialised ordinal must be a
	// no-op; the neighbour list size must not change.
	preNbrSize := b.GetGraph().GetNeighbors(0, 0).Size()
	if err := b.AddGraphNode(0); err != nil {
		t.Fatalf("AddGraphNode(initialised 0): %v", err)
	}
	if got := b.GetGraph().GetNeighbors(0, 0).Size(); got != preNbrSize {
		t.Errorf("AddGraphNode on initialised ordinal mutated neighbours: %d -> %d",
			preNbrSize, got)
	}

	// Now add the remaining nodes — these were not in the source.
	for node := initial; node < total; node++ {
		if err := b.AddGraphNode(node); err != nil {
			t.Fatalf("AddGraphNode(%d): %v", node, err)
		}
	}
	dst, err := b.GetCompletedGraph()
	if err != nil {
		t.Fatalf("GetCompletedGraph: %v", err)
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
		t.Errorf("graph not rooted after incremental adds")
	}
}

// TestInitializedHnswGraphBuilder_BuildShadowsAddNode verifies the
// shadowed Build routes per-node calls through the override. After a
// Build over the full ordinal range, every previously-initialised
// ordinal must have its source neighbour list preserved (i.e. the
// override skipped it); only the new ordinals are linked.
func TestInitializedHnswGraphBuilder_BuildShadowsAddNode(t *testing.T) {
	const initial = 6
	const total = 12
	coords := linspace(total, 0, 8)
	src := buildSourceGraph(t, coords[:initial], 4, 10, 1)

	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(initial)
	initialized, err := util.NewFixedBitSet(total)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 0; i < initial; i++ {
		initialized.Set(i)
	}

	b, err := NewInitializedHnswGraphBuilderFromGraph(
		sup, 10, 42, src, ord, initialized, total,
	)
	if err != nil {
		t.Fatalf("NewInitializedHnswGraphBuilderFromGraph: %v", err)
	}

	// Capture neighbour fingerprints of the initialised ordinals.
	type fp struct {
		nodes []int
	}
	pre := make([]fp, initial)
	for i := 0; i < initial; i++ {
		nbrs := b.GetGraph().GetNeighbors(0, i)
		pre[i] = fp{nodes: append([]int(nil), nbrs.Nodes()[:nbrs.Size()]...)}
	}

	dst, err := b.Build(total)
	if err != nil {
		t.Fatalf("Build(%d): %v", total, err)
	}
	if dst.Size() != total {
		t.Fatalf("dst.Size=%d, want %d", dst.Size(), total)
	}

	// Initialised ordinals: the override's contract is that Build's
	// per-node call is a no-op for them — they do not get a second
	// pass through the search/connect pipeline. Backward edges from
	// newly-inserted nodes may still grow their neighbour list (this
	// is standard HNSW insertion, performed by the new nodes' own
	// AddGraphNode, not by the skipped one). The observable
	// invariant therefore is that the original neighbours all
	// survive and no original neighbour was reordered or evicted by
	// a re-insertion. Any extra entries must be among the new
	// ordinals.
	for i := 0; i < initial; i++ {
		nbrs := dst.GetNeighbors(0, i)
		// All original neighbours must still appear in the post-Build
		// neighbour list.
		postSet := make(map[int]struct{}, nbrs.Size())
		for k := 0; k < nbrs.Size(); k++ {
			postSet[nbrs.Nodes()[k]] = struct{}{}
		}
		for _, want := range pre[i].nodes {
			if _, ok := postSet[want]; !ok {
				t.Errorf("initialised node %d: original nbr %d missing after Build",
					i, want)
			}
		}
		// Any additional neighbours must be from the new range
		// [initial, total).
		preSet := make(map[int]struct{}, len(pre[i].nodes))
		for _, n := range pre[i].nodes {
			preSet[n] = struct{}{}
		}
		for k := 0; k < nbrs.Size(); k++ {
			n := nbrs.Nodes()[k]
			if _, wasOriginal := preSet[n]; wasOriginal {
				continue
			}
			if n < initial {
				t.Errorf("initialised node %d: unexpected new nbr %d (< initial=%d)",
					i, n, initial)
			}
		}
	}

	// New ordinals: must be present on level 0.
	for node := initial; node < total; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("new node %d missing from level 0 after Build", node)
		}
	}
}

// TestInitializedHnswGraphBuilder_AddNodeWithoutInitializedNodes
// verifies that when initializedNodes is nil the shadowed
// AddGraphNode delegates unconditionally to the embedded base — i.e.
// the override imposes no extra constraint beyond the explicit
// initialised-set guard.
func TestInitializedHnswGraphBuilder_AddNodeWithoutInitializedNodes(t *testing.T) {
	const initial = 4
	const total = 8
	coords := linspace(total, 0, 6)
	src := buildSourceGraph(t, coords[:initial], 4, 10, 1)

	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(initial)
	b, err := NewInitializedHnswGraphBuilderFromGraph(
		sup, 10, 42, src, ord, nil, total,
	)
	if err != nil {
		t.Fatalf("NewInitializedHnswGraphBuilderFromGraph: %v", err)
	}

	// Add the new nodes; with no initialisedNodes bitmap, AddGraphNode
	// always delegates to the embedded base.
	for node := initial; node < total; node++ {
		if err := b.AddGraphNode(node); err != nil {
			t.Fatalf("AddGraphNode(%d): %v", node, err)
		}
	}
	dst, err := b.GetCompletedGraph()
	if err != nil {
		t.Fatalf("GetCompletedGraph: %v", err)
	}
	if dst.Size() != total {
		t.Fatalf("dst.Size=%d, want %d", dst.Size(), total)
	}
	for node := 0; node < total; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from level 0", node)
		}
	}
}

// TestInitializedHnswGraphBuilder_DeletedOrdinalsSkipped exercises
// the deletion path: ordinals mapped to -1 in newOrdMap must not be
// copied, and the neighbour count must reflect only the surviving
// ordinals.
func TestInitializedHnswGraphBuilder_DeletedOrdinalsSkipped(t *testing.T) {
	const sourceSize = 10
	coords := linspace(sourceSize, 0, 5)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	// Mark ordinals 2 and 7 as deleted by mapping them to -1; the
	// surviving ordinals collapse into a contiguous [0, 8) range.
	ord := identityOrdMap(sourceSize)
	ord[2] = -1
	ord[7] = -1
	// Renumber survivors to occupy [0, 8). Survivors at positions
	// > 2 shift down by one, those > 7 shift down by another.
	survivors := 0
	for i := range ord {
		if ord[i] == -1 {
			continue
		}
		ord[i] = survivors
		survivors++
	}
	if survivors != sourceSize-2 {
		t.Fatalf("survivors=%d, want %d", survivors, sourceSize-2)
	}

	// Destination coords reflect the survivor positions only.
	destCoords := make([]float32, 0, survivors)
	for i := 0; i < sourceSize; i++ {
		if ord[i] >= 0 {
			destCoords = append(destCoords, coords[i])
		}
	}
	// Sanity: destCoords aligns with ord so scorer queries land on
	// the right vector for each new ordinal.

	sup := newBuilderScorerSupplier(destCoords)
	b, err := NewInitializedHnswGraphBuilderFromGraph(
		sup, 10, 42, src, ord, nil, survivors,
	)
	if err != nil {
		t.Fatalf("NewInitializedHnswGraphBuilderFromGraph: %v", err)
	}
	dst := b.GetGraph()
	if dst.Size() != survivors {
		t.Fatalf("dst.Size=%d, want %d", dst.Size(), survivors)
	}
	for newOrd := 0; newOrd < survivors; newOrd++ {
		if !dst.NodeExistAtLevel(0, newOrd) {
			t.Errorf("survivor %d missing from dst level 0", newOrd)
		}
		nbrs := dst.GetNeighbors(0, newOrd)
		// Every neighbour must be a valid survivor ordinal.
		for i := 0; i < nbrs.Size(); i++ {
			nb := nbrs.Nodes()[i]
			if nb < 0 || nb >= survivors {
				t.Errorf("survivor %d nbr[%d]=%d out of [0,%d)",
					newOrd, i, nb, survivors)
			}
		}
	}
}

// TestInitializedHnswGraphBuilder_InitGraphConvenience exercises the
// InitGraph factory wrapper. It mirrors the structural-copy test but
// goes through the convenience entry point that does not track
// initialised nodes.
func TestInitializedHnswGraphBuilder_InitGraphConvenience(t *testing.T) {
	const n = 6
	coords := linspace(n, 0, 4)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	sup := newBuilderScorerSupplier(coords)
	ord := identityOrdMap(n)
	dst, err := InitGraph(src, ord, n, 10, sup)
	if err != nil {
		t.Fatalf("InitGraph: %v", err)
	}
	if dst.Size() != n {
		t.Fatalf("dst.Size=%d, want %d", dst.Size(), n)
	}
	for i := 0; i < n; i++ {
		if !dst.NodeExistAtLevel(0, i) {
			t.Errorf("node %d missing from dst level 0", i)
		}
	}
}

// TestInitializedHnswGraphBuilder_OutOfBoundsOrdMap verifies the
// factory surfaces an error when the source graph references an
// ordinal beyond newOrdMap's length. The malformed input is the
// caller's bug; we want a typed error rather than a panic.
func TestInitializedHnswGraphBuilder_OutOfBoundsOrdMap(t *testing.T) {
	const n = 4
	coords := linspace(n, 0, 3)
	src := buildSourceGraph(t, coords, 4, 10, 1)
	sup := newBuilderScorerSupplier(coords)
	// newOrdMap shorter than source.Size() — accessing
	// newOrdMap[oldOrd] for oldOrd >= 2 must error.
	ord := []int{0, 1}
	_, err := NewInitializedHnswGraphBuilderFromGraph(
		sup, 10, 42, src, ord, nil, n,
	)
	if err == nil {
		t.Fatalf("short newOrdMap: want error, got nil")
	}
}
