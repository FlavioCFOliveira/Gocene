// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/grouping"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// PointVectorStrategy is a SpatialStrategy that indexes points as separate
// X/Y (longitude/latitude) fields using DoublePoint for efficient range queries.
//
// This strategy is ideal for:
// - Simple point data (locations)
// - Range queries on coordinates
// - Distance-based sorting when combined with ValueSource
//
// This is the Go port of Lucene's PointVectorStrategy.
type PointVectorStrategy struct {
	*BaseSpatialStrategy
	xFieldName string
	yFieldName string
}

// NewPointVectorStrategy creates a new PointVectorStrategy.
//
// Parameters:
//   - fieldName: The base field name; x and y fields will be named "{fieldName}_x" and "{fieldName}_y"
//   - ctx: The spatial context for coordinate transformations
//
// Returns an error if the field name is empty or context is nil.
func NewPointVectorStrategy(fieldName string, ctx *SpatialContext) (*PointVectorStrategy, error) {
	base, err := NewBaseSpatialStrategy(fieldName, ctx)
	if err != nil {
		return nil, err
	}

	return &PointVectorStrategy{
		BaseSpatialStrategy: base,
		xFieldName:          fieldName + "_x",
		yFieldName:          fieldName + "_y",
	}, nil
}

// NewPointVectorStrategyWithFieldNames creates a new PointVectorStrategy with custom field names.
//
// Parameters:
//   - fieldName: The base field name
//   - xFieldName: The custom field name for X coordinates (longitude)
//   - yFieldName: The custom field name for Y coordinates (latitude)
//   - ctx: The spatial context
func NewPointVectorStrategyWithFieldNames(fieldName, xFieldName, yFieldName string, ctx *SpatialContext) (*PointVectorStrategy, error) {
	base, err := NewBaseSpatialStrategy(fieldName, ctx)
	if err != nil {
		return nil, err
	}

	if xFieldName == "" || yFieldName == "" {
		return nil, fmt.Errorf("x and y field names cannot be empty")
	}

	return &PointVectorStrategy{
		BaseSpatialStrategy: base,
		xFieldName:          xFieldName,
		yFieldName:          yFieldName,
	}, nil
}

// GetXFieldName returns the field name for X coordinates.
func (s *PointVectorStrategy) GetXFieldName() string {
	return s.xFieldName
}

// GetYFieldName returns the field name for Y coordinates.
func (s *PointVectorStrategy) GetYFieldName() string {
	return s.yFieldName
}

// CreateIndexableFields generates the IndexableField instances for indexing a point shape.
// Creates two DoublePoint fields: one for X (longitude) and one for Y (latitude).
//
// The shape must be a Point shape. For other shape types, an error is returned.
func (s *PointVectorStrategy) CreateIndexableFields(shape Shape) ([]document.IndexableField, error) {
	var point Point
	switch s := shape.(type) {
	case Point:
		point = s
	default:
		// Try to get center from other shapes
		point = shape.GetCenter()
	}

	// Validate coordinates are within world bounds
	bounds := s.spatialContext.WorldBounds
	if point.X < bounds.MinX || point.X > bounds.MaxX {
		return nil, fmt.Errorf("x coordinate %f is outside world bounds [%f, %f]", point.X, bounds.MinX, bounds.MaxX)
	}
	if point.Y < bounds.MinY || point.Y > bounds.MaxY {
		return nil, fmt.Errorf("y coordinate %f is outside world bounds [%f, %f]", point.Y, bounds.MinY, bounds.MaxY)
	}

	// Create DoublePoint fields for X and Y
	xField := document.NewDoublePoint(s.xFieldName, point.X)
	yField := document.NewDoublePoint(s.yFieldName, point.Y)

	return []document.IndexableField{xField, yField}, nil
}

// MakeQuery creates a spatial query for the given operation and shape.
//
// Supports the following operations:
//   - SpatialOperationIntersects: Creates a bounding box query
//   - SpatialOperationIsWithin: Creates a query for points within the shape
//   - SpatialOperationContains: Not fully supported (returns intersection query)
//
// For Point shapes, creates a PointRangeQuery on both X and Y fields.
// For Rectangle shapes, creates a bounding box query.
func (s *PointVectorStrategy) MakeQuery(operation SpatialOperation, shape Shape) (search.Query, error) {
	switch operation {
	case SpatialOperationIntersects:
		return s.makeIntersectsQuery(shape)
	case SpatialOperationIsWithin:
		return s.makeIsWithinQuery(shape)
	case SpatialOperationContains:
		return s.makeContainsQuery(shape)
	default:
		return nil, fmt.Errorf("operation %s not supported by PointVectorStrategy", operation)
	}
}

// makeIntersectsQuery creates a query for shapes that intersect with the query shape.
func (s *PointVectorStrategy) makeIntersectsQuery(shape Shape) (search.Query, error) {
	bbox := shape.GetBoundingBox()

	// Create range queries for X and Y dimensions
	xRangeQuery, err := search.NewPointRangeQuery(
		s.xFieldName,
		document.PackDouble(bbox.MinX),
		document.PackDouble(bbox.MaxX),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create X range query: %w", err)
	}

	yRangeQuery, err := search.NewPointRangeQuery(
		s.yFieldName,
		document.PackDouble(bbox.MinY),
		document.PackDouble(bbox.MaxY),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Y range query: %w", err)
	}

	// Combine with BooleanQuery (AND)
	bq := search.NewBooleanQuery()
	bq.Add(xRangeQuery, search.MUST)
	bq.Add(yRangeQuery, search.MUST)
	return bq, nil
}

// makeIsWithinQuery creates a query for points within the query shape.
// For PointVectorStrategy, this is equivalent to intersects for the bounding box.
func (s *PointVectorStrategy) makeIsWithinQuery(shape Shape) (search.Query, error) {
	// For points indexed as vectors, "within" a shape means the point's coordinates
	// are within the shape's bounding box. For more precise containment,
	// the shape would need to be more complex.
	return s.makeIntersectsQuery(shape)
}

// makeContainsQuery creates a query for shapes containing the query shape.
// PointVectorStrategy doesn't support true contains queries on indexed points.
func (s *PointVectorStrategy) makeContainsQuery(shape Shape) (search.Query, error) {
	// Points can't "contain" other shapes in the traditional sense
	// Return an intersection query as a best-effort approximation
	return s.makeIntersectsQuery(shape)
}

// MakeDistanceValueSource creates a ValueSource that returns the distance
// from indexed points to the specified center point.
//
// The multiplier can be used to convert between distance units.
// For geographic coordinates, the distance is returned in kilometers.
func (s *PointVectorStrategy) MakeDistanceValueSource(center Point, multiplier float64) (grouping.ValueSource, error) {
	return NewDistanceValueSource(s.xFieldName, s.yFieldName, center, multiplier, s.spatialContext.Calculator), nil
}

// DistanceValueSource provides distance values for documents with point fields.
type DistanceValueSource struct {
	xFieldName string
	yFieldName string
	center     Point
	multiplier float64
	calculator DistanceCalculator
}

// NewDistanceValueSource creates a new DistanceValueSource.
func NewDistanceValueSource(xFieldName, yFieldName string, center Point, multiplier float64, calculator DistanceCalculator) *DistanceValueSource {
	return &DistanceValueSource{
		xFieldName: xFieldName,
		yFieldName: yFieldName,
		center:     center,
		multiplier: multiplier,
		calculator: calculator,
	}
}

// GetValues returns the values for the given context.
func (dvs *DistanceValueSource) GetValues(context *index.LeafReaderContext) (grouping.ValueSourceValues, error) {
	return &distanceValueSourceValues{
		xFieldName: dvs.xFieldName,
		yFieldName: dvs.yFieldName,
		center:     dvs.center,
		multiplier: dvs.multiplier,
		calculator: dvs.calculator,
		reader:     context.LeafReader(),
		xValues:    make(map[int]float64),
		yValues:    make(map[int]float64),
	}, nil
}

// Description returns a description of this value source.
func (dvs *DistanceValueSource) Description() string {
	return fmt.Sprintf("distance(%s,%s from %v)", dvs.xFieldName, dvs.yFieldName, dvs.center)
}

// Ensure DistanceValueSource implements ValueSource
var _ grouping.ValueSource = (*DistanceValueSource)(nil)

// distanceValueSourceValues provides distance values for documents.
type distanceValueSourceValues struct {
	xFieldName string
	yFieldName string
	center     Point
	multiplier float64
	calculator DistanceCalculator
	reader     *index.LeafReader
	xValues    map[int]float64
	yValues    map[int]float64
}

// DoubleVal returns the distance value for the given document.
func (dvv *distanceValueSourceValues) DoubleVal(doc int) (float64, error) {
	// Get cached X and Y values
	x, xOk := dvv.xValues[doc]
	if !xOk {
		// Read from doc values
		x = dvv.readXValue(doc)
		dvv.xValues[doc] = x
	}

	y, yOk := dvv.yValues[doc]
	if !yOk {
		// Read from doc values
		y = dvv.readYValue(doc)
		dvv.yValues[doc] = y
	}

	// Calculate distance from center
	point := Point{X: x, Y: y}
	distance := dvv.calculator.Distance(point, dvv.center)

	// Apply multiplier
	return distance * dvv.multiplier, nil
}

// readXValue reads the X coordinate from doc values.
func (dvv *distanceValueSourceValues) readXValue(doc int) float64 {
	// Placeholder implementation
	// In a full implementation, this would read from NumericDocValues
	return 0
}

// readYValue reads the Y coordinate from doc values.
func (dvv *distanceValueSourceValues) readYValue(doc int) float64 {
	// Placeholder implementation
	// In a full implementation, this would read from NumericDocValues
	return 0
}

// FloatVal returns the float value for the given document.
func (dvv *distanceValueSourceValues) FloatVal(doc int) (float32, error) {
	val, err := dvv.DoubleVal(doc)
	return float32(val), err
}

// IntVal returns the int value for the given document.
func (dvv *distanceValueSourceValues) IntVal(doc int) (int, error) {
	val, err := dvv.DoubleVal(doc)
	return int(val), err
}

// LongVal returns the long value for the given document.
func (dvv *distanceValueSourceValues) LongVal(doc int) (int64, error) {
	val, err := dvv.DoubleVal(doc)
	return int64(val), err
}

// StrVal returns the string value for the given document.
func (dvv *distanceValueSourceValues) StrVal(doc int) (string, error) {
	val, err := dvv.DoubleVal(doc)
	return fmt.Sprintf("%f", val), err
}

// Exists returns true if a value exists for the given document.
func (dvv *distanceValueSourceValues) Exists(doc int) bool {
	_, xOk := dvv.xValues[doc]
	_, yOk := dvv.yValues[doc]
	return xOk || yOk
}

// Ensure distanceValueSourceValues implements ValueSourceValues
var _ grouping.ValueSourceValues = (*distanceValueSourceValues)(nil)
