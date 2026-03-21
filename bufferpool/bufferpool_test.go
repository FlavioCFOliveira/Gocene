// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bufferpool

import (
	"testing"
)

func TestNew(t *testing.T) {
	pool := New(1024)
	if pool == nil {
		t.Fatal("New returned nil")
	}
	if pool.Size() != 1024 {
		t.Errorf("Expected size 1024, got %d", pool.Size())
	}
}

func TestNew_DefaultSize(t *testing.T) {
	pool := New(0)
	if pool.Size() != 8192 {
		t.Errorf("Expected default size 8192, got %d", pool.Size())
	}
}

func TestPool_GetPut(t *testing.T) {
	pool := New(1024)

	// Get a buffer
	buf := pool.Get()
	if len(buf) != 1024 {
		t.Errorf("Expected buffer length 1024, got %d", len(buf))
	}

	// Write to buffer
	for i := range buf {
		buf[i] = byte(i % 256)
	}

	// Put it back
	pool.Put(buf)

	// Get another buffer (should be reused)
	buf2 := pool.Get()
	if len(buf2) != 1024 {
		t.Errorf("Expected buffer length 1024, got %d", len(buf2))
	}
}

func TestPool_PutNil(t *testing.T) {
	pool := New(1024)
	// Should not panic
	pool.Put(nil)
}

func TestNewSizedPool(t *testing.T) {
	sp := NewSizedPool(1024, 4096, 16384)
	if sp == nil {
		t.Fatal("NewSizedPool returned nil")
	}

	if len(sp.pools) != 3 {
		t.Errorf("Expected 3 pools, got %d", len(sp.pools))
	}
}

func TestSizedPool_Get(t *testing.T) {
	sp := NewSizedPool(1024, 4096)

	// Get a buffer that fits in 1024 pool
	buf := sp.Get(512)
	if cap(buf) != 1024 {
		t.Errorf("Expected buffer capacity 1024, got %d", cap(buf))
	}

	// Get a buffer that fits in 4096 pool
	buf2 := sp.Get(2048)
	if cap(buf2) != 4096 {
		t.Errorf("Expected buffer capacity 4096, got %d", cap(buf2))
	}

	// Get a buffer that doesn't fit in any pool
	buf3 := sp.Get(8192)
	if cap(buf3) != 8192 {
		t.Errorf("Expected buffer capacity 8196, got %d", cap(buf3))
	}

	// Put buffers back
	sp.Put(buf)
	sp.Put(buf2)
	sp.Put(buf3)
}

func TestSizedPool_PutNil(t *testing.T) {
	sp := NewSizedPool(1024)
	// Should not panic
	sp.Put(nil)
}

func BenchmarkPool_GetPut(b *testing.B) {
	pool := New(8192)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

func BenchmarkPool_GetPutParallel(b *testing.B) {
	pool := New(8192)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			pool.Put(buf)
		}
	})
}
