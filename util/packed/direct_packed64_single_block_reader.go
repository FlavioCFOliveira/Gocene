// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// DirectPacked64SingleBlockReader provides random access over an
// on-disk PackedInts stream encoded with FormatPackedSingleBlock, by
// seeking and reading the relevant 64-bit block on every Get. It
// does not buffer values, so it trades CPU and I/O cost for zero
// heap residency.
//
// This is the Go port of
// org.apache.lucene.util.packed.DirectPacked64SingleBlockReader in
// Apache Lucene 10.4.0.
type DirectPacked64SingleBlockReader struct {
	readerImpl
	in             store.IndexInput
	bitsPerValue   int
	startPointer   int64
	valuesPerBlock int
	mask           int64
}

// NewDirectPacked64SingleBlockReader builds a reader over valueCount
// values of bitsPerValue width, starting at the current file pointer
// of in. The reader retains in and seeks within it on every Get.
func NewDirectPacked64SingleBlockReader(bitsPerValue, valueCount int, in store.IndexInput) *DirectPacked64SingleBlockReader {
	return &DirectPacked64SingleBlockReader{
		readerImpl:     readerImpl{valueCount: valueCount},
		in:             in,
		bitsPerValue:   bitsPerValue,
		startPointer:   in.GetFilePointer(),
		valuesPerBlock: 64 / bitsPerValue,
		mask:           ^(^int64(0) << uint(bitsPerValue)),
	}
}

// Get returns the value at the given index. The underlying input is
// seeked and a single 64-bit block is read on every call. Any I/O
// failure surfaces as a panic, matching the reference behavior of
// the Java original (IllegalStateException("failed", e)).
func (r *DirectPacked64SingleBlockReader) Get(index int) int64 {
	blockOffset := index / r.valuesPerBlock
	skip := int64(blockOffset) << 3
	if err := r.in.SetPosition(r.startPointer + skip); err != nil {
		panic(err)
	}
	block, err := r.in.ReadLong()
	if err != nil {
		panic(err)
	}
	offsetInBlock := index % r.valuesPerBlock
	return int64(uint64(block)>>uint(offsetInBlock*r.bitsPerValue)) & r.mask
}

// GetBulk fills arr[off:off+length] with up to length values starting
// at index using the generic sequential bulk-get.
func (r *DirectPacked64SingleBlockReader) GetBulk(index int, arr []int64, off, length int) int {
	return readerBulkGet(r, index, arr, off, length)
}

// RamBytesUsed mirrors the Java reference which reports zero heap
// usage because the reader holds no decoded buffer.
func (r *DirectPacked64SingleBlockReader) RamBytesUsed() int64 { return 0 }
