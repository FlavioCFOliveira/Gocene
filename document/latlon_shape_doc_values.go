// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// LatLonShapeDocValues is a doc-values backed accessor over a tessellated
// geographic shape. Mirrors Lucene 10.4.0's LatLonShapeDocValues.
//
// Lucene's class serialises centroid, bbox, and the triangle list into a
// custom binary format. Sprint 21 ships the structural surface plus
// per-triangle accessors. Full byte-for-byte serialisation is deferred to
// the geo query/scorer sprint (backlog #2697).
type LatLonShapeDocValues struct {
	triangles []byte // packed ShapeFieldBytes per triangle
}

// NewLatLonShapeDocValues wraps an already-tessellated triangle byte
// stream. Each triangle occupies ShapeFieldBytes (28 bytes).
func NewLatLonShapeDocValues(triangles []byte) (*LatLonShapeDocValues, error) {
	if len(triangles)%ShapeFieldBytes != 0 {
		return nil, fmt.Errorf("triangle stream length %d not a multiple of %d", len(triangles), ShapeFieldBytes)
	}
	dup := make([]byte, len(triangles))
	copy(dup, triangles)
	return &LatLonShapeDocValues{triangles: dup}, nil
}

// NumTriangles returns the number of triangles stored.
func (d *LatLonShapeDocValues) NumTriangles() int { return len(d.triangles) / ShapeFieldBytes }

// Triangle returns the decoded triangle at the given index.
func (d *LatLonShapeDocValues) Triangle(i int) (DecodedTriangle, error) {
	if i < 0 || i >= d.NumTriangles() {
		return DecodedTriangle{}, fmt.Errorf("triangle index %d out of range [0, %d)", i, d.NumTriangles())
	}
	return DecodeTriangle(d.triangles[i*ShapeFieldBytes : (i+1)*ShapeFieldBytes])
}

// Bytes returns a defensive copy of the underlying triangle payload.
func (d *LatLonShapeDocValues) Bytes() []byte {
	out := make([]byte, len(d.triangles))
	copy(out, d.triangles)
	return out
}

// LatLonShapeDocValuesField stores a LatLonShape as binary doc-values.
//
// Go port of Lucene 10.4.0's LatLonShapeDocValuesField. Wraps a
// LatLonShapeDocValues payload inside a BinaryDocValuesField.
type LatLonShapeDocValuesField struct {
	*BinaryDocValuesField
	shape *LatLonShapeDocValues
}

// NewLatLonShapeDocValuesField creates a new LatLonShapeDocValuesField
// from a Polygon. The polygon is tessellated; an error is returned if
// the pre-existing tessellator stub cannot handle it.
func NewLatLonShapeDocValuesField(name string, polygon geo.Polygon) (*LatLonShapeDocValuesField, error) {
	triangles, err := geo.Tessellate(polygon, false)
	if err != nil {
		return nil, fmt.Errorf("tessellate polygon: %w", err)
	}
	payload := make([]byte, 0, len(triangles)*ShapeFieldBytes)
	for _, tri := range triangles {
		ay := geo.EncodeLatitude(tri.AY())
		ax := geo.EncodeLongitude(tri.AX())
		by := geo.EncodeLatitude(tri.BY())
		bx := geo.EncodeLongitude(tri.BX())
		cy := geo.EncodeLatitude(tri.CY())
		cx := geo.EncodeLongitude(tri.CX())
		buf, err := EncodeTriangle(ax, ay, bx, by, cx, cy,
			tri.EdgeFromPolygon(0), tri.EdgeFromPolygon(1), tri.EdgeFromPolygon(2))
		if err != nil {
			return nil, err
		}
		payload = append(payload, buf...)
	}
	dv, err := NewLatLonShapeDocValues(payload)
	if err != nil {
		return nil, err
	}
	b, err := NewBinaryDocValuesField(name, payload)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeDocValuesField{BinaryDocValuesField: b, shape: dv}, nil
}

// Shape returns the wrapped LatLonShapeDocValues accessor.
func (f *LatLonShapeDocValuesField) Shape() *LatLonShapeDocValues { return f.shape }
