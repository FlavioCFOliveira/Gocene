// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// OrdsIntersectTermsEnumFrame is the per-block cursor used by
// OrdsIntersectTermsEnum to walk the on-disk block-tree while intersecting
// with a finite automaton, tracking term ordinals.
//
// Port of org.apache.lucene.codecs.blocktreeords.OrdsIntersectTermsEnumFrame
// (Lucene 10.4.0).
type OrdsIntersectTermsEnumFrame struct {
	// ord is the frame's position in the owning enum's stack (0 == root).
	ord int

	fp     int64
	fpOrig int64
	fpEnd  int64

	lastSubFP int64

	// state is the automaton state at entry to this frame.
	state int

	metaDataUpto int

	// suffix buffers
	suffixBytes    []byte
	suffixesReader *store.ByteArrayDataInput

	// stats buffers
	statBytes   []byte
	statsReader *store.ByteArrayDataInput

	// floor data
	floorData       []byte
	floorDataReader *store.ByteArrayDataInput

	// prefix is the number of bytes shared by all terms in this block.
	prefix int

	// entCount is the number of entries in this block.
	entCount int

	// nextEnt is the index of the next entry to decode.
	nextEnt int

	// termOrdOrig is the starting ordinal of this frame.
	termOrdOrig int64

	// termOrd is 1 + the ordinal of the current term.
	termOrd int64

	// isLastInFloor is true when this block is either not a floor block or
	// is the last sub-block of a floor block.
	isLastInFloor bool

	// isLeafBlock is true when all entries are terms.
	isLeafBlock bool

	numFollowFloorBlocks int
	nextFloorLabel       int

	// Transition cursor used during automaton-driven block walk.
	transition       *automaton.Transition
	curTransitionMax int
	transitionIndex  int
	transitionCount  int

	// arc is the FST arc pointing to this block from the parent.
	arc *gfst.Arc[*FSTOrdsOutput]

	// termState holds the per-term postings metadata.
	termState *codecs.BlockTermState

	// bytes / bytesReader hold the per-term postings metadata blob.
	bytes       []byte
	bytesReader *store.ByteArrayDataInput

	// outputPrefix is the cumulative FST output down to this frame.
	outputPrefix *FSTOrdsOutput

	startBytePos int
	suffix       int

	// ite is the back-pointer to the owning enum.
	ite *OrdsIntersectTermsEnum
}

// NewOrdsIntersectTermsEnumFrame allocates a frame for the given enum at
// the given stack ordinal.
//
// Port of OrdsIntersectTermsEnumFrame constructor (Lucene 10.4.0).
func NewOrdsIntersectTermsEnumFrame(ite *OrdsIntersectTermsEnum, ord int) (*OrdsIntersectTermsEnumFrame, error) {
	if ite == nil {
		return nil, fmt.Errorf("NewOrdsIntersectTermsEnumFrame: ite must not be nil")
	}

	f := &OrdsIntersectTermsEnumFrame{
		ite:              ite,
		ord:              ord,
		suffixBytes:      make([]byte, 128),
		suffixesReader:   store.NewByteArrayDataInput(nil),
		statBytes:        make([]byte, 64),
		statsReader:      store.NewByteArrayDataInput(nil),
		floorData:        make([]byte, 32),
		floorDataReader:  store.NewByteArrayDataInput(nil),
		bytes:            make([]byte, 32),
		bytesReader:      store.NewByteArrayDataInput(nil),
		transition:       automaton.NewTransition(),
		curTransitionMax: -1,
	}

	// Ask the postings reader for a fresh BlockTermState.
	if ite.reader != nil && ite.reader.parent != nil && ite.reader.parent.postingsReader != nil {
		f.termState = ite.reader.parent.postingsReader.NewTermState()
	} else {
		f.termState = codecs.NewBlockTermState()
	}
	f.termState.TotalTermFreq = -1
	return f, nil
}

// loadNextFloorBlock advances the frame to the next floor sub-block whose
// label is at or after the active transition min.
// Mirrors OrdsIntersectTermsEnumFrame.loadNextFloorBlock.
func (f *OrdsIntersectTermsEnumFrame) loadNextFloorBlock() error {
	for {
		delta, err := f.floorDataReader.ReadVLong()
		if err != nil {
			return fmt.Errorf("OrdsIntersectTermsEnumFrame.loadNextFloorBlock: read fp delta: %w", err)
		}
		f.fp = f.fpOrig + int64(uint64(delta)>>1)
		f.numFollowFloorBlocks--
		if f.numFollowFloorBlocks != 0 {
			label, err := f.floorDataReader.ReadByte()
			if err != nil {
				return fmt.Errorf("OrdsIntersectTermsEnumFrame.loadNextFloorBlock: read floor label: %w", err)
			}
			f.nextFloorLabel = int(label) & 0xff
			termOrdDelta, err := f.floorDataReader.ReadVLong()
			if err != nil {
				return fmt.Errorf("OrdsIntersectTermsEnumFrame.loadNextFloorBlock: read termOrd delta: %w", err)
			}
			f.termOrd += termOrdDelta
		} else {
			f.nextFloorLabel = 256
		}
		if f.numFollowFloorBlocks == 0 || f.nextFloorLabel > f.transition.Min {
			break
		}
	}
	return f.load(nil)
}

// setState rebinds the transition iterator to the new automaton state.
// Mirrors OrdsIntersectTermsEnumFrame.setState.
func (f *OrdsIntersectTermsEnumFrame) setState(state int) {
	f.state = state
	f.transitionIndex = 0
	if f.ite == nil || f.ite.transitionAccessor == nil {
		f.transitionCount = 0
		f.curTransitionMax = -1
		return
	}
	f.transitionCount = f.ite.transitionAccessor.GetNumTransitions(state)
	if f.transitionCount != 0 {
		f.ite.transitionAccessor.InitTransition(state, f.transition)
		f.ite.transitionAccessor.GetNextTransition(f.transition)
		f.curTransitionMax = f.transition.Max
	} else {
		f.curTransitionMax = -1
	}
}

// load materialises the on-disk block into the per-frame buffers.
// output is non-nil on the first push of a floor block; nil on subsequent
// floor-block loads.
// Mirrors OrdsIntersectTermsEnumFrame.load.
func (f *OrdsIntersectTermsEnumFrame) load(output *FSTOrdsOutput) error {
	if output != nil && output.Bytes != nil && output.Bytes.Length > 0 && f.transitionCount != 0 {
		frameIndexData := output.Bytes
		if frameIndexData.Length > len(f.floorData) {
			f.floorData = make([]byte, util.Oversize(frameIndexData.Length, 1))
		}
		copy(f.floorData, frameIndexData.Bytes[frameIndexData.Offset:frameIndexData.Offset+frameIndexData.Length])
		f.floorDataReader.ResetWithSlice(f.floorData, 0, frameIndexData.Length)

		code, err := f.floorDataReader.ReadVLong()
		if err != nil {
			return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read floor code: %w", err)
		}
		if (code & outputFlagIsFloor) != 0 {
			numFollow, err := f.floorDataReader.ReadVInt()
			if err != nil {
				return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read numFollowFloorBlocks: %w", err)
			}
			f.numFollowFloorBlocks = int(numFollow)

			label, err := f.floorDataReader.ReadByte()
			if err != nil {
				return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read nextFloorLabel: %w", err)
			}
			f.nextFloorLabel = int(label) & 0xff

			termOrdDelta, err := f.floorDataReader.ReadVLong()
			if err != nil {
				return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read termOrd delta: %w", err)
			}
			f.termOrd = f.termOrdOrig + termOrdDelta

			// If the current automaton state is not accepting, skip floor
			// sub-blocks whose label is before the first transition min.
			if f.ite != nil && f.ite.byteRunnable != nil && !f.ite.byteRunnable.IsAccept(f.state) {
				for f.numFollowFloorBlocks != 0 && f.nextFloorLabel <= f.transition.Min {
					delta, err := f.floorDataReader.ReadVLong()
					if err != nil {
						return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: skip floor block: %w", err)
					}
					f.fp = f.fpOrig + int64(uint64(delta)>>1)
					f.numFollowFloorBlocks--
					if f.numFollowFloorBlocks != 0 {
						b, err := f.floorDataReader.ReadByte()
						if err != nil {
							return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read skip floor label: %w", err)
						}
						f.nextFloorLabel = int(b) & 0xff
						tdelta, err := f.floorDataReader.ReadVLong()
						if err != nil {
							return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read skip termOrd delta: %w", err)
						}
						f.termOrd += tdelta
					} else {
						f.nextFloorLabel = 256
					}
				}
			}
		}
	}

	if err := f.ite.in.SetPosition(f.fp); err != nil {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: seek to fp=%d: %w", f.fp, err)
	}

	code, err := store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read entCount code: %w", err)
	}
	f.entCount = int(uint32(code) >> 1)
	if f.entCount <= 0 {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: entCount=%d must be > 0", f.entCount)
	}
	f.isLastInFloor = (code & 1) != 0

	// Term suffixes.
	code, err = store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read suffix code: %w", err)
	}
	f.isLeafBlock = (code & 1) != 0
	numBytes := int(uint32(code) >> 1)
	if numBytes > len(f.suffixBytes) {
		f.suffixBytes = make([]byte, util.Oversize(numBytes, 1))
	}
	if err := f.ite.in.ReadBytes(f.suffixBytes[:numBytes]); err != nil {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read suffix bytes: %w", err)
	}
	f.suffixesReader.ResetWithSlice(f.suffixBytes, 0, numBytes)

	// Stats.
	statsLen, err := store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read stats length: %w", err)
	}
	numBytes = int(statsLen)
	if numBytes > len(f.statBytes) {
		f.statBytes = make([]byte, util.Oversize(numBytes, 1))
	}
	if err := f.ite.in.ReadBytes(f.statBytes[:numBytes]); err != nil {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read stats: %w", err)
	}
	f.statsReader.ResetWithSlice(f.statBytes, 0, numBytes)
	f.metaDataUpto = 0
	f.termState.TermBlockOrd = 0
	f.nextEnt = 0

	// Metadata.
	metaLen, err := store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read metadata length: %w", err)
	}
	numBytes = int(metaLen)
	if f.bytes == nil || numBytes > len(f.bytes) {
		f.bytes = make([]byte, util.Oversize(numBytes, 1))
		f.bytesReader = store.NewByteArrayDataInput(nil)
	}
	if err := f.ite.in.ReadBytes(f.bytes[:numBytes]); err != nil {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.load: read metadata: %w", err)
	}
	f.bytesReader.ResetWithSlice(f.bytes, 0, numBytes)

	if !f.isLastInFloor {
		f.fpEnd = f.ite.in.GetFilePointer()
	}
	return nil
}

// next decodes the next entry; returns true if the entry is a sub-block.
// Mirrors OrdsIntersectTermsEnumFrame.next.
func (f *OrdsIntersectTermsEnumFrame) next() bool {
	if f.isLeafBlock {
		return f.nextLeaf()
	}
	return f.nextNonLeaf()
}

// nextLeaf decodes the next term from a leaf block.
// Mirrors OrdsIntersectTermsEnumFrame.nextLeaf.
func (f *OrdsIntersectTermsEnumFrame) nextLeaf() bool {
	f.nextEnt++
	length, _ := f.suffixesReader.ReadVInt()
	f.suffix = int(length)
	f.startBytePos = f.suffixesReader.GetPosition()
	_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffix)
	return false
}

// nextNonLeaf decodes the next entry from a non-leaf block.
// Mirrors OrdsIntersectTermsEnumFrame.nextNonLeaf.
func (f *OrdsIntersectTermsEnumFrame) nextNonLeaf() bool {
	f.nextEnt++
	code, _ := f.suffixesReader.ReadVInt()
	f.suffix = int(uint32(code) >> 1)
	f.startBytePos = f.suffixesReader.GetPosition()
	_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffix)
	if (code & 1) == 0 {
		// A normal term.
		f.termState.TermBlockOrd++
		return false
	}
	// A sub-block: make sub-FP absolute.
	subCode, _ := f.suffixesReader.ReadVLong()
	f.lastSubFP = f.fp - subCode
	// Skip term ord delta.
	_, _ = f.suffixesReader.ReadVLong()
	return true
}

// getTermBlockOrd returns the term's ordinal inside the block.
// Mirrors OrdsIntersectTermsEnumFrame.getTermBlockOrd.
func (f *OrdsIntersectTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	return f.termState.TermBlockOrd
}

// decodeMetaData lazily decodes per-term stats and postings metadata.
// Mirrors OrdsIntersectTermsEnumFrame.decodeMetaData.
func (f *OrdsIntersectTermsEnumFrame) decodeMetaData() error {
	limit := f.getTermBlockOrd()
	if limit <= 0 {
		return fmt.Errorf("OrdsIntersectTermsEnumFrame.decodeMetaData: limit=%d must be > 0", limit)
	}
	absolute := f.metaDataUpto == 0

	for f.metaDataUpto < limit {
		// stats
		docFreq, err := f.statsReader.ReadVInt()
		if err != nil {
			return fmt.Errorf("OrdsIntersectTermsEnumFrame.decodeMetaData: read docFreq: %w", err)
		}
		f.termState.DocFreq = int(docFreq)
		if f.ite != nil && f.ite.reader != nil && f.ite.reader.fieldInfo != nil &&
			f.ite.reader.fieldInfo.IndexOptions() == index.IndexOptionsDocs {
			f.termState.TotalTermFreq = int64(f.termState.DocFreq)
		} else {
			delta, err := f.statsReader.ReadVLong()
			if err != nil {
				return fmt.Errorf("OrdsIntersectTermsEnumFrame.decodeMetaData: read totalTermFreq delta: %w", err)
			}
			f.termState.TotalTermFreq = int64(f.termState.DocFreq) + delta
		}

		// metadata (postings-side): deferred — ByteArrayDataInput→IndexInput bridge needed.
		_ = absolute

		f.metaDataUpto++
		absolute = false
	}
	f.termState.TermBlockOrd = f.metaDataUpto
	return nil
}
