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
	field string
	terms []*index.Term
	slop  int
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

// Clone creates a copy of this query.
func (q *PhraseQuery) Clone() Query {
	clonedTerms := make([]*index.Term, len(q.terms))
	for i, term := range q.terms {
		clonedTerms[i] = term.Clone()
	}
	return &PhraseQuery{
		BaseQuery: &BaseQuery{},
		field:     q.field,
		terms:     clonedTerms,
		slop:      q.slop,
	}
}

// Equals checks if this query equals another.
func (q *PhraseQuery) Equals(other Query) bool {
	if o, ok := other.(*PhraseQuery); ok {
		if q.field != o.field || q.slop != o.slop || len(q.terms) != len(o.terms) {
			return false
		}
		for i, term := range q.terms {
			if !term.Equals(o.terms[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *PhraseQuery) HashCode() int {
	hash := 0
	for _, term := range q.terms {
		hash = hash*31 + term.HashCode()
	}
	hash = hash*31 + q.slop
	return hash
}
