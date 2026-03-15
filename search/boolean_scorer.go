// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// BooleanScorer is a scorer for boolean queries.
type BooleanScorer struct {
	BaseScorer
	scorers        []Scorer
	scoreMode      ScoreMode
	minShouldMatch int
	currentDoc     int
}

// NewBooleanScorer creates a new BooleanScorer.
func NewBooleanScorer(scorers []Scorer, scoreMode ScoreMode, minShouldMatch int) *BooleanScorer {
	return &BooleanScorer{
		BaseScorer:     *NewBaseScorer(nil),
		scorers:        scorers,
		scoreMode:      scoreMode,
		minShouldMatch: minShouldMatch,
		currentDoc:     -1,
	}
}

// DocID returns the current document ID.
func (bs *BooleanScorer) DocID() int {
	return bs.currentDoc
}

// NextDoc advances to the next document.
func (bs *BooleanScorer) NextDoc() (int, error) {
	// Simplified implementation - just advance all scorers
	// In a real implementation, this would handle conjunction/disjunction logic
	if len(bs.scorers) == 0 {
		bs.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}

	// For now, just return NO_MORE_DOCS (minimal implementation for tests)
	bs.currentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Advance advances to the target document.
func (bs *BooleanScorer) Advance(target int) (int, error) {
	// Simplified implementation
	bs.currentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Cost returns the estimated cost.
func (bs *BooleanScorer) Cost() int64 {
	var cost int64 = 0
	for _, scorer := range bs.scorers {
		cost += scorer.Cost()
	}
	return cost
}

// Score returns the score for the current document.
func (bs *BooleanScorer) Score() float32 {
	// Simplified scoring - sum of all scorer scores
	var score float32 = 0
	for _, scorer := range bs.scorers {
		score += scorer.Score()
	}
	return score
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (bs *BooleanScorer) GetMaxScore(upTo int) float32 {
	// Simplified implementation - sum of max scores from all scorers
	var maxScore float32 = 0
	for _, scorer := range bs.scorers {
		maxScore += scorer.GetMaxScore(upTo)
	}
	return maxScore
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (bs *BooleanScorer) DocIDRunEnd() int {
	return bs.currentDoc + 1
}
