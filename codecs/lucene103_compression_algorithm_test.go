// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
)

func TestCompressionAlgorithm_CodeAndString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		alg  CompressionAlgorithm
		code int
		name string
	}{
		{CompressionNoCompression, 0x00, "NO_COMPRESSION"},
		{CompressionLowercaseASCII, 0x01, "LOWERCASE_ASCII"},
		{CompressionLZ4, 0x02, "LZ4"},
	}
	for _, c := range cases {
		if got := c.alg.Code(); got != c.code {
			t.Errorf("%s: Code() = %d, want %d", c.name, got, c.code)
		}
		if got := c.alg.String(); got != c.name {
			t.Errorf("String() for code %d = %q, want %q", c.code, got, c.name)
		}
	}
}

func TestCompressionAlgorithmByCode_RoundTrip(t *testing.T) {
	t.Parallel()
	for _, want := range []CompressionAlgorithm{
		CompressionNoCompression, CompressionLowercaseASCII, CompressionLZ4,
	} {
		got, err := CompressionAlgorithmByCode(want.Code())
		if err != nil {
			t.Fatalf("ByCode(%d): %v", want.Code(), err)
		}
		if got != want {
			t.Errorf("ByCode(%d) = %v, want %v", want.Code(), got, want)
		}
	}
}

func TestCompressionAlgorithmByCode_Illegal(t *testing.T) {
	t.Parallel()
	for _, code := range []int{-1, 3, 255, 1000} {
		if _, err := CompressionAlgorithmByCode(code); err == nil {
			t.Errorf("ByCode(%d) returned nil error; want failure", code)
		}
	}
}

func TestCompressionAlgorithm_Read_NoCompression(t *testing.T) {
	t.Parallel()
	in := []byte("the quick brown fox jumps over the lazy dog")
	src := store.NewByteArrayDataInput(in)
	out := make([]byte, len(in))
	if err := CompressionNoCompression.Read(src, out, len(in)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip mismatch:\n  want %q\n  got  %q", in, out)
	}
}

func TestCompressionAlgorithm_Read_LowercaseASCII(t *testing.T) {
	t.Parallel()
	in := []byte("hello.world.example.com.with.many.dots")
	w := store.NewByteArrayDataOutput(64)
	tmp := make([]byte, len(in))
	ok, err := compress.Compress(in, len(in), tmp, w)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if !ok {
		t.Fatal("Compress refused all-lowercase input")
	}
	src := store.NewByteArrayDataInput(w.GetBytes())
	out := make([]byte, len(in))
	if err := CompressionLowercaseASCII.Read(src, out, len(in)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip mismatch:\n  want %q\n  got  %q", in, out)
	}
}

func TestCompressionAlgorithm_Read_LZ4(t *testing.T) {
	t.Parallel()
	in := bytes.Repeat([]byte("AaBbCcDdEeFfGg-"), 32)
	w := store.NewByteArrayDataOutput(len(in) + 64)
	if err := compress.LZ4Compress(in, 0, len(in), w, compress.NewFastCompressionHashTable()); err != nil {
		t.Fatalf("LZ4Compress: %v", err)
	}
	src := store.NewByteArrayDataInput(w.GetBytes())
	out := make([]byte, len(in))
	if err := CompressionLZ4.Read(src, out, len(in)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip mismatch (%d bytes)", len(in))
	}
}

func TestCompressionAlgorithm_Read_LengthOutOfRange(t *testing.T) {
	t.Parallel()
	src := store.NewByteArrayDataInput([]byte{1, 2, 3})
	out := make([]byte, 2)
	if err := CompressionNoCompression.Read(src, out, 5); err == nil {
		t.Errorf("expected error for length > len(out)")
	}
	if err := CompressionNoCompression.Read(src, out, -1); err == nil {
		t.Errorf("expected error for negative length")
	}
}
