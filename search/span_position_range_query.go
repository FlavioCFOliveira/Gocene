// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// SpanPositionRangeQuery matches spans within a specific position range.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanPositionRangeQuery.
type SpanPositionRangeQuery struct {
	BaseSpanQuery
	match SpanQuery
	start int
	end   int
}

// NewSpanPositionRangeQuery creates a new SpanPositionRangeQuery.
// match: the query to match
// start: the start position (inclusive)
// end: the end position (exclusive)
func NewSpanPositionRangeQuery(match SpanQuery, start, end int) *SpanPositionRangeQuery {
	return &SpanPositionRangeQuery{
		BaseSpanQuery: *NewBaseSpanQuery(match.GetField()),
		match:         match,
		start:         start,
		end:           end,
	}
}

// Match returns the match query.
func (q *SpanPositionRangeQuery) Match() SpanQuery {
	return q.match
}

// Start returns the start position.
func (q *SpanPositionRangeQuery) Start() int {
	return q.start
}

// End returns the end position.
func (q *SpanPositionRangeQuery) End() int {
	return q.end
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanPositionRangeQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanPositionRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanPositionRangeQuery) Clone() Query {
	return &SpanPositionRangeQuery{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		match:         q.match.Clone().(SpanQuery),
		start:         q.start,
		end:           q.end,
	}
}

// Equals checks if this query equals another.
func (q *SpanPositionRangeQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanPositionRangeQuery); ok {
		return q.field == o.field &&
			q.start == o.start &&
			q.end == o.end &&
			q.match.Equals(o.match)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanPositionRangeQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.match.HashCode()
	h = 31*h + q.start
	h = 31*h + q.end
	return h
}

// String returns a string representation of the query.
func (q *SpanPositionRangeQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("SpanPositionRangeQuery(field=%s, match=%s, start=%d, end=%d)",
			q.field, q.match.String(q.field), q.start, q.end)
	}
	return fmt.Sprintf("SpanPositionRangeQuery(match=%s, start=%d, end=%d)",
		q.match.String(q.field), q.start, q.end)
}

// Ensure SpanPositionRangeQuery implements SpanQuery
var _ SpanQuery = (*SpanPositionRangeQuery)(nil)
