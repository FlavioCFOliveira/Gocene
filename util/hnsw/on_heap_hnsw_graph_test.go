// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Compile-time assertions: OnHeapHnswGraph satisfies both the
// HnswGraph contract and the util.Accountable contract its Java
// counterpart implements.
var (
	_ HnswGraph        = (*OnHeapHnswGraph)(nil)
	_ util.Accountable = (*OnHeapHnswGraph)(nil)
)

// expectPanicContains runs fn and reports a test error unless fn
// panics with a stringified message containing substr. The recovered
// value is normalised via a type switch (string / error / fmt.Stringer)
// before the substring check.
func expectPanicContains(t *testing.T, substr string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic containing %q, got none", substr)
		}
		var msg string
		switch v := r.(type) {
		case string:
			msg = v
		case error:
			msg = v.Error()
		case interface{ String() string }:
			msg = v.String()
		}
		if !strings.Contains(msg, substr) {
			t.Fatalf("panic value %v does not contain %q", r, substr)
		}
	}()
	fn()
}

// TestOnHeapHnswGraph_NoGrowth mirrors Java's testNoGrowth: a graph
// constructed with an explicit numNodes upper bound must reject any
// AddNode whose node ordinal is out of range.
func TestOnHeapHnswGraph_NoGrowth(t *testing.T) {
	g := NewOnHeapHnswGraph(10, 100)
	expectPanicContains(t, "does not expect to grow", func() {
		g.AddNode(1, 100)
	})
}

// TestOnHeapHnswGraph_IncompleteGraphThrow mirrors Java's
// testIncompleteGraphThrow: getNodesOnLevel(0) errors out when size and
// maxNodeID disagree (i.e. there's a hole in the [0, maxNodeID] range).
func TestOnHeapHnswGraph_IncompleteGraphThrow(t *testing.T) {
	g := NewOnHeapHnswGraph(10, -1)
	g.AddNode(1, 0)
	g.AddNode(0, 0)
	it, err := g.GetNodesOnLevel(1)
	if err != nil {
		t.Fatalf("GetNodesOnLevel(1) returned err=%v", err)
	}
	if got := it.Size(); got != 1 {
		t.Errorf("GetNodesOnLevel(1).Size() = %d, want 1", got)
	}
	g.AddNode(0, 5)
	if _, err := g.GetNodesOnLevel(0); err == nil {
		t.Fatalf("GetNodesOnLevel(0) returned nil error, want IllegalStateException")
	} else if !strings.Contains(err.Error(), "graph build not complete") {
		t.Errorf("error %q does not mention incomplete build", err.Error())
	}
}

// addNodeInsertions reproduces the Java helper used inside
// testGraphGrowth / testGraphBuildOutOfOrder: for a given node, picks a
// random top level and inserts the node from that level down to 0,
// promoting the entry node as it goes. Returns the chosen top level.
func addNodeInsertions(g *OnHeapHnswGraph, node, maxLevel int, levelToNodes [][]int, rng *rand.Rand) int {
	level := rng.IntN(maxLevel)
	for l := level; l >= 0; l-- {
		g.AddNode(l, node)
		g.TrySetNewEntryNode(node, l)
		curLevels, _ := g.NumLevels()
		if l > curLevels-1 {
			g.TryPromoteNewEntryNode(node, l, curLevels-1)
		}
		levelToNodes[l] = append(levelToNodes[l], node)
	}
	return level
}

// assertGraphEquals checks that GetNodesOnLevel returns exactly the
// nodes recorded in levelToNodes, in the same order. Mirrors the Java
// reference's assertGraphEquals helper.
func assertGraphEquals(t *testing.T, g *OnHeapHnswGraph, levelToNodes [][]int) {
	t.Helper()
	numLevels, err := g.NumLevels()
	if err != nil {
		t.Fatalf("NumLevels() err=%v", err)
	}
	for l := 0; l < numLevels; l++ {
		it, err := g.GetNodesOnLevel(l)
		if err != nil {
			t.Fatalf("GetNodesOnLevel(%d) err=%v", l, err)
		}
		want := levelToNodes[l]
		if got := it.Size(); got != len(want) {
			t.Errorf("level %d: Size() = %d, want %d", l, got, len(want))
			continue
		}
		idx := 0
		for it.HasNext() {
			got := it.NextInt()
			if idx >= len(want) {
				t.Errorf("level %d: extra element %d", l, got)
				break
			}
			if got != want[idx] {
				t.Errorf("level %d: element %d = %d, want %d", l, idx, got, want[idx])
			}
			idx++
		}
	}
}

// TestOnHeapHnswGraph_GraphGrowth mirrors Java's testGraphGrowth: build
// a 101-node graph with random levels and verify GetNodesOnLevel
// returns each level's nodes in insertion order.
func TestOnHeapHnswGraph_GraphGrowth(t *testing.T) {
	g := NewOnHeapHnswGraph(10, -1)
	maxLevel := 5
	levelToNodes := make([][]int, maxLevel)
	for i := range levelToNodes {
		levelToNodes[i] = make([]int, 0)
	}
	rng := rand.New(rand.NewPCG(0xCA5CADE, 0xDEADBEEF))
	for i := 0; i < 101; i++ {
		addNodeInsertions(g, i, maxLevel, levelToNodes, rng)
	}
	assertGraphEquals(t, g, levelToNodes)
}

// TestOnHeapHnswGraph_GraphBuildOutOfOrder mirrors Java's
// testGraphBuildOutOfOrder: 100 nodes inserted in a shuffled order,
// then the expected level lists are sorted because
// GetNodesOnLevel(level>0) yields nodes in ascending ordinal order.
func TestOnHeapHnswGraph_GraphBuildOutOfOrder(t *testing.T) {
	g := NewOnHeapHnswGraph(10, -1)
	const maxLevel = 5
	const numNodes = 100
	levelToNodes := make([][]int, maxLevel)
	for i := range levelToNodes {
		levelToNodes[i] = make([]int, 0)
	}
	insertions := make([]int, numNodes)
	for i := range insertions {
		insertions[i] = i
	}
	rng := rand.New(rand.NewPCG(0x12345, 0x67890))
	for i := 0; i < 40; i++ {
		p1 := rng.IntN(numNodes)
		p2 := rng.IntN(numNodes)
		insertions[p1], insertions[p2] = insertions[p2], insertions[p1]
	}
	for _, i := range insertions {
		addNodeInsertions(g, i, maxLevel, levelToNodes, rng)
	}
	for i := range levelToNodes {
		sort.Ints(levelToNodes[i])
	}
	assertGraphEquals(t, g, levelToNodes)
}

// TestOnHeapHnswGraph_EntryNodeDefaults verifies the initial entry-node
// state (-1, level 1) matches the Java reference's default.
func TestOnHeapHnswGraph_EntryNodeDefaults(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	en, err := g.EntryNode()
	if err != nil {
		t.Fatalf("EntryNode err=%v", err)
	}
	if en != -1 {
		t.Errorf("EntryNode() = %d, want -1", en)
	}
	nl, err := g.NumLevels()
	if err != nil {
		t.Fatalf("NumLevels err=%v", err)
	}
	if nl != 2 {
		t.Errorf("NumLevels() = %d, want 2", nl)
	}
}

// TestOnHeapHnswGraph_TrySetNewEntryNode covers the CAS contract: the
// first call succeeds, subsequent calls fail until tryPromote rewrites
// the entry.
func TestOnHeapHnswGraph_TrySetNewEntryNode(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	if !g.TrySetNewEntryNode(0, 3) {
		t.Fatalf("first TrySetNewEntryNode returned false")
	}
	en, _ := g.EntryNode()
	if en != 0 {
		t.Errorf("EntryNode after set = %d, want 0", en)
	}
	nl, _ := g.NumLevels()
	if nl != 4 {
		t.Errorf("NumLevels after set = %d, want 4", nl)
	}
	if g.TrySetNewEntryNode(1, 5) {
		t.Errorf("second TrySetNewEntryNode returned true, want false")
	}
	en, _ = g.EntryNode()
	if en != 0 {
		t.Errorf("EntryNode after rejected set = %d, want 0", en)
	}
}

// TestOnHeapHnswGraph_TryPromoteNewEntryNode covers the level-matching
// CAS: a promotion proceeds only when the caller's expectOldLevel
// matches the current entry level.
func TestOnHeapHnswGraph_TryPromoteNewEntryNode(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	g.TrySetNewEntryNode(0, 1)
	// stale expectOldLevel -> false
	if g.TryPromoteNewEntryNode(1, 2, 0) {
		t.Errorf("TryPromoteNewEntryNode(level=2, expect=0) returned true, want false")
	}
	en, _ := g.EntryNode()
	if en != 0 {
		t.Errorf("EntryNode after rejected promote = %d, want 0", en)
	}
	// matching expectOldLevel -> true
	if !g.TryPromoteNewEntryNode(1, 2, 1) {
		t.Errorf("TryPromoteNewEntryNode(level=2, expect=1) returned false, want true")
	}
	en, _ = g.EntryNode()
	if en != 1 {
		t.Errorf("EntryNode after accepted promote = %d, want 1", en)
	}
	nl, _ := g.NumLevels()
	if nl != 3 {
		t.Errorf("NumLevels after accepted promote = %d, want 3", nl)
	}
}

// TestOnHeapHnswGraph_TryPromoteRequiresStrictlyHigherLevel asserts
// Java's `assert level > expectOldLevel` invariant is enforced.
func TestOnHeapHnswGraph_TryPromoteRequiresStrictlyHigherLevel(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	g.TrySetNewEntryNode(0, 1)
	expectPanicContains(t, "TryPromoteNewEntryNode", func() {
		g.TryPromoteNewEntryNode(1, 1, 1)
	})
}

// TestOnHeapHnswGraph_GetNeighborsAndMutation builds a small graph,
// connects nodes through the returned NeighborArray, then iterates them
// via SeekLevel + NextNeighbor.
func TestOnHeapHnswGraph_GetNeighborsAndMutation(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	// Add three nodes at level 0 only.
	for n := 0; n < 3; n++ {
		g.AddNode(0, n)
	}
	// Connect 0 -> 1 -> 2 via NeighborArray. Scores descending.
	g.GetNeighbors(0, 0).AddInOrder(1, 0.9)
	g.GetNeighbors(0, 0).AddInOrder(2, 0.7)
	g.GetNeighbors(0, 1).AddInOrder(2, 0.8)

	if err := g.SeekLevel(0, 0); err != nil {
		t.Fatalf("SeekLevel(0,0) err=%v", err)
	}
	if got := g.NeighborCount(); got != 2 {
		t.Errorf("NeighborCount() = %d, want 2", got)
	}
	wantSeq := []int{1, 2, util.NO_MORE_DOCS}
	for i, want := range wantSeq {
		got, err := g.NextNeighbor()
		if err != nil {
			t.Fatalf("NextNeighbor[%d] err=%v", i, err)
		}
		if got != want {
			t.Errorf("NextNeighbor[%d] = %d, want %d", i, got, want)
		}
	}
}

// TestOnHeapHnswGraph_GetNeighborsPanicsOnUnknownNode verifies the
// panic surface mirrors Java's three assertion sites.
func TestOnHeapHnswGraph_GetNeighborsPanicsOnUnknownNode(t *testing.T) {
	g := NewOnHeapHnswGraph(8, 4)
	g.AddNode(0, 0)
	// Out-of-bounds node.
	expectPanicContains(t, "out of range", func() {
		g.GetNeighbors(0, 100)
	})
	// Existing node but level not allocated.
	expectPanicContains(t, "only", func() {
		g.GetNeighbors(3, 0)
	})
	// Non-existent node within capacity (graph[1] is nil).
	expectPanicContains(t, "not added", func() {
		g.GetNeighbors(0, 1)
	})
}

// TestOnHeapHnswGraph_MaxConn verifies the M parameter is reflected by
// MaxConn() (which mirrors Java's nsize - 1).
func TestOnHeapHnswGraph_MaxConn(t *testing.T) {
	for _, m := range []int{1, 4, 16, 64} {
		g := NewOnHeapHnswGraph(m, -1)
		if got := g.MaxConn(); got != m {
			t.Errorf("MaxConn(M=%d) = %d, want %d", m, got, m)
		}
	}
}

// TestOnHeapHnswGraph_MaxNodeID_NoGrowth verifies the fixed-size
// shortcut: MaxNodeID returns len(graph)-1 regardless of how many
// AddNode calls have arrived.
func TestOnHeapHnswGraph_MaxNodeID_NoGrowth(t *testing.T) {
	g := NewOnHeapHnswGraph(8, 50)
	if got := g.MaxNodeID(); got != 49 {
		t.Errorf("MaxNodeID() = %d, want 49", got)
	}
	g.AddNode(0, 0)
	if got := g.MaxNodeID(); got != 49 {
		t.Errorf("MaxNodeID() after add = %d, want 49", got)
	}
}

// TestOnHeapHnswGraph_MaxNodeID_Growable verifies that the atomic
// counter is honoured when the graph can grow.
func TestOnHeapHnswGraph_MaxNodeID_Growable(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	if got := g.MaxNodeID(); got != -1 {
		t.Errorf("MaxNodeID() before any add = %d, want -1", got)
	}
	g.AddNode(0, 0)
	g.AddNode(0, 7)
	g.AddNode(0, 3)
	if got := g.MaxNodeID(); got != 7 {
		t.Errorf("MaxNodeID() = %d, want 7", got)
	}
}

// TestOnHeapHnswGraph_NodeExistAtLevel covers both happy and missing
// paths of NodeExistAtLevel.
func TestOnHeapHnswGraph_NodeExistAtLevel(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	g.AddNode(2, 0)
	g.AddNode(1, 0)
	g.AddNode(0, 0)
	g.AddNode(0, 1)

	cases := []struct {
		level, node int
		want        bool
	}{
		{0, 0, true},
		{1, 0, true},
		{2, 0, true},
		{3, 0, false},
		{0, 1, true},
		{1, 1, false},
		{0, 2, false},
		{0, 999, false},
	}
	for _, c := range cases {
		if got := g.NodeExistAtLevel(c.level, c.node); got != c.want {
			t.Errorf("NodeExistAtLevel(level=%d, node=%d) = %v, want %v",
				c.level, c.node, got, c.want)
		}
	}
}

// TestOnHeapHnswGraph_SeekLevelResetsCursor checks that re-seeking
// returns the iterator to the start of the new target's neighbour list.
func TestOnHeapHnswGraph_SeekLevelResetsCursor(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	g.AddNode(0, 0)
	g.AddNode(0, 1)
	g.GetNeighbors(0, 0).AddInOrder(1, 0.9)
	g.GetNeighbors(0, 1).AddInOrder(0, 0.9)

	_ = g.SeekLevel(0, 0)
	if got, _ := g.NextNeighbor(); got != 1 {
		t.Errorf("first NextNeighbor (target 0) = %d, want 1", got)
	}
	_ = g.SeekLevel(0, 1)
	if got, _ := g.NextNeighbor(); got != 0 {
		t.Errorf("after reseek NextNeighbor (target 1) = %d, want 0", got)
	}
	if got, _ := g.NextNeighbor(); got != util.NO_MORE_DOCS {
		t.Errorf("post-exhaustion NextNeighbor = %d, want NO_MORE_DOCS", got)
	}
}

// TestOnHeapHnswGraph_NextNeighborBeforeSeek confirms the defensive
// default: with no prior SeekLevel, NextNeighbor returns NO_MORE_DOCS
// and NeighborCount returns 0.
func TestOnHeapHnswGraph_NextNeighborBeforeSeek(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	if got, _ := g.NextNeighbor(); got != util.NO_MORE_DOCS {
		t.Errorf("NextNeighbor before seek = %d, want NO_MORE_DOCS", got)
	}
	if got := g.NeighborCount(); got != 0 {
		t.Errorf("NeighborCount before seek = %d, want 0", got)
	}
}

// TestOnHeapHnswGraph_GetNodesOnLevelZeroBeforeBuildComplete verifies
// that level 0 also rejects the call when there is a hole in the
// ordinal range; mirrors the Java guard for non-zero levels too.
func TestOnHeapHnswGraph_GetNodesOnLevelHoleDetection(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	g.AddNode(0, 0)
	g.AddNode(0, 4) // creates a hole at ordinals 1..3
	if _, err := g.GetNodesOnLevel(0); err == nil {
		t.Fatalf("GetNodesOnLevel(0) returned nil error, want incomplete-build error")
	}
}

// TestOnHeapHnswGraph_TrySetNewEntryNode_Concurrent stresses the CAS:
// many goroutines race to publish their own entry node; exactly one
// must succeed and every other call must observe the published state.
func TestOnHeapHnswGraph_TrySetNewEntryNode_Concurrent(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	const goroutines = 64
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
	)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			if g.TrySetNewEntryNode(i, 1) {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if winners != 1 {
		t.Errorf("winners = %d, want exactly 1", winners)
	}
	en, _ := g.EntryNode()
	if en < 0 || en >= goroutines {
		t.Errorf("EntryNode() = %d, want a value in [0, %d)", en, goroutines)
	}
}

// TestOnHeapHnswGraph_RamBytesUsed_NonZero checks the accountable
// surface: a populated graph reports a positive ram footprint.
func TestOnHeapHnswGraph_RamBytesUsed_NonZero(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	for n := 0; n < 16; n++ {
		g.AddNode(0, n)
	}
	if got := g.RamBytesUsed(); got <= 0 {
		t.Errorf("RamBytesUsed() = %d, want > 0", got)
	}
}

// TestOnHeapHnswGraph_String covers the toString format.
func TestOnHeapHnswGraph_String(t *testing.T) {
	g := NewOnHeapHnswGraph(8, -1)
	g.AddNode(0, 0)
	g.TrySetNewEntryNode(0, 0)
	s := g.String()
	for _, want := range []string{"OnHeapHnswGraph", "size=1", "numLevels=1", "entryNode=0"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, missing %q", s, want)
		}
	}
}

// TestOnHeapHnswGraph_IterationContract reproduces the abstract
// HnswGraph iteration contract: SeekLevel(level, target) followed by
// repeated NextNeighbor until NO_MORE_DOCS is observed.
func TestOnHeapHnswGraph_IterationContract(t *testing.T) {
	g := NewOnHeapHnswGraph(4, -1)
	g.AddNode(1, 0)
	g.AddNode(0, 0)
	g.AddNode(1, 1)
	g.AddNode(0, 1)
	g.AddNode(1, 2)
	g.AddNode(0, 2)

	// Connect node 0 on level 1 to nodes 1 and 2 (descending scores).
	g.GetNeighbors(1, 0).AddInOrder(1, 0.9)
	g.GetNeighbors(1, 0).AddInOrder(2, 0.7)

	if err := g.SeekLevel(1, 0); err != nil {
		t.Fatalf("SeekLevel err=%v", err)
	}
	got := []int{}
	for {
		n, err := g.NextNeighbor()
		if err != nil {
			t.Fatalf("NextNeighbor err=%v", err)
		}
		if n == util.NO_MORE_DOCS {
			break
		}
		got = append(got, n)
	}
	want := []int{1, 2}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("neighbor[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
