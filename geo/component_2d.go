// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
// See also http://www.apache.org/licenses/LICENSE-2.0

package geo

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// WithinRelation is used by WithinTriangle to check the within relationship
// between a triangle and the query shape (e.g. if the query shape is within
// the triangle).
//
// This is the Go port of org.apache.lucene.geo.Component2D.WithinRelation.
type WithinRelation int

const (
	// WithinCandidate means the shape is a candidate for within. Typically
	// this is returned if the query shape is fully inside the triangle or
	// if the query shape intersects only edges that do not belong to the
	// original shape.
	WithinCandidate WithinRelation = iota

	// WithinNotWithin means the query shape intersects an edge that does
	// belong to the original shape or any point of the triangle is inside
	// the shape.
	WithinNotWithin

	// WithinDisjoint means the query shape is disjoint with the triangle.
	WithinDisjoint
)

// Component2D is a 2D Geometry object that supports spatial relationships
// with bounding boxes, triangles and points.
//
// This is the Go port of org.apache.lucene.geo.Component2D.
type Component2D interface {
	// GetMinX returns the min X value for the component.
	GetMinX() float64

	// GetMaxX returns the max X value for the component.
	GetMaxX() float64

	// GetMinY returns the min Y value for the component.
	GetMinY() float64

	// GetMaxY returns the max Y value for the component.
	GetMaxY() float64

	// Contains relates this component2D with a point.
	Contains(x, y float64) bool

	// Relate relates this component2D with a bounding box.
	Relate(minX, maxX, minY, maxY float64) codecs.Relation

	// IntersectsLine returns true if this component2D intersects the provided line.
	IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool

	// IntersectsTriangle returns true if this component2D intersects the provided triangle.
	IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool

	// ContainsLine returns true if this component2D contains the provided line.
	ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool

	// ContainsTriangle returns true if this component2D contains the provided triangle.
	ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool

	// WithinPoint computes the within relation of this component2D with a point.
	WithinPoint(x, y float64) WithinRelation

	// WithinLine computes the within relation of this component2D with a line.
	WithinLine(minX, maxX, minY, maxY, aX, aY, ab bool, bX, bY float64) WithinRelation

	// WithinTriangle computes the within relation of this component2D with a triangle.
	WithinTriangle(minX, maxX, minY, maxY, aX, aY, ab bool, bX, bY, bc bool, cX, cY, ca bool) WithinRelation
}

// IntersectsLine reports whether the component intersects the provided line.
// This mirrors the default method in the Java Component2D interface.
func IntersectsLine(c Component2D, aX, aY, bX, bY float64) bool {
	minY := math.Min(aY, bY)
	minX := math.Min(aX, bX)
	maxY := math.Max(aY, bY)
	maxX := math.Max(aX, bX)
	return c.IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY)
}

// IntersectsTriangle reports whether the component intersects the provided triangle.
// This mirrors the default method in the Java Component2D interface.
func IntersectsTriangle(c Component2D, aX, aY, bX, bY, cX, cY float64) bool {
	minY := math.Min(math.Min(aY, bY), cY)
	minX := math.Min(math.Min(aX, bX), cX)
	maxY := math.Max(math.Max(aY, bY), cY)
	maxX := math.Max(math.Max(aX, bX), cX)
	return c.IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY)
}

// ContainsLine reports whether the component contains the provided line.
// This mirrors the default method in the Java Component2D interface.
func ContainsLine(c Component2D, aX, aY, bX, bY float64) bool {
	minY := math.Min(aY, bY)
	minX := math.Min(aX, bX)
	maxY := math.Max(aY, bY)
	maxX := math.Max(aX, bX)
	return c.ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY)
}

// ContainsTriangle reports whether the component contains the provided triangle.
// This mirrors the default method in the Java Component2D interface.
func ContainsTriangle(c Component2D, aX, aY, bX, bY, cX, cY float64) bool {
	minY := math.Min(math.Min(aY, bY), cY)
	minX := math.Min(math.Min(aX, bX), cX)
	maxY := math.Max(math.Max(aY, bY), cY)
	maxX := math.Max(math.Max(aX, bX), cX)
	return c.ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY)
}

// WithinLine computes the within relation of the component with a line.
// This mirrors the default method in the Java Component2D interface.
func WithinLine(c Component2D, aX, aY, ab bool, bX, bY float64) WithinRelation {
	minY := math.Min(aY, bY)
	minX := math.Min(aX, bX)
	maxY := math.Max(aY, bY)
	maxX := math.Max(aX, bX)
	return c.WithinLine(minX, maxX, minY, maxY, aX, aY, ab, bX, bY)
}

// WithinTriangle computes the within relation of the component with a triangle.
// This mirrors the default method in the Java Component2D interface.
func WithinTriangle(c Component2D, aX, aY, ab bool, bX, bY, bc bool, cX, cY, ca bool) WithinRelation {
	minY := math.Min(math.Min(aY, bY), cY)
	minX := math.Min(math.Min(aX, bX), cX)
	maxY := math.Max(math.Max(aY, bY), cY)
	maxX := math.Max(math.Max(aX, bX), cX)
	return c.WithinTriangle(minX, maxX, minY, maxY, aX, aY, ab, bX, bY, bc, cX, cY, ca)
}

// Disjoint computes whether the two bounding boxes are disjoint.
func Disjoint(minX1, maxX1, minY1, maxY1, minX2, maxX2, minY2, maxY2 float64) bool {
	return maxY1 < minY2 || minY1 > maxY2 || maxX1 < minX2 || minX1 > maxX2
}

// Within computes whether the first bounding box 1 is within the second bounding box.
func Within(minX1, maxX1, minY1, maxY1, minX2, maxX2, minY2, maxY2 float64) bool {
	return minY2 <= minY1 && maxY2 >= maxY1 && minX2 <= minX1 && maxX2 >= maxX1
}

// ContainsPoint returns true if rectangle (defined by minX, maxX, minY, maxY) contains the X Y point.
func ContainsPoint(x, y, minX, maxX, minY, maxY float64) bool {
	return x >= minX && x <= maxX && y >= minY && y <= maxY
}

// PointInTriangle computes whether the given x, y point is in a triangle; uses the winding order method.
func PointInTriangle(minX, maxX, minY, maxY, x, y, aX, aY, bX, bY, cX, cY float64) bool {
	// check the bounding box because if the triangle is degenerated, e.g points and lines, we need
	// to filter out coplanar points that are not part of the triangle.
	if x >= minX && x <= maxX && y >= minY && y <= maxY {
		a := Orient(x, y, aX, aY, bX, bY)
		b := Orient(x, y, bX, bY, cX, cY)
		if a == 0 || b == 0 || (a < 0) == (b < 0) {
			c := Orient(x, y, cX, cY, aX, aY)
			return c == 0 || (c < 0) == (b < 0 || a < 0)
		}
		return false
	}
	return false
}
