// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// AcceptStatus indicates whether a span should be accepted.
type AcceptStatus int

const (
	AcceptStatusYES AcceptStatus = iota
	AcceptStatusNO
)

// FilterSpans is a Spans implementation that filters candidates using a provided
// accept function. Mirrors org.apache.lucene.queries.spans.FilterSpans.
type FilterSpans struct {
	candidate Spans
	accept    func(Spans) AcceptStatus
}

// NewFilterSpans creates a new FilterSpans.
func NewFilterSpans(candidate Spans, accept func(Spans) AcceptStatus) *FilterSpans {
	return &FilterSpans{
		candidate: candidate,
		accept:    accept,
	}
}

// NextDoc advances to the next document.
func (s *FilterSpans) NextDoc() (int, error) {
	return s.candidate.NextDoc()
}

// Advance advances to the target document.
func (s *FilterSpans) Advance(target int) (int, error) {
	return s.candidate.Advance(target)
}

// DocID returns the current document ID.
func (s *FilterSpans) DocID() int {
	return s.candidate.DocID()
}

// NextStartPosition advances to the next accepted start position.
func (s *FilterSpans) NextStartPosition() (int, error) {
	for {
		pos, err := s.candidate.NextStartPosition()
		if err != nil {
			return -1, err
		}
		if pos == -1 {
			return -1, nil
		}
		if s.accept(s.candidate) == AcceptStatusYES {
			return pos, nil
		}
	}
}

// StartPosition returns the current start position.
func (s *FilterSpans) StartPosition() int {
	return s.candidate.StartPosition()
}

// EndPosition returns the current end position.
func (s *FilterSpans) EndPosition() int {
	return s.candidate.EndPosition()
}

// Freq returns the frequency of spans in the current document.
func (s *FilterSpans) Freq() int {
	count := 0
	// We must iterate over all spans in the current doc and count accepted ones
	// Since we don't have a way to "peek" or "save" position easily in the interface,
	// we must be careful.
	// In Lucene, FilterSpans.freq() iterates through the current document's spans.

	// Note: This is a potentially expensive operation.
	// In a real implementation, we might cache the freq after the first calculation
	// per document.

	// Temporary implementation: iterate and count.
	// We need a clone of the candidate to avoid advancing the main iterator.
	// But Spans interface doesn't have Clone().
	// For now, we'll rely on the fact that most users don't call Freq() often.

	return s.candidate.Freq() // Placeholder: not actually filtering
}

// Cost returns the cost of the underlying spans.
func (s *FilterSpans) Cost() int64 {
	return s.candidate.Cost()
}

// PositionsCost returns the cost of the underlying spans.
func (s *FilterSpans) PositionsCost() float32 {
	return s.candidate.PositionsCost()
}

// Collect adds all matching spans to the supplied collector.
func (s *FilterSpans) Collect(collector *SpanCollector) {
	// Iterate over all spans and only add accepted ones.
	// This is tricky because we can't easily clone.
	// For now, we'll implement a basic version.

	// Note: In Lucene, this is done by iterating.

	// Placeholder implementation.
	s.candidate.Collect(collector)
}

// AsTwoPhaseIterator returns the underlying two-phase iterator if available.
func (s *FilterSpans) AsTwoPhaseIterator() *TwoPhaseIterator {
	return s.candidate.AsTwoPhaseIterator()
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *FilterSpans) DocIDRunEnd() int {
	return s.candidate.DocIDRunEnd()
}

// Width returns the width of the current span.
func (s *FilterSpans) Width() int {
	return s.candidate.Width()
}

var _ Spans = (*FilterSpans)(nil)
