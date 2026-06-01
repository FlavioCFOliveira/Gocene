// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoWideRectangleFields holds all computed fields for GeoWideRectangle.
type geoWideRectangleFields struct {
	topLat           float64
	bottomLat        float64
	leftLon          float64
	rightLon         float64
	cosMiddleLat     float64
	ulhc, urhc       *GeoPoint
	lrhc, llhc       *GeoPoint
	topPlane         *SidedPlane
	bottomPlane      *SidedPlane
	leftPlane        *SidedPlane
	rightPlane       *SidedPlane
	topPlanePoints   []*GeoPoint
	bottomPlanePoints []*GeoPoint
	leftPlanePoints  []*GeoPoint
	rightPlanePoints []*GeoPoint
	centerPoint      *GeoPoint
	eitherBound      Membership
	edgePoints       []*GeoPoint
}

// wideRectEitherBound is a Membership that is true when the point is within
// either the left or the right longitude plane of a wide rectangle.
//
// Port of GeoWideRectangle.EitherBound.
type wideRectEitherBound struct {
	leftPlane  *SidedPlane
	rightPlane *SidedPlane
}

func (e *wideRectEitherBound) IsWithin(x, y, z float64) bool {
	return e.leftPlane.IsWithin(x, y, z) || e.rightPlane.IsWithin(x, y, z)
}

// NewGeoWideRectangle constructs a GeoWideRectangle. The longitude extent must
// be at least PI - MinimumAngularResolution.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideRectangle(PlanetModel,...).
func NewGeoWideRectangle(pm *PlanetModel, topLat, bottomLat, leftLon, rightLon float64) (*GeoWideRectangle, error) {
	if topLat > math.Pi*0.5 || topLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoWideRectangle: top latitude out of range: %g", topLat)
	}
	if bottomLat > math.Pi*0.5 || bottomLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoWideRectangle: bottom latitude out of range: %g", bottomLat)
	}
	if topLat < bottomLat {
		return nil, fmt.Errorf("geom: GeoWideRectangle: top latitude less than bottom latitude")
	}
	if leftLon < -math.Pi || leftLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoWideRectangle: left longitude out of range: %g", leftLon)
	}
	if rightLon < -math.Pi || rightLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoWideRectangle: right longitude out of range: %g", rightLon)
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += 2.0 * math.Pi
	}
	if extent < minWideExtent {
		return nil, fmt.Errorf("geom: GeoWideRectangle: width of rectangle too small: %g", extent)
	}

	sinTopLat, cosTopLat := math.Sin(topLat), math.Cos(topLat)
	sinBottomLat, cosBottomLat := math.Sin(bottomLat), math.Cos(bottomLat)
	sinLeftLon, cosLeftLon := math.Sin(leftLon), math.Cos(leftLon)
	sinRightLon, cosRightLon := math.Sin(rightLon), math.Cos(rightLon)

	ulhc := NewGeoPointTrigLatLon(pm, sinTopLat, sinLeftLon, cosTopLat, cosLeftLon, topLat, leftLon)
	urhc := NewGeoPointTrigLatLon(pm, sinTopLat, sinRightLon, cosTopLat, cosRightLon, topLat, rightLon)
	lrhc := NewGeoPointTrigLatLon(pm, sinBottomLat, sinRightLon, cosBottomLat, cosRightLon, bottomLat, rightLon)
	llhc := NewGeoPointTrigLatLon(pm, sinBottomLat, sinLeftLon, cosBottomLat, cosLeftLon, bottomLat, leftLon)

	middleLat := (topLat + bottomLat) * 0.5
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
	bottomPlane := NewSidedPlaneHorizontal(&centerPoint.Vector, pm, sinBottomLat)
	leftPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosLeftLon, sinLeftLon)
	rightPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosRightLon, sinRightLon)

	if topPlane == nil || bottomPlane == nil || leftPlane == nil || rightPlane == nil {
		return nil, fmt.Errorf("geom: GeoWideRectangle: degenerate bounding planes")
	}

	eitherBound := &wideRectEitherBound{leftPlane: leftPlane, rightPlane: rightPlane}

	r := &GeoWideRectangle{GeoBaseBBox: makeBBox(pm)}
	r.f = geoWideRectangleFields{
		topLat:            topLat,
		bottomLat:         bottomLat,
		leftLon:           leftLon,
		rightLon:          rightLon,
		cosMiddleLat:      cosMiddleLat,
		ulhc:              ulhc,
		urhc:              urhc,
		lrhc:              lrhc,
		llhc:              llhc,
		topPlane:          topPlane,
		bottomPlane:       bottomPlane,
		leftPlane:         leftPlane,
		rightPlane:        rightPlane,
		topPlanePoints:    []*GeoPoint{ulhc, urhc},
		bottomPlanePoints: []*GeoPoint{llhc, lrhc},
		leftPlanePoints:   []*GeoPoint{ulhc, llhc},
		rightPlanePoints:  []*GeoPoint{urhc, lrhc},
		centerPoint:       centerPoint,
		eitherBound:       eitherBound,
		edgePoints:        []*GeoPoint{ulhc},
	}
	return r, nil
}

// IsWithin reports whether (x,y,z) is inside the wide rectangle.
//
// Port of GeoWideRectangle.isWithin.
func (r *GeoWideRectangle) IsWithin(x, y, z float64) bool {
	return r.f.topPlane.IsWithin(x, y, z) &&
		r.f.bottomPlane.IsWithin(x, y, z) &&
		(r.f.leftPlane.IsWithin(x, y, z) || r.f.rightPlane.IsWithin(x, y, z))
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoWideRectangle.getRadius.
func (r *GeoWideRectangle) GetRadius() float64 {
	centerAngle := (r.f.rightLon - (r.f.rightLon+r.f.leftLon)*0.5) * r.f.cosMiddleLat
	topAngle := r.f.centerPoint.ArcDistance(&r.f.urhc.Vector)
	bottomAngle := r.f.centerPoint.ArcDistance(&r.f.llhc.Vector)
	return math.Max(centerAngle, math.Max(topAngle, bottomAngle))
}

// GetCenter returns the center point.
func (r *GeoWideRectangle) GetCenter() *GeoPoint { return r.f.centerPoint }

// GetEdgePoints returns a sample edge point.
func (r *GeoWideRectangle) GetEdgePoints() []*GeoPoint { return r.f.edgePoints }

// Expand returns a rectangle expanded by the given angle.
//
// Port of GeoWideRectangle.expand.
func (r *GeoWideRectangle) Expand(angle float64) GeoBBox {
	newTopLat := r.f.topLat + angle
	newBottomLat := r.f.bottomLat - angle
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
	bbox, err := MakeGeoBBox(r.PlanetModelField, newTopLat, newBottomLat, newLeftLon, newRightLon)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the wide rectangle.
//
// Port of GeoWideRectangle.intersects(Plane,GeoPoint[],Membership...).
func (r *GeoWideRectangle) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := r.PlanetModelField
	return p.Intersects(pm, &r.f.topPlane.Plane, notablePoints, r.f.topPlanePoints, bounds, r.f.bottomPlane, r.f.eitherBound) ||
		p.Intersects(pm, &r.f.bottomPlane.Plane, notablePoints, r.f.bottomPlanePoints, bounds, r.f.topPlane, r.f.eitherBound) ||
		p.Intersects(pm, &r.f.leftPlane.Plane, notablePoints, r.f.leftPlanePoints, bounds, r.f.topPlane, r.f.bottomPlane) ||
		p.Intersects(pm, &r.f.rightPlane.Plane, notablePoints, r.f.rightPlanePoints, bounds, r.f.topPlane, r.f.bottomPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoWideRectangle.intersects(GeoShape).
func (r *GeoWideRectangle) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&r.f.topPlane.Plane, r.f.topPlanePoints, r.f.bottomPlane, r.f.eitherBound) ||
		geoShape.Intersects(&r.f.bottomPlane.Plane, r.f.bottomPlanePoints, r.f.topPlane, r.f.eitherBound) ||
		geoShape.Intersects(&r.f.leftPlane.Plane, r.f.leftPlanePoints, r.f.topPlane, r.f.bottomPlane) ||
		geoShape.Intersects(&r.f.rightPlane.Plane, r.f.rightPlanePoints, r.f.topPlane, r.f.bottomPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (r *GeoWideRectangle) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(r.IsWithin, r.f.edgePoints, r.intersectsShape, geoShape)
}

// GetBounds accumulates the wide rectangle's bounding information.
//
// Port of GeoWideRectangle.getBounds.
func (r *GeoWideRectangle) GetBounds(bounds Bounds) {
	geoBaseGetBounds(r, r.PlanetModelField, bounds)
	pm := r.PlanetModelField
	bounds.
		IsWide().
		AddHorizontalPlane(pm, r.f.topLat, &r.f.topPlane.Plane, r.f.bottomPlane, r.f.eitherBound).
		AddVerticalPlane(pm, r.f.rightLon, &r.f.rightPlane.Plane, r.f.topPlane, r.f.bottomPlane).
		AddHorizontalPlane(pm, r.f.bottomLat, &r.f.bottomPlane.Plane, r.f.topPlane, r.f.eitherBound).
		AddVerticalPlane(pm, r.f.leftLon, &r.f.leftPlane.Plane, r.f.topPlane, r.f.bottomPlane).
		AddIntersection(pm, &r.f.leftPlane.Plane, &r.f.rightPlane.Plane, r.f.topPlane, r.f.bottomPlane).
		AddPoint(r.f.ulhc).
		AddPoint(r.f.urhc).
		AddPoint(r.f.lrhc).
		AddPoint(r.f.llhc)
}

// String returns a debug representation.
func (r *GeoWideRectangle) String() string {
	return fmt.Sprintf("GeoWideRectangle: {planetmodel=%v, toplat=%g(%g), bottomlat=%g(%g), leftlon=%g(%g), rightlon=%g(%g)}",
		r.PlanetModelField,
		r.f.topLat, r.f.topLat*180.0/math.Pi,
		r.f.bottomLat, r.f.bottomLat*180.0/math.Pi,
		r.f.leftLon, r.f.leftLon*180.0/math.Pi,
		r.f.rightLon, r.f.rightLon*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoWideRectangle)(nil)
	_ GeoShape = (*GeoWideRectangle)(nil)
)
