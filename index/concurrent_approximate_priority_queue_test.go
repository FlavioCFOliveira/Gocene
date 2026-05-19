// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"
	"sync/atomic"
	"testing"
)

// Acceptance: covers the public surface of concurrentApproximatePriorityQueue
// (constructor validation, add/poll, contains/remove, concurrent producers/
// consumers) and the inlined approximatePriorityQueue helper's slot layout,
// sparse-vs-dense behaviour, and isEmpty invariant.

func acceptAny(any) bool { return true }

func TestApproximatePriorityQueue_AddPoll_SparseSlot(t *testing.T) {
	q := newApproximatePriorityQueue()
	if !q.isEmpty() {
		t.Fatalf("new queue should be empty")
	}
	// weight = 1 << 62 → 1 leading zero → expectedSlot = 1.
	q.add("hi", int64(1)<<62)
	if q.isEmpty() {
		t.Fatalf("queue should not be empty after add")
	}
	if q.usedSlots != 1<<1 {
		t.Fatalf("usedSlots = %b, want bit 1 set", q.usedSlots)
	}
	got, ok := q.poll(acceptAny)
	if !ok || got != "hi" {
		t.Fatalf("poll = (%v, %v), want (hi, true)", got, ok)
	}
	if !q.isEmpty() {
		t.Fatalf("queue should be empty after polling sole entry")
	}
}

func TestApproximatePriorityQueue_AddPoll_DenseSlot(t *testing.T) {
	q := newApproximatePriorityQueue()
	// Fill every sparse slot with weight that maps to slot 0
	// (max weight → 0 leading zeros). 64 inserts saturate the bitset.
	const heavy = int64(-1) // 0xFFFFFFFFFFFFFFFF → 0 leading zeros
	for i := 0; i < longSize; i++ {
		q.add(i, heavy)
	}
	if q.usedSlots+1 != 0 {
		t.Fatalf("expected all sparse slots used")
	}
	// 65th add must spill into the dense tail.
	q.add("dense", heavy)
	if len(q.slots) != longSize+1 {
		t.Fatalf("len(slots) = %d, want %d", len(q.slots), longSize+1)
	}
	// Predicate selecting only the dense entry must remove it from the tail.
	got, ok := q.poll(func(v any) bool { _, isStr := v.(string); return isStr })
	if !ok || got != "dense" {
		t.Fatalf("poll dense = (%v, %v), want (dense, true)", got, ok)
	}
	if len(q.slots) != longSize {
		t.Fatalf("len(slots) = %d, want %d after dense remove", len(q.slots), longSize)
	}
}

func TestApproximatePriorityQueue_ContainsRemove(t *testing.T) {
	q := newApproximatePriorityQueue()
	q.add("a", 1)
	q.add("b", 2)
	if !q.contains("a") || !q.contains("b") {
		t.Fatalf("contains expected true for inserted entries")
	}
	if q.contains("c") {
		t.Fatalf("contains expected false for absent entry")
	}
	if !q.remove("a") {
		t.Fatalf("remove expected true for present entry")
	}
	if q.remove("a") {
		t.Fatalf("remove expected false on second call")
	}
	if !q.remove("b") {
		t.Fatalf("remove expected true for b")
	}
	if !q.isEmpty() {
		t.Fatalf("queue should be empty after removing all entries")
	}
}

func TestApproximatePriorityQueue_NilPanics(t *testing.T) {
	q := newApproximatePriorityQueue()
	for name, fn := range map[string]func(){
		"add":      func() { q.add(nil, 1) },
		"contains": func() { q.contains(nil) },
		"remove":   func() { q.remove(nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s(nil) should panic", name)
				}
			}()
			fn()
		})
	}
}

func TestConcurrentApproximatePriorityQueue_ConstructorBounds(t *testing.T) {
	cases := []int{-1, 0, maxQueueConcurrency + 1}
	for _, c := range cases {
		c := c
		t.Run("", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("concurrency=%d should panic", c)
				}
			}()
			newConcurrentApproximatePriorityQueue(c)
		})
	}
	// Valid bounds must succeed.
	for _, c := range []int{minQueueConcurrency, maxQueueConcurrency} {
		q := newConcurrentApproximatePriorityQueue(c)
		if q.concurrency != c {
			t.Fatalf("concurrency = %d, want %d", q.concurrency, c)
		}
		if len(q.queues) != c || len(q.locks) != c {
			t.Fatalf("internal arrays must match concurrency")
		}
	}
	// Default constructor must respect bounds.
	def := newConcurrentApproximatePriorityQueueDefault()
	if def.concurrency < minQueueConcurrency || def.concurrency > maxQueueConcurrency {
		t.Fatalf("default concurrency %d out of range", def.concurrency)
	}
}

func TestConcurrentApproximatePriorityQueue_AddPollSingle(t *testing.T) {
	q := newConcurrentApproximatePriorityQueue(4)
	q.add("x", 100)
	if !q.contains("x") {
		t.Fatalf("contains expected true")
	}
	got, ok := q.poll(acceptAny)
	if !ok || got != "x" {
		t.Fatalf("poll = (%v, %v), want (x, true)", got, ok)
	}
	if _, ok := q.poll(acceptAny); ok {
		t.Fatalf("second poll should be empty")
	}
}

func TestConcurrentApproximatePriorityQueue_PollPredicateSkip(t *testing.T) {
	q := newConcurrentApproximatePriorityQueue(2)
	q.add("keep", 5)
	q.add("skip", 7)
	got, ok := q.poll(func(v any) bool { return v == "keep" })
	if !ok || got != "keep" {
		t.Fatalf("poll = (%v, %v), want (keep, true)", got, ok)
	}
	if !q.contains("skip") {
		t.Fatalf("skipped entry must remain in queue")
	}
}

func TestConcurrentApproximatePriorityQueue_Remove(t *testing.T) {
	q := newConcurrentApproximatePriorityQueue(3)
	q.add("a", 1)
	q.add("b", 2)
	if !q.remove("a") {
		t.Fatalf("remove a expected true")
	}
	if q.remove("a") {
		t.Fatalf("remove a second time expected false")
	}
	if !q.remove("b") {
		t.Fatalf("remove b expected true")
	}
}

func TestConcurrentApproximatePriorityQueue_Concurrent(t *testing.T) {
	const (
		producers      = 8
		entriesPerProd = 256
	)
	q := newConcurrentApproximatePriorityQueue(4)

	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		p := p
		go func() {
			defer wg.Done()
			for i := 0; i < entriesPerProd; i++ {
				q.add(p*entriesPerProd+i, int64(i+1))
			}
		}()
	}
	wg.Wait()

	var polled atomic.Int64
	wg.Add(producers)
	for c := 0; c < producers; c++ {
		go func() {
			defer wg.Done()
			for {
				_, ok := q.poll(acceptAny)
				if !ok {
					return
				}
				polled.Add(1)
			}
		}()
	}
	wg.Wait()

	if got, want := polled.Load(), int64(producers*entriesPerProd); got != want {
		t.Fatalf("polled = %d, want %d", got, want)
	}
	if _, ok := q.poll(acceptAny); ok {
		t.Fatalf("queue should be drained")
	}
}
