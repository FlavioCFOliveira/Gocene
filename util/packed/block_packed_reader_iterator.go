// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockPackedReaderIterator reads the wire format produced by
// BlockPackedWriter. Use Next or Skip to consume the stream.
//
// This is the Go port of org.apache.lucene.util.packed.BlockPackedReaderIterator
// in Apache Lucene 10.4.0.
type BlockPackedReaderIterator struct {
	in                store.DataInput
	packedIntsVersion int
	valueCount        int64
	blockSize         int
	values            []int64
	blocks            []byte
	off               int
	ord               int64
}

// NewBlockPackedReaderIterator returns an iterator over a stream of
// valueCount longs encoded with BlockPackedWriter at blockSize.
func NewBlockPackedReaderIterator(in store.DataInput, packedIntsVersion, blockSize int, valueCount int64) (*BlockPackedReaderIterator, error) {
	if _, err := CheckBlockSize(blockSize, BlockPackedMinBlockSize, BlockPackedMaxBlockSize); err != nil {
		return nil, err
	}
	r := &BlockPackedReaderIterator{
		packedIntsVersion: packedIntsVersion,
		blockSize:         blockSize,
		values:            make([]int64, blockSize),
	}
	r.Reset(in, valueCount)
	return r, nil
}

// Reset rewires the iterator to a new DataInput / valueCount pair.
func (r *BlockPackedReaderIterator) Reset(in store.DataInput, valueCount int64) {
	r.in = in
	r.valueCount = valueCount
	r.off = r.blockSize
	r.ord = 0
}

// Ord returns the offset of the next value to read.
func (r *BlockPackedReaderIterator) Ord() int64 { return r.ord }

// Next reads the next value from the stream.
func (r *BlockPackedReaderIterator) Next() (int64, error) {
	if r.ord == r.valueCount {
		return 0, io.EOF
	}
	if r.off == r.blockSize {
		if err := r.refill(); err != nil {
			return 0, err
		}
	}
	v := r.values[r.off]
	r.off++
	r.ord++
	return v, nil
}

func (r *BlockPackedReaderIterator) refill() error {
	tokenByte, err := r.in.ReadByte()
	if err != nil {
		return err
	}
	token := int(tokenByte) & 0xFF
	minEqualsZero := (token & bpwMinValueEqualsZero) != 0
	bitsPerValue := token >> bpwBpvShift
	if bitsPerValue > 64 {
		return fmt.Errorf("packed: corrupted block (bpv=%d > 64)", bitsPerValue)
	}
	var minValue int64
	if !minEqualsZero {
		v, err := blockPackedReadVLong(r.in)
		if err != nil {
			return err
		}
		minValue = util.ZigZagDecodeInt64(1 + v)
	}
	if bitsPerValue == 0 {
		for i := range r.values {
			r.values[i] = minValue
		}
		r.off = 0
		return nil
	}
	dec, err := GetDecoder(FormatPacked, r.packedIntsVersion, bitsPerValue)
	if err != nil {
		return err
	}
	iterations := r.blockSize / dec.ByteValueCount()
	blocksSize := iterations * dec.ByteBlockCount()
	if cap(r.blocks) < blocksSize {
		r.blocks = make([]byte, blocksSize)
	} else {
		r.blocks = r.blocks[:blocksSize]
	}
	remaining := r.valueCount - r.ord
	blockValueCount := int64(r.blockSize)
	if remaining < blockValueCount {
		blockValueCount = remaining
	}
	blockBytes := FormatPacked.ByteCount(r.packedIntsVersion, int(blockValueCount), bitsPerValue)
	if err := r.in.ReadBytes(r.blocks[:blockBytes]); err != nil {
		return err
	}
	dec.DecodeBytes(r.blocks, 0, r.values, 0, iterations)
	if minValue != 0 {
		for i := int64(0); i < blockValueCount; i++ {
			r.values[i] += minValue
		}
	}
	r.off = 0
	return nil
}

// blockPackedReadVLong is BlockPacked's readVLong: like
// DataInput.readVLong but tolerant of values that fill the 9 bytes
// (e.g. a negative encoded as a 9-byte VLong).
func blockPackedReadVLong(in store.DataInput) (int64, error) {
	var l int64
	for shift := uint(0); shift < 56; shift += 7 {
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		l |= int64(b&0x7F) << shift
		if int8(b) >= 0 {
			return l, nil
		}
	}
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	l |= int64(b) << 56
	return l, nil
}

// Skip skips exactly count values.
func (r *BlockPackedReaderIterator) Skip(count int64) error {
	if count < 0 || r.ord+count > r.valueCount {
		return io.EOF
	}
	// 1. consume buffered values
	remaining := int64(r.blockSize - r.off)
	skipBuffer := count
	if skipBuffer > remaining {
		skipBuffer = remaining
	}
	r.off += int(skipBuffer)
	r.ord += skipBuffer
	count -= skipBuffer
	if count == 0 {
		return nil
	}
	// 2. skip whole blocks header-by-header
	for count >= int64(r.blockSize) {
		tokenByte, err := r.in.ReadByte()
		if err != nil {
			return err
		}
		token := int(tokenByte) & 0xFF
		bpv := token >> bpwBpvShift
		if bpv > 64 {
			return fmt.Errorf("packed: corrupted block (bpv=%d > 64)", bpv)
		}
		if (token & bpwMinValueEqualsZero) == 0 {
			if _, err := blockPackedReadVLong(r.in); err != nil {
				return err
			}
		}
		blockBytes := FormatPacked.ByteCount(r.packedIntsVersion, r.blockSize, bpv)
		if err := r.skipBytes(blockBytes); err != nil {
			return err
		}
		r.ord += int64(r.blockSize)
		count -= int64(r.blockSize)
	}
	if count == 0 {
		return nil
	}
	// 3. consume the remainder via a regular refill
	if err := r.refill(); err != nil {
		return err
	}
	r.ord += count
	r.off += int(count)
	return nil
}

func (r *BlockPackedReaderIterator) skipBytes(n int64) error {
	if cap(r.blocks) < int(n) {
		r.blocks = make([]byte, n)
	} else {
		r.blocks = r.blocks[:n]
	}
	return r.in.ReadBytes(r.blocks)
}
