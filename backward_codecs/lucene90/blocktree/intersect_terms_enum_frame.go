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

	// termStateRef is the same term state as PostingsReaderBase sees it: the
	// interface value whose dynamic type is the codec's own BlockTermState
	// subclass, which the codec narrows back with a type assertion. The
	// BlockTermState field beside it is the widened view of that same object.
	termStateRef index.TermState

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
	// Java's IntersectTermsEnumFrame(IntersectTermsEnum, int) does:
	//
	//	this.termState = ite.fr.parent.postingsReader.newTermState();
	//	this.termState.totalTermFreq = -1;
	//
	// Gocene additionally has a stub FieldReader path (NewFieldReader), which
	// Lucene does not, and whose parent reader is nil; termState stays nil
	// there and decodeMetaData reports it rather than decoding.
	if ite != nil && ite.fr != nil && ite.fr.parent != nil && ite.fr.parent.postingsReader != nil {
		f.termStateRef = ite.fr.parent.postingsReader.NewTermState()
		f.termState = codecs.BaseState(f.termStateRef)
		f.termState.TotalTermFreq = -1
	}
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

// decodeMetaData lazily decodes per-term statistics and postings metadata for
// every term up to getTermBlockOrd().
//
// Port of
// org.apache.lucene.backward_codecs.lucene90.blocktree.IntersectTermsEnumFrame#decodeMetaData()
// in Apache Lucene 10.5.0.
func (f *intersectTermsEnumFrame) decodeMetaData() error {
	if f.termState == nil {
		return fmt.Errorf("lucene90 blocktree: intersect frame has no term state (FieldReader is not wired)")
	}

	limit := f.getTermBlockOrd()
	absolute := f.metaDataUpto == 0

	for f.metaDataUpto < limit {
		// stats
		if f.statsSingletonRunLength > 0 {
			f.termState.DocFreq = 1
			f.termState.TotalTermFreq = 1
			f.statsSingletonRunLength--
		} else {
			token, err := f.statsReader.ReadVInt()
			if err != nil {
				return fmt.Errorf("lucene90 blocktree: intersect decodeMetaData: read stats token: %w", err)
			}
			if token&1 == 1 {
				f.termState.DocFreq = 1
				f.termState.TotalTermFreq = 1
				f.statsSingletonRunLength = int(uint32(token) >> 1)
			} else {
				f.termState.DocFreq = int(uint32(token) >> 1)
				if f.ite.fr.fieldInfo.IndexOptions() == index.IndexOptionsDocs {
					f.termState.TotalTermFreq = int64(f.termState.DocFreq)
				} else {
					delta, err := f.statsReader.ReadVLong()
					if err != nil {
						return fmt.Errorf("lucene90 blocktree: intersect decodeMetaData: read total term freq: %w", err)
					}
					f.termState.TotalTermFreq = int64(f.termState.DocFreq) + delta
				}
			}
		}

		// metadata
		if err := f.ite.fr.parent.postingsReader.DecodeTerm(
			f.bytesReader,
			f.ite.fr.fieldInfo,
			f.termStateRef,
			absolute,
		); err != nil {
			return fmt.Errorf("lucene90 blocktree: intersect decodeMetaData: decode term: %w", err)
		}

		f.metaDataUpto++
		absolute = false
	}
	f.termState.TermBlockOrd = f.metaDataUpto
	return nil
}
