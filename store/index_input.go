// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
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

	// ReadBytesN reads n bytes into a new slice.
	ReadBytesN(n int) ([]byte, error)
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

// ReadBytesN reads n bytes into a new slice.
func (in *BaseIndexInput) ReadBytesN(n int) ([]byte, error) {
	out := make([]byte, n)
	if err := in.ReadBytes(out, 0, n); err != nil {
		return nil, err
	}
	return out, nil
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


// RandomAccessInput provides random access to read primitive types from an
// IndexInput.
//
// This is the Go port of org.apache.lucene.store.RandomAccessInput. Unlike
// IndexInput, RandomAccessInput has no concept of file position: all reads
// are absolute. Like IndexInput, it is only intended for use by a single
// thread. All numeric reads use little-endian byte order to match the Lucene
// on-disk format.
type RandomAccessInput interface {
	// Length returns the number of bytes in the underlying file.
	Length() int64

	// ReadByteAt reads a byte at the given absolute position.
	ReadByteAt(pos int64) (byte, error)

	// ReadShortAt reads a 16-bit little-endian value at the given absolute
	// position.
	ReadShortAt(pos int64) (int16, error)

	// ReadIntAt reads a 32-bit little-endian value at the given absolute
	// position.
	ReadIntAt(pos int64) (int32, error)

	// ReadLongAt reads a 64-bit little-endian value at the given absolute
	// position.
	ReadLongAt(pos int64) (int64, error)
}

// PrefetchableRandomAccessInput is an optional capability that a
// RandomAccessInput may implement to advertise prefetching support. Callers
// should type-assert to it before calling Prefetch.
//
// Mirrors RandomAccessInput.prefetch in Lucene 10.4.0 which defaults to a
// no-op; Gocene exposes it as an optional interface to keep the base
// RandomAccessInput interface minimal.
type PrefetchableRandomAccessInput interface {
	Prefetch(offset int64, length int64) error
}

// LoadedReporterRandomAccessInput is an optional capability that a
// RandomAccessInput may implement to expose whether its data is currently
// resident in physical memory. Returns (isLoaded, true) when the answer is
// known, or (false, false) otherwise.
//
// Mirrors RandomAccessInput.isLoaded() in Lucene 10.4.0 which returns an
// Optional<Boolean>; the (bool, bool) return value is the idiomatic Go shape.
type LoadedReporterRandomAccessInput interface {
	IsLoaded() (loaded bool, known bool)
}


// Compile-time interface assertions
// These ensure that implementations properly satisfy the segregated interfaces
var (
	// ByteArrayDataInput assertions
	_ DataInput           = (*ByteArrayDataInput)(nil)
	_ VariableLengthInput = (*ByteArrayDataInput)(nil)
)
