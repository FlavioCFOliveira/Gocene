// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
)

// BufferedIndexOutput provides buffering for an IndexOutput.
// It is a Go port of the buffering logic used in Lucene's IndexOutput implementations,
// such as the internal buffering in OutputStreamIndexOutput.
type BufferedIndexOutput struct {
	out            IndexOutput
	buffer         []byte
	bufferPosition int
	bufferSize     int
}

// NewBufferedIndexOutput creates a new BufferedIndexOutput that wraps the given IndexOutput.
// If bufferSize <= 0, a default size of 1024 is used.
func NewBufferedIndexOutput(out IndexOutput, bufferSize int) *BufferedIndexOutput {
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	return &BufferedIndexOutput{
		out:            out,
		buffer:         make([]byte, bufferSize),
		bufferPosition: 0,
		bufferSize:     bufferSize,
	}
}

// WriteByte writes a single byte, using the buffer when possible.
func (out *BufferedIndexOutput) WriteByte(b byte) error {
	if out.bufferPosition >= out.bufferSize {
		if err := out.Flush(); err != nil {
			return err
		}
	}
	out.buffer[out.bufferPosition] = b
	out.bufferPosition++
	return nil
}

// WriteBytes writes all bytes from b, using the buffer when possible.
func (out *BufferedIndexOutput) WriteBytes(b []byte, offset, length int) error {
	return out.WriteBytesN(b, length)
}

// WriteBytesN writes exactly n bytes from b, using the buffer when possible.
func (out *BufferedIndexOutput) WriteBytesN(b []byte, n int) error {
	if n < 0 || n > len(b) {
		return fmt.Errorf("invalid length %d for byte slice of length %d", n, len(b))
	}

	// If the write is larger than the buffer, flush and write directly to avoid unnecessary copy
	if n >= out.bufferSize {
		if err := out.Flush(); err != nil {
			return err
		}
		return out.out.WriteBytes(b, 0, n)
	}

	// Use the buffer for smaller writes
	if out.bufferPosition+n > out.bufferSize {
		if err := out.Flush(); err != nil {
			return err
		}
	}

	copy(out.buffer[out.bufferPosition:], b[:n])
	out.bufferPosition += n
	return nil
}

func (out *BufferedIndexOutput) WriteShort(i int16) error {
	b := []byte{byte(i), byte(i >> 8)}
	return out.WriteBytes(b, 0, 2)
}

// WriteInt writes a 32-bit value as little-endian.
func (out *BufferedIndexOutput) WriteInt(i int32) error {
	b := []byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)}
	return out.WriteBytes(b, 0, 4)
}

// WriteLong writes a 64-bit value as little-endian.
func (out *BufferedIndexOutput) WriteLong(i int64) error {
	b := []byte{
		byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24),
		byte(i >> 32), byte(i >> 40), byte(i >> 48), byte(i >> 56),
	}
	return out.WriteBytes(b, 0, 8)
}

// WriteString writes a string.
func (out *BufferedIndexOutput) WriteString(s string) error {
	return out.out.WriteString(s)
}

// WriteVInt writes a variable-length integer.
func (out *BufferedIndexOutput) WriteVInt(i int32) error {
	return out.out.WriteVInt(i)
}

// WriteVLong writes a variable-length long.
func (out *BufferedIndexOutput) WriteVLong(i int64) error {
	return out.out.WriteVLong(i)
}

// WriteZInt writes a zig-zag encoded integer.
func (out *BufferedIndexOutput) WriteZInt(i int32) error {
	return out.out.WriteZInt(i)
}

// WriteZLong writes a zig-zag encoded long.
func (out *BufferedIndexOutput) WriteZLong(i int64) error {
	return out.out.WriteZLong(i)
}

// Flush flushes any buffered bytes to the underlying output.
func (out *BufferedIndexOutput) Flush() error {
	if out.bufferPosition > 0 {
		if err := out.out.WriteBytes(out.buffer, 0, out.bufferPosition); err != nil {
			return err
		}
		out.bufferPosition = 0
	}
	return nil
}

// GetBufferSize returns the current buffer size.
func (out *BufferedIndexOutput) GetBufferSize() int {
	return out.bufferSize
}

// SetBufferSize changes the buffer size.
// This flushes any existing buffer contents first.
func (out *BufferedIndexOutput) SetBufferSize(size int) error {
	if err := out.Flush(); err != nil {
		return err
	}
	if size <= 0 {
		size = 1024
	}
	out.bufferSize = size
	out.buffer = make([]byte, size)
	out.bufferPosition = 0
	return nil
}

// GetFilePointer returns the current position in the output, including buffered bytes.
func (out *BufferedIndexOutput) GetFilePointer() int64 {
	return out.out.GetFilePointer() + int64(out.bufferPosition)
}

// GetName returns the name of the file being written.
func (out *BufferedIndexOutput) GetName() string {
	return out.out.GetName()
}

// SetPosition changes the current position in the output.
func (out *BufferedIndexOutput) SetPosition(pos int64) error {
	out.bufferPosition = 0
	out.buffer = make([]byte, out.bufferSize)
	return out.out.SetPosition(pos)
}

// Close flushes any remaining buffered data and closes the underlying output.
func (out *BufferedIndexOutput) Close() error {
	if err := out.Flush(); err != nil {
		return err
	}
	return out.out.Close()
}

// Length returns the total length of the output.
func (out *BufferedIndexOutput) Length() int64 {
	return out.out.Length()
}

// CopyBytes copies bytes from the given input into this output.
func (out *BufferedIndexOutput) CopyBytes(input DataInput, numBytes int64) error {
	return out.out.CopyBytes(input, numBytes)
}

// WriteMapOfStrings writes a map of strings.
func (out *BufferedIndexOutput) WriteMapOfStrings(m map[string]string) error {
	return out.out.WriteMapOfStrings(m)
}

// WriteSetOfStrings writes a set of strings.
func (out *BufferedIndexOutput) WriteSetOfStrings(s []string) error {
	return out.out.WriteSetOfStrings(s)
}

// WriteGroupVInts writes a group of VInts.
func (out *BufferedIndexOutput) WriteGroupVInts(values []int32, limit int) error {
	return out.out.WriteGroupVInts(values, limit)
}
