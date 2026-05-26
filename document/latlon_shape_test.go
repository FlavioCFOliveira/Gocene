// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// triangle helper for tests.
func newTriPolygon(t *testing.T) geo.Polygon {
	t.Helper()
	// Simple closed triangle.
	return geo.MustNewPolygon(
		[]float64{0, 0, 1, 0},
		[]float64{0, 1, 0, 0},
	)
}

func TestLatLonShape_CreateIndexableFieldsPolygonChecked(t *testing.T) {
	poly := newTriPolygon(t)
	fields, err := CreateIndexableFieldsFromLatLonPolygonChecked("loc", poly, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) == 0 {
		t.Fatalf("no triangles produced")
	}
	for i, f := range fields {
		if f == nil {
			t.Fatalf("nil field at index %d", i)
		}
		if len(f.BinaryValue()) != ShapeFieldBytes {
			t.Fatalf("field %d wrong length: %d", i, len(f.BinaryValue()))
		}
	}
}

func TestLatLonShape_CreateIndexableFieldsPolygonWithHoles(t *testing.T) {
	// Polygon with a hole — the full tessellator (Sprint 116) handles these
	// via hole-elimination and earcut decomposition.
	// Outer ring: 10°×10° box; inner hole: 2°×2° box near the origin.
	// Expect at least 2 triangles and no error.
	t.Parallel()
	poly := geo.MustNewPolygon(
		[]float64{0, 0, 10, 10, 0},
		[]float64{0, 10, 10, 0, 0},
		geo.MustNewPolygon(
			[]float64{1, 1, 2, 2, 1},
			[]float64{1, 2, 2, 1, 1},
		),
	)
	fields, err := CreateIndexableFieldsFromLatLonPolygonChecked("loc", poly, false)
	if err != nil {
		t.Fatalf("unexpected error for polygon with hole: %v", err)
	}
	if len(fields) < 2 {
		t.Fatalf("expected at least 2 triangles for polygon with hole, got %d", len(fields))
	}
	// Each field must carry the full 28-byte ShapeField payload.
	for i, f := range fields {
		if len(f.BinaryValue()) != ShapeFieldBytes {
			t.Errorf("field[%d] payload length = %d; want %d", i, len(f.BinaryValue()), ShapeFieldBytes)
		}
	}
}

func TestLatLonShape_CreateIndexableFieldsPointArray(t *testing.T) {
	fields, err := CreateIndexableFieldsFromLatLonPointArray("loc", 38.7, -9.1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("want 1 field, got %d", len(fields))
	}
	if len(fields[0].BinaryValue()) != ShapeFieldBytes {
		t.Fatalf("wrong length: %d", len(fields[0].BinaryValue()))
	}
}

func TestLatLonShape_CreateIndexableFieldsPointRejectsOutOfRange(t *testing.T) {
	if _, err := CreateIndexableFieldsFromLatLonPointArray("loc", 91, 0); err == nil {
		t.Fatalf("expected error for lat=91")
	}
	if _, err := CreateIndexableFieldsFromLatLonPointArray("loc", 0, 181); err == nil {
		t.Fatalf("expected error for lon=181")
	}
}

func TestLatLonShape_CreateDocValueFieldPolygon(t *testing.T) {
	poly := newTriPolygon(t)
	dv, err := CreateDocValueFieldFromLatLonPolygon("loc", poly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dv.Shape().NumTriangles() == 0 {
		t.Fatalf("no triangles in DV")
	}
}

func TestLatLonShape_CreateDocValueFieldPolygonChecked(t *testing.T) {
	poly := newTriPolygon(t)
	dv, err := CreateDocValueFieldFromLatLonPolygonChecked("loc", poly, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dv.Shape().NumTriangles() == 0 {
		t.Fatalf("no triangles in DV")
	}
}

func TestLatLonShape_CreateDocValueFieldLine(t *testing.T) {
	line := geo.MustNewLine(
		[]float64{0, 1, 2},
		[]float64{0, 1, 2},
	)
	dv, err := CreateDocValueFieldFromLatLonLine("loc", line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3 vertices → 2 segments.
	if got, want := dv.Shape().NumTriangles(), 2; got != want {
		t.Fatalf("NumTriangles = %d, want %d", got, want)
	}
}

func TestLatLonShape_CreateDocValueFieldPoint(t *testing.T) {
	dv, err := CreateDocValueFieldFromLatLonPoint("loc", 38.7, -9.1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := dv.Shape().NumTriangles(), 1; got != want {
		t.Fatalf("NumTriangles = %d, want %d", got, want)
	}
	tri, err := dv.Shape().Triangle(0)
	if err != nil {
		t.Fatal(err)
	}
	wantY := geo.EncodeLatitude(38.7)
	wantX := geo.EncodeLongitude(-9.1)
	if tri.AY != wantY || tri.AX != wantX {
		t.Fatalf("AY/AX = %d/%d, want %d/%d", tri.AY, tri.AX, wantY, wantX)
	}
}

func TestLatLonShape_CreateDocValueFieldFromBytes(t *testing.T) {
	// Build a triangle stream by hand.
	tri, err := EncodeTriangle(10, 20, 30, 40, 50, 60, true, false, true)
	if err != nil {
		t.Fatal(err)
	}
	dv, err := CreateDocValueFieldFromBytes("loc", tri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dv.Shape().NumTriangles() != 1 {
		t.Fatalf("NumTriangles = %d", dv.Shape().NumTriangles())
	}
	got, err := dv.Shape().Triangle(0)
	if err != nil {
		t.Fatal(err)
	}
	// EncodeTriangle(ax, ay, ...) stores (ay, ax) and DecodeTriangle reads
	// AY then AX back; with input (ax=10, ay=20) we round-trip AX=10, AY=20.
	if got.AX != 10 || got.AY != 20 || !got.AB || got.BC || !got.CA {
		t.Fatalf("decode mismatch: %+v", got)
	}
	// Bytes payload returned by the field must equal the input.
	if !bytes.Equal(dv.BinaryValue(), tri) {
		t.Fatalf("binary payload mismatch")
	}
}

func TestLatLonShape_CreateDocValueFieldFromBytesRejectsMisaligned(t *testing.T) {
	bad := make([]byte, ShapeFieldBytes-1)
	if _, err := CreateDocValueFieldFromBytes("loc", bad); err == nil {
		t.Fatalf("expected misalignment error")
	}
}

func TestLatLonShape_CreateDocValueFieldFromTriangles(t *testing.T) {
	tris := []DecodedTriangle{
		{AX: 1, AY: 2, BX: 3, BY: 4, CX: 5, CY: 6, AB: true},
		{AX: 7, AY: 8, BX: 9, BY: 10, CX: 11, CY: 12, BC: true},
	}
	dv, err := CreateDocValueFieldFromTriangles("loc", tris)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dv.Shape().NumTriangles() != 2 {
		t.Fatalf("NumTriangles = %d, want 2", dv.Shape().NumTriangles())
	}
	// Round-trip AX/AY and edge flags (B/C vertex round-trip is deferred).
	for i, want := range tris {
		got, err := dv.Shape().Triangle(i)
		if err != nil {
			t.Fatal(err)
		}
		if got.AX != want.AX || got.AY != want.AY {
			t.Fatalf("triangle %d AX/AY = %d/%d, want %d/%d", i, got.AX, got.AY, want.AX, want.AY)
		}
		if got.AB != want.AB || got.BC != want.BC || got.CA != want.CA {
			t.Fatalf("triangle %d edges = %v/%v/%v, want %v/%v/%v",
				i, got.AB, got.BC, got.CA, want.AB, want.BC, want.CA)
		}
	}
}

func TestLatLonShape_CreateDocValueFieldFromFields(t *testing.T) {
	t1, err := NewShapeFieldTriangle("loc", 1, 2, 3, 4, 5, 6, true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := NewShapeFieldTriangle("loc", 7, 8, 9, 10, 11, 12, false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	dv, err := CreateDocValueFieldFromFields("loc", []*ShapeFieldTriangle{t1, t2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dv.Shape().NumTriangles() != 2 {
		t.Fatalf("NumTriangles = %d, want 2", dv.Shape().NumTriangles())
	}
	// The aggregated payload must match the per-field BinaryValue concat.
	want := append(append([]byte(nil), t1.BinaryValue()...), t2.BinaryValue()...)
	if !bytes.Equal(dv.BinaryValue(), want) {
		t.Fatalf("aggregated payload mismatch")
	}
}

func TestLatLonShape_CreateDocValueFieldFromFieldsRejectsNil(t *testing.T) {
	t1, err := NewShapeFieldTriangle("loc", 1, 2, 3, 4, 5, 6, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateDocValueFieldFromFields("loc", []*ShapeFieldTriangle{t1, nil}); err == nil {
		t.Fatalf("expected error for nil indexable field")
	}
}

func TestLatLonShape_CreateLatLonShapeDocValues(t *testing.T) {
	tri, err := EncodeTriangle(1, 2, 3, 4, 5, 6, true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	dv, err := CreateLatLonShapeDocValues(tri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dv.NumTriangles() != 1 {
		t.Fatalf("NumTriangles = %d", dv.NumTriangles())
	}
}
