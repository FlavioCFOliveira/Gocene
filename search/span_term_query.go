// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanTermQuery matches documents containing a specific term at specific positions.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanTermQuery.
type SpanTermQuery struct {
	BaseSpanQuery
	term *index.Term
}

// NewSpanTermQuery creates a new SpanTermQuery.
func NewSpanTermQuery(term *index.Term) *SpanTermQuery {
	return &SpanTermQuery{
		BaseSpanQuery: *NewBaseSpanQuery(term.Field),
		term:          term,
	}
}

// Term returns the term.
func (q *SpanTermQuery) Term() *index.Term {
	return q.term
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanTermQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanTermQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanTermQuery) Clone() Query {
	return NewSpanTermQuery(index.NewTerm(q.term.Field, q.term.Text()))
}

// Equals checks if this query equals another.
func (q *SpanTermQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanTermQuery); ok {
		return q.term.Equals(o.term)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanTermQuery) HashCode() int {
	return q.term.HashCode()
}

// String returns a string representation of the query.
func (q *SpanTermQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("SpanTermQuery(field=%s, term=%s)", q.field, q.term.Text())
	}
	return fmt.Sprintf("SpanTermQuery(term=%s)", q.term.Text())
}

// Ensure SpanTermQuery implements SpanQuery
var _ SpanQuery = (*SpanTermQuery)(nil)
