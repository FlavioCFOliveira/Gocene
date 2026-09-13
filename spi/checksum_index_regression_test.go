// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"bytes"
	"hash/crc32"
	"io"
	"testing"
)

// This file pins the checksum-wrapper defects that let bytes past the digest.
//
//	D  ChecksumIndexOutput.WriteString delegated to the wrapped output, so the
//	   string's bytes never entered the digest and never advanced the file
//	   pointer. A file framed through it recorded the wrong CRC in its footer,
//	   which Apache Lucene rejects. CopyBytes had the same shape.
//	E  ChecksumIndexInput.ReadString / ReadInts / ReadLongs / ReadFloats were the
//	   read-side twins.
//	G  ChecksumIndexInput.SetPosition accepted a backward seek and silently reset
//	   the digest. ChecksumIndexInput.seek (ChecksumIndexInput.java:56-64) throws
//	   IllegalStateException instead, precisely so the checksum can never be
//	   quietly invalidated.
//
// In Java none of readString/writeString/copyBytes is declared on a checksum
// wrapper: they are concrete DataInput/DataOutput methods built from
// readVInt+readBytes / writeVInt+writeBytes, so they always run through the
// primitives the wrapper does override.

// memIndexOutput is a minimal IndexOutput over an in-memory buffer. It carries
// no checksum of its own, so anything the wrapper fails to digest is visible.
type memIndexOutput struct {
	*BaseIndexOutput
	buf bytes.Buffer
}

func newMemIndexOutput(name string) *memIndexOutput {
	return &memIndexOutput{BaseIndexOutput: NewBaseIndexOutput(name)}
}

func (o *memIndexOutput) WriteByte(b byte) error {
	o.buf.WriteByte(b)
	o.IncrementFilePointer(1)
	return nil
}

func (o *memIndexOutput) WriteBytes(b []byte, offset, length int) error {
	o.buf.Write(b[offset : offset+length])
	o.IncrementFilePointer(int64(length))
	return nil
}

func (o *memIndexOutput) WriteBytesN(b []byte, n int) error { return o.WriteBytes(b, 0, n) }

func (o *memIndexOutput) WriteShort(i int16) error {
	return o.WriteBytes([]byte{byte(i), byte(i >> 8)}, 0, 2)
}

func (o *memIndexOutput) WriteInt(i int32) error {
	return o.WriteBytes([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)}, 0, 4)
}

func (o *memIndexOutput) WriteLong(i int64) error {
	if err := o.WriteInt(int32(i)); err != nil {
		return err
	}
	return o.WriteInt(int32(i >> 32))
}

func (o *memIndexOutput) WriteVInt(i int32) error {
	v := uint32(i)
	for v & ^uint32(0x7F) != 0 {
		if err := o.WriteByte(byte(v&0x7F | 0x80)); err != nil {
			return err
		}
		v >>= 7
	}
	return o.WriteByte(byte(v))
}

func (o *memIndexOutput) WriteVLong(i int64) error {
	v := uint64(i)
	for v & ^uint64(0x7F) != 0 {
		if err := o.WriteByte(byte(v&0x7F | 0x80)); err != nil {
			return err
		}
		v >>= 7
	}
	return o.WriteByte(byte(v))
}

func (o *memIndexOutput) WriteZInt(i int32) error { return o.WriteVInt(i<<1 ^ i>>31) }
func (o *memIndexOutput) WriteZLong(i int64) error {
	return o.WriteVLong(i<<1 ^ i>>63)
}

func (o *memIndexOutput) WriteString(s string) error {
	if err := o.WriteVInt(int32(len(s))); err != nil {
		return err
	}
	return o.WriteBytes([]byte(s), 0, len(s))
}

func (o *memIndexOutput) WriteMapOfStrings(m map[string]string) error {
	if err := o.WriteVInt(int32(len(m))); err != nil {
		return err
	}
	for k, v := range m {
		if err := o.WriteString(k); err != nil {
			return err
		}
		if err := o.WriteString(v); err != nil {
			return err
		}
	}
	return nil
}

func (o *memIndexOutput) WriteSetOfStrings(s []string) error {
	if err := o.WriteVInt(int32(len(s))); err != nil {
		return err
	}
	for _, v := range s {
		if err := o.WriteString(v); err != nil {
			return err
		}
	}
	return nil
}

func (o *memIndexOutput) WriteGroupVInts(values []int32, limit int) error {
	for i := 0; i < limit; i++ {
		if err := o.WriteVInt(values[i]); err != nil {
			return err
		}
	}
	return nil
}

func (o *memIndexOutput) CopyBytes(in DataInput, numBytes int64) error {
	b := make([]byte, numBytes)
	if err := in.ReadBytes(b, 0, int(numBytes)); err != nil {
		return err
	}
	return o.WriteBytes(b, 0, len(b))
}

func (o *memIndexOutput) Length() int64               { return o.GetFilePointer() }
func (o *memIndexOutput) SetPosition(pos int64) error { return nil }
func (o *memIndexOutput) Close() error                { return nil }

// memIndexInput is a minimal IndexInput over an in-memory buffer. Like
// memIndexOutput it carries no checksum of its own.
type memIndexInput struct {
	*BaseIndexInput
	BaseDataInput
	data []byte
}

func newMemIndexInput(name string, data []byte) *memIndexInput {
	in := &memIndexInput{
		BaseIndexInput: NewBaseIndexInput(name, int64(len(data))),
		data:           data,
	}
	in.Core = in
	return in
}

func (i *memIndexInput) ReadByte() (byte, error) {
	fp := i.GetFilePointer()
	if fp >= int64(len(i.data)) {
		return 0, io.EOF
	}
	i.SetFilePointer(fp + 1)
	return i.data[fp], nil
}

func (i *memIndexInput) ReadBytes(b []byte, offset, length int) error {
	fp := i.GetFilePointer()
	if fp+int64(length) > int64(len(i.data)) {
		return io.EOF
	}
	copy(b[offset:offset+length], i.data[fp:fp+int64(length)])
	i.SetFilePointer(fp + int64(length))
	return nil
}

func (i *memIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := i.ReadBytes(b, 0, n); err != nil {
		return nil, err
	}
	return b, nil
}

func (i *memIndexInput) SkipBytes(n int64) error {
	return i.SetPosition(i.GetFilePointer() + n)
}

func (i *memIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > int64(len(i.data)) {
		return io.EOF
	}
	i.SetFilePointer(pos)
	return nil
}

func (i *memIndexInput) Clone() IndexInput {
	c := newMemIndexInput("clone", i.data)
	c.SetFilePointer(i.GetFilePointer())
	return c
}

func (i *memIndexInput) Slice(desc string, offset, length int64) (IndexInput, error) {
	return newMemIndexInput(desc, i.data[offset:offset+length]), nil
}

func (i *memIndexInput) Close() error { return nil }

// TestChecksumIndexOutputDigestsStrings is the regression test for defect D.
// The digest and the file pointer must both account for every byte the string
// writers emit; otherwise the CodecUtil footer records a CRC Lucene rejects.
func TestChecksumIndexOutputDigestsStrings(t *testing.T) {
	raw := newMemIndexOutput("digest.bin")
	out := NewChecksumIndexOutput(raw)

	if err := out.WriteString("a string that must enter the digest"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := out.WriteMapOfStrings(map[string]string{"key": "value"}); err != nil {
		t.Fatalf("WriteMapOfStrings: %v", err)
	}
	if err := out.WriteSetOfStrings([]string{"one", "two"}); err != nil {
		t.Fatalf("WriteSetOfStrings: %v", err)
	}

	written := raw.buf.Bytes()
	if got, want := out.GetFilePointer(), int64(len(written)); got != want {
		t.Fatalf("file pointer %d did not track the %d bytes written", got, want)
	}
	// Computed independently of the wrapper, over the bytes that actually landed.
	want := crc32.ChecksumIEEE(written)
	if out.GetChecksum() != want {
		t.Fatalf("digest 0x%08x != CRC-32 over the written bytes 0x%08x", out.GetChecksum(), want)
	}
}

// TestChecksumIndexOutputDigestsCopyBytes pins the same property for CopyBytes,
// which also used to be handed to the wrapped output.
func TestChecksumIndexOutputDigestsCopyBytes(t *testing.T) {
	payload := []byte("bytes copied straight through the checksum")
	raw := newMemIndexOutput("copy.bin")
	out := NewChecksumIndexOutput(raw)

	src := newMemIndexInput("src.bin", payload)
	if err := out.CopyBytes(src, int64(len(payload))); err != nil {
		t.Fatalf("CopyBytes: %v", err)
	}
	if got, want := out.GetFilePointer(), int64(len(payload)); got != want {
		t.Fatalf("file pointer %d, want %d", got, want)
	}
	if got, want := out.GetChecksum(), crc32.ChecksumIEEE(payload); got != want {
		t.Fatalf("digest 0x%08x != CRC-32 over the copied bytes 0x%08x", got, want)
	}
}

// TestChecksumIndexInputDigestsStrings is the regression test for defect E: the
// read side must pull the string through its own ReadByte/ReadBytes so that the
// bytes are digested and the file pointer advances by the full amount. Before
// the fix CheckFooter failed at exactly the string's length short of the truth.
func TestChecksumIndexInputDigestsStrings(t *testing.T) {
	raw := newMemIndexOutput("read.bin")
	out := NewChecksumIndexOutput(raw)
	const s = "a string that must enter the read digest"
	if err := out.WriteString(s); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := out.WriteMapOfStrings(map[string]string{"k": "v"}); err != nil {
		t.Fatalf("WriteMapOfStrings: %v", err)
	}
	if err := out.WriteSetOfStrings([]string{"s"}); err != nil {
		t.Fatalf("WriteSetOfStrings: %v", err)
	}
	payload := append([]byte(nil), raw.buf.Bytes()...)

	in := NewChecksumIndexInput(newMemIndexInput("read.bin", payload))
	back, err := in.ReadString()
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if back != s {
		t.Fatalf("round trip gave %q, want %q", back, s)
	}
	if _, err := in.ReadMapOfStrings(); err != nil {
		t.Fatalf("ReadMapOfStrings: %v", err)
	}
	if _, err := in.ReadSetOfStrings(); err != nil {
		t.Fatalf("ReadSetOfStrings: %v", err)
	}

	fp := in.GetFilePointer()
	if fp != int64(len(payload)) {
		t.Fatalf("file pointer %d did not advance over all %d bytes", fp, len(payload))
	}
	if got, want := in.GetChecksum(), crc32.ChecksumIEEE(payload[:fp]); got != want {
		t.Fatalf("digest 0x%08x != CRC-32 over the bytes read 0x%08x", got, want)
	}
	// The two sides must agree: that is what makes a Gocene-written footer
	// readable by Lucene and a Lucene-written footer readable by Gocene.
	if in.GetChecksum() != out.GetChecksum() {
		t.Fatalf("read digest 0x%08x != write digest 0x%08x", in.GetChecksum(), out.GetChecksum())
	}
}

// TestChecksumIndexInputRejectsBackwardSeek is the regression test for defect G.
// ChecksumIndexInput.seek (ChecksumIndexInput.java:56-64) throws
// IllegalStateException on a backward seek; it never resets the digest.
func TestChecksumIndexInputRejectsBackwardSeek(t *testing.T) {
	payload := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	in := NewChecksumIndexInput(newMemIndexInput("seek.bin", payload))

	if err := in.SetPosition(16); err != nil {
		t.Fatalf("forward seek must be allowed: %v", err)
	}
	if got, want := in.GetChecksum(), crc32.ChecksumIEEE(payload[:16]); got != want {
		t.Fatalf("forward seek must digest what it skips: 0x%08x != 0x%08x", got, want)
	}

	before := in.GetChecksum()
	if err := in.SetPosition(4); err == nil {
		t.Fatal("backward seek was accepted; Lucene throws IllegalStateException")
	}
	if in.GetChecksum() != before {
		t.Fatalf("refused seek reset the digest: 0x%08x -> 0x%08x", before, in.GetChecksum())
	}
	if in.GetFilePointer() != 16 {
		t.Fatalf("refused seek moved the file pointer to %d", in.GetFilePointer())
	}
	if err := in.SetPosition(16); err != nil {
		t.Fatalf("seek to the current position must be a no-op: %v", err)
	}
}
