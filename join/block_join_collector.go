// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// BlockJoinCollector is a collector for block join queries.
// It collects documents while tracking parent-child relationships.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.BlockJoinCollector.
type BlockJoinCollector struct {
	// parentFilter identifies parent documents
	parentFilter *FixedBitSet

	// childFilter identifies child documents
	childFilter *FixedBitSet

	// collectedDocs tracks collected document IDs
	collectedDocs []int

	// collectedScores tracks scores for collected documents
	collectedScores []float32

	// totalHits is the total number of hits
	totalHits int

	// scoreMode determines how scores are tracked
	scoreMode ScoreMode
}

// NewBlockJoinCollector creates a new BlockJoinCollector.
func NewBlockJoinCollector(parentFilter, childFilter *FixedBitSet, scoreMode ScoreMode) *BlockJoinCollector {
	return &BlockJoinCollector{
		parentFilter:    parentFilter,
		childFilter:     childFilter,
		collectedDocs:   make([]int, 0),
		collectedScores: make([]float32, 0),
		scoreMode:       scoreMode,
	}
}

// Collect collects a document.
func (c *BlockJoinCollector) Collect(doc int) error {
	c.collectedDocs = append(c.collectedDocs, doc)
	c.totalHits++
	return nil
}

// CollectWithScore collects a document with its score.
func (c *BlockJoinCollector) CollectWithScore(doc int, score float32) error {
	c.collectedDocs = append(c.collectedDocs, doc)
	c.collectedScores = append(c.collectedScores, score)
	c.totalHits++
	return nil
}

// GetTotalHits returns the total number of hits.
func (c *BlockJoinCollector) GetTotalHits() int {
	return c.totalHits
}

// GetCollectedDocs returns the collected document IDs.
func (c *BlockJoinCollector) GetCollectedDocs() []int {
	return c.collectedDocs
}

// GetCollectedScores returns the collected scores.
func (c *BlockJoinCollector) GetCollectedScores() []float32 {
	return c.collectedScores
}

// IsParent returns true if the given document is a parent document.
func (c *BlockJoinCollector) IsParent(doc int) bool {
	if c.parentFilter == nil {
		return false
	}
	return c.parentFilter.Get(doc)
}

// IsChild returns true if the given document is a child document.
func (c *BlockJoinCollector) IsChild(doc int) bool {
	if c.childFilter == nil {
		return false
	}
	return c.childFilter.Get(doc)
}

// GetParentDoc returns the parent document for a child document.
// Returns -1 if no parent is found.
func (c *BlockJoinCollector) GetParentDoc(childDoc int) int {
	if c.parentFilter == nil {
		return -1
	}

	// Search forward for the next parent document
	for doc := childDoc + 1; doc < c.parentFilter.Size(); doc++ {
		if c.parentFilter.Get(doc) {
			return doc
		}
	}
	return -1
}

// GetChildrenDocs returns all children documents for a parent document.
func (c *BlockJoinCollector) GetChildrenDocs(parentDoc int) []int {
	if c.childFilter == nil {
		return []int{}
	}

	children := make([]int, 0)
	// Search backwards for children before this parent
	for doc := parentDoc - 1; doc >= 0; doc-- {
		if c.childFilter.Get(doc) {
			children = append([]int{doc}, children...)
		} else if c.parentFilter.Get(doc) {
			// Found another parent, stop
			break
		}
	}
	return children
}

// ComputeParentScore computes the score for a parent based on its children.
func (c *BlockJoinCollector) ComputeParentScore(children []int) float32 {
	if len(children) == 0 {
		return 0
	}

	switch c.scoreMode {
	case None:
		return 0
	case Avg:
		var sum float32
		for _, child := range children {
			sum += c.getScoreForDoc(child)
		}
		return sum / float32(len(children))
	case Max:
		max := float32(0)
		for _, child := range children {
			score := c.getScoreForDoc(child)
			if score > max {
				max = score
			}
		}
		return max
	case Total:
		var sum float32
		for _, child := range children {
			sum += c.getScoreForDoc(child)
		}
		return sum
	case Min:
		min := float32(0)
		for i, child := range children {
			score := c.getScoreForDoc(child)
			if i == 0 || score < min {
				min = score
			}
		}
		return min
	default:
		return 0
	}
}

// getScoreForDoc returns the score for a document.
func (c *BlockJoinCollector) getScoreForDoc(doc int) float32 {
	// Find the score in collected scores
	for i, collectedDoc := range c.collectedDocs {
		if collectedDoc == doc && i < len(c.collectedScores) {
			return c.collectedScores[i]
		}
	}
	return 0
}

// Reset resets the collector for reuse.
func (c *BlockJoinCollector) Reset() {
	c.collectedDocs = c.collectedDocs[:0]
	c.collectedScores = c.collectedScores[:0]
	c.totalHits = 0
}

// BlockJoinCollectorManager manages BlockJoinCollector instances.
type BlockJoinCollectorManager struct {
	parentFilter BitSetProducer
	childFilter  BitSetProducer
	scoreMode    ScoreMode
}

// NewBlockJoinCollectorManager creates a new BlockJoinCollectorManager.
func NewBlockJoinCollectorManager(parentFilter, childFilter BitSetProducer, scoreMode ScoreMode) *BlockJoinCollectorManager {
	return &BlockJoinCollectorManager{
		parentFilter: parentFilter,
		childFilter:  childFilter,
		scoreMode:    scoreMode,
	}
}

// NewCollector creates a new BlockJoinCollector for the given context.
func (m *BlockJoinCollectorManager) NewCollector(context *index.LeafReaderContext) (*BlockJoinCollector, error) {
	parentBits, err := m.parentFilter.GetBitSet(context)
	if err != nil {
		return nil, err
	}

	childBits, err := m.childFilter.GetBitSet(context)
	if err != nil {
		return nil, err
	}

	return NewBlockJoinCollector(parentBits, childBits, m.scoreMode), nil
}
