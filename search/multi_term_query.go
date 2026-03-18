// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// MultiTermQuery is the base class for queries that expand to multiple terms.
// This is the Go port of Lucene's org.apache.lucene.search.MultiTermQuery.
type MultiTermQuery struct {
	BaseQuery
	field string
	term  *index.Term
}

// NewMultiTermQuery creates a new MultiTermQuery.
func NewMultiTermQuery(field string, term *index.Term) *MultiTermQuery {
	return &MultiTermQuery{
		field: field,
		term:  term,
	}
}

// GetField returns the field for this query.
func (q *MultiTermQuery) GetField() string {
	return q.field
}

// GetTerm returns the term for this query.
func (q *MultiTermQuery) GetTerm() *index.Term {
	return q.term
}

// Rewrite rewrites this query to a simpler form.
func (q *MultiTermQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *MultiTermQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *MultiTermQuery) Clone() Query {
	return &MultiTermQuery{
		field: q.field,
		term:  index.NewTerm(q.term.Field, q.term.Text()),
	}
}

// Equals checks if this query equals another.
func (q *MultiTermQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*MultiTermQuery); ok {
		return q.field == o.field && q.term.Equals(o.term)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *MultiTermQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.term.HashCode()
	return h
}

// String returns a string representation of the query.
func (q *MultiTermQuery) String(field string) string {
	return "MultiTermQuery(field=" + q.field + ", term=" + q.term.Text() + ")"
}

// Ensure MultiTermQuery implements Query
var _ Query = (*MultiTermQuery)(nil)
