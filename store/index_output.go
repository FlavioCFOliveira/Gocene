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
	"math"
	"sort"
)

// NamedOutput provides access to the file name for index outputs.
// This is a segregated interface for components that have an associated name.
type NamedOutput interface {
	// GetName returns the name of the file being written.
	GetName() string
}

// IndexOutput provides random access to write index files.
//
// IndexOutput is the abstract base class for writing index files.
// It provides methods for writing primitive types (byte, int, long, etc.)
// and arbitrary byte arrays. All writes are byte-aligned.
//
// This is the Go port of Lucene's org.apache.lucene.store.IndexOutput.
// This interface is composed of smaller, focused interfaces following
// the Interface Segregation Principle.
type IndexOutput interface {
	// DataOutput provides basic write operations
	DataOutput

	// RandomAccess provides position-aware operations
	RandomAccess

	// NamedOutput provides access to the file name
	NamedOutput

	// Closable provides resource cleanup
	Closable
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

	// WriteShort writes a 16-bit value.
	WriteShort(i int16) error

	// WriteInt writes a 32-bit value.
	WriteInt(i int32) error

	// WriteLong writes a 64-bit value.
	WriteLong(i int64) error

	// WriteString writes a string.
	WriteString(s string) error
}

// VariableLengthOutput provides methods for writing variable-length encoded data.
// This is a segregated interface for components that need VInt/VLong support.
type VariableLengthOutput interface {
	// WriteVInt writes a variable-length integer (up to 5 bytes).
	// This is Lucene's variable-length integer encoding.
	WriteVInt(i int32) error

	// WriteVLong writes a variable-length long (up to 9 bytes).
	WriteVLong(i int64) error
}

// BufferedOutput provides buffer management operations for buffered IndexOutput implementations.
// This is a segregated interface for components that use buffering.
type BufferedOutput interface {
	// Flush flushes any buffered bytes to the underlying output.
	Flush() error

	// GetBufferSize returns the current buffer size.
	GetBufferSize() int

	// SetBufferSize changes the buffer size.
	SetBufferSize(size int) error
}

// Ensure interfaces are properly defined
var (
	_ DataOutput            = (*ByteArrayDataOutput)(nil)
	_ VariableLengthOutput  = (*ByteArrayDataOutput)(nil)
	_ BufferedOutput        = (*BufferedIndexOutput)(nil)
)

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

// WriteShort writes a 16-bit value.
func (out *ByteArrayDataOutput) WriteShort(i int16) error {
	b := []byte{byte(i >> 8), byte(i)}
	return out.WriteBytes(b)
}

// WriteInt writes a 32-bit value.
func (out *ByteArrayDataOutput) WriteInt(i int32) error {
	b := []byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)}
	return out.WriteBytes(b)
}

// WriteLong writes a 64-bit value.
func (out *ByteArrayDataOutput) WriteLong(i int64) error {
	b := []byte{
		byte(i >> 56), byte(i >> 48), byte(i >> 40), byte(i >> 32),
		byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i),
	}
	return out.WriteBytes(b)
}

// WriteString writes a string.
func (out *ByteArrayDataOutput) WriteString(s string) error {
	return WriteString(out, s)
}

// WriteVInt writes a variable-length integer.
func (out *ByteArrayDataOutput) WriteVInt(i int32) error {
	for (i & ^int32(0x7F)) != 0 {
		if err := out.WriteByte(byte((i & 0x7F) | 0x80)); err != nil {
			return err
		}
		i >>= 7
	}
	return out.WriteByte(byte(i))
}

// WriteVLong writes a variable-length long.
func (out *ByteArrayDataOutput) WriteVLong(i int64) error {
	for (i & ^int64(0x7F)) != 0 {
		if err := out.WriteByte(byte((i & 0x7F) | 0x80)); err != nil {
			return err
		}
		i >>= 7
	}
	return out.WriteByte(byte(i))
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

// WriteShort writes a 16-bit value.
func (out *BufferedIndexOutput) WriteShort(i int16) error {
	b := []byte{byte(i >> 8), byte(i)}
	return out.WriteBytes(b)
}

// WriteInt writes a 32-bit value.
func (out *BufferedIndexOutput) WriteInt(i int32) error {
	b := []byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)}
	return out.WriteBytes(b)
}

// WriteLong writes a 64-bit value.
func (out *BufferedIndexOutput) WriteLong(i int64) error {
	b := []byte{
		byte(i >> 56), byte(i >> 48), byte(i >> 40), byte(i >> 32),
		byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i),
	}
	return out.WriteBytes(b)
}

// WriteString writes a string.
func (out *BufferedIndexOutput) WriteString(s string) error {
	return WriteString(out, s)
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

// Length returns the total length of the file written so far.
func (out *BufferedIndexOutput) Length() int64 {
	return out.GetFilePointer() + int64(out.bufferPosition)
}

// SetPosition sets the current position for writing.
// Note: This is not fully supported for BufferedIndexOutput and returns an error.
func (out *BufferedIndexOutput) SetPosition(pos int64) error {
	return fmt.Errorf("SetPosition not supported for BufferedIndexOutput")
}

// WriteVInt writes a variable-length integer.
func (out *BufferedIndexOutput) WriteVInt(i int32) error {
	return WriteVInt(out, i)
}

// WriteVLong writes a variable-length long.
func (out *BufferedIndexOutput) WriteVLong(i int64) error {
	return WriteVLong(out, i)
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

// WriteMapOfStrings writes a map of strings as a VInt size followed by key-value pairs.
func WriteMapOfStrings(out DataOutput, m map[string]string) error {
	if err := WriteVInt(out, int32(len(m))); err != nil {
		return err
	}
	for k, v := range m {
		if err := WriteString(out, k); err != nil {
			return err
		}
		if err := WriteString(out, v); err != nil {
			return err
		}
	}
	return nil
}

// WriteSetOfStrings writes a set of strings as a VInt size followed by strings.
func WriteSetOfStrings(out DataOutput, s map[string]struct{}) error {
	if err := WriteVInt(out, int32(len(s))); err != nil {
		return err
	}
	// Sort for deterministic output
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := WriteString(out, k); err != nil {
			return err
		}
	}
	return nil
}

// WriteMapOfIntToSetOfStrings writes a map of int to set of strings.
func WriteMapOfIntToSetOfStrings(out DataOutput, m map[int]map[string]struct{}) error {
	if err := WriteVInt(out, int32(len(m))); err != nil {
		return err
	}
	// Sort for deterministic output
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		if err := WriteVInt(out, int32(k)); err != nil {
			return err
		}
		if err := WriteSetOfStrings(out, m[k]); err != nil {
			return err
		}
	}
	return nil
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

// AlignOffset aligns the given offset to multiples of alignmentBytes bytes by rounding up.
// The alignment must be a power of 2.
// This is the Go equivalent of Lucene's IndexOutput.alignOffset().
//
// Parameters:
//   - offset: the offset to align (must be non-negative)
//   - alignmentBytes: the alignment boundary (must be a positive power of 2)
//
// Returns:
//   - the aligned offset
//   - error if offset is negative or alignmentBytes is not a power of 2
func AlignOffset(offset int64, alignmentBytes int) (int64, error) {
	if offset < 0 {
		return 0, errors.New("offset must be non-negative")
	}
	if alignmentBytes <= 0 || (alignmentBytes&(alignmentBytes-1)) != 0 {
		return 0, errors.New("alignment must be a positive power of 2")
	}
	// Check for overflow: offset - 1 + alignmentBytes could overflow
	if offset > 0 && offset-1 > math.MaxInt64-int64(alignmentBytes) {
		return 0, errors.New("arithmetic overflow in alignOffset")
	}
	// Formula: ((offset - 1) + alignmentBytes) & (-alignmentBytes)
	// This rounds up to the next multiple of alignmentBytes
	return (offset - 1 + int64(alignmentBytes)) & (-int64(alignmentBytes)), nil
}

// AlignFilePointer aligns the current file pointer to multiples of alignmentBytes bytes
// by writing zero bytes. This improves reads with memory-mapped I/O.
// The alignment must be a power of 2.
// This is the Go equivalent of Lucene's IndexOutput.alignFilePointer().
//
// Parameters:
//   - out: the IndexOutput to align
//   - alignmentBytes: the alignment boundary (must be a positive power of 2)
//
// Returns:
//   - the new file pointer after alignment
//   - error if alignment fails
func AlignFilePointer(out IndexOutput, alignmentBytes int) (int64, error) {
	offset := out.GetFilePointer()
	alignedOffset, err := AlignOffset(offset, alignmentBytes)
	if err != nil {
		return 0, err
	}
	count := int(alignedOffset - offset)
	for i := 0; i < count; i++ {
		if err := out.WriteByte(0); err != nil {
			return 0, err
		}
	}
	return alignedOffset, nil
}

// Compile-time interface assertions
// These ensure that implementations properly satisfy the segregated interfaces
var (
	// ByteArrayDataOutput assertions
	_ DataOutput           = (*ByteArrayDataOutput)(nil)
	_ VariableLengthOutput = (*ByteArrayDataOutput)(nil)

	// BufferedIndexOutput assertions (base class - concrete implementations must provide full interface)
	_ DataOutput           = (*BufferedIndexOutput)(nil)
	_ RandomAccess         = (*BufferedIndexOutput)(nil)
	_ NamedOutput          = (*BufferedIndexOutput)(nil)
	_ Closable             = (*BufferedIndexOutput)(nil)
	_ BufferedOutput       = (*BufferedIndexOutput)(nil)
)
