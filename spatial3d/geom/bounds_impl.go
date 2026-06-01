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
//
// Longitude is tracked as a circular arc with a left and right endpoint
// (hasLon = false means no longitude has been recorded yet). This matches the
// Java nullable Double semantics for leftLongitude/rightLongitude exactly.
type LatLonBounds struct {
	GeoBaseBounds
	minLatitude   float64
	maxLatitude   float64
	leftLongitude float64 // meaningful only when hasLon == true
	rightLongitude float64 // meaningful only when hasLon == true
	hasLon        bool
	hasLat        bool
}

// NewLatLonBounds creates an empty LatLonBounds.
func NewLatLonBounds() *LatLonBounds {
	return &LatLonBounds{
		minLatitude: math.MaxFloat64,
		maxLatitude: -math.MaxFloat64,
	}
}

// GetMinLatitude returns the minimum latitude seen so far, or math.MaxFloat64 if
// no latitude has been recorded.
func (b *LatLonBounds) GetMinLatitude() float64 { return b.minLatitude }

// GetMaxLatitude returns the maximum latitude seen so far, or -math.MaxFloat64 if
// no latitude has been recorded.
func (b *LatLonBounds) GetMaxLatitude() float64 { return b.maxLatitude }

// GetMinLongitude returns the left (minimum) longitude of the recorded arc. This
// is the Java leftLongitude; it is only meaningful when CheckNoLongitudeBound()
// returns false and HasLon() returns true.
func (b *LatLonBounds) GetMinLongitude() float64 { return b.leftLongitude }

// GetMaxLongitude returns the right (maximum) longitude of the recorded arc. This
// is the Java rightLongitude; it is only meaningful when CheckNoLongitudeBound()
// returns false and HasLon() returns true.
func (b *LatLonBounds) GetMaxLongitude() float64 { return b.rightLongitude }

// HasLon reports whether at least one longitude has been recorded.
func (b *LatLonBounds) HasLon() bool { return b.hasLon }

// HasLat reports whether at least one latitude has been recorded.
func (b *LatLonBounds) HasLat() bool { return b.hasLat }

// CheckNoLongitudeBound reports whether no longitude bound was signalled.
func (b *LatLonBounds) CheckNoLongitudeBound() bool { return b.noLongitudeBound }

// CheckNoTopLatitudeBound reports whether no top latitude bound was signalled.
func (b *LatLonBounds) CheckNoTopLatitudeBound() bool { return b.noTopLatitudeBound }

// CheckNoBottomLatitudeBound reports whether no bottom latitude bound was signalled.
func (b *LatLonBounds) CheckNoBottomLatitudeBound() bool { return b.noBottomLatitudeBound }

// addLatitudeBound updates the latitude envelope with the given value.
//
// Port of LatLonBounds.addLatitudeBound.
func (b *LatLonBounds) addLatitudeBound(latitude float64) {
	if !b.noTopLatitudeBound && latitude > b.maxLatitude {
		b.maxLatitude = latitude
	}
	if !b.noBottomLatitudeBound && latitude < b.minLatitude {
		b.minLatitude = latitude
	}
	b.hasLat = true
}

// addLongitudeBound expands the circular-arc longitude envelope to include longitude.
//
// Port of LatLonBounds.addLongitudeBound. The algorithm keeps a left/right pair
// representing the minimal arc that covers all submitted longitudes. If the arc
// ever reaches or exceeds π the envelope is promoted to "no bound".
func (b *LatLonBounds) addLongitudeBound(longitude float64) {
	if b.noLongitudeBound {
		return
	}
	if !b.hasLon {
		b.leftLongitude = longitude
		b.rightLongitude = longitude
		b.hasLon = true
	} else {
		curLeft := b.leftLongitude
		curRight := b.rightLongitude
		// Normalise the right endpoint so curRight >= curLeft (circular arithmetic).
		if curRight < curLeft {
			curRight += 2.0 * math.Pi
		}
		// Shift longitude into [curLeft, curLeft+2π) for comparison.
		if longitude < curLeft {
			longitude += 2.0 * math.Pi
		}
		if longitude < curLeft || longitude > curRight {
			// Outside the current arc; choose the smaller of left-extension and
			// right-extension.
			var leftExtAmt, rightExtAmt float64
			if longitude < curLeft {
				leftExtAmt = curLeft - longitude
			} else {
				leftExtAmt = curLeft + 2.0*math.Pi - longitude
			}
			if longitude > curRight {
				rightExtAmt = longitude - curRight
			} else {
				rightExtAmt = longitude + 2.0*math.Pi - curRight
			}
			if leftExtAmt < rightExtAmt {
				newLeft := b.leftLongitude - leftExtAmt
				for newLeft <= -math.Pi {
					newLeft += 2.0 * math.Pi
				}
				b.leftLongitude = newLeft
			} else {
				newRight := b.rightLongitude + rightExtAmt
				for newRight > math.Pi {
					newRight -= 2.0 * math.Pi
				}
				b.rightLongitude = newRight
			}
		}
	}
	// Check whether the arc has grown to ≥ π.
	testRight := b.rightLongitude
	if testRight < b.leftLongitude {
		testRight += 2.0 * math.Pi
	}
	if testRight-b.leftLongitude >= math.Pi {
		b.noLongitudeBound = true
		b.hasLon = false
	}
}

// AddPlane accumulates lat/lon bounds for this plane intersected with the planet.
//
// Port of LatLonBounds.addPlane: delegates to Plane.RecordBoundsForLatLon.
func (b *LatLonBounds) AddPlane(pm *PlanetModel, plane *Plane, bounds ...Membership) Bounds {
	plane.RecordBoundsForLatLon(pm, b, bounds...)
	return b
}

// AddHorizontalPlane records the latitude bound implied by a horizontal plane.
//
// Port of LatLonBounds.addHorizontalPlane.
func (b *LatLonBounds) AddHorizontalPlane(_ *PlanetModel, latitude float64, _ *Plane, _ ...Membership) Bounds {
	if !b.noTopLatitudeBound || !b.noBottomLatitudeBound {
		b.addLatitudeBound(latitude)
	}
	return b
}

// AddVerticalPlane records the longitude bound implied by a vertical plane.
//
// Port of LatLonBounds.addVerticalPlane.
func (b *LatLonBounds) AddVerticalPlane(_ *PlanetModel, longitude float64, _ *Plane, _ ...Membership) Bounds {
	if !b.noLongitudeBound {
		b.addLongitudeBound(longitude)
	}
	return b
}

// AddIntersection accumulates lat/lon bounds for the intersection of two planes.
//
// Port of LatLonBounds.addIntersection: delegates to
// Plane.RecordBoundsForLatLonWithPlane.
func (b *LatLonBounds) AddIntersection(pm *PlanetModel, plane1, plane2 *Plane, bounds ...Membership) Bounds {
	if plane1 != nil && plane2 != nil {
		plane1.RecordBoundsForLatLonWithPlane(pm, b, plane2, bounds...)
	}
	return b
}

// AddPoint records a GeoPoint's lat/lon into the bounds.
//
// Port of LatLonBounds.addPoint.
func (b *LatLonBounds) AddPoint(p *GeoPoint) Bounds {
	if !b.noLongitudeBound {
		b.addLongitudeBound(p.GetLongitude())
	}
	if !b.noTopLatitudeBound || !b.noBottomLatitudeBound {
		b.addLatitudeBound(p.GetLatitude())
	}
	return b
}

// AddXValue records the longitude of the point (X-dominant extremum).
//
// Port of LatLonBounds.addXValue: x-value extrema lie on a meridian, so the
// meaningful bound is the longitude of the point.
func (b *LatLonBounds) AddXValue(p *GeoPoint) Bounds {
	if !b.noLongitudeBound {
		b.addLongitudeBound(p.GetLongitude())
	}
	return b
}

// AddYValue records the longitude of the point (Y-dominant extremum).
//
// Port of LatLonBounds.addYValue.
func (b *LatLonBounds) AddYValue(p *GeoPoint) Bounds {
	if !b.noLongitudeBound {
		b.addLongitudeBound(p.GetLongitude())
	}
	return b
}

// AddZValue records the latitude of the point (Z-dominant extremum).
//
// Port of LatLonBounds.addZValue: z-value extrema lie on a parallel, so the
// meaningful bound is the latitude of the point.
func (b *LatLonBounds) AddZValue(p *GeoPoint) Bounds {
	if !b.noTopLatitudeBound || !b.noBottomLatitudeBound {
		b.addLatitudeBound(p.GetLatitude())
	}
	return b
}

// IsWide promotes the longitude envelope to "no bound".
//
// Port of LatLonBounds.isWide: delegates to noLongitudeBound().
func (b *LatLonBounds) IsWide() Bounds { return b.NoLongitudeBound() }

// NoLongitudeBound marks longitude as unbounded.
//
// Port of LatLonBounds.noLongitudeBound.
func (b *LatLonBounds) NoLongitudeBound() Bounds {
	b.noLongitudeBound = true
	b.hasLon = false
	return b
}

// NoTopLatitudeBound marks the top latitude as unbounded.
//
// Port of LatLonBounds.noTopLatitudeBound.
func (b *LatLonBounds) NoTopLatitudeBound() Bounds {
	b.noTopLatitudeBound = true
	b.maxLatitude = -math.MaxFloat64
	return b
}

// NoBottomLatitudeBound marks the bottom latitude as unbounded.
//
// Port of LatLonBounds.noBottomLatitudeBound.
func (b *LatLonBounds) NoBottomLatitudeBound() Bounds {
	b.noBottomLatitudeBound = true
	b.minLatitude = math.MaxFloat64
	return b
}

// NoBound marks all bounds as unbounded.
//
// Port of LatLonBounds.noBound.
func (b *LatLonBounds) NoBound(_ *PlanetModel) Bounds {
	return b.NoLongitudeBound().NoTopLatitudeBound().NoBottomLatitudeBound()
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
