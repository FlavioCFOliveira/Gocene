// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerScorer.
package search

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// QueryProfilerScorer is a search.Scorer wrapper that records how much time is
// spent on each operation (NextDoc, Advance, Score, GetMaxScore). It holds
// timers sourced from a QueryProfilerBreakdown and delegates all calls to the
// wrapped scorer.
//
// Mirrors org.apache.lucene.sandbox.search.QueryProfilerScorer.
type QueryProfilerScorer struct {
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

// NextDoc advances to the next matching document.
func (s *QueryProfilerScorer) NextDoc() (int, error) {
	s.nextDocTimer.Start()
	defer s.nextDocTimer.Stop()
	return s.scorer.NextDoc()
}

// Advance advances to the first document at or beyond target.
func (s *QueryProfilerScorer) Advance(target int) (int, error) {
	s.advanceTimer.Start()
	defer s.advanceTimer.Stop()
	return s.scorer.Advance(target)
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs.
func (s *QueryProfilerScorer) DocIDRunEnd() int { return s.scorer.DocIDRunEnd() }

// Cost returns the approximate cost of iteration.
func (s *QueryProfilerScorer) Cost() int64 { return s.scorer.Cost() }

// Score returns the score for the current document.
func (s *QueryProfilerScorer) Score() float32 {
	s.scoreTimer.Start()
	defer s.scoreTimer.Stop()
	return s.scorer.Score()
}

// GetMaxScore returns the maximum score for documents up to upTo.
func (s *QueryProfilerScorer) GetMaxScore(upTo int) float32 {
	s.computeMaxScoreTimer.Start()
	defer s.computeMaxScoreTimer.Stop()
	return s.scorer.GetMaxScore(upTo)
}

var _ search.Scorer = (*QueryProfilerScorer)(nil)
