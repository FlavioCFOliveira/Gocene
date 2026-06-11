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

// ForwardBytesReader is the Go port of the package-private
// org.apache.lucene.util.fst.ForwardBytesReader. It reads forward from
// a single byte slice. Lucene only uses this reader in test code that
// inspects the FST byte stream in writer order; Gocene exposes it for
// the same diagnostic purpose.
//
// Integer primitives use little-endian decoding to mirror Lucene's
// canonical DataInput byte order (versionLittleEndian onward).
type ForwardBytesReader struct {
	bytes []byte
	pos   int
}

// NewForwardBytesReader builds a forward reader over bytes. Initial
// position is 0.
func NewForwardBytesReader(bytes []byte) *ForwardBytesReader {
	return &ForwardBytesReader{bytes: bytes}
}

// ReadByte implements store.DataInput.
func (r *ForwardBytesReader) ReadByte() (byte, error) {
	if r.pos >= len(r.bytes) {
		return 0, io.EOF
	}
	b := r.bytes[r.pos]
	r.pos++
	return b, nil
}

// ReadBytes implements store.DataInput.
func (r *ForwardBytesReader) ReadBytes(b []byte) error {
	if r.pos+len(b) > len(r.bytes) {
		return io.EOF
	}
	copy(b, r.bytes[r.pos:r.pos+len(b)])
	r.pos += len(b)
	return nil
}

// ReadBytesN implements store.DataInput.
func (r *ForwardBytesReader) ReadBytesN(n int) ([]byte, error) {
	if n < 0 {
		return nil, errors.New("ForwardBytesReader.ReadBytesN: negative n")
	}
	out := make([]byte, n)
	if err := r.ReadBytes(out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReadShort reads a little-endian int16, matching the canonical
// Lucene 9+ on-disk encoding.
func (r *ForwardBytesReader) ReadShort() (int16, error) {
	if r.pos+2 > len(r.bytes) {
		return 0, io.EOF
	}
	v := int16(binary.LittleEndian.Uint16(r.bytes[r.pos:]))
	r.pos += 2
	return v, nil
}

// ReadInt reads a little-endian int32.
func (r *ForwardBytesReader) ReadInt() (int32, error) {
	if r.pos+4 > len(r.bytes) {
		return 0, io.EOF
	}
	v := int32(binary.LittleEndian.Uint32(r.bytes[r.pos:]))
	r.pos += 4
	return v, nil
}

// ReadLong reads a little-endian int64.
func (r *ForwardBytesReader) ReadLong() (int64, error) {
	if r.pos+8 > len(r.bytes) {
		return 0, io.EOF
	}
	v := int64(binary.LittleEndian.Uint64(r.bytes[r.pos:]))
	r.pos += 8
	return v, nil
}

// ReadString is unsupported on this reader; the FST byte stream never
// contains a string.
func (r *ForwardBytesReader) ReadString() (string, error) {
	return "", errors.New("ForwardBytesReader: ReadString not supported")
}

// ReadVInt implements store.VariableLengthInput.
func (r *ForwardBytesReader) ReadVInt() (int32, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int32(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("ForwardBytesReader: corrupted VInt")
		}
		b, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
	}
	return result, nil
}

// ReadVLong implements store.VariableLengthInput.
func (r *ForwardBytesReader) ReadVLong() (int64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int64(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("ForwardBytesReader: corrupted VLong")
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
func (r *ForwardBytesReader) GetPosition() int64 { return int64(r.pos) }

// SetPosition implements BytesReader.
func (r *ForwardBytesReader) SetPosition(pos int64) {
	// Clamp negative positions to 0 and positions past the end to len(r.bytes)
	// to prevent OOB panics when reading from a crafted/corrupted FST.
	if pos < 0 {
		r.pos = 0
	} else if int(pos) > len(r.bytes) {
		r.pos = len(r.bytes)
	} else {
		r.pos = int(pos)
	}
}

// SkipBytes implements BytesReader. n must be non-negative for a
// forward reader; negative skips rewind the position (mirrors the
// Lucene contract used by reverse readers).
func (r *ForwardBytesReader) SkipBytes(n int64) error {
	r.pos += int(n)
	return nil
}

// Compile-time interface check.
var _ BytesReader = (*ForwardBytesReader)(nil)
