// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestCodecUtil_HeaderLength(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	codecName := "FooBar"
	version := int32(5)
	if err := codecs.WriteHeader(out, codecName, version); err != nil {
		t.Fatal(err)
	}

	data := "this is the data"
	if err := store.WriteString(out, data); err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	headerLen := codecs.HeaderLength(codecName)
	if err := in.SetPosition(int64(headerLen)); err != nil {
		t.Fatal(err)
	}

	readData, err := store.ReadString(in)
	if err != nil {
		t.Fatal(err)
	}

	if readData != data {
		t.Errorf("Expected %s, got %s", data, readData)
	}
}

func TestCodecUtil_WriteTooLongHeader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	tooLong := strings.Repeat("a", 128)
	err = codecs.WriteHeader(out, tooLong, 5)
	if err == nil {
		t.Error("Expected error for too long header name, got nil")
	}
}

func TestCodecUtil_WriteNonAsciiHeader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	err = codecs.WriteHeader(out, "\u1234", 5)
	if err == nil {
		t.Error("Expected error for non-ascii header name, got nil")
	}
}

func TestCodecUtil_ReadHeaderWrongMagic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteInt32(out, 1234); err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	_, err = codecs.CheckHeader(in, "bogus", 1, 1)
	if err == nil {
		t.Error("Expected error for wrong magic number, got nil")
	}
}

func TestCodecUtil_ChecksumEntireFile(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	// Use ChecksumIndexOutput to compute CRC as we write
	checksumOut := store.NewChecksumIndexOutput(out)

	if err := codecs.WriteHeader(checksumOut, "FooBar", 5); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteString(checksumOut, "this is the data"); err != nil {
		t.Fatal(err)
	}
	if err := codecs.WriteFooter(checksumOut); err != nil {
		t.Fatal(err)
	}
	checksumOut.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	_, err = codecs.ChecksumEntireFile(in)
	if err != nil {
		t.Errorf("Checksum validation failed: %v", err)
	}
}

func TestCodecUtil_CheckFooterValid(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}
	checksumOut := store.NewChecksumIndexOutput(out)

	if err := codecs.WriteHeader(checksumOut, "FooBar", 5); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteString(checksumOut, "this is the data"); err != nil {
		t.Fatal(err)
	}
	if err := codecs.WriteFooter(checksumOut); err != nil {
		t.Fatal(err)
	}
	checksumOut.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	checksumIn := store.NewChecksumIndexInput(in)
	defer checksumIn.Close()

	// Forward seek must process bytes to update checksum
	if err := checksumIn.SetPosition(checksumIn.Length() - 16); err != nil {
		t.Fatal(err)
	}

	_, err = codecs.CheckFooter(checksumIn)
	if err != nil {
		t.Errorf("Footer validation failed: %v", err)
	}
}

func TestCodecUtil_CheckFooterValidAtFooter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}
	checksumOut := store.NewChecksumIndexOutput(out)

	if err := codecs.WriteHeader(checksumOut, "FooBar", 5); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteString(checksumOut, "this is the data"); err != nil {
		t.Fatal(err)
	}
	if err := codecs.WriteFooter(checksumOut); err != nil {
		t.Fatal(err)
	}
	checksumOut.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	checksumIn := store.NewChecksumIndexInput(in)
	defer checksumIn.Close()

	// Read everything
	if _, err := codecs.CheckHeader(checksumIn, "FooBar", 5, 5); err != nil {
		t.Fatal(err)
	}
	if data, err := store.ReadString(checksumIn); err != nil || data != "this is the data" {
		t.Fatalf("Read error or data mismatch: %v", err)
	}

	// Now we are exactly at the footer
	_, err = codecs.CheckFooter(checksumIn)
	if err != nil {
		t.Errorf("Footer validation failed: %v", err)
	}
}

func TestCodecUtil_CheckFooterInvalid(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	if err := codecs.WriteHeader(out, "FooBar", 5); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteString(out, "this is the data"); err != nil {
		t.Fatal(err)
	}
	// Manual bogus footer
	if err := store.WriteInt32(out, codecs.FOOTER_MAGIC); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteInt32(out, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteInt64(out, 1234567); err != nil { // bogus checksum
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	checksumIn := store.NewChecksumIndexInput(in)
	defer checksumIn.Close()

	if err := checksumIn.SetPosition(checksumIn.Length() - 16); err != nil {
		t.Fatal(err)
	}

	_, err = codecs.CheckFooter(checksumIn)
	if err == nil {
		t.Error("Expected error for invalid checksum, got nil")
	}
}

func TestCodecUtil_SegmentHeaderLength(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	id := make([]byte, 16)
	rand.Read(id)
	suffix := "xyz"

	if err := codecs.WriteIndexHeader(out, "FooBar", 5, id, suffix); err != nil {
		t.Fatal(err)
	}

	data := "this is the data"
	if err := store.WriteString(out, data); err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	headerLen := codecs.IndexHeaderLength("FooBar", suffix)
	if err := in.SetPosition(int64(headerLen)); err != nil {
		t.Fatal(err)
	}

	readData, err := store.ReadString(in)
	if err != nil {
		t.Fatal(err)
	}

	if readData != data {
		t.Errorf("Expected %s, got %s", data, readData)
	}
}

func TestCodecUtil_WriteTooLongSuffix(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	tooLong := strings.Repeat("a", 256)
	id := make([]byte, 16)
	err = codecs.WriteIndexHeader(out, "foobar", 5, id, tooLong)
	if err == nil {
		t.Error("Expected error for too long suffix, got nil")
	}
}

func TestCodecUtil_WriteVeryLongSuffix(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}

	justLongEnough := strings.Repeat("a", 255)
	id := make([]byte, 16)
	rand.Read(id)

	if err := codecs.WriteIndexHeader(out, "foobar", 5, id, justLongEnough); err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	_, err = codecs.CheckIndexHeader(in, "foobar", 5, 5, id, justLongEnough)
	if err != nil {
		t.Fatalf("Header validation failed: %v", err)
	}

	if in.GetFilePointer() != in.Length() {
		t.Errorf("Expected file pointer at end, got %d", in.GetFilePointer())
	}
}

func TestCodecUtil_ReadBogusCRC(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}
	store.WriteInt64(out, -1)
	store.WriteInt64(out, 1<<32)
	store.WriteInt64(out, -(1 << 32))
	store.WriteInt64(out, (1<<32)-1)
	out.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	for i := 0; i < 3; i++ {
		_, err := codecs.ReadCRC(in)
		if err == nil {
			t.Errorf("Expected error for bogus CRC at index %d, got nil", i)
		}
	}

	crc, err := codecs.ReadCRC(in)
	if err != nil {
		t.Errorf("Unexpected error for valid CRC: %v", err)
	}
	if crc != (1<<32)-1 {
		t.Errorf("Expected %d, got %d", uint32((1<<32)-1), crc)
	}
}

func TestCodecUtil_TruncatedFile(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("temp", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("temp", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	_, err = codecs.ChecksumEntireFile(in)
	if err == nil {
		t.Error("Expected error for truncated file, got nil")
	}

	_, err = codecs.RetrieveChecksum(in)
	if err == nil {
		t.Error("Expected error for truncated file, got nil")
	}
}

func TestCodecUtil_RetrieveChecksum(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("foo", store.IOContextWrite)
	if err != nil {
		t.Fatal(err)
	}
	checksumOut := store.NewChecksumIndexOutput(out)
	checksumOut.WriteByte(42)
	codecs.WriteFooter(checksumOut)
	checksumOut.Close()

	in, err := dir.OpenInput("foo", store.IOContextRead)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	_, err = codecs.RetrieveChecksum(in)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
