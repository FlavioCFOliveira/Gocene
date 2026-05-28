// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// NewGeoRectangle constructs a GeoRectangle with the given latitude/longitude
// bounds (radians). The longitude extent must be no greater than PI.
//
// Port of org.apache.lucene.spatial3d.geom.GeoRectangle(PlanetModel,double,double,double,double).
func NewGeoRectangle(pm *PlanetModel, topLat, bottomLat, leftLon, rightLon float64) (*GeoRectangle, error) {
	if topLat > math.Pi*0.5 || topLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoRectangle: top latitude out of range: %g", topLat)
	}
	if bottomLat > math.Pi*0.5 || bottomLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoRectangle: bottom latitude out of range: %g", bottomLat)
	}
	if topLat < bottomLat {
		return nil, fmt.Errorf("geom: GeoRectangle: top latitude less than bottom latitude")
	}
	if leftLon < -math.Pi || leftLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoRectangle: left longitude out of range: %g", leftLon)
	}
	if rightLon < -math.Pi || rightLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoRectangle: right longitude out of range: %g", rightLon)
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += 2.0 * math.Pi
	}
	if extent > math.Pi {
		return nil, fmt.Errorf("geom: GeoRectangle: width of rectangle too great: %g", extent)
	}

	r := &GeoRectangle{
		GeoBaseBBox: makeBBox(pm),
		topLat:      topLat,
		bottomLat:   bottomLat,
		leftLon:     leftLon,
		rightLon:    rightLon,
	}

	sinTopLat, cosTopLat := math.Sin(topLat), math.Cos(topLat)
	sinBottomLat, cosBottomLat := math.Sin(bottomLat), math.Cos(bottomLat)
	sinLeftLon, cosLeftLon := math.Sin(leftLon), math.Cos(leftLon)
	sinRightLon, cosRightLon := math.Sin(rightLon), math.Cos(rightLon)

	r.ulhc = NewGeoPointTrigLatLon(pm, sinTopLat, sinLeftLon, cosTopLat, cosLeftLon, topLat, leftLon)
	r.urhc = NewGeoPointTrigLatLon(pm, sinTopLat, sinRightLon, cosTopLat, cosRightLon, topLat, rightLon)
	r.lrhc = NewGeoPointTrigLatLon(pm, sinBottomLat, sinRightLon, cosBottomLat, cosRightLon, bottomLat, rightLon)
	r.llhc = NewGeoPointTrigLatLon(pm, sinBottomLat, sinLeftLon, cosBottomLat, cosLeftLon, bottomLat, leftLon)

	middleLat := (topLat + bottomLat) * 0.5
	sinMiddleLat := math.Sin(middleLat)
	r.cosMiddleLat = math.Cos(middleLat)
	// Normalise so the right longitude exceeds the left.
	nRightLon := rightLon
	for leftLon > nRightLon {
		nRightLon += math.Pi * 2.0
	}
	middleLon := (leftLon + nRightLon) * 0.5
	sinMiddleLon := math.Sin(middleLon)
	cosMiddleLon := math.Cos(middleLon)

	r.centerPoint = NewGeoPointTrig(pm, sinMiddleLat, sinMiddleLon, r.cosMiddleLat, cosMiddleLon)

	r.topPlane = NewSidedPlaneHorizontal(&r.llhc.Vector, pm, sinTopLat)
	r.bottomPlane = NewSidedPlaneHorizontal(&r.urhc.Vector, pm, sinBottomLat)
	r.leftPlane = NewSidedPlaneVertical(&r.urhc.Vector, cosLeftLon, sinLeftLon)
	r.rightPlane = NewSidedPlaneVertical(&r.llhc.Vector, cosRightLon, sinRightLon)
	r.backingPlane = NewSidedPlane(&r.centerPoint.Vector, cosMiddleLon, sinMiddleLon, 0.0, 0.0)
	if r.topPlane == nil || r.bottomPlane == nil || r.leftPlane == nil ||
		r.rightPlane == nil || r.backingPlane == nil {
		return nil, fmt.Errorf("geom: GeoRectangle: degenerate bounding planes")
	}

	r.topPlanePoints = []*GeoPoint{r.ulhc, r.urhc}
	r.bottomPlanePoints = []*GeoPoint{r.llhc, r.lrhc}
	r.leftPlanePoints = []*GeoPoint{r.ulhc, r.llhc}
	r.rightPlanePoints = []*GeoPoint{r.urhc, r.lrhc}
	r.rectangleEdgePoints = []*GeoPoint{r.ulhc}

	return r, nil
}

// IsWithin reports whether (x,y,z) is inside the rectangle.
//
// Port of GeoRectangle.isWithin.
func (r *GeoRectangle) IsWithin(x, y, z float64) bool {
	return r.backingPlane.IsWithin(x, y, z) &&
		r.topPlane.IsWithin(x, y, z) &&
		r.bottomPlane.IsWithin(x, y, z) &&
		r.leftPlane.IsWithin(x, y, z) &&
		r.rightPlane.IsWithin(x, y, z)
}

// GetRadius returns the bounding-circle radius of the rectangle.
//
// Port of GeoRectangle.getRadius.
func (r *GeoRectangle) GetRadius() float64 {
	centerAngle := (r.rightLon - (r.rightLon+r.leftLon)*0.5) * r.cosMiddleLat
	topAngle := r.centerPoint.ArcDistance(&r.urhc.Vector)
	bottomAngle := r.centerPoint.ArcDistance(&r.llhc.Vector)
	return math.Max(centerAngle, math.Max(topAngle, bottomAngle))
}

// GetCenter returns the rectangle's centre point.
func (r *GeoRectangle) GetCenter() *GeoPoint { return r.centerPoint }

// GetEdgePoints returns sample points on the rectangle edge.
func (r *GeoRectangle) GetEdgePoints() []*GeoPoint { return r.rectangleEdgePoints }

// Expand returns a rectangle expanded by the given angle (radians).
//
// Port of GeoRectangle.expand.
func (r *GeoRectangle) Expand(angle float64) GeoBBox {
	newTopLat := r.topLat + angle
	newBottomLat := r.bottomLat - angle
	currentLonSpan := r.rightLon - r.leftLon
	if currentLonSpan < 0.0 {
		currentLonSpan += math.Pi * 2.0
	}
	newLeftLon := r.leftLon - angle
	newRightLon := r.rightLon + angle
	if currentLonSpan+2.0*angle >= math.Pi*2.0 {
		newLeftLon = -math.Pi
		newRightLon = math.Pi
	}
	bbox, err := MakeGeoBBox(r.PlanetModelField, newTopLat, newBottomLat, newLeftLon, newRightLon)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p (within bounds) crosses the rectangle.
//
// Port of GeoRectangle.intersects(Plane,GeoPoint[],Membership...).
func (r *GeoRectangle) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := r.PlanetModelField
	return p.Intersects(pm, &r.topPlane.Plane, notablePoints, r.topPlanePoints, bounds, r.bottomPlane, r.leftPlane, r.rightPlane) ||
		p.Intersects(pm, &r.bottomPlane.Plane, notablePoints, r.bottomPlanePoints, bounds, r.topPlane, r.leftPlane, r.rightPlane) ||
		p.Intersects(pm, &r.leftPlane.Plane, notablePoints, r.leftPlanePoints, bounds, r.rightPlane, r.topPlane, r.bottomPlane) ||
		p.Intersects(pm, &r.rightPlane.Plane, notablePoints, r.rightPlanePoints, bounds, r.leftPlane, r.topPlane, r.bottomPlane)
}

// GetBounds accumulates the rectangle's bounding information.
//
// Port of GeoRectangle.getBounds.
func (r *GeoRectangle) GetBounds(bounds Bounds) {
	geoBaseGetBounds(r, r.PlanetModelField, bounds)
	pm := r.PlanetModelField
	bounds.
		AddHorizontalPlane(pm, r.topLat, &r.topPlane.Plane, r.bottomPlane, r.leftPlane, r.rightPlane).
		AddVerticalPlane(pm, r.rightLon, &r.rightPlane.Plane, r.topPlane, r.bottomPlane, r.leftPlane).
		AddHorizontalPlane(pm, r.bottomLat, &r.bottomPlane.Plane, r.topPlane, r.leftPlane, r.rightPlane).
		AddVerticalPlane(pm, r.leftLon, &r.leftPlane.Plane, r.topPlane, r.bottomPlane, r.rightPlane).
		AddPoint(r.ulhc).
		AddPoint(r.urhc).
		AddPoint(r.llhc).
		AddPoint(r.lrhc)
}

// String returns a debug representation.
func (r *GeoRectangle) String() string {
	return fmt.Sprintf("GeoRectangle: {planetmodel=%v, toplat=%g, bottomlat=%g, leftlon=%g, rightlon=%g}",
		r.PlanetModelField, r.topLat, r.bottomLat, r.leftLon, r.rightLon)
}

var (
	_ GeoBBox  = (*GeoRectangle)(nil)
	_ GeoShape = (*GeoRectangle)(nil)
)
