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
// Sprint 21 ships the static factory surface; full tessellation is
// limited by the pre-existing geo.Tessellator stub (rejects polygons it
// cannot handle with ErrTessellatorUnsupported). Static query factories
// (NewBoxQuery / NewDistanceQuery / NewPolygonQuery / NewGeometryQuery)
// deferred — backlog #2697.

// CreateIndexableFieldsFromLatLonPolygon tessellates a Polygon and returns
// the per-triangle ShapeFieldTriangle fields ready to be added to a
// Document.
//
// Returns an error wrapping geo.ErrTessellatorUnsupported when the
// tessellator stub cannot decompose the supplied polygon.
func CreateIndexableFieldsFromLatLonPolygon(fieldName string, polygon geo.Polygon) ([]*ShapeFieldTriangle, error) {
	triangles, err := geo.Tessellate(polygon, false)
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
func CreateIndexableFieldsFromLatLonPoint(fieldName string, latitude, longitude float64) (*ShapeFieldTriangle, error) {
	if err := validateLatLon(latitude, longitude); err != nil {
		return nil, err
	}
	x := geo.EncodeLongitude(longitude)
	y := geo.EncodeLatitude(latitude)
	return NewShapeFieldTriangle(fieldName, x, y, x, y, x, y, false, false, false)
}
