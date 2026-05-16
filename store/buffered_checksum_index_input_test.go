// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/* (BufferedChecksumIndexInput
// has no dedicated test peer in Lucene 10.4.0; behaviour is exercised indirectly
// via CodecUtil tests and IndexFileNames round-trips. Tests below cover the
// observable contract: checksum accumulation, forward seek semantics, and
// rejection of Clone/Slice).

package store

import (
	"errors"
	"hash/crc32"
	"testing"
)

func newCRCTestInput(data []byte) *BufferedChecksumIndexInput {
	in := &fakeIndexInput{
		BaseIndexInput: NewBaseIndexInput("fake", int64(len(data))),
		data:           data,
	}
	return NewBufferedChecksumIndexInput(in)
}

// fakeIndexInput is a tiny IndexInput backed by an in-memory slice used only
// to drive BufferedChecksumIndexInput tests. It satisfies the full IndexInput
// interface but only implements the methods the test exercises.
type fakeIndexInput struct {
	*BaseIndexInput
	data []byte
}

func (f *fakeIndexInput) ReadByte() (byte, error) {
	if f.GetFilePointer() >= int64(len(f.data)) {
		return 0, errors.New("EOF")
	}
	b := f.data[f.GetFilePointer()]
	f.SetFilePointer(f.GetFilePointer() + 1)
	return b, nil
}

func (f *fakeIndexInput) ReadBytes(b []byte) error {
	fp := f.GetFilePointer()
	if fp+int64(len(b)) > int64(len(f.data)) {
		return errors.New("EOF")
	}
	copy(b, f.data[fp:fp+int64(len(b))])
	f.SetFilePointer(fp + int64(len(b)))
	return nil
}

func (f *fakeIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := f.ReadBytes(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (f *fakeIndexInput) ReadShort() (int16, error) {
	b, err := f.ReadBytesN(2)
	if err != nil {
		return 0, err
	}
	return int16(b[0]) | int16(b[1])<<8, nil
}

func (f *fakeIndexInput) ReadInt() (int32, error) {
	b, err := f.ReadBytesN(4)
	if err != nil {
		return 0, err
	}
	return int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16 | int32(b[3])<<24, nil
}

func (f *fakeIndexInput) ReadLong() (int64, error) {
	b, err := f.ReadBytesN(8)
	if err != nil {
		return 0, err
	}
	var v int64
	for i := 0; i < 8; i++ {
		v |= int64(b[i]) << (8 * i)
	}
	return v, nil
}

func (f *fakeIndexInput) ReadString() (string, error) { return ReadString(f) }

func (f *fakeIndexInput) Close() error                                   { return nil }
func (f *fakeIndexInput) Slice(string, int64, int64) (IndexInput, error) { return nil, nil }
func (f *fakeIndexInput) Clone() IndexInput                              { return f }
func (f *fakeIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > int64(len(f.data)) {
		return errors.New("invalid pos")
	}
	f.SetFilePointer(pos)
	return nil
}

func TestBufferedChecksumIndexInput_AccumulatesChecksum(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	in := newCRCTestInput(data)
	buf := make([]byte, len(data))
	if err := in.ReadBytes(buf); err != nil {
		t.Fatalf("ReadBytes failed: %v", err)
	}
	want := crc32.ChecksumIEEE(data)
	if got := in.GetChecksum(); got != want {
		t.Fatalf("checksum = %#x, want %#x", got, want)
	}
}

func TestBufferedChecksumIndexInput_SkipBytes_AdvancesChecksum(t *testing.T) {
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i & 0xFF)
	}
	in := newCRCTestInput(data)
	if err := in.SkipBytes(1500); err != nil {
		t.Fatalf("SkipBytes failed: %v", err)
	}
	want := crc32.ChecksumIEEE(data[:1500])
	if got := in.GetChecksum(); got != want {
		t.Fatalf("checksum after skip = %#x, want %#x", got, want)
	}
}

func TestBufferedChecksumIndexInput_SetPosition_ForwardOnly(t *testing.T) {
	data := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60}
	in := newCRCTestInput(data)
	if err := in.SetPosition(3); err != nil {
		t.Fatalf("SetPosition(3) failed: %v", err)
	}
	if err := in.SetPosition(1); err == nil {
		t.Fatalf("SetPosition(1) should reject backward seek, got nil error")
	}
}

func TestBufferedChecksumIndexInput_SkipBytes_RejectsNegative(t *testing.T) {
	in := newCRCTestInput([]byte{0})
	if err := in.SkipBytes(-1); err == nil {
		t.Fatalf("SkipBytes(-1) should fail")
	}
}

func TestBufferedChecksumIndexInput_SliceRejected(t *testing.T) {
	in := newCRCTestInput([]byte{0})
	_, err := in.Slice("x", 0, 1)
	if !errors.Is(err, ErrBufferedChecksumNotSupported) {
		t.Fatalf("Slice should return ErrBufferedChecksumNotSupported, got %v", err)
	}
}
