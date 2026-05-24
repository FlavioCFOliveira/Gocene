// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package spatial3d provides Geo3D point fields, sort fields, and queries
// for three-dimensional sphere geometry.
//
// Port of org.apache.lucene.spatial3d.
//
// Deviation: Field, SortField, FieldComparator, IntersectVisitor, and
// DocIdSetBuilder are all part of the index-layer infrastructure that is not
// yet ported. Concrete implementations of Geo3DPoint, Geo3DDocValuesField,
// the sort-field and comparator types, PointInGeo3DShapeQuery, and
// PointInShapeIntersectVisitor are therefore deferred to backlog #2693.
// This file delivers the public type stubs so callers can compile against
// the package API.
package spatial3d

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
)

// ---------------------------------------------------------------------------
// Geo3DPoint — stub
//
// Port of org.apache.lucene.spatial3d.Geo3DPoint.
// ---------------------------------------------------------------------------

// Geo3DPoint is a point stored as a three-dimensional XYZ coordinate.
//
// Full implementation (Field embedding, encoding, query helpers) deferred to
// backlog #2693.
//
// Port of org.apache.lucene.spatial3d.Geo3DPoint.
type Geo3DPoint struct {
	planetModel *geom.PlanetModel
	X, Y, Z     float64
	fieldName   string
}

// NewGeo3DPointLatLon constructs a Geo3DPoint from latitude and longitude.
//
// Port of org.apache.lucene.spatial3d.Geo3DPoint(String,double,double).
func NewGeo3DPointLatLon(name string, lat, lon float64) *Geo3DPoint {
	pm := geom.SPHERE
	p := geom.NewGeoPointLatLon(pm, lat, lon)
	return &Geo3DPoint{planetModel: pm, X: p.X, Y: p.Y, Z: p.Z, fieldName: name}
}

// NewGeo3DPointXYZ constructs a Geo3DPoint from raw XYZ coordinates.
//
// Port of org.apache.lucene.spatial3d.Geo3DPoint(String,double,double,double).
func NewGeo3DPointXYZ(name string, x, y, z float64) *Geo3DPoint {
	return &Geo3DPoint{planetModel: geom.SPHERE, X: x, Y: y, Z: z, fieldName: name}
}

// NewGeo3DPointXYZModel constructs a Geo3DPoint with an explicit PlanetModel.
//
// Port of org.apache.lucene.spatial3d.Geo3DPoint(String,PlanetModel,double,double,double).
func NewGeo3DPointXYZModel(name string, pm *geom.PlanetModel, x, y, z float64) *Geo3DPoint {
	return &Geo3DPoint{planetModel: pm, X: x, Y: y, Z: z, fieldName: name}
}

// String returns a human-readable representation.
func (p *Geo3DPoint) String() string {
	return fmt.Sprintf("Geo3DPoint<%s:[%g,%g,%g]>", p.fieldName, p.X, p.Y, p.Z)
}

// ---------------------------------------------------------------------------
// Geo3DDocValuesField — stub
//
// Port of org.apache.lucene.spatial3d.Geo3DDocValuesField.
// ---------------------------------------------------------------------------

// Geo3DDocValuesField is a doc-values field storing an XYZ location.
//
// Full implementation deferred to backlog #2693.
//
// Port of org.apache.lucene.spatial3d.Geo3DDocValuesField.
type Geo3DDocValuesField struct {
	planetModel *geom.PlanetModel
	X, Y, Z     float64
	fieldName   string
}

// NewGeo3DDocValuesField constructs a Geo3DDocValuesField from a GeoPoint.
func NewGeo3DDocValuesField(name string, pm *geom.PlanetModel, point *geom.GeoPoint) *Geo3DDocValuesField {
	return &Geo3DDocValuesField{planetModel: pm, X: point.X, Y: point.Y, Z: point.Z, fieldName: name}
}

// SetLocationValue updates the stored location.
func (f *Geo3DDocValuesField) SetLocationValue(x, y, z float64) {
	f.X, f.Y, f.Z = x, y, z
}

// String returns a human-readable representation.
func (f *Geo3DDocValuesField) String() string {
	return fmt.Sprintf("Geo3DDocValuesField<%s:[%g,%g,%g]>", f.fieldName, f.X, f.Y, f.Z)
}

// ---------------------------------------------------------------------------
// Geo3DUtil — package-level utilities
//
// Port of org.apache.lucene.spatial3d.Geo3DUtil (package-private in Java).
// ---------------------------------------------------------------------------

// RadiansPerDegree is the factor to convert degrees to radians.
const RadiansPerDegree = math.Pi / 180.0

// EncodeDimension encodes a double dimension value to a 4-byte integer.
// Full encoding algorithm deferred to #2693.
//
// Port of org.apache.lucene.spatial3d.Geo3DPoint.encodeDimension.
func EncodeDimension(pm *geom.PlanetModel, value float64) int32 {
	// Deferred to #2693 — stub returns clamped scaled int.
	if value > pm.MaxValue {
		value = pm.MaxValue
	} else if value < -pm.MaxValue {
		value = -pm.MaxValue
	}
	return int32(value / pm.Decode)
}

// DecodeDimension decodes a 4-byte encoded dimension value.
// Full decoding algorithm deferred to #2693.
//
// Port of org.apache.lucene.spatial3d.Geo3DPoint.decodeDimension.
func DecodeDimension(pm *geom.PlanetModel, encoded int32) float64 {
	return float64(encoded) * pm.Decode
}

// ---------------------------------------------------------------------------
// PointInGeo3DShapeQuery — stub
//
// Port of org.apache.lucene.spatial3d.PointInGeo3DShapeQuery.
// ---------------------------------------------------------------------------

// PointInGeo3DShapeQuery is a query that matches documents whose Geo3DPoint
// falls inside the given GeoShape.
//
// Full implementation (Weight, Scorer, IntersectVisitor) deferred to #2693.
//
// Port of org.apache.lucene.spatial3d.PointInGeo3DShapeQuery.
type PointInGeo3DShapeQuery struct {
	field string
	shape geom.GeoShape
}

// NewPointInGeo3DShapeQuery constructs a PointInGeo3DShapeQuery.
//
// Port of org.apache.lucene.spatial3d.PointInGeo3DShapeQuery(String,GeoShape).
func NewPointInGeo3DShapeQuery(field string, shape geom.GeoShape) *PointInGeo3DShapeQuery {
	return &PointInGeo3DShapeQuery{field: field, shape: shape}
}

// GetField returns the field name.
func (q *PointInGeo3DShapeQuery) GetField() string { return q.field }

// GetShape returns the query shape.
func (q *PointInGeo3DShapeQuery) GetShape() geom.GeoShape { return q.shape }

// String returns a human-readable representation.
func (q *PointInGeo3DShapeQuery) String() string {
	return fmt.Sprintf("PointInGeo3DShapeQuery{field=%s}", q.field)
}

// ---------------------------------------------------------------------------
// PointInShapeIntersectVisitor — stub
//
// Port of org.apache.lucene.spatial3d.PointInShapeIntersectVisitor.
// ---------------------------------------------------------------------------

// PointInShapeIntersectVisitor visits BKD leaf nodes testing whether each
// encoded point falls within a GeoShape.
//
// Full implementation (DocIdSetBuilder integration) deferred to #2693.
//
// Port of org.apache.lucene.spatial3d.PointInShapeIntersectVisitor.
type PointInShapeIntersectVisitor struct {
	shape  geom.GeoShape
	bounds *geom.XYZBounds
}

// NewPointInShapeIntersectVisitor constructs a PointInShapeIntersectVisitor.
func NewPointInShapeIntersectVisitor(shape geom.GeoShape, bounds *geom.XYZBounds) *PointInShapeIntersectVisitor {
	return &PointInShapeIntersectVisitor{shape: shape, bounds: bounds}
}

// ---------------------------------------------------------------------------
// Geo3DPointSortField / comparators — stubs
//
// Ports of org.apache.lucene.spatial3d.Geo3DPointSortField,
// Geo3DPointOutsideSortField, Geo3DPointDistanceComparator,
// Geo3DPointOutsideDistanceComparator.
//
// All depend on SortField/FieldComparator infrastructure deferred to #2693.
// ---------------------------------------------------------------------------

// Geo3DPointSortField is a SortField that sorts by distance to a Geo3D shape.
//
// Port of org.apache.lucene.spatial3d.Geo3DPointSortField.
type Geo3DPointSortField struct {
	field       string
	shape       geom.GeoShape
	planetModel *geom.PlanetModel
}

// NewGeo3DPointSortField constructs a Geo3DPointSortField.
func NewGeo3DPointSortField(field string, pm *geom.PlanetModel, shape geom.GeoShape) *Geo3DPointSortField {
	return &Geo3DPointSortField{field: field, planetModel: pm, shape: shape}
}

// String returns a human-readable representation.
func (s *Geo3DPointSortField) String() string {
	return fmt.Sprintf("Geo3DPointSortField{field=%s}", s.field)
}

// Geo3DPointOutsideSortField is a SortField that sorts by outside-distance to a Geo3D shape.
//
// Port of org.apache.lucene.spatial3d.Geo3DPointOutsideSortField.
type Geo3DPointOutsideSortField struct {
	field       string
	shape       geom.GeoShape
	planetModel *geom.PlanetModel
}

// NewGeo3DPointOutsideSortField constructs a Geo3DPointOutsideSortField.
func NewGeo3DPointOutsideSortField(field string, pm *geom.PlanetModel, shape geom.GeoShape) *Geo3DPointOutsideSortField {
	return &Geo3DPointOutsideSortField{field: field, planetModel: pm, shape: shape}
}

// String returns a human-readable representation.
func (s *Geo3DPointOutsideSortField) String() string {
	return fmt.Sprintf("Geo3DPointOutsideSortField{field=%s}", s.field)
}

// Geo3DPointDistanceComparator computes in-shape distance for sorting.
//
// Port of org.apache.lucene.spatial3d.Geo3DPointDistanceComparator.
// Deferred to #2693.
type Geo3DPointDistanceComparator struct {
	field       string
	planetModel *geom.PlanetModel
	shape       geom.GeoShape
}

// Geo3DPointOutsideDistanceComparator computes outside-shape distance for sorting.
//
// Port of org.apache.lucene.spatial3d.Geo3DPointOutsideDistanceComparator.
// Deferred to #2693.
type Geo3DPointOutsideDistanceComparator struct {
	field       string
	planetModel *geom.PlanetModel
	shape       geom.GeoShape
}
