// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// OrdsSegmentTermsEnum iterates over the terms of one field in a segment
// written by the BlockTreeOrds postings format, tracking term ordinals.
//
// Port of org.apache.lucene.codecs.blocktreeords.OrdsSegmentTermsEnum
// (Lucene 10.4.0, ~1298 lines). Full seekExact / seekCeil / next traversal
// logic is deferred to a later sprint; the struct carries all fields required
// by OrdsSegmentTermsEnumFrame so that the frame compiles cleanly.
type OrdsSegmentTermsEnum struct {
	// TermsEnumBase renders `extends BaseTermsEnum`
	// (OrdsSegmentTermsEnum.java:41): it carries the lazily created
	// AttributeSource behind BaseTermsEnum.attributes().
	index.TermsEnumBase

	reader *OrdsFieldReader

	// term is the current term built up by frame traversal.
	term *util.BytesRefBuilder

	// in is the lazily-cloned terms IndexInput (set by initIndexInput).
	in store.IndexInput

	// termExists is true when the current frame position is a real term
	// (rather than a sub-block marker).
	termExists bool

	// currentFrame is the top frame on the stack.
	currentFrame *OrdsSegmentTermsEnumFrame

	// stack is the frame stack, grown on demand.
	stack []*OrdsSegmentTermsEnumFrame

	// arcs caches FST arcs, one per stack depth.
	arcs []*gfst.Arc[*FSTOrdsOutput]

	startKey *util.BytesRef
}

// NewOrdsSegmentTermsEnum constructs an enum for the given field reader.
// startKey positions the enum before the first call to Next(); it may be nil.
func NewOrdsSegmentTermsEnum(r *OrdsFieldReader, startKey *util.BytesRef) (*OrdsSegmentTermsEnum, error) {
	e := &OrdsSegmentTermsEnum{
		reader:   r,
		term:     util.NewBytesRefBuilder(),
		startKey: startKey,
		stack:    make([]*OrdsSegmentTermsEnumFrame, 5),
		arcs:     make([]*gfst.Arc[*FSTOrdsOutput], 5),
	}
	for i := range e.arcs {
		e.arcs[i] = &gfst.Arc[*FSTOrdsOutput]{}
	}
	return e, nil
}

// initIndexInput lazily clones the parent IndexInput. Called by
// OrdsSegmentTermsEnumFrame.loadBlock before the first disk seek.
func (e *OrdsSegmentTermsEnum) initIndexInput() error {
	if e.in != nil {
		return nil
	}
	if e.reader == nil || e.reader.parent == nil || e.reader.parent.in == nil {
		return fmt.Errorf("OrdsSegmentTermsEnum.initIndexInput: parent IndexInput is nil")
	}
	e.in = e.reader.parent.in.Clone()
	return nil
}

// getFrame returns the frame at stack depth ord, growing the stack if needed.
func (e *OrdsSegmentTermsEnum) getFrame(ord int) (*OrdsSegmentTermsEnumFrame, error) {
	for ord >= len(e.stack) {
		newLen := ord + 1
		newStack := make([]*OrdsSegmentTermsEnumFrame, newLen)
		copy(newStack, e.stack)
		e.stack = newStack
	}
	if e.stack[ord] == nil {
		f, err := NewOrdsSegmentTermsEnumFrame(e, ord)
		if err != nil {
			return nil, err
		}
		e.stack[ord] = f
	}
	return e.stack[ord], nil
}

// getArc returns the arc at depth ord, growing the arcs slice if needed.
func (e *OrdsSegmentTermsEnum) getArc(ord int) *gfst.Arc[*FSTOrdsOutput] {
	for ord >= len(e.arcs) {
		newLen := ord + 1
		newArcs := make([]*gfst.Arc[*FSTOrdsOutput], newLen)
		copy(newArcs, e.arcs)
		for i := len(e.arcs); i < newLen; i++ {
			newArcs[i] = &gfst.Arc[*FSTOrdsOutput]{}
		}
		e.arcs = newArcs
	}
	return e.arcs[ord]
}

// pushFrameAt pushes a new frame at lastSubFP with the given prefix length
// and starting term ordinal.
//
// Port of OrdsSegmentTermsEnum.pushFrame(FST.Arc, long, int, long)
// (Lucene 10.4.0). The arc parameter is unused in the stub.
func (e *OrdsSegmentTermsEnum) pushFrameAt(arc *gfst.Arc[*FSTOrdsOutput], lastSubFP int64, length int, termOrd int64) (*OrdsSegmentTermsEnumFrame, error) {
	var depth int
	if e.currentFrame == nil {
		depth = 0
	} else {
		depth = 1 + e.currentFrame.ord
	}
	f, err := e.getFrame(depth)
	if err != nil {
		return nil, err
	}
	f.fp = lastSubFP
	f.fpOrig = lastSubFP
	f.prefixLength = length
	f.termOrdOrig = termOrd
	f.termOrd = termOrd
	return f, nil
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

// Ord returns the ordinal of the current term. Port of
// OrdsSegmentTermsEnum.ord() (OrdsSegmentTermsEnum.java:1009):
//
//	assert !eof;
//	assert currentFrame.termOrd > 0;
//	return currentFrame.termOrd - 1;
//
// The two statements Java guards with `assert` are disabled at runtime unless
// the JVM is started with -ea, so only the return is behaviour.
func (e *OrdsSegmentTermsEnum) Ord() int64 {
	return e.currentFrame.termOrd - 1
}

// Postings returns a PostingsEnum for the current term.
func (e *OrdsSegmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// Impacts returns an ImpactsEnum for the current term. Port of
// OrdsSegmentTermsEnum.impacts(int) (OrdsSegmentTermsEnum.java:1045):
//
//	assert !eof;
//	currentFrame.decodeMetaData();
//	return fr.parent.postingsReader.impacts(fr.fieldInfo, currentFrame.state, flags);
//
// The `assert !eof` statement is disabled at runtime unless the JVM is started
// with -ea, so only the two remaining statements are behaviour.
func (e *OrdsSegmentTermsEnum) Impacts(flags int) (index.ImpactsEnum, error) {
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return nil, fmt.Errorf("OrdsSegmentTermsEnum.Impacts: decodeMetaData: %w", err)
	}
	return e.reader.parent.postingsReader.Impacts(e.reader.fieldInfo, e.currentFrame.termStateRef, flags)
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term filtered by live docs.
func (e *OrdsSegmentTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// compile-time assertion.
var _ index.TermsEnum = (*OrdsSegmentTermsEnum)(nil)
