// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/util"
	"math"
)

// PostingsEnum provides an iterator over the postings (documents) for a term.
// This is the Go port of Lucene's org.apache.lucene.index.PostingsEnum.
type PostingsEnum interface {
	// DocIdSetIterator is the Java superclass: PostingsEnum extends
	// DocIdSetIterator (org.apache.lucene.index.PostingsEnum, Lucene 10.5.0).
	// It contributes DocID, NextDoc, Advance, Cost, IntoBitSet and DocIDRunEnd.
	DocIdSetIterator

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
}

// Impacts conveys information about upcoming impacts (i.e. (freq, norm)
// pairs that may trigger non-zero scores) within a postings list. Mirrors
// org.apache.lucene.index.Impacts from Apache Lucene 10.4.0.
type Impacts interface {
	// NumLevels returns the number of levels of impact summary information.
	// Always > 0 and may differ across positions in the same postings list.
	NumLevels() int

	// GetDocIDUpTo returns the maximum inclusive doc ID up to which the
	// impacts returned by GetImpacts(level) are valid. Non-decreasing in level.
	GetDocIDUpTo(level int) int

	// GetImpacts returns the (freq, norm) impacts for the given level. The
	// returned buffer is never empty and is only guaranteed to be valid until
	// the iterator advances.
	GetImpacts(level int) *util.FreqAndNormBuffer
}

// ImpactsSource produces Impacts and supports shallow-advance to allow callers
// to retrieve more precise impact information for upcoming docs. Mirrors
// org.apache.lucene.index.ImpactsSource from Apache Lucene 10.5.0.
type ImpactsSource interface {
	// AdvanceShallow shallow-advances to target. Cheaper than calling Advance
	// on the underlying iterator and lets subsequent GetImpacts calls ignore
	// doc IDs less than target.
	AdvanceShallow(target int) error

	// GetImpacts returns Impacts for upcoming doc IDs greater than or equal
	// to the maximum of the current docID and the last AdvanceShallow target.
	GetImpacts() (Impacts, error)
}

// ImpactsEnum is the PostingsEnum extension that also exposes ImpactsSource.
type ImpactsEnum interface {
	PostingsEnum
	ImpactsSource
}

const (
	// NO_MORE_DOCS is the doc ID returned by a DocIdSetIterator (and therefore
	// by PostingsEnum) once it is exhausted.
	//
	// Mirrors org.apache.lucene.search.DocIdSetIterator.NO_MORE_DOCS
	// (DocIdSetIterator.java:111), which is Integer.MAX_VALUE. It was -1 here,
	// contradicting Lucene as well as util.NO_MORE_DOCS and
	// index.DocIdSetIteratorNoMoreDocs, both of which already use math.MaxInt32.
	// With the two sentinels disagreeing, a postings enum produced through the
	// spi surface never compared equal to the exhaustion value the generic
	// iterator code tests for.
	NO_MORE_DOCS = math.MaxInt32

	// NO_MORE_POSITIONS is returned by PostingsEnum when there are no more positions.
	NO_MORE_POSITIONS = -1

	// PostingsFlagNone requests only doc IDs in postings.
	// Mirrors org.apache.lucene.index.PostingsEnum.NONE (value 0).
	PostingsFlagNone = 0

	// PostingsFlagFreqs requests term frequencies in postings.
	// Mirrors org.apache.lucene.index.PostingsEnum.FREQS (value 8).
	PostingsFlagFreqs = 1 << 3

	// PostingsFlagPositions requests term positions and frequencies.
	// Implies PostingsFlagFreqs. Mirrors PostingsEnum.POSITIONS (value 24).
	PostingsFlagPositions = PostingsFlagFreqs | 1<<4

	// PostingsFlagOffsets requests character offsets in postings.
	// Implies PostingsFlagPositions. Mirrors PostingsEnum.OFFSETS (value 56).
	PostingsFlagOffsets = PostingsFlagPositions | 1<<5

	// PostingsFlagPayloads requests payload bytes in postings.
	// Implies PostingsFlagPositions. Mirrors PostingsEnum.PAYLOADS
	// (PostingsEnum.java:58 = POSITIONS | 1 << 6, value 88).
	PostingsFlagPayloads = PostingsFlagPositions | 1<<6

	// PostingsFlagAll requests all available postings data.
	// Mirrors PostingsEnum.ALL (PostingsEnum.java:64 = OFFSETS | PAYLOADS,
	// value 120).
	PostingsFlagAll = PostingsFlagOffsets | PostingsFlagPayloads
)

// PostingsEnumBase provides a base implementation of the PostingsEnum interface.
//
// CurrentDoc is exported so that embedders in other packages can update
// the cached document ID directly via promotion (e.g. d.CurrentDoc++ or
// d.CurrentDoc = NO_MORE_DOCS) without needing dedicated mutation
// helpers. The field is otherwise an implementation detail: production
// code should treat it as write-only and rely on DocID() for reads.
type PostingsEnumBase struct {
	CurrentDoc int
}

// DocID returns the current document ID.
func (p *PostingsEnumBase) DocID() int {
	return p.CurrentDoc
}

// DocIDRunEnd returns the end of the current run of documents.
func (p *PostingsEnumBase) DocIDRunEnd() (int, error) {
	return p.DocID(), nil
}

// SetCurrentDoc updates the cached current document ID. Equivalent to
// assigning CurrentDoc directly; kept for callers that prefer a method.
func (p *PostingsEnumBase) SetCurrentDoc(docID int) {
	p.CurrentDoc = docID
}

// NewPostingsEnumBase builds a PostingsEnumBase positioned at the given
// initial document ID. Callers should normally pass -1 to indicate the
// pre-positioned state.
func NewPostingsEnumBase(initialDocID int) PostingsEnumBase {
	return PostingsEnumBase{CurrentDoc: initialDocID}
}

// EmptyPostingsEnum is a PostingsEnum with no postings.
type EmptyPostingsEnum struct {
	PostingsEnumBase
}

// NextDoc returns NO_MORE_DOCS.
func (e *EmptyPostingsEnum) NextDoc() (int, error) {
	e.CurrentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Advance returns NO_MORE_DOCS.
func (e *EmptyPostingsEnum) Advance(target int) (int, error) {
	e.CurrentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Freq returns 0.
func (e *EmptyPostingsEnum) Freq() (int, error) {
	return 0, nil
}

// DocIDRunEnd returns NO_MORE_DOCS.
func (e *EmptyPostingsEnum) DocIDRunEnd() (int, error) {
	return NO_MORE_DOCS, nil
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
		PostingsEnumBase: PostingsEnumBase{CurrentDoc: -1},
	}
}

// NextDoc advances to the document.
func (s *SingleDocPostingsEnum) NextDoc() (int, error) {
	if !s.positioned {
		s.positioned = true
		s.CurrentDoc = s.docID
		return s.docID, nil
	}
	s.CurrentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Advance advances to the document if it matches.
func (s *SingleDocPostingsEnum) Advance(target int) (int, error) {
	if !s.positioned && s.docID >= target {
		s.positioned = true
		s.CurrentDoc = s.docID
		return s.docID, nil
	}
	s.positioned = true
	s.CurrentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Freq returns the term frequency.
func (s *SingleDocPostingsEnum) Freq() (int, error) {
	if !s.positioned || s.CurrentDoc == NO_MORE_DOCS {
		return 0, nil
	}
	return s.freq, nil
}

// DocIDRunEnd returns the current doc ID.
func (s *SingleDocPostingsEnum) DocIDRunEnd() (int, error) {
	return s.DocID(), nil
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

// SinglePostingsEnum is a PostingsEnum for a single posting (used by SingleTermTerms).
type SinglePostingsEnum struct {
	PostingsEnumBase
	docFreq int
	freq    int
	count   int
}

// NewSinglePostingsEnum creates a new SinglePostingsEnum.
func NewSinglePostingsEnum(docFreq, freq int) *SinglePostingsEnum {
	return &SinglePostingsEnum{
		docFreq:          docFreq,
		freq:             freq,
		count:            0,
		PostingsEnumBase: PostingsEnumBase{CurrentDoc: -1},
	}
}

// NextDoc advances to the next document.
func (s *SinglePostingsEnum) NextDoc() (int, error) {
	if s.count == 0 {
		s.count = 1
		s.CurrentDoc = 0 // Single document at position 0
		return s.CurrentDoc, nil
	}
	s.CurrentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Advance advances to the document if it matches.
func (s *SinglePostingsEnum) Advance(target int) (int, error) {
	if s.count == 0 && target <= 0 {
		s.count = 1
		s.CurrentDoc = 0
		return 0, nil
	}
	s.count = 1
	s.CurrentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Freq returns the term frequency.
func (s *SinglePostingsEnum) Freq() (int, error) {
	if s.CurrentDoc == NO_MORE_DOCS {
		return 0, nil
	}
	return s.freq, nil
}

// NextPosition returns NO_MORE_POSITIONS.
func (s *SinglePostingsEnum) NextPosition() (int, error) {
	return NO_MORE_POSITIONS, nil
}

// StartOffset returns -1.
func (s *SinglePostingsEnum) StartOffset() (int, error) {
	return -1, nil
}

// EndOffset returns -1.
func (s *SinglePostingsEnum) EndOffset() (int, error) {
	return -1, nil
}

// GetPayload returns nil.
func (s *SinglePostingsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}

// Cost returns the docFreq.
func (s *SinglePostingsEnum) Cost() int64 {
	return int64(s.docFreq)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (e *EmptyPostingsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(e, upTo, bitSet, offset)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *SingleDocPostingsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *SinglePostingsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}
