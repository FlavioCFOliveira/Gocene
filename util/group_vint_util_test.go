// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"math/rand"
	"testing"
)

// TestGroupVIntFixture pins the byte format to a hand-computed reference
// for the input [0, 1, 256, 65536]. Lengths are 1,1,2,3 so the control
// byte packs (0)|(0<<2)|(1<<4)|(2<<6) = 0x90 and the payloads are
// 0x00 | 0x01 | 0x00 0x01 | 0x00 0x00 0x01.
func TestGroupVIntFixture(t *testing.T) {
	src := []uint32{0, 1, 256, 65536}
	out, err := GroupVIntEncode(nil, src, 0)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := []byte{0x90, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x01}
	if !bytes.Equal(out, want) {
		t.Fatalf("encoded bytes: got % x want % x", out, want)
	}

	dst := make([]uint32, 4)
	consumed, err := GroupVIntDecode(dst, out, 0, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if consumed != len(out) {
		t.Fatalf("consumed %d want %d", consumed, len(out))
	}
	for i, v := range src {
		if dst[i] != v {
			t.Fatalf("decoded[%d] got %d want %d", i, dst[i], v)
		}
	}
}

// TestGroupVIntAllMax exercises the worst-case path where every value
// requires the full 4 bytes.
func TestGroupVIntAllMax(t *testing.T) {
	src := []uint32{0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF}
	out, err := GroupVIntEncode(nil, src, 0)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// Control byte: each length-1 = 3 -> 0b11 11 11 11 = 0xFF.
	if out[0] != 0xFF {
		t.Fatalf("control byte got 0x%02x want 0xFF", out[0])
	}
	if len(out) != 17 {
		t.Fatalf("encoded length got %d want 17", len(out))
	}
	dst := make([]uint32, 4)
	if _, err := GroupVIntDecode(dst, out, 0, 0); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i, v := range src {
		if dst[i] != v {
			t.Fatalf("decoded[%d] got %x want %x", i, dst[i], v)
		}
	}
}

// TestGroupVIntAllMin exercises the best-case path where every value
// fits in a single byte.
func TestGroupVIntAllMin(t *testing.T) {
	src := []uint32{0, 1, 127, 255}
	out, err := GroupVIntEncode(nil, src, 0)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if out[0] != 0x00 {
		t.Fatalf("control byte got 0x%02x want 0x00", out[0])
	}
	if len(out) != 5 {
		t.Fatalf("encoded length got %d want 5", len(out))
	}
	dst := make([]uint32, 4)
	if _, err := GroupVIntDecode(dst, out, 0, 0); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i, v := range src {
		if dst[i] != v {
			t.Fatalf("decoded[%d] got %d want %d", i, dst[i], v)
		}
	}
}

// TestGroupVIntRandomRoundTrip drives many random groups through encode
// + decode and verifies bit-exact equality and stream concatenation.
func TestGroupVIntRandomRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(0xdeadbeef))
	const groups = 1024
	src := make([]uint32, 4*groups)
	for i := range src {
		// Use a bit-length distribution biased toward smaller values
		// (typical of Lucene posting lists) but with a long tail.
		bits := rng.Intn(32) + 1
		mask := uint32((1 << uint(bits)) - 1)
		src[i] = rng.Uint32() & mask
	}

	var encoded []byte
	for g := 0; g < groups; g++ {
		out, err := GroupVIntEncode(encoded, src, g*4)
		if err != nil {
			t.Fatalf("encode group %d: %v", g, err)
		}
		encoded = out
	}

	dst := make([]uint32, len(src))
	srcOff := 0
	for g := 0; g < groups; g++ {
		n, err := GroupVIntDecode(dst, encoded, srcOff, g*4)
		if err != nil {
			t.Fatalf("decode group %d: %v", g, err)
		}
		srcOff += n
	}
	if srcOff != len(encoded) {
		t.Fatalf("consumed %d of %d encoded bytes", srcOff, len(encoded))
	}
	for i := range src {
		if dst[i] != src[i] {
			t.Fatalf("round-trip mismatch at %d: got %d want %d", i, dst[i], src[i])
		}
	}
}

func TestGroupVIntErrors(t *testing.T) {
	t.Run("encode source too short", func(t *testing.T) {
		if _, err := GroupVIntEncode(nil, []uint32{1, 2}, 0); err == nil {
			t.Fatalf("expected error for short source")
		}
	})
	t.Run("decode source exhausted", func(t *testing.T) {
		dst := make([]uint32, 4)
		if _, err := GroupVIntDecode(dst, nil, 0, 0); err == nil {
			t.Fatalf("expected error for empty source")
		}
	})
	t.Run("decode source truncated", func(t *testing.T) {
		dst := make([]uint32, 4)
		// Control byte says four 1-byte values (5 bytes total), but only 3 present.
		if _, err := GroupVIntDecode(dst, []byte{0x00, 0x01, 0x02}, 0, 0); err == nil {
			t.Fatalf("expected error for truncated source")
		}
	})
	t.Run("decode destination too short", func(t *testing.T) {
		if _, err := GroupVIntDecode(make([]uint32, 2), []byte{0x00, 1, 2, 3, 4}, 0, 0); err == nil {
			t.Fatalf("expected error for short destination")
		}
	})
}

func TestVintLen32(t *testing.T) {
	tests := []struct {
		v    uint32
		want int
	}{
		{0, 1},
		{1, 1},
		{255, 1},
		{256, 2},
		{65535, 2},
		{65536, 3},
		{1<<24 - 1, 3},
		{1 << 24, 4},
		{0xFFFFFFFF, 4},
	}
	for _, tc := range tests {
		if got := vintLen32(tc.v); got != tc.want {
			t.Fatalf("vintLen32(%d) got %d want %d", tc.v, got, tc.want)
		}
	}
}

func TestGroupVIntMaxBytes(t *testing.T) {
	if got := GroupVIntMaxBytes(0); got != 0 {
		t.Fatalf("MaxBytes(0) got %d want 0", got)
	}
	if got := GroupVIntMaxBytes(4); got != 17 {
		t.Fatalf("MaxBytes(4) got %d want 17", got)
	}
	if got := GroupVIntMaxBytes(8); got != 34 {
		t.Fatalf("MaxBytes(8) got %d want 34", got)
	}
	// Partial groups are not counted.
	if got := GroupVIntMaxBytes(3); got != 0 {
		t.Fatalf("MaxBytes(3) got %d want 0", got)
	}
}
