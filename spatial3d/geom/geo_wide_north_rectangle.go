// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// geoWideNorthRectangleFields holds all computed fields for GeoWideNorthRectangle.
type geoWideNorthRectangleFields struct {
	bottomLat         float64
	leftLon           float64
	rightLon          float64
	cosMiddleLat      float64
	lrhc, llhc        *GeoPoint
	bottomPlane       *SidedPlane
	leftPlane         *SidedPlane
	rightPlane        *SidedPlane
	bottomPlanePoints []*GeoPoint
	leftPlanePoints   []*GeoPoint
	rightPlanePoints  []*GeoPoint
	centerPoint       *GeoPoint
	eitherBound       Membership
	edgePoints        []*GeoPoint
}

// wideNorthEitherBound is a Membership satisfied when the point is within
// either the left or the right longitude plane.
//
// Port of GeoWideNorthRectangle.EitherBound.
type wideNorthEitherBound struct {
	leftPlane  *SidedPlane
	rightPlane *SidedPlane
}

func (e *wideNorthEitherBound) IsWithin(x, y, z float64) bool {
	return e.leftPlane.IsWithin(x, y, z) || e.rightPlane.IsWithin(x, y, z)
}

// NewGeoWideNorthRectangle constructs a GeoWideNorthRectangle. The longitude
// extent must be at least PI - MinimumAngularResolution.
//
// Port of org.apache.lucene.spatial3d.geom.GeoWideNorthRectangle(PlanetModel,...).
func NewGeoWideNorthRectangle(pm *PlanetModel, bottomLat, leftLon, rightLon float64) (*GeoWideNorthRectangle, error) {
	if bottomLat > math.Pi*0.5 || bottomLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoWideNorthRectangle: bottom latitude out of range: %g", bottomLat)
	}
	if leftLon < -math.Pi || leftLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoWideNorthRectangle: left longitude out of range: %g", leftLon)
	}
	if rightLon < -math.Pi || rightLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoWideNorthRectangle: right longitude out of range: %g", rightLon)
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += 2.0 * math.Pi
	}
	if extent < minWideExtent {
		return nil, fmt.Errorf("geom: GeoWideNorthRectangle: width of rectangle too small: %g", extent)
	}

	sinBottomLat, cosBottomLat := math.Sin(bottomLat), math.Cos(bottomLat)
	sinLeftLon, cosLeftLon := math.Sin(leftLon), math.Cos(leftLon)
	sinRightLon, cosRightLon := math.Sin(rightLon), math.Cos(rightLon)

	lrhc := NewGeoPointTrigLatLon(pm, sinBottomLat, sinRightLon, cosBottomLat, cosRightLon, bottomLat, rightLon)
	llhc := NewGeoPointTrigLatLon(pm, sinBottomLat, sinLeftLon, cosBottomLat, cosLeftLon, bottomLat, leftLon)

	middleLat := (math.Pi*0.5 + bottomLat) * 0.5
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

	bottomPlane := NewSidedPlaneHorizontal(&centerPoint.Vector, pm, sinBottomLat)
	leftPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosLeftLon, sinLeftLon)
	rightPlane := NewSidedPlaneVertical(&centerPoint.Vector, cosRightLon, sinRightLon)

	if bottomPlane == nil || leftPlane == nil || rightPlane == nil {
		return nil, fmt.Errorf("geom: GeoWideNorthRectangle: degenerate bounding planes")
	}

	eitherBound := &wideNorthEitherBound{leftPlane: leftPlane, rightPlane: rightPlane}

	r := &GeoWideNorthRectangle{GeoBaseBBox: makeBBox(pm)}
	r.f = geoWideNorthRectangleFields{
		bottomLat:         bottomLat,
		leftLon:           leftLon,
		rightLon:          rightLon,
		cosMiddleLat:      cosMiddleLat,
		lrhc:              lrhc,
		llhc:              llhc,
		bottomPlane:       bottomPlane,
		leftPlane:         leftPlane,
		rightPlane:        rightPlane,
		bottomPlanePoints: []*GeoPoint{llhc, lrhc},
		leftPlanePoints:   []*GeoPoint{pm.NorthPole, llhc},
		rightPlanePoints:  []*GeoPoint{pm.NorthPole, lrhc},
		centerPoint:       centerPoint,
		eitherBound:       eitherBound,
		edgePoints:        []*GeoPoint{pm.NorthPole},
	}
	return r, nil
}

// IsWithin reports whether (x,y,z) is inside the wide north rectangle.
//
// Port of GeoWideNorthRectangle.isWithin.
func (r *GeoWideNorthRectangle) IsWithin(x, y, z float64) bool {
	return r.f.bottomPlane.IsWithin(x, y, z) &&
		(r.f.leftPlane.IsWithin(x, y, z) || r.f.rightPlane.IsWithin(x, y, z))
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoWideNorthRectangle.getRadius.
func (r *GeoWideNorthRectangle) GetRadius() float64 {
	centerAngle := (r.f.rightLon - (r.f.rightLon+r.f.leftLon)*0.5) * r.f.cosMiddleLat
	bottomAngle := r.f.centerPoint.ArcDistance(&r.f.llhc.Vector)
	return math.Max(centerAngle, bottomAngle)
}

// GetCenter returns the center point.
func (r *GeoWideNorthRectangle) GetCenter() *GeoPoint { return r.f.centerPoint }

// GetEdgePoints returns a sample edge point (the north pole).
func (r *GeoWideNorthRectangle) GetEdgePoints() []*GeoPoint { return r.f.edgePoints }

// Expand returns a rectangle expanded by the given angle.
//
// Port of GeoWideNorthRectangle.expand.
func (r *GeoWideNorthRectangle) Expand(angle float64) GeoBBox {
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
	bbox, err := MakeGeoBBox(r.PlanetModelField, math.Pi*0.5, newBottomLat, newLeftLon, newRightLon)
	if err != nil {
		return nil
	}
	return bbox
}

// Intersects reports whether the plane p crosses the wide north rectangle.
//
// Port of GeoWideNorthRectangle.intersects(Plane,GeoPoint[],Membership...).
func (r *GeoWideNorthRectangle) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := r.PlanetModelField
	return p.Intersects(pm, &r.f.bottomPlane.Plane, notablePoints, r.f.bottomPlanePoints, bounds, r.f.eitherBound) ||
		p.Intersects(pm, &r.f.leftPlane.Plane, notablePoints, r.f.leftPlanePoints, bounds, r.f.bottomPlane) ||
		p.Intersects(pm, &r.f.rightPlane.Plane, notablePoints, r.f.rightPlanePoints, bounds, r.f.bottomPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoWideNorthRectangle.intersects(GeoShape).
func (r *GeoWideNorthRectangle) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&r.f.bottomPlane.Plane, r.f.bottomPlanePoints, r.f.eitherBound) ||
		geoShape.Intersects(&r.f.leftPlane.Plane, r.f.leftPlanePoints, r.f.bottomPlane) ||
		geoShape.Intersects(&r.f.rightPlane.Plane, r.f.rightPlanePoints, r.f.bottomPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (r *GeoWideNorthRectangle) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(r.IsWithin, r.f.edgePoints, r.intersectsShape, geoShape)
}

// GetBounds accumulates the wide north rectangle's bounding information.
//
// Port of GeoWideNorthRectangle.getBounds.
func (r *GeoWideNorthRectangle) GetBounds(bounds Bounds) {
	geoBaseGetBounds(r, r.PlanetModelField, bounds)
	pm := r.PlanetModelField
	bounds.
		IsWide().
		AddHorizontalPlane(pm, r.f.bottomLat, &r.f.bottomPlane.Plane, r.f.eitherBound).
		AddVerticalPlane(pm, r.f.leftLon, &r.f.leftPlane.Plane, r.f.bottomPlane).
		AddVerticalPlane(pm, r.f.rightLon, &r.f.rightPlane.Plane, r.f.bottomPlane).
		AddIntersection(pm, &r.f.leftPlane.Plane, &r.f.rightPlane.Plane, r.f.bottomPlane).
		AddPoint(r.f.llhc).
		AddPoint(r.f.lrhc).
		AddPoint(pm.NorthPole)
}

// String returns a debug representation.
func (r *GeoWideNorthRectangle) String() string {
	return fmt.Sprintf("GeoWideNorthRectangle: {planetmodel=%v, bottomlat=%g(%g), leftlon=%g(%g), rightlon=%g(%g)}",
		r.PlanetModelField,
		r.f.bottomLat, r.f.bottomLat*180.0/math.Pi,
		r.f.leftLon, r.f.leftLon*180.0/math.Pi,
		r.f.rightLon, r.f.rightLon*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoWideNorthRectangle)(nil)
	_ GeoShape = (*GeoWideNorthRectangle)(nil)
)
