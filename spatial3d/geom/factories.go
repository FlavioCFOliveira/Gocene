// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
)

// ---------------------------------------------------------------------------
// Internal construction helpers — avoid repetitive deep struct literals.
// ---------------------------------------------------------------------------

func makeBase(pm *PlanetModel) GeoBaseShape {
	return GeoBaseShape{BasePlanetObject: BasePlanetObject{planetModel: pm}}
}

func makeMem(pm *PlanetModel) GeoBaseMembershipShape {
	return GeoBaseMembershipShape{GeoBaseShape: makeBase(pm)}
}

func makeArea(pm *PlanetModel) GeoBaseAreaShape {
	return GeoBaseAreaShape{GeoBaseMembershipShape: makeMem(pm)}
}

func makeBBox(pm *PlanetModel) GeoBaseBBox {
	return GeoBaseBBox{GeoBaseAreaShape: makeArea(pm)}
}

func makeCircle(pm *PlanetModel, radius float64) GeoBaseCircle {
	return GeoBaseCircle{GeoBaseMembershipShape: makeMem(pm), radius: radius}
}

func makePolygon(pm *PlanetModel) GeoBasePolygon {
	return GeoBasePolygon{GeoBaseMembershipShape: makeMem(pm)}
}

func makePath(pm *PlanetModel, cutoffAngle float64) GeoBasePath {
	return GeoBasePath{GeoBaseMembershipShape: makeMem(pm), cutoffAngle: cutoffAngle}
}

// ---------------------------------------------------------------------------
// GeoCircleFactory
//
// Port of org.apache.lucene.spatial3d.geom.GeoCircleFactory.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// GeoCircleFactory
//
// Port of org.apache.lucene.spatial3d.geom.GeoCircleFactory.
// ---------------------------------------------------------------------------

// MakeGeoCircle creates a GeoCircle from a center lat/lon and cutoff angle.
// A cutoff angle below the minimum angular resolution yields a degenerate point.
//
// Port of org.apache.lucene.spatial3d.geom.GeoCircleFactory.makeGeoCircle.
func MakeGeoCircle(pm *PlanetModel, latitude, longitude, cutoffAngle float64) (GeoCircle, error) {
	if cutoffAngle < MinimumAngularResolution {
		return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, latitude, longitude)), nil
	}
	return NewGeoStandardCircle(pm, latitude, longitude, cutoffAngle)
}

// MakeGeoExactCircle creates a GeoExactCircle from a center lat/lon, surface
// radius, and accuracy. Returns an error if the parameters are out of range.
//
// Port of org.apache.lucene.spatial3d.geom.GeoCircleFactory.makeExactGeoCircle.
func MakeGeoExactCircle(pm *PlanetModel, lat, lon, radius, accuracy float64) (GeoCircle, error) {
	return NewGeoExactCircle(pm, lat, lon, radius, accuracy)
}

// ---------------------------------------------------------------------------
// GeoPathFactory
//
// Port of org.apache.lucene.spatial3d.geom.GeoPathFactory.
// ---------------------------------------------------------------------------

// MakeGeoPath creates a GeoPath from a cutoff angle and a list of waypoints.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPathFactory.makeGeoPath.
//
// Waypoints define the great-circle path: consecutive pairs produce segments
// bounded by cutoffAngle (angular half-width in radians). The final segment
// connects the last waypoint back to the first if the path is closed. A path
// with fewer than 2 waypoints produces a zero-width degenerate path at the
// single point.
func MakeGeoPath(pm *PlanetModel, cutoffAngle float64, waypoints []*GeoPoint) GeoPath {
	path := &GeoStandardPath{GeoBasePath: makePath(pm, cutoffAngle)}

	// Filter nil waypoints.
	pts := make([]*GeoPoint, 0, len(waypoints))
	for _, p := range waypoints {
		if p != nil {
			pts = append(pts, p)
		}
	}

	if len(pts) == 0 {
		return path
	}

	if len(pts) == 1 {
		// Single point: degenerate path at that location.
		path.center = pts[0]
		return path
	}

	// Store the waypoint sequence for path segment construction.
	path.waypoints = pts
	path.closed = false // open path by default

	return path
}

// ---------------------------------------------------------------------------
// GeoPolygonFactory — stub
//
// Port of org.apache.lucene.spatial3d.geom.GeoPolygonFactory.
// Full polygon-building algorithm deferred to #2693.
// ---------------------------------------------------------------------------

// MakeGeoConvexPolygon creates a GeoConvexPolygon from an ordered point list,
// chosen so that any point adjacent to a segment provides an interior
// measurement. Use this only when the polygon is known to be convex with an
// extent no larger than PI.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPolygonFactory.makeGeoConvexPolygon.
func MakeGeoConvexPolygon(pm *PlanetModel, pointList []*GeoPoint) (GeoPolygon, error) {
	return NewGeoConvexPolygon(pm, pointList, nil)
}

// MakeGeoConvexPolygonWithHoles is the holes-aware form of MakeGeoConvexPolygon.
//
// Port of GeoPolygonFactory.makeGeoConvexPolygon(PlanetModel,List,List).
func MakeGeoConvexPolygonWithHoles(pm *PlanetModel, pointList []*GeoPoint, holes []GeoPolygon) (GeoPolygon, error) {
	return NewGeoConvexPolygon(pm, pointList, holes)
}

// MakeGeoConcavePolygon creates a GeoConcavePolygon from an ordered point list,
// chosen so that any point adjacent to a segment provides an exterior
// measurement. Use this only when the polygon is known to be concave with an
// extent larger than PI.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPolygonFactory.makeGeoConcavePolygon.
func MakeGeoConcavePolygon(pm *PlanetModel, pointList []*GeoPoint) (GeoPolygon, error) {
	return NewGeoConcavePolygon(pm, pointList, nil)
}

// MakeGeoConcavePolygonWithHoles is the holes-aware form of MakeGeoConcavePolygon.
//
// Port of GeoPolygonFactory.makeGeoConcavePolygon(PlanetModel,List,List).
func MakeGeoConcavePolygonWithHoles(pm *PlanetModel, pointList []*GeoPoint, holes []GeoPolygon) (GeoPolygon, error) {
	return NewGeoConcavePolygon(pm, pointList, holes)
}

// MakeGeoPolygon creates a GeoPolygon from a list of GeoPoints.
//
// Simplified port of org.apache.lucene.spatial3d.geom.GeoPolygonFactory.makeGeoPolygon.
// First attempts to build a GeoConvexPolygon; if the number of points exceeds
// the convex-polygon bound (extreme extent >PI in any dimension), falls back to
// GeoConcavePolygon. Complex self-intersecting polygons that require the full
// tiling algorithm (GeoComplexPolygon) are not yet supported and will fall back
// to GeoConcavePolygon as a conservative approximation.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPolygonFactory.makeGeoPolygon.
func MakeGeoPolygon(pm *PlanetModel, pointList []*GeoPoint) (GeoPolygon, error) {
	return MakeGeoPolygonWithHoles(pm, pointList, nil)
}

// MakeGeoPolygonWithHoles creates a GeoPolygon with optional holes.
//
// Port of GeoPolygonFactory.makeGeoPolygon(PlanetModel, List<GeoPoint>, List<GeoPolygon>).
func MakeGeoPolygonWithHoles(pm *PlanetModel, pointList []*GeoPoint, holes []GeoPolygon) (GeoPolygon, error) {
	if len(pointList) < 3 {
		return nil, fmt.Errorf("geom: MakeGeoPolygon: need at least 3 points, got %d", len(pointList))
	}

	// Try convex first; if it fails (e.g. edge-plane conflicts on concave input), use concave.
	convex, err := NewGeoConvexPolygon(pm, pointList, holes)
	if err == nil {
		return convex, nil
	}
	// Fall back to concave polygon.
	return NewGeoConcavePolygon(pm, pointList, holes)
}

// ---------------------------------------------------------------------------
// GeoAreaFactory — stub
//
// Port of org.apache.lucene.spatial3d.geom.GeoAreaFactory.
// ---------------------------------------------------------------------------

// MakeGeoArea creates a GeoArea from XYZ bounds.
//
// Port of org.apache.lucene.spatial3d.geom.GeoAreaFactory.makeGeoArea.
// Deferred to #2693.
func MakeGeoArea(pm *PlanetModel, minX, maxX, minY, maxY, minZ, maxZ float64) GeoArea {
	return MakeXYZSolid(pm, minX, maxX, minY, maxY, minZ, maxZ)
}

// ---------------------------------------------------------------------------
// GeoPointShapeFactory
//
// Port of org.apache.lucene.spatial3d.geom.GeoPointShapeFactory.
// ---------------------------------------------------------------------------

// MakeGeoPointShape creates a GeoPointShape from a lat/lon.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPointShapeFactory.makeGeoPointShape.
func MakeGeoPointShape(pm *PlanetModel, lat, lon float64) GeoPointShape {
	return &GeoPointShapeImpl{
		GeoBaseBBox: makeBBox(pm),
		point:       NewGeoPointLatLon(pm, lat, lon),
	}
}

// ---------------------------------------------------------------------------
// GeoS2ShapeFactory
//
// Port of org.apache.lucene.spatial3d.geom.GeoS2ShapeFactory.
// ---------------------------------------------------------------------------

// MakeGeoS2Shape creates a convex 4-sided polygon from four points in CCW order.
//
// Port of org.apache.lucene.spatial3d.geom.GeoS2ShapeFactory.makeGeoS2Shape.
func MakeGeoS2Shape(pm *PlanetModel, point1, point2, point3, point4 *GeoPoint) GeoS2Shape {
	return NewGeoS2Shape(pm, point1, point2, point3, point4)
}
