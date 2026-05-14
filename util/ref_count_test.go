// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// TestRefCount_InitialState verifies a freshly constructed RefCount
// starts at 1, matching the Java initialiser.
func TestRefCount_InitialState(t *testing.T) {
	r := NewRefCount[int](42, nil)
	if c := r.GetRefCount(); c != 1 {
		t.Fatalf("initial refCount=%d want 1", c)
	}
	if v := r.Get(); v != 42 {
		t.Fatalf("Get=%d want 42", v)
	}
}

// TestRefCount_IncDecBalanced exercises matched IncRef/DecRef pairs.
func TestRefCount_IncDecBalanced(t *testing.T) {
	r := NewRefCount[int](1, nil)
	r.IncRef()
	r.IncRef()
	if c := r.GetRefCount(); c != 3 {
		t.Fatalf("after 2 IncRef refCount=%d want 3", c)
	}
	if err := r.DecRef(); err != nil {
		t.Fatalf("DecRef: %v", err)
	}
	if err := r.DecRef(); err != nil {
		t.Fatalf("DecRef: %v", err)
	}
	if c := r.GetRefCount(); c != 1 {
		t.Fatalf("after 2 DecRef refCount=%d want 1", c)
	}
}

// TestRefCount_ReleaseOnZero verifies release is invoked exactly once
// when the count transitions to 0.
func TestRefCount_ReleaseOnZero(t *testing.T) {
	var releases int32
	r := NewRefCount[string]("payload", func(s string) error {
		atomic.AddInt32(&releases, 1)
		return nil
	})
	if err := r.DecRef(); err != nil {
		t.Fatalf("DecRef: %v", err)
	}
	if got := atomic.LoadInt32(&releases); got != 1 {
		t.Fatalf("expected release called once, got %d", got)
	}
}

// TestRefCount_NoReleaseAboveZero verifies release is NOT invoked
// when the count is still positive.
func TestRefCount_NoReleaseAboveZero(t *testing.T) {
	var releases int32
	r := NewRefCount[string]("payload", func(s string) error {
		atomic.AddInt32(&releases, 1)
		return nil
	})
	r.IncRef()
	if err := r.DecRef(); err != nil {
		t.Fatalf("DecRef: %v", err)
	}
	if got := atomic.LoadInt32(&releases); got != 0 {
		t.Fatalf("expected release NOT called, got %d", got)
	}
}

// TestRefCount_OverRelease verifies the IllegalStateException analogue.
func TestRefCount_OverRelease(t *testing.T) {
	r := NewRefCount[int](1, nil)
	if err := r.DecRef(); err != nil {
		t.Fatalf("first DecRef: %v", err)
	}
	if err := r.DecRef(); !errors.Is(err, ErrRefCountOverRelease) {
		t.Fatalf("expected ErrRefCountOverRelease, got %v", err)
	}
}

// TestRefCount_ReleaseFailureRestoresCount verifies the recoverable
// release failure semantics.
func TestRefCount_ReleaseFailureRestoresCount(t *testing.T) {
	errBoom := errors.New("boom")
	r := NewRefCount[string]("payload", func(s string) error { return errBoom })
	if err := r.DecRef(); !errors.Is(err, errBoom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
	if c := r.GetRefCount(); c != 1 {
		t.Fatalf("expected refCount restored to 1, got %d", c)
	}
}

// TestRefCount_Concurrent stresses concurrent Inc/DecRef with a
// fixed initial count and balanced operations.
func TestRefCount_Concurrent(t *testing.T) {
	var releases int32
	r := NewRefCount[int](0, func(int) error {
		atomic.AddInt32(&releases, 1)
		return nil
	})
	const N = 1024
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.IncRef()
			if err := r.DecRef(); err != nil {
				t.Errorf("concurrent DecRef: %v", err)
			}
		}()
	}
	wg.Wait()
	// Final balanced state: count must still be 1 (the initial ref).
	if c := r.GetRefCount(); c != 1 {
		t.Fatalf("final refCount=%d want 1", c)
	}
	if got := atomic.LoadInt32(&releases); got != 0 {
		t.Fatalf("expected zero releases, got %d", got)
	}
}

// TestRefCount_Close mirrors a typical io.Closer interaction.
func TestRefCount_Close(t *testing.T) {
	var released bool
	r := NewRefCount[int](7, func(int) error { released = true; return nil })
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !released {
		t.Fatalf("expected release on Close")
	}
}
