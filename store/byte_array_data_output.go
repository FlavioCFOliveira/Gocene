// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// ByteArrayDataOutput is a port of org.apache.lucene.store.ByteArrayDataOutput.
// It provides a DataOutput backed by a single byte array.
//
// WARNING: This class omits most low-level checks.
type ByteArrayDataOutput struct {
	BaseDataOutput
	bytes []byte
	pos   int
	limit int
}

// NewByteArrayDataOutput creates a new ByteArrayDataOutput backed by the given byte slice.
func NewByteArrayDataOutput(bytes []byte) *ByteArrayDataOutput {
	out := &ByteArrayDataOutput{}
	out.Reset(bytes)
	return out
}

// NewByteArrayDataOutputWithOffset creates a new ByteArrayDataOutput backed by the given byte slice,
// starting at the specified offset and with the given length.
func NewByteArrayDataOutputWithOffset(bytes []byte, offset, len int) *ByteArrayDataOutput {
	out := &ByteArrayDataOutput{}
	out.ResetWithOffset(bytes, offset, len)
	return out
}

// Reset resets the output to write into the given byte slice.
func (out *ByteArrayDataOutput) Reset(bytes []byte) {
	out.ResetWithOffset(bytes, 0, len(bytes))
}

// ResetWithOffset resets the output to write into the given byte slice, starting at the specified offset
// and with the given length.
func (out *ByteArrayDataOutput) ResetWithOffset(bytes []byte, offset, len int) {
	out.bytes = bytes
	out.pos = offset
	out.limit = offset + len
	out.primitive = out
}

// GetPosition returns the current position.
func (out *ByteArrayDataOutput) GetPosition() int {
	return out.pos
}

// WriteByte implements PrimitiveWriter.
func (out *ByteArrayDataOutput) WriteByte(b byte) error {
	out.bytes[out.pos] = b
	out.pos++
	return nil
}

// WriteBytes implements PrimitiveWriter.
func (out *ByteArrayDataOutput) WriteBytes(b []byte, offset, length int) error {
	copy(out.bytes[out.pos:], b[offset:offset+length])
	out.pos += length
	return nil
}
