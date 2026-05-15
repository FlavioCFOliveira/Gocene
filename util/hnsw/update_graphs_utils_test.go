// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestUpdateGraphsUtils_EncodeDecode_RoundTrip table-drives the
// encode/decode helpers across the full range of value1 values
// ComputeJoinSet ever produces (small non-negative gains) plus a handful
// of boundary cases that exercise the sign-extension paths in the
// decoder. The Java reference algorithm only emits non-negative value1,
// but the encoder/decoder pair is generic enough that we verify the
// round-trip in both halves of the int32 range for safety.
func TestUpdateGraphsUtils_EncodeDecode_RoundTrip(t *testing.T) {
	cases := []struct {
		name           string
		value1, value2 int
	}{
		{"zero/zero", 0, 0},
		{"small/small", 1, 1},
		{"small/medium", 5, 42},
		{"target-gain/node-zero", 100, 0},
		{"high-gain/large-node", 65535, 1 << 20},
		{"int32-max-half/zero", 1 << 30, 0},
		{"zero/int32-max", 0, 1<<31 - 1},
		{"value1 just below int32 max", 1<<31 - 1, 7},
		{"value2 = -1 (sign-extend trap)", 3, -1},
		{"value2 = int32-min", 9, -(1 << 31)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := encode(tc.value1, tc.value2)
			gotV1 := decodeValue1(enc)
			gotV2 := decodeValue2(enc)
			if gotV1 != tc.value1 {
				t.Errorf("decodeValue1(encode(%d,%d)) = %d, want %d",
					tc.value1, tc.value2, gotV1, tc.value1)
			}
			if gotV2 != tc.value2 {
				t.Errorf("decodeValue2(encode(%d,%d)) = %d, want %d",
					tc.value1, tc.value2, gotV2, tc.value2)
			}
		})
	}
}

// TestUpdateGraphsUtils_EncodeOrder verifies that the encoder's sign-flip
// causes a min-heap to surface the largest-value1 entry first — the
// load-bearing invariant exploited by ComputeJoinSet's priority queue.
func TestUpdateGraphsUtils_EncodeOrder(t *testing.T) {
	h := util.NewLongHeapMin(8)
	pairs := []struct{ gain, node int }{
		{3, 100},
		{7, 200},
		{1, 300},
		{12, 400},
		{5, 500},
	}
	for _, p := range pairs {
		h.Push(encode(p.gain, p.node))
	}
	// Pop and verify the gains come out in descending order.
	wantGains := []int{12, 7, 5, 3, 1}
	for i, want := range wantGains {
		got := decodeValue1(h.Pop())
		if got != want {
			t.Errorf("pop %d: gain = %d, want %d", i, got, want)
		}
	}
	if h.Size() != 0 {
		t.Errorf("heap size after drain = %d, want 0", h.Size())
	}
}

// TestUpdateGraphsUtils_CoverageTarget locks in the k = max(2, ceil(deg/4))
// behaviour, including the boundary at degree 9 where the formula
// transitions from the constant 2 to the ceil-div branch.
func TestUpdateGraphsUtils_CoverageTarget(t *testing.T) {
	cases := []struct {
		degree, want int
	}{
		{0, 2},
		{1, 2},
		{8, 2}, // last value still in the constant-2 range
		{9, 3}, // first value using ceil(deg/4) — ceil(9/4) = 3
		{10, 3},
		{11, 3},
		{12, 3},
		{13, 4}, // ceil(13/4) = 4
		{16, 4},
		{17, 5},
		{32, 8},
		{33, 9},
		{100, 25},
	}
	for _, tc := range cases {
		if got := coverageTarget(tc.degree); got != tc.want {
			t.Errorf("coverageTarget(%d) = %d, want %d", tc.degree, got, tc.want)
		}
	}
}

// TestUpdateGraphsUtils_ComputeJoinSet_Empty covers the size == 0 early
// return: ComputeJoinSet on the empty graph returns an empty set without
// instantiating a heap (LongHeap requires positive capacity).
func TestUpdateGraphsUtils_ComputeJoinSet_Empty(t *testing.T) {
	g := Empty()
	j, err := ComputeJoinSet(g)
	if err != nil {
		t.Fatalf("ComputeJoinSet(empty) failed: %v", err)
	}
	if len(j) != 0 {
		t.Errorf("len(joinSet) = %d, want 0", len(j))
	}
}

// TestUpdateGraphsUtils_ComputeJoinSet_SingleNode tests the smallest
// non-empty graph: one node, no neighbours. Initial gain = 2 + 0 = 2;
// gExit = 2. The node is popped, j = {0}, gTot = 2 >= gExit, loop ends.
func TestUpdateGraphsUtils_ComputeJoinSet_SingleNode(t *testing.T) {
	g := newMockHnswGraph([][][]int{{{}}})
	j, err := ComputeJoinSet(g)
	if err != nil {
		t.Fatalf("ComputeJoinSet failed: %v", err)
	}
	if len(j) != 1 {
		t.Fatalf("len(joinSet) = %d, want 1", len(j))
	}
	if _, ok := j[0]; !ok {
		t.Errorf("joinSet = %v, want {0}", joinSetKeys(j))
	}
}

// TestUpdateGraphsUtils_ComputeJoinSet_Pair tests two nodes connected
// reciprocally. Each node has degree 1, k = 2, initial gain = 2 + 1 = 3,
// gExit = 4. The first pop picks one node; the second pop's iteration
// either finds a stale-but-still-positive entry or commits the second
// node. Either way, every node must be covered by the join set or its
// neighbour list.
func TestUpdateGraphsUtils_ComputeJoinSet_Pair(t *testing.T) {
	g := newMockHnswGraph([][][]int{{{1}, {0}}})
	j, err := ComputeJoinSet(g)
	if err != nil {
		t.Fatalf("ComputeJoinSet failed: %v", err)
	}
	assertCovers(t, [][]int{{1}, {0}}, j)
}

// TestUpdateGraphsUtils_ComputeJoinSet_Star verifies the hub of a star
// graph is preferred: the hub has degree N-1 and thus the largest gain
// among all nodes, so the min-heap surfaces it first.
func TestUpdateGraphsUtils_ComputeJoinSet_Star(t *testing.T) {
	// 5-node star: 0 is the hub; 1..4 each link back to 0.
	g := newMockHnswGraph([][][]int{{
		{1, 2, 3, 4},
		{0},
		{0},
		{0},
		{0},
	}})
	j, err := ComputeJoinSet(g)
	if err != nil {
		t.Fatalf("ComputeJoinSet failed: %v", err)
	}
	// Node 0 has the highest gain — must be picked first.
	if _, ok := j[0]; !ok {
		t.Errorf("hub node 0 not in joinSet %v", joinSetKeys(j))
	}
	assertCovers(t, [][]int{{1, 2, 3, 4}, {0}, {0}, {0}, {0}}, j)
}

// TestUpdateGraphsUtils_ComputeJoinSet_TwoChains covers a disconnected
// graph: two independent 2-cycles {0<->1} and {2<->3}. Both components
// must be hit by the join set since each node has degree 1 and the
// coverage target is k = 2.
func TestUpdateGraphsUtils_ComputeJoinSet_TwoChains(t *testing.T) {
	g := newMockHnswGraph([][][]int{{
		{1},
		{0},
		{3},
		{2},
	}})
	j, err := ComputeJoinSet(g)
	if err != nil {
		t.Fatalf("ComputeJoinSet failed: %v", err)
	}
	// The exact identity of the picked nodes depends on heap tie-breaking
	// (which is implementation-defined for equal-gain entries) but every
	// component must contain at least one picked node.
	for _, comp := range [][]int{{0, 1}, {2, 3}} {
		hit := false
		for _, n := range comp {
			if _, ok := j[n]; ok {
				hit = true
				break
			}
		}
		if !hit {
			t.Errorf("component %v has no node in joinSet %v", comp, joinSetKeys(j))
		}
	}
}

// TestUpdateGraphsUtils_ComputeJoinSet_DenseGraph exercises the
// ceil-div branch of coverageTarget (degree >= 9) by constructing a
// 12-node graph where every node has 9 distinct neighbours. The
// resulting k = 3, gExit = 36; ComputeJoinSet should converge quickly
// because each picked node contributes gain = 12 (k + degree).
func TestUpdateGraphsUtils_ComputeJoinSet_DenseGraph(t *testing.T) {
	const n = 12
	adj := make([][]int, n)
	for i := 0; i < n; i++ {
		ns := make([]int, 0, 9)
		for off := 1; off <= 9; off++ {
			ns = append(ns, (i+off)%n)
		}
		adj[i] = ns
	}
	g := newMockHnswGraph([][][]int{adj})
	j, err := ComputeJoinSet(g)
	if err != nil {
		t.Fatalf("ComputeJoinSet failed: %v", err)
	}
	if len(j) == 0 {
		t.Fatalf("joinSet empty on dense graph")
	}
	if len(j) > n {
		t.Errorf("joinSet size %d > graph size %d", len(j), n)
	}
	// On a dense graph the algorithm should select a strict subset of
	// nodes — the inverse cover ratio implies fewer than n picks for a
	// fully-connected-ish graph. Two checks: every picked node is a
	// valid ordinal, and the picked count stays comfortably below n.
	for v := range j {
		if v < 0 || v >= n {
			t.Errorf("invalid node %d in joinSet", v)
		}
	}
	if len(j) > n/2 {
		// Soft expectation: a 12-node graph with k=3 should resolve
		// in well under 6 picks. Flag (not fail) if the algorithm
		// regresses noticeably; this isn't a correctness invariant
		// of the Java spec but is a useful behavioural sanity check.
		t.Logf("joinSet size %d > n/2 (%d) — denser than expected", len(j), n/2)
	}
}

// TestUpdateGraphsUtils_ComputeJoinSet_PropagatesError verifies that an
// HnswGraph implementation surfacing an error from SeekLevel is
// propagated up through ComputeJoinSet without being swallowed.
func TestUpdateGraphsUtils_ComputeJoinSet_PropagatesError(t *testing.T) {
	wantErr := &sentinelError{msg: "seek failed at v=2"}
	g := &errorHnswGraph{size: 4, failSeekAt: 2, err: wantErr}
	_, err := ComputeJoinSet(g)
	if err == nil {
		t.Fatalf("ComputeJoinSet returned nil error, want %v", wantErr)
	}
	if err != wantErr {
		t.Errorf("error = %v, want %v", err, wantErr)
	}
}

// joinSetKeys returns a sorted slice of the join-set keys, used only for
// nicer error messages.
func joinSetKeys(j map[int]struct{}) []int {
	out := make([]int, 0, len(j))
	for k := range j {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// assertCovers verifies that every node is either picked into the join
// set j or has a picked neighbour. The Java reference does not
// specify this as a hard invariant but the algorithm's purpose is to
// build a near-cover, so it is a strong behavioural check for any
// non-empty graph in which every node has at least one neighbour.
func assertCovers(t *testing.T, adj [][]int, j map[int]struct{}) {
	t.Helper()
	for v, ns := range adj {
		if _, ok := j[v]; ok {
			continue
		}
		covered := false
		for _, u := range ns {
			if _, ok := j[u]; ok {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("node %d (neighbours %v) is neither in joinSet nor has a picked neighbour; joinSet=%v",
				v, ns, joinSetKeys(j))
		}
	}
}

// errorHnswGraph is a minimal HnswGraph that triggers an error from
// SeekLevel when the target node equals failSeekAt. Used to exercise
// the error-propagation path of ComputeJoinSet without requiring a
// full mock graph.
type errorHnswGraph struct {
	size       int
	failSeekAt int
	err        error
}

func (e *errorHnswGraph) SeekLevel(level, target int) error {
	if target == e.failSeekAt {
		return e.err
	}
	return nil
}
func (e *errorHnswGraph) Size() int                  { return e.size }
func (e *errorHnswGraph) NextNeighbor() (int, error) { return util.NO_MORE_DOCS, nil }
func (e *errorHnswGraph) NumLevels() (int, error)    { return 1, nil }
func (e *errorHnswGraph) MaxConn() int               { return UnknownMaxConn }
func (e *errorHnswGraph) EntryNode() (int, error)    { return 0, nil }
func (e *errorHnswGraph) NeighborCount() int         { return 0 }
func (e *errorHnswGraph) GetNodesOnLevel(level int) (NodesIterator, error) {
	return NewDenseNodesIterator(e.size), nil
}

// sentinelError is a trivial named error used by the propagation test
// so error identity (==) is meaningful.
type sentinelError struct{ msg string }

func (s *sentinelError) Error() string { return s.msg }

// compile-time assertion: errorHnswGraph implements HnswGraph.
var _ HnswGraph = (*errorHnswGraph)(nil)
