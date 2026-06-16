// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanScorer.java

package spans

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpanScorer scores documents using a Spans iterator and an optional SimScorer.
//
// Mirrors org.apache.lucene.queries.spans.SpanScorer.
//
// Deviations from Java:
//   - Java's scorer field is Similarity.SimScorer; Gocene uses search.SimScorer.
//   - Java's norms field is NumericDocValues; Gocene uses index.NumericDocValues.
//   - Score() returns float32 directly; Java throws IOException.
type SpanScorer struct {
	spans     Spans
	simScorer search.SimScorer
	norms     index.NumericDocValues

	// accumulated sloppy freq for the last scored document
	freq          float32
	lastScoredDoc int
}

// newSpanScorer constructs a SpanScorer.
// simScorer may be nil when scoring is not needed.
// norms may be nil if no norm values are indexed.
func newSpanScorer(spans Spans, simScorer search.SimScorer, norms index.NumericDocValues) *SpanScorer {
	return &SpanScorer{
		spans:         spans,
		simScorer:     simScorer,
		norms:         norms,
		lastScoredDoc: -1,
	}
}

// DocID returns the current document ID.
func (s *SpanScorer) DocID() int { return s.spans.DocID() }

// NextDoc advances to the next document.
func (s *SpanScorer) NextDoc() (int, error) { return s.spans.NextDoc() }

// Advance advances to the first document >= target.
func (s *SpanScorer) Advance(target int) (int, error) { return s.spans.Advance(target) }

// Cost returns the estimated iteration cost.
func (s *SpanScorer) Cost() int64 { return s.spans.Cost() }

// DocIDRunEnd returns the conservative upper bound on the current document run.
func (s *SpanScorer) DocIDRunEnd() int { return s.spans.DocIDRunEnd() }

// TwoPhaseIterator returns a TwoPhaseIterator view, or nil.
func (s *SpanScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	return s.spans.AsTwoPhaseIterator()
}

// GetSpans returns the underlying Spans iterator.
func (s *SpanScorer) GetSpans() Spans { return s.spans }

// setFreqCurrentDoc accumulates the sloppy frequency for the current document.
// This is called at most once per document.
func (s *SpanScorer) setFreqCurrentDoc() error {
	s.freq = 0.0

	if err := s.spans.DoStartCurrentDoc(); err != nil {
		return err
	}

	// Ensure we are positioned at -1 start/end.
	startPos, err := s.spans.NextStartPosition()
	if err != nil {
		return err
	}
	if startPos == NoMorePositions {
		return nil
	}
	for {
		if s.simScorer == nil {
			// Scoring not required — just set freq to 1 and return.
			s.freq = 1
			return nil
		}
		s.freq += 1.0 / (1.0 + float32(s.spans.Width()))
		if err := s.spans.DoCurrentSpans(); err != nil {
			return err
		}
		next, err := s.spans.NextStartPosition()
		if err != nil {
			return err
		}
		if next == NoMorePositions {
			break
		}
	}
	return nil
}

// ensureFreq computes the sloppy frequency if not already done for the current doc.
func (s *SpanScorer) ensureFreq() error {
	cur := s.DocID()
	if s.lastScoredDoc != cur {
		if err := s.setFreqCurrentDoc(); err != nil {
			return err
		}
		s.lastScoredDoc = cur
	}
	return nil
}

// Score returns the score for the current document.
func (s *SpanScorer) Score() float32 {
	if err := s.ensureFreq(); err != nil {
		return 0
	}
	return s.scoreCurrentDoc()
}

// scoreCurrentDoc computes the score using SimScorer + norm.
func (s *SpanScorer) scoreCurrentDoc() float32 {
	if s.simScorer == nil {
		return 0
	}
	return s.simScorer.Score(s.DocID(), s.freq, 1)
}

// sloppyFreq returns the accumulated sloppy frequency; used by SpanWeight.Explain.
func (s *SpanScorer) sloppyFreq() (float32, error) {
	if err := s.ensureFreq(); err != nil {
		return 0, err
	}
	return s.freq, nil
}

// GetMaxScore returns an upper bound for the score (unbounded).
func (s *SpanScorer) GetMaxScore(_ int) float32 { return 1<<24 - 1 } // Float.MAX_VALUE analogue

// AdvanceShallow returns search.NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. Span scorers do not expose
// per-block impact information.
func (s *SpanScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

var _ search.Scorer = (*SpanScorer)(nil)
