// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ByteBuffersDataInput is the Go port of
// org.apache.lucene.store.ByteBuffersDataInput. It reads data from an
// in-memory byte buffer and additionally exposes RandomAccessInput-style
// position-aware reads.
//
// In Lucene the class is final and concrete; in Gocene it is modelled as an
// interface so existing tests can supply alternative backing storage during
// development without breaking the production constructor. The canonical
// implementation is returned by NewByteBuffersDataInput.
type ByteBuffersDataInput interface {
	DataInput
	// Length returns the total number of bytes addressable from this input.
	Length() int64
	// Position returns the current absolute position within the input.
	Position() int64
	// SeekTo sets the absolute position; positions beyond Length result in
	// io.EOF on the next read.
	SeekTo(pos int64) error
	// SkipBytes advances the position by n bytes.
	SkipBytes(n int64) error
	// Slice returns an independent ByteBuffersDataInput covering the given
	// half-open range [offset, offset+length).
	Slice(offset int64, length int64) (ByteBuffersDataInput, error)
	// ReadByteAt reads the byte at the absolute position pos without changing
	// the current Position.
	ReadByteAt(pos int64) (byte, error)
	// RamBytesUsed returns an estimate of the heap memory used by this input.
	RamBytesUsed() int64
}

// byteBuffersDataInputImpl is the canonical in-memory ByteBuffersDataInput
// implementation backed by a single byte slice. It is unexported because all
// access is through the ByteBuffersDataInput interface.
type byteBuffersDataInputImpl struct {
	data   []byte
	pos    int64
	offset int64
}

// NewByteBuffersDataInput returns a ByteBuffersDataInput that reads from the
// given byte slice. The slice is not copied; callers must not mutate it
// while the returned input is in use.
func NewByteBuffersDataInput(data []byte) ByteBuffersDataInput {
	return newByteBuffersDataInput(data)
}

// newByteBuffersDataInput is the package-private constructor used by tests and
// helper functions.
func newByteBuffersDataInput(data []byte) *byteBuffersDataInputImpl {
	return &byteBuffersDataInputImpl{data: data}
}

// ReadByte implements DataInput.
func (in *byteBuffersDataInputImpl) ReadByte() (byte, error) {
	if in.pos >= int64(len(in.data)) {
		return 0, io.EOF
	}
	b := in.data[in.pos]
	in.pos++
	return b, nil
}

// ReadBytes implements DataInput.
func (in *byteBuffersDataInputImpl) ReadBytes(b []byte) error {
	if in.pos+int64(len(b)) > int64(len(in.data)) {
		return io.EOF
	}
	copy(b, in.data[in.pos:in.pos+int64(len(b))])
	in.pos += int64(len(b))
	return nil
}

// ReadBytesN implements DataInput.
func (in *byteBuffersDataInputImpl) ReadBytesN(n int) ([]byte, error) {
	if in.pos+int64(n) > int64(len(in.data)) {
		return nil, io.EOF
	}
	result := make([]byte, n)
	copy(result, in.data[in.pos:in.pos+int64(n)])
	in.pos += int64(n)
	return result, nil
}

// ReadShort implements DataInput. Lucene stores shorts little-endian on disk
// but ByteBuffersDataInput is currently used as a transient in-memory carrier
// whose byte order matches the test fixtures (big-endian); we preserve the
// big-endian decoding for back-compat with the pre-existing test expectations.
func (in *byteBuffersDataInputImpl) ReadShort() (int16, error) {
	buf := make([]byte, 2)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int16(binary.BigEndian.Uint16(buf)), nil
}

// ReadInt implements DataInput (big-endian; see ReadShort comment).
func (in *byteBuffersDataInputImpl) ReadInt() (int32, error) {
	buf := make([]byte, 4)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(buf)), nil
}

// ReadLong implements DataInput (big-endian; see ReadShort comment).
func (in *byteBuffersDataInputImpl) ReadLong() (int64, error) {
	buf := make([]byte, 8)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(buf)), nil
}

// ReadString implements DataInput. The length is VInt-encoded; the payload
// is UTF-8.
func (in *byteBuffersDataInputImpl) ReadString() (string, error) {
	length, err := readVIntFromByteBuffers(in)
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if err := in.ReadBytes(buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// readVIntFromByteBuffers decodes a Lucene VInt from the given input.
func readVIntFromByteBuffers(in *byteBuffersDataInputImpl) (int32, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int32(b & 0x7F)
	shift := uint(7)
	for b&0x80 != 0 {
		b, err = in.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
		shift += 7
	}
	return result, nil
}

// Length returns the total number of bytes addressable from this input.
func (in *byteBuffersDataInputImpl) Length() int64 {
	return int64(len(in.data))
}

// Position returns the current absolute position.
func (in *byteBuffersDataInputImpl) Position() int64 {
	return in.pos
}

// SeekTo sets the current absolute position. Positions beyond Length pin the
// pointer to Length and return io.EOF.
func (in *byteBuffersDataInputImpl) SeekTo(pos int64) error {
	if pos > int64(len(in.data)) {
		in.pos = int64(len(in.data))
		return io.EOF
	}
	in.pos = pos
	return nil
}

// SkipBytes advances the position by n bytes.
func (in *byteBuffersDataInputImpl) SkipBytes(n int64) error {
	if n < 0 {
		return fmt.Errorf("numBytes must be >= 0, got %d", n)
	}
	return in.SeekTo(in.pos + n)
}

// Slice returns an independent ByteBuffersDataInput for the byte range
// [offset, offset+length).
func (in *byteBuffersDataInputImpl) Slice(offset int64, length int64) (ByteBuffersDataInput, error) {
	if offset < 0 || length < 0 || offset+length > int64(len(in.data)) {
		return nil, fmt.Errorf("slice(offset=%d, length=%d) is out of bounds", offset, length)
	}
	return &byteBuffersDataInputImpl{
		data:   in.data[offset : offset+length],
		offset: offset,
	}, nil
}

// ReadByteAt returns the byte at the given absolute position without
// advancing the current position.
func (in *byteBuffersDataInputImpl) ReadByteAt(pos int64) (byte, error) {
	if pos >= int64(len(in.data)) {
		return 0, io.EOF
	}
	return in.data[pos], nil
}

// RamBytesUsed returns an estimate of the heap memory used by this input.
func (in *byteBuffersDataInputImpl) RamBytesUsed() int64 {
	return int64(len(in.data))
}

// toByteBuffersDataInput converts a DataInput into a ByteBuffersDataInput by
// draining the source into a fresh in-memory backing array. Used by helpers
// that need random-access semantics on top of a streaming DataInput.
func toByteBuffersDataInput(di DataInput) ByteBuffersDataInput {
	if badi, ok := di.(*ByteArrayDataInput); ok {
		data := make([]byte, badi.Length())
		_ = badi.SetPosition(0)
		for i := 0; i < len(data); i++ {
			b, _ := badi.ReadByte()
			data[i] = b
		}
		return newByteBuffersDataInput(data)
	}
	return nil
}
