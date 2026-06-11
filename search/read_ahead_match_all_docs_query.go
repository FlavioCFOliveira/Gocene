// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/ReadAheadMatchAllDocsQuery.java
//
// ReadAheadMatchAllDocsQuery is a helper Query that matches all documents
// by returning a DenseConjunctionBulkScorer over a single clause.  It is
// used to validate TopFieldCollector read-ahead compatibility.

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// ReadAheadMatchAllDocsQuery matches all documents using a
// DenseConjunctionBulkScorer.  Mirrors
// org.apache.lucene.search.ReadAheadMatchAllDocsQuery.
type ReadAheadMatchAllDocsQuery struct {
	*BaseQuery
}

// NewReadAheadMatchAllDocsQuery creates a new ReadAheadMatchAllDocsQuery.
func NewReadAheadMatchAllDocsQuery() *ReadAheadMatchAllDocsQuery {
	return &ReadAheadMatchAllDocsQuery{BaseQuery: &BaseQuery{}}
}

// String returns a string representation of this query.
func (q *ReadAheadMatchAllDocsQuery) String(_ string) string {
	return "ReadAheadMatchAllDocsQuery"
}

// Equals checks if this query equals another.
func (q *ReadAheadMatchAllDocsQuery) Equals(other Query) bool {
	_, ok := other.(*ReadAheadMatchAllDocsQuery)
	return ok
}

// HashCode returns a hash code for this query.
func (q *ReadAheadMatchAllDocsQuery) HashCode() int {
	return 0
}

// Visit visits this query.  Mirrors ReadAheadMatchAllDocsQuery.visit().
func (q *ReadAheadMatchAllDocsQuery) Visit(visitor QueryVisitor) {
	// no-op: this query does not match specific terms/fields
}

// Clone creates a copy of this query.
func (q *ReadAheadMatchAllDocsQuery) Clone() Query {
	return NewReadAheadMatchAllDocsQuery()
}

// Rewrite rewrites this query.
func (q *ReadAheadMatchAllDocsQuery) Rewrite(_ IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a weight for scoring.
func (q *ReadAheadMatchAllDocsQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewConstantScoreWeight(
		q,
		boost,
		func(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
			maxDoc := ctx.Reader().MaxDoc()
			return NewSingleClauseDenseScorerSupplier(maxDoc, boost), nil
		},
		func(ctx *index.LeafReaderContext) bool { return true },
	), nil
}

// SingleClauseDenseScorerSupplier provides a scorer over a single
// DenseConjunctionBulkScorer clause covering [0, maxDoc).  It mirrors
// the anonymous ScorerSupplier inside ReadAheadMatchAllDocsQuery's
// ConstantScoreWeight.
type SingleClauseDenseScorerSupplier struct {
	*BaseScorerSupplier
	maxDoc int
	score  float32
}

// NewSingleClauseDenseScorerSupplier creates a supplier for the given
// document range and constant score.
func NewSingleClauseDenseScorerSupplier(maxDoc int, score float32) *SingleClauseDenseScorerSupplier {
	return &SingleClauseDenseScorerSupplier{
		BaseScorerSupplier: NewBaseScorerSupplier(int64(maxDoc)),
		maxDoc:             maxDoc,
		score:              score,
	}
}

// Get returns a Scorer for single-clause dense iteration.
func (s *SingleClauseDenseScorerSupplier) Get(_ int64) (Scorer, error) {
	return NewConstantScoreScorer(s.score, TOP_DOCS, NewRangeDocIdSetIterator(0, s.maxDoc)), nil
}

// BulkScorer returns a DenseConjunctionBulkScorer over a single
// all-docs clause.
func (s *SingleClauseDenseScorerSupplier) BulkScorer() (BulkScorer, error) {
	return NewDenseConjunctionBulkScorer(
		[]DocIdSetIterator{NewRangeDocIdSetIterator(0, s.maxDoc)},
		nil,
		s.maxDoc,
		s.score,
	)
}

// Verify interface compliance.
var (
	_ Query         = (*ReadAheadMatchAllDocsQuery)(nil)
	_ ScorerSupplier = (*SingleClauseDenseScorerSupplier)(nil)
)
