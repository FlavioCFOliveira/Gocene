// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Source: lucene/core/src/test/org/apache/lucene/util/TestSmallFloat.java
// Purpose: byte-level parity for SmallFloat encoders.

package util

import (
	"math"
	"math/rand"
	"testing"
)

// orig315 mirrors the Java reference implementation. Used as a slow
// oracle to validate the optimised implementation byte-for-byte.
func origFloatToByte(f float32, numMantissaBits, zeroExp int) byte {
	if f < 0 {
		f = 0
	}
	if f == 0 {
		return 0
	}
	bits := int32(math.Float32bits(f))
	smallfloat := int(bits >> (24 - numMantissaBits))
	fzero := (63 - zeroExp) << numMantissaBits
	switch {
	case smallfloat <= fzero:
		return 1
	case smallfloat >= fzero+0x100:
		return 0xFF
	default:
		return byte(smallfloat - fzero)
	}
}

func TestSmallFloat_ByteToFloatRoundTripAllBytes_315(t *testing.T) {
	for i := 0; i < 256; i++ {
		b := byte(i)
		f := Byte315ToFloat(b)
		b2 := FloatToByte315(f)
		if b != b2 {
			t.Fatalf("round-trip 315 failed for byte %d: got %d (f=%v)", b, b2, f)
		}
	}
}

func TestSmallFloat_ByteToFloatRoundTripAllBytes_Generic(t *testing.T) {
	// Iterate over the (numMantissaBits, zeroExp) configurations Lucene
	// actually uses (and the broader range still produces valid IEEE
	// floats). The pair (nm=1, ze=0) overflows the sign bit and is
	// not a supported configuration in Lucene.
	for nm := 2; nm <= 5; nm++ {
		for ze := nm; ze <= 32; ze++ {
			for i := 0; i < 256; i++ {
				b := byte(i)
				f := ByteToFloat(b, nm, ze)
				b2 := FloatToByte(f, nm, ze)
				if b != b2 {
					t.Fatalf("round-trip generic failed for nm=%d ze=%d byte=%d: got %d (f=%v)", nm, ze, b, b2, f)
				}
			}
		}
	}
}

func TestSmallFloat_NegativeMapsToZero(t *testing.T) {
	for _, f := range []float32{-1, -math.MaxFloat32, -math.SmallestNonzeroFloat32} {
		if FloatToByte315(f) != 0 {
			t.Fatalf("FloatToByte315(%v) want 0", f)
		}
		if FloatToByte(f, 3, 15) != 0 {
			t.Fatalf("FloatToByte(%v) want 0", f)
		}
	}
}

func TestSmallFloat_UnderflowRoundsUpToOne(t *testing.T) {
	f := float32(math.SmallestNonzeroFloat32)
	if b := FloatToByte315(f); b != 1 {
		t.Fatalf("underflow 315: got %d want 1", b)
	}
	if b := FloatToByte(f, 3, 15); b != 1 {
		t.Fatalf("underflow generic: got %d want 1", b)
	}
}

func TestSmallFloat_OverflowSaturates(t *testing.T) {
	f := float32(math.MaxFloat32)
	if b := FloatToByte315(f); b != 0xFF {
		t.Fatalf("overflow 315: got %d want 0xFF", b)
	}
	if b := FloatToByte(f, 3, 15); b != 0xFF {
		t.Fatalf("overflow generic: got %d want 0xFF", b)
	}
}

func TestSmallFloat_ByteToFloat315IsMonotonic(t *testing.T) {
	prev := float32(-1)
	for i := 0; i < 256; i++ {
		v := Byte315ToFloat(byte(i))
		if v < prev {
			t.Fatalf("non-monotonic at byte %d: %v < %v", i, v, prev)
		}
		prev = v
	}
}

func TestSmallFloat_RandomFloatAgainstOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for k := 0; k < 100_000; k++ {
		bits := rng.Uint32()
		f := math.Float32frombits(bits)
		if math.IsNaN(float64(f)) {
			continue
		}
		want315 := origFloatToByte(f, 3, 15)
		if got := FloatToByte315(f); got != want315 {
			t.Fatalf("FloatToByte315 mismatch f=%v: got %d want %d", f, got, want315)
		}
		if got := FloatToByte(f, 3, 15); got != want315 {
			t.Fatalf("FloatToByte mismatch f=%v: got %d want %d", f, got, want315)
		}
	}
}

func TestSmallFloat_FloatToByte315KnownVectors(t *testing.T) {
	// Vectors derived from the Lucene 10.4.0 algorithm with
	// numMantissaBits=3, zeroExp=15 and verified bit-by-bit against
	// the reference implementation.
	//
	//   smallfloat = floatBits >> 21
	//   fzero      = (63 - 15) << 3 = 384
	//   byte       = smallfloat - fzero (clamped to [1, 0xFF])
	cases := []struct {
		f float32
		b byte
	}{
		{0, 0},
		{1, 124},   // 0x3F800000 >> 21 = 508; 508-384 = 124
		{2, 128},   // 0x40000000 >> 21 = 512; 512-384 = 128
		{0.5, 120}, // 0x3F000000 >> 21 = 504; 504-384 = 120
		{0.125, 112},
		{1e9, 243},
	}
	for _, c := range cases {
		if got := FloatToByte315(c.f); got != c.b {
			t.Fatalf("FloatToByte315(%v)=%d want %d", c.f, got, c.b)
		}
	}
}

func TestLongToInt4_RoundTrip(t *testing.T) {
	for _, v := range []int64{0, 1, 7, 8, 15, 16, 31, 100, 1 << 30, math.MaxInt32} {
		enc, err := LongToInt4(v)
		if err != nil {
			t.Fatalf("LongToInt4(%d): %v", v, err)
		}
		got := Int4ToLong(enc)
		if v < 8 && got != v {
			t.Fatalf("subnormal LongToInt4 round-trip: %d -> %d -> %d", v, enc, got)
		}
		// For normal values we keep the 4 most significant bits, so
		// the decoded value may lose low-order bits; ensure ordering
		// is preserved by also testing monotonicity below.
		_ = got
	}
}

func TestLongToInt4_Monotonicity(t *testing.T) {
	prev := -1
	for v := int64(0); v < 1024; v++ {
		enc, err := LongToInt4(v)
		if err != nil {
			t.Fatalf("LongToInt4(%d): %v", v, err)
		}
		if enc < prev {
			t.Fatalf("non-monotonic at %d: %d < %d", v, enc, prev)
		}
		prev = enc
	}
}

func TestLongToInt4_NegativeRejected(t *testing.T) {
	if _, err := LongToInt4(-1); err == nil {
		t.Fatal("expected error for negative input")
	}
}

func TestIntToByte4_RoundTripBoundary(t *testing.T) {
	for _, v := range []int{0, 1, 7, NumFreeValues() - 1, NumFreeValues(), NumFreeValues() + 1, math.MaxInt32} {
		b, err := IntToByte4(v)
		if err != nil {
			t.Fatalf("IntToByte4(%d): %v", v, err)
		}
		got := Byte4ToInt(b)
		if v < NumFreeValues() {
			if got != v {
				t.Fatalf("free range round-trip lost value: %d -> %d -> %d", v, b, got)
			}
		}
	}
}

func TestIntToByte4_NegativeRejected(t *testing.T) {
	if _, err := IntToByte4(-1); err == nil {
		t.Fatal("expected error for negative input")
	}
}

func TestIntToByte4_Monotonicity(t *testing.T) {
	prev := -1
	for v := 0; v < 4096; v++ {
		b, err := IntToByte4(v)
		if err != nil {
			t.Fatalf("IntToByte4(%d): %v", v, err)
		}
		if int(b) < prev {
			t.Fatalf("non-monotonic at %d: %d < %d", v, b, prev)
		}
		prev = int(b)
	}
}

// Preserve the original 5.2 round-trip coverage.
func TestSmallFloat_ByteToFloatRoundTripAllBytes_52(t *testing.T) {
	for i := 0; i < 256; i++ {
		b := byte(i)
		f := ByteToFloat52(b)
		b2 := FloatToByte52(f)
		if b != b2 {
			t.Fatalf("round-trip 52 failed for byte %d: got %d (f=%v)", b, b2, f)
		}
	}
}
