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

	"github.com/FlavioCFOliveira/Gocene/store"
)

// ReverseRandomAccessReader reads bytes in reverse order from a
// RandomAccessInput. It is the Go port of Lucene's package-private
// ReverseRandomAccessReader and is used by OffHeapFSTStore when the
// FST bytes live in an IndexInput rather than in a heap byte slice.
type ReverseRandomAccessReader struct {
	in  store.RandomAccessInput
	pos int64
}

// NewReverseRandomAccessReader builds a reverse reader over the given
// random-access input. The caller is expected to position the reader
// via SetPosition before consuming bytes.
func NewReverseRandomAccessReader(in store.RandomAccessInput) *ReverseRandomAccessReader {
	return &ReverseRandomAccessReader{in: in}
}

// ReadByte implements DataInput.
func (r *ReverseRandomAccessReader) ReadByte() (byte, error) {
	if r.pos < 0 {
		return 0, errors.New("ReverseRandomAccessReader: position out of range")
	}
	b, err := r.in.ReadByteAt(r.pos)
	if err != nil {
		return 0, err
	}
	r.pos--
	return b, nil
}

// ReadBytes implements DataInput.
func (r *ReverseRandomAccessReader) ReadBytes(b []byte) error {
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
func (r *ReverseRandomAccessReader) ReadBytesN(n int) ([]byte, error) {
	out := make([]byte, n)
	if err := r.ReadBytes(out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReadShort implements DataInput.
func (r *ReverseRandomAccessReader) ReadShort() (int16, error) {
	lo, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	hi, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	return int16(binary.BigEndian.Uint16([]byte{hi, lo})), nil
}

// ReadInt implements DataInput.
func (r *ReverseRandomAccessReader) ReadInt() (int32, error) {
	b3, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	b2, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	b1, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	b0, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32([]byte{b0, b1, b2, b3})), nil
}

// ReadLong implements DataInput.
func (r *ReverseRandomAccessReader) ReadLong() (int64, error) {
	var raw [8]byte
	for i := 7; i >= 0; i-- {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		raw[i] = b
	}
	return int64(binary.BigEndian.Uint64(raw[:])), nil
}

// ReadString is not used by the FST reverse reader.
func (r *ReverseRandomAccessReader) ReadString() (string, error) {
	return "", errors.New("ReverseRandomAccessReader: ReadString not supported")
}

// ReadVInt implements VariableLengthInput.
func (r *ReverseRandomAccessReader) ReadVInt() (int32, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int32(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("ReverseRandomAccessReader: corrupted VInt")
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
func (r *ReverseRandomAccessReader) ReadVLong() (int64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int64(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("ReverseRandomAccessReader: corrupted VLong")
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
func (r *ReverseRandomAccessReader) GetPosition() int64 { return r.pos }

// SetPosition implements BytesReader.
func (r *ReverseRandomAccessReader) SetPosition(pos int64) { r.pos = pos }

// SkipBytes implements BytesReader. See ReverseBytesReader.SkipBytes
// for the reverse-direction semantics.
func (r *ReverseRandomAccessReader) SkipBytes(n int64) error {
	r.pos -= n
	return nil
}
