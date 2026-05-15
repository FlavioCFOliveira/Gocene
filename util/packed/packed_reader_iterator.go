// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// packedReaderIterator implements ReaderIterator over the wire
// format produced by PackedWriter.
type packedReaderIterator struct {
	in                store.DataInput
	packedIntsVersion int
	format            Format
	bulkOperation     BulkOperation
	nextBlocks        []byte
	nextValues        *util.LongsRef
	iterations        int
	valueCount        int
	bitsPerValue      int
	position          int
}

func newPackedReaderIterator(format Format, packedIntsVersion, valueCount, bitsPerValue int, in store.DataInput, mem int) (*packedReaderIterator, error) {
	op, err := BulkOperationOf(format, bitsPerValue)
	if err != nil {
		return nil, err
	}
	iterations := op.ComputeIterations(valueCount, mem)
	nextValues := util.NewLongsRefFromSlice(make([]int64, iterations*op.ByteValueCount()), 0, 0)
	nextValues.Offset = len(nextValues.Longs)
	return &packedReaderIterator{
		in:                in,
		packedIntsVersion: packedIntsVersion,
		format:            format,
		bulkOperation:     op,
		nextBlocks:        make([]byte, iterations*op.ByteBlockCount()),
		nextValues:        nextValues,
		iterations:        iterations,
		valueCount:        valueCount,
		bitsPerValue:      bitsPerValue,
		position:          -1,
	}, nil
}

// GetBitsPerValue returns the bits-per-value of the stream.
func (it *packedReaderIterator) GetBitsPerValue() int { return it.bitsPerValue }

// Size returns the number of values held in the stream.
func (it *packedReaderIterator) Size() int { return it.valueCount }

// Ord returns the index of the most recently returned value.
func (it *packedReaderIterator) Ord() int { return it.position }

// Next reads the next value from the stream.
func (it *packedReaderIterator) Next() (int64, error) {
	nextValues, err := it.NextN(1)
	if err != nil {
		return 0, err
	}
	v := nextValues.Longs[nextValues.Offset]
	nextValues.Offset++
	nextValues.Length--
	return v, nil
}

// NextN reads at least one and up to count values, returning a
// LongsRef view into the iterator's internal buffer. The caller MUST
// NOT modify the returned slice.
func (it *packedReaderIterator) NextN(count int) (*util.LongsRef, error) {
	if count <= 0 {
		return nil, io.ErrUnexpectedEOF
	}
	it.nextValues.Offset += it.nextValues.Length

	remaining := it.valueCount - it.position - 1
	if remaining <= 0 {
		return nil, io.EOF
	}
	if remaining < count {
		count = remaining
	}

	if it.nextValues.Offset == len(it.nextValues.Longs) {
		remainingBlocks := it.format.ByteCount(it.packedIntsVersion, remaining, it.bitsPerValue)
		blocksToRead := int(remainingBlocks)
		if blocksToRead > len(it.nextBlocks) {
			blocksToRead = len(it.nextBlocks)
		}
		if blocksToRead > 0 {
			if err := it.in.ReadBytes(it.nextBlocks[:blocksToRead]); err != nil {
				return nil, err
			}
		}
		if blocksToRead < len(it.nextBlocks) {
			for i := blocksToRead; i < len(it.nextBlocks); i++ {
				it.nextBlocks[i] = 0
			}
		}
		it.bulkOperation.DecodeBytes(it.nextBlocks, 0, it.nextValues.Longs, 0, it.iterations)
		it.nextValues.Offset = 0
	}

	available := len(it.nextValues.Longs) - it.nextValues.Offset
	if available < count {
		count = available
	}
	it.nextValues.Length = count
	it.position += it.nextValues.Length
	return it.nextValues, nil
}
