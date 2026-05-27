// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
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
		parentComparator: parentComparator,
		parentsFilter:    parentsFilter,
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

	// parentsBits is the BitSet of parent documents in the current leaf.
	// It is populated lazily on SetContext and is used by getParentDoc to
	// resolve the parent of a child document via NextSetBit.
	parentsBits *FixedBitSet
}

// NewBlockJoinComparator creates a new BlockJoinComparator.
func NewBlockJoinComparator(parentComparator search.FieldComparator, parentsFilter BitSetProducer, numHits int) *BlockJoinComparator {
	return &BlockJoinComparator{
		parentComparator: parentComparator,
		parentsFilter:    parentsFilter,
		parentDocs:       make([]int, numHits),
		numHits:          numHits,
	}
}

// SetContext resolves the parents BitSet for the given leaf reader context.
// It must be called before Compare/Copy/CompareBottom when the comparator
// transitions to a new leaf reader, mirroring Lucene's getLeafComparator hook.
func (c *BlockJoinComparator) SetContext(ctx *index.LeafReaderContext) error {
	if c.parentsFilter == nil || ctx == nil {
		c.parentsBits = nil
		return nil
	}
	bits, err := c.parentsFilter.GetBitSet(ctx)
	if err != nil {
		return err
	}
	c.parentsBits = bits
	return nil
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

// getParentDoc finds the parent document for the given child document.
//
// In a block-joined index, parents are written *after* their children so the
// parent of childDoc is the first set bit at or after childDoc in the
// parentsFilter BitSet. If childDoc is itself a parent it is returned as-is.
// If no parents BitSet has been resolved (SetContext was not called or the
// filter is nil), childDoc is returned unchanged so the comparator degrades
// to plain doc-id comparison rather than producing garbage.
func (c *BlockJoinComparator) getParentDoc(childDoc int) int {
	if c.parentsBits == nil {
		return childDoc
	}
	parent := c.parentsBits.NextSetBit(childDoc)
	if parent < 0 {
		return childDoc
	}
	return parent
}

// Ensure BlockJoinComparator implements FieldComparator
var _ search.FieldComparator = (*BlockJoinComparator)(nil)

// Ensure BlockJoinComparatorSource implements FieldComparatorSource
var _ search.FieldComparatorSource = (*BlockJoinComparatorSource)(nil)
