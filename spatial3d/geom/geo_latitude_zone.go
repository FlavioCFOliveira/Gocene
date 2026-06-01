// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoLatitudeZoneFields holds all computed fields for GeoLatitudeZone.
type geoLatitudeZoneFields struct {
	topLat              float64
	bottomLat           float64
	cosTopLat           float64
	cosBottomLat        float64
	topPlane            *SidedPlane
	bottomPlane         *SidedPlane
	interiorPoint       *GeoPoint
	topBoundaryPoint    *GeoPoint
	bottomBoundaryPoint *GeoPoint
	edgePoints          []*GeoPoint
}

// latZonePlanePoints is the empty notable-points set for latitude zone planes.
var latZonePlanePoints = []*GeoPoint{}

// NewGeoLatitudeZone constructs a GeoLatitudeZone bounded between topLat and
// bottomLat, covering all longitudes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoLatitudeZone(PlanetModel,double,double).
func NewGeoLatitudeZone(pm *PlanetModel, topLat, bottomLat float64) *GeoLatitudeZone {
	sinTopLat := math.Sin(topLat)
	sinBottomLat := math.Sin(bottomLat)
	cosTopLat := math.Cos(topLat)
	cosBottomLat := math.Cos(bottomLat)

	// Interior point at middle latitude, lon=0.
	middleLat := (topLat + bottomLat) * 0.5
	sinMiddleLat := math.Sin(middleLat)
	cosMiddleLat := math.Sqrt(1.0 - sinMiddleLat*sinMiddleLat)
	// NewGeoPointTrig mirrors GeoPoint(pm, sinLat, sinLon=0, cosLat, cosLon=1).
	interiorPoint := NewGeoPointTrig(pm, sinMiddleLat, 0.0, cosMiddleLat, 1.0)
	topBoundaryPoint := NewGeoPointTrig(pm, sinTopLat, 0.0, cosTopLat, 1.0)
	bottomBoundaryPoint := NewGeoPointTrig(pm, sinBottomLat, 0.0, cosBottomLat, 1.0)

	topPlane := NewSidedPlaneHorizontal(&interiorPoint.Vector, pm, sinTopLat)
	bottomPlane := NewSidedPlaneHorizontal(&interiorPoint.Vector, pm, sinBottomLat)

	z := &GeoLatitudeZone{GeoBaseBBox: makeBBox(pm)}
	z.f = geoLatitudeZoneFields{
		topLat:              topLat,
		bottomLat:           bottomLat,
		cosTopLat:           cosTopLat,
		cosBottomLat:        cosBottomLat,
		topPlane:            topPlane,
		bottomPlane:         bottomPlane,
		interiorPoint:       interiorPoint,
		topBoundaryPoint:    topBoundaryPoint,
		bottomBoundaryPoint: bottomBoundaryPoint,
		edgePoints:          []*GeoPoint{topBoundaryPoint, bottomBoundaryPoint},
	}
	return z
}

// IsWithin reports whether (x,y,z) is inside the latitude zone.
//
// Port of GeoLatitudeZone.isWithin.
func (z *GeoLatitudeZone) IsWithin(x, y, zz float64) bool {
	return z.f.topPlane.IsWithin(x, y, zz) && z.f.bottomPlane.IsWithin(x, y, zz)
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoLatitudeZone.getRadius.
func (z *GeoLatitudeZone) GetRadius() float64 {
	if z.f.topLat > 0.0 && z.f.bottomLat < 0.0 {
		return math.Pi
	}
	maxCosLat := z.f.cosTopLat
	if maxCosLat < z.f.cosBottomLat {
		maxCosLat = z.f.cosBottomLat
	}
	return maxCosLat * math.Pi
}

// GetCenter returns the interior point.
func (z *GeoLatitudeZone) GetCenter() *GeoPoint { return z.f.interiorPoint }

// GetEdgePoints returns the top and bottom boundary points.
func (z *GeoLatitudeZone) GetEdgePoints() []*GeoPoint { return z.f.edgePoints }

// Expand returns a bbox expanded by the given angle.
//
// Port of GeoLatitudeZone.expand.
func (z *GeoLatitudeZone) Expand(angle float64) GeoBBox {
	newTopLat := z.f.topLat + angle
	newBottomLat := z.f.bottomLat - angle
	bbox, err := MakeGeoBBox(z.PlanetModelField, newTopLat, newBottomLat, -math.Pi, math.Pi)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the latitude zone.
//
// Port of GeoLatitudeZone.intersects(Plane,GeoPoint[],Membership...).
func (z *GeoLatitudeZone) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := z.PlanetModelField
	return p.Intersects(pm, &z.f.topPlane.Plane, notablePoints, latZonePlanePoints, bounds, z.f.bottomPlane) ||
		p.Intersects(pm, &z.f.bottomPlane.Plane, notablePoints, latZonePlanePoints, bounds, z.f.topPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoLatitudeZone.intersects(GeoShape).
func (z *GeoLatitudeZone) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&z.f.topPlane.Plane, latZonePlanePoints, z.f.bottomPlane) ||
		geoShape.Intersects(&z.f.bottomPlane.Plane, latZonePlanePoints, z.f.topPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (z *GeoLatitudeZone) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(z.IsWithin, z.f.edgePoints, z.intersectsShape, geoShape)
}

// GetBounds accumulates the latitude zone's bounding information.
//
// Port of GeoLatitudeZone.getBounds.
func (z *GeoLatitudeZone) GetBounds(bounds Bounds) {
	geoBaseGetBounds(z, z.PlanetModelField, bounds)
	pm := z.PlanetModelField
	bounds.
		NoLongitudeBound().
		AddHorizontalPlane(pm, z.f.topLat, &z.f.topPlane.Plane).
		AddHorizontalPlane(pm, z.f.bottomLat, &z.f.bottomPlane.Plane)
}

// String returns a debug representation.
func (z *GeoLatitudeZone) String() string {
	return fmt.Sprintf("GeoLatitudeZone: {planetmodel=%v, toplat=%g(%g), bottomlat=%g(%g)}",
		z.PlanetModelField,
		z.f.topLat, z.f.topLat*180.0/math.Pi,
		z.f.bottomLat, z.f.bottomLat*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoLatitudeZone)(nil)
	_ GeoShape = (*GeoLatitudeZone)(nil)
)
