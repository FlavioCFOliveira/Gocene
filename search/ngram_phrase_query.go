// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// NGramPhraseQuery is a PhraseQuery which is optimized for n-gram phrase query.
// For example, when you query "ABCD" on a 2-gram field, you may want to use
// NGramPhraseQuery rather than PhraseQuery, because NGramPhraseQuery will
// rewrite the query to "AB/0 CD/2", while PhraseQuery will query
// "AB/0 BC/1 CD/2" (where term/position).
//
// Mirrors org.apache.lucene.search.NGramPhraseQuery.
type NGramPhraseQuery struct {
	BaseQuery
	n           int
	phraseQuery *PhraseQuery
}

// NewNGramPhraseQuery constructs an NGramPhraseQuery with the given gram size
// and the underlying phrase query. phraseQuery must be non-nil.
func NewNGramPhraseQuery(n int, phraseQuery *PhraseQuery) *NGramPhraseQuery {
	if phraseQuery == nil {
		panic("NGramPhraseQuery: phraseQuery must not be nil")
	}
	return &NGramPhraseQuery{
		n:           n,
		phraseQuery: phraseQuery,
	}
}

// Rewrite rewrites the query to a simpler form.
func (q *NGramPhraseQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	terms := q.phraseQuery.GetTerms()
	positions := q.phraseQuery.GetPositions()

	isOptimizable := q.phraseQuery.GetSlop() == 0 &&
		q.n >= 2 && // non-overlap n-gram cannot be optimized
		len(terms) >= 3 // short ones can't be optimized

	if isOptimizable {
		for i := 1; i < len(positions); i++ {
			if positions[i] != positions[i-1]+1 {
				isOptimizable = false
				break
			}
		}
	}

	if !isOptimizable {
		return q.phraseQuery.Rewrite(searcher)
	}

	builder := NewPhraseQueryBuilder()
	for i := 0; i < len(terms); i++ {
		if i%q.n == 0 || i == len(terms)-1 {
			builder.AddWithPosition(terms[i], i)
		}
	}
	return builder.Build(), nil
}

// Visit walks the query tree.
func (q *NGramPhraseQuery) Visit(visitor QueryVisitor) {
	q.phraseQuery.Visit(visitor.GetSubVisitor(MUST, q))
}

// Equals checks structural equality.
func (q *NGramPhraseQuery) Equals(other Query) bool {
	o, ok := other.(*NGramPhraseQuery)
	if !ok {
		return false
	}
	return q.n == o.n && q.phraseQuery.Equals(o.phraseQuery)
}

// HashCode returns a stable hash.
func (q *NGramPhraseQuery) HashCode() int {
	h := 1 // Simplified classHash()
	h = 31*h + q.phraseQuery.HashCode()
	h = 31*h + q.n
	return h
}

// N returns the n in n-gram.
func (q *NGramPhraseQuery) N() int {
	return q.n
}

// GetTerms returns the list of terms.
func (q *NGramPhraseQuery) GetTerms() []*index.Term {
	return q.phraseQuery.GetTerms()
}

// GetPositions returns the list of relative positions that each term should appear at.
func (q *NGramPhraseQuery) GetPositions() []int {
	return q.phraseQuery.GetPositions()
}

// ToString prints a user-readable version of this query.
func (q *NGramPhraseQuery) ToString(f string) string {
	return q.phraseQuery.ToString(f)
}

// String returns a debug representation.
func (q *NGramPhraseQuery) String() string {
	return q.ToString("")
}

// Clone returns an independent copy.
func (q *NGramPhraseQuery) Clone() Query {
	// phraseQuery is immutable once built, but we should follow the pattern.
	// Since PhraseQuery is a struct and we have a pointer, we can't easily "Clone" it
	// unless PhraseQuery implements Clone().
	// Looking at phrase_query.go, it doesn't implement Clone() explicitly on the struct,
	// but we can rebuild it using a builder.
	builder := NewPhraseQueryBuilder()
	builder.SetSlop(q.phraseQuery.GetSlop())
	for i, term := range q.phraseQuery.GetTerms() {
		builder.AddWithPosition(term, q.phraseQuery.GetPositions()[i])
	}
	return &NGramPhraseQuery{
		n:           q.n,
		phraseQuery: builder.Build(),
	}
}

// CreateWeight delegates to the underlying PhraseQuery.
func (q *NGramPhraseQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.phraseQuery.CreateWeight(searcher, needsScores, boost)
}
