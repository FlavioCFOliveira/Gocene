// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoDegenerateHorizontalLineFields holds all computed fields for GeoDegenerateHorizontalLine.
type geoDegenerateHorizontalLineFields struct {
	latitude    float64
	leftLon     float64
	rightLon    float64
	lhc, rhc    *GeoPoint
	plane       *Plane
	leftPlane   *SidedPlane
	rightPlane  *SidedPlane
	planePoints []*GeoPoint
	centerPoint *GeoPoint
	edgePoints  []*GeoPoint
}

// NewGeoDegenerateHorizontalLine constructs a GeoDegenerateHorizontalLine at the
// given latitude, bounded between leftLon and rightLon. The longitude extent must
// not exceed PI.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateHorizontalLine(PlanetModel,...).
func NewGeoDegenerateHorizontalLine(pm *PlanetModel, latitude, leftLon, rightLon float64) (*GeoDegenerateHorizontalLine, error) {
	if latitude > math.Pi*0.5 || latitude < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoDegenerateHorizontalLine: latitude out of range: %g", latitude)
	}
	if leftLon < -math.Pi || leftLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoDegenerateHorizontalLine: left longitude out of range: %g", leftLon)
	}
	if rightLon < -math.Pi || rightLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoDegenerateHorizontalLine: right longitude out of range: %g", rightLon)
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += 2.0 * math.Pi
	}
	if extent > math.Pi {
		return nil, fmt.Errorf("geom: GeoDegenerateHorizontalLine: width of rectangle too great: %g", extent)
	}

	sinLatitude := math.Sin(latitude)
	cosLatitude := math.Cos(latitude)
	sinLeftLon, cosLeftLon := math.Sin(leftLon), math.Cos(leftLon)
	sinRightLon, cosRightLon := math.Sin(rightLon), math.Cos(rightLon)

	lhc := NewGeoPointTrigLatLon(pm, sinLatitude, sinLeftLon, cosLatitude, cosLeftLon, latitude, leftLon)
	rhc := NewGeoPointTrigLatLon(pm, sinLatitude, sinRightLon, cosLatitude, cosRightLon, latitude, rightLon)

	plane := NewPlaneHorizontal(pm, sinLatitude)

	nRightLon := rightLon
	for leftLon > nRightLon {
		nRightLon += math.Pi * 2.0
	}
	middleLon := (leftLon + nRightLon) * 0.5
	sinMiddleLon := math.Sin(middleLon)
	cosMiddleLon := math.Cos(middleLon)

	centerPoint := NewGeoPointTrig(pm, sinLatitude, sinMiddleLon, cosLatitude, cosMiddleLon)
	leftPlane := NewSidedPlaneVertical(&rhc.Vector, cosLeftLon, sinLeftLon)
	rightPlane := NewSidedPlaneVertical(&lhc.Vector, cosRightLon, sinRightLon)

	if leftPlane == nil || rightPlane == nil {
		return nil, fmt.Errorf("geom: GeoDegenerateHorizontalLine: degenerate bounding planes")
	}

	l := &GeoDegenerateHorizontalLine{GeoBaseBBox: makeBBox(pm)}
	l.f = geoDegenerateHorizontalLineFields{
		latitude:    latitude,
		leftLon:     leftLon,
		rightLon:    rightLon,
		lhc:         lhc,
		rhc:         rhc,
		plane:       plane,
		leftPlane:   leftPlane,
		rightPlane:  rightPlane,
		planePoints: []*GeoPoint{lhc, rhc},
		centerPoint: centerPoint,
		edgePoints:  []*GeoPoint{centerPoint},
	}
	return l, nil
}

// IsWithin reports whether (x,y,z) is on the degenerate horizontal line.
//
// Port of GeoDegenerateHorizontalLine.isWithin.
func (l *GeoDegenerateHorizontalLine) IsWithin(x, y, z float64) bool {
	return l.f.plane.EvaluateIsZeroXYZ(x, y, z) &&
		l.f.leftPlane.IsWithin(x, y, z) &&
		l.f.rightPlane.IsWithin(x, y, z)
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoDegenerateHorizontalLine.getRadius.
func (l *GeoDegenerateHorizontalLine) GetRadius() float64 {
	topAngle := l.f.centerPoint.ArcDistance(&l.f.rhc.Vector)
	bottomAngle := l.f.centerPoint.ArcDistance(&l.f.lhc.Vector)
	return math.Max(topAngle, bottomAngle)
}

// GetCenter returns the center point.
func (l *GeoDegenerateHorizontalLine) GetCenter() *GeoPoint { return l.f.centerPoint }

// GetEdgePoints returns the center point as the sole edge point.
func (l *GeoDegenerateHorizontalLine) GetEdgePoints() []*GeoPoint { return l.f.edgePoints }

// Expand returns a bbox expanded by the given angle.
//
// Port of GeoDegenerateHorizontalLine.expand.
func (l *GeoDegenerateHorizontalLine) Expand(angle float64) GeoBBox {
	newTopLat := l.f.latitude + angle
	newBottomLat := l.f.latitude - angle
	currentLonSpan := l.f.rightLon - l.f.leftLon
	if currentLonSpan < 0.0 {
		currentLonSpan += math.Pi * 2.0
	}
	newLeftLon := l.f.leftLon - angle
	newRightLon := l.f.rightLon + angle
	if currentLonSpan+2.0*angle >= math.Pi*2.0 {
		newLeftLon = -math.Pi
		newRightLon = math.Pi
	}
	bbox, err := MakeGeoBBox(l.PlanetModelField, newTopLat, newBottomLat, newLeftLon, newRightLon)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the degenerate horizontal line.
//
// Port of GeoDegenerateHorizontalLine.intersects(Plane,GeoPoint[],Membership...).
func (l *GeoDegenerateHorizontalLine) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	return p.Intersects(l.PlanetModelField, l.f.plane, notablePoints, l.f.planePoints, bounds, l.f.leftPlane, l.f.rightPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoDegenerateHorizontalLine.intersects(GeoShape).
func (l *GeoDegenerateHorizontalLine) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(l.f.plane, l.f.planePoints, l.f.leftPlane, l.f.rightPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoDegenerateHorizontalLine.getRelationship (degenerate: no area, so
// CONTAINS and OVERLAPS are the only non-DISJOINT outcomes).
func (l *GeoDegenerateHorizontalLine) GetRelationship(geoShape GeoShape) int {
	if l.intersectsShape(geoShape) {
		return RelOverlaps
	}
	if mem, ok := geoShape.(Membership); ok {
		if mem.IsWithin(l.f.centerPoint.X, l.f.centerPoint.Y, l.f.centerPoint.Z) {
			return RelContains
		}
	}
	return RelDisjoint
}

// GetBounds accumulates the degenerate horizontal line's bounding information.
//
// Port of GeoDegenerateHorizontalLine.getBounds.
func (l *GeoDegenerateHorizontalLine) GetBounds(bounds Bounds) {
	geoBaseGetBounds(l, l.PlanetModelField, bounds)
	bounds.
		AddHorizontalPlane(l.PlanetModelField, l.f.latitude, l.f.plane, l.f.leftPlane, l.f.rightPlane).
		AddPoint(l.f.lhc).
		AddPoint(l.f.rhc)
}

// String returns a debug representation.
func (l *GeoDegenerateHorizontalLine) String() string {
	return fmt.Sprintf("GeoDegenerateHorizontalLine: {planetmodel=%v, latitude=%g(%g), leftlon=%g(%g), rightLon=%g(%g)}",
		l.PlanetModelField,
		l.f.latitude, l.f.latitude*180.0/math.Pi,
		l.f.leftLon, l.f.leftLon*180.0/math.Pi,
		l.f.rightLon, l.f.rightLon*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoDegenerateHorizontalLine)(nil)
	_ GeoShape = (*GeoDegenerateHorizontalLine)(nil)
)
