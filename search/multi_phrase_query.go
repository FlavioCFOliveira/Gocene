// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// MultiPhraseQuery is a generalized version of PhraseQuery, with the possibility of adding more than one term
// at the same position that are treated as a disjunction (OR).
//
// This is a faithful port of org.apache.lucene.search.MultiPhraseQuery from Apache Lucene 10.5.0.
type MultiPhraseQuery struct {
	*BaseQuery
	field      string
	termArrays [][]*index.Term
	positions  []int
	slop       int
}

// MultiPhraseQueryBuilder is a builder for multi-phrase queries.
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

// NewMultiPhraseQueryBuilderFromQuery creates a builder with the same configuration as the provided query.
func NewMultiPhraseQueryBuilderFromQuery(mq *MultiPhraseQuery) *MultiPhraseQueryBuilder {
	length := len(mq.termArrays)
	builder := &MultiPhraseQueryBuilder{
		field:      mq.field,
		termArrays: make([][]*index.Term, 0, length),
		positions:  make([]int, 0, length),
		slop:       mq.slop,
	}

	for i := 0; i < length; i++ {
		builder.termArrays = append(builder.termArrays, mq.termArrays[i])
		builder.positions = append(builder.positions, mq.positions[i])
	}

	return builder
}

// SetSlop sets the phrase slop for this query.
func (b *MultiPhraseQueryBuilder) SetSlop(s int) *MultiPhraseQueryBuilder {
	if s < 0 {
		panic("slop value cannot be negative")
	}
	b.slop = s
	return b
}

// Add adds a single term at the next position in the phrase.
func (b *MultiPhraseQueryBuilder) Add(term *index.Term) *MultiPhraseQueryBuilder {
	return b.AddTerms([]*index.Term{term})
}

// AddTerms adds multiple terms at the next position in the phrase. Any of the terms may match (a disjunction).
func (b *MultiPhraseQueryBuilder) AddTerms(terms []*index.Term) *MultiPhraseQueryBuilder {
	position := 0
	if len(b.positions) > 0 {
		position = b.positions[len(b.positions)-1] + 1
	}
	return b.AddTermsAtPosition(terms, position)
}

// AddTermsAtPosition allows specifying the relative position of terms within the phrase.
func (b *MultiPhraseQueryBuilder) AddTermsAtPosition(terms []*index.Term, position int) *MultiPhraseQueryBuilder {
	if terms == nil {
		panic("Term array must not be null")
	}
	if len(b.termArrays) == 0 {
		b.field = terms[0].Field
	}

	for _, term := range terms {
		if term.Field != b.field {
			panic(fmt.Sprintf("All phrase terms must be in the same field (%s): %v", b.field, term))
		}
	}

	b.termArrays = append(b.termArrays, terms)
	b.positions = append(b.positions, position)

	return b
}

// Build returns the fully constructed (and immutable) MultiPhraseQuery.
func (b *MultiPhraseQueryBuilder) Build() *MultiPhraseQuery {
	return &MultiPhraseQuery{
		BaseQuery:  &BaseQuery{},
		field:      b.field,
		termArrays: b.termArrays,
		positions:  b.positions,
		slop:       b.slop,
	}
}

// NewMultiPhraseQuery creates a new MultiPhraseQuery.
func NewMultiPhraseQuery(field string, termArrays [][]*index.Term, positions []int, slop int) *MultiPhraseQuery {
	return &MultiPhraseQuery{
		BaseQuery:  &BaseQuery{},
		field:      field,
		termArrays: termArrays,
		positions:  positions,
		slop:       slop,
	}
}

// GetSlop returns the phrase slop.
func (q *MultiPhraseQuery) GetSlop() int {
	return q.slop
}

// GetTermArrays returns the arrays of arrays of terms in the multi-phrase.
func (q *MultiPhraseQuery) GetTermArrays() [][]*index.Term {
	return q.termArrays
}

// GetPositions returns the relative positions of terms in this phrase.
func (q *MultiPhraseQuery) GetPositions() []int {
	return q.positions
}

// Rewrite rewrites the query to a simpler form.
func (q *MultiPhraseQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	if len(q.termArrays) == 0 {
		return NewMatchNoDocsQuery("empty MultiPhraseQuery"), nil
	} else if len(q.termArrays) == 1 { // optimize one-term case
		terms := q.termArrays[0]
		bq := NewBooleanQueryBuilder()
		for _, term := range terms {
			bq.Add(NewTermQuery(term), SHOULD)
		}
		return bq.Build(), nil
	}
	return q, nil
}

// Visit walks the query tree.
func (q *MultiPhraseQuery) Visit(visitor QueryVisitor) {
	if !visitor.AcceptField(q.field) {
		return
	}
	v := visitor.GetSubVisitor(MUST, q)
	for _, terms := range q.termArrays {
		sv := v.GetSubVisitor(SHOULD, q)
		sv.ConsumeTerms(q, terms...)
	}
}

// CreateWeight creates a Weight for this query.
func (q *MultiPhraseQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewMultiPhraseWeight(q, searcher, scoreMode, boost)
}

// String prints a user-readable version of this query.
func (q *MultiPhraseQuery) String() string {
	var buffer strings.Builder
	if q.field != "" {
		buffer.WriteString(q.field)
		buffer.WriteString(":")
	}

	buffer.WriteString("\"")
	lastPos := -1

	for i := 0; i < len(q.termArrays); i++ {
		terms := q.termArrays[i]
		position := q.positions[i]
		if i != 0 {
			buffer.WriteString(" ")
			for j := 1; j < (position - lastPos); j++ {
				buffer.WriteString("? ")
			}
		}
		if len(terms) > 1 {
			buffer.WriteString("(")
			for j := 0; j < len(terms); j++ {
				buffer.WriteString(terms[j].Text())
				if j < len(terms)-1 {
					buffer.WriteString(" ")
				}
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

// Equals returns true if the other query is equal to this.
func (q *MultiPhraseQuery) Equals(other spi.Query) bool {
	o, ok := other.(*MultiPhraseQuery)
	if !ok {
		return false
	}
	if q.slop != o.slop {
		return false
	}
	if len(q.termArrays) != len(o.termArrays) {
		return false
	}
	for i := 0; i < len(q.termArrays); i++ {
		t1 := q.termArrays[i]
		t2 := o.termArrays[i]
		if len(t1) != len(t2) {
			return false
		}
		for j := 0; j < len(t1); j++ {
			if !t1[j].Equals(t2[j]) {
				return false
			}
		}
	}
	if len(q.positions) != len(o.positions) {
		return false
	}
	for i := 0; i < len(q.positions); i++ {
		if q.positions[i] != o.positions[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code value for this object.
func (q *MultiPhraseQuery) HashCode() int {
	hash := 1
	for _, termArray := range q.termArrays {
		arrayHash := 1
		for _, term := range termArray {
			arrayHash = 31*arrayHash + term.HashCode()
		}
		hash = 31*hash + arrayHash
	}
	hash = 31*hash + q.slop
	for _, pos := range q.positions {
		hash = 31*hash + pos
	}
	return hash
}
