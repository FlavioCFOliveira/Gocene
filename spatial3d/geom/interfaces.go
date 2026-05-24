// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package geom provides the 3-D sphere geometry primitives for Geo3D.
//
// Port of org.apache.lucene.spatial3d.geom.
//
// Deviation: All algorithmic bodies are deferred to backlog #2693.
// This sprint delivers the full public interface contract so that callers
// can compile against the API.
package geom

import "io"

// Relationship constants for GeoArea.GetRelationship.
const (
	// RelContains means the referenced shape CONTAINS this area.
	RelContains = 0
	// RelWithin means the referenced shape IS WITHIN this area.
	RelWithin = 1
	// RelOverlaps means the referenced shape OVERLAPS this area.
	RelOverlaps = 2
	// RelDisjoint means the referenced shape has no relation to this area.
	RelDisjoint = 3
)

// SerializableObject is implemented by types that can be written to a stream.
//
// Port of org.apache.lucene.spatial3d.geom.SerializableObject.
type SerializableObject interface {
	Write(outputStream io.Writer) error
}

// Membership describes whether a point is a member of a shape.
//
// Port of org.apache.lucene.spatial3d.geom.Membership.
type Membership interface {
	// IsWithin reports whether point (x,y,z) is a member of this shape.
	IsWithin(x, y, z float64) bool
}

// Bounded is implemented by shapes that can check whether a plane is within
// their bounds.
//
// Port of org.apache.lucene.spatial3d.geom.Bounded.
type Bounded interface {
	// IsWithin reports whether point (x,y,z) is within the bounded region.
	IsWithin(x, y, z float64) bool
}

// PlanetObject is a Membership whose membership test uses the planet model.
//
// Port of org.apache.lucene.spatial3d.geom.PlanetObject.
type PlanetObject interface {
	Membership
	// GetPlanetModel returns the planet model associated with this object.
	GetPlanetModel() *PlanetModel
}

// GeoBounds is the base interface for shapes that can accumulate bounds.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBounds.
type GeoBounds interface {
	// GetBounds accumulates bounding information into b.
	GetBounds(b Bounds)
}

// GeoShape is a shape that participates in geo-area queries.
//
// Port of org.apache.lucene.spatial3d.geom.GeoShape.
type GeoShape interface {
	GeoBounds
	// GetEdgePoints returns sample points on the outer edge.
	GetEdgePoints() []*GeoPoint
	// Intersects reports whether the plane (within bounds) crosses this shape.
	Intersects(plane *Plane, notablePoints []*GeoPoint, bounds ...Membership) bool
}

// GeoArea is a region of the sphere used for geo hash computation.
//
// Port of org.apache.lucene.spatial3d.geom.GeoArea.
type GeoArea interface {
	Membership
	// GetRelationship returns how shape relates to this area.
	GetRelationship(shape GeoShape) int
}

// GeoMembershipShape combines GeoShape and Membership.
//
// Port of org.apache.lucene.spatial3d.geom.GeoMembershipShape.
type GeoMembershipShape interface {
	GeoShape
	Membership
}

// GeoAreaShape combines GeoArea and GeoShape.
//
// Port of org.apache.lucene.spatial3d.geom.GeoAreaShape.
type GeoAreaShape interface {
	GeoArea
	GeoShape
}

// GeoBBox is an axis-aligned bounding box on the sphere.
//
// Port of org.apache.lucene.spatial3d.geom.GeoBBox.
type GeoBBox interface {
	GeoAreaShape
	// Expand returns a BBox expanded by the given angle in radians.
	Expand(angleRadians float64) GeoBBox
}

// GeoCircle is a circle on the sphere defined by a centre and radius.
//
// Port of org.apache.lucene.spatial3d.geom.GeoCircle.
type GeoCircle interface {
	GeoMembershipShape
	// GetRadius returns the radius in radians.
	GetRadius() float64
}

// GeoPolygon is a polygon on the sphere.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPolygon.
type GeoPolygon interface {
	GeoMembershipShape
}

// GeoPath is a path with width on the sphere.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPath.
type GeoPath interface {
	GeoMembershipShape
}

// GeoPointShape is a degenerate shape representing a single point.
// It extends both GeoCircle and GeoBBox.
//
// Port of org.apache.lucene.spatial3d.geom.GeoPointShape.
type GeoPointShape interface {
	GeoCircle
	GeoBBox
}

// GeoSizeable can return a bounding-box estimate.
//
// Port of org.apache.lucene.spatial3d.geom.GeoSizeable.
type GeoSizeable interface {
	// GetRadius returns the radius of a bounding circle.
	GetRadius() float64
}

// GeoDistance can compute distances.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDistance.
type GeoDistance interface {
	GeoMembershipShape
	// ComputeDistance returns the distance from this shape to the given point.
	ComputeDistance(distanceStyle DistanceStyle, x, y, z float64) float64
}

// GeoDistanceShape combines GeoDistance with GeoMembershipShape.
//
// Port of org.apache.lucene.spatial3d.geom.GeoDistanceShape.
type GeoDistanceShape interface {
	GeoDistance
	GeoSizeable
}

// GeoOutsideDistance computes distances from outside the shape.
//
// Port of org.apache.lucene.spatial3d.geom.GeoOutsideDistance.
type GeoOutsideDistance interface {
	GeoMembershipShape
	// ComputeOutsideDistance returns the distance to the nearest edge.
	ComputeOutsideDistance(distanceStyle DistanceStyle, x, y, z float64) float64
}

// GeoS2Shape is a shape backed by S2 geometry.
//
// Port of org.apache.lucene.spatial3d.geom.GeoS2Shape.
type GeoS2Shape interface {
	GeoMembershipShape
}

// Bounds accumulates bounding information for a shape.
//
// Port of org.apache.lucene.spatial3d.geom.Bounds.
type Bounds interface {
	AddPlane(pm *PlanetModel, plane *Plane, bounds ...Membership) Bounds
	AddHorizontalPlane(pm *PlanetModel, latitude float64, plane *Plane, bounds ...Membership) Bounds
	AddVerticalPlane(pm *PlanetModel, longitude float64, plane *Plane, bounds ...Membership) Bounds
	AddIntersection(pm *PlanetModel, plane1, plane2 *Plane, bounds ...Membership) Bounds
	AddPoint(point *GeoPoint) Bounds
	AddXValue(point *GeoPoint) Bounds
	AddYValue(point *GeoPoint) Bounds
	AddZValue(point *GeoPoint) Bounds
	IsWide() Bounds
	NoLongitudeBound() Bounds
	NoTopLatitudeBound() Bounds
	NoBottomLatitudeBound() Bounds
	NoBound(pm *PlanetModel) Bounds
}

// DistanceStyle describes how distances are computed.
//
// Port of org.apache.lucene.spatial3d.geom.DistanceStyle.
type DistanceStyle interface {
	// ToAggregationForm converts a distance to its aggregation form.
	ToAggregationForm(distance float64) float64
	// FromAggregationForm converts from aggregation form back to distance.
	FromAggregationForm(aggregatedDistance float64) float64
	// ToSlice converts a distance to a 1-D slice value for filtering.
	ToSlice(distance float64) float64
	// GetMagnitude returns the actual distance from aggregation form.
	GetMagnitude(aggregatedDistance float64) float64
	// IsLessThan reports whether agg1 < agg2 in aggregation form.
	IsLessThan(agg1, agg2 float64) bool
}

// XYZSolid is a 3-D solid (volume) bounded by six XYZ planes.
// It extends GeoArea (Membership + GetRelationship) and PlanetObject
// (Membership + GetPlanetModel).
//
// Port of org.apache.lucene.spatial3d.geom.XYZSolid.
type XYZSolid interface {
	GeoArea
	// GetPlanetModel returns the associated PlanetModel.
	GetPlanetModel() *PlanetModel
}
