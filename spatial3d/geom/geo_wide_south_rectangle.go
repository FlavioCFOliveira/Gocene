// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoWideSouthRectangleFields holds all computed fields for GeoWideSouthRectangle.
type geoWideSouthRectangleFields struct {
	topLat           float64
	leftLon          float64
	rightLon         float64
	cosMiddleLat     float64
	ulhc, urhc       *GeoPoint
	topPlane         *SidedPlane
	leftPlane        *SidedPlane
	rightPlane       *SidedPlane
	topPlanePoints   []*GeoPoint
	leftPlanePoints  []*GeoPoint
	rightPlanePoints []*GeoPoint
	centerPoint      *GeoPoint
	eitherBound      Membership
	edgePoints       []*GeoPoint
}

// wideSouthEitherBound is a Membership satisfied when the point is within
// either the left or the right longitude plane.
//
// Port of GeoWideSouthRectangle.EitherBound.
type wideSouthEitherBound struct {
	leftPlane  *SidedPlane
	rightPlane *SidedPlane
}

func (e *wideSouthEitherBound) IsWithin(x, y, z float64) bool {
	return e.leftPlane.IsWithin(x, y, z) || e.rightPlane.IsWithin(x, y, z)
}

// NewGeoWideSouthRectangle constructs a GeoWideSouthRectangle. The longitude
// extent must be at least PI - MinimumAngularResolution.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideSouthRectangle(PlanetModel,...).
func NewGeoWideSouthRectangle(pm *PlanetModel, topLat, leftLon, rightLon float64) (*GeoWideSouthRectangle, error) {
	if topLat > math.Pi*0.5 || topLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoWideSouthRectangle: top latitude out of range: %g", topLat)
	}
	if leftLon < -math.Pi || leftLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoWideSouthRectangle: left longitude out of range: %g", leftLon)
	}
	if rightLon < -math.Pi || rightLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoWideSouthRectangle: right longitude out of range: %g", rightLon)
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += 2.0 * math.Pi
	}
	if extent < minWideExtent {
		return nil, fmt.Errorf("geom: GeoWideSouthRectangle: width of rectangle too small: %g", extent)
	}

	sinTopLat, cosTopLat := math.Sin(topLat), math.Cos(topLat)
	sinLeftLon, cosLeftLon := math.Sin(leftLon), math.Cos(leftLon)
	sinRightLon, cosRightLon := math.Sin(rightLon), math.Cos(rightLon)

	ulhc := NewGeoPointTrigLatLon(pm, sinTopLat, sinLeftLon, cosTopLat, cosLeftLon, topLat, leftLon)
	urhc := NewGeoPointTrigLatLon(pm, sinTopLat, sinRightLon, cosTopLat, cosRightLon, topLat, rightLon)

	middleLat := (topLat - math.Pi*0.5) * 0.5
	sinMiddleLat := math.Sin(middleLat)
	cosMiddleLat := math.Cos(middleLat)

	nRightLon := rightLon
	for leftLon > nRightLon {
		nRightLon += math.Pi * 2.0
	}
	middleLon := (leftLon + nRightLon) * 0.5
	sinMiddleLon := math.Sin(middleLon)
	cosMiddleLon := math.Cos(middleLon)

	centerPoint := NewGeoPointTrig(pm, sinMiddleLat, sinMiddleLon, cosMiddleLat, cosMiddleLon)

	topPlane := NewSidedPlaneHorizontal(&centerPoint.Vector, pm, sinTopLat)
	leftPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosLeftLon, sinLeftLon)
	rightPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosRightLon, sinRightLon)

	if topPlane == nil || leftPlane == nil || rightPlane == nil {
		return nil, fmt.Errorf("geom: GeoWideSouthRectangle: degenerate bounding planes")
	}

	eitherBound := &wideSouthEitherBound{leftPlane: leftPlane, rightPlane: rightPlane}

	r := &GeoWideSouthRectangle{GeoBaseBBox: makeBBox(pm)}
	r.f = geoWideSouthRectangleFields{
		topLat:           topLat,
		leftLon:          leftLon,
		rightLon:         rightLon,
		cosMiddleLat:     cosMiddleLat,
		ulhc:             ulhc,
		urhc:             urhc,
		topPlane:         topPlane,
		leftPlane:        leftPlane,
		rightPlane:       rightPlane,
		topPlanePoints:   []*GeoPoint{ulhc, urhc},
		leftPlanePoints:  []*GeoPoint{ulhc, pm.SouthPole},
		rightPlanePoints: []*GeoPoint{urhc, pm.SouthPole},
		centerPoint:      centerPoint,
		eitherBound:      eitherBound,
		edgePoints:       []*GeoPoint{pm.SouthPole},
	}
	return r, nil
}

// IsWithin reports whether (x,y,z) is inside the wide south rectangle.
//
// Port of GeoWideSouthRectangle.isWithin.
func (r *GeoWideSouthRectangle) IsWithin(x, y, z float64) bool {
	return r.f.topPlane.IsWithin(x, y, z) &&
		(r.f.leftPlane.IsWithin(x, y, z) || r.f.rightPlane.IsWithin(x, y, z))
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoWideSouthRectangle.getRadius.
func (r *GeoWideSouthRectangle) GetRadius() float64 {
	centerAngle := (r.f.rightLon - (r.f.rightLon+r.f.leftLon)*0.5) * r.f.cosMiddleLat
	topAngle := r.f.centerPoint.ArcDistance(&r.f.urhc.Vector)
	return math.Max(centerAngle, topAngle)
}

// GetCenter returns the center point.
func (r *GeoWideSouthRectangle) GetCenter() *GeoPoint { return r.f.centerPoint }

// GetEdgePoints returns a sample edge point (the south pole).
func (r *GeoWideSouthRectangle) GetEdgePoints() []*GeoPoint { return r.f.edgePoints }

// Expand returns a rectangle expanded by the given angle.
//
// Port of GeoWideSouthRectangle.expand.
func (r *GeoWideSouthRectangle) Expand(angle float64) GeoBBox {
	newTopLat := r.f.topLat + angle
	currentLonSpan := r.f.rightLon - r.f.leftLon
	if currentLonSpan < 0.0 {
		currentLonSpan += math.Pi * 2.0
	}
	newLeftLon := r.f.leftLon - angle
	newRightLon := r.f.rightLon + angle
	if currentLonSpan+2.0*angle >= math.Pi*2.0 {
		newLeftLon = -math.Pi
		newRightLon = math.Pi
	}
	bbox, err := MakeGeoBBox(r.PlanetModelField, newTopLat, -math.Pi*0.5, newLeftLon, newRightLon)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the wide south rectangle.
//
// Port of GeoWideSouthRectangle.intersects(Plane,GeoPoint[],Membership...).
func (r *GeoWideSouthRectangle) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := r.PlanetModelField
	return p.Intersects(pm, &r.f.topPlane.Plane, notablePoints, r.f.topPlanePoints, bounds, r.f.eitherBound) ||
		p.Intersects(pm, &r.f.leftPlane.Plane, notablePoints, r.f.leftPlanePoints, bounds, r.f.topPlane) ||
		p.Intersects(pm, &r.f.rightPlane.Plane, notablePoints, r.f.rightPlanePoints, bounds, r.f.topPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoWideSouthRectangle.intersects(GeoShape).
func (r *GeoWideSouthRectangle) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&r.f.topPlane.Plane, r.f.topPlanePoints, r.f.eitherBound) ||
		geoShape.Intersects(&r.f.leftPlane.Plane, r.f.leftPlanePoints, r.f.topPlane) ||
		geoShape.Intersects(&r.f.rightPlane.Plane, r.f.rightPlanePoints, r.f.topPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (r *GeoWideSouthRectangle) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(r.IsWithin, r.f.edgePoints, r.intersectsShape, geoShape)
}

// GetBounds accumulates the wide south rectangle's bounding information.
//
// Port of GeoWideSouthRectangle.getBounds.
func (r *GeoWideSouthRectangle) GetBounds(bounds Bounds) {
	geoBaseGetBounds(r, r.PlanetModelField, bounds)
	pm := r.PlanetModelField
	bounds.
		IsWide().
		AddHorizontalPlane(pm, r.f.topLat, &r.f.topPlane.Plane, r.f.eitherBound).
		AddVerticalPlane(pm, r.f.rightLon, &r.f.rightPlane.Plane, r.f.topPlane).
		AddVerticalPlane(pm, r.f.leftLon, &r.f.leftPlane.Plane, r.f.topPlane).
		AddIntersection(pm, &r.f.leftPlane.Plane, &r.f.rightPlane.Plane, r.f.topPlane).
		AddPoint(r.f.ulhc).
		AddPoint(r.f.urhc).
		AddPoint(pm.SouthPole)
}

// String returns a debug representation.
func (r *GeoWideSouthRectangle) String() string {
	return fmt.Sprintf("GeoWideSouthRectangle: {planetmodel=%v, toplat=%g(%g), leftlon=%g(%g), rightlon=%g(%g)}",
		r.PlanetModelField,
		r.f.topLat, r.f.topLat*180.0/math.Pi,
		r.f.leftLon, r.f.leftLon*180.0/math.Pi,
		r.f.rightLon, r.f.rightLon*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoWideSouthRectangle)(nil)
	_ GeoShape = (*GeoWideSouthRectangle)(nil)
)
