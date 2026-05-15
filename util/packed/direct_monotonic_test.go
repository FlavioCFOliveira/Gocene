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

// trackingByteOutput is a tiny DataOutputAt that records every written
// byte and reports its own running position; used to feed both the
// meta and data streams when round-tripping DirectMonotonicWriter.
type trackingByteOutput struct {
	out *store.ByteArrayDataOutput
}

func newTrackingByteOutput(initialCapacity int) *trackingByteOutput {
	return &trackingByteOutput{out: store.NewByteArrayDataOutput(initialCapacity)}
}

func (t *trackingByteOutput) WriteByte(b byte) error    { return t.out.WriteByte(b) }
func (t *trackingByteOutput) WriteBytes(b []byte) error { return t.out.WriteBytes(b) }
func (t *trackingByteOutput) WriteBytesN(b []byte, n int) error {
	return t.out.WriteBytesN(b, n)
}
func (t *trackingByteOutput) WriteShort(i int16) error   { return t.out.WriteShort(i) }
func (t *trackingByteOutput) WriteInt(i int32) error     { return t.out.WriteInt(i) }
func (t *trackingByteOutput) WriteLong(i int64) error    { return t.out.WriteLong(i) }
func (t *trackingByteOutput) WriteString(s string) error { return t.out.WriteString(s) }
func (t *trackingByteOutput) GetFilePointer() int64      { return int64(len(t.out.GetBytes())) }

func (t *trackingByteOutput) Bytes() []byte { return t.out.GetBytes() }

// TestDirectMonotonicRoundTrip writes a known monotonic sequence and
// reads it back through DirectMonotonicReader using the meta produced
// by the writer.
func TestDirectMonotonicRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		blockShift int
		values     []int64
	}{
		{"perfectly linear", 6, linearSequence(0, 3, 256)},
		{"slightly noisy linear", 6, noisySequence(100, 5, 0.7, 256, 42)},
		{"all-zero", 5, make([]int64, 64)},
		{"single block", 4, linearSequence(7, 11, 12)},
		{"large slope", 8, linearSequence(0, 1_000_000, 600)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := newTrackingByteOutput(64)
			data := newTrackingByteOutput(64)
			w, err := NewDirectMonotonicWriter(meta, data, int64(len(tc.values)), tc.blockShift)
			if err != nil {
				t.Fatalf("NewDirectMonotonicWriter: %v", err)
			}
			for _, v := range tc.values {
				if err := w.Add(v); err != nil {
					t.Fatalf("Add(%d): %v", v, err)
				}
			}
			if err := w.Finish(); err != nil {
				t.Fatalf("Finish: %v", err)
			}

			metaIn := store.NewByteArrayDataInput(meta.Bytes())
			parsedMeta, err := LoadDirectMonotonicMeta(metaIn, int64(len(tc.values)), tc.blockShift)
			if err != nil {
				t.Fatalf("LoadDirectMonotonicMeta: %v", err)
			}
			dataIn := &byteSliceRandomAccess{data: data.Bytes()}
			reader, err := NewDirectMonotonicReader(parsedMeta, dataIn)
			if err != nil {
				t.Fatalf("NewDirectMonotonicReader: %v", err)
			}
			for i, want := range tc.values {
				if got := reader.Get(int64(i)); got != want {
					t.Errorf("[%d]: got %d want %d", i, got, want)
				}
			}
		})
	}
}

// TestDirectMonotonicBinarySearch verifies the binary search helper on
// a known sorted sequence with no duplicates.
func TestDirectMonotonicBinarySearch(t *testing.T) {
	t.Parallel()
	values := linearSequence(0, 7, 257) // 0, 7, 14, ..., 1792
	meta := newTrackingByteOutput(64)
	data := newTrackingByteOutput(64)
	const blockShift = 6
	w, err := NewDirectMonotonicWriter(meta, data, int64(len(values)), blockShift)
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
	metaIn := store.NewByteArrayDataInput(meta.Bytes())
	parsedMeta, err := LoadDirectMonotonicMeta(metaIn, int64(len(values)), blockShift)
	if err != nil {
		t.Fatal(err)
	}
	dataIn := &byteSliceRandomAccess{data: data.Bytes()}
	reader, err := NewDirectMonotonicReader(parsedMeta, dataIn)
	if err != nil {
		t.Fatal(err)
	}

	// Hit
	idx, err := reader.BinarySearch(0, int64(len(values)), 70) // values[10]
	if err != nil {
		t.Fatal(err)
	}
	if idx != 10 {
		t.Errorf("BinarySearch(70) = %d, want 10", idx)
	}
	// Miss — value 71 falls between values[10]=70 and values[11]=77.
	idx, err = reader.BinarySearch(0, int64(len(values)), 71)
	if err != nil {
		t.Fatal(err)
	}
	if idx >= 0 {
		t.Errorf("BinarySearch(71) = %d, want negative insertion point", idx)
	}
}

// TestDirectMonotonicRejectsBlockShift exercises the bounds-checking
// path so we don't silently accept invalid configurations.
func TestDirectMonotonicRejectsBlockShift(t *testing.T) {
	t.Parallel()
	meta := newTrackingByteOutput(0)
	data := newTrackingByteOutput(0)
	if _, err := NewDirectMonotonicWriter(meta, data, 10, DirectMonotonicMinBlockShift-1); err == nil {
		t.Error("expected error for blockShift below MIN, got nil")
	}
	if _, err := NewDirectMonotonicWriter(meta, data, 10, DirectMonotonicMaxBlockShift+1); err == nil {
		t.Error("expected error for blockShift above MAX, got nil")
	}
	if _, err := NewDirectMonotonicWriter(meta, data, -1, 6); err == nil {
		t.Error("expected error for negative numValues, got nil")
	}
}

func linearSequence(start, step int64, n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = start + step*int64(i)
	}
	return out
}

func noisySequence(start, step int64, jitter float64, n int, seed int64) []int64 {
	r := rand.New(rand.NewSource(seed))
	out := make([]int64, n)
	cur := start
	for i := range out {
		cur += step + int64(r.Float64()*jitter*float64(step))
		out[i] = cur
	}
	return out
}

// guard against unused imports if signatures move later
var _ = binary.LittleEndian
