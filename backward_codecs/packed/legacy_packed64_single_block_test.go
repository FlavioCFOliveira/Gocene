// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// writePackedSingleBlockNH writes valueCount values using the
// PACKED_SINGLE_BLOCK format (no header) into a ByteBuffersDirectory
// and returns an IndexInput positioned at the start.
func writePackedSingleBlockNH(t *testing.T, values []int64, bitsPerValue int) store.IndexInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	out, err := dir.CreateOutput("sb.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	w, err := packed.GetWriterNoHeader(out, packed.FormatPackedSingleBlock, len(values), bitsPerValue, 256)
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

	in, err := dir.OpenInput("sb.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return in
}

// TestLegacyPacked64SingleBlock_Roundtrip tests all supported bitsPerValue.
func TestLegacyPacked64SingleBlock_Roundtrip(t *testing.T) {
	supportedBpv := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 16, 21, 32}
	for _, bpv := range supportedBpv {
		bpv := bpv
		t.Run("", func(t *testing.T) {
			maxVal := int64((1 << uint(bpv)) - 1)
			n := 10
			values := make([]int64, n)
			for i := range values {
				values[i] = int64(i) % (maxVal + 1)
			}

			in := writePackedSingleBlockNH(t, values, bpv)
			r, err := newLegacyPacked64SingleBlock(in, n, bpv)
			if err != nil {
				t.Fatalf("bpv=%d: newLegacyPacked64SingleBlock: %v", bpv, err)
			}

			if got := r.Size(); got != n {
				t.Errorf("bpv=%d: Size(): got %d want %d", bpv, got, n)
			}
			for i, want := range values {
				if got := r.Get(i); got != want {
					t.Errorf("bpv=%d [%d]: got %d want %d", bpv, i, got, want)
				}
			}
		})
	}
}

// TestLegacyPacked64SingleBlock_UnsupportedBpv verifies that unsupported
// bitsPerValue returns an error.
func TestLegacyPacked64SingleBlock_UnsupportedBpv(t *testing.T) {
	if _, err := newLegacyPacked64SingleBlock(nil, 10, 11); err == nil {
		t.Error("bpv=11: expected error for unsupported bitsPerValue")
	}
}

// TestLegacyPacked64SingleBlock_GetBulk verifies GetBulk reads same values as Get.
func TestLegacyPacked64SingleBlock_GetBulk(t *testing.T) {
	values := []int64{0, 1, 2, 3, 4, 5, 6, 7}
	bpv := 4
	in := writePackedSingleBlockNH(t, values, bpv)

	r, err := newLegacyPacked64SingleBlock(in, len(values), bpv)
	if err != nil {
		t.Fatalf("newLegacyPacked64SingleBlock: %v", err)
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

// TestLegacyPacked64SingleBlock_String verifies String returns non-empty.
func TestLegacyPacked64SingleBlock_String(t *testing.T) {
	in := writePackedSingleBlockNH(t, []int64{1, 2}, 4)
	r, err := newLegacyPacked64SingleBlock(in, 2, 4)
	if err != nil {
		t.Fatalf("newLegacyPacked64SingleBlock: %v", err)
	}
	if s := r.String(); s == "" {
		t.Error("String(): expected non-empty")
	}
}

// TestIsSupported verifies the IsSupported helper.
func TestIsSupported(t *testing.T) {
	supported := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 16, 21, 32}
	notSupported := []int{0, 11, 13, 15, 17, 20, 31, 33, 64}

	for _, bpv := range supported {
		if !IsSupported(bpv) {
			t.Errorf("IsSupported(%d): expected true", bpv)
		}
	}
	for _, bpv := range notSupported {
		if IsSupported(bpv) {
			t.Errorf("IsSupported(%d): expected false", bpv)
		}
	}
}

// TestGetReaderNoHeaderFormat_SingleBlock verifies GetReaderNoHeaderFormat
// with FormatPackedSingleBlock.
func TestGetReaderNoHeaderFormat_SingleBlock(t *testing.T) {
	values := []int64{0, 7, 3, 15}
	bpv := 4
	in := writePackedSingleBlockNH(t, values, bpv)

	r, err := GetReaderNoHeaderFormat(in, packed.FormatPackedSingleBlock, packed.VersionCurrent, len(values), bpv)
	if err != nil {
		t.Fatalf("GetReaderNoHeaderFormat: %v", err)
	}
	for i, want := range values {
		if got := r.Get(i); got != want {
			t.Errorf("[%d]: got %d want %d", i, got, want)
		}
	}
}

// TestGetReaderNoHeaderFormat_UnknownFormat verifies error for unknown format.
func TestGetReaderNoHeaderFormat_UnknownFormat(t *testing.T) {
	if _, err := GetReaderNoHeaderFormat(nil, packed.Format(99), packed.VersionCurrent, 4, 4); err == nil {
		t.Error("unknown format: expected error")
	}
}
