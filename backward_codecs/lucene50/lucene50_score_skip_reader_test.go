// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene50ScoreSkipReader_PanicsOnOldVersion verifies that construction
// panics for version < VersionImpactSkipData.
func TestLucene50ScoreSkipReader_PanicsOnOldVersion(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for old version but got none")
		}
	}()
	stream := newTestStream(t)
	NewLucene50ScoreSkipReader(VersionStart, stream, 2, false, false, false)
}

// TestLucene50ScoreSkipReader_Construction verifies construction and initial
// state.
func TestLucene50ScoreSkipReader_Construction(t *testing.T) {
	const maxLevels = 3
	stream := newTestStream(t)
	r := NewLucene50ScoreSkipReader(VersionCurrent, stream, maxLevels, true, false, true)

	if r.impactData == nil || len(r.impactData) != maxLevels {
		t.Errorf("impactData len: got %v want %d", len(r.impactData), maxLevels)
	}
	if r.impactDataLength == nil || len(r.impactDataLength) != maxLevels {
		t.Errorf("impactDataLength len: got %v want %d", len(r.impactDataLength), maxLevels)
	}
	if len(r.perLevelImpacts) != maxLevels {
		t.Errorf("perLevelImpacts len: got %d want %d", len(r.perLevelImpacts), maxLevels)
	}
	// Each level should be pre-populated with (MaxInt32, 1).
	for i, b := range r.perLevelImpacts {
		if b.Size != 1 {
			t.Errorf("perLevelImpacts[%d].Size: got %d want 1", i, b.Size)
		}
		if b.Freqs[0] != math.MaxInt32 {
			t.Errorf("perLevelImpacts[%d].Freqs[0]: got %d want MaxInt32", i, b.Freqs[0])
		}
		if b.Norms[0] != 1 {
			t.Errorf("perLevelImpacts[%d].Norms[0]: got %d want 1", i, b.Norms[0])
		}
	}
}

// TestLucene50ScoreSkipReader_GetImpactsNotNil verifies that GetImpacts()
// returns a non-nil Impacts.
func TestLucene50ScoreSkipReader_GetImpactsNotNil(t *testing.T) {
	stream := newTestStream(t)
	r := NewLucene50ScoreSkipReader(VersionCurrent, stream, 2, false, false, false)
	if r.GetImpacts() == nil {
		t.Error("GetImpacts() returned nil")
	}
}

// TestLucene50ScoreSkipReader_NumLevels verifies initial numLevels.
func TestLucene50ScoreSkipReader_NumLevels(t *testing.T) {
	stream := newTestStream(t)
	r := NewLucene50ScoreSkipReader(VersionCurrent, stream, 2, false, false, false)
	impacts := r.GetImpacts()
	if impacts.NumLevels() != 1 {
		t.Errorf("NumLevels: got %d want 1", impacts.NumLevels())
	}
}

// TestLucene50ScoreSkipReader_DecodeImpacts verifies the static impact
// decoding function against a hand-constructed encoding.
//
// Encoding for two impacts (freq=5, norm=3) and (freq=7, norm=5):
//
//	Impact 1: freq=5, norm=3
//	  freqDelta = (5-0)*2 - 1 = 9? No, freq increments from 0.
//	  Actually freq starts at 0, increments by 1 + (freqDelta >> 1).
//	  For freq=5: freq += 1 + (fd >> 1) → 0 + 1 + (fd >> 1) = 5 → fd >> 1 = 4 → fd = 8.
//	  bit0 = 0, so norm++ (1). But we want norm=3, so we need the ZLong path.
//	  freqDelta = 8 | 0x01 = 9 → freq += 1 + (9>>1) = 1+4 = 5. Then norm += 1 + zlong.
//	  We want norm=3: 1 + zlong = 3 → zlong = 2 → zigzag(2) = 4 → VLong 4 → byte 0x04.
//	  freqDelta VLong = 9 → byte 0x09. normDelta ZLong: value=2, zigzag=4 → byte 0x04.
//
//	Impact 2: freq=7, norm=5 (delta from freq=5, norm=3)
//	  freq += 1 + (fd >> 1) = 7: need += 2, so 1 + (fd>>1) = 2 → fd>>1=1 → fd=2.
//	  bit0=0, norm++. norm becomes 4. But we want 5, so use ZLong.
//	  fd = 2|0x01 = 3 → freq += 1 + (3>>1) = 1+1 = 2. norm += 1 + zlong.
//	  We want norm=5 (delta from 3 = +2): 1 + zlong = 2 → zlong = 1 → zigzag = 2 → byte 0x02.
//	  freqDelta VLong = 3 → byte 0x03. normDelta ZLong = 1 zigzag = 2 → byte 0x02.
//
// Encoded: [0x09, 0x04, 0x03, 0x02]
func TestLucene50ScoreSkipReader_DecodeImpacts(t *testing.T) {
	// Build encoded bytes manually.
	raw := []byte{0x09, 0x04, 0x03, 0x02}
	in := store.NewByteArrayDataInput(raw)
	buf := decodeImpactsFromReader(in, nil)

	if buf == nil {
		t.Fatal("decodeImpactsFromReader returned nil")
	}
	if buf.Size != 2 {
		t.Fatalf("Size: got %d want 2", buf.Size)
	}
	if buf.Freqs[0] != 5 {
		t.Errorf("Freqs[0]: got %d want 5", buf.Freqs[0])
	}
	if buf.Norms[0] != 3 {
		t.Errorf("Norms[0]: got %d want 3", buf.Norms[0])
	}
	if buf.Freqs[1] != 7 {
		t.Errorf("Freqs[1]: got %d want 7", buf.Freqs[1])
	}
	if buf.Norms[1] != 5 {
		t.Errorf("Norms[1]: got %d want 5", buf.Norms[1])
	}
}

// TestLucene50ScoreSkipReader_DecodeImpacts_NormIncrement verifies the
// norm-only increment path (bit0 of freqDelta == 0).
//
// Impact (freq=3, norm=2): fd = (3-1)*2 = 4, bit0=0.
//   freq += 1 + (4>>1) = 1+2 = 3. norm++ = 1. But norm should be 2.
//   We need fd | bit0, so: fd=4|1=5 → freq = 1 + (5>>1) = 1+2 = 3. norm += 1 + zlong.
//   zlong=1 → zigzag=2 → byte 0x02. fd VLong=5 → byte 0x05.
//
// Actually let's just test the norm++ path directly with a simpler example:
// Impact (freq=2, norm=1): fd = (2-1)*2 = 2, bit0=0.
//   freq += 1 + (2>>1) = 1+1 = 2. norm++ = 1.
//   Encoded: [0x02]
func TestLucene50ScoreSkipReader_DecodeImpacts_SingleNormIncrement(t *testing.T) {
	raw := []byte{0x02}
	in := store.NewByteArrayDataInput(raw)
	buf := decodeImpactsFromReader(in, nil)
	if buf.Size != 1 {
		t.Fatalf("Size: got %d want 1", buf.Size)
	}
	if buf.Freqs[0] != 2 {
		t.Errorf("Freqs[0]: got %d want 2", buf.Freqs[0])
	}
	if buf.Norms[0] != 1 {
		t.Errorf("Norms[0]: got %d want 1", buf.Norms[0])
	}
}

// TestLucene50ScoreSkipReader_ZigzagDecodeLong verifies the zig-zag long
// decode helper for representative values.
func TestLucene50ScoreSkipReader_ZigzagDecodeLong(t *testing.T) {
	cases := []struct {
		in   int64
		want int64
	}{
		{0, 0},
		{1, -1},
		{2, 1},
		{3, -2},
		{4, 2},
	}
	for _, tc := range cases {
		if got := zigzagDecodeLong(tc.in); got != tc.want {
			t.Errorf("zigzagDecodeLong(%d): got %d want %d", tc.in, got, tc.want)
		}
	}
}

// TestLucene50ScoreSkipReader_Close verifies that Close does not panic.
func TestLucene50ScoreSkipReader_Close(t *testing.T) {
	stream := newTestStream(t)
	r := NewLucene50ScoreSkipReader(VersionCurrent, stream, 2, false, false, false)
	if err := r.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}
}
