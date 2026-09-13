// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsEnum provides an iterator over the terms dictionary for a field.
// This is the Go port of Lucene's org.apache.lucene.index.TermsEnum.
type TermsEnum interface {
	// Attributes returns the related attributes.
	//
	// Mirrors the abstract org.apache.lucene.index.TermsEnum#attributes(),
	// whose Java signature is {@code public abstract AttributeSource
	// attributes()}. Every enumerator has one: Lucene supplies the default
	// body on BaseTermsEnum, which lazily creates a single AttributeSource
	// and returns it thereafter. [TermsEnumBase] carries that same default
	// here, so embedding it is enough.
	Attributes() *util.AttributeSource

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

	// Ord returns the ordinal of the current term.
	// Returns -1 if not positioned or at the end.
	Ord() int64

	// DocFreq returns the number of documents containing the current term.
	DocFreq() (int, error)

	// TotalTermFreq returns the total number of occurrences of the current term.
	// Returns -1 if frequencies were not indexed.
	TotalTermFreq() (int64, error)

	// Postings returns a PostingsEnum for the current term.
	// The flags parameter controls what information is returned.
	Postings(flags int) (PostingsEnum, error)

	// Impacts returns an ImpactsEnum for the current term.
	// The flags parameter controls what information is returned.
	Impacts(flags int) (ImpactsEnum, error)

	// PostingsWithLiveDocs returns a PostingsEnum for the current term,
	// with live docs applied.
	PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error)
}

// TermsEnumBase provides a base implementation of the TermsEnum interface.
type TermsEnumBase struct {
	currentTerm *Term
	// atts mirrors the private AttributeSource field of
	// org.apache.lucene.index.BaseTermsEnum: it stays nil until the first
	// Attributes() call and is reused for every call thereafter.
	atts *util.AttributeSource
}

// Attributes returns the related attributes, reproducing the default body of
// org.apache.lucene.index.BaseTermsEnum#attributes() in Apache Lucene 10.5.0:
//
//	if (atts == null) { atts = new AttributeSource(); }
//	return atts;
func (t *TermsEnumBase) Attributes() *util.AttributeSource {
	if t.atts == nil {
		t.atts = util.NewAttributeSource()
	}
	return t.atts
}

// Term returns the current term.
func (t *TermsEnumBase) Term() *Term {
	return t.currentTerm
}

func (t *TermsEnumBase) Ord() int64 {
	return -1
}

// SetCurrentTerm sets the cached current term for embedded TermsEnum
// implementations that need to update their reported position without
// reaching into the (unexported) currentTerm field directly.
//
// Embedders should call SetCurrentTerm(nil) to signal an exhausted enum
// and SetCurrentTerm(term) when advancing to a fresh term.
func (t *TermsEnumBase) SetCurrentTerm(term *Term) {
	t.currentTerm = term
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

func (e *EmptyTermsEnum) Ord() int64 {
	return -1
}

// TotalTermFreq returns 0.
func (e *EmptyTermsEnum) TotalTermFreq() (int64, error) {
	return 0, nil
}

// Postings returns an empty PostingsEnum.
func (e *EmptyTermsEnum) Postings(flags int) (PostingsEnum, error) {
	return &EmptyPostingsEnum{}, nil
}

func (e *EmptyTermsEnum) Impacts(flags int) (ImpactsEnum, error) {
	return nil, nil
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

func (s *SingleTermsEnum) Ord() int64 {
	if !s.positioned {
		return -1
	}
	return 0
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

func (s *SingleTermsEnum) Impacts(flags int) (ImpactsEnum, error) {
	return nil, nil
}

// PostingsWithLiveDocs returns a PostingsEnum for the single term.
func (s *SingleTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	if !s.positioned {
		return nil, nil
	}
	return &EmptyPostingsEnum{}, nil
}
