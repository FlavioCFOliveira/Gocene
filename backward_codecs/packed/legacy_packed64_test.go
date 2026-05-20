// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// writePackedNH writes valueCount values using the PACKED format (no header)
// into a ByteBuffersDirectory and returns an IndexInput positioned at the start.
func writePackedNH(t *testing.T, values []int64, bitsPerValue int) store.IndexInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	out, err := dir.CreateOutput("test.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	w, err := packed.GetWriterNoHeader(out, packed.FormatPacked, len(values), bitsPerValue, 256)
	if err != nil {
		t.Fatalf("GetWriterNoHeader: %v", err)
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			t.Fatalf("Add(%d): %v", v, err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	in, err := dir.OpenInput("test.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return in
}

// TestLegacyPacked64_Roundtrip writes N values with a given bitsPerValue,
// then reads them back via newLegacyPacked64 and verifies equality.
func TestLegacyPacked64_Roundtrip(t *testing.T) {
	tests := []struct {
		name         string
		values       []int64
		bitsPerValue int
	}{
		{
			name:         "single value bpv=8",
			values:       []int64{42},
			bitsPerValue: 8,
		},
		{
			name:         "four values bpv=4",
			values:       []int64{0, 7, 8, 15},
			bitsPerValue: 4,
		},
		{
			name:         "cross-long-boundary bpv=5",
			values:       []int64{31, 0, 15, 8, 1, 5, 3, 2, 31, 31, 0, 1, 2, 3},
			bitsPerValue: 5,
		},
		{
			name:         "bpv=1 all zeros",
			values:       []int64{0, 0, 0, 0, 0, 0, 0, 0},
			bitsPerValue: 1,
		},
		{
			name:         "bpv=1 alternating",
			values:       []int64{1, 0, 1, 0, 1, 0, 1, 0},
			bitsPerValue: 1,
		},
		{
			name:         "bpv=32",
			values:       []int64{0, 1, (1 << 32) - 1, 12345678},
			bitsPerValue: 32,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := writePackedNH(t, tc.values, tc.bitsPerValue)

			r, err := newLegacyPacked64(packed.VersionCurrent, in, len(tc.values), tc.bitsPerValue)
			if err != nil {
				t.Fatalf("newLegacyPacked64: %v", err)
			}

			if got := r.Size(); got != len(tc.values) {
				t.Errorf("Size(): got %d want %d", got, len(tc.values))
			}

			for i, want := range tc.values {
				if got := r.Get(i); got != want {
					t.Errorf("[%d] Get: got %d want %d", i, got, want)
				}
			}
		})
	}
}

// TestLegacyPacked64_GetBulk verifies GetBulk reads the same values as Get.
func TestLegacyPacked64_GetBulk(t *testing.T) {
	values := []int64{3, 1, 4, 1, 5, 9, 2, 6}
	bpv := 4
	in := writePackedNH(t, values, bpv)

	r, err := newLegacyPacked64(packed.VersionCurrent, in, len(values), bpv)
	if err != nil {
		t.Fatalf("newLegacyPacked64: %v", err)
	}

	buf := make([]int64, len(values))
	n := r.GetBulk(0, buf, 0, len(values))
	if n != len(values) {
		t.Fatalf("GetBulk returned %d want %d", n, len(values))
	}
	for i, want := range values {
		if buf[i] != want {
			t.Errorf("[%d]: got %d want %d", i, buf[i], want)
		}
	}
}

// TestLegacyPacked64_String verifies String() is non-empty.
func TestLegacyPacked64_String(t *testing.T) {
	in := writePackedNH(t, []int64{1, 2}, 4)
	r, err := newLegacyPacked64(packed.VersionCurrent, in, 2, 4)
	if err != nil {
		t.Fatalf("newLegacyPacked64: %v", err)
	}
	if s := r.String(); s == "" {
		t.Error("String(): expected non-empty")
	}
}

// TestGetReaderNoHeader_PackedFormat verifies GetReaderNoHeader wraps
// newLegacyPacked64 correctly for the PACKED format.
func TestGetReaderNoHeader_PackedFormat(t *testing.T) {
	values := []int64{0, 1, 2, 3, 4, 5, 6, 7}
	bpv := 3
	in := writePackedNH(t, values, bpv)

	r, err := GetReaderNoHeader(in, packed.VersionCurrent, len(values), bpv)
	if err != nil {
		t.Fatalf("GetReaderNoHeader: %v", err)
	}
	if r.Size() != len(values) {
		t.Errorf("Size(): got %d want %d", r.Size(), len(values))
	}
	for i, want := range values {
		if got := r.Get(i); got != want {
			t.Errorf("[%d] Get: got %d want %d", i, got, want)
		}
	}
}

// TestGetReaderNoHeader_ZeroBpv verifies the zero-reader path when bitsPerValue == 0.
func TestGetReaderNoHeader_ZeroBpv(t *testing.T) {
	r, err := GetReaderNoHeader(nil, packed.VersionCurrent, 10, 0)
	if err != nil {
		t.Fatalf("GetReaderNoHeader (bpv=0): %v", err)
	}
	if r.Size() != 10 {
		t.Errorf("Size(): got %d want 10", r.Size())
	}
	for i := 0; i < 10; i++ {
		if v := r.Get(i); v != 0 {
			t.Errorf("Get(%d): got %d want 0", i, v)
		}
	}
}

// TestGetReaderNoHeader_InvalidVersion verifies that an unsupported version
// returns an error.
func TestGetReaderNoHeader_InvalidVersion(t *testing.T) {
	_, err := GetReaderNoHeader(nil, -1, 4, 4)
	if err == nil {
		t.Error("expected error for invalid version")
	}
}
