// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math"
	"math/rand/v2"
	"testing"
)

// checkValidityTernary asserts the 3-ary min-heap property for every node.
// Mirrors TestTernaryLongHeap#checkValidity in Lucene.
func checkValidityTernary(t *testing.T, heap *TernaryLongHeap) {
	t.Helper()
	arr := heap.HeapArray()
	size := heap.Size()
	for parent := 1; parent <= size; parent++ {
		firstChild := ternaryArity*(parent-1) + 2
		lastChild := firstChild + ternaryArity - 1
		if lastChild > size {
			lastChild = size
		}
		for c := firstChild; c <= lastChild; c++ {
			if arr[parent] > arr[c] {
				t.Fatalf("heap invariant violated: heap[%d]=%d > heap[%d]=%d",
					parent, arr[parent], c, arr[c])
			}
		}
	}
}

// TestTernaryLongHeap_PQ ports Lucene's testPQ: insert N random values and
// verify Pop returns them in non-decreasing order with the original sum.
func TestTernaryLongHeap_PQ(t *testing.T) {
	const count = 10000
	r := rand.New(rand.NewPCG(0xC0FFEE, 0xDEADBEEF))
	pq := NewTernaryLongHeap(count)

	var sum, sum2 int64
	for i := 0; i < count; i++ {
		next := int64(r.Uint64())
		sum += next
		pq.Push(next)
	}
	last := int64(math.MinInt64)
	for i := 0; i < count; i++ {
		next := pq.Pop()
		if next < last {
			t.Fatalf("pop sequence not non-decreasing: %d after %d", next, last)
		}
		last = next
		sum2 += last
	}
	if sum != sum2 {
		t.Fatalf("sum mismatch: pushed=%d popped=%d", sum, sum2)
	}
}

// TestTernaryLongHeap_Clear mirrors Lucene's testClear.
func TestTernaryLongHeap_Clear(t *testing.T) {
	pq := NewTernaryLongHeap(3)
	pq.Push(2)
	pq.Push(3)
	pq.Push(1)
	if got := pq.Size(); got != 3 {
		t.Fatalf("size=%d, want 3", got)
	}
	pq.Clear()
	if got := pq.Size(); got != 0 {
		t.Fatalf("size after clear=%d, want 0", got)
	}
}

// TestTernaryLongHeap_ExceedBounds mirrors Lucene's testExceedBounds: a
// 1-capacity heap should grow unbounded under Push.
func TestTernaryLongHeap_ExceedBounds(t *testing.T) {
	pq := NewTernaryLongHeap(1)
	pq.Push(2)
	pq.Push(0)
	if got := pq.Size(); got != 2 {
		t.Fatalf("size=%d, want 2", got)
	}
	if got := pq.Top(); got != 0 {
		t.Fatalf("top=%d, want 0", got)
	}
}

// TestTernaryLongHeap_FixedSize mirrors Lucene's testFixedSize: with a
// 3-capacity heap, InsertWithOverflow keeps the three largest values, so the
// top (least) settles at 3.
func TestTernaryLongHeap_FixedSize(t *testing.T) {
	pq := NewTernaryLongHeap(3)
	pq.InsertWithOverflow(2)
	pq.InsertWithOverflow(3)
	pq.InsertWithOverflow(1)
	pq.InsertWithOverflow(5)
	pq.InsertWithOverflow(7)
	pq.InsertWithOverflow(1)
	if got := pq.Size(); got != 3 {
		t.Fatalf("size=%d, want 3", got)
	}
	if got := pq.Top(); got != 3 {
		t.Fatalf("top=%d, want 3", got)
	}
}

// TestTernaryLongHeap_DuplicateValues mirrors Lucene's testDuplicateValues
// including the assertion over the raw heap array layout after UpdateTop.
func TestTernaryLongHeap_DuplicateValues(t *testing.T) {
	pq := NewTernaryLongHeap(3)
	pq.Push(2)
	pq.Push(3)
	pq.Push(1)
	if got := pq.Top(); got != 1 {
		t.Fatalf("top=%d, want 1", got)
	}
	pq.UpdateTop(3)
	if got := pq.Size(); got != 3 {
		t.Fatalf("size=%d, want 3", got)
	}
	got := pq.HeapArray()
	want := []int64{0, 2, 3, 3}
	if len(got) < len(want) {
		t.Fatalf("heap array too short: %d", len(got))
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("heap[%d]=%d, want %d (full=%v)", i, got[i], v, got[:len(want)])
		}
	}
}

// TestTernaryLongHeap_Insertions mirrors Lucene's testInsertions.
func TestTernaryLongHeap_Insertions(t *testing.T) {
	r := rand.New(rand.NewPCG(0x12345, 0x67890))
	numDocs := 1 + r.IntN(100)
	pq := NewTernaryLongHeap(numDocs)
	var lastLeast int64
	haveLastLeast := false
	for i := 0; i < numDocs*10; i++ {
		newEntry := int64(r.Uint64() & 0x7FFFFFFFFFFFFFFF)
		pq.InsertWithOverflow(newEntry)
		checkValidityTernary(t, pq)
		newLeast := pq.Top()
		if haveLastLeast && newLeast != newEntry && newLeast != lastLeast {
			if newLeast > newEntry {
				t.Fatalf("newLeast=%d > newEntry=%d", newLeast, newEntry)
			}
			if newLeast < lastLeast {
				t.Fatalf("newLeast=%d < lastLeast=%d", newLeast, lastLeast)
			}
		}
		lastLeast = newLeast
		haveLastLeast = true
	}
}

// TestTernaryLongHeap_Invalid mirrors Lucene's testInvalid: -1, 0 and
// MaxArrayLength must all be rejected.
func TestTernaryLongHeap_Invalid(t *testing.T) {
	for _, bad := range []int{-1, 0, MaxArrayLength} {
		t.Run("", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic for initialCapacity=%d", bad)
				}
			}()
			_ = NewTernaryLongHeap(bad)
		})
	}
}

// TestTernaryLongHeap_Unbounded mirrors Lucene's testUnbounded.
func TestTernaryLongHeap_Unbounded(t *testing.T) {
	r := rand.New(rand.NewPCG(0xA1, 0xB2))
	initialSize := r.IntN(10) + 1
	pq := NewTernaryLongHeap(initialSize)
	num := r.IntN(100) + 1
	maxValue := int64(math.MinInt64)
	count := 0
	for i := 0; i < num; i++ {
		v := int64(r.Uint64())
		if r.IntN(2) == 0 {
			pq.Push(v)
			count++
		} else {
			full := pq.Size() >= initialSize
			if pq.InsertWithOverflow(v) {
				if !full {
					count++
				}
			}
		}
		if v > maxValue {
			maxValue = v
		}
	}
	if pq.Size() != count {
		t.Fatalf("size=%d, count=%d", pq.Size(), count)
	}
	last := int64(math.MinInt64)
	for pq.Size() > 0 {
		top := pq.Top()
		next := pq.Pop()
		if top != next {
			t.Fatalf("top=%d != next=%d", top, next)
		}
		count--
		if next < last {
			t.Fatalf("next=%d < last=%d", next, last)
		}
		last = next
	}
	if count != 0 {
		t.Fatalf("count=%d, want 0", count)
	}
	if last != maxValue {
		t.Fatalf("last=%d != maxValue=%d", last, maxValue)
	}
}

// TestTernaryLongHeap_Filled exercises the size+initialValue constructor.
func TestTernaryLongHeap_Filled(t *testing.T) {
	pq := NewTernaryLongHeapFilled(5, 42)
	if pq.Size() != 5 {
		t.Fatalf("size=%d, want 5", pq.Size())
	}
	for i := 1; i <= 5; i++ {
		if got := pq.Get(i); got != 42 {
			t.Fatalf("Get(%d)=%d, want 42", i, got)
		}
	}
}

// TestTernaryLongHeap_PushAll moves every element from one heap into another.
func TestTernaryLongHeap_PushAll(t *testing.T) {
	a := NewTernaryLongHeap(4)
	a.Push(3)
	a.Push(1)
	a.Push(4)
	b := NewTernaryLongHeap(4)
	b.PushAll(a)
	if b.Size() != 3 {
		t.Fatalf("size=%d, want 3", b.Size())
	}
	got := []int64{b.Pop(), b.Pop(), b.Pop()}
	want := []int64{1, 3, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pop[%d]=%d, want %d", i, got[i], want[i])
		}
	}
}

// TestTernaryLongHeap_PopEmpty checks the explicit panic on Pop with no
// elements (Lucene throws IllegalStateException).
func TestTernaryLongHeap_PopEmpty(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on empty Pop")
		}
	}()
	_ = NewTernaryLongHeap(1).Pop()
}
