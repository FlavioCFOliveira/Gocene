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
//
// Port of org.apache.lucene.spatial3d.geom.GeoNorthRectangle.
type GeoNorthRectangle struct{ GeoBaseBBox }

// GeoSouthRectangle is a rectangle extending to the south pole.
//
// Port of org.apache.lucene.spatial3d.geom.GeoSouthRectangle.
type GeoSouthRectangle struct{ GeoBaseBBox }

// GeoWideRectangle is a rectangle that spans more than π in longitude.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideRectangle.
type GeoWideRectangle struct{ GeoBaseBBox }

// GeoWideNorthRectangle is a wide rectangle extending to the north pole.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideNorthRectangle.
type GeoWideNorthRectangle struct{ GeoBaseBBox }

// GeoWideSouthRectangle is a wide rectangle extending to the south pole.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideSouthRectangle.
type GeoWideSouthRectangle struct{ GeoBaseBBox }

// GeoWorld represents the whole sphere.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWorld.
type GeoWorld struct{ GeoBaseBBox }

// IsWithin always returns true for the world shape.
func (w *GeoWorld) IsWithin(_, _, _ float64) bool { return true }

// GetRelationship returns RelContains for the world shape.
func (w *GeoWorld) GetRelationship(_ GeoShape) int { return RelContains }

// GeoLongitudeSlice is a slice bounded by two meridians.
//
// Port of org.apache.lucene.spatial3d.geom.GeoLongitudeSlice.
type GeoLongitudeSlice struct{ GeoBaseBBox }

// GeoWideLongitudeSlice is a longitude slice spanning more than π.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideLongitudeSlice.
type GeoWideLongitudeSlice struct{ GeoBaseBBox }

// GeoLatitudeZone is a band between two latitude parallels.
//
// Port of org.apache.lucene.spatial3d.geom.GeoLatitudeZone.
type GeoLatitudeZone struct{ GeoBaseBBox }

// GeoNorthLatitudeZone is a band north of a latitude.
//
// Port of org.apache.lucene.spatial3d.geom.GeoNorthLatitudeZone.
type GeoNorthLatitudeZone struct{ GeoBaseBBox }

// GeoSouthLatitudeZone is a band south of a latitude.
//
// Port of org.apache.lucene.spatial3d.geom.GeoSouthLatitudeZone.
type GeoSouthLatitudeZone struct{ GeoBaseBBox }

// GeoDegenerateHorizontalLine is a degenerate horizontal (latitude) line.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateHorizontalLine.
type GeoDegenerateHorizontalLine struct{ GeoBaseBBox }

// GeoWideDegenerateHorizontalLine is a wide degenerate horizontal line.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideDegenerateHorizontalLine.
type GeoWideDegenerateHorizontalLine struct{ GeoBaseBBox }

// GeoDegenerateLongitudeSlice is a degenerate longitude slice (a meridian).
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateLongitudeSlice.
type GeoDegenerateLongitudeSlice struct{ GeoBaseBBox }

// GeoDegenerateLatitudeZone is a degenerate latitude zone (a point).
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateLatitudeZone.
type GeoDegenerateLatitudeZone struct{ GeoBaseBBox }

// GeoDegenerateVerticalLine is a vertical degenerate line.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateVerticalLine.
type GeoDegenerateVerticalLine struct{ GeoBaseBBox }

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
//
// Port of org.apache.lucene.spatial3d.geom.GeoExactCircle.
type GeoExactCircle struct{ GeoBaseCircle }

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

// GeoS2ShapeImpl is an S2-backed shape.
//
// Port of org.apache.lucene.spatial3d.geom.GeoS2Shape.
type GeoS2ShapeImpl struct{ GeoBaseMembershipShape }
