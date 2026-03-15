// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// MultiPhraseQuery is a generalized version of PhraseQuery that allows multiple terms
// at the same position that are treated as a disjunction (OR).
//
// For example, to search for "Microsoft app*" you would add "microsoft" as a single term,
// then find all terms with "app" prefix and add them as a group.
//
// This is the Go port of Lucene's org.apache.lucene.search.MultiPhraseQuery.
type MultiPhraseQuery struct {
	*BaseQuery
	field      string
	termArrays [][]*index.Term
	positions  []int
	slop       int
}

// MultiPhraseQueryBuilder builds MultiPhraseQuery instances.
type MultiPhraseQueryBuilder struct {
	field      string
	termArrays [][]*index.Term
	positions  []int
	slop       int
}

// NewMultiPhraseQueryBuilder creates a new builder for MultiPhraseQuery.
func NewMultiPhraseQueryBuilder() *MultiPhraseQueryBuilder {
	return &MultiPhraseQueryBuilder{
		field:      "",
		termArrays: make([][]*index.Term, 0),
		positions:  make([]int, 0),
		slop:       0,
	}
}

// NewMultiPhraseQueryBuilderFromQuery creates a builder from an existing MultiPhraseQuery.
func NewMultiPhraseQueryBuilderFromQuery(query *MultiPhraseQuery) *MultiPhraseQueryBuilder {
	builder := &MultiPhraseQueryBuilder{
		field:      query.field,
		termArrays: make([][]*index.Term, len(query.termArrays)),
		positions:  make([]int, len(query.positions)),
		slop:       query.slop,
	}
	for i, terms := range query.termArrays {
		builder.termArrays[i] = make([]*index.Term, len(terms))
		for j, term := range terms {
			builder.termArrays[i][j] = term.Clone()
		}
	}
	copy(builder.positions, query.positions)
	return builder
}

// SetSlop sets the phrase slop for this query.
func (b *MultiPhraseQueryBuilder) SetSlop(slop int) *MultiPhraseQueryBuilder {
	b.slop = slop
	return b
}

// Add adds a single term at the next position in the phrase.
func (b *MultiPhraseQueryBuilder) Add(term *index.Term) *MultiPhraseQueryBuilder {
	return b.AddTerms([]*index.Term{term})
}

// AddTerms adds multiple terms at the next position in the phrase.
// Any of the terms may match (a disjunction/OR).
func (b *MultiPhraseQueryBuilder) AddTerms(terms []*index.Term) *MultiPhraseQueryBuilder {
	position := 0
	if len(b.positions) > 0 {
		position = b.positions[len(b.positions)-1] + 1
	}
	return b.AddTermsAtPosition(terms, position)
}

// AddTermsAtPosition adds multiple terms at a specific position in the phrase.
// This allows specifying custom relative positions.
func (b *MultiPhraseQueryBuilder) AddTermsAtPosition(terms []*index.Term, position int) *MultiPhraseQueryBuilder {
	if len(terms) == 0 {
		return b
	}

	if len(b.termArrays) == 0 {
		b.field = terms[0].Field
	}

	// Validate all terms are in the same field
	for _, term := range terms {
		if term.Field != b.field {
			panic(fmt.Sprintf("All phrase terms must be in the same field (%s): %v", b.field, term))
		}
	}

	termsCopy := make([]*index.Term, len(terms))
	for i, term := range terms {
		termsCopy[i] = term.Clone()
	}

	b.termArrays = append(b.termArrays, termsCopy)
	b.positions = append(b.positions, position)
	return b
}

// Build creates a MultiPhraseQuery from this builder.
func (b *MultiPhraseQueryBuilder) Build() *MultiPhraseQuery {
	return &MultiPhraseQuery{
		BaseQuery:  &BaseQuery{},
		field:      b.field,
		termArrays: b.termArrays,
		positions:  b.positions,
		slop:       b.slop,
	}
}

// NewMultiPhraseQuery creates a new MultiPhraseQuery with the given field, term arrays, positions, and slop.
func NewMultiPhraseQuery(field string, termArrays [][]*index.Term, positions []int, slop int) *MultiPhraseQuery {
	// Copy term arrays
	termArraysCopy := make([][]*index.Term, len(termArrays))
	for i, terms := range termArrays {
		termArraysCopy[i] = make([]*index.Term, len(terms))
		for j, term := range terms {
			termArraysCopy[i][j] = term.Clone()
		}
	}

	positionsCopy := make([]int, len(positions))
	copy(positionsCopy, positions)

	return &MultiPhraseQuery{
		BaseQuery:  &BaseQuery{},
		field:      field,
		termArrays: termArraysCopy,
		positions:  positionsCopy,
		slop:       slop,
	}
}

// GetSlop returns the phrase slop.
func (q *MultiPhraseQuery) GetSlop() int {
	return q.slop
}

// SetSlop sets the phrase slop.
func (q *MultiPhraseQuery) SetSlop(slop int) {
	q.slop = slop
}

// GetTermArrays returns the arrays of terms in the multi-phrase.
// Each inner array represents terms at a single position (OR'd together).
func (q *MultiPhraseQuery) GetTermArrays() [][]*index.Term {
	result := make([][]*index.Term, len(q.termArrays))
	for i, terms := range q.termArrays {
		result[i] = make([]*index.Term, len(terms))
		for j, term := range terms {
			result[i][j] = term.Clone()
		}
	}
	return result
}

// GetPositions returns the relative positions of terms in this phrase.
func (q *MultiPhraseQuery) GetPositions() []int {
	result := make([]int, len(q.positions))
	copy(result, q.positions)
	return result
}

// Field returns the field name.
func (q *MultiPhraseQuery) Field() string {
	return q.field
}

// Clone creates a copy of this query.
func (q *MultiPhraseQuery) Clone() Query {
	return NewMultiPhraseQuery(q.field, q.termArrays, q.positions, q.slop)
}

// Equals checks if this query equals another.
func (q *MultiPhraseQuery) Equals(other Query) bool {
	if o, ok := other.(*MultiPhraseQuery); ok {
		if q.field != o.field || q.slop != o.slop {
			return false
		}
		if len(q.termArrays) != len(o.termArrays) || len(q.positions) != len(o.positions) {
			return false
		}
		// Compare positions
		for i, pos := range q.positions {
			if pos != o.positions[i] {
				return false
			}
		}
		// Compare term arrays
		for i, terms := range q.termArrays {
			if len(terms) != len(o.termArrays[i]) {
				return false
			}
			for j, term := range terms {
				if !term.Equals(o.termArrays[i][j]) {
					return false
				}
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *MultiPhraseQuery) HashCode() int {
	hash := 0
	// Hash term arrays
	for _, terms := range q.termArrays {
		for _, term := range terms {
			hash = 31*hash + term.HashCode()
		}
	}
	hash = hash*31 + q.slop
	// Hash positions
	for _, pos := range q.positions {
		hash = 31*hash + pos
	}
	return hash
}

// Rewrite rewrites the query to a simpler form.
func (q *MultiPhraseQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.termArrays) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	if len(q.termArrays) == 1 {
		// Optimize one-term case to a BooleanQuery with OR
		terms := q.termArrays[0]
		if len(terms) == 1 {
			return NewTermQuery(terms[0]), nil
		}
		bq := NewBooleanQuery()
		for _, term := range terms {
			bq.Add(NewTermQuery(term), SHOULD)
		}
		return bq, nil
	}
	return q, nil
}

// String returns a string representation of this query.
func (q *MultiPhraseQuery) String() string {
	var buffer strings.Builder
	if q.field != "" {
		buffer.WriteString(q.field)
		buffer.WriteString(":")
	}

	buffer.WriteString("\"")
	lastPos := -1

	for i, terms := range q.termArrays {
		position := q.positions[i]
		if i > 0 {
			buffer.WriteString(" ")
			for j := 1; j < (position - lastPos); j++ {
				buffer.WriteString("? ")
			}
		}
		if len(terms) > 1 {
			buffer.WriteString("(")
			for j, term := range terms {
				if j > 0 {
					buffer.WriteString(" ")
				}
				buffer.WriteString(term.Text())
			}
			buffer.WriteString(")")
		} else if len(terms) == 1 {
			buffer.WriteString(terms[0].Text())
		}
		lastPos = position
	}
	buffer.WriteString("\"")

	if q.slop != 0 {
		buffer.WriteString(fmt.Sprintf("~%d", q.slop))
	}

	return buffer.String()
}
