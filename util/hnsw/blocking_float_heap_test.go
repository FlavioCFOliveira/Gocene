// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
)

// TestBlockingFloatHeap_BasicOperations is the Go port of
// TestBlockingFloatHeap.testBasicOperations from Lucene 10.4.0.
// With maxSize=3, offering {2,4,1,3} retains {2,3,4} (the largest
// three); peek returns 2, then polling yields 2, 3, 4 in order.
func TestBlockingFloatHeap_BasicOperations(t *testing.T) {
	h := NewBlockingFloatHeap(3)
	h.Offer(2)
	h.Offer(4)
	h.Offer(1)
	h.Offer(3)
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

// TestBlockingFloatHeap_BasicOperations2 is the Go port of
// TestBlockingFloatHeap.testBasicOperations2. A capacity-N heap is
// loaded with N random floats; the polled sequence must be
// non-decreasing, and (because nothing is discarded) the sums before
// and after must match within Lucene's epsilon.
func TestBlockingFloatHeap_BasicOperations2(t *testing.T) {
	// atLeast(10): Lucene returns >=10 with random padding. We pick a
	// deterministic value > 10 so the test is reproducible without
	// depending on RandomizedRunner.
	const size = 25
	h := NewBlockingFloatHeap(size)
	r := rand.New(rand.NewPCG(0xC0FFEE, 0xBADC0DE))

	var sum, sum2 float64
	for i := 0; i < size; i++ {
		next := r.Float32() * 100 // [0, 100)
		sum += float64(next)
		h.Offer(next)
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

// TestBlockingFloatHeap_MultipleThreads is the Go port of
// TestBlockingFloatHeap.testMultipleThreads. A capacity-1 heap is
// hammered by many goroutines, each tracking a local monotonically
// increasing "bottom value". After each Offer the goroutine peeks
// and asserts the global top is >= its own bottom; it then adopts
// the global top as its new bottom.
//
// The invariant being pinned: the global top is monotonically
// non-decreasing across the lifetime of the heap (in a capacity-1
// min-heap, Offer either replaces or keeps the top, never lowers
// it).
func TestBlockingFloatHeap_MultipleThreads(t *testing.T) {
	// randomIntBetween(3, 20) — fix at a value high enough to stress
	// contention without exploding the test runtime on slow hardware.
	const numGoroutines = 12
	const minIterations = 50
	const maxIterations = 200

	r := rand.New(rand.NewPCG(0xFACEFEED, 0xCAFEBABE))
	// Pre-roll per-goroutine iteration counts deterministically.
	iters := make([]int, numGoroutines)
	for i := range iters {
		iters[i] = minIterations + r.IntN(maxIterations-minIterations+1)
	}

	globalHeap := NewBlockingFloatHeap(1)
	// Seed slot 1 with -Inf so the first Offer in every goroutine
	// can observe a "global top" without the special-case of an
	// empty heap (Java does Offer first, so this is moot, but we
	// preserve the structure).
	globalHeap.Offer(float32(math.Inf(-1)))

	var start sync.WaitGroup
	start.Add(1) // gate goroutines until all are constructed (CountDownLatch)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Track if any goroutine reports a violation. Using a flag plus
	// t.Errorf keeps the failure attributable to the right goroutine.
	var fail atomic.Bool

	for g := 0; g < numGoroutines; g++ {
		g := g
		numIterations := iters[g]
		go func() {
			defer wg.Done()
			start.Wait() // CountDownLatch.await

			var bottomValue float32 = 0
			// Per-goroutine RNG seeded from the goroutine index for
			// reproducibility without sharing state.
			lr := rand.New(rand.NewPCG(uint64(g)+1, 0xA5A5A5A5))

			for k := 0; k < numIterations; k++ {
				bottomValue += float32(lr.IntN(6)) // [0,5]
				globalHeap.Offer(bottomValue)
				// (No Thread.sleep — Go's scheduler interleaves
				// goroutines naturally, and sleeping just slows the
				// test without adding coverage.)

				globalTop := globalHeap.Peek()
				if globalTop < bottomValue {
					t.Errorf("goroutine %d iter %d: globalTop=%v < bottomValue=%v",
						g, k, globalTop, bottomValue)
					fail.Store(true)
					return
				}
				bottomValue = globalTop
			}
		}()
	}

	start.Done() // CountDownLatch.countDown
	wg.Wait()
	if fail.Load() {
		t.FailNow()
	}
}

// TestBlockingFloatHeap_OfferReturnsTop pins the Java return-value
// contract: Offer returns the new top (least value) of the heap
// after the call. Distinct from FloatHeap.Offer which returns bool.
func TestBlockingFloatHeap_OfferReturnsTop(t *testing.T) {
	h := NewBlockingFloatHeap(3)
	if got, want := h.Offer(7), float32(7); got != want {
		t.Fatalf("Offer(7) on empty heap: got %v want %v", got, want)
	}
	if got, want := h.Offer(3), float32(3); got != want {
		t.Fatalf("Offer(3) (new min): got %v want %v", got, want)
	}
	if got, want := h.Offer(5), float32(3); got != want {
		t.Fatalf("Offer(5) (heap full now): got %v want %v", got, want)
	}
	// Heap is full {3,5,7}, min=3. Offer 4 — must replace min, new top=4.
	if got, want := h.Offer(4), float32(4); got != want {
		t.Fatalf("Offer(4) on full heap: got %v want %v", got, want)
	}
	// Offer 1 — strictly less than min(4), heap unchanged, top stays 4.
	if got, want := h.Offer(1), float32(4); got != want {
		t.Fatalf("Offer(1) (below min, discarded): got %v want %v", got, want)
	}
}

// TestBlockingFloatHeap_OfferEqualToTop covers the "value >= heap[1]"
// branch (note: >= not >). When value equals the current top, Java
// still calls updateTop; observable top is unchanged, but the path
// is exercised.
func TestBlockingFloatHeap_OfferEqualToTop(t *testing.T) {
	h := NewBlockingFloatHeap(3)
	h.Offer(10)
	h.Offer(20)
	h.Offer(30)
	if got, want := h.Size(), 3; got != want {
		t.Fatalf("Size after fill: got %d want %d", got, want)
	}
	// Equal to min: retained per Java's >= rule.
	if got, want := h.Offer(10), float32(10); got != want {
		t.Fatalf("Offer(10) equal-to-min: got %v want %v", got, want)
	}
	if got, want := h.Size(), 3; got != want {
		t.Fatalf("Size after equal-to-min: got %d want %d", got, want)
	}
	// Drain and confirm we still have one 10, one 20, one 30.
	got := []float32{h.Poll(), h.Poll(), h.Poll()}
	want := []float32{10, 20, 30}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("drain[%d]: got %v want %v (full got=%v)", i, got[i], want[i], got)
		}
	}
}

// TestBlockingFloatHeap_OfferMany exercises the bulk-insert path,
// which expects ascending-sorted input. It must produce the same
// final state as offering values one by one, and it must honour the
// early-break on values[i] < heap[1] (verified indirectly: the
// final state is correct even when many low values would otherwise
// be processed).
func TestBlockingFloatHeap_OfferMany(t *testing.T) {
	t.Run("partial-fill", func(t *testing.T) {
		h := NewBlockingFloatHeap(5)
		// Below maxSize: all four values are pushed.
		got := h.OfferMany([]float32{1, 3, 5, 7}, 4)
		if got != 1 {
			t.Fatalf("OfferMany partial: top got %v want 1", got)
		}
		if h.Size() != 4 {
			t.Fatalf("Size: got %d want 4", h.Size())
		}
	})

	t.Run("overflow-with-ascending", func(t *testing.T) {
		// maxSize=3. Insert 1..6 ascending. Walked from largest down:
		//   i=5: push 6  (size 0->1)
		//   i=4: push 5  (size 1->2)
		//   i=3: push 4  (size 2->3, heap full)
		//   i=2: 3 >= top(4)? No -> break.
		// Final heap: {4,5,6}, top=4.
		h := NewBlockingFloatHeap(3)
		got := h.OfferMany([]float32{1, 2, 3, 4, 5, 6}, 6)
		if got != 4 {
			t.Fatalf("OfferMany overflow: top got %v want 4", got)
		}
		drained := []float32{h.Poll(), h.Poll(), h.Poll()}
		want := []float32{4, 5, 6}
		for i := range drained {
			if drained[i] != want[i] {
				t.Fatalf("drain[%d]: got %v want %v (full=%v)", i, drained[i], want[i], drained)
			}
		}
	})

	t.Run("length-shorter-than-slice", func(t *testing.T) {
		// length parameter must be honoured: only the first `len`
		// elements are inserted (those are sorted; tail is ignored).
		h := NewBlockingFloatHeap(10)
		got := h.OfferMany([]float32{1, 2, 3, 99, 100}, 3)
		if got != 1 {
			t.Fatalf("top after partial OfferMany: got %v want 1", got)
		}
		if h.Size() != 3 {
			t.Fatalf("Size: got %d want 3", h.Size())
		}
	})

	t.Run("zero-length-no-op", func(t *testing.T) {
		h := NewBlockingFloatHeap(3)
		h.Offer(42)
		got := h.OfferMany([]float32{1, 2, 3}, 0)
		if got != 42 {
			t.Fatalf("OfferMany length=0: top got %v want 42 (unchanged)", got)
		}
		if h.Size() != 1 {
			t.Fatalf("Size after no-op OfferMany: got %d want 1", h.Size())
		}
	})

	t.Run("matches-single-offer-loop", func(t *testing.T) {
		// Streaming the same ascending sequence through Offer and
		// OfferMany must produce the same final drain order.
		input := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

		a := NewBlockingFloatHeap(5)
		for _, v := range input {
			a.Offer(v)
		}
		var aDrain []float32
		for a.Size() > 0 {
			aDrain = append(aDrain, a.Poll())
		}

		b := NewBlockingFloatHeap(5)
		b.OfferMany(input, len(input))
		var bDrain []float32
		for b.Size() > 0 {
			bDrain = append(bDrain, b.Poll())
		}

		if len(aDrain) != len(bDrain) {
			t.Fatalf("drain lengths differ: Offer=%v OfferMany=%v", aDrain, bDrain)
		}
		for i := range aDrain {
			if aDrain[i] != bDrain[i] {
				t.Fatalf("drain[%d] differs: Offer=%v OfferMany=%v", i, aDrain, bDrain)
			}
		}
	})
}

// TestBlockingFloatHeap_PollEmptyPanics confirms Poll on an empty
// heap panics with ErrEmptyHeap, mirroring Java's
// IllegalStateException. (Not present in the Java test peer; added
// to lock in the contract documented in BlockingFloatHeap.poll.)
func TestBlockingFloatHeap_PollEmptyPanics(t *testing.T) {
	h := NewBlockingFloatHeap(2)
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

// TestBlockingFloatHeap_NewRejectsNonPositive locks in the
// constructor's input contract.
func TestBlockingFloatHeap_NewRejectsNonPositive(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		n := n
		t.Run("", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("NewBlockingFloatHeap(%d): expected panic, got none", n)
				}
			}()
			_ = NewBlockingFloatHeap(n)
		})
	}
}

// TestBlockingFloatHeap_ConcurrentOfferPoll stresses the lock under
// mixed offer/poll traffic. The end-to-end invariant we pin: at
// every moment the heap's Size() reads back coherently (>=0,
// <=maxSize) and a final drain yields a non-decreasing sequence.
//
// This is intentionally heavier than the Java test peer, which only
// exercises a capacity-1 heap. With a larger capacity we expose
// races in the multi-level sift code paths.
func TestBlockingFloatHeap_ConcurrentOfferPoll(t *testing.T) {
	const (
		capacity  = 64
		producers = 8
		pollers   = 4
		perProd   = 500
	)
	h := NewBlockingFloatHeap(capacity)

	var producerWG sync.WaitGroup
	producerWG.Add(producers)
	for p := 0; p < producers; p++ {
		p := p
		go func() {
			defer producerWG.Done()
			lr := rand.New(rand.NewPCG(uint64(p)+1, 0x12345678))
			for i := 0; i < perProd; i++ {
				h.Offer(lr.Float32() * 1000)
			}
		}()
	}

	// Pollers run concurrently and stop once producers are done AND
	// the heap is empty. They also assert Size() bounds.
	done := make(chan struct{})
	var pollerWG sync.WaitGroup
	pollerWG.Add(pollers)
	for p := 0; p < pollers; p++ {
		go func() {
			defer pollerWG.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				// Peek size; if the heap has data, drain one element.
				// Otherwise yield by going around the loop.
				if sz := h.Size(); sz < 0 || sz > capacity {
					t.Errorf("Size out of bounds during contention: %d", sz)
					return
				}
				// Best-effort poll: guard with size check inside the
				// loop because we have multiple pollers competing.
				if h.Size() > 0 {
					// Recover from the race where another poller
					// drained the last element between our Size() and
					// Poll().
					func() {
						defer func() { _ = recover() }()
						_ = h.Poll()
					}()
				}
			}
		}()
	}

	producerWG.Wait()
	close(done)
	pollerWG.Wait()

	// Final drain on the main goroutine — must be monotonic.
	last := float32(math.Inf(-1))
	for h.Size() > 0 {
		v := h.Poll()
		if v < last {
			t.Fatalf("final drain not monotonic: got %v after %v", v, last)
		}
		last = v
	}
	if got := h.Size(); got != 0 {
		t.Fatalf("Size after final drain: got %d want 0", got)
	}
}

// TestBlockingFloatHeap_ConcurrentTopKCorrectness streams a known
// set of values through many goroutines into a top-K heap. After
// all producers finish, draining must yield exactly the K largest
// values in ascending order, regardless of interleaving.
func TestBlockingFloatHeap_ConcurrentTopKCorrectness(t *testing.T) {
	const (
		k         = 17
		producers = 8
		perProd   = 250
	)
	totalValues := producers * perProd

	r := rand.New(rand.NewPCG(0xDEADBEEF, 0xFEEDFACE))
	values := make([]float32, totalValues)
	for i := range values {
		values[i] = r.Float32() * 10000
	}

	h := NewBlockingFloatHeap(k)
	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		p := p
		go func() {
			defer wg.Done()
			start := p * perProd
			end := start + perProd
			for i := start; i < end; i++ {
				h.Offer(values[i])
			}
		}()
	}
	wg.Wait()

	// Reference: sort all values descending, take top K, then sort
	// ascending to match poll order.
	sorted := append([]float32(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	wantTopK := sorted[totalValues-k:]

	if got, want := h.Size(), k; got != want {
		t.Fatalf("Size after concurrent fill: got %d want %d", got, want)
	}
	for i := 0; i < k; i++ {
		got := h.Poll()
		if got != wantTopK[i] {
			t.Fatalf("Poll #%d: got %v want %v (full want=%v)", i, got, wantTopK[i], wantTopK)
		}
	}
}

// TestBlockingFloatHeap_ConcurrentOfferMany exercises the bulk path
// under contention. Each goroutine offers a sorted block; the final
// state must contain the top-K values across the full union.
func TestBlockingFloatHeap_ConcurrentOfferMany(t *testing.T) {
	const (
		k         = 13
		producers = 6
		perProd   = 200
	)
	totalValues := producers * perProd

	r := rand.New(rand.NewPCG(0xABCDEF01, 0x10FEDCBA))
	values := make([]float32, totalValues)
	for i := range values {
		values[i] = r.Float32() * 5000
	}

	h := NewBlockingFloatHeap(k)
	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		p := p
		go func() {
			defer wg.Done()
			block := append([]float32(nil), values[p*perProd:(p+1)*perProd]...)
			sort.Slice(block, func(i, j int) bool { return block[i] < block[j] })
			h.OfferMany(block, len(block))
		}()
	}
	wg.Wait()

	sorted := append([]float32(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	wantTopK := sorted[totalValues-k:]

	if got, want := h.Size(), k; got != want {
		t.Fatalf("Size after concurrent OfferMany: got %d want %d", got, want)
	}
	for i := 0; i < k; i++ {
		got := h.Poll()
		if got != wantTopK[i] {
			t.Fatalf("Poll #%d: got %v want %v (full want=%v)", i, got, wantTopK[i], wantTopK)
		}
	}
}
