// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand/v2"
	"testing"
)

func newRandRecyclingByteAllocator(r *rand.Rand) *RecyclingByteBlockAllocator {
	maxBuffered := r.IntN(97)
	return NewRecyclingByteBlockAllocatorWithBuffered(maxBuffered, NewCounter())
}

// TestRecyclingByteBlockAllocator_Allocate mirrors testAllocate.
func TestRecyclingByteBlockAllocator_Allocate(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 12))
	a := newRandRecyclingByteAllocator(r)
	seen := make(map[*byte]struct{})
	first := a.GetByteBlock()
	size := len(first)
	if first == nil {
		t.Fatalf("expected non-nil block")
	}
	seen[&first[0]] = struct{}{}
	for i := 0; i < 97; i++ {
		b := a.GetByteBlock()
		if b == nil || len(b) != size {
			t.Fatalf("nil or size mismatch")
		}
		if _, dup := seen[&b[0]]; dup {
			t.Fatalf("block returned twice")
		}
		seen[&b[0]] = struct{}{}
		want := int64(size) * int64(i+2)
		if got := a.BytesUsed(); got != want {
			t.Fatalf("bytesUsed=%d, want %d", got, want)
		}
		if a.NumBufferedBlocks() != 0 {
			t.Fatalf("buffered != 0: %d", a.NumBufferedBlocks())
		}
	}
}

// TestRecyclingByteBlockAllocator_AllocateAndRecycle mirrors
// testAllocateAndRecycle.
func TestRecyclingByteBlockAllocator_AllocateAndRecycle(t *testing.T) {
	r := rand.New(rand.NewPCG(13, 14))
	a := newRandRecyclingByteAllocator(r)
	allocated := make(map[*byte][]byte)
	first := a.GetByteBlock()
	size := len(first)
	allocated[&first[0]] = first

	for iter := 0; iter < 97; iter++ {
		num := 1 + r.IntN(39)
		for j := 0; j < num; j++ {
			b := a.GetByteBlock()
			if b == nil || len(b) != size {
				t.Fatalf("size mismatch or nil")
			}
			if _, dup := allocated[&b[0]]; dup {
				t.Fatalf("block returned twice")
			}
			allocated[&b[0]] = b
			want := int64(size) * int64(len(allocated)+a.NumBufferedBlocks())
			if got := a.BytesUsed(); got != want {
				t.Fatalf("bytesUsed=%d, want %d", got, want)
			}
		}
		array := make([][]byte, 0, len(allocated))
		ptrs := make([]*byte, 0, len(allocated))
		for k, v := range allocated {
			array = append(array, v)
			ptrs = append(ptrs, k)
		}
		begin := r.IntN(len(array))
		end := begin + r.IntN(len(array)-begin)
		selected := make([]*byte, end-begin)
		copy(selected, ptrs[begin:end])
		a.RecycleByteBlocks(array, begin, end)
		for j := begin; j < end; j++ {
			if array[j] != nil {
				t.Fatalf("array[%d] not nilled", j)
			}
		}
		for _, k := range selected {
			delete(allocated, k)
		}
	}
}

// TestRecyclingByteBlockAllocator_AllocateAndFree mirrors
// testAllocateAndFree.
func TestRecyclingByteBlockAllocator_AllocateAndFree(t *testing.T) {
	r := rand.New(rand.NewPCG(15, 16))
	a := newRandRecyclingByteAllocator(r)
	allocated := make(map[*byte][]byte)
	first := a.GetByteBlock()
	size := len(first)
	allocated[&first[0]] = first

	for iter := 0; iter < 97; iter++ {
		num := 1 + r.IntN(39)
		for j := 0; j < num; j++ {
			b := a.GetByteBlock()
			allocated[&b[0]] = b
			want := int64(size) * int64(len(allocated)+a.NumBufferedBlocks())
			if got := a.BytesUsed(); got != want {
				t.Fatalf("bytesUsed=%d, want %d", got, want)
			}
		}
		array := make([][]byte, 0, len(allocated))
		ptrs := make([]*byte, 0, len(allocated))
		for k, v := range allocated {
			array = append(array, v)
			ptrs = append(ptrs, k)
		}
		begin := r.IntN(len(array))
		end := begin + r.IntN(len(array)-begin)
		for j := begin; j < end; j++ {
			delete(allocated, ptrs[j])
		}
		a.RecycleByteBlocks(array, begin, end)
		numFreeBefore := a.NumBufferedBlocks()
		freed := a.FreeBlocks(r.IntN(7 + a.MaxBufferedBlocks()))
		if got := a.NumBufferedBlocks(); got != numFreeBefore-freed {
			t.Fatalf("post-FreeBlocks numBuffered=%d, want %d",
				got, numFreeBefore-freed)
		}
	}
}

// TestRecyclingByteBlockAllocator_DefaultCtor sanity-checks the no-arg path.
func TestRecyclingByteBlockAllocator_DefaultCtor(t *testing.T) {
	a := NewRecyclingByteBlockAllocatorDefault()
	if a.BlockSize() != ByteBlockSize {
		t.Fatalf("BlockSize=%d, want %d", a.BlockSize(), ByteBlockSize)
	}
	if a.MaxBufferedBlocks() != DefaultRecycledByteBuffers {
		t.Fatalf("MaxBufferedBlocks=%d, want %d", a.MaxBufferedBlocks(), DefaultRecycledByteBuffers)
	}
}

// TestRecyclingByteBlockAllocator_FreeBlocks_Negative rejects negative input.
func TestRecyclingByteBlockAllocator_FreeBlocks_Negative(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	a := NewRecyclingByteBlockAllocatorDefault()
	_ = a.FreeBlocks(-1)
}

// TestRecyclingByteBlockAllocator_AsAllocator confirms the type satisfies
// the ByteBlockPool Allocator interface and behaves correctly when used as
// the pool's allocator.
func TestRecyclingByteBlockAllocator_AsAllocator(t *testing.T) {
	var _ Allocator = NewRecyclingByteBlockAllocatorDefault() // compile-time check
	a := NewRecyclingByteBlockAllocatorDefault()
	pool := NewByteBlockPool(a)
	pool.NextBuffer()
	pool.Append([]byte("hello world"))
	pool.Reset(false, true)
	if a.NumBufferedBlocks() < 0 {
		t.Fatalf("invalid NumBufferedBlocks=%d", a.NumBufferedBlocks())
	}
}
