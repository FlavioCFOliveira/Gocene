// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// SpanFirstQuery matches spans that start at the first position (position 0).
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanFirstQuery.
type SpanFirstQuery struct {
	BaseSpanQuery
	match SpanQuery
	end   int
}

// NewSpanFirstQuery creates a new SpanFirstQuery.
// match: the query to match
// end: the maximum end position (exclusive)
func NewSpanFirstQuery(match SpanQuery, end int) *SpanFirstQuery {
	return &SpanFirstQuery{
		BaseSpanQuery: *NewBaseSpanQuery(match.GetField()),
		match:         match,
		end:           end,
	}
}

// Match returns the match query.
func (q *SpanFirstQuery) Match() SpanQuery {
	return q.match
}

// End returns the end position.
func (q *SpanFirstQuery) End() int {
	return q.end
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanFirstQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanFirstQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanFirstQuery) Clone() Query {
	return &SpanFirstQuery{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		match:         q.match.Clone().(SpanQuery),
		end:           q.end,
	}
}

// Equals checks if this query equals another.
func (q *SpanFirstQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanFirstQuery); ok {
		return q.field == o.field && q.end == o.end && q.match.Equals(o.match)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanFirstQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.match.HashCode()
	h = 31*h + q.end
	return h
}

// String returns a string representation of the query.
func (q *SpanFirstQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("SpanFirstQuery(field=%s, match=%s, end=%d)",
			q.field, q.match.String(q.field), q.end)
	}
	return fmt.Sprintf("SpanFirstQuery(match=%s, end=%d)",
		q.match.String(q.field), q.end)
}

// Ensure SpanFirstQuery implements SpanQuery
var _ SpanQuery = (*SpanFirstQuery)(nil)
