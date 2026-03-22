// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// GC-1002: Spans iterator interface
// Spans is an iterator over span matches for (doc, start, end) tuples.
// This is the Go port of Lucene's org.apache.lucene.search.spans.Spans.
type Spans struct {
	doc      int
	freq     int
	position int
	start    int
	end      int
	docs     []int
	starts   []int
	ends     []int
	index    int
}

// NewSpans creates a new Spans iterator.
func NewSpans(docs []int, starts []int, ends []int) *Spans {
	return &Spans{
		doc:    -1,
		docs:   docs,
		starts: starts,
		ends:   ends,
		index:  -1,
	}
}

// NextDoc advances to the next document.
func (s *Spans) NextDoc() (int, error) {
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

// Advance advances to the specified document.
func (s *Spans) Advance(target int) (int, error) {
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

// DocID returns the current document ID.
func (s *Spans) DocID() int {
	return s.doc
}

// NextStartPosition advances to the next start position.
func (s *Spans) NextStartPosition() (int, error) {
	if s.position < s.end {
		s.position++
		return s.position, nil
	}
	return -1, nil
}

// StartPosition returns the start position.
func (s *Spans) StartPosition() int {
	return s.start
}

// EndPosition returns the end position.
func (s *Spans) EndPosition() int {
	return s.end
}

// Freq returns the frequency of spans in the current document.
func (s *Spans) Freq() int {
	return s.end - s.start
}

// Width returns the width of the current span.
func (s *Spans) Width() int {
	return s.end - s.start
}

// Cost returns the estimated cost of iterating through all documents.
func (s *Spans) Cost() int64 {
	return int64(len(s.docs))
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *Spans) DocIDRunEnd() int {
	if s.doc == NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	// Find the end of consecutive doc IDs
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

// Ensure Spans implements DocIdSetIterator
var _ DocIdSetIterator = (*Spans)(nil)

// EmptySpans is a Spans with no documents.
var EmptySpans = &Spans{
	doc:    NO_MORE_DOCS,
	docs:   []int{},
	starts: []int{},
	ends:   []int{},
	index:  -1,
}
