// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// SegmentTermsEnum iterates through all terms in a single field of the
// Lucene 9.0 block-tree terms dictionary.
//
// The Lucene 9.0 format differs from Lucene 4.0 in that it uses an
// OutputAccumulator to accumulate FST arc outputs across the frame chain
// when following floor blocks, replacing the per-frame outputPrefix approach.
//
// When the owning FieldReader has no FST index (stub/registry path), all
// navigation methods return ErrBlockTraversalNotAvailable; existing tests that
// construct a stub FieldReader continue to pass unchanged.
//
// Port of
// org.apache.lucene.backward_codecs.lucene90.blocktree.SegmentTermsEnum
// (Lucene 10.4.0).
type SegmentTermsEnum struct {
	index.TermsEnumBase

	// in is the lazily-initialised clone of the terms file (.tim).
	in store.IndexInput

	// stack holds the active frames, indexed by frame ord. staticFrame
	// (ord -1) holds seek-by-TermState / cached-seek state. currentFrame
	// is the frame the cursor currently sits in.
	stack        []*segmentTermsEnumFrame
	staticFrame  *segmentTermsEnumFrame
	currentFrame *segmentTermsEnumFrame

	// termExists is true when the cursor sits on a real term.
	termExists bool

	// fr is the owning FieldReader.
	fr *FieldReader

	// targetBeforeCurrentLength records the seek-state frame ord that remains
	// valid for the in-flight seek.
	targetBeforeCurrentLength int

	// validIndexPrefix is the prefix length confirmed by the FST index during
	// the last seekCeil/seekExact (0 while only next()-ing).
	validIndexPrefix int

	// eof is true after the last term has been returned and the enum is
	// exhausted.
	eof bool

	// term is the growable buffer backing Term().
	term *util.BytesRefBuilder

	// arcs is the per-depth arc cache used during FST walks.
	arcs []*fst.Arc[*util.BytesRef]

	// fstReader is the BytesReader obtained from the FST index; nil for stub
	// FieldReaders.
	fstReader fst.BytesReader

	// scratchReader is a shared ByteArrayDataInput used for temporary decoding.
	scratchReader *store.ByteArrayDataInput

	// outputAccum accumulates FST arc outputs across the frame chain.
	// Port of SegmentTermsEnum.OutputAccumulator in Java.
	outputAccum outputAccumulator
}

// newSegmentTermsEnum constructs a SegmentTermsEnum for the given FieldReader.
// Port of SegmentTermsEnum(FieldReader).
func newSegmentTermsEnum(fr *FieldReader) (*SegmentTermsEnum, error) {
	if fr == nil {
		return nil, fmt.Errorf("lucene90 blocktree: FieldReader must not be nil")
	}

	e := &SegmentTermsEnum{
		fr:            fr,
		term:          util.NewBytesRefBuilder(),
		arcs:          make([]*fst.Arc[*util.BytesRef], 1),
		scratchReader: store.NewByteArrayDataInput(nil),
	}

	e.staticFrame = newSegmentTermsEnumFrame(e, -1)
	e.currentFrame = e.staticFrame

	for i := range e.arcs {
		e.arcs[i] = &fst.Arc[*util.BytesRef]{}
	}

	// Wire the FST reader only when the FieldReader has a live index.
	if fr.index != nil {
		e.fstReader = fr.index.GetBytesReader()
		// Seed the root arc (empty-string prefix) in arcs[0].
		fr.index.GetFirstArc(e.arcs[0])
	}

	return e, nil
}

// initIndexInput lazily clones the parent reader's .tim IndexInput.
// Port of SegmentTermsEnum.initIndexInput().
func (e *SegmentTermsEnum) initIndexInput() error {
	if e.in != nil {
		return nil
	}
	if e.fr.parent == nil || e.fr.parent.termsIn == nil {
		return errors.New("SegmentTermsEnum: field reader has no parent terms input")
	}
	e.in = e.fr.parent.termsIn.Clone()
	return nil
}

// getFrame grows the frame stack to at least ord+1 entries and returns
// stack[ord]. Port of SegmentTermsEnum.getFrame(int).
func (e *SegmentTermsEnum) getFrame(ord int) *segmentTermsEnumFrame {
	if ord >= len(e.stack) {
		grown := make([]*segmentTermsEnumFrame, util.Oversize(ord+1, 1))
		copy(grown, e.stack)
		for i := len(e.stack); i < len(grown); i++ {
			grown[i] = newSegmentTermsEnumFrame(e, i)
		}
		e.stack = grown
	}
	return e.stack[ord]
}

// getArc returns the arc at the given depth, growing the arcs slice as needed.
// Port of SegmentTermsEnum.getArc(int).
func (e *SegmentTermsEnum) getArc(ord int) *fst.Arc[*util.BytesRef] {
	if ord >= len(e.arcs) {
		grown := make([]*fst.Arc[*util.BytesRef], util.Oversize(ord+1, 1))
		copy(grown, e.arcs)
		for i := len(e.arcs); i < len(grown); i++ {
			grown[i] = &fst.Arc[*util.BytesRef]{}
		}
		e.arcs = grown
	}
	return e.arcs[ord]
}

// growTerm ensures the term builder has at least n bytes of capacity and sets
// its logical length to n.
func (e *SegmentTermsEnum) growTerm(n int) {
	e.term.Grow(n)
	e.term.SetLength(n)
}

// Term returns the current term, or nil at EOF or before navigation.
func (e *SegmentTermsEnum) Term() *index.Term {
	if e.eof {
		return nil
	}
	b := e.term.Get()
	if b == nil || b.Length == 0 {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.Name, b)
}

// pushFrame pushes a new frame whose block pointer and flags are encoded in
// frameData (the accumulated FST output). Mirrors the two-arg Java
// SegmentTermsEnum.pushFrame(arc, frameData, length).
func (e *SegmentTermsEnum) pushFrame(arc *fst.Arc[*util.BytesRef], frameData *util.BytesRef, length int) (*segmentTermsEnumFrame, error) {
	e.outputAccum.reset()
	e.outputAccum.push(frameData)
	return e.pushFrameFromAccum(arc, length)
}

// pushFrameFromAccum reads the fp+flags from the OutputAccumulator and pushes
// a new frame. Mirrors the one-arg Java SegmentTermsEnum.pushFrame(arc, length)
// which reads from the already-populated accumulator.
func (e *SegmentTermsEnum) pushFrameFromAccum(arc *fst.Arc[*util.BytesRef], length int) (*segmentTermsEnumFrame, error) {
	e.outputAccum.prepareRead()
	code, err := e.fr.parent.readVLongOutput(&e.outputAccum)
	if err != nil {
		return nil, fmt.Errorf("pushFrameFromAccum: readVLongOutput: %w", err)
	}
	fp := code >> outputFlagsNumBits
	f := e.getFrame(1 + e.currentFrame.ord)
	f.hasTerms = (code & outputFlagHasTerms) != 0
	f.hasTermsOrig = f.hasTerms
	f.isFloor = (code & outputFlagIsFloor) != 0
	if f.isFloor {
		f.setFloorData(&e.outputAccum)
	}
	return e.pushFrameFromSubFP(arc, fp, length)
}

// pushFrameFromSubFP is the three-arg Java pushFrame(arc, fp, length). It
// re-uses an existing frame slot when the fp matches and the frame is already
// loaded (and the ord is within the seek-valid range). Port of
// SegmentTermsEnum.pushFrame(arc, fp, length).
func (e *SegmentTermsEnum) pushFrameFromSubFP(arc *fst.Arc[*util.BytesRef], fp int64, length int) (*segmentTermsEnumFrame, error) {
	f := e.getFrame(1 + e.currentFrame.ord)
	f.arc = arc
	if f.fpOrig == fp && f.nextEnt != -1 {
		if f.ord > e.targetBeforeCurrentLength {
			f.rewind()
		}
		if length != f.prefixLength {
			return nil, fmt.Errorf("pushFrameFromSubFP: length=%d != prefixLength=%d on reused frame", length, f.prefixLength)
		}
	} else {
		f.nextEnt = -1
		f.prefixLength = length
		f.state.TermBlockOrd = 0
		f.fpOrig = fp
		f.fp = fp
		f.lastSubFP = -1
	}
	return f, nil
}

// Next advances to the next term in the block-tree, descending sub-blocks and
// crossing floor blocks. Port of SegmentTermsEnum.next().
func (e *SegmentTermsEnum) Next() (*index.Term, error) {
	if e.fr.index == nil {
		return nil, ErrBlockTraversalNotAvailable
	}
	if e.eof {
		return nil, nil
	}

	if e.in == nil {
		// Fresh enum: push the root frame and load the first block.
		arc := e.fr.index.GetFirstArc(e.arcs[0])
		cf, err := e.pushFrame(arc, e.fr.rootCode, 0)
		if err != nil {
			return nil, err
		}
		e.currentFrame = cf
		if err := e.currentFrame.loadBlock(); err != nil {
			return nil, err
		}
	}

	e.targetBeforeCurrentLength = e.currentFrame.ord

	if e.currentFrame == e.staticFrame {
		// A prior seek cached a term but no block was loaded; re-seek to it.
		ok, err := e.seekExactInternal(e.term.Get())
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("SegmentTermsEnum.Next: re-seek to pending term failed")
		}
	}

	// Pop finished blocks.
	for e.currentFrame.nextEnt == e.currentFrame.entCount {
		if !e.currentFrame.isLastInFloor {
			if err := e.currentFrame.loadNextFloorBlock(); err != nil {
				return nil, err
			}
			break
		}
		if e.currentFrame.ord == 0 {
			e.eof = true
			e.term.Clear()
			e.validIndexPrefix = 0
			e.currentFrame.rewind()
			e.termExists = false
			return nil, nil
		}
		lastFP := e.currentFrame.fpOrig
		e.currentFrame = e.stack[e.currentFrame.ord-1]
		if e.currentFrame.nextEnt == -1 || e.currentFrame.lastSubFP != lastFP {
			e.currentFrame.scanToFloorFrame(e.term.Get())
			if err := e.currentFrame.loadBlock(); err != nil {
				return nil, err
			}
			if err := e.currentFrame.scanToSubBlock(lastFP); err != nil {
				return nil, err
			}
		}
		if e.currentFrame.prefixLength < e.validIndexPrefix {
			e.validIndexPrefix = e.currentFrame.prefixLength
		}
	}

	for {
		isSub, err := e.currentFrame.next()
		if err != nil {
			return nil, err
		}
		if isSub {
			cf, err := e.pushFrameFromSubFP(nil, e.currentFrame.lastSubFP, e.term.Length())
			if err != nil {
				return nil, err
			}
			e.currentFrame = cf
			if err := e.currentFrame.loadBlock(); err != nil {
				return nil, err
			}
		} else {
			return e.Term(), nil
		}
	}
}

// seekExactInternal implements the common prefix re-use path shared by
// SeekExact and Next. Port of SegmentTermsEnum.prepareSeekExact + seekExact.
func (e *SegmentTermsEnum) seekExactInternal(target *util.BytesRef) (bool, error) {
	if e.fr.index == nil {
		return false, ErrBlockTraversalNotAvailable
	}

	if e.fr.numTerms > 0 {
		if e.fr.minTerm != nil && bytes.Compare(target.ValidBytes(), e.fr.minTerm.ValidBytes()) < 0 {
			return false, nil
		}
		if e.fr.maxTerm != nil && bytes.Compare(target.ValidBytes(), e.fr.maxTerm.ValidBytes()) > 0 {
			return false, nil
		}
	}

	e.growTerm(1 + target.Length)
	e.eof = false
	e.outputAccum.reset()

	var arc *fst.Arc[*util.BytesRef]
	var targetUpto int
	e.targetBeforeCurrentLength = e.currentFrame.ord

	if e.currentFrame != e.staticFrame {
		// Re-use common prefix with the previous seek.
		arc = e.arcs[0]
		e.outputAccum.push(arc.Output())
		targetUpto = 0

		lastFrame := e.stack[0]
		targetLimit := target.Length
		if e.validIndexPrefix < targetLimit {
			targetLimit = e.validIndexPrefix
		}
		cmp := 0
		for targetUpto < targetLimit {
			cmp = int(e.term.ByteAt(targetUpto)&0xff) - int(target.Bytes[target.Offset+targetUpto]&0xff)
			if cmp != 0 {
				break
			}
			arc = e.arcs[1+targetUpto]
			e.outputAccum.push(arc.Output())
			if arc.IsFinal() {
				lastFrame = e.stack[1+lastFrame.ord]
			}
			targetUpto++
		}
		if cmp == 0 {
			cmp = bytes.Compare(
				e.term.Bytes()[targetUpto:e.term.Length()],
				target.Bytes[target.Offset+targetUpto:target.Offset+target.Length],
			)
		}
		if cmp < 0 {
			e.currentFrame = lastFrame
		} else if cmp > 0 {
			e.targetBeforeCurrentLength = lastFrame.ord
			e.currentFrame = lastFrame
			e.currentFrame.rewind()
		} else {
			// Target equals current term.
			if e.termExists {
				return true, nil
			}
		}
	} else {
		e.targetBeforeCurrentLength = -1
		arc = e.fr.index.GetFirstArc(e.arcs[0])
		e.outputAccum.push(arc.Output())
		e.currentFrame = e.staticFrame
		targetUpto = 0
		e.outputAccum.push(arc.NextFinalOutput())
		cf, err := e.pushFrameFromAccum(arc, 0)
		if err != nil {
			return false, err
		}
		e.currentFrame = cf
		e.outputAccum.popN(1)
	}

	for targetUpto < target.Length {
		targetLabel := int(target.Bytes[target.Offset+targetUpto]) & 0xff
		nextArc, err := e.fr.index.FindTargetArc(targetLabel, arc, e.getArc(1+targetUpto), e.fstReader)
		if err != nil {
			return false, fmt.Errorf("seekExactInternal: FindTargetArc: %w", err)
		}
		if nextArc == nil {
			// Index exhausted.
			e.validIndexPrefix = e.currentFrame.prefixLength
			e.currentFrame.scanToFloorFrame(target)
			if !e.currentFrame.hasTerms {
				e.termExists = false
				e.growTerm(1 + targetUpto)
				e.term.SetByteAt(targetUpto, byte(targetLabel))
				return false, nil
			}
			if err := e.currentFrame.loadBlock(); err != nil {
				return false, err
			}
			status, err := e.currentFrame.scanToTerm(target, true)
			if err != nil {
				return false, err
			}
			return status == index.SeekStatusFound, nil
		}
		arc = nextArc
		e.term.SetByteAt(targetUpto, byte(targetLabel))
		e.outputAccum.push(arc.Output())
		targetUpto++
		if arc.IsFinal() {
			e.outputAccum.push(arc.NextFinalOutput())
			cf, err := e.pushFrameFromAccum(arc, targetUpto)
			if err != nil {
				return false, err
			}
			e.currentFrame = cf
			e.outputAccum.popN(1)
		}
	}

	e.validIndexPrefix = e.currentFrame.prefixLength
	e.currentFrame.scanToFloorFrame(target)
	if !e.currentFrame.hasTerms {
		e.termExists = false
		e.growTerm(targetUpto)
		return false, nil
	}
	if err := e.currentFrame.loadBlock(); err != nil {
		return false, err
	}
	status, err := e.currentFrame.scanToTerm(target, true)
	if err != nil {
		return false, err
	}
	return status == index.SeekStatusFound, nil
}

// SeekExact positions the enumerator on term, returning true if found.
// Port of SegmentTermsEnum.seekExact(BytesRef).
func (e *SegmentTermsEnum) SeekExact(term *index.Term) (bool, error) {
	if e.fr.index == nil {
		return false, ErrBlockTraversalNotAvailable
	}
	if term == nil {
		return false, errors.New("SegmentTermsEnum.SeekExact: term must not be nil")
	}
	return e.seekExactInternal(term.BytesValue())
}

// SeekCeil positions the enumerator at term, or at the next ceiling term if it
// does not exist. Returns the positioned term or nil at END.
// Port of SegmentTermsEnum.seekCeil(BytesRef).
func (e *SegmentTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	if e.fr.index == nil {
		return nil, ErrBlockTraversalNotAvailable
	}
	if term == nil {
		return nil, errors.New("SegmentTermsEnum.SeekCeil: term must not be nil")
	}
	target := term.BytesValue()
	e.growTerm(1 + target.Length)
	e.eof = false
	e.outputAccum.reset()

	var arc *fst.Arc[*util.BytesRef]
	var targetUpto int
	e.targetBeforeCurrentLength = e.currentFrame.ord

	if e.currentFrame != e.staticFrame {
		arc = e.arcs[0]
		e.outputAccum.push(arc.Output())
		targetUpto = 0

		lastFrame := e.stack[0]
		targetLimit := target.Length
		if e.validIndexPrefix < targetLimit {
			targetLimit = e.validIndexPrefix
		}
		cmp := 0
		for targetUpto < targetLimit {
			cmp = int(e.term.ByteAt(targetUpto)&0xff) - int(target.Bytes[target.Offset+targetUpto]&0xff)
			if cmp != 0 {
				break
			}
			arc = e.arcs[1+targetUpto]
			e.outputAccum.push(arc.Output())
			if arc.IsFinal() {
				lastFrame = e.stack[1+lastFrame.ord]
			}
			targetUpto++
		}
		if cmp == 0 {
			cmp = bytes.Compare(
				e.term.Bytes()[targetUpto:e.term.Length()],
				target.Bytes[target.Offset+targetUpto:target.Offset+target.Length],
			)
		}
		if cmp < 0 {
			e.currentFrame = lastFrame
		} else if cmp > 0 {
			e.targetBeforeCurrentLength = 0
			e.currentFrame = lastFrame
			e.currentFrame.rewind()
		} else {
			if e.termExists {
				return e.Term(), nil
			}
		}
	} else {
		e.targetBeforeCurrentLength = -1
		arc = e.fr.index.GetFirstArc(e.arcs[0])
		e.outputAccum.push(arc.Output())
		e.currentFrame = e.staticFrame
		targetUpto = 0
		e.outputAccum.push(arc.NextFinalOutput())
		cf, err := e.pushFrameFromAccum(arc, 0)
		if err != nil {
			return nil, err
		}
		e.currentFrame = cf
		e.outputAccum.popN(1)
	}

	for targetUpto < target.Length {
		targetLabel := int(target.Bytes[target.Offset+targetUpto]) & 0xff
		nextArc, err := e.fr.index.FindTargetArc(targetLabel, arc, e.getArc(1+targetUpto), e.fstReader)
		if err != nil {
			return nil, fmt.Errorf("SeekCeil: FindTargetArc: %w", err)
		}
		if nextArc == nil {
			e.validIndexPrefix = e.currentFrame.prefixLength
			e.currentFrame.scanToFloorFrame(target)
			if err := e.currentFrame.loadBlock(); err != nil {
				return nil, err
			}
			status, err := e.currentFrame.scanToTerm(target, false)
			if err != nil {
				return nil, err
			}
			return e.finishSeekCeil(status, target)
		}
		e.term.SetByteAt(targetUpto, byte(targetLabel))
		arc = nextArc
		e.outputAccum.push(arc.Output())
		targetUpto++
		if arc.IsFinal() {
			e.outputAccum.push(arc.NextFinalOutput())
			cf, err := e.pushFrameFromAccum(arc, targetUpto)
			if err != nil {
				return nil, err
			}
			e.currentFrame = cf
			e.outputAccum.popN(1)
		}
	}

	e.validIndexPrefix = e.currentFrame.prefixLength
	e.currentFrame.scanToFloorFrame(target)
	if err := e.currentFrame.loadBlock(); err != nil {
		return nil, err
	}
	status, err := e.currentFrame.scanToTerm(target, false)
	if err != nil {
		return nil, err
	}
	return e.finishSeekCeil(status, target)
}

// finishSeekCeil resolves the seekCeil result after scanToTerm: on END it
// copies the target into the term buffer and advances via Next() to the
// ceiling. Port of the tail of SegmentTermsEnum.seekCeil.
func (e *SegmentTermsEnum) finishSeekCeil(status index.SeekStatus, target *util.BytesRef) (*index.Term, error) {
	if status == index.SeekStatusEnd {
		e.term.CopyBytes(target.Bytes, target.Offset, target.Length)
		e.termExists = false
		nxt, err := e.Next()
		if err != nil {
			return nil, err
		}
		return nxt, nil // nil == END, non-nil == ceiling term
	}
	return e.Term(), nil
}

// DocFreq returns the document frequency of the current term.
// Port of SegmentTermsEnum.docFreq().
func (e *SegmentTermsEnum) DocFreq() (int, error) {
	if e.fr.index == nil {
		return 0, ErrBlockTraversalNotAvailable
	}
	if e.eof || e.currentFrame == nil {
		return 0, nil
	}
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return 0, fmt.Errorf("SegmentTermsEnum.DocFreq: %w", err)
	}
	return e.currentFrame.state.DocFreq, nil
}

// TotalTermFreq returns the total term frequency of the current term.
// Port of SegmentTermsEnum.totalTermFreq().
func (e *SegmentTermsEnum) TotalTermFreq() (int64, error) {
	if e.fr.index == nil {
		return 0, ErrBlockTraversalNotAvailable
	}
	if e.eof || e.currentFrame == nil {
		return 0, nil
	}
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return 0, fmt.Errorf("SegmentTermsEnum.TotalTermFreq: %w", err)
	}
	return e.currentFrame.state.TotalTermFreq, nil
}

// Postings decodes the current term's metadata and returns a PostingsEnum.
// Port of SegmentTermsEnum.postings(PostingsEnum, int).
func (e *SegmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if e.fr.index == nil {
		return nil, ErrBlockTraversalNotAvailable
	}
	if e.eof || e.currentFrame == nil {
		return &index.EmptyPostingsEnum{}, nil
	}
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return nil, fmt.Errorf("SegmentTermsEnum.Postings: decodeMetaData: %w", err)
	}
	pe, err := e.fr.parent.postingsReader.Postings(e.fr.fieldInfo, e.currentFrame.state, nil, flags)
	if err != nil {
		return nil, fmt.Errorf("SegmentTermsEnum.Postings: postingsReader.Postings: %w", err)
	}
	return pe, nil
}

// PostingsWithLiveDocs forwards to Postings; live-docs filtering is applied
// at a higher layer by callers.
func (e *SegmentTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// compile-time assertion
var _ index.TermsEnum = (*SegmentTermsEnum)(nil)
