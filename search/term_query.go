// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermQuery matches documents containing a specific term.
type TermQuery struct {
	*BaseQuery
	term *index.Term
}

// NewTermQuery creates a new TermQuery.
func NewTermQuery(term *index.Term) *TermQuery {
	return &TermQuery{
		BaseQuery: &BaseQuery{},
		term:      term,
	}
}

// Term returns the term being searched.
func (q *TermQuery) Term() *index.Term {
	return q.term
}

func (q *TermQuery) Clone() Query {
	return NewTermQuery(q.term.Clone())
}

func (q *TermQuery) Equals(other Query) bool {
	if o, ok := other.(*TermQuery); ok {
		return q.term.Equals(o.term)
	}
	return false
}

func (q *TermQuery) HashCode() int {
	return q.term.HashCode()
}

// Rewrite rewrites the query to a simpler form.
func (q *TermQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *TermQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewTermWeight(q, q.term, searcher, needsScores), nil
}

func (q *TermQuery) String() string {
	return q.term.String()
}
