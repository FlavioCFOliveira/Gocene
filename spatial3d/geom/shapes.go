// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

// This file contains all concrete GeoShape stubs. Full implementations are
// deferred to backlog #2693.

// ---------------------------------------------------------------------------
// Bounding-box shapes
// ---------------------------------------------------------------------------

// GeoRectangle is a bounding box limited on four sides (top/bottom latitude,
// left/right longitude). Its left-right extent must be at most PI.
//
// Port of org.apache.lucene.spatial3d.geom.GeoRectangle.
type GeoRectangle struct {
	GeoBaseBBox
	topLat, bottomLat   float64
	leftLon, rightLon   float64
	cosMiddleLat        float64
	ulhc, urhc          *GeoPoint // upper-left, upper-right hand corners
	lrhc, llhc          *GeoPoint // lower-right, lower-left hand corners
	topPlane            *SidedPlane
	bottomPlane         *SidedPlane
	leftPlane           *SidedPlane
	rightPlane          *SidedPlane
	backingPlane        *SidedPlane
	topPlanePoints      []*GeoPoint
	bottomPlanePoints   []*GeoPoint
	leftPlanePoints     []*GeoPoint
	rightPlanePoints    []*GeoPoint
	centerPoint         *GeoPoint
	rectangleEdgePoints []*GeoPoint
}

// GeoNorthRectangle is a rectangle extending to the north pole.
// The left-right maximum extent for this shape is PI.
//
// Port of org.apache.lucene.spatial3d.geom.GeoNorthRectangle.
type GeoNorthRectangle struct {
	GeoBaseBBox
	f geoNorthRectangleFields
}

// GeoSouthRectangle is a rectangle extending to the south pole.
// The left-right maximum extent for this shape is PI.
//
// Port of org.apache.lucene.spatial3d.geom.GeoSouthRectangle.
type GeoSouthRectangle struct {
	GeoBaseBBox
	f geoSouthRectangleFields
}

// GeoWideRectangle is a rectangle that spans more than π in longitude.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideRectangle.
type GeoWideRectangle struct {
	GeoBaseBBox
	f geoWideRectangleFields
}

// GeoWideNorthRectangle is a wide rectangle extending to the north pole.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideNorthRectangle.
type GeoWideNorthRectangle struct {
	GeoBaseBBox
	f geoWideNorthRectangleFields
}

// GeoWideSouthRectangle is a wide rectangle extending to the south pole.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideSouthRectangle.
type GeoWideSouthRectangle struct {
	GeoBaseBBox
	f geoWideSouthRectangleFields
}

// GeoWorld represents the whole sphere.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWorld.
type GeoWorld struct {
	GeoBaseBBox
	originPoint *GeoPoint
}

// IsWithin always returns true for the world shape.
func (w *GeoWorld) IsWithin(_, _, _ float64) bool { return true }

// GetRelationship returns the spatial relationship between the world and path.
//
// Per GeoWorld.getRelationship: if path has edge points, the path is WITHIN
// the world (RelWithin). If path has no edge points (e.g. another world-wide
// shape), they OVERLAP. A nil path is treated as an empty world-level shape
// that contains the world area (RelContains), preserving backward compatibility.
//
// Port of GeoWorld.getRelationship.
func (w *GeoWorld) GetRelationship(path GeoShape) int {
	if path == nil {
		return RelContains
	}
	if len(path.GetEdgePoints()) > 0 {
		return RelWithin
	}
	return RelOverlaps
}

// GeoLongitudeSlice is a slice bounded by two meridians.
// The left-right maximum extent for this shape is PI.
//
// Port of org.apache.lucene.spatial3d.geom.GeoLongitudeSlice.
type GeoLongitudeSlice struct {
	GeoBaseBBox
	f geoLongitudeSliceFields
}

// GeoWideLongitudeSlice is a longitude slice spanning more than π.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideLongitudeSlice.
type GeoWideLongitudeSlice struct {
	GeoBaseBBox
	f geoWideLongitudeSliceFields
}

// GeoLatitudeZone is a band between two latitude parallels.
//
// Port of org.apache.lucene.spatial3d.geom.GeoLatitudeZone.
type GeoLatitudeZone struct {
	GeoBaseBBox
	f geoLatitudeZoneFields
}

// GeoNorthLatitudeZone is a band north of a latitude.
//
// Port of org.apache.lucene.spatial3d.geom.GeoNorthLatitudeZone.
type GeoNorthLatitudeZone struct {
	GeoBaseBBox
	f geoNorthLatitudeZoneFields
}

// GeoSouthLatitudeZone is a band south of a latitude.
//
// Port of org.apache.lucene.spatial3d.geom.GeoSouthLatitudeZone.
type GeoSouthLatitudeZone struct {
	GeoBaseBBox
	f geoSouthLatitudeZoneFields
}

// GeoDegenerateHorizontalLine is a degenerate horizontal (latitude) line.
// The left-right maximum extent for this shape is PI.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateHorizontalLine.
type GeoDegenerateHorizontalLine struct {
	GeoBaseBBox
	f geoDegenerateHorizontalLineFields
}

// GeoWideDegenerateHorizontalLine is a wide degenerate horizontal line
// (more than π in longitude extent).
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideDegenerateHorizontalLine.
type GeoWideDegenerateHorizontalLine struct {
	GeoBaseBBox
	f geoWideDegenerateHorizontalLineFields
}

// GeoDegenerateLongitudeSlice is a degenerate longitude slice (a single meridian).
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateLongitudeSlice.
type GeoDegenerateLongitudeSlice struct {
	GeoBaseBBox
	f geoDegenerateLongitudeSliceFields
}

// GeoDegenerateLatitudeZone is a degenerate latitude zone (a full latitude circle).
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateLatitudeZone.
type GeoDegenerateLatitudeZone struct {
	GeoBaseBBox
	f geoDegenerateLatitudeZoneFields
}

// GeoDegenerateVerticalLine is a vertical degenerate line (a longitude segment).
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateVerticalLine.
type GeoDegenerateVerticalLine struct {
	GeoBaseBBox
	f geoDegenerateVerticalLineFields
}

// GeoDegeneratePoint is a single point that simultaneously satisfies GeoBBox
// and GeoCircle (a degenerate bounding box / degenerate circle).
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegeneratePoint.
type GeoDegeneratePoint struct {
	GeoBaseBBox
	GeoBaseCircle
	point *GeoPoint
}

// NewGeoDegeneratePoint constructs a degenerate point shape at the given point.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegeneratePoint(PlanetModel,GeoPoint).
func NewGeoDegeneratePoint(pm *PlanetModel, point *GeoPoint) *GeoDegeneratePoint {
	return &GeoDegeneratePoint{GeoBaseBBox: makeBBox(pm), point: point}
}

// GetPoint returns the underlying point.
func (p *GeoDegeneratePoint) GetPoint() *GeoPoint { return p.point }

// GetCenter returns the underlying point.
func (p *GeoDegeneratePoint) GetCenter() *GeoPoint { return p.point }

// GetRadius returns 0 — a point has zero radius.
func (p *GeoDegeneratePoint) GetRadius() float64 { return 0 }

// IsWithin reports whether (x,y,z) is numerically identical to this point.
//
// Port of GeoDegeneratePoint.isWithin.
func (p *GeoDegeneratePoint) IsWithin(x, y, z float64) bool {
	return p.point.IsNumericallyIdentical(x, y, z)
}

// GetEdgePoints returns the sole edge point.
func (p *GeoDegeneratePoint) GetEdgePoints() []*GeoPoint {
	if p.point == nil {
		return nil
	}
	return []*GeoPoint{p.point}
}

// GetBounds accumulates the single point.
//
// Port of GeoDegeneratePoint.getBounds.
func (p *GeoDegeneratePoint) GetBounds(bounds Bounds) {
	geoBaseGetBounds(p, p.PlanetModelField, bounds)
	bounds.AddPoint(p.point)
}

// ---------------------------------------------------------------------------
// Circles
// ---------------------------------------------------------------------------

// GeoStandardCircle is a standard circle on the sphere (an ellipse on a
// non-spherical world): the set of points cut off by a single sided plane at a
// fixed cutoff angle from the centre.
//
// Port of org.apache.lucene.spatial3d.geom.GeoStandardCircle.
type GeoStandardCircle struct {
	GeoBaseCircle
	center      *GeoPoint
	cutoffAngle float64
	circlePlane *SidedPlane // nil means the whole world
	edgePoints  []*GeoPoint
}

// GeoExactCircle is a circle that exactly traces the sphere surface.
// The circle edge is approximated by Vincenty-formula sector planes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoExactCircle.
type GeoExactCircle struct {
	GeoBaseCircle
	f geoExactCircleFields
}

// ---------------------------------------------------------------------------
// Polygons
// ---------------------------------------------------------------------------

// GeoConvexPolygon is a convex polygon on the sphere. It must be convex with a
// maximum extent no larger than PI; a point is inside when it is on the inside
// of every edge plane.
//
// Port of org.apache.lucene.spatial3d.geom.GeoConvexPolygon.
type GeoConvexPolygon struct {
	GeoBasePolygon
	points          []*GeoPoint
	isInternalEdges []bool
	holes           []GeoPolygon
	edges           []*SidedPlane
	startBounds     []*SidedPlane
	endBounds       []*SidedPlane
	notableEdgePts  [][]*GeoPoint
	edgePoints      []*GeoPoint
	prevBrotherMap  map[*SidedPlane]*SidedPlane
	nextBrotherMap  map[*SidedPlane]*SidedPlane
}

// GeoConcavePolygon is a concave polygon on the sphere (extent larger than PI).
// A point is inside when it is on the inside of *any* edge plane.
//
// Port of org.apache.lucene.spatial3d.geom.GeoConcavePolygon.
type GeoConcavePolygon struct {
	GeoBasePolygon
	points          []*GeoPoint
	isInternalEdges []bool
	holes           []GeoPolygon
	edges           []*SidedPlane
	invertedEdges   []*SidedPlane
	startBounds     []*SidedPlane
	endBounds       []*SidedPlane
	notableEdgePts  [][]*GeoPoint
	edgePoints      []*GeoPoint
	prevBrotherMap  map[*SidedPlane]*SidedPlane
	nextBrotherMap  map[*SidedPlane]*SidedPlane
}

// GeoCompositePolygon is a composite polygon made of multiple sub-polygons.
//
// Port of org.apache.lucene.spatial3d.geom.GeoCompositePolygon.
type GeoCompositePolygon struct{ GeoBasePolygon }

// GeoComplexPolygon is a complex (possibly self-intersecting) polygon.
//
// Port of org.apache.lucene.spatial3d.geom.GeoComplexPolygon.
type GeoComplexPolygon struct{ GeoBasePolygon }

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------

// GeoStandardPath is a standard (rectangular cross-section) path.
//
// Port of org.apache.lucene.spatial3d.geom.GeoStandardPath.
type GeoStandardPath struct{ GeoBasePath }

// GeoDegeneratePath is a degenerate (zero-width) path.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegeneratePath.
type GeoDegeneratePath struct{ GeoBasePath }

// ---------------------------------------------------------------------------
// Composite shapes
// ---------------------------------------------------------------------------

// GeoCompositeMembershipShape is a union of membership shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoCompositeMembershipShape.
type GeoCompositeMembershipShape struct {
	GeoBaseCompositeMembershipShape
}

// GeoCompositeAreaShape is a union of area shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoCompositeAreaShape.
type GeoCompositeAreaShape struct {
	GeoBaseCompositeAreaShape
}

// ---------------------------------------------------------------------------
// Point shape
// ---------------------------------------------------------------------------

// GeoPointShapeImpl is a single-point GeoPointShape.
// It is simultaneously a degenerate circle and a degenerate bounding box.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPointShape (no separate impl class).
type GeoPointShapeImpl struct {
	GeoBaseBBox
	GeoBaseCircle
	point *GeoPoint
}

// GetPoint returns the underlying point.
func (s *GeoPointShapeImpl) GetPoint() *GeoPoint { return s.point }

// GetRadius returns 0 — a point has no radius.
func (s *GeoPointShapeImpl) GetRadius() float64 { return 0 }

// Expand returns nil — deferred to #2693.
func (s *GeoPointShapeImpl) Expand(_ float64) GeoBBox { return nil }

// IsWithin reports whether (x,y,z) is at this point (deferred to #2693).
func (s *GeoPointShapeImpl) IsWithin(_, _, _ float64) bool { return false }

// GetEdgePoints returns the underlying point as the sole edge point.
func (s *GeoPointShapeImpl) GetEdgePoints() []*GeoPoint {
	if s.point == nil {
		return nil
	}
	return []*GeoPoint{s.point}
}

// GetRelationship returns RelDisjoint — deferred to #2693.
func (s *GeoPointShapeImpl) GetRelationship(_ GeoShape) int { return RelDisjoint }

// ---------------------------------------------------------------------------
// S2 shape
// ---------------------------------------------------------------------------

// GeoS2ShapeImpl is an S2-backed shape (a fast convex 4-sided polygon).
//
// The four points must be supplied in CCW order and form a convex quadrilateral.
//
// Port of org.apache.lucene.spatial3d.geom.GeoS2Shape.
type GeoS2ShapeImpl struct {
	GeoBaseMembershipShape
	point1, point2, point3, point4            *GeoPoint
	plane1, plane2, plane3, plane4            *SidedPlane
	plane1Points, plane2Points, plane3Points, plane4Points []*GeoPoint
	edgePoints                                []*GeoPoint
}

// NewGeoS2Shape constructs a GeoS2ShapeImpl from four points in CCW order.
//
// Port of GeoS2Shape(PlanetModel,GeoPoint,GeoPoint,GeoPoint,GeoPoint).
func NewGeoS2Shape(pm *PlanetModel, point1, point2, point3, point4 *GeoPoint) *GeoS2ShapeImpl {
	s := &GeoS2ShapeImpl{
		GeoBaseMembershipShape: makeMem(pm),
		point1: point1,
		point2: point2,
		point3: point3,
		point4: point4,
	}
	s.plane1 = NewSidedPlaneThreeVectors(&point4.Vector, &point1.Vector, &point2.Vector)
	s.plane2 = NewSidedPlaneThreeVectors(&point1.Vector, &point2.Vector, &point3.Vector)
	s.plane3 = NewSidedPlaneThreeVectors(&point2.Vector, &point3.Vector, &point4.Vector)
	s.plane4 = NewSidedPlaneThreeVectors(&point3.Vector, &point4.Vector, &point1.Vector)

	s.plane1Points = []*GeoPoint{point1, point2}
	s.plane2Points = []*GeoPoint{point2, point3}
	s.plane3Points = []*GeoPoint{point3, point4}
	s.plane4Points = []*GeoPoint{point4, point1}

	s.edgePoints = []*GeoPoint{point1}
	return s
}

// IsWithin reports whether (x,y,z) is inside all four bounding planes.
func (s *GeoS2ShapeImpl) IsWithin(x, y, z float64) bool {
	return s.plane1.IsWithin(x, y, z) &&
		s.plane2.IsWithin(x, y, z) &&
		s.plane3.IsWithin(x, y, z) &&
		s.plane4.IsWithin(x, y, z)
}

// GetEdgePoints returns the edge points of this shape.
func (s *GeoS2ShapeImpl) GetEdgePoints() []*GeoPoint {
	return s.edgePoints
}

// Intersects reports whether the given plane intersects this S2 shape.
//
// Port of GeoS2Shape.intersects(Plane,GeoPoint[],Membership...).
func (s *GeoS2ShapeImpl) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	return p.Intersects(s.PlanetModelField, &s.plane1.Plane, notablePoints, s.plane1Points, bounds, s.plane2, s.plane4) ||
		p.Intersects(s.PlanetModelField, &s.plane2.Plane, notablePoints, s.plane2Points, bounds, s.plane3, s.plane1) ||
		p.Intersects(s.PlanetModelField, &s.plane3.Plane, notablePoints, s.plane3Points, bounds, s.plane4, s.plane2) ||
		p.Intersects(s.PlanetModelField, &s.plane4.Plane, notablePoints, s.plane4Points, bounds, s.plane1, s.plane3)
}

// GetBounds accumulates bounding information.
//
// Port of GeoS2Shape.getBounds.
func (s *GeoS2ShapeImpl) GetBounds(bounds Bounds) {
	bounds.
		AddPlane(s.PlanetModelField, &s.plane1.Plane, s.plane2, s.plane4).
		AddPlane(s.PlanetModelField, &s.plane2.Plane, s.plane3, s.plane1).
		AddPlane(s.PlanetModelField, &s.plane3.Plane, s.plane4, s.plane2).
		AddPlane(s.PlanetModelField, &s.plane4.Plane, s.plane1, s.plane3).
		AddPoint(s.point1).
		AddPoint(s.point2).
		AddPoint(s.point3).
		AddPoint(s.point4)
}

// GetRelationship computes the spatial relationship with the given shape.
//
// Port of GeoS2Shape.getRelationship via GeoBaseAreaShape.getRelationship.
func (s *GeoS2ShapeImpl) GetRelationship(path GeoShape) int {
	return geoAreaGetRelationship(
		func(x, y, z float64) bool { return s.IsWithin(x, y, z) },
		s.edgePoints,
		func(_ GeoShape) bool { return false },
		path,
	)
}
