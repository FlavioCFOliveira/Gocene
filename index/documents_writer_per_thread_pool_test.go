// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestDocumentsWriterPerThreadPool.java
// (Apache Lucene 10.5.0).

package index

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func newTestDWPTPool(directory store.Directory) *DocumentsWriterPerThreadPool {
	return NewDocumentsWriterPerThreadPool(func() *DocumentsWriterPerThread {
		return NewDocumentsWriterPerThread(
			util.Latest.Major,
			"",
			directory,
			directory,
			newIndexWriterConfig().LiveIndexWriterConfig,
			NewDocumentsWriterDeleteQueue(nil),
			nil,
			new(atomic.Int64),
			false)
	})
}

func TestDocumentsWriterPerThreadPoolLockReleaseAndClose(t *testing.T) {
	directory := newDirectory()
	defer directory.Close()
	owner := util.NewLockOwner()
	pool := newTestDWPTPool(directory)

	first, err := pool.GetAndLock(owner)
	if err != nil {
		t.Fatalf("getAndLock: %v", err)
	}
	if pool.Size() != 1 {
		t.Fatalf("expected 1, got %d", pool.Size())
	}
	second, err := pool.GetAndLock(owner)
	if err != nil {
		t.Fatalf("getAndLock: %v", err)
	}
	if pool.Size() != 2 {
		t.Fatalf("expected 2, got %d", pool.Size())
	}
	pool.MarksAsFreeAndUnlock(owner, first)
	if pool.Size() != 2 {
		t.Fatalf("expected 2, got %d", pool.Size())
	}
	third, err := pool.GetAndLock(owner)
	if err != nil {
		t.Fatalf("getAndLock: %v", err)
	}
	if first != third {
		t.Fatal("assertSame(first, third)")
	}
	if pool.Size() != 2 {
		t.Fatalf("expected 2, got %d", pool.Size())
	}
	pool.Checkout(owner, third)
	if pool.Size() != 1 {
		t.Fatalf("expected 1, got %d", pool.Size())
	}

	pool.Close()
	if pool.Size() != 1 {
		t.Fatalf("expected 1, got %d", pool.Size())
	}
	pool.MarksAsFreeAndUnlock(owner, second)
	if pool.Size() != 1 {
		t.Fatalf("expected 1, got %d", pool.Size())
	}
	for _, lastPerThread := range pool.FilterAndLock(owner, func(*DocumentsWriterPerThread) bool { return true }) {
		pool.Checkout(owner, lastPerThread)
		lastPerThread.Unlock(owner)
	}
	if pool.Size() != 0 {
		t.Fatalf("expected 0, got %d", pool.Size())
	}
}

// dwptPoolGoroutineWaitingInNewWriter renders
// `t.getState().equals(Thread.State.WAITING)` for the thread blocked in
// DocumentsWriterPerThreadPool.newWriter (the monitor wait on
// takenWriterPermits): it reports whether a goroutine is parked on the pool's
// condition variable inside newWriter.
func dwptPoolGoroutineWaitingInNewWriter() bool {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	for _, g := range strings.Split(string(buf[:n]), "\n\n") {
		if strings.Contains(g, "(*DocumentsWriterPerThreadPool).newWriter") &&
			strings.Contains(g, "sync.(*Cond).Wait") {
			return true
		}
	}
	return false
}

func TestDocumentsWriterPerThreadPoolCloseWhileNewWritersLocked(t *testing.T) {
	directory := newDirectory()
	defer directory.Close()
	owner := util.NewLockOwner()
	pool := newTestDWPTPool(directory)

	first, err := pool.GetAndLock(owner)
	if err != nil {
		t.Fatalf("getAndLock: %v", err)
	}
	pool.LockNewWriters()
	latch := make(chan struct{})
	var th sync.WaitGroup
	th.Add(1)
	go func() {
		defer th.Done()
		close(latch)
		_, err := pool.GetAndLock(util.NewLockOwner())
		var ace *store.AlreadyClosedException
		if !errors.As(err, &ace) {
			t.Errorf("fail(): expected AlreadyClosedException, got %v", err)
		}
		// fine
	}()
	<-latch
	for !dwptPoolGoroutineWaitingInNewWriter() {
		runtime.Gosched()
	}
	first.Unlock(owner)
	pool.Close()
	pool.UnlockNewWriters()
	for _, perThread := range pool.FilterAndLock(owner, func(*DocumentsWriterPerThread) bool { return true }) {
		if !pool.Checkout(owner, perThread) {
			t.Fatal("assertTrue(pool.checkout(perThread))")
		}
		perThread.Unlock(owner)
	}
	if pool.Size() != 0 {
		t.Fatalf("expected 0, got %d", pool.Size())
	}
	th.Wait()
}
