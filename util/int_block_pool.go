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
//	http://www.apache.org/licenses/LICENSE-2.0

package util

// IntBlockPool constants mirror Lucene's IntBlockPool sizing.
const (
	// IntBlockShift is used to derive a buffer index from a global int offset.
	IntBlockShift = 13

	// IntBlockSize is the number of int32 slots in each buffer.
	IntBlockSize = 1 << IntBlockShift // 8192

	// IntBlockMask masks an offset to its position inside the current buffer.
	IntBlockMask = IntBlockSize - 1
)

// IntAllocator allocates and recycles fixed-size int32 blocks used by
// IntBlockPool. It is the int counterpart to ByteBlockPool's Allocator.
type IntAllocator interface {
	// RecycleIntBlocks recycles the int blocks in blocks[start:end].
	RecycleIntBlocks(blocks [][]int32, start, end int)
	// GetIntBlock returns a new int block of size IntBlockSize.
	GetIntBlock() []int32
}

// DirectIntAllocator is a simple IntAllocator that never recycles, matching
// the behaviour of Lucene's IntBlockPool.DirectAllocator.
type DirectIntAllocator struct{}

// NewDirectIntAllocator creates a new DirectIntAllocator.
func NewDirectIntAllocator() *DirectIntAllocator {
	return &DirectIntAllocator{}
}

// RecycleIntBlocks is a no-op for DirectIntAllocator.
func (d *DirectIntAllocator) RecycleIntBlocks(blocks [][]int32, start, end int) {}

// GetIntBlock returns a freshly-allocated int block of size IntBlockSize.
func (d *DirectIntAllocator) GetIntBlock() []int32 {
	return make([]int32, IntBlockSize)
}

// IntBlockPool is the int analogue of ByteBlockPool. It manages a growable
// array of fixed-size int32 buffers handed out through the configured
// IntAllocator. This is a direct port of org.apache.lucene.util.IntBlockPool.
type IntBlockPool struct {
	// Buffers is the array of buffers currently used in the pool. Buffers are
	// allocated on demand. Do not modify outside of this type.
	Buffers [][]int32

	// bufferUpto is the index into Buffers pointing to the current head buffer.
	bufferUpto int

	// IntUpto is the position inside the current head buffer.
	IntUpto int

	// Buffer is the current head buffer (aliases Buffers[bufferUpto]).
	Buffer []int32

	// IntOffset is the global offset of the first slot of the current head
	// buffer (matches Lucene's intOffset field, initialised to -IntBlockSize).
	IntOffset int

	allocator IntAllocator
}

// NewIntBlockPool creates a new IntBlockPool with the default DirectIntAllocator.
func NewIntBlockPool() *IntBlockPool {
	return NewIntBlockPoolWithAllocator(NewDirectIntAllocator())
}

// NewIntBlockPoolWithAllocator creates a new IntBlockPool backed by the given
// IntAllocator. The pool starts in an empty state; NextBuffer must be called
// before the first int is written.
func NewIntBlockPoolWithAllocator(allocator IntAllocator) *IntBlockPool {
	return &IntBlockPool{
		Buffers:    make([][]int32, 10),
		bufferUpto: -1,
		IntUpto:    IntBlockSize,
		IntOffset:  -IntBlockSize,
		allocator:  allocator,
	}
}

// Reset resets the pool to its initial state, optionally reusing the first
// buffer. Buffers that are not reused are returned to the allocator via
// RecycleIntBlocks. When zeroFillBuffers is true the recycled buffers are
// zero-filled first; this is required by slice pools that rely on the buffers
// being zero to detect slice ends. The semantics match
// IntBlockPool.reset(boolean,boolean) in Lucene.
func (p *IntBlockPool) Reset(zeroFillBuffers, reuseFirst bool) {
	if p.bufferUpto == -1 {
		return
	}

	if zeroFillBuffers {
		for i := 0; i < p.bufferUpto; i++ {
			fillInt32(p.Buffers[i], 0)
		}
		// Partial zero fill the final buffer.
		fillInt32(p.Buffers[p.bufferUpto][:p.IntUpto], 0)
	}

	if p.bufferUpto > 0 || !reuseFirst {
		offset := 0
		if reuseFirst {
			offset = 1
		}
		// Recycle all but the first buffer.
		p.allocator.RecycleIntBlocks(p.Buffers, offset, 1+p.bufferUpto)
		for i := offset; i < p.bufferUpto+1; i++ {
			p.Buffers[i] = nil
		}
	}

	if reuseFirst {
		p.bufferUpto = 0
		p.IntUpto = 0
		p.IntOffset = 0
		p.Buffer = p.Buffers[0]
	} else {
		p.bufferUpto = -1
		p.IntUpto = IntBlockSize
		p.IntOffset = -IntBlockSize
		p.Buffer = nil
	}
}

// NextBuffer advances the pool to its next buffer. It must be called once
// after construction to initialise the head buffer; Reset already advances to
// the first buffer when reuseFirst is true and may obviate this call.
func (p *IntBlockPool) NextBuffer() {
	if 1+p.bufferUpto == len(p.Buffers) {
		newBuffers := make([][]int32, int(float64(len(p.Buffers))*1.5))
		copy(newBuffers, p.Buffers)
		p.Buffers = newBuffers
	}
	p.Buffer = p.allocator.GetIntBlock()
	p.Buffers[1+p.bufferUpto] = p.Buffer
	p.bufferUpto++

	p.IntUpto = 0
	// Math.addExact in Java; in Go int overflow is silent, so check explicitly.
	newOffset := p.IntOffset + IntBlockSize
	if (IntBlockSize > 0 && newOffset < p.IntOffset) ||
		(IntBlockSize < 0 && newOffset > p.IntOffset) {
		panic("integer overflow in IntBlockPool.NextBuffer")
	}
	p.IntOffset = newOffset
}

// BufferUpto returns the index of the current head buffer or -1 when the
// pool has not yet been advanced. Exposed because production code in Lucene
// occasionally reads the underlying state for debugging and tests rely on it.
func (p *IntBlockPool) BufferUpto() int {
	return p.bufferUpto
}

// fillInt32 fills the given slice with v. Kept private to avoid clashing
// with potential future generic helpers in this package.
func fillInt32(s []int32, v int32) {
	for i := range s {
		s[i] = v
	}
}
