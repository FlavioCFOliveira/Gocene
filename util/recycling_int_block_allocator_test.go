// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand/v2"
	"testing"
)

func newRandRecyclingIntAllocator(r *rand.Rand) *RecyclingIntBlockAllocator {
	blockSize := 1 << (2 + r.IntN(15))
	maxBuffered := r.IntN(97)
	return NewRecyclingIntBlockAllocator(blockSize, maxBuffered, NewCounter())
}

// TestRecyclingIntBlockAllocator_Allocate mirrors testAllocate: every block
// returned must be distinct, of the same size, and the running bytesUsed
// total must include the just-issued block.
func TestRecyclingIntBlockAllocator_Allocate(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	a := newRandRecyclingIntAllocator(r)
	seen := make(map[*int32]struct{})
	first := a.GetIntBlock()
	size := len(first)
	if first == nil {
		t.Fatalf("expected non-nil block")
	}
	seen[&first[0]] = struct{}{}

	for i := 0; i < 97; i++ {
		b := a.GetIntBlock()
		if b == nil || len(b) != size {
			t.Fatalf("size mismatch or nil: %v %d", b == nil, len(b))
		}
		if _, dup := seen[&b[0]]; dup {
			t.Fatalf("block returned twice")
		}
		seen[&b[0]] = struct{}{}
		want := int64(4*size) * int64(i+2)
		if got := a.BytesUsed(); got != want {
			t.Fatalf("bytesUsed=%d, want %d", got, want)
		}
		if a.NumBufferedBlocks() != 0 {
			t.Fatalf("expected 0 buffered blocks, got %d", a.NumBufferedBlocks())
		}
	}
}

// TestRecyclingIntBlockAllocator_AllocateAndRecycle mirrors
// testAllocateAndRecycle.
func TestRecyclingIntBlockAllocator_AllocateAndRecycle(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	a := newRandRecyclingIntAllocator(r)
	allocated := make(map[*int32][]int32)
	first := a.GetIntBlock()
	size := len(first)
	allocated[&first[0]] = first

	for iter := 0; iter < 97; iter++ {
		num := 1 + r.IntN(39)
		for j := 0; j < num; j++ {
			b := a.GetIntBlock()
			if b == nil || len(b) != size {
				t.Fatalf("nil or size mismatch")
			}
			if _, dup := allocated[&b[0]]; dup {
				t.Fatalf("block returned twice")
			}
			allocated[&b[0]] = b
			want := int64(4*size) * int64(len(allocated)+a.NumBufferedBlocks())
			if got := a.BytesUsed(); got != want {
				t.Fatalf("bytesUsed=%d, want %d (allocated=%d buffered=%d)",
					got, want, len(allocated), a.NumBufferedBlocks())
			}
		}
		// Pick a random window and recycle.
		array := make([][]int32, 0, len(allocated))
		ptrs := make([]*int32, 0, len(allocated))
		for k, v := range allocated {
			array = append(array, v)
			ptrs = append(ptrs, k)
		}
		begin := r.IntN(len(array))
		end := begin + r.IntN(len(array)-begin)
		// Snapshot the selected slice so we can update `allocated` after the recycle.
		selected := make([]*int32, end-begin)
		copy(selected, ptrs[begin:end])
		a.RecycleIntBlocks(array, begin, end)
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

// TestRecyclingIntBlockAllocator_AllocateAndFree mirrors testAllocateAndFree.
func TestRecyclingIntBlockAllocator_AllocateAndFree(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	a := newRandRecyclingIntAllocator(r)
	allocated := make(map[*int32][]int32)
	first := a.GetIntBlock()
	size := len(first)
	allocated[&first[0]] = first

	for iter := 0; iter < 97; iter++ {
		num := 1 + r.IntN(39)
		for j := 0; j < num; j++ {
			b := a.GetIntBlock()
			allocated[&b[0]] = b
			want := int64(4*size) * int64(len(allocated)+a.NumBufferedBlocks())
			if got := a.BytesUsed(); got != want {
				t.Fatalf("bytesUsed=%d, want %d", got, want)
			}
		}
		// Pick a window, drop from the allocated set, recycle.
		array := make([][]int32, 0, len(allocated))
		ptrs := make([]*int32, 0, len(allocated))
		for k, v := range allocated {
			array = append(array, v)
			ptrs = append(ptrs, k)
		}
		begin := r.IntN(len(array))
		end := begin + r.IntN(len(array)-begin)
		for j := begin; j < end; j++ {
			delete(allocated, ptrs[j])
		}
		a.RecycleIntBlocks(array, begin, end)
		// FreeBlocks should not over-report.
		numFreeBefore := a.NumBufferedBlocks()
		freed := a.FreeBlocks(r.IntN(7 + a.MaxBufferedBlocks()))
		if got := a.NumBufferedBlocks(); got != numFreeBefore-freed {
			t.Fatalf("post-FreeBlocks numBuffered=%d, want %d",
				got, numFreeBefore-freed)
		}
	}
}

// TestRecyclingIntBlockAllocator_DefaultCtor sanity-checks the no-arg path.
func TestRecyclingIntBlockAllocator_DefaultCtor(t *testing.T) {
	a := NewRecyclingIntBlockAllocatorDefault()
	if a.BlockSize() != IntBlockSize {
		t.Fatalf("BlockSize=%d, want %d", a.BlockSize(), IntBlockSize)
	}
	if a.MaxBufferedBlocks() != DefaultRecycledIntBuffers {
		t.Fatalf("MaxBufferedBlocks=%d, want %d", a.MaxBufferedBlocks(), DefaultRecycledIntBuffers)
	}
}

// TestRecyclingIntBlockAllocator_FreeBlocks_Negative rejects negative input.
func TestRecyclingIntBlockAllocator_FreeBlocks_Negative(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic for negative argument")
		}
	}()
	a := NewRecyclingIntBlockAllocatorDefault()
	_ = a.FreeBlocks(-1)
}
