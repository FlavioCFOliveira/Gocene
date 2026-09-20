// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/IntervalScorer.java

package intervals

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IntervalScorer scores documents by computing a sloppy frequency over all
// matching intervals and applying an IntervalScoreFunction.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalScorer.
type IntervalScorer struct {
	search.BaseScorer

	intervals     IntervalIterator
	simScorer     search.SimScorer
	boost         float32
	minExtent     int
	freq          float32
	lastScoredDoc int
}

// NewIntervalScorer creates an IntervalScorer.
//
// Mirrors IntervalScorer(IntervalIterator, int, float, IntervalScoreFunction).
func NewIntervalScorer(intervals IntervalIterator, minExtent int, boost float32, scoreFunction IntervalScoreFunction) *IntervalScorer {
	return &IntervalScorer{
		intervals:     intervals,
		simScorer:     scoreFunction.Scorer(boost),
		boost:         boost,
		minExtent:     minExtent,
		lastScoredDoc: -1,
	}
}

// DocID mirrors IntervalScorer.docID(): intervals.docID().
func (s *IntervalScorer) DocID() int { return s.intervals.DocID() }

// Score mirrors IntervalScorer.score(): ensureFreq(); simScorer.score(freq, 1).
func (s *IntervalScorer) Score() (float32, error) {
	if err := s.ensureFreq(); err != nil {
		return 0, err
	}
	return s.simScorer.Score104(s.freq, 1), nil
}

// Freq mirrors the package-private IntervalScorer.freq().
func (s *IntervalScorer) Freq() (float32, error) {
	if err := s.ensureFreq(); err != nil {
		return 0, err
	}
	return s.freq, nil
}

// ensureFreq mirrors IntervalScorer.ensureFreq().
func (s *IntervalScorer) ensureFreq() error {
	if s.lastScoredDoc == s.DocID() {
		return nil
	}
	s.lastScoredDoc = s.DocID()
	s.freq = 0
	for {
		length := s.intervals.End() - s.intervals.Start() + 1
		s.freq += float32(1.0 / math.Max(float64(length-s.minExtent+1), 1))
		next, err := s.intervals.NextInterval()
		if err != nil {
			return err
		}
		if next == NoMoreIntervals {
			return nil
		}
	}
}

// Iterator mirrors IntervalScorer.iterator():
// TwoPhaseIterator.asDocIdSetIterator(twoPhaseIterator()).
func (s *IntervalScorer) Iterator() search.DocIdSetIterator {
	return search.AsDocIdSetIterator(s.TwoPhaseIterator())
}

// TwoPhaseIterator mirrors IntervalScorer.twoPhaseIterator(), which returns an
// anonymous TwoPhaseIterator over the interval iterator whose matches() advances
// to the next interval and whose matchCost() is the iterator's match cost.
func (s *IntervalScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	return search.NewTwoPhaseIterator(s.intervals, (*intervalScorerVerifier)(s))
}

// intervalScorerVerifier carries the body of the anonymous TwoPhaseIterator
// subclass returned by IntervalScorer.twoPhaseIterator().
type intervalScorerVerifier IntervalScorer

// Matches mirrors the anonymous TwoPhaseIterator.matches():
// intervals.nextInterval() != IntervalIterator.NO_MORE_INTERVALS.
func (v *intervalScorerVerifier) Matches() (bool, error) {
	next, err := v.intervals.NextInterval()
	if err != nil {
		return false, err
	}
	return next != NoMoreIntervals, nil
}

// MatchCost mirrors the anonymous TwoPhaseIterator.matchCost():
// intervals.matchCost().
func (v *intervalScorerVerifier) MatchCost() float32 {
	return v.intervals.MatchCost()
}

// GetMaxScore mirrors IntervalScorer.getMaxScore(int): boost.
func (s *IntervalScorer) GetMaxScore(upTo int) (float32, error) { return s.boost, nil }

// NextDocsAndScores carries the concrete body of Scorer.nextDocsAndScores in
// Apache Lucene 10.5.0, which IntervalScorer does not override.
func (s *IntervalScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

var (
	_ search.Scorer           = (*IntervalScorer)(nil)
	_ search.TwoPhaseVerifier = (*intervalScorerVerifier)(nil)
)
