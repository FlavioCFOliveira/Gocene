// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerScorer.
package search

import (
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// QueryProfilerScorer is a search.Scorer wrapper that records how much time is
// spent on each operation (NextDoc, Advance, Score, GetMaxScore). It holds
// timers sourced from a QueryProfilerBreakdown and delegates all calls to the
// wrapped scorer.
//
// Mirrors org.apache.lucene.sandbox.search.QueryProfilerScorer.
type QueryProfilerScorer struct {
	// Java's QueryProfilerScorer extends Scorer and does not override
	// smoothingScore; setMinCompetitiveScore is timed below.
	search.BaseScorable

	scorer               search.Scorer
	scoreTimer           *QueryProfilerTimer
	nextDocTimer         *QueryProfilerTimer
	advanceTimer         *QueryProfilerTimer
	computeMaxScoreTimer *QueryProfilerTimer
}

// newQueryProfilerScorer wraps scorer with profiling timers from profile.
func newQueryProfilerScorer(scorer search.Scorer, profile *QueryProfilerBreakdown) *QueryProfilerScorer {
	return &QueryProfilerScorer{
		scorer:               scorer,
		scoreTimer:           profile.GetTimer(TimingTypeScore),
		nextDocTimer:         profile.GetTimer(TimingTypeNextDoc),
		advanceTimer:         profile.GetTimer(TimingTypeAdvance),
		computeMaxScoreTimer: profile.GetTimer(TimingTypeComputeMaxScore),
	}
}

// DocID returns the current document ID.
func (s *QueryProfilerScorer) DocID() int { return s.scorer.DocID() }

// Iterator returns a timing view over the wrapped scorer's iterator.
//
// Java: QueryProfilerScorer#iterator() wraps scorer.iterator() in a
// FilterDocIdSetIterator whose advance/nextDoc are timed.
func (s *QueryProfilerScorer) Iterator() search.DocIdSetIterator {
	return &queryProfilerDocIdSetIterator{
		in:           s.scorer.Iterator(),
		nextDocTimer: s.nextDocTimer,
		advanceTimer: s.advanceTimer,
	}
}

// TwoPhaseIterator delegates to the wrapped scorer.
func (s *QueryProfilerScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	return s.scorer.TwoPhaseIterator()
}

// GetChildren delegates to the wrapped scorer.
func (s *QueryProfilerScorer) GetChildren() ([]search.ChildScorable, error) {
	return s.scorer.GetChildren()
}

// Score returns the score for the current document.
func (s *QueryProfilerScorer) Score() (float32, error) {
	s.scoreTimer.Start()
	defer s.scoreTimer.Stop()
	return s.scorer.Score()
}

// GetMaxScore returns the maximum score for documents up to upTo.
func (s *QueryProfilerScorer) GetMaxScore(upTo int) (float32, error) {
	s.computeMaxScoreTimer.Start()
	defer s.computeMaxScoreTimer.Stop()
	return s.scorer.GetMaxScore(upTo)
}

// NextDocsAndScores carries the default body of Scorer#nextDocsAndScores.
func (s *QueryProfilerScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// queryProfilerDocIdSetIterator renders the anonymous FilterDocIdSetIterator
// that QueryProfilerScorer#iterator() returns: nextDoc and advance are timed,
// everything else delegates.
type queryProfilerDocIdSetIterator struct {
	in           search.DocIdSetIterator
	nextDocTimer *QueryProfilerTimer
	advanceTimer *QueryProfilerTimer
}

func (it *queryProfilerDocIdSetIterator) DocID() int { return it.in.DocID() }

func (it *queryProfilerDocIdSetIterator) NextDoc() (int, error) {
	it.nextDocTimer.Start()
	defer it.nextDocTimer.Stop()
	return it.in.NextDoc()
}

func (it *queryProfilerDocIdSetIterator) Advance(target int) (int, error) {
	it.advanceTimer.Start()
	defer it.advanceTimer.Stop()
	return it.in.Advance(target)
}

func (it *queryProfilerDocIdSetIterator) Cost() int64 { return it.in.Cost() }

func (it *queryProfilerDocIdSetIterator) DocIDRunEnd() (int, error) { return it.in.DocIDRunEnd() }

func (it *queryProfilerDocIdSetIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return it.in.IntoBitSet(upTo, bitSet, offset)
}

// AdvanceShallow delegates to the wrapped scorer so that the block boundary
// reported here is consistent with the block-max upper bound returned by
// GetMaxScore. It is timed under the same compute-max-score timer because
// shallow advances exist solely to feed block-max score computation.
func (s *QueryProfilerScorer) AdvanceShallow(target int) (int, error) {
	s.computeMaxScoreTimer.Start()
	defer s.computeMaxScoreTimer.Stop()
	return s.scorer.AdvanceShallow(target)
}

var _ search.Scorer = (*QueryProfilerScorer)(nil)
