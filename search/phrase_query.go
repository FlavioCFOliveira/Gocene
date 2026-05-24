// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PhraseQuery matches documents containing a particular sequence of terms.
//
// positions[i] is the query position of terms[i] relative to the phrase
// start. For a plain consecutive phrase "A B C" the positions are [0,1,2].
// Gaps (position increments > 1) are used to represent missing words in the
// query ("drug _ _ drug" → positions [0,3]).
type PhraseQuery struct {
	*BaseQuery
	field     string
	terms     []*index.Term
	positions []int // query-position of each term; nil means 0,1,2,…
	slop      int
}

// NewPhraseQuery creates a new PhraseQuery with consecutive positions 0,1,2,…
func NewPhraseQuery(field string, terms ...*index.Term) *PhraseQuery {
	return &PhraseQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		terms:     terms,
		positions: nil,
		slop:      0,
	}
}

// NewPhraseQueryWithSlop creates a new PhraseQuery with a custom slop and
// consecutive positions.
func NewPhraseQueryWithSlop(slop int, field string, terms ...*index.Term) *PhraseQuery {
	return &PhraseQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		terms:     terms,
		positions: nil,
		slop:      slop,
	}
}

// newPhraseQueryWithPositions is the package-private constructor that carries
// explicit per-term query positions (used by PhraseQueryBuilder).
func newPhraseQueryWithPositions(slop int, field string, terms []*index.Term, positions []int) *PhraseQuery {
	return &PhraseQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		terms:     terms,
		positions: positions,
		slop:      slop,
	}
}

// Positions returns the per-term query positions.
// If no explicit positions were set the returned slice is [0, 1, 2, …].
func (q *PhraseQuery) Positions() []int {
	if q.positions != nil {
		return q.positions
	}
	pos := make([]int, len(q.terms))
	for i := range pos {
		pos[i] = i
	}
	return pos
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
	var clonedPositions []int
	if q.positions != nil {
		clonedPositions = make([]int, len(q.positions))
		copy(clonedPositions, q.positions)
	}
	return &PhraseQuery{
		BaseQuery: &BaseQuery{},
		field:     q.field,
		terms:     clonedTerms,
		positions: clonedPositions,
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
// Mirrors PhraseQuery.rewrite() from Lucene 10.4.0.
func (q *PhraseQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.terms) == 0 {
		return NewMatchNoDocsQueryWithReason("empty PhraseQuery"), nil
	}
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

// CreateWeight creates a Weight for this query.
func (q *PhraseQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewPhraseWeight(q, searcher, needsScores)
}

// Ensure PhraseQuery implements Query
var _ Query = (*PhraseQuery)(nil)

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
// Explicit positions are preserved; if all positions equal [0,1,2,…] (i.e. no
// holes) the positions slice is set to nil so callers get the cheap path.
func (b *PhraseQueryBuilder) Build() *PhraseQuery {
	// Check whether the positions are the trivial consecutive sequence.
	consecutive := true
	for i, p := range b.positions {
		if p != i {
			consecutive = false
			break
		}
	}
	if consecutive {
		return NewPhraseQueryWithSlop(b.slop, b.field, b.terms...)
	}
	posCopy := make([]int, len(b.positions))
	copy(posCopy, b.positions)
	termsCopy := make([]*index.Term, len(b.terms))
	copy(termsCopy, b.terms)
	return newPhraseQueryWithPositions(b.slop, b.field, termsCopy, posCopy)
}
