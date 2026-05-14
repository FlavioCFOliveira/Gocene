// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"testing"
)

func TestIntBlockPool_Constants(t *testing.T) {
	if IntBlockShift != 13 {
		t.Fatalf("IntBlockShift=%d want 13", IntBlockShift)
	}
	if IntBlockSize != 1<<13 {
		t.Fatalf("IntBlockSize=%d want %d", IntBlockSize, 1<<13)
	}
	if IntBlockMask != IntBlockSize-1 {
		t.Fatalf("IntBlockMask=%d want %d", IntBlockMask, IntBlockSize-1)
	}
}

func TestIntBlockPool_InitialState(t *testing.T) {
	p := NewIntBlockPool()
	if p.BufferUpto() != -1 {
		t.Fatalf("BufferUpto()=%d want -1", p.BufferUpto())
	}
	if p.IntUpto != IntBlockSize {
		t.Fatalf("IntUpto=%d want %d", p.IntUpto, IntBlockSize)
	}
	if p.IntOffset != -IntBlockSize {
		t.Fatalf("IntOffset=%d want %d", p.IntOffset, -IntBlockSize)
	}
	if p.Buffer != nil {
		t.Fatalf("Buffer should be nil before NextBuffer")
	}
}

func TestIntBlockPool_NextBufferAdvancesOffsets(t *testing.T) {
	p := NewIntBlockPool()
	p.NextBuffer()
	if p.BufferUpto() != 0 {
		t.Fatalf("BufferUpto=%d want 0", p.BufferUpto())
	}
	if p.IntUpto != 0 {
		t.Fatalf("IntUpto=%d want 0", p.IntUpto)
	}
	if p.IntOffset != 0 {
		t.Fatalf("IntOffset=%d want 0", p.IntOffset)
	}
	if len(p.Buffer) != IntBlockSize {
		t.Fatalf("Buffer length=%d want %d", len(p.Buffer), IntBlockSize)
	}
	p.NextBuffer()
	if p.BufferUpto() != 1 {
		t.Fatalf("BufferUpto=%d want 1", p.BufferUpto())
	}
	if p.IntOffset != IntBlockSize {
		t.Fatalf("IntOffset=%d want %d", p.IntOffset, IntBlockSize)
	}
}

func TestIntBlockPool_GrowsBuffersArray(t *testing.T) {
	p := NewIntBlockPool()
	initialCap := len(p.Buffers)
	// Force growth: allocate initialCap+1 buffers.
	for i := 0; i < initialCap+1; i++ {
		p.NextBuffer()
	}
	if len(p.Buffers) <= initialCap {
		t.Fatalf("Buffers slice did not grow: len=%d initial=%d", len(p.Buffers), initialCap)
	}
	// Java uses (int)(length * 1.5), i.e. 10 -> 15.
	want := int(float64(initialCap) * 1.5)
	if len(p.Buffers) != want {
		t.Fatalf("Buffers slice grew to %d want %d", len(p.Buffers), want)
	}
}

func TestIntBlockPool_ResetNoReuseClearsState(t *testing.T) {
	p := NewIntBlockPool()
	p.NextBuffer()
	p.Buffer[0] = 42
	p.IntUpto = 10

	p.Reset(false, false)
	if p.BufferUpto() != -1 {
		t.Fatalf("BufferUpto=%d want -1", p.BufferUpto())
	}
	if p.IntUpto != IntBlockSize {
		t.Fatalf("IntUpto=%d want %d", p.IntUpto, IntBlockSize)
	}
	if p.IntOffset != -IntBlockSize {
		t.Fatalf("IntOffset=%d want %d", p.IntOffset, -IntBlockSize)
	}
	if p.Buffer != nil {
		t.Fatalf("Buffer should be nil after Reset(false, false)")
	}
}

func TestIntBlockPool_ResetReuseFirstKeepsBuffer(t *testing.T) {
	p := NewIntBlockPool()
	p.NextBuffer()
	first := p.Buffer
	p.Buffer[3] = 99
	p.IntUpto = 7

	p.Reset(false, true)

	if p.BufferUpto() != 0 {
		t.Fatalf("BufferUpto=%d want 0", p.BufferUpto())
	}
	if p.IntUpto != 0 {
		t.Fatalf("IntUpto=%d want 0", p.IntUpto)
	}
	if p.IntOffset != 0 {
		t.Fatalf("IntOffset=%d want 0", p.IntOffset)
	}
	if &p.Buffer[0] != &first[0] {
		t.Fatalf("Reset(reuseFirst=true) did not reuse first buffer")
	}
	if p.Buffer[3] != 99 {
		t.Fatalf("Reset(zeroFillBuffers=false) erased buffer contents")
	}
}

func TestIntBlockPool_ResetZeroFillFullBuffers(t *testing.T) {
	p := NewIntBlockPool()
	p.NextBuffer()
	// Fill first buffer completely.
	for i := range p.Buffer {
		p.Buffer[i] = int32(i + 1)
	}
	p.IntUpto = IntBlockSize
	first := p.Buffer

	p.NextBuffer()
	// Write 7 values into the second (head) buffer; partial-fill region.
	for i := 0; i < 7; i++ {
		p.Buffer[i] = int32(i + 100)
	}
	p.IntUpto = 7

	p.Reset(true, true)

	for i, v := range first {
		if v != 0 {
			t.Fatalf("first buffer not zero-filled at i=%d: %d", i, v)
		}
	}
}

func TestIntBlockPool_ResetZeroFillPartialFinalBuffer(t *testing.T) {
	p := NewIntBlockPool()
	p.NextBuffer()
	// Write 5 values into the head buffer.
	for i := 0; i < 5; i++ {
		p.Buffer[i] = int32(i + 1)
	}
	// Pre-populate indices 5..9 — these must NOT be zeroed by Reset because
	// they sit beyond IntUpto and are considered "untouched" by Lucene.
	for i := 5; i < 10; i++ {
		p.Buffer[i] = 777
	}
	p.IntUpto = 5

	p.Reset(true, true)

	// Slots [0,5) were inside [0, IntUpto) and must be zeroed.
	for i := 0; i < 5; i++ {
		if p.Buffer[i] != 0 {
			t.Fatalf("buffer not zero-filled at i=%d: %d", i, p.Buffer[i])
		}
	}
	// Slots beyond IntUpto are untouched. Lucene zero-fills only [0, intUpto).
	for i := 5; i < 10; i++ {
		if p.Buffer[i] != 777 {
			t.Fatalf("buffer beyond IntUpto modified at i=%d: %d", i, p.Buffer[i])
		}
	}
}

// recyclingIntAllocator records every Recycle call and reuses returned blocks.
type recyclingIntAllocator struct {
	pool          [][]int32
	recycledStart []int
	recycledEnd   []int
	recycledLens  []int
	allocations   int
}

func (r *recyclingIntAllocator) GetIntBlock() []int32 {
	if n := len(r.pool); n > 0 {
		blk := r.pool[n-1]
		r.pool = r.pool[:n-1]
		return blk
	}
	r.allocations++
	return make([]int32, IntBlockSize)
}

func (r *recyclingIntAllocator) RecycleIntBlocks(blocks [][]int32, start, end int) {
	r.recycledStart = append(r.recycledStart, start)
	r.recycledEnd = append(r.recycledEnd, end)
	r.recycledLens = append(r.recycledLens, end-start)
	for i := start; i < end; i++ {
		if blocks[i] != nil {
			r.pool = append(r.pool, blocks[i])
		}
	}
}

func TestIntBlockPool_ResetCallsRecycle(t *testing.T) {
	alloc := &recyclingIntAllocator{}
	p := NewIntBlockPoolWithAllocator(alloc)
	p.NextBuffer()
	p.NextBuffer()
	p.NextBuffer()

	p.Reset(false, false)
	if len(alloc.recycledStart) != 1 {
		t.Fatalf("Recycle calls=%d want 1", len(alloc.recycledStart))
	}
	if alloc.recycledStart[0] != 0 || alloc.recycledEnd[0] != 3 {
		t.Fatalf("Recycle range=[%d,%d) want [0,3)", alloc.recycledStart[0], alloc.recycledEnd[0])
	}
	// Buffers in [0, bufferUpto] must be nil-ed afterwards.
	for i := 0; i < 3; i++ {
		if p.Buffers[i] != nil {
			t.Fatalf("Buffers[%d] not nil after Reset", i)
		}
	}
}

func TestIntBlockPool_ResetReuseFirstRecyclesTail(t *testing.T) {
	alloc := &recyclingIntAllocator{}
	p := NewIntBlockPoolWithAllocator(alloc)
	p.NextBuffer()
	p.NextBuffer()
	p.NextBuffer()
	first := p.Buffers[0]

	p.Reset(false, true)
	if len(alloc.recycledStart) != 1 {
		t.Fatalf("Recycle calls=%d want 1", len(alloc.recycledStart))
	}
	if alloc.recycledStart[0] != 1 || alloc.recycledEnd[0] != 3 {
		t.Fatalf("Recycle range=[%d,%d) want [1,3)", alloc.recycledStart[0], alloc.recycledEnd[0])
	}
	if p.Buffer == nil || &p.Buffer[0] != &first[0] {
		t.Fatalf("first buffer not preserved")
	}
}

func TestIntBlockPool_ResetReuseFirstWithSingleBufferKeepsBufferUpto(t *testing.T) {
	// When bufferUpto == 0 AND reuseFirst is true Lucene short-circuits the
	// recycle path: no buffer is released and the head remains as-is.
	alloc := &recyclingIntAllocator{}
	p := NewIntBlockPoolWithAllocator(alloc)
	p.NextBuffer()

	p.Reset(false, true)
	if len(alloc.recycledStart) != 0 {
		t.Fatalf("expected no Recycle call but got %d", len(alloc.recycledStart))
	}
	if p.BufferUpto() != 0 {
		t.Fatalf("BufferUpto=%d want 0", p.BufferUpto())
	}
}

func TestIntBlockPool_WriteThenReadBackPattern(t *testing.T) {
	p := NewIntBlockPool()
	// Write a known pattern across multiple buffers.
	const total = 3 * IntBlockSize
	for i := 0; i < total; i++ {
		if p.IntUpto == IntBlockSize {
			p.NextBuffer()
		}
		p.Buffer[p.IntUpto] = int32(i)
		p.IntUpto++
	}
	// Read back and verify positional correctness.
	for i := 0; i < total; i++ {
		bufIdx := i >> IntBlockShift
		slot := i & IntBlockMask
		got := p.Buffers[bufIdx][slot]
		if got != int32(i) {
			t.Fatalf("global %d -> buffers[%d][%d]=%d want %d", i, bufIdx, slot, got, i)
		}
	}
}

func TestIntBlockPool_DirectAllocator(t *testing.T) {
	a := NewDirectIntAllocator()
	b := a.GetIntBlock()
	if len(b) != IntBlockSize {
		t.Fatalf("block size=%d want %d", len(b), IntBlockSize)
	}
	// Recycle is a no-op; should not panic regardless of inputs.
	a.RecycleIntBlocks([][]int32{nil, nil}, 0, 2)
}

func TestIntBlockPool_RandomisedNextBufferGrowth(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	const iters = 64
	p := NewIntBlockPool()
	for i := 0; i < iters; i++ {
		p.NextBuffer()
		if p.IntOffset != i*IntBlockSize {
			t.Fatalf("iter %d: IntOffset=%d want %d", i, p.IntOffset, i*IntBlockSize)
		}
		// Touch a random slot to make sure the buffer is usable.
		idx := rng.Intn(IntBlockSize)
		p.Buffer[idx] = int32(i)
		if p.Buffers[i][idx] != int32(i) {
			t.Fatalf("iter %d: Buffers[%d][%d]=%d want %d", i, i, idx, p.Buffers[i][idx], int32(i))
		}
	}
}
