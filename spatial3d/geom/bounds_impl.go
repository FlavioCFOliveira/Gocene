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

// XYZBounds accumulates raw XYZ extents.
//
// Port of org.apache.lucene.spatial3d.geom.XYZBounds.
type XYZBounds struct {
	MinimumX float64
	MaximumX float64
	MinimumY float64
	MaximumY float64
	MinimumZ float64
	MaximumZ float64
}

// NewXYZBounds creates an empty XYZBounds.
func NewXYZBounds() *XYZBounds {
	return &XYZBounds{
		MinimumX: math.MaxFloat64, MaximumX: -math.MaxFloat64,
		MinimumY: math.MaxFloat64, MaximumY: -math.MaxFloat64,
		MinimumZ: math.MaxFloat64, MaximumZ: -math.MaxFloat64,
	}
}

// AddXValue records the point's X.
func (b *XYZBounds) AddXValue(p *GeoPoint) Bounds {
	if p.X < b.MinimumX {
		b.MinimumX = p.X
	}
	if p.X > b.MaximumX {
		b.MaximumX = p.X
	}
	return b
}

// AddYValue records the point's Y.
func (b *XYZBounds) AddYValue(p *GeoPoint) Bounds {
	if p.Y < b.MinimumY {
		b.MinimumY = p.Y
	}
	if p.Y > b.MaximumY {
		b.MaximumY = p.Y
	}
	return b
}

// AddZValue records the point's Z.
func (b *XYZBounds) AddZValue(p *GeoPoint) Bounds {
	if p.Z < b.MinimumZ {
		b.MinimumZ = p.Z
	}
	if p.Z > b.MaximumZ {
		b.MaximumZ = p.Z
	}
	return b
}

// AddPoint records all three coordinates.
func (b *XYZBounds) AddPoint(p *GeoPoint) Bounds {
	b.AddXValue(p)
	b.AddYValue(p)
	b.AddZValue(p)
	return b
}

// AddPlane is a no-op stub — deferred to #2693.
func (b *XYZBounds) AddPlane(_ *PlanetModel, _ *Plane, _ ...Membership) Bounds { return b }

// AddHorizontalPlane is a no-op stub.
func (b *XYZBounds) AddHorizontalPlane(_ *PlanetModel, _ float64, _ *Plane, _ ...Membership) Bounds {
	return b
}

// AddVerticalPlane is a no-op stub.
func (b *XYZBounds) AddVerticalPlane(_ *PlanetModel, _ float64, _ *Plane, _ ...Membership) Bounds {
	return b
}

// AddIntersection is a no-op stub.
func (b *XYZBounds) AddIntersection(_ *PlanetModel, _, _ *Plane, _ ...Membership) Bounds {
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

// NoBound is a no-op.
func (b *XYZBounds) NoBound(_ *PlanetModel) Bounds { return b }

var _ Bounds = (*XYZBounds)(nil)
