// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.IDVersionSegmentTermsEnum.
package idversion

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// vbtOutputFlagsNumBits is the number of output flag bits in the FST long
// output. Mirrors VersionBlockTreeTermsWriter.OUTPUT_FLAGS_NUM_BITS.
const vbtOutputFlagsNumBits = 2

// vbtOutputFlagIsFloor marks the block as a floor block.
// Mirrors VersionBlockTreeTermsWriter.OUTPUT_FLAG_IS_FLOOR.
const vbtOutputFlagIsFloor = 0x1

// vbtOutputFlagHasTerms marks the block as containing terms.
// Mirrors VersionBlockTreeTermsWriter.OUTPUT_FLAG_HAS_TERMS.
const vbtOutputFlagHasTerms = 0x2

// vbtFSTOutputs is the PairOutputs used by VersionBlockTree for its FST.
// Mirrors VersionBlockTreeTermsWriter.FST_OUTPUTS.
var vbtFSTOutputs = fst.NewPairOutputs(
	fst.ByteSequenceOutputs(),
	fst.PositiveIntOutputs(),
)

// vbtNoOutput is the no-output sentinel for the pair outputs.
// Mirrors VersionBlockTreeTermsWriter.NO_OUTPUT.
var vbtNoOutput = vbtFSTOutputs.GetNoOutput()

// IDVersionSegmentTermsEnum iterates through terms in a single field of an
// IDVersion index. It is public so that callers can cast it to call
// SeekExactWithVersion for optimistic-concurrency and GetVersion to retrieve
// the version of the currently seek'd term.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.IDVersionSegmentTermsEnum.
type IDVersionSegmentTermsEnum struct {
	index.TermsEnumBase

	// in is the lazily-initialised clone of the terms (.tim) file.
	in store.IndexInput

	// stack is the frame stack; grows on demand.
	stack []*idVersionSegmentTermsEnumFrame

	// staticFrame is a sentinel frame used before the first seek.
	staticFrame *idVersionSegmentTermsEnumFrame

	// currentFrame is the currently active frame.
	currentFrame *idVersionSegmentTermsEnumFrame

	// termExists is true when the current term is a real term (not a
	// sub-block pointer).
	termExists bool

	// fr is the owning VersionFieldReader.
	fr *VersionFieldReader

	// targetBeforeCurrentLength is the frame ordinal of the last seeked frame.
	targetBeforeCurrentLength int

	scratchReader *store.ByteArrayDataInput

	// validIndexPrefix is the length of the prefix confirmed by the FST index
	// during the last seek.
	validIndexPrefix int

	// eof is true when the enum has been exhausted.
	eof bool

	// term accumulates the current term bytes.
	term util.BytesRefBuilder

	// fstReader is the bytes reader for the FST index.
	fstReader fst.BytesReader

	// arcs holds the per-depth FST arcs used during seek.
	arcs []*fst.Arc[*fst.Pair[*util.BytesRef, int64]]
}

// newIDVersionSegmentTermsEnum constructs an IDVersionSegmentTermsEnum for the
// given VersionFieldReader.
//
// Mirrors IDVersionSegmentTermsEnum(VersionFieldReader).
func newIDVersionSegmentTermsEnum(fr *VersionFieldReader) (*IDVersionSegmentTermsEnum, error) {
	if fr == nil {
		return nil, fmt.Errorf("newIDVersionSegmentTermsEnum: VersionFieldReader must not be nil")
	}

	e := &IDVersionSegmentTermsEnum{
		fr:            fr,
		stack:         make([]*idVersionSegmentTermsEnumFrame, 0),
		arcs:          make([]*fst.Arc[*fst.Pair[*util.BytesRef, int64]], 1),
		scratchReader: store.NewByteArrayDataInput(nil),
	}

	// allocate the arc slice entries
	for i := range e.arcs {
		e.arcs[i] = &fst.Arc[*fst.Pair[*util.BytesRef, int64]]{}
	}

	var err error
	e.staticFrame, err = newIDVersionSegmentTermsEnumFrame(e, -1)
	if err != nil {
		return nil, fmt.Errorf("newIDVersionSegmentTermsEnum: static frame: %w", err)
	}

	if fr.Index != nil {
		e.fstReader = fr.Index.GetBytesReader()
	}

	// Load the root arc if an FST index is present. Empty-string prefix must
	// have a final arc with output.
	if fr.Index != nil {
		arc := fr.Index.GetFirstArc(e.arcs[0])
		if !arc.IsFinal() {
			return nil, fmt.Errorf("newIDVersionSegmentTermsEnum: root FST arc must be final")
		}
	}

	e.currentFrame = e.staticFrame
	return e, nil
}

// initIndexInput lazily clones the terms file IndexInput from the parent reader.
func (e *IDVersionSegmentTermsEnum) initIndexInput() {
	if e.in == nil {
		parent := e.fr.Parent
		if parent == nil || parent.In == nil {
			return
		}
		indexIn, ok := parent.In.(store.IndexInput)
		if !ok {
			return
		}
		e.in = indexIn.Clone()
	}
}

// getFrame grows the frame stack to at least ord+1 entries and returns
// stack[ord].
func (e *IDVersionSegmentTermsEnum) getFrame(ord int) (*idVersionSegmentTermsEnumFrame, error) {
	if ord >= len(e.stack) {
		newLen := util.Oversize(1+ord, 1)
		grown := make([]*idVersionSegmentTermsEnumFrame, newLen)
		copy(grown, e.stack)
		for i := len(e.stack); i < newLen; i++ {
			f, err := newIDVersionSegmentTermsEnumFrame(e, i)
			if err != nil {
				return nil, fmt.Errorf("getFrame: ord=%d: %w", i, err)
			}
			grown[i] = f
		}
		e.stack = grown
	}
	return e.stack[ord], nil
}

// getArc grows the arc slice to at least ord+1 entries and returns arcs[ord].
func (e *IDVersionSegmentTermsEnum) getArc(ord int) *fst.Arc[*fst.Pair[*util.BytesRef, int64]] {
	if ord >= len(e.arcs) {
		newLen := util.Oversize(1+ord, 1)
		grown := make([]*fst.Arc[*fst.Pair[*util.BytesRef, int64]], newLen)
		copy(grown, e.arcs)
		for i := len(e.arcs); i < newLen; i++ {
			grown[i] = &fst.Arc[*fst.Pair[*util.BytesRef, int64]]{}
		}
		e.arcs = grown
	}
	return e.arcs[ord]
}

// pushFrameData pushes a frame sought via an FST arc and pair output.
//
// Mirrors IDVersionSegmentTermsEnum.pushFrame(Arc, Pair, int) — the variant
// that decodes the fp and flags from the pair output1 field.
func (e *IDVersionSegmentTermsEnum) pushFrameData(
	arc *fst.Arc[*fst.Pair[*util.BytesRef, int64]],
	frameData *fst.Pair[*util.BytesRef, int64],
	length int,
) (*idVersionSegmentTermsEnumFrame, error) {
	e.scratchReader.ResetWithSlice(
		frameData.Output1.Bytes,
		frameData.Output1.Offset,
		frameData.Output1.Length,
	)
	code, err := e.scratchReader.ReadVLong()
	if err != nil {
		return nil, fmt.Errorf("pushFrameData: read code: %w", err)
	}
	fpSeek := int64(uint64(code) >> vbtOutputFlagsNumBits)

	f, err := e.getFrame(1 + e.currentFrame.ord)
	if err != nil {
		return nil, err
	}
	// maxIDVersion uses Long.MAX_VALUE - output2 (inverted to allow min-heap semantics).
	f.maxIDVersion = int64(^uint64(0)>>1) - int64(frameData.Output2)
	f.hasTerms = (code & vbtOutputFlagHasTerms) != 0
	f.hasTermsOrig = f.hasTerms
	f.isFloor = (code & vbtOutputFlagIsFloor) != 0
	if f.isFloor {
		f.setFloorData(e.scratchReader, frameData.Output1)
	}
	return e.pushFrameFP(arc, fpSeek, length)
}

// pushFrameFP pushes (or reuses) a frame for the given fp.
//
// Mirrors IDVersionSegmentTermsEnum.pushFrame(Arc, long, int) — the variant
// that takes a raw file pointer.
func (e *IDVersionSegmentTermsEnum) pushFrameFP(
	arc *fst.Arc[*fst.Pair[*util.BytesRef, int64]],
	fp int64,
	length int,
) (*idVersionSegmentTermsEnumFrame, error) {
	f, err := e.getFrame(1 + e.currentFrame.ord)
	if err != nil {
		return nil, err
	}
	f.arc = arc
	if f.fpOrig == fp && f.nextEnt != -1 {
		if f.prefixLength > e.targetBeforeCurrentLength {
			f.rewind()
		}
		// assert length == f.prefixLength
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

// SeekExact seeks to the exact term. Delegates to SeekExactWithVersion(target, 0).
func (e *IDVersionSegmentTermsEnum) SeekExact(term *index.Term) (bool, error) {
	target := term.BytesValue()
	return e.SeekExactWithVersion(target, 0)
}

// GetVersion returns the version of the currently seek'd term. Only valid
// after a successful SeekExact / SeekExactWithVersion call.
func (e *IDVersionSegmentTermsEnum) GetVersion() int64 {
	extra := globalTermStateRegistry.lookup(e.currentFrame.state)
	if extra == nil {
		return 0
	}
	return extra.IDVersion
}

// SeekExactWithVersion is the optimised seekExact that can fast-fail if the
// version stored with the requested ID is below minIDVersion.
//
// Mirrors IDVersionSegmentTermsEnum.seekExact(BytesRef, long).
func (e *IDVersionSegmentTermsEnum) SeekExactWithVersion(target *util.BytesRef, minIDVersion int64) (bool, error) {
	if e.fr.Index == nil {
		return false, fmt.Errorf("IDVersionSegmentTermsEnum.SeekExactWithVersion: terms index was not loaded")
	}

	e.term.Grow(1 + target.Length)
	e.eof = false

	var (
		arc    *fst.Arc[*fst.Pair[*util.BytesRef, int64]]
		output *fst.Pair[*util.BytesRef, int64]
	)
	targetUpto := 0
	startFrameFP := e.currentFrame.fp
	changed := false

	e.targetBeforeCurrentLength = e.currentFrame.ord

	if e.currentFrame != e.staticFrame {
		// Re-use existing seek state: find the common prefix.
		arc = e.arcs[0]
		output = arc.Output()
		targetUpto = 0
		lastFrame := e.stack[0]
		targetLimit := target.Length
		if validIndexPrefix := e.validIndexPrefix; validIndexPrefix < targetLimit {
			targetLimit = validIndexPrefix
		}
		cmp := 0

		for targetUpto < targetLimit {
			cmp = int(e.term.ByteAt(targetUpto)&0xff) - int(target.Bytes[target.Offset+targetUpto]&0xff)
			if cmp != 0 {
				break
			}
			arc = e.arcs[1+targetUpto]
			no := vbtFSTOutputs.GetNoOutput()
			if arc.Output() != no {
				output = vbtFSTOutputs.Add(output, arc.Output())
			}
			if arc.IsFinal() {
				lastFrame = e.stack[1+lastFrame.ord]
			}
			targetUpto++
		}

		if cmp == 0 {
			termBytes := e.term.Bytes()[:e.term.Length()]
			targetBytes := target.Bytes[target.Offset+targetUpto : target.Offset+target.Length]
			cmp = bytes.Compare(termBytes[targetUpto:], targetBytes)
		}

		if cmp < 0 {
			e.currentFrame = lastFrame
		} else if cmp > 0 {
			e.targetBeforeCurrentLength = 0
			changed = true
			e.currentFrame = lastFrame
			e.currentFrame.rewind()
		} else {
			// exact same term
			if e.termExists {
				if e.currentFrame.maxIDVersion < minIDVersion {
					return false, nil
				}
				if err := e.currentFrame.decodeMetaData(); err != nil {
					return false, err
				}
				extra := globalTermStateRegistry.lookup(e.currentFrame.state)
				if extra != nil && extra.IDVersion < minIDVersion {
					return false, nil
				}
				return true, nil
			}
		}
	} else {
		e.targetBeforeCurrentLength = -1
		arc = e.fr.Index.GetFirstArc(e.arcs[0])
		output = arc.Output()
		e.currentFrame = e.staticFrame
		targetUpto = 0
		addedOutput := arc.NextFinalOutput()
		sumOutput := vbtFSTOutputs.Add(output, addedOutput)
		var err error
		e.currentFrame, err = e.pushFrameData(arc, sumOutput, 0)
		if err != nil {
			return false, err
		}
	}

	// Walk the FST index to find the target.
	for targetUpto < target.Length {
		targetLabel := int(target.Bytes[target.Offset+targetUpto]) & 0xff

		nextArc, err := e.fr.Index.FindTargetArc(targetLabel, arc, e.getArc(1+targetUpto), e.fstReader)
		if err != nil {
			return false, fmt.Errorf("IDVersionSegmentTermsEnum.SeekExactWithVersion: FindTargetArc: %w", err)
		}

		if nextArc == nil {
			// Index exhausted: scan the current block.
			e.validIndexPrefix = e.currentFrame.prefixLength
			e.currentFrame.scanToFloorFrame(target)

			if !e.currentFrame.hasTerms {
				e.termExists = false
				e.term.SetByteAt(targetUpto, byte(targetLabel))
				e.term.SetLength(1 + targetUpto)
				return false, nil
			}

			if e.currentFrame.maxIDVersion < minIDVersion {
				if e.currentFrame.fp != startFrameFP || changed {
					e.termExists = false
					e.term.SetByteAt(targetUpto, byte(targetLabel))
					e.term.SetLength(1 + targetUpto)
					e.validIndexPrefix = min(e.validIndexPrefix, e.term.Length())
				}
				return false, nil
			}

			if err := e.currentFrame.loadBlock(); err != nil {
				return false, err
			}
			result, err := e.currentFrame.scanToTerm(target, true)
			if err != nil {
				return false, err
			}
			if result == index.SeekStatusFound {
				if err := e.currentFrame.decodeMetaData(); err != nil {
					return false, err
				}
				extra := globalTermStateRegistry.lookup(e.currentFrame.state)
				if extra != nil && extra.IDVersion < minIDVersion {
					return false, nil
				}
				return true, nil
			}
			return false, nil
		}

		// Follow this arc.
		arc = nextArc
		if e.term.ByteAt(targetUpto) != byte(targetLabel) {
			changed = true
			e.term.SetByteAt(targetUpto, byte(targetLabel))
			e.termExists = false
		}
		no := vbtFSTOutputs.GetNoOutput()
		if arc.Output() != no {
			output = vbtFSTOutputs.Add(output, arc.Output())
		}
		targetUpto++

		if arc.IsFinal() {
			addedOutput := arc.NextFinalOutput()
			sumOutput := vbtFSTOutputs.Add(output, addedOutput)
			var err error
			e.currentFrame, err = e.pushFrameData(arc, sumOutput, targetUpto)
			if err != nil {
				return false, err
			}
		}
	}

	e.validIndexPrefix = e.currentFrame.prefixLength
	e.currentFrame.scanToFloorFrame(target)

	if !e.currentFrame.hasTerms {
		e.termExists = false
		e.term.SetLength(targetUpto)
		return false, nil
	}

	if e.currentFrame.maxIDVersion < minIDVersion {
		e.termExists = false
		e.term.SetLength(targetUpto)
		return false, nil
	}

	if err := e.currentFrame.loadBlock(); err != nil {
		return false, err
	}
	result, err := e.currentFrame.scanToTerm(target, true)
	if err != nil {
		return false, err
	}
	if result == index.SeekStatusFound {
		if err := e.currentFrame.decodeMetaData(); err != nil {
			return false, err
		}
		extra := globalTermStateRegistry.lookup(e.currentFrame.state)
		if extra != nil && extra.IDVersion < minIDVersion {
			return false, nil
		}
		return true, nil
	}
	return false, nil
}

// SeekCeil seeks to the first term >= text.
//
// Mirrors IDVersionSegmentTermsEnum.seekCeil(BytesRef).
func (e *IDVersionSegmentTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	target := term.BytesValue()
	if e.fr.Index == nil {
		return nil, fmt.Errorf("IDVersionSegmentTermsEnum.SeekCeil: terms index was not loaded")
	}
	e.term.Grow(1 + target.Length)
	e.eof = false

	var (
		arc    *fst.Arc[*fst.Pair[*util.BytesRef, int64]]
		output *fst.Pair[*util.BytesRef, int64]
	)
	targetUpto := 0
	e.targetBeforeCurrentLength = e.currentFrame.ord

	if e.currentFrame != e.staticFrame {
		arc = e.arcs[0]
		output = arc.Output()
		targetUpto = 0
		lastFrame := e.stack[0]
		targetLimit := target.Length
		if vip := e.validIndexPrefix; vip < targetLimit {
			targetLimit = vip
		}
		cmp := 0

		for targetUpto < targetLimit {
			cmp = int(e.term.ByteAt(targetUpto)&0xff) - int(target.Bytes[target.Offset+targetUpto]&0xff)
			if cmp != 0 {
				break
			}
			arc = e.arcs[1+targetUpto]
			no := vbtFSTOutputs.GetNoOutput()
			if arc.Output() != no {
				output = vbtFSTOutputs.Add(output, arc.Output())
			}
			if arc.IsFinal() {
				lastFrame = e.stack[1+lastFrame.ord]
			}
			targetUpto++
		}

		if cmp == 0 {
			termBytes := e.term.Bytes()[:e.term.Length()]
			targetBytes := target.Bytes[target.Offset+targetUpto : target.Offset+target.Length]
			cmp = bytes.Compare(termBytes[targetUpto:], targetBytes)
		}

		if cmp < 0 {
			e.currentFrame = lastFrame
		} else if cmp > 0 {
			e.targetBeforeCurrentLength = 0
			e.currentFrame = lastFrame
			e.currentFrame.rewind()
		} else {
			if e.termExists {
				return e.currentIndexTerm(), nil
			}
		}
	} else {
		e.targetBeforeCurrentLength = -1
		arc = e.fr.Index.GetFirstArc(e.arcs[0])
		output = arc.Output()
		e.currentFrame = e.staticFrame
		targetUpto = 0
		addedOutput := arc.NextFinalOutput()
		sumOutput := vbtFSTOutputs.Add(output, addedOutput)
		var err error
		e.currentFrame, err = e.pushFrameData(arc, sumOutput, 0)
		if err != nil {
			return nil, err
		}
	}

	for targetUpto < target.Length {
		targetLabel := int(target.Bytes[target.Offset+targetUpto]) & 0xff
		nextArc, err := e.fr.Index.FindTargetArc(targetLabel, arc, e.getArc(1+targetUpto), e.fstReader)
		if err != nil {
			return nil, fmt.Errorf("IDVersionSegmentTermsEnum.SeekCeil: FindTargetArc: %w", err)
		}

		if nextArc == nil {
			e.validIndexPrefix = e.currentFrame.prefixLength
			e.currentFrame.scanToFloorFrame(target)
			if err := e.currentFrame.loadBlock(); err != nil {
				return nil, err
			}
			result, err := e.currentFrame.scanToTerm(target, false)
			if err != nil {
				return nil, err
			}
			if result == index.SeekStatusEnd {
				e.term.CopyBytesRef(target)
				e.termExists = false
				t, err := e.Next()
				if err != nil {
					return nil, err
				}
				if t != nil {
					return t, nil
				}
				return nil, nil // END
			}
			return e.currentIndexTerm(), nil
		}

		e.term.SetByteAt(targetUpto, byte(targetLabel))
		arc = nextArc
		no := vbtFSTOutputs.GetNoOutput()
		if arc.Output() != no {
			output = vbtFSTOutputs.Add(output, arc.Output())
		}
		targetUpto++

		if arc.IsFinal() {
			addedOutput := arc.NextFinalOutput()
			sumOutput := vbtFSTOutputs.Add(output, addedOutput)
			var err error
			e.currentFrame, err = e.pushFrameData(arc, sumOutput, targetUpto)
			if err != nil {
				return nil, err
			}
		}
	}

	e.validIndexPrefix = e.currentFrame.prefixLength
	e.currentFrame.scanToFloorFrame(target)
	if err := e.currentFrame.loadBlock(); err != nil {
		return nil, err
	}
	result, err := e.currentFrame.scanToTerm(target, false)
	if err != nil {
		return nil, err
	}
	if result == index.SeekStatusEnd {
		e.term.CopyBytesRef(target)
		e.termExists = false
		t, err := e.Next()
		if err != nil {
			return nil, err
		}
		return t, nil
	}
	return e.currentIndexTerm(), nil
}

// Next advances to the next term.
//
// Mirrors IDVersionSegmentTermsEnum.next().
func (e *IDVersionSegmentTermsEnum) Next() (*index.Term, error) {
	if e.in == nil {
		// Fresh enum: seek to first term.
		if e.fr.Index != nil {
			arc := e.fr.Index.GetFirstArc(e.arcs[0])
			var err error
			e.currentFrame, err = e.pushFrameData(arc, e.fr.RootCode, 0)
			if err != nil {
				return nil, err
			}
			if err := e.currentFrame.loadBlock(); err != nil {
				return nil, err
			}
		}
	}

	e.targetBeforeCurrentLength = e.currentFrame.ord
	e.eof = false

	if e.currentFrame == e.staticFrame {
		// Re-seek to pending term.
		found, err := e.SeekExactWithVersion(e.term.Get(), 0)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("IDVersionSegmentTermsEnum.Next: re-seek to pending term failed")
		}
	}

	// Pop finished blocks.
	for e.currentFrame.nextEnt == e.currentFrame.entCount {
		if !e.currentFrame.isLastInFloor {
			if err := e.currentFrame.loadNextFloorBlock(); err != nil {
				return nil, err
			}
		} else {
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
				e.currentFrame.scanToSubBlock(lastFP)
			}
			e.validIndexPrefix = min(e.validIndexPrefix, e.currentFrame.prefixLength)
		}
	}

	for {
		if e.currentFrame.next() {
			// Push to new block.
			var err error
			e.currentFrame, err = e.pushFrameFP(nil, e.currentFrame.lastSubFP, e.term.Length())
			if err != nil {
				return nil, err
			}
			e.currentFrame.isFloor = false
			if err := e.currentFrame.loadBlock(); err != nil {
				return nil, err
			}
		} else {
			return e.currentIndexTerm(), nil
		}
	}
}

// Term returns the current term.
func (e *IDVersionSegmentTermsEnum) Term() *index.Term {
	b := e.term.Get()
	if b == nil || b.Length == 0 {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.FieldInfo.Name(), b)
}

// DocFreq returns 1 (IDVersion stores exactly one document per term).
func (e *IDVersionSegmentTermsEnum) DocFreq() (int, error) {
	if e.eof {
		return 0, fmt.Errorf("IDVersionSegmentTermsEnum.DocFreq: not positioned")
	}
	return 1, nil
}

// TotalTermFreq returns 1.
func (e *IDVersionSegmentTermsEnum) TotalTermFreq() (int64, error) {
	if e.eof {
		return 0, fmt.Errorf("IDVersionSegmentTermsEnum.TotalTermFreq: not positioned")
	}
	return 1, nil
}

// Postings returns a PostingsEnum for the current term.
func (e *IDVersionSegmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if e.eof {
		return nil, fmt.Errorf("IDVersionSegmentTermsEnum.Postings: not positioned")
	}
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return nil, err
	}
	return e.fr.Parent.PostingsReader.Postings(e.fr.FieldInfo, e.currentFrame.state, nil, flags)
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term (live docs
// are not applied — IDVersion indexes have one doc per term).
func (e *IDVersionSegmentTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// SeekExactWithState repositions the enum to the given term using a cached
// TermState.
func (e *IDVersionSegmentTermsEnum) SeekExactWithState(target *index.Term, state index.TermState) error {
	if state == nil {
		return fmt.Errorf("IDVersionSegmentTermsEnum.SeekExactWithState: state must not be nil")
	}
	e.eof = false
	if target.BytesValue().BytesRefCompareTo(e.term.Get()) != 0 || !e.termExists {
		e.currentFrame = e.staticFrame
		e.term.CopyBytesRef(target.BytesValue())
		e.currentFrame.metaDataUpto = e.currentFrame.getTermBlockOrd()
		e.validIndexPrefix = 0
	}
	return nil
}

// TermState returns a snapshot of the current term state.
func (e *IDVersionSegmentTermsEnum) TermState() (index.TermState, error) {
	if e.eof {
		return nil, fmt.Errorf("IDVersionSegmentTermsEnum.TermState: not positioned")
	}
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return nil, err
	}
	snap := e.currentFrame.state.Clone()
	return &blockTermStateWrapper{snap}, nil
}

// blockTermStateWrapper adapts *codecs.BlockTermState to satisfy
// index.TermState. This avoids importing codecs into the index package.
type blockTermStateWrapper struct {
	bts *codecs.BlockTermState
}

// CopyFrom copies another TermState into this wrapper.
func (w *blockTermStateWrapper) CopyFrom(other index.TermState) error {
	o, ok := other.(*blockTermStateWrapper)
	if !ok {
		return fmt.Errorf("blockTermStateWrapper.CopyFrom: incompatible type %T", other)
	}
	w.bts.CopyFrom(o.bts)
	return nil
}

var _ index.TermState = (*blockTermStateWrapper)(nil)

// String returns a human-readable description.
func (e *IDVersionSegmentTermsEnum) String() string {
	return fmt.Sprintf("IDVersionSegmentTermsEnum(seg=%v)", e.fr.Parent)
}

// currentIndexTerm returns the current term as an *index.Term.
func (e *IDVersionSegmentTermsEnum) currentIndexTerm() *index.Term {
	b := e.term.Get()
	if b == nil || b.Length == 0 {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.FieldInfo.Name(), b)
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// compile-time assertion
var _ index.TermsEnum = (*IDVersionSegmentTermsEnum)(nil)
