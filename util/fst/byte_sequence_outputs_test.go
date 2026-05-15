// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func brOf(s string) *util.BytesRef {
	b := []byte(s)
	return &util.BytesRef{Bytes: b, Offset: 0, Length: len(b)}
}

func TestByteSequenceOutputsSingleton(t *testing.T) {
	if ByteSequenceOutputs() != ByteSequenceOutputs() {
		t.Fatal("singleton identity broken")
	}
}

func TestByteSequenceOutputsCommon(t *testing.T) {
	o := ByteSequenceOutputs()
	no := o.GetNoOutput()

	if got := o.Common(brOf("foobar"), brOf("food")); string(got.Bytes[got.Offset:got.Offset+got.Length]) != "foo" {
		t.Fatalf("Common: want 'foo', got %q", got.String())
	}
	if got := o.Common(brOf("abc"), brOf("xyz")); got != no {
		t.Fatalf("Common: no common prefix should return NO_OUTPUT singleton")
	}
	a := brOf("hello")
	if got := o.Common(a, brOf("hello")); !bytes.Equal(got.ValidBytes(), []byte("hello")) {
		t.Fatalf("Common: equal strings should give the full prefix")
	}
	// One is prefix of the other.
	pfx := brOf("foo")
	if got := o.Common(pfx, brOf("foobar")); got != pfx {
		t.Fatalf("Common: shorter is prefix => return shorter (output1)")
	}
}

func TestByteSequenceOutputsSubtract(t *testing.T) {
	o := ByteSequenceOutputs()
	no := o.GetNoOutput()

	full := brOf("foobar")
	pfx := brOf("foo")
	got := o.Subtract(full, pfx)
	if !bytes.Equal(got.ValidBytes(), []byte("bar")) {
		t.Fatalf("Subtract: want 'bar', got %q", got.String())
	}
	if got := o.Subtract(full, no); got != full {
		t.Fatalf("Subtract by NO_OUTPUT should return original ref")
	}
	if got := o.Subtract(full, brOf("foobar")); got != no {
		t.Fatalf("Subtract entire string should return NO_OUTPUT")
	}
}

func TestByteSequenceOutputsAdd(t *testing.T) {
	o := ByteSequenceOutputs()
	no := o.GetNoOutput()
	if got := o.Add(brOf("foo"), brOf("bar")); !bytes.Equal(got.ValidBytes(), []byte("foobar")) {
		t.Fatalf("Add: want 'foobar', got %q", got.String())
	}
	pfx := brOf("foo")
	if got := o.Add(no, pfx); got != pfx {
		t.Fatalf("Add(NO_OUTPUT, x) should return x")
	}
	if got := o.Add(pfx, no); got != pfx {
		t.Fatalf("Add(x, NO_OUTPUT) should return x")
	}
}

func TestByteSequenceOutputsWriteReadRoundTrip(t *testing.T) {
	o := ByteSequenceOutputs()
	cases := [][]byte{
		{},
		{0x00},
		{0xFF, 0xAA},
		[]byte("a long-ish payload with many bytes"),
	}
	for _, payload := range cases {
		out := store.NewByteArrayDataOutput(64)
		in := &util.BytesRef{Bytes: payload, Offset: 0, Length: len(payload)}
		if err := o.Write(in, out); err != nil {
			t.Fatalf("Write: %v", err)
		}
		di := store.NewByteArrayDataInput(out.GetBytes())
		got, err := o.Read(di)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if !bytes.Equal(got.ValidBytes(), payload) {
			t.Fatalf("round-trip mismatch: want %x got %x", payload, got.ValidBytes())
		}
	}
}

func TestByteSequenceOutputsByteFormatFixture(t *testing.T) {
	// Hard-coded byte format fixture: writing a 4-byte payload {0xDE,0xAD,0xBE,0xEF}
	// must produce exactly [0x04, 0xDE, 0xAD, 0xBE, 0xEF] (VInt(4) + 4 bytes).
	// This catches drift in either the VInt encoding or the raw byte writer.
	o := ByteSequenceOutputs()
	out := store.NewByteArrayDataOutput(8)
	payload := &util.BytesRef{Bytes: []byte{0xDE, 0xAD, 0xBE, 0xEF}, Offset: 0, Length: 4}
	if err := o.Write(payload, out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := []byte{0x04, 0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(out.GetBytes(), want) {
		t.Fatalf("byte format drift: want % x got % x", want, out.GetBytes())
	}

	// And empty payload encodes as a single 0x00 byte.
	out2 := store.NewByteArrayDataOutput(2)
	if err := o.Write(o.GetNoOutput(), out2); err != nil {
		t.Fatalf("Write empty: %v", err)
	}
	if !bytes.Equal(out2.GetBytes(), []byte{0x00}) {
		t.Fatalf("empty encoding: want [00] got % x", out2.GetBytes())
	}
}

func TestByteSequenceOutputsSkipOutput(t *testing.T) {
	o := ByteSequenceOutputs()
	out := store.NewByteArrayDataOutput(16)
	if err := o.Write(brOf("payload-here"), out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := out.WriteByte(0x42); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	if err := o.SkipOutput(in); err != nil {
		t.Fatalf("SkipOutput: %v", err)
	}
	b, err := in.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte after skip: %v", err)
	}
	if b != 0x42 {
		t.Fatalf("post-skip byte: got 0x%02X want 0x42", b)
	}
}

func TestByteSequenceOutputsString(t *testing.T) {
	if got := ByteSequenceOutputsSingleton().String(); got != "ByteSequenceOutputs" {
		t.Fatalf("String() = %q", got)
	}
}
