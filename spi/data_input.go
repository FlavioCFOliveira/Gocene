// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// DataInput is a port of org.apache.lucene.store.DataInput.
// It defines the interface for performing read operations of Lucene's low-level data types.
//
// DataInput may only be used from one thread, because it is not thread safe (it keeps
// internal state like file position). To allow multithreaded use, every DataInput instance
// must be cloned before used in another thread.
type DataInput interface {
	// ReadByte reads and returns a single byte.
	ReadByte() (byte, error)

	// ReadBytes reads a specified number of bytes into an array at the specified offset.
	ReadBytes(b []byte, offset, len int) error

	// ReadShort reads two bytes and returns a short (LE byte order).
	ReadShort() (int16, error)

	// ReadInt reads four bytes and returns an int (LE byte order).
	ReadInt() (int32, error)

	// ReadVInt reads an int stored in variable-length format.
	ReadVInt() (int32, error)

	// ReadZInt reads a zig-zag-encoded variable-length integer.
	ReadZInt() (int32, error)

	// ReadLong reads eight bytes and returns a long (LE byte order).
	ReadLong() (int64, error)

	// ReadLongs reads a specified number of longs.
	ReadLongs(dst []int64, offset, length int) error

	// ReadInts reads a specified number of ints into an array at the specified offset.
	ReadInts(dst []int32, offset, length int) error

	// ReadFloats reads a specified number of floats into an array at the specified offset.
	ReadFloats(dst []float32, offset, len int) error

	// ReadVLong reads a long stored in variable-length format.
	ReadVLong() (int64, error)

	// ReadZLong reads a zig-zag-encoded variable-length long.
	ReadZLong() (int64, error)

	// ReadString reads a string.
	ReadString() (string, error)

	// ReadMapOfStrings reads a map[string]string previously written.
	ReadMapOfStrings() (map[string]string, error)

	// ReadSetOfStrings reads a set of strings previously written.
	ReadSetOfStrings() ([]string, error)

	// SkipBytes skips over numBytes bytes.
	SkipBytes(int64) error
}

// DataInputCore represents the methods that must be implemented by the underlying storage.
type DataInputCore interface {
	ReadByte() (byte, error)
	ReadBytes(b []byte, offset, len int) error
	SkipBytes(numBytes int64) error
}
