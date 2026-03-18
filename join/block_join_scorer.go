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

// ToChildBlockJoinScorer is a scorer for ToChildBlockJoinQuery.
// It matches child documents whose parent documents match the parent query.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.ToChildBlockJoinScorer.
type ToChildBlockJoinScorer struct {
	// weight is the parent weight
	weight *ToChildBlockJoinWeight

	// parentScorer is the scorer for parent documents
	parentScorer search.Scorer

	// parentsBits is the bitset identifying parent documents
	parentsBits *FixedBitSet

	// scoreMode determines how parent scores are propagated
	scoreMode ScoreMode

	// boost is the query boost
	boost float32

	// currentChildDoc is the current child document ID
	currentChildDoc int

	// currentParentDoc is the current parent document ID
	currentParentDoc int

	// parentScore is the score of the current parent
	parentScore float32
}

// NewToChildBlockJoinScorer creates a new ToChildBlockJoinScorer.
func NewToChildBlockJoinScorer(weight *ToChildBlockJoinWeight, parentScorer search.Scorer, parentsBits *FixedBitSet, scoreMode ScoreMode, boost float32) *ToChildBlockJoinScorer {
	return &ToChildBlockJoinScorer{
		weight:           weight,
		parentScorer:     parentScorer,
		parentsBits:      parentsBits,
		scoreMode:        scoreMode,
		boost:            boost,
		currentChildDoc:  -1,
		currentParentDoc: -1,
		parentScore:      0,
	}
}

// DocID returns the current document ID.
func (s *ToChildBlockJoinScorer) DocID() int {
	return s.currentChildDoc
}

// NextDoc advances to the next document.
func (s *ToChildBlockJoinScorer) NextDoc() (int, error) {
	for {
		// Advance parent scorer to next parent
		parentDoc, err := s.parentScorer.NextDoc()
		if err != nil {
			return 0, err
		}

		if parentDoc == search.NO_MORE_DOCS {
			s.currentChildDoc = search.NO_MORE_DOCS
			return search.NO_MORE_DOCS, nil
		}

		s.currentParentDoc = parentDoc
		s.parentScore = s.parentScorer.Score()

		// Find the previous parent to determine the start of this child block
		startDoc := s.findPreviousParent(parentDoc) + 1

		// Return the first child in this block
		if startDoc < parentDoc {
			s.currentChildDoc = startDoc
			return startDoc, nil
		}
		// If no children (parent follows parent), continue to next parent
	}
}

// findPreviousParent finds the document ID of the previous parent before the given doc.
func (s *ToChildBlockJoinScorer) findPreviousParent(doc int) int {
	// Search backwards to find the previous set bit
	for i := doc - 1; i >= 0; i-- {
		if s.parentsBits.Get(i) {
			return i
		}
	}
	return -1
}

// Score returns the score of the current document.
func (s *ToChildBlockJoinScorer) Score() float32 {
	if s.scoreMode == None {
		return s.boost
	}
	return s.parentScore * s.boost
}

// Advance advances to the given document.
func (s *ToChildBlockJoinScorer) Advance(target int) (int, error) {
	// Advance parent scorer to find which block contains the target
	if target > s.currentParentDoc {
		parentDoc, err := s.parentScorer.Advance(target)
		if err != nil {
			return 0, err
		}

		if parentDoc == search.NO_MORE_DOCS {
			s.currentChildDoc = search.NO_MORE_DOCS
			return search.NO_MORE_DOCS, nil
		}

		s.currentParentDoc = parentDoc
		s.parentScore = s.parentScorer.Score()

		// Find the start of this child block
		startDoc := s.findPreviousParent(parentDoc) + 1

		if startDoc <= target && target < parentDoc {
			s.currentChildDoc = target
			return target, nil
		}

		if startDoc < parentDoc {
			s.currentChildDoc = startDoc
			return startDoc, nil
		}
	}

	// Otherwise, use NextDoc
	return s.NextDoc()
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *ToChildBlockJoinScorer) GetMaxScore(upTo int) float32 {
	// Return the max score from parent scorer
	return s.parentScorer.GetMaxScore(upTo) * s.boost
}

// Cost returns the estimated cost of this scorer.
func (s *ToChildBlockJoinScorer) Cost() int64 {
	return s.parentScorer.Cost()
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs.
func (s *ToChildBlockJoinScorer) DocIDRunEnd() int {
	// Return the parent doc as the end of the run
	return s.currentParentDoc
}

// GetChildren returns child scorers.
func (s *ToChildBlockJoinScorer) GetChildren() search.Scorer {
	return s.parentScorer
}

// Ensure ToChildBlockJoinScorer implements Scorer
var _ search.Scorer = (*ToChildBlockJoinScorer)(nil)

// ToParentBlockJoinScorer is a scorer for ToParentBlockJoinQuery.
// It matches parent documents that have children matching the child query.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.ToParentBlockJoinScorer.
type ToParentBlockJoinScorer struct {
	// weight is the parent weight
	weight *ToParentBlockJoinWeight

	// childScorer is the scorer for child documents
	childScorer search.Scorer

	// parentsBits is the bitset identifying parent documents
	parentsBits *FixedBitSet

	// scoreMode determines how child scores are combined
	scoreMode ScoreMode

	// boost is the query boost
	boost float32

	// currentParentDoc is the current parent document ID
	currentParentDoc int

	// accumulatedScore is the combined score of matching children
	accumulatedScore float32

	// childCount is the number of matching children for the current parent
	childCount int

	// currentChildDoc is the current child document ID
	currentChildDoc int
}

// NewToParentBlockJoinScorer creates a new ToParentBlockJoinScorer.
func NewToParentBlockJoinScorer(weight *ToParentBlockJoinWeight, childScorer search.Scorer, parentsBits *FixedBitSet, scoreMode ScoreMode, boost float32) *ToParentBlockJoinScorer {
	return &ToParentBlockJoinScorer{
		weight:           weight,
		childScorer:      childScorer,
		parentsBits:      parentsBits,
		scoreMode:        scoreMode,
		boost:            boost,
		currentParentDoc: -1,
		accumulatedScore: 0,
		childCount:       0,
		currentChildDoc:  -1,
	}
}

// DocID returns the current document ID.
func (s *ToParentBlockJoinScorer) DocID() int {
	return s.currentParentDoc
}

// NextDoc advances to the next document.
func (s *ToParentBlockJoinScorer) NextDoc() (int, error) {
	for {
		// Advance child scorer to find next matching child
		childDoc, err := s.childScorer.NextDoc()
		if err != nil {
			return 0, err
		}

		if childDoc == search.NO_MORE_DOCS {
			s.currentParentDoc = search.NO_MORE_DOCS
			return search.NO_MORE_DOCS, nil
		}

		s.currentChildDoc = childDoc

		// Find the parent of this child
		parentDoc := s.findParent(childDoc)
		if parentDoc < 0 {
			// No parent found, continue to next child
			continue
		}

		// If we found a new parent, process it
		if parentDoc != s.currentParentDoc {
			// Reset accumulated score for new parent
			s.resetAccumulatedScore()
			s.currentParentDoc = parentDoc
		}

		// Accumulate score from this child
		childScore := s.childScorer.Score()
		s.accumulateScore(childScore)

		return parentDoc, nil
	}
}

// findParent finds the parent document ID for the given child document.
func (s *ToParentBlockJoinScorer) findParent(childDoc int) int {
	// Search forward to find the next parent
	for i := childDoc; i < s.parentsBits.Size(); i++ {
		if s.parentsBits.Get(i) {
			return i
		}
	}
	return -1
}

// resetAccumulatedScore resets the accumulated score and child count.
func (s *ToParentBlockJoinScorer) resetAccumulatedScore() {
	s.accumulatedScore = 0
	s.childCount = 0
}

// accumulateScore adds a child score to the accumulated score.
func (s *ToParentBlockJoinScorer) accumulateScore(score float32) {
	switch s.scoreMode {
	case Avg, Total:
		s.accumulatedScore += score
	case Max:
		if score > s.accumulatedScore {
			s.accumulatedScore = score
		}
	case Min:
		if s.childCount == 0 || score < s.accumulatedScore {
			s.accumulatedScore = score
		}
	}
	s.childCount++
}

// Score returns the score of the current document.
func (s *ToParentBlockJoinScorer) Score() float32 {
	if s.scoreMode == None || s.childCount == 0 {
		return s.boost
	}

	switch s.scoreMode {
	case Avg:
		if s.childCount > 0 {
			return (s.accumulatedScore / float32(s.childCount)) * s.boost
		}
		return s.boost
	case Max, Min, Total:
		return s.accumulatedScore * s.boost
	default:
		return s.boost
	}
}

// Advance advances to the given document.
func (s *ToParentBlockJoinScorer) Advance(target int) (int, error) {
	// If the target is a parent, we need to accumulate all child scores up to it
	if target > s.currentParentDoc {
		// Reset accumulated score for potential new parent
		s.resetAccumulatedScore()

		// Advance child scorer to collect scores
		for {
			childDoc, err := s.childScorer.Advance(target)
			if err != nil {
				return 0, err
			}

			if childDoc == search.NO_MORE_DOCS {
				s.currentParentDoc = search.NO_MORE_DOCS
				return search.NO_MORE_DOCS, nil
			}

			s.currentChildDoc = childDoc

			// Find the parent of this child
			parentDoc := s.findParent(childDoc)
			if parentDoc < 0 {
				continue
			}

			// If we've passed the target, this parent is our result
			if parentDoc >= target {
				s.currentParentDoc = parentDoc
				childScore := s.childScorer.Score()
				s.accumulateScore(childScore)
				return parentDoc, nil
			}
		}
	}

	// Otherwise, use NextDoc
	return s.NextDoc()
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *ToParentBlockJoinScorer) GetMaxScore(upTo int) float32 {
	// Return the max score from child scorer
	return s.childScorer.GetMaxScore(upTo) * s.boost
}

// Cost returns the estimated cost of this scorer.
func (s *ToParentBlockJoinScorer) Cost() int64 {
	return s.childScorer.Cost()
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs.
func (s *ToParentBlockJoinScorer) DocIDRunEnd() int {
	// Find the next parent to determine the end of the run
	nextParent := s.parentsBits.NextSetBit(s.currentParentDoc + 1)
	if nextParent < 0 {
		return s.parentsBits.Size()
	}
	return nextParent - 1
}

// GetChildren returns child scorers.
func (s *ToParentBlockJoinScorer) GetChildren() search.Scorer {
	return s.childScorer
}

// Ensure ToParentBlockJoinScorer implements Scorer
var _ search.Scorer = (*ToParentBlockJoinScorer)(nil)
