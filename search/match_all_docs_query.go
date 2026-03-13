// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// MatchAllDocsQuery matches all documents in the index.
type MatchAllDocsQuery struct {
	*BaseQuery
}

// NewMatchAllDocsQuery creates a new MatchAllDocsQuery.
func NewMatchAllDocsQuery() *MatchAllDocsQuery {
	return &MatchAllDocsQuery{
		BaseQuery: &BaseQuery{},
	}
}

// Clone creates a copy of this query.
func (q *MatchAllDocsQuery) Clone() Query {
	return NewMatchAllDocsQuery()
}

// Equals checks if this query equals another.
func (q *MatchAllDocsQuery) Equals(other Query) bool {
	_, ok := other.(*MatchAllDocsQuery)
	return ok
}

// HashCode returns a hash code for this query.
func (q *MatchAllDocsQuery) HashCode() int {
	return 0
}

// Rewrite rewrites the query to a simpler form.
func (q *MatchAllDocsQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *MatchAllDocsQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewMatchAllDocsWeight(q, boost), nil
}

// MatchAllDocsWeight is the Weight implementation for MatchAllDocsQuery.
type MatchAllDocsWeight struct {
	*BaseWeight
	boost float32
}

// NewMatchAllDocsWeight creates a new MatchAllDocsWeight.
func NewMatchAllDocsWeight(query Query, boost float32) *MatchAllDocsWeight {
	return &MatchAllDocsWeight{
		BaseWeight: NewBaseWeight(query),
		boost:      boost,
	}
}

// Scorer creates a scorer for this weight.
func (w *MatchAllDocsWeight) Scorer(reader IndexReader) (Scorer, error) {
	return NewMatchAllDocsScorer(w, reader.MaxDoc(), w.boost), nil
}

// MatchAllDocsScorer is the Scorer implementation for MatchAllDocsQuery.
type MatchAllDocsScorer struct {
	*BaseScorer
	maxDoc int
	doc    int
	score  float32
}

// NewMatchAllDocsScorer creates a new MatchAllDocsScorer.
func NewMatchAllDocsScorer(weight Weight, maxDoc int, score float32) *MatchAllDocsScorer {
	return &MatchAllDocsScorer{
		BaseScorer: NewBaseScorer(weight),
		maxDoc:     maxDoc,
		doc:        -1,
		score:      score,
	}
}

// DocID returns the current document ID.
func (s *MatchAllDocsScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the next document.
func (s *MatchAllDocsScorer) NextDoc() (int, error) {
	s.doc++
	if s.doc >= s.maxDoc {
		s.doc = NO_MORE_DOCS
	}
	return s.doc, nil
}

// Advance advances to the target document.
func (s *MatchAllDocsScorer) Advance(target int) (int, error) {
	s.doc = target
	if s.doc >= s.maxDoc {
		s.doc = NO_MORE_DOCS
	}
	return s.doc, nil
}

// Score returns the score.
func (s *MatchAllDocsScorer) Score() float32 {
	return s.score
}

// Cost returns the cost.
func (s *MatchAllDocsScorer) Cost() int64 {
	return int64(s.maxDoc)
}
