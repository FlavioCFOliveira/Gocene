// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// IndriAndQuery is the AND combinator of IndriQuery clauses, scored under
// ScoreMode.TOP_SCORES.
//
// Mirrors org.apache.lucene.search.IndriAndQuery.
type IndriAndQuery struct {
	IndriQuery
}

// NewIndriAndQuery builds an IndriAndQuery from clauses.
func NewIndriAndQuery(clauses []*BooleanClause) *IndriAndQuery {
	return &IndriAndQuery{IndriQuery: *NewIndriQuery(clauses)}
}

// CreateWeight builds an IndriAndWeight. The current Gocene port keeps the
// weight as a thin wrapper around a BooleanWeight-equivalent because the
// full Indri scoring loop (with belief networks and smoothing scores) is
// deferred to the Indri-specific scoring rewrite.
func (q *IndriAndQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewIndriAndWeight(q, searcher, boost), nil
}
