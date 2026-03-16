// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// RandomAccess provides position-aware operations for index files.
// This is a segregated interface for components that need position tracking.
type RandomAccess interface {
	// GetFilePointer returns the current position in the file.
	GetFilePointer() int64

	// SetPosition changes the current position in the file.
	// The next read will occur at the new position.
	// This is the Lucene Seek() equivalent; renamed to avoid Go vet warnings
	// about io.Seeker interface mismatch.
	SetPosition(pos int64) error

	// Length returns the total length of the file in bytes.
	Length() int64
}

// Slicable provides the ability to create subsets of an IndexInput.
// This is a segregated interface for components that support slicing.
type Slicable interface {
	// Slice returns a subset of this IndexInput starting at
	// the given offset with the specified length.
	// The returned IndexInput is independent of this one.
	Slice(desc string, offset int64, length int64) (IndexInput, error)
}

// Closable provides the ability to release resources.
// This is a segregated interface for resource cleanup.
type Closable interface {
	// Close closes this resource, releasing any resources.
	Close() error
}

// Cloneable provides the ability to create independent copies.
// This is a segregated interface for components that support cloning.
type Cloneable interface {
	// Clone returns a clone of this IndexInput.
	// The clone initially shares the same file position, but
	// subsequent reads/closes on either the original or clone
	// do not affect the other.
	Clone() IndexInput
}

// IndexInput provides random access to an index file.
//
// IndexInput is the abstract base class for reading index files.
// It provides methods for reading primitive types (byte, int, long, etc.)
// and arbitrary byte arrays. All reads are byte-aligned.
//
// This is the Go port of Lucene's org.apache.lucene.store.IndexInput.
// This interface is composed of smaller, focused interfaces following
// the Interface Segregation Principle.
type IndexInput interface {
	// DataInput provides basic read operations
	DataInput

	// RandomAccess provides position-aware operations
	RandomAccess

	// Slicable provides the ability to create subsets
	Slicable

	// Cloneable provides the ability to create independent copies
	Cloneable

	// Closable provides resource cleanup
	Closable
}

// DataInput defines the interface for reading basic data types.
// This is embedded in IndexInput and other read interfaces.
type DataInput interface {
	// ReadByte reads a single byte.
	// Returns io.EOF if at end of file.
	ReadByte() (byte, error)

	// ReadBytes reads len(b) bytes into b.
	// Returns io.EOF if end of file is reached before reading len(b) bytes.
	ReadBytes(b []byte) error

	// ReadBytesN reads exactly n bytes and returns them.
	// Returns io.EOF if end of file is reached before reading n bytes.
	ReadBytesN(n int) ([]byte, error)

	// ReadShort reads a 16-bit value.
	ReadShort() (int16, error)

	// ReadInt reads a 32-bit value.
	ReadInt() (int32, error)

	// ReadLong reads a 64-bit value.
	ReadLong() (int64, error)

	// ReadString reads a string.
	ReadString() (string, error)
}

// VariableLengthInput provides methods for reading variable-length encoded data.
// This is a segregated interface for components that need VInt/VLong support.
type VariableLengthInput interface {
	// ReadVInt reads a variable-length integer (up to 5 bytes).
	// This is Lucene's variable-length integer encoding.
	ReadVInt() (int32, error)

	// ReadVLong reads a variable-length long (up to 9 bytes).
	ReadVLong() (int64, error)
}

// BufferedInput provides buffer management operations for buffered IndexInput implementations.
// This is a segregated interface for components that use buffering.
type BufferedInput interface {
	// GetBufferSize returns the current buffer size.
	GetBufferSize() int

	// SetBufferSize changes the buffer size.
	SetBufferSize(size int)
}

// Ensure interfaces are properly defined
var (
	_ DataInput           = (*ByteArrayDataInput)(nil)
	_ VariableLengthInput = (*ByteArrayDataInput)(nil)
	_ BufferedInput       = (*BufferedIndexInput)(nil)
)

// BaseIndexInput provides common functionality for IndexInput implementations.
// Embed this struct in concrete IndexInput implementations.
type BaseIndexInput struct {
	// desc is a description of this IndexInput (for debugging/messages)
	desc string

	// length is the total length of the input
	length int64

	// filePointer is the current position in the input
	filePointer int64
}

// NewBaseIndexInput creates a new BaseIndexInput.
func NewBaseIndexInput(desc string, length int64) *BaseIndexInput {
	return &BaseIndexInput{
		desc:        desc,
		length:      length,
		filePointer: 0,
	}
}

// GetDescription returns the description of this IndexInput.
func (in *BaseIndexInput) GetDescription() string {
	return in.desc
}

// GetFilePointer returns the current position in the input.
func (in *BaseIndexInput) GetFilePointer() int64 {
	return in.filePointer
}

// SetFilePointer sets the current position in the input.
func (in *BaseIndexInput) SetFilePointer(pos int64) {
	in.filePointer = pos
}

// Length returns the total length of the input.
func (in *BaseIndexInput) Length() int64 {
	return in.length
}

// ValidateSeek checks if a seek position is valid.
func (in *BaseIndexInput) ValidateSeek(pos int64) error {
	if pos < 0 {
		return fmt.Errorf("seek position %d is negative", pos)
	}
	if pos > in.length {
		return fmt.Errorf("seek position %d exceeds length %d", pos, in.length)
	}
	return nil
}

// ValidateSlice checks if a slice specification is valid.
func (in *BaseIndexInput) ValidateSlice(offset int64, length int64) error {
	if offset < 0 {
		return fmt.Errorf("slice offset %d is negative", offset)
	}
	if length < 0 {
		return fmt.Errorf("slice length %d is negative", length)
	}
	if offset+length > in.length {
		return fmt.Errorf("slice offset %d + length %d exceeds input length %d", offset, length, in.length)
	}
	return nil
}

// SkipBytes skips n bytes forward in the input.
func (in *BaseIndexInput) SkipBytes(n int64) error {
	if n < 0 {
		return errors.New("cannot skip negative bytes")
	}
	newPos := in.filePointer + n
	if newPos > in.length {
		return fmt.Errorf("cannot skip %d bytes past end of file", n)
	}
	in.filePointer = newPos
	return nil
}

// ByteArrayDataInput implements DataInput over a byte slice.
// This is useful for reading from in-memory buffers.
type ByteArrayDataInput struct {
	bytes []byte
	pos   int
}

// NewByteArrayDataInput creates a new ByteArrayDataInput from the given bytes.
func NewByteArrayDataInput(bytes []byte) *ByteArrayDataInput {
	return &ByteArrayDataInput{
		bytes: bytes,
		pos:   0,
	}
}

// ReadByte reads a single byte.
func (in *ByteArrayDataInput) ReadByte() (byte, error) {
	if in.pos >= len(in.bytes) {
		return 0, io.EOF
	}
	b := in.bytes[in.pos]
	in.pos++
	return b, nil
}

// ReadBytes reads len(b) bytes into b.
func (in *ByteArrayDataInput) ReadBytes(b []byte) error {
	if in.pos+len(b) > len(in.bytes) {
		return io.EOF
	}
	copy(b, in.bytes[in.pos:in.pos+len(b)])
	in.pos += len(b)
	return nil
}

// ReadBytesN reads exactly n bytes.
func (in *ByteArrayDataInput) ReadBytesN(n int) ([]byte, error) {
	if in.pos+n > len(in.bytes) {
		return nil, io.EOF
	}
	result := make([]byte, n)
	copy(result, in.bytes[in.pos:in.pos+n])
	in.pos += n
	return result, nil
}

// ReadShort reads a 16-bit value.
func (in *ByteArrayDataInput) ReadShort() (int16, error) {
	if in.pos+2 > len(in.bytes) {
		return 0, io.EOF
	}
	v := binary.LittleEndian.Uint16(in.bytes[in.pos:])
	in.pos += 2
	return int16(v), nil
}

// ReadInt reads a 32-bit value.
func (in *ByteArrayDataInput) ReadInt() (int32, error) {
	if in.pos+4 > len(in.bytes) {
		return 0, io.EOF
	}
	v := binary.LittleEndian.Uint32(in.bytes[in.pos:])
	in.pos += 4
	return int32(v), nil
}

// ReadLong reads a 64-bit value.
func (in *ByteArrayDataInput) ReadLong() (int64, error) {
	if in.pos+8 > len(in.bytes) {
		return 0, io.EOF
	}
	v := binary.LittleEndian.Uint64(in.bytes[in.pos:])
	in.pos += 8
	return int64(v), nil
}

// ReadString reads a string.
func (in *ByteArrayDataInput) ReadString() (string, error) {
	length, err := in.ReadVInt()
	if err != nil {
		return "", err
	}
	if in.pos+int(length) > len(in.bytes) {
		return "", io.EOF
	}
	s := string(in.bytes[in.pos : in.pos+int(length)])
	in.pos += int(length)
	return s, nil
}

// ReadVInt reads a variable-length integer.
func (in *ByteArrayDataInput) ReadVInt() (int32, error) {
	var result int32
	shift := 0
	for {
		if in.pos >= len(in.bytes) {
			return 0, io.EOF
		}
		b := in.bytes[in.pos]
		in.pos++
		result |= int32(b&0x7F) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("corrupted VInt")
		}
	}
	return result, nil
}

// ReadVLong reads a variable-length long.
func (in *ByteArrayDataInput) ReadVLong() (int64, error) {
	var result int64
	shift := 0
	for {
		if in.pos >= len(in.bytes) {
			return 0, io.EOF
		}
		b := in.bytes[in.pos]
		in.pos++
		result |= int64(b&0x7F) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("corrupted VLong")
		}
	}
	return result, nil
}

// GetPosition returns the current position.
func (in *ByteArrayDataInput) GetPosition() int {
	return in.pos
}

// SetPosition sets the current position.
func (in *ByteArrayDataInput) SetPosition(pos int) error {
	if pos < 0 || pos > len(in.bytes) {
		return fmt.Errorf("position %d out of range [0, %d]", pos, len(in.bytes))
	}
	in.pos = pos
	return nil
}

// Length returns the total length.
func (in *ByteArrayDataInput) Length() int {
	return len(in.bytes)
}

// Reset resets the input to read from new bytes.
func (in *ByteArrayDataInput) Reset(bytes []byte) {
	in.bytes = bytes
	in.pos = 0
}

// ResetWithSlice resets the input to read from a slice of bytes.
// This is the Go equivalent of Java's reset(bytes, offset, len).
func (in *ByteArrayDataInput) ResetWithSlice(bytes []byte, offset, length int) {
	in.bytes = bytes[offset : offset+length]
	in.pos = 0
}

// EOF returns true if the end of the input has been reached.
func (in *ByteArrayDataInput) EOF() bool {
	return in.pos >= len(in.bytes)
}

// BufferedIndexInput is a base class for buffered IndexInput implementations.
// It provides buffering for small reads while allowing direct access for large reads.
type BufferedIndexInput struct {
	*BaseIndexInput

	// buffer is the read buffer
	buffer []byte

	// bufferStart is the position in the file where the buffer starts
	bufferStart int64

	// bufferLength is the number of valid bytes in the buffer
	bufferLength int

	// bufferPosition is the current position within the buffer
	bufferPosition int
}

// NewBufferedIndexInput creates a new BufferedIndexInput.
// If bufferSize is <= 0, a default size of 1024 is used.
func NewBufferedIndexInput(desc string, length int64, bufferSize int) *BufferedIndexInput {
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	return &BufferedIndexInput{
		BaseIndexInput: NewBaseIndexInput(desc, length),
		buffer:         make([]byte, bufferSize),
		bufferStart:    0,
		bufferLength:   0,
		bufferPosition: 0,
	}
}

// ReadByte reads a single byte, using the buffer when possible.
func (in *BufferedIndexInput) ReadByte() (byte, error) {
	if in.bufferPosition >= in.bufferLength {
		if err := in.refill(); err != nil {
			return 0, err
		}
		if in.bufferLength == 0 {
			return 0, io.EOF
		}
	}
	b := in.buffer[in.bufferPosition]
	in.bufferPosition++
	in.SetFilePointer(in.GetFilePointer() + 1)
	return b, nil
}

// ReadBytes reads len(b) bytes into b.
func (in *BufferedIndexInput) ReadBytes(b []byte) error {
	// If the read is larger than the buffer, bypass the buffer
	if len(b) > len(in.buffer) {
		// First flush any buffered bytes
		if in.bufferPosition < in.bufferLength {
			n := in.bufferLength - in.bufferPosition
			if n > len(b) {
				n = len(b)
			}
			copy(b[:n], in.buffer[in.bufferPosition:in.bufferPosition+n])
			in.bufferPosition += n
			in.SetFilePointer(in.GetFilePointer() + int64(n))
			b = b[n:]
			if len(b) == 0 {
				return nil
			}
		}

		// Read directly from the source
		n, err := in.readInternal(b)
		if err != nil {
			return err
		}
		in.SetFilePointer(in.GetFilePointer() + int64(n))
		return nil
	}

	// Use the buffer for smaller reads
	for len(b) > 0 {
		if in.bufferPosition >= in.bufferLength {
			if err := in.refill(); err != nil {
				return err
			}
			if in.bufferLength == 0 {
				return io.EOF
			}
		}
		n := in.bufferLength - in.bufferPosition
		if n > len(b) {
			n = len(b)
		}
		copy(b[:n], in.buffer[in.bufferPosition:in.bufferPosition+n])
		in.bufferPosition += n
		in.SetFilePointer(in.GetFilePointer() + int64(n))
		b = b[n:]
	}
	return nil
}

// ReadBytesN reads exactly n bytes.
func (in *BufferedIndexInput) ReadBytesN(n int) ([]byte, error) {
	result := make([]byte, n)
	if err := in.ReadBytes(result); err != nil {
		return nil, err
	}
	return result, nil
}

// ReadShort reads a 16-bit value.
func (in *BufferedIndexInput) ReadShort() (int16, error) {
	buf := make([]byte, 2)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf)), nil
}

// ReadInt reads a 32-bit value.
func (in *BufferedIndexInput) ReadInt() (int32, error) {
	buf := make([]byte, 4)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf)), nil
}

// ReadLong reads a 64-bit value.
func (in *BufferedIndexInput) ReadLong() (int64, error) {
	buf := make([]byte, 8)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf)), nil
}

// ReadString reads a string.
func (in *BufferedIndexInput) ReadString() (string, error) {
	length, err := in.ReadVInt()
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if err := in.ReadBytes(buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// ReadVInt reads a variable-length integer.
func (in *BufferedIndexInput) ReadVInt() (int32, error) {
	var result int32
	shift := 0
	for {
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("corrupted VInt")
		}
	}
	return result, nil
}

// SetPosition changes the current position.
// This is the Lucene Seek() equivalent.
func (in *BufferedIndexInput) SetPosition(pos int64) error {
	if err := in.ValidateSeek(pos); err != nil {
		return err
	}

	// Check if the seek is within our buffer
	if pos >= in.bufferStart && pos < in.bufferStart+int64(in.bufferLength) {
		in.bufferPosition = int(pos - in.bufferStart)
	} else {
		// Seek is outside buffer, invalidate it
		in.bufferStart = pos
		in.bufferLength = 0
		in.bufferPosition = 0
	}
	in.SetFilePointer(pos)
	return nil
}

// refill fills the buffer from the underlying source.
// This must be implemented by subclasses or the file parameter.
func (in *BufferedIndexInput) refill() error {
	start := in.GetFilePointer()
	n, err := in.readInternal(in.buffer)
	if err != nil {
		return err
	}
	in.bufferStart = start
	in.bufferLength = n
	in.bufferPosition = 0
	return nil
}

// readInternal reads directly from the source into b.
// Subclasses must override this method.
func (in *BufferedIndexInput) readInternal(b []byte) (int, error) {
	return 0, errors.New("readInternal not implemented")
}

// GetBufferSize returns the buffer size.
func (in *BufferedIndexInput) GetBufferSize() int {
	return len(in.buffer)
}

// SetBufferSize changes the buffer size.
func (in *BufferedIndexInput) SetBufferSize(size int) {
	if size <= 0 {
		size = 1024
	}
	in.buffer = make([]byte, size)
	in.bufferLength = 0
	in.bufferPosition = 0
}

// ReadVLong reads a variable-length long.
func (in *BufferedIndexInput) ReadVLong() (int64, error) {
	var result int64
	shift := 0
	for {
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("corrupted VLong")
		}
	}
	return result, nil
}

// Slice returns a subset of this IndexInput.
func (in *BufferedIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	return nil, fmt.Errorf("Slice not implemented for BufferedIndexInput")
}

// Clone returns a clone of this IndexInput.
func (in *BufferedIndexInput) Clone() IndexInput {
	return nil
}

// Close closes this IndexInput.
func (in *BufferedIndexInput) Close() error {
	return nil
}

// ReadUint16 reads a 16-bit unsigned integer in big-endian format.
func ReadUint16(in DataInput) (uint16, error) {
	b, err := in.ReadBytesN(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
}

// ReadUint32 reads a 32-bit unsigned integer in big-endian format.
func ReadUint32(in DataInput) (uint32, error) {
	b, err := in.ReadBytesN(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b), nil
}

// ReadUint64 reads a 64-bit unsigned integer in big-endian format.
func ReadUint64(in DataInput) (uint64, error) {
	b, err := in.ReadBytesN(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b), nil
}

// ReadInt16 reads a 16-bit signed integer in big-endian format.
func ReadInt16(in DataInput) (int16, error) {
	v, err := ReadUint16(in)
	if err != nil {
		return 0, err
	}
	return int16(v), nil
}

// ReadInt32 reads a 32-bit signed integer in big-endian format.
func ReadInt32(in DataInput) (int32, error) {
	v, err := ReadUint32(in)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

// ReadInt64 reads a 64-bit signed integer in big-endian format.
func ReadInt64(in DataInput) (int64, error) {
	v, err := ReadUint64(in)
	if err != nil {
		return 0, err
	}
	return int64(v), nil
}

// ReadVInt reads a variable-length integer (up to 5 bytes).
// This is Lucene's variable-length integer encoding.
func ReadVInt(in DataInput) (int32, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	var i int32 = int32(b) & 0x7F
	for shift := 7; (b & 0x80) != 0; shift += 7 {
		b, err = in.ReadByte()
		if err != nil {
			return 0, err
		}
		i |= int32(b&0x7F) << shift
	}
	return i, nil
}

// ReadVLong reads a variable-length long (up to 9 bytes).
func ReadVLong(in DataInput) (int64, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	var i int64 = int64(b) & 0x7F
	for shift := 7; (b & 0x80) != 0; shift += 7 {
		b, err = in.ReadByte()
		if err != nil {
			return 0, err
		}
		i |= int64(b&0x7F) << shift
	}
	return i, nil
}

// ReadString reads a string written by writeString (length-prefixed UTF-8).
func ReadString(in DataInput) (string, error) {
	length, err := ReadVInt(in)
	if err != nil {
		return "", err
	}
	if length < 0 {
		return "", errors.New("negative string length")
	}
	if length == 0 {
		return "", nil
	}
	bytes, err := in.ReadBytesN(int(length))
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ReadMapOfStrings reads a map of strings written by WriteMapOfStrings.
func ReadMapOfStrings(in DataInput) (map[string]string, error) {
	size, err := ReadVInt(in)
	if err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, errors.New("negative map size")
	}
	if size == 0 {
		return nil, nil
	}
	m := make(map[string]string, size)
	for i := int32(0); i < size; i++ {
		key, err := ReadString(in)
		if err != nil {
			return nil, err
		}
		value, err := ReadString(in)
		if err != nil {
			return nil, err
		}
		m[key] = value
	}
	return m, nil
}

// ReadSetOfStrings reads a set of strings written by WriteSetOfStrings.
func ReadSetOfStrings(in DataInput) (map[string]struct{}, error) {
	size, err := ReadVInt(in)
	if err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, errors.New("negative set size")
	}
	if size == 0 {
		return nil, nil
	}
	s := make(map[string]struct{}, size)
	for i := int32(0); i < size; i++ {
		str, err := ReadString(in)
		if err != nil {
			return nil, err
		}
		s[str] = struct{}{}
	}
	return s, nil
}

// RandomAccessInput provides random access to read primitive types from an IndexInput.
// This is the Go port of Lucene's org.apache.lucene.store.RandomAccessInput.
type RandomAccessInput interface {
	// ReadByteAt reads a single byte at the given position.
	ReadByteAt(pos int64) (byte, error)

	// ReadLongAt reads a 64-bit value at the given position in big-endian format.
	ReadLongAt(pos int64) (int64, error)
}

// ReadMapOfIntToSetOfStrings reads a map of int to set of strings written by WriteMapOfIntToSetOfStrings.
func ReadMapOfIntToSetOfStrings(in DataInput) (map[int]map[string]struct{}, error) {
	size, err := ReadVInt(in)
	if err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, errors.New("negative map size")
	}
	if size == 0 {
		return nil, nil
	}
	m := make(map[int]map[string]struct{}, size)
	for i := int32(0); i < size; i++ {
		key, err := ReadVInt(in)
		if err != nil {
			return nil, err
		}
		value, err := ReadSetOfStrings(in)
		if err != nil {
			return nil, err
		}
		m[int(key)] = value
	}
	return m, nil
}

// Compile-time interface assertions
// These ensure that implementations properly satisfy the segregated interfaces
var (
	// ByteArrayDataInput assertions
	_ DataInput           = (*ByteArrayDataInput)(nil)
	_ VariableLengthInput = (*ByteArrayDataInput)(nil)

	// BufferedIndexInput assertions (base class - concrete implementations must provide Clone, Slice)
	_ DataInput           = (*BufferedIndexInput)(nil)
	_ VariableLengthInput = (*BufferedIndexInput)(nil)
	_ RandomAccess        = (*BufferedIndexInput)(nil)
	_ BufferedInput       = (*BufferedIndexInput)(nil)
)
