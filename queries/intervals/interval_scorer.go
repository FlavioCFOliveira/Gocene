// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/IntervalScorer.java

package intervals

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// IntervalScorer scores documents by computing a sloppy frequency over all
// matching intervals and applying an IntervalScoreFunction.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalScorer.
//
// Deviations from Java:
//   - Extends search.BaseScorer (Go port of Scorer); no abstract Weight reference.
type IntervalScorer struct {
	intervals     IntervalIterator
	simScorer     search.SimScorer
	boost         float32
	minExtent     int
	freq          float32
	lastScoredDoc int
}

// NewIntervalScorer creates an IntervalScorer.
func NewIntervalScorer(intervals IntervalIterator, minExtent int, boost float32, scoreFunction IntervalScoreFunction) *IntervalScorer {
	return &IntervalScorer{
		intervals:     intervals,
		simScorer:     scoreFunction.Scorer(boost),
		boost:         boost,
		minExtent:     minExtent,
		lastScoredDoc: -1,
	}
}

// DocID returns the current document ID.
func (s *IntervalScorer) DocID() int { return s.intervals.DocID() }

// DocIDRunEnd returns a conservative upper bound.
func (s *IntervalScorer) DocIDRunEnd() int { return s.DocID() + 1 }

// Cost returns the estimated cost.
func (s *IntervalScorer) Cost() int64 { return s.intervals.Cost() }

// NextDoc advances to the next document.
func (s *IntervalScorer) NextDoc() (int, error) { return s.intervals.NextDoc() }

// Advance advances to at least the given target.
func (s *IntervalScorer) Advance(target int) (int, error) { return s.intervals.Advance(target) }

// Score returns the score for the current document.
// Implements search.Scorer.
func (s *IntervalScorer) Score() float32 {
	_ = s.ensureFreq() // errors stored internally
	return s.simScorer.Score(s.DocID(), s.freq)
}

// GetMaxScore returns the maximum possible score up to the given document.
func (s *IntervalScorer) GetMaxScore(upTo int) float32 { return s.boost }

// Freq returns the sloppy frequency for the current document and any error.
func (s *IntervalScorer) Freq() (float32, error) {
	err := s.ensureFreq()
	return s.freq, err
}

func (s *IntervalScorer) ensureFreq() error {
	if s.lastScoredDoc == s.DocID() {
		return nil
	}
	s.lastScoredDoc = s.DocID()
	s.freq = 0
	for {
		length := s.intervals.End() - s.intervals.Start() + 1
		denom := length - s.minExtent + 1
		if denom < 1 {
			denom = 1
		}
		s.freq += float32(1.0 / math.Max(float64(denom), 1.0))
		next, err := s.intervals.NextInterval()
		if err != nil {
			return err
		}
		if next == NoMoreIntervals {
			break
		}
	}
	return nil
}

var _ search.Scorer = (*IntervalScorer)(nil)
