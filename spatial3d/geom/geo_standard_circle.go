// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// circlePoints is the (empty) set of notable points for a circle.
var circlePoints = []*GeoPoint{}

// geoBaseGetBounds replicates GeoBaseBounds.getBounds: it adds each planet pole
// that is within the shape, with the appropriate no-bound markers.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseBounds.getBounds.
func geoBaseGetBounds(shape Membership, pm *PlanetModel, bounds Bounds) {
	if shape.IsWithin(pm.NorthPole.X, pm.NorthPole.Y, pm.NorthPole.Z) {
		bounds.NoTopLatitudeBound().NoLongitudeBound().AddPoint(pm.NorthPole)
	}
	if shape.IsWithin(pm.SouthPole.X, pm.SouthPole.Y, pm.SouthPole.Z) {
		bounds.NoBottomLatitudeBound().NoLongitudeBound().AddPoint(pm.SouthPole)
	}
	if shape.IsWithin(pm.MinXPole.X, pm.MinXPole.Y, pm.MinXPole.Z) {
		bounds.AddPoint(pm.MinXPole)
	}
	if shape.IsWithin(pm.MaxXPole.X, pm.MaxXPole.Y, pm.MaxXPole.Z) {
		bounds.AddPoint(pm.MaxXPole)
	}
	if shape.IsWithin(pm.MinYPole.X, pm.MinYPole.Y, pm.MinYPole.Z) {
		bounds.AddPoint(pm.MinYPole)
	}
	if shape.IsWithin(pm.MaxYPole.X, pm.MaxYPole.Y, pm.MaxYPole.Z) {
		bounds.AddPoint(pm.MaxYPole)
	}
}

// NewGeoStandardCircle constructs a GeoStandardCircle centred at (lat,lon) with
// the given cutoff angle (radians).
//
// Port of org.apache.lucene.spatial3d.geom.GeoStandardCircle(PlanetModel,double,double,double).
func NewGeoStandardCircle(pm *PlanetModel, lat, lon, cutoffAngle float64) (*GeoStandardCircle, error) {
	if lat < -math.Pi*0.5 || lat > math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoStandardCircle: latitude out of bounds: %g", lat)
	}
	if lon < -math.Pi || lon > math.Pi {
		return nil, fmt.Errorf("geom: GeoStandardCircle: longitude out of bounds: %g", lon)
	}
	if cutoffAngle < 0.0 || cutoffAngle > math.Pi {
		return nil, fmt.Errorf("geom: GeoStandardCircle: cutoff angle out of bounds: %g", cutoffAngle)
	}
	if cutoffAngle < MinimumResolution {
		return nil, fmt.Errorf("geom: GeoStandardCircle: cutoff angle cannot be effectively zero")
	}

	c := &GeoStandardCircle{
		GeoBaseCircle: makeCircle(pm, cutoffAngle),
		center:        NewGeoPointModel(pm, lat, lon),
		cutoffAngle:   cutoffAngle,
	}

	// Compute two points on the circle, at the right angle from the centre,
	// from which the perpendicular plane to the circle is obtained.
	upperLat := lat + cutoffAngle
	upperLon := lon
	if upperLat > math.Pi*0.5 {
		upperLon += math.Pi
		if upperLon > math.Pi {
			upperLon -= 2.0 * math.Pi
		}
		upperLat = math.Pi - upperLat
	}
	lowerLat := lat - cutoffAngle
	lowerLon := lon
	if lowerLat < -math.Pi*0.5 {
		lowerLon += math.Pi
		if lowerLon > math.Pi {
			lowerLon -= 2.0 * math.Pi
		}
		lowerLat = -math.Pi - lowerLat
	}
	upperPoint := NewGeoPointModel(pm, upperLat, upperLon)
	lowerPoint := NewGeoPointModel(pm, lowerLat, lowerLon)

	if math.Abs(cutoffAngle-math.Pi) < MinimumResolution {
		// Circle is the whole world.
		c.circlePlane = nil
		c.edgePoints = []*GeoPoint{}
		return c, nil
	}

	normalPlane := ConstructNormalizedZPlane(&upperPoint.Vector, &lowerPoint.Vector, &c.center.Vector)
	if normalPlane == nil {
		return nil, fmt.Errorf("geom: GeoStandardCircle: couldn't construct normal plane (cutoff %g)", cutoffAngle)
	}
	c.circlePlane = ConstructNormalizedPerpendicularSidedPlane(&c.center.Vector, &normalPlane.Vector, &upperPoint.Vector, &lowerPoint.Vector)
	if c.circlePlane == nil {
		return nil, fmt.Errorf("geom: GeoStandardCircle: couldn't construct circle plane (cutoff %g)", cutoffAngle)
	}
	recomputed := c.circlePlane.GetSampleIntersectionPoint(pm, normalPlane)
	if recomputed == nil {
		return nil, fmt.Errorf("geom: GeoStandardCircle: couldn't construct intersection point (cutoff %g)", cutoffAngle)
	}
	c.edgePoints = []*GeoPoint{recomputed}
	return c, nil
}

// GetRadius returns the cutoff angle (radius in radians).
func (c *GeoStandardCircle) GetRadius() float64 { return c.cutoffAngle }

// GetCenter returns the circle's centre point.
func (c *GeoStandardCircle) GetCenter() *GeoPoint { return c.center }

// IsWithin reports whether (x,y,z) is inside the circle.
//
// Port of GeoStandardCircle.isWithin.
func (c *GeoStandardCircle) IsWithin(x, y, z float64) bool {
	if c.circlePlane == nil {
		return true
	}
	return c.circlePlane.IsWithin(x, y, z)
}

// GetEdgePoints returns sample points on the circle edge.
func (c *GeoStandardCircle) GetEdgePoints() []*GeoPoint { return c.edgePoints }

// Intersects reports whether the plane p (within bounds) crosses the circle.
//
// Port of GeoStandardCircle.intersects(Plane,GeoPoint[],Membership...).
func (c *GeoStandardCircle) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	if c.circlePlane == nil {
		return false
	}
	return c.circlePlane.Intersects(c.PlanetModelField, p, notablePoints, circlePoints, bounds)
}

// GetBounds accumulates the circle's bounding information.
//
// Port of GeoStandardCircle.getBounds. The plane tangent-point tightening done
// by Plane.recordBounds is approximated here by adding the centre and edge
// points; see the package deviation note. isWithin is unaffected.
func (c *GeoStandardCircle) GetBounds(bounds Bounds) {
	geoBaseGetBounds(c, c.PlanetModelField, bounds)
	if c.circlePlane == nil {
		return
	}
	bounds.AddPoint(c.center)
	bounds.AddPlane(c.PlanetModelField, &c.circlePlane.Plane)
	for _, ep := range c.edgePoints {
		bounds.AddPoint(ep)
	}
}

// String returns a debug representation.
func (c *GeoStandardCircle) String() string {
	return fmt.Sprintf("GeoStandardCircle: {planetmodel=%v, center=%v, radius=%g(%g)}",
		c.PlanetModelField, c.center, c.cutoffAngle, c.cutoffAngle*180.0/math.Pi)
}

var (
	_ GeoCircle = (*GeoStandardCircle)(nil)
	_ GeoShape  = (*GeoStandardCircle)(nil)
)
