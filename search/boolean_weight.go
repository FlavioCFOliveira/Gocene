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
func (w *BooleanWeight) Scorer(reader index.IndexReaderInterface) (Scorer, error) {
	// Collect scorers for each clause
	var allScorers []Scorer

	for _, weight := range w.weights {
		if weight == nil {
			continue
		}

		scorer, err := weight.Scorer(reader)
		if err != nil {
			return nil, err
		}

		if scorer != nil {
			allScorers = append(allScorers, scorer)
		}
	}

	// Create a BooleanScorer with all collected scorers
	scoreMode := COMPLETE_NO_SCORES
	if w.needsScores {
		scoreMode = COMPLETE
	}

	return NewBooleanScorer(allScorers, scoreMode, w.query.minShouldMatch), nil
}

// GetValueForNormalization returns the value for normalization.
func (w *BooleanWeight) GetValueForNormalization() float32 {
	sum := float32(0)
	for i, weight := range w.weights {
		if w.query.clauses[i].Occur == MUST || w.query.clauses[i].Occur == SHOULD {
			sum += weight.GetValueForNormalization()
		}
	}
	return sum
}

// Normalize normalizes this weight.
func (w *BooleanWeight) Normalize(norm float32) {
	for _, weight := range w.weights {
		weight.Normalize(norm)
	}
}

// Ensure BooleanWeight implements Weight
var _ Weight = (*BooleanWeight)(nil)
