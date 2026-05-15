// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compress

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestLowercaseAsciiCompression_RoundTripAllLowercase(t *testing.T) {
	t.Parallel()
	in := []byte("hello.world.example.com.with.many.dots")
	out := store.NewByteArrayDataOutput(64)
	tmp := make([]byte, len(in))
	ok, err := Compress(in, len(in), tmp, out)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if !ok {
		t.Fatal("Compress refused all-lowercase input; expected success")
	}
	compressed := out.GetBytes()
	if len(compressed) >= len(in) {
		t.Errorf("compressed (%d bytes) should be smaller than input (%d bytes)", len(compressed), len(in))
	}

	dec := store.NewByteArrayDataInput(compressed)
	restored := make([]byte, len(in))
	if err := Decompress(dec, restored, len(in)); err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(restored, in) {
		t.Errorf("round-trip mismatch:\n  want %q\n  got  %q", in, restored)
	}
}

func TestLowercaseAsciiCompression_RoundTripWithExceptions(t *testing.T) {
	t.Parallel()
	// Mostly lowercase with a couple of capital exceptions interspersed.
	in := []byte("the.quick.brown.fox.jumps.over.the.lazy.dog.now.go.X.faster.Y.now")
	out := store.NewByteArrayDataOutput(64)
	tmp := make([]byte, len(in))
	ok, err := Compress(in, len(in), tmp, out)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if !ok {
		t.Fatal("Compress refused mostly-lowercase input; expected success")
	}
	dec := store.NewByteArrayDataInput(out.GetBytes())
	restored := make([]byte, len(in))
	if err := Decompress(dec, restored, len(in)); err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(restored, in) {
		t.Errorf("round-trip mismatch:\n  want %q\n  got  %q", in, restored)
	}
}

func TestLowercaseAsciiCompression_FailsForUncompressible(t *testing.T) {
	t.Parallel()
	// All upper-half ASCII (high bit set) -> none compressible -> too many exceptions.
	in := make([]byte, 64)
	for i := range in {
		in[i] = 0x80
	}
	out := store.NewByteArrayDataOutput(64)
	tmp := make([]byte, len(in))
	ok, err := Compress(in, len(in), tmp, out)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if ok {
		t.Error("Compress should have refused all-uncompressible input")
	}
}

func TestLowercaseAsciiCompression_FailsForShortInput(t *testing.T) {
	t.Parallel()
	in := []byte("abcde") // < 8 bytes
	out := store.NewByteArrayDataOutput(64)
	tmp := make([]byte, len(in))
	ok, err := Compress(in, len(in), tmp, out)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if ok {
		t.Error("Compress should have refused input shorter than 8 bytes")
	}
}

func TestIsCompressible(t *testing.T) {
	t.Parallel()
	// Digits 0-9 are in [0x1F, 0x3F) when offset by 1.
	for _, b := range []int{'0', '5', '9', 'a', 'm', 'z', '.', '_', '-'} {
		if !isCompressible(b) {
			t.Errorf("isCompressible(0x%02x) = false, want true", b)
		}
	}
	// 0x20 (space) IS in [0x1F, 0x3F) and so is compressible per Lucene.
	for _, b := range []int{0x00, 0x1E, 'A', 'M', 'Z', 0x40, 0x80, 0xFF} {
		if isCompressible(b) {
			t.Errorf("isCompressible(0x%02x) = true, want false", b)
		}
	}
}
