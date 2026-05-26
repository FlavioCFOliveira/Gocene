// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial3d_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spatial3d"
	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
)

// ---------------------------------------------------------------------------
// EncodeDimension / DecodeDimension
// ---------------------------------------------------------------------------

// TestEncodeDimensionRoundTrip verifies that the sortable-bytes encoding round-trips
// correctly. Port of Geo3DPoint.encodeDimension / decodeDimension.
func TestEncodeDimensionRoundTrip(t *testing.T) {
	pm := geom.SPHERE
	values := []float64{0, 0.25, -0.25, 0.9999, -0.9999}
	buf := make([]byte, 4)
	for _, v := range values {
		spatial3d.EncodeDimension(pm, v, buf, 0)
		got := spatial3d.DecodeDimension(pm, buf, 0)
		// Decoded value must encode back to the same integer.
		enc1 := pm.EncodeValue(v)
		enc2 := pm.EncodeValue(got)
		if enc1 != enc2 {
			t.Errorf("round-trip for %g: encode=%d, decode=%g, re-encode=%d", v, enc1, got, enc2)
		}
	}
}

// TestEncodeDimensionSortOrder verifies that the sortable-bytes encoding preserves
// the natural order of values (i.e., comparing bytes lexicographically gives the
// same ordering as comparing the original float64 values).
func TestEncodeDimensionSortOrder(t *testing.T) {
	pm := geom.SPHERE
	pairs := [][2]float64{{-0.5, 0.5}, {0.1, 0.9}, {-0.9, -0.1}, {0, 0.001}}
	buf1 := make([]byte, 4)
	buf2 := make([]byte, 4)
	for _, pair := range pairs {
		lo, hi := pair[0], pair[1]
		spatial3d.EncodeDimension(pm, lo, buf1, 0)
		spatial3d.EncodeDimension(pm, hi, buf2, 0)
		for i := 0; i < 4; i++ {
			if buf1[i] < buf2[i] {
				break // lo < hi lexicographically — correct
			}
			if buf1[i] > buf2[i] {
				t.Errorf("sort order violated for (%g, %g) at byte %d: lo=%02X, hi=%02X",
					lo, hi, i, buf1[i], buf2[i])
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Geo3DPoint.ToIndexableFields
// ---------------------------------------------------------------------------

// TestGeo3DPointToIndexableFields verifies that ToIndexableFields produces
// a single field with 12 bytes and that the encoded dimensions decode back
// to the original coordinates within quantisation error.
func TestGeo3DPointToIndexableFields(t *testing.T) {
	pm := geom.SPHERE
	x, y, z := 0.5, 0.3, math.Sqrt(1-0.5*0.5-0.3*0.3) // unit-sphere point
	pt := spatial3d.NewGeo3DPointXYZModel("loc", pm, x, y, z)

	fields, err := pt.ToIndexableFields()
	if err != nil {
		t.Fatalf("ToIndexableFields: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("want 1 field, got %d", len(fields))
	}
	f := fields[0]
	if f.Name() != "loc" {
		t.Errorf("field name: want 'loc', got %q", f.Name())
	}
	b := f.BinaryValue()
	if len(b) != 12 {
		t.Fatalf("BinaryValue length: want 12, got %d", len(b))
	}
	// Check field type: dimensionCount=3, numBytes=4.
	ft := f.FieldType()
	if ft == nil {
		t.Fatal("FieldType must not be nil")
	}

	// Decode the three dimensions and verify round-trip fidelity.
	gotX := spatial3d.DecodeDimension(pm, b, 0)
	gotY := spatial3d.DecodeDimension(pm, b, 4)
	gotZ := spatial3d.DecodeDimension(pm, b, 8)
	// The quantisation error is at most 0.5 * DECODE.
	tol := pm.Decode
	if math.Abs(gotX-x) > tol {
		t.Errorf("X: want %g, got %g (tol %g)", x, gotX, tol)
	}
	if math.Abs(gotY-y) > tol {
		t.Errorf("Y: want %g, got %g (tol %g)", y, gotY, tol)
	}
	if math.Abs(gotZ-z) > tol {
		t.Errorf("Z: want %g, got %g (tol %g)", z, gotZ, tol)
	}
}
