// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.codecs.idversion.IDVersionSegmentTermsEnumFrame.
package idversion

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// idVersionSegmentTermsEnumFrame is a single block-tree frame used by
// IDVersionSegmentTermsEnum. It mirrors
// org.apache.lucene.sandbox.codecs.idversion.IDVersionSegmentTermsEnumFrame.
//
// Each frame corresponds to one floor-block level in the block-tree terms
// dictionary. The enum maintains a stack of frames; the active frame is
// currentFrame.
type idVersionSegmentTermsEnumFrame struct {
	// ord is the depth of this frame; -1 for the static (sentinel) frame.
	ord int

	// maxIDVersion is the maximum version of any term in this block. The enum
	// uses this to fast-fail seekExact when minIDVersion is set.
	maxIDVersion int64

	// arc is the FST arc that led to this frame (nil for root/static frame).
	arc interface{} // *fst.Arc[*fst.Pair[*util.BytesRef, int64]] — held as interface{} to avoid generic-parameter noise at the frame level; the enum accesses it via type assertion.

	// fp / fpOrig / fpEnd are file pointers into the terms (.tim) file.
	fp     int64
	fpOrig int64
	fpEnd  int64

	// suffixBytes / suffixesReader hold the decoded suffix bytes for entries.
	suffixBytes    []byte
	suffixesReader *store.ByteArrayDataInput

	// floorData / floorDataReader hold floor-block metadata.
	floorData       []byte
	floorDataReader *store.ByteArrayDataInput

	// prefixLength is the depth (prefix shared by all terms in this block).
	prefixLength int

	// entCount is the number of entries (terms + sub-blocks) in the block.
	entCount int

	// nextEnt is the index of the next entry to decode (-1 = block not loaded).
	nextEnt int

	// isLastInFloor is true when this is the last sub-block of a floor block,
	// or the block is not a floor block at all.
	isLastInFloor bool

	// isLeafBlock is true when every entry in the block is a term.
	isLeafBlock bool

	// lastSubFP is the file pointer of the last sub-block seen.
	lastSubFP int64

	// nextFloorLabel is the first byte label of the next floor sub-block.
	nextFloorLabel int

	// numFollowFloorBlocks is the number of floor sub-blocks not yet visited.
	numFollowFloorBlocks int

	// metaDataUpto tracks how many terms in this block have had their metadata
	// lazily decoded.
	metaDataUpto int

	// hasTerms / hasTermsOrig record whether the block contains any terms.
	hasTerms     bool
	hasTermsOrig bool

	// isFloor is true when this frame was entered via a floor-block arc.
	isFloor bool

	// state is the per-term postings metadata for the current term.
	// It is a *codecs.BlockTermState allocated by the postings reader; the
	// extra IDVersion / DocID fields are stored in globalTermStateRegistry.
	state *codecs.BlockTermState

	// bytes_ / bytesReader hold the raw per-term postings metadata blob.
	bytes_      []byte
	bytesReader *store.ByteArrayDataInput

	// startBytePos / suffixLength / subCode are scratch variables set during
	// scanToTermLeaf / scanToTermNonLeaf.
	startBytePos int
	suffixLength int
	subCode      int64

	// ste is the owning IDVersionSegmentTermsEnum.
	ste *IDVersionSegmentTermsEnum
}

// newIDVersionSegmentTermsEnumFrame constructs a frame at the given ordinal.
//
// Mirrors IDVersionSegmentTermsEnumFrame(IDVersionSegmentTermsEnum, int).
func newIDVersionSegmentTermsEnumFrame(ste *IDVersionSegmentTermsEnum, ord int) (*idVersionSegmentTermsEnumFrame, error) {
	if ste == nil {
		return nil, fmt.Errorf("newIDVersionSegmentTermsEnumFrame: ste must not be nil")
	}

	bts := ste.fr.Parent.PostingsReader.NewTermState()
	bts.TotalTermFreq = -1

	f := &idVersionSegmentTermsEnumFrame{
		ord:             ord,
		maxIDVersion:    0,
		suffixBytes:     make([]byte, 128),
		suffixesReader:  store.NewByteArrayDataInput(nil),
		floorData:       make([]byte, 32),
		floorDataReader: store.NewByteArrayDataInput(nil),
		lastSubFP:       -1,
		nextFloorLabel:  0,
		bytes_:          make([]byte, 32),
		bytesReader:     store.NewByteArrayDataInput(nil),
		ste:             ste,
		state:           bts,
	}

	return f, nil
}

// setFloorData copies the floor-block bytes from source starting at the
// current position in in.
//
// Mirrors IDVersionSegmentTermsEnumFrame.setFloorData.
func (f *idVersionSegmentTermsEnumFrame) setFloorData(in *store.ByteArrayDataInput, source *util.BytesRef) {
	numBytes := source.Length - (in.GetPosition() - source.Offset)
	if len(f.floorData) < numBytes {
		f.floorData = make([]byte, util.Oversize(numBytes, 1))
	}
	copy(f.floorData[:numBytes], source.Bytes[source.Offset+in.GetPosition():])
	f.floorDataReader.ResetWithSlice(f.floorData, 0, numBytes)
	numFollowFloor, _ := f.floorDataReader.ReadVInt()
	f.numFollowFloorBlocks = int(numFollowFloor)
	nextLabel, _ := f.floorDataReader.ReadByte()
	f.nextFloorLabel = int(nextLabel) & 0xff
}

// getTermBlockOrd returns the term block ordinal for the current position.
func (f *idVersionSegmentTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	return f.state.TermBlockOrd
}

// loadNextFloorBlock advances to the next floor sub-block.
//
// Mirrors IDVersionSegmentTermsEnumFrame.loadNextFloorBlock.
func (f *idVersionSegmentTermsEnumFrame) loadNextFloorBlock() error {
	f.fp = f.fpEnd
	f.nextEnt = -1
	return f.loadBlock()
}

// loadBlock reads and decodes the current block from the terms file.
//
// Mirrors IDVersionSegmentTermsEnumFrame.loadBlock.
func (f *idVersionSegmentTermsEnumFrame) loadBlock() error {
	f.ste.initIndexInput()

	if f.nextEnt != -1 {
		return nil // already loaded
	}

	if err := f.ste.in.SetPosition(f.fp); err != nil {
		return fmt.Errorf("IDVersionSegmentTermsEnumFrame.loadBlock: seek to fp=%d: %w", f.fp, err)
	}

	vli, ok := f.ste.in.(store.VariableLengthInput)
	if !ok {
		return fmt.Errorf("IDVersionSegmentTermsEnumFrame.loadBlock: IndexInput does not implement VariableLengthInput")
	}

	code, err := vli.ReadVInt()
	if err != nil {
		return fmt.Errorf("IDVersionSegmentTermsEnumFrame.loadBlock: read entCount code: %w", err)
	}
	f.entCount = int(code >> 1)
	f.isLastInFloor = (code & 1) != 0

	// suffix bytes
	code, err = vli.ReadVInt()
	if err != nil {
		return fmt.Errorf("IDVersionSegmentTermsEnumFrame.loadBlock: read suffix code: %w", err)
	}
	f.isLeafBlock = (code & 1) != 0
	numBytes := int(code >> 1)
	if len(f.suffixBytes) < numBytes {
		f.suffixBytes = make([]byte, util.Oversize(numBytes, 1))
	}
	if err := f.ste.in.ReadBytes(f.suffixBytes[:numBytes]); err != nil {
		return fmt.Errorf("IDVersionSegmentTermsEnumFrame.loadBlock: read suffixes: %w", err)
	}
	f.suffixesReader.ResetWithSlice(f.suffixBytes, 0, numBytes)

	f.metaDataUpto = 0
	f.state.TermBlockOrd = 0
	f.nextEnt = 0
	f.lastSubFP = -1

	// metadata blob
	numBytesV, err := vli.ReadVInt()
	if err != nil {
		return fmt.Errorf("IDVersionSegmentTermsEnumFrame.loadBlock: read metadata size: %w", err)
	}
	numBytes = int(numBytesV)
	if len(f.bytes_) < numBytes {
		f.bytes_ = make([]byte, util.Oversize(numBytes, 1))
	}
	if err := f.ste.in.ReadBytes(f.bytes_[:numBytes]); err != nil {
		return fmt.Errorf("IDVersionSegmentTermsEnumFrame.loadBlock: read metadata: %w", err)
	}
	f.bytesReader.ResetWithSlice(f.bytes_, 0, numBytes)

	f.fpEnd = f.ste.in.GetFilePointer()
	return nil
}

// rewind resets the frame to re-read from fpOrig.
//
// Mirrors IDVersionSegmentTermsEnumFrame.rewind.
func (f *idVersionSegmentTermsEnumFrame) rewind() {
	f.fp = f.fpOrig
	f.nextEnt = -1
	f.hasTerms = f.hasTermsOrig
	if f.isFloor {
		if err := f.floorDataReader.SetPosition(0); err == nil {
			numFollowFloor, _ := f.floorDataReader.ReadVInt()
			f.numFollowFloorBlocks = int(numFollowFloor)
			nextLabel, _ := f.floorDataReader.ReadByte()
			f.nextFloorLabel = int(nextLabel) & 0xff
		}
	}
}

// next advances to the next entry. Returns true if the entry is a sub-block.
func (f *idVersionSegmentTermsEnumFrame) next() bool {
	if f.isLeafBlock {
		return f.nextLeaf()
	}
	return f.nextNonLeaf()
}

// nextLeaf decodes the next entry in a leaf block.
func (f *idVersionSegmentTermsEnumFrame) nextLeaf() bool {
	f.nextEnt++
	suffLen, _ := f.suffixesReader.ReadVInt()
	f.suffixLength = int(suffLen)
	f.startBytePos = f.suffixesReader.GetPosition()
	newLen := f.prefixLength + f.suffixLength
	f.ste.term.SetLength(newLen)
	f.ste.term.Grow(newLen)
	_ = f.suffixesReader.ReadBytes(f.ste.term.Bytes()[f.prefixLength : f.prefixLength+f.suffixLength])
	f.ste.termExists = true
	return false
}

// nextNonLeaf decodes the next entry in a non-leaf block.
func (f *idVersionSegmentTermsEnumFrame) nextNonLeaf() bool {
	f.nextEnt++
	code, _ := f.suffixesReader.ReadVInt()
	f.suffixLength = int(code >> 1)
	f.startBytePos = f.suffixesReader.GetPosition()
	newLen := f.prefixLength + f.suffixLength
	f.ste.term.SetLength(newLen)
	f.ste.term.Grow(newLen)
	_ = f.suffixesReader.ReadBytes(f.ste.term.Bytes()[f.prefixLength : f.prefixLength+f.suffixLength])
	if (code & 1) == 0 {
		f.ste.termExists = true
		f.subCode = 0
		f.state.TermBlockOrd++
		return false
	}
	f.ste.termExists = false
	subCode, _ := f.suffixesReader.ReadVLong()
	f.subCode = int64(subCode)
	f.lastSubFP = f.fp - f.subCode
	return true
}

// scanToFloorFrame advances the frame to the correct floor sub-block for
// target.
//
// Mirrors IDVersionSegmentTermsEnumFrame.scanToFloorFrame.
func (f *idVersionSegmentTermsEnumFrame) scanToFloorFrame(target *util.BytesRef) {
	if !f.isFloor || target.Length <= f.prefixLength {
		return
	}
	targetLabel := int(target.Bytes[target.Offset+f.prefixLength]) & 0xff
	if targetLabel < f.nextFloorLabel {
		return
	}

	var newFP = f.fpOrig
	for {
		code, _ := f.floorDataReader.ReadVLong()
		newFP = f.fpOrig + int64(code>>1)
		f.hasTerms = (code & 1) != 0
		f.isLastInFloor = f.numFollowFloorBlocks == 1
		f.numFollowFloorBlocks--
		if f.isLastInFloor {
			f.nextFloorLabel = 256
			break
		}
		nextLabel, _ := f.floorDataReader.ReadByte()
		f.nextFloorLabel = int(nextLabel) & 0xff
		if targetLabel < f.nextFloorLabel {
			break
		}
	}
	if newFP != f.fp {
		f.nextEnt = -1
		f.fp = newFP
	}
}

// decodeMetaData lazily decodes term metadata up to the current term block
// ordinal.
//
// Mirrors IDVersionSegmentTermsEnumFrame.decodeMetaData.
func (f *idVersionSegmentTermsEnumFrame) decodeMetaData() error {
	limit := f.getTermBlockOrd()
	absolute := f.metaDataUpto == 0

	for f.metaDataUpto < limit {
		f.state.DocFreq = 1
		f.state.TotalTermFreq = 1
		if err := f.ste.fr.Parent.PostingsReader.DecodeTermFromBytesReader(
			f.bytesReader, f.state, absolute,
		); err != nil {
			return fmt.Errorf("IDVersionSegmentTermsEnumFrame.decodeMetaData: %w", err)
		}
		f.metaDataUpto++
		absolute = false
	}
	f.state.TermBlockOrd = f.metaDataUpto
	return nil
}

// prefixMatches returns true when target shares the same prefix bytes as the
// current frame.
func (f *idVersionSegmentTermsEnumFrame) prefixMatches(target *util.BytesRef) bool {
	for i := 0; i < f.prefixLength; i++ {
		if target.Bytes[target.Offset+i] != f.ste.term.ByteAt(i) {
			return false
		}
	}
	return true
}

// scanToSubBlock advances the suffix reader to the sub-block with the given
// sub-FP.
//
// Mirrors IDVersionSegmentTermsEnumFrame.scanToSubBlock.
func (f *idVersionSegmentTermsEnumFrame) scanToSubBlock(subFP int64) {
	if f.lastSubFP == subFP {
		return
	}
	targetSubCode := f.fp - subFP
	for {
		f.nextEnt++
		code, _ := f.suffixesReader.ReadVInt()
		var skipLen int
		if f.isLeafBlock {
			skipLen = int(code)
		} else {
			skipLen = int(code >> 1)
		}
		_ = f.suffixesReader.SetPosition(f.suffixesReader.GetPosition() + skipLen)
		if (code & 1) != 0 {
			subCode, _ := f.suffixesReader.ReadVLong()
			if targetSubCode == int64(subCode) {
				f.lastSubFP = subFP
				return
			}
		} else {
			f.state.TermBlockOrd++
		}
	}
}

// scanToTerm dispatches to the leaf or non-leaf scan.
func (f *idVersionSegmentTermsEnumFrame) scanToTerm(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.isLeafBlock {
		return f.scanToTermLeaf(target, exactOnly)
	}
	return f.scanToTermNonLeaf(target, exactOnly)
}

// scanToTermLeaf scans entries in a leaf block for target.
//
// Mirrors IDVersionSegmentTermsEnumFrame.scanToTermLeaf.
func (f *idVersionSegmentTermsEnumFrame) scanToTermLeaf(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	f.ste.termExists = true
	f.subCode = 0

	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
		}
		return index.SeekStatusEnd, nil
	}

	for {
		f.nextEnt++
		suffLen, _ := f.suffixesReader.ReadVInt()
		f.suffixLength = int(suffLen)
		f.startBytePos = f.suffixesReader.GetPosition()
		_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength)

		// Compare suffix vs target suffix.
		cmp := bytes.Compare(
			f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength],
			target.Bytes[target.Offset+f.prefixLength:target.Offset+target.Length],
		)
		if cmp < 0 {
			// keep scanning
		} else if cmp > 0 {
			f.fillTerm()
			return index.SeekStatusNotFound, nil
		} else {
			f.fillTerm()
			return index.SeekStatusFound, nil
		}

		if f.nextEnt == f.entCount {
			if exactOnly {
				f.fillTerm()
			}
			return index.SeekStatusEnd, nil
		}
	}
}

// scanToTermNonLeaf scans entries in a non-leaf block for target.
//
// Mirrors IDVersionSegmentTermsEnumFrame.scanToTermNonLeaf.
func (f *idVersionSegmentTermsEnumFrame) scanToTermNonLeaf(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
			f.ste.termExists = f.subCode == 0
		}
		return index.SeekStatusEnd, nil
	}

	for f.nextEnt < f.entCount {
		f.nextEnt++
		code, _ := f.suffixesReader.ReadVInt()
		f.suffixLength = int(code >> 1)
		f.ste.termExists = (code & 1) == 0
		f.startBytePos = f.suffixesReader.GetPosition()
		_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength)
		if f.ste.termExists {
			f.state.TermBlockOrd++
			f.subCode = 0
		} else {
			subCode, _ := f.suffixesReader.ReadVLong()
			f.subCode = int64(subCode)
			f.lastSubFP = f.fp - f.subCode
		}

		cmp := bytes.Compare(
			f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength],
			target.Bytes[target.Offset+f.prefixLength:target.Offset+target.Length],
		)
		if cmp < 0 {
			continue
		} else if cmp > 0 {
			f.fillTerm()
			if !exactOnly && !f.ste.termExists {
				pushed, err := f.ste.pushFrameFP(nil, f.lastSubFP, f.prefixLength+f.suffixLength)
				if err != nil {
					return 0, err
				}
				if err := pushed.loadBlock(); err != nil {
					return 0, err
				}
				for pushed.next() {
					pushed, err = f.ste.pushFrameFP(nil, pushed.lastSubFP, f.ste.term.Length())
					if err != nil {
						return 0, err
					}
					if err := pushed.loadBlock(); err != nil {
						return 0, err
					}
				}
			}
			return index.SeekStatusNotFound, nil
		} else {
			f.fillTerm()
			return index.SeekStatusFound, nil
		}
	}

	if exactOnly {
		f.fillTerm()
	}
	return index.SeekStatusEnd, nil
}

// fillTerm copies the current suffix into ste.term.
func (f *idVersionSegmentTermsEnumFrame) fillTerm() {
	termLength := f.prefixLength + f.suffixLength
	f.ste.term.SetLength(termLength)
	f.ste.term.Grow(termLength)
	copy(f.ste.term.Bytes()[f.prefixLength:], f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength])
}
