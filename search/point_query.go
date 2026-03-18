// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// PointQuery is the base class for point-based queries.
// It provides common functionality for queries that operate on point fields
// indexed using the BKD tree structure.
//
// This is the Go port of Lucene's org.apache.lucene.search.PointQuery.
type PointQuery struct {
	field       string
	numDims     int
	bytesPerDim int
}

// NewPointQuery creates a new PointQuery.
func NewPointQuery(field string, numDims, bytesPerDim int) *PointQuery {
	return &PointQuery{
		field:       field,
		numDims:     numDims,
		bytesPerDim: bytesPerDim,
	}
}

// Field returns the field name.
func (q *PointQuery) Field() string {
	return q.field
}

// NumDims returns the number of dimensions.
func (q *PointQuery) NumDims() int {
	return q.numDims
}

// BytesPerDim returns the number of bytes per dimension.
func (q *PointQuery) BytesPerDim() int {
	return q.bytesPerDim
}

// Rewrite rewrites this query to a more primitive form.
func (q *PointQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *PointQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	// TODO: Implement when PointValues API is complete
	return nil, fmt.Errorf("PointQuery weight not yet implemented")
}

// String returns a string representation of the query.
func (q *PointQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("PointQuery(field=%s)", q.field)
	}
	return "PointQuery"
}

// Clone creates a copy of this query.
func (q *PointQuery) Clone() Query {
	return &PointQuery{
		field:       q.field,
		numDims:     q.numDims,
		bytesPerDim: q.bytesPerDim,
	}
}

// Equals checks if this query equals another.
func (q *PointQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*PointQuery); ok {
		return q.field == o.field && q.numDims == o.numDims && q.bytesPerDim == o.bytesPerDim
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *PointQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.numDims
	h = 31*h + q.bytesPerDim
	return h
}

// Ensure PointQuery implements Query
var _ Query = (*PointQuery)(nil)
