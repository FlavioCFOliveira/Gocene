// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// Query is the abstract base class for all queries in the index package.
// This is a minimal interface for index-level query operations.
type Query interface {
	// Rewrite rewrites the query to a simpler form.
	Rewrite(reader *spi.IndexReader) (Query, error)
	// Clone creates a copy of this query.
	Clone() Query
	// Equals checks if this query equals another.
	Equals(other Query) bool
	// HashCode returns a hash code for this query.
	HashCode() int
	// CreateWeight creates a Weight for this query.
	CreateWeight(searcher IndexSearcher, needsScores bool, boost float32) (Weight, error)
}

// MatchAllDocsQuery matches all documents in the index.
type MatchAllDocsQuery struct{}

func (q *MatchAllDocsQuery) Rewrite(reader *spi.IndexReader) (Query, error) {
	return q, nil
}

func (q *MatchAllDocsQuery) Clone() Query {
	return q
}

func (q *MatchAllDocsQuery) Equals(other Query) bool {
	_, ok := other.(*MatchAllDocsQuery)
	return ok
}

func (q *MatchAllDocsQuery) HashCode() int {
	return 0
}

func (q *MatchAllDocsQuery) CreateWeight(searcher IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return nil, fmt.Errorf("MatchAllDocsQuery is not used for scoring")
}

// IndexSearcher is a minimal interface for searching.
type IndexSearcher interface {
	Search(query Query, n int) (*TopDocs, error)
}

// Weight is a minimal interface for query weights.
type Weight interface {
	// GetValue returns the weight value.
	GetValue() float64
}

// TopDocs represents the top-scoring documents.
type TopDocs struct {
	TotalHits int
	ScoreDocs []ScoreDoc
}

// ScoreDoc represents a scored document.
type ScoreDoc struct {
	Doc   int
	Score float32
}
