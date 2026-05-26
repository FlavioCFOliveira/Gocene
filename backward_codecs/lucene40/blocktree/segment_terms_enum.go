// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
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
	if e.in == nil && e.fr.parent != nil && e.fr.parent.termsIn != nil {
		e.in = e.fr.parent.termsIn.Clone()
	}
}

// getFrame grows the frame stack to at least ord+1 entries and returns
// stack[ord].
func (e *SegmentTermsEnum) getFrame(ord int) *segmentTermsEnumFrame {
	if ord >= len(e.stack) {
		newLen := util.Oversize(ord+1, 8)
		grown := make([]*segmentTermsEnumFrame, newLen)
		copy(grown, e.stack)
		for i := len(e.stack); i < len(grown); i++ {
			grown[i] = newSegmentTermsEnumFrame(e, i)
		}
		e.stack = grown
	}
	return e.stack[ord]
}

// getArc grows the arcs slice to at least ord+1 entries and returns arcs[ord].
func (e *SegmentTermsEnum) getArc(ord int) *fst.Arc[*util.BytesRef] {
	if ord >= len(e.arcs) {
		newLen := util.Oversize(ord+1, 8)
		grown := make([]*fst.Arc[*util.BytesRef], newLen)
		copy(grown, e.arcs)
		for i := len(e.arcs); i < len(grown); i++ {
			grown[i] = &fst.Arc[*util.BytesRef]{}
		}
		e.arcs = grown
	}
	return e.arcs[ord]
}

// pushFrame pushes a frame using FST output data and length.
//
// Port of SegmentTermsEnum.pushFrame(Arc, BytesRef, int).
func (e *SegmentTermsEnum) pushFrame(
	arc *fst.Arc[*util.BytesRef],
	frameData *util.BytesRef,
	length int,
) *segmentTermsEnumFrame {
	e.scratchReader.Reset(frameData.Bytes[frameData.Offset : frameData.Offset+frameData.Length])
	code, _ := store.ReadVLong(e.scratchReader)
	fpSeek := int64(uint64(code) >> OutputFlagsNumBits)
	f := e.getFrame(1 + e.currentFrame.ord)
	f.hasTerms = (code & OutputFlagHasTerms) != 0
	f.hasTermsOrig = f.hasTerms
	f.isFloor = (code & OutputFlagIsFloor) != 0
	if f.isFloor {
		f.setFloorData(e.scratchReader, frameData)
	}
	return e.pushFrameFP(arc, fpSeek, length)
}

// pushFrameFP pushes (or reuses) a frame at the given absolute file pointer.
//
// Port of SegmentTermsEnum.pushFrame(Arc, long, int).
func (e *SegmentTermsEnum) pushFrameFP(
	arc *fst.Arc[*util.BytesRef],
	fp int64,
	length int,
) *segmentTermsEnumFrame {
	f := e.getFrame(1 + e.currentFrame.ord)
	f.arc = arc
	if f.fpOrig == fp && f.nextEnt != -1 {
		if f.ord > e.targetBeforeCurrentLength {
			f.rewind()
		}
	} else {
		f.nextEnt = -1
		f.prefix = length
		if f.state != nil {
			f.state.TermBlockOrd = 0
		}
		f.fpOrig = fp
		f.fp = fp
		f.lastSubFP = -1
	}
	return f
}

// Term returns the current term.
func (e *SegmentTermsEnum) Term() *index.Term {
	b := e.term.Get()
	if b == nil || b.Length == 0 {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), b)
}

// DocFreq returns the document frequency of the current term.
//
// Port of SegmentTermsEnum.docFreq().
func (e *SegmentTermsEnum) DocFreq() (int, error) {
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return 0, err
	}
	if e.currentFrame.state == nil {
		return 0, fmt.Errorf("blocktree DocFreq: term state is nil")
	}
	return e.currentFrame.state.DocFreq, nil
}

// TotalTermFreq returns the total frequency of the current term.
//
// Port of SegmentTermsEnum.totalTermFreq().
func (e *SegmentTermsEnum) TotalTermFreq() (int64, error) {
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return 0, err
	}
	if e.currentFrame.state == nil {
		return 0, fmt.Errorf("blocktree TotalTermFreq: term state is nil")
	}
	return e.currentFrame.state.TotalTermFreq, nil
}

// Postings returns a PostingsEnum for the current term.
//
// Port of SegmentTermsEnum.postings(PostingsEnum, int).
func (e *SegmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return nil, err
	}
	if e.currentFrame.state == nil {
		return nil, fmt.Errorf("blocktree Postings: term state is nil")
	}
	return e.fr.parent.postingsReader.Postings(
		e.fr.fieldInfo,
		e.currentFrame.state,
		nil,
		flags,
	)
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term with live
// docs applied.
func (e *SegmentTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// Next advances to the next term.
//
// Port of SegmentTermsEnum.next().
func (e *SegmentTermsEnum) Next() (*index.Term, error) {
	e.initIndexInput()
	if e.in == nil {
		// No backing file available (e.g. test-only FieldReader with no parent).
		return nil, nil
	}

	for {
		// Pop finished blocks.
		for e.currentFrame.nextEnt == e.currentFrame.entCount {
			if !e.currentFrame.isLastInFloor {
				if err := e.currentFrame.loadNextFloorBlock(); err != nil {
					return nil, err
				}
			} else {
				if e.currentFrame.ord == 0 {
					e.term.Clear()
					return nil, nil // EOF
				}
				lastFP := e.currentFrame.fpOrig
				e.currentFrame = e.stack[e.currentFrame.ord-1]
				_ = lastFP // assertion in Java; log it away
			}
		}

		isSubBlock, err := e.currentFrame.next()
		if err != nil {
			return nil, err
		}
		if isSubBlock {
			// Recurse into sub-block.
			e.currentFrame = e.pushFrameFP(nil, e.currentFrame.lastSubFP, e.term.Length())
			e.currentFrame.fpOrig = e.currentFrame.fp
			if err2 := e.currentFrame.loadBlock(); err2 != nil {
				return nil, err2
			}
			continue
		}

		if e.termExists {
			t := index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.term.Get())
			return t, nil
		}
		// Sub-block entry that was not a term; continue.
	}
}

// SeekCeil seeks to the first term >= target.
//
// Port of SegmentTermsEnum.seekCeil(BytesRef) (simplified: falls through to
// the index-guided path when an FST index is present; linear scan when not).
func (e *SegmentTermsEnum) SeekCeil(target *index.Term) (*index.Term, error) {
	e.initIndexInput()

	targetRef := &util.BytesRef{
		Bytes:  []byte(target.Text()),
		Offset: 0,
		Length: len(target.Text()),
	}

	// Use seekExact first; if exact, return it.
	found, err := e.seekExactRef(targetRef)
	if err != nil {
		return nil, err
	}
	if found {
		t := index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.term.Get())
		return t, nil
	}

	// Otherwise, next() will give us the first term >= target.
	return e.Next()
}

// SeekExact seeks to the given term exactly.
//
// Port of SegmentTermsEnum.seekExact(BytesRef).
func (e *SegmentTermsEnum) SeekExact(target *index.Term) (bool, error) {
	e.initIndexInput()
	targetRef := &util.BytesRef{
		Bytes:  []byte(target.Text()),
		Offset: 0,
		Length: len(target.Text()),
	}
	return e.seekExactRef(targetRef)
}

// seekExactRef performs exact seek on raw bytes.
//
// This implementation does an FST-guided block seek followed by a linear
// block scan.  The full seek logic (multi-level floor-block tracking with
// output accumulation) matches the Java source at
// org.apache.lucene.backward_codecs.lucene40.blocktree.SegmentTermsEnum.seekExact.
func (e *SegmentTermsEnum) seekExactRef(target *util.BytesRef) (bool, error) {
	if e.fr.fstIndex == nil || e.fr.rootCode == nil {
		return false, nil
	}

	e.targetBeforeCurrentLength = e.currentFrame.ord

	// Walk the FST index to find the deepest block that could contain target.
	arc := e.fr.fstIndex.GetFirstArc(e.getArc(0))
	outputs := fst.ByteSequenceOutputs()
	output := outputs.GetNoOutput()

	var lastMatchingFrame *segmentTermsEnumFrame

	targetUpto := 0
	for targetUpto < target.Length {
		nextArc, err := e.fr.fstIndex.FindTargetArc(
			int(target.Bytes[target.Offset+targetUpto])&0xFF,
			arc,
			e.getArc(1+targetUpto),
			e.fstReader,
		)
		if err != nil || nextArc == nil {
			break
		}
		arc = nextArc
		output = outputs.Add(output, arc.Output())
		if arc.IsFinal() {
			combined := outputs.Add(output, arc.NextFinalOutput())
			lastMatchingFrame = e.pushFrame(arc, combined, targetUpto+1)
		}
		targetUpto++
	}

	// Seek to root if nothing matched.
	if lastMatchingFrame == nil {
		arc = e.fr.fstIndex.GetFirstArc(e.getArc(0))
		lastMatchingFrame = e.pushFrame(arc, e.fr.rootCode, 0)
	}
	e.currentFrame = lastMatchingFrame
	e.currentFrame.scanToFloorFrame(target)
	if err := e.currentFrame.loadBlock(); err != nil {
		return false, err
	}

	status, err := e.currentFrame.scanToTerm(target, true)
	if err != nil {
		return false, err
	}
	return status == seekStatusFound, nil
}

// compile-time assertions
var _ index.TermsEnum = (*SegmentTermsEnum)(nil)
var _ codecs.PostingsReaderBase = (*noopPostingsReaderForCompile)(nil)

// noopPostingsReaderForCompile is a compile-time placeholder used only in
// assertions; it is never instantiated.
type noopPostingsReaderForCompile struct{}

func (n *noopPostingsReaderForCompile) Init(_ store.IndexInput, _ *codecs.SegmentReadState) error {
	return nil
}
func (n *noopPostingsReaderForCompile) NewTermState() *codecs.BlockTermState {
	return codecs.NewBlockTermState()
}
func (n *noopPostingsReaderForCompile) DecodeTerm(_ store.DataInput, _ *index.FieldInfo, _ *codecs.BlockTermState, _ bool) error {
	return nil
}
func (n *noopPostingsReaderForCompile) Postings(_ *index.FieldInfo, _ *codecs.BlockTermState, _ index.PostingsEnum, _ int) (index.PostingsEnum, error) {
	return nil, nil
}
func (n *noopPostingsReaderForCompile) Impacts(_ *index.FieldInfo, _ *codecs.BlockTermState, _ int) (index.ImpactsEnum, error) {
	return nil, nil
}
func (n *noopPostingsReaderForCompile) CheckIntegrity() error { return nil }
func (n *noopPostingsReaderForCompile) Close() error          { return nil }
