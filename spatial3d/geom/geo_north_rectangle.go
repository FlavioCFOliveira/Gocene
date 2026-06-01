// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"fmt"
	"math"
)

// GeoNorthRectangle fields holds the computed state added by the constructor.
// The type itself is declared in shapes.go.

// geoNorthRectangleFields holds all computed fields for GeoNorthRectangle.
type geoNorthRectangleFields struct {
	bottomLat         float64
	leftLon           float64
	rightLon          float64
	cosMiddleLat      float64
	lrhc, llhc        *GeoPoint
	bottomPlane       *SidedPlane
	leftPlane         *SidedPlane
	rightPlane        *SidedPlane
	backingPlane      *SidedPlane
	bottomPlanePoints []*GeoPoint
	leftPlanePoints   []*GeoPoint
	rightPlanePoints  []*GeoPoint
	centerPoint       *GeoPoint
	edgePoints        []*GeoPoint
}

// NewGeoNorthRectangle constructs a GeoNorthRectangle covering from bottomLat
// to the north pole, between leftLon and rightLon. The longitude extent must
// not exceed PI (use GeoWideNorthRectangle for wider slices).
//
// Port of org.apache.lucene.spatial3d.geom.GeoNorthRectangle(PlanetModel,double,double,double).
func NewGeoNorthRectangle(pm *PlanetModel, bottomLat, leftLon, rightLon float64) (*GeoNorthRectangle, error) {
	if bottomLat > math.Pi*0.5 || bottomLat < -math.Pi*0.5 {
		return nil, fmt.Errorf("geom: GeoNorthRectangle: bottom latitude out of range: %g", bottomLat)
	}
	if leftLon < -math.Pi || leftLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoNorthRectangle: left longitude out of range: %g", leftLon)
	}
	if rightLon < -math.Pi || rightLon > math.Pi {
		return nil, fmt.Errorf("geom: GeoNorthRectangle: right longitude out of range: %g", rightLon)
	}
	extent := rightLon - leftLon
	if extent < 0.0 {
		extent += 2.0 * math.Pi
	}
	if extent > math.Pi {
		return nil, fmt.Errorf("geom: GeoNorthRectangle: width of rectangle too great: %g", extent)
	}

	sinBottomLat, cosBottomLat := math.Sin(bottomLat), math.Cos(bottomLat)
	sinLeftLon, cosLeftLon := math.Sin(leftLon), math.Cos(leftLon)
	sinRightLon, cosRightLon := math.Sin(rightLon), math.Cos(rightLon)

	lrhc := NewGeoPointTrigLatLon(pm, sinBottomLat, sinRightLon, cosBottomLat, cosRightLon, bottomLat, rightLon)
	llhc := NewGeoPointTrigLatLon(pm, sinBottomLat, sinLeftLon, cosBottomLat, cosLeftLon, bottomLat, leftLon)

	middleLat := (math.Pi*0.5 + bottomLat) * 0.5
	sinMiddleLat := math.Sin(middleLat)
	cosMiddleLat := math.Cos(middleLat)

	// Normalize so that rightLon >= leftLon for midpoint computation.
	nRightLon := rightLon
	for leftLon > nRightLon {
		nRightLon += math.Pi * 2.0
	}
	middleLon := (leftLon + nRightLon) * 0.5
	sinMiddleLon := math.Sin(middleLon)
	cosMiddleLon := math.Cos(middleLon)

	centerPoint := NewGeoPointTrig(pm, sinMiddleLat, sinMiddleLon, cosMiddleLat, cosMiddleLon)

	// bottomPlane is sided so that the north pole is "inside".
	bottomPlane := NewSidedPlaneHorizontal(&pm.NorthPole.Vector, pm, sinBottomLat)
	leftPlane := NewSidedPlaneVertical(&lrhc.Vector, cosLeftLon, sinLeftLon)
	rightPlane := NewSidedPlaneVertical(&llhc.Vector, cosRightLon, sinRightLon)
	backingPlane := NewSidedPlane(&centerPoint.Vector, cosMiddleLon, sinMiddleLon, 0.0, 0.0)

	if bottomPlane == nil || leftPlane == nil || rightPlane == nil || backingPlane == nil {
		return nil, fmt.Errorf("geom: GeoNorthRectangle: degenerate bounding planes")
	}

	r := &GeoNorthRectangle{GeoBaseBBox: makeBBox(pm)}
	r.f = geoNorthRectangleFields{
		bottomLat:         bottomLat,
		leftLon:           leftLon,
		rightLon:          rightLon,
		cosMiddleLat:      cosMiddleLat,
		lrhc:              lrhc,
		llhc:              llhc,
		bottomPlane:       bottomPlane,
		leftPlane:         leftPlane,
		rightPlane:        rightPlane,
		backingPlane:      backingPlane,
		bottomPlanePoints: []*GeoPoint{llhc, lrhc},
		leftPlanePoints:   []*GeoPoint{pm.NorthPole, llhc},
		rightPlanePoints:  []*GeoPoint{pm.NorthPole, lrhc},
		centerPoint:       centerPoint,
		edgePoints:        []*GeoPoint{pm.NorthPole},
	}
	return r, nil
}

// IsWithin reports whether (x,y,z) is inside the north rectangle.
//
// Port of GeoNorthRectangle.isWithin.
func (r *GeoNorthRectangle) IsWithin(x, y, z float64) bool {
	return r.f.backingPlane.IsWithin(x, y, z) &&
		r.f.bottomPlane.IsWithin(x, y, z) &&
		r.f.leftPlane.IsWithin(x, y, z) &&
		r.f.rightPlane.IsWithin(x, y, z)
}

// GetRadius returns the bounding-circle radius.
//
// Port of GeoNorthRectangle.getRadius.
func (r *GeoNorthRectangle) GetRadius() float64 {
	centerAngle := (r.f.rightLon - (r.f.rightLon+r.f.leftLon)*0.5) * r.f.cosMiddleLat
	bottomAngle := r.f.centerPoint.ArcDistance(&r.f.llhc.Vector)
	return math.Max(centerAngle, bottomAngle)
}

// GetCenter returns the center point.
func (r *GeoNorthRectangle) GetCenter() *GeoPoint { return r.f.centerPoint }

// GetEdgePoints returns a sample point on the edge (the north pole).
func (r *GeoNorthRectangle) GetEdgePoints() []*GeoPoint { return r.f.edgePoints }

// Expand returns a rectangle expanded by the given angle.
//
// Port of GeoNorthRectangle.expand.
func (r *GeoNorthRectangle) Expand(angle float64) GeoBBox {
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

// Intersects reports whether the plane p crosses the north rectangle.
//
// Port of GeoNorthRectangle.intersects(Plane,GeoPoint[],Membership...).
func (r *GeoNorthRectangle) Intersects(p *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool {
	pm := r.PlanetModelField
	return p.Intersects(pm, &r.f.bottomPlane.Plane, notablePoints, r.f.bottomPlanePoints, bounds, r.f.leftPlane, r.f.rightPlane) ||
		p.Intersects(pm, &r.f.leftPlane.Plane, notablePoints, r.f.leftPlanePoints, bounds, r.f.rightPlane, r.f.bottomPlane) ||
		p.Intersects(pm, &r.f.rightPlane.Plane, notablePoints, r.f.rightPlanePoints, bounds, r.f.leftPlane, r.f.bottomPlane)
}

// intersectsShape is the GeoShape-level intersection check used by GetRelationship.
//
// Port of GeoNorthRectangle.intersects(GeoShape).
func (r *GeoNorthRectangle) intersectsShape(geoShape GeoShape) bool {
	return geoShape.Intersects(&r.f.bottomPlane.Plane, r.f.bottomPlanePoints, r.f.leftPlane, r.f.rightPlane) ||
		geoShape.Intersects(&r.f.leftPlane.Plane, r.f.leftPlanePoints, r.f.rightPlane, r.f.bottomPlane) ||
		geoShape.Intersects(&r.f.rightPlane.Plane, r.f.rightPlanePoints, r.f.leftPlane, r.f.bottomPlane)
}

// GetRelationship returns the spatial relationship between this shape and geoShape.
//
// Port of GeoBaseAreaShape.getRelationship.
func (r *GeoNorthRectangle) GetRelationship(geoShape GeoShape) int {
	return geoAreaGetRelationship(r.IsWithin, r.f.edgePoints, r.intersectsShape, geoShape)
}

// GetBounds accumulates the north rectangle's bounding information.
//
// Port of GeoNorthRectangle.getBounds.
func (r *GeoNorthRectangle) GetBounds(bounds Bounds) {
	geoBaseGetBounds(r, r.PlanetModelField, bounds)
	pm := r.PlanetModelField
	bounds.
		AddHorizontalPlane(pm, r.f.bottomLat, &r.f.bottomPlane.Plane, r.f.leftPlane, r.f.rightPlane).
		AddVerticalPlane(pm, r.f.leftLon, &r.f.leftPlane.Plane, r.f.bottomPlane, r.f.rightPlane).
		AddVerticalPlane(pm, r.f.rightLon, &r.f.rightPlane.Plane, r.f.bottomPlane, r.f.leftPlane).
		AddPoint(r.f.llhc).
		AddPoint(r.f.lrhc).
		AddPoint(pm.NorthPole)
}

// String returns a debug representation.
func (r *GeoNorthRectangle) String() string {
	return fmt.Sprintf("GeoNorthRectangle: {planetmodel=%v, bottomlat=%g(%g), leftlon=%g(%g), rightlon=%g(%g)}",
		r.PlanetModelField,
		r.f.bottomLat, r.f.bottomLat*180.0/math.Pi,
		r.f.leftLon, r.f.leftLon*180.0/math.Pi,
		r.f.rightLon, r.f.rightLon*180.0/math.Pi)
}

var (
	_ GeoBBox  = (*GeoNorthRectangle)(nil)
	_ GeoShape = (*GeoNorthRectangle)(nil)
)
