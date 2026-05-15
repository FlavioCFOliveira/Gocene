// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// monotonicReaderDataInput is the input surface MonotonicBlockPackedReader
// requires: a DataInput plus the VInt/VLong helpers from VariableLengthInput.
type monotonicReaderDataInput interface {
	store.DataInput
	ReadVInt() (int32, error)
	ReadVLong() (int64, error)
}

// MonotonicBlockPackedReader provides random access to a stream
// written with MonotonicBlockPackedWriter. The reader loads every
// block's packed bytes into a heap-resident byte slice; random
// access is implemented inline (no DirectReader hop).
//
// This is the Go port of org.apache.lucene.util.packed.MonotonicBlockPackedReader
// in Apache Lucene 10.4.0.
type MonotonicBlockPackedReader struct {
	blockShift int
	blockMask  int
	valueCount int64
	minValues  []int64
	averages   []float32
	subReaders []func(index int64) int64
	totalBytes int64
}

// NewMonotonicBlockPackedReader reads valueCount values from in
// (assuming blockSize-sized blocks at the given PackedInts version).
func NewMonotonicBlockPackedReader(in monotonicReaderDataInput, packedIntsVersion, blockSize int, valueCount int64) (*MonotonicBlockPackedReader, error) {
	blockShift, err := CheckBlockSize(blockSize, BlockPackedMinBlockSize, BlockPackedMaxBlockSize)
	if err != nil {
		return nil, err
	}
	numBlocks, err := NumBlocks(valueCount, blockSize)
	if err != nil {
		return nil, err
	}
	r := &MonotonicBlockPackedReader{
		blockShift: blockShift,
		blockMask:  blockSize - 1,
		valueCount: valueCount,
		minValues:  make([]int64, numBlocks),
		averages:   make([]float32, numBlocks),
		subReaders: make([]func(int64) int64, numBlocks),
	}
	for i := 0; i < numBlocks; i++ {
		z, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		r.minValues[i] = util.ZigZagDecodeInt64(z)

		avgInt, err := in.ReadInt()
		if err != nil {
			return nil, err
		}
		r.averages[i] = math.Float32frombits(uint32(avgInt))

		bpv32, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		bitsPerValue := int(bpv32)
		if bitsPerValue > 64 {
			return nil, fmt.Errorf("packed: corrupted block (bpv=%d > 64)", bitsPerValue)
		}
		if bitsPerValue == 0 {
			r.subReaders[i] = zeroSubReader
			continue
		}
		size := int64(blockSize)
		if rem := valueCount - int64(i)*int64(blockSize); rem < size {
			size = rem
		}
		byteCount := FormatPacked.ByteCount(packedIntsVersion, int(size), bitsPerValue)
		r.totalBytes += byteCount
		buf := make([]byte, byteCount)
		if err := in.ReadBytes(buf); err != nil {
			return nil, err
		}
		bpv := bitsPerValue
		r.subReaders[i] = makeMonotonicSubReader(buf, bpv)
	}
	return r, nil
}

func zeroSubReader(_ int64) int64 { return 0 }

// makeMonotonicSubReader returns a closure that mirrors Lucene's inline
// LongValues#get for a fixed-width packed block. The decoding logic
// matches Lucene's MonotonicBlockPackedReader subReaders[].get exactly.
func makeMonotonicSubReader(blocks []byte, bitsPerValue int) func(int64) int64 {
	const blockSizeBits = 8
	const blockBitsShift = 3
	const modMask = blockSizeBits - 1
	maskRight := (int64(1) << uint(bitsPerValue)) - 1
	bpvMinusBlockSize := int64(bitsPerValue) - blockSizeBits
	return func(index int64) int64 {
		majorBitPos := index * int64(bitsPerValue)
		blockOffset := int(uint64(majorBitPos) >> blockBitsShift)
		endBits := (majorBitPos & modMask) + bpvMinusBlockSize
		if endBits <= 0 {
			// single block
			return (int64(uint64(blocks[blockOffset])>>uint(-endBits)) & maskRight)
		}
		// multiple blocks
		value := (int64(uint64(blocks[blockOffset])<<uint(endBits)) & maskRight)
		blockOffset++
		for endBits > blockSizeBits {
			endBits -= blockSizeBits
			value |= int64(uint64(blocks[blockOffset]) << uint(endBits))
			blockOffset++
		}
		return value | (int64(uint64(blocks[blockOffset]) >> uint(blockSizeBits-endBits)))
	}
}

// Get returns the value at index.
func (r *MonotonicBlockPackedReader) Get(index int64) int64 {
	if index < 0 || index >= r.valueCount {
		panic(fmt.Sprintf("packed: index=%d valueCount=%d", index, r.valueCount))
	}
	block := int(uint64(index) >> uint(r.blockShift))
	idx := int64(int(index) & r.blockMask)
	return monotonicExpected(r.minValues[block], r.averages[block], int(idx)) + r.subReaders[block](idx)
}

// Size returns the total number of values.
func (r *MonotonicBlockPackedReader) Size() int64 { return r.valueCount }

// RamBytesUsed approximates the heap footprint of the reader (the
// per-block packed buffers dominate).
func (r *MonotonicBlockPackedReader) RamBytesUsed() int64 {
	return int64(8*len(r.minValues)) + int64(4*len(r.averages)) + r.totalBytes + 64
}
