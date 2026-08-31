// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"cmp"
	"slices"
)

// disjunctionScoreBlockBoundaryPropagator is a helper to propagate block boundaries for disjunctions.
// Because a disjunction matches if any of its sub clauses matches, it is tempting to return the
// minimum block boundary across all clauses. The problem is that it might then make the query
// slow when the minimum competitive score is high and low-scoring clauses don't drive iteration
// anymore. So this class computes block boundaries only across clauses whose maximum score is
// greater than or equal to the minimum competitive score, or the maximum scoring clause if there
// is no such clause.
type disjunctionScoreBlockBoundaryPropagator struct {
	scorers   []Scorer
	maxScores []float32
	leadIndex int
}

// NewDisjunctionScoreBlockBoundaryPropagator creates a new DisjunctionScoreBlockBoundaryPropagator.
func NewDisjunctionScoreBlockBoundaryPropagator(scorers []Scorer) (*disjunctionScoreBlockBoundaryPropagator, error) {
	// Copy the slice to avoid mutating the input
	s := make([]Scorer, len(scorers))
	copy(s, scorers)

	for _, scorer := range s {
		if _, err := scorer.AdvanceShallow(0); err != nil {
			return nil, err
		}
	}

	// Sort by max score ascending, then by cost ascending.
	slices.SortFunc(s, func(a, b Scorer) int {
		scoreA := a.GetMaxScore(NO_MORE_DOCS)
		scoreB := b.GetMaxScore(NO_MORE_DOCS)
		if scoreA != scoreB {
			return cmp.Compare(scoreA, scoreB)
		}
		return cmp.Compare(a.Cost(), b.Cost())
	})

	maxScores := make([]float32, len(s))
	for i, scorer := range s {
		maxScores[i] = scorer.GetMaxScore(NO_MORE_DOCS)
	}

	return &disjunctionScoreBlockBoundaryPropagator{
		scorers:   s,
		maxScores: maxScores,
		leadIndex: 0,
	}, nil
}

// AdvanceShallow advances to the block of documents that contains target.
func (p *disjunctionScoreBlockBoundaryPropagator) AdvanceShallow(target int) (int, error) {
	// For scorers that are below the lead index, just propagate.
	for i := 0; i < p.leadIndex; i++ {
		s := p.scorers[i]
		if s.DocID() < target {
			if _, err := s.AdvanceShallow(target); err != nil {
				return 0, err
			}
		}
	}

	// For scorers above the lead index, we take the minimum boundary.
	leadScorer := p.scorers[p.leadIndex]
	targetForLead := leadScorer.DocID()
	if target > targetForLead {
		targetForLead = target
	}
	upTo, err := leadScorer.AdvanceShallow(targetForLead)
	if err != nil {
		return 0, err
	}

	for i := p.leadIndex + 1; i < len(p.scorers); i++ {
		scorer := p.scorers[i]
		if scorer.DocID() <= target {
			boundary, err := scorer.AdvanceShallow(target)
			if err != nil {
				return 0, err
			}
			if boundary < upTo {
				upTo = boundary
			}
		}
	}

	// If the maximum scoring clauses are beyond `target`, then we use their
	// docID as a boundary. It helps not consider them when computing the
	// maximum score and get a lower score upper bound.
	for i := len(p.scorers) - 1; i > p.leadIndex; i-- {
		scorer := p.scorers[i]
		if scorer.DocID() > target {
			if scorer.DocID()-1 < upTo {
				upTo = scorer.DocID() - 1
			}
		} else {
			break
		}
	}

	return upTo, nil
}

// SetMinCompetitiveScore sets the minimum competitive score to filter out clauses
// that score less than this threshold.
func (p *disjunctionScoreBlockBoundaryPropagator) SetMinCompetitiveScore(minScore float32) {
	// Update the lead index if necessary
	for p.leadIndex < len(p.maxScores)-1 && minScore > p.maxScores[p.leadIndex] {
		p.leadIndex++
	}
}
