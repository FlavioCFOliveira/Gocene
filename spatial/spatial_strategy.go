// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package spatial provides spatial indexing and search capabilities
// for location-based queries. It implements various spatial strategies
// including point vector, bounding box, and prefix tree approaches.
//
// This is the Go port of Lucene's org.apache.lucene.spatial package.
package spatial

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/grouping"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpatialStrategy defines the interface for spatial indexing strategies.
// Implementations determine how shapes are indexed and queried.
//
// This is the Go port of Lucene's SpatialStrategy.
type SpatialStrategy interface {
	// GetFieldName returns the name of the field this strategy is associated with.
	GetFieldName() string

	// CreateIndexableFields generates the IndexableField instances needed to index
	// the given shape. The fields are added to the document for indexing.
	CreateIndexableFields(shape Shape) ([]document.IndexableField, error)

	// MakeQuery creates a spatial query for the given operation and shape.
	// The operation determines the spatial relationship to match (Intersects, Within, etc).
	MakeQuery(operation SpatialOperation, shape Shape) (search.Query, error)

	// MakeDistanceValueSource creates a ValueSource that returns the distance
	// from the center of the indexed shape to a specified point.
	// This is used for sorting or boosting by distance.
	MakeDistanceValueSource(point Point, multiplier float64) (grouping.ValueSource, error)
}

// BaseSpatialStrategy provides common functionality for all spatial strategies.
type BaseSpatialStrategy struct {
	fieldName       string
	spatialContext  *SpatialContext
}

// NewBaseSpatialStrategy creates a new base spatial strategy.
func NewBaseSpatialStrategy(fieldName string, ctx *SpatialContext) (*BaseSpatialStrategy, error) {
	if fieldName == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}
	if ctx == nil {
		return nil, fmt.Errorf("spatial context cannot be nil")
	}
	return &BaseSpatialStrategy{
		fieldName:      fieldName,
		spatialContext: ctx,
	}, nil
}

// GetFieldName returns the field name.
func (s *BaseSpatialStrategy) GetFieldName() string {
	return s.fieldName
}

// GetSpatialContext returns the spatial context.
func (s *BaseSpatialStrategy) GetSpatialContext() *SpatialContext {
	return s.spatialContext
}

// SpatialContext provides context for spatial operations including
// the world bounds and distance calculations.
type SpatialContext struct {
	// WorldBounds defines the valid world coordinate bounds.
	// For most cases: MinX=-180, MinY=-90, MaxX=180, MaxY=90 (geographic).
	WorldBounds *Rectangle

	// Geo indicates whether this context uses geographic coordinates (true)
	// or Cartesian coordinates (false).
	Geo bool

	// Calculator provides distance calculation methods.
	Calculator DistanceCalculator
}

// NewSpatialContext creates a new spatial context with default geographic settings.
func NewSpatialContext() *SpatialContext {
	return &SpatialContext{
		WorldBounds: NewRectangle(-180, -90, 180, 90),
		Geo:         true,
		Calculator:  &HaversineCalculator{},
	}
}

// NewSpatialContextGeo creates a new geographic spatial context.
func NewSpatialContextGeo() *SpatialContext {
	return NewSpatialContext()
}

// NewSpatialContextCartesian creates a new Cartesian spatial context.
func NewSpatialContextCartesian(minX, minY, maxX, maxY float64) *SpatialContext {
	return &SpatialContext{
		WorldBounds: NewRectangle(minX, minY, maxX, maxY),
		Geo:         false,
		Calculator:  &CartesianCalculator{},
	}
}

// DistanceCalculator provides methods for calculating distances between points.
type DistanceCalculator interface {
	// Distance calculates the distance between two points.
	Distance(p1, p2 Point) float64

	// DistanceFromDegrees calculates distance from angular degrees.
	DistanceFromDegrees(degrees float64) float64
}

// HaversineCalculator calculates distances using the haversine formula
// for spherical coordinates.
type HaversineCalculator struct{}

// Distance calculates the great circle distance between two points using haversine formula.
// Result is in kilometers.
func (h *HaversineCalculator) Distance(p1, p2 Point) float64 {
	return HaversineDistance(p1.Y, p1.X, p2.Y, p2.X)
}

// DistanceFromDegrees converts angular degrees to distance.
// At the equator, 1 degree of longitude is approximately 111.32 km.
func (h *HaversineCalculator) DistanceFromDegrees(degrees float64) float64 {
	// Earth's radius in kilometers
	const earthRadiusKm = 6371.0
	return degrees * earthRadiusKm * 2 * 3.141592653589793 / 360
}

// CartesianCalculator calculates distances in Cartesian coordinates.
type CartesianCalculator struct{}

// Distance calculates the Euclidean distance between two points.
func (c *CartesianCalculator) Distance(p1, p2 Point) float64 {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	return sqrt(dx*dx + dy*dy)
}

// DistanceFromDegrees returns the same value (no conversion needed in Cartesian).
func (c *CartesianCalculator) DistanceFromDegrees(degrees float64) float64 {
	return degrees
}

// sqrt is a helper for square root calculation.
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// SpatialOperation represents the type of spatial relationship to query.
type SpatialOperation int

const (
	// SpatialOperationIntersects matches shapes that intersect the query shape.
	SpatialOperationIntersects SpatialOperation = iota
	// SpatialOperationIsWithin matches shapes that are within the query shape.
	SpatialOperationIsWithin
	// SpatialOperationContains matches shapes that contain the query shape.
	SpatialOperationContains
	// SpatialOperationIsDisjointTo matches shapes that do not intersect the query shape.
	SpatialOperationIsDisjointTo
	// SpatialOperationEquals matches shapes that are equal to the query shape.
	SpatialOperationEquals
	// SpatialOperationOverlaps matches shapes that overlap the query shape.
	SpatialOperationOverlaps
)

// String returns the string representation of the spatial operation.
func (op SpatialOperation) String() string {
	switch op {
	case SpatialOperationIntersects:
		return "Intersects"
	case SpatialOperationIsWithin:
		return "IsWithin"
	case SpatialOperationContains:
		return "Contains"
	case SpatialOperationIsDisjointTo:
		return "IsDisjointTo"
	case SpatialOperationEquals:
		return "Equals"
	case SpatialOperationOverlaps:
		return "Overlaps"
	default:
		return "Unknown"
	}
}

// Point represents a spatial point with X (longitude) and Y (latitude) coordinates.
type Point struct {
	X float64 // Longitude or X coordinate
	Y float64 // Latitude or Y coordinate
}

// NewPoint creates a new Point with the given coordinates.
func NewPoint(x, y float64) Point {
	return Point{X: x, Y: y}
}

// String returns a string representation of the point.
func (p Point) String() string {
	return fmt.Sprintf("Point(%f, %f)", p.X, p.Y)
}

// Rectangle represents an axis-aligned bounding box.
type Rectangle struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

// NewRectangle creates a new Rectangle.
func NewRectangle(minX, minY, maxX, maxY float64) *Rectangle {
	return &Rectangle{
		MinX: minX,
		MinY: minY,
		MaxX: maxX,
		MaxY: maxY,
	}
}

// Center returns the center point of the rectangle.
func (r *Rectangle) Center() Point {
	return Point{
		X: (r.MinX + r.MaxX) / 2,
		Y: (r.MinY + r.MaxY) / 2,
	}
}

// ContainsPoint checks if the rectangle contains the given point.
func (r *Rectangle) ContainsPoint(p Point) bool {
	return p.X >= r.MinX && p.X <= r.MaxX && p.Y >= r.MinY && p.Y <= r.MaxY
}

// Intersects checks if this rectangle intersects another rectangle.
func (r *Rectangle) Intersects(other *Rectangle) bool {
	return r.MinX <= other.MaxX && r.MaxX >= other.MinX &&
		r.MinY <= other.MaxY && r.MaxY >= other.MinY
}

// Width returns the width of the rectangle.
func (r *Rectangle) Width() float64 {
	return r.MaxX - r.MinX
}

// Height returns the height of the rectangle.
func (r *Rectangle) Height() float64 {
	return r.MaxY - r.MinY
}

// Area returns the area of the rectangle.
func (r *Rectangle) Area() float64 {
	return r.Width() * r.Height()
}

// String returns a string representation of the rectangle.
func (r *Rectangle) String() string {
	return fmt.Sprintf("Rectangle(%f, %f, %f, %f)", r.MinX, r.MinY, r.MaxX, r.MaxY)
}

// Shape is the interface for all spatial shapes.
type Shape interface {
	// GetBoundingBox returns the bounding box of the shape.
	GetBoundingBox() *Rectangle

	// GetCenter returns the center point of the shape.
	GetCenter() Point

	// Intersects checks if this shape intersects with another shape.
	Intersects(other Shape) bool

	// Contains checks if this shape contains another shape.
	Contains(other Shape) bool

	// IsWithin checks if this shape is within another shape.
	IsWithin(other Shape) bool

	// String returns a string representation of the shape.
	String() string
}

// HaversineDistance calculates the great circle distance between two points
// on the Earth specified in decimal degrees (latitude/longitude).
// Returns distance in kilometers.
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	// Convert latitude and longitude from degrees to radians
	lat1Rad := lat1 * 3.141592653589793 / 180
	lat2Rad := lat2 * 3.141592653589793 / 180
	latDiff := (lat2 - lat1) * 3.141592653589793 / 180
	lonDiff := (lon2 - lon1) * 3.141592653589793 / 180

	// Haversine formula
	a := sin(latDiff/2)*sin(latDiff/2) +
		cos(lat1Rad)*cos(lat2Rad)*
			sin(lonDiff/2)*sin(lonDiff/2)
	c := 2 * atan2(sqrt(a), sqrt(1-a))

	return earthRadiusKm * c
}

// sin calculates sine of an angle in radians.
func sin(x float64) float64 {
	// Taylor series approximation for sin(x)
	// sin(x) = x - x^3/3! + x^5/5! - x^7/7! + ...
	x2 := x * x
	x3 := x2 * x
	x5 := x3 * x2
	x7 := x5 * x2
	return x - x3/6 + x5/120 - x7/5040
}

// cos calculates cosine of an angle in radians.
func cos(x float64) float64 {
	// Taylor series approximation for cos(x)
	// cos(x) = 1 - x^2/2! + x^4/4! - x^6/6! + ...
	x2 := x * x
	x4 := x2 * x2
	x6 := x4 * x2
	return 1 - x2/2 + x4/24 - x6/720
}

// atan2 calculates the arctangent of y/x.
func atan2(y, x float64) float64 {
	// Using a simplified approximation
	if x > 0 {
		return atan(y / x)
	} else if x < 0 {
		if y >= 0 {
			return atan(y/x) + 3.141592653589793
		}
		return atan(y/x) - 3.141592653589793
	} else {
		if y > 0 {
			return 3.141592653589793 / 2
		} else if y < 0 {
			return -3.141592653589793 / 2
		}
		return 0
	}
}

// atan calculates the arctangent of x.
func atan(x float64) float64 {
	// Taylor series: atan(x) = x - x^3/3 + x^5/5 - x^7/7 + ...
	if x > 1 {
		return 3.141592653589793/2 - atan(1/x)
	}
	if x < -1 {
		return -3.141592653589793/2 - atan(1/x)
	}
	x2 := x * x
	x3 := x2 * x
	x5 := x3 * x2
	x7 := x5 * x2
	return x - x3/3 + x5/5 - x7/7
}
