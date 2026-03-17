// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// RangeFieldQuery is a query that matches documents with range field values
// that intersect with the query range.
//
// This is the Go port of Lucene's org.apache.lucene.search.RangeFieldQuery.
type RangeFieldQuery struct {
	field          string
	queryMin       []byte
	queryMax       []byte
	queryType      RangeFieldQueryType
}

// RangeFieldQueryType defines the type of range query.
type RangeFieldQueryType int

const (
	// RangeFieldQueryTypeIntersects matches ranges that intersect
	RangeFieldQueryTypeIntersects RangeFieldQueryType = iota
	// RangeFieldQueryTypeContains matches ranges that contain the query range
	RangeFieldQueryTypeContains
	// RangeFieldQueryTypeWithin matches ranges that are within the query range
	RangeFieldQueryTypeWithin
)

// NewRangeFieldQuery creates a new RangeFieldQuery.
func NewRangeFieldQuery(field string, queryMin, queryMax []byte, queryType RangeFieldQueryType) *RangeFieldQuery {
	return &RangeFieldQuery{
		field:     field,
		queryMin:  queryMin,
		queryMax:  queryMax,
		queryType: queryType,
	}
}

// Field returns the field name.
func (q *RangeFieldQuery) Field() string {
	return q.field
}

// QueryMin returns the query minimum value.
func (q *RangeFieldQuery) QueryMin() []byte {
	return q.queryMin
}

// QueryMax returns the query maximum value.
func (q *RangeFieldQuery) QueryMax() []byte {
	return q.queryMax
}

// QueryType returns the query type.
func (q *RangeFieldQuery) QueryType() RangeFieldQueryType {
	return q.queryType
}

// Rewrite rewrites this query to a more primitive form.
func (q *RangeFieldQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *RangeFieldQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	// TODO: Implement when PointValues API is complete
	return nil, fmt.Errorf("RangeFieldQuery weight not yet implemented")
}

// Clone creates a copy of this query.
func (q *RangeFieldQuery) Clone() Query {
	minCopy := make([]byte, len(q.queryMin))
	copy(minCopy, q.queryMin)
	maxCopy := make([]byte, len(q.queryMax))
	copy(maxCopy, q.queryMax)

	return &RangeFieldQuery{
		field:     q.field,
		queryMin:  minCopy,
		queryMax:  maxCopy,
		queryType: q.queryType,
	}
}

// Equals checks if this query equals another.
func (q *RangeFieldQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*RangeFieldQuery); ok {
		if q.field != o.field || q.queryType != o.queryType {
			return false
		}
		if len(q.queryMin) != len(o.queryMin) || len(q.queryMax) != len(o.queryMax) {
			return false
		}
		for i := range q.queryMin {
			if q.queryMin[i] != o.queryMin[i] {
				return false
			}
		}
		for i := range q.queryMax {
			if q.queryMax[i] != o.queryMax[i] {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *RangeFieldQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	for _, b := range q.queryMin {
		h = 31*h + int(b)
	}
	for _, b := range q.queryMax {
		h = 31*h + int(b)
	}
	h = 31*h + int(q.queryType)
	return h
}

// String returns a string representation of the query.
func (q *RangeFieldQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("RangeFieldQuery(field=%s, type=%v)", q.field, q.queryType)
	}
	return fmt.Sprintf("RangeFieldQuery(type=%v)", q.queryType)
}

// Ensure RangeFieldQuery implements Query
var _ Query = (*RangeFieldQuery)(nil)
