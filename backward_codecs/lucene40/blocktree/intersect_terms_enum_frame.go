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
// block tree during IntersectTermsEnum traversal.
//
// Port of the package-private class
// org.apache.lucene.backward_codecs.lucene40.blocktree.IntersectTermsEnumFrame
// (Lucene 10.4.0).
//
// The full block-loading and navigation logic (load, next, decodeMetaData,
// etc.) is deferred to a later sprint; only the field declarations and a
// constructor are provided here.
type intersectTermsEnumFrame struct {
	ord int

	fp        int64
	fpOrig    int64
	fpEnd     int64
	lastSubFP int64

	// Automaton state at the start of this block.
	state     int
	lastState int

	metaDataUpto int

	suffixBytes    []byte
	suffixesReader *store.ByteArrayDataInput

	suffixLengthBytes   []byte
	suffixLengthsReader *store.ByteArrayDataInput

	statBytes               []byte
	statsSingletonRunLength int
	statsReader             *store.ByteArrayDataInput

	floorData       []byte
	floorDataReader *store.ByteArrayDataInput

	// prefix is the length of the prefix shared by all terms in this block.
	prefix int

	// entCount is the number of entries (term or sub-block) in this block.
	entCount int

	// nextEnt is the index of the next entry to read.
	nextEnt int

	isLastInFloor bool
	isLeafBlock   bool

	numFollowFloorBlocks int
	nextFloorLabel       int

	// transition is the current automaton transition being matched.
	transition      automaton.Transition
	transitionIndex int
	transitionCount int

	arc *fst.Arc[*util.BytesRef]

	// termState holds decoded per-term metadata.
	termState *codecs.BlockTermState

	// bytes holds encoded per-term metadata (lazy-decoded).
	bytes       []byte
	bytesReader *store.ByteArrayDataInput

	// outputPrefix is the cumulative FST output up to this frame.
	outputPrefix *util.BytesRef

	startBytePos int
	suffix       int

	// ite is the owning IntersectTermsEnum.
	ite *IntersectTermsEnum

	version int32
}

// newIntersectTermsEnumFrame constructs a frame for the given
// IntersectTermsEnum at stack ordinal ord.
//
// Port of IntersectTermsEnumFrame(IntersectTermsEnum, int).
func newIntersectTermsEnumFrame(ite *IntersectTermsEnum, ord int) *intersectTermsEnumFrame {
	f := &intersectTermsEnumFrame{
		ite:             ite,
		ord:             ord,
		suffixBytes:     make([]byte, 128),
		suffixesReader:  store.NewByteArrayDataInput(nil),
		statBytes:       make([]byte, 64),
		statsReader:     store.NewByteArrayDataInput(nil),
		floorData:       make([]byte, 32),
		floorDataReader: store.NewByteArrayDataInput(nil),
		bytes:           make([]byte, 32),
		bytesReader:     store.NewByteArrayDataInput(nil),
		termState:       ite.fr.parent.postingsReader.NewTermState(),
		version:         ite.fr.parent.version,
	}
	if f.termState != nil {
		f.termState.TotalTermFreq = -1
	}

	if f.version >= VersionCompressedSuffixes {
		f.suffixLengthBytes = make([]byte, 32)
		f.suffixLengthsReader = store.NewByteArrayDataInput(nil)
	} else {
		f.suffixLengthsReader = f.suffixesReader
	}

	return f
}

// getTermBlockOrd returns the number of terms decoded so far in this block.
func (f *intersectTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	if f.termState == nil {
		return 0
	}
	return f.termState.TermBlockOrd
}
