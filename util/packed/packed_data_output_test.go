// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestPackedDataOutputDeterministic verifies that writing the same
// sequence with the same bitsPerValue produces a byte-for-byte
// deterministic stream. This guards against accidental endianness
// or bit-ordering changes.
func TestPackedDataOutputDeterministic(t *testing.T) {
	t.Parallel()
	first := encodeFixed(t, []int64{0xAA, 0x55, 0xFF}, 8)
	second := encodeFixed(t, []int64{0xAA, 0x55, 0xFF}, 8)
	if !bytes.Equal(first, second) {
		t.Fatalf("non-deterministic output:\n  first  = %x\n  second = %x", first, second)
	}
}

// TestPackedDataOutputKnownLayout exercises the explicit byte
// layout for a tiny sequence to lock the wire format. The values
// {5, 2, 7} at 3 bits per value pack into the bit stream
// 101_010_111 (9 bits). The first 8 bits form 0b10101011 = 0xAB.
// The remaining 1 bit is the most significant bit of the second
// byte: 0b10000000 = 0x80 after flushing pads the trailing 7 bits
// with zeros.
func TestPackedDataOutputKnownLayout(t *testing.T) {
	t.Parallel()
	got := encodeFixed(t, []int64{5, 2, 7}, 3)
	want := []byte{0xAB, 0x80}
	if !bytes.Equal(got, want) {
		t.Fatalf("layout: got %x want %x", got, want)
	}
}

func encodeFixed(t *testing.T, values []int64, bpv int) []byte {
	t.Helper()
	out := store.NewByteArrayDataOutput(8)
	w := NewPackedDataOutput(out)
	for _, v := range values {
		if err := w.WriteLong(v, bpv); err != nil {
			t.Fatalf("WriteLong: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	return out.GetBytes()
}

// TestPackedDataOutputRejectsOverflow verifies that writing a value
// that does not fit in the declared bitsPerValue returns an error.
func TestPackedDataOutputRejectsOverflow(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(8)
	w := NewPackedDataOutput(out)
	if err := w.WriteLong(256, 8); err == nil {
		t.Fatalf("expected error for value=256 with bpv=8")
	}
	if err := w.WriteLong(-1, 8); err == nil {
		t.Fatalf("expected error for negative value with bpv=8")
	}
}

// TestPackedDataOutputAllowsFull64 verifies that bitsPerValue=64
// accepts any int64.
func TestPackedDataOutputAllowsFull64(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(8)
	w := NewPackedDataOutput(out)
	for _, v := range []int64{0, -1, 0x7FFFFFFFFFFFFFFF, -0x8000000000000000} {
		if err := w.WriteLong(v, 64); err != nil {
			t.Fatalf("WriteLong(%d, 64) err=%v", v, err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	r := NewPackedDataInput(in)
	want := []int64{0, -1, 0x7FFFFFFFFFFFFFFF, -0x8000000000000000}
	for i, w := range want {
		got, err := r.ReadLong(64)
		if err != nil {
			t.Fatalf("ReadLong[%d]: %v", i, err)
		}
		if got != w {
			t.Fatalf("[%d]: got %d want %d", i, got, w)
		}
	}
}
