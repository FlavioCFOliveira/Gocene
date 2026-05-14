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

// DefaultRecycledByteBuffers is the default maximum number of recycled byte
// blocks held by a RecyclingByteBlockAllocator. Matches Lucene's
// RecyclingByteBlockAllocator.DEFAULT_BUFFERED_BLOCKS.
const DefaultRecycledByteBuffers = 64

// RecyclingByteBlockAllocator is a port of
// org.apache.lucene.util.RecyclingByteBlockAllocator. It pairs with
// ByteBlockPool, reusing recycled blocks up to maxBufferedBlocks before
// allocating fresh, and tracking the running byte total in a Counter.
//
// Concurrency: not safe for concurrent use, matching Lucene's note.
type RecyclingByteBlockAllocator struct {
	blockSize         int
	freeByteBlocks    [][]byte
	maxBufferedBlocks int
	freeBlocks        int
	bytesUsed         *Counter
}

// NewRecyclingByteBlockAllocator constructs a recycler. A nil Counter is
// replaced by a fresh one.
func NewRecyclingByteBlockAllocator(blockSize, maxBufferedBlocks int, bytesUsed *Counter) *RecyclingByteBlockAllocator {
	if blockSize <= 0 {
		panic(fmt.Sprintf("blockSize must be > 0; got: %d", blockSize))
	}
	if maxBufferedBlocks < 0 {
		panic(fmt.Sprintf("maxBufferedBlocks must be >= 0; got: %d", maxBufferedBlocks))
	}
	if bytesUsed == nil {
		bytesUsed = NewCounter()
	}
	return &RecyclingByteBlockAllocator{
		blockSize:         blockSize,
		freeByteBlocks:    make([][]byte, maxBufferedBlocks),
		maxBufferedBlocks: maxBufferedBlocks,
		bytesUsed:         bytesUsed,
	}
}

// NewRecyclingByteBlockAllocatorDefault constructs a recycler with the
// default ByteBlockPool block size and DefaultRecycledByteBuffers buffered
// blocks. Mirrors Lucene's no-arg constructor.
func NewRecyclingByteBlockAllocatorDefault() *RecyclingByteBlockAllocator {
	return NewRecyclingByteBlockAllocator(ByteBlockSize, DefaultRecycledByteBuffers, NewCounter())
}

// NewRecyclingByteBlockAllocatorWithBuffered mirrors Lucene's two-arg
// constructor: a fixed ByteBlockSize block, the given maxBufferedBlocks and
// counter. A nil counter is replaced by a fresh one.
func NewRecyclingByteBlockAllocatorWithBuffered(maxBufferedBlocks int, bytesUsed *Counter) *RecyclingByteBlockAllocator {
	return NewRecyclingByteBlockAllocator(ByteBlockSize, maxBufferedBlocks, bytesUsed)
}

// GetByteBlock returns a recycled block when available, otherwise allocates
// a fresh blockSize-byte block and accounts for it in bytesUsed.
func (r *RecyclingByteBlockAllocator) GetByteBlock() []byte {
	if r.freeBlocks == 0 {
		r.bytesUsed.AddAndGet(int64(r.blockSize))
		return make([]byte, r.blockSize)
	}
	r.freeBlocks--
	b := r.freeByteBlocks[r.freeBlocks]
	r.freeByteBlocks[r.freeBlocks] = nil
	return b
}

// RecycleByteBlocks places blocks[start:end] back into the recycler up to
// maxBufferedBlocks. Overflow is dropped and accounted for.
func (r *RecyclingByteBlockAllocator) RecycleByteBlocks(blocks [][]byte, start, end int) {
	numBlocks := r.maxBufferedBlocks - r.freeBlocks
	if end-start < numBlocks {
		numBlocks = end - start
	}
	size := r.freeBlocks + numBlocks
	if size >= len(r.freeByteBlocks) {
		newCap := oversize(size, 8) // sizeof(reference)
		newBlocks := make([][]byte, newCap)
		copy(newBlocks, r.freeByteBlocks[:r.freeBlocks])
		r.freeByteBlocks = newBlocks
	}
	stop := start + numBlocks
	for i := start; i < stop; i++ {
		r.freeByteBlocks[r.freeBlocks] = blocks[i]
		r.freeBlocks++
		blocks[i] = nil
	}
	for i := stop; i < end; i++ {
		blocks[i] = nil
	}
	dropped := int64(end-stop) * int64(r.blockSize)
	r.bytesUsed.AddAndGet(-dropped)
}

// NumBufferedBlocks returns the current number of recycled blocks.
func (r *RecyclingByteBlockAllocator) NumBufferedBlocks() int { return r.freeBlocks }

// BytesUsed returns the running total of bytes allocated.
func (r *RecyclingByteBlockAllocator) BytesUsed() int64 { return r.bytesUsed.Get() }

// MaxBufferedBlocks returns the configured upper bound on recycled blocks.
func (r *RecyclingByteBlockAllocator) MaxBufferedBlocks() int { return r.maxBufferedBlocks }

// BlockSize returns the per-block byte count this allocator hands out.
func (r *RecyclingByteBlockAllocator) BlockSize() int { return r.blockSize }

// FreeBlocks drops up to num recycled blocks. Returns the count actually
// freed. Each freed block subtracts blockSize bytes from the running total.
func (r *RecyclingByteBlockAllocator) FreeBlocks(num int) int {
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
		r.freeByteBlocks[r.freeBlocks] = nil
	}
	r.bytesUsed.AddAndGet(-int64(count) * int64(r.blockSize))
	return count
}
