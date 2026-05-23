// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// decodeImpacts99
// ---------------------------------------------------------------------------

// TestDecodeImpacts99_Empty verifies that an empty buffer yields size==0.
func TestDecodeImpacts99_Empty(t *testing.T) {
	in := store.NewByteArrayDataInput([]byte{})
	buf := index.NewFreqAndNormBuffer()
	out := decodeImpacts99(in, buf)
	if out.Size != 0 {
		t.Errorf("Size: got %d, want 0", out.Size)
	}
}

// TestDecodeImpacts99_SingleNormIncrOnly verifies decoding of one impact pair
// where freqDelta bit-0 is unset (norm increments by 1).
func TestDecodeImpacts99_SingleNormIncrOnly(t *testing.T) {
	// freqDelta=2 → bit0=0 → freq = 1+(2>>1)=2, norm = 0+1=1
	badi := buildBADI(t, func(out store.DataOutput) error {
		return store.WriteVInt(out, 2) // freqDelta=2
	})
	buf := index.NewFreqAndNormBuffer()
	out := decodeImpacts99(badi, buf)
	if out.Size != 1 {
		t.Fatalf("Size: got %d, want 1", out.Size)
	}
	if out.Freqs[0] != 2 {
		t.Errorf("Freqs[0]: got %d, want 2", out.Freqs[0])
	}
	if out.Norms[0] != 1 {
		t.Errorf("Norms[0]: got %d, want 1", out.Norms[0])
	}
}

// TestDecodeImpacts99_SingleWithZLong verifies decoding where freqDelta bit-0
// is set (norm carries a ZigZag-encoded delta).
func TestDecodeImpacts99_SingleWithZLong(t *testing.T) {
	// freqDelta=3 → bit0=1 → freq=1+(3>>1)=2; ZLong=0 → normDelta=ZigZagDecode(0)=0 → norm=1+0=1
	badi := buildBADI(t, func(out store.DataOutput) error {
		if err := store.WriteVInt(out, 3); err != nil { // freqDelta=3
			return err
		}
		return store.WriteVLong(out, 0) // ZLong=0
	})
	buf := index.NewFreqAndNormBuffer()
	out := decodeImpacts99(badi, buf)
	if out.Size != 1 {
		t.Fatalf("Size: got %d, want 1", out.Size)
	}
	if out.Freqs[0] != 2 {
		t.Errorf("Freqs[0]: got %d, want 2", out.Freqs[0])
	}
	if out.Norms[0] != 1 {
		t.Errorf("Norms[0]: got %d, want 1", out.Norms[0])
	}
}

// TestDecodeImpacts99_Reuse verifies that a non-nil reuse buffer is returned.
func TestDecodeImpacts99_Reuse(t *testing.T) {
	in := store.NewByteArrayDataInput([]byte{})
	reuse := index.NewFreqAndNormBuffer()
	out := decodeImpacts99(in, reuse)
	if out != reuse {
		t.Error("expected the reuse buffer to be returned")
	}
}

// TestDecodeImpacts99_NilReuse verifies that a nil reuse input allocates a
// new buffer.
func TestDecodeImpacts99_NilReuse(t *testing.T) {
	in := store.NewByteArrayDataInput([]byte{})
	out := decodeImpacts99(in, nil)
	if out == nil {
		t.Error("expected non-nil buffer when reuse is nil")
	}
}

// ---------------------------------------------------------------------------
// trimLucene99
// ---------------------------------------------------------------------------

func TestTrimLucene99_ExactMultiple(t *testing.T) {
	if got := trimLucene99(128); got != 127 {
		t.Errorf("trim(128): got %d, want 127", got)
	}
	if got := trimLucene99(256); got != 255 {
		t.Errorf("trim(256): got %d, want 255", got)
	}
}

func TestTrimLucene99_NotMultiple(t *testing.T) {
	if got := trimLucene99(100); got != 100 {
		t.Errorf("trim(100): got %d, want 100", got)
	}
	if got := trimLucene99(129); got != 129 {
		t.Errorf("trim(129): got %d, want 129", got)
	}
}

// ---------------------------------------------------------------------------
// Lucene99ScoreSkipReader construction
// ---------------------------------------------------------------------------

// TestNewLucene99ScoreSkipReader_InitialState verifies post-construction
// invariants: numLevels==1, first perLevelImpacts entry has sentinel values.
func TestNewLucene99ScoreSkipReader_InitialState(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	// Write a single zero byte so OpenInput succeeds.
	out, _ := dir.CreateOutput("skip.bin", store.IOContext{})
	_ = out.WriteByte(0)
	_ = out.Close()
	in, _ := dir.OpenInput("skip.bin", store.IOContext{})
	t.Cleanup(func() { _ = in.Close() })

	r := NewLucene99ScoreSkipReader(in, 4, true, false, false)

	imp := r.GetImpacts()
	if imp == nil {
		t.Fatal("GetImpacts(): got nil")
	}
	if imp.NumLevels() != 1 {
		t.Errorf("NumLevels(): got %d, want 1", imp.NumLevels())
	}
	// Initial perLevelImpacts[0] must have one sentinel entry.
	buf := imp.GetImpacts(0)
	if buf.Size != 1 {
		t.Fatalf("initial impacts[0].Size: got %d, want 1", buf.Size)
	}
	if buf.Freqs[0] != math.MaxInt32 {
		t.Errorf("initial Freqs[0]: got %d, want MaxInt32", buf.Freqs[0])
	}
	if buf.Norms[0] != 1 {
		t.Errorf("initial Norms[0]: got %d, want 1", buf.Norms[0])
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildBADI writes data via fn into a byte slice and wraps it in a
// ByteArrayDataInput.
func buildBADI(t *testing.T, fn func(store.DataOutput) error) *store.ByteArrayDataInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, err := dir.CreateOutput("tmp.bin", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := fn(out); err != nil {
		t.Fatalf("write fn: %v", err)
	}
	_ = out.Close()
	in, err := dir.OpenInput("tmp.bin", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	buf := make([]byte, in.Length())
	_ = in.ReadBytes(buf)
	return store.NewByteArrayDataInput(buf)
}
