// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"hash/crc32"
)

// BufferedChecksumIndexInput is a simple ChecksumIndexInput implementation
// that wraps another IndexInput and computes a CRC32 checksum of every byte
// read.
//
// This is the Go port of org.apache.lucene.store.BufferedChecksumIndexInput
// (Apache Lucene 10.4.0). Per the Lucene contract:
//   - Clone and Slice are not supported and return an error.
//   - The checksum is computed using BufferedChecksum wrapping a CRC32.
//   - GetFilePointer and Length delegate to the wrapped IndexInput.
type BufferedChecksumIndexInput struct {
	main   IndexInput
	digest *BufferedChecksum
}

// ErrBufferedChecksumNotSupported is returned by Clone / Slice on a
// BufferedChecksumIndexInput, matching Lucene's UnsupportedOperationException.
var ErrBufferedChecksumNotSupported = errors.New("BufferedChecksumIndexInput does not support Clone or Slice")

// NewBufferedChecksumIndexInput wraps the given IndexInput so that every byte
// read updates a CRC32 checksum exposed via GetChecksum.
func NewBufferedChecksumIndexInput(main IndexInput) *BufferedChecksumIndexInput {
	return &BufferedChecksumIndexInput{
		main:   main,
		digest: NewBufferedChecksum(crc32.NewIEEE()),
	}
}

// ReadByte reads a single byte from the underlying input and updates the
// checksum.
func (in *BufferedChecksumIndexInput) ReadByte() (byte, error) {
	b, err := in.main.ReadByte()
	if err != nil {
		return 0, err
	}
	in.digest.Update(b)
	return b, nil
}

// ReadBytes reads len(b) bytes into b and updates the checksum.
func (in *BufferedChecksumIndexInput) ReadBytes(b []byte) error {
	if err := in.main.ReadBytes(b); err != nil {
		return err
	}
	in.digest.UpdateBytes(b)
	return nil
}

// ReadBytesN reads exactly n bytes and returns them; updates the checksum.
func (in *BufferedChecksumIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b); err != nil {
		return nil, err
	}
	return b, nil
}

// ReadShort reads a 16-bit LE value via the underlying input and updates the
// checksum.
func (in *BufferedChecksumIndexInput) ReadShort() (int16, error) {
	v, err := in.main.ReadShort()
	if err != nil {
		return 0, err
	}
	in.digest.UpdateShort(v)
	return v, nil
}

// ReadInt reads a 32-bit LE value via the underlying input and updates the
// checksum.
func (in *BufferedChecksumIndexInput) ReadInt() (int32, error) {
	v, err := in.main.ReadInt()
	if err != nil {
		return 0, err
	}
	in.digest.UpdateInt(v)
	return v, nil
}

// ReadLong reads a 64-bit LE value via the underlying input and updates the
// checksum.
func (in *BufferedChecksumIndexInput) ReadLong() (int64, error) {
	v, err := in.main.ReadLong()
	if err != nil {
		return 0, err
	}
	in.digest.UpdateLong(v)
	return v, nil
}

// ReadString reads a length-prefixed UTF-8 string and updates the checksum.
func (in *BufferedChecksumIndexInput) ReadString() (string, error) {
	return ReadString(in)
}

// GetChecksum returns the CRC32 checksum computed over every byte read so
// far. Matches Lucene's getChecksum().
func (in *BufferedChecksumIndexInput) GetChecksum() uint32 {
	return in.digest.Sum32()
}

// GetFilePointer delegates to the underlying input.
func (in *BufferedChecksumIndexInput) GetFilePointer() int64 {
	return in.main.GetFilePointer()
}

// Length delegates to the underlying input.
func (in *BufferedChecksumIndexInput) Length() int64 {
	return in.main.Length()
}

// Close closes the underlying input.
func (in *BufferedChecksumIndexInput) Close() error {
	return in.main.Close()
}

// SetPosition implements forward-only seeking; backward seeks return an
// error (matching Lucene's ChecksumIndexInput.seek contract). Forward seeks
// are realised by reading the intervening bytes through the checksum.
func (in *BufferedChecksumIndexInput) SetPosition(pos int64) error {
	cur := in.main.GetFilePointer()
	if pos < cur {
		return fmt.Errorf("BufferedChecksumIndexInput cannot seek backwards (pos=%d, fp=%d)", pos, cur)
	}
	if pos == cur {
		return nil
	}
	return in.SkipBytes(pos - cur)
}

// SkipBytes skips n bytes by reading through the buffered checksum, ensuring
// the running CRC reflects the skipped bytes.
func (in *BufferedChecksumIndexInput) SkipBytes(n int64) error {
	if n < 0 {
		return fmt.Errorf("skipBytes must be >= 0, got %d", n)
	}
	const skipBufferSize = 1024
	buf := make([]byte, skipBufferSize)
	for n > 0 {
		step := int64(skipBufferSize)
		if n < step {
			step = n
		}
		if err := in.ReadBytes(buf[:step]); err != nil {
			return err
		}
		n -= step
	}
	return nil
}

// Clone returns nil because BufferedChecksumIndexInput does not support
// cloning, matching Lucene's UnsupportedOperationException. Callers should
// check before invoking; future-proof callers can detect via the
// CheckedCloneable assertion.
func (in *BufferedChecksumIndexInput) Clone() IndexInput {
	// Returning nil here keeps the IndexInput interface contract from leaking
	// errors through Clone, matching Java's behaviour of throwing at call
	// time. Production callers should never invoke Clone on this type.
	return nil
}

// Slice returns ErrBufferedChecksumNotSupported, matching Lucene's
// UnsupportedOperationException for this method.
func (in *BufferedChecksumIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	return nil, ErrBufferedChecksumNotSupported
}

// Ensure BufferedChecksumIndexInput satisfies the IndexInput interface.
var _ IndexInput = (*BufferedChecksumIndexInput)(nil)
