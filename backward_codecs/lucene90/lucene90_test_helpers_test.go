// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"encoding/binary"
	"io"
	"testing"

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

// --- Tests for bytesIndexInput ---

func TestBytesIndexInput_NilData(t *testing.T) {
	b := newBytesIndexInput(nil)
	if b == nil {
		t.Fatal("newBytesIndexInput returned nil")
	}
	if b.Length() != 0 {
		t.Fatalf("Length: got %d, want 0", b.Length())
	}
}

func TestBytesIndexInput_EmptyData(t *testing.T) {
	b := newBytesIndexInput([]byte{})
	if b.Length() != 0 {
		t.Fatalf("Length: got %d, want 0", b.Length())
	}
}

func TestBytesIndexInput_ReadByte(t *testing.T) {
	b := newBytesIndexInput([]byte{0x42})
	v, err := b.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if v != 0x42 {
		t.Fatalf("got %02x, want 42", v)
	}
	if b.GetFilePointer() != 1 {
		t.Fatalf("file pointer after ReadByte: got %d, want 1", b.GetFilePointer())
	}
}

func TestBytesIndexInput_ReadByteEOF(t *testing.T) {
	b := newBytesIndexInput([]byte{})
	_, err := b.ReadByte()
	if err == nil {
		t.Fatal("expected EOF on empty input")
	}
}

func TestBytesIndexInput_ReadBytes(t *testing.T) {
	b := newBytesIndexInput([]byte{0x01, 0x02, 0x03, 0x04})
	dst := make([]byte, 4)
	if err := b.ReadBytes(dst); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	expected := []byte{0x01, 0x02, 0x03, 0x04}
	for i := range expected {
		if dst[i] != expected[i] {
			t.Fatalf("dst[%d]: got %02x, want %02x", i, dst[i], expected[i])
		}
	}
}

func TestBytesIndexInput_SetPosition(t *testing.T) {
	b := newBytesIndexInput([]byte{0x01, 0x02, 0x03})
	if err := b.SetPosition(1); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	if b.GetFilePointer() != 1 {
		t.Fatalf("file pointer: got %d, want 1", b.GetFilePointer())
	}
}

func TestBytesIndexInput_SetPositionOutOfBounds(t *testing.T) {
	b := newBytesIndexInput([]byte{0x01})
	if err := b.SetPosition(5); err == nil {
		t.Fatal("expected error for out-of-bounds position")
	}
}

func TestBytesIndexInput_Clone(t *testing.T) {
	b := newBytesIndexInput([]byte{0x01, 0x02})
	b.pos = 1
	clone := b.Clone()
	if clone.GetFilePointer() != 1 {
		t.Fatalf("clone file pointer: got %d, want 1", clone.GetFilePointer())
	}
	// Advancing original should not affect clone.
	b.SetPosition(0)
	if clone.GetFilePointer() != 1 {
		t.Fatalf("clone file pointer after original advance: got %d, want 1", clone.GetFilePointer())
	}
}

func TestBytesIndexInput_Slice(t *testing.T) {
	b := newBytesIndexInput([]byte{0x01, 0x02, 0x03, 0x04, 0x05})
	s, err := b.Slice("test", 1, 3)
	if err != nil {
		t.Fatalf("Slice: %v", err)
	}
	if s.Length() != 3 {
		t.Fatalf("slice length: got %d, want 3", s.Length())
	}
	v, err := s.ReadByte()
	if err != nil {
		t.Fatalf("slice ReadByte: %v", err)
	}
	if v != 0x02 {
		t.Fatalf("slice first byte: got %02x, want 02", v)
	}
}

func TestBytesIndexInput_SliceOutOfBounds(t *testing.T) {
	b := newBytesIndexInput([]byte{0x01, 0x02})
	_, err := b.Slice("test", 0, 10)
	if err == nil {
		t.Fatal("expected error for out-of-bounds slice")
	}
}

func TestBytesIndexInput_Close(t *testing.T) {
	b := newBytesIndexInput([]byte{0x01})
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestBytesIndexInput_ReadInt(t *testing.T) {
	b := newBytesIndexInput([]byte{0x00, 0x00, 0x00, 0x2a})
	v, err := b.ReadInt()
	if err != nil {
		t.Fatalf("ReadInt: %v", err)
	}
	if v != 42 {
		t.Fatalf("ReadInt: got %d, want 42", v)
	}
}

func TestBytesIndexInput_ReadLong(t *testing.T) {
	b := newBytesIndexInput([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2a})
	v, err := b.ReadLong()
	if err != nil {
		t.Fatalf("ReadLong: %v", err)
	}
	if v != 42 {
		t.Fatalf("ReadLong: got %d, want 42", v)
	}
}

func TestBytesIndexInput_ReadShort(t *testing.T) {
	b := newBytesIndexInput([]byte{0x00, 0x2a})
	v, err := b.ReadShort()
	if err != nil {
		t.Fatalf("ReadShort: %v", err)
	}
	if v != 42 {
		t.Fatalf("ReadShort: got %d, want 42", v)
	}
}

func TestBytesIndexInput_ReadBytesN(t *testing.T) {
	b := newBytesIndexInput([]byte{0x01, 0x02, 0x03})
	out, err := b.ReadBytesN(2)
	if err != nil {
		t.Fatalf("ReadBytesN: %v", err)
	}
	if len(out) != 2 || out[0] != 0x01 || out[1] != 0x02 {
		t.Fatalf("ReadBytesN: got %v, want [1 2]", out)
	}
}

func TestBytesIndexInput_ReadBytesNEOF(t *testing.T) {
	b := newBytesIndexInput([]byte{0x01})
	_, err := b.ReadBytesN(5)
	if err == nil {
		t.Fatal("expected EOF for ReadBytesN beyond length")
	}
}

func TestBytesIndexInput_ImplementsStoreIndexInput(t *testing.T) {
	b := newBytesIndexInput(nil)
	var _ store.IndexInput = b
}

