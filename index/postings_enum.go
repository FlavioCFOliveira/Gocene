// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// PostingsEnum provides an iterator over the postings (documents) for a term.
// This is the Go port of Lucene's org.apache.lucene.index.PostingsEnum.
type PostingsEnum interface {
	// NextDoc advances to the next document in the postings list.
	// Returns the doc ID or -1 (NO_MORE_DOCS) if there are no more documents.
	NextDoc() (int, error)

	// Advance advances to the first document with doc ID >= target.
	// Returns the doc ID or -1 (NO_MORE_DOCS) if there are no more documents.
	Advance(target int) (int, error)

	// DocID returns the current document ID.
	// Returns -1 if the iterator is not positioned or -2 (NO_MORE_DOCS) at the end.
	DocID() int

	// Freq returns the term frequency in the current document.
	// This is the number of occurrences of the term in the current document.
	Freq() (int, error)

	// NextPosition advances to the next occurrence of the term in the current document.
	// Returns the position or -1 if there are no more positions.
	NextPosition() (int, error)

	// StartOffset returns the start character offset of the current occurrence.
	// Returns -1 if offsets were not indexed.
	StartOffset() (int, error)

	// EndOffset returns the end character offset of the current occurrence.
	// Returns -1 if offsets were not indexed.
	EndOffset() (int, error)

	// GetPayload returns the payload bytes for the current occurrence.
	// Returns nil if there is no payload.
	GetPayload() ([]byte, error)

	// Cost returns an estimate of the cost of iterating over all postings.
	// Higher values indicate higher cost.
	Cost() int64
}

const (
	// NO_MORE_DOCS is returned by PostingsEnum when there are no more documents.
	NO_MORE_DOCS = -1

	// NO_MORE_POSITIONS is returned by PostingsEnum when there are no more positions.
	NO_MORE_POSITIONS = -1
)

// PostingsEnumBase provides a base implementation of the PostingsEnum interface.
type PostingsEnumBase struct {
	currentDoc int
}

// DocID returns the current document ID.
func (p *PostingsEnumBase) DocID() int {
	return p.currentDoc
}

// EmptyPostingsEnum is a PostingsEnum with no postings.
type EmptyPostingsEnum struct {
	PostingsEnumBase
}

// NextDoc returns NO_MORE_DOCS.
func (e *EmptyPostingsEnum) NextDoc() (int, error) {
	e.currentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Advance returns NO_MORE_DOCS.
func (e *EmptyPostingsEnum) Advance(target int) (int, error) {
	e.currentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Freq returns 0.
func (e *EmptyPostingsEnum) Freq() (int, error) {
	return 0, nil
}

// NextPosition returns NO_MORE_POSITIONS.
func (e *EmptyPostingsEnum) NextPosition() (int, error) {
	return NO_MORE_POSITIONS, nil
}

// StartOffset returns -1.
func (e *EmptyPostingsEnum) StartOffset() (int, error) {
	return -1, nil
}

// EndOffset returns -1.
func (e *EmptyPostingsEnum) EndOffset() (int, error) {
	return -1, nil
}

// GetPayload returns nil.
func (e *EmptyPostingsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}

// Cost returns 0.
func (e *EmptyPostingsEnum) Cost() int64 {
	return 0
}

// SingleDocPostingsEnum is a PostingsEnum for a single document.
type SingleDocPostingsEnum struct {
	PostingsEnumBase
	docID      int
	freq       int
	positioned bool
}

// NewSingleDocPostingsEnum creates a new SingleDocPostingsEnum.
func NewSingleDocPostingsEnum(docID, freq int) *SingleDocPostingsEnum {
	return &SingleDocPostingsEnum{
		docID:            docID,
		freq:             freq,
		positioned:       false,
		PostingsEnumBase: PostingsEnumBase{currentDoc: -1},
	}
}

// NextDoc advances to the document.
func (s *SingleDocPostingsEnum) NextDoc() (int, error) {
	if !s.positioned {
		s.positioned = true
		s.currentDoc = s.docID
		return s.docID, nil
	}
	s.currentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Advance advances to the document if it matches.
func (s *SingleDocPostingsEnum) Advance(target int) (int, error) {
	if !s.positioned && s.docID >= target {
		s.positioned = true
		s.currentDoc = s.docID
		return s.docID, nil
	}
	s.positioned = true
	s.currentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Freq returns the term frequency.
func (s *SingleDocPostingsEnum) Freq() (int, error) {
	if !s.positioned || s.currentDoc == NO_MORE_DOCS {
		return 0, nil
	}
	return s.freq, nil
}

// NextPosition returns NO_MORE_POSITIONS.
func (s *SingleDocPostingsEnum) NextPosition() (int, error) {
	return NO_MORE_POSITIONS, nil
}

// StartOffset returns -1.
func (s *SingleDocPostingsEnum) StartOffset() (int, error) {
	return -1, nil
}

// EndOffset returns -1.
func (s *SingleDocPostingsEnum) EndOffset() (int, error) {
	return -1, nil
}

// GetPayload returns nil.
func (s *SingleDocPostingsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}

// Cost returns 1.
func (s *SingleDocPostingsEnum) Cost() int64 {
	return 1
}
