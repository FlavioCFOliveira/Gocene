// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestLockableConcurrentApproximatePriorityQueue.java
// (Apache Lucene 10.5.0).

package index

import (
	"math/rand"
	"sync"
	"testing"
	"unsafe"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// weightedLock ports the private static WeightedLock (a Lock over a
// ReentrantLock carrying a weight).
type weightedLock struct {
	lock   util.ReentrantLock
	weight int64
}

func (w *weightedLock) Lock(owner util.LockOwner)         { w.lock.Lock(owner) }
func (w *weightedLock) TryLock(owner util.LockOwner) bool { return w.lock.TryLock(owner) }
func (w *weightedLock) Unlock(owner util.LockOwner)       { w.lock.Unlock(owner) }

// hashCode renders Object.hashCode(), the identity hash of the lock.
func (w *weightedLock) hashCode() int64 {
	return int64(int32(uintptr(unsafe.Pointer(w))))
}

func TestLockableConcurrentApproximatePriorityQueueNeverReturnNullOnNonEmptyQueue(t *testing.T) {
	iters := atLeast(10)
	for iter := 0; iter < iters; iter++ {
		concurrency := 1 + rand.Intn(16)
		queue := newLockableConcurrentApproximatePriorityQueueWithConcurrency[*weightedLock](concurrency)
		numThreads := 2 + rand.Intn(15)
		startingGun := make(chan struct{})
		var threads sync.WaitGroup
		for th := 0; th < numThreads; th++ {
			threads.Add(1)
			go func() {
				defer threads.Done()
				owner := util.NewLockOwner()
				<-startingGun
				lock := &weightedLock{}
				lock.Lock(owner)
				lock.weight++ // Simulate a DWPT whose RAM usage increases
				queue.addAndUnlock(owner, lock, lock.weight)
				for i := 0; i < 10_000; i++ {
					lock = queue.lockAndPoll(owner)
					if lock == nil {
						t.Errorf("assertNotNull(lock)")
						return
					}
					queue.addAndUnlock(owner, lock, lock.hashCode())
				}
			}()
		}
		close(startingGun)
		threads.Wait()
	}
}
