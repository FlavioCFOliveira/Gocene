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

// xyShapeEncoder is the production ShapeDocValuesEncoder used by the
// cartesian XYShape family. Mirrors Lucene 10.4.0's XYShapeDocValues
// Encoder: X and Y are raw float32 cartesian coordinates and the
// int32 ⇆ float64 mapping routes through geo.XYEncode / geo.XYDecode.
//
// The Java reference exposes this as the package-private Encoder
// nested in XYShapeDocValues. In Gocene the Encoder must be
// importable from the search package so the doc-values query family
// (XYShapeDocValuesQuery, GOC-3225) can build a *ShapeDocValues from a
// per-doc binary payload — Gocene splits the Java
// "XYShapeDocValues extends ShapeDocValues" inheritance into two
// unrelated types, so the encoder strategy must be reachable from
// outside the document package.
//
// The ShapeDocValuesEncoder interface uses float64 to remain symmetric
// with the lat/lon encoder; the XY conversion narrows to float32 at
// the geo.XYEncode boundary, which is the same precision Lucene's
// XYEncodingUtils enforces.
type xyShapeEncoder struct{}

// XYShapeDocValuesEncoder is the singleton instance of the production
// XY ShapeDocValuesEncoder. Stateless and concurrency safe; callers
// should reuse the singleton.
var XYShapeDocValuesEncoder ShapeDocValuesEncoder = xyShapeEncoder{}

// EncodeX maps a cartesian X coordinate to the int32 quantised value.
func (xyShapeEncoder) EncodeX(x float64) int32 { return geo.XYEncode(float32(x)) }

// EncodeY maps a cartesian Y coordinate to the int32 quantised value.
func (xyShapeEncoder) EncodeY(y float64) int32 { return geo.XYEncode(float32(y)) }

// DecodeX maps a quantised int32 back to a cartesian X coordinate.
func (xyShapeEncoder) DecodeX(x int32) float64 { return float64(geo.XYDecode(x)) }

// DecodeY maps a quantised int32 back to a cartesian Y coordinate.
func (xyShapeEncoder) DecodeY(y int32) float64 { return float64(geo.XYDecode(y)) }

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
// polygon. The polygon is tessellated via geo.TessellateXY (no
// self-intersection checks).
//
// Mirrors Java XYShape#createDocValueField(String, XYPolygon).
func NewXYShapeDocValuesField(name string, xs, ys []float64) (*XYShapeDocValuesField, error) {
	return NewXYShapeDocValuesFieldChecked(name, xs, ys, false)
}

// NewXYShapeDocValuesFieldChecked creates a new XYShapeDocValuesField
// from an XY polygon, honouring the checkSelfIntersections flag
// forwarded to the tessellator.
//
// Mirrors Java XYShape#createDocValueField(String, XYPolygon, boolean).
func NewXYShapeDocValuesFieldChecked(name string, xs, ys []float64, checkSelfIntersections bool) (*XYShapeDocValuesField, error) {
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("xs/ys length mismatch: %d vs %d", len(xs), len(ys))
	}
	if len(xs) < 3 {
		return nil, fmt.Errorf("XY polygon requires at least three vertices")
	}
	triangles, err := geo.TessellateXY(xs, ys, 0, checkSelfIntersections)
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
	return assembleXYShapeDocValuesField(name, payload)
}

// NewXYShapeDocValuesFieldLine creates an XYShapeDocValuesField over the
// segments of the supplied XY line. Each segment is encoded as a
// degenerate "line" triangle (C coincident with A).
//
// Mirrors Java XYShape#createDocValueField(String, XYLine).
func NewXYShapeDocValuesFieldLine(name string, xs, ys []float32) (*XYShapeDocValuesField, error) {
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("xs/ys length mismatch: %d vs %d", len(xs), len(ys))
	}
	if len(xs) < 2 {
		return nil, fmt.Errorf("XY line requires at least two vertices; got %d", len(xs))
	}
	payload := make([]byte, 0, (len(xs)-1)*ShapeFieldBytes)
	for i := 0; i+1 < len(xs); i++ {
		ax := geo.XYEncode(xs[i])
		ay := geo.XYEncode(ys[i])
		bx := geo.XYEncode(xs[i+1])
		by := geo.XYEncode(ys[i+1])
		// Third vertex coincides with the first (degenerate "line" triangle).
		buf, err := EncodeTriangle(ax, ay, bx, by, ax, ay, true, true, true)
		if err != nil {
			return nil, err
		}
		payload = append(payload, buf...)
	}
	return assembleXYShapeDocValuesField(name, payload)
}

// NewXYShapeDocValuesFieldPoint creates an XYShapeDocValuesField holding
// a single (x, y) point as a degenerate triangle where all three
// vertices coincide.
//
// Mirrors Java XYShape#createDocValueField(String, float, float).
func NewXYShapeDocValuesFieldPoint(name string, x, y float32) (*XYShapeDocValuesField, error) {
	if _, err := geo.XYCheckVal(x); err != nil {
		return nil, fmt.Errorf("invalid x: %w", err)
	}
	if _, err := geo.XYCheckVal(y); err != nil {
		return nil, fmt.Errorf("invalid y: %w", err)
	}
	xi := geo.XYEncode(x)
	yi := geo.XYEncode(y)
	buf, err := EncodeTriangle(xi, yi, xi, yi, xi, yi, true, true, true)
	if err != nil {
		return nil, err
	}
	return assembleXYShapeDocValuesField(name, buf)
}

// NewXYShapeDocValuesFieldFromBytes wraps an already-encoded triangle
// byte payload as an XYShapeDocValuesField.
//
// Mirrors Java XYShape#createDocValueField(String, BytesRef). The
// caller retains ownership of binaryValue; the underlying constructor
// copies before storing.
func NewXYShapeDocValuesFieldFromBytes(name string, binaryValue []byte) (*XYShapeDocValuesField, error) {
	if len(binaryValue)%ShapeFieldBytes != 0 {
		return nil, fmt.Errorf("triangle stream length %d not a multiple of %d", len(binaryValue), ShapeFieldBytes)
	}
	return assembleXYShapeDocValuesField(name, binaryValue)
}

// NewXYShapeDocValuesFieldFromTriangles encodes the supplied slice of
// DecodedTriangle records into an XYShapeDocValuesField.
//
// Mirrors Java XYShape#createDocValueField(String, List<DecodedTriangle>).
//
// Note: the Gocene EncodeTriangle layout does not round-trip
// BX/BY/CX/CY today (full Lucene rotation is deferred — backlog #2697).
// For now the supplied B/C vertices are encoded but cannot be recovered
// intact by DecodeTriangle. Edge flags and AX/AY round-trip cleanly.
func NewXYShapeDocValuesFieldFromTriangles(name string, triangles []DecodedTriangle) (*XYShapeDocValuesField, error) {
	payload := make([]byte, 0, len(triangles)*ShapeFieldBytes)
	for _, t := range triangles {
		buf, err := EncodeTriangle(t.AX, t.AY, t.BX, t.BY, t.CX, t.CY, t.AB, t.BC, t.CA)
		if err != nil {
			return nil, err
		}
		payload = append(payload, buf...)
	}
	return assembleXYShapeDocValuesField(name, payload)
}

// NewXYShapeDocValuesFieldFromFields aggregates the encoded payloads of
// a slice of ShapeFieldTriangle indexable fields into a single
// XYShapeDocValuesField.
//
// Mirrors Java XYShape#createDocValueField(String, Field[]).
func NewXYShapeDocValuesFieldFromFields(name string, indexableFields []*ShapeFieldTriangle) (*XYShapeDocValuesField, error) {
	payload := make([]byte, 0, len(indexableFields)*ShapeFieldBytes)
	for i, f := range indexableFields {
		if f == nil {
			return nil, fmt.Errorf("nil indexable field at index %d", i)
		}
		bv := f.BinaryValue()
		if len(bv) != ShapeFieldBytes {
			return nil, fmt.Errorf("indexable field %d binary length %d != %d", i, len(bv), ShapeFieldBytes)
		}
		payload = append(payload, bv...)
	}
	return assembleXYShapeDocValuesField(name, payload)
}

// assembleXYShapeDocValuesField builds an XYShapeDocValuesField from a
// fully encoded triangle byte stream. The payload is the source of
// truth; the wrapped XYShapeDocValues is built from a copy held inside
// the field so reader and writer paths share no mutable buffer.
func assembleXYShapeDocValuesField(name string, payload []byte) (*XYShapeDocValuesField, error) {
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
