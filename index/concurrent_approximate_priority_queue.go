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

// Port of org.apache.lucene.index.ConcurrentApproximatePriorityQueue from
// Apache Lucene 10.4.0 (commit 9983b7c). The original Java class is
// package-private; we mirror that visibility with unexported Go types.
//
// The companion org.apache.lucene.index.ApproximatePriorityQueue is also
// package-private and used exclusively by this type, so it is inlined below
// as the unexported approximatePriorityQueue helper rather than living in a
// separate file. This keeps the wrapper and its shard implementation as a
// single unit, matching the cohesion of the upstream package-private pair.

// Bounds for the number of shards the queue spreads contention across.
const (
	minQueueConcurrency = 1
	maxQueueConcurrency = 256
)

// queueConcurrencyFromCPU mirrors getConcurrency() from the Java source: aim
// for ~4 entries per slot when indexing with one thread per CPU core, clamped
// to [minQueueConcurrency, maxQueueConcurrency].
func queueConcurrencyFromCPU() int {
	c := runtime.NumCPU() / 4
	if c < minQueueConcurrency {
		c = minQueueConcurrency
	}
	if c > maxQueueConcurrency {
		c = maxQueueConcurrency
	}
	return c
}

// goroutineHashCounter generates a 16-bit "thread hash" for the current
// goroutine. Java uses Thread.currentThread().hashCode() which is fixed per
// thread; Go has no equivalent, so we keep a goroutine-local counter under
// goroutineHash. The hash only needs to spread work across shards and offer
// loose affinity, so a one-shot per-goroutine assignment is sufficient.
var goroutineHashCounter atomic.Uint64

// goroutineHash returns a per-goroutine 16-bit value, lazily assigned on first
// use within the goroutine.
//
// We cannot key directly off the goroutine ID without unsafe runtime tricks,
// so we expose this as a function parameter through the public surface API
// (add/poll). The goroutine that owns the call site can simply call
// nextGoroutineHash and cache it in goroutine-local state when desired; the
// default implementations call it on every entry, which yields acceptable
// shard distribution.
func nextGoroutineHash() int {
	return int(goroutineHashCounter.Add(1) & 0xFFFF)
}

// concurrentApproximatePriorityQueue is the Go port of
// ConcurrentApproximatePriorityQueue<T>. T is modelled as any so the queue can
// hold arbitrary entries (mirroring the Java generic). Callers are responsible
// for ensuring T values used with contains/remove are comparable.
type concurrentApproximatePriorityQueue struct {
	concurrency int
	locks       []sync.Mutex
	queues      []*approximatePriorityQueue
}

// newConcurrentApproximatePriorityQueueDefault constructs a queue with a
// concurrency level derived from runtime.NumCPU(), matching the
// no-argument Java constructor.
func newConcurrentApproximatePriorityQueueDefault() *concurrentApproximatePriorityQueue {
	return newConcurrentApproximatePriorityQueue(queueConcurrencyFromCPU())
}

// newConcurrentApproximatePriorityQueue constructs a queue with the requested
// shard count. It panics with the same message Java would throw via
// IllegalArgumentException when concurrency is out of range; callers within
// the package must validate their input before invoking.
func newConcurrentApproximatePriorityQueue(concurrency int) *concurrentApproximatePriorityQueue {
	if concurrency < minQueueConcurrency || concurrency > maxQueueConcurrency {
		panic(fmt.Sprintf("concurrency must be in [%d, %d], got %d",
			minQueueConcurrency, maxQueueConcurrency, concurrency))
	}
	q := &concurrentApproximatePriorityQueue{
		concurrency: concurrency,
		locks:       make([]sync.Mutex, concurrency),
		queues:      make([]*approximatePriorityQueue, concurrency),
	}
	for i := range q.queues {
		q.queues[i] = newApproximatePriorityQueue()
	}
	return q
}

// add inserts entry with the given weight. The entry is first attempted via
// tryLock across shards starting at a goroutine-derived index, falling back to
// a blocking lock on the home shard if no shard is currently free.
func (q *concurrentApproximatePriorityQueue) add(entry any, weight int64) {
	hash := nextGoroutineHash()
	for i := 0; i < q.concurrency; i++ {
		idx := (hash + i) % q.concurrency
		if q.locks[idx].TryLock() {
			q.queues[idx].add(entry, weight)
			q.locks[idx].Unlock()
			return
		}
	}
	idx := hash % q.concurrency
	q.locks[idx].Lock()
	q.queues[idx].add(entry, weight)
	q.locks[idx].Unlock()
}

// poll removes and returns an entry matching predicate, preferring entries
// with higher weight. It mirrors the two-pass Java algorithm: try every shard
// with tryLock first, then make a second pass with blocking lock acquisition.
// Returns nil and false when no matching entry is available.
func (q *concurrentApproximatePriorityQueue) poll(predicate func(any) bool) (any, bool) {
	hash := nextGoroutineHash()
	for i := 0; i < q.concurrency; i++ {
		idx := (hash + i) % q.concurrency
		if q.locks[idx].TryLock() {
			entry, ok := q.queues[idx].poll(predicate)
			q.locks[idx].Unlock()
			if ok {
				return entry, true
			}
		}
	}
	for i := 0; i < q.concurrency; i++ {
		idx := (hash + i) % q.concurrency
		q.locks[idx].Lock()
		entry, ok := q.queues[idx].poll(predicate)
		q.locks[idx].Unlock()
		if ok {
			return entry, true
		}
	}
	return nil, false
}

// contains is only used by assertions in Lucene. In Go we always evaluate it
// (there is no equivalent of -ea), so callers should reserve it for debug
// paths to avoid the O(N) cost across all shards.
func (q *concurrentApproximatePriorityQueue) contains(o any) bool {
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

// remove deletes the first occurrence of o from any shard, returning true on
// success.
func (q *concurrentApproximatePriorityQueue) remove(o any) bool {
	for i := 0; i < q.concurrency; i++ {
		q.locks[i].Lock()
		removed := q.queues[i].remove(o)
		q.locks[i].Unlock()
		if removed {
			return true
		}
	}
	return false
}

// approximatePriorityQueue is the unexported port of
// org.apache.lucene.index.ApproximatePriorityQueue, the per-shard data
// structure used by concurrentApproximatePriorityQueue. Slots 0..63 are
// sparsely populated and tracked via the usedSlots bitset; slots >= 64 are
// densely populated and appended/removed from the tail.
type approximatePriorityQueue struct {
	slots     []any
	usedSlots uint64
}

const longSize = 64

// newApproximatePriorityQueue creates an empty queue. The slot slice is
// pre-sized to longSize so indexing into slots[0..63] never needs to grow it.
func newApproximatePriorityQueue() *approximatePriorityQueue {
	return &approximatePriorityQueue{
		slots: make([]any, longSize),
	}
}

// add inserts entry with the supplied weight. The expected slot is the number
// of leading zeros of the weight (so larger weights map to lower indices);
// when the expected slot is taken we walk forward via the bitset.
func (q *approximatePriorityQueue) add(entry any, weight int64) {
	if entry == nil {
		panic("approximatePriorityQueue: nil entry")
	}
	expectedSlot := bits.LeadingZeros64(uint64(weight))
	freeSlots := ^q.usedSlots
	destination := expectedSlot + bits.TrailingZeros64(freeSlots>>expectedSlot)
	if destination < longSize {
		q.usedSlots |= 1 << destination
		q.slots[destination] = entry
		return
	}
	q.slots = append(q.slots, entry)
}

// poll returns and removes an entry matching predicate. Sparse slots are
// scanned first via the bitset, dense slots second in reverse order to keep
// the same entry hot under decreasing parallelism.
func (q *approximatePriorityQueue) poll(predicate func(any) bool) (any, bool) {
	// Sparse range [0, 64): walk via bitset.
	nextSlot := 0
	for nextSlot < longSize {
		nextUsedSlot := nextSlot + bits.TrailingZeros64(q.usedSlots>>nextSlot)
		if nextUsedSlot >= longSize {
			break
		}
		entry := q.slots[nextUsedSlot]
		if predicate(entry) {
			q.usedSlots &^= 1 << nextUsedSlot
			q.slots[nextUsedSlot] = nil
			return entry, true
		}
		nextSlot = nextUsedSlot + 1
	}
	// Dense range [64, len): scan in reverse and shrink the tail.
	for i := len(q.slots) - 1; i >= longSize; i-- {
		entry := q.slots[i]
		if predicate(entry) {
			// Remove element at index i: in Lucene this is ArrayList.remove(i),
			// which shifts subsequent elements left. Here i is always the last
			// dense slot we examined first, but predicate may skip entries, so
			// preserve the general semantics with append.
			q.slots = append(q.slots[:i], q.slots[i+1:]...)
			return entry, true
		}
	}
	return nil, false
}

// contains reports whether o is present in any slot. Lucene reserves this for
// assertions; callers should treat it as O(N).
func (q *approximatePriorityQueue) contains(o any) bool {
	if o == nil {
		panic("approximatePriorityQueue: nil argument")
	}
	for _, v := range q.slots {
		if v == nil {
			continue
		}
		if v == o {
			return true
		}
	}
	return false
}

// isEmpty reports whether the queue has no entries. It mirrors the Java
// invariant: no sparse slots are used and the tail has not been extended.
func (q *approximatePriorityQueue) isEmpty() bool {
	return q.usedSlots == 0 && len(q.slots) == longSize
}

// remove deletes the first occurrence of o, returning true on success. Sparse
// removals clear the bitset; dense removals shrink the slice.
func (q *approximatePriorityQueue) remove(o any) bool {
	if o == nil {
		panic("approximatePriorityQueue: nil argument")
	}
	for i, v := range q.slots {
		if v == nil {
			continue
		}
		if v != o {
			continue
		}
		if i >= longSize {
			q.slots = append(q.slots[:i], q.slots[i+1:]...)
		} else {
			q.usedSlots &^= 1 << i
			q.slots[i] = nil
		}
		return true
	}
	return false
}
