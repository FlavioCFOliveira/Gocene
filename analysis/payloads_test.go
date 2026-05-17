// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bytes"
	"math"
	"testing"
)

func TestPayloadHelper_FloatRoundTrip(t *testing.T) {
	cases := []float32{0, 1, -1, math.Pi, math.MaxFloat32, -math.MaxFloat32, math.SmallestNonzeroFloat32}
	for _, v := range cases {
		buf := EncodeFloatPayload(v)
		if len(buf) != 4 {
			t.Fatalf("EncodeFloatPayload(%v): expected 4 bytes, got %d", v, len(buf))
		}
		got := DecodeFloatPayload(buf)
		if got != v {
			t.Errorf("round-trip mismatch: in=%v out=%v", v, got)
		}
	}
}

func TestPayloadHelper_FloatBigEndian(t *testing.T) {
	// 1.0 in IEEE 754 single precision big-endian = 0x3F800000
	buf := EncodeFloatPayload(1.0)
	want := []byte{0x3F, 0x80, 0x00, 0x00}
	if !bytes.Equal(buf, want) {
		t.Errorf("EncodeFloatPayload(1.0) = %x, want %x", buf, want)
	}
}

func TestPayloadHelper_IntRoundTrip(t *testing.T) {
	cases := []int32{0, 1, -1, math.MaxInt32, math.MinInt32, 0x12345678}
	for _, v := range cases {
		buf := EncodeIntPayload(v)
		if len(buf) != 4 {
			t.Fatalf("EncodeIntPayload(%v): expected 4 bytes, got %d", v, len(buf))
		}
		got := DecodeIntPayload(buf, 0)
		if got != v {
			t.Errorf("round-trip mismatch: in=%v out=%v", v, got)
		}
	}
}

func TestPayloadHelper_IntBigEndian(t *testing.T) {
	buf := EncodeIntPayload(0x12345678)
	want := []byte{0x12, 0x34, 0x56, 0x78}
	if !bytes.Equal(buf, want) {
		t.Errorf("EncodeIntPayload(0x12345678) = %x, want %x", buf, want)
	}
}

func TestIdentityPayloadEncoder(t *testing.T) {
	enc := NewIdentityPayloadEncoder()
	in := []byte("hello world")
	got := enc.Encode(in)
	if !bytes.Equal(got, in) {
		t.Errorf("Encode(%q) = %q, want %q", in, got, in)
	}
	// Should be a copy, not a reference
	got[0] = 'X'
	if in[0] == 'X' {
		t.Errorf("IdentityPayloadEncoder.Encode must return a copy, not the original buffer")
	}

	// EncodeSlice with offset
	got2 := enc.EncodeSlice(in, 6, 5)
	if !bytes.Equal(got2, []byte("world")) {
		t.Errorf("EncodeSlice(%q, 6, 5) = %q, want %q", in, got2, "world")
	}
}

func TestFloatPayloadEncoder(t *testing.T) {
	enc := NewFloatPayloadEncoder()
	got := enc.Encode([]byte("1.0"))
	want := []byte{0x3F, 0x80, 0x00, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("FloatPayloadEncoder.Encode(1.0) = %x, want %x", got, want)
	}

	// Slice form
	got2 := enc.EncodeSlice([]byte("xxx2.5yyy"), 3, 3)
	wantF := float32(2.5)
	if DecodeFloatPayload(got2) != wantF {
		t.Errorf("EncodeSlice slice mismatch: got %v, want %v", DecodeFloatPayload(got2), wantF)
	}
}

func TestIntegerPayloadEncoder(t *testing.T) {
	enc := NewIntegerPayloadEncoder()
	got := enc.Encode([]byte("12345"))
	if DecodeIntPayload(got, 0) != 12345 {
		t.Errorf("IntegerPayloadEncoder round-trip failed: got %d", DecodeIntPayload(got, 0))
	}
}
