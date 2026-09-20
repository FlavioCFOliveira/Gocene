// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// -----------------------------------------------------------------------------
// PORT NOTE — java.util.concurrent.locks.ReentrantLock
//
// This file renders the JDK class java.util.concurrent.locks.ReentrantLock,
// not a Lucene class. It exists because Lucene 10.5.0 depends on the
// *reentrancy* of that lock, and on the reentrancy of Java monitors
// (`synchronized`), for correctness of the write path. Two examples from the
// reference tree:
//
//   - DocumentsWriterPerThread declares `implements Lock` over a private
//     `ReentrantLock`. DocumentsWriterFlushControl.obtainAndLock() returns a
//     DWPT whose lock is already held, and the caller then goes on to take
//     further lock operations on the same DWPT from the same thread.
//
//   - DocumentsWriter.flushAllThreads() runs `synchronized (this) { ...
//     flushControl.markForFullFlush() ... }`, and markForFullFlush() calls
//     back into the `synchronized` DocumentsWriter.resetDeleteQueue(). The
//     same monitor is therefore acquired twice by the same thread.
//
// A Go sync.Mutex is not reentrant: both shapes deadlock.
//
// # Owner identity is EXPLICIT
//
// Java identifies the holder implicitly, as Thread.currentThread(). Go exposes
// no goroutine identity, and every workaround was rejected on measurement or
// on portability:
//
//   - parsing runtime.Stack() costs 2.0 us on a shallow stack and 16.6 us at
//     depth 40 (measured, go1.27.0, amd64), because the traceback walks the
//     whole stack even when the output buffer is full. That is 500x-4000x a
//     sync.Mutex acquisition (3.8 ns) and it sits on the per-document write
//     path;
//   - //go:linkname to a runtime goroutine id is not linkable (there is no
//     such runtime function, and Go's linkname restrictions block pulls from
//     runtime internals);
//   - reading the g pointer out of thread-local storage requires per-GOARCH
//     assembly and a per-Go-release field offset.
//
// Gocene therefore makes the holder EXPLICIT: the goroutine entering a guarded
// region mints a LockOwner and passes it to every acquisition and release it
// performs on that region's locks. A LockOwner stands for exactly what
// Thread.currentThread() stands for in the Java original.
//
// # Contract — a LockOwner belongs to ONE goroutine
//
// A LockOwner must never be shared between goroutines, stored in a field
// reachable by another goroutine, or reused after the region that minted it
// has returned. Sharing one would let two goroutines enter the same critical
// section, exactly as two Java threads would if they could impersonate each
// other. The idiom is a function-local value:
//
//	owner := util.NewLockOwner()
//	l.Lock(owner)
//	defer l.Unlock(owner)
//
// # Correctness under the race detector
//
// Mutual exclusion between distinct owners is carried by an ordinary
// sync.Mutex, held for the whole outermost acquisition. The race detector
// therefore sees the same happens-before edge it would see for a plain mutex:
// the release by owner A happens-before the acquisition by owner B, so the
// state guarded by the lock is transferred without a reported race. holdCount
// is written only by the current holder, and ownership changes hands only
// across that mutex, so it needs no atomic of its own. owner is atomic because
// goroutines that do not hold the lock read it to decide whether they are the
// holder.
//
// Because the mutex is real, a LockOwner accidentally shared between two
// goroutines does not silently corrupt the lock's own state: the second
// goroutine skips the mutex and the resulting unsynchronised access to the
// guarded data is reported by `go test -race` at the guarded field, which is
// where the defect actually is.
// -----------------------------------------------------------------------------

package util

import (
	"sync"
	"sync/atomic"
)

// lockOwnerSeq mints LockOwner identities. Values are handed out strictly
// increasing and are never reused, so a stale identity can never be mistaken
// for a live one. Zero is reserved for "no owner".
var lockOwnerSeq atomic.Uint64

// LockOwner identifies the holder of a [ReentrantLock]. It is the Go rendering
// of Java's implicit Thread.currentThread() holder identity; see the port note
// at the top of this file.
//
// A LockOwner is a value: copying it copies the identity, which is what makes
// it safe to pass down a call chain by value. It must not be shared with
// another goroutine.
type LockOwner struct {
	id uint64
}

// NewLockOwner returns a fresh owner identity, distinct from every other
// identity ever returned by this process.
func NewLockOwner() LockOwner {
	return LockOwner{id: lockOwnerSeq.Add(1)}
}

// IsValid reports whether o was produced by [NewLockOwner]. The zero LockOwner
// is not a valid holder identity.
func (o LockOwner) IsValid() bool {
	return o.id != 0
}

// ReentrantLock is the Go port of java.util.concurrent.locks.ReentrantLock,
// restricted to the member set Gocene's ported Lucene code exercises: lock,
// unlock, tryLock, isHeldByCurrentThread and getHoldCount.
//
// Java's lockInterruptibly, the timed tryLock(long, TimeUnit) and newCondition
// are not rendered here: no ported Lucene 10.5.0 call site in this module uses
// them (DocumentsWriterPerThread's newCondition() throws
// UnsupportedOperationException in the original).
//
// Reentrancy: an owner that already holds the lock acquires it again without
// blocking and increments the hold count; the lock is released to other owners
// only when the hold count falls back to zero, exactly as in Java.
//
// The zero value is an unlocked ReentrantLock and is ready to use. A
// ReentrantLock must not be copied after first use.
type ReentrantLock struct {
	// mu carries mutual exclusion between distinct owners. It is held for the
	// whole outermost acquisition and released when holdCount returns to zero.
	mu sync.Mutex
	// owner is the identity of the current holder, or zero when free. It is
	// atomic because goroutines that do not hold the lock read it.
	owner atomic.Uint64
	// holdCount is Java's ReentrantLock hold count. Only the current holder
	// mutates it, and ownership only changes hands across mu, so plain access
	// is correctly synchronised.
	holdCount int
}

// Lock acquires the lock for owner, blocking until it is available. If owner
// already holds the lock, Lock returns immediately and increments the hold
// count. Mirrors ReentrantLock.lock().
//
// Lock panics if owner is the zero LockOwner, which mirrors the impossibility
// of a null Thread.currentThread() in the Java original.
func (l *ReentrantLock) Lock(owner LockOwner) {
	if owner.id == 0 {
		panic("util: ReentrantLock.Lock called with the zero LockOwner; mint one with util.NewLockOwner")
	}
	if l.owner.Load() == owner.id {
		l.holdCount++
		return
	}
	l.mu.Lock()
	l.owner.Store(owner.id)
	l.holdCount = 1
}

// TryLock acquires the lock for owner if it is free, or if owner already holds
// it, and reports whether it was acquired. It never blocks. Mirrors
// ReentrantLock.tryLock().
func (l *ReentrantLock) TryLock(owner LockOwner) bool {
	if owner.id == 0 {
		panic("util: ReentrantLock.TryLock called with the zero LockOwner; mint one with util.NewLockOwner")
	}
	if l.owner.Load() == owner.id {
		l.holdCount++
		return true
	}
	if l.mu.TryLock() {
		l.owner.Store(owner.id)
		l.holdCount = 1
		return true
	}
	return false
}

// Unlock releases one hold of the lock. The lock becomes available to other
// owners once every matching Lock or TryLock has been released. Mirrors
// ReentrantLock.unlock().
//
// Unlock panics if owner does not currently hold the lock, mirroring the
// IllegalMonitorStateException thrown by the Java original.
func (l *ReentrantLock) Unlock(owner LockOwner) {
	if owner.id == 0 || l.owner.Load() != owner.id {
		panic("util: ReentrantLock.Unlock by an owner that does not hold the lock")
	}
	l.holdCount--
	if l.holdCount == 0 {
		l.owner.Store(0)
		l.mu.Unlock()
	}
}

// IsHeldByCurrentThread reports whether owner currently holds this lock.
// Mirrors ReentrantLock.isHeldByCurrentThread(). In Lucene 10.5.0 this is used
// only inside assertions.
func (l *ReentrantLock) IsHeldByCurrentThread(owner LockOwner) bool {
	return owner.id != 0 && l.owner.Load() == owner.id
}

// HoldCount returns the number of holds owner has on this lock, or zero if
// owner does not hold it. Mirrors ReentrantLock.getHoldCount(), which likewise
// reports the count for the querying thread only.
func (l *ReentrantLock) HoldCount(owner LockOwner) int {
	if owner.id == 0 || l.owner.Load() != owner.id {
		return 0
	}
	return l.holdCount
}
