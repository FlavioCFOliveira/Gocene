// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "math"

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

// MakeGeoBBox creates the appropriate GeoBBox for the given lat/lon bounds.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBBoxFactory.makeGeoBBox.
func MakeGeoBBox(pm *PlanetModel, topLat, bottomLat, leftLon, rightLon float64) GeoBBox {
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
	lonFull := longitudesEqual(leftLon, -math.Pi) && longitudesEqual(rightLon, math.Pi) ||
		longitudesEqual(rightLon, -math.Pi) && longitudesEqual(leftLon, math.Pi)
	if lonFull {
		if isNorthPole(topLat) && isSouthPole(bottomLat) {
			return &GeoWorld{GeoBaseBBox: makeBBox(pm)}
		}
		if latitudesEqual(topLat, bottomLat) {
			return &GeoDegenerateLatitudeZone{GeoBaseBBox: makeBBox(pm)}
		}
		if isNorthPole(topLat) {
			return &GeoNorthLatitudeZone{GeoBaseBBox: makeBBox(pm)}
		}
		if isSouthPole(bottomLat) {
			return &GeoSouthLatitudeZone{GeoBaseBBox: makeBBox(pm)}
		}
		return &GeoLatitudeZone{GeoBaseBBox: makeBBox(pm)}
	}
	// Remaining branching deferred to #2693; return a stub GeoRectangle.
	return &GeoRectangle{GeoBaseBBox: makeBBox(pm)}
}

func longitudesEqual(a, b float64) bool { return math.Abs(a-b) < MinimumResolution }
func latitudesEqual(a, b float64) bool  { return math.Abs(a-b) < MinimumResolution }
func isNorthPole(lat float64) bool      { return math.Abs(lat-math.Pi*0.5) < MinimumResolution }
func isSouthPole(lat float64) bool      { return math.Abs(lat+math.Pi*0.5) < MinimumResolution }

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

// MakeGeoPolygon creates a GeoPolygon from a list of GeoPoints.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPolygonFactory.makeGeoPolygon.
func MakeGeoPolygon(pm *PlanetModel, _ []*GeoPoint) GeoPolygon {
	return &GeoConvexPolygon{GeoBasePolygon: makePolygon(pm)}
}

// MakeGeoConcavePolygon creates a GeoConcavePolygon.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPolygonFactory.makeGeoConcavePolygon.
func MakeGeoConcavePolygon(pm *PlanetModel, _ []*GeoPoint) GeoPolygon {
	return &GeoConcavePolygon{GeoBasePolygon: makePolygon(pm)}
}

// MakeGeoConvexPolygon creates a GeoConvexPolygon.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPolygonFactory.makeGeoConvexPolygon.
func MakeGeoConvexPolygon(pm *PlanetModel, _ []*GeoPoint) GeoPolygon {
	return &GeoConvexPolygon{GeoBasePolygon: makePolygon(pm)}
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
