// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// ErrBlockTraversalNotAvailable is returned by IntersectTermsEnum navigation
// methods that require full block-tree traversal logic (loadBlock / nextEntry),
// which is deferred until FieldReader is fully ported for lucene90.
var ErrBlockTraversalNotAvailable = errors.New(
	"lucene90 blocktree: full block traversal not yet implemented",
)

// outputAccumulator accumulates FST arc outputs across a frame chain so that
// the concatenated output can be fed to a BlockTreeTermsReader when loading
// floor blocks. It mirrors the static inner class
// SegmentTermsEnum.OutputAccumulator in Lucene90BlockTreeTermsReader.java.
//
// It also acts as a DataInput so that pushFrame can call readVLongOutput
// directly on the accumulated byte sequence. The read cursor is managed by
// prepareRead / readByte.
type outputAccumulator struct {
	outputs []*util.BytesRef
	num     int

	// Read-cursor fields — set by prepareRead; advanced by readByte.
	outputIndex int // index into outputs[] of the BytesRef currently being read
	index       int // byte offset within outputs[outputIndex]
}

// push appends output to the accumulator (no-op for nil or empty outputs).
func (a *outputAccumulator) push(output *util.BytesRef) {
	if output == nil || output.Length == 0 {
		return
	}
	if a.num >= len(a.outputs) {
		grown := make([]*util.BytesRef, util.Oversize(a.num+1, 1))
		copy(grown, a.outputs)
		a.outputs = grown
	}
	a.outputs[a.num] = output
	a.num++
}

// pop removes the top output (must match the pushed value by identity).
func (a *outputAccumulator) pop(output *util.BytesRef) {
	if output == nil || output.Length == 0 {
		return
	}
	a.num--
}

// popN removes n entries from the top.
func (a *outputAccumulator) popN(n int) {
	a.num -= n
}

// outputCount returns the number of accumulated outputs.
func (a *outputAccumulator) outputCount() int { return a.num }

// reset clears the accumulator so it can be reused for a new seek.
// Port of OutputAccumulator.reset().
func (a *outputAccumulator) reset() {
	a.num = 0
}

// prepareRead resets the read cursor to the start of the first output.
// Must be called before readByte. Port of OutputAccumulator.prepareRead().
func (a *outputAccumulator) prepareRead() {
	a.outputIndex = 0
	a.index = 0
}

// readByte reads the next byte from the accumulated outputs, advancing the
// cursor across BytesRef boundaries. Port of OutputAccumulator.readByte().
func (a *outputAccumulator) readByte() (byte, error) {
	cur := a.outputs[a.outputIndex]
	if a.index >= cur.Length {
		a.outputIndex++
		if a.outputIndex >= a.num {
			return 0, errors.New("outputAccumulator: read past end")
		}
		cur = a.outputs[a.outputIndex]
		a.index = 0
	}
	b := cur.Bytes[cur.Offset+a.index]
	a.index++
	return b, nil
}

// setFloorData positions floorReader at the floor data within the last pushed
// output, starting at the current read cursor. Port of
// OutputAccumulator.setFloorData(ByteArrayDataInput).
func (a *outputAccumulator) setFloorData(floorReader *store.ByteArrayDataInput) {
	output := a.outputs[a.outputIndex]
	floorReader.ResetWithSlice(output.Bytes, output.Offset+a.index, output.Length-a.index)
}

// IntersectTermsEnum implements efficient intersection of the lucene90
// block-tree terms dictionary with a compiled automaton.
//
// Lucene 9.0 differs from Lucene 4.0 (see lucene40/blocktree) in its use of
// an OutputAccumulator to accumulate FST arc outputs across the frame chain,
// rather than tracking a single cumulative BytesRef per frame.
//
// Navigation (next, seekToStartTerm) is deferred until FieldReader is fully
// ported. Until then, all traversal methods return
// ErrBlockTraversalNotAvailable.
//
// Port of
// org.apache.lucene.backward_codecs.lucene90.blocktree.IntersectTermsEnum
// (Lucene 10.4.0).
type IntersectTermsEnum struct {
	index.TermsEnumBase

	// fr is the owning FieldReader.
	fr *FieldReader

	// runAutomaton is the byte-runnable compiled automaton.
	runAutomaton automaton.ByteRunnable

	// transitionAccessor provides ordered transition access.
	transitionAccessor automaton.TransitionAccessor

	// commonSuffix is the automaton's required common suffix (may be nil).
	commonSuffix *util.BytesRef

	// stack holds per-level frame state; grown on demand.
	stack []*intersectTermsEnumFrame

	// currentFrame is the active frame.
	currentFrame *intersectTermsEnumFrame

	// term holds the current term bytes.
	term *util.BytesRef

	// savedStartTerm records the initial seek target (for assertions).
	savedStartTerm *util.BytesRef

	// outputAccum accumulates FST arc outputs across the frame chain.
	outputAccum outputAccumulator
}

// newIntersectTermsEnum constructs an IntersectTermsEnum for the given
// FieldReader and compiled automaton.
//
// startTerm is the lower bound for the initial seek; may be nil to start at
// the beginning of the field.
//
// Port of IntersectTermsEnum(FieldReader, TransitionAccessor, ByteRunnable,
// BytesRef, BytesRef).
func newIntersectTermsEnum(
	fr *FieldReader,
	compiled *automaton.CompiledAutomaton,
	startTerm *util.BytesRef,
) (*IntersectTermsEnum, error) {
	if fr == nil {
		return nil, fmt.Errorf("lucene90 blocktree: FieldReader must not be nil")
	}
	if compiled == nil {
		return nil, fmt.Errorf("lucene90 blocktree: compiled automaton must not be nil")
	}

	e := &IntersectTermsEnum{
		fr:                 fr,
		runAutomaton:       compiled.RunAutomaton,
		transitionAccessor: compiled.Automaton,
		commonSuffix:       compiled.CommonSuffixRef,
		term:               &util.BytesRef{},
	}

	// Initialise frame stack with 5 pre-allocated frames.
	e.stack = make([]*intersectTermsEnumFrame, 5)
	for i := range e.stack {
		e.stack[i] = newIntersectTermsEnumFrame(e, i)
	}

	// Record start term for assertion / seek purposes.
	if startTerm != nil {
		cp := *startTerm
		e.savedStartTerm = &cp
	}

	// Root frame occupies stack[0].
	e.currentFrame = e.stack[0]

	// Full init (cloning termsIn from fr.parent, loading root block, running
	// seekToStartTerm) is deferred until FieldReader is fully ported.

	return e, nil
}

// getFrame grows the frame stack to at least ord+1 entries and returns
// stack[ord].
func (e *IntersectTermsEnum) getFrame(ord int) *intersectTermsEnumFrame {
	if ord >= len(e.stack) {
		grown := make([]*intersectTermsEnumFrame, util.Oversize(ord+1, 1))
		copy(grown, e.stack)
		for i := len(e.stack); i < len(grown); i++ {
			grown[i] = newIntersectTermsEnumFrame(e, i)
		}
		e.stack = grown
	}
	return e.stack[ord]
}

// Term returns the current term.
func (e *IntersectTermsEnum) Term() *index.Term {
	if e.term == nil || e.term.Length == 0 {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.Name, e.term)
}

// Next advances to the next term matching the automaton.
//
// Deferred: requires full block traversal logic and a real FieldReader.
func (e *IntersectTermsEnum) Next() (*index.Term, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// SeekCeil is not supported by IntersectTermsEnum.
func (e *IntersectTermsEnum) SeekCeil(_ *index.Term) (*index.Term, error) {
	return nil, fmt.Errorf("lucene90 blocktree: IntersectTermsEnum does not support SeekCeil")
}

// SeekExact is not supported by IntersectTermsEnum.
func (e *IntersectTermsEnum) SeekExact(_ *index.Term) (bool, error) {
	return false, fmt.Errorf("lucene90 blocktree: IntersectTermsEnum does not support SeekExact")
}

// DocFreq returns the document frequency of the current term.
//
// Deferred: requires metadata decoding.
func (e *IntersectTermsEnum) DocFreq() (int, error) {
	return 0, ErrBlockTraversalNotAvailable
}

// TotalTermFreq returns the total term frequency of the current term.
//
// Deferred: requires metadata decoding.
func (e *IntersectTermsEnum) TotalTermFreq() (int64, error) {
	return 0, ErrBlockTraversalNotAvailable
}

// Postings returns a PostingsEnum for the current term.
//
// Deferred: requires metadata decoding and PostingsReaderBase wiring.
func (e *IntersectTermsEnum) Postings(_ int) (index.PostingsEnum, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term.
//
// Deferred: requires metadata decoding and PostingsReaderBase wiring.
func (e *IntersectTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (index.PostingsEnum, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// compile-time assertion
var _ index.TermsEnum = (*IntersectTermsEnum)(nil)
