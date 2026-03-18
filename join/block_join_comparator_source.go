// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BlockJoinComparatorSource creates comparators for sorting documents in block join queries.
// It allows sorting by parent document fields while returning child documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.BlockJoinComparatorSource.
type BlockJoinComparatorSource struct {
	// parentComparator is the comparator for parent documents
	parentComparator search.FieldComparator

	// parentsFilter identifies parent documents
	parentsFilter BitSetProducer
}

// NewBlockJoinComparatorSource creates a new BlockJoinComparatorSource.
// Parameters:
//   - parentComparator: the comparator to use for parent documents
//   - parentsFilter: the BitSetProducer identifying parent documents
func NewBlockJoinComparatorSource(parentComparator search.FieldComparator, parentsFilter BitSetProducer) *BlockJoinComparatorSource {
	return &BlockJoinComparatorSource{
		parentComparator:   parentComparator,
		parentsFilter:      parentsFilter,
	}
}

// NewComparator creates a new comparator for the given sort field.
func (s *BlockJoinComparatorSource) NewComparator(field *search.SortField, numHits int) search.FieldComparator {
	// Return a block join comparator that wraps the parent comparator
	return NewBlockJoinComparator(s.parentComparator, s.parentsFilter, numHits)
}

// BlockJoinComparator compares documents based on their parent document values.
// This allows sorting child documents by parent document fields.
type BlockJoinComparator struct {
	// parentComparator is the comparator for parent documents
	parentComparator search.FieldComparator

	// parentsFilter identifies parent documents
	parentsFilter BitSetProducer

	// parentDocs caches parent document IDs for slots
	parentDocs []int

	// numHits is the number of hits
	numHits int
}

// NewBlockJoinComparator creates a new BlockJoinComparator.
func NewBlockJoinComparator(parentComparator search.FieldComparator, parentsFilter BitSetProducer, numHits int) *BlockJoinComparator {
	return &BlockJoinComparator{
		parentComparator:   parentComparator,
		parentsFilter:      parentsFilter,
		parentDocs:         make([]int, numHits),
		numHits:            numHits,
	}
}

// Compare compares doc1 and doc2.
// Returns -1 if doc1 < doc2, 0 if equal, 1 if doc1 > doc2.
func (c *BlockJoinComparator) Compare(doc1, doc2 int) int {
	// Get parent documents for both docs
	parent1 := c.getParentDoc(doc1)
	parent2 := c.getParentDoc(doc2)

	// Compare using parent documents
	return c.parentComparator.Compare(parent1, parent2)
}

// SetBottom sets the bottom document for the priority queue.
func (c *BlockJoinComparator) SetBottom(doc int) {
	// Get parent document for this child
	parentDoc := c.getParentDoc(doc)

	// Set bottom in parent comparator
	c.parentComparator.SetBottom(parentDoc)
}

// CompareBottom compares the given doc with the bottom doc.
func (c *BlockJoinComparator) CompareBottom(doc int) int {
	// Get parent document for this child
	parentDoc := c.getParentDoc(doc)

	// Compare with bottom using parent comparator
	return c.parentComparator.CompareBottom(parentDoc)
}

// Copy copies the value from the given doc to the slot.
func (c *BlockJoinComparator) Copy(slot int, doc int) {
	// Store the parent document ID for this slot
	parentDoc := c.getParentDoc(doc)
	c.parentDocs[slot] = parentDoc

	// Copy in parent comparator
	c.parentComparator.Copy(slot, parentDoc)
}

// SetScorer sets the scorer.
func (c *BlockJoinComparator) SetScorer(scorer search.Scorer) {
	c.parentComparator.SetScorer(scorer)
}

// getParentDoc finds the parent document for a given child document.
// In a block-joined index, the parent is the last document in the block.
func (c *BlockJoinComparator) getParentDoc(childDoc int) int {
	// Search forward from childDoc to find the next parent
	// This is a simplified implementation
	// In a full implementation, we would use the parentsFilter BitSetProducer
	// to find the actual parent document

	// For now, assume the parent is at a fixed offset or use a placeholder
	// This should be replaced with proper BitSetProducer logic
	return childDoc + 1 // Placeholder - parent is next document
}

// Ensure BlockJoinComparator implements FieldComparator
var _ search.FieldComparator = (*BlockJoinComparator)(nil)

// Ensure BlockJoinComparatorSource implements FieldComparatorSource
var _ search.FieldComparatorSource = (*BlockJoinComparatorSource)(nil)
