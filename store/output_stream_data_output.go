// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"io"
)

// OutputStreamDataOutput is the Go port of
// org.apache.lucene.store.OutputStreamDataOutput.
//
// It wraps a plain io.Writer and exposes the DataOutput surface (byte, bytes,
// little-endian fixed-width integers, VInt/VLong, string). When the wrapped
// writer also implements io.Closer the Close method is forwarded so callers
// can use this type as a drop-in replacement for the Java wrapper around an
// OutputStream.
type OutputStreamDataOutput struct {
	w   io.Writer
	buf [8]byte // scratch buffer for fixed-width writes (no heap allocations).
}

// NewOutputStreamDataOutput wraps the given io.Writer.
func NewOutputStreamDataOutput(w io.Writer) *OutputStreamDataOutput {
	return &OutputStreamDataOutput{w: w}
}

// WriteByte writes a single byte.
func (o *OutputStreamDataOutput) WriteByte(b byte) error {
	o.buf[0] = b
	_, err := o.w.Write(o.buf[:1])
	return err
}

// WriteBytes writes all bytes from b.
func (o *OutputStreamDataOutput) WriteBytes(b []byte) error {
	_, err := o.w.Write(b)
	return err
}

// WriteBytesN writes the first n bytes from b.
func (o *OutputStreamDataOutput) WriteBytesN(b []byte, n int) error {
	if n < 0 || n > len(b) {
		return ErrIO
	}
	_, err := o.w.Write(b[:n])
	return err
}

// WriteShort writes a 16-bit little-endian value.
func (o *OutputStreamDataOutput) WriteShort(v int16) error {
	binary.LittleEndian.PutUint16(o.buf[:2], uint16(v))
	_, err := o.w.Write(o.buf[:2])
	return err
}

// WriteInt writes a 32-bit little-endian value.
func (o *OutputStreamDataOutput) WriteInt(v int32) error {
	binary.LittleEndian.PutUint32(o.buf[:4], uint32(v))
	_, err := o.w.Write(o.buf[:4])
	return err
}

// WriteLong writes a 64-bit little-endian value.
func (o *OutputStreamDataOutput) WriteLong(v int64) error {
	binary.LittleEndian.PutUint64(o.buf[:8], uint64(v))
	_, err := o.w.Write(o.buf[:8])
	return err
}

// WriteVInt writes a Lucene variable-length integer.
func (o *OutputStreamDataOutput) WriteVInt(v int32) error {
	u := uint32(v)
	for u&^0x7F != 0 {
		if err := o.WriteByte(byte(u&0x7F | 0x80)); err != nil {
			return err
		}
		u >>= 7
	}
	return o.WriteByte(byte(u))
}

// WriteVLong writes a Lucene variable-length long.
func (o *OutputStreamDataOutput) WriteVLong(v int64) error {
	u := uint64(v)
	for u&^0x7F != 0 {
		if err := o.WriteByte(byte(u&0x7F | 0x80)); err != nil {
			return err
		}
		u >>= 7
	}
	return o.WriteByte(byte(u))
}

// WriteString writes a length-prefixed UTF-8 string using the package
// WriteString helper.
func (o *OutputStreamDataOutput) WriteString(s string) error {
	return WriteString(o, s)
}

// Close forwards to the wrapped writer's Close method when it implements
// io.Closer; otherwise Close is a no-op.
func (o *OutputStreamDataOutput) Close() error {
	if c, ok := o.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Compile-time assertion that OutputStreamDataOutput satisfies DataOutput.
var _ DataOutput = (*OutputStreamDataOutput)(nil)
