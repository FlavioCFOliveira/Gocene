// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// OrdsSegmentTermsEnum iterates over the terms of one field in a segment
// written by the BlockTreeOrds postings format, tracking term ordinals.
//
// Port of org.apache.lucene.codecs.blocktreeords.OrdsSegmentTermsEnum
// (Lucene 10.4.0, ~1298 lines). Full traversal logic is deferred to
// sprint task 3189 (OrdsSegmentTermsEnumFrame) and beyond.
type OrdsSegmentTermsEnum struct {
	reader   *OrdsFieldReader
	startKey *util.BytesRef
}

// NewOrdsSegmentTermsEnum constructs an enum for the given field reader.
// startKey positions the enum before the first call to Next(); it may be nil.
func NewOrdsSegmentTermsEnum(r *OrdsFieldReader, startKey *util.BytesRef) (*OrdsSegmentTermsEnum, error) {
	return &OrdsSegmentTermsEnum{reader: r, startKey: startKey}, nil
}

// Next advances to the next term.
func (e *OrdsSegmentTermsEnum) Next() (*index.Term, error) { return nil, nil }

// SeekCeil seeks to term or the next term after it.
func (e *OrdsSegmentTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return nil, nil
}

// SeekExact seeks to the exact term.
func (e *OrdsSegmentTermsEnum) SeekExact(term *index.Term) (bool, error) { return false, nil }

// Term returns the current term.
func (e *OrdsSegmentTermsEnum) Term() *index.Term { return nil }

// DocFreq returns the document frequency of the current term.
func (e *OrdsSegmentTermsEnum) DocFreq() (int, error) { return 0, nil }

// TotalTermFreq returns the total term frequency of the current term.
func (e *OrdsSegmentTermsEnum) TotalTermFreq() (int64, error) { return 0, nil }

// Postings returns a PostingsEnum for the current term.
func (e *OrdsSegmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term filtered by live docs.
func (e *OrdsSegmentTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// compile-time assertion.
var _ index.TermsEnum = (*OrdsSegmentTermsEnum)(nil)
