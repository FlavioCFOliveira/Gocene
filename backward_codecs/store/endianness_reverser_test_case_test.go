// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"

	gstore "github.com/FlavioCFOliveira/Gocene/store"
)

// TestEndiannessReverserDataOutput_WriteShort verifies that WriteShort
// byte-swaps the value before writing.
func TestEndiannessReverserDataOutput_WriteShort(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	raw, err := dir.CreateOutput("dout_short.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	rev := NewEndiannessReverserDataOutput(raw)

	const val int16 = 0x0102
	if err := rev.WriteShort(val); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("dout_short.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	got, err := in.ReadShort()
	if err != nil {
		t.Fatal(err)
	}
	want := int16(reverseBytes16(uint16(val)))
	if got != want {
		t.Errorf("got %#x, want %#x", got, want)
	}
}

// TestEndiannessReverserDataOutput_WriteInt verifies that WriteInt byte-swaps
// the value before writing.
func TestEndiannessReverserDataOutput_WriteInt(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	raw, err := dir.CreateOutput("dout_int.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	rev := NewEndiannessReverserDataOutput(raw)

	const val int32 = 0x01020304
	if err := rev.WriteInt(val); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("dout_int.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	got, err := in.ReadInt()
	if err != nil {
		t.Fatal(err)
	}
	want := int32(reverseBytes32(uint32(val)))
	if got != want {
		t.Errorf("got %#x, want %#x", got, want)
	}
}

// TestEndiannessReverserDataOutput_WriteLong verifies that WriteLong byte-swaps
// the value before writing.
func TestEndiannessReverserDataOutput_WriteLong(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	raw, err := dir.CreateOutput("dout_long.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	rev := NewEndiannessReverserDataOutput(raw)

	const val int64 = 0x0102030405060708
	if err := rev.WriteLong(val); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("dout_long.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	got, err := in.ReadLong()
	if err != nil {
		t.Fatal(err)
	}
	want := int64(reverseBytes64(uint64(val)))
	if got != want {
		t.Errorf("got %#x, want %#x", got, want)
	}
}

// TestEndiannessReverserDataOutput_WriteByteAndBytes verifies that single bytes
// and byte slices pass through unchanged.
func TestEndiannessReverserDataOutput_WriteByteAndBytes(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	raw, err := dir.CreateOutput("dout_bytes.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	rev := NewEndiannessReverserDataOutput(raw)

	if err := rev.WriteByte(0xAB); err != nil {
		t.Fatal(err)
	}
	if err := rev.WriteBytes([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("dout_bytes.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	b, err := in.ReadByte()
	if err != nil {
		t.Fatal(err)
	}
	if b != 0xAB {
		t.Errorf("ReadByte: got %#x want 0xAB", b)
	}

	rest := make([]byte, 5)
	if err := in.ReadBytes(rest); err != nil {
		t.Fatal(err)
	}
	if string(rest) != "hello" {
		t.Errorf("ReadBytes: got %q want %q", rest, "hello")
	}
}

// TestEndiannessReverserDataInput_ReadShort verifies that ReadShort on the data
// input wrapper byte-swaps correctly.
func TestEndiannessReverserDataInput_ReadShort(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	// Write LE shorts using a normal output.
	out, err := dir.CreateOutput("din_short.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteShort(int16(0x0102)); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteShort(int16(0x0304)); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("din_short.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	rev := NewEndiannessReverserDataInput(in)
	// First short: raw LE = 0x0102 → reversed = 0x0201
	got, err := rev.ReadShort()
	if err != nil {
		t.Fatal(err)
	}
	if want := int16(0x0201); got != want {
		t.Errorf("first short: got %#x want %#x", got, want)
	}
	// Second short: raw LE = 0x0304 → reversed = 0x0403
	got, err = rev.ReadShort()
	if err != nil {
		t.Fatal(err)
	}
	if want := int16(0x0403); got != want {
		t.Errorf("second short: got %#x want %#x", got, want)
	}
}

// TestEndiannessReverserDataInput_ReadInt verifies ReadInt byte-swapping.
func TestEndiannessReverserDataInput_ReadInt(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("din_int.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	const leVal int32 = 0x01020304
	if err := out.WriteInt(leVal); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("din_int.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	rev := NewEndiannessReverserDataInput(in)
	got, err := rev.ReadInt()
	if err != nil {
		t.Fatal(err)
	}
	if want := int32(0x04030201); got != want {
		t.Errorf("got %#x want %#x", got, want)
	}
}

// TestEndiannessReverserDataInput_ReadLong verifies ReadLong byte-swapping.
func TestEndiannessReverserDataInput_ReadLong(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("din_long.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	const leVal int64 = 0x0102030405060708
	if err := out.WriteLong(leVal); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("din_long.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	rev := NewEndiannessReverserDataInput(in)
	got, err := rev.ReadLong()
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(0x0807060504030201); got != want {
		t.Errorf("got %#x want %#x", got, want)
	}
}

// TestEndiannessReverserDataInput_ReadByteAndBytes verifies that single bytes
// and byte slices pass through unchanged.
func TestEndiannessReverserDataInput_ReadByteAndBytes(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("din_bytes.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteByte(0xAB); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteBytes([]byte("world")); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("din_bytes.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	rev := NewEndiannessReverserDataInput(in)
	b, err := rev.ReadByte()
	if err != nil {
		t.Fatal(err)
	}
	if b != 0xAB {
		t.Errorf("ReadByte: got %#x want 0xAB", b)
	}
	rest := make([]byte, 5)
	if err := rev.ReadBytes(rest); err != nil {
		t.Fatal(err)
	}
	if string(rest) != "world" {
		t.Errorf("ReadBytes: got %q want %q", rest, "world")
	}
}

// TestEndiannessReverserDataInput_ReadString verifies string pass-through.
func TestEndiannessReverserDataInput_ReadString(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("din_str.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteString("hello endianness"); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("din_str.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	rev := NewEndiannessReverserDataInput(in)
	got, err := rev.ReadString()
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello endianness" {
		t.Errorf("ReadString: got %q want %q", got, "hello endianness")
	}
}
