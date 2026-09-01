// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// GC-1002: Spans iterator interface
// Spans is an iterator over span matches for (doc, start, end) tuples.
// This is the Go port of Lucene's org.apache.lucene.search.spans.Spans.
type Spans interface {
	// NextDoc advances to the next document.
	NextDoc() (int, error)
	// Advance advances to the specified document.
	Advance(target int) (int, error)
	// DocID returns the current document ID.
	DocID() int
	// NextStartPosition advances to the next start position. It returns -1
	// (Lucene's Spans.NO_MORE_POSITIONS) once every occurrence has been consumed.
	NextStartPosition() (int, error)
	// StartPosition returns the start position.
	StartPosition() int
	// EndPosition returns the end position.
	EndPosition() int
	// Freq returns the frequency of spans in the current document.
	Freq() int
	// Width returns the width of the current span.
	Width() int
	// Cost returns the estimated cost of iterating through all documents.
	Cost() int64
	// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
	DocIDRunEnd() int
	// Collect adds all matching spans to the supplied collector.
	Collect(collector *SpanCollector)
	// PositionsCost returns the cost of iterating positions.
	PositionsCost() float32
	// AsTwoPhaseIterator returns the iterator as a two-phase iterator.
	AsTwoPhaseIterator() *TwoPhaseIterator
}

// BaseSpans is the default implementation of Spans.
// It supports both array-backed and postings-backed iteration.
type BaseSpans struct {
	doc      int
	freq     int
	position int
	start    int
	end      int
	docs     []int
	starts   []int
	ends     []int
	index    int

	// postings, when non-nil, switches BaseSpans into TermSpans mode: positions are
	// drained from the term's PostingsEnum rather than the array backend.
	postings index.PostingsEnum
	count    int // number of positions consumed in the current document
}

// NewSpans creates a new array-backed Spans iterator.
func NewSpans(docs []int, starts []int, ends []int) Spans {
	return &BaseSpans{
		doc:    -1,
		docs:   docs,
		starts: starts,
		ends:   ends,
		index:  -1,
	}
}

// NewTermSpans creates a postings-backed Spans over a single term's positions.
// It is the Go port of org.apache.lucene.queries.spans.TermSpans: the supplied
// PostingsEnum must have been opened with positions (PostingsEnum.POSITIONS).
func NewTermSpans(postings index.PostingsEnum) Spans {
	return &BaseSpans{
		doc:      -1,
		position: -1,
		postings: postings,
		index:    -1,
	}
}

func (s *BaseSpans) postingsBacked() bool {
	return s.postings != nil
}

func (s *BaseSpans) NextDoc() (int, error) {
	if s.postingsBacked() {
		d, err := s.postings.NextDoc()
		if err != nil {
			s.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, err
		}
		return s.onPostingsDoc(d)
	}
	s.index++
	if s.index >= len(s.docs) {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.doc = s.docs[s.index]
	s.start = s.starts[s.index]
	s.end = s.ends[s.index]
	s.position = s.start
	return s.doc, nil
}

func (s *BaseSpans) Advance(target int) (int, error) {
	if s.postingsBacked() {
		d, err := postingsAdvanceTo(s.postings, target)
		if err != nil {
			s.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, err
		}
		return s.onPostingsDoc(d)
	}
	for s.index < len(s.docs) {
		s.index++
		if s.index >= len(s.docs) {
			s.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		if s.docs[s.index] >= target {
			s.doc = s.docs[s.index]
			s.start = s.starts[s.index]
			s.end = s.ends[s.index]
			s.position = s.start
			return s.doc, nil
		}
	}
	s.doc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

func (s *BaseSpans) onPostingsDoc(d int) (int, error) {
	if d == index.NO_MORE_DOCS {
		s.doc = NO_MORE_DOCS
		s.position = -1
		return NO_MORE_DOCS, nil
	}
	freq, err := s.postings.Freq()
	if err != nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, err
	}
	s.doc = d
	s.freq = freq
	s.count = 0
	s.position = -1
	return s.doc, nil
}

func (s *BaseSpans) DocID() int {
	return s.doc
}

func (s *BaseSpans) NextStartPosition() (int, error) {
	if s.postingsBacked() {
		if s.count == s.freq {
			s.position = -1
			return -1, nil
		}
		pos, err := s.postings.NextPosition()
		if err != nil {
			return -1, err
		}
		s.position = pos
		s.count++
		return s.position, nil
	}
	if s.position < s.end {
		s.position++
		return s.position, nil
	}
	return -1, nil
}

func (s *BaseSpans) StartPosition() int {
	if s.postingsBacked() {
		return s.position
	}
	return s.start
}

func (s *BaseSpans) EndPosition() int {
	if s.postingsBacked() {
		if s.position == -1 {
			return -1
		}
		return s.position + 1
	}
	return s.end
}

func (s *BaseSpans) Freq() int {
	if s.postingsBacked() {
		return s.freq
	}
	return s.end - s.start
}

func (s *BaseSpans) Width() int {
	if s.postingsBacked() {
		return 0
	}
	return s.end - s.start
}

func (s *BaseSpans) Cost() int64 {
	if s.postingsBacked() {
		return s.postings.Cost()
	}
	return int64(len(s.docs))
}

func (s *BaseSpans) DocIDRunEnd() int {
	if s.doc == NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	if s.postingsBacked() {
		return s.doc + 1
	}
	end := s.doc + 1
	for i := s.index + 1; i < len(s.docs); i++ {
		if s.docs[i] == end {
			end++
		} else {
			break
		}
	}
	return end
}

func (s *BaseSpans) Collect(collector *SpanCollector) {
	for s.DocID() != NO_MORE_DOCS {
		start := s.StartPosition()
		for start != -1 {
			collector.AddSpan(s.DocID(), start, s.EndPosition())
			start = s.NextStartPosition()
		}
		s.NextDoc()
	}
}

func (s *BaseSpans) PositionsCost() float32 {
	if s.postingsBacked() {
		// This is a simplified estimate. In a real implementation,
		// we would use the term's actual cost.
		return 4.0
	}
	return 0.0
}

func (s *BaseSpans) AsTwoPhaseIterator() *TwoPhaseIterator {
	return NewTwoPhaseIterator(s)
}

// EmptySpans is a Spans with no documents.
var EmptySpans = &BaseSpans{
	doc:    NO_MORE_DOCS,
	docs:   []int{},
	starts: []int{},
	ends:   []int{},
	index:  -1,
}
