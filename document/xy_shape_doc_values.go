// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// XYShapeDocValues is a doc-values backed accessor over a tessellated XY
// shape. Mirrors Lucene 10.4.0's XYShapeDocValues.
//
// Full byte-for-byte serialisation deferred — see backlog #2697.
type XYShapeDocValues struct {
	triangles []byte
}

// NewXYShapeDocValues wraps an already-tessellated triangle byte stream.
func NewXYShapeDocValues(triangles []byte) (*XYShapeDocValues, error) {
	if len(triangles)%ShapeFieldBytes != 0 {
		return nil, fmt.Errorf("triangle stream length %d not a multiple of %d", len(triangles), ShapeFieldBytes)
	}
	dup := make([]byte, len(triangles))
	copy(dup, triangles)
	return &XYShapeDocValues{triangles: dup}, nil
}

// NumTriangles returns the count of triangles stored.
func (d *XYShapeDocValues) NumTriangles() int { return len(d.triangles) / ShapeFieldBytes }

// Triangle returns the decoded triangle at the given index.
func (d *XYShapeDocValues) Triangle(i int) (DecodedTriangle, error) {
	if i < 0 || i >= d.NumTriangles() {
		return DecodedTriangle{}, fmt.Errorf("triangle index %d out of range [0, %d)", i, d.NumTriangles())
	}
	return DecodeTriangle(d.triangles[i*ShapeFieldBytes : (i+1)*ShapeFieldBytes])
}

// Bytes returns a defensive copy of the payload.
func (d *XYShapeDocValues) Bytes() []byte {
	out := make([]byte, len(d.triangles))
	copy(out, d.triangles)
	return out
}

// XYShapeDocValuesField stores an XYShape as binary doc-values.
type XYShapeDocValuesField struct {
	*BinaryDocValuesField
	shape *XYShapeDocValues
}

// NewXYShapeDocValuesField creates a new XYShapeDocValuesField from an XY
// polygon. The polygon is tessellated via geo.TessellateXY.
func NewXYShapeDocValuesField(name string, xs, ys []float64) (*XYShapeDocValuesField, error) {
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("xs/ys length mismatch")
	}
	if len(xs) < 3 {
		return nil, fmt.Errorf("XY polygon requires at least three vertices")
	}
	triangles, err := geo.TessellateXY(xs, ys, 0, false)
	if err != nil {
		return nil, fmt.Errorf("tessellate xy polygon: %w", err)
	}
	payload := make([]byte, 0, len(triangles)*ShapeFieldBytes)
	for _, tri := range triangles {
		ax := geo.XYEncode(float32(tri.AX()))
		ay := geo.XYEncode(float32(tri.AY()))
		bx := geo.XYEncode(float32(tri.BX()))
		by := geo.XYEncode(float32(tri.BY()))
		cx := geo.XYEncode(float32(tri.CX()))
		cy := geo.XYEncode(float32(tri.CY()))
		buf, err := EncodeTriangle(ax, ay, bx, by, cx, cy,
			tri.EdgeFromPolygon(0), tri.EdgeFromPolygon(1), tri.EdgeFromPolygon(2))
		if err != nil {
			return nil, err
		}
		payload = append(payload, buf...)
	}
	dv, err := NewXYShapeDocValues(payload)
	if err != nil {
		return nil, err
	}
	b, err := NewBinaryDocValuesField(name, payload)
	if err != nil {
		return nil, err
	}
	return &XYShapeDocValuesField{BinaryDocValuesField: b, shape: dv}, nil
}

// Shape returns the wrapped XYShapeDocValues accessor.
func (f *XYShapeDocValuesField) Shape() *XYShapeDocValues { return f.shape }
