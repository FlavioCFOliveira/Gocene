// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "math"

// GeoBaseBounds is the common abstract base for bounds accumulators.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBaseBounds (partial).
type GeoBaseBounds struct {
	noLongitudeBound      bool
	noTopLatitudeBound    bool
	noBottomLatitudeBound bool
	wide                  bool
}

// LatLonBounds accumulates geographic bounding values.
//
// Port of org.apache.lucene.spatial3d.geom.LatLonBounds.
type LatLonBounds struct {
	GeoBaseBounds
	minLatitude  float64
	maxLatitude  float64
	minLongitude float64
	maxLongitude float64
	hasLatLon    bool
}

// NewLatLonBounds creates an empty LatLonBounds.
func NewLatLonBounds() *LatLonBounds {
	return &LatLonBounds{
		minLatitude:  math.MaxFloat64,
		maxLatitude:  -math.MaxFloat64,
		minLongitude: math.MaxFloat64,
		maxLongitude: -math.MaxFloat64,
	}
}

// GetMinLatitude returns the minimum latitude seen so far.
func (b *LatLonBounds) GetMinLatitude() float64 { return b.minLatitude }

// GetMaxLatitude returns the maximum latitude seen so far.
func (b *LatLonBounds) GetMaxLatitude() float64 { return b.maxLatitude }

// GetMinLongitude returns the minimum longitude seen so far.
func (b *LatLonBounds) GetMinLongitude() float64 { return b.minLongitude }

// GetMaxLongitude returns the maximum longitude seen so far.
func (b *LatLonBounds) GetMaxLongitude() float64 { return b.maxLongitude }

// CheckNoLongitudeBound reports whether no longitude bound was signalled.
func (b *LatLonBounds) CheckNoLongitudeBound() bool { return b.noLongitudeBound }

// CheckNoTopLatitudeBound reports whether no top latitude bound was signalled.
func (b *LatLonBounds) CheckNoTopLatitudeBound() bool { return b.noTopLatitudeBound }

// CheckNoBottomLatitudeBound reports whether no bottom latitude bound was signalled.
func (b *LatLonBounds) CheckNoBottomLatitudeBound() bool { return b.noBottomLatitudeBound }

// AddPlane is a no-op stub — deferred to #2693.
func (b *LatLonBounds) AddPlane(_ *PlanetModel, _ *Plane, _ ...Membership) Bounds { return b }

// AddHorizontalPlane records the latitude bound.
func (b *LatLonBounds) AddHorizontalPlane(_ *PlanetModel, latitude float64, _ *Plane, _ ...Membership) Bounds {
	if latitude < b.minLatitude {
		b.minLatitude = latitude
	}
	if latitude > b.maxLatitude {
		b.maxLatitude = latitude
	}
	b.hasLatLon = true
	return b
}

// AddVerticalPlane records the longitude bound.
func (b *LatLonBounds) AddVerticalPlane(_ *PlanetModel, longitude float64, _ *Plane, _ ...Membership) Bounds {
	if longitude < b.minLongitude {
		b.minLongitude = longitude
	}
	if longitude > b.maxLongitude {
		b.maxLongitude = longitude
	}
	b.hasLatLon = true
	return b
}

// AddIntersection is a no-op stub — deferred to #2693.
func (b *LatLonBounds) AddIntersection(_ *PlanetModel, _, _ *Plane, _ ...Membership) Bounds {
	return b
}

// AddPoint records a GeoPoint's lat/lon.
func (b *LatLonBounds) AddPoint(p *GeoPoint) Bounds {
	lat := p.GetLatitude()
	lon := p.GetLongitude()
	if lat < b.minLatitude {
		b.minLatitude = lat
	}
	if lat > b.maxLatitude {
		b.maxLatitude = lat
	}
	if lon < b.minLongitude {
		b.minLongitude = lon
	}
	if lon > b.maxLongitude {
		b.maxLongitude = lon
	}
	b.hasLatLon = true
	return b
}

// AddXValue is a no-op stub.
func (b *LatLonBounds) AddXValue(_ *GeoPoint) Bounds { return b }

// AddYValue is a no-op stub.
func (b *LatLonBounds) AddYValue(_ *GeoPoint) Bounds { return b }

// AddZValue is a no-op stub.
func (b *LatLonBounds) AddZValue(_ *GeoPoint) Bounds { return b }

// IsWide marks as wide.
func (b *LatLonBounds) IsWide() Bounds { b.wide = true; return b }

// NoLongitudeBound marks as having no longitude bound.
func (b *LatLonBounds) NoLongitudeBound() Bounds { b.noLongitudeBound = true; return b }

// NoTopLatitudeBound marks as having no top latitude bound.
func (b *LatLonBounds) NoTopLatitudeBound() Bounds { b.noTopLatitudeBound = true; return b }

// NoBottomLatitudeBound marks as having no bottom latitude bound.
func (b *LatLonBounds) NoBottomLatitudeBound() Bounds { b.noBottomLatitudeBound = true; return b }

// NoBound marks as having no bound at all.
func (b *LatLonBounds) NoBound(_ *PlanetModel) Bounds {
	b.noLongitudeBound = true
	b.noTopLatitudeBound = true
	b.noBottomLatitudeBound = true
	return b
}

var _ Bounds = (*LatLonBounds)(nil)

// GeoBounds is the concrete accumulator returned by most shapes.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBounds.
// Alias over LatLonBounds for naming consistency with the Java API.
type GeoBoundsAccumulator = LatLonBounds

// NewGeoBounds creates a new bounds accumulator.
func NewGeoBounds() *GeoBoundsAccumulator { return NewLatLonBounds() }

// xyzBoundsFudgeFactor is added to maximums and subtracted from minimums to
// compensate for potential error deltas, so the accumulated box is guaranteed
// to equal or exceed the boundary of the shape.
//
// Port of XYZBounds.FUDGE_FACTOR = Vector.MINIMUM_RESOLUTION * 1e3.
const xyzBoundsFudgeFactor = MinimumResolution * 1e3

// XYZBounds accumulates raw XYZ extents.
//
// Port of org.apache.lucene.spatial3d.geom.XYZBounds.
//
// The Java reference stores each extent as a nullable Double (unset == null).
// Gocene mirrors that with explicit hasX/hasY/hasZ flags so the "smallest /
// largest possible value" guards used by Plane.RecordBounds behave identically:
// an unset extent must report false from the isSmallest/isLargest tests.
type XYZBounds struct {
	MinimumX float64
	MaximumX float64
	MinimumY float64
	MaximumY float64
	MinimumZ float64
	MaximumZ float64

	hasX bool
	hasY bool
	hasZ bool
}

// NewXYZBounds creates an empty XYZBounds.
func NewXYZBounds() *XYZBounds {
	return &XYZBounds{
		MinimumX: math.MaxFloat64, MaximumX: -math.MaxFloat64,
		MinimumY: math.MaxFloat64, MaximumY: -math.MaxFloat64,
		MinimumZ: math.MaxFloat64, MaximumZ: -math.MaxFloat64,
	}
}

// addXValueRaw records a specific X value, widened by the fudge factor.
//
// Port of XYZBounds.addXValue(double).
func (b *XYZBounds) addXValueRaw(x float64) {
	small := x - xyzBoundsFudgeFactor
	if !b.hasX || b.MinimumX > small {
		b.MinimumX = small
	}
	large := x + xyzBoundsFudgeFactor
	if !b.hasX || b.MaximumX < large {
		b.MaximumX = large
	}
	b.hasX = true
}

// addYValueRaw records a specific Y value, widened by the fudge factor.
//
// Port of XYZBounds.addYValue(double).
func (b *XYZBounds) addYValueRaw(y float64) {
	small := y - xyzBoundsFudgeFactor
	if !b.hasY || b.MinimumY > small {
		b.MinimumY = small
	}
	large := y + xyzBoundsFudgeFactor
	if !b.hasY || b.MaximumY < large {
		b.MaximumY = large
	}
	b.hasY = true
}

// addZValueRaw records a specific Z value, widened by the fudge factor.
//
// Port of XYZBounds.addZValue(double).
func (b *XYZBounds) addZValueRaw(z float64) {
	small := z - xyzBoundsFudgeFactor
	if !b.hasZ || b.MinimumZ > small {
		b.MinimumZ = small
	}
	large := z + xyzBoundsFudgeFactor
	if !b.hasZ || b.MaximumZ < large {
		b.MaximumZ = large
	}
	b.hasZ = true
}

// HasX reports whether any X extent has been recorded.
func (b *XYZBounds) HasX() bool { return b.hasX }

// HasY reports whether any Y extent has been recorded.
func (b *XYZBounds) HasY() bool { return b.hasY }

// HasZ reports whether any Z extent has been recorded.
func (b *XYZBounds) HasZ() bool { return b.hasZ }

// isSmallestMinX reports whether MinimumX has reached the planet's minimum X.
//
// Port of XYZBounds.isSmallestMinX.
func (b *XYZBounds) isSmallestMinX(pm *PlanetModel) bool {
	if !b.hasX {
		return false
	}
	return b.MinimumX-pm.GetMinimumXValue() < MinimumResolution
}

// isLargestMaxX reports whether MaximumX has reached the planet's maximum X.
//
// Port of XYZBounds.isLargestMaxX.
func (b *XYZBounds) isLargestMaxX(pm *PlanetModel) bool {
	if !b.hasX {
		return false
	}
	return pm.GetMaximumXValue()-b.MaximumX < MinimumResolution
}

// isSmallestMinY reports whether MinimumY has reached the planet's minimum Y.
//
// Port of XYZBounds.isSmallestMinY.
func (b *XYZBounds) isSmallestMinY(pm *PlanetModel) bool {
	if !b.hasY {
		return false
	}
	return b.MinimumY-pm.GetMinimumYValue() < MinimumResolution
}

// isLargestMaxY reports whether MaximumY has reached the planet's maximum Y.
//
// Port of XYZBounds.isLargestMaxY.
func (b *XYZBounds) isLargestMaxY(pm *PlanetModel) bool {
	if !b.hasY {
		return false
	}
	return pm.GetMaximumYValue()-b.MaximumY < MinimumResolution
}

// isSmallestMinZ reports whether MinimumZ has reached the planet's minimum Z.
//
// Port of XYZBounds.isSmallestMinZ.
func (b *XYZBounds) isSmallestMinZ(pm *PlanetModel) bool {
	if !b.hasZ {
		return false
	}
	return b.MinimumZ-pm.GetMinimumZValue() < MinimumResolution
}

// isLargestMaxZ reports whether MaximumZ has reached the planet's maximum Z.
//
// Port of XYZBounds.isLargestMaxZ.
func (b *XYZBounds) isLargestMaxZ(pm *PlanetModel) bool {
	if !b.hasZ {
		return false
	}
	return pm.GetMaximumZValue()-b.MaximumZ < MinimumResolution
}

// AddXValue records the point's X, widened by the fudge factor.
//
// Port of XYZBounds.addXValue(GeoPoint).
func (b *XYZBounds) AddXValue(p *GeoPoint) Bounds {
	b.addXValueRaw(p.X)
	return b
}

// AddYValue records the point's Y, widened by the fudge factor.
//
// Port of XYZBounds.addYValue(GeoPoint).
func (b *XYZBounds) AddYValue(p *GeoPoint) Bounds {
	b.addYValueRaw(p.Y)
	return b
}

// AddZValue records the point's Z, widened by the fudge factor.
//
// Port of XYZBounds.addZValue(GeoPoint).
func (b *XYZBounds) AddZValue(p *GeoPoint) Bounds {
	b.addZValueRaw(p.Z)
	return b
}

// AddPoint records all three coordinates.
//
// Port of XYZBounds.addPoint.
func (b *XYZBounds) AddPoint(p *GeoPoint) Bounds {
	b.AddXValue(p)
	b.AddYValue(p)
	b.AddZValue(p)
	return b
}

// AddPlane records the extrema of the plane's intersection with the planet,
// constrained by the supplied Membership bounds.
//
// Port of XYZBounds.addPlane: delegates to Plane.recordBounds.
func (b *XYZBounds) AddPlane(pm *PlanetModel, plane *Plane, bounds ...Membership) Bounds {
	plane.RecordBounds(pm, b, bounds...)
	return b
}

// AddHorizontalPlane records a horizontal plane via AddPlane.
//
// Port of XYZBounds.addHorizontalPlane.
func (b *XYZBounds) AddHorizontalPlane(pm *PlanetModel, _ float64, plane *Plane, bounds ...Membership) Bounds {
	return b.AddPlane(pm, plane, bounds...)
}

// AddVerticalPlane records a vertical plane via AddPlane.
//
// Port of XYZBounds.addVerticalPlane.
func (b *XYZBounds) AddVerticalPlane(pm *PlanetModel, _ float64, plane *Plane, bounds ...Membership) Bounds {
	return b.AddPlane(pm, plane, bounds...)
}

// AddIntersection accumulates bounds for the intersection of two planes p and q
// and the planet surface, within the supplied Membership bounds.
//
// Port of org.apache.lucene.spatial3d.geom.XYZBounds.addIntersection, which
// calls Plane.recordBounds(PlanetModel, XYZBounds, Plane, Membership...).
// Previously a no-op stub; now wired to Plane.RecordBoundsWithPlane (rmp #4790).
func (b *XYZBounds) AddIntersection(pm *PlanetModel, p, q *Plane, bounds ...Membership) Bounds {
	if p != nil && q != nil {
		p.RecordBoundsWithPlane(pm, b, q, bounds...)
	}
	return b
}

// IsWide is a no-op.
func (b *XYZBounds) IsWide() Bounds { return b }

// NoLongitudeBound is a no-op.
func (b *XYZBounds) NoLongitudeBound() Bounds { return b }

// NoTopLatitudeBound is a no-op.
func (b *XYZBounds) NoTopLatitudeBound() Bounds { return b }

// NoBottomLatitudeBound is a no-op.
func (b *XYZBounds) NoBottomLatitudeBound() Bounds { return b }

// NoBound widens the bounds to the entire planet box.
//
// Port of XYZBounds.noBound.
func (b *XYZBounds) NoBound(pm *PlanetModel) Bounds {
	b.MinimumX = pm.GetMinimumXValue()
	b.MaximumX = pm.GetMaximumXValue()
	b.MinimumY = pm.GetMinimumYValue()
	b.MaximumY = pm.GetMaximumYValue()
	b.MinimumZ = pm.GetMinimumZValue()
	b.MaximumZ = pm.GetMaximumZValue()
	b.hasX, b.hasY, b.hasZ = true, true, true
	return b
}

// IsWithin reports whether (x,y,z) lies inside the accumulated box. Returns
// false for any dimension that has not yet been recorded.
//
// Port of XYZBounds.isWithin(double,double,double).
func (b *XYZBounds) IsWithin(x, y, z float64) bool {
	return b.hasX && x >= b.MinimumX && x <= b.MaximumX &&
		b.hasY && y >= b.MinimumY && y <= b.MaximumY &&
		b.hasZ && z >= b.MinimumZ && z <= b.MaximumZ
}

var _ Bounds = (*XYZBounds)(nil)
