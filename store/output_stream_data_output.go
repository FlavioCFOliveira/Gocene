// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
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
	*BaseDataOutput
	w   io.Writer
	buf [8]byte // scratch buffer for fixed-width writes (no heap allocations).
}

// NewOutputStreamDataOutput wraps the given io.Writer.
func NewOutputStreamDataOutput(w io.Writer) *OutputStreamDataOutput {
	out := &OutputStreamDataOutput{w: w}
	out.BaseDataOutput = NewBaseDataOutput(out)
	return out
}

// WriteByte writes a single byte.
func (o *OutputStreamDataOutput) WriteByte(b byte) error {
	o.buf[0] = b
	_, err := o.w.Write(o.buf[:1])
	return err
}

// WriteBytes writes all bytes from b, starting at the given offset.
func (o *OutputStreamDataOutput) WriteBytes(b []byte, offset, length int) error {
	if length <= 0 {
		return nil
	}
	_, err := o.w.Write(b[offset : offset+length])
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
