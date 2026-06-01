// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoDegenerateVerticalLineFields holds all computed fields for GeoDegenerateVerticalLine.
type geoDegenerateVerticalLineFields struct {
	topLat       float64
	bottomLat    float64
	longitude    float64
	uhc, lhc     *GeoPoint
	topPlane     *SidedPlane
	bottomPlane  *SidedPlane
	boundingPlane *SidedPlane
	plane        *Plane
	planePoints  []*GeoPoint
	centerPoint  *GeoPoint
	edgePoints   []*GeoPoint
}

// NewGeoDegenerateVerticalLine constructs a GeoDegenerateVerticalLine at the
// given longitude, between topLat and bottomLat.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateVerticalLine(PlanetModel,...).
func NewGeoDegenerateVerticalLine(pm *PlanetModel, topLat, bottomLat, longitude float64) (*GeoDegenerateVerticalLine, error) {
	if topLat > math.Pi*0.5 || topLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoDegenerateVerticalLine: top latitude out of range: %g", topLat)
	}
	if bottomLat > math.Pi*0.5 || bottomLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoDegenerateVerticalLine: bottom latitude out of range: %g", bottomLat)
	}
	if topLat < bottomLat {
		return nil, fmt.Errorf("geom: GeoDegenerateVerticalLine: top latitude less than bottom latitude")
	}
	if longitude < -math.Pi || longitude > math.Pi {
		return nil, fmt.Errorf("geom: GeoDegenerateVerticalLine: longitude out of range: %g", longitude)
	}

	sinTopLat, cosTopLat := math.Sin(topLat), math.Cos(topLat)
	sinBottomLat, cosBottomLat := math.Sin(bottomLat), math.Cos(bottomLat)
	sinLongitude, cosLongitude := math.Sin(longitude), math.Cos(longitude)

	uhc := NewGeoPointTrigLatLon(pm, sinTopLat, sinLongitude, cosTopLat, cosLongitude, topLat, longitude)
	lhc := NewGeoPointTrigLatLon(pm, sinBottomLat, sinLongitude, cosBottomLat, cosLongitude, bottomLat, longitude)

	// plane: cosLon*x + sinLon*y = 0 (the vertical plane at this longitude).
	plane := NewPlane(cosLongitude, sinLongitude, 0.0, 0.0)

	middleLat := (topLat + bottomLat) * 0.5
	sinMiddleLat := math.Sin(middleLat)
	cosMiddleLat := math.Cos(middleLat)
	centerPoint := NewGeoPointTrig(pm, sinMiddleLat, sinLongitude, cosMiddleLat, cosLongitude)

	topPlane := NewSidedPlaneHorizontal(&lhc.Vector, pm, sinTopLat)
	bottomPlane := NewSidedPlaneHorizontal(&uhc.Vector, pm, sinBottomLat)
	// boundingPlane: normal (-sinLon, cosLon, 0), D=0.
	boundingPlane := NewSidedPlane(&centerPoint.Vector, -sinLongitude, cosLongitude, 0.0, 0.0)

	if topPlane == nil || bottomPlane == nil || boundingPlane == nil {
		return nil, fmt.Errorf("geom: GeoDegenerateVerticalLine: degenerate bounding planes")
	}

	v := &GeoDegenerateVerticalLine{GeoBaseBBox: makeBBox(pm)}
	v.f = geoDegenerateVerticalLineFields{
		topLat:        topLat,
		bottomLat:     bottomLat,
		longitude:     longitude,
		uhc:           uhc,
		lhc:           lhc,
		topPlane:      topPlane,
		bottomPlane:   bottomPlane,
		boundingPlane: boundingPlane,
		plane:         plane,
		planePoints:   []*GeoPoint{uhc, lhc},
		centerPoint:   centerPoint,
		edgePoints:    []*GeoPoint{centerPoint},
	}
	return v, nil
}

// IsWithin reports whether (x,y,z) is on the degenerate vertical line.
//
// Port of GeoDegenerateVerticalLine.isWithin.
func (v *GeoDegenerateVerticalLine) IsWithin(x, y, z float64) bool {
	return v.f.plane.EvaluateIsZeroXYZ(x, y, z) &&
		v.f.boundingPlane.IsWithin(x, y, z) &&
		v.f.topPlane.IsWithin(x, y, z) &&
		v.f.bottomPlane.IsWithin(x, y, z)
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoDegenerateVerticalLine.getRadius.
func (v *GeoDegenerateVerticalLine) GetRadius() float64 {
	topAngle := v.f.centerPoint.ArcDistance(&v.f.uhc.Vector)
	bottomAngle := v.f.centerPoint.ArcDistance(&v.f.lhc.Vector)
	return math.Max(topAngle, bottomAngle)
}

// GetCenter returns the center point.
func (v *GeoDegenerateVerticalLine) GetCenter() *GeoPoint { return v.f.centerPoint }

// GetEdgePoints returns the center point.
func (v *GeoDegenerateVerticalLine) GetEdgePoints() []*GeoPoint { return v.f.edgePoints }

// Expand returns a bbox expanded by the given angle.
//
// Port of GeoDegenerateVerticalLine.expand.
func (v *GeoDegenerateVerticalLine) Expand(angle float64) GeoBBox {
	newTopLat := v.f.topLat + angle
	newBottomLat := v.f.bottomLat - angle
	newLeftLon := v.f.longitude - angle
	newRightLon := v.f.longitude + angle
	currentLonSpan := 2.0 * angle
	if currentLonSpan+2.0*angle >= math.Pi*2.0 {
		newLeftLon = -math.Pi
		newRightLon = math.Pi
	}
	bbox, err := MakeGeoBBox(v.PlanetModelField, newTopLat, newBottomLat, newLeftLon, newRightLon)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the degenerate vertical line.
//
// Port of GeoDegenerateVerticalLine.intersects(Plane,GeoPoint[],Membership...).
func (v *GeoDegenerateVerticalLine) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	return p.Intersects(v.PlanetModelField, v.f.plane, notablePoints, v.f.planePoints, bounds, v.f.boundingPlane, v.f.topPlane, v.f.bottomPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoDegenerateVerticalLine.intersects(GeoShape).
func (v *GeoDegenerateVerticalLine) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(v.f.plane, v.f.planePoints, v.f.boundingPlane, v.f.topPlane, v.f.bottomPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoDegenerateVerticalLine.getRelationship.
func (v *GeoDegenerateVerticalLine) GetRelationship(geoShape GeoShape) int {
	if v.intersectsShape(geoShape) {
		return RelOverlaps
	}
	if mem, ok := geoShape.(Membership); ok {
		if mem.IsWithin(v.f.centerPoint.X, v.f.centerPoint.Y, v.f.centerPoint.Z) {
			return RelContains
		}
	}
	return RelDisjoint
}

// GetBounds accumulates the degenerate vertical line's bounding information.
//
// Port of GeoDegenerateVerticalLine.getBounds.
func (v *GeoDegenerateVerticalLine) GetBounds(bounds Bounds) {
	geoBaseGetBounds(v, v.PlanetModelField, bounds)
	pm := v.PlanetModelField
	bounds.
		AddVerticalPlane(pm, v.f.longitude, v.f.plane, v.f.boundingPlane, v.f.topPlane, v.f.bottomPlane).
		AddPoint(v.f.uhc).
		AddPoint(v.f.lhc)
}

// String returns a debug representation.
func (v *GeoDegenerateVerticalLine) String() string {
	return fmt.Sprintf("GeoDegenerateVerticalLine: {longitude=%g(%g), toplat=%g(%g), bottomlat=%g(%g)}",
		v.f.longitude, v.f.longitude*180.0/math.Pi,
		v.f.topLat, v.f.topLat*180.0/math.Pi,
		v.f.bottomLat, v.f.bottomLat*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoDegenerateVerticalLine)(nil)
	_ GeoShape = (*GeoDegenerateVerticalLine)(nil)
)
