// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoExactCircleFields holds all computed fields for GeoExactCircle.
type geoExactCircleFields struct {
	center         *GeoPoint
	radius         float64
	actualAccuracy float64
	edgePoints     []*GeoPoint
	circleSlices   []exactCircleSlice
}

// exactCircleSlice is one sector of the GeoExactCircle approximation.
//
// Port of GeoExactCircle.CircleSlice.
type exactCircleSlice struct {
	circlePlane      *SidedPlane
	plane1, plane2   *SidedPlane
	notableEdgePoints []*GeoPoint
}

// approximationSlice is a temporary description of a section while being built.
//
// Port of GeoExactCircle.ApproximationSlice.
type approximationSlice struct {
	plane              *SidedPlane
	endPoint1          *GeoPoint
	point1Bearing      float64
	endPoint2          *GeoPoint
	point2Bearing      float64
	middlePoint        *GeoPoint
	middlePointBearing float64
	mustSplit          bool
}

// NewGeoExactCircle constructs a GeoExactCircle centred at (lat,lon) with the
// given surface radius and allowed linear-distance accuracy.
//
// Port of org.apache.lucene.spatial3d.geom.GeoExactCircle(PlanetModel,...).
func NewGeoExactCircle(pm *PlanetModel, lat, lon, radius, accuracy float64) (*GeoExactCircle, error) {
	if lat < -math.Pi*0.5 || lat > math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoExactCircle: latitude out of bounds: %g", lat)
	}
	if lon < -math.Pi || lon > math.Pi {
		return nil, fmt.Errorf("geom: GeoExactCircle: longitude out of bounds: %g", lon)
	}
	if radius < 0.0 {
		return nil, fmt.Errorf("geom: GeoExactCircle: radius out of bounds: %g", radius)
	}
	if radius < MinimumResolution {
		return nil, fmt.Errorf("geom: GeoExactCircle: radius cannot be effectively zero")
	}
	if pm.MinimumPoleDistance-radius < MinimumResolution {
		return nil, fmt.Errorf("geom: GeoExactCircle: radius out of bounds for this planet model")
	}

	actualAccuracy := accuracy
	if actualAccuracy < MinimumResolution {
		actualAccuracy = MinimumResolution
	}

	center := NewGeoPointModel(pm, lat, lon)

	northPoint := pm.SurfacePointOnBearing(center, radius, 0.0)
	southPoint := pm.SurfacePointOnBearing(center, radius, math.Pi)
	eastPoint := pm.SurfacePointOnBearing(center, radius, math.Pi*0.5)
	westPoint := pm.SurfacePointOnBearing(center, radius, math.Pi*1.5)

	var edgePoint *GeoPoint
	slices := make([]approximationSlice, 0, 4)
	if pm.ZScaling > pm.XYScaling {
		slices = append(slices,
			newApproximationSlice(center, eastPoint, math.Pi*0.5, westPoint, -math.Pi*0.5, northPoint, 0.0, true),
			newApproximationSlice(center, westPoint, math.Pi*1.5, eastPoint, math.Pi*0.5, southPoint, math.Pi, true),
		)
		edgePoint = eastPoint
	} else {
		slices = append(slices,
			newApproximationSlice(center, northPoint, 0.0, southPoint, math.Pi, eastPoint, math.Pi*0.5, true),
			newApproximationSlice(center, southPoint, math.Pi, northPoint, math.Pi*2.0, westPoint, math.Pi*1.5, true),
		)
		edgePoint = northPoint
	}

	circleSlices := make([]exactCircleSlice, 0, 32)

	for len(slices) > 0 {
		thisSlice := slices[len(slices)-1]
		slices = slices[:len(slices)-1]

		if thisSlice.plane == nil {
			return nil, fmt.Errorf("geom: GeoExactCircle: could not construct approximation plane")
		}

		interp1Bearing := (thisSlice.point1Bearing + thisSlice.middlePointBearing) * 0.5
		interp1 := pm.SurfacePointOnBearing(center, radius, interp1Bearing)
		interp2Bearing := (thisSlice.point2Bearing + thisSlice.middlePointBearing) * 0.5
		interp2 := pm.SurfacePointOnBearing(center, radius, interp2Bearing)

		if !thisSlice.mustSplit &&
			math.Abs(thisSlice.plane.EvaluateXYZ(interp1.X, interp1.Y, interp1.Z)) < actualAccuracy &&
			math.Abs(thisSlice.plane.EvaluateXYZ(interp2.X, interp2.Y, interp2.Z)) < actualAccuracy {
			circleSlices = append(circleSlices, newCircleSlice(thisSlice.plane, thisSlice.endPoint1, thisSlice.endPoint2, center, thisSlice.middlePoint))
		} else {
			s1 := newApproximationSlice(center, thisSlice.endPoint1, thisSlice.point1Bearing, thisSlice.middlePoint, thisSlice.middlePointBearing, interp1, interp1Bearing, false)
			s2 := newApproximationSlice(center, thisSlice.middlePoint, thisSlice.middlePointBearing, thisSlice.endPoint2, thisSlice.point2Bearing, interp2, interp2Bearing, false)
			slices = append(slices, s1, s2)
		}
	}

	c := &GeoExactCircle{GeoBaseCircle: makeCircle(pm, radius)}
	c.f = geoExactCircleFields{
		center:         center,
		radius:         radius,
		actualAccuracy: actualAccuracy,
		edgePoints:     []*GeoPoint{edgePoint},
		circleSlices:   circleSlices,
	}
	return c, nil
}

// newApproximationSlice builds an approximation slice and computes its plane.
func newApproximationSlice(center, ep1 *GeoPoint, b1 float64, ep2 *GeoPoint, b2 float64, mid *GeoPoint, midBearing float64, mustSplit bool) approximationSlice {
	s := approximationSlice{
		endPoint1:          ep1,
		point1Bearing:      b1,
		endPoint2:          ep2,
		point2Bearing:      b2,
		middlePoint:        mid,
		middlePointBearing: midBearing,
		mustSplit:          mustSplit,
	}
	s.plane = ConstructNormalizedThreePointSidedPlane(&center.Vector, &ep1.Vector, &ep2.Vector, &mid.Vector)
	return s
}

// newCircleSlice builds a finalised circle sector from its bounding plane and endpoints.
func newCircleSlice(circlePlane *SidedPlane, ep1, ep2, center, check *GeoPoint) exactCircleSlice {
	p1 := NewSidedPlaneThreeVectors(&check.Vector, &ep1.Vector, &center.Vector)
	p2 := NewSidedPlaneThreeVectors(&check.Vector, &ep2.Vector, &center.Vector)
	return exactCircleSlice{
		circlePlane:       circlePlane,
		plane1:            p1,
		plane2:            p2,
		notableEdgePoints: []*GeoPoint{ep1, ep2},
	}
}

// GetRadius returns the circle's surface radius.
//
// Port of GeoExactCircle.getRadius.
func (c *GeoExactCircle) GetRadius() float64 { return c.f.radius }

// GetCenter returns the circle's center point.
//
// Port of GeoExactCircle.getCenter.
func (c *GeoExactCircle) GetCenter() *GeoPoint { return c.f.center }

// IsWithin reports whether (x,y,z) is inside the exact circle.
//
// Port of GeoExactCircle.isWithin.
func (c *GeoExactCircle) IsWithin(x, y, z float64) bool {
	for _, sl := range c.f.circleSlices {
		if sl.circlePlane.IsWithin(x, y, z) &&
			(sl.plane1 == nil || sl.plane1.IsWithin(x, y, z)) &&
			(sl.plane2 == nil || sl.plane2.IsWithin(x, y, z)) {
			return true
		}
	}
	return false
}

// GetEdgePoints returns a sample edge point.
//
// Port of GeoExactCircle.getEdgePoints.
func (c *GeoExactCircle) GetEdgePoints() []*GeoPoint { return c.f.edgePoints }

// Intersects reports whether the plane p crosses the exact circle.
//
// Port of GeoExactCircle.intersects(Plane,GeoPoint[],Membership...).
func (c *GeoExactCircle) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := c.PlanetModelField
	for _, sl := range c.f.circleSlices {
		var extraBounds []Membership
		if sl.plane1 != nil {
			extraBounds = append(extraBounds, sl.plane1)
		}
		if sl.plane2 != nil {
			extraBounds = append(extraBounds, sl.plane2)
		}
		if sl.circlePlane.Intersects(pm, p, notablePoints, sl.notableEdgePoints, bounds, extraBounds...) {
			return true
		}
	}
	return false
}

// GetBounds accumulates the exact circle's bounding information.
//
// Port of GeoExactCircle.getBounds.
func (c *GeoExactCircle) GetBounds(bounds Bounds) {
	geoBaseGetBounds(c, c.PlanetModelField, bounds)
	bounds.AddPoint(c.f.center)
	pm := c.PlanetModelField
	for _, sl := range c.f.circleSlices {
		var extraBounds []Membership
		if sl.plane1 != nil {
			extraBounds = append(extraBounds, sl.plane1)
		}
		if sl.plane2 != nil {
			extraBounds = append(extraBounds, sl.plane2)
		}
		bounds.AddPlane(pm, &sl.circlePlane.Plane, extraBounds...)
		for _, ep := range sl.notableEdgePoints {
			bounds.AddPoint(ep)
		}
	}
}

// String returns a debug representation.
func (c *GeoExactCircle) String() string {
	return fmt.Sprintf("GeoExactCircle: {planetmodel=%v, center=%v, radius=%g(%g), accuracy=%g}",
		c.PlanetModelField, c.f.center, c.f.radius, c.f.radius*180.0/math.Pi, c.f.actualAccuracy)
}

var (
	_ GeoCircle = (*GeoExactCircle)(nil)
	_ GeoShape  = (*GeoExactCircle)(nil)
)
