// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestPerContextCache_GetReturnsInitial(t *testing.T) {
	t.Parallel()

	c := NewPerContextCache[int](func() int { return 42 })
	defer func() { _ = c.Close() }()

	if got := c.Get("ctx1"); got != 42 {
		t.Errorf("Get = %d, want 42", got)
	}
}

func TestPerContextCache_GetCachesAcrossCalls(t *testing.T) {
	t.Parallel()

	var n int32
	c := NewPerContextCache[int32](func() int32 {
		return atomic.AddInt32(&n, 1)
	})
	defer func() { _ = c.Close() }()

	first := c.Get("k")
	second := c.Get("k")
	if first != second {
		t.Errorf("two calls returned %d, %d — want stable", first, second)
	}
	if atomic.LoadInt32(&n) != 1 {
		t.Errorf("initial invoked %d times, want 1", atomic.LoadInt32(&n))
	}
}

func TestPerContextCache_PerContextIsolation(t *testing.T) {
	t.Parallel()

	c := NewPerContextCache[string](func() string { return "" })
	defer func() { _ = c.Close() }()

	c.Put("a", "alpha")
	c.Put("b", "beta")

	if got := c.Get("a"); got != "alpha" {
		t.Errorf("Get(a) = %q", got)
	}
	if got := c.Get("b"); got != "beta" {
		t.Errorf("Get(b) = %q", got)
	}
}

func TestPerContextCache_Remove(t *testing.T) {
	t.Parallel()

	c := NewPerContextCache[int](nil)
	defer func() { _ = c.Close() }()

	c.Put("a", 1)
	if !c.Remove("a") {
		t.Errorf("Remove(a) = false, want true")
	}
	if c.Remove("a") {
		t.Errorf("Remove(a) again = true, want false")
	}
	if got := c.Get("a"); got != 0 {
		t.Errorf("Get(a) after Remove = %d, want 0", got)
	}
}

func TestPerContextCache_NilInitial_ReturnsZero(t *testing.T) {
	t.Parallel()

	c := NewPerContextCache[int](nil)
	defer func() { _ = c.Close() }()

	if got := c.Get("k"); got != 0 {
		t.Errorf("nil initial -> Get = %d, want 0", got)
	}
}

func TestPerContextCache_CloseDropsAll(t *testing.T) {
	t.Parallel()

	c := NewPerContextCache[int](func() int { return 7 })
	c.Put("a", 1)
	c.Put("b", 2)
	if l := c.Len(); l != 2 {
		t.Errorf("Len before Close = %d, want 2", l)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
	if l := c.Len(); l != 0 {
		t.Errorf("Len after Close = %d, want 0", l)
	}
	// Get after Close returns the zero value and does not re-populate.
	if got := c.Get("a"); got != 0 {
		t.Errorf("Get after Close = %d, want 0", got)
	}
	// Idempotent.
	if err := c.Close(); err != nil {
		t.Errorf("second Close error: %v", err)
	}
	// Put after Close is a no-op.
	c.Put("a", 99)
	if got := c.Get("a"); got != 0 {
		t.Errorf("Put after Close took effect: got %d", got)
	}
}

func TestPerContextCache_Concurrent(t *testing.T) {
	t.Parallel()

	const goroutines = 32
	const perCtxCalls = 256

	c := NewPerContextCache[int](func() int { return -1 })
	defer func() { _ = c.Close() }()

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			ctx := g
			c.Put(ctx, g*10)
			for i := 0; i < perCtxCalls; i++ {
				if v := c.Get(ctx); v != g*10 {
					t.Errorf("goroutine %d got %d, want %d", g, v, g*10)
					return
				}
			}
		}()
	}
	wg.Wait()
	if c.Len() != goroutines {
		t.Errorf("Len = %d, want %d", c.Len(), goroutines)
	}
}

// Compile-time check: the Java alias retains the same usable surface.
func TestCloseableThreadLocal_AliasCompiles(t *testing.T) {
	t.Parallel()

	c := NewCloseableThreadLocal[*int](nil)
	defer func() { _ = c.Close() }()

	v := 5
	c.Put("k", &v)
	if got := c.Get("k"); got == nil || *got != 5 {
		t.Errorf("alias Get = %v", got)
	}
}
