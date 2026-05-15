// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"testing"
)

// TestFloatHeap_BasicOperations is the Go port of
// TestFloatHeap.testBasicOperations from Lucene 10.4.0. With
// maxSize=3, offering {2,4,1,3} retains the largest three (4,3,2)
// in heap form; polling yields them in ascending order.
func TestFloatHeap_BasicOperations(t *testing.T) {
	h := NewFloatHeap(3)
	if !h.Offer(2) {
		t.Fatalf("Offer(2): want true")
	}
	if !h.Offer(4) {
		t.Fatalf("Offer(4): want true")
	}
	if !h.Offer(1) {
		t.Fatalf("Offer(1): want true")
	}
	// Heap is full; 3 > min (2), so 2 is evicted and 3 is kept.
	if !h.Offer(3) {
		t.Fatalf("Offer(3) on full heap with 3>min: want true")
	}
	if got, want := h.Size(), 3; got != want {
		t.Fatalf("Size: got %d want %d", got, want)
	}
	if got, want := h.Peek(), float32(2); got != want {
		t.Fatalf("Peek: got %v want %v", got, want)
	}

	if got, want := h.Poll(), float32(2); got != want {
		t.Fatalf("Poll #1: got %v want %v", got, want)
	}
	if got, want := h.Poll(), float32(3); got != want {
		t.Fatalf("Poll #2: got %v want %v", got, want)
	}
	if got, want := h.Poll(), float32(4); got != want {
		t.Fatalf("Poll #3: got %v want %v", got, want)
	}
	if got, want := h.Size(), 0; got != want {
		t.Fatalf("Size after drain: got %d want %d", got, want)
	}
}

// TestFloatHeap_BasicOperations2 is the Go port of
// TestFloatHeap.testBasicOperations2. A capacity-N heap is loaded
// with N random floats; the polled sequence must be
// non-decreasing, and (because nothing is discarded) the sums
// before and after must match within the same epsilon Lucene uses.
func TestFloatHeap_BasicOperations2(t *testing.T) {
	// atLeast(10): Lucene's helper returns a number >= 10 with some
	// random padding. We pick a deterministic value > 10 so the test
	// is reproducible without depending on Lucene's RandomizedRunner.
	const size = 25
	h := NewFloatHeap(size)
	// Seeded PCG keeps the test deterministic; the assertion
	// (monotonic poll, sum equality) does not depend on the seed.
	r := rand.New(rand.NewPCG(0xC0FFEE, 0xBADC0DE))

	var sum, sum2 float64
	for i := 0; i < size; i++ {
		next := r.Float32() * 100 // [0, 100)
		sum += float64(next)
		if !h.Offer(next) {
			t.Fatalf("Offer #%d returned false (unexpected with fresh heap)", i)
		}
	}

	last := float32(math.Inf(-1))
	for i := 0; i < size; i++ {
		next := h.Poll()
		if next < last {
			t.Fatalf("Poll #%d not monotonic: got %v, previous %v", i, next, last)
		}
		last = next
		sum2 += float64(last)
	}
	if delta := math.Abs(sum - sum2); delta > 0.01 {
		t.Fatalf("sum/sum2 diverge by %v (>0.01); sum=%v sum2=%v", delta, sum, sum2)
	}
}

// TestFloatHeap_Clear is the Go port of TestFloatHeap.testClear.
// In particular it pins the (intentional) Java quirk that Peek()
// after Clear() returns the prior top value rather than a zero.
func TestFloatHeap_Clear(t *testing.T) {
	h := NewFloatHeap(3)
	h.Offer(20)
	h.Offer(40)
	h.Offer(30)
	if got, want := h.Size(), 3; got != want {
		t.Fatalf("Size after 3 offers: got %d want %d", got, want)
	}
	if got, want := h.Peek(), float32(20); got != want {
		t.Fatalf("Peek before clear: got %v want %v", got, want)
	}

	h.Clear()
	if got, want := h.Size(), 0; got != want {
		t.Fatalf("Size after Clear: got %d want %d", got, want)
	}
	// Java quirk preserved: Peek does not check size; slot 1 still
	// holds the previous min (20).
	if got, want := h.Peek(), float32(20); got != want {
		t.Fatalf("Peek after Clear (preserved slot): got %v want %v", got, want)
	}

	h.Offer(15)
	h.Offer(35)
	if got, want := h.Size(), 2; got != want {
		t.Fatalf("Size after two offers post-Clear: got %d want %d", got, want)
	}
	if got, want := h.Peek(), float32(15); got != want {
		t.Fatalf("Peek post-Clear+offers: got %v want %v", got, want)
	}

	if got, want := h.Poll(), float32(15); got != want {
		t.Fatalf("Poll #1: got %v want %v", got, want)
	}
	if got, want := h.Poll(), float32(35); got != want {
		t.Fatalf("Poll #2: got %v want %v", got, want)
	}
	if got, want := h.Size(), 0; got != want {
		t.Fatalf("Size after drain: got %d want %d", got, want)
	}
}

// TestFloatHeap_OfferDiscardsBelowMin exercises the full-heap
// rejection branch of Offer (Java: value < heap[1] => return false).
func TestFloatHeap_OfferDiscardsBelowMin(t *testing.T) {
	h := NewFloatHeap(3)
	h.Offer(10)
	h.Offer(20)
	h.Offer(30)
	// Heap is full, min is 10. A value strictly less than 10 must be
	// rejected and the heap state must be untouched.
	if h.Offer(5) {
		t.Fatalf("Offer(5) on full heap with min=10: want false")
	}
	if got, want := h.Size(), 3; got != want {
		t.Fatalf("Size after rejected offer: got %d want %d", got, want)
	}
	if got, want := h.Peek(), float32(10); got != want {
		t.Fatalf("Peek after rejected offer: got %v want %v", got, want)
	}
	// Equal-to-min must NOT be rejected (Java uses strict <).
	if !h.Offer(10) {
		t.Fatalf("Offer(10) when min=10: want true (strict-less rejection)")
	}
}

// TestFloatHeap_GetHeapReturnsLiveCopy verifies that GetHeap
// returns a fresh slice containing exactly the live elements in
// some order (heap order, not sorted), and that mutating the
// returned slice does not affect the heap.
func TestFloatHeap_GetHeapReturnsLiveCopy(t *testing.T) {
	h := NewFloatHeap(5)
	in := []float32{7, 1, 4, 9, 3}
	for _, v := range in {
		h.Offer(v)
	}

	got := h.GetHeap()
	if len(got) != len(in) {
		t.Fatalf("GetHeap length: got %d want %d", len(got), len(in))
	}
	// Compare as multisets: the heap holds exactly the inputs.
	sortedIn := append([]float32(nil), in...)
	sortedGot := append([]float32(nil), got...)
	sort.Slice(sortedIn, func(i, j int) bool { return sortedIn[i] < sortedIn[j] })
	sort.Slice(sortedGot, func(i, j int) bool { return sortedGot[i] < sortedGot[j] })
	for i := range sortedIn {
		if sortedIn[i] != sortedGot[i] {
			t.Fatalf("GetHeap multiset mismatch at %d: got %v want %v (full got=%v in=%v)",
				i, sortedGot[i], sortedIn[i], got, in)
		}
	}

	// Mutating the returned slice must not affect the heap.
	got[0] = 999
	if h.Peek() == 999 {
		t.Fatalf("GetHeap returned a live view, not a copy: mutating it changed Peek()")
	}
}

// TestFloatHeap_PollEmptyPanics confirms Poll on an empty heap
// panics with ErrEmptyHeap (mirroring Java's IllegalStateException).
func TestFloatHeap_PollEmptyPanics(t *testing.T) {
	h := NewFloatHeap(2)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Poll on empty heap: expected panic, got none")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic value is not an error: %T %v", r, r)
		}
		if !errors.Is(err, ErrEmptyHeap) {
			t.Fatalf("panic value: got %v want %v", err, ErrEmptyHeap)
		}
	}()
	_ = h.Poll()
}

// TestFloatHeap_NewFloatHeapRejectsNonPositive locks in the
// constructor's input contract.
func TestFloatHeap_NewFloatHeapRejectsNonPositive(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		n := n
		t.Run("", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("NewFloatHeap(%d): expected panic, got none", n)
				}
			}()
			_ = NewFloatHeap(n)
		})
	}
}

// TestFloatHeap_CapacityBoundary stress-tests the bounded behaviour
// of Offer: streaming 1000 values through a capacity-K heap must
// leave the K largest values, polled in ascending order.
func TestFloatHeap_CapacityBoundary(t *testing.T) {
	const (
		stream = 1000
		k      = 17
	)
	r := rand.New(rand.NewPCG(0xDEADBEEF, 0xFEEDFACE))

	values := make([]float32, stream)
	for i := range values {
		values[i] = r.Float32() * 1000
	}

	h := NewFloatHeap(k)
	for _, v := range values {
		h.Offer(v)
	}
	if got, want := h.Size(), k; got != want {
		t.Fatalf("Size after streaming: got %d want %d", got, want)
	}

	// Reference: sort the full stream and take the K largest, then
	// sort those ascending to match the poll order.
	sortedAsc := append([]float32(nil), values...)
	sort.Slice(sortedAsc, func(i, j int) bool { return sortedAsc[i] < sortedAsc[j] })
	wantTopK := sortedAsc[stream-k:] // ascending order, K largest

	for i := 0; i < k; i++ {
		got := h.Poll()
		if got != wantTopK[i] {
			t.Fatalf("Poll #%d: got %v want %v (full want=%v)", i, got, wantTopK[i], wantTopK)
		}
	}
	if got, want := h.Size(), 0; got != want {
		t.Fatalf("Size after full drain: got %d want %d", got, want)
	}
}
