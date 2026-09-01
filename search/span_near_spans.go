// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
)

// GapSpans matches a gap of a fixed width.
type GapSpans struct {
	BaseSpans
	doc int
	pos int
	width int
}

func NewGapSpans(width int) Spans {
	return &GapSpans{
		doc:   -1,
		pos:   -1,
		width: width,
	}
}

func (s *GapSpans) NextStartPosition() (int, error) {
	s.pos++
	return s.pos, nil
}

func (s *GapSpans) StartPosition() int {
	return s.pos
}

func (s *GapSpans) EndPosition() int {
	return s.pos + s.width
}

func (s *GapSpans) Width() int {
	return s.width
}

func (s *GapSpans) NextDoc() (int, error) {
	s.pos = -1
	s.doc++
	return s.doc, nil
}

func (s *GapSpans) Advance(target int) (int, error) {
	s.pos = -1
	s.doc = target
	return s.doc, nil
}

func (s *GapSpans) DocID() int {
	return s.doc
}

func (s *GapSpans) Cost() int64 {
	return 0
}

func (s *GapSpans) PositionsCost() float32 {
	return 0
}

func (s *GapSpans) Collect(collector *SpanCollector) {
	// Gaps don't contribute to the final span result themselves,
	// they only constrain the positions of other spans.
}

// NearSpansOrdered implements an ordered near search.
type NearSpansOrdered struct {
	BaseSpans
	slop     int
	subSpans []Spans
	current  []int // current start positions
}

func NewNearSpansOrdered(slop int, subSpans []Spans) Spans {
	return &NearSpansOrdered{
		slop:     slop,
		subSpans: subSpans,
		current:  make([]int, len(subSpans)),
	}
}

func (s *NearSpansOrdered) NextDoc() (int, error) {
	// This is a simplified implementation.
	// A real NearSpansOrdered would use a more complex algorithm to find the next document.
	// For now, we'll just advance all sub-spans.

	// Implementation placeholder.
	return NO_MORE_DOCS, nil
}

func (s *NearSpansOrdered) Advance(target int) (int, error) {
	// Implementation placeholder.
	return NO_MORE_DOCS, nil
}

func (s *NearSpansOrdered) DocID() int {
	return s.doc
}

func (s *NearSpansOrdered) NextStartPosition() (int, error) {
	// Implementation placeholder.
	return -1, nil
}

func (s *NearSpansOrdered) StartPosition() int {
	return s.start
}

func (s *NearSpansOrdered) EndPosition() int {
	return s.end
}

func (s *NearSpansOrdered) Freq() int {
	return s.freq
}

func (s *NearSpansOrdered) Width() int {
	return s.end - s.start
}

func (s *NearSpansOrdered) Cost() int64 {
	var total int64
	for _, ss := range s.subSpans {
		total += ss.Cost()
	}
	return total
}

func (s *NearSpansOrdered) DocIDRunEnd() int {
	return s.doc + 1
}

func (s *NearSpansOrdered) Collect(collector *SpanCollector) {
	// Implementation placeholder.
}

func (s *NearSpansOrdered) PositionsCost() float32 {
	var total float32
	for _, ss := range s.subSpans {
		total += ss.PositionsCost()
	}
	return total
}

func (s *NearSpansOrdered) AsTwoPhaseIterator() *TwoPhaseIterator {
	return NewTwoPhaseIterator(s)
}

// NearSpansUnordered implements an unordered near search.
type NearSpansUnordered struct {
	BaseSpans
	slop     int
	subSpans []Spans
}

func NewNearSpansUnordered(slop int, subSpans []Spans) Spans {
	return &NearSpansUnordered{
		slop:     slop,
		subSpans: subSpans,
	}
}

func (s *NearSpansUnordered) NextDoc() (int, error) {
	// Implementation placeholder.
	return NO_MORE_DOCS, nil
}

func (s *NearSpansUnordered) Advance(target int) (int, error) {
	// Implementation placeholder.
	return NO_MORE_DOCS, nil
}

func (s *NearSpansUnordered) DocID() int {
	return s.doc
}

func (s *NearSpansUnordered) NextStartPosition() (int, error) {
	// Implementation placeholder.
	return -1, nil
}

func (s *NearSpansUnordered) StartPosition() int {
	return s.start
}

func (s *NearSpansUnordered) EndPosition() int {
	return s.end
}

func (s *NearSpansUnordered) Freq() int {
	return s.freq
}

func (s *NearSpansUnordered) Width() int {
	return s.end - s.start
}

func (s *NearSpansUnordered) Cost() int64 {
	var total int64
	for _, ss := range s.subSpans {
		total += ss.Cost()
	}
	return total
}

func (s *NearSpansUnordered) DocIDRunEnd() int {
	return s.doc + 1
}

func (s *NearSpansUnordered) Collect(collector *SpanCollector) {
	// Implementation placeholder.
}

func (s *NearSpansUnordered) PositionsCost() float32 {
	var total float32
	for _, ss := range s.subSpans {
		total += ss.PositionsCost()
	}
	return total
}

func (s *NearSpansUnordered) AsTwoPhaseIterator() *TwoPhaseIterator {
	return NewTwoPhaseIterator(s)
}
