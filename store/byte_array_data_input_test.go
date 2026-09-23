// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// Port of lucene/core/src/test/org/apache/lucene/store/TestByteArrayDataInput.java
// (Apache Lucene 10.5.0).

import (
	"encoding/binary"
	"testing"
)

func TestByteArrayDataInput_Basic(t *testing.T) {
	bytes := []byte{1, 65}
	in := NewByteArrayDataInput(bytes)
	s, err := in.ReadString()
	if err != nil {
		t.Fatalf("readString: %v", err)
	}
	if s != "A" {
		t.Fatalf("readString: got %q, want %q", s, "A")
	}
	if !in.EOF() {
		t.Fatal("expected eof")
	}

	bytes = []byte{1, 1, 65}
	in.ResetWithSlice(bytes, 1, 2)
	s, err = in.ReadString()
	if err != nil {
		t.Fatalf("readString: %v", err)
	}
	if s != "A" {
		t.Fatalf("readString: got %q, want %q", s, "A")
	}
	if !in.EOF() {
		t.Fatal("expected eof")
	}
}

func TestByteArrayDataInput_Datatypes(t *testing.T) {
	// write some primitives using ByteArrayDataOutput:
	bytes := make([]byte, 32)
	out := NewByteArrayDataOutput(bytes)
	mustNoErr(t, out.WriteByte(43))
	mustNoErr(t, out.WriteShort(12345))
	mustNoErr(t, out.WriteInt(1234567890))
	mustNoErr(t, out.WriteLong(1234567890123456789))
	size := out.GetPosition()
	if size != 15 {
		t.Fatalf("size: got %d, want 15", size)
	}

	// read the primitives using a little-endian decoder to ensure encoding in
	// byte array is LE:
	buf := bytes[:size]
	if buf[0] != 43 {
		t.Fatalf("byte: got %d, want 43", buf[0])
	}
	if v := int16(binary.LittleEndian.Uint16(buf[1:])); v != 12345 {
		t.Fatalf("short: got %d, want 12345", v)
	}
	if v := int32(binary.LittleEndian.Uint32(buf[3:])); v != 1234567890 {
		t.Fatalf("int: got %d, want 1234567890", v)
	}
	if v := int64(binary.LittleEndian.Uint64(buf[7:])); v != 1234567890123456789 {
		t.Fatalf("long: got %d, want 1234567890123456789", v)
	}
	if remaining := size - 15; remaining != 0 {
		t.Fatalf("remaining: got %d, want 0", remaining)
	}

	// read the primitives using ByteArrayDataInput:
	in := NewByteArrayDataInputWithOffset(bytes, 0, size)
	b, err := in.ReadByte()
	mustNoErr(t, err)
	if b != 43 {
		t.Fatalf("readByte: got %d, want 43", b)
	}
	sh, err := in.ReadShort()
	mustNoErr(t, err)
	if sh != 12345 {
		t.Fatalf("readShort: got %d, want 12345", sh)
	}
	i, err := in.ReadInt()
	mustNoErr(t, err)
	if i != 1234567890 {
		t.Fatalf("readInt: got %d, want 1234567890", i)
	}
	l, err := in.ReadLong()
	mustNoErr(t, err)
	if l != 1234567890123456789 {
		t.Fatalf("readLong: got %d, want 1234567890123456789", l)
	}
	if !in.EOF() {
		t.Fatal("expected eof")
	}
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
