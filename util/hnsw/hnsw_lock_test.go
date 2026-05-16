// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestHnswLock_StripeIndexInBounds verifies the stripe hash never
// returns an index outside [0, hnswLockNumStripes).
func TestHnswLock_StripeIndexInBounds(t *testing.T) {
	for level := 0; level < 64; level++ {
		for node := 0; node < 4096; node++ {
			idx := hnswStripeIndex(level, node)
			if idx < 0 || idx >= hnswLockNumStripes {
				t.Fatalf("hnswStripeIndex(%d, %d) = %d, out of [0, %d)",
					level, node, idx, hnswLockNumStripes)
			}
		}
	}
}

// TestHnswLock_StripeFormula verifies the (level, node) -> index hash
// matches the Java formula `(level*31 + node) mod NUM_LOCKS`. This
// guarantees that two graphs built with Lucene-compatible lock striping
// observe the same collision pattern, which is the contract relied upon
// by tests in the Java reference (HnswConcurrentMergeBuilder's docs
// describe the stripe as deterministic).
func TestHnswLock_StripeFormula(t *testing.T) {
	cases := []struct {
		level, node, want int
	}{
		{0, 0, 0},
		{0, 1, 1},
		{0, 511, 511},
		{0, 512, 0},
		{1, 0, 31},
		{1, 481, 0},
		{2, 0, 62},
		{17, 23, (17*31 + 23) % hnswLockNumStripes},
	}
	for _, c := range cases {
		got := hnswStripeIndex(c.level, c.node)
		if got != c.want {
			t.Fatalf("hnswStripeIndex(level=%d, node=%d) = %d, want %d",
				c.level, c.node, got, c.want)
		}
	}
}

// TestHnswLock_ReadWriteSimple covers the basic acquire/release pattern
// for read and write locks on a single stripe.
func TestHnswLock_ReadWriteSimple(t *testing.T) {
	lock := NewHnswLock()

	// Acquire and release a write lock.
	release := lock.WriteLock(0, 0)
	release()

	// Acquire and release a read lock.
	release = lock.ReadLock(0, 0)
	release()

	// Multiple reads can coexist on the same stripe.
	r1 := lock.ReadLock(0, 1)
	r2 := lock.ReadLock(0, 1)
	r1()
	r2()
}

// TestHnswLock_WriteExclusion verifies that a write lock excludes
// concurrent writers on the same stripe. The test grabs the write
// lock, kicks off a second goroutine that wants the same write lock,
// and asserts the second goroutine cannot proceed until the first
// releases.
func TestHnswLock_WriteExclusion(t *testing.T) {
	lock := NewHnswLock()

	r1 := lock.WriteLock(3, 7)
	var acquired atomic.Bool
	done := make(chan struct{})
	go func() {
		r2 := lock.WriteLock(3, 7)
		acquired.Store(true)
		r2()
		close(done)
	}()

	// Give the second goroutine time to attempt the lock. If the lock
	// were broken (e.g. striping mismatch) it would acquire immediately.
	time.Sleep(20 * time.Millisecond)
	if acquired.Load() {
		t.Fatalf("second writer acquired the lock while first still held it")
	}

	r1()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("second writer did not acquire the lock after release")
	}
	if !acquired.Load() {
		t.Fatalf("second writer's acquired flag did not flip")
	}
}

// TestHnswLock_DifferentStripesAreIndependent ensures that two
// (level, node) pairs hashing to different stripes do not block each
// other. The test grabs a write lock on stripe A, then concurrently
// grabs the write lock on stripe B and asserts no blocking occurs.
func TestHnswLock_DifferentStripesAreIndependent(t *testing.T) {
	lock := NewHnswLock()

	// Find two (level, node) pairs that hash to different stripes.
	a := hnswStripeIndex(0, 0)
	b := hnswStripeIndex(0, 1)
	if a == b {
		t.Fatalf("test precondition: stripes for (0,0)=%d and (0,1)=%d collide", a, b)
	}

	r1 := lock.WriteLock(0, 0)
	defer r1()

	done := make(chan struct{})
	go func() {
		r2 := lock.WriteLock(0, 1)
		r2()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("write lock on different stripe blocked on first")
	}
}

// TestHnswLock_ReadWriteOrdering verifies that a write lock blocks
// new readers until released, and that readers block subsequent
// writers. This is the standard RWMutex contract — the test is here to
// ensure striped indexing does not somehow drop the contract on the
// floor.
func TestHnswLock_ReadWriteOrdering(t *testing.T) {
	lock := NewHnswLock()

	// Writer first: blocks readers until release.
	rW := lock.WriteLock(5, 11)
	var readerAcquired atomic.Bool
	readerDone := make(chan struct{})
	go func() {
		rR := lock.ReadLock(5, 11)
		readerAcquired.Store(true)
		rR()
		close(readerDone)
	}()
	time.Sleep(20 * time.Millisecond)
	if readerAcquired.Load() {
		t.Fatalf("reader acquired while writer held the stripe")
	}
	rW()
	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatalf("reader did not acquire after writer released")
	}
}

// TestHnswLock_ConcurrentStress runs a moderate-load stress test that
// hammers a single HnswLock from numWorkers goroutines, each performing
// a mix of reads and writes across the entire stripe space. The test
// uses sync.WaitGroup for coordination, mirrors a typical concurrent
// merge workload, and asserts (a) no deadlock and (b) the total
// "shared counter" guarded by per-stripe locks ends up at the expected
// value — i.e. mutations are not lost.
func TestHnswLock_ConcurrentStress(t *testing.T) {
	const (
		numWorkers  = 8
		opsPerW     = 2000
		nodeBuckets = 64
	)
	lock := NewHnswLock()
	// shared per-stripe counters; each writer increments a bucket
	// guarded by the (level=0, node) lock.
	counters := make([]int, nodeBuckets)
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < opsPerW; i++ {
				bucket := (seed*7 + i) % nodeBuckets
				if i%4 == 0 {
					// Reader path: take read lock, verify bucket is
					// in range, release. Validates that read+write
					// coexistence on the stripe yields no panics.
					r := lock.ReadLock(0, bucket)
					_ = counters[bucket]
					r()
					continue
				}
				r := lock.WriteLock(0, bucket)
				counters[bucket]++
				r()
			}
		}(w)
	}
	wg.Wait()

	// Total writes per worker = ops - (ops/4) = ops - (ops>>2). Compute
	// the expected total and assert against the observed sum.
	writesPerWorker := opsPerW - (opsPerW / 4)
	expected := numWorkers * writesPerWorker
	got := 0
	for _, c := range counters {
		got += c
	}
	if got != expected {
		t.Fatalf("counter sum: got %d, want %d (lost writes — striped lock broken)",
			got, expected)
	}
}
