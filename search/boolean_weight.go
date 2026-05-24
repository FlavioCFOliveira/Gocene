// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// BooleanWeight is the Weight implementation for BooleanQuery.
// This is the Go port of Lucene's org.apache.lucene.search.BooleanWeight.
type BooleanWeight struct {
	*BaseWeight
	query         *BooleanQuery
	searcher      *IndexSearcher
	needsScores   bool
	weights       []Weight
	scorerEnabled []bool
}

// NewBooleanWeight creates a new BooleanWeight.
func NewBooleanWeight(query *BooleanQuery, searcher *IndexSearcher, needsScores bool) (*BooleanWeight, error) {
	w := &BooleanWeight{
		BaseWeight:    NewBaseWeight(query),
		query:         query,
		searcher:      searcher,
		needsScores:   needsScores,
		weights:       make([]Weight, len(query.clauses)),
		scorerEnabled: make([]bool, len(query.clauses)),
	}

	// Create weights for each clause
	for i, clause := range query.clauses {
		// For FILTER clauses, we don't need scores
		clauseNeedsScores := needsScores && clause.Occur != FILTER
		weight, err := clause.Query.CreateWeight(searcher, clauseNeedsScores, 1.0)
		if err != nil {
			return nil, err
		}
		w.weights[i] = weight
		w.scorerEnabled[i] = clauseNeedsScores
	}

	return w, nil
}

// Scorer creates a scorer for this weight.
func (w *BooleanWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	var mustScorers []Scorer
	var filterScorers []Scorer
	var shouldScorers []Scorer
	var mustNotScorers []Scorer

	for i, weight := range w.weights {
		if weight == nil {
			continue
		}
		clause := w.query.clauses[i]
		scorer, err := weight.Scorer(context)
		if err != nil {
			return nil, err
		}
		if scorer == nil {
			// A nil scorer for MUST or FILTER means no documents can match.
			if clause.Occur == MUST || clause.Occur == FILTER {
				return nil, nil
			}
			continue
		}
		switch clause.Occur {
		case MUST:
			mustScorers = append(mustScorers, scorer)
		case FILTER:
			filterScorers = append(filterScorers, scorer)
		case SHOULD:
			shouldScorers = append(shouldScorers, scorer)
		case MUST_NOT:
			mustNotScorers = append(mustNotScorers, scorer)
		}
	}

	scoreMode := COMPLETE_NO_SCORES
	if w.needsScores {
		scoreMode = COMPLETE
	}

	return NewBooleanScorerWithClauses(
		mustScorers, filterScorers, shouldScorers, mustNotScorers,
		scoreMode, w.query.minShouldMatch,
	), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *BooleanWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

// Explain returns an explanation of the score for the given document.
func (w *BooleanWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(false, 0, "BooleanWeight explanation not implemented"), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *BooleanWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *BooleanWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	for _, weight := range w.weights {
		if weight != nil && !weight.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *BooleanWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *BooleanWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure BooleanWeight implements Weight
var _ Weight = (*BooleanWeight)(nil)
