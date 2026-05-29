// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package spatial3d provides Geo3D point fields, sort fields, and queries
// for three-dimensional sphere geometry.
//
// Port of org.apache.lucene.spatial3d.
//
// The geometric engine in the geom sub-package is implemented (rmp #4682):
// Plane.FindIntersections / Intersects, SidedPlane membership, and the
// GeoStandardCircle (within-circle), GeoRectangle (within-bbox), and
// GeoConvexPolygon / GeoConcavePolygon (point-in-polygon) shapes, with
// behavioural parity against Lucene 10.4.0 verified by unit tests.
//
// The shape-query path is wired (rmp #4750): PointInGeo3DShapeQuery is a
// ConstantScore query whose Weight walks the BKD point tree via
// PointInShapeIntersectVisitor, decodes each 3-D point with DecodeDimension,
// and admits a document iff geom.GeoShape.IsWithin reports membership.
// IsWithin is the authoritative final gate, so the produced document set is
// exactly correct for any GeoShape (circle, bbox, convex/concave polygon).
// See geo3d_query.go for the Weight/Scorer/visitor implementation.
//
// Deviation (performance-only, correctness-preserving): the visitor's
// Compare(minPacked, maxPacked) always returns CELL_CROSSES_QUERY, so the BKD
// walk visits every leaf and gates every point through IsWithin rather than
// pruning sub-trees with the shape's XYZ bounding box. This is because the
// bounding-box engine (XYZBounds.AddPlane-family and XYZSolid.GetRelationship)
// is still stubbed; restoring it as a BKD prefilter is tracked by rmp #4768.
// Reading on-disk Geo3DPoint values through LeafReader.GetPointValues is
// tracked by rmp #4769; until that lands, the query is exercised against an
// in-memory PointValues stub (see geo3d_query_test.go).
//
// The Geo3DPoint sort fields and comparators remain deferred (backlog #2693).
//
// Already delivered (T4650):
//   - Correct PlanetModel construction (xyScaling = a/meanRadius).
//   - PlanetModel and GeoPoint binary serialisation (round-trip compatible with
//     Lucene 10.4.0 SerializableObject wire format).
//   - EncodeDimension / DecodeDimension using IntToSortableBytes / SortableBytesToInt.
//   - Geo3DPoint.ToIndexableFields producing a 3-dimension × 4-byte BKD encoding.
package spatial3d

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// geo3dPointType is the FieldType for a Geo3DPoint: 3 dimensions × 4 bytes.
// Mirrors Lucene's static Geo3DPoint.TYPE = new FieldType(); setDimensions(3,4); freeze().
var geo3dPointType *document.FieldType

func init() {
	geo3dPointType = document.NewFieldType()
	geo3dPointType.SetIndexed(true)
	geo3dPointType.SetDimensions(3, 4)
	geo3dPointType.Freeze()
}

// ---------------------------------------------------------------------------
// Geo3DPoint
//
// Port of org.apache.lucene.spatial3d.Geo3DPoint.
// ---------------------------------------------------------------------------

// Geo3DPoint is a point stored as a three-dimensional XYZ coordinate.
//
// ToIndexableFields produces the wire-compatible BKD encoding (3 dims × 4 bytes)
// using IntToSortableBytes(PlanetModel.encodeValue(coord)).
//
// The query infrastructure (PointInGeo3DShapeQuery, PointInShapeIntersectVisitor)
// is implemented in geo3d_query.go (rmp #4750).
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

// ToIndexableFields returns a slice containing a single point field encoding
// the XYZ coordinate as 3 × 4-byte sortable integers.
//
// Port of Geo3DPoint.fillFieldsData: encodeDimension(x,0) + encodeDimension(y,4) + encodeDimension(z,8).
func (p *Geo3DPoint) ToIndexableFields() ([]document.IndexableField, error) {
	bytes := make([]byte, 3*bytesPerDim)
	EncodeDimension(p.planetModel, p.X, bytes, 0)
	EncodeDimension(p.planetModel, p.Y, bytes, bytesPerDim)
	EncodeDimension(p.planetModel, p.Z, bytes, 2*bytesPerDim)
	f, err := document.NewField(p.fieldName, bytes, geo3dPointType)
	if err != nil {
		return nil, fmt.Errorf("geo3d: ToIndexableFields: %w", err)
	}
	return []document.IndexableField{f}, nil
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
// Port of org.apache.lucene.spatial3d.Geo3DUtil (package-private in Java)
// and the static helpers on org.apache.lucene.spatial3d.Geo3DPoint.
// ---------------------------------------------------------------------------

// RadiansPerDegree is the factor to convert degrees to radians.
const RadiansPerDegree = math.Pi / 180.0

// bytesPerDim is the number of bytes per BKD dimension (Integer.BYTES in Java).
const bytesPerDim = 4

// EncodeDimension encodes a coordinate value to sortable bytes in-place.
//
// Equivalent to NumericUtils.intToSortableBytes(planetModel.encodeValue(value)).
// Port of org.apache.lucene.spatial3d.Geo3DPoint.encodeDimension.
func EncodeDimension(pm *geom.PlanetModel, value float64, bytes []byte, offset int) {
	util.IntToSortableBytes(pm.EncodeValue(value), bytes, offset)
}

// DecodeDimension decodes a sortable-bytes-encoded coordinate value.
//
// Equivalent to planetModel.decodeValue(NumericUtils.sortableBytesToInt(...)).
// Port of org.apache.lucene.spatial3d.Geo3DPoint.decodeDimension.
func DecodeDimension(pm *geom.PlanetModel, value []byte, offset int) float64 {
	return pm.DecodeValue(util.SortableBytesToInt(value, offset))
}

// PointInGeo3DShapeQuery and PointInShapeIntersectVisitor are implemented in
// geo3d_query.go (rmp #4750).

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
