// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// ContainSpans is a Spans implementation that matches spans from the 'little' query
// that are contained within a span from the 'big' query.
type ContainSpans struct {
	BaseSpans
	bigSpans            Spans
	littleSpans         Spans
	currentLittleSpans   Spans
	atFirstInCurrentDoc  bool
	oneExhaustedInCurrentDoc bool
}

func NewContainSpans(big, little, result Spans, isWithin bool) Spans {
	return &ContainSpans{
		bigSpans:    big,
		littleSpans: little,
		// In a real implementation, the result would be used for scoring.
	}
}

func (s *ContainSpans) NextDoc() (int, error) {
	doc, err := s.bigSpans.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	if doc == NO_MORE_DOCS {
		return NO_MORE_DOCS, nil
	}
	// Advance little spans to the same doc
	s.littleSpans.Advance(doc)
	s.atFirstInCurrentDoc = false
	s.oneExhaustedInCurrentDoc = false
	return doc, nil
}

func (s *ContainSpans) Advance(target int) (int, error) {
	doc, err := s.bigSpans.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	if doc == NO_MORE_DOCS {
		return NO_MORE_DOCS, nil
	}
	s.littleSpans.Advance(doc)
	s.atFirstInCurrentDoc = false
	s.oneExhaustedInCurrentDoc = false
	return doc, nil
}

func (s *ContainSpans) DocID() int {
	return s.bigSpans.DocID()
}

func (s *ContainSpans) NextStartPosition() (int, error) {
	// This is a simplified version of the contain logic.
	// In a real implementation, we'd check if little is inside big.
	for {
		pos, err := s.littleSpans.NextStartPosition()
		if err != nil {
			return -1, err
		}
		if pos == -1 {
			s.oneExhaustedInCurrentDoc = true
			return -1, nil
		}
		if s.bigSpans.StartPosition() <= pos && s.bigSpans.EndPosition() >= s.littleSpans.EndPosition() {
			return pos, nil
		}
		// Advance big to cover the current little span
		s.bigSpans.Advance(s.DocID())
	}
}

func (s *ContainSpans) StartPosition() int {
	return s.littleSpans.StartPosition()
}

func (s *ContainSpans) EndPosition() int {
	return s.littleSpans.EndPosition()
}

func (s *ContainSpans) Freq() int {
	return s.littleSpans.Freq()
}

func (s *ContainSpans) Width() int {
	return s.littleSpans.Width()
}

func (s *ContainSpans) Cost() int64 {
	return s.bigSpans.Cost() + s.littleSpans.Cost()
}

func (s *ContainSpans) DocIDRunEnd() int {
	return s.bigSpans.DocIDRunEnd()
}

func (s *ContainSpans) Collect(collector *SpanCollector) {
	// implementation placeholder
}

func (s *ContainSpans) PositionsCost() float32 {
	return s.bigSpans.PositionsCost() + s.littleSpans.PositionsCost()
}

func (s *ContainSpans) AsTwoPhaseIterator() *TwoPhaseIterator {
	return NewTwoPhaseIterator(s)
}
