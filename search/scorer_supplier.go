// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// ScorerSupplier provides a lazy way to create Scorers.
// It allows for cost-based optimization before creating the actual scorer.
type ScorerSupplier interface {
	// Get returns a Scorer for the given leadCost.
	// The leadCost is an estimate of the number of documents that the scorer
	// will be asked to score.
	Get(leadCost int64) (Scorer, error)

	// Cost returns an estimate of the number of documents this scorer will match.
	Cost() int64

	// SetTopLevelScoringClause marks this as a top-level scoring clause.
	// This is used for optimizations when scoring is needed.
	SetTopLevelScoringClause()
}

// BaseScorerSupplier provides common functionality for ScorerSupplier implementations.
type BaseScorerSupplier struct {
	cost int64
}

// NewBaseScorerSupplier creates a new BaseScorerSupplier.
func NewBaseScorerSupplier(cost int64) *BaseScorerSupplier {
	return &BaseScorerSupplier{cost: cost}
}

// Cost returns the estimated cost.
func (s *BaseScorerSupplier) Cost() int64 {
	return s.cost
}

// SetTopLevelScoringClause is a no-op default implementation.
func (s *BaseScorerSupplier) SetTopLevelScoringClause() {}
