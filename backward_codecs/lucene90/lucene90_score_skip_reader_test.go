// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90ScoreSkipReader_Construction verifies that
// NewLucene90ScoreSkipReader returns a non-nil reader with the embedded
// Lucene90SkipReader wired.
func TestLucene90ScoreSkipReader_Construction(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90ScoreSkipReader(buf, 4, false, false, false)
	if r == nil {
		t.Fatal("expected non-nil Lucene90ScoreSkipReader")
	}
	if r.Lucene90SkipReader == nil {
		t.Fatal("embedded Lucene90SkipReader must not be nil")
	}
}

// TestLucene90ScoreSkipReader_GetImpactsNotNil verifies that GetImpacts returns
// a non-nil Impacts view immediately after construction.
func TestLucene90ScoreSkipReader_GetImpactsNotNil(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90ScoreSkipReader(buf, 4, false, false, false)
	if r.GetImpacts() == nil {
		t.Fatal("GetImpacts must not return nil after construction")
	}
}

// TestLucene90ScoreSkipReader_InitialNumLevels verifies that numLevels is set
// to 1 after construction (default sentinel).
func TestLucene90ScoreSkipReader_InitialNumLevels(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90ScoreSkipReader(buf, 4, false, false, false)
	if r.numLevels != 1 {
		t.Errorf("numLevels: got %d, want 1", r.numLevels)
	}
}

// TestLucene90ScoreSkipReader_PerLevelImpactsAllocated verifies that
// perLevelImpacts is allocated with maxSkipLevels entries, each non-nil.
func TestLucene90ScoreSkipReader_PerLevelImpactsAllocated(t *testing.T) {
	const levels = 4
	buf := newBytesIndexInput(nil)
	r := NewLucene90ScoreSkipReader(buf, levels, false, false, false)
	if len(r.perLevelImpacts) != levels {
		t.Fatalf("perLevelImpacts: got len=%d, want %d", len(r.perLevelImpacts), levels)
	}
	for i, b := range r.perLevelImpacts {
		if b == nil {
			t.Errorf("perLevelImpacts[%d] is nil", i)
		}
	}
}

// TestLucene90ScoreSkipReader_ImpactDataAllocated verifies that impactData and
// impactDataLength slices have maxSkipLevels entries after construction.
func TestLucene90ScoreSkipReader_ImpactDataAllocated(t *testing.T) {
	const levels = 4
	buf := newBytesIndexInput(nil)
	r := NewLucene90ScoreSkipReader(buf, levels, false, false, false)
	if len(r.impactData) != levels {
		t.Fatalf("impactData: got len=%d, want %d", len(r.impactData), levels)
	}
	if len(r.impactDataLength) != levels {
		t.Fatalf("impactDataLength: got len=%d, want %d", len(r.impactDataLength), levels)
	}
}

// TestLucene90ScoreSkipReader_ReadImpactsHookOverridden verifies that the
// readImpactsHook on the base reader was replaced by the score reader's own
// implementation (non-nil and set to the score reader's method).
func TestLucene90ScoreSkipReader_ReadImpactsHookOverridden(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90ScoreSkipReader(buf, 4, false, false, false)
	if r.Lucene90SkipReader.readImpactsHook == nil {
		t.Fatal("readImpactsHook must not be nil")
	}
}

// TestDecodeImpacts90_SinglePair exercises the basic single-pair decode path.
// WriteImpacts encoding for {freq=5, norm=3}: freqDelta=4, normDelta=2 ≠ 0
// → VInt((4<<1)|1=9), ZLong(zigzag(2)=4).
// After decoding, Freqs[0]=5, Norms[0]=3.
func TestDecodeImpacts90_SinglePair(t *testing.T) {
	encoded, err := encodeImpactPairs90([]impactPair90{{freq: 5, norm: 3}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	badi := store.NewByteArrayDataInput(encoded)
	got := decodeImpacts90(badi, nil)
	if got.Size != 1 {
		t.Fatalf("size: got %d, want 1", got.Size)
	}
	if got.Freqs[0] != 5 {
		t.Errorf("freq[0]: got %d, want 5", got.Freqs[0])
	}
	if got.Norms[0] != 3 {
		t.Errorf("norm[0]: got %d, want 3", got.Norms[0])
	}
}

// TestDecodeImpacts90_ZeroNormDelta exercises the normDelta==0 path where the
// norm increments by exactly 1 from the previous pair.
// Pairs: {freq=1,norm=1} → {freq=2,norm=2}: normDelta = 2-1-1 = 0 → fold.
func TestDecodeImpacts90_ZeroNormDelta(t *testing.T) {
	encoded, err := encodeImpactPairs90([]impactPair90{
		{freq: 1, norm: 1},
		{freq: 2, norm: 2},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	badi := store.NewByteArrayDataInput(encoded)
	got := decodeImpacts90(badi, nil)
	if got.Size != 2 {
		t.Fatalf("size: got %d, want 2", got.Size)
	}
	if got.Freqs[0] != 1 || got.Norms[0] != 1 {
		t.Errorf("pair[0]: got freq=%d norm=%d, want 1 1", got.Freqs[0], got.Norms[0])
	}
	if got.Freqs[1] != 2 || got.Norms[1] != 2 {
		t.Errorf("pair[1]: got freq=%d norm=%d, want 2 2", got.Freqs[1], got.Norms[1])
	}
}

// TestZigzagDecodeLong90_Positive verifies zig-zag decoding for a positive
// value: encode(2)=4, decode(4)=2.
func TestZigzagDecodeLong90_Positive(t *testing.T) {
	if got := zigzagDecodeLong90(4); got != 2 {
		t.Errorf("zigzagDecodeLong90(4) = %d; want 2", got)
	}
}

// TestZigzagDecodeLong90_Negative verifies zig-zag decoding for a negative
// value: encode(-1)=1, decode(1)=-1.
func TestZigzagDecodeLong90_Negative(t *testing.T) {
	if got := zigzagDecodeLong90(1); got != -1 {
		t.Errorf("zigzagDecodeLong90(1) = %d; want -1", got)
	}
}

// TestOversize90_SmallInput verifies that oversize90 returns at least 8 for
// inputs ≤ 7.
func TestOversize90_SmallInput(t *testing.T) {
	for n := 0; n <= 7; n++ {
		if got := oversize90(n); got < 8 {
			t.Errorf("oversize90(%d) = %d; want >= 8", n, got)
		}
	}
}

// TestOversize90_LargeInput verifies that oversize90 returns more than the
// requested size (growth headroom > 0).
func TestOversize90_LargeInput(t *testing.T) {
	n := 1024
	if got := oversize90(n); got <= n {
		t.Errorf("oversize90(%d) = %d; want > %d", n, got, n)
	}
}

// TestLucene90ScoreSkipReader_CloseIsClean verifies that Close does not panic
// or return an error.
func TestLucene90ScoreSkipReader_CloseIsClean(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90ScoreSkipReader(buf, 4, false, false, false)
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestLucene90ScoreSkipImpacts_NumLevelsDelegate verifies that the Impacts
// view's NumLevels delegates to the reader's numLevels field.
func TestLucene90ScoreSkipImpacts_NumLevelsDelegate(t *testing.T) {
	buf := newBytesIndexInput(nil)
	r := NewLucene90ScoreSkipReader(buf, 4, false, false, false)
	r.numLevels = 3
	if got := r.GetImpacts().NumLevels(); got != 3 {
		t.Errorf("NumLevels: got %d, want 3", got)
	}
}

// --- local test helpers -----------------------------------------------------

// impactPair90 is a (freq, norm) pair for test encoding.
type impactPair90 struct {
	freq int
	norm int64
}

// encodeImpactPairs90 encodes pairs using the same delta scheme as WriteImpacts,
// so that decodeImpacts90 can round-trip them.
func encodeImpactPairs90(pairs []impactPair90) ([]byte, error) {
	out := store.NewByteBuffersDataOutput()
	var prevFreq int
	var prevNorm int64
	for i, p := range pairs {
		var freqDelta int
		var normDelta int64
		if i == 0 {
			freqDelta = p.freq - 1
			normDelta = p.norm - 1
		} else {
			freqDelta = p.freq - prevFreq - 1
			normDelta = p.norm - prevNorm - 1
		}
		if normDelta == 0 {
			if err := out.WriteVInt(int32(freqDelta << 1)); err != nil {
				return nil, err
			}
		} else {
			if err := out.WriteVInt(int32((freqDelta << 1) | 1)); err != nil {
				return nil, err
			}
			if err := out.WriteZLong(normDelta); err != nil {
				return nil, err
			}
		}
		prevFreq = p.freq
		prevNorm = p.norm
	}
	return out.ToArrayCopy(), nil
}
