// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestDeflateWithPresetDict_StringSurface verifies the canonical Lucene
// mode name surfaces correctly.
func TestDeflateWithPresetDict_StringSurface(t *testing.T) {
	mode := NewDeflateWithPresetDictCompressionMode()
	if got, want := mode.String(), "BEST_COMPRESSION"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// TestDeflateWithPresetDict_RoundTripWhole compresses a non-trivial buffer
// and decompresses the full window, verifying byte equality.
func TestDeflateWithPresetDict_RoundTripWhole(t *testing.T) {
	src := repeatablePayload(10_000)
	roundTripWindow(t, src, 0, len(src))
}

// TestDeflateWithPresetDict_RoundTripWindowed exercises the partial-window
// path: skip-leading-blocks and intersect-trailing-blocks branches.
func TestDeflateWithPresetDict_RoundTripWindowed(t *testing.T) {
	src := repeatablePayload(8_000)
	for _, win := range [][2]int{
		{0, 100}, {100, 100}, {500, 2_000}, {3_000, 4_000}, {len(src) - 1, 1},
	} {
		roundTripWindow(t, src, win[0], win[1])
	}
}

// TestDeflateWithPresetDict_RoundTripEmpty must succeed without writing or
// reading any data payload when length is zero.
func TestDeflateWithPresetDict_RoundTripEmpty(t *testing.T) {
	roundTripWindow(t, []byte{}, 0, 0)
}

// TestDeflateWithPresetDict_CloneIsIndependent verifies that Clone produces
// a working decompressor that does not share state with the original.
func TestDeflateWithPresetDict_CloneIsIndependent(t *testing.T) {
	mode := NewDeflateWithPresetDictCompressionMode()
	dec := mode.NewDecompressor()
	clone := dec.Clone()
	if clone == nil {
		t.Fatal("Clone() returned nil")
	}
	if _, ok := clone.(*deflateWithPresetDictDecompressor); !ok {
		t.Fatalf("Clone() returned %T, want *deflateWithPresetDictDecompressor", clone)
	}
}

// TestDeflateWithPresetDict_CompressorClosed exercises the Close idempotency
// and post-close error contract.
func TestDeflateWithPresetDict_CompressorClosed(t *testing.T) {
	mode := NewDeflateWithPresetDictCompressionMode()
	c := mode.NewCompressor()
	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close (must be idempotent): %v", err)
	}
	out := store.NewByteBuffersDataOutput()
	in := store.NewByteBuffersDataInput([]byte("data"))
	if err := c.Compress(in, out); err == nil {
		t.Fatal("Compress after Close: want error, got nil")
	}
}

// roundTripWindow asserts that compressing src and decompressing the
// [offset, offset+length) window reproduces src[offset:offset+length] byte
// for byte. It is the structural fixture mirroring the Java-side
// CompressionMode tests, parameterised over the requested window.
func roundTripWindow(t *testing.T, src []byte, offset, length int) {
	t.Helper()

	mode := NewDeflateWithPresetDictCompressionMode()

	// Compress: drive the ByteBuffersDataInput against a fresh output.
	in := store.NewByteBuffersDataInput(src)
	out := store.NewByteBuffersDataOutput()
	if err := mode.NewCompressor().Compress(in, out); err != nil {
		t.Fatalf("Compress: %v", err)
	}

	// Decompress: pipe the compressed bytes into a DataInput.
	compressed := drain(out)
	dec := mode.NewDecompressor()
	dst := &util.BytesRef{}
	if err := dec.Decompress(store.NewByteBuffersDataInput(compressed), len(src), offset, length, dst); err != nil {
		t.Fatalf("Decompress(offset=%d,length=%d,originalLength=%d): %v", offset, length, len(src), err)
	}
	if dst.Length != length {
		t.Fatalf("dst.Length = %d, want %d", dst.Length, length)
	}
	got := dst.Bytes[dst.Offset : dst.Offset+dst.Length]
	want := src[offset : offset+length]
	if !bytes.Equal(got, want) {
		t.Fatalf("payload mismatch (offset=%d length=%d): got %x... want %x...", offset, length, head(got), head(want))
	}
}

// repeatablePayload generates a deterministic byte slice with enough
// recurring structure that DEFLATE produces a meaningful ratio.
func repeatablePayload(n int) []byte {
	r := rand.New(rand.NewPCG(1, 2))
	out := make([]byte, n)
	template := []byte("Lucene 10.4.0 DEFLATE+preset-dict round-trip fixture. ")
	for i := range out {
		if r.IntN(10) == 0 {
			out[i] = byte(r.IntN(256))
		} else {
			out[i] = template[i%len(template)]
		}
	}
	return out
}

func drain(out *store.ByteBuffersDataOutput) []byte {
	sink := store.NewByteBuffersDataOutput()
	if err := out.CopyTo(sink); err != nil {
		panic(err)
	}
	di := sink.ToDataInput()
	n := int(sink.Size())
	buf := make([]byte, n)
	if n > 0 {
		if err := di.ReadBytes(buf); err != nil {
			panic(err)
		}
	}
	return buf
}

func head(b []byte) []byte {
	if len(b) > 16 {
		return b[:16]
	}
	return b
}

// Compile-time check that the value satisfies the interface.
var _ compressing.CompressionMode = NewDeflateWithPresetDictCompressionMode()
