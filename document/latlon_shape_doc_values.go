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
	return NewLatLonShapeDocValuesFieldPolygonChecked(name, polygon, false)
}

// NewLatLonShapeDocValuesFieldPolygonChecked creates a new
// LatLonShapeDocValuesField from a Polygon, honouring the
// checkSelfIntersections flag forwarded to the tessellator.
//
// Mirrors Java LatLonShape#createDocValueField(String, Polygon, boolean).
func NewLatLonShapeDocValuesFieldPolygonChecked(name string, polygon geo.Polygon, checkSelfIntersections bool) (*LatLonShapeDocValuesField, error) {
	triangles, err := geo.Tessellate(polygon, checkSelfIntersections)
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
	return assembleLatLonShapeDocValuesField(name, payload)
}

// NewLatLonShapeDocValuesFieldLine creates a LatLonShapeDocValuesField
// over the segments of the supplied Line. Each segment is encoded as a
// degenerate "line" triangle.
//
// Mirrors Java LatLonShape#createDocValueField(String, Line).
func NewLatLonShapeDocValuesFieldLine(name string, line geo.Line) (*LatLonShapeDocValuesField, error) {
	numPoints := line.NumPoints()
	if numPoints < 2 {
		return nil, fmt.Errorf("line requires at least two vertices; got %d", numPoints)
	}
	payload := make([]byte, 0, (numPoints-1)*ShapeFieldBytes)
	for i := 0; i+1 < numPoints; i++ {
		ay := geo.EncodeLatitude(line.Lat(i))
		ax := geo.EncodeLongitude(line.Lon(i))
		by := geo.EncodeLatitude(line.Lat(i + 1))
		bx := geo.EncodeLongitude(line.Lon(i + 1))
		// Third vertex coincides with the first (degenerate "line" triangle).
		buf, err := EncodeTriangle(ax, ay, bx, by, ax, ay, true, true, true)
		if err != nil {
			return nil, err
		}
		payload = append(payload, buf...)
	}
	return assembleLatLonShapeDocValuesField(name, payload)
}

// NewLatLonShapeDocValuesFieldPoint creates a LatLonShapeDocValuesField
// holding a single (lat, lon) point as a degenerate triangle.
//
// Mirrors Java LatLonShape#createDocValueField(String, double, double).
func NewLatLonShapeDocValuesFieldPoint(name string, latitude, longitude float64) (*LatLonShapeDocValuesField, error) {
	if err := validateLatLon(latitude, longitude); err != nil {
		return nil, err
	}
	x := geo.EncodeLongitude(longitude)
	y := geo.EncodeLatitude(latitude)
	buf, err := EncodeTriangle(x, y, x, y, x, y, true, true, true)
	if err != nil {
		return nil, err
	}
	return assembleLatLonShapeDocValuesField(name, buf)
}

// NewLatLonShapeDocValuesFieldFromBytes wraps an already-encoded triangle
// byte payload as a LatLonShapeDocValuesField.
//
// Mirrors Java LatLonShape#createDocValueField(String, BytesRef). The
// caller retains ownership of binaryValue; the underlying constructor
// copies before storing.
func NewLatLonShapeDocValuesFieldFromBytes(name string, binaryValue []byte) (*LatLonShapeDocValuesField, error) {
	if len(binaryValue)%ShapeFieldBytes != 0 {
		return nil, fmt.Errorf("triangle stream length %d not a multiple of %d", len(binaryValue), ShapeFieldBytes)
	}
	return assembleLatLonShapeDocValuesField(name, binaryValue)
}

// NewLatLonShapeDocValuesFieldFromTriangles encodes the supplied slice
// of DecodedTriangle records into a LatLonShapeDocValuesField.
//
// Mirrors Java LatLonShape#createDocValueField(String, List<DecodedTriangle>).
//
// Note: the Gocene EncodeTriangle layout does not round-trip BX/BY/CX/CY
// today (full Lucene rotation is deferred — backlog #2697). For now the
// supplied B/C vertices are encoded but cannot be recovered intact by
// DecodeTriangle. Edge flags and AX/AY round-trip cleanly.
func NewLatLonShapeDocValuesFieldFromTriangles(name string, triangles []DecodedTriangle) (*LatLonShapeDocValuesField, error) {
	payload := make([]byte, 0, len(triangles)*ShapeFieldBytes)
	for _, t := range triangles {
		buf, err := EncodeTriangle(t.AX, t.AY, t.BX, t.BY, t.CX, t.CY, t.AB, t.BC, t.CA)
		if err != nil {
			return nil, err
		}
		payload = append(payload, buf...)
	}
	return assembleLatLonShapeDocValuesField(name, payload)
}

// NewLatLonShapeDocValuesFieldFromFields aggregates the encoded payloads
// of a slice of ShapeFieldTriangle indexable fields into a single
// LatLonShapeDocValuesField.
//
// Mirrors Java LatLonShape#createDocValueField(String, Field[]).
func NewLatLonShapeDocValuesFieldFromFields(name string, indexableFields []*ShapeFieldTriangle) (*LatLonShapeDocValuesField, error) {
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
	return assembleLatLonShapeDocValuesField(name, payload)
}

// assembleLatLonShapeDocValuesField builds a LatLonShapeDocValuesField
// from a fully encoded triangle byte stream. The payload is the source
// of truth; the wrapped LatLonShapeDocValues is built from a copy held
// inside the field so reader and writer paths share no mutable buffer.
func assembleLatLonShapeDocValuesField(name string, payload []byte) (*LatLonShapeDocValuesField, error) {
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
