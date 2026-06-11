// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"

	gstore "github.com/FlavioCFOliveira/Gocene/store"
)

// TestEndiannessReverserChecksumIndexInput_Clone verifies that Clone returns nil.
func TestEndiannessReverserChecksumIndexInput_Clone(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("clone_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteByte(0x01); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("clone_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}

	csum := NewEndiannessReverserChecksumIndexInput(in)
	if cl := csum.Clone(); cl != nil {
		t.Error("Clone: expected nil")
	}
	if err := csum.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestEndiannessReverserChecksumIndexInput_ReadBytesN verifies ReadBytesN
// pass-through.
func TestEndiannessReverserChecksumIndexInput_ReadBytesN(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("rbn_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("readbytesn test")
	if err := out.WriteBytes(data); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("rbn_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := NewEndiannessReverserChecksumIndexInput(in)
	got, err := csum.ReadBytesN(len(data))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("ReadBytesN: got %q want %q", got, data)
	}
	if csum.GetChecksum() == 0 {
		t.Error("expected non-zero checksum after reading bytes")
	}
}

// TestEndiannessReverserChecksumIndexInput_ReadBytes verifies ReadBytes
// pass-through.
func TestEndiannessReverserChecksumIndexInput_ReadBytes(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("rb_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("readbytes")
	if err := out.WriteBytes(data); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("rb_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := NewEndiannessReverserChecksumIndexInput(in)
	buf := make([]byte, len(data))
	if err := csum.ReadBytes(buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != string(data) {
		t.Errorf("ReadBytes: got %q want %q", buf, data)
	}
}

// TestEndiannessReverserChecksumIndexInput_ReadString verifies string
// pass-through.
func TestEndiannessReverserChecksumIndexInput_ReadString(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("rs_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteString("checksum string test"); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("rs_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := NewEndiannessReverserChecksumIndexInput(in)
	got, err := csum.ReadString()
	if err != nil {
		t.Fatal(err)
	}
	if got != "checksum string test" {
		t.Errorf("ReadString: got %q want %q", got, "checksum string test")
	}
}

// TestEndiannessReverserChecksumIndexInput_FilePointer verifies the
// GetFilePointer and Length pass-through.
func TestEndiannessReverserChecksumIndexInput_FilePointer(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("fp_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteInt(0x01020304); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteInt(0x05060708); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("fp_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := NewEndiannessReverserChecksumIndexInput(in)

	if got := csum.Length(); got != 8 {
		t.Errorf("Length: got %d want 8", got)
	}
	if got := csum.GetFilePointer(); got != 0 {
		t.Errorf("GetFilePointer: got %d want 0", got)
	}
	// Read first int (4 bytes), then check pointer.
	if _, err := csum.ReadInt(); err != nil {
		t.Fatal(err)
	}
	if got := csum.GetFilePointer(); got != 4 {
		t.Errorf("GetFilePointer after ReadInt: got %d want 4", got)
	}
}

// TestEndiannessReverserChecksumIndexInput_ReadInt verifies that ReadInt
// byte-swaps while the raw bytes are checksummed.
func TestEndiannessReverserChecksumIndexInput_ReadInt(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	const leVal int32 = 0x01020304
	out, err := dir.CreateOutput("csum_int.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteInt(leVal); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("csum_int.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := NewEndiannessReverserChecksumIndexInput(in)
	got, err := csum.ReadInt()
	if err != nil {
		t.Fatal(err)
	}
	want := int32(reverseBytes32(uint32(leVal)))
	if got != want {
		t.Errorf("ReadInt: got %#x want %#x", got, want)
	}
	if csum.GetChecksum() == 0 {
		t.Error("expected non-zero checksum after reading int")
	}
}

// TestEndiannessReverserChecksumIndexInput_ReadLong verifies that ReadLong
// byte-swaps while the raw bytes are checksummed.
func TestEndiannessReverserChecksumIndexInput_ReadLong(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	const leVal int64 = 0x1122334455667788
	out, err := dir.CreateOutput("csum_long.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteLong(leVal); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("csum_long.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := NewEndiannessReverserChecksumIndexInput(in)
	got, err := csum.ReadLong()
	if err != nil {
		t.Fatal(err)
	}
	want := int64(reverseBytes64(uint64(leVal)))
	if got != want {
		t.Errorf("ReadLong: got %#x want %#x", got, want)
	}
	if csum.GetChecksum() == 0 {
		t.Error("expected non-zero checksum after reading long")
	}
}

// TestEndiannessReverserChecksumIndexInput_Slice_failsWithUnsupported verifies
// that Slice propagates the underlying BufferedChecksumIndexInput error since
// that wrapper does not support Clone or Slice.
func TestEndiannessReverserChecksumIndexInput_Slice_failsWithUnsupported(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("slice_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteInt(0x01020304); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("slice_test.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := NewEndiannessReverserChecksumIndexInput(in)
	_, err = csum.Slice("test_slice", 0, 4)
	if err == nil {
		t.Fatal("expected error from Slice (BufferedChecksumIndexInput does not support it)")
	}
}

// TestEndiannessReverserChecksumIndexInput_ReadMultipleTypes verifies that
// reading multiple types sequentially accumulates the checksum properly.
func TestEndiannessReverserChecksumIndexInput_ReadMultipleTypes(t *testing.T) {
	dir := gstore.NewByteBuffersDirectory()
	defer dir.Close()
	ctx := gstore.IOContext{Context: gstore.ContextRead}

	out, err := dir.CreateOutput("multi.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteByte(0xAA); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteShort(int16(0x1122)); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("multi.dat", ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := NewEndiannessReverserChecksumIndexInput(in)
	if _, err := csum.ReadByte(); err != nil {
		t.Fatal(err)
	}
	if _, err := csum.ReadShort(); err != nil {
		t.Fatal(err)
	}
	if csum.GetChecksum() == 0 {
		t.Error("expected non-zero checksum after reading byte+short")
	}
}
