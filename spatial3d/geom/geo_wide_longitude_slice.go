// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoWideLongitudeSliceFields holds all computed fields for GeoWideLongitudeSlice.
type geoWideLongitudeSliceFields struct {
	leftLon     float64
	rightLon    float64
	leftPlane   *SidedPlane
	rightPlane  *SidedPlane
	planePoints []*GeoPoint
	centerPoint *GeoPoint
	edgePoints  []*GeoPoint
}

// NewGeoWideLongitudeSlice constructs a GeoWideLongitudeSlice. The longitude
// extent must be at least PI - MinimumAngularResolution.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideLongitudeSlice(PlanetModel,double,double).
func NewGeoWideLongitudeSlice(pm *PlanetModel, leftLon, rightLon float64) (*GeoWideLongitudeSlice, error) {
	if leftLon < -math.Pi || leftLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoWideLongitudeSlice: left longitude out of range: %g", leftLon)
	}
	if rightLon < -math.Pi || rightLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoWideLongitudeSlice: right longitude out of range: %g", rightLon)
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += 2.0 * math.Pi
	}
	if extent < minWideExtent {
		return nil, fmt.Errorf("geom: GeoWideLongitudeSlice: width of rectangle too small: %g", extent)
	}

	sinLeftLon, cosLeftLon := math.Sin(leftLon), math.Cos(leftLon)
	sinRightLon, cosRightLon := math.Sin(rightLon), math.Cos(rightLon)

	nRightLon := rightLon
	for leftLon > nRightLon {
		nRightLon += math.Pi * 2.0
	}
	middleLon := (leftLon + nRightLon) * 0.5
	// Wrap middleLon into [-PI, PI].
	for middleLon > math.Pi {
		middleLon -= math.Pi * 2.0
	}
	for middleLon < -math.Pi {
		middleLon += math.Pi * 2.0
	}

	centerPoint := NewGeoPointLatLon(pm, 0.0, middleLon)

	leftPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosLeftLon, sinLeftLon)
	rightPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosRightLon, sinRightLon)

	if leftPlane == nil || rightPlane == nil {
		return nil, fmt.Errorf("geom: GeoWideLongitudeSlice: degenerate bounding planes")
	}

	s := &GeoWideLongitudeSlice{GeoBaseBBox: makeBBox(pm)}
	s.f = geoWideLongitudeSliceFields{
		leftLon:     leftLon,
		rightLon:    rightLon,
		leftPlane:   leftPlane,
		rightPlane:  rightPlane,
		planePoints: []*GeoPoint{pm.NorthPole, pm.SouthPole},
		centerPoint: centerPoint,
		edgePoints:  []*GeoPoint{pm.NorthPole},
	}
	return s, nil
}

// IsWithin reports whether (x,y,z) is inside the wide longitude slice.
//
// Port of GeoWideLongitudeSlice.isWithin.
func (s *GeoWideLongitudeSlice) IsWithin(x, y, z float64) bool {
	return s.f.leftPlane.IsWithin(x, y, z) || s.f.rightPlane.IsWithin(x, y, z)
}

// GetRadius returns the radius of the wide longitude slice.
//
// Port of GeoWideLongitudeSlice.getRadius.
func (s *GeoWideLongitudeSlice) GetRadius() float64 {
	extent := s.f.rightLon - s.f.leftLon
	if extent < 0.0 {
		extent += math.Pi * 2.0
	}
	return math.Max(math.Pi*0.5, extent*0.5)
}

// GetCenter returns the center point.
func (s *GeoWideLongitudeSlice) GetCenter() *GeoPoint { return s.f.centerPoint }

// GetEdgePoints returns a sample edge point (the north pole).
func (s *GeoWideLongitudeSlice) GetEdgePoints() []*GeoPoint { return s.f.edgePoints }

// Expand returns a bbox expanded by the given angle.
//
// Port of GeoWideLongitudeSlice.expand.
func (s *GeoWideLongitudeSlice) Expand(angle float64) GeoBBox {
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

// Intersects reports whether the plane p crosses the wide longitude slice.
//
// Port of GeoWideLongitudeSlice.intersects(Plane,GeoPoint[],Membership...).
func (s *GeoWideLongitudeSlice) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := s.PlanetModelField
	// Wide: intersection can ignore the left/right cross-bounds.
	return p.Intersects(pm, &s.f.leftPlane.Plane, notablePoints, s.f.planePoints, bounds) ||
		p.Intersects(pm, &s.f.rightPlane.Plane, notablePoints, s.f.planePoints, bounds)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoWideLongitudeSlice.intersects(GeoShape).
func (s *GeoWideLongitudeSlice) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&s.f.leftPlane.Plane, s.f.planePoints) ||
		geoShape.Intersects(&s.f.rightPlane.Plane, s.f.planePoints)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (s *GeoWideLongitudeSlice) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(s.IsWithin, s.f.edgePoints, s.intersectsShape, geoShape)
}

// GetBounds accumulates the wide longitude slice's bounding information.
//
// Port of GeoWideLongitudeSlice.getBounds.
func (s *GeoWideLongitudeSlice) GetBounds(bounds Bounds) {
	geoBaseGetBounds(s, s.PlanetModelField, bounds)
	pm := s.PlanetModelField
	bounds.
		IsWide().
		AddVerticalPlane(pm, s.f.leftLon, &s.f.leftPlane.Plane).
		AddVerticalPlane(pm, s.f.rightLon, &s.f.rightPlane.Plane).
		AddIntersection(pm, &s.f.leftPlane.Plane, &s.f.rightPlane.Plane).
		AddPoint(pm.NorthPole).
		AddPoint(pm.SouthPole)
}

// String returns a debug representation.
func (s *GeoWideLongitudeSlice) String() string {
	return fmt.Sprintf("GeoWideLongitudeSlice: {planetmodel=%v, leftlon=%g(%g), rightlon=%g(%g)}",
		s.PlanetModelField,
		s.f.leftLon, s.f.leftLon*180.0/math.Pi,
		s.f.rightLon, s.f.rightLon*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoWideLongitudeSlice)(nil)
	_ GeoShape = (*GeoWideLongitudeSlice)(nil)
)
