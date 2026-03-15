// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"encoding/binary"
	"io"
)

// OutputStreamIndexOutput implements IndexOutput by wrapping an io.Writer.
// This is useful for writing to byte buffers, files, or other output streams.
//
// This is the Go port of Lucene's org.apache.lucene.store.OutputStreamIndexOutput.
type OutputStreamIndexOutput struct {
	*BaseIndexOutput

	// writer is the underlying writer
	writer io.Writer

	// buffer is a reusable buffer for writing primitives
	buffer []byte
}

// NewOutputStreamIndexOutput creates a new OutputStreamIndexOutput.
//
// Parameters:
//   - resourceDescription: a description of the resource (for error messages)
//   - name: the name of the file being written
//   - writer: the underlying writer to write to
//   - bufferSize: the buffer size (currently unused but kept for API compatibility)
func NewOutputStreamIndexOutput(resourceDescription, name string, writer io.Writer, bufferSize int) *OutputStreamIndexOutput {
	return &OutputStreamIndexOutput{
		BaseIndexOutput: NewBaseIndexOutput(name),
		writer:          writer,
		buffer:          make([]byte, 8), // enough for int64
	}
}

// WriteByte writes a single byte.
func (out *OutputStreamIndexOutput) WriteByte(b byte) error {
	out.buffer[0] = b
	if _, err := out.writer.Write(out.buffer[:1]); err != nil {
		return err
	}
	out.IncrementFilePointer(1)
	return nil
}

// WriteBytes writes all bytes from b.
func (out *OutputStreamIndexOutput) WriteBytes(b []byte) error {
	return out.WriteBytesN(b, len(b))
}

// WriteBytesN writes exactly n bytes from b.
func (out *OutputStreamIndexOutput) WriteBytesN(b []byte, n int) error {
	if n < 0 || n > len(b) {
		return ErrIO
	}
	if _, err := out.writer.Write(b[:n]); err != nil {
		return err
	}
	out.IncrementFilePointer(int64(n))
	return nil
}

// WriteShort writes a 16-bit signed integer in little-endian format.
// This matches Lucene's OutputStreamIndexOutput behavior.
func (out *OutputStreamIndexOutput) WriteShort(v int16) error {
	binary.LittleEndian.PutUint16(out.buffer, uint16(v))
	if _, err := out.writer.Write(out.buffer[:2]); err != nil {
		return err
	}
	out.IncrementFilePointer(2)
	return nil
}

// WriteInt writes a 32-bit signed integer in little-endian format.
// This matches Lucene's OutputStreamIndexOutput behavior.
func (out *OutputStreamIndexOutput) WriteInt(v int32) error {
	binary.LittleEndian.PutUint32(out.buffer, uint32(v))
	if _, err := out.writer.Write(out.buffer[:4]); err != nil {
		return err
	}
	out.IncrementFilePointer(4)
	return nil
}

// WriteLong writes a 64-bit signed integer in little-endian format.
// This matches Lucene's OutputStreamIndexOutput behavior.
func (out *OutputStreamIndexOutput) WriteLong(v int64) error {
	binary.LittleEndian.PutUint64(out.buffer, uint64(v))
	if _, err := out.writer.Write(out.buffer[:8]); err != nil {
		return err
	}
	out.IncrementFilePointer(8)
	return nil
}

// WriteString writes a string.
func (out *OutputStreamIndexOutput) WriteString(s string) error {
	return WriteString(out, s)
}

// Length returns the current file pointer as the length.
func (out *OutputStreamIndexOutput) Length() int64 {
	return out.GetFilePointer()
}

// Close closes this output.
func (out *OutputStreamIndexOutput) Close() error {
	// Nothing to close for the writer itself, but we could add
	// a closer interface check if needed
	return nil
}

// ByteBufferWriter is a writer that captures bytes for testing
type ByteBufferWriter struct {
	buf *bytes.Buffer
}

// NewByteBufferWriter creates a new ByteBufferWriter.
func NewByteBufferWriter() *ByteBufferWriter {
	return &ByteBufferWriter{
		buf: &bytes.Buffer{},
	}
}

// Write implements io.Writer.
func (w *ByteBufferWriter) Write(p []byte) (n int, err error) {
	return w.buf.Write(p)
}

// Bytes returns the written bytes.
func (w *ByteBufferWriter) Bytes() []byte {
	return w.buf.Bytes()
}
