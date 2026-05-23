// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// segmentTermsEnumFrame is a single level in the SegmentTermsEnum frame stack.
//
// Full block-loading logic (loadBlock, nextEntry, scanToTerm, etc.) is deferred
// until FieldReader is fully ported for lucene90.
//
// Port of
// org.apache.lucene.backward_codecs.lucene90.blocktree.SegmentTermsEnumFrame.
type segmentTermsEnumFrame struct {
	// ord is the depth of this frame (−1 for staticFrame).
	ord int

	// outputNum is the count of FST arc outputs pushed to the parent
	// SegmentTermsEnum.outputAccumulator for this frame's arc chain.
	outputNum int
}

// newSegmentTermsEnumFrame allocates a frame at the given ordinal.
func newSegmentTermsEnumFrame(ord int) *segmentTermsEnumFrame {
	return &segmentTermsEnumFrame{ord: ord}
}

// SegmentTermsEnum iterates through all terms in a single field of the
// Lucene 9.0 block-tree terms dictionary.
//
// Lucene 9.0 (vs Lucene 4.0) adds an OutputAccumulator to accumulate FST arc
// outputs across the frame chain when following floor blocks, replacing the
// per-frame outputPrefix BytesRef approach.
//
// Full block-loading and navigation logic (loadBlock, nextEntry, seekCeil,
// seekExact, next implementations) is deferred until FieldReader is fully
// ported. Until then, all navigation methods return
// ErrBlockTraversalNotAvailable.
//
// Port of
// org.apache.lucene.backward_codecs.lucene90.blocktree.SegmentTermsEnum
// (Lucene 10.4.0).
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

	// validIndexPrefix is the length of the prefix confirmed by the FST index
	// during the last seekCeil/seekExact.
	validIndexPrefix int

	term      util.BytesRefBuilder
	fstReader fst.BytesReader

	arcs []*fst.Arc[*util.BytesRef]

	// outputAccum accumulates FST arc outputs across the frame chain.
	// This is the Lucene 9.0 replacement for the per-frame outputPrefix.
	outputAccum outputAccumulator
}

// newSegmentTermsEnum constructs a SegmentTermsEnum for the given FieldReader.
//
// Port of SegmentTermsEnum(FieldReader).
func newSegmentTermsEnum(fr *FieldReader) (*SegmentTermsEnum, error) {
	if fr == nil {
		return nil, fmt.Errorf("lucene90 blocktree: FieldReader must not be nil")
	}

	e := &SegmentTermsEnum{
		fr:            fr,
		stack:         make([]*segmentTermsEnumFrame, 0),
		arcs:          make([]*fst.Arc[*util.BytesRef], 1),
		scratchReader: store.NewByteArrayDataInput(nil),
	}

	e.staticFrame = newSegmentTermsEnumFrame(-1)

	// fstReader is set only when FieldReader has a real FST index; the stub
	// FieldReader does not, so fstReader remains nil.
	e.fstReader = nil

	for i := range e.arcs {
		e.arcs[i] = &fst.Arc[*util.BytesRef]{}
	}

	e.currentFrame = e.staticFrame

	// Full init (cloning termsIn from fr.parent, loading root block) is
	// deferred until FieldReader is fully ported.

	return e, nil
}

// getFrame grows the frame stack to at least ord+1 entries and returns
// stack[ord].
func (e *SegmentTermsEnum) getFrame(ord int) *segmentTermsEnumFrame {
	if ord >= len(e.stack) {
		grown := make([]*segmentTermsEnumFrame, util.Oversize(ord+1, 1))
		copy(grown, e.stack)
		for i := len(e.stack); i < len(grown); i++ {
			grown[i] = newSegmentTermsEnumFrame(i)
		}
		e.stack = grown
	}
	return e.stack[ord]
}

// Term returns the current term.
func (e *SegmentTermsEnum) Term() *index.Term {
	b := e.term.Get()
	if b == nil || b.Length == 0 {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.Name, b)
}

// Next advances to the next term.
//
// Deferred: requires full block traversal logic and a real FieldReader.
func (e *SegmentTermsEnum) Next() (*index.Term, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// SeekCeil seeks to the first term >= text.
//
// Deferred: requires full block traversal logic.
func (e *SegmentTermsEnum) SeekCeil(_ *index.Term) (*index.Term, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// SeekExact seeks to text exactly.
//
// Deferred: requires full block traversal logic.
func (e *SegmentTermsEnum) SeekExact(_ *index.Term) (bool, error) {
	return false, ErrBlockTraversalNotAvailable
}

// DocFreq returns the document frequency of the current term.
//
// Deferred: requires metadata decoding.
func (e *SegmentTermsEnum) DocFreq() (int, error) {
	return 0, ErrBlockTraversalNotAvailable
}

// TotalTermFreq returns the total term frequency of the current term.
//
// Deferred: requires metadata decoding.
func (e *SegmentTermsEnum) TotalTermFreq() (int64, error) {
	return 0, ErrBlockTraversalNotAvailable
}

// Postings returns a PostingsEnum for the current term.
//
// Deferred: requires metadata decoding and PostingsReaderBase wiring.
func (e *SegmentTermsEnum) Postings(_ int) (index.PostingsEnum, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term.
//
// Deferred: requires metadata decoding and PostingsReaderBase wiring.
func (e *SegmentTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (index.PostingsEnum, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// compile-time assertion
var _ index.TermsEnum = (*SegmentTermsEnum)(nil)
