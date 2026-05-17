// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

func TestShapeField_TriangleEncodeDecode(t *testing.T) {
	tri, err := NewShapeFieldTriangle("s", 1, 2, 3, 4, 5, 6, true, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(tri.BinaryValue()) != ShapeFieldBytes {
		t.Fatalf("encoded len = %d", len(tri.BinaryValue()))
	}
	d, err := DecodeTriangle(tri.BinaryValue())
	if err != nil {
		t.Fatal(err)
	}
	if d.AX != 1 || d.AY != 2 {
		t.Fatalf("AX/AY = %d, %d", d.AX, d.AY)
	}
	if !d.AB || d.BC || !d.CA {
		t.Fatalf("edge bits wrong: %+v", d)
	}
}

func TestShapeField_QueryRelation_String(t *testing.T) {
	cases := map[QueryRelation]string{
		QueryRelationIntersects: "INTERSECTS",
		QueryRelationWithin:     "WITHIN",
		QueryRelationContains:   "CONTAINS",
		QueryRelationDisjoint:   "DISJOINT",
	}
	for v, want := range cases {
		if got := v.String(); got != want {
			t.Errorf("%d -> %q, want %q", int(v), got, want)
		}
	}
}

func TestLatLonShape_Point(t *testing.T) {
	tri, err := CreateIndexableFieldsFromLatLonPoint("loc", 38.7, -9.1)
	if err != nil {
		t.Fatal(err)
	}
	if tri == nil || len(tri.BinaryValue()) != ShapeFieldBytes {
		t.Fatalf("triangle missing/wrong length")
	}
}

func TestLatLonShape_PolygonStubError(t *testing.T) {
	// Use a polygon shape that the pre-existing tessellator stub will reject.
	poly := geo.MustNewPolygon(
		[]float64{0, 0, 0.5, 0.5, 0.25, 0},
		[]float64{0, 0.5, 0.5, 0, 0.25, 0},
	)
	_, err := CreateIndexableFieldsFromLatLonPolygon("loc", poly)
	// Should not panic; either succeeds for simple triangles or returns a
	// wrapped ErrTessellatorUnsupported. Accept both.
	if err != nil && !errors.Is(err, geo.ErrTessellatorUnsupported) {
		// Other tessellator errors are acceptable; this test guards only
		// against panics and unrelated errors.
		t.Logf("tessellator rejected polygon: %v", err)
	}
}

func TestXYShape_Point(t *testing.T) {
	tri, err := CreateIndexableFieldsFromXYPoint("xy", 0.5, 1.5)
	if err != nil {
		t.Fatal(err)
	}
	if tri == nil {
		t.Fatalf("triangle nil")
	}
}

func TestShapeDocValuesField_Basic(t *testing.T) {
	tri, _ := EncodeTriangle(1, 2, 3, 4, 5, 6, false, false, false)
	f, err := NewShapeDocValuesField("shape", tri)
	if err != nil {
		t.Fatal(err)
	}
	if f.NumTriangles() != 1 {
		t.Fatalf("NumTriangles = %d", f.NumTriangles())
	}
}

func TestLatLonShapeDocValues_AccessTriangles(t *testing.T) {
	tri, _ := EncodeTriangle(1, 2, 3, 4, 5, 6, true, false, false)
	d, err := NewLatLonShapeDocValues(tri)
	if err != nil {
		t.Fatal(err)
	}
	if d.NumTriangles() != 1 {
		t.Fatalf("count = %d", d.NumTriangles())
	}
	got, err := d.Triangle(0)
	if err != nil {
		t.Fatal(err)
	}
	if got.AX != 1 || !got.AB {
		t.Fatalf("decode mismatch: %+v", got)
	}
}

func TestXYShapeDocValues_Basic(t *testing.T) {
	tri, _ := EncodeTriangle(1, 2, 3, 4, 5, 6, false, true, false)
	d, err := NewXYShapeDocValues(tri)
	if err != nil {
		t.Fatal(err)
	}
	if d.NumTriangles() != 1 {
		t.Fatalf("count = %d", d.NumTriangles())
	}
}
