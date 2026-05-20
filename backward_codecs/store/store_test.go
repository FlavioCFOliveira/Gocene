// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"math/rand"
	"testing"

	gstore "github.com/FlavioCFOliveira/Gocene/store"
)

// openDir returns a SimpleFSDirectory backed by a temp dir.
func openDir(t *testing.T) *gstore.SimpleFSDirectory {
	t.Helper()
	dir, err := gstore.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	return dir
}

// ─────────────────────────────────────────────────────────────────────────────
// EndiannessReverserIndexInput / IndexOutput
// ─────────────────────────────────────────────────────────────────────────────

func TestEndiannessReverserIndexInput_ReadShort(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	const n = 30
	values := make([]int16, n)
	rng := rand.New(rand.NewSource(42))
	for i := range values {
		values[i] = int16(rng.Int31())
	}

	// Write raw (LE) shorts using a normal output.
	out, err := dir.CreateOutput("test_short.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	for _, v := range values {
		if err := out.WriteShort(v); err != nil {
			t.Fatalf("WriteShort: %v", err)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	// Read back normally (LE) — baseline.
	plain, err := dir.OpenInput("test_short.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput plain: %v", err)
	}
	defer plain.Close()

	// Read via endianness-reversing wrapper.
	wrapped := NewEndiannessReverserIndexInput(func() gstore.IndexInput {
		in, err := dir.OpenInput("test_short.dat", ctx)
		if err != nil {
			t.Fatalf("OpenInput wrapped: %v", err)
		}
		return in
	}())
	defer wrapped.Close()

	for i, want := range values {
		got, err := plain.ReadShort()
		if err != nil {
			t.Fatalf("[%d] plain ReadShort: %v", i, err)
		}
		rev, err := wrapped.ReadShort()
		if err != nil {
			t.Fatalf("[%d] wrapped ReadShort: %v", i, err)
		}
		// The wrapper byte-swaps on read, so reverting gives the original.
		if reverseBytes16(uint16(rev)) != uint16(got) {
			t.Errorf("[%d] want reverseBytes(wrapped)==%d, got plain=%d", i, reverseBytes16(uint16(rev)), got)
		}
		// Directly: wrapper should return the byte-swapped value.
		if rev != int16(reverseBytes16(uint16(want))) {
			t.Errorf("[%d] want wrapper=%d, got %d", i, int16(reverseBytes16(uint16(want))), rev)
		}
	}
}

func TestEndiannessReverserIndexInput_ReadInt(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	const n = 30
	rng := rand.New(rand.NewSource(43))
	values := make([]int32, n)
	for i := range values {
		values[i] = rng.Int31()
	}

	out, err := dir.CreateOutput("test_int.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	for _, v := range values {
		if err := out.WriteInt(v); err != nil {
			t.Fatalf("WriteInt: %v", err)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	plain, err := dir.OpenInput("test_int.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput plain: %v", err)
	}
	defer plain.Close()

	wrapped := NewEndiannessReverserIndexInput(func() gstore.IndexInput {
		in, err := dir.OpenInput("test_int.dat", ctx)
		if err != nil {
			t.Fatalf("OpenInput wrapped: %v", err)
		}
		return in
	}())
	defer wrapped.Close()

	for i, want := range values {
		got, err := plain.ReadInt()
		if err != nil {
			t.Fatalf("[%d] plain ReadInt: %v", i, err)
		}
		rev, err := wrapped.ReadInt()
		if err != nil {
			t.Fatalf("[%d] wrapped ReadInt: %v", i, err)
		}
		// plain reads LE; reversal should equal plain
		if reverseBytes32(uint32(rev)) != uint32(got) {
			t.Errorf("[%d] reverseBytes(wrapped)=%d != plain=%d", i, reverseBytes32(uint32(rev)), got)
		}
		_ = want
	}
}

func TestEndiannessReverserIndexInput_ReadLong(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	const n = 30
	rng := rand.New(rand.NewSource(44))
	values := make([]int64, n)
	for i := range values {
		values[i] = rng.Int63()
	}

	out, err := dir.CreateOutput("test_long.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	for _, v := range values {
		if err := out.WriteLong(v); err != nil {
			t.Fatalf("WriteLong: %v", err)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	plain, err := dir.OpenInput("test_long.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput plain: %v", err)
	}
	defer plain.Close()

	wrapped := NewEndiannessReverserIndexInput(func() gstore.IndexInput {
		in, err := dir.OpenInput("test_long.dat", ctx)
		if err != nil {
			t.Fatalf("OpenInput wrapped: %v", err)
		}
		return in
	}())
	defer wrapped.Close()

	for i, want := range values {
		got, err := plain.ReadLong()
		if err != nil {
			t.Fatalf("[%d] plain ReadLong: %v", i, err)
		}
		rev, err := wrapped.ReadLong()
		if err != nil {
			t.Fatalf("[%d] wrapped ReadLong: %v", i, err)
		}
		if reverseBytes64(uint64(rev)) != uint64(got) {
			t.Errorf("[%d] reverseBytes(wrapped)=%d != plain=%d", i, reverseBytes64(uint64(rev)), got)
		}
		_ = want
	}
}

// TestEndiannessReverserIndexOutput writes via the reversing output and reads
// back the raw bytes, verifying the byte-swap is applied on the wire.
func TestEndiannessReverserIndexOutput_WriteShort(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	raw, err := dir.CreateOutput("rev_short.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	rev := NewEndiannessReverserIndexOutput(raw)

	const val int16 = 0x0102
	if err := rev.WriteShort(val); err != nil {
		t.Fatalf("WriteShort: %v", err)
	}
	if err := rev.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("rev_short.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	got, err := in.ReadShort()
	if err != nil {
		t.Fatalf("ReadShort: %v", err)
	}
	// The output wrote BE bytes, so raw read (LE) returns the reversed value.
	want := int16(reverseBytes16(uint16(val)))
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestEndiannessReverserIndexOutput_WriteInt(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	raw, err := dir.CreateOutput("rev_int.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	rev := NewEndiannessReverserIndexOutput(raw)

	const val int32 = 0x01020304
	if err := rev.WriteInt(val); err != nil {
		t.Fatalf("WriteInt: %v", err)
	}
	if err := rev.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("rev_int.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	got, err := in.ReadInt()
	if err != nil {
		t.Fatalf("ReadInt: %v", err)
	}
	want := int32(reverseBytes32(uint32(val)))
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestEndiannessReverserIndexOutput_WriteLong(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	raw, err := dir.CreateOutput("rev_long.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	rev := NewEndiannessReverserIndexOutput(raw)

	const val int64 = 0x0102030405060708
	if err := rev.WriteLong(val); err != nil {
		t.Fatalf("WriteLong: %v", err)
	}
	if err := rev.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("rev_long.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	got, err := in.ReadLong()
	if err != nil {
		t.Fatalf("ReadLong: %v", err)
	}
	want := int64(reverseBytes64(uint64(val)))
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

// TestEndiannessReverserChecksumIndexInput verifies that bytes read through
// the checksum wrapper accumulate the checksum correctly.
func TestEndiannessReverserChecksumIndexInput_Checksum(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	// Write a single byte so we can compare checksums.
	out, err := dir.CreateOutput("csum.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteByte(0xAB); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	plain, err := dir.OpenInput("csum.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}

	csum := NewEndiannessReverserChecksumIndexInput(plain)
	b, err := csum.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0xAB {
		t.Errorf("got %x, want 0xAB", b)
	}
	// Checksum must be non-zero after reading one byte.
	if csum.GetChecksum() == 0 {
		t.Error("expected non-zero checksum after reading one byte")
	}
	if err := csum.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestEndiannessReverserChecksumIndexInput_ReadShort verifies that short values
// are byte-swapped while the raw bytes are checksummed.
func TestEndiannessReverserChecksumIndexInput_ReadShort(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	const val int16 = 0x0102
	out, err := dir.CreateOutput("csum_short.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteShort(val); err != nil {
		t.Fatalf("WriteShort: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	plain, err := dir.OpenInput("csum_short.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}

	csum := NewEndiannessReverserChecksumIndexInput(plain)
	got, err := csum.ReadShort()
	if err != nil {
		t.Fatalf("ReadShort: %v", err)
	}
	if err := csum.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Output wrote LE 0x0102, reverser reads 0x0201.
	want := int16(reverseBytes16(uint16(val)))
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// EndiannessReverserUtil factory functions
// ─────────────────────────────────────────────────────────────────────────────

func TestEndiannessReverserUtil_OpenInputAndCreateOutput(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	// Write via reversing output factory.
	out, err := CreateOutput(dir, "util_test.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	const val = int32(-559038737) // 0xDEADBEEF as signed
	if err := out.WriteInt(val); err != nil {
		t.Fatalf("WriteInt: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Read back via reversing input factory — should get original value.
	in, err := OpenInput(dir, "util_test.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	got, err := in.ReadInt()
	if err != nil {
		t.Fatalf("ReadInt: %v", err)
	}
	if got != val {
		t.Errorf("round-trip: got 0x%08X, want 0x%08X", got, val)
	}
}

func TestEndiannessReverserUtil_OpenChecksumInput(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("csum_util.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteByte(0xFF); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	csum, err := OpenChecksumInput(dir, "csum_util.dat", ctx)
	if err != nil {
		t.Fatalf("OpenChecksumInput: %v", err)
	}
	defer csum.Close()

	b, err := csum.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0xFF {
		t.Errorf("got %x, want 0xFF", b)
	}
	if csum.GetChecksum() == 0 {
		t.Error("expected non-zero checksum")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func reverseBytes16(v uint16) uint16 { return (v>>8)&0xFF | (v&0xFF)<<8 }
func reverseBytes32(v uint32) uint32 {
	return (v>>24)&0xFF | (v>>8)&0xFF00 | (v&0xFF00)<<8 | (v&0xFF)<<24
}
func reverseBytes64(v uint64) uint64 {
	lo := uint64(reverseBytes32(uint32(v >> 32)))
	hi := uint64(reverseBytes32(uint32(v)))
	return hi<<32 | lo
}
