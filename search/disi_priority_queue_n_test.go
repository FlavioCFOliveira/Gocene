// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/DisiPriorityQueueN.java
//
// No dedicated Java test peer found.  These tests cover the Go public
// contract of the 0-indexed min-heap implementation.

package search_test

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// makeDisiWrapperForN builds a DisiWrapper positioned at the given doc ID.
// SetDoc is called so that heap-ordering tests see the correct doc without
// needing a full NextDoc traversal.
func makeDisiWrapperForN(docID int) *search.DisiWrapper {
	sc := newConstantScorer([]int{docID}, 1, 1)
	w := search.NewDisiWrapper(sc, false)
	w.SetDoc(docID)
	return w
}

// TestDisiPriorityQueueN_Empty verifies behaviour on an empty queue.
func TestDisiPriorityQueueN_Empty(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(10)
	if pq.Size() != 0 {
		t.Errorf("Size()=%d, want 0", pq.Size())
	}
	if pq.Top() != nil {
		t.Errorf("Top()=%v, want nil", pq.Top())
	}
	if pq.Top2() != nil {
		t.Errorf("Top2()=%v, want nil", pq.Top2())
	}
	if pq.TopList() != nil {
		t.Errorf("TopList()=%v, want nil", pq.TopList())
	}
	if pq.Pop() != nil {
		t.Errorf("Pop()=%v, want nil", pq.Pop())
	}
}

// TestDisiPriorityQueueN_AddOne verifies a single-entry queue.
func TestDisiPriorityQueueN_AddOne(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(5)
	w := makeDisiWrapperForN(7)
	top := pq.Add(w)
	if top != w {
		t.Errorf("Add returned %v, want %v", top, w)
	}
	if pq.Size() != 1 {
		t.Errorf("Size()=%d, want 1", pq.Size())
	}
	if pq.Top() != w {
		t.Errorf("Top() != w")
	}
	if pq.Top2() != nil {
		t.Errorf("Top2() should be nil for single entry")
	}
}

// TestDisiPriorityQueueN_MinOrdering verifies insertions maintain min-heap order.
func TestDisiPriorityQueueN_MinOrdering(t *testing.T) {
	docs := []int{5, 1, 9, 3, 7, 2}
	pq := search.NewDisiPriorityQueueN(len(docs))
	for _, d := range docs {
		pq.Add(makeDisiWrapperForN(d))
	}

	// Pop all and verify ascending order.
	var got []int
	for pq.Size() > 0 {
		got = append(got, pq.Pop().Doc())
	}
	sorted := make([]int, len(docs))
	copy(sorted, docs)
	sort.Ints(sorted)
	for i, want := range sorted {
		if got[i] != want {
			t.Errorf("got[%d]=%d, want %d", i, got[i], want)
		}
	}
}

// TestDisiPriorityQueueN_Top2 verifies Top2 returns second-smallest.
func TestDisiPriorityQueueN_Top2(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(5)
	pq.Add(makeDisiWrapperForN(10))
	pq.Add(makeDisiWrapperForN(5))
	pq.Add(makeDisiWrapperForN(7))

	// Top should be 5, Top2 should be 7.
	if pq.Top().Doc() != 5 {
		t.Errorf("Top().doc=%d, want 5", pq.Top().Doc())
	}
	top2 := pq.Top2()
	if top2 == nil {
		t.Fatal("Top2() is nil")
	}
	if top2.Doc() != 7 {
		t.Errorf("Top2().doc=%d, want 7", top2.Doc())
	}
}

// TestDisiPriorityQueueN_Top2_Two verifies Top2 with exactly 2 entries.
func TestDisiPriorityQueueN_Top2_Two(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(5)
	pq.Add(makeDisiWrapperForN(3))
	pq.Add(makeDisiWrapperForN(8))
	top2 := pq.Top2()
	if top2 == nil {
		t.Fatal("Top2() is nil for 2 entries")
	}
	if top2.Doc() != 8 {
		t.Errorf("Top2().doc=%d, want 8", top2.Doc())
	}
}

// TestDisiPriorityQueueN_TopList_AllSame verifies TopList chains all entries
// with the same docID.
func TestDisiPriorityQueueN_TopList_AllSame(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(5)
	for i := 0; i < 4; i++ {
		w := makeDisiWrapperForN(3)
		pq.Add(w)
	}
	// Advance all wrappers to doc=3 (they're already at doc=-1 after construction).
	// We need to actually call NextDoc to position them.
	// Re-create: make them at doc=3 by using SetDoc.
	pq.Clear()
	ws := make([]*search.DisiWrapper, 4)
	for i := range ws {
		sc := newConstantScorer([]int{3}, 1, 1)
		w := search.NewDisiWrapper(sc, false)
		// Advance to doc=3.
		w.SetDoc(3)
		ws[i] = w
		pq.Add(w)
	}

	list := pq.TopList()
	count := 0
	for w := list; w != nil; w = w.Next() {
		count++
	}
	if count != 4 {
		t.Errorf("TopList chain length=%d, want 4", count)
	}
}

// TestDisiPriorityQueueN_UpdateTop verifies re-heapify after modifying top.
func TestDisiPriorityQueueN_UpdateTop(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(5)
	pq.Add(makeDisiWrapperForN(1))
	pq.Add(makeDisiWrapperForN(5))
	pq.Add(makeDisiWrapperForN(3))

	// Modify top to doc=10 (it becomes a large value).
	pq.Top().SetDoc(10)
	newTop := pq.UpdateTop()
	if newTop.Doc() != 3 {
		t.Errorf("UpdateTop().doc=%d, want 3", newTop.Doc())
	}
}

// TestDisiPriorityQueueN_UpdateTopWith verifies replacement of top entry.
func TestDisiPriorityQueueN_UpdateTopWith(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(5)
	pq.Add(makeDisiWrapperForN(2))
	pq.Add(makeDisiWrapperForN(8))

	replacement := makeDisiWrapperForN(5)
	newTop := pq.UpdateTopWith(replacement)
	// Replacement doc=5 is larger than the other entry doc=8? No, 5 < 8 → new top=5.
	if newTop.Doc() != 5 {
		t.Errorf("UpdateTopWith().doc=%d, want 5", newTop.Doc())
	}
}

// TestDisiPriorityQueueN_Clear verifies Clear resets the queue.
func TestDisiPriorityQueueN_Clear(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(5)
	for _, d := range []int{1, 2, 3} {
		pq.Add(makeDisiWrapperForN(d))
	}
	pq.Clear()
	if pq.Size() != 0 {
		t.Errorf("Size()=%d after Clear, want 0", pq.Size())
	}
	if pq.Top() != nil {
		t.Errorf("Top() non-nil after Clear")
	}
}

// TestDisiPriorityQueueN_AddAll verifies bulk insertion with Floyd heapify.
func TestDisiPriorityQueueN_AddAll(t *testing.T) {
	docs := []int{9, 1, 5, 3, 7, 2, 8, 4, 6}
	entries := make([]*search.DisiWrapper, len(docs))
	for i, d := range docs {
		entries[i] = makeDisiWrapperForN(d)
	}

	pq := search.NewDisiPriorityQueueN(len(docs))
	pq.AddAll(entries, 0, len(entries))

	if pq.Size() != len(docs) {
		t.Errorf("Size()=%d, want %d", pq.Size(), len(docs))
	}
	// Pop all and verify ascending.
	var got []int
	for pq.Size() > 0 {
		got = append(got, pq.Pop().Doc())
	}
	sorted := make([]int, len(docs))
	copy(sorted, docs)
	sort.Ints(sorted)
	for i, want := range sorted {
		if got[i] != want {
			t.Errorf("pop order got[%d]=%d, want %d", i, got[i], want)
		}
	}

// TestDisiPriorityQueueN_AddAll_CapacityPanic verifies AddAll panics on overflow.
}
func TestDisiPriorityQueueN_AddAll_CapacityPanic(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(2)
	entries := []*search.DisiWrapper{makeDisiWrapperForN(1), makeDisiWrapperForN(2), makeDisiWrapperForN(3)}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on AddAll overflow, got none")
		}
	}()
	pq.AddAll(entries, 0, 3)
}

// TestDisiPriorityQueueN_HeapAll verifies HeapAll iterates all entries.
func TestDisiPriorityQueueN_HeapAll(t *testing.T) {
	pq := search.NewDisiPriorityQueueN(5)
	docs := []int{1, 3, 5}
	for _, d := range docs {
		pq.Add(makeDisiWrapperForN(d))
	}
	count := 0
	for range pq.HeapAll() {
		count++
	}
	if count != len(docs) {
		t.Errorf("HeapAll count=%d, want %d", count, len(docs))
	}
}