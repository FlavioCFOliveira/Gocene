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

// BBoxStrategy is a SpatialStrategy that indexes shapes as bounding boxes
// using four DoublePoint fields: minX, maxX, minY, maxY.
//
// This strategy is ideal for:
//   - Indexing arbitrary shapes by their bounding box
//   - Fast spatial queries using range filters
//   - Intersects, Within, and Contains operations on bounding boxes
//
// The strategy stores the bounding box coordinates as separate fields
// to enable efficient range queries on each dimension.
//
// This is the Go port of Lucene's BBoxStrategy.
type BBoxStrategy struct {
	*BaseSpatialStrategy
	minXFieldName string
	maxXFieldName string
	minYFieldName string
	maxYFieldName string
}

// NewBBoxStrategy creates a new BBoxStrategy with default field naming.
//
// Parameters:
//   - fieldName: The base field name; bounding box fields will be named
//     "{fieldName}_minX", "{fieldName}_maxX", "{fieldName}_minY", "{fieldName}_maxY"
//   - ctx: The spatial context for coordinate transformations
//
// Returns an error if the field name is empty or context is nil.
func NewBBoxStrategy(fieldName string, ctx *SpatialContext) (*BBoxStrategy, error) {
	base, err := NewBaseSpatialStrategy(fieldName, ctx)
	if err != nil {
		return nil, err
	}

	return &BBoxStrategy{
		BaseSpatialStrategy: base,
		minXFieldName:       fieldName + "_minX",
		maxXFieldName:       fieldName + "_maxX",
		minYFieldName:       fieldName + "_minY",
		maxYFieldName:       fieldName + "_maxY",
	}, nil
}

// NewBBoxStrategyWithFieldNames creates a new BBoxStrategy with custom field names.
//
// Parameters:
//   - fieldName: The base field name
//   - minXFieldName: Custom field name for minimum X coordinate
//   - maxXFieldName: Custom field name for maximum X coordinate
//   - minYFieldName: Custom field name for minimum Y coordinate
//   - maxYFieldName: Custom field name for maximum Y coordinate
//   - ctx: The spatial context
func NewBBoxStrategyWithFieldNames(fieldName, minXFieldName, maxXFieldName, minYFieldName, maxYFieldName string, ctx *SpatialContext) (*BBoxStrategy, error) {
	base, err := NewBaseSpatialStrategy(fieldName, ctx)
	if err != nil {
		return nil, err
	}

	if minXFieldName == "" || maxXFieldName == "" || minYFieldName == "" || maxYFieldName == "" {
		return nil, fmt.Errorf("all bounding box field names must be non-empty")
	}

	return &BBoxStrategy{
		BaseSpatialStrategy: base,
		minXFieldName:       minXFieldName,
		maxXFieldName:       maxXFieldName,
		minYFieldName:       minYFieldName,
		maxYFieldName:       maxYFieldName,
	}, nil
}

// GetMinXFieldName returns the field name for minimum X coordinate.
func (s *BBoxStrategy) GetMinXFieldName() string {
	return s.minXFieldName
}

// GetMaxXFieldName returns the field name for maximum X coordinate.
func (s *BBoxStrategy) GetMaxXFieldName() string {
	return s.maxXFieldName
}

// GetMinYFieldName returns the field name for minimum Y coordinate.
func (s *BBoxStrategy) GetMinYFieldName() string {
	return s.minYFieldName
}

// GetMaxYFieldName returns the field name for maximum Y coordinate.
func (s *BBoxStrategy) GetMaxYFieldName() string {
	return s.maxYFieldName
}

// CreateIndexableFields generates the IndexableField instances for indexing a shape's bounding box.
// Creates four DoublePoint fields: minX, maxX, minY, maxY representing the shape's bounding box.
//
// The shape can be any Shape type (Point, Rectangle, or future shape types).
// The bounding box of the shape is extracted and stored.
func (s *BBoxStrategy) CreateIndexableFields(shape Shape) ([]document.IndexableField, error) {
	bbox := shape.GetBoundingBox()

	// Validate coordinates are within world bounds
	bounds := s.spatialContext.WorldBounds
	if bbox.MinX < bounds.MinX || bbox.MaxX > bounds.MaxX {
		return nil, fmt.Errorf("X coordinates [%f, %f] are outside world bounds [%f, %f]",
			bbox.MinX, bbox.MaxX, bounds.MinX, bounds.MaxX)
	}
	if bbox.MinY < bounds.MinY || bbox.MaxY > bounds.MaxY {
		return nil, fmt.Errorf("Y coordinates [%f, %f] are outside world bounds [%f, %f]",
			bbox.MinY, bbox.MaxY, bounds.MinY, bounds.MaxY)
	}

	// Validate that min <= max
	if bbox.MinX > bbox.MaxX {
		return nil, fmt.Errorf("minX (%f) cannot be greater than maxX (%f)", bbox.MinX, bbox.MaxX)
	}
	if bbox.MinY > bbox.MaxY {
		return nil, fmt.Errorf("minY (%f) cannot be greater than maxY (%f)", bbox.MinY, bbox.MaxY)
	}

	// Create DoublePoint fields for bounding box coordinates
	minXField := document.NewDoublePoint(s.minXFieldName, bbox.MinX)
	maxXField := document.NewDoublePoint(s.maxXFieldName, bbox.MaxX)
	minYField := document.NewDoublePoint(s.minYFieldName, bbox.MinY)
	maxYField := document.NewDoublePoint(s.maxYFieldName, bbox.MaxY)

	return []document.IndexableField{minXField, maxXField, minYField, maxYField}, nil
}

// MakeQuery creates a spatial query for the given operation and shape.
//
// Supports the following operations:
//   - SpatialOperationIntersects: Matches shapes whose bounding boxes intersect the query shape's bounding box
//   - SpatialOperationIsWithin: Matches shapes that are completely within the query shape's bounding box
//   - SpatialOperationContains: Matches shapes that completely contain the query shape's bounding box
//
// For all operations, the query shape's bounding box is used.
func (s *BBoxStrategy) MakeQuery(operation SpatialOperation, shape Shape) (search.Query, error) {
	switch operation {
	case SpatialOperationIntersects:
		return s.makeIntersectsQuery(shape)
	case SpatialOperationIsWithin:
		return s.makeIsWithinQuery(shape)
	case SpatialOperationContains:
		return s.makeContainsQuery(shape)
	default:
		return nil, fmt.Errorf("operation %s not supported by BBoxStrategy", operation)
	}
}

// makeIntersectsQuery creates a query for shapes that intersect with the query shape's bounding box.
//
// Two bounding boxes intersect if:
//   - box1.minX <= box2.maxX AND box1.maxX >= box2.minX (X overlap)
//   - box1.minY <= box2.maxY AND box1.maxY >= box2.minY (Y overlap)
//
// This is implemented as a boolean query with range constraints.
func (s *BBoxStrategy) makeIntersectsQuery(shape Shape) (search.Query, error) {
	bbox := shape.GetBoundingBox()

	// For intersection, we need:
// indexed_minX <= query_maxX AND indexed_maxX >= query_minX
// indexed_minY <= query_maxY AND indexed_maxY >= query_minY

	// Create range queries for each condition
	minXQuery, err := search.NewPointRangeQuery(
		s.minXFieldName,
		document.PackDouble(s.spatialContext.WorldBounds.MinX),
		document.PackDouble(bbox.MaxX),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create minX range query: %w", err)
	}

	maxXQuery, err := search.NewPointRangeQuery(
		s.maxXFieldName,
		document.PackDouble(bbox.MinX),
		document.PackDouble(s.spatialContext.WorldBounds.MaxX),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create maxX range query: %w", err)
	}

	minYQuery, err := search.NewPointRangeQuery(
		s.minYFieldName,
		document.PackDouble(s.spatialContext.WorldBounds.MinY),
		document.PackDouble(bbox.MaxY),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create minY range query: %w", err)
	}

	maxYQuery, err := search.NewPointRangeQuery(
		s.maxYFieldName,
		document.PackDouble(bbox.MinY),
		document.PackDouble(s.spatialContext.WorldBounds.MaxY),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create maxY range query: %w", err)
	}

	// Combine with BooleanQuery (AND)
	bq := search.NewBooleanQuery()
	bq.Add(minXQuery, search.MUST)
	bq.Add(maxXQuery, search.MUST)
	bq.Add(minYQuery, search.MUST)
	bq.Add(maxYQuery, search.MUST)

	return bq, nil
}

// makeIsWithinQuery creates a query for shapes that are within the query shape's bounding box.
//
// A shape is within another if:
//   - indexed_minX >= query_minX AND indexed_maxX <= query_maxX
//   - indexed_minY >= query_minY AND indexed_maxY <= query_maxY
func (s *BBoxStrategy) makeIsWithinQuery(shape Shape) (search.Query, error) {
	bbox := shape.GetBoundingBox()

	// For "within", we need:
// indexed_minX >= query_minX AND indexed_maxX <= query_maxX
// indexed_minY >= query_minY AND indexed_maxY <= query_maxY

	minXQuery, err := search.NewPointRangeQuery(
		s.minXFieldName,
		document.PackDouble(bbox.MinX),
		document.PackDouble(s.spatialContext.WorldBounds.MaxX),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create minX range query: %w", err)
	}

	maxXQuery, err := search.NewPointRangeQuery(
		s.maxXFieldName,
		document.PackDouble(s.spatialContext.WorldBounds.MinX),
		document.PackDouble(bbox.MaxX),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create maxX range query: %w", err)
	}

	minYQuery, err := search.NewPointRangeQuery(
		s.minYFieldName,
		document.PackDouble(bbox.MinY),
		document.PackDouble(s.spatialContext.WorldBounds.MaxY),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create minY range query: %w", err)
	}

	maxYQuery, err := search.NewPointRangeQuery(
		s.maxYFieldName,
		document.PackDouble(s.spatialContext.WorldBounds.MinY),
		document.PackDouble(bbox.MaxY),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create maxY range query: %w", err)
	}

	// Combine with BooleanQuery (AND)
	bq := search.NewBooleanQuery()
	bq.Add(minXQuery, search.MUST)
	bq.Add(maxXQuery, search.MUST)
	bq.Add(minYQuery, search.MUST)
	bq.Add(maxYQuery, search.MUST)

	return bq, nil
}

// makeContainsQuery creates a query for shapes that contain the query shape's bounding box.
//
// A shape contains another if:
//   - indexed_minX <= query_minX AND indexed_maxX >= query_maxX
//   - indexed_minY <= query_minY AND indexed_maxY >= query_maxY
func (s *BBoxStrategy) makeContainsQuery(shape Shape) (search.Query, error) {
	bbox := shape.GetBoundingBox()

	// For "contains", we need:
// indexed_minX <= query_minX AND indexed_maxX >= query_maxX
// indexed_minY <= query_minY AND indexed_maxY >= query_maxY

	minXQuery, err := search.NewPointRangeQuery(
		s.minXFieldName,
		document.PackDouble(s.spatialContext.WorldBounds.MinX),
		document.PackDouble(bbox.MinX),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create minX range query: %w", err)
	}

	maxXQuery, err := search.NewPointRangeQuery(
		s.maxXFieldName,
		document.PackDouble(bbox.MaxX),
		document.PackDouble(s.spatialContext.WorldBounds.MaxX),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create maxX range query: %w", err)
	}

	minYQuery, err := search.NewPointRangeQuery(
		s.minYFieldName,
		document.PackDouble(s.spatialContext.WorldBounds.MinY),
		document.PackDouble(bbox.MinY),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create minY range query: %w", err)
	}

	maxYQuery, err := search.NewPointRangeQuery(
		s.maxYFieldName,
		document.PackDouble(bbox.MaxY),
		document.PackDouble(s.spatialContext.WorldBounds.MaxY),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create maxY range query: %w", err)
	}

	// Combine with BooleanQuery (AND)
	bq := search.NewBooleanQuery()
	bq.Add(minXQuery, search.MUST)
	bq.Add(maxXQuery, search.MUST)
	bq.Add(minYQuery, search.MUST)
	bq.Add(maxYQuery, search.MUST)

	return bq, nil
}

// MakeDistanceValueSource creates a ValueSource that returns the distance
// from the center of indexed shapes to the specified point.
//
// The distance is calculated from the center of each shape's bounding box
// to the specified center point. The multiplier can be used to convert
// between distance units.
func (s *BBoxStrategy) MakeDistanceValueSource(center Point, multiplier float64) (grouping.ValueSource, error) {
	return NewBBoxDistanceValueSource(
		s.minXFieldName,
		s.maxXFieldName,
		s.minYFieldName,
		s.maxYFieldName,
		center,
		multiplier,
		s.spatialContext.Calculator,
	), nil
}

// BBoxDistanceValueSource provides distance values from the center of bounding boxes.
type BBoxDistanceValueSource struct {
	minXFieldName string
	maxXFieldName string
	minYFieldName string
	maxYFieldName string
	center        Point
	multiplier    float64
	calculator    DistanceCalculator
}

// NewBBoxDistanceValueSource creates a new BBoxDistanceValueSource.
func NewBBoxDistanceValueSource(minXFieldName, maxXFieldName, minYFieldName, maxYFieldName string, center Point, multiplier float64, calculator DistanceCalculator) *BBoxDistanceValueSource {
	return &BBoxDistanceValueSource{
		minXFieldName: minXFieldName,
		maxXFieldName: maxXFieldName,
		minYFieldName: minYFieldName,
		maxYFieldName: maxYFieldName,
		center:        center,
		multiplier:    multiplier,
		calculator:    calculator,
	}
}

// GetValues returns the values for the given context.
func (dvs *BBoxDistanceValueSource) GetValues(context *index.LeafReaderContext) (grouping.ValueSourceValues, error) {
	return &bboxDistanceValueSourceValues{
		minXFieldName: dvs.minXFieldName,
		maxXFieldName: dvs.maxXFieldName,
		minYFieldName: dvs.minYFieldName,
		maxYFieldName: dvs.maxYFieldName,
		center:        dvs.center,
		multiplier:    dvs.multiplier,
		calculator:    dvs.calculator,
		minXValues:    make(map[int]float64),
		maxXValues:    make(map[int]float64),
		minYValues:    make(map[int]float64),
		maxYValues:    make(map[int]float64),
	}, nil
}

// Description returns a description of this value source.
func (dvs *BBoxDistanceValueSource) Description() string {
	return fmt.Sprintf("bbox_distance(%s,%s,%s,%s from %v)",
		dvs.minXFieldName, dvs.maxXFieldName, dvs.minYFieldName, dvs.maxYFieldName, dvs.center)
}

// Ensure BBoxDistanceValueSource implements ValueSource
var _ grouping.ValueSource = (*BBoxDistanceValueSource)(nil)

// bboxDistanceValueSourceValues provides distance values for documents with bounding box fields.
type bboxDistanceValueSourceValues struct {
	minXFieldName string
	maxXFieldName string
	minYFieldName string
	maxYFieldName string
	center        Point
	multiplier    float64
	calculator    DistanceCalculator
	minXValues    map[int]float64
	maxXValues    map[int]float64
	minYValues    map[int]float64
	maxYValues    map[int]float64
}

// DoubleVal returns the distance value for the given document.
// The distance is calculated from the center of the shape's bounding box.
func (dvv *bboxDistanceValueSourceValues) DoubleVal(doc int) (float64, error) {
	// Get cached values
	minX, minXOk := dvv.minXValues[doc]
	if !minXOk {
		minX = dvv.readMinXValue(doc)
		dvv.minXValues[doc] = minX
	}

	maxX, maxXOk := dvv.maxXValues[doc]
	if !maxXOk {
		maxX = dvv.readMaxXValue(doc)
		dvv.maxXValues[doc] = maxX
	}

	minY, minYOk := dvv.minYValues[doc]
	if !minYOk {
		minY = dvv.readMinYValue(doc)
		dvv.minYValues[doc] = minY
	}

	maxY, maxYOk := dvv.maxYValues[doc]
	if !maxYOk {
		maxY = dvv.readMaxYValue(doc)
		dvv.maxYValues[doc] = maxY
	}

	// Calculate center of the bounding box
	bboxCenter := Point{
		X: (minX + maxX) / 2,
		Y: (minY + maxY) / 2,
	}

	// Calculate distance from center
	distance := dvv.calculator.Distance(bboxCenter, dvv.center)

	// Apply multiplier
	return distance * dvv.multiplier, nil
}

// readMinXValue reads the minX coordinate from doc values.
func (dvv *bboxDistanceValueSourceValues) readMinXValue(doc int) float64 {
	// Placeholder implementation
	// In a full implementation, this would read from NumericDocValues
	return 0
}

// readMaxXValue reads the maxX coordinate from doc values.
func (dvv *bboxDistanceValueSourceValues) readMaxXValue(doc int) float64 {
	// Placeholder implementation
	return 0
}

// readMinYValue reads the minY coordinate from doc values.
func (dvv *bboxDistanceValueSourceValues) readMinYValue(doc int) float64 {
	// Placeholder implementation
	return 0
}

// readMaxYValue reads the maxY coordinate from doc values.
func (dvv *bboxDistanceValueSourceValues) readMaxYValue(doc int) float64 {
	// Placeholder implementation
	return 0
}

// FloatVal returns the float value for the given document.
func (dvv *bboxDistanceValueSourceValues) FloatVal(doc int) (float32, error) {
	val, err := dvv.DoubleVal(doc)
	return float32(val), err
}

// IntVal returns the int value for the given document.
func (dvv *bboxDistanceValueSourceValues) IntVal(doc int) (int, error) {
	val, err := dvv.DoubleVal(doc)
	return int(val), err
}

// LongVal returns the long value for the given document.
func (dvv *bboxDistanceValueSourceValues) LongVal(doc int) (int64, error) {
	val, err := dvv.DoubleVal(doc)
	return int64(val), err
}

// StrVal returns the string value for the given document.
func (dvv *bboxDistanceValueSourceValues) StrVal(doc int) (string, error) {
	val, err := dvv.DoubleVal(doc)
	return fmt.Sprintf("%f", val), err
}

// Exists returns true if a value exists for the given document.
func (dvv *bboxDistanceValueSourceValues) Exists(doc int) bool {
	_, minXOk := dvv.minXValues[doc]
	_, maxXOk := dvv.maxXValues[doc]
	_, minYOk := dvv.minYValues[doc]
	_, maxYOk := dvv.maxYValues[doc]
	return minXOk || maxXOk || minYOk || maxYOk
}

// Ensure bboxDistanceValueSourceValues implements ValueSourceValues
var _ grouping.ValueSourceValues = (*bboxDistanceValueSourceValues)(nil)
