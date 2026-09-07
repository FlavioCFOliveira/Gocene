// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"hash"
	"hash/adler32"
	"math"
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
	return out.IndexOutput.WriteBytes(b, 0, len(b))
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
	// BufferedIndexOutput assertions (base class - concrete implementations must provide full interface)
	_ DataOutput     = (*BufferedIndexOutput)(nil)
	_ RandomAccess   = (*BufferedIndexOutput)(nil)
	_ NamedOutput    = (*BufferedIndexOutput)(nil)
	_ Closable       = (*BufferedIndexOutput)(nil)
	_ BufferedOutput = (*BufferedIndexOutput)(nil)
)
