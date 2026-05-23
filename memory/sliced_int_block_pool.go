// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of the static inner class org.apache.lucene.index.memory.MemoryIndex.SlicedIntBlockPool
// together with its SliceWriter and SliceReader inner classes.

package memory

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SlicedIntBlockPool is an IntBlockPool variant that supports writing and reading
// arbitrary-length int slices stored inside the pool's flat buffer space.
// Slices grow through a level-based scheme: when the current slice is full the
// last slot is overwritten with a forwarding address that points to the next,
// larger slice. The level sizes are fixed by levelSizeArray.
//
// This is a port of MemoryIndex.SlicedIntBlockPool in Apache Lucene 10.4.0.
type SlicedIntBlockPool struct {
	*util.IntBlockPool
}

// nextLevelArray maps the current slice level to the next level index.
var nextLevelArray = [10]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 9}

// levelSizeArray holds the number of ints in a slice at each level.
var levelSizeArray = [10]int{2, 4, 8, 16, 16, 32, 32, 64, 64, 128}

// firstLevelSize is the initial slice size.
const firstLevelSize = 2 // levelSizeArray[0]

// NewSlicedIntBlockPool creates a SlicedIntBlockPool backed by the given allocator.
func NewSlicedIntBlockPool(allocator util.IntAllocator) *SlicedIntBlockPool {
	return &SlicedIntBlockPool{
		IntBlockPool: util.NewIntBlockPoolWithAllocator(allocator),
	}
}

// newSlice allocates a fresh slice of the given size inside the pool and returns
// its absolute (global) start offset.
func (p *SlicedIntBlockPool) newSlice(size int) int {
	if p.IntUpto > util.IntBlockSize-size {
		p.NextBuffer()
	}
	upto := p.IntUpto
	p.IntUpto += size
	p.Buffer[p.IntUpto-1] = 16 // sentinel: level 0 end marker
	return upto
}

// allocSlice links the slice ending at sliceOffset (in the given buffer) to a
// new, larger slice. It returns the relative offset inside the new head buffer.
func (p *SlicedIntBlockPool) allocSlice(slice []int32, sliceOffset int) int {
	level := int(slice[sliceOffset]) & 15
	newLevel := nextLevelArray[level]
	newSize := levelSizeArray[newLevel]

	if p.IntUpto > util.IntBlockSize-newSize {
		p.NextBuffer()
	}

	newUpto := p.IntUpto
	offset := newUpto + p.IntOffset
	p.IntUpto += newSize

	// Overwrite the last slot of the old slice with a forwarding address.
	slice[sliceOffset] = int32(offset)

	// Write the new level marker at the end of the new slice.
	p.Buffer[p.IntUpto-1] = int32(16 | newLevel)

	return newUpto
}

// ---------------------------------------------------------------------------
// SliceWriter
// ---------------------------------------------------------------------------

// SliceWriter writes int values into a sequence of chained slices inside a
// SlicedIntBlockPool. A single SliceWriter instance can be reused across
// multiple non-overlapping slices.
//
// Port of MemoryIndex.SlicedIntBlockPool.SliceWriter.
type SliceWriter struct {
	offset int
	pool   *SlicedIntBlockPool
}

// NewSliceWriter creates a SliceWriter that writes into pool.
func NewSliceWriter(pool *SlicedIntBlockPool) *SliceWriter {
	return &SliceWriter{pool: pool}
}

// Reset repositions the writer to the given absolute slice offset, allowing
// it to continue writing an already-started slice.
func (w *SliceWriter) Reset(sliceOffset int) {
	w.offset = sliceOffset
}

// WriteInt appends value to the current slice, allocating a continuation slice
// if the current one is full.
func (w *SliceWriter) WriteInt(value int32) {
	bufIdx := w.offset >> util.IntBlockShift
	ints := w.pool.Buffers[bufIdx]
	relOff := w.offset & util.IntBlockMask
	if ints[relOff] != 0 {
		// Current slot is the end-of-slice sentinel; chain to a new slice.
		relOff = w.pool.allocSlice(ints, relOff)
		ints = w.pool.Buffer
		w.offset = relOff + w.pool.IntOffset
	}
	ints[relOff] = value
	w.offset++
}

// StartNewSlice allocates a fresh first-level slice, positions the writer at
// its start, and returns the absolute start offset. Pass the returned value
// as startOffset to SliceReader.Reset.
func (w *SliceWriter) StartNewSlice() int {
	w.offset = w.pool.newSlice(firstLevelSize) + w.pool.IntOffset
	return w.offset
}

// CurrentOffset returns the absolute offset of the next slot to be written.
// Pass this value as endOffset to SliceReader.Reset once the slice is complete,
// or to SliceWriter.Reset to resume writing.
func (w *SliceWriter) CurrentOffset() int {
	return w.offset
}

// ---------------------------------------------------------------------------
// SliceReader
// ---------------------------------------------------------------------------

// SliceReader reads int values from a chain of slices created by SliceWriter.
//
// Port of MemoryIndex.SlicedIntBlockPool.SliceReader.
type SliceReader struct {
	pool         *SlicedIntBlockPool
	upto         int
	bufferUpto   int
	bufferOffset int
	buffer       []int32
	limit        int
	level        int
	end          int
}

// NewSliceReader creates a SliceReader that reads from pool.
func NewSliceReader(pool *SlicedIntBlockPool) *SliceReader {
	return &SliceReader{pool: pool}
}

// Reset positions the reader to read the slice delimited by [startOffset, endOffset).
// startOffset is the value returned by SliceWriter.StartNewSlice;
// endOffset is the value returned by SliceWriter.CurrentOffset.
func (r *SliceReader) Reset(startOffset, endOffset int) {
	r.bufferUpto = startOffset / util.IntBlockSize
	r.bufferOffset = r.bufferUpto * util.IntBlockSize
	r.end = endOffset
	r.level = 0

	r.buffer = r.pool.Buffers[r.bufferUpto]
	r.upto = startOffset & util.IntBlockMask

	firstSize := levelSizeArray[0]
	if startOffset+firstSize >= endOffset {
		// Only one slice to read.
		r.limit = endOffset & util.IntBlockMask
	} else {
		r.limit = r.upto + firstSize - 1
	}
}

// EndOfSlice reports whether all values in the slice have been consumed.
// ReadInt must not be called once EndOfSlice returns true.
func (r *SliceReader) EndOfSlice() bool {
	return r.upto+r.bufferOffset == r.end
}

// ReadInt reads and returns the next int in the slice.
// Panics if called after EndOfSlice returns true.
func (r *SliceReader) ReadInt() int32 {
	if r.upto == r.limit {
		r.nextSlice()
	}
	v := r.buffer[r.upto]
	r.upto++
	return v
}

// nextSlice follows the forwarding address at the current slice's limit to the
// next slice in the chain.
func (r *SliceReader) nextSlice() {
	nextIndex := int(r.buffer[r.limit])
	r.level = nextLevelArray[r.level]
	newSize := levelSizeArray[r.level]

	r.bufferUpto = nextIndex / util.IntBlockSize
	r.bufferOffset = r.bufferUpto * util.IntBlockSize

	r.buffer = r.pool.Buffers[r.bufferUpto]
	r.upto = nextIndex & util.IntBlockMask

	if nextIndex+newSize >= r.end {
		// Final slice.
		r.limit = r.end - r.bufferOffset
	} else {
		r.limit = r.upto + newSize - 1
	}
}
