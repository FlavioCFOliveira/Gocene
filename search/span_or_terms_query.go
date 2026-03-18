// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanOrTermsQuery matches documents containing any of the specified terms as spans.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanOrTermsQuery.
type SpanOrTermsQuery struct {
	BaseSpanQuery
	terms []*index.Term
}

// NewSpanOrTermsQuery creates a new SpanOrTermsQuery.
func NewSpanOrTermsQuery(terms ...*index.Term) *SpanOrTermsQuery {
	if len(terms) == 0 {
		return nil
	}

	// All terms must have the same field
	field := terms[0].Field
	for _, term := range terms {
		if term.Field != field {
			return nil
		}
	}

	return &SpanOrTermsQuery{
		BaseSpanQuery: *NewBaseSpanQuery(field),
		terms:         terms,
	}
}

// Terms returns the terms.
func (q *SpanOrTermsQuery) Terms() []*index.Term {
	return q.terms
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanOrTermsQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.terms) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	if len(q.terms) == 1 {
		return NewSpanTermQuery(q.terms[0]), nil
	}

	// Rewrite to SpanOrQuery of SpanTermQueries
	clauses := make([]SpanQuery, len(q.terms))
	for i, term := range q.terms {
		clauses[i] = NewSpanTermQuery(term)
	}
	return NewSpanOrQuery(clauses...), nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanOrTermsQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanOrTermsQuery) Clone() Query {
	termsCopy := make([]*index.Term, len(q.terms))
	for i, term := range q.terms {
		termsCopy[i] = index.NewTerm(term.Field, term.Text())
	}
	return &SpanOrTermsQuery{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		terms:         termsCopy,
	}
}

// Equals checks if this query equals another.
func (q *SpanOrTermsQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanOrTermsQuery); ok {
		if q.field != o.field || len(q.terms) != len(o.terms) {
			return false
		}
		for i := range q.terms {
			if !q.terms[i].Equals(o.terms[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanOrTermsQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	for _, term := range q.terms {
		h = 31*h + term.HashCode()
	}
	return h
}

// String returns a string representation of the query.
func (q *SpanOrTermsQuery) String(field string) string {
	var termStrs []string
	for _, term := range q.terms {
		termStrs = append(termStrs, term.Text())
	}

	if field == "" || field != q.field {
		return fmt.Sprintf("SpanOrTermsQuery(field=%s, terms=%v)", q.field, termStrs)
	}
	return fmt.Sprintf("SpanOrTermsQuery(terms=%v)", termStrs)
}

// Ensure SpanOrTermsQuery implements SpanQuery
var _ SpanQuery = (*SpanOrTermsQuery)(nil)
