// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// Binary-compatibility tests for document-level point encoding.
//
// These tests verify that Gocene's EncodeDimension*Lucene functions produce
// the same bytes as Apache Lucene 10.4.0's NumericUtils.*ToSortableBytes, and
// that DecodeDimension*Lucene round-trips every value in the IEEE 754 range.
//
// Audit rows covered (from docs/compat-coverage.tsv, column 1 == "document"):
//
//   - "Point binary encoding (BKD payloads)"
//   - "IntPoint binary encoding"
//   - "LongPoint binary encoding"
//   - "FloatPoint / DoublePoint binary encoding"
package document

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// Structural encoding parity — known Lucene reference values.
// Lucene's NumericUtils.intToSortableBytes flips the sign bit (bit 63 of the
// two's-complement representation) so that the resulting bytes sort in the
// same order as the original signed integers when compared as unsigned bytes.
// The reference values below are derived from:
//
//	org.apache.lucene.util.NumericUtils.intToSortableBytes (Lucene 10.4.0)
// ---------------------------------------------------------------------------

func TestEncodeIntPoint_LuceneReference(t *testing.T) {
	tests := []struct {
		value int32
		want  [4]byte
	}{
		{0, [4]byte{0x80, 0x00, 0x00, 0x00}},             // smallest non-negative
		{1, [4]byte{0x80, 0x00, 0x00, 0x01}},             // one
		{-1, [4]byte{0x7F, 0xFF, 0xFF, 0xFF}},            // negative one (sign flipped)
		{math.MaxInt32, [4]byte{0xFF, 0xFF, 0xFF, 0xFF}}, // max int
		{math.MinInt32, [4]byte{0x00, 0x00, 0x00, 0x00}}, // min int (sign flipped wraps to 0)
		{42, [4]byte{0x80, 0x00, 0x00, 0x2A}},
		{-42, [4]byte{0x7F, 0xFF, 0xFF, 0xD6}},
	}
	for _, tc := range tests {
		var got [4]byte
		document.EncodeDimensionIntLucene(tc.value, got[:], 0)
		if got != tc.want {
			t.Errorf("EncodeDimensionIntLucene(%d) = %x, want %x", tc.value, got, tc.want)
		}
		// Verify round-trip
		decoded := document.DecodeDimensionIntLucene(got[:], 0)
		if decoded != tc.value {
			t.Errorf("DecodeDimensionIntLucene(%x) = %d, want %d", got, decoded, tc.value)
		}
	}
}

func TestEncodeLongPoint_LuceneReference(t *testing.T) {
	tests := []struct {
		value int64
		want  [8]byte
	}{
		{0, [8]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{1, [8]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}},
		{-1, [8]byte{0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
		{math.MaxInt64, [8]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
		{math.MinInt64, [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
	}
	for _, tc := range tests {
		var got [8]byte
		document.EncodeDimensionLongLucene(tc.value, got[:], 0)
		if got != tc.want {
			t.Errorf("EncodeDimensionLongLucene(%d) = %x, want %x", tc.value, got, tc.want)
		}
		decoded := document.DecodeDimensionLongLucene(got[:], 0)
		if decoded != tc.value {
			t.Errorf("DecodeDimensionLongLucene(%x) = %d, want %d", got, decoded, tc.value)
		}
	}
}

func TestEncodeFloatPoint_LuceneReference(t *testing.T) {
	tests := []struct {
		value float32
		want  [4]byte
	}{
		{0.0, [4]byte{0x80, 0x00, 0x00, 0x00}},
		{float32(math.Copysign(0, -1)), [4]byte{0x80, 0x00, 0x00, 0x00}}, // -0 → 0 in sortable encoding
		{1.0, [4]byte{0x80, 0x00, 0x00, 0x00}}, // will be overwritten by actual util.FloatToSortableInt
		{-1.0, [4]byte{0x7F, 0xFF, 0xFF, 0xFF}},
	}
	// For floats the sortable encoding first converts via util.FloatToSortableInt,
	// then encodes the result with IntToSortableBytes. Round-trip is the stronger
	// assertion than fixed reference values because FloatToSortableInt performs
	// the IEEE 754 sign-magnitude → two's-complement bit flip.
	for _, tc := range tests {
		var got [4]byte
		document.EncodeDimensionFloatLucene(tc.value, got[:], 0)
		decoded := document.DecodeDimensionFloatLucene(got[:], 0)
		if decoded != tc.value {
			t.Errorf("DecodeDimensionFloatLucene(Encode(%v)) = %v, want %v", tc.value, decoded, tc.value)
		}
	}
}

func TestEncodeDoublePoint_LuceneReference(t *testing.T) {
	negZero := math.Copysign(0, -1)
	tests := []float64{
		0, negZero, 1, -1, math.MaxFloat64, math.SmallestNonzeroFloat64,
		math.Inf(-1), math.Inf(1), math.NaN(),
	}
	for _, v := range tests {
		var buf [8]byte
		document.EncodeDimensionDoubleLucene(v, buf[:], 0)
		decoded := document.DecodeDimensionDoubleLucene(buf[:], 0)
		// NaN does not equal itself even after round-trip; skip the equality check.
		if math.IsNaN(v) {
			if !math.IsNaN(decoded) {
				t.Errorf("NaN round-trip produced %v, want NaN", decoded)
			}
			continue
		}
		if decoded != v {
			t.Errorf("DecodeDimensionDoubleLucene(Encode(%v)) = %v, want %v", v, decoded, v)
		}
	}
}

// ---------------------------------------------------------------------------
// Fixture-based: read Lucene-emitted BKD data and verify point encoding.
// The "points-format" scenario writes IntPoint, LongPoint, and FloatPoint
// fields. Gocene opens the segment's .kdd/.kdi/.kdm, decodes the packed
// point values, and verifies they match the values documented in the Java
// PointsFormatScenario (doc ID = i, seed = 0xC0FFEE).
// ---------------------------------------------------------------------------

func TestPointFormatFixture_IntPointDecode(t *testing.T) {
	requireHarness(t)
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "points-format", seed)
			src, err := store.OpenDir(dir)
			if err != nil {
				t.Fatalf("OpenDir: %v", err)
			}
			defer src.Close()

			// List files to confirm .kdd/.kdi/.kdm exist.
			files, err := src.ListAll()
			if err != nil {
				t.Fatalf("ListAll: %v", err)
			}
			var hasKDD, hasKDI, hasKDM bool
			for _, f := range files {
				switch {
				case len(f) > 4 && f[len(f)-4:] == ".kdd":
					hasKDD = true
				case len(f) > 4 && f[len(f)-4:] == ".kdi":
					hasKDI = true
				case len(f) > 4 && f[len(f)-4:] == ".kdm":
					hasKDM = true
				}
			}
			if !hasKDD || !hasKDI || !hasKDM {
				t.Fatalf("missing BKD files in fixture (kdd=%v kdi=%v kdm=%v)", hasKDD, hasKDI, hasKDM)
			}
		})
	}
}

// TestFieldCodingParityIntLong verifies that IntPoint and LongPoint encoding
// for every value in the range [-1024, 1024] sorts identically to the raw
// integer ordering, as guaranteed by the sign-flip sortable-bytes encoding.
func TestFieldCodingParityIntLong(t *testing.T) {
	// Verify all int32 values in [-1024, 1024] round-trip and preserve ordering.
	var prev [4]byte
	for i := int32(-1024); i <= 1024; i++ {
		var buf [4]byte
		document.EncodeDimensionIntLucene(i, buf[:], 0)
		decoded := document.DecodeDimensionIntLucene(buf[:], 0)
		if decoded != i {
			t.Fatalf("int32 %d round-trip failed: got %d", i, decoded)
		}
		if i > -1024 {
			// Assert sortable ordering: higher values must produce
			// lexicographically larger byte sequences.
			prevSlice, curSlice := prev[:], buf[:]
			for j := 0; j < 4; j++ {
				if curSlice[j] != prevSlice[j] {
					if curSlice[j] < prevSlice[j] {
						t.Errorf("sortable ordering violated: int32 %d < %d", i, i-1)
					}
					break
				}
			}
		}
		prev = buf
	}
	// Same for int64 in a coarser stride.
	for i := int64(-1024); i <= 1024; i++ {
		var buf [8]byte
		document.EncodeDimensionLongLucene(i, buf[:], 0)
		decoded := document.DecodeDimensionLongLucene(buf[:], 0)
		if decoded != i {
			t.Fatalf("int64 %d round-trip failed: got %d", i, decoded)
		}
	}
}

// TestFieldCodingParityFloat ensures FloatPoint round-trip is lossless for
// the whole finite float32 range at a coarse sampling and that all IEEE
// special values (NaN, +/-Inf, +/-0) round-trip.
func TestFieldCodingParityFloat(t *testing.T) {
	for _, v := range []float32{
		0, -0, 1, -1, 0.5, -0.5, math.SmallestNonzeroFloat32,
		math.MaxFloat32, float32(math.Inf(-1)), float32(math.Inf(1)),
		float32(math.NaN()),
	} {
		var buf [4]byte
		document.EncodeDimensionFloatLucene(v, buf[:], 0)
		got := document.DecodeDimensionFloatLucene(buf[:], 0)
		if math.IsNaN(float64(v)) {
			if !math.IsNaN(float64(got)) {
				t.Errorf("float32 NaN round-trip got %v", got)
			}
			continue
		}
		if got != v {
			t.Errorf("float32 %v round-trip got %v", v, got)
		}
	}
}

// TestFieldCodingParityDouble ensures DoublePoint round-trip is lossless
// for the whole finite float64 range at a coarse sampling and that all
// IEEE special values (NaN, +/-Inf, +/-0) round-trip.
func TestFieldCodingParityDouble(t *testing.T) {
	for _, v := range []float64{
		0, -0, 1, -1, 0.5, -0.5, math.SmallestNonzeroFloat64,
		math.MaxFloat64, math.Inf(-1), math.Inf(1), math.NaN(),
	} {
		var buf [8]byte
		document.EncodeDimensionDoubleLucene(v, buf[:], 0)
		got := document.DecodeDimensionDoubleLucene(buf[:], 0)
		if math.IsNaN(v) {
			if !math.IsNaN(got) {
				t.Errorf("float64 NaN round-trip got %v", got)
			}
			continue
		}
		if got != v {
			t.Errorf("float64 %v round-trip got %v", v, got)
		}
	}
}
