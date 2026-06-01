// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// IndexReader is a minimal interface needed by Query.
type IndexReader interface {
	DocCount() int
	NumDocs() int
	MaxDoc() int
}

// Query is the abstract base class for all queries.
type Query interface {
	// Rewrite rewrites the query to a simpler form.
	Rewrite(reader IndexReader) (Query, error)
	// Clone creates a copy of this query.
	Clone() Query
	// Equals checks if this query equals another.
	Equals(other Query) bool
	// HashCode returns a hash code for this query.
	HashCode() int
	// CreateWeight creates a Weight for this query.
	CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error)
}

// scoreModeWeightCreator is the optional, ScoreMode-aware sibling of
// Query.CreateWeight.
//
// In Apache Lucene 10.4.0 every Query.createWeight receives the full ScoreMode
// enum (COMPLETE / COMPLETE_NO_SCORES / TOP_SCORES / TOP_DOCS /
// TOP_DOCS_WITH_SCORES), which lets composite queries forward a precise mode to
// their children — for example BooleanQuery forwards COMPLETE_NO_SCORES to
// FILTER / MUST_NOT clauses, and ConstantScoreQuery forwards COMPLETE_NO_SCORES
// or TOP_DOCS to its wrapped query depending on exhaustiveness.
//
// Gocene's stable Query.CreateWeight signature collapses that enum to a
// needsScores bool, which would prevent a sub-query from observing anything but
// COMPLETE / COMPLETE_NO_SCORES. Queries that must propagate the exact ScoreMode
// to their children (BooleanQuery, ConstantScoreQuery) — and test wrappers that
// assert on the received mode — implement this interface. IndexSearcher's
// createWeight dispatch prefers it when present and otherwise falls back to the
// bool-based CreateWeight (collapsing the mode via ScoreMode.needsScores), so
// the change is fully backward compatible with the dozens of existing
// CreateWeight implementations.
type scoreModeWeightCreator interface {
	// CreateWeightScoreMode builds a Weight for the given full ScoreMode.
	CreateWeightScoreMode(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error)
}

// BaseQuery provides common functionality for queries.
type BaseQuery struct{}

func (q *BaseQuery) Rewrite(reader IndexReader) (Query, error) { return q, nil }
func (q *BaseQuery) Clone() Query                              { return q }
func (q *BaseQuery) Equals(other Query) bool                   { return false }
func (q *BaseQuery) HashCode() int                             { return 0 }
func (q *BaseQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return nil, nil
}
