// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package packed is the Go port of org.apache.lucene.util.packed.
//
// It provides byte-compatible packed integer encodings, monotonic
// readers/writers, and growable mutable arrays used by codecs.
package packed

import (
	"fmt"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PackedInts overhead constants used by FastestFormatAndBits.
const (
	// Fastest selects a direct implementation (≤700% memory overhead).
	Fastest float32 = 7.0
	// Fast selects a reasonably fast implementation (≤50% overhead).
	Fast float32 = 0.5
	// Default acceptable overhead ratio.
	Default float32 = 0.25
	// Compact selects the most memory-efficient implementation (0%).
	Compact float32 = 0.0
	// DefaultBufferSize is the default amount of memory used for bulk
	// serialization operations.
	DefaultBufferSize = 1024 // 1K
)

// PackedInts codec constants.
const (
	// CodecName is the codec identifier used in serialized streams.
	CodecName = "PackedInts"
	// VersionMonotonicWithoutZigzag corresponds to Lucene's monotonic
	// encoding without zig-zag (version 2).
	VersionMonotonicWithoutZigzag = 2
	// VersionStart is the lowest supported version.
	VersionStart = VersionMonotonicWithoutZigzag
	// VersionCurrent is the version used when writing new streams.
	VersionCurrent = VersionMonotonicWithoutZigzag
)

// CheckVersion verifies that the supplied version is within the
// supported range. It returns an error for unsupported versions.
func CheckVersion(version int) error {
	if version < VersionStart {
		return fmt.Errorf("packed: version is too old, should be at least %d (got %d)", VersionStart, version)
	}
	if version > VersionCurrent {
		return fmt.Errorf("packed: version is too new, should be at most %d (got %d)", VersionCurrent, version)
	}
	return nil
}

// Format identifies the wire encoding used for packed integers.
type Format int

const (
	// FormatPacked is the compact format with contiguous bits.
	FormatPacked Format = 0
	// FormatPackedSingleBlock keeps each value within a 64-bit
	// boundary. Only a subset of bitsPerValue values are supported.
	FormatPackedSingleBlock Format = 1
)

// ID returns the numeric identifier of the format (matches Java's
// Format.getId()).
func (f Format) ID() int {
	return int(f)
}

// FormatByID returns the format with the given identifier or an
// error for unknown ids.
func FormatByID(id int) (Format, error) {
	switch Format(id) {
	case FormatPacked, FormatPackedSingleBlock:
		return Format(id), nil
	}
	return 0, fmt.Errorf("packed: unknown format id %d", id)
}

// ByteCount returns how many bytes are needed to store valueCount
// values of size bitsPerValue under this format.
func (f Format) ByteCount(packedIntsVersion, valueCount, bitsPerValue int) int64 {
	if bitsPerValue < 0 || bitsPerValue > 64 {
		panic(fmt.Sprintf("packed: bitsPerValue out of range: %d", bitsPerValue))
	}
	switch f {
	case FormatPacked:
		// ceil(valueCount * bitsPerValue / 8)
		bits := int64(valueCount) * int64(bitsPerValue)
		return (bits + 7) / 8
	default:
		// PACKED_SINGLE_BLOCK uses long-aligned blocks
		return 8 * int64(f.LongCount(packedIntsVersion, valueCount, bitsPerValue))
	}
}

// LongCount returns how many 64-bit blocks are needed to store the
// values under this format.
func (f Format) LongCount(packedIntsVersion, valueCount, bitsPerValue int) int {
	if bitsPerValue < 0 || bitsPerValue > 64 {
		panic(fmt.Sprintf("packed: bitsPerValue out of range: %d", bitsPerValue))
	}
	switch f {
	case FormatPackedSingleBlock:
		valuesPerBlock := 64 / bitsPerValue
		return (valueCount + valuesPerBlock - 1) / valuesPerBlock
	default:
		byteCount := f.ByteCount(packedIntsVersion, valueCount, bitsPerValue)
		return int((byteCount + 7) >> 3)
	}
}

// IsSupported reports whether the format supports the requested
// bitsPerValue.
func (f Format) IsSupported(bitsPerValue int) bool {
	switch f {
	case FormatPackedSingleBlock:
		return packed64SingleBlockIsSupported(bitsPerValue)
	default:
		return bitsPerValue >= 1 && bitsPerValue <= 64
	}
}

// OverheadPerValue returns the per-value overhead, in bits.
func (f Format) OverheadPerValue(bitsPerValue int) float32 {
	switch f {
	case FormatPackedSingleBlock:
		valuesPerBlock := 64 / bitsPerValue
		overhead := 64 % bitsPerValue
		return float32(overhead) / float32(valuesPerBlock)
	default:
		return 0
	}
}

// OverheadRatio returns OverheadPerValue divided by bitsPerValue.
func (f Format) OverheadRatio(bitsPerValue int) float32 {
	return f.OverheadPerValue(bitsPerValue) / float32(bitsPerValue)
}

// FormatAndBits pairs a Format with a bitsPerValue choice.
type FormatAndBits struct {
	Format       Format
	BitsPerValue int
}

// FastestFormatAndBits selects the (Format, bitsPerValue) pair that
// yields the fastest reader whose overhead is below
// acceptableOverheadRatio. Pass valueCount = -1 when the size is
// unknown.
func FastestFormatAndBits(valueCount, bitsPerValue int, acceptableOverheadRatio float32) FormatAndBits {
	if valueCount == -1 {
		valueCount = int(^uint(0) >> 1) // math.MaxInt
	}

	if acceptableOverheadRatio < Compact {
		acceptableOverheadRatio = Compact
	}
	if acceptableOverheadRatio > Fastest {
		acceptableOverheadRatio = Fastest
	}
	acceptableOverheadPerValue := acceptableOverheadRatio * float32(bitsPerValue)

	maxBitsPerValue := bitsPerValue + int(acceptableOverheadPerValue)

	var actualBitsPerValue int
	switch {
	case bitsPerValue <= 8 && maxBitsPerValue >= 8:
		actualBitsPerValue = 8
	case bitsPerValue <= 16 && maxBitsPerValue >= 16:
		actualBitsPerValue = 16
	case bitsPerValue <= 32 && maxBitsPerValue >= 32:
		actualBitsPerValue = 32
	case bitsPerValue <= 64 && maxBitsPerValue >= 64:
		actualBitsPerValue = 64
	default:
		actualBitsPerValue = bitsPerValue
	}
	_ = valueCount // valueCount unused in current selection logic, matches Java
	return FormatAndBits{Format: FormatPacked, BitsPerValue: actualBitsPerValue}
}

// Decoder reads packed values from blocks.
//
// All Decoder/Encoder bulk operations match the layout produced by
// Lucene's BulkOperation classes byte-for-byte.
type Decoder interface {
	LongBlockCount() int
	LongValueCount() int
	ByteBlockCount() int
	ByteValueCount() int

	DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int)
	DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int)
	DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int)
	DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int)
}

// Encoder writes packed values to blocks.
type Encoder interface {
	LongBlockCount() int
	LongValueCount() int
	ByteBlockCount() int
	ByteValueCount() int

	EncodeLongsToLongs(values []int64, valuesOffset int, blocks []int64, blocksOffset, iterations int)
	EncodeLongsToBytes(values []int64, valuesOffset int, blocks []byte, blocksOffset, iterations int)
	EncodeIntsToLongs(values []int32, valuesOffset int, blocks []int64, blocksOffset, iterations int)
	EncodeIntsToBytes(values []int32, valuesOffset int, blocks []byte, blocksOffset, iterations int)
}

// BulkOperation is both an Encoder and a Decoder. It is the shared
// interface returned by BulkOperationOf.
type BulkOperation interface {
	Decoder
	Encoder
	ComputeIterations(valueCount, ramBudget int) int
}

// Reader is a read-only random access array of unsigned long values.
type Reader interface {
	util.Accountable
	// Get returns the value at the given index.
	Get(index int) int64
	// GetBulk reads up to len longs starting from index into
	// arr[off:off+len]; returns the number of values actually read.
	GetBulk(index int, arr []int64, off, length int) int
	// Size is the number of values stored.
	Size() int
}

// ReaderIterator is a run-once iterator over a packed stream.
type ReaderIterator interface {
	// Next reads the next value from the stream.
	Next() (int64, error)
	// NextN reads between 1 and count next values. The returned
	// LongsRef MUST NOT be modified by the caller.
	NextN(count int) (*util.LongsRef, error)
	// GetBitsPerValue returns the bits-per-value of the stream.
	GetBitsPerValue() int
	// Size returns the total number of values in the stream.
	Size() int
	// Ord returns the index of the most recently returned value.
	Ord() int
}

// Mutable is a packed integer array that supports random-access
// reads and writes.
type Mutable interface {
	Reader
	// GetBitsPerValue returns the number of bits used to store any
	// given value.
	GetBitsPerValue() int
	// Set assigns the value at the given index.
	Set(index int, value int64)
	// SetBulk writes up to len longs starting at off in arr into
	// this mutable starting at index. Returns the number written.
	SetBulk(index int, arr []int64, off, length int) int
	// Fill assigns val to every position in [fromIndex, toIndex).
	Fill(fromIndex, toIndex int, val int64)
	// Clear zeroes all positions.
	Clear()
	// GetFormat returns the on-disk format used by this mutable.
	GetFormat() Format
}

// Writer is a write-once serializer for packed integer streams.
type Writer interface {
	// Add appends a value to the stream.
	Add(v int64) error
	// Finish flushes any remaining bytes and pads with zeros so
	// that the declared valueCount is reached.
	Finish() error
	// BitsPerValue returns the bits-per-value of the stream.
	BitsPerValue() int
	// Ord returns the index of the most recently written value.
	Ord() int
	// Format returns the on-disk format used by this writer.
	Format() Format
}

// readerImpl is a small base that stores valueCount. It mirrors
// PackedInts.ReaderImpl.
type readerImpl struct {
	valueCount int
}

func (r *readerImpl) Size() int { return r.valueCount }

// readerBulkGet provides the default sequential bulk-get used by
// readers that lack a faster path.
func readerBulkGet(r Reader, index int, arr []int64, off, length int) int {
	if length <= 0 {
		panic("packed: length must be > 0")
	}
	gets := length
	if remaining := r.Size() - index; remaining < gets {
		gets = remaining
	}
	for i, o := index, off; i < index+gets; i, o = i+1, o+1 {
		arr[o] = r.Get(i)
	}
	return gets
}

// mutableBulkSet provides the default sequential bulk-set used by
// mutables that lack a faster path.
func mutableBulkSet(m Mutable, index int, arr []int64, off, length int) int {
	if length <= 0 {
		panic("packed: length must be > 0")
	}
	if remaining := m.Size() - index; remaining < length {
		length = remaining
	}
	for i, o := index, off; i < index+length; i, o = i+1, o+1 {
		m.Set(i, arr[o])
	}
	return length
}

// mutableFill provides the default sequential fill.
func mutableFill(m Mutable, fromIndex, toIndex int, val int64) {
	for i := fromIndex; i < toIndex; i++ {
		m.Set(i, val)
	}
}

// NullReader is a Reader where every value equals zero. It is used
// as the bitsPerValue=0 short-circuit.
type NullReader struct {
	valueCount int
}

// NewNullReader returns a NullReader of the given length.
func NewNullReader(valueCount int) *NullReader {
	return &NullReader{valueCount: valueCount}
}

// Get returns zero.
func (n *NullReader) Get(_ int) int64 { return 0 }

// GetBulk fills arr[off:off+length] with zeros and returns the
// number of values written.
func (n *NullReader) GetBulk(index int, arr []int64, off, length int) int {
	if length <= 0 {
		panic("packed: length must be > 0")
	}
	if remaining := n.valueCount - index; remaining < length {
		length = remaining
	}
	for i := off; i < off+length; i++ {
		arr[i] = 0
	}
	return length
}

// Size returns the number of zero values.
func (n *NullReader) Size() int { return n.valueCount }

// RamBytesUsed mirrors Java's accounting.
func (n *NullReader) RamBytesUsed() int64 {
	return util.AlignObjectSize(int64(util.NumBytesObjectRef + 4))
}

// GetDecoder returns a Decoder for the given format/bitsPerValue.
func GetDecoder(format Format, version, bitsPerValue int) (Decoder, error) {
	if err := CheckVersion(version); err != nil {
		return nil, err
	}
	return BulkOperationOf(format, bitsPerValue)
}

// GetEncoder returns an Encoder for the given format/bitsPerValue.
func GetEncoder(format Format, version, bitsPerValue int) (Encoder, error) {
	if err := CheckVersion(version); err != nil {
		return nil, err
	}
	return BulkOperationOf(format, bitsPerValue)
}

// GetReaderIteratorNoHeader restores a ReaderIterator over a stream
// that does not contain metadata. The caller must supply the format,
// version, valueCount and bitsPerValue out-of-band.
func GetReaderIteratorNoHeader(in store.DataInput, format Format, version, valueCount, bitsPerValue, mem int) (ReaderIterator, error) {
	if err := CheckVersion(version); err != nil {
		return nil, err
	}
	return newPackedReaderIterator(format, version, valueCount, bitsPerValue, in, mem)
}

// GetMutable returns a Mutable able to hold valueCount values, each
// of bitsPerValue, choosing the implementation based on the
// acceptableOverheadRatio.
func GetMutable(valueCount, bitsPerValue int, acceptableOverheadRatio float32) Mutable {
	fab := FastestFormatAndBits(valueCount, bitsPerValue, acceptableOverheadRatio)
	return GetMutableForFormat(valueCount, fab.BitsPerValue, fab.Format)
}

// GetMutableForFormat returns a Mutable using the supplied format.
func GetMutableForFormat(valueCount, bitsPerValue int, format Format) Mutable {
	if valueCount < 0 {
		panic("packed: valueCount must be non-negative")
	}
	switch format {
	case FormatPackedSingleBlock:
		return newPacked64SingleBlock(valueCount, bitsPerValue)
	case FormatPacked:
		return newPacked64(valueCount, bitsPerValue)
	default:
		panic(fmt.Sprintf("packed: unknown format %d", format))
	}
}

// GetWriterNoHeader returns a write-once Writer over the supplied
// DataOutput. No metadata is written; the caller must persist
// format, valueCount, bitsPerValue and version out-of-band.
//
// If valueCount is -1 the writer accepts an unknown number of
// values; otherwise it tracks the count and writes trailing zeros
// when Finish is called below the expected length.
func GetWriterNoHeader(out store.DataOutput, format Format, valueCount, bitsPerValue, mem int) (Writer, error) {
	return newPackedWriter(format, out, valueCount, bitsPerValue, mem)
}

// BitsRequired returns the minimum number of bits needed to
// represent maxValue (always ≥1).
func BitsRequired(maxValue int64) int {
	if maxValue < 0 {
		panic(fmt.Sprintf("packed: maxValue must be non-negative (got: %d)", maxValue))
	}
	return UnsignedBitsRequired(uint64(maxValue))
}

// UnsignedBitsRequired returns the bit width needed to encode v as
// an unsigned value (always ≥1).
func UnsignedBitsRequired(v uint64) int {
	b := 64 - bits.LeadingZeros64(v)
	if b < 1 {
		return 1
	}
	return b
}

// MaxValue returns the largest unsigned value representable with
// bitsPerValue bits.
func MaxValue(bitsPerValue int) int64 {
	if bitsPerValue == 64 {
		return 0x7FFFFFFFFFFFFFFF // Long.MAX_VALUE in Java
	}
	return int64(^(^uint64(0) << uint(bitsPerValue)))
}

// Copy copies len values from src[srcPos:] into dest[destPos:]
// using at most mem bytes for buffering.
func Copy(src Reader, srcPos int, dest Mutable, destPos, length, mem int) {
	if srcPos+length > src.Size() {
		panic("packed: srcPos+length exceeds src.Size()")
	}
	if destPos+length > dest.Size() {
		panic("packed: destPos+length exceeds dest.Size()")
	}
	capacity := mem >> 3
	if capacity == 0 {
		for i := 0; i < length; i++ {
			dest.Set(destPos, src.Get(srcPos))
			srcPos++
			destPos++
		}
		return
	}
	if length > 0 {
		bufLen := capacity
		if bufLen > length {
			bufLen = length
		}
		buf := make([]int64, bufLen)
		copyWithBuf(src, srcPos, dest, destPos, length, buf)
	}
}

func copyWithBuf(src Reader, srcPos int, dest Mutable, destPos, length int, buf []int64) {
	remaining := 0
	for length > 0 {
		toRead := length
		if free := len(buf) - remaining; toRead > free {
			toRead = free
		}
		read := src.GetBulk(srcPos, buf, remaining, toRead)
		srcPos += read
		length -= read
		remaining += read

		written := dest.SetBulk(destPos, buf, 0, remaining)
		destPos += written
		if written < remaining {
			copy(buf, buf[written:remaining])
		}
		remaining -= written
	}
	for remaining > 0 {
		written := dest.SetBulk(destPos, buf, 0, remaining)
		destPos += written
		remaining -= written
		copy(buf, buf[written:written+remaining])
	}
}

// CheckBlockSize verifies that blockSize is a power of two within
// [minBlockSize, maxBlockSize] and returns log2(blockSize).
func CheckBlockSize(blockSize, minBlockSize, maxBlockSize int) (int, error) {
	if blockSize < minBlockSize || blockSize > maxBlockSize {
		return 0, fmt.Errorf("packed: blockSize must be >= %d and <= %d, got %d", minBlockSize, maxBlockSize, blockSize)
	}
	if blockSize&(blockSize-1) != 0 {
		return 0, fmt.Errorf("packed: blockSize must be a power of two, got %d", blockSize)
	}
	return bits.TrailingZeros64(uint64(blockSize)), nil
}

// NumBlocks returns the number of blocks of size blockSize needed
// to store size values.
func NumBlocks(size int64, blockSize int) (int, error) {
	numBlocks := int(size/int64(blockSize)) + boolToInt(size%int64(blockSize) != 0)
	if int64(numBlocks)*int64(blockSize) < size {
		return 0, fmt.Errorf("packed: size is too large for this block size")
	}
	return numBlocks, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
