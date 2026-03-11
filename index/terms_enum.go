// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsEnum provides an iterator over the terms dictionary for a field.
// This is the Go port of Lucene's org.apache.lucene.index.TermsEnum.
type TermsEnum interface {
	// Next advances to the next term in the enumeration.
	// Returns the term or nil if the end has been reached.
	Next() (*Term, error)

	// SeekCeil seeks to the specified term or, if the term doesn't exist,
	// to the next term after it (ceiling).
	// Returns the current term or nil if the end has been reached.
	SeekCeil(term *Term) (*Term, error)

	// SeekExact seeks to the specified term.
	// Returns true if the term was found, false otherwise.
	SeekExact(term *Term) (bool, error)

	// Term returns the current term in the enumeration.
	// Returns nil if not positioned or at the end.
	Term() *Term

	// DocFreq returns the number of documents containing the current term.
	DocFreq() (int, error)

	// TotalTermFreq returns the total number of occurrences of the current term.
	// Returns -1 if frequencies were not indexed.
	TotalTermFreq() (int64, error)

	// Postings returns a PostingsEnum for the current term.
	// The flags parameter controls what information is returned.
	Postings(flags int) (PostingsEnum, error)

	// PostingsWithLiveDocs returns a PostingsEnum for the current term,
	// with live docs applied.
	PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error)
}

// TermsEnumBase provides a base implementation of the TermsEnum interface.
type TermsEnumBase struct {
	currentTerm *Term
}

// Term returns the current term.
func (t *TermsEnumBase) Term() *Term {
	return t.currentTerm
}

// EmptyTermsEnum is a TermsEnum with no terms.
type EmptyTermsEnum struct {
	TermsEnumBase
}

// Next returns nil (no terms).
func (e *EmptyTermsEnum) Next() (*Term, error) {
	return nil, nil
}

// SeekCeil returns nil (no terms).
func (e *EmptyTermsEnum) SeekCeil(term *Term) (*Term, error) {
	return nil, nil
}

// SeekExact returns false (term not found).
func (e *EmptyTermsEnum) SeekExact(term *Term) (bool, error) {
	return false, nil
}

// DocFreq returns 0.
func (e *EmptyTermsEnum) DocFreq() (int, error) {
	return 0, nil
}

// TotalTermFreq returns 0.
func (e *EmptyTermsEnum) TotalTermFreq() (int64, error) {
	return 0, nil
}

// Postings returns an empty PostingsEnum.
func (e *EmptyTermsEnum) Postings(flags int) (PostingsEnum, error) {
	return &EmptyPostingsEnum{}, nil
}

// PostingsWithLiveDocs returns an empty PostingsEnum.
func (e *EmptyTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return &EmptyPostingsEnum{}, nil
}

// SingleTermsEnum is a TermsEnum over a single term.
type SingleTermsEnum struct {
	TermsEnumBase
	term       *Term
	docFreq    int
	totalFreq  int64
	positioned bool
}

// NewSingleTermsEnum creates a new SingleTermsEnum.
func NewSingleTermsEnum(term *Term, docFreq int, totalFreq int64) *SingleTermsEnum {
	return &SingleTermsEnum{
		term:       term,
		docFreq:    docFreq,
		totalFreq:  totalFreq,
		positioned: false,
	}
}

// Next advances to the term.
func (s *SingleTermsEnum) Next() (*Term, error) {
	if !s.positioned {
		s.positioned = true
		s.currentTerm = s.term
		return s.term, nil
	}
	// Already returned the single term
	return nil, nil
}

// SeekCeil seeks to the single term if it matches or is after.
func (s *SingleTermsEnum) SeekCeil(seekTerm *Term) (*Term, error) {
	if seekTerm == nil {
		s.positioned = true
		s.currentTerm = s.term
		return s.term, nil
	}

	cmp := s.term.CompareTo(seekTerm)
	if cmp >= 0 {
		// Our term is at or after the seek term
		s.positioned = true
		s.currentTerm = s.term
		return s.term, nil
	}
	// Our term is before the seek term, nothing matches
	return nil, nil
}

// SeekExact seeks to the exact term.
func (s *SingleTermsEnum) SeekExact(seekTerm *Term) (bool, error) {
	if seekTerm == nil {
		return false, nil
	}

	if s.term.Equals(seekTerm) {
		s.positioned = true
		s.currentTerm = s.term
		return true, nil
	}
	return false, nil
}

// DocFreq returns the document frequency.
func (s *SingleTermsEnum) DocFreq() (int, error) {
	if !s.positioned {
		return 0, nil
	}
	return s.docFreq, nil
}

// TotalTermFreq returns the total term frequency.
func (s *SingleTermsEnum) TotalTermFreq() (int64, error) {
	if !s.positioned {
		return 0, nil
	}
	return s.totalFreq, nil
}

// Postings returns a PostingsEnum for the single term.
func (s *SingleTermsEnum) Postings(flags int) (PostingsEnum, error) {
	if !s.positioned {
		return nil, nil
	}
	return &EmptyPostingsEnum{}, nil
}

// PostingsWithLiveDocs returns a PostingsEnum for the single term.
func (s *SingleTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	if !s.positioned {
		return nil, nil
	}
	return &EmptyPostingsEnum{}, nil
}
