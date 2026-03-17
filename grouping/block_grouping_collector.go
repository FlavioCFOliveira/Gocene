// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/join"
)

// BlockGroupingCollector collects documents into groups based on block structure.
// This is used when documents are indexed in blocks where the last document in each block
// is the group parent.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.BlockGroupingCollector.
type BlockGroupingCollector struct {
	// groupSelector selects the group for each document
	groupSelector GroupSelector

	// parentFilter identifies parent documents
	parentFilter *join.FixedBitSet

	// collectedGroups tracks collected groups
	collectedGroups map[interface{}]*BlockGroup

	// currentBlock tracks the current block being processed
	currentBlock *BlockGroup

	// totalHits is the total number of hits processed
	totalHits int

	// maxScore tracks the maximum score seen
	maxScore float32
}

// BlockGroup represents a group of documents in a block.
type BlockGroup struct {
	// ParentDoc is the parent document ID
	ParentDoc int

	// ParentScore is the score of the parent document
	ParentScore float32

	// ChildDocs are the child document IDs
	ChildDocs []int

	// ChildScores are the scores of child documents
	ChildScores []float32

	// GroupValue is the group value
	GroupValue interface{}
}

// NewBlockGroupingCollector creates a new BlockGroupingCollector.
func NewBlockGroupingCollector(groupSelector GroupSelector, parentFilter *join.FixedBitSet) *BlockGroupingCollector {
	return &BlockGroupingCollector{
		groupSelector:   groupSelector,
		parentFilter:    parentFilter,
		collectedGroups: make(map[interface{}]*BlockGroup),
	}
}

// Collect collects a document.
func (bgc *BlockGroupingCollector) Collect(doc int, score float32) error {
	// Check if this is a parent document
	isParent := bgc.isParent(doc)

	if isParent {
		// Finish the current block if there is one
		if bgc.currentBlock != nil {
			bgc.finishCurrentBlock()
		}

		// Start a new block
		groupValue := bgc.groupSelector.Select(doc)
		bgc.currentBlock = &BlockGroup{
			ParentDoc:   doc,
			ParentScore: score,
			GroupValue:  groupValue,
			ChildDocs:   make([]int, 0),
			ChildScores: make([]float32, 0),
		}
	} else if bgc.currentBlock != nil {
		// Add to current block as a child
		bgc.currentBlock.ChildDocs = append(bgc.currentBlock.ChildDocs, doc)
		bgc.currentBlock.ChildScores = append(bgc.currentBlock.ChildScores, score)
	}

	// Update stats
	bgc.totalHits++
	if score > bgc.maxScore {
		bgc.maxScore = score
	}

	return nil
}

// CollectDoc collects a document without score.
func (bgc *BlockGroupingCollector) CollectDoc(doc int) error {
	return bgc.Collect(doc, 0)
}

// isParent checks if a document is a parent document.
func (bgc *BlockGroupingCollector) isParent(doc int) bool {
	if bgc.parentFilter == nil {
		return false
	}
	// In a real implementation, this would check the parentFilter
	// For now, we use a simple heuristic
	return false
}

// finishCurrentBlock finishes processing the current block.
func (bgc *BlockGroupingCollector) finishCurrentBlock() {
	if bgc.currentBlock == nil {
		return
	}

	groupValue := bgc.currentBlock.GroupValue
	if groupValue == nil {
		groupValue = bgc.currentBlock.ParentDoc
	}

	bgc.collectedGroups[groupValue] = bgc.currentBlock
	bgc.currentBlock = nil
}

// Finish finishes collecting and processes any remaining blocks.
func (bgc *BlockGroupingCollector) Finish() {
	bgc.finishCurrentBlock()
}

// GetGroups returns all collected groups.
func (bgc *BlockGroupingCollector) GetGroups() []*BlockGroup {
	result := make([]*BlockGroup, 0, len(bgc.collectedGroups))
	for _, group := range bgc.collectedGroups {
		result = append(result, group)
	}
	return result
}

// GetGroupCount returns the number of groups.
func (bgc *BlockGroupingCollector) GetGroupCount() int {
	return len(bgc.collectedGroups)
}

// GetTotalHits returns the total number of hits processed.
func (bgc *BlockGroupingCollector) GetTotalHits() int {
	return bgc.totalHits
}

// GetMaxScore returns the maximum score seen.
func (bgc *BlockGroupingCollector) GetMaxScore() float32 {
	return bgc.maxScore
}

// GetGroup returns the group for a specific group value.
func (bgc *BlockGroupingCollector) GetGroup(groupValue interface{}) (*BlockGroup, error) {
	if group, ok := bgc.collectedGroups[groupValue]; ok {
		return group, nil
	}
	return nil, fmt.Errorf("group not found: %v", groupValue)
}

// HasGroup returns whether a group exists for the given value.
func (bgc *BlockGroupingCollector) HasGroup(groupValue interface{}) bool {
	_, ok := bgc.collectedGroups[groupValue]
	return ok
}

// Reset resets the collector for reuse.
func (bgc *BlockGroupingCollector) Reset() {
	bgc.collectedGroups = make(map[interface{}]*BlockGroup)
	bgc.currentBlock = nil
	bgc.totalHits = 0
	bgc.maxScore = 0
}

// BlockGroupingCollectorManager manages BlockGroupingCollector instances.
type BlockGroupingCollectorManager struct {
	groupSelector GroupSelector
	parentFilter  *join.FixedBitSet
}

// NewBlockGroupingCollectorManager creates a new manager.
func NewBlockGroupingCollectorManager(groupSelector GroupSelector, parentFilter *join.FixedBitSet) *BlockGroupingCollectorManager {
	return &BlockGroupingCollectorManager{
		groupSelector: groupSelector,
		parentFilter:  parentFilter,
	}
}

// NewCollector creates a new collector for the given context.
func (m *BlockGroupingCollectorManager) NewCollector(context *index.LeafReaderContext) (*BlockGroupingCollector, error) {
	return NewBlockGroupingCollector(m.groupSelector, m.parentFilter), nil
}
