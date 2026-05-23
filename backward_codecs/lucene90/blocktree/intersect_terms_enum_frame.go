// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// intersectTermsEnumFrame holds the per-level state for one level of the
// block-tree terms dictionary during IntersectTermsEnum traversal.
//
// Lucene 9.0 (vs. the Lucene 4.0 variant in backward_codecs/lucene40) drops
// the outputPrefix BytesRef in favour of the OutputAccumulator on the owning
// enum; the outputNum field records how many arc outputs this frame pushed so
// they can be popped on exit.
//
// The full block-loading and navigation logic (load, loadNextFloorBlock, next,
// decodeMetaData) is deferred until FieldReader is fully ported.
//
// Port of
// org.apache.lucene.backward_codecs.lucene90.blocktree.IntersectTermsEnumFrame
// (Lucene 10.4.0).
type intersectTermsEnumFrame struct {
	// ord is the frame's depth in the stack (0 = root).
	ord int

	fp        int64
	fpOrig    int64
	fpEnd     int64
	lastSubFP int64

	// state is the automaton state upon entering this block.
	state int

	// lastState is the automaton state before the most recent suffix byte.
	lastState int

	metaDataUpto int

	suffixBytes    []byte
	suffixesReader *store.ByteArrayDataInput

	suffixLengthBytes   []byte
	suffixLengthsReader *store.ByteArrayDataInput

	statBytes               []byte
	statsSingletonRunLength int
	statsReader             *store.ByteArrayDataInput

	floorDataReader *store.ByteArrayDataInput

	// prefix is the length of the prefix shared by all terms in this block.
	prefix int

	// entCount is the number of entries (term or sub-block) in this block.
	entCount int

	// nextEnt is the index of the next entry to decode (-1 = not loaded).
	nextEnt int

	isLastInFloor bool
	isLeafBlock   bool

	numFollowFloorBlocks int
	nextFloorLabel       int

	// transition is the current automaton transition being evaluated.
	transition      automaton.Transition
	transitionIndex int
	transitionCount int

	arc *fst.Arc[*util.BytesRef]

	// termState holds lazily-decoded per-term metadata.
	termState *codecs.BlockTermState

	// bytes and bytesReader hold encoded per-term metadata for lazy decode.
	bytes       []byte
	bytesReader *store.ByteArrayDataInput

	// outputNum records the number of FST arc outputs pushed to the parent
	// IntersectTermsEnum.outputAccum from this frame's arc chain, so they can
	// be popped on exit.
	outputNum int

	// startBytePos / suffix are scratch fields updated by next* helpers.
	startBytePos int
	suffix       int

	// ite is the owning IntersectTermsEnum.
	ite *IntersectTermsEnum
}

// newIntersectTermsEnumFrame constructs a frame for the given
// IntersectTermsEnum at stack ordinal ord.
//
// termState is nil when the owning FieldReader has not been fully wired
// (its parent and postingsReader are still stub-only).
//
// Port of IntersectTermsEnumFrame(IntersectTermsEnum, int).
func newIntersectTermsEnumFrame(ite *IntersectTermsEnum, ord int) *intersectTermsEnumFrame {
	f := &intersectTermsEnumFrame{
		ite:                 ite,
		ord:                 ord,
		suffixBytes:         make([]byte, 128),
		suffixesReader:      store.NewByteArrayDataInput(nil),
		suffixLengthBytes:   make([]byte, 32),
		suffixLengthsReader: store.NewByteArrayDataInput(nil),
		statBytes:           make([]byte, 64),
		statsReader:         store.NewByteArrayDataInput(nil),
		floorDataReader:     store.NewByteArrayDataInput(nil),
		bytes:               make([]byte, 32),
		bytesReader:         store.NewByteArrayDataInput(nil),
		nextEnt:             -1,
	}
	// termState is intentionally nil when FieldReader is a stub (parent and
	// postingsReader not yet wired).  Callers that need termState must defer
	// until the FieldReader is fully ported.
	return f
}

// getTermBlockOrd returns the number of terms decoded in the current block.
//
// Port of IntersectTermsEnumFrame.getTermBlockOrd().
func (f *intersectTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	if f.termState == nil {
		return 0
	}
	return f.termState.TermBlockOrd
}

// setState sets the automaton state for this frame and initialises the
// transition iterator.
//
// Port of IntersectTermsEnumFrame.setState(int).
func (f *intersectTermsEnumFrame) setState(state int) {
	f.state = state
	f.transitionIndex = 0
	if f.ite == nil || f.ite.transitionAccessor == nil {
		f.transitionCount = 0
		f.transition.Min = -1
		f.transition.Max = -1
		return
	}
	f.transitionCount = f.ite.transitionAccessor.GetNumTransitions(state)
	if f.transitionCount != 0 {
		f.ite.transitionAccessor.InitTransition(state, &f.transition)
		f.ite.transitionAccessor.GetNextTransition(&f.transition)
	} else {
		// Must set Min to -1 so the "label < min" check never falsely triggers.
		f.transition.Min = -1
		// Must set Max to -1 so we immediately step to the next transition and
		// then pop this frame.
		f.transition.Max = -1
	}
}
