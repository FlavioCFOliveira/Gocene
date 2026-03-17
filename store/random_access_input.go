// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"fmt"
)

// ByteArrayRandomAccessInput implements RandomAccessInput over a byte slice.
// This provides efficient random access to in-memory data.
//
// This is the Go port of Lucene's RandomAccessInput implementation
// backed by byte arrays.
type ByteArrayRandomAccessInput struct {
	bytes []byte
}

// NewByteArrayRandomAccessInput creates a new ByteArrayRandomAccessInput
// from the given byte slice. The slice is not copied, so the caller
// should not modify it after passing to this function.
func NewByteArrayRandomAccessInput(bytes []byte) *ByteArrayRandomAccessInput {
	return &ByteArrayRandomAccessInput{
		bytes: bytes,
	}
}

// ReadByteAt reads a single byte at the given position.
func (in *ByteArrayRandomAccessInput) ReadByteAt(pos int64) (byte, error) {
	if pos < 0 || pos >= int64(len(in.bytes)) {
		return 0, fmt.Errorf("position %d out of range [0, %d]", pos, len(in.bytes))
	}
	return in.bytes[pos], nil
}

// ReadShortAt reads a 16-bit value at the given position in little-endian format.
func (in *ByteArrayRandomAccessInput) ReadShortAt(pos int64) (int16, error) {
	if pos < 0 || pos+2 > int64(len(in.bytes)) {
		return 0, fmt.Errorf("position %d out of range for short read [0, %d]", pos, len(in.bytes))
	}
	v := binary.LittleEndian.Uint16(in.bytes[pos:])
	return int16(v), nil
}

// ReadIntAt reads a 32-bit value at the given position in little-endian format.
func (in *ByteArrayRandomAccessInput) ReadIntAt(pos int64) (int32, error) {
	if pos < 0 || pos+4 > int64(len(in.bytes)) {
		return 0, fmt.Errorf("position %d out of range for int read [0, %d]", pos, len(in.bytes))
	}
	v := binary.LittleEndian.Uint32(in.bytes[pos:])
	return int32(v), nil
}

// ReadLongAt reads a 64-bit value at the given position in little-endian format.
func (in *ByteArrayRandomAccessInput) ReadLongAt(pos int64) (int64, error) {
	if pos < 0 || pos+8 > int64(len(in.bytes)) {
		return 0, fmt.Errorf("position %d out of range for long read [0, %d]", pos, len(in.bytes))
	}
	v := binary.LittleEndian.Uint64(in.bytes[pos:])
	return int64(v), nil
}

// Length returns the total length of the input in bytes.
func (in *ByteArrayRandomAccessInput) Length() int64 {
	return int64(len(in.bytes))
}

// Slice returns a new ByteArrayRandomAccessInput that shares the underlying
// byte slice starting at offset with the given length.
func (in *ByteArrayRandomAccessInput) Slice(offset int64, length int64) (*ByteArrayRandomAccessInput, error) {
	if offset < 0 {
		return nil, fmt.Errorf("slice offset %d is negative", offset)
	}
	if length < 0 {
		return nil, fmt.Errorf("slice length %d is negative", length)
	}
	if offset+length > int64(len(in.bytes)) {
		return nil, fmt.Errorf("slice offset %d + length %d exceeds input length %d", offset, length, len(in.bytes))
	}
	return NewByteArrayRandomAccessInput(in.bytes[offset : offset+length]), nil
}

// Ensure ByteArrayRandomAccessInput implements RandomAccessInput
var _ RandomAccessInput = (*ByteArrayRandomAccessInput)(nil)
