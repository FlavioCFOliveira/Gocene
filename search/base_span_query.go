// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SpanQuery is the interface for span queries.
// Span queries are used for positional/proximity-based search.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanQuery.
type SpanQuery interface {
	// Query methods
	CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error)
	Rewrite(reader IndexReader) (Query, error)
	Clone() Query
	Equals(other Query) bool
	HashCode() int
	String(field string) string
	// Span-specific method
	GetField() string
}

// BaseSpanQuery provides common functionality for span queries.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanQuery base functionality.
type BaseSpanQuery struct {
	field string
}

// NewBaseSpanQuery creates a new BaseSpanQuery.
func NewBaseSpanQuery(field string) *BaseSpanQuery {
	return &BaseSpanQuery{field: field}
}

// GetField returns the field for this query.
func (q *BaseSpanQuery) GetField() string {
	return q.field
}

// Rewrite rewrites the query to a simpler form.
func (q *BaseSpanQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// Clone creates a copy of this query.
func (q *BaseSpanQuery) Clone() Query {
	return &BaseSpanQuery{field: q.field}
}

// Equals checks if this query equals another.
func (q *BaseSpanQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*BaseSpanQuery); ok {
		return q.field == o.field
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *BaseSpanQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	return h
}

// CreateWeight creates a Weight for this query.
func (q *BaseSpanQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return nil, nil
}

// String returns a string representation of the query.
func (q *BaseSpanQuery) String(field string) string {
	return "BaseSpanQuery(field=" + q.field + ")"
}
