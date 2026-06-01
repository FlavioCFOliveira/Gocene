// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoDegenerateLongitudeSliceFields holds all computed fields for
// GeoDegenerateLongitudeSlice.
type geoDegenerateLongitudeSliceFields struct {
	longitude     float64
	boundingPlane *SidedPlane
	plane         *Plane
	interiorPoint *GeoPoint
	edgePoints    []*GeoPoint
	planePoints   []*GeoPoint
}

// NewGeoDegenerateLongitudeSlice constructs a GeoDegenerateLongitudeSlice — a
// single meridian (a half great circle from pole to pole).
//
// Port of org.apache.lucene.spatial3d.geom.GeoDegenerateLongitudeSlice(PlanetModel,double).
func NewGeoDegenerateLongitudeSlice(pm *PlanetModel, longitude float64) (*GeoDegenerateLongitudeSlice, error) {
	if longitude < -math.Pi || longitude > math.Pi {
		return nil, fmt.Errorf("geom: GeoDegenerateLongitudeSlice: longitude out of range: %g", longitude)
	}

	sinLongitude := math.Sin(longitude)
	cosLongitude := math.Cos(longitude)

	// plane: cosLon*x + sinLon*y = 0.
	plane := NewPlane(cosLongitude, sinLongitude, 0.0, 0.0)

	// interiorPoint at lat=0, lon=longitude.
	interiorPoint := NewGeoPointTrig(pm, 0.0, sinLongitude, 1.0, cosLongitude)

	// boundingPlane: perpendicular to the longitude plane, sided so interiorPoint is inside.
	// Normal: (-sinLon, cosLon, 0), D=0.
	boundingPlane := NewSidedPlane(&interiorPoint.Vector, -sinLongitude, cosLongitude, 0.0, 0.0)
	if boundingPlane == nil {
		return nil, fmt.Errorf("geom: GeoDegenerateLongitudeSlice: degenerate bounding plane")
	}

	s := &GeoDegenerateLongitudeSlice{GeoBaseBBox: makeBBox(pm)}
	s.f = geoDegenerateLongitudeSliceFields{
		longitude:     longitude,
		boundingPlane: boundingPlane,
		plane:         plane,
		interiorPoint: interiorPoint,
		edgePoints:    []*GeoPoint{interiorPoint},
		planePoints:   []*GeoPoint{pm.NorthPole, pm.SouthPole},
	}
	return s, nil
}

// IsWithin reports whether (x,y,z) is on the degenerate longitude slice.
//
// Port of GeoDegenerateLongitudeSlice.isWithin.
func (s *GeoDegenerateLongitudeSlice) IsWithin(x, y, z float64) bool {
	return s.f.plane.EvaluateIsZeroXYZ(x, y, z) && s.f.boundingPlane.IsWithin(x, y, z)
}

// GetRadius returns PI/2.
//
// Port of GeoDegenerateLongitudeSlice.getRadius.
func (s *GeoDegenerateLongitudeSlice) GetRadius() float64 { return math.Pi * 0.5 }

// GetCenter returns the interior point.
func (s *GeoDegenerateLongitudeSlice) GetCenter() *GeoPoint { return s.f.interiorPoint }

// GetEdgePoints returns the interior point.
func (s *GeoDegenerateLongitudeSlice) GetEdgePoints() []*GeoPoint { return s.f.edgePoints }

// Expand returns a bbox expanded by the given angle.
//
// Port of GeoDegenerateLongitudeSlice.expand.
func (s *GeoDegenerateLongitudeSlice) Expand(angle float64) GeoBBox {
	newLeftLon := s.f.longitude - angle
	newRightLon := s.f.longitude + angle
	currentLonSpan := 2.0 * angle
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

// Intersects reports whether the plane p crosses the degenerate longitude slice.
//
// Port of GeoDegenerateLongitudeSlice.intersects(Plane,GeoPoint[],Membership...).
func (s *GeoDegenerateLongitudeSlice) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	return p.Intersects(s.PlanetModelField, s.f.plane, notablePoints, s.f.planePoints, bounds, s.f.boundingPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoDegenerateLongitudeSlice.intersects(GeoShape).
func (s *GeoDegenerateLongitudeSlice) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(s.f.plane, s.f.planePoints, s.f.boundingPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoDegenerateLongitudeSlice.getRelationship.
func (s *GeoDegenerateLongitudeSlice) GetRelationship(geoShape GeoShape) int {
	if s.intersectsShape(geoShape) {
		return RelOverlaps
	}
	if mem, ok := geoShape.(Membership); ok {
		if mem.IsWithin(s.f.interiorPoint.X, s.f.interiorPoint.Y, s.f.interiorPoint.Z) {
			return RelContains
		}
	}
	return RelDisjoint
}

// GetBounds accumulates the degenerate longitude slice's bounding information.
//
// Port of GeoDegenerateLongitudeSlice.getBounds.
func (s *GeoDegenerateLongitudeSlice) GetBounds(bounds Bounds) {
	geoBaseGetBounds(s, s.PlanetModelField, bounds)
	pm := s.PlanetModelField
	bounds.
		AddVerticalPlane(pm, s.f.longitude, s.f.plane, s.f.boundingPlane).
		AddPoint(pm.NorthPole).
		AddPoint(pm.SouthPole)
}

// String returns a debug representation.
func (s *GeoDegenerateLongitudeSlice) String() string {
	return fmt.Sprintf("GeoDegenerateLongitudeSlice: {planetmodel=%v, longitude=%g(%g)}",
		s.PlanetModelField, s.f.longitude, s.f.longitude*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoDegenerateLongitudeSlice)(nil)
	_ GeoShape = (*GeoDegenerateLongitudeSlice)(nil)
)
