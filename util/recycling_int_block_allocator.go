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

import "fmt"

// DefaultRecycledIntBuffers is the default maximum number of recycled int
// blocks held by a RecyclingIntBlockAllocator. Matches Lucene's
// RecyclingIntBlockAllocator.DEFAULT_BUFFERED_BLOCKS.
const DefaultRecycledIntBuffers = 64

// RecyclingIntBlockAllocator is a port of
// org.apache.lucene.util.RecyclingIntBlockAllocator. It reuses recycled int
// blocks up to maxBufferedBlocks before falling back to fresh allocations,
// tracking the running byte total in a Counter.
//
// Concurrency: not safe for concurrent use, matching Lucene's note.
type RecyclingIntBlockAllocator struct {
	blockSize         int
	freeIntBlocks     [][]int32
	maxBufferedBlocks int
	freeBlocks        int
	bytesUsed         *Counter
}

// NewRecyclingIntBlockAllocator constructs a recycler with the given block
// size and maxBufferedBlocks. The Counter tracks running byte usage; a nil
// counter allocates a fresh one.
func NewRecyclingIntBlockAllocator(blockSize, maxBufferedBlocks int, bytesUsed *Counter) *RecyclingIntBlockAllocator {
	if blockSize <= 0 {
		panic(fmt.Sprintf("blockSize must be > 0; got: %d", blockSize))
	}
	if maxBufferedBlocks < 0 {
		panic(fmt.Sprintf("maxBufferedBlocks must be >= 0; got: %d", maxBufferedBlocks))
	}
	if bytesUsed == nil {
		bytesUsed = NewCounter()
	}
	return &RecyclingIntBlockAllocator{
		blockSize:         blockSize,
		freeIntBlocks:     make([][]int32, maxBufferedBlocks),
		maxBufferedBlocks: maxBufferedBlocks,
		bytesUsed:         bytesUsed,
	}
}

// NewRecyclingIntBlockAllocatorDefault constructs a recycler with the
// default IntBlockPool block size and DefaultRecycledIntBuffers buffered
// blocks. Mirrors Lucene's no-arg constructor.
func NewRecyclingIntBlockAllocatorDefault() *RecyclingIntBlockAllocator {
	return NewRecyclingIntBlockAllocator(IntBlockSize, DefaultRecycledIntBuffers, NewCounter())
}

// GetIntBlock returns a recycled block when one is available, otherwise
// allocates a fresh block and accounts for it in bytesUsed.
func (r *RecyclingIntBlockAllocator) GetIntBlock() []int32 {
	if r.freeBlocks == 0 {
		// Account for blockSize * sizeof(int32) bytes.
		r.bytesUsed.AddAndGet(int64(r.blockSize) * 4)
		return make([]int32, r.blockSize)
	}
	r.freeBlocks--
	b := r.freeIntBlocks[r.freeBlocks]
	r.freeIntBlocks[r.freeBlocks] = nil
	return b
}

// RecycleIntBlocks places blocks[start:end] back into the recycler up to
// maxBufferedBlocks; any overflow is dropped and accounted for.
func (r *RecyclingIntBlockAllocator) RecycleIntBlocks(blocks [][]int32, start, end int) {
	numBlocks := r.maxBufferedBlocks - r.freeBlocks
	if end-start < numBlocks {
		numBlocks = end - start
	}
	size := r.freeBlocks + numBlocks
	if size >= len(r.freeIntBlocks) {
		newCap := oversize(size, 8) // sizeof(reference)
		newBlocks := make([][]int32, newCap)
		copy(newBlocks, r.freeIntBlocks[:r.freeBlocks])
		r.freeIntBlocks = newBlocks
	}
	stop := start + numBlocks
	for i := start; i < stop; i++ {
		r.freeIntBlocks[r.freeBlocks] = blocks[i]
		r.freeBlocks++
		blocks[i] = nil
	}
	for i := stop; i < end; i++ {
		blocks[i] = nil
	}
	dropped := int64(end-stop) * int64(r.blockSize) * 4
	r.bytesUsed.AddAndGet(-dropped)
}

// NumBufferedBlocks returns the current number of recycled blocks.
func (r *RecyclingIntBlockAllocator) NumBufferedBlocks() int { return r.freeBlocks }

// BytesUsed returns the total number of bytes currently allocated.
func (r *RecyclingIntBlockAllocator) BytesUsed() int64 { return r.bytesUsed.Get() }

// MaxBufferedBlocks returns the configured upper bound on recycled blocks.
func (r *RecyclingIntBlockAllocator) MaxBufferedBlocks() int { return r.maxBufferedBlocks }

// FreeBlocks drops up to num recycled blocks, returning how many were
// actually freed. Each freed block subtracts blockSize * sizeof(int32) bytes
// from the running total.
func (r *RecyclingIntBlockAllocator) FreeBlocks(num int) int {
	if num < 0 {
		panic(fmt.Sprintf("free blocks must be >= 0 but was: %d", num))
	}
	var stop, count int
	if num > r.freeBlocks {
		stop = 0
		count = r.freeBlocks
	} else {
		stop = r.freeBlocks - num
		count = num
	}
	for r.freeBlocks > stop {
		r.freeBlocks--
		r.freeIntBlocks[r.freeBlocks] = nil
	}
	r.bytesUsed.AddAndGet(-int64(count) * int64(r.blockSize) * 4)
	return count
}

// BlockSize returns the per-block element count this allocator hands out.
func (r *RecyclingIntBlockAllocator) BlockSize() int { return r.blockSize }
