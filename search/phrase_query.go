// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

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

// NewPhraseQueryWithSlop creates a new PhraseQuery with a custom slop.
func NewPhraseQueryWithSlop(slop int, field string, terms ...*index.Term) *PhraseQuery {
	return &PhraseQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		terms:     terms,
		slop:      slop,
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

// Rewrite rewrites the query to a simpler form.
func (q *PhraseQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.terms) == 1 {
		return NewTermQuery(q.terms[0]), nil
	}
	return q, nil
}

func (q *PhraseQuery) String() string {
	buffer := q.field + ":\""
	for i, term := range q.terms {
		if i > 0 {
			buffer += " "
		}
		buffer += term.Text()
	}
	buffer += "\""
	if q.slop > 0 {
		buffer += fmt.Sprintf("~%d", q.slop)
	}
	return buffer
}

// NewPhraseQueryWithStrings creates a PhraseQuery from a field and multiple strings.
func NewPhraseQueryWithStrings(field string, terms ...string) *PhraseQuery {
	termObjects := make([]*index.Term, len(terms))
	for i, t := range terms {
		termObjects[i] = index.NewTerm(field, t)
	}
	return NewPhraseQuery(field, termObjects...)
}

// PhraseQueryBuilder builds PhraseQuery instances with position support.
type PhraseQueryBuilder struct {
	field     string
	terms     []*index.Term
	positions []int
	slop      int
}

// NewPhraseQueryBuilder creates a new builder for PhraseQuery.
func NewPhraseQueryBuilder() *PhraseQueryBuilder {
	return &PhraseQueryBuilder{
		field:     "",
		terms:     make([]*index.Term, 0),
		positions: make([]int, 0),
		slop:      0,
	}
}

// SetSlop sets the phrase slop.
func (b *PhraseQueryBuilder) SetSlop(slop int) *PhraseQueryBuilder {
	b.slop = slop
	return b
}

// AddTerm adds a term at the next position.
func (b *PhraseQueryBuilder) AddTerm(term *index.Term) *PhraseQueryBuilder {
	if b.field == "" {
		b.field = term.Field
	}
	b.terms = append(b.terms, term.Clone())
	b.positions = append(b.positions, len(b.terms)-1)
	return b
}

// AddTermAtPosition adds a term at a specific position.
func (b *PhraseQueryBuilder) AddTermAtPosition(term *index.Term, position int) *PhraseQueryBuilder {
	if b.field == "" {
		b.field = term.Field
	}
	b.terms = append(b.terms, term.Clone())
	b.positions = append(b.positions, position)
	return b
}

// Build creates a PhraseQuery from this builder.
func (b *PhraseQueryBuilder) Build() *PhraseQuery {
	return NewPhraseQueryWithSlop(b.slop, b.field, b.terms...)
}
