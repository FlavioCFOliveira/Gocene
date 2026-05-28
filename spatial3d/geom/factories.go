// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// ---------------------------------------------------------------------------
// Internal construction helpers — avoid repetitive deep struct literals.
// ---------------------------------------------------------------------------

func makeBase(pm *PlanetModel) GeoBaseShape {
	return GeoBaseShape{BasePlanetObject: BasePlanetObject{PlanetModelField: pm}}
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
// GeoBBoxFactory
//
// Port of org.apache.lucene.spatial3d.geom.GeoBBoxFactory.
// ---------------------------------------------------------------------------

// minWideExtent is the longitude span at or above which a rectangle is "wide".
//
// Port of GeoWideRectangle.MIN_WIDE_EXTENT.
const minWideExtent = math.Pi - MinimumAngularResolution

// errBBoxVariantUnsupported marks a GeoBBox sub-shape whose full implementation
// has not yet been ported (wide rectangles, latitude/longitude zones,
// degenerate lines). Their isWithin/getBounds engine is deferred so the factory
// reports a clear error rather than returning a silently non-matching stub.
var errBBoxVariantUnsupported = fmt.Errorf("geom: this GeoBBox variant is not yet implemented in Gocene")

// MakeGeoBBox creates the appropriate GeoBBox for the given lat/lon bounds.
//
// The standard rectangle case (extent < PI, not pole-touching, not a full
// longitude band) plus the whole-world and degenerate-point cases are fully
// implemented. The wide-rectangle, latitude-zone, longitude-slice and
// degenerate-line variants are not yet ported and return
// errBBoxVariantUnsupported.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBBoxFactory.makeGeoBBox.
func MakeGeoBBox(pm *PlanetModel, topLat, bottomLat, leftLon, rightLon float64) (GeoBBox, error) {
	halfPI := math.Pi * 0.5
	if topLat > halfPI {
		topLat = halfPI
	}
	if bottomLat < -halfPI {
		bottomLat = -halfPI
	}
	if leftLon < -math.Pi {
		leftLon = -math.Pi
	}
	if rightLon > math.Pi {
		rightLon = math.Pi
	}
	if (longitudesEqual(leftLon, -math.Pi) && longitudesEqual(rightLon, math.Pi)) ||
		(longitudesEqual(rightLon, -math.Pi) && longitudesEqual(leftLon, math.Pi)) {
		if isNorthPole(topLat) && isSouthPole(bottomLat) {
			return &GeoWorld{GeoBaseBBox: makeBBox(pm)}, nil
		}
		if latitudesEqual(topLat, bottomLat) {
			if isNorthPole(topLat) {
				return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, topLat, 0.0)), nil
			}
			if isSouthPole(bottomLat) {
				return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, bottomLat, 0.0)), nil
			}
		}
		// Latitude-zone variants not yet ported.
		return nil, errBBoxVariantUnsupported
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += math.Pi * 2.0
	}
	if isNorthPole(topLat) && isSouthPole(bottomLat) {
		// Longitude-slice variants not yet ported.
		return nil, errBBoxVariantUnsupported
	}
	if longitudesEqual(leftLon, rightLon) {
		if latitudesEqual(topLat, bottomLat) {
			return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, topLat, leftLon)), nil
		}
		// Degenerate vertical line not yet ported.
		return nil, errBBoxVariantUnsupported
	}
	if extent >= minWideExtent {
		// Wide-rectangle variants not yet ported.
		return nil, errBBoxVariantUnsupported
	}
	if latitudesEqual(topLat, bottomLat) {
		if isNorthPole(topLat) {
			return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, topLat, 0.0)), nil
		}
		if isSouthPole(bottomLat) {
			return NewGeoDegeneratePoint(pm, NewGeoPointModel(pm, bottomLat, 0.0)), nil
		}
		// Degenerate horizontal line not yet ported.
		return nil, errBBoxVariantUnsupported
	}
	if isNorthPole(topLat) || isSouthPole(bottomLat) {
		// North/south rectangle variants not yet ported.
		return nil, errBBoxVariantUnsupported
	}
	return NewGeoRectangle(pm, topLat, bottomLat, leftLon, rightLon)
}

// longitudesEqual matches GeoBBoxFactory.longitudesEquals.
func longitudesEqual(a, b float64) bool { return math.Abs(a-b) < MinimumAngularResolution }

// latitudesEqual matches GeoBBoxFactory.latitudesEquals: equal angle or equal
// sin (to catch latitudes describing the same plane).
func latitudesEqual(a, b float64) bool {
	return math.Abs(a-b) < MinimumAngularResolution ||
		math.Abs(math.Sin(a)-math.Sin(b)) < MinimumResolution
}

func isNorthPole(lat float64) bool { return latitudesEqual(lat, math.Pi*0.5) }
func isSouthPole(lat float64) bool { return latitudesEqual(lat, -math.Pi*0.5) }

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

// MakeGeoExactCircle creates a GeoExactCircle from a center lat/lon, radius (in metres),
// and accuracy. Full computation deferred to #2693.
//
// Port of org.apache.lucene.spatial3d.geom.GeoCircleFactory.makeExactGeoCircle.
func MakeGeoExactCircle(pm *PlanetModel, _, _, _, _ float64) GeoCircle {
	return &GeoExactCircle{GeoBaseCircle: makeCircle(pm, 0)}
}

// ---------------------------------------------------------------------------
// GeoPathFactory
//
// Port of org.apache.lucene.spatial3d.geom.GeoPathFactory.
// ---------------------------------------------------------------------------

// MakeGeoPath creates a GeoPath from a cutoff angle and a list of waypoints.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPathFactory.makeGeoPath.
// Full degenerate-path branching deferred to #2693.
func MakeGeoPath(pm *PlanetModel, cutoffAngle float64, _ []*GeoPoint) GeoPath {
	return &GeoStandardPath{GeoBasePath: makePath(pm, cutoffAngle)}
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

// MakeGeoPolygon creates a GeoPolygon from a list of GeoPoints, using winding
// order to decide siding.
//
// The general orientation-aware factory (GeoPolygonFactory.makeGeoPolygon),
// which tiles arbitrary, possibly self-spanning polygons, is not yet ported and
// returns errPolygonFactoryUnsupported. Callers that know their polygon is
// convex or concave should use MakeGeoConvexPolygon / MakeGeoConcavePolygon.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPolygonFactory.makeGeoPolygon.
func MakeGeoPolygon(_ *PlanetModel, _ []*GeoPoint) (GeoPolygon, error) {
	return nil, errPolygonFactoryUnsupported
}

// errPolygonFactoryUnsupported marks the general winding-order polygon factory
// as not yet ported.
var errPolygonFactoryUnsupported = fmt.Errorf("geom: GeoPolygonFactory.makeGeoPolygon (winding-order tiling) is not yet implemented in Gocene; use MakeGeoConvexPolygon or MakeGeoConcavePolygon")

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
// GeoS2ShapeFactory — stub
//
// Port of org.apache.lucene.spatial3d.geom.GeoS2ShapeFactory.
// ---------------------------------------------------------------------------

// MakeGeoS2Shape creates a GeoS2Shape from an S2-encoded cell ID.
//
// Port of org.apache.lucene.spatial3d.geom.GeoS2ShapeFactory.makeGeoS2Shape.
// Deferred to #2693.
func MakeGeoS2Shape(pm *PlanetModel, _ interface{}) GeoS2Shape {
	return &GeoS2ShapeImpl{GeoBaseMembershipShape: makeMem(pm)}
}
