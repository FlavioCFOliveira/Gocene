// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// SegmentTermsEnum iterates through all terms in a single field of the
// Lucene 4.0 block-tree terms dictionary.
//
// Port of
// org.apache.lucene.backward_codecs.lucene40.blocktree.SegmentTermsEnum
// (Lucene 10.4.0).
//
// The full block-loading and navigation logic (loadBlock, nextEntry,
// seekCeil/seekExact/next implementations) is deferred to a later sprint.
// Until then all navigation methods return ErrBlockTraversalNotAvailable.
type SegmentTermsEnum struct {
	index.TermsEnumBase

	// in is the lazily-initialised clone of the terms file.
	in store.IndexInput

	stack       []*segmentTermsEnumFrame
	staticFrame *segmentTermsEnumFrame

	// currentFrame is the active frame.
	currentFrame *segmentTermsEnumFrame

	// termExists is true when the current term is a real term (not a
	// sub-block pointer).
	termExists bool

	// fr is the owning FieldReader.
	fr *FieldReader

	targetBeforeCurrentLength int

	scratchReader *store.ByteArrayDataInput

	// validIndexPrefix is the length of the prefix that was confirmed by
	// the FST index during the last seekCeil/seekExact.
	validIndexPrefix int

	term      util.BytesRefBuilder
	fstReader fst.BytesReader

	arcs []*fst.Arc[*util.BytesRef]
}

// newSegmentTermsEnum constructs a SegmentTermsEnum for the given FieldReader.
//
// Port of SegmentTermsEnum(FieldReader).
func newSegmentTermsEnum(fr *FieldReader) (*SegmentTermsEnum, error) {
	e := &SegmentTermsEnum{
		fr:            fr,
		stack:         make([]*segmentTermsEnumFrame, 0),
		arcs:          make([]*fst.Arc[*util.BytesRef], 1),
		scratchReader: store.NewByteArrayDataInput(nil),
	}

	e.staticFrame = newSegmentTermsEnumFrame(e, -1)

	if fr.fstIndex == nil {
		e.fstReader = nil
	} else {
		e.fstReader = fr.fstIndex.GetBytesReader()
	}

	for i := range e.arcs {
		e.arcs[i] = &fst.Arc[*util.BytesRef]{}
	}

	e.currentFrame = e.staticFrame

	if fr.fstIndex != nil {
		_ = fr.fstIndex.GetFirstArc(e.arcs[0])
	}

	e.validIndexPrefix = 0
	return e, nil
}

// initIndexInput lazily clones the terms file input.
func (e *SegmentTermsEnum) initIndexInput() {
	if e.in == nil {
		e.in = e.fr.parent.termsIn.Clone()
	}
}

// Term returns the current term.
func (e *SegmentTermsEnum) Term() *index.Term {
	b := e.term.Get()
	if b == nil || b.Length == 0 {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), b)
}

// Next advances to the next term.
//
// Deferred: requires full block traversal logic.
func (e *SegmentTermsEnum) Next() (*index.Term, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// SeekCeil seeks to the given term or the next term after it.
//
// Deferred: requires full block traversal logic.
func (e *SegmentTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// SeekExact seeks to the given term exactly.
//
// Deferred: requires full block traversal logic.
func (e *SegmentTermsEnum) SeekExact(term *index.Term) (bool, error) {
	return false, ErrBlockTraversalNotAvailable
}

// DocFreq returns the document frequency of the current term.
//
// Deferred: requires metadata decoding from the current block frame.
func (e *SegmentTermsEnum) DocFreq() (int, error) {
	return 0, ErrBlockTraversalNotAvailable
}

// TotalTermFreq returns the total frequency of the current term.
//
// Deferred: requires metadata decoding from the current block frame.
func (e *SegmentTermsEnum) TotalTermFreq() (int64, error) {
	return 0, ErrBlockTraversalNotAvailable
}

// Postings returns a PostingsEnum for the current term.
//
// Deferred: requires metadata decoding and PostingsReaderBase wiring.
func (e *SegmentTermsEnum) Postings(_ int) (index.PostingsEnum, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term with live
// docs applied.
//
// Deferred: requires metadata decoding and PostingsReaderBase wiring.
func (e *SegmentTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (index.PostingsEnum, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// compile-time assertions
var _ index.TermsEnum = (*SegmentTermsEnum)(nil)
