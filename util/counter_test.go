// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"sync"
	"testing"
)

func TestSerialCounter_AddAndGet(t *testing.T) {
	t.Parallel()

	c := NewSerialCounter()
	if got := c.Get(); got != 0 {
		t.Errorf("zero counter Get = %d, want 0", got)
	}
	if got := c.AddAndGet(5); got != 5 {
		t.Errorf("AddAndGet(5) = %d, want 5", got)
	}
	if got := c.AddAndGet(-3); got != 2 {
		t.Errorf("AddAndGet(-3) = %d, want 2", got)
	}
	if got := c.Get(); got != 2 {
		t.Errorf("Get after deltas = %d, want 2", got)
	}
}

func TestNewCounterOf_ThreadSafeReturnsAtomic(t *testing.T) {
	t.Parallel()

	c := NewCounterOf(true)
	const goroutines = 64
	const perGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				c.AddAndGet(1)
			}
		}()
	}
	wg.Wait()

	if got := c.Get(); got != int64(goroutines*perGoroutine) {
		t.Errorf("atomic counter total = %d, want %d", got, goroutines*perGoroutine)
	}
}

func TestNewCounterOf_NotThreadSafeReturnsSerial(t *testing.T) {
	t.Parallel()

	c := NewCounterOf(false)
	if _, ok := c.(*SerialCounter); !ok {
		t.Errorf("NewCounterOf(false) returned %T, want *SerialCounter", c)
	}
}

func TestNewCounterThreadSafe_ReturnsAtomic(t *testing.T) {
	t.Parallel()

	c := NewCounterThreadSafe()
	if _, ok := c.(*Counter); !ok {
		t.Errorf("NewCounterThreadSafe returned %T, want *Counter", c)
	}
}

// Static type checks: both variants satisfy CounterAPI.
var (
	_ CounterAPI = (*Counter)(nil)
	_ CounterAPI = (*SerialCounter)(nil)
)
