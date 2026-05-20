// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene84

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// bytesAsIndexInput writes data into a ByteBuffersDirectory and returns an
// IndexInput over it. This is the only way to obtain a fully-compliant
// store.IndexInput (with Clone) from raw bytes in tests.
func bytesAsIndexInput(t *testing.T, data []byte) store.IndexInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput("skip.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes(data); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}
	in, err := dir.OpenInput("skip.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return in
}

// --- trim84 ---

// TestTrim84_ExactMultiple verifies that exact multiples of blockSize are decremented.
func TestTrim84_ExactMultiple(t *testing.T) {
	for _, df := range []int{128, 256, 512, 1024} {
		if got := trim84(df); got != df-1 {
			t.Errorf("trim84(%d): got %d, want %d", df, got, df-1)
		}
	}
}

// TestTrim84_NonMultiple verifies that non-multiples pass through unchanged.
func TestTrim84_NonMultiple(t *testing.T) {
	for _, df := range []int{1, 127, 129, 255, 257} {
		if got := trim84(df); got != df {
			t.Errorf("trim84(%d): got %d, want %d", df, got, df)
		}
	}
}

// TestTrim84_ZeroIsMultiple verifies zero is treated as a multiple of blockSize
// (0 % 128 == 0), so trim84 returns -1, matching Java's Lucene84SkipReader.trim(0).
func TestTrim84_ZeroIsMultiple(t *testing.T) {
	if got := trim84(0); got != -1 {
		t.Errorf("trim84(0): got %d, want -1", got)
	}
}

// --- decodeImpacts84 ---

// encodeImpacts84 hand-encodes a sequence of (freq, norm) pairs using the
// Lucene 8.4 impact encoding scheme, matching decodeImpacts84.
// Used only for test construction; not part of production code.
func encodeImpacts84(pairs [][2]int64) []byte {
	buf := store.NewByteArrayDataOutput(64)
	var prevFreq, prevNorm int64
	for _, p := range pairs {
		freq, norm := p[0], p[1]
		freqDelta := freq - prevFreq - 1
		normDelta := norm - prevNorm - 1
		if normDelta != 0 {
			// set bit 0 to signal norm is encoded
			encoded := int32((freqDelta << 1) | 1)
			_ = buf.WriteVInt(encoded)
			_ = buf.WriteVLong(zigZagEncodeI64(normDelta))
		} else {
			encoded := int32(freqDelta << 1)
			_ = buf.WriteVInt(encoded)
		}
		prevFreq = freq
		prevNorm = norm
	}
	return buf.GetBytes()[:buf.GetPosition()]
}

func zigZagEncodeI64(v int64) int64 {
	return (v << 1) ^ (v >> 63)
}

// TestDecodeImpacts84_Single decodes a single (freq=1, norm=1) impact.
func TestDecodeImpacts84_Single(t *testing.T) {
	// Single entry: freq=1 (freqDelta=0, normDelta=0 → bit0=0)
	data := encodeImpacts84([][2]int64{{1, 1}})
	in := store.NewByteArrayDataInput(data)
	buf := decodeImpacts84(in, nil)
	if buf.Size != 1 {
		t.Fatalf("size: got %d, want 1", buf.Size)
	}
	if buf.Freqs[0] != 1 {
		t.Errorf("Freqs[0]: got %d, want 1", buf.Freqs[0])
	}
	if buf.Norms[0] != 1 {
		t.Errorf("Norms[0]: got %d, want 1", buf.Norms[0])
	}
}

// TestDecodeImpacts84_Multiple decodes a sequence of impacts.
func TestDecodeImpacts84_Multiple(t *testing.T) {
	pairs := [][2]int64{
		{1, 1},
		{3, 2},
		{7, 5},
	}
	data := encodeImpacts84(pairs)
	in := store.NewByteArrayDataInput(data)
	buf := decodeImpacts84(in, nil)
	if buf.Size != 3 {
		t.Fatalf("size: got %d, want 3", buf.Size)
	}
	for i, p := range pairs {
		if buf.Freqs[i] != int(p[0]) {
			t.Errorf("Freqs[%d]: got %d, want %d", i, buf.Freqs[i], p[0])
		}
		if buf.Norms[i] != p[1] {
			t.Errorf("Norms[%d]: got %d, want %d", i, buf.Norms[i], p[1])
		}
	}
}

// TestDecodeImpacts84_Reuse verifies the reuse path overwrites previous data.
func TestDecodeImpacts84_Reuse(t *testing.T) {
	reuse := index.NewFreqAndNormBuffer()
	reuse.Add(999, 999)

	data := encodeImpacts84([][2]int64{{2, 3}})
	in := store.NewByteArrayDataInput(data)
	buf := decodeImpacts84(in, reuse)
	if buf != reuse {
		t.Fatal("expected reuse to be returned")
	}
	if buf.Size != 1 || buf.Freqs[0] != 2 || buf.Norms[0] != 3 {
		t.Errorf("unexpected buffer contents: size=%d freq=%d norm=%d", buf.Size, buf.Freqs[0], buf.Norms[0])
	}
}

// TestDecodeImpacts84_Empty returns size 0 on empty input.
func TestDecodeImpacts84_Empty(t *testing.T) {
	in := store.NewByteArrayDataInput([]byte{})
	buf := decodeImpacts84(in, nil)
	if buf.Size != 0 {
		t.Errorf("size: got %d, want 0", buf.Size)
	}
}

// --- Lucene84ScoreSkipReader ---

// TestLucene84ScoreSkipReader_DefaultImpacts verifies initial GetImpacts
// returns MaxInt32/1 sentinel after construction (no SkipTo called).
func TestLucene84ScoreSkipReader_DefaultImpacts(t *testing.T) {
	r := NewLucene84ScoreSkipReader(bytesAsIndexInput(t, []byte{}), 4, false, false, false)
	impacts := r.GetImpacts()
	if impacts == nil {
		t.Fatal("GetImpacts returned nil")
	}
	// numLevels defaults to 1.
	if impacts.NumLevels() != 1 {
		t.Errorf("NumLevels: got %d, want 1", impacts.NumLevels())
	}
	buf := impacts.GetImpacts(0)
	if buf.Size != 1 {
		t.Fatalf("default impact size: got %d, want 1", buf.Size)
	}
	if buf.Freqs[0] != math.MaxInt32 {
		t.Errorf("default freq: got %d, want MaxInt32", buf.Freqs[0])
	}
	if buf.Norms[0] != 1 {
		t.Errorf("default norm: got %d, want 1", buf.Norms[0])
	}
}

// TestLucene84ScoreSkipReader_ImpactsInterface verifies the type satisfies index.Impacts.
func TestLucene84ScoreSkipReader_ImpactsInterface(t *testing.T) {
	r := NewLucene84ScoreSkipReader(bytesAsIndexInput(t, []byte{}), 2, false, false, false)
	var _ index.Impacts = r.GetImpacts()
}

// --- Lucene84SkipReader accessors ---

// TestLucene84SkipReader_InitPointers verifies that Init sets all base pointers.
func TestLucene84SkipReader_InitPointers(t *testing.T) {
	r := newLucene84SkipReader(bytesAsIndexInput(t, []byte{}), 1, true, false, false)
	if err := r.Init(0, 100, 200, 300, 1); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if r.GetDocPointer() != 100 {
		t.Errorf("DocPointer: got %d, want 100", r.GetDocPointer())
	}
	if r.GetPosPointer() != 200 {
		t.Errorf("PosPointer: got %d, want 200", r.GetPosPointer())
	}
	// No payloads → payPointer nil, GetPayPointer returns lastPayPointer=300.
	if r.GetPayPointer() != 300 {
		t.Errorf("PayPointer: got %d, want 300", r.GetPayPointer())
	}
}

// TestLucene84SkipReader_Close verifies Close does not panic.
func TestLucene84SkipReader_Close(t *testing.T) {
	r := newLucene84SkipReader(bytesAsIndexInput(t, []byte{}), 1, false, false, false)
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestLucene84SkipReader_GetNextSkipDoc verifies GetNextSkipDoc delegates to base.
func TestLucene84SkipReader_GetNextSkipDoc(t *testing.T) {
	r := newLucene84SkipReader(bytesAsIndexInput(t, []byte{}), 1, false, false, false)
	// Before any SkipTo, base doc is 0.
	if got := r.GetNextSkipDoc(); got != 0 {
		t.Errorf("GetNextSkipDoc: got %d, want 0", got)
	}
}
