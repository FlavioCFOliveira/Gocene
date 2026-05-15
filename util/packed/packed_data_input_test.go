// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestPackedDataInputRoundTrip exercises PackedDataOutput ->
// PackedDataInput for every bitsPerValue in the test spectrum.
//
// The output side is exercised here even though its dedicated test
// peer lives in PackedDataOutput's file — round-tripping is the
// only meaningful integration test for these two cooperating types.
func TestPackedDataInputRoundTrip(t *testing.T) {
	t.Parallel()
	const n = 1024
	for _, bpv := range bitsPerValueSpectrum {
		r := rand.New(rand.NewSource(int64(bpv) * 17))
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
		pdo := NewPackedDataOutput(out)
		for _, v := range values {
			if err := pdo.WriteLong(v, bpv); err != nil {
				t.Fatalf("WriteLong bpv=%d err=%v", bpv, err)
			}
		}
		if err := pdo.Flush(); err != nil {
			t.Fatalf("Flush err=%v", err)
		}

		in := store.NewByteArrayDataInput(out.GetBytes())
		pdi := NewPackedDataInput(in)
		for i, want := range values {
			got, err := pdi.ReadLong(bpv)
			if err != nil {
				t.Fatalf("ReadLong bpv=%d i=%d err=%v", bpv, i, err)
			}
			if got != want {
				t.Fatalf("bpv=%d i=%d: got %d, want %d", bpv, i, got, want)
			}
		}
	}
}

// TestPackedDataInputSkipToNextByte verifies that SkipToNextByte
// realigns reads on the next byte boundary.
func TestPackedDataInputSkipToNextByte(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(8)
	pdo := NewPackedDataOutput(out)
	// 3 + 3 = 6 bits in first byte; the byte will have 2 trailing zero pad bits.
	_ = pdo.WriteLong(5, 3) // 101
	_ = pdo.WriteLong(2, 3) // 010
	_ = pdo.Flush()
	// Now write a full byte starting at the new boundary.
	if err := out.WriteByte(0xAB); err != nil {
		t.Fatal(err)
	}

	in := store.NewByteArrayDataInput(out.GetBytes())
	pdi := NewPackedDataInput(in)
	if v, _ := pdi.ReadLong(3); v != 5 {
		t.Fatalf("first read: %d", v)
	}
	if v, _ := pdi.ReadLong(3); v != 2 {
		t.Fatalf("second read: %d", v)
	}
	pdi.SkipToNextByte()
	b, err := in.ReadByte()
	if err != nil {
		t.Fatal(err)
	}
	if b != 0xAB {
		t.Fatalf("trailing byte: %x", b)
	}
}
