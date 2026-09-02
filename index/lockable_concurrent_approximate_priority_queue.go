// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "sync/atomic"

// Port of org.apache.lucene.index.LockableConcurrentApproximatePriorityQueue
// from Apache Lucene 10.4.0 (commit 9983b7c). The Java type is generic over
// {@code T extends Lock}; here we model the bound as a Go constraint that
// captures the only two Lock methods the implementation actually exercises:
// TryLock (called from the poll predicate) and Unlock (called from
// addAndUnlock).
//
// The underlying shard queue stores entries as any (mirroring the upstream
// raw-typed inner field). The wrapper bridges the generic boundary by
// asserting on poll and by accepting T on add; the constraint guarantees the
// assertion always succeeds.

// queueLock is the Go equivalent of the {@code T extends Lock} bound. Only the
// two methods used by LockableConcurrentApproximatePriorityQueue are required;
// the broader java.util.concurrent.locks.Lock surface is intentionally not
// modelled because the wrapper never calls anything else.
type queueLock interface {
	// TryLock acquires the lock without blocking and returns true on success.
	// Mirrors java.util.concurrent.locks.Lock#tryLock().
	TryLock() bool
	// Unlock releases a previously acquired lock. Mirrors
	// java.util.concurrent.locks.Lock#unlock().
	Unlock()
}

// lockableConcurrentApproximatePriorityQueue mirrors the package-private Java
// class of the same name. Like the original, instances are constructed with
// either an explicit concurrency level or the runtime-derived default.
type lockableConcurrentApproximatePriorityQueue[T queueLock] struct {
	queue             *concurrentApproximatePriorityQueue
	addAndUnlockCount atomic.Int32
}

// newLockableConcurrentApproximatePriorityQueueDefault matches the Java
// no-argument constructor by deferring to the default concurrency derived from
// runtime.NumCPU().
func newLockableConcurrentApproximatePriorityQueueDefault[T queueLock]() *lockableConcurrentApproximatePriorityQueue[T] {
	return &lockableConcurrentApproximatePriorityQueue[T]{
		queue: newConcurrentApproximatePriorityQueueDefault(),
	}
}

// newLockableConcurrentApproximatePriorityQueue matches the explicit
// concurrency constructor. Validation lives in the underlying shard queue;
// passing an out-of-range concurrency panics there.
func newLockableConcurrentApproximatePriorityQueue[T queueLock](concurrency int) *lockableConcurrentApproximatePriorityQueue[T] {
	return &lockableConcurrentApproximatePriorityQueue[T]{
		queue: newConcurrentApproximatePriorityQueue(concurrency),
	}
}

// lockAndPoll locks an entry and polls it from the queue, in that order. The
// returned entry is already locked by the calling goroutine; the caller owns
// the unlock. Returns the zero value and false when no entry can be locked.
//
// The retry loop mirrors the Java do-while: a parallel addAndUnlock that
// completed since we entered the loop may have made a previously contended
// entry available, so we try one more time before giving up.
func (q *lockableConcurrentApproximatePriorityQueue[T]) lockAndPoll() (T, bool) {
	var zero T
	for {
		before := q.addAndUnlockCount.Load()
		raw, ok := q.queue.poll(func(e any) bool {
			// The constraint guarantees this assertion succeeds for every
			// entry inserted through addAndUnlock.
			return e.(T).TryLock()
		})
		if ok {
			return raw.(T), true
		}
		if before == q.addAndUnlockCount.Load() {
			return zero, false
		}
	}
}

// remove deletes an entry from the queue, returning true on success. It
// delegates to the underlying shard queue which performs O(N) equality lookup
// across all shards.
func (q *lockableConcurrentApproximatePriorityQueue[T]) remove(entry T) bool {
	return q.queue.remove(entry)
}

// contains reports whether the entry is currently in the queue. Lucene only
// uses this from assertions; callers should treat it as O(N).
func (q *lockableConcurrentApproximatePriorityQueue[T]) contains(entry T) bool {
	return q.queue.contains(entry)
}

// addAndUnlock adds an entry to the queue and unlocks it, in that order. The
// counter increment after unlock is what allows lockAndPoll's retry loop to
// detect that a concurrent producer made a new entry available.
func (q *lockableConcurrentApproximatePriorityQueue[T]) addAndUnlock(entry T, weight int64) {
	q.queue.add(entry, weight)
	entry.Unlock()
	q.addAndUnlockCount.Add(1)
}
