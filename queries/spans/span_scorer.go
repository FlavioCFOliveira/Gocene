// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanScorer.java

package spans

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SpanScorer is a basic search.Scorer over Spans.
//
// Mirrors org.apache.lucene.queries.spans.SpanScorer (Apache Lucene 10.5.0).
//
// Java's SpanScorer extends Scorer and overrides only docID(), iterator(),
// twoPhaseIterator(), score() and getMaxScore(int); every other member of
// Scorer/Scorable keeps the base-class body, which search.BaseScorer carries.
type SpanScorer struct {
	search.BaseScorer

	spans     Spans
	simScorer search.SimScorer
	norms     index.NumericDocValues

	// accumulated sloppy freq (computed in setFreqCurrentDoc)
	freq float32

	// last doc we called setFreqCurrentDoc() for
	lastScoredDoc int
}

// newSpanScorer constructs a SpanScorer.
// simScorer may be nil when scoring is not needed.
// norms may be nil if no norm values are indexed.
//
// Mirrors the sole constructor SpanScorer(Spans, SimScorer, NumericDocValues).
func newSpanScorer(spans Spans, simScorer search.SimScorer, norms index.NumericDocValues) *SpanScorer {
	if spans == nil {
		panic("SpanScorer: spans must not be nil")
	}
	return &SpanScorer{
		spans:         spans,
		simScorer:     simScorer,
		norms:         norms,
		lastScoredDoc: -1,
	}
}

// GetSpans returns the Spans for this Scorer.
//
// Mirrors SpanScorer.getSpans().
func (s *SpanScorer) GetSpans() Spans { return s.spans }

// DocID returns the current document ID.
//
// Mirrors SpanScorer.docID().
func (s *SpanScorer) DocID() int { return s.spans.DocID() }

// Iterator returns the Spans, which is itself a DocIdSetIterator.
//
// Mirrors SpanScorer.iterator().
func (s *SpanScorer) Iterator() search.DocIdSetIterator { return s.spans }

// TwoPhaseIterator returns a TwoPhaseIterator view, or nil.
//
// Mirrors SpanScorer.twoPhaseIterator().
func (s *SpanScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	return s.spans.AsTwoPhaseIterator()
}

// scoreCurrentDoc scores the current doc with the similarity using the
// slop-adjusted freq.
//
// Mirrors SpanScorer.scoreCurrentDoc().
func (s *SpanScorer) scoreCurrentDoc() (float32, error) {
	if s.simScorer == nil {
		panic("SpanScorer has a null docScorer!")
	}
	norm := int64(1)
	if s.norms != nil {
		exact, err := s.norms.AdvanceExact(s.DocID())
		if err != nil {
			return 0, err
		}
		if exact {
			norm, err = s.norms.LongValue()
			if err != nil {
				return 0, err
			}
		}
	}
	return s.simScorer.Score104(s.freq, norm), nil
}

// setFreqCurrentDoc sets freq for the current document.
// This will be called at most once per document.
//
// Mirrors SpanScorer.setFreqCurrentDoc().
func (s *SpanScorer) setFreqCurrentDoc() error {
	s.freq = 0.0

	if err := s.spans.DoStartCurrentDoc(); err != nil {
		return err
	}

	startPos, err := s.spans.NextStartPosition()
	if err != nil {
		return err
	}
	for {
		if s.simScorer == nil { // scores not required, break out here
			s.freq = 1
			return nil
		}
		s.freq += 1.0 / (1.0 + float32(s.spans.Width()))
		if err := s.spans.DoCurrentSpans(); err != nil {
			return err
		}
		startPos, err = s.spans.NextStartPosition()
		if err != nil {
			return err
		}
		if startPos == NoMorePositions {
			return nil
		}
	}
}

// ensureFreq makes sure setFreqCurrentDoc has been called for the current doc.
//
// Mirrors SpanScorer.ensureFreq().
func (s *SpanScorer) ensureFreq() error {
	currentDoc := s.DocID()
	if s.lastScoredDoc != currentDoc {
		if err := s.setFreqCurrentDoc(); err != nil {
			return err
		}
		s.lastScoredDoc = currentDoc
	}
	return nil
}

// Score returns the score for the current document.
//
// Mirrors SpanScorer.score().
func (s *SpanScorer) Score() (float32, error) {
	if err := s.ensureFreq(); err != nil {
		return 0, err
	}
	return s.scoreCurrentDoc()
}

// GetMaxScore returns Float.POSITIVE_INFINITY, the value Java's
// SpanScorer.getMaxScore(int) returns for every upTo.
func (s *SpanScorer) GetMaxScore(upTo int) (float32, error) {
	return float32(math.Inf(1)), nil
}

// NextDocsAndScores carries the concrete body of Scorer.nextDocsAndScores in
// Apache Lucene 10.5.0, which SpanScorer inherits without overriding.
func (s *SpanScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// sloppyFreq returns the intermediate "sloppy freq" adjusted for edit distance.
//
// Mirrors SpanScorer.sloppyFreq().
func (s *SpanScorer) sloppyFreq() (float32, error) {
	if err := s.ensureFreq(); err != nil {
		return 0, err
	}
	return s.freq, nil
}

var _ search.Scorer = (*SpanScorer)(nil)
