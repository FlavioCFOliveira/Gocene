// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"
	"strings"
)

// SpatialOperationBboxIntersects matches bounding boxes that intersect.
const SpatialOperationBboxIntersects SpatialOperation = 6

// SpatialOperationBboxWithin matches bounding boxes that are within.
const SpatialOperationBboxWithin SpatialOperation = 7

// GetSpatialOperationFromString returns the SpatialOperation for a given string.
// Returns (SpatialOperationIntersects, false) if the string is not recognized.
func GetSpatialOperationFromString(s string) (SpatialOperation, bool) {
	switch strings.ToLower(s) {
	case "intersects":
		return SpatialOperationIntersects, true
	case "iswithin", "within":
		return SpatialOperationIsWithin, true
	case "contains":
		return SpatialOperationContains, true
	case "disjoint", "isdisjointto":
		return SpatialOperationIsDisjointTo, true
	case "equals":
		return SpatialOperationEquals, true
	case "overlaps":
		return SpatialOperationOverlaps, true
	case "bboxintersects":
		return SpatialOperationBboxIntersects, true
	case "bboxwithin":
		return SpatialOperationBboxWithin, true
	default:
		return SpatialOperationIntersects, false
	}
}

// AllSpatialOperations returns all supported spatial operations.
func AllSpatialOperations() []SpatialOperation {
	return []SpatialOperation{
		SpatialOperationIntersects,
		SpatialOperationIsWithin,
		SpatialOperationContains,
		SpatialOperationIsDisjointTo,
		SpatialOperationEquals,
		SpatialOperationOverlaps,
		SpatialOperationBboxIntersects,
		SpatialOperationBboxWithin,
	}
}

// SpatialArgs holds the arguments for a spatial query.
// This includes the shape, operation, and optional parameters.
type SpatialArgs struct {
	// Operation is the spatial operation to perform.
	Operation SpatialOperation

	// Shape is the query shape.
	Shape Shape

	// DistErrPct is the maximum distance error percentage (0.0 to 1.0).
	// This controls the precision of the spatial calculation.
	DistErrPct float64

	// DistErr is the maximum distance error in degrees.
	// If set, this overrides DistErrPct.
	DistErr float64
}

// NewSpatialArgs creates a new SpatialArgs with the given operation and shape.
func NewSpatialArgs(operation SpatialOperation, shape Shape) *SpatialArgs {
	return &SpatialArgs{
		Operation:  operation,
		Shape:      shape,
		DistErrPct: 0.025, // Default 2.5% error
		DistErr:    -1,    // Use DistErrPct by default
	}
}

// GetOperation returns the spatial operation.
func (sa *SpatialArgs) GetOperation() SpatialOperation {
	return sa.Operation
}

// GetShape returns the query shape.
func (sa *SpatialArgs) GetShape() Shape {
	return sa.Shape
}

// GetDistErrPct returns the distance error percentage.
func (sa *SpatialArgs) GetDistErrPct() float64 {
	return sa.DistErrPct
}

// SetDistErrPct sets the distance error percentage.
func (sa *SpatialArgs) SetDistErrPct(distErrPct float64) {
	sa.DistErrPct = distErrPct
}

// GetDistErr returns the distance error in degrees.
// If DistErr is not set (negative), calculates from DistErrPct.
func (sa *SpatialArgs) GetDistErr(ctx *SpatialContext) float64 {
	if sa.DistErr >= 0 {
		return sa.DistErr
	}
	// Calculate from DistErrPct based on shape extent
	if sa.Shape == nil {
		return 0
	}
	bbox := sa.Shape.GetBoundingBox()
	if bbox == nil {
		return 0
	}
	// Use the larger dimension
	width := bbox.MaxX - bbox.MinX
	height := bbox.MaxY - bbox.MinY
	maxDim := width
	if height > maxDim {
		maxDim = height
	}
	return maxDim * sa.DistErrPct
}

// SetDistErr sets the distance error in degrees.
func (sa *SpatialArgs) SetDistErr(distErr float64) {
	sa.DistErr = distErr
}

// String returns a string representation of these arguments.
func (sa *SpatialArgs) String() string {
	return fmt.Sprintf("SpatialArgs{op=%s, shape=%v, distErrPct=%f}",
		sa.Operation, sa.Shape, sa.DistErrPct)
}

// SpatialArgsParser parses spatial argument strings into SpatialArgs.
//
// The parser supports formats like:
//   - "Intersects(POINT(-1 1))"
//   - "IsWithin(CIRCLE(-1 1 d=10))"
//   - "Contains(ENVELOPE(-1, 1, 1, -1))"
type SpatialArgsParser struct {
	ctx *SpatialContext
}

// NewSpatialArgsParser creates a new parser with the given spatial context.
func NewSpatialArgsParser(ctx *SpatialContext) *SpatialArgsParser {
	return &SpatialArgsParser{
		ctx: ctx,
	}
}

// Parse parses a spatial argument string into SpatialArgs.
//
// Supported formats:
//   - "Intersects(POINT(-1 1))" - Point intersection
//   - "IsWithin(CIRCLE(-1 1 d=10))" - Circle within
//   - "Contains(ENVELOPE(-1, 1, 1, -1))" - Envelope contains
//   - "Intersects(BBOX(-1, 1, 1, -1))" - Bounding box intersection
func (p *SpatialArgsParser) Parse(arg string) (*SpatialArgs, error) {
	if arg == "" {
		return nil, fmt.Errorf("empty spatial argument")
	}

	// Trim whitespace
	arg = strings.TrimSpace(arg)

	// Find the opening parenthesis
	idx := strings.Index(arg, "(")
	if idx == -1 {
		return nil, fmt.Errorf("invalid spatial argument format: missing '('")
	}

	// Extract operation name
	opName := strings.TrimSpace(arg[:idx])
	if opName == "" {
		return nil, fmt.Errorf("missing operation name")
	}

	// Parse operation
	operation, ok := GetSpatialOperationFromString(opName)
	if !ok {
		return nil, fmt.Errorf("unknown spatial operation: %s", opName)
	}

	// Check for closing parenthesis
	if !strings.HasSuffix(arg, ")") {
		return nil, fmt.Errorf("invalid spatial argument format: missing ')'")
	}

	// Extract shape string
	shapeStr := arg[idx+1 : len(arg)-1]
	shapeStr = strings.TrimSpace(shapeStr)

	// Parse shape
	shape, err := p.parseShape(shapeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse shape: %w", err)
	}

	// Create SpatialArgs
	args := NewSpatialArgs(operation, shape)

	return args, nil
}

// parseShape parses a shape string into a Shape.
//
// Supported formats:
//   - "POINT(-1 1)" - Point
//   - "CIRCLE(-1 1 d=10)" - Circle with center and distance
//   - "ENVELOPE(-1, 1, 1, -1)" - Envelope (minX, maxX, maxY, minY)
//   - "BBOX(-1, 1, 1, -1)" - Bounding box (minX, maxX, maxY, minY)
//   - "RECTANGLE(-1, 1, -1, 1)" - Rectangle (minX, maxX, minY, maxY)
func (p *SpatialArgsParser) parseShape(shapeStr string) (Shape, error) {
	shapeStr = strings.TrimSpace(shapeStr)

	// Find the opening parenthesis
	idx := strings.Index(shapeStr, "(")
	if idx == -1 {
		return nil, fmt.Errorf("invalid shape format: missing '('")
	}

	shapeType := strings.ToUpper(strings.TrimSpace(shapeStr[:idx]))
	if !strings.HasSuffix(shapeStr, ")") {
		return nil, fmt.Errorf("invalid shape format: missing ')'")
	}

	// Extract parameters
	paramsStr := shapeStr[idx+1 : len(shapeStr)-1]
	paramsStr = strings.TrimSpace(paramsStr)

	switch shapeType {
	case "POINT":
		return p.parsePoint(paramsStr)
	case "CIRCLE":
		return p.parseCircle(paramsStr)
	case "ENVELOPE", "BBOX":
		return p.parseEnvelope(paramsStr)
	case "RECTANGLE":
		return p.parseRectangle(paramsStr)
	default:
		return nil, fmt.Errorf("unsupported shape type: %s", shapeType)
	}
}

// parsePoint parses a point from a string like "-1 1".
func (p *SpatialArgsParser) parsePoint(paramsStr string) (Point, error) {
	parts := strings.Fields(paramsStr)
	if len(parts) != 2 {
		return Point{}, fmt.Errorf("point requires 2 coordinates, got %d", len(parts))
	}

	var x, y float64
	if _, err := fmt.Sscanf(parts[0], "%f", &x); err != nil {
		return Point{}, fmt.Errorf("invalid x coordinate: %s", parts[0])
	}
	if _, err := fmt.Sscanf(parts[1], "%f", &y); err != nil {
		return Point{}, fmt.Errorf("invalid y coordinate: %s", parts[1])
	}

	return Point{X: x, Y: y}, nil
}

// parseCircle parses a circle from a string like "-1 1 d=10".
func (p *SpatialArgsParser) parseCircle(paramsStr string) (*Circle, error) {
	// Split by space, but also handle d= parameter
	parts := strings.Fields(paramsStr)
	if len(parts) < 2 {
		return nil, fmt.Errorf("circle requires center coordinates")
	}

	var x, y float64
	if _, err := fmt.Sscanf(parts[0], "%f", &x); err != nil {
		return nil, fmt.Errorf("invalid x coordinate: %s", parts[0])
	}
	if _, err := fmt.Sscanf(parts[1], "%f", &y); err != nil {
		return nil, fmt.Errorf("invalid y coordinate: %s", parts[1])
	}

	// Default radius
	radius := 1.0

	// Parse optional parameters
	for i := 2; i < len(parts); i++ {
		param := parts[i]
		if strings.HasPrefix(param, "d=") {
			if _, err := fmt.Sscanf(param[2:], "%f", &radius); err != nil {
				return nil, fmt.Errorf("invalid distance: %s", param[2:])
			}
		} else if strings.HasPrefix(param, "radius=") {
			if _, err := fmt.Sscanf(param[7:], "%f", &radius); err != nil {
				return nil, fmt.Errorf("invalid radius: %s", param[7:])
			}
		}
	}

	return NewCircle(x, y, radius), nil
}

// parseEnvelope parses an envelope from a string like "-1, 1, 1, -1".
// Format: minX, maxX, maxY, minY (OGC standard)
func (p *SpatialArgsParser) parseEnvelope(paramsStr string) (*Rectangle, error) {
	// Split by comma
	parts := strings.Split(paramsStr, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("envelope requires 4 values (minX, maxX, maxY, minY), got %d", len(parts))
	}

	var minX, maxX, maxY, minY float64
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &minX); err != nil {
		return nil, fmt.Errorf("invalid minX: %s", parts[0])
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &maxX); err != nil {
		return nil, fmt.Errorf("invalid maxX: %s", parts[1])
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[2]), "%f", &maxY); err != nil {
		return nil, fmt.Errorf("invalid maxY: %s", parts[2])
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[3]), "%f", &minY); err != nil {
		return nil, fmt.Errorf("invalid minY: %s", parts[3])
	}

	return NewRectangle(minX, minY, maxX, maxY), nil
}

// parseRectangle parses a rectangle from a string like "-1, 1, -1, 1".
// Format: minX, maxX, minY, maxY
func (p *SpatialArgsParser) parseRectangle(paramsStr string) (*Rectangle, error) {
	// Split by comma
	parts := strings.Split(paramsStr, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("rectangle requires 4 values (minX, maxX, minY, maxY), got %d", len(parts))
	}

	var minX, maxX, minY, maxY float64
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &minX); err != nil {
		return nil, fmt.Errorf("invalid minX: %s", parts[0])
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &maxX); err != nil {
		return nil, fmt.Errorf("invalid maxX: %s", parts[1])
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[2]), "%f", &minY); err != nil {
		return nil, fmt.Errorf("invalid minY: %s", parts[2])
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[3]), "%f", &maxY); err != nil {
		return nil, fmt.Errorf("invalid maxY: %s", parts[3])
	}

	return NewRectangle(minX, minY, maxX, maxY), nil
}

// Circle represents a circular shape.
type Circle struct {
	Center Point
	Radius float64
}

// NewCircle creates a new circle with the given center and radius.
func NewCircle(x, y, radius float64) *Circle {
	return &Circle{
		Center: Point{X: x, Y: y},
		Radius: radius,
	}
}

// GetBoundingBox returns the bounding box of this circle.
func (c *Circle) GetBoundingBox() *Rectangle {
	return NewRectangle(
		c.Center.X-c.Radius,
		c.Center.Y-c.Radius,
		c.Center.X+c.Radius,
		c.Center.Y+c.Radius,
	)
}

// GetCenter returns the center point of this circle.
func (c *Circle) GetCenter() Point {
	return c.Center
}

// Intersects checks if this circle intersects with another shape.
func (c *Circle) Intersects(other Shape) bool {
	// Simplified intersection check using bounding boxes
	return c.GetBoundingBox().Intersects(other)
}

// Contains checks if this circle contains another shape.
func (c *Circle) Contains(other Shape) bool {
	// Simplified containment check
	otherBBox := other.GetBoundingBox()
	if otherBBox == nil {
		return false
	}
	// Check if all corners of other are within the circle
	corners := []Point{
		{X: otherBBox.MinX, Y: otherBBox.MinY},
		{X: otherBBox.MinX, Y: otherBBox.MaxY},
		{X: otherBBox.MaxX, Y: otherBBox.MinY},
		{X: otherBBox.MaxX, Y: otherBBox.MaxY},
	}
	for _, corner := range corners {
		dx := corner.X - c.Center.X
		dy := corner.Y - c.Center.Y
		dist := dx*dx + dy*dy
		if dist > c.Radius*c.Radius {
			return false
		}
	}
	return true
}

// IsWithin checks if this circle is within another shape.
func (c *Circle) IsWithin(other Shape) bool {
	// Check if the circle's bounding box is within the other shape
	return c.GetBoundingBox().IsWithin(other)
}

// String returns a string representation of this circle.
func (c *Circle) String() string {
	return fmt.Sprintf("Circle(%v, radius=%f)", c.Center, c.Radius)
}

// Ensure Circle implements Shape
var _ Shape = (*Circle)(nil)
