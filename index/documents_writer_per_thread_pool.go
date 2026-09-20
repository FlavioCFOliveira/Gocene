// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocumentsWriterPerThreadPool controls DocumentsWriterPerThread instances and
// their thread assignments during indexing.
// This is the Go port of org.apache.lucene.index.DocumentsWriterPerThreadPool.
type DocumentsWriterPerThreadPool struct {
	mu sync.Mutex

	dwpts map[*DocumentsWriterPerThread]struct{}
	// freeList manages available DWPTs, biasing towards those with higher RAM usage
	// to balance segments.
	freeList *lockableConcurrentApproximatePriorityQueue

	dwptFactory func() *DocumentsWriterPerThread
	// takenWriterPermits is used as a semaphore to block creation of new writers.
	takenWriterPermits int
	// closed renders Java's `private volatile boolean closed`: it is read by
	// ensureOpen, which Java does not declare synchronized, so it must be
	// readable without holding pool.mu.
	closed atomic.Bool
	cond   *sync.Cond
}

func NewDocumentsWriterPerThreadPool(dwptFactory func() *DocumentsWriterPerThread) *DocumentsWriterPerThreadPool {
	pool := &DocumentsWriterPerThreadPool{
		dwpts:       make(map[*DocumentsWriterPerThread]struct{}),
		dwptFactory: dwptFactory,
	}
	pool.cond = sync.NewCond(&pool.mu)
	pool.freeList = newLockableConcurrentApproximatePriorityQueue()
	return pool
}

func (pool *DocumentsWriterPerThreadPool) Size() int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return len(pool.dwpts)
}

func (pool *DocumentsWriterPerThreadPool) LockNewWriters() {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.takenWriterPermits++
}

func (pool *DocumentsWriterPerThreadPool) UnlockNewWriters() {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.takenWriterPermits--
	if pool.takenWriterPermits == 0 {
		pool.cond.Broadcast()
	}
}

// newWriter returns a new, already locked DocumentsWriterPerThread.
//
// Java declares this method `private synchronized`, so it acquires the pool
// monitor itself; callers must not hold it.
func (pool *DocumentsWriterPerThreadPool) newWriter(owner util.LockOwner) *DocumentsWriterPerThread {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	for pool.takenWriterPermits > 0 {
		pool.cond.Wait()
	}

	if err := pool.ensureOpen(); err != nil {
		panic(err)
	}

	dwpt := pool.dwptFactory()
	dwpt.Lock(owner) // lock so nobody else will get this DWPT
	pool.dwpts[dwpt] = struct{}{}
	return dwpt
}

func (pool *DocumentsWriterPerThreadPool) GetAndLock(owner util.LockOwner) *DocumentsWriterPerThread {
	if err := pool.ensureOpen(); err != nil {
		panic(err)
	}

	dwpt := pool.freeList.lockAndPoll(owner)
	if dwpt != nil {
		return dwpt
	}

	// newWriter() adds the DWPT to the `dwpts` set as a side-effect. However it
	// is not added to `freeList` at this point, it will be added later on once
	// DocumentsWriter has indexed a document into this DWPT and then gives it
	// back to the pool by calling MarksAsFreeAndUnlock.
	//
	// Java's getAndLock is not synchronized; newWriter acquires the monitor.
	return pool.newWriter(owner)
}

// ensureOpen mirrors Java's `private void ensureOpen()`, which is deliberately
// not synchronized: it only reads the volatile `closed` flag. It is called from
// newWriter, which already holds the pool monitor, so taking pool.mu here would
// self-deadlock -- Java's monitors are reentrant, Go's sync.Mutex is not.
func (pool *DocumentsWriterPerThreadPool) ensureOpen() error {
	if pool.closed.Load() {
		return fmt.Errorf("DWPTPool is already closed")
	}
	return nil
}

// IsRegistered reports whether the given DWPT is currently registered with
// this pool. Mirrors DocumentsWriterPerThreadPool.isRegistered.
func (pool *DocumentsWriterPerThreadPool) IsRegistered(dwpt *DocumentsWriterPerThread) bool {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	_, ok := pool.dwpts[dwpt]
	return ok
}

func (pool *DocumentsWriterPerThreadPool) MarksAsFreeAndUnlock(owner util.LockOwner, dwpt *DocumentsWriterPerThread) {
	ramBytesUsed := dwpt.RamBytesUsed()

	// Lucene asserts these are false.
	if dwpt.IsFlushPending() || dwpt.IsAborted() {
		panic("DWPT cannot be marked as free if flush is pending or aborted")
	}

	if !pool.IsRegistered(dwpt) {
		panic("we tried to add a DWPT back to the pool but the pool doesn't know about this DWPT")
	}

	pool.freeList.addAndUnlock(owner, dwpt, ramBytesUsed)
}

// Iter visits every registered DWPT, stopping early when visit returns false.
//
// Mirrors DocumentsWriterPerThreadPool.iterator(), which is documented as
// "copy on read": the registry snapshot is taken under the pool monitor and
// the callback runs outside it, exactly like Java's
// List.copyOf(dwpts).iterator().
func (pool *DocumentsWriterPerThreadPool) Iter(visit func(*DocumentsWriterPerThread) bool) {
	for _, dwpt := range pool.Slice() {
		if !visit(dwpt) {
			return
		}
	}
}

// Slice returns a snapshot of all active DWPTs.
func (pool *DocumentsWriterPerThreadPool) Slice() []*DocumentsWriterPerThread {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	res := make([]*DocumentsWriterPerThread, 0, len(pool.dwpts))
	for dwpt := range pool.dwpts {
		res = append(res, dwpt)
	}
	return res
}

func (pool *DocumentsWriterPerThreadPool) FilterAndLock(owner util.LockOwner, predicate func(*DocumentsWriterPerThread) bool) []*DocumentsWriterPerThread {
	var list []*DocumentsWriterPerThread
	for _, dwpt := range pool.Slice() {
		if predicate(dwpt) {
			dwpt.Lock(owner)
			if pool.IsRegistered(dwpt) {
				list = append(list, dwpt)
			} else {
				dwpt.Unlock(owner)
			}
		}
	}
	return list
}

func (pool *DocumentsWriterPerThreadPool) Checkout(owner util.LockOwner, dwpt *DocumentsWriterPerThread) bool {
	if !dwpt.IsHeldByCurrentThread(owner) {
		panic("DWPT must be held by the current thread for checkout")
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	if _, ok := pool.dwpts[dwpt]; ok {
		delete(pool.dwpts, dwpt)
		pool.freeList.remove(dwpt)
		return true
	}
	return false
}

// Close mirrors Java's `public synchronized void close()`.
func (pool *DocumentsWriterPerThreadPool) Close() {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.closed.Store(true)
}

// --- Internal Priority Queue Implementation ---
