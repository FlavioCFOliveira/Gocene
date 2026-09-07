// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math/bits"
	"runtime"
	"sync"
	"sync/atomic"
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
	closed              bool
	cond                *sync.Cond
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

func (pool *DocumentsWriterPerThreadPool) isRegistered(dwpt *DocumentsWriterPerThread) bool {
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

	if !pool.isRegistered(dwpt) {
		panic("we tried to add a DWPT back to the pool but the pool doesn't know about this DWPT")
	}

	pool.freeList.addAndUnlock(dwpt, ramBytesUsed)
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
			if pool.isRegistered(dwpt) {
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

type approximatePriorityQueue struct {
	slots     []*DocumentsWriterPerThread
	usedSlots uint64
}

func newApproximatePriorityQueue() *approximatePriorityQueue {
	return &approximatePriorityQueue{
		slots: make([]*DocumentsWriterPerThread, 64),
	}
}

func (q *approximatePriorityQueue) add(entry *DocumentsWriterPerThread, weight int64) {
	expectedSlot := bits.LeadingZeros64(uint64(weight))
	freeSlots := ^q.usedSlots
	destinationSlot := expectedSlot + bits.TrailingZeros64(freeSlots>>expectedSlot)

	if destinationSlot < 64 {
		q.usedSlots |= 1 << destinationSlot
		q.slots[destinationSlot] = entry
	} else {
		q.slots = append(q.slots, entry)
	}
}

func (q *approximatePriorityQueue) poll(predicate func(*DocumentsWriterPerThread) bool) *DocumentsWriterPerThread {
	nextSlot := 0
	for {
		nextUsedSlot := nextSlot + bits.TrailingZeros64(q.usedSlots>>nextSlot)
		if nextUsedSlot >= 64 {
			break
		}
		entry := q.slots[nextUsedSlot]
		if predicate(entry) {
			q.usedSlots &= ^(1 << nextUsedSlot)
			q.slots[nextUsedSlot] = nil
			return entry
		}
		nextSlot = nextUsedSlot + 1
		if nextSlot >= 64 {
			break
		}
	}

	for i := len(q.slots) - 1; i >= 64; i-- {
		entry := q.slots[i]
		if predicate(entry) {
			q.slots[i] = nil
			// We don't actually remove from slice to avoid expensive shifts;
			// we just leave nil and poll skips them.
			// But the Java implementation uses lit.remove().
			// To be faithful, we should shift or use a different structure.
			// Given the "translation only" scope and a small number of elements,
			// we can just shift.
			q.slots = append(q.slots[:i], q.slots[i+1:]...)
			return entry
		}
	}
	return nil
}

func (q *approximatePriorityQueue) contains(o *DocumentsWriterPerThread) bool {
	for _, s := range q.slots {
		if s == o {
			return true
		}
	}
	return false
}

func (q *approximatePriorityQueue) remove(o *DocumentsWriterPerThread) bool {
	for i, s := range q.slots {
		if s == o {
			if i < 64 {
				q.usedSlots &= ^(1 << i)
				q.slots[i] = nil
			} else {
				q.slots = append(q.slots[:i], q.slots[i+1:]...)
			}
			return true
		}
	}
	return false
}

type concurrentApproximatePriorityQueue struct {
	concurrency int
	locks       []sync.Mutex
	queues      []*approximatePriorityQueue
}

func newConcurrentApproximatePriorityQueue() *concurrentApproximatePriorityQueue {
	coreCount := runtime.NumCPU()
	concurrency := coreCount / 4
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 256 {
		concurrency = 256
	}

	locks := make([]sync.Mutex, concurrency)
	queues := make([]*approximatePriorityQueue, concurrency)
	for i := 0; i < concurrency; i++ {
		queues[i] = newApproximatePriorityQueue()
	}

	return &concurrentApproximatePriorityQueue{
		concurrency: concurrency,
		locks:       locks,
		queues:      queues,
	}
}

func (q *concurrentApproximatePriorityQueue) add(entry *DocumentsWriterPerThread, weight int64) {
	// In Go, we don't have a stable thread ID. We'll use a random offset.
	// This approximates the thread-based distribution of Lucene.
	// Since we can't easily get a hash, we use a simple sequence or random.
	// For a faithful translation, we'll use a simple offset.
	start := 0 // In a real scenario, this would be a thread-local or random value.

	for i := 0; i < q.concurrency; i++ {
		index := (start + i) % q.concurrency
		if q.locks[index].TryLock() {
			q.queues[index].add(entry, weight)
			q.locks[index].Unlock()
			return
		}
	}
	// Fallback to blocking on the first slot.
	q.locks[0].Lock()
	q.queues[0].add(entry, weight)
	q.locks[0].Unlock()
}

func (q *concurrentApproximatePriorityQueue) poll(predicate func(*DocumentsWriterPerThread) bool) *DocumentsWriterPerThread {
	start := 0
	for i := 0; i < q.concurrency; i++ {
		index := (start + i) % q.concurrency
		if q.locks[index].TryLock() {
			entry := q.queues[index].poll(predicate)
			q.locks[index].Unlock()
			if entry != nil {
				return entry
			}
		}
	}
	for i := 0; i < q.concurrency; i++ {
		index := (start + i) % q.concurrency
		q.locks[index].Lock()
		entry := q.queues[index].poll(predicate)
		q.locks[index].Unlock()
		if entry != nil {
			return entry
		}
	}
	return nil
}

func (q *concurrentApproximatePriorityQueue) remove(o *DocumentsWriterPerThread) bool {
	for i := 0; i < q.concurrency; i++ {
		q.locks[i].Lock()
		if q.queues[i].remove(o) {
			q.locks[i].Unlock()
			return true
		}
		q.locks[i].Unlock()
	}
	return false
}

func (q *concurrentApproximatePriorityQueue) contains(o *DocumentsWriterPerThread) bool {
	for i := 0; i < q.concurrency; i++ {
		q.locks[i].Lock()
		found := q.queues[i].contains(o)
		q.locks[i].Unlock()
		if found {
			return true
		}
	}
	return false
}

type lockableConcurrentApproximatePriorityQueue struct {
	queue                *concurrentApproximatePriorityQueue
	addAndUnlockCounter atomic.Int32
}

func newLockableConcurrentApproximatePriorityQueue() *lockableConcurrentApproximatePriorityQueue {
	return &lockableConcurrentApproximatePriorityQueue{
		queue: newConcurrentApproximatePriorityQueue(),
	}
}

func (l *lockableConcurrentApproximatePriorityQueue) lockAndPoll() *DocumentsWriterPerThread {
	for {
		count := l.addAndUnlockCounter.Load()
		// Predicate is tryLock, matching Lucene's Lock::tryLock.
		entry := l.queue.poll(func(dwpt *DocumentsWriterPerThread) bool {
			return dwpt.TryLock()
		})
		if entry != nil {
			return entry
		}
		if count == l.addAndUnlockCounter.Load() {
			break
		}
	}
	return nil
}

func (l *lockableConcurrentApproximatePriorityQueue) addAndUnlock(entry *DocumentsWriterPerThread, weight int64) {
	l.queue.add(entry, weight)
	entry.Unlock()
	l.addAndUnlockCounter.Add(1)
}

func (l *lockableConcurrentApproximatePriorityQueue) remove(o *DocumentsWriterPerThread) bool {
	return l.queue.remove(o)
}

func (l *lockableConcurrentApproximatePriorityQueue) contains(o *DocumentsWriterPerThread) bool {
	return l.queue.contains(o)
}
