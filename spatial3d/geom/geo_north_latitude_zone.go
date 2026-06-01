// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoNorthLatitudeZoneFields holds all computed fields for GeoNorthLatitudeZone.
type geoNorthLatitudeZoneFields struct {
	bottomLat           float64
	cosBottomLat        float64
	bottomPlane         *SidedPlane
	interiorPoint       *GeoPoint
	bottomBoundaryPoint *GeoPoint
	edgePoints          []*GeoPoint
}

// northLatZonePlanePoints is the empty notable-points set for north latitude zone.
var northLatZonePlanePoints = []*GeoPoint{}

// NewGeoNorthLatitudeZone constructs a GeoNorthLatitudeZone — the set of all
// points north of bottomLat.
//
// Port of org.apache.lucene.spatial3d.geom.GeoNorthLatitudeZone(PlanetModel,double).
func NewGeoNorthLatitudeZone(pm *PlanetModel, bottomLat float64) *GeoNorthLatitudeZone {
	sinBottomLat := math.Sin(bottomLat)
	cosBottomLat := math.Cos(bottomLat)

	middleLat := (math.Pi*0.5 + bottomLat) * 0.5
	sinMiddleLat := math.Sin(middleLat)
	cosMiddleLat := math.Sqrt(1.0 - sinMiddleLat*sinMiddleLat)
	interiorPoint := NewGeoPointTrig(pm, sinMiddleLat, 0.0, cosMiddleLat, 1.0)
	bottomBoundaryPoint := NewGeoPointTrig(pm, sinBottomLat, 0.0, cosBottomLat, 1.0)

	bottomPlane := NewSidedPlaneHorizontal(&interiorPoint.Vector, pm, sinBottomLat)

	z := &GeoNorthLatitudeZone{GeoBaseBBox: makeBBox(pm)}
	z.f = geoNorthLatitudeZoneFields{
		bottomLat:           bottomLat,
		cosBottomLat:        cosBottomLat,
		bottomPlane:         bottomPlane,
		interiorPoint:       interiorPoint,
		bottomBoundaryPoint: bottomBoundaryPoint,
		edgePoints:          []*GeoPoint{bottomBoundaryPoint},
	}
	return z
}

// IsWithin reports whether (x,y,z) is north of the bottom latitude.
//
// Port of GeoNorthLatitudeZone.isWithin.
func (z *GeoNorthLatitudeZone) IsWithin(x, y, zz float64) bool {
	return z.f.bottomPlane.IsWithin(x, y, zz)
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoNorthLatitudeZone.getRadius.
func (z *GeoNorthLatitudeZone) GetRadius() float64 {
	if z.f.bottomLat < 0.0 {
		return math.Pi
	}
	return z.f.cosBottomLat * math.Pi
}

// GetCenter returns the interior point.
func (z *GeoNorthLatitudeZone) GetCenter() *GeoPoint { return z.f.interiorPoint }

// GetEdgePoints returns the bottom boundary point.
func (z *GeoNorthLatitudeZone) GetEdgePoints() []*GeoPoint { return z.f.edgePoints }

// Expand returns a bbox expanded by the given angle.
//
// Port of GeoNorthLatitudeZone.expand.
func (z *GeoNorthLatitudeZone) Expand(angle float64) GeoBBox {
	newBottomLat := z.f.bottomLat - angle
	bbox, err := MakeGeoBBox(z.PlanetModelField, math.Pi*0.5, newBottomLat, -math.Pi, math.Pi)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the north latitude zone.
//
// Port of GeoNorthLatitudeZone.intersects(Plane,GeoPoint[],Membership...).
func (z *GeoNorthLatitudeZone) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	return p.Intersects(z.PlanetModelField, &z.f.bottomPlane.Plane, notablePoints, northLatZonePlanePoints, bounds)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoNorthLatitudeZone.intersects(GeoShape).
func (z *GeoNorthLatitudeZone) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&z.f.bottomPlane.Plane, northLatZonePlanePoints)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (z *GeoNorthLatitudeZone) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(z.IsWithin, z.f.edgePoints, z.intersectsShape, geoShape)
}

// GetBounds accumulates the north latitude zone's bounding information.
//
// Port of GeoNorthLatitudeZone.getBounds.
func (z *GeoNorthLatitudeZone) GetBounds(bounds Bounds) {
	geoBaseGetBounds(z, z.PlanetModelField, bounds)
	bounds.AddHorizontalPlane(z.PlanetModelField, z.f.bottomLat, &z.f.bottomPlane.Plane)
}

// String returns a debug representation.
func (z *GeoNorthLatitudeZone) String() string {
	return fmt.Sprintf("GeoNorthLatitudeZone: {planetmodel=%v, bottomlat=%g(%g)}",
		z.PlanetModelField, z.f.bottomLat, z.f.bottomLat*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoNorthLatitudeZone)(nil)
	_ GeoShape = (*GeoNorthLatitudeZone)(nil)
)
