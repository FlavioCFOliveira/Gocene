// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// LatLonShape provides static factories for indexing geographic shapes
// (polygons, lines, points) as tessellated triangle fields.
//
// Go port of Lucene 10.4.0's org.apache.lucene.document.LatLonShape.
//
// Sprint 21 shipped the initial static factory surface; Sprint 55 extends
// it with the remaining Lucene-parity factories (checkSelfIntersections
// variants, doc-value factories from Line / Point / BytesRef / triangle
// list / indexable Field array, and CreateLatLonShapeDocValues).
//
// Full tessellation remains limited by the pre-existing geo.Tessellator
// stub (rejects polygons with holes or self-intersections with
// ErrTessellatorUnsupported). Static query factories (NewBoxQuery /
// NewDistanceQuery / NewPolygonQuery / NewLineQuery / NewPointQuery /
// NewGeometryQuery / NewSlowDocValuesBoxQuery) cannot live in this
// package because document/ may not import search/ (cycle); they are
// tracked under backlog #2697 and will live under search/ when ported.

// CreateIndexableFieldsFromLatLonPolygon tessellates a Polygon and returns
// the per-triangle ShapeFieldTriangle fields ready to be added to a
// Document.
//
// Equivalent to Java's LatLonShape#createIndexableFields(String, Polygon).
//
// Returns an error wrapping geo.ErrTessellatorUnsupported when the
// tessellator stub cannot decompose the supplied polygon.
func CreateIndexableFieldsFromLatLonPolygon(fieldName string, polygon geo.Polygon) ([]*ShapeFieldTriangle, error) {
	return CreateIndexableFieldsFromLatLonPolygonChecked(fieldName, polygon, false)
}

// CreateIndexableFieldsFromLatLonPolygonChecked tessellates a Polygon
// optionally validating self-intersections, and returns the per-triangle
// ShapeFieldTriangle fields ready to be added to a Document.
//
// Equivalent to Java's LatLonShape#createIndexableFields(String, Polygon, boolean).
//
// The checkSelfIntersections argument is forwarded to geo.Tessellate;
// the current tessellator stub accepts it for API parity but treats it
// as a no-op (callers must supply non-self-intersecting polygons).
func CreateIndexableFieldsFromLatLonPolygonChecked(fieldName string, polygon geo.Polygon, checkSelfIntersections bool) ([]*ShapeFieldTriangle, error) {
	triangles, err := geo.Tessellate(polygon, checkSelfIntersections)
	if err != nil {
		return nil, fmt.Errorf("tessellate latlon polygon: %w", err)
	}
	out := make([]*ShapeFieldTriangle, 0, len(triangles))
	for _, tri := range triangles {
		// Encode lat/lon to int32 using geo encoders before passing on.
		ay := geo.EncodeLatitude(tri.AY())
		ax := geo.EncodeLongitude(tri.AX())
		by := geo.EncodeLatitude(tri.BY())
		bx := geo.EncodeLongitude(tri.BX())
		cy := geo.EncodeLatitude(tri.CY())
		cx := geo.EncodeLongitude(tri.CX())
		field, err := NewShapeFieldTriangle(fieldName, ax, ay, bx, by, cx, cy,
			tri.EdgeFromPolygon(0), tri.EdgeFromPolygon(1), tri.EdgeFromPolygon(2))
		if err != nil {
			return nil, err
		}
		out = append(out, field)
	}
	return out, nil
}

// CreateIndexableFieldsFromLatLonLine encodes a line as a degenerate
// triangle per segment.
func CreateIndexableFieldsFromLatLonLine(fieldName string, line geo.Line) ([]*ShapeFieldTriangle, error) {
	lats := line.Lats()
	lons := line.Lons()
	if len(lats) < 2 {
		return nil, fmt.Errorf("line requires at least two vertices")
	}
	out := make([]*ShapeFieldTriangle, 0, len(lats)-1)
	for i := 0; i+1 < len(lats); i++ {
		ay := geo.EncodeLatitude(lats[i])
		ax := geo.EncodeLongitude(lons[i])
		by := geo.EncodeLatitude(lats[i+1])
		bx := geo.EncodeLongitude(lons[i+1])
		// Third vertex coincides with B → degenerate "line" triangle.
		field, err := NewShapeFieldTriangle(fieldName, ax, ay, bx, by, bx, by, true, false, false)
		if err != nil {
			return nil, err
		}
		out = append(out, field)
	}
	return out, nil
}

// CreateIndexableFieldsFromLatLonPoint encodes a single (lat, lon) point
// as a degenerate triangle where all three vertices coincide.
//
// Returns the single triangle field. For the Lucene-parity surface that
// returns a one-element array (matching createIndexableFields(String,
// double, double) -> Field[]) use CreateIndexableFieldsFromLatLonPointArray.
func CreateIndexableFieldsFromLatLonPoint(fieldName string, latitude, longitude float64) (*ShapeFieldTriangle, error) {
	if err := validateLatLon(latitude, longitude); err != nil {
		return nil, err
	}
	x := geo.EncodeLongitude(longitude)
	y := geo.EncodeLatitude(latitude)
	return NewShapeFieldTriangle(fieldName, x, y, x, y, x, y, false, false, false)
}

// CreateIndexableFieldsFromLatLonPointArray is the Lucene-parity variant
// of CreateIndexableFieldsFromLatLonPoint that returns a single-element
// slice (matching Java's createIndexableFields(String, double, double)
// -> Field[]). Useful for callers that bulk-iterate indexable fields
// without distinguishing point from line/polygon.
func CreateIndexableFieldsFromLatLonPointArray(fieldName string, latitude, longitude float64) ([]*ShapeFieldTriangle, error) {
	tri, err := CreateIndexableFieldsFromLatLonPoint(fieldName, latitude, longitude)
	if err != nil {
		return nil, err
	}
	return []*ShapeFieldTriangle{tri}, nil
}

// CreateDocValueFieldFromLatLonPolygon tessellates the supplied Polygon
// (without self-intersection checks) and returns a doc-values backed
// LatLonShapeDocValuesField, equivalent to Java's
// LatLonShape#createDocValueField(String, Polygon).
func CreateDocValueFieldFromLatLonPolygon(fieldName string, polygon geo.Polygon) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesField(fieldName, polygon)
}

// CreateDocValueFieldFromLatLonPolygonChecked tessellates the supplied
// Polygon honouring checkSelfIntersections, equivalent to Java's
// LatLonShape#createDocValueField(String, Polygon, boolean).
func CreateDocValueFieldFromLatLonPolygonChecked(fieldName string, polygon geo.Polygon, checkSelfIntersections bool) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldPolygonChecked(fieldName, polygon, checkSelfIntersections)
}

// CreateDocValueFieldFromLatLonLine returns a LatLonShapeDocValuesField
// over the segments of the supplied Line, equivalent to Java's
// LatLonShape#createDocValueField(String, Line).
func CreateDocValueFieldFromLatLonLine(fieldName string, line geo.Line) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldLine(fieldName, line)
}

// CreateDocValueFieldFromLatLonPoint returns a LatLonShapeDocValuesField
// holding a single point, equivalent to Java's
// LatLonShape#createDocValueField(String, double, double).
func CreateDocValueFieldFromLatLonPoint(fieldName string, latitude, longitude float64) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldPoint(fieldName, latitude, longitude)
}

// CreateDocValueFieldFromBytes wraps an already-encoded triangle byte
// payload as a LatLonShapeDocValuesField, equivalent to Java's
// LatLonShape#createDocValueField(String, BytesRef).
//
// The caller retains ownership of binaryValue; the constructor copies.
func CreateDocValueFieldFromBytes(fieldName string, binaryValue []byte) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldFromBytes(fieldName, binaryValue)
}

// CreateDocValueFieldFromTriangles encodes a slice of DecodedTriangle
// records as a LatLonShapeDocValuesField, equivalent to Java's
// LatLonShape#createDocValueField(String, List<DecodedTriangle>).
//
// Each DecodedTriangle must carry the AX/AY plus the three edge bits;
// vertex B/C are reconstructed from the rotated layout when
// EncodeTriangle gains the full Lucene rotation (backlog #2697).
func CreateDocValueFieldFromTriangles(fieldName string, triangles []DecodedTriangle) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldFromTriangles(fieldName, triangles)
}

// CreateDocValueFieldFromFields extracts the binary payload of each
// indexable ShapeFieldTriangle and aggregates them into a
// LatLonShapeDocValuesField, equivalent to Java's
// LatLonShape#createDocValueField(String, Field[]).
func CreateDocValueFieldFromFields(fieldName string, indexableFields []*ShapeFieldTriangle) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldFromFields(fieldName, indexableFields)
}

// CreateLatLonShapeDocValues wraps a pre-encoded triangle byte payload
// as a LatLonShapeDocValues accessor, equivalent to Java's
// LatLonShape#createLatLonShapeDocValues(BytesRef).
func CreateLatLonShapeDocValues(binaryValue []byte) (*LatLonShapeDocValues, error) {
	return NewLatLonShapeDocValues(binaryValue)
}
