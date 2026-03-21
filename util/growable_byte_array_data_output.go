// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"unsafe"
)

// GrowableByteArrayDataOutput is a DataOutput that writes to a growable byte array.
//
// This is the Go port of Lucene's org.apache.lucene.util.GrowableByteArrayDataOutput.
//
// This is useful for building byte arrays dynamically without knowing the final size
// in advance. The internal buffer grows automatically as needed.
type GrowableByteArrayDataOutput struct {
	// bytes is the underlying byte slice
	bytes []byte

	// length is the current write position
	length int

	// initialSize is the initial buffer size
	initialSize int
}

// NewGrowableByteArrayDataOutput creates a new GrowableByteArrayDataOutput
// with the specified initial size.
func NewGrowableByteArrayDataOutput(initialSize int) *GrowableByteArrayDataOutput {
	if initialSize <= 0 {
		initialSize = 1024
	}

	return &GrowableByteArrayDataOutput{
		bytes:       make([]byte, initialSize),
		length:      0,
		initialSize: initialSize,
	}
}

// WriteByte writes a single byte.
func (g *GrowableByteArrayDataOutput) WriteByte(b byte) error {
	if g.length >= len(g.bytes) {
		// Grow the buffer
		newSize := len(g.bytes) * 2
		if newSize < g.length+1 {
			newSize = g.length + 1
		}
		newBytes := make([]byte, newSize)
		copy(newBytes, g.bytes[:g.length])
		g.bytes = newBytes
	}

	g.bytes[g.length] = b
	g.length++
	return nil
}

// WriteBytes writes multiple bytes.
func (g *GrowableByteArrayDataOutput) WriteBytes(buf []byte) error {
	if len(buf) == 0 {
		return nil
	}

	// Ensure capacity
	if g.length+len(buf) > len(g.bytes) {
		newSize := len(g.bytes) * 2
		for newSize < g.length+len(buf) {
			newSize *= 2
		}
		newBytes := make([]byte, newSize)
		copy(newBytes, g.bytes[:g.length])
		g.bytes = newBytes
	}

	copy(g.bytes[g.length:], buf)
	g.length += len(buf)
	return nil
}

// GetBytes returns the written bytes as a slice.
// The returned slice is a copy of the internal buffer.
func (g *GrowableByteArrayDataOutput) GetBytes() []byte {
	result := make([]byte, g.length)
	copy(result, g.bytes[:g.length])
	return result
}

// GetBytesRef returns a BytesRef pointing to the written data.
// Note: The returned BytesRef references the internal buffer, so it may be
// invalidated by subsequent writes.
func (g *GrowableByteArrayDataOutput) GetBytesRef() *BytesRef {
	return NewBytesRef(g.bytes[:g.length])
}

// Length returns the number of bytes written.
func (g *GrowableByteArrayDataOutput) Length() int {
	return g.length
}

// Capacity returns the current capacity of the internal buffer.
func (g *GrowableByteArrayDataOutput) Capacity() int {
	return len(g.bytes)
}

// Reset clears the output and resets to initial state.
func (g *GrowableByteArrayDataOutput) Reset() {
	g.length = 0
	// Keep the buffer but reset position
}

// String returns a string representation of the written bytes.
func (g *GrowableByteArrayDataOutput) String() string {
	return string(g.bytes[:g.length])
}

// WriteString writes a string as UTF-8 bytes.
// Uses unsafe conversion to avoid heap allocation.
func (g *GrowableByteArrayDataOutput) WriteString(s string) error {
	if len(s) > 0 {
		data := unsafe.Slice(unsafe.StringData(s), len(s))
		return g.WriteBytes(data)
	}
	return nil
}

// WriteInt32 writes a 32-bit integer in big-endian format.
func (g *GrowableByteArrayDataOutput) WriteInt32(v int32) error {
	buf := make([]byte, 4)
	buf[0] = byte(v >> 24)
	buf[1] = byte(v >> 16)
	buf[2] = byte(v >> 8)
	buf[3] = byte(v)
	return g.WriteBytes(buf)
}

// WriteInt64 writes a 64-bit integer in big-endian format.
func (g *GrowableByteArrayDataOutput) WriteInt64(v int64) error {
	buf := make([]byte, 8)
	buf[0] = byte(v >> 56)
	buf[1] = byte(v >> 48)
	buf[2] = byte(v >> 40)
	buf[3] = byte(v >> 32)
	buf[4] = byte(v >> 24)
	buf[5] = byte(v >> 16)
	buf[6] = byte(v >> 8)
	buf[7] = byte(v)
	return g.WriteBytes(buf)
}

// WriteVInt writes a variable-length integer.
func (g *GrowableByteArrayDataOutput) WriteVInt(v int32) error {
	for (v & ^0x7F) != 0 {
		if err := g.WriteByte(byte((v & 0x7F) | 0x80)); err != nil {
			return err
		}
		v >>= 7
	}
	return g.WriteByte(byte(v))
}

// WriteVLong writes a variable-length long.
func (g *GrowableByteArrayDataOutput) WriteVLong(v int64) error {
	for (v & ^0x7F) != 0 {
		if err := g.WriteByte(byte((v & 0x7F) | 0x80)); err != nil {
			return err
		}
		v >>= 7
	}
	return g.WriteByte(byte(v))
}

// Ensure GrowableByteArrayDataOutput implements fmt.Stringer
var _ fmt.Stringer = (*GrowableByteArrayDataOutput)(nil)
