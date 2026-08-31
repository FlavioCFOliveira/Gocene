// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// CreateIndexableFieldsPolygon creates indexable fields for polygon geometry.
// Mirrors Lucene 10.4.0's LatLonShape.createIndexableFields(String, Polygon).
func CreateIndexableFieldsPolygon(fieldName string, polygon geo.Polygon) ([]*Field, error) {
	return CreateIndexableFieldsPolygonChecked(fieldName, polygon, false)
}

// CreateIndexableFieldsPolygonChecked creates indexable fields for polygon geometry.
// If checkSelfIntersections is set to true, the validity of the provided polygon
// is checked with a small performance penalty.
// Mirrors Lucene 10.4.0's LatLonShape.createIndexableFields(String, Polygon, boolean).
func CreateIndexableFieldsPolygonChecked(fieldName string, polygon geo.Polygon, checkSelfIntersections bool) ([]*Field, error) {
	tessellation, err := geo.Tessellate(polygon, checkSelfIntersections)
	if err != nil {
		return nil, fmt.Errorf("tessellate polygon: %w", err)
	}
	fields := make([]*Field, len(tessellation))
	for i, t := range tessellation {
		triangleField, err := NewShapeFieldTriangle(
			fieldName,
			geo.EncodeLongitude(t.AX()),
			geo.EncodeLatitude(t.AY()),
			geo.EncodeLongitude(t.BX()),
			geo.EncodeLatitude(t.BY()),
			geo.EncodeLongitude(t.CX()),
			geo.EncodeLatitude(t.CY()),
			t.EdgeFromPolygon(0),
			t.EdgeFromPolygon(1),
			t.EdgeFromPolygon(2),
		)
		if err != nil {
			return nil, err
		}
		fields[i] = triangleField.Field
	}
	return fields, nil
}

// CreateIndexableFieldsLine creates indexable fields for line geometry.
// Mirrors Lucene 10.4.0's LatLonShape.createIndexableFields(String, Line).
func CreateIndexableFieldsLine(fieldName string, line geo.Line) ([]*Field, error) {
	numPoints := line.NumPoints()
	if numPoints < 2 {
		return nil, fmt.Errorf("line requires at least two vertices; got %d", numPoints)
	}
	fields := make([]*Field, numPoints-1)
	for i := 0; i < numPoints-1; i++ {
		triangleField, err := NewShapeFieldTriangle(
			fieldName,
			geo.EncodeLongitude(line.Lon(i)),
			geo.EncodeLatitude(line.Lat(i)),
			geo.EncodeLongitude(line.Lon(i+1)),
			geo.EncodeLatitude(line.Lat(i+1)),
			geo.EncodeLongitude(line.Lon(i)),
			geo.EncodeLatitude(line.Lat(i)),
			true, true, true,
		)
		if err != nil {
			return nil, err
		}
		fields[i] = triangleField.Field
	}
	return fields, nil
}

// CreateIndexableFieldsPoint creates indexable fields for a lat, lon geo point.
// Mirrors Lucene 10.4.0's LatLonShape.createIndexableFields(String, double, double).
func CreateIndexableFieldsPoint(fieldName string, lat, lon float64) ([]*Field, error) {
	triangleField, err := NewShapeFieldTriangle(
		fieldName,
		geo.EncodeLongitude(lon),
		geo.EncodeLatitude(lat),
		geo.EncodeLongitude(lon),
		geo.EncodeLatitude(lat),
		geo.EncodeLongitude(lon),
		geo.EncodeLatitude(lat),
		true, true, true,
	)
	if err != nil {
		return nil, err
	}
	return []*Field{triangleField.Field}, nil
}

// CreateDocValueFieldPolygon creates a doc value field for lat lon polygon geometry.
// Mirrors Lucene 10.4.0's LatLonShape.createDocValueField(String, Polygon).
func CreateDocValueFieldPolygon(fieldName string, polygon geo.Polygon) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesField(fieldName, polygon)
}

// CreateDocValueFieldPolygonChecked creates a doc value field for lat lon polygon geometry.
// Mirrors Lucene 10.4.0's LatLonShape.createDocValueField(String, Polygon, boolean).
func CreateDocValueFieldPolygonChecked(fieldName string, polygon geo.Polygon, checkSelfIntersections bool) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldPolygonChecked(fieldName, polygon, checkSelfIntersections)
}

// CreateDocValueFieldLine creates a doc value field for lat lon line geometry.
// Mirrors Lucene 10.4.0's LatLonShape.createDocValueField(String, Line).
func CreateDocValueFieldLine(fieldName string, line geo.Line) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldLine(fieldName, line)
}

// CreateDocValueFieldPoint creates a doc value field for lat lon geo point.
// Mirrors Lucene 10.4.0's LatLonShape.createDocValueField(String, double, double).
func CreateDocValueFieldPoint(fieldName string, lat, lon float64) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldPoint(fieldName, lat, lon)
}

// CreateDocValueFieldFromBytes creates a LatLonShapeDocValuesField from existing encoding.
// Mirrors Lucene 10.4.0's LatLonShape.createDocValueField(String, BytesRef).
func CreateDocValueFieldFromBytes(fieldName string, binaryValue []byte) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldFromBytes(fieldName, binaryValue)
}

// CreateDocValueFieldFromTriangles creates a LatLonShapeDocValuesField from a tessellation.
// Mirrors Lucene 10.4.0's LatLonShape.createDocValueField(String, List<DecodedTriangle>).
func CreateDocValueFieldFromTriangles(fieldName string, triangles []DecodedTriangle) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldFromTriangles(fieldName, triangles)
}

// CreateDocValueFieldFromFields creates a shape docvalue field from indexable fields.
// Mirrors Lucene 10.4.0's LatLonShape.createDocValueField(String, Field[]).
func CreateDocValueFieldFromFields(fieldName string, indexableFields []*ShapeFieldTriangle) (*LatLonShapeDocValuesField, error) {
	return NewLatLonShapeDocValuesFieldFromFields(fieldName, indexableFields)
}

// CreateLatLonShapeDocValues creates the LatLonShapeDocValues from a binary payload.
// Mirrors Lucene 10.4.0's LatLonShape.createLatLonShapeDocValues(BytesRef).
func CreateLatLonShapeDocValues(binaryValue []byte) (*LatLonShapeDocValues, error) {
	return NewLatLonShapeDocValues(binaryValue)
}
