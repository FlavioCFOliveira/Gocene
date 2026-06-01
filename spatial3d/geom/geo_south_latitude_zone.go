// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoSouthLatitudeZoneFields holds all computed fields for GeoSouthLatitudeZone.
type geoSouthLatitudeZoneFields struct {
	topLat           float64
	cosTopLat        float64
	topPlane         *SidedPlane
	interiorPoint    *GeoPoint
	topBoundaryPoint *GeoPoint
	edgePoints       []*GeoPoint
}

// southLatZonePlanePoints is the empty notable-points set for south latitude zone.
var southLatZonePlanePoints = []*GeoPoint{}

// NewGeoSouthLatitudeZone constructs a GeoSouthLatitudeZone — the set of all
// points south of topLat.
//
// Port of org.apache.lucene.spatial3d.geom.GeoSouthLatitudeZone(PlanetModel,double).
func NewGeoSouthLatitudeZone(pm *PlanetModel, topLat float64) *GeoSouthLatitudeZone {
	sinTopLat := math.Sin(topLat)
	cosTopLat := math.Cos(topLat)

	middleLat := (topLat - math.Pi*0.5) * 0.5
	sinMiddleLat := math.Sin(middleLat)
	cosMiddleLat := math.Sqrt(1.0 - sinMiddleLat*sinMiddleLat)
	interiorPoint := NewGeoPointTrig(pm, sinMiddleLat, 0.0, cosMiddleLat, 1.0)
	topBoundaryPoint := NewGeoPointTrig(pm, sinTopLat, 0.0, cosTopLat, 1.0)

	topPlane := NewSidedPlaneHorizontal(&interiorPoint.Vector, pm, sinTopLat)

	z := &GeoSouthLatitudeZone{GeoBaseBBox: makeBBox(pm)}
	z.f = geoSouthLatitudeZoneFields{
		topLat:           topLat,
		cosTopLat:        cosTopLat,
		topPlane:         topPlane,
		interiorPoint:    interiorPoint,
		topBoundaryPoint: topBoundaryPoint,
		edgePoints:       []*GeoPoint{topBoundaryPoint},
	}
	return z
}

// IsWithin reports whether (x,y,z) is south of the top latitude.
//
// Port of GeoSouthLatitudeZone.isWithin.
func (z *GeoSouthLatitudeZone) IsWithin(x, y, zz float64) bool {
	return z.f.topPlane.IsWithin(x, y, zz)
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoSouthLatitudeZone.getRadius.
func (z *GeoSouthLatitudeZone) GetRadius() float64 {
	if z.f.topLat > 0.0 {
		return math.Pi
	}
	return z.f.cosTopLat * math.Pi
}

// GetCenter returns the interior point.
func (z *GeoSouthLatitudeZone) GetCenter() *GeoPoint { return z.f.interiorPoint }

// GetEdgePoints returns the top boundary point.
func (z *GeoSouthLatitudeZone) GetEdgePoints() []*GeoPoint { return z.f.edgePoints }

// Expand returns a bbox expanded by the given angle.
//
// Port of GeoSouthLatitudeZone.expand.
func (z *GeoSouthLatitudeZone) Expand(angle float64) GeoBBox {
	newTopLat := z.f.topLat + angle
	bbox, err := MakeGeoBBox(z.PlanetModelField, newTopLat, -math.Pi*0.5, -math.Pi, math.Pi)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the south latitude zone.
//
// Port of GeoSouthLatitudeZone.intersects(Plane,GeoPoint[],Membership...).
func (z *GeoSouthLatitudeZone) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	return p.Intersects(z.PlanetModelField, &z.f.topPlane.Plane, notablePoints, southLatZonePlanePoints, bounds)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoSouthLatitudeZone.intersects(GeoShape).
func (z *GeoSouthLatitudeZone) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&z.f.topPlane.Plane, southLatZonePlanePoints)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (z *GeoSouthLatitudeZone) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(z.IsWithin, z.f.edgePoints, z.intersectsShape, geoShape)
}

// GetBounds accumulates the south latitude zone's bounding information.
//
// Port of GeoSouthLatitudeZone.getBounds.
func (z *GeoSouthLatitudeZone) GetBounds(bounds Bounds) {
	geoBaseGetBounds(z, z.PlanetModelField, bounds)
	bounds.AddHorizontalPlane(z.PlanetModelField, z.f.topLat, &z.f.topPlane.Plane)
}

// String returns a debug representation.
func (z *GeoSouthLatitudeZone) String() string {
	return fmt.Sprintf("GeoSouthLatitudeZone: {planetmodel=%v, toplat=%g(%g)}",
		z.PlanetModelField, z.f.topLat, z.f.topLat*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoSouthLatitudeZone)(nil)
	_ GeoShape = (*GeoSouthLatitudeZone)(nil)
)
