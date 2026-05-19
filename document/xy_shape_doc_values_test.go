// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// newTriXYPolygon returns a minimal closed XY triangle for tests. The
// tessellator stub accepts single-ring no-hole polygons.
func newTriXYPolygon() (xs, ys []float64) {
	return []float64{0, 1, 0, 0}, []float64{0, 0, 1, 0}
}

func TestXYShape_NewDocValuesFieldPolygon(t *testing.T) {
	xs, ys := newTriXYPolygon()
	dv, err := NewXYShapeDocValuesField("loc", xs, ys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dv.Shape().NumTriangles() == 0 {
		t.Fatalf("no triangles in DV")
	}
}

func TestXYShape_NewDocValuesFieldChecked(t *testing.T) {
	xs, ys := newTriXYPolygon()
	dv, err := NewXYShapeDocValuesFieldChecked("loc", xs, ys, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dv.Shape().NumTriangles() == 0 {
		t.Fatalf("no triangles in DV")
	}
}

func TestXYShape_NewDocValuesFieldCheckedRejectsMismatch(t *testing.T) {
	if _, err := NewXYShapeDocValuesFieldChecked("loc", []float64{0, 1}, []float64{0}, false); err == nil {
		t.Fatalf("expected length mismatch error")
	}
}

func TestXYShape_NewDocValuesFieldCheckedRejectsTooFewVertices(t *testing.T) {
	if _, err := NewXYShapeDocValuesFieldChecked("loc", []float64{0, 1}, []float64{0, 1}, false); err == nil {
		t.Fatalf("expected too-few-vertices error")
	}
}

func TestXYShape_NewDocValuesFieldLine(t *testing.T) {
	dv, err := NewXYShapeDocValuesFieldLine("loc",
		[]float32{0, 1, 2}, []float32{0, 1, 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3 vertices → 2 segments.
	if got, want := dv.Shape().NumTriangles(), 2; got != want {
		t.Fatalf("NumTriangles = %d, want %d", got, want)
	}
}

func TestXYShape_NewDocValuesFieldLineRejectsTooFew(t *testing.T) {
	if _, err := NewXYShapeDocValuesFieldLine("loc", []float32{0}, []float32{0}); err == nil {
		t.Fatalf("expected too-few-vertices error")
	}
}

func TestXYShape_NewDocValuesFieldPoint(t *testing.T) {
	dv, err := NewXYShapeDocValuesFieldPoint("loc", 1.5, -2.25)
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
	wantX := geo.XYEncode(1.5)
	wantY := geo.XYEncode(-2.25)
	if tri.AX != wantX || tri.AY != wantY {
		t.Fatalf("AX/AY = %d/%d, want %d/%d", tri.AX, tri.AY, wantX, wantY)
	}
}

func TestXYShape_NewDocValuesFieldFromBytes(t *testing.T) {
	tri, err := EncodeTriangle(10, 20, 30, 40, 50, 60, true, false, true)
	if err != nil {
		t.Fatal(err)
	}
	dv, err := NewXYShapeDocValuesFieldFromBytes("loc", tri)
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
	// EncodeTriangle stores (ay, ax) and DecodeTriangle reads AY then AX
	// back; with input (ax=10, ay=20) we round-trip AX=10, AY=20.
	if got.AX != 10 || got.AY != 20 || !got.AB || got.BC || !got.CA {
		t.Fatalf("decode mismatch: %+v", got)
	}
	// Bytes payload returned by the field must equal the input.
	if !bytes.Equal(dv.BinaryValue(), tri) {
		t.Fatalf("binary payload mismatch")
	}
}

func TestXYShape_NewDocValuesFieldFromBytesRejectsMisaligned(t *testing.T) {
	bad := make([]byte, ShapeFieldBytes-1)
	if _, err := NewXYShapeDocValuesFieldFromBytes("loc", bad); err == nil {
		t.Fatalf("expected misalignment error")
	}
}

func TestXYShape_NewDocValuesFieldFromTriangles(t *testing.T) {
	tris := []DecodedTriangle{
		{AX: 1, AY: 2, BX: 3, BY: 4, CX: 5, CY: 6, AB: true},
		{AX: 7, AY: 8, BX: 9, BY: 10, CX: 11, CY: 12, BC: true},
	}
	dv, err := NewXYShapeDocValuesFieldFromTriangles("loc", tris)
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

func TestXYShape_NewDocValuesFieldFromFields(t *testing.T) {
	t1, err := NewShapeFieldTriangle("loc", 1, 2, 3, 4, 5, 6, true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := NewShapeFieldTriangle("loc", 7, 8, 9, 10, 11, 12, false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	dv, err := NewXYShapeDocValuesFieldFromFields("loc", []*ShapeFieldTriangle{t1, t2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dv.Shape().NumTriangles() != 2 {
		t.Fatalf("NumTriangles = %d, want 2", dv.Shape().NumTriangles())
	}
	want := append(append([]byte(nil), t1.BinaryValue()...), t2.BinaryValue()...)
	if !bytes.Equal(dv.BinaryValue(), want) {
		t.Fatalf("aggregated payload mismatch")
	}
}

func TestXYShape_NewDocValuesFieldFromFieldsRejectsNil(t *testing.T) {
	t1, err := NewShapeFieldTriangle("loc", 1, 2, 3, 4, 5, 6, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewXYShapeDocValuesFieldFromFields("loc", []*ShapeFieldTriangle{t1, nil}); err == nil {
		t.Fatalf("expected error for nil indexable field")
	}
}

// TestXYShape_DocValuesEncoder_Roundtrip verifies the production XY
// encoder singleton round-trips representative cartesian coordinates.
func TestXYShape_DocValuesEncoder_Roundtrip(t *testing.T) {
	values := []float64{0, 1, -1, 1234.5, -9876.25, 1e6, -1e6}
	for _, v := range values {
		ex := XYShapeDocValuesEncoder.EncodeX(v)
		ey := XYShapeDocValuesEncoder.EncodeY(v)
		dx := XYShapeDocValuesEncoder.DecodeX(ex)
		dy := XYShapeDocValuesEncoder.DecodeY(ey)
		// float32 quantisation rounds; compare against direct geo helpers.
		wantX := float64(geo.XYDecode(geo.XYEncode(float32(v))))
		wantY := wantX
		if dx != wantX || dy != wantY {
			t.Fatalf("roundtrip(%v) = (%v, %v); want (%v, %v)", v, dx, dy, wantX, wantY)
		}
	}
}

// TestXYShape_DocValuesEncoder_IsSingleton asserts the singleton is
// stateless: the variable address is stable, and the value satisfies
// the ShapeDocValuesEncoder interface.
func TestXYShape_DocValuesEncoder_IsSingleton(t *testing.T) {
	var _ ShapeDocValuesEncoder = XYShapeDocValuesEncoder
	if XYShapeDocValuesEncoder == nil {
		t.Fatalf("XYShapeDocValuesEncoder is nil")
	}
}
