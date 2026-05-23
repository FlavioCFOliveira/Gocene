// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// OrdsFieldReader is BlockTree's implementation of Terms for the
// BlockTreeOrds postings format. It is constructed once per field by
// OrdsBlockTreeTermsReader and exposes the index.Terms surface for
// downstream consumers.
//
// Port of org.apache.lucene.codecs.blocktreeords.OrdsFieldReader
// (Lucene 10.4.0).
type OrdsFieldReader struct {
	parent           *OrdsBlockTreeTermsReader
	fieldInfo        *index.FieldInfo
	numTerms         int64
	sumTotalTermFreq int64
	sumDocFreq       int64
	docCount         int
	indexStartFP     int64
	rootBlockFP      int64
	rootCode         *FSTOrdsOutput
	minTerm          *util.BytesRef
	maxTerm          *util.BytesRef

	// index is the FST index loaded eagerly when indexIn is non-nil.
	index *gfst.FST[*FSTOrdsOutput]
}

// NewOrdsFieldReader constructs an OrdsFieldReader from the per-field
// metadata stored in the .tio index file.
//
// indexIn may be nil for fields that have no FST index block (e.g. a
// segment that was written without an index). When non-nil, the reader
// seeks to indexStartFP and materialises the FST immediately.
//
// Port of OrdsFieldReader constructor (Lucene 10.4.0).
func NewOrdsFieldReader(
	parent *OrdsBlockTreeTermsReader,
	fieldInfo *index.FieldInfo,
	numTerms int64,
	rootCode *FSTOrdsOutput,
	sumTotalTermFreq, sumDocFreq int64,
	docCount int,
	indexStartFP int64,
	indexIn store.IndexInput,
	minTerm, maxTerm *util.BytesRef,
) (*OrdsFieldReader, error) {
	if numTerms <= 0 {
		return nil, fmt.Errorf("OrdsFieldReader: numTerms must be > 0, got %d", numTerms)
	}
	if fieldInfo == nil {
		return nil, errors.New("OrdsFieldReader: fieldInfo must not be nil")
	}
	if rootCode == nil {
		return nil, errors.New("OrdsFieldReader: rootCode must not be nil")
	}

	// Derive rootBlockFP: read the first VLong from rootCode.bytes and
	// shift out the OUTPUT_FLAGS_NUM_BITS flag bits.
	rc := rootCode.Bytes
	badi := store.NewByteArrayDataInput(rc.Bytes[rc.Offset : rc.Offset+rc.Length])
	rootVLong, err := badi.ReadVLong()
	if err != nil {
		return nil, fmt.Errorf("OrdsFieldReader: reading rootBlockFP: %w", err)
	}
	rootBlockFP := rootVLong >> outputFlagsNumBits

	r := &OrdsFieldReader{
		parent:           parent,
		fieldInfo:        fieldInfo,
		numTerms:         numTerms,
		sumTotalTermFreq: sumTotalTermFreq,
		sumDocFreq:       sumDocFreq,
		docCount:         docCount,
		indexStartFP:     indexStartFP,
		rootBlockFP:      rootBlockFP,
		rootCode:         rootCode,
		minTerm:          minTerm,
		maxTerm:          maxTerm,
	}

	if indexIn != nil {
		clone := indexIn.Clone()
		if err := clone.SetPosition(indexStartFP); err != nil {
			return nil, fmt.Errorf("OrdsFieldReader: seek to %d: %w", indexStartFP, err)
		}
		meta, err := gfst.ReadMetadata[*FSTOrdsOutput](clone, FSTOutputs())
		if err != nil {
			return nil, fmt.Errorf("OrdsFieldReader: FST metadata: %w", err)
		}
		fst, err := gfst.NewFSTFromDataInput(meta, clone)
		if err != nil {
			return nil, fmt.Errorf("OrdsFieldReader: FST load: %w", err)
		}
		r.index = fst
	}
	return r, nil
}

// GetMin implements index.Terms.
func (r *OrdsFieldReader) GetMin() (*index.Term, error) {
	if r.minTerm == nil {
		// Older index that did not store min/maxTerm — fall back to
		// scanning via the enumerator.
		return nil, nil
	}
	return index.NewTerm(r.fieldInfo.Name(), string(r.minTerm.Bytes[r.minTerm.Offset:r.minTerm.Offset+r.minTerm.Length])), nil
}

// GetMax implements index.Terms.
func (r *OrdsFieldReader) GetMax() (*index.Term, error) {
	if r.maxTerm == nil {
		return nil, nil
	}
	return index.NewTerm(r.fieldInfo.Name(), string(r.maxTerm.Bytes[r.maxTerm.Offset:r.maxTerm.Offset+r.maxTerm.Length])), nil
}

// HasFreqs implements index.Terms.
func (r *OrdsFieldReader) HasFreqs() bool {
	return r.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs
}

// HasOffsets implements index.Terms.
func (r *OrdsFieldReader) HasOffsets() bool {
	return r.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

// HasPositions implements index.Terms.
func (r *OrdsFieldReader) HasPositions() bool {
	return r.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
}

// HasPayloads implements index.Terms.
func (r *OrdsFieldReader) HasPayloads() bool {
	return r.fieldInfo.HasPayloads()
}

// GetIterator implements index.Terms.  Returns an OrdsSegmentTermsEnum
// positioned before the first term.
func (r *OrdsFieldReader) GetIterator() (index.TermsEnum, error) {
	return NewOrdsSegmentTermsEnum(r, nil)
}

// GetIteratorWithSeek implements index.Terms.
func (r *OrdsFieldReader) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	var startKey *util.BytesRef
	if seekTerm != nil {
		startKey = seekTerm.BytesValue()
	}
	return NewOrdsSegmentTermsEnum(r, startKey)
}

// GetPostingsReader implements index.Terms. Returns an empty PostingsEnum
// because PostingsReaderBase wiring is deferred until OrdsBlockTreeTermsReader
// is fully integrated.
func (r *OrdsFieldReader) GetPostingsReader(_ string, _ int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// Size implements index.Terms.
func (r *OrdsFieldReader) Size() int64 { return r.numTerms }

// GetSumTotalTermFreq implements index.Terms.
func (r *OrdsFieldReader) GetSumTotalTermFreq() (int64, error) { return r.sumTotalTermFreq, nil }

// GetSumDocFreq implements index.Terms.
func (r *OrdsFieldReader) GetSumDocFreq() (int64, error) { return r.sumDocFreq, nil }

// GetDocCount implements index.Terms.
func (r *OrdsFieldReader) GetDocCount() (int, error) { return r.docCount, nil }

// Intersect returns an OrdsIntersectTermsEnum accepting terms matched by
// the given CompiledAutomaton, starting at startTerm (may be nil).
//
// Port of OrdsFieldReader.intersect(CompiledAutomaton, BytesRef).
func (r *OrdsFieldReader) Intersect(compiled *automaton.CompiledAutomaton, startTerm *util.BytesRef) (index.TermsEnum, error) {
	if compiled == nil {
		return nil, errors.New("OrdsFieldReader.Intersect: compiled must not be nil")
	}
	return NewOrdsIntersectTermsEnum(r, compiled, startTerm)
}

// String implements fmt.Stringer.
func (r *OrdsFieldReader) String() string {
	return fmt.Sprintf("OrdsBlockTreeTerms(terms=%d,postings=%d,positions=%d,docs=%d)",
		r.numTerms, r.sumDocFreq, r.sumTotalTermFreq, r.docCount)
}

// compile-time assertion.
var _ index.Terms = (*OrdsFieldReader)(nil)
