// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PhraseQuery matches documents containing a particular sequence of terms.
type PhraseQuery struct {
	slop      int
	field     string
	terms     []*index.Term
	positions []int
}

// PhraseQueryBuilder is a builder for phrase queries.
type PhraseQueryBuilder struct {
	slop      int
	maxTerms  int
	terms     []*index.Term
	positions []int
}

// NewPhraseQueryBuilder creates a new builder for phrase queries.
func NewPhraseQueryBuilder() *PhraseQueryBuilder {
	return &PhraseQueryBuilder{
		slop:     0,
		maxTerms: -1,
		terms:    make([]*index.Term, 0),
		positions: make([]int, 0),
	}
}

// SetSlop sets the slop.
func (b *PhraseQueryBuilder) SetSlop(slop int) *PhraseQueryBuilder {
	b.slop = slop
	return b
}

// SetMaxTerms sets the maximum number of terms allowed in the phrase query.
func (b *PhraseQueryBuilder) SetMaxTerms(maxTerms int) *PhraseQueryBuilder {
	b.maxTerms = maxTerms
	return b
}

// Add adds a term to the end of the query phrase.
func (b *PhraseQueryBuilder) Add(term *index.Term) *PhraseQueryBuilder {
	pos := 0
	if len(b.positions) > 0 {
		pos = 1 + b.positions[len(b.positions)-1]
	}
	return b.AddWithPosition(term, pos)
}

// AddWithPosition adds a term to the end of the query phrase at the specified position.
func (b *PhraseQueryBuilder) AddWithPosition(term *index.Term, position int) *PhraseQueryBuilder {
	if term == nil {
		panic("Cannot add a null term to PhraseQuery")
	}
	if position < 0 {
		panic(fmt.Sprintf("Positions must be >= 0, got %d", position))
	}
	if len(b.positions) > 0 {
		lastPosition := b.positions[len(b.positions)-1]
		if position < lastPosition {
			panic(fmt.Sprintf("Positions must be added in order, got %d after %d", position, lastPosition))
		}
	}
	if len(b.terms) > 0 && term.Field() != b.terms[0].Field() {
		panic(fmt.Sprintf("All terms must be on the same field, got %s and %s", term.Field(), b.terms[0].Field()))
	}
	if b.maxTerms > 0 && len(b.terms) >= b.maxTerms {
		panic(fmt.Sprintf("The current number of terms is %d, which exceeds the limit of %d", len(b.terms), b.maxTerms))
	}
	b.terms = append(b.terms, term)
	b.positions = append(b.positions, position)
	return b
}

// Build builds a phrase query based on the terms that have been added.
func (b *PhraseQueryBuilder) Build() *PhraseQuery {
	return newPhraseQuery(b.slop, b.terms, b.positions)
}

// NewPhraseQuery creates a phrase query which will match documents that contain the given list of terms
// at consecutive positions in field, and at a maximum edit distance of slop.
func NewPhraseQuery(slop int, field string, terms ...string) *PhraseQuery {
	luceneTerms := make([]*index.Term, len(terms))
	for i, t := range terms {
		luceneTerms[i] = index.NewTerm(field, t)
	}
	return NewPhraseQueryWithTerms(slop, field, luceneTerms...)
}

// NewPhraseQueryWithTerms creates a phrase query which will match documents that contain the given list of terms
// at consecutive positions in field, and at a maximum edit distance of slop.
func NewPhraseQueryWithTerms(slop int, field string, terms ...*index.Term) *PhraseQuery {
	positions := make([]int, len(terms))
	for i := range positions {
		positions[i] = i
	}
	return newPhraseQuery(slop, terms, positions)
}

// NewPhraseQueryWithBytes creates a phrase query which will match documents that contain the given list of terms
// at consecutive positions in field, and at a maximum edit distance of slop.
func NewPhraseQueryWithBytes(slop int, field string, terms ...[]byte) *PhraseQuery {
	luceneTerms := make([]*index.Term, len(terms))
	for i, t := range terms {
		luceneTerms[i] = index.NewTermBytes(field, t)
	}
	return NewPhraseQueryWithTerms(slop, field, luceneTerms...)
}

func newPhraseQuery(slop int, terms []*index.Term, positions []int) *PhraseQuery {
	if len(terms) != len(positions) {
		panic("Must have as many terms as positions")
	}
	if slop < 0 {
		panic(fmt.Sprintf("Slop must be >= 0, got %d", slop))
	}
	for _, term := range terms {
		if term == nil {
			panic("Cannot add a null term to PhraseQuery")
		}
	}
	for i := 1; i < len(terms); i++ {
		if terms[i-1].Field() != terms[i].Field() {
			panic("All terms should have the same field")
		}
	}
	for _, pos := range positions {
		if pos < 0 {
			panic(fmt.Sprintf("Positions must be >= 0, got %d", pos))
		}
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] < positions[i-1] {
			panic(fmt.Sprintf("Positions should not go backwards, got %d before %d", positions[i-1], positions[i]))
		}
	}

	var field string
	if len(terms) > 0 {
		field = terms[0].Field()
	}

	return &PhraseQuery{
		slop:      slop,
		field:     field,
		terms:     terms,
		positions: positions,
	}
}

// GetSlop returns the slop for this PhraseQuery.
func (q *PhraseQuery) GetSlop() int {
	return q.slop
}

// GetField returns the field this query applies to.
func (q *PhraseQuery) GetField() string {
	return q.field
}

// GetTerms returns the list of terms in this phrase.
func (q *PhraseQuery) GetTerms() []*index.Term {
	return q.terms
}

// GetPositions returns the relative positions of terms in this phrase.
func (q *PhraseQuery) GetPositions() []int {
	return q.positions
}

// Rewrite rewrites the query to a simpler form.
func (q *PhraseQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	if len(q.terms) == 0 {
		return NewMatchNoDocsQuery("empty PhraseQuery"), nil
	} else if len(q.terms) == 1 {
		return NewTermQuery(q.terms[0]), nil
	} else if q.positions[0] != 0 {
		newPositions := make([]int, len(q.positions))
		for i := 0; i < len(q.positions); i++ {
			newPositions[i] = q.positions[i] - q.positions[0]
		}
		return newPhraseQuery(q.slop, q.terms, newPositions), nil
	}
	return q, nil
}

// Visit walks the query tree.
func (q *PhraseQuery) Visit(visitor QueryVisitor) {
	if !visitor.AcceptField(q.field) {
		return
	}
	v := visitor.GetSubVisitor(MUST, q)
	v.ConsumeTerms(q, q.terms...)
}

// CreateWeight creates a Weight for this query.
func (q *PhraseQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewPhraseWeight(q, searcher, scoreMode, boost), nil
}

// CreateWeightBasic implements the basic Query interface.
func (q *PhraseQuery) CreateWeightBasic(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	mode := COMPLETE
	if !needsScores {
		mode = COMPLETE_NO_SCORES
	}
	return q.CreateWeight(searcher, mode, boost)
}

// ToString prints a user-readable version of this query.
func (q *PhraseQuery) ToString(f string) string {
	var buffer strings.Builder
	if q.field != "" && q.field != f {
		buffer.WriteString(q.field)
		buffer.WriteString(":")
	}

	buffer.WriteString("\"")
	var maxPosition int
	if len(q.positions) == 0 {
		maxPosition = -1
	} else {
		maxPosition = q.positions[len(q.positions)-1]
	}

	pieces := make([]string, maxPosition+1)
	for i := 0; i < len(q.terms); i++ {
		pos := q.positions[i]
		s := pieces[pos]
		if s == "" {
			s = q.terms[i].Text()
		} else {
			s = s + "|" + q.terms[i].Text()
		}
		pieces[pos] = s
	}

	for i := 0; i < len(pieces); i++ {
		if i > 0 {
			buffer.WriteByte(' ')
		}
		s := pieces[i]
		if s == "" {
			buffer.WriteByte('?')
		} else {
			buffer.WriteString(s)
		}
	}
	buffer.WriteString("\"")

	if q.slop != 0 {
		buffer.WriteByte('~')
		buffer.WriteString(fmt.Sprintf("%d", q.slop))
	}

	return buffer.String()
}

// Equals returns true iff other is equal to this.
func (q *PhraseQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*PhraseQuery); ok {
		if q.slop != otherQuery.slop {
			return false
		}
		if len(q.terms) != len(otherQuery.terms) {
			return false
		}
		for i := range q.terms {
			if !q.terms[i].Equals(otherQuery.terms[i]) {
				return false
			}
		}
		if len(q.positions) != len(otherQuery.positions) {
			return false
		}
		for i := range q.positions {
			if q.positions[i] != otherQuery.positions[i] {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code value for this object.
func (q *PhraseQuery) HashCode() int {
	h := 1 // Simplified classHash()
	h = 31*h + q.slop

	// Hash terms
	termHash := 0
	for _, t := range q.terms {
		termHash = 31*termHash + t.HashCode()
	}
	h = 31*h + termHash

	// Hash positions
	posHash := 0
	for _, p := range q.positions {
		posHash = 31*posHash + p
	}
	h = 31*h + posHash

	return h
}

// postingsAndFreq contains term postings and position information for phrase matching.
type postingsAndFreq struct {
	postings PostingsEnum
	impacts  ImpactsEnum
	position int
	terms    []*index.Term
	nTerms   int
}

// NewPostingsAndFreq creates a postingsAndFreq instance.
func NewPostingsAndFreq(postings PostingsEnum, impacts ImpactsEnum, position int, terms ...*index.Term) *postingsAndFreq {
	var finalTerms []*index.Term
	nTerms := len(terms)
	if nTerms > 0 {
		if nTerms == 1 {
			finalTerms = terms
		} else {
			finalTerms = make([]*index.Term, nTerms)
			copy(finalTerms, terms)
			sort.Slice(finalTerms, func(i, j int) bool {
				return finalTerms[i].CompareTo(finalTerms[j]) < 0
			})
		}
	}
	return &postingsAndFreq{
		postings: postings,
		impacts:  impacts,
		position: position,
		terms:    finalTerms,
		nTerms:   nTerms,
	}
}

// NewPostingsAndFreqWithList creates a postingsAndFreq instance from a list of terms.
func NewPostingsAndFreqWithList(postings PostingsEnum, impacts ImpactsEnum, position int, termsList []*index.Term) *postingsAndFreq {
	nTerms := len(termsList)
	var finalTerms []*index.Term
	if nTerms > 0 {
		finalTerms = make([]*index.Term, nTerms)
		copy(finalTerms, termsList)
		if nTerms > 1 {
			sort.Slice(finalTerms, func(i, j int) bool {
				return finalTerms[i].CompareTo(finalTerms[j]) < 0
			})
		}
	}
	return &postingsAndFreq{
		postings: postings,
		impacts:  impacts,
		position: position,
		terms:    finalTerms,
		nTerms:   nTerms,
	}
}

// CompareTo compares this postingsAndFreq with another.
func (p *postingsAndFreq) CompareTo(other *postingsAndFreq) int {
	if p.position != other.position {
		return p.position - other.position
	}
	if p.nTerms != other.nTerms {
		return p.nTerms - other.nTerms
	}
	if p.nTerms == 0 {
		return 0
	}
	for i := 0; i < len(p.terms); i++ {
		res := p.terms[i].CompareTo(other.terms[i])
		if res != 0 {
			return res
		}
	}
	return 0
}

// HashCode returns a hash code for this object.
func (p *postingsAndFreq) HashCode() int {
	prime := 31
	result := 1
	result = prime*result + p.position
	for i := 0; i < p.nTerms; i++ {
		result = prime*result + p.terms[i].HashCode()
	}
	return result
}

// Equals checks if this postingsAndFreq is equal to another.
func (p *postingsAndFreq) Equals(other interface{}) bool {
	if p == other {
		return true
	}
	if other == nil {
		return false
	}
	otherP, ok := other.(*postingsAndFreq)
	if !ok {
		return false
	}
	if p.position != otherP.position {
		return false
	}
	if p.terms == nil {
		return otherP.terms == nil
	}
	if len(p.terms) != len(otherP.terms) {
		return false
	}
	for i := range p.terms {
		if !p.terms[i].Equals(otherP.terms[i]) {
			return false
		}
	}
	return true
}

// TermPositionsCost returns an expected cost in simple operations of processing the occurrences
// of a term in a document that contains the term.
func TermPositionsCost(termsEnum index.TermsEnum) float32 {
	docFreq := termsEnum.DocFreq()
	totalTermFreq := termsEnum.TotalTermFreq()
	expOccurrencesInMatchingDoc := float32(totalTermFreq) / float32(docFreq)
	return 256 + expOccurrencesInMatchingDoc*7
}
