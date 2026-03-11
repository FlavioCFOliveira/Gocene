// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// Terms provides access to the terms dictionary for a specific field.
// This is the Go port of Lucene's org.apache.lucene.index.Terms.
//
// Terms represents the collection of all terms for a given field in an index.
// It provides statistics about the terms and allows iteration over them.
type Terms interface {
	// GetIterator returns a TermsEnum for iterating over all terms in this field.
	// The returned TermsEnum is positioned before the first term.
	// Use TermsEnum.Next() to advance to the first term.
	GetIterator() (TermsEnum, error)

	// GetIteratorWithSeek returns a TermsEnum positioned at or after the given term.
	// If the term exists, the iterator is positioned at that term.
	// If the term doesn't exist, the iterator is positioned at the next term.
	// Returns nil TermsEnum if there are no terms on or after the given term.
	GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error)

	// Size returns the number of unique terms in this field.
	// Returns -1 if the size is unknown (e.g., for multi-segment indexes).
	Size() int64

	// GetDocCount returns the number of documents that contain at least one
	// occurrence of any term in this field.
	// This is the sum of docFreq across all unique terms, not the count of unique terms.
	GetDocCount() (int, error)

	// GetSumDocFreq returns the total number of postings for this field.
	// This is the sum of docFreq (number of documents containing each term)
	// across all terms.
	// Returns -1 if this statistic is not available.
	GetSumDocFreq() (int64, error)

	// GetSumTotalTermFreq returns the total number of term occurrences for this field.
	// This is the sum of totalTermFreq (total number of occurrences of each term)
	// across all terms.
	// Returns -1 if positions are not indexed (e.g., IndexOptions.DOCS_ONLY).
	GetSumTotalTermFreq() (int64, error)

	// HasFreqs returns true if term frequencies (tf) are available for this field.
	// This requires IndexOptions.DOCS_AND_FREQS or higher.
	HasFreqs() bool

	// HasOffsets returns true if term offsets are available for this field.
	// This requires IndexOptions.DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS.
	HasOffsets() bool

	// HasPositions returns true if term positions are available for this field.
	// This requires IndexOptions.DOCS_AND_FREQS_AND_POSITIONS or higher.
	HasPositions() bool

	// HasPayloads returns true if payloads are available for this field.
	HasPayloads() bool

	// GetMin returns the smallest term in this field (lexicographically first).
	// Returns nil if there are no terms.
	GetMin() (*Term, error)

	// GetMax returns the largest term in this field (lexicographically last).
	// Returns nil if there are no terms.
	GetMax() (*Term, error)
}

// TermsBase provides a base implementation of the Terms interface.
// Embed this struct in your custom Terms implementation to get default
// implementations for common methods.
type TermsBase struct{}

// Size returns -1 by default (unknown size).
func (t *TermsBase) Size() int64 {
	return -1
}

// GetDocCount returns 0 by default.
func (t *TermsBase) GetDocCount() (int, error) {
	return 0, nil
}

// GetSumDocFreq returns -1 by default (unknown).
func (t *TermsBase) GetSumDocFreq() (int64, error) {
	return -1, nil
}

// GetSumTotalTermFreq returns -1 by default (unknown).
func (t *TermsBase) GetSumTotalTermFreq() (int64, error) {
	return -1, nil
}

// HasFreqs returns false by default.
func (t *TermsBase) HasFreqs() bool {
	return false
}

// HasOffsets returns false by default.
func (t *TermsBase) HasOffsets() bool {
	return false
}

// HasPositions returns false by default.
func (t *TermsBase) HasPositions() bool {
	return false
}

// HasPayloads returns false by default.
func (t *TermsBase) HasPayloads() bool {
	return false
}

// TermsStats holds statistics for a Terms instance.
// This is useful for passing around term statistics without
// holding a reference to the full Terms object.
type TermsStats struct {
	// DocCount is the number of documents containing at least one term
	DocCount int

	// SumDocFreq is the sum of docFreq across all terms
	SumDocFreq int64

	// SumTotalTermFreq is the sum of totalTermFreq across all terms
	SumTotalTermFreq int64

	// TermCount is the number of unique terms
	TermCount int64
}

// EmptyTerms is a Terms implementation with no terms.
// Useful for fields that exist but have no indexed terms.
type EmptyTerms struct {
	TermsBase
}

// GetIterator returns an empty TermsEnum.
func (e *EmptyTerms) GetIterator() (TermsEnum, error) {
	return &EmptyTermsEnum{}, nil
}

// GetIteratorWithSeek returns an empty TermsEnum.
func (e *EmptyTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	return &EmptyTermsEnum{}, nil
}

// GetMin returns nil.
func (e *EmptyTerms) GetMin() (*Term, error) {
	return nil, nil
}

// GetMax returns nil.
func (e *EmptyTerms) GetMax() (*Term, error) {
	return nil, nil
}

// Size returns 0.
func (e *EmptyTerms) Size() int64 {
	return 0
}

// SingleTermTerms is a Terms implementation containing exactly one term.
// This is useful for testing and for special cases.
type SingleTermTerms struct {
	TermsBase
	term      *Term
	docFreq   int
	totalFreq int64
}

// NewSingleTermTerms creates a new SingleTermTerms with the given term.
func NewSingleTermTerms(term *Term, docFreq int, totalFreq int64) *SingleTermTerms {
	return &SingleTermTerms{
		term:      term,
		docFreq:   docFreq,
		totalFreq: totalFreq,
	}
}

// GetIterator returns a TermsEnum for the single term.
func (s *SingleTermTerms) GetIterator() (TermsEnum, error) {
	return NewSingleTermsEnum(s.term, s.docFreq, s.totalFreq), nil
}

// GetIteratorWithSeek returns a TermsEnum positioned at the given term or after.
func (s *SingleTermTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	if seekTerm == nil {
		return s.GetIterator()
	}
	cmp := s.term.CompareTo(seekTerm)
	if cmp < 0 {
		// seek term is after our single term, so nothing matches
		return &EmptyTermsEnum{}, nil
	}
	// our term is at or after the seek term
	return s.GetIterator()
}

// Size returns 1.
func (s *SingleTermTerms) Size() int64 {
	return 1
}

// GetDocCount returns 1 (one document contains this term).
func (s *SingleTermTerms) GetDocCount() (int, error) {
	return 1, nil
}

// GetSumDocFreq returns the docFreq of the single term.
func (s *SingleTermTerms) GetSumDocFreq() (int64, error) {
	return int64(s.docFreq), nil
}

// GetSumTotalTermFreq returns the total frequency of the single term.
func (s *SingleTermTerms) GetSumTotalTermFreq() (int64, error) {
	return s.totalFreq, nil
}

// GetMin returns the single term.
func (s *SingleTermTerms) GetMin() (*Term, error) {
	return s.term, nil
}

// GetMax returns the single term.
func (s *SingleTermTerms) GetMax() (*Term, error) {
	return s.term, nil
}
