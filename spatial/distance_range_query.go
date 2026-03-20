// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DistanceRangeQuery finds documents within a distance range from a point.
// This is useful for finding documents that are neither too close nor too far.
type DistanceRangeQuery struct {
	fieldName   string
	center      Point
	minDistance float64
	maxDistance float64
	prefixTree  SpatialPrefixTree
	detailLevel int
	calculator  DistanceCalculator
}

// NewDistanceRangeQuery creates a new distance range query.
//
// Parameters:
//   - fieldName: The field name containing the spatial data
//   - center: The center point for distance calculation
//   - minDistance: The minimum distance (inclusive)
//   - maxDistance: The maximum distance (inclusive)
//   - prefixTree: The spatial prefix tree
//   - detailLevel: The detail level for cell subdivision
//   - calculator: The distance calculator to use
func NewDistanceRangeQuery(fieldName string, center Point, minDistance, maxDistance float64, prefixTree SpatialPrefixTree, detailLevel int, calculator DistanceCalculator) *DistanceRangeQuery {
	return &DistanceRangeQuery{
		fieldName:   fieldName,
		center:      center,
		minDistance: minDistance,
		maxDistance: maxDistance,
		prefixTree:  prefixTree,
		detailLevel: detailLevel,
		calculator:  calculator,
	}
}

// Rewrite rewrites this query into a more primitive form.
// For distance range queries, we create a donut-shaped search area
// and find cells that intersect with this area.
func (q *DistanceRangeQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	// Create the outer search area (max distance)
	outerMinLon := q.center.X - q.maxDistance
	outerMaxLon := q.center.X + q.maxDistance
	outerMinLat := q.center.Y - q.maxDistance
	outerMaxLat := q.center.Y + q.maxDistance

	// Normalize the outer bounding box
	if outerMinLon < -180 {
		outerMinLon = -180
	}
	if outerMaxLon > 180 {
		outerMaxLon = 180
	}
	if outerMinLat < -90 {
		outerMinLat = -90
	}
	if outerMaxLat > 90 {
		outerMaxLat = 90
	}

	// Create a rectangle representing the outer search area
	outerSearchArea := NewRectangle(outerMinLon, outerMinLat, outerMaxLon, outerMaxLat)

	// Get cells that intersect with the outer search area
	cells, err := q.prefixTree.GetCellsForShape(outerSearchArea, q.detailLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells for distance range query: %w", err)
	}

	if len(cells) == 0 {
		return search.NewMatchNoDocsQuery(), nil
	}

	// Extract unique cell tokens
	seenTokens := make(map[string]bool)
	var tokens []string
	for _, cell := range cells {
		token := cell.GetToken()
		if !seenTokens[token] {
			tokens = append(tokens, token)
			seenTokens[token] = true
		}
	}

	// Create a BooleanQuery with TermQuery clauses (OR)
	bq := search.NewBooleanQuery()
	for _, token := range tokens {
		term := index.NewTerm(q.fieldName, token)
		tq := search.NewTermQuery(term)
		bq.Add(tq, search.SHOULD)
	}
	bq.SetMinimumNumberShouldMatch(1)

	return bq, nil
}

// Clone creates a copy of this query.
func (q *DistanceRangeQuery) Clone() search.Query {
	return NewDistanceRangeQuery(q.fieldName, q.center, q.minDistance, q.maxDistance, q.prefixTree, q.detailLevel, q.calculator)
}

// Equals checks if this query equals another.
func (q *DistanceRangeQuery) Equals(other search.Query) bool {
	o, ok := other.(*DistanceRangeQuery)
	if !ok {
		return false
	}
	return q.fieldName == o.fieldName &&
		q.center == o.center &&
		q.minDistance == o.minDistance &&
		q.maxDistance == o.maxDistance
}

// HashCode returns a hash code for this query.
func (q *DistanceRangeQuery) HashCode() int {
	result := 31
	result = 31*result + hashCode(q.fieldName)
	result = 31*result + int(q.minDistance*1000)
	result = 31*result + int(q.maxDistance*1000)
	return result
}

// CreateWeight creates a Weight for this query.
func (q *DistanceRangeQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	rewritten, err := q.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// GetCenter returns the center point for this query.
func (q *DistanceRangeQuery) GetCenter() Point {
	return q.center
}

// GetMinDistance returns the minimum distance threshold.
func (q *DistanceRangeQuery) GetMinDistance() float64 {
	return q.minDistance
}

// GetMaxDistance returns the maximum distance threshold.
func (q *DistanceRangeQuery) GetMaxDistance() float64 {
	return q.maxDistance
}

// GetFieldName returns the field name for this query.
func (q *DistanceRangeQuery) GetFieldName() string {
	return q.fieldName
}

// String returns a string representation of this query.
func (q *DistanceRangeQuery) String() string {
	return fmt.Sprintf("DistanceRangeQuery(field=%s, center=%v, min=%f, max=%f)",
		q.fieldName, q.center, q.minDistance, q.maxDistance)
}

// Ensure DistanceRangeQuery implements Query
var _ search.Query = (*DistanceRangeQuery)(nil)

// ShapeValues provides access to shape values from documents.
// This is used for retrieving spatial data during search and sorting.
type ShapeValues struct {
	shape Shape
}

// NewShapeValues creates a new ShapeValues with the given shape.
func NewShapeValues(shape Shape) *ShapeValues {
	return &ShapeValues{
		shape: shape,
	}
}

// GetShape returns the shape.
func (sv *ShapeValues) GetShape() Shape {
	return sv.shape
}

// GetBoundingBox returns the bounding box of the shape.
func (sv *ShapeValues) GetBoundingBox() *Rectangle {
	if sv.shape == nil {
		return nil
	}
	return sv.shape.GetBoundingBox()
}

// GetCenter returns the center point of the shape.
func (sv *ShapeValues) GetCenter() Point {
	if sv.shape == nil {
		return Point{}
	}
	return sv.shape.GetCenter()
}

// CalculateDistance calculates the distance from this shape to a point.
func (sv *ShapeValues) CalculateDistance(calculator DistanceCalculator, point Point) float64 {
	if sv.shape == nil || calculator == nil {
		return -1
	}
	center := sv.shape.GetCenter()
	return calculator.Distance(center, point)
}

// String returns a string representation of these shape values.
func (sv *ShapeValues) String() string {
	return fmt.Sprintf("ShapeValues{shape=%v}", sv.shape)
}

// ShapeValuesSource provides a ValueSource for shape-based distance calculations.
// This can be used for sorting by distance or returning distance values.
type ShapeValuesSource struct {
	fieldName  string
	center     Point
	calculator DistanceCalculator
}

// NewShapeValuesSource creates a new ShapeValuesSource.
//
// Parameters:
//   - fieldName: The field name containing the shape data
//   - center: The center point for distance calculations
//   - calculator: The distance calculator to use
func NewShapeValuesSource(fieldName string, center Point, calculator DistanceCalculator) *ShapeValuesSource {
	return &ShapeValuesSource{
		fieldName:  fieldName,
		center:     center,
		calculator: calculator,
	}
}

// GetFieldName returns the field name for this value source.
func (svs *ShapeValuesSource) GetFieldName() string {
	return svs.fieldName
}

// GetCenter returns the center point for this value source.
func (svs *ShapeValuesSource) GetCenter() Point {
	return svs.center
}

// Description returns a description of this value source.
func (svs *ShapeValuesSource) Description() string {
	return fmt.Sprintf("shape_distance(%s from %v)", svs.fieldName, svs.center)
}
