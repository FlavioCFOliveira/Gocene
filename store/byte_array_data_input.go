// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// ByteArrayDataInput is a port of org.apache.lucene.store.ByteArrayDataInput.
// It provides a DataInput backed by a single byte array.
//
// WARNING: This class omits all low-level checks.
type ByteArrayDataInput struct {
	spi.BaseDataInput
	bytes []byte
	pos   int
	limit int
}

// NewByteArrayDataInput creates a new ByteArrayDataInput backed by the given byte slice.
func NewByteArrayDataInput(bytes []byte) *ByteArrayDataInput {
	in := &ByteArrayDataInput{}
	in.Reset(bytes)
	return in
}

// NewByteArrayDataInputWithOffset creates a new ByteArrayDataInput backed by the given byte slice,
// starting at the specified offset and with the given length.
func NewByteArrayDataInputWithOffset(bytes []byte, offset, len int) *ByteArrayDataInput {
	in := &ByteArrayDataInput{}
	in.ResetWithOffset(bytes, offset, len)
	return in
}

// Reset resets the input to read from the given byte slice.
func (in *ByteArrayDataInput) Reset(bytes []byte) {
	in.ResetWithOffset(bytes, 0, len(bytes))
}

// ResetWithOffset resets the input to read from the given byte slice, starting at the specified offset
// and with the given length.
func (in *ByteArrayDataInput) ResetWithOffset(bytes []byte, offset, len int) {
	in.bytes = bytes
	in.pos = offset
	in.limit = offset + len
	in.Core = in
}

// Rewind sets the position to 0.
// NOTE: this is not correct if ResetWithOffset was called with a non-zero offset.
func (in *ByteArrayDataInput) Rewind() {
	in.pos = 0
}

// GetPosition returns the current position.
func (in *ByteArrayDataInput) GetPosition() int {
	return in.pos
}

// SetPosition sets the current position.
func (in *ByteArrayDataInput) SetPosition(pos int) {
	in.pos = pos
}

// Length returns the total number of bytes available to read.
func (in *ByteArrayDataInput) Length() int {
	return in.limit
}

// EOF returns true if the current position has reached the limit.
func (in *ByteArrayDataInput) EOF() bool {
	return in.pos == in.limit
}

// ReadByte implements DataInputCore.
func (in *ByteArrayDataInput) ReadByte() (byte, error) {
	if in.pos >= in.limit {
		return 0, io.EOF
	}
	b := in.bytes[in.pos]
	in.pos++
	return b, nil
}

// ReadBytes implements DataInputCore.
func (in *ByteArrayDataInput) ReadBytes(b []byte, offset, len int) error {
	if in.pos+len > in.limit {
		return io.EOF
	}
	copy(b[offset:], in.bytes[in.pos:in.pos+len])
	in.pos += len
	return nil
}

// SkipBytes implements DataInputCore.
func (in *ByteArrayDataInput) SkipBytes(count int64) error {
	in.pos += int(count)
	return nil
}

// Clone returns a clone of this stream.
func (in *ByteArrayDataInput) Clone() DataInput {
	clone := &ByteArrayDataInput{
		bytes: in.bytes,
		pos:   in.pos,
		limit: in.limit,
	}
	clone.Core = clone
	return clone
}
