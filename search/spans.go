// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// GC-1002: Spans iterator interface
// Spans is an iterator over span matches for (doc, start, end) tuples.
// This is the Go port of Lucene's org.apache.lucene.search.spans.Spans.
//
// Spans has two backends. The array backend (docs/starts/ends) replays a
// pre-computed list of single-position spans and is used by callers that build
// spans eagerly. The postings backend (postings != nil) iterates a term's
// PostingsEnum lazily and faithfully reproduces Lucene's TermSpans: one span
// per term occurrence, with zero width and endPosition == startPosition + 1.
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

	// postings, when non-nil, switches Spans into TermSpans mode: positions are
	// drained from the term's PostingsEnum rather than the array backend.
	postings index.PostingsEnum
	count    int // number of positions consumed in the current document
}

// NewSpans creates a new array-backed Spans iterator.
func NewSpans(docs []int, starts []int, ends []int) *Spans {
	return &Spans{
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
func NewTermSpans(postings index.PostingsEnum) *Spans {
	return &Spans{
		doc:      -1,
		position: -1,
		postings: postings,
		index:    -1,
	}
}

// postingsBacked reports whether this Spans drains positions from a PostingsEnum.
func (s *Spans) postingsBacked() bool {
	return s.postings != nil
}

// NextDoc advances to the next document.
func (s *Spans) NextDoc() (int, error) {
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

// Advance advances to the specified document.
func (s *Spans) Advance(target int) (int, error) {
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

// onPostingsDoc resets the position state after the underlying PostingsEnum
// settles on a new document, mirroring TermSpans.nextDoc/advance.
func (s *Spans) onPostingsDoc(d int) (int, error) {
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

// DocID returns the current document ID.
func (s *Spans) DocID() int {
	return s.doc
}

// NextStartPosition advances to the next start position. It returns -1
// (Lucene's Spans.NO_MORE_POSITIONS) once every occurrence has been consumed.
func (s *Spans) NextStartPosition() (int, error) {
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

// StartPosition returns the start position.
func (s *Spans) StartPosition() int {
	if s.postingsBacked() {
		return s.position
	}
	return s.start
}

// EndPosition returns the end position. For the postings (TermSpans) backend it
// is startPosition + 1, matching Lucene's TermSpans.endPosition().
func (s *Spans) EndPosition() int {
	if s.postingsBacked() {
		if s.position == -1 {
			return -1
		}
		return s.position + 1
	}
	return s.end
}

// Freq returns the frequency of spans in the current document.
func (s *Spans) Freq() int {
	if s.postingsBacked() {
		return s.freq
	}
	return s.end - s.start
}

// Width returns the width of the current span. The postings (TermSpans) backend
// always reports 0, matching Lucene's TermSpans.width().
func (s *Spans) Width() int {
	if s.postingsBacked() {
		return 0
	}
	return s.end - s.start
}

// Cost returns the estimated cost of iterating through all documents.
func (s *Spans) Cost() int64 {
	if s.postingsBacked() {
		return s.postings.Cost()
	}
	return int64(len(s.docs))
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *Spans) DocIDRunEnd() int {
	if s.doc == NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	if s.postingsBacked() {
		return s.doc + 1
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
