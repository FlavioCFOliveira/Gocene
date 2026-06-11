// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestDisiPriorityQueue2.java
//   No Java test peer exists — synthetic Go tests covering the contract.

package search

import "testing"

func disiWrapper2(doc int) *DisiWrapper { return &DisiWrapper{doc: doc} }

// TestDisiPriorityQueue2_Empty verifies behaviour of an empty queue.
func TestDisiPriorityQueue2_Empty(t *testing.T) {
	pq := NewDisiPriorityQueue2()
	if pq.Size() != 0 {
		t.Fatalf("expected size 0, got %d", pq.Size())
	}
	if pq.Top() != nil {
		t.Fatal("expected nil Top")
	}
	if pq.Top2() != nil {
		t.Fatal("expected nil Top2")
	}
	if pq.TopList() != nil {
		t.Fatal("expected nil TopList")
	}
}

// TestDisiPriorityQueue2_SingleEntry verifies single-entry behaviour.
func TestDisiPriorityQueue2_SingleEntry(t *testing.T) {
	pq := NewDisiPriorityQueue2()
	w := disiWrapper2(5)
	pq.Add(w)
	if pq.Size() != 1 {
		t.Fatalf("expected size 1, got %d", pq.Size())
	}
	if pq.Top() != w {
		t.Fatal("expected top to be the added entry")
	}
	if pq.Top2() != nil {
		t.Fatal("expected nil Top2")
	}
}

// TestDisiPriorityQueue2_TwoEntriesOrdering verifies ordering: smaller doc is top.
func TestDisiPriorityQueue2_TwoEntriesOrdering(t *testing.T) {
	pq := NewDisiPriorityQueue2()
	a := disiWrapper2(10)
	b := disiWrapper2(3)
	pq.Add(a)
	pq.Add(b)
	if pq.Size() != 2 {
		t.Fatalf("expected size 2, got %d", pq.Size())
	}
	if pq.Top().doc != 3 {
		t.Fatalf("expected top.doc=3, got %d", pq.Top().doc)
	}
	if pq.Top2().doc != 10 {
		t.Fatalf("expected top2.doc=10, got %d", pq.Top2().doc)
	}
}

// TestDisiPriorityQueue2_Pop verifies Pop removes the top.
func TestDisiPriorityQueue2_Pop(t *testing.T) {
	pq := NewDisiPriorityQueue2()
	a := disiWrapper2(3)
	b := disiWrapper2(7)
	pq.Add(b)
	pq.Add(a)
	got := pq.Pop()
	if got != a {
		t.Fatalf("expected to pop a (doc=3)")
	}
	if pq.Size() != 1 {
		t.Fatalf("expected size 1, got %d", pq.Size())
	}
	if pq.Top() != b {
		t.Fatalf("expected top to be b after pop")
	}
	if pq.Top2() != nil {
		t.Fatal("expected nil Top2 after pop")
	}
}

// TestDisiPriorityQueue2_UpdateTop verifies UpdateTop after modifying doc.
func TestDisiPriorityQueue2_UpdateTop(t *testing.T) {
	pq := NewDisiPriorityQueue2()
	a := disiWrapper2(3)
	b := disiWrapper2(7)
	pq.Add(a)
	pq.Add(b)
	// Move top ahead so it is no longer the smallest
	pq.Top().doc = 100
	pq.UpdateTop()
	if pq.Top().doc != 7 {
		t.Fatalf("expected new top doc=7, got %d", pq.Top().doc)
	}
}

// TestDisiPriorityQueue2_TopList_SameDoc verifies that both entries appear in
// topList when they share the same doc.
func TestDisiPriorityQueue2_TopList_SameDoc(t *testing.T) {
	pq := NewDisiPriorityQueue2()
	a := disiWrapper2(5)
	b := disiWrapper2(5)
	pq.Add(a)
	pq.Add(b)
	list := pq.TopList()
	count := 0
	for w := list; w != nil; w = w.next {
		count++
	}
	if count != 2 {
		t.Fatalf("expected topList length 2, got %d", count)
	}
}

// TestDisiPriorityQueue2_Clear verifies Clear empties the queue.
func TestDisiPriorityQueue2_Clear(t *testing.T) {
	pq := NewDisiPriorityQueue2()
	pq.Add(disiWrapper2(1))
	pq.Add(disiWrapper2(2))
	pq.Clear()
	if pq.Size() != 0 {
		t.Fatalf("expected size 0 after Clear, got %d", pq.Size())
	}

// TestDisiPriorityQueue2_ThirdEntryPanics verifies that adding a 3rd entry panics.
}
func TestDisiPriorityQueue2_ThirdEntryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on 3rd Add")
		}
	}()
	pq := NewDisiPriorityQueue2()
	pq.Add(disiWrapper2(1))
	pq.Add(disiWrapper2(2))
	pq.Add(disiWrapper2(3)) // must panic
}