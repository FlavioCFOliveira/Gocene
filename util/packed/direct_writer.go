// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// DirectWriterSupportedBitsPerValue is the ordered list of supported
// bitsPerValue values for DirectWriter / DirectReader.
//
// The companion DirectReader assumes this exact set; it is the wire
// contract.
var DirectWriterSupportedBitsPerValue = [...]int{1, 2, 4, 8, 12, 16, 20, 24, 28, 32, 40, 48, 56, 64}

// directWriterPaddingMax is the largest number of padding bytes
// appended after a DirectWriter stream so that values can be read
// with a single aligned load.
const directWriterPaddingMax = 3

// DirectWriter writes long values with a fixed bitsPerValue to a
// DataOutput so that DirectReader can read them via random access.
//
// Unlike PackedInts, the byte layout is little-endian and reads can
// be performed in O(1) without buffering — at the cost of a small
// amount of trailing padding so that the last value can still be
// loaded with a single int/long read.
type DirectWriter struct {
	out          store.DataOutput
	numValues    int64
	bitsPerValue int
	count        int64
	finished     bool

	off        int
	nextBlocks []byte
	nextValues []int64
}

// GetDirectWriter returns a DirectWriter for the given (numValues,
// bitsPerValue). bitsPerValue must be a member of
// DirectWriterSupportedBitsPerValue.
func GetDirectWriter(out store.DataOutput, numValues int64, bitsPerValue int) (*DirectWriter, error) {
	if err := checkDirectBitsPerValue(bitsPerValue); err != nil {
		return nil, err
	}

	const memoryBudgetInBits = 8 * DefaultBufferSize
	bufferSize := memoryBudgetInBits / (64 + bitsPerValue)
	// Round up to next multiple of 64
	bufferSize = (bufferSize + 63) & 0xFFFFFFC0
	if bufferSize == 0 {
		bufferSize = 64
	}

	return &DirectWriter{
		out:          out,
		numValues:    numValues,
		bitsPerValue: bitsPerValue,
		nextValues:   make([]int64, bufferSize),
		nextBlocks:   make([]byte, bufferSize*bitsPerValue/8+8-1),
	}, nil
}

// Add appends a value to the stream.
func (w *DirectWriter) Add(v int64) error {
	if w.bitsPerValue != 64 {
		if v < 0 || uint64(v) > uint64(MaxValue(w.bitsPerValue)) {
			return fmt.Errorf("packed: value %d does not fit in %d bits", v, w.bitsPerValue)
		}
	}
	if w.finished {
		return errors.New("packed: writer is finished")
	}
	if w.count >= w.numValues {
		return io.ErrShortWrite
	}
	w.nextValues[w.off] = v
	w.off++
	if w.off == len(w.nextValues) {
		if err := w.flush(); err != nil {
			return err
		}
	}
	w.count++
	return nil
}

// Finish flushes pending values and writes the trailing padding
// bytes required so reads can fetch each value with a single
// aligned load.
func (w *DirectWriter) Finish() error {
	if w.count != w.numValues {
		return fmt.Errorf("packed: wrong number of values added, expected: %d, got: %d", w.numValues, w.count)
	}
	if w.finished {
		return errors.New("packed: writer already finished")
	}
	if err := w.flush(); err != nil {
		return err
	}
	padding := directPaddingBytes(w.bitsPerValue)
	for i := 0; i < padding; i++ {
		if err := w.out.WriteByte(0); err != nil {
			return err
		}
	}
	w.finished = true
	return nil
}

func (w *DirectWriter) flush() error {
	if w.off == 0 {
		return nil
	}
	for i := w.off; i < len(w.nextValues); i++ {
		w.nextValues[i] = 0
	}
	directEncode(w.nextValues, w.off, w.nextBlocks, w.bitsPerValue)
	blockCount := int(FormatPacked.ByteCount(VersionCurrent, w.off, w.bitsPerValue))
	if err := w.out.WriteBytes(w.nextBlocks[:blockCount]); err != nil {
		return err
	}
	w.off = 0
	return nil
}

// directEncode packs upTo values from nextValues into nextBlocks
// using the DirectWriter byte layout (little-endian).
func directEncode(nextValues []int64, upTo int, nextBlocks []byte, bitsPerValue int) {
	switch {
	case bitsPerValue&7 == 0:
		bytesPerValue := bitsPerValue / 8
		for i, o := 0, 0; i < upTo; i, o = i+1, o+bytesPerValue {
			l := uint64(nextValues[i])
			switch {
			case bitsPerValue > 32:
				binary.LittleEndian.PutUint64(nextBlocks[o:], l)
			case bitsPerValue > 16:
				binary.LittleEndian.PutUint32(nextBlocks[o:], uint32(l))
			case bitsPerValue > 8:
				binary.LittleEndian.PutUint16(nextBlocks[o:], uint16(l))
			default:
				nextBlocks[o] = byte(l)
			}
		}
	case bitsPerValue < 8:
		valuesPerLong := 64 / bitsPerValue
		for i, o := 0, 0; i < upTo; i, o = i+valuesPerLong, o+8 {
			var v uint64
			for j := 0; j < valuesPerLong; j++ {
				v |= uint64(nextValues[i+j]) << uint(bitsPerValue*j)
			}
			binary.LittleEndian.PutUint64(nextBlocks[o:], v)
		}
	default:
		// bitsPerValue is 12, 20 or 28: pack 2 values into a wider int
		numBytesFor2Values := bitsPerValue * 2 / 8
		for i, o := 0, 0; i < upTo; i, o = i+2, o+numBytesFor2Values {
			l1 := uint64(nextValues[i])
			l2 := uint64(nextValues[i+1])
			merged := l1 | (l2 << uint(bitsPerValue))
			if bitsPerValue <= 16 {
				binary.LittleEndian.PutUint32(nextBlocks[o:], uint32(merged))
			} else {
				binary.LittleEndian.PutUint64(nextBlocks[o:], merged)
			}
		}
	}
}

// directPaddingBytes returns the number of zero bytes that follow a
// DirectWriter stream so that the last value can be loaded with a
// single int/long read.
func directPaddingBytes(bitsPerValue int) int {
	var paddingBits int
	switch {
	case bitsPerValue > 32:
		paddingBits = 64 - bitsPerValue
	case bitsPerValue > 16:
		paddingBits = 32 - bitsPerValue
	case bitsPerValue > 8:
		paddingBits = 16 - bitsPerValue
	default:
		paddingBits = 0
	}
	return (paddingBits + 7) / 8
}

// DirectWriterBytesRequired returns the number of bytes that
// encoding numValues values with bitsPerValue will produce, padding
// included.
func DirectWriterBytesRequired(numValues int64, bitsPerValue int) (int64, error) {
	if err := checkDirectBitsPerValue(bitsPerValue); err != nil {
		return 0, err
	}
	bytes := (numValues*int64(bitsPerValue) + 7) / 8
	return bytes + int64(directPaddingBytes(bitsPerValue)), nil
}

// DirectWriterBitsRequired rounds bitsRequired up to the next
// bitsPerValue supported by DirectWriter.
func DirectWriterBitsRequired(maxValue int64) int {
	return directWriterRoundBits(BitsRequired(maxValue))
}

// DirectWriterUnsignedBitsRequired rounds the unsigned bit width up
// to the next bitsPerValue supported by DirectWriter.
func DirectWriterUnsignedBitsRequired(maxValue uint64) int {
	return directWriterRoundBits(UnsignedBitsRequired(maxValue))
}

func directWriterRoundBits(bitsRequired int) int {
	supported := DirectWriterSupportedBitsPerValue[:]
	idx := sort.SearchInts(supported, bitsRequired)
	if idx == len(supported) {
		return supported[len(supported)-1]
	}
	return supported[idx]
}

func checkDirectBitsPerValue(bitsPerValue int) error {
	for _, b := range DirectWriterSupportedBitsPerValue {
		if b == bitsPerValue {
			return nil
		}
	}
	return fmt.Errorf("packed: unsupported bitsPerValue %d; did you use DirectWriterBitsRequired?", bitsPerValue)
}
