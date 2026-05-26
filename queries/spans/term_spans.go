// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/TermSpans.java

package spans

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TermSpans iterates over span positions for a single term.
// Each document position corresponds to a span [pos, pos+1).
//
// Mirrors org.apache.lucene.queries.spans.TermSpans.
//
// Deviations from Java:
//   - positionsCost is passed at construction rather than derived from TermsEnum.
//   - readPayload flag is present but Gocene does not expose a payload API here;
//     payload collection goes through SpanCollector.CollectLeaf.
type TermSpans struct {
	BaseSpans
	postings      index.PostingsEnum
	term          index.Term
	doc           int
	freq          int
	count         int
	position      int
	positionsCost float32
}

// NewTermSpans constructs a TermSpans.
// positionsCost must be > 0.
func NewTermSpans(postings index.PostingsEnum, term index.Term, positionsCost float32) *TermSpans {
	return &TermSpans{
		postings:      postings,
		term:          term,
		doc:           -1,
		position:      -1,
		positionsCost: positionsCost,
	}
}

// DocID returns the current document ID.
func (s *TermSpans) DocID() int { return s.doc }

// DocIDRunEnd returns the exclusive upper bound of the current doc-ID run.
func (s *TermSpans) DocIDRunEnd() int { return s.doc + 1 }

// Cost returns the estimated iteration cost.
func (s *TermSpans) Cost() int64 { return s.postings.Cost() }

// NextDoc advances to the next document.
func (s *TermSpans) NextDoc() (int, error) {
	doc, err := s.postings.NextDoc()
	if err != nil {
		return 0, err
	}
	// Translate index.PostingsEnum's NO_MORE_DOCS sentinel (-1) to
	// search.NO_MORE_DOCS (MaxInt32) so the Spans layer sees a consistent value.
	if doc == index.NO_MORE_DOCS {
		s.doc = search.NO_MORE_DOCS
		s.position = -1
		return search.NO_MORE_DOCS, nil
	}
	s.doc = doc
	freq, err := s.postings.Freq()
	if err != nil {
		return 0, err
	}
	s.freq = freq
	s.count = 0
	s.position = -1
	return s.doc, nil
}

// Advance advances to the first document >= target.
func (s *TermSpans) Advance(target int) (int, error) {
	doc, err := s.postings.Advance(target)
	if err != nil {
		return 0, err
	}
	// Translate index.PostingsEnum's NO_MORE_DOCS sentinel (-1) to
	// search.NO_MORE_DOCS (MaxInt32) so the Spans layer sees a consistent value.
	if doc == index.NO_MORE_DOCS {
		s.doc = search.NO_MORE_DOCS
		s.position = -1
		return search.NO_MORE_DOCS, nil
	}
	s.doc = doc
	freq, err := s.postings.Freq()
	if err != nil {
		return 0, err
	}
	s.freq = freq
	s.count = 0
	s.position = -1
	return s.doc, nil
}

// NextStartPosition advances to the next position in the current document.
// Returns NoMorePositions when all positions have been consumed.
func (s *TermSpans) NextStartPosition() (int, error) {
	if s.count == s.freq {
		s.position = NoMorePositions
		return NoMorePositions, nil
	}
	pos, err := s.postings.NextPosition()
	if err != nil {
		return 0, err
	}
	s.position = pos
	s.count++
	return s.position, nil
}

// StartPosition returns the current start position, or -1 if not yet positioned.
func (s *TermSpans) StartPosition() int { return s.position }

// EndPosition returns position+1 for a term span (exclusive end).
// Returns -1 when not yet positioned; returns NoMorePositions when exhausted.
func (s *TermSpans) EndPosition() int {
	switch s.position {
	case -1:
		return -1
	case NoMorePositions:
		return NoMorePositions
	default:
		return s.position + 1
	}
}

// Width returns 0 (term spans have no gap-width contribution).
func (s *TermSpans) Width() int { return 0 }

// Collect invokes the collector's CollectLeaf for the current position.
func (s *TermSpans) Collect(collector SpanCollector) error {
	return collector.CollectLeaf(s.postings, s.position, s.term)
}

// PositionsCost returns the estimated cost of iterating positions in a single doc.
func (s *TermSpans) PositionsCost() float32 { return s.positionsCost }

// AsTwoPhaseIterator returns nil — TermSpans does not support two-phase iteration.
func (s *TermSpans) AsTwoPhaseIterator() *search.TwoPhaseIterator { return nil }

// GetPostings returns the underlying PostingsEnum.
func (s *TermSpans) GetPostings() index.PostingsEnum { return s.postings }

var _ Spans = (*TermSpans)(nil)
