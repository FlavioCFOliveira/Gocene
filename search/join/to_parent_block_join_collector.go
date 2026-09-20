// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ToParentBlockJoinCollector collects parent documents that have matching children.
// This is the Go port of Lucene's org.apache.lucene.search.join.ToParentBlockJoinCollector.
type ToParentBlockJoinCollector struct {
	// childQuery is the query to match child documents
	childQuery search.Query

	// parentFilter identifies parent documents
	parentFilter BitSetProducer

	// collectedParents tracks collected parent document IDs
	collectedParents []int

	// parentScores tracks scores for collected parents
	parentScores []float32

	// totalHits is the total number of parent hits
	totalHits int

	// scoreMode determines how child scores are combined into parent scores
	scoreMode ScoreMode

	// currentChildHits tracks child hits for the current parent block
	currentChildHits []childHit

	// currentParentDoc is the current parent being processed
	currentParentDoc int
}

// childHit represents a hit on a child document.
type childHit struct {
	doc   int
	score float32
}

// NewToParentBlockJoinCollector creates a new ToParentBlockJoinCollector.
func NewToParentBlockJoinCollector(childQuery search.Query, parentFilter BitSetProducer, scoreMode ScoreMode) *ToParentBlockJoinCollector {
	return &ToParentBlockJoinCollector{
		childQuery:       childQuery,
		parentFilter:     parentFilter,
		collectedParents: make([]int, 0),
		parentScores:     make([]float32, 0),
		scoreMode:        scoreMode,
		currentChildHits: make([]childHit, 0),
		currentParentDoc: -1,
	}
}

// Collect collects a child document hit.
func (c *ToParentBlockJoinCollector) Collect(doc int, score float32) error {
	// Find the parent for this child
	parentDoc := c.findParent(doc)
	if parentDoc == -1 {
		return fmt.Errorf("no parent found for child document %d", doc)
	}

	// If this is a new parent, process the previous block
	if c.currentParentDoc != -1 && c.currentParentDoc != parentDoc {
		c.processCurrentBlock()
	}

	c.currentParentDoc = parentDoc
	c.currentChildHits = append(c.currentChildHits, childHit{doc: doc, score: score})

	return nil
}

// CollectDoc collects a document without score.
func (c *ToParentBlockJoinCollector) CollectDoc(doc int) error {
	return c.Collect(doc, 0)
}

// findParent finds the parent document for a given child document.
func (c *ToParentBlockJoinCollector) findParent(childDoc int) int {
	// Search forward from the child document to find the next parent
	// In a block-joined index, parents are at block boundaries
	// This is a simplified implementation
	return childDoc + 1 // Placeholder
}

// processCurrentBlock processes the current block of child hits.
func (c *ToParentBlockJoinCollector) processCurrentBlock() {
	if c.currentParentDoc == -1 || len(c.currentChildHits) == 0 {
		return
	}

	// Compute the parent score based on child scores
	parentScore := c.computeParentScore(c.currentChildHits)

	// Add the parent to collected documents
	c.collectedParents = append(c.collectedParents, c.currentParentDoc)
	c.parentScores = append(c.parentScores, parentScore)
	c.totalHits++

	// Reset for next block
	c.currentChildHits = c.currentChildHits[:0]
}

// computeParentScore computes the parent score from child scores.
func (c *ToParentBlockJoinCollector) computeParentScore(childHits []childHit) float32 {
	if len(childHits) == 0 {
		return 0
	}

	switch c.scoreMode {
	case None:
		return 0
	case Avg:
		var sum float32
		for _, hit := range childHits {
			sum += hit.score
		}
		return sum / float32(len(childHits))
	case Max:
		max := float32(0)
		for _, hit := range childHits {
			if hit.score > max {
				max = hit.score
			}
		}
		return max
	case Total:
		var sum float32
		for _, hit := range childHits {
			sum += hit.score
		}
		return sum
	case Min:
		if len(childHits) == 0 {
			return 0
		}
		min := childHits[0].score
		for _, hit := range childHits[1:] {
			if hit.score < min {
				min = hit.score
			}
		}
		return min
	default:
		return 0
	}
}

// Finish finishes collecting and processes any remaining hits.
func (c *ToParentBlockJoinCollector) Finish() {
	c.processCurrentBlock()
}

// GetTotalHits returns the total number of parent hits.
func (c *ToParentBlockJoinCollector) GetTotalHits() int {
	return c.totalHits
}

// GetCollectedParents returns the collected parent document IDs.
func (c *ToParentBlockJoinCollector) GetCollectedParents() []int {
	return c.collectedParents
}

// GetParentScores returns the scores for collected parents.
func (c *ToParentBlockJoinCollector) GetParentScores() []float32 {
	return c.parentScores
}

// GetTopDocs returns the top N parent documents.
func (c *ToParentBlockJoinCollector) GetTopDocs(n int) []ParentDoc {
	if n <= 0 || n > len(c.collectedParents) {
		n = len(c.collectedParents)
	}

	docs := make([]ParentDoc, n)
	for i := 0; i < n; i++ {
		docs[i] = ParentDoc{
			Doc:   c.collectedParents[i],
			Score: c.parentScores[i],
		}
	}
	return docs
}

// Reset resets the collector for reuse.
func (c *ToParentBlockJoinCollector) Reset() {
	c.collectedParents = c.collectedParents[:0]
	c.parentScores = c.parentScores[:0]
	c.currentChildHits = c.currentChildHits[:0]
	c.currentParentDoc = -1
	c.totalHits = 0
}

// ParentDoc represents a parent document with its score.
type ParentDoc struct {
	Doc   int
	Score float32
}

// ToParentBlockJoinCollectorManager manages ToParentBlockJoinCollector instances.
type ToParentBlockJoinCollectorManager struct {
	childQuery   search.Query
	parentFilter BitSetProducer
	scoreMode    ScoreMode
}

// NewToParentBlockJoinCollectorManager creates a new manager.
func NewToParentBlockJoinCollectorManager(childQuery search.Query, parentFilter BitSetProducer, scoreMode ScoreMode) *ToParentBlockJoinCollectorManager {
	return &ToParentBlockJoinCollectorManager{
		childQuery:   childQuery,
		parentFilter: parentFilter,
		scoreMode:    scoreMode,
	}
}

// NewCollector creates a new collector for the given context.
func (m *ToParentBlockJoinCollectorManager) NewCollector(context *index.LeafReaderContext) (*ToParentBlockJoinCollector, error) {
	return NewToParentBlockJoinCollector(m.childQuery, m.parentFilter, m.scoreMode), nil
}
