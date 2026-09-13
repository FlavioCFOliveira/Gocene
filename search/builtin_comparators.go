// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/FieldComparator.java
//     (RelevanceComparator)
//   lucene/core/src/java/org/apache/lucene/search/comparators/DocComparator.java
//
// These are the two sort keys that do not read DocValues: SCORE (relevance) and
// DOC (document id). They let a Sort mix score/doc keys with DocValues keys —
// most commonly as a secondary tie-break — without the DocValues comparator
// factory rejecting the field.

// relevanceComparator sorts by relevance score. As in Lucene, compare is
// reversed (higher score is "smaller") because the default score sort is
// descending; the SortField's reverse flag (true for NewSortByScore) feeds the
// reverseMul that the queue applies on top.
//
// Mirrors org.apache.lucene.search.FieldComparator.RelevanceComparator.
type relevanceComparator struct {
	BaseFieldComparator

	scores   []float32
	bottom   float32
	topValue float32
	scorer   Scorable
}

func newRelevanceComparator(numHits int) *relevanceComparator {
	return &relevanceComparator{scores: make([]float32, numHits)}
}

func (c *relevanceComparator) Compare(slot1, slot2 int) int {
	return cmpFloat32(c.scores[slot2], c.scores[slot1])
}

func (c *relevanceComparator) Value(slot int) any { return c.scores[slot] }

func (c *relevanceComparator) setReader(IndexReader) error { return nil }

// GetLeafComparator returns the receiver: RelevanceComparator implements
// LeafFieldComparator itself and its getLeafComparator returns `this`.
func (c *relevanceComparator) GetLeafComparator(*index.LeafReaderContext) (LeafFieldComparator, error) {
	return c, nil
}

// SetTopValue records the top score for CompareTop.
//
// Mirrors FieldComparator.RelevanceComparator.setTopValue(Float).
func (c *relevanceComparator) SetTopValue(value any) { c.topValue = topValueFloat32(value) }

// CompareValues is reversed intentionally, because relevance by default sorts
// descending.
//
// Mirrors FieldComparator.RelevanceComparator.compareValues(Float, Float),
// whose body is second.compareTo(first).
func (c *relevanceComparator) CompareValues(first, second any) int {
	return cmpFloat32(topValueFloat32(second), topValueFloat32(first))
}

func (c *relevanceComparator) SetBottom(slot int) error { c.bottom = c.scores[slot]; return nil }

func (c *relevanceComparator) CompareBottom(doc int) (int, error) {
	score, err := c.score()
	if err != nil {
		return 0, err
	}
	return cmpFloat32(score, c.bottom), nil
}

// CompareTop compares the current document's score with the top score.
//
// Mirrors FieldComparator.RelevanceComparator.compareTop.
func (c *relevanceComparator) CompareTop(doc int) (int, error) {
	docValue, err := c.score()
	if err != nil {
		return 0, err
	}
	return cmpFloat32(docValue, c.topValue), nil
}

func (c *relevanceComparator) Copy(slot, doc int) error {
	score, err := c.score()
	if err != nil {
		return err
	}
	c.scores[slot] = score
	return nil
}

func (c *relevanceComparator) score() (float32, error) {
	if c.scorer == nil {
		return 0, nil
	}
	return c.scorer.Score()
}

func (c *relevanceComparator) SetScorer(scorer Scorable) error { c.scorer = scorer; return nil }
func (c *relevanceComparator) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}
func (c *relevanceComparator) SetHitsThresholdReached() error { return nil }

// docComparator sorts by global document id (ascending). The leaf-local doc is
// rebased with the segment's docBase, set when GetLeafCollector binds the leaf.
//
// Mirrors org.apache.lucene.search.comparators.DocComparator.
type docComparator struct {
	BaseFieldComparator

	docIDs   []int
	bottom   int
	topValue int
	docBase  int
}

func newDocComparator(numHits int) *docComparator {
	return &docComparator{docIDs: make([]int, numHits)}
}

func (c *docComparator) Compare(slot1, slot2 int) int {
	return cmpInt(c.docIDs[slot1], c.docIDs[slot2])
}

func (c *docComparator) Value(slot int) any { return c.docIDs[slot] }

// SetTopValue records the top document id for CompareTop.
//
// Mirrors DocComparator.setTopValue(Integer).
func (c *docComparator) SetTopValue(value any) { c.topValue = int(topValueInt32(value)) }

// GetLeafComparator records the segment's docBase and returns the receiver.
//
// Mirrors DocComparator.getLeafComparator(LeafReaderContext), which builds a
// DocLeafComparator holding context.docBase; this port carries no per-leaf
// skipping state, so the single instance is the leaf comparator.
func (c *docComparator) GetLeafComparator(context *index.LeafReaderContext) (LeafFieldComparator, error) {
	if context != nil {
		c.docBase = context.DocBase
	}
	return c, nil
}

// setReader cannot learn the docBase from the minimal IndexReader, so the
// collector forwards it via SetDocBase on the leaf collector path; here we reset
// to 0 and rely on the leaf collector wiring. For correctness within a single
// segment the docBase is constant, and the cross-segment ordering is preserved
// because docIDs are stored globally once copied.
func (c *docComparator) setReader(IndexReader) error { return nil }

func (c *docComparator) SetBottom(slot int) error { c.bottom = c.docIDs[slot]; return nil }

func (c *docComparator) CompareBottom(doc int) (int, error) {
	return cmpInt(c.bottom, c.docBase+doc), nil
}

// CompareTop compares the top document id with this document's global id.
//
// Mirrors DocComparator.DocLeafComparator.compareTop.
func (c *docComparator) CompareTop(doc int) (int, error) {
	return cmpInt(c.topValue, c.docBase+doc), nil
}

func (c *docComparator) Copy(slot, doc int) error {
	c.docIDs[slot] = c.docBase + doc
	return nil
}

func (c *docComparator) SetScorer(Scorable) error { return nil }
func (c *docComparator) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}
func (c *docComparator) SetHitsThresholdReached() error { return nil }

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

var (
	_ simpleFieldComparator = (*relevanceComparator)(nil)
	_ simpleFieldComparator = (*docComparator)(nil)
)
