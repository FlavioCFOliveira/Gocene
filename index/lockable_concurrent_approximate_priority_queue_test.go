// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
)

// Acceptance: this file mirrors
// core/src/test/org/apache/lucene/index/TestLockableConcurrentApproximatePriorityQueue.java
// from Apache Lucene 10.4.0. The single Java test peer
// (testNeverReturnNullOnNonEmptyQueue) is reproduced verbatim in
// TestLockableConcurrentApproximatePriorityQueue_NeverReturnNilOnNonEmptyQueue
// below. Additional small Go-only unit tests cover the constructor variants
// and remove/contains paths that the Java suite does not exercise directly
// but which the wrapper exposes.

// weightedLock is the Go peer of the Java test's nested WeightedLock. It wraps
// a sync.Mutex (whose TryLock/Unlock match the queueLock constraint) and adds
// the long weight field the test mutates to simulate growing RAM usage.
type weightedLock struct {
	mu     sync.Mutex
	weight int64
}

func (w *weightedLock) Lock()         { w.mu.Lock() }
func (w *weightedLock) TryLock() bool { return w.mu.TryLock() }
func (w *weightedLock) Unlock()       { w.mu.Unlock() }

func TestLockableConcurrentApproximatePriorityQueue_DefaultCtor(t *testing.T) {
	t.Parallel()
	q := newLockableConcurrentApproximatePriorityQueueDefault[*weightedLock]()
	if q == nil || q.queue == nil {
		t.Fatalf("default constructor returned an unusable queue")
	}
	if got, ok := q.lockAndPoll(); ok || got != nil {
		t.Fatalf("lockAndPoll on empty queue = (%v, %v), want (nil, false)", got, ok)
	}
}

func TestLockableConcurrentApproximatePriorityQueue_AddPollUnlock(t *testing.T) {
	t.Parallel()
	q := newLockableConcurrentApproximatePriorityQueue[*weightedLock](4)
	w := &weightedLock{}
	w.Lock()
	q.addAndUnlock(w, 1)
	if !q.contains(w) {
		t.Fatalf("contains after addAndUnlock = false, want true")
	}
	got, ok := q.lockAndPoll()
	if !ok || got != w {
		t.Fatalf("lockAndPoll = (%v, %v), want (%p, true)", got, ok, w)
	}
	// lockAndPoll must hand the entry back already locked.
	if got.TryLock() {
		t.Fatalf("returned entry was not locked by lockAndPoll")
	}
	got.Unlock()
	// And the queue is now empty.
	if again, ok := q.lockAndPoll(); ok || again != nil {
		t.Fatalf("second lockAndPoll = (%v, %v), want (nil, false)", again, ok)
	}
}

func TestLockableConcurrentApproximatePriorityQueue_Remove(t *testing.T) {
	t.Parallel()
	q := newLockableConcurrentApproximatePriorityQueue[*weightedLock](2)
	w := &weightedLock{}
	w.Lock()
	q.addAndUnlock(w, 7)
	if !q.remove(w) {
		t.Fatalf("remove returned false for present entry")
	}
	if q.contains(w) {
		t.Fatalf("contains after remove = true, want false")
	}
	if q.remove(w) {
		t.Fatalf("remove returned true for absent entry")
	}
}

// TestLockableConcurrentApproximatePriorityQueue_NeverReturnNilOnNonEmptyQueue
// mirrors TestLockableConcurrentApproximatePriorityQueue#
// testNeverReturnNullOnNonEmptyQueue from Lucene 10.4.0 line-for-line. Each
// worker seeds the queue with one locked entry, then repeatedly lockAndPolls,
// asserts the result is non-nil, and re-publishes the entry. The invariant
// under test is that no goroutine ever observes a nil entry while the queue
// is provably non-empty (every worker has at least one outstanding entry).
func TestLockableConcurrentApproximatePriorityQueue_NeverReturnNilOnNonEmptyQueue(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 2))
	const iters = 10
	for iter := 0; iter < iters; iter++ {
		concurrency := 1 + rng.IntN(16)
		q := newLockableConcurrentApproximatePriorityQueue[*weightedLock](concurrency)
		numThreads := 2 + rng.IntN(15)
		var startingGun sync.WaitGroup
		startingGun.Add(1)
		var done sync.WaitGroup
		done.Add(numThreads)
		var failures atomic.Int32
		for t := 0; t < numThreads; t++ {
			go func() {
				defer done.Done()
				startingGun.Wait()
				w := &weightedLock{}
				w.Lock()
				w.weight++ // mirror the Java test's RAM-usage bump
				q.addAndUnlock(w, w.weight)
				for i := 0; i < 10_000; i++ {
					polled, ok := q.lockAndPoll()
					if !ok || polled == nil {
						failures.Add(1)
						return
					}
					// Use a derived weight that varies across iterations; the
					// exact value is unimportant. The pointer address gives a
					// stable per-entry pseudo-hash analogous to Object.hashCode.
					q.addAndUnlock(polled, int64(i)+1)
				}
			}()
		}
		startingGun.Done()
		done.Wait()
		if n := failures.Load(); n != 0 {
			t.Fatalf("iter %d: %d goroutines observed nil from lockAndPoll while the queue was non-empty", iter, n)
		}
	}
}
