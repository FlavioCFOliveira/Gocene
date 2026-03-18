// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BlockJoinScorer is a scorer for block join queries.
// It iterates over parent documents and scores them based on matching child documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.BlockJoinScorer.
type BlockJoinScorer struct {
	// childScorer is the scorer for child documents
	childScorer search.Scorer

	// parentScorer is the scorer for parent documents
	parentScorer search.Scorer

	// scoreMode determines how child scores are combined
	scoreMode ScoreMode

	// currentParentDoc is the current parent document ID
	currentParentDoc int

	// currentChildDoc is the current child document ID
	currentChildDoc int

	// accumulatedScore is used for accumulating scores across children
	accumulatedScore float32

	// childCount is the number of matching children for the current parent
	childCount int
}

// NewBlockJoinScorer creates a new BlockJoinScorer.
// Parameters:
//   - childScorer: the scorer for child documents
//   - parentScorer: the scorer for parent documents
//   - scoreMode: how to combine scores from child documents
func NewBlockJoinScorer(childScorer search.Scorer, parentScorer search.Scorer, scoreMode ScoreMode) *BlockJoinScorer {
	return &BlockJoinScorer{
		childScorer:      childScorer,
		parentScorer:     parentScorer,
		scoreMode:        scoreMode,
		currentParentDoc: -1,
		currentChildDoc:  -1,
		accumulatedScore: 0,
		childCount:       0,
	}
}

// GetChildScorer returns the child scorer.
func (s *BlockJoinScorer) GetChildScorer() search.Scorer {
	return s.childScorer
}

// GetParentScorer returns the parent scorer.
func (s *BlockJoinScorer) GetParentScorer() search.Scorer {
	return s.parentScorer
}

// GetScoreMode returns the score mode.
func (s *BlockJoinScorer) GetScoreMode() ScoreMode {
	return s.scoreMode
}

// NextDoc advances to the next document.
func (s *BlockJoinScorer) NextDoc() (int, error) {
	// Advance to the next parent document
	parentDoc, err := s.parentScorer.NextDoc()
	if err != nil {
		return 0, err
	}

	if parentDoc == search.NO_MORE_DOCS {
		s.currentParentDoc = search.NO_MORE_DOCS
		return search.NO_MORE_DOCS, nil
	}

	s.currentParentDoc = parentDoc

	// Reset accumulated score for the new parent
	s.resetAccumulatedScore()

	// Collect scores from all matching children up to this parent
	err = s.collectChildScores(parentDoc)
	if err != nil {
		return 0, err
	}

	return parentDoc, nil
}

// DocID returns the current document ID.
func (s *BlockJoinScorer) DocID() int {
	return s.currentParentDoc
}

// Score returns the score of the current document.
func (s *BlockJoinScorer) Score() float32 {
	if s.scoreMode == None {
		return s.parentScorer.Score()
	}

	if s.childCount == 0 {
		return 0
	}

	switch s.scoreMode {
	case Avg:
		return s.accumulatedScore / float32(s.childCount)
	case Max:
		return s.accumulatedScore
	case Min:
		return s.accumulatedScore
	case Total:
		return s.accumulatedScore
	default:
		return s.parentScorer.Score()
	}
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *BlockJoinScorer) GetMaxScore(upTo int) float32 {
	// For block join, we return the max score from the parent scorer
	// In a full implementation, this would consider child scores as well
	return s.parentScorer.GetMaxScore(upTo)
}

// Advance advances to the given document.
func (s *BlockJoinScorer) Advance(target int) (int, error) {
	// Advance parent to the target
	parentDoc, err := s.parentScorer.Advance(target)
	if err != nil {
		return 0, err
	}

	if parentDoc == search.NO_MORE_DOCS {
		s.currentParentDoc = search.NO_MORE_DOCS
		return search.NO_MORE_DOCS, nil
	}

	s.currentParentDoc = parentDoc

	// Reset accumulated score
	s.resetAccumulatedScore()

	// Collect scores from matching children
	err = s.collectChildScores(parentDoc)
	if err != nil {
		return 0, err
	}

	return parentDoc, nil
}

// Cost returns the estimated cost of this scorer.
func (s *BlockJoinScorer) Cost() int64 {
	// Return the cost of the parent scorer
	// In a full implementation, this might consider child costs as well
	return s.parentScorer.Cost()
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs.
func (s *BlockJoinScorer) DocIDRunEnd() int {
	// Delegate to parent scorer
	return s.parentScorer.DocIDRunEnd()
}

// GetChildren returns the child scorer.
func (s *BlockJoinScorer) GetChildren() search.Scorer {
	return s.childScorer
}

// GetChildCount returns the number of matching children for the current parent.
func (s *BlockJoinScorer) GetChildCount() int {
	return s.childCount
}

// resetAccumulatedScore resets the accumulated score and child count.
func (s *BlockJoinScorer) resetAccumulatedScore() {
	s.accumulatedScore = 0
	s.childCount = 0
}

// collectChildScores collects scores from all matching children up to the given parent.
func (s *BlockJoinScorer) collectChildScores(parentDoc int) error {
	// Advance child scorer to collect matching children
	// In a real implementation, this would iterate through children
	// that belong to the current parent block

	// For now, use a simplified approach
	if s.currentChildDoc == -1 {
		// Initialize child scorer
		childDoc, err := s.childScorer.NextDoc()
		if err != nil {
			return err
		}
		s.currentChildDoc = childDoc
	}

	// Collect scores from children that are before the parent
	for s.currentChildDoc != search.NO_MORE_DOCS && s.currentChildDoc < parentDoc {
		childScore := s.childScorer.Score()

		switch s.scoreMode {
		case Avg, Total:
			s.accumulatedScore += childScore
		case Max:
			if childScore > s.accumulatedScore {
				s.accumulatedScore = childScore
			}
		case Min:
			if s.childCount == 0 || childScore < s.accumulatedScore {
				s.accumulatedScore = childScore
			}
		}

		s.childCount++

		// Advance to next child
		childDoc, err := s.childScorer.NextDoc()
		if err != nil {
			return err
		}
		s.currentChildDoc = childDoc
	}

	return nil
}

// Ensure BlockJoinScorer implements Scorer
var _ search.Scorer = (*BlockJoinScorer)(nil)
