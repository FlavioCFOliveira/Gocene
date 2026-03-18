// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BlockJoinWeight is the Weight implementation for block join queries.
// It wraps the child query's weight and handles the block join scoring logic.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.BlockJoinWeight.
type BlockJoinWeight struct {
	// query is the parent block join query
	query search.Query

	// childWeight is the weight of the child query
	childWeight search.Weight

	// parentWeight is the weight of the parent filter
	parentWeight search.Weight

	// scoreMode determines how child scores are combined
	scoreMode ScoreMode
}

// NewBlockJoinWeight creates a new BlockJoinWeight.
// Parameters:
//   - query: the parent block join query
//   - childWeight: the weight of the child query
//   - parentWeight: the weight of the parent filter
//   - scoreMode: how to combine scores from child documents
func NewBlockJoinWeight(query search.Query, childWeight search.Weight, parentWeight search.Weight, scoreMode ScoreMode) *BlockJoinWeight {
	return &BlockJoinWeight{
		query:        query,
		childWeight:  childWeight,
		parentWeight: parentWeight,
		scoreMode:    scoreMode,
	}
}

// GetQuery returns the parent query.
func (w *BlockJoinWeight) GetQuery() search.Query {
	return w.query
}

// GetChildWeight returns the child weight.
func (w *BlockJoinWeight) GetChildWeight() search.Weight {
	return w.childWeight
}

// GetParentWeight returns the parent weight.
func (w *BlockJoinWeight) GetParentWeight() search.Weight {
	return w.parentWeight
}

// GetScoreMode returns the score mode.
func (w *BlockJoinWeight) GetScoreMode() ScoreMode {
	return w.scoreMode
}

// Scorer creates a scorer for this weight.
func (w *BlockJoinWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	// Create child and parent scorers
	childScorer, err := w.childWeight.Scorer(context)
	if err != nil {
		return nil, fmt.Errorf("failed to create child scorer: %w", err)
	}

	parentScorer, err := w.parentWeight.Scorer(context)
	if err != nil {
		return nil, fmt.Errorf("failed to create parent scorer: %w", err)
	}

	// If either scorer is nil, return nil
	if childScorer == nil || parentScorer == nil {
		return nil, nil
	}

	// Create the block join scorer
	return NewBlockJoinScorer(childScorer, parentScorer, w.scoreMode), nil
}

// ScorerSupplier creates a ScorerSupplier for this weight.
func (w *BlockJoinWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	// Get child scorer supplier
	childSupplier, err := w.childWeight.ScorerSupplier(context)
	if err != nil {
		return nil, fmt.Errorf("failed to create child scorer supplier: %w", err)
	}

	// Get parent scorer supplier
	parentSupplier, err := w.parentWeight.ScorerSupplier(context)
	if err != nil {
		return nil, fmt.Errorf("failed to create parent scorer supplier: %w", err)
	}

	// If either supplier is nil, return nil
	if childSupplier == nil || parentSupplier == nil {
		return nil, nil
	}

	// Create a scorer supplier that wraps both
	return &blockJoinScorerSupplier{
		childSupplier:  childSupplier,
		parentSupplier: parentSupplier,
		scoreMode:      w.scoreMode,
	}, nil
}

// blockJoinScorerSupplier implements ScorerSupplier for BlockJoinWeight.
type blockJoinScorerSupplier struct {
	childSupplier  search.ScorerSupplier
	parentSupplier search.ScorerSupplier
	scoreMode      ScoreMode
}

// Get returns a Scorer for the given leadCost.
func (s *blockJoinScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	childScorer, err := s.childSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}

	parentScorer, err := s.parentSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}

	if childScorer == nil || parentScorer == nil {
		return nil, nil
	}

	return NewBlockJoinScorer(childScorer, parentScorer, s.scoreMode), nil
}

// Cost returns an estimate of the number of documents this scorer will match.
func (s *blockJoinScorerSupplier) Cost() int64 {
	// Return the minimum of child and parent costs
	childCost := s.childSupplier.Cost()
	parentCost := s.parentSupplier.Cost()
	if childCost < parentCost {
		return childCost
	}
	return parentCost
}

// SetTopLevelScoringClause marks this as a top-level scoring clause.
func (s *blockJoinScorerSupplier) SetTopLevelScoringClause() {
	s.childSupplier.SetTopLevelScoringClause()
	s.parentSupplier.SetTopLevelScoringClause()
}

// Explain returns an explanation of the score for the given document.
func (w *BlockJoinWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	// Get the scorer
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}

	if scorer == nil {
		return search.NewExplanation(false, 0, "no matching documents"), nil
	}

	// Advance to the target document
	actualDoc, err := scorer.Advance(doc)
	if err != nil {
		return nil, err
	}

	if actualDoc != doc {
		return search.NewExplanation(false, 0, fmt.Sprintf("document %d does not match", doc)), nil
	}

	// Get the score
	score := scorer.Score()

	// Create explanation
	explanation := search.NewExplanation(true, score, fmt.Sprintf("BlockJoinQuery, score mode: %s", w.scoreMode))

	// Add child explanation if available
	if w.childWeight != nil {
		childExplain, err := w.childWeight.Explain(context, doc)
		if err == nil && childExplain != nil {
			explanation.AddDetail(childExplain)
		}
	}

	return explanation, nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *BlockJoinWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	// Delegate to the standard implementation
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *BlockJoinWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	// Block join weights are generally not cacheable due to their complex nature
	return false
}

// Count returns the count of matching documents in sub-linear time.
// Returns -1 if the count cannot be computed efficiently.
func (w *BlockJoinWeight) Count(context *index.LeafReaderContext) (int, error) {
	// Block join queries require full evaluation
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *BlockJoinWeight) Matches(context *index.LeafReaderContext, doc int) (search.Matches, error) {
	// Get the scorer
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}

	if scorer == nil {
		return nil, nil
	}

	// Advance to the target document
	actualDoc, err := scorer.Advance(doc)
	if err != nil {
		return nil, err
	}

	if actualDoc != doc {
		return nil, nil
	}

	// Return basic matches
	return search.NewBaseMatches(w.query, doc), nil
}

// Ensure BlockJoinWeight implements Weight
var _ search.Weight = (*BlockJoinWeight)(nil)
