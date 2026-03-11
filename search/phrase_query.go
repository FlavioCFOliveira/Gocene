// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// PhraseQuery matches documents containing a particular sequence of terms.
type PhraseQuery struct {
	*BaseQuery
	field     string
	terms     []*index.Term
	slop      int
}

// NewPhraseQuery creates a new PhraseQuery.
func NewPhraseQuery(field string, terms ...*index.Term) *PhraseQuery {
	return &PhraseQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		terms:     terms,
		slop:      0,
	}
}

// Field returns the field name.
func (q *PhraseQuery) Field() string {
	return q.field
}

// Terms returns the terms in this query.
func (q *PhraseQuery) Terms() []*index.Term {
	return q.terms
}

// GetSlop returns the slop (maximum distance between terms).
func (q *PhraseQuery) GetSlop() int {
	return q.slop
}

// SetSlop sets the slop.
func (q *PhraseQuery) SetSlop(slop int) {
	q.slop = slop
}

// AddTerm adds a term to this query.
func (q *PhraseQuery) AddTerm(term *index.Term) {
	q.terms = append(q.terms, term)
}
