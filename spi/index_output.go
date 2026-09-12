// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

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
