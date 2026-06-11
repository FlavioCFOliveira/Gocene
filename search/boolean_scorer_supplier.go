// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
)

// BooleanScorerSupplier provides scorers for BooleanQuery.
// It handles cost-based optimization and lead cost propagation.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/java/org/apache/lucene/search/BooleanScorerSupplier.java
type BooleanScorerSupplier struct {
	weight         Weight
	subs           map[Occur][]ScorerSupplier
	scoreMode      ScoreMode
	minShouldMatch int
	maxDoc         int
	cost           int64 // cached; -1 means not yet computed
	topLevel       bool
}

// NewBooleanScorerSupplier creates a new BooleanScorerSupplier.
func NewBooleanScorerSupplier(weight Weight, subs map[Occur][]ScorerSupplier, scoreMode ScoreMode, minShouldMatch int, maxDoc int) *BooleanScorerSupplier {
	return &BooleanScorerSupplier{
		weight:         weight,
		subs:           subs,
		scoreMode:      scoreMode,
		minShouldMatch: minShouldMatch,
		maxDoc:         maxDoc,
		cost:           -1,
	}
}

// computeShouldCost returns the cost of the SHOULD clauses, honouring minShouldMatch.
// Mirrors BooleanScorerSupplier.computeShouldCost().
func (bss *BooleanScorerSupplier) computeShouldCost() int64 {
	optional := bss.subs[SHOULD]
	if len(optional) == 0 {
		return 0
	}
	costs := make([]int64, len(optional))
	for i, s := range optional {
		costs[i] = s.Cost()
	}
	return CostWithMinShouldMatch(costs, len(optional), bss.minShouldMatch)
}

// computeCost computes the true cost and stores it in bss.cost.
// Mirrors BooleanScorerSupplier.computeCost().
func (bss *BooleanScorerSupplier) computeCost() int64 {
	// minimum cost of all required (MUST + FILTER) clauses
	minRequired := int64(math.MaxInt64)
	hasRequired := false
	for _, s := range bss.subs[MUST] {
		if c := s.Cost(); c < minRequired {
			minRequired = c
			hasRequired = true
		}
	}
	for _, s := range bss.subs[FILTER] {
		if c := s.Cost(); c < minRequired {
			minRequired = c
			hasRequired = true
		}
	}

	if hasRequired && bss.minShouldMatch == 0 {
		return minRequired
	}

	shouldCost := bss.computeShouldCost()
	if hasRequired {
		if minRequired < shouldCost {
			return minRequired
		}
		return shouldCost
	}
	return shouldCost
}

// Cost returns an estimate of the number of documents this scorer will match.
func (bss *BooleanScorerSupplier) Cost() int64 {
	if bss.cost == -1 {
		bss.cost = bss.computeCost()
	}
	return bss.cost
}

// Get returns a Scorer for the given leadCost.
// Mirrors BooleanScorerSupplier.get() + getInternal().
func (bss *BooleanScorerSupplier) Get(leadCost int64) (Scorer, error) {
	// Clamp leadCost to our own cost.
	if c := bss.Cost(); leadCost > c {
		leadCost = c
	}

	// Collect scorers by occur type from the sub-suppliers.
	mustScorers := make([]Scorer, 0)
	filterScorers := make([]Scorer, 0)
	shouldScorers := make([]Scorer, 0)
	mustNotScorers := make([]Scorer, 0)

	for _, s := range bss.subs[MUST] {
		scorer, err := s.Get(leadCost)
		if err != nil {
			return nil, err
		}
		if scorer != nil {
			mustScorers = append(mustScorers, scorer)
		}
	}
	for _, s := range bss.subs[FILTER] {
		scorer, err := s.Get(leadCost)
		if err != nil {
			return nil, err
		}
		if scorer != nil {
			filterScorers = append(filterScorers, scorer)
		}
	}
	for _, s := range bss.subs[SHOULD] {
		scorer, err := s.Get(leadCost)
		if err != nil {
			return nil, err
		}
		if scorer != nil {
			shouldScorers = append(shouldScorers, scorer)
		}
	}
	for _, s := range bss.subs[MUST_NOT] {
		scorer, err := s.Get(leadCost)
		if err != nil {
			return nil, err
		}
		if scorer != nil {
			mustNotScorers = append(mustNotScorers, scorer)
		}
	}

	scorer := NewBooleanScorerWithClauses(
		mustScorers, filterScorers, shouldScorers, mustNotScorers,
		bss.scoreMode, bss.minShouldMatch,
	)

	return scorer, nil
}

// BulkScorer returns a BulkScorer for this boolean query.
func (bss *BooleanScorerSupplier) BulkScorer() (BulkScorer, error) {
	scorer, err := bss.Get(math.MaxInt64)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// SetTopLevelScoringClause marks this as a top-level scoring clause.
// Mirrors BooleanScorerSupplier.setTopLevelScoringClause().
func (bss *BooleanScorerSupplier) SetTopLevelScoringClause() {
	bss.topLevel = true
	// Propagate if there is a single scoring clause.
	if len(bss.subs[SHOULD])+len(bss.subs[MUST]) == 1 {
		for _, clause := range bss.subs[SHOULD] {
			clause.SetTopLevelScoringClause()
		}
		for _, clause := range bss.subs[MUST] {
			clause.SetTopLevelScoringClause()
		}
	}
}

// String returns a string representation.
func (bss *BooleanScorerSupplier) String() string {
	return fmt.Sprintf("BooleanScorerSupplier(cost=%d)", bss.Cost())
}
