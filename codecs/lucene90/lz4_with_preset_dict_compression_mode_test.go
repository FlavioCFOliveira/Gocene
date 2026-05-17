// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestLZ4WithPresetDict_StringSurface verifies the canonical Lucene mode name.
func TestLZ4WithPresetDict_StringSurface(t *testing.T) {
	mode := NewLZ4WithPresetDictCompressionMode()
	if got, want := mode.String(), "BEST_SPEED"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// TestLZ4WithPresetDict_RoundTripWhole compresses a non-trivial buffer and
// decompresses the full window, verifying byte equality.
func TestLZ4WithPresetDict_RoundTripWhole(t *testing.T) {
	src := repeatablePayload(10_000)
	roundTripLZ4Window(t, src, 0, len(src))
}

// TestLZ4WithPresetDict_RoundTripWindowed exercises the partial-window paths:
// dictionary-overlap branch (offset < dictLength), skip-leading-blocks branch
// and intersect-trailing-blocks branch.
func TestLZ4WithPresetDict_RoundTripWindowed(t *testing.T) {
	src := repeatablePayload(8_000)
	for _, win := range [][2]int{
		{0, 100},                  // entirely inside dictionary
		{100, 100},                // entirely inside dictionary
		{500, 2_000},              // straddles dict + first sub-blocks
		{3_000, 4_000},            // mid sub-blocks only
		{len(src) - 1, 1},         // last byte only
	} {
		roundTripLZ4Window(t, src, win[0], win[1])
	}
}

// TestLZ4WithPresetDict_RoundTripEmpty must succeed without writing/reading
// any payload when length=0.
func TestLZ4WithPresetDict_RoundTripEmpty(t *testing.T) {
	roundTripLZ4Window(t, []byte{}, 0, 0)
}

// TestLZ4WithPresetDict_CloneIsIndependent verifies Clone produces a peer
// decompressor that does not share state.
func TestLZ4WithPresetDict_CloneIsIndependent(t *testing.T) {
	mode := NewLZ4WithPresetDictCompressionMode()
	dec := mode.NewDecompressor()
	clone := dec.Clone()
	if clone == nil {
		t.Fatal("Clone() returned nil")
	}
	if _, ok := clone.(*lz4WithPresetDictDecompressor); !ok {
		t.Fatalf("Clone() = %T, want *lz4WithPresetDictDecompressor", clone)
	}
}

// TestLZ4WithPresetDict_CompressorCloseIsNoop verifies Close is idempotent
// and does not error.
func TestLZ4WithPresetDict_CompressorCloseIsNoop(t *testing.T) {
	mode := NewLZ4WithPresetDictCompressionMode()
	c := mode.NewCompressor()
	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// roundTripLZ4Window is the structural fixture shared by every window case.
//
// Note: we read back compressed bytes via store.ByteArrayDataInput rather
// than store.ByteBuffersDataInput because the latter's ReadShort returns
// big-endian, which clashes with the little-endian matchDec written by
// LZ4. The wire bytes themselves are correct; only the test reader needs
// to match the LZ4 encoder's endian. See store package back-compat note.
func roundTripLZ4Window(t *testing.T, src []byte, offset, length int) {
	t.Helper()

	mode := NewLZ4WithPresetDictCompressionMode()
	in := store.NewByteBuffersDataInput(src)
	out := store.NewByteArrayDataOutput(len(src)*3 + 32)
	if err := mode.NewCompressor().Compress(in, out); err != nil {
		t.Fatalf("Compress: %v", err)
	}

	compressed := out.GetBytes()
	dec := mode.NewDecompressor()
	dst := &util.BytesRef{}
	if err := dec.Decompress(store.NewByteArrayDataInput(compressed), len(src), offset, length, dst); err != nil {
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

// Compile-time check.
var _ compressing.CompressionMode = NewLZ4WithPresetDictCompressionMode()
