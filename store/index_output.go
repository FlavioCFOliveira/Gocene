// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/adler32"
)

// IndexOutput provides random access to write index files.
//
// IndexOutput is the abstract base class for writing index files.
// It provides methods for writing primitive types (byte, int, long, etc.)
// and arbitrary byte arrays. All writes are byte-aligned.
//
// This is the Go port of Lucene's org.apache.lucene.store.IndexOutput.
type IndexOutput interface {
	// DataOutput provides basic write operations
	DataOutput

	// GetFilePointer returns the current position in the file.
	GetFilePointer() int64

	// Length returns the total length of the file written so far.
	Length() int64

	// GetName returns the name of the file being written.
	GetName() string
}

// DataOutput defines the interface for writing basic data types.
// This is embedded in IndexOutput and other write interfaces.
type DataOutput interface {
	// WriteByte writes a single byte.
	WriteByte(b byte) error

	// WriteBytes writes all bytes from b.
	WriteBytes(b []byte) error

	// WriteBytesN writes exactly len(b) bytes from b.
	WriteBytesN(b []byte, len int) error
}

// BaseIndexOutput provides common functionality for IndexOutput implementations.
// Embed this struct in concrete IndexOutput implementations.
type BaseIndexOutput struct {
	// name is the name of the file being written
	name string

	// filePointer is the current position in the output
	filePointer int64
}

// NewBaseIndexOutput creates a new BaseIndexOutput.
func NewBaseIndexOutput(name string) *BaseIndexOutput {
	return &BaseIndexOutput{
		name:        name,
		filePointer: 0,
	}
}

// GetName returns the name of the file being written.
func (out *BaseIndexOutput) GetName() string {
	return out.name
}

// GetFilePointer returns the current position in the output.
func (out *BaseIndexOutput) GetFilePointer() int64 {
	return out.filePointer
}

// SetFilePointer sets the current position in the output.
// This should only be called by implementations.
func (out *BaseIndexOutput) SetFilePointer(pos int64) {
	out.filePointer = pos
}

// IncrementFilePointer increments the file pointer by n bytes.
func (out *BaseIndexOutput) IncrementFilePointer(n int64) {
	out.filePointer += n
}

// WriteBytes writes all bytes from b.
func (out *BaseIndexOutput) WriteBytes(b []byte) error {
	return errors.New("WriteBytes not implemented in BaseIndexOutput")
}

// WriteBytesN writes exactly n bytes from b.
func (out *BaseIndexOutput) WriteBytesN(b []byte, n int) error {
	return errors.New("WriteBytesN not implemented in BaseIndexOutput")
}

// WriteByte writes a single byte.
func (out *BaseIndexOutput) WriteByte(b byte) error {
	return errors.New("WriteByte not implemented in BaseIndexOutput")
}

// ByteArrayDataOutput implements DataOutput over a byte slice.
// This is useful for writing to in-memory buffers.
type ByteArrayDataOutput struct {
	bytes []byte
	pos   int
}

// NewByteArrayDataOutput creates a new ByteArrayDataOutput with the given initial capacity.
func NewByteArrayDataOutput(initialCapacity int) *ByteArrayDataOutput {
	return &ByteArrayDataOutput{
		bytes: make([]byte, 0, initialCapacity),
		pos:   0,
	}
}

// WriteByte writes a single byte.
func (out *ByteArrayDataOutput) WriteByte(b byte) error {
	if out.pos >= len(out.bytes) {
		out.bytes = append(out.bytes, b)
	} else {
		out.bytes[out.pos] = b
	}
	out.pos++
	return nil
}

// WriteBytes writes all bytes from b.
func (out *ByteArrayDataOutput) WriteBytes(b []byte) error {
	return out.WriteBytesN(b, len(b))
}

// WriteBytesN writes exactly n bytes from b.
func (out *ByteArrayDataOutput) WriteBytesN(b []byte, n int) error {
	if n < 0 || n > len(b) {
		return fmt.Errorf("invalid length %d for byte slice of length %d", n, len(b))
	}
	if out.pos+len(b) > len(out.bytes) {
		out.bytes = append(out.bytes, b...)
	} else {
		copy(out.bytes[out.pos:], b[:n])
	}
	out.pos += n
	return nil
}

// GetBytes returns the written bytes.
func (out *ByteArrayDataOutput) GetBytes() []byte {
	return out.bytes[:out.pos]
}

// GetPosition returns the current position.
func (out *ByteArrayDataOutput) GetPosition() int {
	return out.pos
}

// Reset resets the output for reuse.
func (out *ByteArrayDataOutput) Reset() {
	out.pos = 0
}

// Length returns the total length.
func (out *ByteArrayDataOutput) Length() int {
	return out.pos
}

// BufferedIndexOutput is a base class for buffered IndexOutput implementations.
// It provides buffering for small writes while allowing direct access for large writes.
type BufferedIndexOutput struct {
	*BaseIndexOutput

	// buffer is the write buffer
	buffer []byte

	// bufferPosition is the current position within the buffer
	bufferPosition int

	// bufferSize is the size of the buffer
	bufferSize int
}

// NewBufferedIndexOutput creates a new BufferedIndexOutput.
// If bufferSize is <= 0, a default size of 1024 is used.
func NewBufferedIndexOutput(name string, bufferSize int) *BufferedIndexOutput {
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	return &BufferedIndexOutput{
		BaseIndexOutput: NewBaseIndexOutput(name),
		buffer:          make([]byte, bufferSize),
		bufferPosition:  0,
		bufferSize:      bufferSize,
	}
}

// WriteByte writes a single byte, using the buffer when possible.
func (out *BufferedIndexOutput) WriteByte(b byte) error {
	if out.bufferPosition >= out.bufferSize {
		if err := out.flush(); err != nil {
			return err
		}
	}
	out.buffer[out.bufferPosition] = b
	out.bufferPosition++
	out.IncrementFilePointer(1)
	return nil
}

// WriteBytes writes all bytes from b.
func (out *BufferedIndexOutput) WriteBytes(b []byte) error {
	return out.WriteBytesN(b, len(b))
}

// WriteBytesN writes exactly n bytes from b.
func (out *BufferedIndexOutput) WriteBytesN(b []byte, n int) error {
	if n < 0 || n > len(b) {
		return fmt.Errorf("invalid length %d for byte slice of length %d", n, len(b))
	}

	// If the write is larger than the buffer, flush and write directly
	if n >= out.bufferSize {
		if err := out.flush(); err != nil {
			return err
		}
		if err := out.writeInternal(b[:n]); err != nil {
			return err
		}
		out.IncrementFilePointer(int64(n))
		return nil
	}

	// Use the buffer for smaller writes
	if out.bufferPosition+n > out.bufferSize {
		if err := out.flush(); err != nil {
			return err
		}
	}

	copy(out.buffer[out.bufferPosition:], b[:n])
	out.bufferPosition += n
	out.IncrementFilePointer(int64(n))
	return nil
}

// Flush flushes any buffered bytes to the underlying output.
func (out *BufferedIndexOutput) Flush() error {
	return out.flush()
}

// flush flushes the buffer to the underlying output.
func (out *BufferedIndexOutput) flush() error {
	if out.bufferPosition > 0 {
		if err := out.writeInternal(out.buffer[:out.bufferPosition]); err != nil {
			return err
		}
		out.bufferPosition = 0
	}
	return nil
}

// writeInternal writes directly to the underlying output.
// Subclasses must override this method.
func (out *BufferedIndexOutput) writeInternal(b []byte) error {
	return errors.New("writeInternal not implemented")
}

// GetBufferSize returns the buffer size.
func (out *BufferedIndexOutput) GetBufferSize() int {
	return out.bufferSize
}

// SetBufferSize changes the buffer size.
// This flushes any existing buffer contents first.
func (out *BufferedIndexOutput) SetBufferSize(size int) error {
	if err := out.flush(); err != nil {
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

// Close flushes and closes the output.
// Subclasses should call this via defer.
func (out *BufferedIndexOutput) Close() error {
	return out.flush()
}

// WriteUint16 writes a 16-bit unsigned integer in big-endian format.
func WriteUint16(out DataOutput, v uint16) error {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return out.WriteBytes(b)
}

// WriteUint32 writes a 32-bit unsigned integer in big-endian format.
func WriteUint32(out DataOutput, v uint32) error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return out.WriteBytes(b)
}

// WriteUint64 writes a 64-bit unsigned integer in big-endian format.
func WriteUint64(out DataOutput, v uint64) error {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return out.WriteBytes(b)
}

// WriteInt16 writes a 16-bit signed integer in big-endian format.
func WriteInt16(out DataOutput, v int16) error {
	return WriteUint16(out, uint16(v))
}

// WriteInt32 writes a 32-bit signed integer in big-endian format.
func WriteInt32(out DataOutput, v int32) error {
	return WriteUint32(out, uint32(v))
}

// WriteInt64 writes a 64-bit signed integer in big-endian format.
func WriteInt64(out DataOutput, v int64) error {
	return WriteUint64(out, uint64(v))
}

// WriteVInt writes a variable-length integer (up to 5 bytes).
// This is Lucene's variable-length integer encoding.
func WriteVInt(out DataOutput, i int32) error {
	for (i & ^int32(0x7F)) != 0 {
		if err := out.WriteByte(byte((i & 0x7F) | 0x80)); err != nil {
			return err
		}
		i >>= 7
	}
	return out.WriteByte(byte(i))
}

// WriteVLong writes a variable-length long (up to 9 bytes).
func WriteVLong(out DataOutput, i int64) error {
	for (i & ^int64(0x7F)) != 0 {
		if err := out.WriteByte(byte((i & 0x7F) | 0x80)); err != nil {
			return err
		}
		i >>= 7
	}
	return out.WriteByte(byte(i))
}

// WriteString writes a string as length-prefixed UTF-8.
// The length is written as a VInt, followed by the UTF-8 bytes.
func WriteString(out DataOutput, s string) error {
	if err := WriteVInt(out, int32(len(s))); err != nil {
		return err
	}
	return out.WriteBytes([]byte(s))
}

// IndexOutputWithDigest wraps an IndexOutput and computes a digest (checksum).
// This is useful for verifying data integrity on write.
type IndexOutputWithDigest struct {
	IndexOutput
	digest hash.Hash32
}

// NewIndexOutputWithDigest creates a new IndexOutputWithDigest.
func NewIndexOutputWithDigest(out IndexOutput) *IndexOutputWithDigest {
	return &IndexOutputWithDigest{
		IndexOutput: out,
		digest:      adler32.New(),
	}
}

// WriteByte writes a single byte and updates the digest.
func (out *IndexOutputWithDigest) WriteByte(b byte) error {
	out.digest.Write([]byte{b})
	return out.IndexOutput.WriteByte(b)
}

// WriteBytes writes all bytes from b and updates the digest.
func (out *IndexOutputWithDigest) WriteBytes(b []byte) error {
	out.digest.Write(b)
	return out.IndexOutput.WriteBytes(b)
}

// WriteBytesN writes exactly n bytes from b and updates the digest.
func (out *IndexOutputWithDigest) WriteBytesN(b []byte, n int) error {
	if n < 0 || n > len(b) {
		return fmt.Errorf("invalid length %d for byte slice of length %d", n, len(b))
	}
	out.digest.Write(b[:n])
	return out.IndexOutput.WriteBytesN(b, n)
}

// GetDigest returns the current checksum value.
func (out *IndexOutputWithDigest) GetDigest() uint32 {
	return out.digest.Sum32()
}

// ResetDigest resets the checksum computation.
func (out *IndexOutputWithDigest) ResetDigest() {
	out.digest.Reset()
}

// ErrIO is a generic I/O error.
var ErrIO = errors.New("I/O error")
