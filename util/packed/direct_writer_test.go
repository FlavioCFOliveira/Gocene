// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// directBitsPerValueSpectrum lists the DirectWriter-supported
// bitsPerValue values; this is the full set used by the test peer
// for round-trip coverage.
var directBitsPerValueSpectrum = []int{1, 2, 4, 8, 12, 16, 20, 24, 28, 32, 40, 48, 56, 64}

func TestDirectWriterBitsRequired(t *testing.T) {
	t.Parallel()
	cases := []struct {
		max  int64
		want int
	}{
		{0, 1}, {1, 1}, {2, 2}, {3, 2}, {4, 4}, {7, 4}, {8, 4},
		{15, 4}, {16, 8}, {255, 8}, {256, 12}, {0xFFF, 12},
		{0x1000, 16}, {0x100000, 24}, {0x100000000, 40},
	}
	for _, c := range cases {
		if got := DirectWriterBitsRequired(c.max); got != c.want {
			t.Errorf("DirectWriterBitsRequired(%d) = %d, want %d", c.max, got, c.want)
		}
	}
}

func TestDirectWriterBytesRequired(t *testing.T) {
	t.Parallel()
	cases := []struct {
		n    int64
		bpv  int
		want int64
	}{
		{0, 8, 0},
		{1, 8, 1},
		{100, 8, 100},
		{8, 1, 1},
		{16, 4, 8},
		{10, 12, 15 + 1}, // (10*12 + 7) / 8 = 15; padding = 1 byte (16-12=4 bits)
		{10, 64, 80},     // padding=0 (64-64=0 bits)
		{10, 20, 25 + 2}, // (10*20)/8 = 25; padding = 2 bytes (32-20=12 bits -> 2 bytes)
	}
	for _, c := range cases {
		got, err := DirectWriterBytesRequired(c.n, c.bpv)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got != c.want {
			t.Errorf("DirectWriterBytesRequired(%d,%d) = %d, want %d", c.n, c.bpv, got, c.want)
		}
	}
}

func TestDirectWriterByteCompatibility(t *testing.T) {
	t.Parallel()
	// Encoding {1, 2, 3, 4} with bpv=8 produces bytes [1,2,3,4]+padding=0 bytes.
	out := store.NewByteArrayDataOutput(8)
	w, err := GetDirectWriter(out, 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []int64{1, 2, 3, 4} {
		if err := w.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	got := out.GetBytes()
	if len(got) != 4 || got[0] != 1 || got[1] != 2 || got[2] != 3 || got[3] != 4 {
		t.Fatalf("bytes: %v", got)
	}
}

// TestDirectWriter16Bit verifies the little-endian encoding for
// bpv=16. The reader (task 1090) will round-trip the same values.
func TestDirectWriter16Bit(t *testing.T) {
	t.Parallel()
	values := []int64{0x0102, 0xABCD, 0xFFFF}
	out := store.NewByteArrayDataOutput(16)
	w, err := GetDirectWriter(out, int64(len(values)), 16)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	got := out.GetBytes()
	expectedDataBytes := 2 * len(values)
	expectedPadding := 0 // 16<=8 false, 16<=16 true => 16-16=0 padding
	if len(got) != expectedDataBytes+expectedPadding {
		t.Fatalf("len=%d expected=%d (raw=%v)", len(got), expectedDataBytes+expectedPadding, got)
	}
	for i, v := range values {
		u := binary.LittleEndian.Uint16(got[i*2:])
		if int64(u) != v {
			t.Fatalf("[%d]: got %d want %d", i, u, v)
		}
	}
}

// TestDirectWriterRoundTripBytes uses encoded bytes to verify that
// encoding is deterministic across runs and matches the expected
// byte count.
func TestDirectWriterRoundTripBytes(t *testing.T) {
	t.Parallel()
	const n = 1024
	for _, bpv := range directBitsPerValueSpectrum {
		r := rand.New(rand.NewSource(int64(bpv) * 211))
		values := make([]int64, n)
		mask := uint64(MaxValue(bpv))
		for i := range values {
			if bpv == 64 {
				values[i] = int64(r.Uint64())
			} else {
				values[i] = int64(r.Uint64() & mask)
			}
		}
		out := store.NewByteArrayDataOutput(64)
		w, err := GetDirectWriter(out, n, bpv)
		if err != nil {
			t.Fatalf("getInstance bpv=%d err=%v", bpv, err)
		}
		for _, v := range values {
			if err := w.Add(v); err != nil {
				t.Fatalf("Add bpv=%d err=%v", bpv, err)
			}
		}
		if err := w.Finish(); err != nil {
			t.Fatalf("Finish bpv=%d err=%v", bpv, err)
		}
		gotBytes := int64(len(out.GetBytes()))
		wantBytes, _ := DirectWriterBytesRequired(n, bpv)
		if gotBytes != wantBytes {
			t.Fatalf("bpv=%d wrote %d bytes, expected %d", bpv, gotBytes, wantBytes)
		}
	}
}

func TestDirectWriterRejectsInvalidBitsPerValue(t *testing.T) {
	t.Parallel()
	for _, bpv := range []int{0, 3, 5, 7, 9, 11, 30, 33, 65} {
		out := store.NewByteArrayDataOutput(8)
		if _, err := GetDirectWriter(out, 4, bpv); err == nil {
			t.Errorf("bpv=%d: expected error", bpv)
		}
	}
}

func TestDirectWriterAddBeyondNumValuesFails(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(8)
	w, _ := GetDirectWriter(out, 2, 8)
	_ = w.Add(1)
	_ = w.Add(2)
	if err := w.Add(3); err == nil {
		t.Error("expected error when exceeding numValues")
	}
}

func TestDirectWriterFinishBeforeFull(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(8)
	w, _ := GetDirectWriter(out, 5, 8)
	_ = w.Add(1)
	if err := w.Finish(); err == nil {
		t.Error("expected error when finishing before adding all values")
	}
}
