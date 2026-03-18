// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// SpanContainingQuery matches spans that contain another span.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanContainingQuery.
type SpanContainingQuery struct {
	BaseSpanQuery
	big   SpanQuery
	small SpanQuery
}

// NewSpanContainingQuery creates a new SpanContainingQuery.
// big: the containing span query
// small: the contained span query
func NewSpanContainingQuery(big, small SpanQuery) *SpanContainingQuery {
	if big.GetField() != small.GetField() {
		return nil
	}

	return &SpanContainingQuery{
		BaseSpanQuery: *NewBaseSpanQuery(big.GetField()),
		big:           big,
		small:         small,
	}
}

// Big returns the big (containing) query.
func (q *SpanContainingQuery) Big() SpanQuery {
	return q.big
}

// Small returns the small (contained) query.
func (q *SpanContainingQuery) Small() SpanQuery {
	return q.small
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanContainingQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanContainingQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanContainingQuery) Clone() Query {
	return &SpanContainingQuery{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		big:           q.big.Clone().(SpanQuery),
		small:         q.small.Clone().(SpanQuery),
	}
}

// Equals checks if this query equals another.
func (q *SpanContainingQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanContainingQuery); ok {
		return q.field == o.field &&
			q.big.Equals(o.big) &&
			q.small.Equals(o.small)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanContainingQuery) HashCode() int {
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
func (q *SpanContainingQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("SpanContainingQuery(field=%s, big=%s, small=%s)",
			q.field, q.big.String(q.field), q.small.String(q.field))
	}
	return fmt.Sprintf("SpanContainingQuery(big=%s, small=%s)",
		q.big.String(q.field), q.small.String(q.field))
}

// Ensure SpanContainingQuery implements SpanQuery
var _ SpanQuery = (*SpanContainingQuery)(nil)
