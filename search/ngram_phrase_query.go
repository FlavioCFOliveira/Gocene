// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// NGramPhraseQuery wraps a PhraseQuery whose terms come from an n-gram
// tokenizer, allowing the query to be optimised by dropping intermediate terms
// at rewrite time when slop is 0, n >= 2, the query has 3+ consecutive terms,
// and all positions are sequential.
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
	return &NGramPhraseQuery{n: n, phraseQuery: phraseQuery}
}

// N returns the gram size.
func (q *NGramPhraseQuery) N() int { return q.n }

// PhraseQuery returns the underlying PhraseQuery.
func (q *NGramPhraseQuery) PhraseQuery() *PhraseQuery { return q.phraseQuery }

// String returns a debug representation.
func (q *NGramPhraseQuery) String() string {
	return fmt.Sprintf("NGramPhraseQuery(n=%d, phrase=%v)", q.n, sprintQuery(q.phraseQuery))
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
	h := 17
	h = 31*h + q.n
	h = 31*h + q.phraseQuery.HashCode()
	return h
}

// Clone returns an independent copy.
func (q *NGramPhraseQuery) Clone() Query {
	clone := *q.phraseQuery
	return &NGramPhraseQuery{n: q.n, phraseQuery: &clone}
}

// Rewrite returns the underlying PhraseQuery directly. In Lucene the rewrite
// also drops every Nth term to compact the n-gram phrase; that optimisation
// requires position-level access on PhraseQuery and is left to a follow-up
// task once the necessary accessors are exposed.
func (q *NGramPhraseQuery) Rewrite(reader IndexReader) (Query, error) {
	return q.phraseQuery, nil
}

// CreateWeight delegates to the underlying PhraseQuery.
func (q *NGramPhraseQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.phraseQuery.CreateWeight(searcher, needsScores, boost)
}
