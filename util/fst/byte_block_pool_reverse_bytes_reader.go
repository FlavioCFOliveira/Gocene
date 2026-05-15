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

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ByteBlockPoolReverseBytesReader is the Go port of the package-private
// org.apache.lucene.util.fst.ByteBlockPoolReverseBytesReader. It reads
// in reverse from a util.ByteBlockPool. Each call to ReadByte returns
// the byte at the current position and decrements the position by one.
//
// The reader keeps a posDelta so that callers can address bytes using
// FST node addresses (whose value differs from the local offset inside
// the pool); the actual pool offset is pos - posDelta.
type ByteBlockPoolReverseBytesReader struct {
	buf      *util.ByteBlockPool
	posDelta int64
	pos      int64
}

// NewByteBlockPoolReverseBytesReader wraps a pool as a reverse reader.
func NewByteBlockPoolReverseBytesReader(buf *util.ByteBlockPool) *ByteBlockPoolReverseBytesReader {
	return &ByteBlockPoolReverseBytesReader{buf: buf}
}

// SetPosDelta records the difference between the FST node address and
// the local copied-node address inside the underlying pool.
func (r *ByteBlockPoolReverseBytesReader) SetPosDelta(delta int64) { r.posDelta = delta }

// ReadByte implements store.DataInput.
func (r *ByteBlockPoolReverseBytesReader) ReadByte() (byte, error) {
	if r.pos < 0 {
		return 0, io.EOF
	}
	b := r.buf.ReadByteAt(r.pos)
	r.pos--
	return b, nil
}

// ReadBytes implements store.DataInput.
func (r *ByteBlockPoolReverseBytesReader) ReadBytes(b []byte) error {
	for i := range b {
		v, err := r.ReadByte()
		if err != nil {
			return err
		}
		b[i] = v
	}
	return nil
}

// ReadBytesN implements store.DataInput.
func (r *ByteBlockPoolReverseBytesReader) ReadBytesN(n int) ([]byte, error) {
	if n < 0 {
		return nil, errors.New("ByteBlockPoolReverseBytesReader.ReadBytesN: negative n")
	}
	out := make([]byte, n)
	if err := r.ReadBytes(out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReadShort implements store.DataInput. The two-byte assembly mirrors
// ReverseBytesReader.ReadShort: the LSB (in forward writer order) is
// read first, then the MSB.
func (r *ByteBlockPoolReverseBytesReader) ReadShort() (int16, error) {
	if r.pos < 1 {
		return 0, io.EOF
	}
	lo := r.buf.ReadByteAt(r.pos)
	r.pos--
	hi := r.buf.ReadByteAt(r.pos)
	r.pos--
	return int16(binary.BigEndian.Uint16([]byte{hi, lo})), nil
}

// ReadInt implements store.DataInput.
func (r *ByteBlockPoolReverseBytesReader) ReadInt() (int32, error) {
	if r.pos < 3 {
		return 0, io.EOF
	}
	b3 := r.buf.ReadByteAt(r.pos)
	r.pos--
	b2 := r.buf.ReadByteAt(r.pos)
	r.pos--
	b1 := r.buf.ReadByteAt(r.pos)
	r.pos--
	b0 := r.buf.ReadByteAt(r.pos)
	r.pos--
	return int32(binary.BigEndian.Uint32([]byte{b0, b1, b2, b3})), nil
}

// ReadLong implements store.DataInput.
func (r *ByteBlockPoolReverseBytesReader) ReadLong() (int64, error) {
	if r.pos < 7 {
		return 0, io.EOF
	}
	var raw [8]byte
	for i := 7; i >= 0; i-- {
		raw[i] = r.buf.ReadByteAt(r.pos)
		r.pos--
	}
	return int64(binary.BigEndian.Uint64(raw[:])), nil
}

// ReadString is not used by the FST reverse reader.
func (r *ByteBlockPoolReverseBytesReader) ReadString() (string, error) {
	return "", errors.New("ByteBlockPoolReverseBytesReader: ReadString not supported")
}

// ReadVInt implements store.VariableLengthInput.
func (r *ByteBlockPoolReverseBytesReader) ReadVInt() (int32, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int32(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("ByteBlockPoolReverseBytesReader: corrupted VInt")
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
func (r *ByteBlockPoolReverseBytesReader) ReadVLong() (int64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int64(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("ByteBlockPoolReverseBytesReader: corrupted VLong")
		}
		b, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
	}
	return result, nil
}

// GetPosition implements BytesReader. Returns the FST-relative
// address, i.e. pos + posDelta.
func (r *ByteBlockPoolReverseBytesReader) GetPosition() int64 { return r.pos + r.posDelta }

// SetPosition implements BytesReader. The argument is an FST-relative
// address; the reader translates it back to a pool offset internally.
func (r *ByteBlockPoolReverseBytesReader) SetPosition(pos int64) { r.pos = pos - r.posDelta }

// SkipBytes implements BytesReader.
func (r *ByteBlockPoolReverseBytesReader) SkipBytes(n int64) error {
	r.pos -= n
	return nil
}

// Compile-time interface check.
var _ BytesReader = (*ByteBlockPoolReverseBytesReader)(nil)
