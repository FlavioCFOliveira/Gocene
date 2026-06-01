// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoLongitudeSliceFields holds all computed fields for GeoLongitudeSlice.
type geoLongitudeSliceFields struct {
	leftLon      float64
	rightLon     float64
	leftPlane    *SidedPlane
	rightPlane   *SidedPlane
	backingPlane *SidedPlane
	planePoints  []*GeoPoint
	centerPoint  *GeoPoint
	edgePoints   []*GeoPoint
}

// NewGeoLongitudeSlice constructs a GeoLongitudeSlice bounded by two meridians.
// The longitude extent must not exceed PI (use GeoWideLongitudeSlice for wider slices).
//
// Port of org.apache.lucene.spatial3d.geom.GeoLongitudeSlice(PlanetModel,double,double).
func NewGeoLongitudeSlice(pm *PlanetModel, leftLon, rightLon float64) (*GeoLongitudeSlice, error) {
	if leftLon < -math.Pi || leftLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoLongitudeSlice: left longitude out of range: %g", leftLon)
	}
	if rightLon < -math.Pi || rightLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoLongitudeSlice: right longitude out of range: %g", rightLon)
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += 2.0 * math.Pi
	}
	if extent > math.Pi {
		return nil, fmt.Errorf("geom: GeoLongitudeSlice: width of rectangle too great: %g", extent)
	}

	sinLeftLon, cosLeftLon := math.Sin(leftLon), math.Cos(leftLon)
	sinRightLon, cosRightLon := math.Sin(rightLon), math.Cos(rightLon)

	nRightLon := rightLon
	for leftLon > nRightLon {
		nRightLon += math.Pi * 2.0
	}
	middleLon := (leftLon + nRightLon) * 0.5
	sinMiddleLon := math.Sin(middleLon)
	cosMiddleLon := math.Cos(middleLon)

	// centerPoint: lat=0, middleLon.
	centerPoint := NewGeoPointTrig(pm, 0.0, sinMiddleLon, 1.0, cosMiddleLon)

	leftPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosLeftLon, sinLeftLon)
	rightPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosRightLon, sinRightLon)

	// backingPlane: normal through (cosMiddleLon, sinMiddleLon, 0), D=0.
	backingPlane := NewSidedPlane(&centerPoint.Vector, cosMiddleLon, sinMiddleLon, 0.0, 0.0)

	if leftPlane == nil || rightPlane == nil || backingPlane == nil {
		return nil, fmt.Errorf("geom: GeoLongitudeSlice: degenerate bounding planes")
	}

	s := &GeoLongitudeSlice{GeoBaseBBox: makeBBox(pm)}
	s.f = geoLongitudeSliceFields{
		leftLon:      leftLon,
		rightLon:     rightLon,
		leftPlane:    leftPlane,
		rightPlane:   rightPlane,
		backingPlane: backingPlane,
		planePoints:  []*GeoPoint{pm.NorthPole, pm.SouthPole},
		centerPoint:  centerPoint,
		edgePoints:   []*GeoPoint{pm.NorthPole},
	}
	return s, nil
}

// IsWithin reports whether (x,y,z) is inside the longitude slice.
//
// Port of GeoLongitudeSlice.isWithin.
func (s *GeoLongitudeSlice) IsWithin(x, y, z float64) bool {
	return s.f.backingPlane.IsWithin(x, y, z) &&
		s.f.leftPlane.IsWithin(x, y, z) &&
		s.f.rightPlane.IsWithin(x, y, z)
}

// GetRadius returns the radius of the longitude slice.
//
// Port of GeoLongitudeSlice.getRadius.
func (s *GeoLongitudeSlice) GetRadius() float64 {
	extent := s.f.rightLon - s.f.leftLon
	if extent < 0.0 {
		extent += math.Pi * 2.0
	}
	return math.Max(math.Pi*0.5, extent*0.5)
}

// GetCenter returns the center point.
func (s *GeoLongitudeSlice) GetCenter() *GeoPoint { return s.f.centerPoint }

// GetEdgePoints returns a sample edge point (the north pole).
func (s *GeoLongitudeSlice) GetEdgePoints() []*GeoPoint { return s.f.edgePoints }

// Expand returns a bbox expanded by the given angle.
//
// Port of GeoLongitudeSlice.expand.
func (s *GeoLongitudeSlice) Expand(angle float64) GeoBBox {
	currentLonSpan := s.f.rightLon - s.f.leftLon
	if currentLonSpan < 0.0 {
		currentLonSpan += math.Pi * 2.0
	}
	newLeftLon := s.f.leftLon - angle
	newRightLon := s.f.rightLon + angle
	if currentLonSpan+2.0*angle >= math.Pi*2.0 {
		newLeftLon = -math.Pi
		newRightLon = math.Pi
	}
	bbox, err := MakeGeoBBox(s.PlanetModelField, math.Pi*0.5, -math.Pi*0.5, newLeftLon, newRightLon)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the longitude slice.
//
// Port of GeoLongitudeSlice.intersects(Plane,GeoPoint[],Membership...).
func (s *GeoLongitudeSlice) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := s.PlanetModelField
	return p.Intersects(pm, &s.f.leftPlane.Plane, notablePoints, s.f.planePoints, bounds, s.f.rightPlane) ||
		p.Intersects(pm, &s.f.rightPlane.Plane, notablePoints, s.f.planePoints, bounds, s.f.leftPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoLongitudeSlice.intersects(GeoShape).
func (s *GeoLongitudeSlice) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&s.f.leftPlane.Plane, s.f.planePoints, s.f.rightPlane) ||
		geoShape.Intersects(&s.f.rightPlane.Plane, s.f.planePoints, s.f.leftPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (s *GeoLongitudeSlice) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(s.IsWithin, s.f.edgePoints, s.intersectsShape, geoShape)
}

// GetBounds accumulates the longitude slice's bounding information.
//
// Port of GeoLongitudeSlice.getBounds.
func (s *GeoLongitudeSlice) GetBounds(bounds Bounds) {
	geoBaseGetBounds(s, s.PlanetModelField, bounds)
	pm := s.PlanetModelField
	bounds.
		AddVerticalPlane(pm, s.f.leftLon, &s.f.leftPlane.Plane, s.f.rightPlane).
		AddVerticalPlane(pm, s.f.rightLon, &s.f.rightPlane.Plane, s.f.leftPlane).
		AddPoint(pm.NorthPole).
		AddPoint(pm.SouthPole)
}

// String returns a debug representation.
func (s *GeoLongitudeSlice) String() string {
	return fmt.Sprintf("GeoLongitudeSlice: {planetmodel=%v, leftlon=%g(%g), rightlon=%g(%g)}",
		s.PlanetModelField,
		s.f.leftLon, s.f.leftLon*180.0/math.Pi,
		s.f.rightLon, s.f.rightLon*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoLongitudeSlice)(nil)
	_ GeoShape = (*GeoLongitudeSlice)(nil)
)
