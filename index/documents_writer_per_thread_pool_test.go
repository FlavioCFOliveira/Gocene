// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// Apache Lucene 10.4.0 does not ship a TestDocumentsWriterPerThreadPool peer;
// the pool is exercised only indirectly through TestIndexWriter and the
// indexing stress suite. The cases below cover the surface the Java pool
// exposes (constructor, getAndLock, marksAsFreeAndUnlock, filterAndLock,
// checkout, lockNewWriters/unlockNewWriters, close) using a minimal stub DWPT
// — the same pattern lockable_concurrent_approximate_priority_queue_test.go
// uses to test the LCAPQ wrapper without dragging in a full DWPT.

// stubDWPT is the test peer of DocumentsWriterPerThread. It implements the
// pooledDWPT contract with a sync.Mutex (TryLock/Lock/Unlock) plus
// configurable state predicates. The owner field tracks the goroutine that
// currently holds the lock so IsHeldByCurrentThread can be modelled in a
// way Go's sync.Mutex does not natively support.
type stubDWPT struct {
	mu       sync.Mutex
	owner    atomic.Int64 // goroutine "id" — opaque token assigned at Lock
	ram      atomic.Int64
	flushing atomic.Bool
	aborted  atomic.Bool
	qadv     atomic.Bool
}

var stubGoroutineID atomic.Int64

// nextStubGoroutineID assigns a unique non-zero token per goroutine on first
// call. Test code holds the token in a goroutine-local slot via runtime.GID
// substitutes; for the tests below we just call this directly inside the
// goroutine that takes the lock and stash it on the stub.
func nextStubGoroutineID() int64 { return stubGoroutineID.Add(1) }

func (s *stubDWPT) Lock() {
	s.mu.Lock()
	s.owner.Store(nextStubGoroutineID())
}

func (s *stubDWPT) TryLock() bool {
	if !s.mu.TryLock() {
		return false
	}
	s.owner.Store(nextStubGoroutineID())
	return true
}

func (s *stubDWPT) Unlock() {
	s.owner.Store(0)
	s.mu.Unlock()
}

// IsHeldByCurrentThread reports whether a non-zero owner is set. The tests
// only call checkout from the goroutine that took the lock, so a non-zero
// owner is a sufficient proxy without runtime.GID gymnastics.
func (s *stubDWPT) IsHeldByCurrentThread() bool { return s.owner.Load() != 0 }
func (s *stubDWPT) RamBytesUsed() int64         { return s.ram.Load() }
func (s *stubDWPT) IsFlushPending() bool        { return s.flushing.Load() }
func (s *stubDWPT) IsAborted() bool             { return s.aborted.Load() }
func (s *stubDWPT) IsQueueAdvanced() bool       { return s.qadv.Load() }

// newStubFactory returns a factory that mints fresh stubs and tracks how many
// times it has been invoked, so tests can assert that the pool only mints on
// free-list miss.
func newStubFactory() (func() *stubDWPT, *atomic.Int32) {
	var n atomic.Int32
	return func() *stubDWPT {
		n.Add(1)
		return &stubDWPT{}
	}, &n
}

func TestDocumentsWriterPerThreadPool_GetAndLockMintsOnEmpty(t *testing.T) {
	t.Parallel()
	factory, mints := newStubFactory()
	pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)

	dwpt := pool.getAndLock()
	if dwpt == nil {
		t.Fatal("getAndLock returned nil on empty pool")
	}
	if got := mints.Load(); got != 1 {
		t.Fatalf("factory invocations = %d, want 1", got)
	}
	if !dwpt.IsHeldByCurrentThread() {
		t.Fatal("newly-minted DWPT must be returned locked")
	}
	if pool.size() != 1 {
		t.Fatalf("pool.size() = %d, want 1", pool.size())
	}
	if !pool.isRegistered(dwpt) {
		t.Fatal("newly-minted DWPT must be registered in the pool")
	}
}

func TestDocumentsWriterPerThreadPool_MarksAsFreeAndUnlockEnablesReuse(t *testing.T) {
	t.Parallel()
	factory, mints := newStubFactory()
	pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)

	first := pool.getAndLock()
	first.ram.Store(1024)
	pool.marksAsFreeAndUnlock(first)

	// Second getAndLock must reuse the first DWPT — no new factory call.
	second := pool.getAndLock()
	if second != first {
		t.Fatalf("getAndLock after marksAsFreeAndUnlock returned a different DWPT (%p vs %p)", second, first)
	}
	if got := mints.Load(); got != 1 {
		t.Fatalf("factory invocations = %d, want 1 (reuse expected)", got)
	}
	if !second.IsHeldByCurrentThread() {
		t.Fatal("reused DWPT must be returned locked")
	}
}

func TestDocumentsWriterPerThreadPool_MarksAsFreeRejectsDirtyState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(*stubDWPT)
		want  func(assertionViolation) bool
	}{
		{
			name:  "flushPending",
			setup: func(d *stubDWPT) { d.flushing.Store(true) },
			want:  func(v assertionViolation) bool { return v.FlushPending },
		},
		{
			name:  "aborted",
			setup: func(d *stubDWPT) { d.aborted.Store(true) },
			want:  func(v assertionViolation) bool { return v.Aborted },
		},
		{
			name:  "queueAdvanced",
			setup: func(d *stubDWPT) { d.qadv.Store(true) },
			want:  func(v assertionViolation) bool { return v.QueueAdv },
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			factory, _ := newStubFactory()
			pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)
			dwpt := pool.getAndLock()
			tc.setup(dwpt)

			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("expected panic on dirty marksAsFreeAndUnlock, got none")
				}
				v, ok := r.(assertionViolation)
				if !ok {
					t.Fatalf("panic value type = %T, want assertionViolation", r)
				}
				if !tc.want(v) {
					t.Fatalf("expected predicate not flagged in %+v", v)
				}
			}()
			pool.marksAsFreeAndUnlock(dwpt)
		})
	}
}

func TestDocumentsWriterPerThreadPool_FilterAndLockSkipsCheckedOut(t *testing.T) {
	t.Parallel()
	factory, _ := newStubFactory()
	pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)

	// Force two distinct DWPTs by taking the first while a second is minted.
	a := pool.getAndLock()
	b := pool.getAndLock()
	pool.marksAsFreeAndUnlock(a)
	pool.marksAsFreeAndUnlock(b)
	if a == b {
		t.Fatalf("test setup invariant: a and b must be distinct DWPTs")
	}

	// Check b out of the pool before filterAndLock runs.
	b.Lock()
	if !pool.checkout(b) {
		t.Fatal("checkout(b) returned false")
	}
	b.Unlock()

	got := pool.filterAndLock(func(*stubDWPT) bool { return true })
	if len(got) != 1 || got[0] != a {
		t.Fatalf("filterAndLock = %v, want [a]", got)
	}
	if !got[0].IsHeldByCurrentThread() {
		t.Fatal("filterAndLock entries must be returned locked")
	}
	got[0].Unlock()
}

func TestDocumentsWriterPerThreadPool_CheckoutUnregisters(t *testing.T) {
	t.Parallel()
	factory, _ := newStubFactory()
	pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)

	dwpt := pool.getAndLock()
	if !pool.checkout(dwpt) {
		t.Fatal("checkout returned false on registered DWPT")
	}
	if pool.isRegistered(dwpt) {
		t.Fatal("DWPT must be removed from pool after checkout")
	}
	if pool.checkout(dwpt) {
		t.Fatal("second checkout must return false")
	}
}

func TestDocumentsWriterPerThreadPool_CheckoutRequiresHeldLock(t *testing.T) {
	t.Parallel()
	factory, _ := newStubFactory()
	pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)

	dwpt := pool.getAndLock()
	dwpt.Unlock()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("checkout without lock-ownership must panic")
		}
	}()
	pool.checkout(dwpt)
}

func TestDocumentsWriterPerThreadPool_LockNewWritersBlocks(t *testing.T) {
	t.Parallel()
	factory, mints := newStubFactory()
	pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)

	pool.lockNewWriters()
	done := make(chan struct{})
	go func() {
		_ = pool.getAndLock()
		close(done)
	}()

	// getAndLock should be parked inside newWriter waiting for the permit.
	select {
	case <-done:
		t.Fatal("getAndLock returned while new-writer permit was held")
	default:
	}
	if got := mints.Load(); got != 0 {
		t.Fatalf("factory invoked while permit held: %d", got)
	}

	pool.unlockNewWriters()
	<-done
	if got := mints.Load(); got != 1 {
		t.Fatalf("factory invocations after unlock = %d, want 1", got)
	}
}

func TestDocumentsWriterPerThreadPool_CloseRejectsFurtherGets(t *testing.T) {
	t.Parallel()
	factory, _ := newStubFactory()
	pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)
	pool.close()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("getAndLock after close must panic")
		}
		var ace *AlreadyClosedException
		if !errors.As(asError(r), &ace) {
			t.Fatalf("panic value type = %T, want *AlreadyClosedException", r)
		}
	}()
	pool.getAndLock()
}

// asError narrows panic recover() results to error, returning the original
// value when it already satisfies the interface and wrapping otherwise. Lets
// errors.As inspect AlreadyClosedException panics.
func asError(v any) error {
	if e, ok := v.(error); ok {
		return e
	}
	return errors.New("panic value is not an error")
}

func TestDocumentsWriterPerThreadPool_SnapshotIsDefensive(t *testing.T) {
	t.Parallel()
	factory, _ := newStubFactory()
	pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)

	a := pool.getAndLock()
	pool.marksAsFreeAndUnlock(a)

	snap := pool.snapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot len = %d, want 1", len(snap))
	}

	// Adding another DWPT after the snapshot must not retroactively appear in
	// it — confirms the slice is a copy, not a live view.
	b := pool.getAndLock()
	pool.marksAsFreeAndUnlock(b)
	if len(snap) != 1 {
		t.Fatalf("snapshot len changed after pool growth = %d, want 1", len(snap))
	}
}

func TestDocumentsWriterPerThreadPool_ConcurrentGetAndLockReuses(t *testing.T) {
	t.Parallel()
	factory, mints := newStubFactory()
	pool := newDocumentsWriterPerThreadPool[*stubDWPT](factory)

	const workers = 8
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				dwpt := pool.getAndLock()
				dwpt.ram.Store(int64(j))
				pool.marksAsFreeAndUnlock(dwpt)
			}
		}()
	}
	wg.Wait()

	// Mint count must be bounded by the number of workers (each worker may at
	// most cause one cold miss before reuse stabilises). Exceeding that bound
	// would indicate the free list is not being consulted on getAndLock.
	if got := mints.Load(); got > int32(workers) {
		t.Fatalf("factory invocations = %d, want <= %d", got, workers)
	}
	if pool.size() < 1 || pool.size() > workers {
		t.Fatalf("pool.size() = %d, want in [1,%d]", pool.size(), workers)
	}
}
