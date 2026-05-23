// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// SeekStatus is the result of OrdsSegmentTermsEnumFrame.scanToTerm.
type SeekStatus int

const (
	// SeekStatusEnd means no more terms.
	SeekStatusEnd SeekStatus = iota
	// SeekStatusFound means the term was found exactly.
	SeekStatusFound
	// SeekStatusNotFound means the current term is the first term after the target.
	SeekStatusNotFound
)

// OrdsSegmentTermsEnumFrame is the per-block cursor used by
// OrdsSegmentTermsEnum to descend the on-disk block-tree, tracking term
// ordinals.
//
// Port of org.apache.lucene.codecs.blocktreeords.OrdsSegmentTermsEnumFrame
// (Lucene 10.4.0).
type OrdsSegmentTermsEnumFrame struct {
	// ord is the frame's position in the owning enum's stack (0 == root).
	ord int

	hasTerms     bool
	hasTermsOrig bool
	isFloor      bool

	// arc is the FST arc pointing to this block from the parent.
	arc *gfst.Arc[*FSTOrdsOutput]

	// fp is the file pointer of the current floor sub-block.
	// fpOrig is the file pointer of the parent floor block.
	// fpEnd is the end of the parent block (for tail-recursion).
	fp     int64
	fpOrig int64
	fpEnd  int64

	// suffix buffers
	suffixBytes    []byte
	suffixesReader *store.ByteArrayDataInput

	// stats buffers
	statBytes   []byte
	statsReader *store.ByteArrayDataInput

	// floor data
	floorData       []byte
	floorDataReader *store.ByteArrayDataInput

	// prefixLength is the number of bytes shared by all terms in this block.
	prefixLength int

	// entCount is the number of entries (term or sub-block) in this block.
	entCount int

	// nextEnt is the next entry to decode; -1 means the block is not yet loaded.
	nextEnt int

	// termOrdOrig is the starting ordinal of this frame (used to reset in rewind).
	termOrdOrig int64

	// termOrd is 1 + the ordinal of the current term.
	termOrd int64

	// isLastInFloor is true when this block is either not a floor block, or
	// is the last sub-block of a floor block.
	isLastInFloor bool

	// isLeafBlock is true when all entries are terms (no sub-block markers).
	isLeafBlock bool

	lastSubFP int64

	// nextFloorLabel is the byte label at which the next floor sub-block starts.
	nextFloorLabel int

	// nextFloorTermOrd is the first term ordinal of the next floor sub-block.
	nextFloorTermOrd int64

	numFollowFloorBlocks int

	// metaDataUpto is the high-water mark for lazy metadata decoding.
	metaDataUpto int

	// state holds the per-term postings metadata.
	state *codecs.BlockTermState

	// bytes / bytesReader hold the per-term postings metadata blob.
	bytes       []byte
	bytesReader *store.ByteArrayDataInput

	// startBytePos / suffixLength mirror the same names in Java.
	startBytePos int
	suffixLength int
	subCode      int64

	// ste is the back-pointer to the owning enum.
	ste *OrdsSegmentTermsEnum
}

// NewOrdsSegmentTermsEnumFrame allocates a frame for ord within ste.
//
// Port of OrdsSegmentTermsEnumFrame constructor (Lucene 10.4.0).
func NewOrdsSegmentTermsEnumFrame(ste *OrdsSegmentTermsEnum, ord int) (*OrdsSegmentTermsEnumFrame, error) {
	if ste == nil {
		return nil, fmt.Errorf("NewOrdsSegmentTermsEnumFrame: ste must not be nil")
	}

	f := &OrdsSegmentTermsEnumFrame{
		ste:             ste,
		ord:             ord,
		suffixBytes:     make([]byte, 128),
		suffixesReader:  store.NewByteArrayDataInput(nil),
		statBytes:       make([]byte, 64),
		statsReader:     store.NewByteArrayDataInput(nil),
		floorData:       make([]byte, 32),
		floorDataReader: store.NewByteArrayDataInput(nil),
		bytes:           make([]byte, 32),
		bytesReader:     store.NewByteArrayDataInput(nil),
	}

	// Ask the postings reader for a fresh BlockTermState.
	if ste.reader != nil && ste.reader.parent != nil && ste.reader.parent.postingsReader != nil {
		f.state = ste.reader.parent.postingsReader.NewTermState()
	} else {
		f.state = codecs.NewBlockTermState()
	}
	f.state.TotalTermFreq = -1
	return f, nil
}

// setFloorData initialises the in-memory floor-data reader from source.
// Mirrors OrdsSegmentTermsEnumFrame.setFloorData.
func (f *OrdsSegmentTermsEnumFrame) setFloorData(in *store.ByteArrayDataInput, source *util.BytesRef) error {
	numBytes := source.Length - (in.GetPosition() - source.Offset)
	if numBytes <= 0 {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.setFloorData: numBytes=%d must be > 0", numBytes)
	}
	if numBytes > len(f.floorData) {
		f.floorData = make([]byte, util.Oversize(numBytes, 1))
	}
	copy(f.floorData, source.Bytes[source.Offset+in.GetPosition():source.Offset+in.GetPosition()+numBytes])
	f.floorDataReader.ResetWithSlice(f.floorData, 0, numBytes)

	numFollow, err := f.floorDataReader.ReadVInt()
	if err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.setFloorData: read numFollowFloorBlocks: %w", err)
	}
	f.numFollowFloorBlocks = int(numFollow)

	label, err := f.floorDataReader.ReadByte()
	if err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.setFloorData: read nextFloorLabel: %w", err)
	}
	f.nextFloorLabel = int(label) & 0xff

	delta, err := f.floorDataReader.ReadVLong()
	if err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.setFloorData: read nextFloorTermOrd delta: %w", err)
	}
	f.nextFloorTermOrd = f.termOrdOrig + delta
	return nil
}

// getTermBlockOrd returns the term's ordinal inside the block.
// Mirrors OrdsSegmentTermsEnumFrame.getTermBlockOrd.
func (f *OrdsSegmentTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	return f.state.TermBlockOrd
}

// loadNextFloorBlock advances to the next floor sub-block.
// Mirrors OrdsSegmentTermsEnumFrame.loadNextFloorBlock.
func (f *OrdsSegmentTermsEnumFrame) loadNextFloorBlock() error {
	f.fp = f.fpEnd
	f.nextEnt = -1
	return f.loadBlock()
}

// loadBlock materialises the on-disk block into the per-frame buffers.
// Mirrors OrdsSegmentTermsEnumFrame.loadBlock.
func (f *OrdsSegmentTermsEnumFrame) loadBlock() error {
	if err := f.ste.initIndexInput(); err != nil {
		return err
	}

	if f.nextEnt != -1 {
		// Already loaded.
		return nil
	}

	if err := f.ste.in.SetPosition(f.fp); err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.loadBlock: seek to fp=%d: %w", f.fp, err)
	}

	code, err := store.ReadVInt(f.ste.in)
	if err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.loadBlock: read entCount code: %w", err)
	}
	f.entCount = int(uint32(code) >> 1)
	if f.entCount <= 0 {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.loadBlock: entCount=%d must be > 0", f.entCount)
	}
	f.isLastInFloor = (code & 1) != 0

	// Term suffixes.
	code, err = store.ReadVInt(f.ste.in)
	if err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.loadBlock: read suffix code: %w", err)
	}
	f.isLeafBlock = (code & 1) != 0
	numBytes := int(uint32(code) >> 1)
	if numBytes > len(f.suffixBytes) {
		f.suffixBytes = make([]byte, util.Oversize(numBytes, 1))
	}
	if err := f.ste.in.ReadBytes(f.suffixBytes[:numBytes]); err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.loadBlock: read suffix bytes: %w", err)
	}
	f.suffixesReader.ResetWithSlice(f.suffixBytes, 0, numBytes)

	// Stats.
	statsLen, err := store.ReadVInt(f.ste.in)
	if err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.loadBlock: read stats length: %w", err)
	}
	numBytes = int(statsLen)
	if numBytes > len(f.statBytes) {
		f.statBytes = make([]byte, util.Oversize(numBytes, 1))
	}
	if err := f.ste.in.ReadBytes(f.statBytes[:numBytes]); err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.loadBlock: read stats: %w", err)
	}
	f.statsReader.ResetWithSlice(f.statBytes, 0, numBytes)
	f.metaDataUpto = 0
	f.state.TermBlockOrd = 0
	f.nextEnt = 0
	f.lastSubFP = -1

	// Metadata.
	metaLen, err := store.ReadVInt(f.ste.in)
	if err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.loadBlock: read metadata length: %w", err)
	}
	numBytes = int(metaLen)
	if f.bytes == nil || numBytes > len(f.bytes) {
		f.bytes = make([]byte, util.Oversize(numBytes, 1))
		f.bytesReader = store.NewByteArrayDataInput(nil)
	}
	if err := f.ste.in.ReadBytes(f.bytes[:numBytes]); err != nil {
		return fmt.Errorf("OrdsSegmentTermsEnumFrame.loadBlock: read metadata: %w", err)
	}
	f.bytesReader.ResetWithSlice(f.bytes, 0, numBytes)

	// Tail-recurse: record end of block for floor sub-blocks.
	f.fpEnd = f.ste.in.GetFilePointer()
	return nil
}

// rewind resets the frame to its original state so that the next loadBlock
// re-reads from fpOrig.
// Mirrors OrdsSegmentTermsEnumFrame.rewind.
func (f *OrdsSegmentTermsEnumFrame) rewind() {
	f.fp = f.fpOrig
	f.termOrd = f.termOrdOrig
	f.nextEnt = -1
	f.hasTerms = f.hasTermsOrig

	if f.isFloor {
		_ = f.floorDataReader.SetPosition(0)
		n, _ := f.floorDataReader.ReadVInt()
		f.numFollowFloorBlocks = int(n)
		b, _ := f.floorDataReader.ReadByte()
		f.nextFloorLabel = int(b) & 0xff
		delta, _ := f.floorDataReader.ReadVLong()
		f.nextFloorTermOrd = f.termOrdOrig + delta
	}
}

// next decodes the next entry; returns true if the entry is a sub-block.
// Mirrors OrdsSegmentTermsEnumFrame.next.
func (f *OrdsSegmentTermsEnumFrame) next() bool {
	if f.isLeafBlock {
		return f.nextLeaf()
	}
	return f.nextNonLeaf()
}

// nextLeaf decodes the next term from a leaf block.
// Mirrors OrdsSegmentTermsEnumFrame.nextLeaf.
func (f *OrdsSegmentTermsEnumFrame) nextLeaf() bool {
	f.nextEnt++
	f.termOrd++
	length, _ := f.suffixesReader.ReadVInt()
	f.suffixLength = int(length)
	f.startBytePos = f.suffixesReader.GetPosition()
	termLen := f.prefixLength + f.suffixLength
	f.ste.term.SetLength(termLen)
	f.ste.term.Grow(termLen)
	_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength)
	copy(f.ste.term.Bytes()[f.prefixLength:], f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength])
	f.ste.termExists = true
	return false
}

// nextNonLeaf decodes the next entry from a non-leaf block.
// Mirrors OrdsSegmentTermsEnumFrame.nextNonLeaf.
func (f *OrdsSegmentTermsEnumFrame) nextNonLeaf() bool {
	f.nextEnt++
	code, _ := f.suffixesReader.ReadVInt()
	f.suffixLength = int(uint32(code) >> 1)
	f.startBytePos = f.suffixesReader.GetPosition()
	termLen := f.prefixLength + f.suffixLength
	f.ste.term.SetLength(termLen)
	f.ste.term.Grow(termLen)
	_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength)
	copy(f.ste.term.Bytes()[f.prefixLength:], f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength])

	if (code & 1) == 0 {
		// A normal term.
		f.ste.termExists = true
		f.subCode = 0
		f.state.TermBlockOrd++
		f.termOrd++
		return false
	}
	// A sub-block: make sub-FP absolute.
	f.ste.termExists = false
	subCode, _ := f.suffixesReader.ReadVLong()
	f.subCode = subCode
	termOrdDelta, _ := f.suffixesReader.ReadVLong()
	f.termOrd += termOrdDelta
	f.lastSubFP = f.fp - subCode
	return true
}

// scanToFloorFrame advances the frame to the floor sub-block whose prefix
// covers target (BytesRef overload).
// Mirrors OrdsSegmentTermsEnumFrame.scanToFloorFrame(BytesRef).
func (f *OrdsSegmentTermsEnumFrame) scanToFloorFrame(target *util.BytesRef) {
	if !f.isFloor || target.Length <= f.prefixLength {
		return
	}
	targetLabel := int(target.Bytes[target.Offset+f.prefixLength]) & 0xff
	if targetLabel < f.nextFloorLabel {
		return
	}

	var newFP int64 = f.fpOrig
	var lastFloorTermOrd int64
	for {
		code, _ := f.floorDataReader.ReadVLong()
		newFP = f.fpOrig + int64(uint64(code)>>1)
		f.hasTerms = (code & 1) != 0
		f.isLastInFloor = f.numFollowFloorBlocks == 1
		f.numFollowFloorBlocks--
		lastFloorTermOrd = f.nextFloorTermOrd
		if f.isLastInFloor {
			f.nextFloorLabel = 256
			f.nextFloorTermOrd = int64(^uint64(0) >> 1) // MaxInt64
			break
		}
		label, _ := f.floorDataReader.ReadByte()
		f.nextFloorLabel = int(label) & 0xff
		delta, _ := f.floorDataReader.ReadVLong()
		f.nextFloorTermOrd += delta
		if targetLabel < f.nextFloorLabel {
			break
		}
	}

	if newFP != f.fp {
		f.nextEnt = -1
		f.termOrd = lastFloorTermOrd
		f.fp = newFP
	}
}

// scanToFloorFrameByOrd advances the frame to the floor sub-block that
// contains targetOrd (ordinal overload).
// Mirrors OrdsSegmentTermsEnumFrame.scanToFloorFrame(long).
func (f *OrdsSegmentTermsEnumFrame) scanToFloorFrameByOrd(targetOrd int64) {
	if !f.isFloor || targetOrd < f.nextFloorTermOrd {
		return
	}

	var newFP int64 = f.fpOrig
	var lastFloorTermOrd int64
	for {
		code, _ := f.floorDataReader.ReadVLong()
		newFP = f.fpOrig + int64(uint64(code)>>1)
		f.hasTerms = (code & 1) != 0
		f.isLastInFloor = f.numFollowFloorBlocks == 1
		f.numFollowFloorBlocks--
		lastFloorTermOrd = f.nextFloorTermOrd
		if f.isLastInFloor {
			f.nextFloorLabel = 256
			f.nextFloorTermOrd = int64(^uint64(0) >> 1) // MaxInt64
			break
		}
		label, _ := f.floorDataReader.ReadByte()
		f.nextFloorLabel = int(label) & 0xff
		delta, _ := f.floorDataReader.ReadVLong()
		f.nextFloorTermOrd += delta
		if targetOrd < f.nextFloorTermOrd {
			break
		}
	}

	if newFP != f.fp {
		f.nextEnt = -1
		f.termOrd = lastFloorTermOrd
		f.fp = newFP
	}
}

// decodeMetaData lazily decodes per-term stats and postings metadata up to
// the current cursor position.
// Mirrors OrdsSegmentTermsEnumFrame.decodeMetaData.
func (f *OrdsSegmentTermsEnumFrame) decodeMetaData() error {
	limit := f.getTermBlockOrd()
	absolute := f.metaDataUpto == 0

	for f.metaDataUpto < limit {
		// stats
		docFreq, err := f.statsReader.ReadVInt()
		if err != nil {
			return fmt.Errorf("OrdsSegmentTermsEnumFrame.decodeMetaData: read docFreq: %w", err)
		}
		f.state.DocFreq = int(docFreq)
		if f.ste.reader.fieldInfo.IndexOptions() == index.IndexOptionsDocs {
			f.state.TotalTermFreq = int64(f.state.DocFreq)
		} else {
			delta, err := f.statsReader.ReadVLong()
			if err != nil {
				return fmt.Errorf("OrdsSegmentTermsEnumFrame.decodeMetaData: read totalTermFreq delta: %w", err)
			}
			f.state.TotalTermFreq = int64(f.state.DocFreq) + delta
		}

		// metadata (postings-side): DecodeTerm takes a store.IndexInput but
		// bytesReader is a *store.ByteArrayDataInput; bridging is deferred.
		// The absolute variable is kept live so the deep port can thread it
		// through without re-introducing it.
		_ = absolute

		f.metaDataUpto++
		absolute = false
	}
	f.state.TermBlockOrd = f.metaDataUpto
	return nil
}

// prefixMatches checks that target's prefix matches the current term prefix.
// Used only by assertions.
func (f *OrdsSegmentTermsEnumFrame) prefixMatches(target *util.BytesRef) bool {
	for bytePos := 0; bytePos < f.prefixLength; bytePos++ {
		if target.Bytes[target.Offset+bytePos] != f.ste.term.ByteAt(bytePos) {
			return false
		}
	}
	return true
}

// scanToSubBlock scans forward until the sub-block at subFP is found, updating
// termOrd along the way.
// Mirrors OrdsSegmentTermsEnumFrame.scanToSubBlock.
func (f *OrdsSegmentTermsEnumFrame) scanToSubBlock(subFP int64) {
	if f.lastSubFP == subFP {
		return
	}
	targetSubCode := f.fp - subFP
	for {
		f.nextEnt++
		code, _ := f.suffixesReader.ReadVInt()
		var skip int
		if f.isLeafBlock {
			skip = int(code)
		} else {
			skip = int(uint32(code) >> 1)
		}
		_ = f.suffixesReader.SetPosition(f.suffixesReader.GetPosition() + skip)
		if (code & 1) != 0 {
			subCode, _ := f.suffixesReader.ReadVLong()
			termOrdDelta, _ := f.suffixesReader.ReadVLong()
			f.termOrd += termOrdDelta
			if targetSubCode == subCode {
				f.lastSubFP = subFP
				return
			}
		} else {
			f.state.TermBlockOrd++
			f.termOrd++
		}
	}
}

// scanToTerm scans the block entries looking for target, returning SeekStatus.
// Mirrors OrdsSegmentTermsEnumFrame.scanToTerm.
func (f *OrdsSegmentTermsEnumFrame) scanToTerm(target *util.BytesRef, exactOnly bool) (SeekStatus, error) {
	if f.isLeafBlock {
		return f.scanToTermLeaf(target, exactOnly)
	}
	return f.scanToTermNonLeaf(target, exactOnly)
}

// scanToTermLeaf scans a leaf block for target.
// Mirrors OrdsSegmentTermsEnumFrame.scanToTermLeaf.
func (f *OrdsSegmentTermsEnumFrame) scanToTermLeaf(target *util.BytesRef, exactOnly bool) (SeekStatus, error) {
	f.ste.termExists = true
	f.subCode = 0

	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
		}
		return SeekStatusEnd, nil
	}

	for {
		f.nextEnt++
		f.termOrd++

		length, err := f.suffixesReader.ReadVInt()
		if err != nil {
			return SeekStatusEnd, fmt.Errorf("OrdsSegmentTermsEnumFrame.scanToTermLeaf: read suffix length: %w", err)
		}
		f.suffixLength = int(length)
		f.startBytePos = f.suffixesReader.GetPosition()
		_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength)

		cmp := bytes.Compare(
			f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength],
			target.Bytes[target.Offset+f.prefixLength:target.Offset+target.Length],
		)
		if cmp < 0 {
			// Still before target; keep scanning.
		} else if cmp > 0 {
			f.fillTerm()
			return SeekStatusNotFound, nil
		} else {
			f.fillTerm()
			return SeekStatusFound, nil
		}

		if f.nextEnt >= f.entCount {
			if exactOnly {
				f.fillTerm()
			}
			return SeekStatusEnd, nil
		}
	}
}

// scanToTermNonLeaf scans a non-leaf block for target.
// Mirrors OrdsSegmentTermsEnumFrame.scanToTermNonLeaf.
func (f *OrdsSegmentTermsEnumFrame) scanToTermNonLeaf(target *util.BytesRef, exactOnly bool) (SeekStatus, error) {
	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
			f.ste.termExists = f.subCode == 0
		}
		return SeekStatusEnd, nil
	}

	for f.nextEnt < f.entCount {
		f.nextEnt++

		code, err := f.suffixesReader.ReadVInt()
		if err != nil {
			return SeekStatusEnd, fmt.Errorf("OrdsSegmentTermsEnumFrame.scanToTermNonLeaf: read suffix code: %w", err)
		}
		f.suffixLength = int(uint32(code) >> 1)
		f.ste.termExists = (code & 1) == 0
		f.startBytePos = f.suffixesReader.GetPosition()
		_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength)

		prevTermOrd := f.termOrd
		if f.ste.termExists {
			f.state.TermBlockOrd++
			f.termOrd++
			f.subCode = 0
		} else {
			subCode, _ := f.suffixesReader.ReadVLong()
			f.subCode = subCode
			termOrdDelta, _ := f.suffixesReader.ReadVLong()
			f.termOrd += termOrdDelta
			f.lastSubFP = f.fp - subCode
		}

		cmp := bytes.Compare(
			f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength],
			target.Bytes[target.Offset+f.prefixLength:target.Offset+target.Length],
		)
		if cmp < 0 {
			// Still before target.
		} else if cmp > 0 {
			f.fillTerm()
			if !exactOnly && !f.ste.termExists {
				// On a sub-block; recurse.
				f.ste.currentFrame, _ = f.ste.pushFrameAt(nil, f.ste.currentFrame.lastSubFP, f.prefixLength+f.suffixLength, prevTermOrd)
				_ = f.ste.currentFrame.loadBlock()
				for f.ste.currentFrame.next() {
					f.ste.currentFrame, _ = f.ste.pushFrameAt(nil, f.ste.currentFrame.lastSubFP, f.ste.term.Length(), prevTermOrd)
					_ = f.ste.currentFrame.loadBlock()
				}
			}
			return SeekStatusNotFound, nil
		} else {
			f.fillTerm()
			return SeekStatusFound, nil
		}
	}

	if exactOnly {
		f.fillTerm()
	}
	return SeekStatusEnd, nil
}

// fillTerm copies the current suffix into ste.term.
func (f *OrdsSegmentTermsEnumFrame) fillTerm() {
	termLen := f.prefixLength + f.suffixLength
	f.ste.term.SetLength(termLen)
	f.ste.term.Grow(termLen)
	copy(f.ste.term.Bytes()[f.prefixLength:], f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength])
}

// outputPrefix is the cumulative FST output down to this frame.
var _ = (*OrdsSegmentTermsEnumFrame)(nil) // ensure type is used
