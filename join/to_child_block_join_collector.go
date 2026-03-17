// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ToChildBlockJoinCollector collects child documents for matching parent documents.
// This is the Go port of Lucene's org.apache.lucene.search.join.ToChildBlockJoinCollector.
type ToChildBlockJoinCollector struct {
	// parentQuery is the query to match parent documents
	parentQuery search.Query

	// childFilter identifies child documents
	childFilter BitSetProducer

	// collectedChildren tracks collected child document IDs
	collectedChildren []int

	// childScores tracks scores for collected children
	childScores []float32

	// totalHits is the total number of child hits
	totalHits int

	// scoreMode determines how parent scores are propagated to children
	scoreMode ScoreMode

	// currentParentDoc is the current parent being processed
	currentParentDoc int

	// currentParentScore is the score of the current parent
	currentParentScore float32
}

// NewToChildBlockJoinCollector creates a new ToChildBlockJoinCollector.
func NewToChildBlockJoinCollector(parentQuery search.Query, childFilter BitSetProducer, scoreMode ScoreMode) *ToChildBlockJoinCollector {
	return &ToChildBlockJoinCollector{
		parentQuery:       parentQuery,
		childFilter:     childFilter,
		collectedChildren: make([]int, 0),
		childScores:     make([]float32, 0),
		scoreMode:       scoreMode,
		currentParentDoc: -1,
	}
}

// Collect collects a parent document hit.
func (c *ToChildBlockJoinCollector) Collect(parentDoc int, parentScore float32) error {
	// Collect all children for this parent
	children, err := c.collectChildren(parentDoc, parentScore)
	if err != nil {
		return err
	}

	c.collectedChildren = append(c.collectedChildren, children...)
	c.totalHits += len(children)

	return nil
}

// CollectDoc collects a document without score.
func (c *ToChildBlockJoinCollector) CollectDoc(parentDoc int) error {
	return c.Collect(parentDoc, 0)
}

// collectChildren collects all children for a given parent document.
func (c *ToChildBlockJoinCollector) collectChildren(parentDoc int, parentScore float32) ([]int, error) {
	// Find all children before this parent
	// In a block-joined index, children come before their parent
	var children []int

	// Search backwards from the parent to find all children
	for doc := parentDoc - 1; doc >= 0; doc-- {
		// Check if this is a child document
		// In a real implementation, we would use the childFilter BitSetProducer
		// For now, we assume all documents before a parent are children
		children = append([]int{doc}, children...)

		// Stop if we hit another parent
		// In a real implementation, we would check the parentFilter
		if doc > 0 && c.isParent(doc-1) {
			break
		}
	}

	// Assign scores to children based on parent score and score mode
	for range children {
		childScore := c.computeChildScore(parentScore, len(children))
		c.childScores = append(c.childScores, childScore)
	}

	return children, nil
}

// isParent checks if a document is a parent document.
func (c *ToChildBlockJoinCollector) isParent(doc int) bool {
	// In a real implementation, this would check the parentFilter
	// For now, we use a simple heuristic
	return false
}

// computeChildScore computes the child score from the parent score.
func (c *ToChildBlockJoinCollector) computeChildScore(parentScore float32, numChildren int) float32 {
	switch c.scoreMode {
	case None:
		return 0
	case Avg:
		return parentScore / float32(numChildren)
	case Max:
		return parentScore
	case Total:
		return parentScore / float32(numChildren)
	case Min:
		return parentScore / float32(numChildren)
	default:
		return parentScore / float32(numChildren)
	}
}

// GetTotalHits returns the total number of child hits.
func (c *ToChildBlockJoinCollector) GetTotalHits() int {
	return c.totalHits
}

// GetCollectedChildren returns the collected child document IDs.
func (c *ToChildBlockJoinCollector) GetCollectedChildren() []int {
	return c.collectedChildren
}

// GetChildScores returns the scores for collected children.
func (c *ToChildBlockJoinCollector) GetChildScores() []float32 {
	return c.childScores
}

// GetTopDocs returns the top N child documents.
func (c *ToChildBlockJoinCollector) GetTopDocs(n int) []ChildDoc {
	if n <= 0 || n > len(c.collectedChildren) {
		n = len(c.collectedChildren)
	}

	docs := make([]ChildDoc, n)
	for i := 0; i < n; i++ {
		docs[i] = ChildDoc{
			Doc:   c.collectedChildren[i],
			Score: c.childScores[i],
		}
	}
	return docs
}

// GetChildrenForParent returns all children for a specific parent document.
func (c *ToChildBlockJoinCollector) GetChildrenForParent(parentDoc int) ([]ChildDoc, error) {
	// In a real implementation, we would track which children belong to which parent
	// For now, we return an error
	return nil, fmt.Errorf("parent-child mapping not tracked in this implementation")
}

// Reset resets the collector for reuse.
func (c *ToChildBlockJoinCollector) Reset() {
	c.collectedChildren = c.collectedChildren[:0]
	c.childScores = c.childScores[:0]
	c.currentParentDoc = -1
	c.currentParentScore = 0
	c.totalHits = 0
}

// ChildDoc represents a child document with its score.
type ChildDoc struct {
	Doc   int
	Score float32
}

// ToChildBlockJoinCollectorManager manages ToChildBlockJoinCollector instances.
type ToChildBlockJoinCollectorManager struct {
	parentQuery search.Query
	childFilter BitSetProducer
	scoreMode   ScoreMode
}

// NewToChildBlockJoinCollectorManager creates a new manager.
func NewToChildBlockJoinCollectorManager(parentQuery search.Query, childFilter BitSetProducer, scoreMode ScoreMode) *ToChildBlockJoinCollectorManager {
	return &ToChildBlockJoinCollectorManager{
		parentQuery: parentQuery,
		childFilter: childFilter,
		scoreMode:   scoreMode,
	}
}

// NewCollector creates a new collector for the given context.
func (m *ToChildBlockJoinCollectorManager) NewCollector(context *index.LeafReaderContext) (*ToChildBlockJoinCollector, error) {
	return NewToChildBlockJoinCollector(m.parentQuery, m.childFilter, m.scoreMode), nil
}
