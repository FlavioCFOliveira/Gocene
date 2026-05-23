// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentPostings holds the per-segment postings state for a single term
// during a merge operation.
//
// Port of the inner class STUniformSplitTermsWriter.SegmentPostings
// (Lucene 10.4.0).
type SegmentPostings struct {
	// SegmentIndex is the index of this segment within the merge.
	SegmentIndex int
	// TermState is the on-disk per-term state for this segment.
	TermState *codecs.BlockTermState
	// MergingBlockReader is the reader for this segment.
	MergingBlockReader *STMergingBlockReader
	// DocMap remaps old doc IDs to merged doc IDs.
	DocMap index.DocMap
}

// GetPostings returns a PostingsEnum for fieldName using the segment reader.
func (sp *SegmentPostings) GetPostings(fieldName string, reuse index.PostingsEnum, flags int) (index.PostingsEnum, error) {
	return sp.MergingBlockReader.Postings(fieldName, sp.TermState, reuse, flags)
}

// MultiSegmentsPostingsEnum combines PostingsEnums from multiple segments
// for a single term and field during merge.
//
// Port of STMergingTermsEnum.MultiSegmentsPostingsEnum (Lucene 10.4.0).
type MultiSegmentsPostingsEnum struct {
	reusablePostingsEnums []index.PostingsEnum
	segmentPostingsList   []*SegmentPostings
	segmentIndex          int
	postingsEnum          index.PostingsEnum
	postingsEnumExhausted bool
	docMap                index.DocMap
	docID                 int
	postingsFlags         int
}

// NewMultiSegmentsPostingsEnum allocates the multi-segment PostingsEnum.
func NewMultiSegmentsPostingsEnum(numSegments int) *MultiSegmentsPostingsEnum {
	return &MultiSegmentsPostingsEnum{
		reusablePostingsEnums: make([]index.PostingsEnum, numSegments),
	}
}

// Reset reinitialises the enum for a new term's segment posting list.
func (e *MultiSegmentsPostingsEnum) Reset(segmentPostingsList []*SegmentPostings) {
	e.segmentPostingsList = segmentPostingsList
	e.segmentIndex = -1
	e.postingsEnumExhausted = true
	e.docID = -1
}

// SetPostingFlags sets the flags for postings retrieval.
func (e *MultiSegmentsPostingsEnum) SetPostingFlags(flags int) {
	e.postingsFlags = flags
}

// DocID returns the current document ID.
func (e *MultiSegmentsPostingsEnum) DocID() int { return e.docID }

// NextDoc advances to the next document.
func (e *MultiSegmentsPostingsEnum) NextDoc() (int, error) {
	for {
		if e.postingsEnumExhausted {
			if e.segmentIndex == len(e.segmentPostingsList)-1 {
				e.docID = index.NO_MORE_DOCS
				return e.docID, nil
			}
			e.segmentIndex++
			sp := e.segmentPostingsList[e.segmentIndex]
			postings, err := sp.GetPostings("", e.reusablePostingsEnums[sp.SegmentIndex], e.postingsFlags)
			if err != nil {
				return 0, err
			}
			e.reusablePostingsEnums[sp.SegmentIndex] = postings
			e.postingsEnum = postings
			e.postingsEnumExhausted = false
			e.docMap = sp.DocMap
		}
		docID, err := e.postingsEnum.NextDoc()
		if err != nil {
			return 0, err
		}
		if docID == index.NO_MORE_DOCS {
			e.postingsEnumExhausted = true
			continue
		}
		mapped := e.docMap.Get(docID)
		if mapped != -1 {
			e.docID = mapped
			return e.docID, nil
		}
	}
}

// Advance is not supported.
func (e *MultiSegmentsPostingsEnum) Advance(target int) (int, error) {
	return 0, errors.New("MultiSegmentsPostingsEnum.Advance: not supported")
}

// Cost returns 0 (cost not useful during merge).
func (e *MultiSegmentsPostingsEnum) Cost() int64 { return 0 }

// Freq returns the frequency for the current document.
func (e *MultiSegmentsPostingsEnum) Freq() (int, error) { return e.postingsEnum.Freq() }

// NextPosition advances to the next position.
func (e *MultiSegmentsPostingsEnum) NextPosition() (int, error) {
	return e.postingsEnum.NextPosition()
}

// StartOffset returns the start offset of the current position.
func (e *MultiSegmentsPostingsEnum) StartOffset() (int, error) {
	return e.postingsEnum.StartOffset()
}

// EndOffset returns the end offset of the current position.
func (e *MultiSegmentsPostingsEnum) EndOffset() (int, error) {
	return e.postingsEnum.EndOffset()
}

// GetPayload returns the payload for the current position.
func (e *MultiSegmentsPostingsEnum) GetPayload() ([]byte, error) {
	return e.postingsEnum.GetPayload()
}

// compile-time assertion that MultiSegmentsPostingsEnum implements PostingsEnum.
var _ index.PostingsEnum = (*MultiSegmentsPostingsEnum)(nil)

// STMergingTermsEnum combines PostingsEnums for the same term from multiple
// segments for a given field.  Used during segment merging.
//
// Port of org.apache.lucene.codecs.uniformsplit.sharedterms.STMergingTermsEnum
// (Lucene 10.4.0).
type STMergingTermsEnum struct {
	// FieldName is the field being merged.
	FieldName string

	// multiPostingsEnum combines per-segment PostingsEnums.
	multiPostingsEnum *MultiSegmentsPostingsEnum

	// term is the current term.
	term *util.BytesRef
}

// NewSTMergingTermsEnum constructs an enum for the given field.
// numSegments is the number of segments in the merge.
func NewSTMergingTermsEnum(fieldName string, numSegments int) *STMergingTermsEnum {
	return &STMergingTermsEnum{
		FieldName:         fieldName,
		multiPostingsEnum: NewMultiSegmentsPostingsEnum(numSegments),
	}
}

// ResetTerm resets this enum with a new term and its per-segment postings.
// segmentPostings must be sorted by segment index.
func (e *STMergingTermsEnum) ResetTerm(term *util.BytesRef, segmentPostings []*SegmentPostings) {
	e.term = term
	e.multiPostingsEnum.Reset(segmentPostings)
}

// Term returns the current term.
func (e *STMergingTermsEnum) Term() *index.Term {
	if e.term == nil {
		return nil
	}
	return index.NewTerm(e.FieldName, string(e.term.Bytes[e.term.Offset:e.term.Offset+e.term.Length]))
}

// Next is not supported (enum is driven by the merge machinery via ResetTerm).
func (e *STMergingTermsEnum) Next() (*index.Term, error) {
	return nil, errors.New("STMergingTermsEnum.Next: not supported")
}

// SeekCeil is not supported.
func (e *STMergingTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return nil, errors.New("STMergingTermsEnum.SeekCeil: not supported")
}

// SeekExact is not supported.
func (e *STMergingTermsEnum) SeekExact(term *index.Term) (bool, error) {
	return false, errors.New("STMergingTermsEnum.SeekExact: not supported")
}

// DocFreq is not supported.
func (e *STMergingTermsEnum) DocFreq() (int, error) {
	return 0, errors.New("STMergingTermsEnum.DocFreq: not supported")
}

// TotalTermFreq is not supported.
func (e *STMergingTermsEnum) TotalTermFreq() (int64, error) {
	return 0, errors.New("STMergingTermsEnum.TotalTermFreq: not supported")
}

// Postings returns a combined PostingsEnum across all segments.
func (e *STMergingTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	e.multiPostingsEnum.SetPostingFlags(flags)
	return e.multiPostingsEnum, nil
}

// PostingsWithLiveDocs delegates to Postings.
func (e *STMergingTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// compile-time assertion.
var _ index.TermsEnum = (*STMergingTermsEnum)(nil)
