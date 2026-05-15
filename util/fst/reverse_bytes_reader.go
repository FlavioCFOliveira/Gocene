// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package fst

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ReverseBytesReader reads in reverse from a backing byte slice. It
// is the Go port of Lucene's package-private ReverseBytesReader: the
// position decrements on every ReadByte and ReadBytes call. The
// position starts at the most-recently-written byte (numBytes-1) and
// walks toward 0 as bytes are consumed.
//
// Numeric primitives (ReadShort/ReadInt/ReadLong) decode using the
// same byte order as Lucene's DataInput: big-endian streams of
// bytes whose individual bytes happen to come out in reverse order.
// To preserve compatibility with the byte stream produced by the
// forward writer, the integer parsers assemble the value by reading
// the most-significant byte first.
type ReverseBytesReader struct {
	bytes []byte
	pos   int64
}

// NewReverseBytesReader builds a reverse reader over the given bytes.
// Initial position is len(bytes)-1 so the first ReadByte yields the
// last byte of the slice.
func NewReverseBytesReader(bytes []byte) *ReverseBytesReader {
	return &ReverseBytesReader{bytes: bytes, pos: int64(len(bytes) - 1)}
}

// ReadByte implements DataInput.
func (r *ReverseBytesReader) ReadByte() (byte, error) {
	if r.pos < 0 {
		return 0, io.EOF
	}
	b := r.bytes[r.pos]
	r.pos--
	return b, nil
}

// ReadBytes implements DataInput.
func (r *ReverseBytesReader) ReadBytes(b []byte) error {
	for i := range b {
		v, err := r.ReadByte()
		if err != nil {
			return err
		}
		b[i] = v
	}
	return nil
}

// ReadBytesN implements DataInput.
func (r *ReverseBytesReader) ReadBytesN(n int) ([]byte, error) {
	if n < 0 {
		return nil, errors.New("ReverseBytesReader.ReadBytesN: negative n")
	}
	out := make([]byte, n)
	if err := r.ReadBytes(out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReadShort implements DataInput. Reads two bytes (MSB first in the
// forward stream order, so the first byte we now consume is the LSB
// position; we assemble the int16 with the same bit pattern as the
// original write).
func (r *ReverseBytesReader) ReadShort() (int16, error) {
	if r.pos < 1 {
		return 0, io.EOF
	}
	// Forward stream: [hi, lo]. With pos pointing at the most recent
	// byte (lo), we read lo then hi.
	lo := r.bytes[r.pos]
	r.pos--
	hi := r.bytes[r.pos]
	r.pos--
	return int16(binary.BigEndian.Uint16([]byte{hi, lo})), nil
}

// ReadInt implements DataInput.
func (r *ReverseBytesReader) ReadInt() (int32, error) {
	if r.pos < 3 {
		return 0, io.EOF
	}
	b3 := r.bytes[r.pos]
	r.pos--
	b2 := r.bytes[r.pos]
	r.pos--
	b1 := r.bytes[r.pos]
	r.pos--
	b0 := r.bytes[r.pos]
	r.pos--
	return int32(binary.BigEndian.Uint32([]byte{b0, b1, b2, b3})), nil
}

// ReadLong implements DataInput.
func (r *ReverseBytesReader) ReadLong() (int64, error) {
	if r.pos < 7 {
		return 0, io.EOF
	}
	var raw [8]byte
	for i := 7; i >= 0; i-- {
		raw[i] = r.bytes[r.pos]
		r.pos--
	}
	return int64(binary.BigEndian.Uint64(raw[:])), nil
}

// ReadString is not used by the FST reverse reader.
func (r *ReverseBytesReader) ReadString() (string, error) {
	return "", errors.New("ReverseBytesReader: ReadString not supported")
}

// ReadVInt implements VariableLengthInput. The forward writer emits
// the low-order continuation bytes first; in the reverse byte stream
// the high-order byte (terminator) is encountered first. We mirror
// Lucene's decoder which special-cases the reverse direction.
//
// NOTE: in Lucene the DataInput.readVInt() decodes the stream byte
// order produced by writeVInt, regardless of read direction. Because
// the reverse reader hands out bytes in writer-relative reverse
// order, callers must call ReadByte directly when reading values
// that were written via writeVInt and laid out tail-first.
func (r *ReverseBytesReader) ReadVInt() (int32, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int32(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("ReverseBytesReader: corrupted VInt")
		}
		b, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
	}
	return result, nil
}

// ReadVLong implements VariableLengthInput.
func (r *ReverseBytesReader) ReadVLong() (int64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int64(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("ReverseBytesReader: corrupted VLong")
		}
		b, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
	}
	return result, nil
}

// GetPosition implements BytesReader.
func (r *ReverseBytesReader) GetPosition() int64 { return r.pos }

// SetPosition implements BytesReader.
func (r *ReverseBytesReader) SetPosition(pos int64) { r.pos = pos }

// SkipBytes implements BytesReader. In the reverse direction a
// positive n moves the underlying byte index backward (i.e.
// "forward" in the reverse iteration order). Negative n is
// permitted and moves the underlying index forward; this matches
// Lucene's contract (BitTableUtil.previousBitSet skips by -2).
func (r *ReverseBytesReader) SkipBytes(n int64) error {
	r.pos -= n
	return nil
}
