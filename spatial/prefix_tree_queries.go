// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// PrefixTreeQuery provides base functionality for prefix tree spatial queries.
// It handles the common logic of finding candidate documents that intersect
// with the query shape's cells.
type PrefixTreeQuery struct {
	fieldName   string
	queryShape  Shape
	prefixTree  SpatialPrefixTree
	detailLevel int
	operation   string
}

// NewPrefixTreeQuery creates a new base prefix tree query.
func NewPrefixTreeQuery(fieldName string, queryShape Shape, prefixTree SpatialPrefixTree, detailLevel int, operation string) *PrefixTreeQuery {
	return &PrefixTreeQuery{
		fieldName:   fieldName,
		queryShape:  queryShape,
		prefixTree:  prefixTree,
		detailLevel: detailLevel,
		operation:   operation,
	}
}

// GetFieldName returns the field name for this query.
func (q *PrefixTreeQuery) GetFieldName() string {
	return q.fieldName
}

// GetQueryShape returns the query shape.
func (q *PrefixTreeQuery) GetQueryShape() Shape {
	return q.queryShape
}

// IntersectsPrefixTreeQuery finds documents that intersect with the query shape.
// This is a filtering query that identifies candidate documents based on
// prefix tree cell intersections.
type IntersectsPrefixTreeQuery struct {
	*PrefixTreeQuery
	fieldCacheProvider *SpatialPrefixTreeFieldCacheProvider
}

// NewIntersectsPrefixTreeQuery creates a new intersects query.
//
// Parameters:
//   - fieldName: The field name containing the prefix tree grid
//   - queryShape: The shape to test intersection against
//   - prefixTree: The spatial prefix tree for cell calculations
//   - detailLevel: The detail level for cell subdivision
func NewIntersectsPrefixTreeQuery(fieldName string, queryShape Shape, prefixTree SpatialPrefixTree, detailLevel int) *IntersectsPrefixTreeQuery {
	return &IntersectsPrefixTreeQuery{
		PrefixTreeQuery: NewPrefixTreeQuery(fieldName, queryShape, prefixTree, detailLevel, "Intersects"),
	}
}

// Rewrite rewrites this query into a more primitive form (TermsQuery).
// This finds all cells that intersect with the query shape and creates
// a Boolean OR query over those cell terms.
func (q *IntersectsPrefixTreeQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	// Get cells that intersect with the query shape
	cells, err := q.prefixTree.GetCellsForShape(q.queryShape, q.detailLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells for query shape: %w", err)
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

	// The minimum should match is 1 (at least one cell must match)
	bq.SetMinimumNumberShouldMatch(1)

	return bq, nil
}

// Clone creates a copy of this query.
func (q *IntersectsPrefixTreeQuery) Clone() search.Query {
	return NewIntersectsPrefixTreeQuery(q.fieldName, q.queryShape, q.prefixTree, q.detailLevel)
}

// Equals checks if this query equals another.
func (q *IntersectsPrefixTreeQuery) Equals(other search.Query) bool {
	o, ok := other.(*IntersectsPrefixTreeQuery)
	if !ok {
		return false
	}
	return q.fieldName == o.fieldName &&
		q.detailLevel == o.detailLevel &&
		q.queryShape == o.queryShape
}

// HashCode returns a hash code for this query.
func (q *IntersectsPrefixTreeQuery) HashCode() int {
	result := 17
	result = 31*result + hashCode(q.fieldName)
	result = 31*result + q.detailLevel
	return result
}

// CreateWeight creates a Weight for this query.
func (q *IntersectsPrefixTreeQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	// Rewrite and then create weight
	rewritten, err := q.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// String returns a string representation of this query.
func (q *IntersectsPrefixTreeQuery) String() string {
	return fmt.Sprintf("IntersectsPrefixTreeQuery(field=%s, shape=%v, level=%d)",
		q.fieldName, q.queryShape, q.detailLevel)
}

// Ensure IntersectsPrefixTreeQuery implements Query
var _ search.Query = (*IntersectsPrefixTreeQuery)(nil)

// IsWithinPrefixTreeQuery finds documents that are fully within the query shape.
//
// Note: This query returns candidates that need post-filtering to ensure
// the indexed shape is actually within (not just intersecting with) the
// query shape.
type IsWithinPrefixTreeQuery struct {
	*PrefixTreeQuery
}

// NewIsWithinPrefixTreeQuery creates a new "within" query.
//
// Parameters:
//   - fieldName: The field name containing the prefix tree grid
//   - queryShape: The shape to test containment against
//   - prefixTree: The spatial prefix tree for cell calculations
//   - detailLevel: The detail level for cell subdivision
func NewIsWithinPrefixTreeQuery(fieldName string, queryShape Shape, prefixTree SpatialPrefixTree, detailLevel int) *IsWithinPrefixTreeQuery {
	return &IsWithinPrefixTreeQuery{
		PrefixTreeQuery: NewPrefixTreeQuery(fieldName, queryShape, prefixTree, detailLevel, "IsWithin"),
	}
}

// Rewrite rewrites this query into a more primitive form.
// For "within" queries, we need to be more selective - we only want documents
// whose cells are completely within the query shape's cells.
func (q *IsWithinPrefixTreeQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	// For now, use the same approach as intersects, but the distinction
	// is in how the results are post-filtered
	// In a full implementation, we would calculate cells that are fully contained
	cells, err := q.prefixTree.GetCellsForShape(q.queryShape, q.detailLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells for query shape: %w", err)
	}

	if len(cells) == 0 {
		return search.NewMatchNoDocsQuery(), nil
	}

	// For IsWithin, we want documents whose indexed cells are completely
	// contained within the query shape. This requires post-filtering.
	// For now, we return the same candidate set as Intersects.
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
func (q *IsWithinPrefixTreeQuery) Clone() search.Query {
	return NewIsWithinPrefixTreeQuery(q.fieldName, q.queryShape, q.prefixTree, q.detailLevel)
}

// Equals checks if this query equals another.
func (q *IsWithinPrefixTreeQuery) Equals(other search.Query) bool {
	o, ok := other.(*IsWithinPrefixTreeQuery)
	if !ok {
		return false
	}
	return q.fieldName == o.fieldName &&
		q.detailLevel == o.detailLevel &&
		q.queryShape == o.queryShape
}

// HashCode returns a hash code for this query.
func (q *IsWithinPrefixTreeQuery) HashCode() int {
	result := 19
	result = 31*result + hashCode(q.fieldName)
	result = 31*result + q.detailLevel
	return result
}

// CreateWeight creates a Weight for this query.
func (q *IsWithinPrefixTreeQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	rewritten, err := q.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// String returns a string representation of this query.
func (q *IsWithinPrefixTreeQuery) String() string {
	return fmt.Sprintf("IsWithinPrefixTreeQuery(field=%s, shape=%v, level=%d)",
		q.fieldName, q.queryShape, q.detailLevel)
}

// Ensure IsWithinPrefixTreeQuery implements Query
var _ search.Query = (*IsWithinPrefixTreeQuery)(nil)

// ContainsPrefixTreeQuery finds documents that contain the query shape.
//
// Note: This is the inverse of IsWithin - it finds indexed shapes that
// completely contain the query shape.
type ContainsPrefixTreeQuery struct {
	*PrefixTreeQuery
}

// NewContainsPrefixTreeQuery creates a new "contains" query.
//
// Parameters:
//   - fieldName: The field name containing the prefix tree grid
//   - queryShape: The shape that should be contained
//   - prefixTree: The spatial prefix tree for cell calculations
//   - detailLevel: The detail level for cell subdivision
func NewContainsPrefixTreeQuery(fieldName string, queryShape Shape, prefixTree SpatialPrefixTree, detailLevel int) *ContainsPrefixTreeQuery {
	return &ContainsPrefixTreeQuery{
		PrefixTreeQuery: NewPrefixTreeQuery(fieldName, queryShape, prefixTree, detailLevel, "Contains"),
	}
}

// Rewrite rewrites this query into a more primitive form.
// For "contains" queries, we look for indexed shapes whose cells
// completely surround the query shape.
func (q *ContainsPrefixTreeQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	// For contains, we need indexed shapes that fully contain the query shape
	// This requires the indexed shape's cells to cover all of the query shape's cells
	// plus potentially more. This is complex and requires post-filtering.
	cells, err := q.prefixTree.GetCellsForShape(q.queryShape, q.detailLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells for query shape: %w", err)
	}

	if len(cells) == 0 {
		return search.NewMatchNoDocsQuery(), nil
	}

	// Get parent cells that might contain the query shape
	// For a shape to contain the query shape, it must have cells at a
	// coarser level that cover the query cells
	seenTokens := make(map[string]bool)
	var tokens []string

	for _, cell := range cells {
		token := cell.GetToken()
		// For contains, we look at parent cells (coarser levels)
		// that might contain this shape
		for i := len(token); i > 0; i-- {
			parentToken := token[:i]
			if !seenTokens[parentToken] {
				tokens = append(tokens, parentToken)
				seenTokens[parentToken] = true
			}
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
func (q *ContainsPrefixTreeQuery) Clone() search.Query {
	return NewContainsPrefixTreeQuery(q.fieldName, q.queryShape, q.prefixTree, q.detailLevel)
}

// Equals checks if this query equals another.
func (q *ContainsPrefixTreeQuery) Equals(other search.Query) bool {
	o, ok := other.(*ContainsPrefixTreeQuery)
	if !ok {
		return false
	}
	return q.fieldName == o.fieldName &&
		q.detailLevel == o.detailLevel &&
		q.queryShape == o.queryShape
}

// HashCode returns a hash code for this query.
func (q *ContainsPrefixTreeQuery) HashCode() int {
	result := 23
	result = 31*result + hashCode(q.fieldName)
	result = 31*result + q.detailLevel
	return result
}

// CreateWeight creates a Weight for this query.
func (q *ContainsPrefixTreeQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	rewritten, err := q.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// String returns a string representation of this query.
func (q *ContainsPrefixTreeQuery) String() string {
	return fmt.Sprintf("ContainsPrefixTreeQuery(field=%s, shape=%v, level=%d)",
		q.fieldName, q.queryShape, q.detailLevel)
}

// Ensure ContainsPrefixTreeQuery implements Query
var _ search.Query = (*ContainsPrefixTreeQuery)(nil)

// DistanceQuery finds documents within a certain distance from a point.
type DistanceQuery struct {
	fieldName   string
	center      Point
	distance    float64
	prefixTree  SpatialPrefixTree
	detailLevel int
	calculator  DistanceCalculator
}

// NewDistanceQuery creates a new distance query.
//
// Parameters:
//   - fieldName: The field name containing the spatial data
//   - center: The center point for distance calculation
//   - distance: The maximum distance (in degrees)
//   - prefixTree: The spatial prefix tree
//   - detailLevel: The detail level for cell subdivision
//   - calculator: The distance calculator to use
func NewDistanceQuery(fieldName string, center Point, distance float64, prefixTree SpatialPrefixTree, detailLevel int, calculator DistanceCalculator) *DistanceQuery {
	return &DistanceQuery{
		fieldName:   fieldName,
		center:      center,
		distance:    distance,
		prefixTree:  prefixTree,
		detailLevel: detailLevel,
		calculator:  calculator,
	}
}

// Rewrite rewrites this query into a more primitive form.
// For distance queries, we create a circle/buffer around the center point
// and find cells that intersect with this buffer area.
func (q *DistanceQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	// Create a circle/buffer shape around the center point
	// The buffer extends q.distance in all directions
	minLon := q.center.X - q.distance
	maxLon := q.center.X + q.distance
	minLat := q.center.Y - q.distance
	maxLat := q.center.Y + q.distance

	// Normalize the bounding box
	if minLon < -180 {
		minLon = -180
	}
	if maxLon > 180 {
		maxLon = 180
	}
	if minLat < -90 {
		minLat = -90
	}
	if maxLat > 90 {
		maxLat = 90
	}

	// Create a rectangle representing the search area
	// (This is a simplification - a true circle would be more accurate)
	searchArea := NewRectangle(minLon, minLat, maxLon, maxLat)

	// Get cells that intersect with the search area
	cells, err := q.prefixTree.GetCellsForShape(searchArea, q.detailLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells for distance query: %w", err)
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
func (q *DistanceQuery) Clone() search.Query {
	return NewDistanceQuery(q.fieldName, q.center, q.distance, q.prefixTree, q.detailLevel, q.calculator)
}

// Equals checks if this query equals another.
func (q *DistanceQuery) Equals(other search.Query) bool {
	o, ok := other.(*DistanceQuery)
	if !ok {
		return false
	}
	return q.fieldName == o.fieldName &&
		q.center == o.center &&
		q.distance == o.distance
}

// HashCode returns a hash code for this query.
func (q *DistanceQuery) HashCode() int {
	result := 29
	result = 31*result + hashCode(q.fieldName)
	result = 31*result + int(q.distance*1000)
	return result
}

// CreateWeight creates a Weight for this query.
func (q *DistanceQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	rewritten, err := q.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// GetCenter returns the center point for this distance query.
func (q *DistanceQuery) GetCenter() Point {
	return q.center
}

// GetDistance returns the distance threshold for this query.
func (q *DistanceQuery) GetDistance() float64 {
	return q.distance
}

// GetFieldName returns the field name for this query.
func (q *DistanceQuery) GetFieldName() string {
	return q.fieldName
}

// String returns a string representation of this query.
func (q *DistanceQuery) String() string {
	return fmt.Sprintf("DistanceQuery(field=%s, center=%v, distance=%f)",
		q.fieldName, q.center, q.distance)
}

// Ensure DistanceQuery implements Query
var _ search.Query = (*DistanceQuery)(nil)

// hashCode is a helper function to generate a simple hash code for a string.
func hashCode(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = 31*h + int(s[i])
	}
	return h
}
