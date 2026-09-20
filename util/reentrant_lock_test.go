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

package util

import (
	"sync"
	"testing"
	"time"
)

func TestLockOwnerIdentitiesAreDistinctAndValid(t *testing.T) {
	var zero LockOwner
	if zero.IsValid() {
		t.Fatal("the zero LockOwner must not be a valid holder identity")
	}
	seen := make(map[LockOwner]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		o := NewLockOwner()
		if !o.IsValid() {
			t.Fatalf("NewLockOwner returned an invalid owner at i=%d", i)
		}
		if _, dup := seen[o]; dup {
			t.Fatalf("NewLockOwner repeated an identity at i=%d", i)
		}
		seen[o] = struct{}{}
	}
}

// TestReentrantLockReentersOnSameOwner is the regression test for the write-path
// hang: Lucene's DocumentsWriterFlushControl.obtainAndLock() hands back an
// already-locked DWPT and the caller acquires it again. A sync.Mutex deadlocks
// here; a ReentrantLock must not.
func TestReentrantLockReentersOnSameOwner(t *testing.T) {
	var l ReentrantLock
	owner := NewLockOwner()

	done := make(chan struct{})
	go func() {
		defer close(done)
		l.Lock(owner)
		l.Lock(owner) // re-entry, as obtainAndLock + updateDocuments do
		if got := l.HoldCount(owner); got != 2 {
			t.Errorf("HoldCount after two Lock calls = %d, want 2", got)
		}
		if !l.IsHeldByCurrentThread(owner) {
			t.Error("IsHeldByCurrentThread = false while holding the lock twice")
		}
		l.Unlock(owner)
		if got := l.HoldCount(owner); got != 1 {
			t.Errorf("HoldCount after one Unlock = %d, want 1", got)
		}
		l.Unlock(owner)
		if got := l.HoldCount(owner); got != 0 {
			t.Errorf("HoldCount after full release = %d, want 0", got)
		}
		if l.IsHeldByCurrentThread(owner) {
			t.Error("IsHeldByCurrentThread = true after full release")
		}
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("re-entrant acquisition deadlocked")
	}
}

func TestReentrantLockTryLockReentersAndExcludes(t *testing.T) {
	var l ReentrantLock
	a := NewLockOwner()
	b := NewLockOwner()

	if !l.TryLock(a) {
		t.Fatal("TryLock on a free lock must succeed")
	}
	if !l.TryLock(a) {
		t.Fatal("TryLock by the holder must succeed (re-entry)")
	}
	if got := l.HoldCount(a); got != 2 {
		t.Fatalf("HoldCount = %d, want 2", got)
	}

	// A different owner must be excluded, and must observe no holds of its own.
	refused := make(chan bool, 1)
	go func() { refused <- l.TryLock(b) }()
	if <-refused {
		t.Fatal("TryLock by a second owner succeeded while the lock was held")
	}
	if got := l.HoldCount(b); got != 0 {
		t.Fatalf("HoldCount for a non-holder = %d, want 0", got)
	}
	if l.IsHeldByCurrentThread(b) {
		t.Fatal("IsHeldByCurrentThread true for a non-holder")
	}

	l.Unlock(a)
	if l.TryLock(b) {
		t.Fatal("lock became available while the holder still had one hold")
	}
	l.Unlock(a)

	acquired := make(chan bool, 1)
	go func() { acquired <- l.TryLock(b) }()
	if !<-acquired {
		t.Fatal("lock did not become available after the hold count reached zero")
	}
}

func TestReentrantLockUnlockByNonHolderPanics(t *testing.T) {
	var l ReentrantLock
	a := NewLockOwner()
	b := NewLockOwner()
	l.Lock(a)
	defer l.Unlock(a)

	defer func() {
		if recover() == nil {
			t.Fatal("Unlock by a non-holder must panic (IllegalMonitorStateException)")
		}
	}()
	l.Unlock(b)
}

func TestReentrantLockZeroOwnerPanics(t *testing.T) {
	var l ReentrantLock
	var zero LockOwner
	defer func() {
		if recover() == nil {
			t.Fatal("Lock with the zero LockOwner must panic")
		}
	}()
	l.Lock(zero)
}

// TestReentrantLockMutualExclusionUnderRace is the -race witness: the counter is
// guarded only by the ReentrantLock, so any failure of mutual exclusion is
// reported as a data race as well as a wrong total.
func TestReentrantLockMutualExclusionUnderRace(t *testing.T) {
	var l ReentrantLock
	guarded := 0

	const goroutines = 16
	const itersPerGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			// Each goroutine mints its own identity, as the contract requires.
			owner := NewLockOwner()
			for i := 0; i < itersPerGoroutine; i++ {
				l.Lock(owner)
				// Nest, so the reentrant path is exercised under contention too.
				l.Lock(owner)
				guarded++
				l.Unlock(owner)
				l.Unlock(owner)
			}
		}()
	}
	wg.Wait()

	if want := goroutines * itersPerGoroutine; guarded != want {
		t.Fatalf("guarded counter = %d, want %d", guarded, want)
	}
	if l.IsLockedByAnyOwnerForTest() {
		t.Fatal("lock left held after every goroutine released it")
	}
}

// IsLockedByAnyOwnerForTest reports whether any owner currently holds the lock.
// It exists for the exclusion test above; ReentrantLock's exported surface
// deliberately mirrors only the Java members Gocene's ported code uses.
func (l *ReentrantLock) IsLockedByAnyOwnerForTest() bool {
	return l.owner.Load() != 0
}
