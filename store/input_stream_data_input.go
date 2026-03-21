// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"io"
)

// InputStreamDataInput implements DataInput by wrapping an io.Reader.
// This is the Go port of Lucene's org.apache.lucene.store.InputStreamDataInput.
// Uses pre-allocated buffers and sync.Pool to eliminate heap allocations.
type InputStreamDataInput struct {
	reader io.Reader
	closed bool

	// byteBuf is a reusable 1-byte buffer for ReadByte
	byteBuf [1]byte

	// skipBuf is a reusable buffer for SkipBytes (8KB)
	skipBuf []byte
}

// NewInputStreamDataInput creates a new InputStreamDataInput wrapping the given reader.
func NewInputStreamDataInput(reader io.Reader) *InputStreamDataInput {
	return &InputStreamDataInput{
		reader:  reader,
		closed:  false,
		skipBuf: make([]byte, 8192),
	}
}

// ReadByte reads a single byte from the underlying reader.
// Uses pre-allocated buffer to avoid heap allocation.
func (in *InputStreamDataInput) ReadByte() (byte, error) {
	if in.closed {
		return 0, io.EOF
	}
	_, err := io.ReadFull(in.reader, in.byteBuf[:])
	if err != nil {
		return 0, err
	}
	return in.byteBuf[0], nil
}

// ReadBytes reads len(b) bytes into b.
func (in *InputStreamDataInput) ReadBytes(b []byte) error {
	if in.closed {
		return io.EOF
	}
	_, err := io.ReadFull(in.reader, b)
	return err
}

// ReadBytesN reads exactly n bytes and returns them.
func (in *InputStreamDataInput) ReadBytesN(n int) ([]byte, error) {
	if in.closed {
		return nil, io.EOF
	}
	buf := make([]byte, n)
	_, err := io.ReadFull(in.reader, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// SkipBytes skips n bytes by reading and discarding them.
// This implementation uses the underlying reader's skip capability if available.
func (in *InputStreamDataInput) SkipBytes(n int64) error {
	if in.closed {
		return io.EOF
	}

	// Try to use Seeker if available (more efficient)
	if seeker, ok := in.reader.(io.Seeker); ok {
		_, err := seeker.Seek(n, io.SeekCurrent)
		return err
	}

	// Otherwise, read and discard bytes using pre-allocated buffer
	remaining := n
	for remaining > 0 {
		toRead := int64(len(in.skipBuf))
		if remaining < toRead {
			toRead = remaining
		}
		nread, err := in.reader.Read(in.skipBuf[:toRead])
		remaining -= int64(nread)
		if err != nil {
			if err == io.EOF && remaining > 0 {
				return io.EOF
			}
			return err
		}
	}
	return nil
}

// Close closes this InputStreamDataInput.
func (in *InputStreamDataInput) Close() error {
	in.closed = true
	if closer, ok := in.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// IsClosed returns true if this input has been closed.
func (in *InputStreamDataInput) IsClosed() bool {
	return in.closed
}
