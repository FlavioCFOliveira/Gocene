// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// writePackedStream serializes valueCount values of bitsPerValue
// width using PackedWriter (FormatPacked) into the named file in
// dir. The on-disk byte layout is the one DirectPackedReader is
// designed to consume — i.e., the same layout Lucene's reference
// PackedWriter produces, with the most significant byte of each
// 64-bit conceptual block written first.
func writePackedStream(t *testing.T, dir *store.ByteBuffersDirectory, name string, bitsPerValue, valueCount int, values []int64) {
	t.Helper()
	out, err := dir.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	w, err := newPackedWriter(FormatPacked, out, valueCount, bitsPerValue, 256)
	if err != nil {
		t.Fatalf("newPackedWriter: %v", err)
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// maskFor returns the unsigned mask for bitsPerValue.
func maskFor(bpv int) uint64 {
	if bpv == 64 {
		return ^uint64(0)
	}
	return (uint64(1) << uint(bpv)) - 1
}

// TestDirectPackedReader_RoundTripAgainstPackedWriter exercises
// every supported bitsPerValue (1..64). For each width it generates
// pseudo-random values fitting in bpv bits, writes them with the
// reference PackedWriter (FormatPacked), then asserts that
// DirectPackedReader returns each one at the corresponding index.
// This covers the full switch in Get, including all multi-byte
// branches (cases 1..9) reached as bpv grows.
func TestDirectPackedReader_RoundTripAgainstPackedWriter(t *testing.T) {
	t.Parallel()
	const valueCount = 257 // odd, > one bulk iteration

	for bpv := 1; bpv <= 64; bpv++ {
		bpv := bpv
		t.Run(name(bpv), func(t *testing.T) {
			t.Parallel()
			mask := maskFor(bpv)
			r := rand.New(rand.NewSource(int64(bpv) * 1_000_003))
			want := make([]int64, valueCount)
			for i := range want {
				want[i] = int64(r.Uint64() & mask)
			}

			dir := store.NewByteBuffersDirectory()
			fname := "packed.bin"
			writePackedStream(t, dir, fname, bpv, valueCount, want)

			in, err := dir.OpenInput(fname, store.IOContext{Context: store.ContextRead})
			if err != nil {
				t.Fatalf("OpenInput: %v", err)
			}
			defer in.Close()

			reader := NewDirectPackedReader(bpv, valueCount, in)
			if got := reader.Size(); got != valueCount {
				t.Fatalf("Size: got %d want %d", got, valueCount)
			}
			if got := reader.RamBytesUsed(); got != 0 {
				t.Fatalf("RamBytesUsed: got %d want 0", got)
			}

			for i := 0; i < valueCount; i++ {
				if got := reader.Get(i); got != want[i] {
					t.Fatalf("Get(%d): got %d want %d", i, got, want[i])
				}
			}

			bulk := make([]int64, valueCount)
			n := reader.GetBulk(0, bulk, 0, valueCount)
			if n != valueCount {
				t.Fatalf("GetBulk n: got %d want %d", n, valueCount)
			}
			for i, v := range bulk {
				if v != want[i] {
					t.Fatalf("GetBulk[%d]: got %d want %d", i, v, want[i])
				}
			}
		})
	}
}

// TestDirectPackedReader_HonoursStartPointer verifies that the
// reader anchors its byte offsets at the file pointer captured at
// construction time, not at zero.
func TestDirectPackedReader_HonoursStartPointer(t *testing.T) {
	t.Parallel()
	const bpv = 17
	const valueCount = 32
	const prefix = 13

	mask := maskFor(bpv)
	want := make([]int64, valueCount)
	for i := range want {
		want[i] = int64(uint64(i*0x9E3779B1) & mask)
	}

	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput("prefixed.bin", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes(make([]byte, prefix)); err != nil {
		t.Fatalf("WriteBytes prefix: %v", err)
	}
	w, err := newPackedWriter(FormatPacked, out, valueCount, bpv, 256)
	if err != nil {
		t.Fatalf("newPackedWriter: %v", err)
	}
	for _, v := range want {
		if err := w.Add(v); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("prefixed.bin", store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()
	if err := in.SetPosition(prefix); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}

	reader := NewDirectPackedReader(bpv, valueCount, in)
	for i := 0; i < valueCount; i++ {
		if got := reader.Get(i); got != want[i] {
			t.Fatalf("Get(%d): got %d want %d", i, got, want[i])
		}
	}
}

// TestDirectPackedReader_KnownVectorBE pins the big-endian wire
// contract with a hand-built byte fixture. Three 16-bit values
// packed at bpv=16 must yield 0x1234, 0x5678, 0x9ABC. If the
// reader regressed to little-endian (e.g., by calling ReadShort/
// ReadInt/ReadLong on store.IndexInput) this test would fail.
func TestDirectPackedReader_KnownVectorBE(t *testing.T) {
	t.Parallel()
	fixture := []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC}
	want := []int64{0x1234, 0x5678, 0x9ABC}

	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput("fixture.bin", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes(fixture); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("fixture.bin", store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	reader := NewDirectPackedReader(16, len(want), in)
	for i, w := range want {
		if got := reader.Get(i); got != w {
			t.Fatalf("Get(%d): got 0x%X want 0x%X", i, got, w)
		}
	}
}

// TestDirectPackedReader_BitsPerValue64 exercises the bpv=64
// branch, which uses the valueMask=-1L shortcut in the reference.
func TestDirectPackedReader_BitsPerValue64(t *testing.T) {
	t.Parallel()
	maxI64 := int64(^uint64(0) >> 1) // 0x7FFF...FF
	minI64 := -maxI64 - 1            // 0x8000...00
	want := []int64{0, 1, -1, 1 << 62, maxI64, minI64}
	valueCount := len(want)

	dir := store.NewByteBuffersDirectory()
	writePackedStream(t, dir, "u64.bin", 64, valueCount, want)

	in, err := dir.OpenInput("u64.bin", store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	reader := NewDirectPackedReader(64, valueCount, in)
	for i, w := range want {
		if got := reader.Get(i); got != w {
			t.Fatalf("Get(%d): got 0x%X want 0x%X", i, uint64(got), uint64(w))
		}
	}
}

// name returns a stable subtest label for a given bitsPerValue.
func name(bpv int) string {
	const digits = "0123456789"
	if bpv < 10 {
		return "bpv=0" + string(digits[bpv])
	}
	return "bpv=" + string(digits[bpv/10]) + string(digits[bpv%10])
}
