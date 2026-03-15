// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

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
func (w *MatchAllDocsWeight) Scorer(reader index.IndexReaderInterface) (Scorer, error) {
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

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *MatchAllDocsScorer) DocIDRunEnd() int {
	return s.maxDoc
}

// MatchNoDocsQuery matches no documents.
// This is used as a placeholder when a query cannot match anything.
type MatchNoDocsQuery struct {
	*BaseQuery
	reason string
}

// NewMatchNoDocsQuery creates a new MatchNoDocsQuery.
func NewMatchNoDocsQuery() *MatchNoDocsQuery {
	return &MatchNoDocsQuery{
		BaseQuery: &BaseQuery{},
		reason:    "MatchNoDocsQuery",
	}
}

// NewMatchNoDocsQueryWithReason creates a new MatchNoDocsQuery with a reason.
func NewMatchNoDocsQueryWithReason(reason string) *MatchNoDocsQuery {
	return &MatchNoDocsQuery{
		BaseQuery: &BaseQuery{},
		reason:    reason,
	}
}

// Clone creates a copy of this query.
func (q *MatchNoDocsQuery) Clone() Query {
	return NewMatchNoDocsQueryWithReason(q.reason)
}

// Equals checks if this query equals another.
func (q *MatchNoDocsQuery) Equals(other Query) bool {
	if o, ok := other.(*MatchNoDocsQuery); ok {
		return q.reason == o.reason
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *MatchNoDocsQuery) HashCode() int {
	hash := 0
	for i := 0; i < len(q.reason); i++ {
		hash = 31*hash + int(q.reason[i])
	}
	return hash
}

// Rewrite rewrites the query to a simpler form.
func (q *MatchNoDocsQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// String returns a string representation of this query.
func (q *MatchNoDocsQuery) String() string {
	return "MatchNoDocsQuery"
}

// CreateWeight creates a Weight for this query.
func (q *MatchNoDocsQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewMatchNoDocsWeight(q), nil
}

// MatchNoDocsWeight is the Weight implementation for MatchNoDocsQuery.
type MatchNoDocsWeight struct {
	*BaseWeight
}

// NewMatchNoDocsWeight creates a new MatchNoDocsWeight.
func NewMatchNoDocsWeight(query Query) *MatchNoDocsWeight {
	return &MatchNoDocsWeight{
		BaseWeight: NewBaseWeight(query),
	}
}

// Scorer creates a scorer for this weight.
func (w *MatchNoDocsWeight) Scorer(reader index.IndexReaderInterface) (Scorer, error) {
	return NewMatchNoDocsScorer(w), nil
}

// GetValueForNormalization returns the value for normalization.
func (w *MatchNoDocsWeight) GetValueForNormalization() float32 {
	return 0
}

// Normalize normalizes this weight.
func (w *MatchNoDocsWeight) Normalize(norm float32) {}

// MatchNoDocsScorer is the Scorer implementation for MatchNoDocsQuery.
type MatchNoDocsScorer struct {
	*BaseScorer
}

// NewMatchNoDocsScorer creates a new MatchNoDocsScorer.
func NewMatchNoDocsScorer(weight Weight) *MatchNoDocsScorer {
	return &MatchNoDocsScorer{
		BaseScorer: NewBaseScorer(weight),
	}
}

// DocID returns the current document ID.
func (s *MatchNoDocsScorer) DocID() int {
	return NO_MORE_DOCS
}

// NextDoc advances to the next document.
func (s *MatchNoDocsScorer) NextDoc() (int, error) {
	return NO_MORE_DOCS, nil
}

// Advance advances to the target document.
func (s *MatchNoDocsScorer) Advance(target int) (int, error) {
	return NO_MORE_DOCS, nil
}

// Score returns the score.
func (s *MatchNoDocsScorer) Score() float32 {
	return 0
}

// Cost returns the cost.
func (s *MatchNoDocsScorer) Cost() int64 {
	return 0
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *MatchNoDocsScorer) DocIDRunEnd() int {
	return NO_MORE_DOCS
}
