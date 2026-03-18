// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// SpanWithinQuery matches spans that are within another span.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanWithinQuery.
type SpanWithinQuery struct {
	BaseSpanQuery
	big   SpanQuery
	small SpanQuery
}

// NewSpanWithinQuery creates a new SpanWithinQuery.
// big: the containing span query
// small: the contained span query
func NewSpanWithinQuery(big, small SpanQuery) *SpanWithinQuery {
	if big.GetField() != small.GetField() {
		return nil
	}

	return &SpanWithinQuery{
		BaseSpanQuery: *NewBaseSpanQuery(big.GetField()),
		big:           big,
		small:         small,
	}
}

// Big returns the big (containing) query.
func (q *SpanWithinQuery) Big() SpanQuery {
	return q.big
}

// Small returns the small (contained) query.
func (q *SpanWithinQuery) Small() SpanQuery {
	return q.small
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanWithinQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanWithinQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanWithinQuery) Clone() Query {
	return &SpanWithinQuery{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		big:           q.big.Clone().(SpanQuery),
		small:         q.small.Clone().(SpanQuery),
	}
}

// Equals checks if this query equals another.
func (q *SpanWithinQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanWithinQuery); ok {
		return q.field == o.field &&
			q.big.Equals(o.big) &&
			q.small.Equals(o.small)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanWithinQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.big.HashCode()
	h = 31*h + q.small.HashCode()
	return h
}

// String returns a string representation of the query.
func (q *SpanWithinQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("SpanWithinQuery(field=%s, big=%s, small=%s)",
			q.field, q.big.String(q.field), q.small.String(q.field))
	}
	return fmt.Sprintf("SpanWithinQuery(big=%s, small=%s)",
		q.big.String(q.field), q.small.String(q.field))
}

// Ensure SpanWithinQuery implements SpanQuery
var _ SpanQuery = (*SpanWithinQuery)(nil)
