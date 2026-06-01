// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoSouthRectangleFields holds all computed fields for GeoSouthRectangle.
type geoSouthRectangleFields struct {
	topLat           float64
	leftLon          float64
	rightLon         float64
	cosMiddleLat     float64
	ulhc, urhc       *GeoPoint
	topPlane         *SidedPlane
	leftPlane        *SidedPlane
	rightPlane       *SidedPlane
	backingPlane     *SidedPlane
	topPlanePoints   []*GeoPoint
	leftPlanePoints  []*GeoPoint
	rightPlanePoints []*GeoPoint
	centerPoint      *GeoPoint
	edgePoints       []*GeoPoint
}

// NewGeoSouthRectangle constructs a GeoSouthRectangle covering from the south
// pole up to topLat, between leftLon and rightLon. The longitude extent must
// not exceed PI (use GeoWideSouthRectangle for wider slices).
//
// Port of org.apache.lucene.spatial3d.geom.GeoSouthRectangle(PlanetModel,double,double,double).
func NewGeoSouthRectangle(pm *PlanetModel, topLat, leftLon, rightLon float64) (*GeoSouthRectangle, error) {
	if topLat > math.Pi*0.5 || topLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoSouthRectangle: top latitude out of range: %g", topLat)
	}
	if leftLon < -math.Pi || leftLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoSouthRectangle: left longitude out of range: %g", leftLon)
	}
	if rightLon < -math.Pi || rightLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoSouthRectangle: right longitude out of range: %g", rightLon)
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += 2.0 * math.Pi
	}
	if extent > math.Pi {
		return nil, fmt.Errorf("geom: GeoSouthRectangle: width of rectangle too great: %g", extent)
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

	topPlane := NewSidedPlaneHorizontal(&pm.SouthPole.Vector, pm, sinTopLat)
	leftPlane := NewSidedPlaneVertical(&urhc.Vector, cosLeftLon, sinLeftLon)
	rightPlane := NewSidedPlaneVertical(&ulhc.Vector, cosRightLon, sinRightLon)
	backingPlane := NewSidedPlane(&centerPoint.Vector, cosMiddleLon, sinMiddleLon, 0.0, 0.0)

	if topPlane == nil || leftPlane == nil || rightPlane == nil || backingPlane == nil {
		return nil, fmt.Errorf("geom: GeoSouthRectangle: degenerate bounding planes")
	}

	r := &GeoSouthRectangle{GeoBaseBBox: makeBBox(pm)}
	r.f = geoSouthRectangleFields{
		topLat:           topLat,
		leftLon:          leftLon,
		rightLon:         rightLon,
		cosMiddleLat:     cosMiddleLat,
		ulhc:             ulhc,
		urhc:             urhc,
		topPlane:         topPlane,
		leftPlane:        leftPlane,
		rightPlane:       rightPlane,
		backingPlane:     backingPlane,
		topPlanePoints:   []*GeoPoint{ulhc, urhc},
		leftPlanePoints:  []*GeoPoint{ulhc, pm.SouthPole},
		rightPlanePoints: []*GeoPoint{urhc, pm.SouthPole},
		centerPoint:      centerPoint,
		edgePoints:       []*GeoPoint{pm.SouthPole},
	}
	return r, nil
}

// IsWithin reports whether (x,y,z) is inside the south rectangle.
//
// Port of GeoSouthRectangle.isWithin.
func (r *GeoSouthRectangle) IsWithin(x, y, z float64) bool {
	return r.f.backingPlane.IsWithin(x, y, z) &&
		r.f.topPlane.IsWithin(x, y, z) &&
		r.f.leftPlane.IsWithin(x, y, z) &&
		r.f.rightPlane.IsWithin(x, y, z)
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoSouthRectangle.getRadius.
func (r *GeoSouthRectangle) GetRadius() float64 {
	centerAngle := (r.f.rightLon - (r.f.rightLon+r.f.leftLon)*0.5) * r.f.cosMiddleLat
	topAngle := r.f.centerPoint.ArcDistance(&r.f.urhc.Vector)
	return math.Max(centerAngle, topAngle)
}

// GetCenter returns the center point.
func (r *GeoSouthRectangle) GetCenter() *GeoPoint { return r.f.centerPoint }

// GetEdgePoints returns a sample point on the edge (the south pole).
func (r *GeoSouthRectangle) GetEdgePoints() []*GeoPoint { return r.f.edgePoints }

// Expand returns a rectangle expanded by the given angle.
//
// Port of GeoSouthRectangle.expand.
func (r *GeoSouthRectangle) Expand(angle float64) GeoBBox {
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

// Intersects reports whether the plane p crosses the south rectangle.
//
// Port of GeoSouthRectangle.intersects(Plane,GeoPoint[],Membership...).
func (r *GeoSouthRectangle) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := r.PlanetModelField
	return p.Intersects(pm, &r.f.topPlane.Plane, notablePoints, r.f.topPlanePoints, bounds, r.f.leftPlane, r.f.rightPlane) ||
		p.Intersects(pm, &r.f.leftPlane.Plane, notablePoints, r.f.leftPlanePoints, bounds, r.f.rightPlane, r.f.topPlane) ||
		p.Intersects(pm, &r.f.rightPlane.Plane, notablePoints, r.f.rightPlanePoints, bounds, r.f.leftPlane, r.f.topPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoSouthRectangle.intersects(GeoShape).
func (r *GeoSouthRectangle) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&r.f.topPlane.Plane, r.f.topPlanePoints, r.f.leftPlane, r.f.rightPlane) ||
		geoShape.Intersects(&r.f.leftPlane.Plane, r.f.leftPlanePoints, r.f.rightPlane, r.f.topPlane) ||
		geoShape.Intersects(&r.f.rightPlane.Plane, r.f.rightPlanePoints, r.f.leftPlane, r.f.topPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (r *GeoSouthRectangle) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(r.IsWithin, r.f.edgePoints, r.intersectsShape, geoShape)
}

// GetBounds accumulates the south rectangle's bounding information.
//
// Port of GeoSouthRectangle.getBounds.
func (r *GeoSouthRectangle) GetBounds(bounds Bounds) {
	geoBaseGetBounds(r, r.PlanetModelField, bounds)
	pm := r.PlanetModelField
	bounds.
		AddHorizontalPlane(pm, r.f.topLat, &r.f.topPlane.Plane, r.f.leftPlane, r.f.rightPlane).
		AddVerticalPlane(pm, r.f.leftLon, &r.f.leftPlane.Plane, r.f.topPlane, r.f.rightPlane).
		AddVerticalPlane(pm, r.f.rightLon, &r.f.rightPlane.Plane, r.f.topPlane, r.f.leftPlane).
		AddPoint(r.f.urhc).
		AddPoint(r.f.ulhc).
		AddPoint(pm.SouthPole)
}

// String returns a debug representation.
func (r *GeoSouthRectangle) String() string {
	return fmt.Sprintf("GeoSouthRectangle: {planetmodel=%v, toplat=%g(%g), leftlon=%g(%g), rightlon=%g(%g)}",
		r.PlanetModelField,
		r.f.topLat, r.f.topLat*180.0/math.Pi,
		r.f.leftLon, r.f.leftLon*180.0/math.Pi,
		r.f.rightLon, r.f.rightLon*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoSouthRectangle)(nil)
	_ GeoShape = (*GeoSouthRectangle)(nil)
)
