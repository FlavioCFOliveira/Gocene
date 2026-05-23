// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"encoding/binary"
	"io"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// bytesIndexInput is a minimal store.IndexInput backed by a byte slice.
// It is used in skip-reader tests that require a store.IndexInput argument
// but never perform actual reads (the readers are initialised, not exercised).
type bytesIndexInput struct {
	data []byte
	pos  int64
}

// newBytesIndexInput returns a bytesIndexInput wrapping data.
// data may be nil (treated as an empty slice).
func newBytesIndexInput(data []byte) *bytesIndexInput {
	if data == nil {
		data = []byte{}
	}
	return &bytesIndexInput{data: data}
}

func (b *bytesIndexInput) GetFilePointer() int64 { return b.pos }
func (b *bytesIndexInput) Length() int64         { return int64(len(b.data)) }

func (b *bytesIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > int64(len(b.data)) {
		return io.ErrUnexpectedEOF
	}
	b.pos = pos
	return nil
}

func (b *bytesIndexInput) ReadByte() (byte, error) {
	if b.pos >= int64(len(b.data)) {
		return 0, io.EOF
	}
	v := b.data[b.pos]
	b.pos++
	return v, nil
}

func (b *bytesIndexInput) ReadBytes(dst []byte) error {
	n := int64(len(dst))
	if b.pos+n > int64(len(b.data)) {
		return io.EOF
	}
	copy(dst, b.data[b.pos:b.pos+n])
	b.pos += n
	return nil
}

func (b *bytesIndexInput) ReadBytesN(n int) ([]byte, error) {
	if b.pos+int64(n) > int64(len(b.data)) {
		return nil, io.EOF
	}
	out := make([]byte, n)
	copy(out, b.data[b.pos:b.pos+int64(n)])
	b.pos += int64(n)
	return out, nil
}

func (b *bytesIndexInput) ReadShort() (int16, error) {
	if b.pos+2 > int64(len(b.data)) {
		return 0, io.EOF
	}
	v := binary.BigEndian.Uint16(b.data[b.pos:])
	b.pos += 2
	return int16(v), nil
}

func (b *bytesIndexInput) ReadInt() (int32, error) {
	if b.pos+4 > int64(len(b.data)) {
		return 0, io.EOF
	}
	v := binary.BigEndian.Uint32(b.data[b.pos:])
	b.pos += 4
	return int32(v), nil
}

func (b *bytesIndexInput) ReadLong() (int64, error) {
	if b.pos+8 > int64(len(b.data)) {
		return 0, io.EOF
	}
	v := binary.BigEndian.Uint64(b.data[b.pos:])
	b.pos += 8
	return int64(v), nil
}

func (b *bytesIndexInput) ReadString() (string, error) {
	n, err := store.ReadVInt(b)
	if err != nil {
		return "", err
	}
	if n < 0 {
		return "", io.ErrUnexpectedEOF
	}
	raw, err := b.ReadBytesN(int(n))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (b *bytesIndexInput) Clone() store.IndexInput {
	clone := &bytesIndexInput{
		data: b.data,
		pos:  b.pos,
	}
	return clone
}

func (b *bytesIndexInput) Slice(desc string, offset int64, length int64) (store.IndexInput, error) {
	if offset < 0 || length < 0 || offset+length > int64(len(b.data)) {
		return nil, io.ErrUnexpectedEOF
	}
	return &bytesIndexInput{data: b.data[offset : offset+length]}, nil
}

func (b *bytesIndexInput) Close() error { return nil }

var _ store.IndexInput = (*bytesIndexInput)(nil)
