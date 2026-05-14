// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

func checkLongHeapValidity(t *testing.T, h *LongHeap) {
	t.Helper()
	arr := h.HeapArray()
	for i := 2; i <= h.Size(); i++ {
		parent := i >> 1
		if h.order == LongHeapMin {
			if arr[parent] > arr[i] {
				t.Fatalf("min-heap invariant broken at i=%d: parent=%d (val=%d) > child=%d", i, parent, arr[parent], arr[i])
			}
		} else {
			if arr[parent] < arr[i] {
				t.Fatalf("max-heap invariant broken at i=%d: parent=%d (val=%d) < child=%d", i, parent, arr[parent], arr[i])
			}
		}
	}
}

func TestLongHeap_PQ(t *testing.T) {
	const count = 10000
	rng := rand.New(rand.NewSource(0x10AD))
	pq := NewLongHeapMin(count)
	var sum, sum2 int64

	for i := 0; i < count; i++ {
		next := rng.Int63() - rng.Int63()
		sum += next
		pq.Push(next)
	}
	last := int64(math.MinInt64)
	for i := 0; i < count; i++ {
		next := pq.Pop()
		if next < last {
			t.Fatalf("pop order: %d < %d", next, last)
		}
		last = next
		sum2 += last
	}
	if sum != sum2 {
		t.Fatalf("sum mismatch: %d vs %d", sum, sum2)
	}
}

func TestLongHeap_Clear(t *testing.T) {
	pq := NewLongHeapMin(3)
	pq.Push(2)
	pq.Push(3)
	pq.Push(1)
	if pq.Size() != 3 {
		t.Fatalf("size before clear=%d", pq.Size())
	}
	pq.Clear()
	if pq.Size() != 0 {
		t.Fatalf("size after clear=%d", pq.Size())
	}
}

func TestLongHeap_ExceedBoundsGrows(t *testing.T) {
	pq := NewLongHeapMin(1)
	pq.Push(2)
	pq.Push(0)
	if pq.Size() != 2 {
		t.Fatalf("size after grow=%d", pq.Size())
	}
	if pq.Top() != 0 {
		t.Fatalf("top=%d want 0", pq.Top())
	}
}

func TestLongHeap_FixedSizeOverflow(t *testing.T) {
	pq := NewLongHeapMin(3)
	pq.InsertWithOverflow(2)
	pq.InsertWithOverflow(3)
	pq.InsertWithOverflow(1)
	pq.InsertWithOverflow(5)
	pq.InsertWithOverflow(7)
	pq.InsertWithOverflow(1)
	if pq.Size() != 3 {
		t.Fatalf("size=%d want 3", pq.Size())
	}
	if pq.Top() != 3 {
		t.Fatalf("top=%d want 3", pq.Top())
	}
}

func TestLongHeap_DuplicateValues(t *testing.T) {
	pq := NewLongHeapMin(3)
	pq.Push(2)
	pq.Push(3)
	pq.Push(1)
	if pq.Top() != 1 {
		t.Fatalf("top=%d want 1", pq.Top())
	}
	pq.UpdateTop(3)
	if pq.Size() != 3 {
		t.Fatalf("size after updateTop=%d", pq.Size())
	}
	want := []int64{0, 2, 3, 3}
	got := pq.HeapArray()
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("HeapArray[%d]=%d want %d", i, got[i], v)
		}
	}
}

func TestLongHeap_Insertions(t *testing.T) {
	rng := rand.New(rand.NewSource(0x52A0))
	numDocsInPQ := 1 + rng.Intn(100)
	pq := NewLongHeapMin(numDocsInPQ)
	var lastLeastSet bool
	var lastLeast int64

	for i := 0; i < numDocsInPQ*10; i++ {
		newEntry := rng.Int63()
		if newEntry < 0 {
			newEntry = -newEntry
		}
		pq.InsertWithOverflow(newEntry)
		checkLongHeapValidity(t, pq)
		newLeast := pq.Top()
		if lastLeastSet && newLeast != newEntry && newLeast != lastLeast {
			if newLeast > newEntry {
				t.Fatalf("newLeast=%d > newEntry=%d", newLeast, newEntry)
			}
			if newLeast < lastLeast {
				t.Fatalf("newLeast=%d < lastLeast=%d", newLeast, lastLeast)
			}
		}
		lastLeast = newLeast
		lastLeastSet = true
	}
}

func TestLongHeap_InvalidCapacity(t *testing.T) {
	for _, c := range []int{-1, 0, MaxArrayLength} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for capacity %d", c)
				}
			}()
			NewLongHeapMin(c)
		}()
	}
}

func TestLongHeap_Unbounded(t *testing.T) {
	rng := rand.New(rand.NewSource(0x99))
	initialSize := 1 + rng.Intn(10)
	pq := NewLongHeapMin(initialSize)
	num := 1 + rng.Intn(100)
	maxValue := int64(math.MinInt64)
	count := 0
	for i := 0; i < num; i++ {
		value := rng.Int63() - rng.Int63()
		if rng.Intn(2) == 0 {
			pq.Push(value)
			count++
		} else {
			full := pq.Size() >= initialSize
			if pq.InsertWithOverflow(value) {
				if !full {
					count++
				}
			}
		}
		if value > maxValue {
			maxValue = value
		}
	}
	if count != pq.Size() {
		t.Fatalf("count=%d size=%d", count, pq.Size())
	}
	last := int64(math.MinInt64)
	for pq.Size() > 0 {
		top := pq.Top()
		next := pq.Pop()
		if top != next {
			t.Fatalf("top=%d != pop=%d", top, next)
		}
		count--
		if next < last {
			t.Fatalf("next=%d < last=%d", next, last)
		}
		last = next
	}
	if count != 0 {
		t.Fatalf("final count=%d want 0", count)
	}
	if last != maxValue {
		t.Fatalf("last=%d maxValue=%d", last, maxValue)
	}
}

func TestLongHeap_PopOnEmptyPanics(t *testing.T) {
	pq := NewLongHeapMin(1)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on empty Pop")
		}
	}()
	pq.Pop()
}

func TestLongHeap_PushAllMerges(t *testing.T) {
	a := NewLongHeapMin(8)
	b := NewLongHeapMin(8)
	a.Push(5)
	a.Push(7)
	b.Push(1)
	b.Push(3)
	a.PushAll(b)
	out := make([]int64, 0, a.Size())
	for a.Size() > 0 {
		out = append(out, a.Pop())
	}
	want := []int64{1, 3, 5, 7}
	for i, v := range want {
		if out[i] != v {
			t.Fatalf("merged out[%d]=%d want %d", i, out[i], v)
		}
	}
}

func TestLongHeap_FilledConstructor(t *testing.T) {
	h := NewLongHeapFilled(5, 42)
	if h.Size() != 5 {
		t.Fatalf("size=%d want 5", h.Size())
	}
	for i := 1; i <= 5; i++ {
		if h.Get(i) != 42 {
			t.Fatalf("Get(%d)=%d want 42", i, h.Get(i))
		}
	}
}

func TestLongHeap_MaxHeapBasic(t *testing.T) {
	pq := NewLongHeapMax(10)
	values := []int64{3, 1, 4, 1, 5, 9, 2, 6, 5, 3}
	for _, v := range values {
		pq.Push(v)
		checkLongHeapValidity(t, pq)
	}
	if pq.Top() != 9 {
		t.Fatalf("max-heap top=%d want 9", pq.Top())
	}
	var out []int64
	for pq.Size() > 0 {
		out = append(out, pq.Pop())
	}
	// Should be sorted descending.
	expected := append([]int64(nil), values...)
	sort.Slice(expected, func(i, j int) bool { return expected[i] > expected[j] })
	for i := range expected {
		if out[i] != expected[i] {
			t.Fatalf("max-heap pop %d: got=%d want=%d", i, out[i], expected[i])
		}
	}
}

func TestLongHeap_MaxHeapInsertWithOverflow(t *testing.T) {
	// Max-heap with InsertWithOverflow keeps the smallest values.
	pq := NewLongHeapMax(3)
	pq.InsertWithOverflow(2)
	pq.InsertWithOverflow(3)
	pq.InsertWithOverflow(1)
	pq.InsertWithOverflow(5)
	pq.InsertWithOverflow(7)
	pq.InsertWithOverflow(0)
	if pq.Size() != 3 {
		t.Fatalf("size=%d want 3", pq.Size())
	}
	// We should retain {0, 1, 2}; the top of a max-heap of those is 2.
	if pq.Top() != 2 {
		t.Fatalf("max-overflow top=%d want 2", pq.Top())
	}
}

func TestLongHeap_GetHeapArray(t *testing.T) {
	pq := NewLongHeapMin(4)
	pq.Push(7)
	pq.Push(2)
	pq.Push(5)
	pq.Push(1)
	arr := pq.HeapArray()
	if arr[0] != 0 {
		t.Fatalf("slot 0 should remain zero, got %d", arr[0])
	}
	// Size+1 elements after that should be initialised; we cannot
	// assert their order beyond the heap invariants.
	if pq.Size() != 4 {
		t.Fatalf("size=%d want 4", pq.Size())
	}
}
