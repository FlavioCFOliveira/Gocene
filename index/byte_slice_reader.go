// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ByteSliceReader is a DataInput that reads byte slices written by Posting and
// PostingVector. It reads the bytes in each slice until it reaches the end of
// that slice, at which point it reads the forwarding address of the next slice
// and jumps to it.
//
// Port of Lucene's org.apache.lucene.index.ByteSliceReader.
//
// Divergence: Lucene reads the forwarding address through ByteSlicePool. As
// ByteSlicePool has not yet been ported, the level/next-level arrays are kept
// here as unexported package-level variables. When ByteSlicePool lands, this
// file should switch to the shared exports.
type ByteSliceReader struct {
	pool         *util.ByteBlockPool
	bufferUpto   int
	buffer       []byte
	Upto         int
	limit        int
	level        int
	BufferOffset int
	EndIndex     int
}

// byteSliceLevelSizeArray mirrors ByteSlicePool.LEVEL_SIZE_ARRAY.
var byteSliceLevelSizeArray = [...]int{5, 14, 20, 30, 40, 40, 80, 80, 120, 200}

// byteSliceNextLevelArray mirrors ByteSlicePool.NEXT_LEVEL_ARRAY.
var byteSliceNextLevelArray = [...]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 9}

// Compile-time guarantee that *ByteSliceReader satisfies store.DataInput.
var _ store.DataInput = (*ByteSliceReader)(nil)

// Init configures the reader to read from the given pool, between startIndex
// (inclusive) and endIndex (exclusive). Both indices are absolute byte offsets
// within the pool. Both must be non-negative and endIndex must be greater than
// or equal to startIndex.
func (r *ByteSliceReader) Init(pool *util.ByteBlockPool, startIndex, endIndex int) error {
	if startIndex < 0 {
		return fmt.Errorf("ByteSliceReader: startIndex must be >= 0, got %d", startIndex)
	}
	if endIndex < 0 {
		return fmt.Errorf("ByteSliceReader: endIndex must be >= 0, got %d", endIndex)
	}
	if endIndex < startIndex {
		return fmt.Errorf("ByteSliceReader: endIndex %d must be >= startIndex %d", endIndex, startIndex)
	}

	r.pool = pool
	r.EndIndex = endIndex

	r.level = 0
	r.bufferUpto = startIndex / util.ByteBlockSize
	r.BufferOffset = r.bufferUpto * util.ByteBlockSize
	r.buffer = pool.GetBuffer(r.bufferUpto)
	r.Upto = startIndex & util.ByteBlockMask

	firstSize := byteSliceLevelSizeArray[0]

	if startIndex+firstSize >= endIndex {
		// Only one slice to read.
		r.limit = endIndex & util.ByteBlockMask
	} else {
		// Reserve 4 trailing bytes for the forwarding address.
		r.limit = r.Upto + firstSize - 4
	}
	return nil
}

// EOF reports whether the reader has consumed every byte up to EndIndex.
func (r *ByteSliceReader) EOF() bool {
	return r.Upto+r.BufferOffset == r.EndIndex
}

// ReadByte reads the next byte, advancing to the next slice when necessary.
// Returns io.EOF if all bytes have already been consumed.
func (r *ByteSliceReader) ReadByte() (byte, error) {
	if r.EOF() {
		return 0, io.EOF
	}
	if r.Upto == r.limit {
		r.nextSlice()
	}
	b := r.buffer[r.Upto]
	r.Upto++
	return b, nil
}

// WriteTo copies the remaining bytes to out and returns the number of bytes
// written.
func (r *ByteSliceReader) WriteTo(out store.DataOutput) (int64, error) {
	var size int64
	for {
		if r.limit+r.BufferOffset == r.EndIndex {
			// Final slice: write up to limit and stop.
			if err := out.WriteBytes(r.buffer, r.Upto, r.limit-r.Upto); err != nil {
				return size, err
			}
			size += int64(r.limit - r.Upto)
			return size, nil
		}
		if err := out.WriteBytes(r.buffer, r.Upto, r.limit-r.Upto); err != nil {
			return size, err
		}
		size += int64(r.limit - r.Upto)
		r.nextSlice()
	}
}

// nextSlice follows the forwarding pointer at the end of the current slice and
// updates the reader to point at the next slice.
func (r *ByteSliceReader) nextSlice() {
	// Forwarding address is stored little-endian over the trailing 4 bytes.
	nextIndex := int(binary.LittleEndian.Uint32(r.buffer[r.limit:]))

	r.level = byteSliceNextLevelArray[r.level]
	newSize := byteSliceLevelSizeArray[r.level]

	r.bufferUpto = nextIndex / util.ByteBlockSize
	r.BufferOffset = r.bufferUpto * util.ByteBlockSize

	r.buffer = r.pool.GetBuffer(r.bufferUpto)
	r.Upto = nextIndex & util.ByteBlockMask

	if nextIndex+newSize >= r.EndIndex {
		// Advancing to the final slice; consume up to EndIndex.
		r.limit = r.EndIndex - r.BufferOffset
	} else {
		// Reserve trailing 4 bytes for the next forwarding address.
		r.limit = r.Upto + newSize - 4
	}
}

// ReadBytes fills b in full, hopping across slice boundaries as needed.
// Returns io.EOF if the request would read past EndIndex.
func (r *ByteSliceReader) ReadBytes(b []byte, offset, length int) error {
	remaining := length
	for remaining > 0 {
		numLeft := r.limit - r.Upto
		if numLeft < remaining {
			if r.BufferOffset+r.limit == r.EndIndex {
				return io.EOF
			}
			copy(b[offset:offset+numLeft], r.buffer[r.Upto:r.limit])
			offset += numLeft
			remaining -= numLeft
			r.nextSlice()
		} else {
			copy(b[offset:offset+remaining], r.buffer[r.Upto:r.Upto+remaining])
			r.Upto += remaining
			return nil
		}
	}
	return nil
}

// ReadBytesN reads exactly n bytes and returns them.
func (r *ByteSliceReader) ReadBytesN(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("ByteSliceReader: ReadBytesN n must be >= 0, got %d", n)
	}
	out := make([]byte, n)
	if err := r.ReadBytes(out, 0, len(out)); err != nil {
		return nil, err
	}
	return out, nil
}

// ReadShort reads a 16-bit little-endian value.
func (r *ByteSliceReader) ReadShort() (int16, error) {
	var buf [2]byte
	if err := r.ReadBytes(buf[:], 0, len(buf)); err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf[:])), nil
}

// ReadInt reads a 32-bit little-endian value.
func (r *ByteSliceReader) ReadInt() (int32, error) {
	var buf [4]byte
	if err := r.ReadBytes(buf[:], 0, len(buf)); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf[:])), nil
}

// ReadLong reads a 64-bit little-endian value.
func (r *ByteSliceReader) ReadLong() (int64, error) {
	var buf [8]byte
	if err := r.ReadBytes(buf[:], 0, len(buf)); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf[:])), nil
}

// ReadString reads a VInt length followed by that many UTF-8 bytes.
func (r *ByteSliceReader) ReadString() (string, error) {
	length, err := r.ReadVInt()
	if err != nil {
		return "", err
	}
	if length < 0 {
		return "", fmt.Errorf("ByteSliceReader: negative string length %d", length)
	}
	buf := make([]byte, length)
	if err := r.ReadBytes(buf, 0, len(buf)); err != nil {
		return "", err
	}
	return string(buf), nil
}

// ReadVInt reads an int stored in variable-length format.
func (r *ByteSliceReader) ReadVInt() (int32, error) {
	var result int32
	shift := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("ByteSliceReader: corrupted VInt")
		}
	}
}

// ReadVLong reads an int64 stored in variable-length format.
func (r *ByteSliceReader) ReadVLong() (int64, error) {
	var result int64
	shift := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("ByteSliceReader: corrupted VLong")
		}
	}
}

// SkipBytes advances the reader by numBytes, hopping across slice boundaries.
// Returns an error if numBytes is negative.
func (r *ByteSliceReader) SkipBytes(numBytes int64) error {
	if numBytes < 0 {
		return fmt.Errorf("ByteSliceReader: numBytes must be >= 0, got %d", numBytes)
	}
	for numBytes > 0 {
		numLeft := int64(r.limit - r.Upto)
		if numLeft < numBytes {
			if int64(r.BufferOffset+r.limit) == int64(r.EndIndex) {
				return io.EOF
			}
			numBytes -= numLeft
			r.nextSlice()
		} else {
			r.Upto += int(numBytes)
			return nil
		}
	}
	return nil
}

func (r *ByteSliceReader) ReadZInt() (int32, error) {
	v, err := r.ReadVInt()
	if err != nil {
		return 0, err
	}
	return (v >> 1) ^ -(v & 1), nil
}

func (r *ByteSliceReader) ReadZLong() (int64, error) {
	var result int64
	shift := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("ByteSliceReader: corrupted VLong")
		}
	}
	return (result >> 1) ^ -(result & 1), nil
}

func (r *ByteSliceReader) ReadInts(dst []int32, offset, length int) error {
	buf := make([]byte, 4*length)
	if err := r.ReadBytes(buf, 0, len(buf)); err != nil {
		return err
	}
	for i := 0; i < length; i++ {
		dst[offset+i] = int32(binary.LittleEndian.Uint32(buf[i*4 : (i+1)*4]))
	}
	return nil
}

func (r *ByteSliceReader) ReadLongs(dst []int64, offset, length int) error {
	buf := make([]byte, 8*length)
	if err := r.ReadBytes(buf, 0, len(buf)); err != nil {
		return err
	}
	for i := 0; i < length; i++ {
		dst[offset+i] = int64(binary.LittleEndian.Uint64(buf[i*8 : (i+1)*8]))
	}
	return nil
}

func (r *ByteSliceReader) ReadFloats(dst []float32, offset, length int) error {
	buf := make([]byte, 4*length)
	if err := r.ReadBytes(buf, 0, len(buf)); err != nil {
		return err
	}
	for i := 0; i < length; i++ {
		dst[offset+i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4 : (i+1)*4]))
	}
	return nil
}

func (r *ByteSliceReader) ReadMapOfStrings() (map[string]string, error) {
	size, err := r.ReadVInt()
	if err != nil {
		return nil, err
	}
	res := make(map[string]string, size)
	for i := 0; i < int(size); i++ {
		k, err := r.ReadString()
		if err != nil {
			return nil, err
		}
		v, err := r.ReadString()
		if err != nil {
			return nil, err
		}
		res[k] = v
	}
	return res, nil
}

func (r *ByteSliceReader) ReadSetOfStrings() ([]string, error) {
	size, err := r.ReadVInt()
	if err != nil {
		return nil, err
	}
	res := make([]string, size)
	for i := 0; i < int(size); i++ {
		s, err := r.ReadString()
		if err != nil {
			return nil, err
		}
		res[i] = s
	}
	return res, nil
}

