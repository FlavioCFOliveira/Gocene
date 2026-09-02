// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"encoding/binary"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ByteSlicePool writes interleaved byte streams into shared fixed-size byte
// arrays. The idea is to allocate slices of increasing lengths. For example,
// the first slice is 5 bytes, the next slice is 14, etc. We start by writing
// our bytes into the first 5 bytes. When we hit the end of the slice, we
// allocate the next slice and then write the address of the new slice into
// the last 4 bytes of the previous slice (the "forwarding address").
//
// Each slice is filled with 0's initially, and we mark the end with a
// non-zero byte. This way the methods that are writing into the slice don't
// need to record its length and instead allocate a new slice once they hit a
// non-zero byte.
//
// Port of Lucene's org.apache.lucene.index.ByteSlicePool.
type ByteSlicePool struct {
	// Pool is the underlying ByteBlockPool. Slices are overlaid on top of its
	// fixed-size blocks. Each slice is contiguous in memory, i.e. it does not
	// straddle multiple blocks.
	Pool *util.ByteBlockPool
}

// ByteSliceLevelSizeArray holds the level sizes for byte slices. The first
// slice is 5 bytes, the second is 14, and so on.
var ByteSliceLevelSizeArray = [...]int{5, 14, 20, 30, 40, 40, 80, 80, 120, 200}

// ByteSliceNextLevelArray holds indexes into ByteSliceLevelSizeArray, to
// quickly navigate to the next slice level. These are encoded on 4 bits in
// the slice, so the values in this array must be less than 16.
//
// ByteSliceNextLevelArray[x] == x + 1, except for the last element, where
// ByteSliceNextLevelArray[x] == x, pointing at the maximum slice size.
var ByteSliceNextLevelArray = [...]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 9}

// ByteSliceFirstLevelSize is the first level size for new slices.
const ByteSliceFirstLevelSize = 5

// NewByteSlicePool creates a new ByteSlicePool backed by the given pool.
func NewByteSlicePool(pool *util.ByteBlockPool) *ByteSlicePool {
	return &ByteSlicePool{Pool: pool}
}

// NewSlice allocates a new slice with the given size and level 0. Returns the
// position where the slice starts within the current buffer.
func (p *ByteSlicePool) NewSlice(size int) (int, error) {
	if size > util.ByteBlockSize {
		return 0, fmt.Errorf("ByteSlicePool: slice size %d should be less than the block size %d", size, util.ByteBlockSize)
	}

	if p.Pool.ByteUpto > util.ByteBlockSize-size {
		p.Pool.NextBuffer()
	}
	upto := p.Pool.ByteUpto
	p.Pool.ByteUpto += size
	p.Pool.Buffer[p.Pool.ByteUpto-1] = 16 // codifies level 0
	return upto, nil
}

// AllocSlice creates a new byte slice in continuation of the provided slice
// and returns its offset into the pool.
//
// upto must point to the last byte of the current slice.
func (p *ByteSlicePool) AllocSlice(slice []byte, upto int) int {
	return p.AllocKnownSizeSlice(slice, upto) >> 8
}

// AllocKnownSizeSlice creates a new byte slice in continuation of the
// provided slice and returns its length packed with its offset into the pool.
//
// The lower 8 bits hold the new slice's length; the upper 24 bits hold the
// offset into the pool.
//
// upto must point to the last byte of the current slice.
func (p *ByteSlicePool) AllocKnownSizeSlice(slice []byte, upto int) int {
	level := int(slice[upto] & 15) // last 4 bits codify the level
	newLevel := ByteSliceNextLevelArray[level]
	newSize := ByteSliceLevelSizeArray[newLevel]

	// Maybe allocate another block.
	if p.Pool.ByteUpto > util.ByteBlockSize-newSize {
		p.Pool.NextBuffer()
	}

	newUpto := p.Pool.ByteUpto
	offset := newUpto + p.Pool.ByteOffset
	p.Pool.ByteUpto += newSize

	// Copy forward the past 3 bytes (which we are about to overwrite with the
	// forwarding address). We actually move 4 bytes at once; the high byte is
	// expected to be 0 (the next newSize bytes were freshly allocated).
	past3Bytes := binary.LittleEndian.Uint32(slice[upto-3:upto+1]) & 0xFFFFFF
	if p.Pool.Buffer[newUpto+3] != 0 {
		panic(fmt.Sprintf("ByteSlicePool: expected buffer[%d] == 0, got %d", newUpto+3, p.Pool.Buffer[newUpto+3]))
	}
	binary.LittleEndian.PutUint32(p.Pool.Buffer[newUpto:newUpto+4], past3Bytes)

	// Write forwarding address at end of last slice.
	binary.LittleEndian.PutUint32(slice[upto-3:upto+1], uint32(offset))

	// Write new level.
	p.Pool.Buffer[p.Pool.ByteUpto-1] = byte(16 | newLevel)

	return ((newUpto + 3) << 8) | (newSize - 3)
}
