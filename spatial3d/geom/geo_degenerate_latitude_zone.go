// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoDegenerateLatitudeZoneFields holds all computed fields for GeoDegenerateLatitudeZone.
type geoDegenerateLatitudeZoneFields struct {
	latitude      float64
	sinLatitude   float64
	plane         *Plane
	interiorPoint *GeoPoint
	edgePoints    []*GeoPoint
}

// degLatZonePlanePoints is the empty notable-points set for degenerate latitude zone.
var degLatZonePlanePoints = []*GeoPoint{}

// NewGeoDegenerateLatitudeZone constructs a GeoDegenerateLatitudeZone — the
// full latitude circle at the given latitude.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateLatitudeZone(PlanetModel,double).
func NewGeoDegenerateLatitudeZone(pm *PlanetModel, latitude float64) *GeoDegenerateLatitudeZone {
	sinLatitude := math.Sin(latitude)
	cosLatitude := math.Cos(latitude)

	plane := NewPlaneHorizontal(pm, sinLatitude)
	// interiorPoint: lon=0.
	interiorPoint := NewGeoPointTrig(pm, sinLatitude, 0.0, cosLatitude, 1.0)

	z := &GeoDegenerateLatitudeZone{GeoBaseBBox: makeBBox(pm)}
	z.f = geoDegenerateLatitudeZoneFields{
		latitude:      latitude,
		sinLatitude:   sinLatitude,
		plane:         plane,
		interiorPoint: interiorPoint,
		edgePoints:    []*GeoPoint{interiorPoint},
	}
	return z
}

// IsWithin reports whether (x,y,z) is on the degenerate latitude zone.
//
// Port of GeoDegenerateLatitudeZone.isWithin — uses a direct z-coordinate check
// against sinLatitude (within 1e-10) as in the Java source.
func (z *GeoDegenerateLatitudeZone) IsWithin(x, y, zz float64) bool {
	return math.Abs(zz-z.f.sinLatitude) < 1e-10
}

// GetRadius returns PI.
//
// Port of GeoDegenerateLatitudeZone.getRadius.
func (z *GeoDegenerateLatitudeZone) GetRadius() float64 { return math.Pi }

// GetCenter returns the interior point.
func (z *GeoDegenerateLatitudeZone) GetCenter() *GeoPoint { return z.f.interiorPoint }

// GetEdgePoints returns the interior point.
func (z *GeoDegenerateLatitudeZone) GetEdgePoints() []*GeoPoint { return z.f.edgePoints }

// Expand returns a bbox expanded by the given angle.
//
// Port of GeoDegenerateLatitudeZone.expand.
func (z *GeoDegenerateLatitudeZone) Expand(angle float64) GeoBBox {
	newTopLat := z.f.latitude + angle
	newBottomLat := z.f.latitude - angle
	bbox, err := MakeGeoBBox(z.PlanetModelField, newTopLat, newBottomLat, -math.Pi, math.Pi)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the degenerate latitude zone.
//
// Port of GeoDegenerateLatitudeZone.intersects(Plane,GeoPoint[],Membership...).
func (z *GeoDegenerateLatitudeZone) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	return p.Intersects(z.PlanetModelField, z.f.plane, notablePoints, degLatZonePlanePoints, bounds)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoDegenerateLatitudeZone.intersects(GeoShape).
func (z *GeoDegenerateLatitudeZone) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(z.f.plane, degLatZonePlanePoints)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoDegenerateLatitudeZone.getRelationship.
func (z *GeoDegenerateLatitudeZone) GetRelationship(geoShape GeoShape) int {
	if z.intersectsShape(geoShape) {
		return RelOverlaps
	}
	if mem, ok := geoShape.(Membership); ok {
		if mem.IsWithin(z.f.interiorPoint.X, z.f.interiorPoint.Y, z.f.interiorPoint.Z) {
			return RelContains
		}
	}
	return RelDisjoint
}

// GetBounds accumulates the degenerate latitude zone's bounding information.
//
// Port of GeoDegenerateLatitudeZone.getBounds.
func (z *GeoDegenerateLatitudeZone) GetBounds(bounds Bounds) {
	geoBaseGetBounds(z, z.PlanetModelField, bounds)
	bounds.NoLongitudeBound().AddHorizontalPlane(z.PlanetModelField, z.f.latitude, z.f.plane)
}

// String returns a debug representation.
func (z *GeoDegenerateLatitudeZone) String() string {
	return fmt.Sprintf("GeoDegenerateLatitudeZone: {planetmodel=%v, lat=%g(%g)}",
		z.PlanetModelField, z.f.latitude, z.f.latitude*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoDegenerateLatitudeZone)(nil)
	_ GeoShape = (*GeoDegenerateLatitudeZone)(nil)
)
