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
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
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
// Geo3DDocValuesField is a doc-values field storing an XYZ location.
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

// FromDegrees converts degrees to radians.
//
// Port of Geo3DUtil.fromDegrees.
func FromDegrees(degrees float64) float64 {
	return degrees * RadiansPerDegree
}

// FromBox creates a GeoBBox from degree-valued lat/lon bounds.
//
// Port of Geo3DUtil.fromBox.
func FromBox(pm *geom.PlanetModel, minLatitude, maxLatitude, minLongitude, maxLongitude float64) (geom.GeoBBox, error) {
	return geom.MakeGeoBBox(pm,
		FromDegrees(maxLatitude),
		FromDegrees(minLatitude),
		FromDegrees(minLongitude),
		FromDegrees(maxLongitude))
}

// FromDistance creates a GeoCircle from lat/lon (in degrees) and a radius in meters.
//
// Port of Geo3DUtil.fromDistance.
func FromDistance(pm *geom.PlanetModel, latitude, longitude, radiusMeters float64) (geom.GeoCircle, error) {
	radiusRadians := radiusMeters / pm.MeanRadius
	return geom.MakeGeoCircle(pm, FromDegrees(latitude), FromDegrees(longitude), radiusRadians)
}

// FromPath creates a GeoPath from arrays of latitudes/longitudes (in degrees) and
// a path width in meters.
//
// Port of Geo3DUtil.fromPath.
func FromPath(pm *geom.PlanetModel, pathLatitudes, pathLongitudes []float64, pathWidthMeters float64) geom.GeoPath {
	if len(pathLatitudes) != len(pathLongitudes) {
		panic("same number of latitudes and longitudes required")
	}
	points := make([]*geom.GeoPoint, len(pathLatitudes))
	for i := range pathLatitudes {
		lat := pathLatitudes[i]
		lon := pathLongitudes[i]
		if lat < -90 || lat > 90 {
			panic("latitude out of bounds")
		}
		if lon < -180 || lon > 180 {
			panic("longitude out of bounds")
		}
		points[i] = geom.NewGeoPointModel(pm, FromDegrees(lat), FromDegrees(lon))
	}
	radiusRadians := pathWidthMeters / (pm.MeanRadius * pm.XYScaling)
	return geom.MakeGeoPath(pm, radiusRadians, points)
}

// NewGeo3DPointSort creates a SortField that orders documents by distance to a Geo3D shape.
//
// Port of Geo3DDocValuesField.newDistanceSort and newPathSort.
func NewGeo3DPointSort(field string, pm *geom.PlanetModel, shape geom.GeoDistanceShape, reverse bool) *search.SortField {
	source := &Geo3DPointSortFieldSource{
		field:       field,
		planetModel: pm,
		shape:       shape,
	}
	return search.NewSortFieldCustom(field, source, reverse)
}

// NewGeo3DPointOutsideSort creates a SortField that orders documents by outside distance to a Geo3D shape.
//
// Port of Geo3DDocValuesField.newOutsideDistanceSort, newOutsideBoxSort, etc.
func NewGeo3DPointOutsideSort(field string, pm *geom.PlanetModel, shape geom.GeoOutsideDistance, reverse bool) *search.SortField {
	source := &Geo3DPointOutsideSortFieldSource{
		field:       field,
		planetModel: pm,
		shape:       shape,
	}
	return search.NewSortFieldCustom(field, source, reverse)
}

// Geo3DPointSortFieldSource creates a Geo3DPointDistanceComparator.
type Geo3DPointSortFieldSource struct {
	field       string
	planetModel *geom.PlanetModel
	shape       geom.GeoDistanceShape
}

func (s *Geo3DPointSortFieldSource) NewComparator(fieldname string, numHits int, pruning search.Pruning, reversed bool) search.FieldComparator {
	return NewGeo3DPointDistanceComparator(fieldname, s.planetModel, s.shape, numHits)
}

// Geo3DPointOutsideSortFieldSource creates a Geo3DPointOutsideDistanceComparator.
type Geo3DPointOutsideSortFieldSource struct {
	field       string
	planetModel *geom.PlanetModel
	shape       geom.GeoOutsideDistance
}

func (s *Geo3DPointOutsideSortFieldSource) NewComparator(fieldname string, numHits int, pruning search.Pruning, reversed bool) search.FieldComparator {
	return NewGeo3DPointOutsideDistanceComparator(fieldname, s.planetModel, s.shape, numHits)
}

// Geo3DPointDistanceComparator computes in-shape distance for sorting.
//
// Port of org.apache.lucene.spatial3d.Geo3DPointDistanceComparator.
type Geo3DPointDistanceComparator struct {
	search.BaseFieldComparator

	field          string
	planetModel    *geom.PlanetModel
	distanceShape  geom.GeoDistanceShape
	values         []float64
	bottomDist     float64
	topValue       float64
	currentDocs    index.SortedNumericDocValues
	pqBounds       *geom.XYZBounds
	setBottomCount int
}

func NewGeo3DPointDistanceComparator(field string, pm *geom.PlanetModel, shape geom.GeoDistanceShape, numHits int) *Geo3DPointDistanceComparator {
	return &Geo3DPointDistanceComparator{
		field:         field,
		planetModel:   pm,
		distanceShape: shape,
		values:        make([]float64, numHits),
	}
}

func (c *Geo3DPointDistanceComparator) Compare(slot1, slot2 int) int {
	if c.values[slot1] < c.values[slot2] {
		return -1
	} else if c.values[slot1] > c.values[slot2] {
		return 1
	}
	return 0
}

func (c *Geo3DPointDistanceComparator) SetBottom(slot int) error {
	c.bottomDist = c.values[slot]
	if c.setBottomCount < 1024 || (c.setBottomCount&0x3F) == 0x3F {
		bounds := &geom.XYZBounds{}
		c.distanceShape.GetDistanceBounds(bounds, geom.ARC, c.bottomDist)
		c.pqBounds = bounds
	}
	c.setBottomCount++
	return nil
}

func (c *Geo3DPointDistanceComparator) SetTopValue(value any) {
	// Java's setTopValue(Double) unboxes with value.doubleValue(); a value of
	// another type raises ClassCastException, which the assertion reproduces.
	c.topValue = value.(float64)
}

func (c *Geo3DPointDistanceComparator) CompareBottom(doc int) (int, error) {
	if doc > c.currentDocs.DocID() {
		if _, err := c.currentDocs.Advance(doc); err != nil {
			return 0, err
		}
	}
	if doc < c.currentDocs.DocID() {
		if c.bottomDist < math.Inf(1) {
			return -1, nil
		}
		return 0, nil
	}

	numValues, err := c.currentDocs.DocValueCount()
	if err != nil {
		return 0, err
	}
	cmp := -1
	encoder := geom.NewDocValueEncoder(c.planetModel)

	for i := 0; i < numValues; i++ {
		encoded, err := c.currentDocs.NextValue()
		if err != nil {
			return 0, err
		}
		x := encoder.DecodeXValue(encoded)
		y := encoder.DecodeYValue(encoded)
		z := encoder.DecodeZValue(encoded)

		if c.pqBounds != nil {
			if x > c.pqBounds.MaximumX || x < c.pqBounds.MinimumX ||
				y > c.pqBounds.MaximumY || y < c.pqBounds.MinimumY ||
				z > c.pqBounds.MaximumZ || z < c.pqBounds.MinimumZ {
				continue
			}
		}

		dist := c.distanceShape.ComputeDistance(geom.ARC, x, y, z)
		if c.bottomDist < dist {
			cmp = 1
		} else if c.bottomDist > dist {
			cmp = -1
		} else {
			cmp = 0
		}
	}
	return cmp, nil
}

func (c *Geo3DPointDistanceComparator) Copy(slot, doc int) error {
	dist, err := c.computeMinimumDistance(doc)
	if err != nil {
		return err
	}
	c.values[slot] = dist
	return nil
}

func (c *Geo3DPointDistanceComparator) SetReader(reader index.LeafReader) error {
	dv, err := reader.GetSortedNumericDocValues(c.field)
	if err != nil {
		return fmt.Errorf("geo3d: SetReader: %w", err)
	}
	c.currentDocs = dv
	return nil
}

// GetLeafComparator binds the comparator to the segment's SortedNumericDocValues
// and returns itself, as Geo3DPointDistanceComparator does (it implements
// LeafFieldComparator).
//
// Mirrors Geo3DPointDistanceComparator.getLeafComparator(LeafReaderContext).
func (c *Geo3DPointDistanceComparator) GetLeafComparator(context *index.LeafReaderContext) (search.LeafFieldComparator, error) {
	if context == nil {
		return nil, fmt.Errorf("geo3d: GetLeafComparator: leaf reader context must not be nil")
	}
	leaf := context.LeafReader()
	if leaf == nil {
		return nil, fmt.Errorf("geo3d: GetLeafComparator: leaf reader context has no reader")
	}
	if err := c.SetReader(leaf); err != nil {
		return nil, err
	}
	return c, nil
}

// SetScorer is empty, as Geo3DPointDistanceComparator.setScorer is.
func (c *Geo3DPointDistanceComparator) SetScorer(search.Scorable) error { return nil }

// CompetitiveIterator returns nil: Geo3DPointDistanceComparator does not override
// the LeafFieldComparator default, which returns null.
func (c *Geo3DPointDistanceComparator) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

// SetHitsThresholdReached is empty: Geo3DPointDistanceComparator does not override
// the LeafFieldComparator default, whose body is empty.
func (c *Geo3DPointDistanceComparator) SetHitsThresholdReached() error { return nil }

func (c *Geo3DPointDistanceComparator) Value(slot int) any {
	return c.values[slot] * c.planetModel.MeanRadius
}

func (c *Geo3DPointDistanceComparator) CompareTop(doc int) (int, error) {
	dist, err := c.computeMinimumDistance(doc)
	if err != nil {
		return 0, err
	}
	if c.topValue < dist {
		return -1, nil
	} else if c.topValue > dist {
		return 1, nil
	}
	return 0, nil
}

func (c *Geo3DPointDistanceComparator) computeMinimumDistance(doc int) (float64, error) {
	if doc > c.currentDocs.DocID() {
		if _, err := c.currentDocs.Advance(doc); err != nil {
			return 0, err
		}
	}
	minValue := math.Inf(1)
	if doc == c.currentDocs.DocID() {
		numValues, err := c.currentDocs.DocValueCount()
		if err != nil {
			return 0, err
		}
		encoder := geom.NewDocValueEncoder(c.planetModel)
		for i := 0; i < numValues; i++ {
			encoded, err := c.currentDocs.NextValue()
			if err != nil {
				return 0, err
			}
			dist := c.distanceShape.ComputeDistance(
				geom.ARC,
				encoder.DecodeXValue(encoded),
				encoder.DecodeYValue(encoded),
				encoder.DecodeZValue(encoded))
			if dist < minValue {
				minValue = dist
			}
		}
	}
	return minValue, nil
}

func (c *Geo3DPointDistanceComparator) ComputeMinimumDistanceMock(encoded int64) float64 {
	encoder := geom.NewDocValueEncoder(c.planetModel)
	return c.distanceShape.ComputeDistance(
		geom.ARC,
		encoder.DecodeXValue(encoded),
		encoder.DecodeYValue(encoded),
		encoder.DecodeZValue(encoded))
}

// Geo3DPointOutsideDistanceComparator computes outside-shape distance for sorting.
//
// Port of org.apache.lucene.spatial3d.Geo3DPointOutsideDistanceComparator.
type Geo3DPointOutsideDistanceComparator struct {
	search.BaseFieldComparator

	field         string
	planetModel   *geom.PlanetModel
	distanceShape geom.GeoOutsideDistance
	values        []float64
	bottomDist    float64
	topValue      float64
	currentDocs   index.SortedNumericDocValues
}

func NewGeo3DPointOutsideDistanceComparator(field string, pm *geom.PlanetModel, shape geom.GeoOutsideDistance, numHits int) *Geo3DPointOutsideDistanceComparator {
	return &Geo3DPointOutsideDistanceComparator{
		field:         field,
		planetModel:   pm,
		distanceShape: shape,
		values:        make([]float64, numHits),
	}
}

func (c *Geo3DPointOutsideDistanceComparator) Compare(slot1, slot2 int) int {
	if c.values[slot1] < c.values[slot2] {
		return -1
	} else if c.values[slot1] > c.values[slot2] {
		return 1
	}
	return 0
}

func (c *Geo3DPointOutsideDistanceComparator) SetBottom(slot int) error {
	c.bottomDist = c.values[slot]
	return nil
}

func (c *Geo3DPointOutsideDistanceComparator) SetTopValue(value any) {
	// Java's setTopValue(Double) unboxes with value.doubleValue(); a value of
	// another type raises ClassCastException, which the assertion reproduces.
	c.topValue = value.(float64)
}

func (c *Geo3DPointOutsideDistanceComparator) CompareBottom(doc int) (int, error) {
	if doc > c.currentDocs.DocID() {
		if _, err := c.currentDocs.Advance(doc); err != nil {
			return 0, err
		}
	}
	if doc < c.currentDocs.DocID() {
		if c.bottomDist < math.Inf(1) {
			return -1, nil
		}
		return 0, nil
	}

	numValues, err := c.currentDocs.DocValueCount()
	if err != nil {
		return 0, err
	}
	cmp := -1
	encoder := geom.NewDocValueEncoder(c.planetModel)

	for i := 0; i < numValues; i++ {
		encoded, err := c.currentDocs.NextValue()
		if err != nil {
			return 0, err
		}
		x := encoder.DecodeXValue(encoded)
		y := encoder.DecodeYValue(encoded)
		z := encoder.DecodeZValue(encoded)

		dist := c.distanceShape.ComputeOutsideDistance(geom.ARC, x, y, z)
		if c.bottomDist < dist {
			cmp = 1
		} else if c.bottomDist > dist {
			cmp = -1
		} else {
			cmp = 0
		}
	}
	return cmp, nil
}

func (c *Geo3DPointOutsideDistanceComparator) Copy(slot, doc int) error {
	dist, err := c.computeMinimumDistance(doc)
	if err != nil {
		return err
	}
	c.values[slot] = dist
	return nil
}

func (c *Geo3DPointOutsideDistanceComparator) SetReader(reader index.LeafReader) error {
	dv, err := reader.GetSortedNumericDocValues(c.field)
	if err != nil {
		return fmt.Errorf("geo3d: SetReader: %w", err)
	}
	c.currentDocs = dv
	return nil
}

// GetLeafComparator binds the comparator to the segment's SortedNumericDocValues
// and returns itself, as Geo3DPointOutsideDistanceComparator does (it implements
// LeafFieldComparator).
//
// Mirrors Geo3DPointOutsideDistanceComparator.getLeafComparator(LeafReaderContext).
func (c *Geo3DPointOutsideDistanceComparator) GetLeafComparator(context *index.LeafReaderContext) (search.LeafFieldComparator, error) {
	if context == nil {
		return nil, fmt.Errorf("geo3d: GetLeafComparator: leaf reader context must not be nil")
	}
	leaf := context.LeafReader()
	if leaf == nil {
		return nil, fmt.Errorf("geo3d: GetLeafComparator: leaf reader context has no reader")
	}
	if err := c.SetReader(leaf); err != nil {
		return nil, err
	}
	return c, nil
}

// SetScorer is empty, as Geo3DPointOutsideDistanceComparator.setScorer is.
func (c *Geo3DPointOutsideDistanceComparator) SetScorer(search.Scorable) error { return nil }

// CompetitiveIterator returns nil: Geo3DPointOutsideDistanceComparator does not override
// the LeafFieldComparator default, which returns null.
func (c *Geo3DPointOutsideDistanceComparator) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

// SetHitsThresholdReached is empty: Geo3DPointOutsideDistanceComparator does not override
// the LeafFieldComparator default, whose body is empty.
func (c *Geo3DPointOutsideDistanceComparator) SetHitsThresholdReached() error { return nil }

func (c *Geo3DPointOutsideDistanceComparator) Value(slot int) any {
	return c.values[slot] * c.planetModel.MeanRadius
}

func (c *Geo3DPointOutsideDistanceComparator) CompareTop(doc int) (int, error) {
	dist, err := c.computeMinimumDistance(doc)
	if err != nil {
		return 0, err
	}
	if c.topValue < dist {
		return -1, nil
	} else if c.topValue > dist {
		return 1, nil
	}
	return 0, nil
}

func (c *Geo3DPointOutsideDistanceComparator) computeMinimumDistance(doc int) (float64, error) {
	if doc > c.currentDocs.DocID() {
		if _, err := c.currentDocs.Advance(doc); err != nil {
			return 0, err
		}
	}
	minValue := math.Inf(1)
	if doc == c.currentDocs.DocID() {
		numValues, err := c.currentDocs.DocValueCount()
		if err != nil {
			return 0, err
		}
		encoder := geom.NewDocValueEncoder(c.planetModel)
		for i := 0; i < numValues; i++ {
			encoded, err := c.currentDocs.NextValue()
			if err != nil {
				return 0, err
			}
			dist := c.distanceShape.ComputeOutsideDistance(
				geom.ARC,
				encoder.DecodeXValue(encoded),
				encoder.DecodeYValue(encoded),
				encoder.DecodeZValue(encoded))
			if dist < minValue {
				minValue = dist
			}
		}
	}
	return minValue, nil
}

func (c *Geo3DPointOutsideDistanceComparator) ComputeMinimumDistanceMock(encoded int64) float64 {
	encoder := geom.NewDocValueEncoder(c.planetModel)
	return c.distanceShape.ComputeOutsideDistance(
		geom.ARC,
		encoder.DecodeXValue(encoded),
		encoder.DecodeYValue(encoded),
		encoder.DecodeZValue(encoded))
}

var (
	_ search.FieldComparator     = (*Geo3DPointDistanceComparator)(nil)
	_ search.LeafFieldComparator = (*Geo3DPointDistanceComparator)(nil)
	_ search.FieldComparator     = (*Geo3DPointOutsideDistanceComparator)(nil)
	_ search.LeafFieldComparator = (*Geo3DPointOutsideDistanceComparator)(nil)
)
