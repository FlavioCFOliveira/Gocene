// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"math"
	"math/rand/v2"
	"sort"
	"testing"
)

// TestNeighborQueue_NeighborsProduct is the Go port of
// TestNeighborQueue.testNeighborsProduct from Lucene 10.4.0. It exercises a
// MIN-heap variant and checks that, with three insertWithOverflow calls into
// a queue of initial size 2, the overflow drops the worst (smallest score)
// element and the top travels from 0.5 to 1.0 across two Pop()s.
func TestNeighborQueue_NeighborsProduct(t *testing.T) {
	nn := NewNeighborQueue(2, false)
	if !nn.InsertWithOverflow(2, 0.5) {
		t.Fatalf("InsertWithOverflow(2, 0.5): want true")
	}
	if !nn.InsertWithOverflow(1, 0.2) {
		t.Fatalf("InsertWithOverflow(1, 0.2): want true")
	}
	// Heap is full at size 2 (scores {0.5, 0.2}); 1.0 > top (0.2 in
	// MIN_HEAP), so the entry is accepted and 0.2 is evicted.
	if !nn.InsertWithOverflow(3, 1.0) {
		t.Fatalf("InsertWithOverflow(3, 1.0): want true")
	}
	if got, want := nn.TopScore(), float32(0.5); got != want {
		t.Fatalf("TopScore after overflow: got %v want %v", got, want)
	}
	nn.Pop()
	if got, want := nn.TopScore(), float32(1.0); got != want {
		t.Fatalf("TopScore after Pop: got %v want %v", got, want)
	}
	nn.Pop()
	if got, want := nn.Size(), 0; got != want {
		t.Fatalf("Size after draining: got %d want %d", got, want)
	}
}

// TestNeighborQueue_NeighborsMaxHeap mirrors Java's testNeighborsMaxHeap. In
// MAX_HEAP mode the queue keeps the SMALLEST scores; an inserted score that
// exceeds the current max-of-kept is rejected.
func TestNeighborQueue_NeighborsMaxHeap(t *testing.T) {
	nn := NewNeighborQueue(2, true)
	if !nn.InsertWithOverflow(2, 2) {
		t.Fatalf("InsertWithOverflow(2, 2): want true")
	}
	if !nn.InsertWithOverflow(1, 1) {
		t.Fatalf("InsertWithOverflow(1, 1): want true")
	}
	// Heap full with worst-of-kept = 2.0; offering 3.0 (a HIGHER score) is
	// REJECTED in MAX-heap mode because the queue retains the smaller scores.
	if nn.InsertWithOverflow(3, 3) {
		t.Fatalf("InsertWithOverflow(3, 3): want false (3 > current max kept)")
	}
	if got, want := nn.TopScore(), float32(2); got != want {
		t.Fatalf("TopScore after rejected overflow: got %v want %v", got, want)
	}
	nn.Pop()
	if got, want := nn.TopScore(), float32(1); got != want {
		t.Fatalf("TopScore after Pop: got %v want %v", got, want)
	}
}

// TestNeighborQueue_TopMaxHeap mirrors Java's testTopMaxHeap and confirms
// that in MAX-heap mode (lower scores better, highest score on top) the
// top-of-queue tracks the maximum.
func TestNeighborQueue_TopMaxHeap(t *testing.T) {
	nn := NewNeighborQueue(2, true)
	nn.Add(1, 2)
	nn.Add(2, 1)
	if got, want := nn.TopScore(), float32(2); got != want {
		t.Fatalf("TopScore: got %v want %v", got, want)
	}
	if got, want := nn.TopNode(), int32(1); got != want {
		t.Fatalf("TopNode: got %d want %d", got, want)
	}
}

// TestNeighborQueue_TopMinHeap mirrors Java's testTopMinHeap and confirms
// the MIN-heap behaviour: top tracks the lowest score.
func TestNeighborQueue_TopMinHeap(t *testing.T) {
	nn := NewNeighborQueue(2, false)
	nn.Add(1, 0.5)
	nn.Add(2, -0.5)
	if got, want := nn.TopScore(), float32(-0.5); got != want {
		t.Fatalf("TopScore: got %v want %v", got, want)
	}
	if got, want := nn.TopNode(), int32(2); got != want {
		t.Fatalf("TopNode: got %d want %d", got, want)
	}
}

// TestNeighborQueue_VisitedCount mirrors Java's testVisitedCount; the queue
// stores the value untouched and returns it on read.
func TestNeighborQueue_VisitedCount(t *testing.T) {
	nn := NewNeighborQueue(2, false)
	nn.SetVisitedCount(100)
	if got, want := nn.VisitedCount(), 100; got != want {
		t.Fatalf("VisitedCount: got %d want %d", got, want)
	}
}

// TestNeighborQueue_Clear mirrors Java's testClear; after Clear, Size and
// VisitedCount are zero and Incomplete is false even when previously set.
func TestNeighborQueue_Clear(t *testing.T) {
	nn := NewNeighborQueue(2, false)
	nn.Add(1, 1.1)
	nn.Add(2, -2.2)
	nn.SetVisitedCount(42)
	nn.MarkIncomplete()
	nn.Clear()

	if got, want := nn.Size(), 0; got != want {
		t.Fatalf("Size after Clear: got %d want %d", got, want)
	}
	if got, want := nn.VisitedCount(), 0; got != want {
		t.Fatalf("VisitedCount after Clear: got %d want %d", got, want)
	}
	if nn.Incomplete() {
		t.Fatalf("Incomplete after Clear: got true want false")
	}
}

// TestNeighborQueue_MaxSizeQueue mirrors Java's testMaxSizeQueue and asserts
// the (initialSize=2, MIN_HEAP) overflow vs. unbounded-Add distinction.
func TestNeighborQueue_MaxSizeQueue(t *testing.T) {
	nn := NewNeighborQueue(2, false)
	nn.Add(1, 1)
	nn.Add(2, 2)
	if got, want := nn.Size(), 2; got != want {
		t.Fatalf("Size after initial Add: got %d want %d", got, want)
	}
	if got, want := nn.TopNode(), int32(1); got != want {
		t.Fatalf("TopNode: got %d want %d", got, want)
	}

	// InsertWithOverflow on a full queue: 3 > top (1.0), accepted, evicts
	// the smallest score; size stays at the configured maximum.
	nn.InsertWithOverflow(3, 3)
	if got, want := nn.Size(), 2; got != want {
		t.Fatalf("Size after InsertWithOverflow: got %d want %d", got, want)
	}
	if got, want := nn.TopNode(), int32(2); got != want {
		t.Fatalf("TopNode after overflow: got %d want %d", got, want)
	}

	// Add extends the queue beyond initialSize without bound.
	nn.Add(4, 1)
	if got, want := nn.Size(), 3; got != want {
		t.Fatalf("Size after unbounded Add: got %d want %d", got, want)
	}
}

// TestNeighborQueue_UnboundedQueue mirrors Java's testUnboundedQueue. With a
// max-heap of initialSize=1 we Add 256 random-score entries; the running
// maximum must be reflected at TopScore/TopNode the whole time.
func TestNeighborQueue_UnboundedQueue(t *testing.T) {
	nn := NewNeighborQueue(1, true)
	// Deterministic seed so the test is reproducible and self-contained.
	rng := rand.New(rand.NewPCG(0xC0FFEE, 0xDEADBEEF))

	maxScore := float32(-2)
	var maxNode int32 = -1
	for i := 0; i < 256; i++ {
		score := rng.Float32()
		if score > maxScore {
			maxScore = score
			maxNode = int32(i)
		}
		nn.Add(int32(i), score)
	}
	if got, want := nn.TopScore(), maxScore; got != want {
		t.Fatalf("TopScore: got %v want %v", got, want)
	}
	if got, want := nn.TopNode(), maxNode; got != want {
		t.Fatalf("TopNode: got %d want %d", got, want)
	}
}

// TestNeighborQueue_InvalidArguments mirrors Java's testInvalidArguments. The
// Java code throws IllegalArgumentException for initialSize=0; in Go the
// underlying TernaryLongHeap panics with a descriptive message.
func TestNeighborQueue_InvalidArguments(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("NewNeighborQueue(0, false): expected panic, got none")
		}
	}()
	NewNeighborQueue(0, false)
}

// TestNeighborQueue_ToString mirrors Java's testToString: the default
// representation embeds the current size.
func TestNeighborQueue_ToString(t *testing.T) {
	if got, want := NewNeighborQueue(2, false).String(), "Neighbors[0]"; got != want {
		t.Fatalf("String: got %q want %q", got, want)
	}

	q := NewNeighborQueue(2, false)
	q.Add(1, 1)
	q.Add(2, 2)
	if got, want := q.String(), "Neighbors[2]"; got != want {
		t.Fatalf("String after Adds: got %q want %q", got, want)
	}
}

// --- Additional Go-side coverage beyond the Java test peer ------------------

// TestNeighborQueue_TieBreakingMinHeap pins the encoding's tie-breaking
// behaviour for the MIN heap. With equal scores, raw(node) is a strictly
// DECREASING function of node id (since the low 32 bits are ^node), so the
// MIN heap's top resolves to the LARGER node id — and InsertWithOverflow
// readily evicts that larger id when a tied entry with a smaller id arrives.
// In Lucene terminology this is the "smaller node id wins" tie-break: the
// smaller id is retained while the larger is evicted.
func TestNeighborQueue_TieBreakingMinHeap(t *testing.T) {
	q := NewNeighborQueue(2, false)
	q.Add(5, 1.0)
	q.Add(3, 1.0)
	// MIN heap with tied scores: largest node id sits at the top.
	if got, want := q.TopNode(), int32(5); got != want {
		t.Fatalf("MIN tie initial: TopNode got %d want %d", got, want)
	}

	// Offer a third entry tied at 1.0 with id=1 (smaller). Smaller id has
	// a LARGER raw long → strictly greater than the current top → accepted.
	// The eviction removes id=5; the new top is id=3 (largest of the two
	// remaining ids {1, 3}).
	if !q.InsertWithOverflow(1, 1.0) {
		t.Fatalf("MIN tie: InsertWithOverflow(1, 1.0): want true")
	}
	if got, want := q.TopNode(), int32(3); got != want {
		t.Fatalf("MIN tie after overflow: TopNode got %d want %d", got, want)
	}

	// Drain the queue: the two remaining nodes must be exactly {1, 3}.
	got := []int32{q.Pop(), q.Pop()}
	if got[0] == got[1] || (got[0] != 1 && got[0] != 3) || (got[1] != 1 && got[1] != 3) {
		t.Fatalf("MIN tie drain: got %v want {1, 3} in some order", got)
	}
}

// TestNeighborQueue_TieBreakingMaxHeap pins the encoding's tie-breaking
// behaviour for the MAX heap. After the order-flip, the encoded long is
// strictly INCREASING in node id for ties, so the underlying min-heap's
// top resolves to the SMALLER node id (top = best = lowest-encoded =
// largest raw = smallest node id on ties).
func TestNeighborQueue_TieBreakingMaxHeap(t *testing.T) {
	q := NewNeighborQueue(2, true)
	q.Add(5, 1.0)
	q.Add(3, 1.0)
	// MAX heap with tied scores: smaller node id sits at the top.
	if got, want := q.TopNode(), int32(3); got != want {
		t.Fatalf("MAX tie initial: TopNode got %d want %d", got, want)
	}

	// Offer an even SMALLER node id with the same score: encoded(1) is
	// strictly less than encoded(3) under the flip, so it falls below the
	// internal min-heap top and is REJECTED.
	if q.InsertWithOverflow(1, 1.0) {
		t.Fatalf("MAX tie: InsertWithOverflow(1, 1.0): want false (smaller id with tied score should be rejected)")
	}
	if got, want := q.TopNode(), int32(3); got != want {
		t.Fatalf("MAX tie after rejected overflow: TopNode got %d want %d", got, want)
	}

	// Offer a LARGER node id with the same score: encoded(7) is strictly
	// greater than encoded(3) under the flip, so it exceeds the internal
	// top and IS accepted — and the smaller node id (3) is retained.
	if !q.InsertWithOverflow(7, 1.0) {
		t.Fatalf("MAX tie: InsertWithOverflow(7, 1.0): want true")
	}
	// After the swap the kept set is {5, 7}; smaller id (5) at top.
	if got, want := q.TopNode(), int32(5); got != want {
		t.Fatalf("MAX tie after accepted overflow: TopNode got %d want %d", got, want)
	}
}

// TestNeighborQueue_PopOrderMinHeap drains a MIN heap and asserts that
// scores come out in ascending order.
func TestNeighborQueue_PopOrderMinHeap(t *testing.T) {
	q := NewNeighborQueue(16, false)
	inputs := []struct {
		node  int32
		score float32
	}{
		{10, 3.0},
		{11, 1.0},
		{12, 4.0},
		{13, 1.5},
		{14, 5.0},
		{15, 9.0},
		{16, 2.0},
		{17, 6.0},
	}
	for _, in := range inputs {
		q.Add(in.node, in.score)
	}
	want := []float32{1.0, 1.5, 2.0, 3.0, 4.0, 5.0, 6.0, 9.0}
	got := make([]float32, 0, len(inputs))
	for q.Size() > 0 {
		got = append(got, q.TopScore())
		q.Pop()
	}
	if !sliceEqualFloat32(got, want) {
		t.Fatalf("MIN drain order: got %v want %v", got, want)
	}
}

// TestNeighborQueue_PopOrderMaxHeap drains a MAX heap and asserts that
// scores come out in descending order.
func TestNeighborQueue_PopOrderMaxHeap(t *testing.T) {
	q := NewNeighborQueue(16, true)
	inputs := []float32{3.0, 1.0, 4.0, 1.5, 5.0, 9.0, 2.0, 6.0}
	for i, s := range inputs {
		q.Add(int32(100+i), s)
	}
	want := []float32{9.0, 6.0, 5.0, 4.0, 3.0, 2.0, 1.5, 1.0}
	got := make([]float32, 0, len(inputs))
	for q.Size() > 0 {
		got = append(got, q.TopScore())
		q.Pop()
	}
	if !sliceEqualFloat32(got, want) {
		t.Fatalf("MAX drain order: got %v want %v", got, want)
	}
}

// TestNeighborQueue_Nodes verifies that Nodes() returns every node id
// currently in the queue (in any order) and never reads outside the live
// range, by reconstructing the set and comparing as sorted slices.
func TestNeighborQueue_Nodes(t *testing.T) {
	q := NewNeighborQueue(8, false)
	ids := []int32{7, 11, 13, 2, 5, 3}
	for i, id := range ids {
		q.Add(id, float32(i))
	}
	got := q.Nodes()
	if len(got) != len(ids) {
		t.Fatalf("Nodes length: got %d want %d", len(got), len(ids))
	}
	gotSorted := append([]int32(nil), got...)
	wantSorted := append([]int32(nil), ids...)
	sort.Slice(gotSorted, func(i, j int) bool { return gotSorted[i] < gotSorted[j] })
	sort.Slice(wantSorted, func(i, j int) bool { return wantSorted[i] < wantSorted[j] })
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("Nodes set mismatch at %d: got %d want %d", i, gotSorted[i], wantSorted[i])
		}
	}

	// Nodes must be a fresh slice that the caller can mutate freely.
	got[0] = -999
	got2 := q.Nodes()
	for _, v := range got2 {
		if v == -999 {
			t.Fatalf("Nodes returned shared storage: caller mutation observable")
		}
	}
}

// TestNeighborQueue_GrowthBeyondInitialCapacity exercises the Add code path
// when the queue grows past its initial capacity, mirroring the JVM
// behaviour where the underlying TernaryLongHeap reallocates.
func TestNeighborQueue_GrowthBeyondInitialCapacity(t *testing.T) {
	q := NewNeighborQueue(2, false)
	for i := 0; i < 100; i++ {
		q.Add(int32(i), float32(100-i))
	}
	if got, want := q.Size(), 100; got != want {
		t.Fatalf("Size: got %d want %d", got, want)
	}
	// The minimum score is 1.0 (i=99 → score=1), node id 99.
	if got, want := q.TopScore(), float32(1); got != want {
		t.Fatalf("TopScore: got %v want %v", got, want)
	}
	if got, want := q.TopNode(), int32(99); got != want {
		t.Fatalf("TopNode: got %d want %d", got, want)
	}
}

// TestNeighborQueue_EmptyTopReturnsZeroValues confirms the documented
// no-emptiness-check behaviour of TopScore/TopNode: on an empty queue the
// methods return the float/int decoded from the zero heap slot.
//
// Java's NeighborQueue does no emptiness check either, so the values are
// purely a consequence of decoding zero through the active order:
//   - MIN heap: order.apply(0) == 0 → decodeNodeID = (int) ~0 = -1
//   - MAX heap: order.apply(0) == ~0 → decodeNodeID = (int) ~~0 = 0
//   - TopScore is 0.0 in both modes (sortableIntToFloat of 0).
func TestNeighborQueue_EmptyTopReturnsZeroValues(t *testing.T) {
	cases := []struct {
		name        string
		maxHeap     bool
		wantTopNode int32
	}{
		{"MIN heap", false, -1},
		{"MAX heap", true, 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			q := NewNeighborQueue(2, c.maxHeap)
			if got, want := q.TopScore(), float32(0); got != want {
				t.Fatalf("empty TopScore: got %v want %v", got, want)
			}
			if got, want := q.TopNode(), c.wantTopNode; got != want {
				t.Fatalf("empty TopNode: got %d want %d", got, want)
			}
		})
	}
}

// TestNeighborQueue_PopOnEmptyPanics confirms Pop on an empty queue panics,
// matching the Java reference which surfaces an IllegalStateException via
// the underlying TernaryLongHeap.
func TestNeighborQueue_PopOnEmptyPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Pop on empty queue: expected panic, got none")
		}
	}()
	NewNeighborQueue(2, false).Pop()
}

// TestNeighborQueue_NegativeScores ensures the encoding round-trips through
// the float<->sortableInt transform for negative and special values.
func TestNeighborQueue_NegativeScores(t *testing.T) {
	q := NewNeighborQueue(8, false)
	scores := []float32{
		-1.5,
		0.0,
		float32(math.Copysign(0, -1)), // -0.0
		1.5,
		float32(math.Inf(-1)),
		float32(math.Inf(1)),
		3.14159,
	}
	for i, s := range scores {
		q.Add(int32(i), s)
	}
	// The smallest float (including -Inf) must surface on top of a MIN heap.
	if got, want := q.TopScore(), float32(math.Inf(-1)); got != want {
		t.Fatalf("TopScore with -Inf present: got %v want %v", got, want)
	}
	q.Pop()
	if got, want := q.TopScore(), float32(-1.5); got != want {
		t.Fatalf("TopScore after popping -Inf: got %v want %v", got, want)
	}
}

// TestNeighborQueue_LargeNodeIDs covers node ids that exercise the full
// int32 range, including negative ids (which are legal int values in Java
// and round-trip through the ^node encoding).
func TestNeighborQueue_LargeNodeIDs(t *testing.T) {
	q := NewNeighborQueue(8, false)
	ids := []int32{0, 1, -1, math.MaxInt32, math.MinInt32, math.MaxInt32 - 1, math.MinInt32 + 1}
	for i, id := range ids {
		q.Add(id, float32(i))
	}

	// Drain — every original id must come back exactly.
	seen := make(map[int32]bool, len(ids))
	for q.Size() > 0 {
		seen[q.TopNode()] = true
		q.Pop()
	}
	for _, id := range ids {
		if !seen[id] {
			t.Fatalf("id %d not recovered after drain", id)
		}
	}
}

// sliceEqualFloat32 reports whether two float32 slices are element-wise
// equal under the strict == predicate (no epsilon).
func sliceEqualFloat32(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
