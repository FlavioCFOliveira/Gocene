// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
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
	closed             bool
	cond               *sync.Cond
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

func (pool *DocumentsWriterPerThreadPool) newWriter() *DocumentsWriterPerThread {
	// This method is called under pool.mu lock.
	for pool.takenWriterPermits > 0 {
		pool.cond.Wait()
	}

	if err := pool.ensureOpen(); err != nil {
		panic(err)
	}

	dwpt := pool.dwptFactory()
	dwpt.Lock() // lock so nobody else will get this DWPT
	pool.dwpts[dwpt] = struct{}{}
	return dwpt
}

func (pool *DocumentsWriterPerThreadPool) GetAndLock() *DocumentsWriterPerThread {
	if err := pool.ensureOpen(); err != nil {
		panic(err)
	}

	dwpt := pool.freeList.lockAndPoll()
	if dwpt != nil {
		return dwpt
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.newWriter()
}

func (pool *DocumentsWriterPerThreadPool) ensureOpen() error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.closed {
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

func (pool *DocumentsWriterPerThreadPool) MarksAsFreeAndUnlock(dwpt *DocumentsWriterPerThread) {
	ramBytesUsed := dwpt.RamBytesUsed()

	// Lucene asserts these are false.
	if dwpt.IsFlushPending() || dwpt.IsAborted() {
		panic("DWPT cannot be marked as free if flush is pending or aborted")
	}

	if !pool.IsRegistered(dwpt) {
		panic("we tried to add a DWPT back to the pool but the pool doesn't know about this DWPT")
	}

	pool.freeList.addAndUnlock(dwpt, ramBytesUsed)
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

func (pool *DocumentsWriterPerThreadPool) FilterAndLock(predicate func(*DocumentsWriterPerThread) bool) []*DocumentsWriterPerThread {
	var list []*DocumentsWriterPerThread
	for _, dwpt := range pool.Slice() {
		if predicate(dwpt) {
			dwpt.Lock()
			if pool.IsRegistered(dwpt) {
				list = append(list, dwpt)
			} else {
				dwpt.Unlock()
			}
		}
	}
	return list
}

func (pool *DocumentsWriterPerThreadPool) Checkout(dwpt *DocumentsWriterPerThread) bool {
	if !dwpt.IsHeldByCurrentThread() {
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

func (pool *DocumentsWriterPerThreadPool) Close() {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.closed = true
}

// --- Internal Priority Queue Implementation ---
