// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// SpanMultiTermQueryWrapper wraps a multi-term query for span matching.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanMultiTermQueryWrapper.
type SpanMultiTermQueryWrapper struct {
	BaseSpanQuery
	query *MultiTermQuery
}

// NewSpanMultiTermQueryWrapper creates a new SpanMultiTermQueryWrapper.
func NewSpanMultiTermQueryWrapper(query *MultiTermQuery) *SpanMultiTermQueryWrapper {
	return &SpanMultiTermQueryWrapper{
		BaseSpanQuery: *NewBaseSpanQuery(query.GetField()),
		query:         query,
	}
}

// Query returns the wrapped query.
func (q *SpanMultiTermQueryWrapper) Query() *MultiTermQuery {
	return q.query
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanMultiTermQueryWrapper) Rewrite(reader IndexReader) (Query, error) {
	// TODO: Rewrite to SpanOrQuery of SpanTermQueries
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanMultiTermQueryWrapper) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanMultiTermQueryWrapper) Clone() Query {
	clonedQuery := q.query.Clone().(*MultiTermQuery)
	return &SpanMultiTermQueryWrapper{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		query:         clonedQuery,
	}
}

// Equals checks if this query equals another.
func (q *SpanMultiTermQueryWrapper) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanMultiTermQueryWrapper); ok {
		return q.field == o.field && q.query.Equals(o.query)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanMultiTermQueryWrapper) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.query.HashCode()
	return h
}

// String returns a string representation of the query.
func (q *SpanMultiTermQueryWrapper) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("SpanMultiTermQueryWrapper(field=%s, query=%s)",
			q.field, q.query.String(q.field))
	}
	return fmt.Sprintf("SpanMultiTermQueryWrapper(query=%s)", q.query.String(q.field))
}

// Ensure SpanMultiTermQueryWrapper implements SpanQuery
var _ SpanQuery = (*SpanMultiTermQueryWrapper)(nil)
