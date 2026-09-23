// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// constantScorer is the synthetic Scorer the bulk-scorer and conjunction
// tests of this package share: it iterates over a fixed, sorted doc set with
// a fixed score and maxScore. The members it does not specialise carry the
// default bodies Lucene's Scorer and DocIdSetIterator give them.
type constantScorer struct {
	docs     []int
	pos      int
	score    float32
	maxScore float32
}

func newConstantScorer(docs []int, score, maxScore float32) *constantScorer {
	return &constantScorer{docs: docs, pos: -1, score: score, maxScore: maxScore}
}

func (s *constantScorer) DocID() int {
	if s.pos < 0 {
		return -1
	}
	if s.pos >= len(s.docs) {
		return search.NO_MORE_DOCS
	}
	return s.docs[s.pos]
}

func (s *constantScorer) NextDoc() (int, error) {
	s.pos++
	return s.DocID(), nil
}

func (s *constantScorer) Advance(target int) (int, error) {
	for s.pos++; s.pos < len(s.docs) && s.docs[s.pos] < target; s.pos++ {
	}
	return s.DocID(), nil
}

func (s *constantScorer) Cost() int64 { return int64(len(s.docs)) }

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd().
func (s *constantScorer) DocIDRunEnd() (int, error) { return s.DocID() + 1, nil }

func (s *constantScorer) Score() (float32, error) { return s.score, nil }

func (s *constantScorer) GetMaxScore(_ int) (float32, error) { return s.maxScore, nil }

func (s *constantScorer) AdvanceShallow(int) (int, error) { return search.NO_MORE_DOCS, nil }

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int).
func (s *constantScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// GetChildren carries the default body of Scorable.getChildren(): no children.
func (s *constantScorer) GetChildren() ([]search.ChildScorable, error) { return nil, nil }

// Iterator returns the scorer itself, which iterates the doc set.
func (s *constantScorer) Iterator() search.DocIdSetIterator { return s }

// NextDocsAndScores carries the default body of Scorer.nextDocsAndScores.
func (s *constantScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// SetMinCompetitiveScore carries the default body of
// Scorable.setMinCompetitiveScore(float): a no-op.
func (s *constantScorer) SetMinCompetitiveScore(minScore float32) error { return nil }

// SmoothingScore carries the default body of Scorable.smoothingScore(int): 0.
func (s *constantScorer) SmoothingScore(docID int) (float32, error) { return 0, nil }

// TwoPhaseIterator carries the default body of Scorer.twoPhaseIterator(): null.
func (s *constantScorer) TwoPhaseIterator() *search.TwoPhaseIterator { return nil }
