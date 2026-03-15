// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"sort"
)

// BooleanScorerSupplier provides scorers for BooleanQuery.
// It handles cost-based optimization and lead cost propagation.
type BooleanScorerSupplier struct {
	weight         Weight
	subs           map[Occur][]ScorerSupplier
	scoreMode      ScoreMode
	minShouldMatch int
	minScore       float32
}

// NewBooleanScorerSupplier creates a new BooleanScorerSupplier.
func NewBooleanScorerSupplier(weight Weight, subs map[Occur][]ScorerSupplier, scoreMode ScoreMode, minShouldMatch int, minScore float32) *BooleanScorerSupplier {
	return &BooleanScorerSupplier{
		weight:         weight,
		subs:           subs,
		scoreMode:      scoreMode,
		minShouldMatch: minShouldMatch,
		minScore:       minScore,
	}
}

// Cost returns an estimate of the number of documents this scorer will match.
func (bss *BooleanScorerSupplier) Cost() int64 {
	var cost int64 = 0

	// For conjunctions (MUST/FILTER), the cost is the minimum of all clause costs
	mustCost := bss.calculateConjunctionCost()
	if mustCost >= 0 {
		cost = mustCost
	}

	// For disjunctions (SHOULD), add the costs
	shouldCost := bss.calculateDisjunctionCost()
	if shouldCost > 0 {
		if cost == 0 {
			cost = shouldCost
		} else {
			// When we have both MUST and SHOULD, the cost is bounded by the MUST cost
			// but we need to account for the SHOULD contribution
			cost = min(cost, shouldCost)
		}
	}

	return cost
}

// calculateConjunctionCost returns the cost for MUST/FILTER clauses.
// Returns -1 if there are no required clauses.
func (bss *BooleanScorerSupplier) calculateConjunctionCost() int64 {
	requiredClauses := make([]ScorerSupplier, 0)
	requiredClauses = append(requiredClauses, bss.subs[MUST]...)
	requiredClauses = append(requiredClauses, bss.subs[FILTER]...)

	if len(requiredClauses) == 0 {
		return -1
	}

	// For conjunctions, the cost is the minimum cost (most restrictive clause)
	minCost := requiredClauses[0].Cost()
	for _, clause := range requiredClauses[1:] {
		if clause.Cost() < minCost {
			minCost = clause.Cost()
		}
	}

	return minCost
}

// calculateDisjunctionCost returns the cost for SHOULD clauses.
func (bss *BooleanScorerSupplier) calculateDisjunctionCost() int64 {
	shouldClauses := bss.subs[SHOULD]
	if len(shouldClauses) == 0 {
		return 0
	}

	// Sort clauses by cost
	sortedClauses := make([]ScorerSupplier, len(shouldClauses))
	copy(sortedClauses, shouldClauses)
	sort.Slice(sortedClauses, func(i, j int) bool {
		return sortedClauses[i].Cost() < sortedClauses[j].Cost()
	})

	// Calculate cost based on minShouldMatch
	if bss.minShouldMatch > 0 && bss.minShouldMatch <= len(sortedClauses) {
		// Sum of the minShouldMatch least costly clauses
		var cost int64 = 0
		for i := 0; i < bss.minShouldMatch; i++ {
			cost += sortedClauses[i].Cost()
		}
		return cost
	}

	// Sum of all clause costs
	var cost int64 = 0
	for _, clause := range shouldClauses {
		cost += clause.Cost()
	}
	return cost
}

// Get returns a Scorer for the given leadCost.
func (bss *BooleanScorerSupplier) Get(leadCost int64) (Scorer, error) {
	// Calculate the actual lead cost to pass to sub-scorers
	actualLeadCost := bss.calculateLeadCost(leadCost)

	// Collect all scorers
	scorers := make([]Scorer, 0)

	// Get MUST/FILTER scorers
	for _, clause := range bss.subs[MUST] {
		scorer, err := clause.Get(actualLeadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
	}
	for _, clause := range bss.subs[FILTER] {
		scorer, err := clause.Get(actualLeadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
	}

	// Get SHOULD scorers
	for _, clause := range bss.subs[SHOULD] {
		scorer, err := clause.Get(actualLeadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
	}

	// Get MUST_NOT scorers (with same lead cost as MUST clauses)
	for _, clause := range bss.subs[MUST_NOT] {
		scorer, err := clause.Get(actualLeadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
	}

	// Return a composite scorer
	return NewBooleanScorer(scorers, bss.scoreMode, bss.minShouldMatch), nil
}

// calculateLeadCost calculates the lead cost to pass to sub-scorers.
func (bss *BooleanScorerSupplier) calculateLeadCost(requestedLeadCost int64) int64 {
	// If there's a conjunction, use the minimum clause cost
	mustCost := bss.calculateConjunctionCost()
	if mustCost >= 0 {
		return mustCost
	}

	// Otherwise, use the requested lead cost
	return requestedLeadCost
}

// BulkScorer returns a BulkScorer for this boolean query.
func (bss *BooleanScorerSupplier) BulkScorer() (BulkScorer, error) {
	// For bulk scoring, use MaxInt64 as the lead cost
	scorer, err := bss.Get(int64(^uint64(0) >> 1)) // MaxInt64
	if err != nil {
		return nil, err
	}

	return NewDefaultBulkScorer(scorer), nil
}

// SetTopLevelScoringClause marks this as a top-level scoring clause.
func (bss *BooleanScorerSupplier) SetTopLevelScoringClause() {
	// Propagate to sub-scorers based on the query structure
	// For disjunctions and conjunctions with multiple scoring clauses,
	// we don't mark individual clauses as top-level.

	mustCount := len(bss.subs[MUST])
	shouldCount := len(bss.subs[SHOULD])
	filterCount := len(bss.subs[FILTER])

	// Single MUST clause with only FILTER clauses -> mark MUST as top-level
	if mustCount == 1 && shouldCount == 0 {
		for _, clause := range bss.subs[MUST] {
			clause.SetTopLevelScoringClause()
		}
		return
	}

	// Single SHOULD clause with only MUST_NOT clauses -> mark SHOULD as top-level
	if shouldCount == 1 && mustCount == 0 && filterCount == 0 {
		for _, clause := range bss.subs[SHOULD] {
			clause.SetTopLevelScoringClause()
		}
		return
	}

	// For other cases (disjunctions, conjunctions with multiple clauses),
	// we don't mark individual clauses as top-level
}

// String returns a string representation.
func (bss *BooleanScorerSupplier) String() string {
	return fmt.Sprintf("BooleanScorerSupplier(cost=%d)", bss.Cost())
}

// min returns the minimum of two int64 values.
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
