// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BulkScorerWrapperScorer is the Go port of
// org.apache.lucene.tests.search.BulkScorerWrapperScorer (Apache Lucene
// 10.5.0): a Scorer implementation that wraps a BulkScorer.
type BulkScorerWrapperScorer struct {
	search.BaseScorer
	scorer search.BulkScorer

	i    int
	doc  int
	next int

	docs         []int
	scores       []float32
	bufferLength int

	it *bulkScorerWrapperIterator
}

// NewBulkScorerWrapperScorer renders the sole constructor
// BulkScorerWrapperScorer(BulkScorer, int bufferSize).
func NewBulkScorerWrapperScorer(scorer search.BulkScorer, bufferSize int) *BulkScorerWrapperScorer {
	s := &BulkScorerWrapperScorer{
		scorer: scorer,
		i:      -1,
		doc:    -1,
		docs:   make([]int, bufferSize),
		scores: make([]float32, bufferSize),
	}
	s.it = &bulkScorerWrapperIterator{s: s}
	return s
}

// refill renders the private refill(int).
func (s *BulkScorerWrapperScorer) refill(target int) error {
	s.bufferLength = 0
	for s.next != search.NO_MORE_DOCS && s.bufferLength == 0 {
		min := max(target, s.next)
		max := min + len(s.docs)
		next, err := s.scorer.Score(&bulkScorerWrapperCollector{outer: s}, nil, min, max)
		if err != nil {
			return err
		}
		s.next = next
	}
	s.i = -1
	return nil
}

// bulkScorerWrapperCollector renders the anonymous LeafCollector of refill.
type bulkScorerWrapperCollector struct {
	search.BaseLeafCollector
	outer  *BulkScorerWrapperScorer
	scorer search.Scorable
}

func (c *bulkScorerWrapperCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

func (c *bulkScorerWrapperCollector) Collect(doc int) error {
	s := c.outer
	s.docs[s.bufferLength] = doc
	score, err := c.scorer.Score()
	if err != nil {
		return err
	}
	s.scores[s.bufferLength] = score
	s.bufferLength++
	return nil
}

func (c *bulkScorerWrapperCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *bulkScorerWrapperCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// Score renders score().
func (s *BulkScorerWrapperScorer) Score() (float32, error) {
	return s.scores[s.i], nil
}

// GetMaxScore renders getMaxScore(int): Float.POSITIVE_INFINITY.
func (s *BulkScorerWrapperScorer) GetMaxScore(upTo int) (float32, error) {
	return float32(math.Inf(1)), nil
}

// DocID renders docID().
func (s *BulkScorerWrapperScorer) DocID() int { return s.doc }

// Iterator renders iterator().
func (s *BulkScorerWrapperScorer) Iterator() search.DocIdSetIterator { return s.it }

// NextDocsAndScores renders the inherited Scorer.nextDocsAndScores.
func (s *BulkScorerWrapperScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// bulkScorerWrapperIterator renders the anonymous DocIdSetIterator of iterator().
type bulkScorerWrapperIterator struct {
	s *BulkScorerWrapperScorer
}

func (it *bulkScorerWrapperIterator) DocID() int { return it.s.doc }

func (it *bulkScorerWrapperIterator) NextDoc() (int, error) { return it.Advance(it.DocID() + 1) }

func (it *bulkScorerWrapperIterator) Advance(target int) (int, error) {
	s := it.s
	if s.bufferLength == 0 || s.docs[s.bufferLength-1] < target {
		if err := s.refill(target); err != nil {
			return 0, err
		}
	}

	// i = Arrays.binarySearch(docs, i + 1, bufferLength, target); if (i < 0) i = -1 - i;
	from := s.i + 1
	s.i = from + sort.SearchInts(s.docs[from:s.bufferLength], target)
	if s.i == s.bufferLength {
		s.doc = search.NO_MORE_DOCS
		return s.doc, nil
	}
	s.doc = s.docs[s.i]
	return s.doc, nil
}

func (it *bulkScorerWrapperIterator) Cost() int64 { return it.s.scorer.Cost() }

func (it *bulkScorerWrapperIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

func (it *bulkScorerWrapperIterator) DocIDRunEnd() (int, error) { return util.DefaultDocIDRunEnd(it) }

var _ search.Scorer = (*BulkScorerWrapperScorer)(nil)
