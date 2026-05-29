// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/FieldComparator.java
//     (RelevanceComparator)
//   lucene/core/src/java/org/apache/lucene/search/comparators/DocComparator.java
//
// These are the two sort keys that do not read DocValues: SCORE (relevance) and
// DOC (document id). They let a Sort mix score/doc keys with DocValues keys —
// most commonly as a secondary tie-break — without the DocValues comparator
// factory rejecting the field.

// newBuiltinComparator builds the SCORE or DOC comparator. It is only called for
// SortField types the DocValues factory does not handle (SCORE, DOC, CUSTOM).
// CUSTOM falls back to a DOC comparator, which keeps the collector functional
// (FieldComparatorSource is not yet wired into this path).
func newBuiltinComparator(numHits int, sf *SortField) sortFieldComparator {
	if sf.Type == SortFieldTypeScore {
		return newRelevanceComparator(numHits)
	}
	return newDocComparator(numHits)
}

// relevanceComparator sorts by relevance score. As in Lucene, compare is
// reversed (higher score is "smaller") because the default score sort is
// descending; the SortField's reverse flag (true for NewSortByScore) feeds the
// reverseMul that the queue applies on top.
//
// Mirrors org.apache.lucene.search.FieldComparator.RelevanceComparator.
type relevanceComparator struct {
	scores []float32
	bottom float32
	scorer Scorable
}

func newRelevanceComparator(numHits int) *relevanceComparator {
	return &relevanceComparator{scores: make([]float32, numHits)}
}

func (c *relevanceComparator) compare(slot1, slot2 int) int {
	return cmpFloat32(c.scores[slot2], c.scores[slot1])
}

func (c *relevanceComparator) value(slot int) any { return c.scores[slot] }

func (c *relevanceComparator) setReader(IndexReader) error { return nil }

func (c *relevanceComparator) SetBottom(slot int) error { c.bottom = c.scores[slot]; return nil }

func (c *relevanceComparator) CompareBottom(doc int) (int, error) {
	score, err := c.score()
	if err != nil {
		return 0, err
	}
	return cmpFloat32(score, c.bottom), nil
}

func (c *relevanceComparator) CompareTop(doc int) (int, error) { return 0, nil }

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
func (c *relevanceComparator) SetHitsThresholdReached() {}

// docComparator sorts by global document id (ascending). The leaf-local doc is
// rebased with the segment's docBase, set when GetLeafCollector binds the leaf.
//
// Mirrors org.apache.lucene.search.comparators.DocComparator.
type docComparator struct {
	docIDs  []int
	bottom  int
	docBase int
}

func newDocComparator(numHits int) *docComparator {
	return &docComparator{docIDs: make([]int, numHits)}
}

func (c *docComparator) compare(slot1, slot2 int) int {
	return cmpInt(c.docIDs[slot1], c.docIDs[slot2])
}

func (c *docComparator) value(slot int) any { return c.docIDs[slot] }

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

func (c *docComparator) CompareTop(doc int) (int, error) { return 0, nil }

func (c *docComparator) Copy(slot, doc int) error {
	c.docIDs[slot] = c.docBase + doc
	return nil
}

func (c *docComparator) SetScorer(Scorable) error { return nil }
func (c *docComparator) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}
func (c *docComparator) SetHitsThresholdReached() {}

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
	_ sortFieldComparator = (*relevanceComparator)(nil)
	_ sortFieldComparator = (*docComparator)(nil)
)
