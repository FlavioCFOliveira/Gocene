// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// mockHnswGraph is the Go translation of the Java reference's
// TestHnswUtil.MockGraph. The 3-D slice nodes[level][node][neighbour] gives
// the adjacency for each level. A nil nodes[level][node] indicates the
// node is absent from that level (mirrors Java's `int[][][]` with nulls
// for absent entries).
//
// The struct retains seek-iterator state intrinsic to the receiver — every
// SeekLevel call resets currentLevel, currentNode, and currentNeighbor so
// subsequent NextNeighbor() invocations stream the right adjacency list.
type mockHnswGraph struct {
	nodes [][][]int

	currentLevel    int
	currentNode     int
	currentNeighbor int
}

func newMockHnswGraph(nodes [][][]int) *mockHnswGraph {
	return &mockHnswGraph{nodes: nodes}
}

// NextNeighbor implements HnswGraph.NextNeighbor.
func (m *mockHnswGraph) NextNeighbor() (int, error) {
	neighbours := m.nodes[m.currentLevel][m.currentNode]
	if m.currentNeighbor >= len(neighbours) {
		return util.NO_MORE_DOCS, nil
	}
	v := neighbours[m.currentNeighbor]
	m.currentNeighbor++
	return v, nil
}

// SeekLevel implements HnswGraph.SeekLevel. Mirrors the Java assertions in
// the form of t.Fatal-style panics so misuse during tests is loud.
func (m *mockHnswGraph) SeekLevel(level, target int) error {
	if level < 0 || level >= len(m.nodes) {
		panic("mockHnswGraph: level out of range")
	}
	if target < 0 || target >= len(m.nodes[level]) {
		panic("mockHnswGraph: target out of range")
	}
	if m.nodes[level][target] == nil {
		panic("mockHnswGraph: target not present on level")
	}
	m.currentLevel = level
	m.currentNode = target
	m.currentNeighbor = 0
	return nil
}

// Size implements HnswGraph.Size — the graph's size is the level-0 width.
func (m *mockHnswGraph) Size() int { return len(m.nodes[0]) }

// NumLevels implements HnswGraph.NumLevels.
func (m *mockHnswGraph) NumLevels() (int, error) { return len(m.nodes), nil }

// EntryNode implements HnswGraph.EntryNode — entry into the apex level
// is always node 0 in the Java reference's mock.
func (m *mockHnswGraph) EntryNode() (int, error) { return 0, nil }

// NeighborCount implements HnswGraph.NeighborCount.
func (m *mockHnswGraph) NeighborCount() int {
	return len(m.nodes[m.currentLevel][m.currentNode])
}

// MaxConn implements HnswGraph.MaxConn. The Java reference returns
// UNKNOWN_MAX_CONN because the mock graph does not enforce a connection
// limit.
func (m *mockHnswGraph) MaxConn() int { return UnknownMaxConn }

// MaxNodeID implements HnswGraph.MaxNodeID. The mock graph uses
// contiguous ordinals on level 0, so the default Size() - 1 yields
// the inclusive maximum.
func (m *mockHnswGraph) MaxNodeID() int { return m.Size() - 1 }

// GetNodesOnLevel implements HnswGraph.GetNodesOnLevel. The returned
// iterator yields, in ascending order, every node ordinal whose slice on
// the requested level is non-nil. The Java original implements this
// inline; the Go port materialises into a slice up-front because
// NodesIterator does not expose a "skip nil" idiom.
func (m *mockHnswGraph) GetNodesOnLevel(level int) (NodesIterator, error) {
	count := 0
	for i := range m.nodes[level] {
		if m.nodes[level][i] != nil {
			count++
		}
	}
	out := make([]int, 0, count)
	for i := range m.nodes[level] {
		if m.nodes[level][i] != nil {
			out = append(out, i)
		}
	}
	return NewArrayNodesIterator(out), nil
}

// componentSizesLevel0 is a convenience wrapper around [ComponentSizes]
// at level 0 — the Java reference exposes a one-argument
// componentSizes(HnswGraph) overload that the test peer relies on. We
// keep the convenience inside the test file rather than the production
// API to avoid bloating the exported surface.
func componentSizesLevel0(t *testing.T, g HnswGraph) []int {
	t.Helper()
	out, err := ComponentSizes(g, 0)
	if err != nil {
		t.Fatalf("ComponentSizes(level=0) failed: %v", err)
	}
	return out
}

// TestHnswUtil_TreeWithCycle mirrors Java's testTreeWithCycle: a tree-
// shaped graph with one back-edge from a leaf to the root is rooted from
// node 0 and consists of a single component.
func TestHnswUtil_TreeWithCycle(t *testing.T) {
	nodes := [][][]int{
		{
			{1, 2}, // node 0
			{3, 4}, // node 1
			{5, 6}, // node 2
			{}, {}, {}, {0},
		},
	}
	g := newMockHnswGraph(nodes)
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted failed: %v", err)
	}
	if !rooted {
		t.Errorf("IsRooted = false, want true")
	}
	got := componentSizesLevel0(t, g)
	if want := []int{7}; !reflect.DeepEqual(got, want) {
		t.Errorf("ComponentSizes = %v, want %v", got, want)
	}
}

// TestHnswUtil_BackLinking mirrors Java's testBackLinking: a graph whose
// nodes 5 and 6 are unreachable from node 0; three components result
// because the reachable subgraph {0..4} forms one, and 5 and 6 each form
// singleton components.
func TestHnswUtil_BackLinking(t *testing.T) {
	nodes := [][][]int{
		{
			{1, 2}, // node 0
			{3, 4}, // node 1
			{0},    // node 2
			{1}, {1}, {1}, {1},
		},
	}
	g := newMockHnswGraph(nodes)
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted failed: %v", err)
	}
	if rooted {
		t.Errorf("IsRooted = true, want false")
	}
	got := componentSizesLevel0(t, g)
	if want := []int{5, 1, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("ComponentSizes = %v, want %v", got, want)
	}
}

// TestHnswUtil_Chain mirrors Java's testChain: a cyclic chain of four
// nodes is rooted from every node, thus strongly connected and a single
// component.
func TestHnswUtil_Chain(t *testing.T) {
	nodes := [][][]int{{{1}, {2}, {3}, {0}}}
	g := newMockHnswGraph(nodes)
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted failed: %v", err)
	}
	if !rooted {
		t.Errorf("IsRooted = false, want true")
	}
	got := componentSizesLevel0(t, g)
	if want := []int{4}; !reflect.DeepEqual(got, want) {
		t.Errorf("ComponentSizes = %v, want %v", got, want)
	}
}

// TestHnswUtil_TwoChains mirrors Java's testTwoChains: two disjoint
// 2-cycles produce two components of size 2 each, the graph is not
// rooted from node 0.
func TestHnswUtil_TwoChains(t *testing.T) {
	nodes := [][][]int{{{2}, {3}, {0}, {1}}}
	g := newMockHnswGraph(nodes)
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted failed: %v", err)
	}
	if rooted {
		t.Errorf("IsRooted = true, want false")
	}
	got := componentSizesLevel0(t, g)
	if want := []int{2, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("ComponentSizes = %v, want %v", got, want)
	}
}

// TestHnswUtil_Levels mirrors Java's testLevels: three-level graph where
// every level is rooted; ComponentSizes at level 0 reports the entire
// graph as one component.
func TestHnswUtil_Levels(t *testing.T) {
	nodes := [][][]int{
		{{1, 2}, {3}, {0}, {0}},
		{{2}, nil, {0}, nil},
		{{}, nil, nil, nil},
	}
	g := newMockHnswGraph(nodes)
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted failed: %v", err)
	}
	if !rooted {
		t.Errorf("IsRooted = false, want true")
	}
	got := componentSizesLevel0(t, g)
	if want := []int{4}; !reflect.DeepEqual(got, want) {
		t.Errorf("ComponentSizes = %v, want %v", got, want)
	}
}

// TestHnswUtil_LevelsNotRooted mirrors Java's testLevelsNotRooted: a
// two-level graph in which node 2 is unreachable from the entry node;
// IsRooted reports false and the level-0 components are {2, 1}.
func TestHnswUtil_LevelsNotRooted(t *testing.T) {
	nodes := [][][]int{
		{{1}, {0}, {0}},
		{{}, nil, nil},
	}
	g := newMockHnswGraph(nodes)
	rooted, err := IsRooted(g)
	if err != nil {
		t.Fatalf("IsRooted failed: %v", err)
	}
	if rooted {
		t.Errorf("IsRooted = true, want false")
	}
	got := componentSizesLevel0(t, g)
	if want := []int{2, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("ComponentSizes = %v, want %v", got, want)
	}
}

// TestHnswUtil_Random ports Java's testRandom — a randomised cross-check
// of IsRooted against a brute-force reference DFS implemented in-test.
// The random topology mirrors the Java construction: levels = ceil(log(N)),
// upper levels sparsely populated, lower levels denser.
func TestHnswUtil_Random(t *testing.T) {
	// atLeast(10) — Lucene's default scale knob. We pick 16 to give a
	// little extra coverage without slowing the unit test loop down.
	const iterations = 16

	// Deterministic PCG seed: rand/v2 requires explicit construction for
	// reproducibility across runs.
	rng := rand.New(rand.NewPCG(0xC0FFEE, 0xDEAD))

	for it := 0; it < iterations; it++ {
		numNodes := rng.IntN(99) + 1
		numLevels := int(math.Ceil(math.Log(float64(numNodes))))
		if numLevels < 1 {
			numLevels = 1
		}
		nodes := make([][][]int, numLevels)
		for level := numLevels - 1; level >= 0; level-- {
			nodes[level] = make([][]int, numNodes)
			for node := 0; node < numNodes; node++ {
				if level > 0 {
					eligible := false
					if level == numLevels-1 && node > 0 {
						eligible = true
					} else if level < numLevels-1 && nodes[level+1][node] == nil {
						eligible = true
					}
					if eligible {
						// Skip some nodes; the skip probability rises with
						// level so the apex stays sparse. Node 0 is forced
						// present (the entry node) by skipping only when
						// the condition above flags this node as eligible
						// to skip.
						if rng.Float64() > math.Exp(-float64(level)) {
							continue
						}
					}
				}
				numNbrs := rng.IntN((numNodes+7)/8 + 1)
				if level == 0 {
					numNbrs *= 2
				}
				nodes[level][node] = make([]int, numNbrs)
				for nbr := 0; nbr < numNbrs; nbr++ {
					for {
						randomNbr := rng.IntN(numNodes)
						if nodes[level][randomNbr] != nil {
							nodes[level][node][nbr] = randomNbr
							break
						}
					}
				}
			}
		}
		g := newMockHnswGraph(nodes)
		got, err := IsRooted(g)
		if err != nil {
			t.Fatalf("[iter %d] IsRooted failed: %v", it, err)
		}
		want := referenceIsRooted(nodes)
		if got != want {
			t.Errorf("[iter %d] IsRooted = %v, want %v (numNodes=%d, numLevels=%d)",
				it, got, want, numNodes, numLevels)
		}
	}
}

// referenceIsRooted is the brute-force reference DFS used by the random
// test — a port of Java's TestHnswUtil.isRooted helper. It walks each
// level from the apex down and verifies that every present node on the
// level is reached from the entry-point set inherited from the next
// higher level.
func referenceIsRooted(nodes [][][]int) bool {
	for level := len(nodes) - 1; level >= 0; level-- {
		if !referenceIsRootedAtLevel(nodes, level) {
			return false
		}
	}
	return true
}

// referenceIsRootedAtLevel checks rooted-ness on a single level.
func referenceIsRootedAtLevel(nodes [][][]int, level int) bool {
	var entryPoints [][]int
	if level == len(nodes)-1 {
		entryPoints = [][]int{nodes[level][0]}
	} else {
		entryPoints = nodes[level+1]
	}
	connected, err := util.NewFixedBitSet(len(nodes[level]))
	if err != nil {
		panic(err)
	}
	count := 0
	for ep := 0; ep < len(entryPoints); ep++ {
		if entryPoints[ep] == nil {
			continue
		}
		stack := []int{ep}
		for len(stack) > 0 {
			n := len(stack) - 1
			node := stack[n]
			stack = stack[:n]
			if connected.Get(node) {
				continue
			}
			connected.Set(node)
			count++
			for _, nbr := range nodes[level][node] {
				stack = append(stack, nbr)
			}
		}
	}
	return count == referenceLevelSize(nodes[level])
}

// referenceLevelSize counts the number of non-nil entries on a level —
// the Java reference's TestHnswUtil.levelSize helper.
func referenceLevelSize(level [][]int) int {
	count := 0
	for _, node := range level {
		if node != nil {
			count++
		}
	}
	return count
}

// TestHnswUtil_LevelOutOfRange exercises the IllegalArgumentException-
// equivalent error returned when [Components] is called with a level
// outside the graph's range.
func TestHnswUtil_LevelOutOfRange(t *testing.T) {
	nodes := [][][]int{{{1}, {0}}}
	g := newMockHnswGraph(nodes)
	_, err := Components(g, 1, nil, 0)
	if err == nil {
		t.Fatalf("Components(level=1) returned nil error, want IllegalArgumentException")
	}
	if !strings.Contains(err.Error(), "Level 1 too large") {
		t.Errorf("error = %q, want substring %q", err.Error(), "Level 1 too large")
	}
}

// TestHnswUtil_ComponentsWithNotFullyConnected verifies that components
// populates the notFullyConnected bitset for every node whose neighbour
// count on the level is strictly less than maxConn. Mirrors the
// behaviour exercised by GraphRepair in the Java codebase even though
// the dedicated test case is not present in TestHnswUtil.java.
func TestHnswUtil_ComponentsWithNotFullyConnected(t *testing.T) {
	// nodes 0..3 with maxConn = 2. Each node's neighbour count:
	//   0 -> [1, 2]      = 2 (full)
	//   1 -> [3]         = 1 (under)
	//   2 -> [0]         = 1 (under)
	//   3 -> []          = 0 (under)
	nodes := [][][]int{
		{
			{1, 2},
			{3},
			{0},
			{},
		},
	}
	g := newMockHnswGraph(nodes)
	bs, err := util.NewFixedBitSet(g.Size())
	if err != nil {
		t.Fatalf("NewFixedBitSet failed: %v", err)
	}
	cs, err := Components(g, 0, bs, 2)
	if err != nil {
		t.Fatalf("Components failed: %v", err)
	}
	if len(cs) != 1 {
		t.Fatalf("len(components) = %d, want 1", len(cs))
	}
	if cs[0].Size() != 4 {
		t.Errorf("components[0].Size() = %d, want 4", cs[0].Size())
	}
	for _, node := range []int{1, 2, 3} {
		if !bs.Get(node) {
			t.Errorf("notFullyConnected.Get(%d) = false, want true", node)
		}
	}
	if bs.Get(0) {
		t.Errorf("notFullyConnected.Get(0) = true, want false (node 0 has 2 neighbours == maxConn)")
	}
}

// TestNextClearBit_PackageInternal exercises the unexported nextClearBit
// helper. The Java helper returns NO_MORE_DOCS where the FixedBitSet
// builtin returns the bitset length — the package-internal call site
// relies on the NO_MORE_DOCS contract, so we verify it here.
func TestNextClearBit_PackageInternal(t *testing.T) {
	bs, err := util.NewFixedBitSet(67) // > 1 word so the multi-word branch runs
	if err != nil {
		t.Fatalf("NewFixedBitSet failed: %v", err)
	}
	// Initial bitset: all clear → nextClearBit(0) = 0.
	if got := nextClearBit(bs, 0); got != 0 {
		t.Errorf("nextClearBit(clear, 0) = %d, want 0", got)
	}
	// Set bit 0; next clear is bit 1.
	bs.Set(0)
	if got := nextClearBit(bs, 0); got != 1 {
		t.Errorf("nextClearBit({0}, 0) = %d, want 1", got)
	}
	// Set bits 0..5; next clear is bit 6.
	for i := 1; i <= 5; i++ {
		bs.Set(i)
	}
	if got := nextClearBit(bs, 0); got != 6 {
		t.Errorf("nextClearBit({0..5}, 0) = %d, want 6", got)
	}
	// Verify the multi-word boundary: set bits 0..63 to force the
	// inner loop to step over the first word.
	for i := 6; i <= 63; i++ {
		bs.Set(i)
	}
	if got := nextClearBit(bs, 0); got != 64 {
		t.Errorf("nextClearBit({0..63}, 0) = %d, want 64", got)
	}
	// Fully set bitset → NO_MORE_DOCS.
	for i := 64; i < bs.Length(); i++ {
		bs.Set(i)
	}
	if got := nextClearBit(bs, 0); got != util.NO_MORE_DOCS {
		t.Errorf("nextClearBit(all-set, 0) = %d, want NO_MORE_DOCS", got)
	}
}

// TestMarkRooted_AlreadyConnected exercises the early-return path in the
// unexported markRooted helper. The Java implementation special-cases an
// entry point that has already been marked connected and returns a
// zero-sized component with the entry point as Start.
func TestMarkRooted_AlreadyConnected(t *testing.T) {
	nodes := [][][]int{{{1}, {0}}}
	g := newMockHnswGraph(nodes)
	connected, err := util.NewFixedBitSet(g.Size())
	if err != nil {
		t.Fatalf("NewFixedBitSet failed: %v", err)
	}
	connected.Set(0)
	c, err := markRooted(g, 0, connected, nil, 0, 0)
	if err != nil {
		t.Fatalf("markRooted failed: %v", err)
	}
	if c.Size() != 0 {
		t.Errorf("size = %d, want 0", c.Size())
	}
	if c.Start() != 0 {
		t.Errorf("start = %d, want 0", c.Start())
	}
}

// TestComponent_Accessors locks the Component getters in place so a
// future refactor cannot silently change the exported field semantics.
func TestComponent_Accessors(t *testing.T) {
	c := NewComponent(7, 42)
	if got, want := c.Start(), 7; got != want {
		t.Errorf("Start = %d, want %d", got, want)
	}
	if got, want := c.Size(), 42; got != want {
		t.Errorf("Size = %d, want %d", got, want)
	}
}

// compile-time assertion: the mock graph implements HnswGraph.
var _ HnswGraph = (*mockHnswGraph)(nil)
