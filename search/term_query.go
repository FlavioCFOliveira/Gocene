// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermQuery matches documents containing a term.
// This is the Go port of Lucene's org.apache.lucene.search.TermQuery.
type TermQuery struct {
	term               *index.Term
	perReaderTermState *index.TermStates
}

// NewTermQuery constructs a query for the term.
func NewTermQuery(t *index.Term) *TermQuery {
	if t == nil {
		panic("term cannot be nil")
	}
	return &TermQuery{
		term: t,
	}
}

// NewTermQueryWithStates constructs a TermQuery that will use the provided
// TermStates instead of building them. This is an expert method.
func NewTermQueryWithStates(t *index.Term, states *index.TermStates) *TermQuery {
	if t == nil {
		panic("term cannot be nil")
	}
	if states == nil {
		panic("states cannot be nil")
	}
	return &TermQuery{
		term:               t,
		perReaderTermState: states,
	}
}

// GetTerm returns the term of this query.
func (q *TermQuery) GetTerm() *index.Term {
	return q.term
}

// GetTermStates returns the TermStates passed to the constructor, or nil if it was not passed.
// This is marked as experimental in Lucene.
func (q *TermQuery) GetTermStates() *index.TermStates {
	return q.perReaderTermState
}

// Rewrite rewrites the query to a simpler form. TermQuery does not rewrite.
func (q *TermQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	return q, nil
}

// Clone creates a copy of this query.
func (q *TermQuery) Clone() Query {
	return &TermQuery{
		term:               q.term,
		perReaderTermState: q.perReaderTermState,
	}
}

// Equals checks if this query equals another.
func (q *TermQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*TermQuery); ok {
		return q.term.Equals(otherQuery.term)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *TermQuery) HashCode() int {
	return classHashTermQuery ^ q.term.HashCode()
}

// CreateWeight creates a Weight for this query.
// This implements the Query interface with a bool-based scoreMode.
func (q *TermQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	// Convert bool to ScoreMode
	var scoreMode ScoreMode
	if needsScores {
		scoreMode = COMPLETE
	} else {
		scoreMode = COMPLETE_NO_SCORES
	}
	return q.CreateWeightScoreMode(searcher, scoreMode, boost)
}

// CreateWeightScoreMode creates a Weight with full ScoreMode information.
// This implements the scoreModeWeightCreator interface.
func (q *TermQuery) CreateWeightScoreMode(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	context := searcher.GetReader().GetTopReaderContext()
	var termState *index.TermStates
	if q.perReaderTermState == nil || !q.perReaderTermState.WasBuiltFor(context) {
		var err error
		termState, err = index.BuildTermStates(searcher.GetReader(), q.term, scoreMode.NeedsScores())
		if err != nil {
			return nil, err
		}
	} else {
		termState = q.perReaderTermState
	}

	return NewTermWeight(searcher, q.term, scoreMode, boost, termState), nil
}

// ToString returns a user-readable version of this query.
func (q *TermQuery) ToString(field string) string {
	if q.term.Field() != field {
		return fmt.Sprintf("%s:%s", q.term.Field(), q.term.Text())
	}
	return q.term.Text()
}

// Ensure TermQuery implements Query
var _ Query = (*TermQuery)(nil)

// Ensure TermQuery implements scoreModeWeightCreator
var _ scoreModeWeightCreator = (*TermQuery)(nil)

// classHashTermQuery seeds the type-stable hash for TermQuery.
// The Java reference uses Query.classHash(), which returns a runtime-class-derived constant.
// Gocene uses a per-type literal so distinct query classes never collide on equal term triples.
const classHashTermQuery = 0x5472_6d51 // "TrmQ"
