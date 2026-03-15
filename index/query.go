// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// Query is the abstract base class for all queries in the index package.
// This is a minimal interface for index-level query operations.
type Query interface {
	// Rewrite rewrites the query to a simpler form.
	Rewrite(reader *IndexReader) (Query, error)
	// Clone creates a copy of this query.
	Clone() Query
	// Equals checks if this query equals another.
	Equals(other Query) bool
	// HashCode returns a hash code for this query.
	HashCode() int
	// CreateWeight creates a Weight for this query.
	CreateWeight(searcher IndexSearcher, needsScores bool, boost float32) (Weight, error)
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
