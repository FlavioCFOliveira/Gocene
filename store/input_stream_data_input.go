// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"io"
)

// InputStreamDataInput implements DataInput by wrapping an io.Reader.
// This is the Go port of Lucene's org.apache.lucene.store.InputStreamDataInput.
type InputStreamDataInput struct {
	reader io.Reader
	closed bool
}

// NewInputStreamDataInput creates a new InputStreamDataInput wrapping the given reader.
func NewInputStreamDataInput(reader io.Reader) *InputStreamDataInput {
	return &InputStreamDataInput{
		reader: reader,
		closed: false,
	}
}

// ReadByte reads a single byte from the underlying reader.
func (in *InputStreamDataInput) ReadByte() (byte, error) {
	if in.closed {
		return 0, io.EOF
	}
	buf := make([]byte, 1)
	_, err := io.ReadFull(in.reader, buf)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
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

	// Otherwise, read and discard bytes
	buf := make([]byte, 8192)
	remaining := n
	for remaining > 0 {
		toRead := int64(len(buf))
		if remaining < toRead {
			toRead = remaining
		}
		nread, err := in.reader.Read(buf[:toRead])
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
