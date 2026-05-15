// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Compile-time assertion: emptyHnswGraph satisfies HnswGraph. The same
// check is exercised at runtime by every test that calls Empty().
var _ HnswGraph = emptyHnswGraph{}

// Compile-time assertion: every concrete iterator satisfies NodesIterator.
var (
	_ NodesIterator = (*ArrayNodesIterator)(nil)
	_ NodesIterator = (*DenseNodesIterator)(nil)
	_ NodesIterator = (*CollectionNodesIterator)(nil)
)

// expectPanicNoSuchElement asserts that fn panics with the string
// "NoSuchElementException", matching the Java reference's
// NoSuchElementException semantics.
func expectPanicNoSuchElement(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic, got none")
		}
		s, ok := r.(string)
		if !ok || s != "NoSuchElementException" {
			t.Fatalf("expected panic %q, got %v", "NoSuchElementException", r)
		}
	}()
	fn()
}

// TestEmptyHnswGraph_AllAccessors verifies that the singleton empty graph
// returns the zero-state contract defined by Java's HnswGraph.EMPTY.
func TestEmptyHnswGraph_AllAccessors(t *testing.T) {
	g := Empty()
	if got := g.Size(); got != 0 {
		t.Errorf("Size() = %d, want 0", got)
	}
	if got := g.NeighborCount(); got != 0 {
		t.Errorf("NeighborCount() = %d, want 0", got)
	}
	if got := g.MaxConn(); got != UnknownMaxConn {
		t.Errorf("MaxConn() = %d, want %d", got, UnknownMaxConn)
	}

	if got, err := g.NumLevels(); err != nil || got != 0 {
		t.Errorf("NumLevels() = (%d, %v), want (0, nil)", got, err)
	}
	if got, err := g.EntryNode(); err != nil || got != 0 {
		t.Errorf("EntryNode() = (%d, %v), want (0, nil)", got, err)
	}
	if got, err := g.NextNeighbor(); err != nil || got != util.NO_MORE_DOCS {
		t.Errorf("NextNeighbor() = (%d, %v), want (NO_MORE_DOCS, nil)", got, err)
	}
	if err := g.SeekLevel(0, 0); err != nil {
		t.Errorf("SeekLevel(0,0) error = %v, want nil", err)
	}

	// Multiple SeekLevel calls must remain no-ops on the empty graph.
	if err := g.SeekLevel(3, 42); err != nil {
		t.Errorf("SeekLevel(3,42) error = %v, want nil", err)
	}
	if got, err := g.NextNeighbor(); err != nil || got != util.NO_MORE_DOCS {
		t.Errorf("post-SeekLevel NextNeighbor() = (%d, %v), want (NO_MORE_DOCS, nil)", got, err)
	}
}

// TestEmptyHnswGraph_GetNodesOnLevel verifies that the empty graph yields
// the exhausted DenseNodesIterator for every level.
func TestEmptyHnswGraph_GetNodesOnLevel(t *testing.T) {
	g := Empty()
	for _, level := range []int{0, 1, 7} {
		it, err := g.GetNodesOnLevel(level)
		if err != nil {
			t.Fatalf("GetNodesOnLevel(%d) error = %v", level, err)
		}
		if it.Size() != 0 {
			t.Errorf("GetNodesOnLevel(%d).Size() = %d, want 0", level, it.Size())
		}
		if it.HasNext() {
			t.Errorf("GetNodesOnLevel(%d).HasNext() = true, want false", level)
		}
	}
}

// TestEmptyHnswGraph_IsSingleton verifies that successive calls to Empty
// return the same interface value, mirroring Java's HnswGraph.EMPTY field.
func TestEmptyHnswGraph_IsSingleton(t *testing.T) {
	if Empty() != Empty() {
		t.Errorf("Empty() returned non-identical values across calls")
	}
}

// TestMaxNodeID_DefaultImplementation verifies the contract that for a
// graph of size n, MaxNodeID returns n-1.
func TestMaxNodeID_DefaultImplementation(t *testing.T) {
	cases := []struct {
		size int
		want int
	}{
		{0, -1},
		{1, 0},
		{42, 41},
	}
	for _, tc := range cases {
		g := &stubGraph{size: tc.size}
		if got := MaxNodeID(g); got != tc.want {
			t.Errorf("MaxNodeID(size=%d) = %d, want %d", tc.size, got, tc.want)
		}
	}
}

// TestArrayNodesIterator_FullConsumption exercises NextInt to completion.
func TestArrayNodesIterator_FullConsumption(t *testing.T) {
	nodes := []int{7, 3, 11, 5}
	it := NewArrayNodesIterator(nodes)
	if it.Size() != len(nodes) {
		t.Errorf("Size() = %d, want %d", it.Size(), len(nodes))
	}

	got := make([]int, 0, len(nodes))
	for it.HasNext() {
		got = append(got, it.NextInt())
	}
	if !reflect.DeepEqual(got, nodes) {
		t.Errorf("iteration = %v, want %v", got, nodes)
	}
	if it.HasNext() {
		t.Errorf("HasNext after exhaustion = true, want false")
	}
}

// TestArrayNodesIterator_PanicAfterExhaustion verifies the Java
// NoSuchElementException-equivalent panic on NextInt past the end.
func TestArrayNodesIterator_PanicAfterExhaustion(t *testing.T) {
	it := NewArrayNodesIterator([]int{1, 2})
	_ = it.NextInt()
	_ = it.NextInt()
	expectPanicNoSuchElement(t, func() { _ = it.NextInt() })
}

// TestArrayNodesIterator_ConsumeRespectsCursor verifies Consume's contract:
// at most min(remaining, len(dest)) elements are copied, and the cursor
// advances correspondingly. Tests both branches: dest smaller than remaining
// (partial drain), and dest larger than remaining (final drain).
func TestArrayNodesIterator_ConsumeRespectsCursor(t *testing.T) {
	nodes := []int{10, 20, 30, 40, 50}
	it := NewArrayNodesIterator(nodes)

	dest := make([]int, 2)
	n := it.Consume(dest)
	if n != 2 {
		t.Fatalf("Consume(len=2) = %d, want 2", n)
	}
	if !reflect.DeepEqual(dest, []int{10, 20}) {
		t.Errorf("first Consume dest = %v, want [10 20]", dest)
	}

	// Second call: dest larger than remaining (3 left, dest length 5).
	dest = make([]int, 5)
	n = it.Consume(dest)
	if n != 3 {
		t.Fatalf("Consume(len=5) = %d, want 3", n)
	}
	if !reflect.DeepEqual(dest[:n], []int{30, 40, 50}) {
		t.Errorf("second Consume dest[:%d] = %v, want [30 40 50]", n, dest[:n])
	}

	if it.HasNext() {
		t.Errorf("HasNext after full Consume = true, want false")
	}
}

// TestArrayNodesIterator_ConsumePanicsWhenExhausted matches the Java
// reference: Consume on an exhausted iterator throws NoSuchElementException.
func TestArrayNodesIterator_ConsumePanicsWhenExhausted(t *testing.T) {
	it := NewArrayNodesIterator([]int{1})
	_ = it.NextInt()
	expectPanicNoSuchElement(t, func() { _ = it.Consume(make([]int, 1)) })
}

// TestArrayNodesIterator_SizeOverride exercises the two-argument constructor
// retained for on-disk back-compatibility: the reported size may be smaller
// than len(nodes), and iteration stops at the recorded size rather than the
// slice length.
func TestArrayNodesIterator_SizeOverride(t *testing.T) {
	nodes := []int{1, 2, 3, 4, 5}
	it := NewArrayNodesIteratorWithSize(nodes, 3)
	if it.Size() != 3 {
		t.Errorf("Size() = %d, want 3", it.Size())
	}
	got := make([]int, 0, 3)
	for it.HasNext() {
		got = append(got, it.NextInt())
	}
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("iteration = %v, want [1 2 3]", got)
	}
}

// TestDenseNodesIterator_FullConsumption verifies that the dense iterator
// yields [0, size) in order.
func TestDenseNodesIterator_FullConsumption(t *testing.T) {
	const size = 5
	it := NewDenseNodesIterator(size)
	if it.Size() != size {
		t.Errorf("Size() = %d, want %d", it.Size(), size)
	}
	for i := 0; i < size; i++ {
		if !it.HasNext() {
			t.Fatalf("HasNext at i=%d = false, want true", i)
		}
		if got := it.NextInt(); got != i {
			t.Errorf("NextInt at i=%d = %d, want %d", i, got, i)
		}
	}
	if it.HasNext() {
		t.Errorf("HasNext after exhaustion = true, want false")
	}
}

// TestDenseNodesIterator_ZeroSize verifies that a zero-sized dense iterator
// reports HasNext == false from the first call (used by the empty graph).
func TestDenseNodesIterator_ZeroSize(t *testing.T) {
	it := NewDenseNodesIterator(0)
	if it.Size() != 0 {
		t.Errorf("Size() = %d, want 0", it.Size())
	}
	if it.HasNext() {
		t.Errorf("HasNext() = true on size=0, want false")
	}
	expectPanicNoSuchElement(t, func() { _ = it.NextInt() })
}

// TestDenseNodesIterator_Consume covers Consume against the [0, size) range.
func TestDenseNodesIterator_Consume(t *testing.T) {
	it := NewDenseNodesIterator(5)

	dest := make([]int, 3)
	n := it.Consume(dest)
	if n != 3 {
		t.Fatalf("Consume(len=3) = %d, want 3", n)
	}
	if !reflect.DeepEqual(dest, []int{0, 1, 2}) {
		t.Errorf("first Consume dest = %v, want [0 1 2]", dest)
	}

	dest = make([]int, 10)
	n = it.Consume(dest)
	if n != 2 {
		t.Fatalf("Consume(len=10) = %d, want 2", n)
	}
	if !reflect.DeepEqual(dest[:n], []int{3, 4}) {
		t.Errorf("second Consume dest[:%d] = %v, want [3 4]", n, dest[:n])
	}
}

// TestDenseNodesIterator_NextIntPanic verifies the NoSuchElementException
// behaviour on exhaustion.
func TestDenseNodesIterator_NextIntPanic(t *testing.T) {
	it := NewDenseNodesIterator(1)
	_ = it.NextInt()
	expectPanicNoSuchElement(t, func() { _ = it.NextInt() })
}

// TestDenseNodesIterator_ConsumePanicsWhenExhausted matches the Java
// reference: Consume on an exhausted iterator throws NoSuchElementException.
func TestDenseNodesIterator_ConsumePanicsWhenExhausted(t *testing.T) {
	it := NewDenseNodesIterator(1)
	_ = it.NextInt()
	expectPanicNoSuchElement(t, func() { _ = it.Consume(make([]int, 1)) })
}

// TestCollectionNodesIterator_FullConsumption walks a slice-backed
// CollectionNodesIterator to completion.
func TestCollectionNodesIterator_FullConsumption(t *testing.T) {
	nodes := []int{42, 7, 13}
	it := NewCollectionNodesIterator(nodes)
	if it.Size() != len(nodes) {
		t.Errorf("Size() = %d, want %d", it.Size(), len(nodes))
	}
	got := make([]int, 0, len(nodes))
	for it.HasNext() {
		got = append(got, it.NextInt())
	}
	if !reflect.DeepEqual(got, nodes) {
		t.Errorf("iteration = %v, want %v", got, nodes)
	}
}

// TestCollectionNodesIterator_Consume verifies that Consume mirrors the
// Java implementation: it copies element by element up to len(dest).
func TestCollectionNodesIterator_Consume(t *testing.T) {
	it := NewCollectionNodesIterator([]int{2, 4, 6, 8})

	dest := make([]int, 3)
	n := it.Consume(dest)
	if n != 3 {
		t.Fatalf("Consume(len=3) = %d, want 3", n)
	}
	if !reflect.DeepEqual(dest, []int{2, 4, 6}) {
		t.Errorf("Consume dest = %v, want [2 4 6]", dest)
	}

	dest = make([]int, 5)
	n = it.Consume(dest)
	if n != 1 {
		t.Fatalf("Consume(len=5) = %d, want 1", n)
	}
	if dest[0] != 8 {
		t.Errorf("Consume dest[0] = %d, want 8", dest[0])
	}
}

// TestCollectionNodesIterator_Panics verifies NoSuchElementException
// behaviour on both NextInt and Consume past the end.
func TestCollectionNodesIterator_Panics(t *testing.T) {
	it := NewCollectionNodesIterator([]int{1})
	_ = it.NextInt()
	expectPanicNoSuchElement(t, func() { _ = it.NextInt() })

	it2 := NewCollectionNodesIterator([]int{1})
	_ = it2.NextInt()
	expectPanicNoSuchElement(t, func() { _ = it2.Consume(make([]int, 1)) })
}

// TestGetSortedNodes_Level0 verifies that level 0 short-circuits to a
// dense iterator over [0, Size()), without delegating to GetNodesOnLevel.
func TestGetSortedNodes_Level0(t *testing.T) {
	g := &stubGraph{size: 4}
	it, err := GetSortedNodes(g, 0)
	if err != nil {
		t.Fatalf("GetSortedNodes(0) error = %v", err)
	}
	if _, ok := it.(*DenseNodesIterator); !ok {
		t.Errorf("GetSortedNodes(0) type = %T, want *DenseNodesIterator", it)
	}
	got := make([]int, 0, 4)
	for it.HasNext() {
		got = append(got, it.NextInt())
	}
	if !reflect.DeepEqual(got, []int{0, 1, 2, 3}) {
		t.Errorf("level-0 sorted = %v, want [0 1 2 3]", got)
	}
	if g.getNodesOnLevelCalls != 0 {
		t.Errorf("GetNodesOnLevel was invoked %d times for level 0, want 0", g.getNodesOnLevelCalls)
	}
}

// TestGetSortedNodes_HigherLevel verifies that for level > 0 the nodes
// returned by GetNodesOnLevel are materialised into an ArrayNodesIterator
// in ascending order, regardless of the source order.
func TestGetSortedNodes_HigherLevel(t *testing.T) {
	unsorted := []int{7, 1, 4, 9, 2}
	g := &stubGraph{
		size:          10,
		nodesByLevel:  map[int][]int{2: unsorted},
		numLevelsVal:  3,
		entryNodeVal:  9,
		maxConnVal:    16,
		neighborCount: 0,
	}

	it, err := GetSortedNodes(g, 2)
	if err != nil {
		t.Fatalf("GetSortedNodes(2) error = %v", err)
	}
	if _, ok := it.(*ArrayNodesIterator); !ok {
		t.Errorf("GetSortedNodes(2) type = %T, want *ArrayNodesIterator", it)
	}
	if it.Size() != len(unsorted) {
		t.Errorf("Size() = %d, want %d", it.Size(), len(unsorted))
	}

	got := make([]int, 0, len(unsorted))
	for it.HasNext() {
		got = append(got, it.NextInt())
	}

	want := append([]int(nil), unsorted...)
	sort.Ints(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sorted nodes = %v, want %v", got, want)
	}
}

// TestGetSortedNodes_PropagatesError verifies that errors raised by
// GetNodesOnLevel surface through GetSortedNodes for non-zero levels.
func TestGetSortedNodes_PropagatesError(t *testing.T) {
	want := errors.New("boom")
	g := &stubGraph{
		size:                10,
		getNodesOnLevelFail: want,
	}
	_, err := GetSortedNodes(g, 1)
	if !errors.Is(err, want) {
		t.Errorf("error = %v, want %v", err, want)
	}
}

// stubGraph is a minimal HnswGraph for default-method tests. Only the
// methods exercised by the tests need meaningful values.
type stubGraph struct {
	size                int
	nodesByLevel        map[int][]int
	numLevelsVal        int
	entryNodeVal        int
	maxConnVal          int
	neighborCount       int
	getNodesOnLevelFail error

	// Counters for test assertions.
	getNodesOnLevelCalls int
}

func (s *stubGraph) SeekLevel(level, target int) error { return nil }
func (s *stubGraph) Size() int                         { return s.size }
func (s *stubGraph) NextNeighbor() (int, error)        { return util.NO_MORE_DOCS, nil }
func (s *stubGraph) NumLevels() (int, error)           { return s.numLevelsVal, nil }
func (s *stubGraph) MaxConn() int                      { return s.maxConnVal }
func (s *stubGraph) EntryNode() (int, error)           { return s.entryNodeVal, nil }
func (s *stubGraph) NeighborCount() int                { return s.neighborCount }

func (s *stubGraph) GetNodesOnLevel(level int) (NodesIterator, error) {
	s.getNodesOnLevelCalls++
	if s.getNodesOnLevelFail != nil {
		return nil, s.getNodesOnLevelFail
	}
	nodes := s.nodesByLevel[level]
	return NewArrayNodesIterator(nodes), nil
}
