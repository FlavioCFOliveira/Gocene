// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// compressionInputAdapter wraps any [store.DataInput] to add VInt/VLong
// methods via the package-level store helpers. This allows any IndexInput
// (including ones that don't declare ReadVInt/ReadVLong natively) to be passed
// to [codecs.CompressionAlgorithm.Read].
type compressionInputAdapter struct {
	store.DataInput
}

func (a compressionInputAdapter) ReadVInt() (int32, error)  { return store.ReadVInt(a.DataInput) }
func (a compressionInputAdapter) ReadVLong() (int64, error) { return store.ReadVLong(a.DataInput) }

// asCompressionInput returns in as a [codecs.CompressionInput], wrapping it
// in an adapter when it doesn't already satisfy the interface natively
// (e.g. MMapIndexInput, SimpleFSIndexInput). ByteBuffersIndexInput and
// BufferedIndexInput-derived inputs satisfy it without wrapping.
func asCompressionInput(in store.IndexInput) codecs.CompressionInput {
	if ci, ok := in.(codecs.CompressionInput); ok {
		return ci
	}
	return compressionInputAdapter{DataInput: in}
}

// segmentTermsEnumFrame is one level of the frame stack inside
// SegmentTermsEnum. It holds all state for a single block of the .tim file:
// file pointer, entry counts, suffix/stats/metadata byte blobs, and the
// per-term BlockTermState used for lazy metadata decoding.
//
// This is the full port of
// org.apache.lucene.backward_codecs.lucene90.blocktree.SegmentTermsEnumFrame.
//
// The Lucene 9.0 frame differs from Lucene 4.0 in two ways:
//  1. It uses an OutputAccumulator (a DataInput across multiple BytesRef
//     outputs) for floor-data reading instead of a fixed BytesRef.
//  2. CompressionAlgorithm codes are identical to Lucene 10.3
//     (NO_COMPRESSION=0, LOWERCASE_ASCII=1, LZ4=2), so the codecs package
//     type can be reused directly.
type segmentTermsEnumFrame struct {
	// ord is the frame's depth in the stack; staticFrame has ord = -1.
	ord int

	hasTerms     bool
	hasTermsOrig bool
	isFloor      bool

	// arc is the FST arc that caused this frame to be pushed (nil for a "next"
	// frame produced during sequential iteration).
	arc *fst.Arc[*util.BytesRef]

	// fp is the .tim file pointer for this block. fpOrig is the floor block's
	// start pointer; fpEnd is just past the last byte of this sub-block.
	fp               int64
	fpOrig           int64
	fpEnd            int64
	totalSuffixBytes int64 // for stats only; not consumed by the enum

	// Per-block suffix data.
	suffixBytes    []byte
	suffixesReader *store.ByteArrayDataInput

	suffixLengthBytes   []byte
	suffixLengthsReader *store.ByteArrayDataInput

	// Per-block stats blob.
	statBytes               []byte
	statsSingletonRunLength int
	statsReader             *store.ByteArrayDataInput

	// rewindPos records the ByteArrayDataInput position of the floor data
	// header so that rewind() can replay it.
	rewindPos int

	// floorDataReader is backed by the floor data portion of the FST output.
	// It is reset at construction and re-used across rewind calls.
	floorDataReader *store.ByteArrayDataInput

	// prefixLength is the number of bytes shared by all terms in this block.
	prefixLength int

	// entCount is the number of entries (term or sub-block) in this block.
	entCount int

	// nextEnt is the next entry index to decode, or -1 if not yet loaded.
	nextEnt int

	isLastInFloor bool
	isLeafBlock   bool
	allEqual      bool

	// lastSubFP is the file pointer of the last sub-block pointer advanced
	// through; used by SegmentTermsEnum.scanToSubBlock.
	lastSubFP int64

	nextFloorLabel       int
	numFollowFloorBlocks int

	// metaDataUpto is the number of terms in this block whose stats + postings
	// metadata have been decoded so far.
	metaDataUpto int

	// state holds the decoded per-term postings state for lazy decode.
	state *codecs.BlockTermState

	// bytes / bytesReader hold the encoded postings metadata blob.
	bytes       []byte
	bytesReader *store.ByteArrayDataInput

	// compressionAlg is the algorithm used to compress the suffix bytes.
	compressionAlg compressionAlg

	// startBytePos / suffixLength / subCode are scratch fields set as a
	// side-effect of next() / scanToTerm and consumed by fillTerm().
	startBytePos int
	suffixLength int
	subCode      int64

	// ste is the owning SegmentTermsEnum.
	ste *SegmentTermsEnum
}

// newSegmentTermsEnumFrame allocates a fully-initialised frame for the given
// SegmentTermsEnum at stack ordinal ord. Mirrors the Java constructor
// SegmentTermsEnumFrame(SegmentTermsEnum, int).
func newSegmentTermsEnumFrame(ste *SegmentTermsEnum, ord int) *segmentTermsEnumFrame {
	f := &segmentTermsEnumFrame{
		ste:                 ste,
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
		lastSubFP:           -1,
		compressionAlg:      noCompression,
	}
	if ste.fr != nil && ste.fr.parent != nil && ste.fr.parent.postingsReader != nil {
		f.state = ste.fr.parent.postingsReader.NewTermState()
	} else {
		f.state = codecs.NewBlockTermState()
	}
	f.state.TotalTermFreq = -1
	return f
}

// setFloorData wires the frame's floor data reader to the current position
// of the OutputAccumulator, which already points at the floor-data bytes of
// the FST arc that produced this block.
//
// Port of SegmentTermsEnumFrame.setFloorData(OutputAccumulator).
func (f *segmentTermsEnumFrame) setFloorData(acc *outputAccumulator) {
	acc.setFloorData(f.floorDataReader)
	f.rewindPos = f.floorDataReader.GetPosition()
	numFollow, _ := f.floorDataReader.ReadVInt()
	f.numFollowFloorBlocks = int(numFollow)
	b, _ := f.floorDataReader.ReadByte()
	f.nextFloorLabel = int(b) & 0xff
}

// getTermBlockOrd returns the term ordinal inside the current block.
// Port of SegmentTermsEnumFrame.getTermBlockOrd().
func (f *segmentTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	return f.state.TermBlockOrd
}

// loadNextFloorBlock advances fp to fpEnd and reloads the block (the next
// floor sub-block written inline in the .tim file).
// Port of SegmentTermsEnumFrame.loadNextFloorBlock().
func (f *segmentTermsEnumFrame) loadNextFloorBlock() error {
	f.fp = f.fpEnd
	f.nextEnt = -1
	return f.loadBlock()
}

// loadBlock reads and decodes the block at fp: it stores the entry count, the
// (possibly compressed) suffix corpus, the suffix-length blob, the stats blob
// and the postings metadata blob into reusable byte slices. Per-term metadata
// is decoded lazily later in decodeMetaData.
//
// Port of SegmentTermsEnumFrame.loadBlock().
func (f *segmentTermsEnumFrame) loadBlock() error {
	if err := f.ste.initIndexInput(); err != nil {
		return err
	}
	if f.nextEnt != -1 {
		// Already loaded.
		return nil
	}
	in := f.ste.in

	if err := in.SetPosition(f.fp); err != nil {
		return fmt.Errorf("loadBlock: seek to fp=%d: %w", f.fp, err)
	}
	code, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("loadBlock: read block code: %w", err)
	}
	f.entCount = int(uint32(code) >> 1)
	if f.entCount <= 0 {
		return fmt.Errorf("loadBlock: entCount=%d must be > 0 at fp=%d", f.entCount, f.fp)
	}
	f.isLastInFloor = (code & 1) != 0

	startSuffixFP := in.GetFilePointer()

	// Suffix encoding: VLong = (numSuffixBytes << 3) | (isLeaf << 2) | comprAlgCode.
	codeL, err := store.ReadVLong(in)
	if err != nil {
		return fmt.Errorf("loadBlock: read suffix code: %w", err)
	}
	f.isLeafBlock = (codeL & 0x04) != 0
	numSuffixBytes := int(uint64(codeL) >> 3)
	if len(f.suffixBytes) < numSuffixBytes {
		f.suffixBytes = make([]byte, util.Oversize(numSuffixBytes, 1))
	}
	alg, err := compressionAlgByCode(int(codeL & 0x03))
	if err != nil {
		return fmt.Errorf("loadBlock: %w", err)
	}
	f.compressionAlg = alg
	if err := f.compressionAlg.Read(asCompressionInput(in), f.suffixBytes, numSuffixBytes); err != nil {
		return fmt.Errorf("loadBlock: decompress suffixes: %w", err)
	}
	f.suffixesReader.ResetWithSlice(f.suffixBytes, 0, numSuffixBytes)

	// Suffix-length encoding: VInt = (numSuffixLengthBytes << 1) | allEqual.
	suffixLenCode, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("loadBlock: read suffix length code: %w", err)
	}
	f.allEqual = (suffixLenCode & 0x01) != 0
	numSuffixLengthBytes := int(uint32(suffixLenCode) >> 1)
	if len(f.suffixLengthBytes) < numSuffixLengthBytes {
		f.suffixLengthBytes = make([]byte, util.Oversize(numSuffixLengthBytes, 1))
	}
	if f.allEqual {
		b, err := in.ReadByte()
		if err != nil {
			return fmt.Errorf("loadBlock: read allEqual suffix length: %w", err)
		}
		for i := range f.suffixLengthBytes[:numSuffixLengthBytes] {
			f.suffixLengthBytes[i] = b
		}
	} else {
		if err := in.ReadBytes(f.suffixLengthBytes[:numSuffixLengthBytes]); err != nil {
			return fmt.Errorf("loadBlock: read suffix lengths: %w", err)
		}
	}
	f.suffixLengthsReader.ResetWithSlice(f.suffixLengthBytes, 0, numSuffixLengthBytes)

	f.totalSuffixBytes = in.GetFilePointer() - startSuffixFP

	// Stats blob.
	numStatBytesV, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("loadBlock: read numStatBytes: %w", err)
	}
	nb := int(numStatBytesV)
	if len(f.statBytes) < nb {
		f.statBytes = make([]byte, util.Oversize(nb, 1))
	}
	if err := in.ReadBytes(f.statBytes[:nb]); err != nil {
		return fmt.Errorf("loadBlock: read statBytes: %w", err)
	}
	f.statsReader.ResetWithSlice(f.statBytes, 0, nb)
	f.statsSingletonRunLength = 0
	f.metaDataUpto = 0

	f.state.TermBlockOrd = 0
	f.nextEnt = 0
	f.lastSubFP = -1

	// Postings metadata blob.
	numMetaBytesV, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("loadBlock: read numMetaBytes: %w", err)
	}
	nm := int(numMetaBytesV)
	if len(f.bytes) < nm {
		f.bytes = make([]byte, util.Oversize(nm, 1))
	}
	if err := in.ReadBytes(f.bytes[:nm]); err != nil {
		return fmt.Errorf("loadBlock: read metaBytes: %w", err)
	}
	f.bytesReader.ResetWithSlice(f.bytes, 0, nm)

	f.fpEnd = in.GetFilePointer()
	return nil
}

// rewind resets the frame to re-scan the block from the beginning.
// Port of SegmentTermsEnumFrame.rewind().
func (f *segmentTermsEnumFrame) rewind() {
	f.fp = f.fpOrig
	f.nextEnt = -1
	f.hasTerms = f.hasTermsOrig
	if f.isFloor {
		// rewindPos is always a valid position — it was captured from
		// GetPosition() after SetFloorData. Ignore the error (out-of-range
		// would be a programming error, not a runtime condition).
		_ = f.floorDataReader.SetPosition(f.rewindPos)
		numFollow, _ := f.floorDataReader.ReadVInt()
		f.numFollowFloorBlocks = int(numFollow)
		b, _ := f.floorDataReader.ReadByte()
		f.nextFloorLabel = int(b) & 0xff
	}
}

// next decodes the next entry (term or sub-block) in the current block.
// Returns (isSub=true) when the entry is a sub-block that the caller must
// descend into. Port of SegmentTermsEnumFrame.next().
func (f *segmentTermsEnumFrame) next() (bool, error) {
	if f.isLeafBlock {
		if err := f.nextLeaf(); err != nil {
			return false, err
		}
		return false, nil
	}
	return f.nextNonLeaf()
}

// nextLeaf advances the cursor through a leaf block (all entries are terms).
// Port of SegmentTermsEnumFrame.nextLeaf().
func (f *segmentTermsEnumFrame) nextLeaf() error {
	f.nextEnt++
	suffLen, err := f.suffixLengthsReader.ReadVInt()
	if err != nil {
		return fmt.Errorf("nextLeaf: read suffixLength: %w", err)
	}
	f.suffixLength = int(suffLen)
	f.startBytePos = f.suffixesReader.GetPosition()
	newLen := f.prefixLength + f.suffixLength
	f.ste.growTerm(newLen)
	if err := f.suffixesReader.ReadBytes(f.ste.term.Bytes()[f.prefixLength:newLen]); err != nil {
		return fmt.Errorf("nextLeaf: read suffix bytes: %w", err)
	}
	f.ste.termExists = true
	return nil
}

// nextNonLeaf advances the cursor through a non-leaf block (may contain
// sub-block entries). Returns (isSub=true) when a sub-block is encountered.
// Port of SegmentTermsEnumFrame.nextNonLeaf().
func (f *segmentTermsEnumFrame) nextNonLeaf() (bool, error) {
	for {
		if f.nextEnt == f.entCount {
			if err := f.loadNextFloorBlock(); err != nil {
				return false, err
			}
			if f.isLeafBlock {
				if err := f.nextLeaf(); err != nil {
					return false, err
				}
				return false, nil
			}
			continue
		}
		f.nextEnt++
		code, err := f.suffixLengthsReader.ReadVInt()
		if err != nil {
			return false, fmt.Errorf("nextNonLeaf: read code: %w", err)
		}
		f.suffixLength = int(uint32(code) >> 1)
		f.startBytePos = f.suffixesReader.GetPosition()
		newLen := f.prefixLength + f.suffixLength
		f.ste.growTerm(newLen)
		if err := f.suffixesReader.ReadBytes(f.ste.term.Bytes()[f.prefixLength:newLen]); err != nil {
			return false, fmt.Errorf("nextNonLeaf: read suffix bytes: %w", err)
		}
		if code&1 == 0 {
			// Normal term.
			f.ste.termExists = true
			f.subCode = 0
			f.state.TermBlockOrd++
			return false, nil
		}
		// Sub-block.
		f.ste.termExists = false
		subCodeV, err := f.suffixLengthsReader.ReadVLong()
		if err != nil {
			return false, fmt.Errorf("nextNonLeaf: read subCode: %w", err)
		}
		f.subCode = subCodeV
		f.lastSubFP = f.fp - f.subCode
		return true, nil
	}
}

// scanToFloorFrame advances the frame to the floor sub-block whose label
// range covers target, if this is a floor block. No-op for non-floor blocks.
// Port of SegmentTermsEnumFrame.scanToFloorFrame(BytesRef).
func (f *segmentTermsEnumFrame) scanToFloorFrame(target *util.BytesRef) {
	if !f.isFloor || target.Length <= f.prefixLength {
		return
	}
	targetLabel := int(target.Bytes[target.Offset+f.prefixLength]) & 0xff
	if targetLabel < f.nextFloorLabel {
		return
	}
	var newFP int64 = f.fpOrig
	for {
		code, err := f.floorDataReader.ReadVLong()
		if err != nil {
			break
		}
		newFP = f.fpOrig + (code >> 1)
		f.hasTerms = (code & 1) != 0
		f.isLastInFloor = (f.numFollowFloorBlocks == 1)
		f.numFollowFloorBlocks--
		if f.isLastInFloor {
			f.nextFloorLabel = 256
			break
		}
		b, err := f.floorDataReader.ReadByte()
		if err != nil {
			break
		}
		f.nextFloorLabel = int(b) & 0xff
		if targetLabel < f.nextFloorLabel {
			break
		}
	}
	if newFP != f.fp {
		f.nextEnt = -1
		f.fp = newFP
	}
}

// decodeMetaData lazily decodes stats and postings metadata for all terms up
// to the current term block ordinal.
// Port of SegmentTermsEnumFrame.decodeMetaData().
func (f *segmentTermsEnumFrame) decodeMetaData() error {
	limit := f.getTermBlockOrd()
	if limit <= 0 {
		return fmt.Errorf("decodeMetaData: limit=%d must be > 0", limit)
	}
	absolute := f.metaDataUpto == 0
	for f.metaDataUpto < limit {
		if f.statsSingletonRunLength > 0 {
			f.state.DocFreq = 1
			f.state.TotalTermFreq = 1
			f.statsSingletonRunLength--
		} else {
			token, err := f.statsReader.ReadVInt()
			if err != nil {
				return fmt.Errorf("decodeMetaData: read stats token: %w", err)
			}
			if token&1 == 1 {
				f.state.DocFreq = 1
				f.state.TotalTermFreq = 1
				f.statsSingletonRunLength = int(uint32(token) >> 1)
			} else {
				f.state.DocFreq = int(uint32(token) >> 1)
				if f.ste.fr.fieldInfo != nil &&
					f.ste.fr.fieldInfo.IndexOptions() == index.IndexOptionsDocs {
					f.state.TotalTermFreq = int64(f.state.DocFreq)
				} else {
					extra, err := f.statsReader.ReadVLong()
					if err != nil {
						return fmt.Errorf("decodeMetaData: read totalTermFreq delta: %w", err)
					}
					f.state.TotalTermFreq = int64(f.state.DocFreq) + extra
				}
			}
		}
		if f.ste.fr.parent != nil && f.ste.fr.parent.postingsReader != nil && f.ste.fr.fieldInfo != nil {
			if err := f.ste.fr.parent.postingsReader.DecodeTerm(
				f.bytesReader, f.ste.fr.fieldInfo, f.state, absolute,
			); err != nil {
				return fmt.Errorf("decodeMetaData: DecodeTerm: %w", err)
			}
		}
		f.metaDataUpto++
		absolute = false
	}
	f.state.TermBlockOrd = f.metaDataUpto
	return nil
}

// scanToSubBlock advances the cursor in a non-leaf block to the sub-block
// with the given file pointer. Called by SegmentTermsEnum.next() when
// returning to a parent frame after exhausting a child block.
// Port of SegmentTermsEnumFrame.scanToSubBlock(long).
func (f *segmentTermsEnumFrame) scanToSubBlock(subFP int64) error {
	if f.lastSubFP == subFP {
		return nil
	}
	targetSubCode := f.fp - subFP
	for {
		f.nextEnt++
		code, err := f.suffixLengthsReader.ReadVInt()
		if err != nil {
			return fmt.Errorf("scanToSubBlock: read code: %w", err)
		}
		suffLen := int(uint32(code) >> 1)
		curPos := f.suffixesReader.GetPosition()
		if err := f.suffixesReader.SetPosition(curPos + suffLen); err != nil {
			return fmt.Errorf("scanToSubBlock: skip suffix: %w", err)
		}
		if code&1 != 0 {
			subCode, err := f.suffixLengthsReader.ReadVLong()
			if err != nil {
				return fmt.Errorf("scanToSubBlock: read subCode: %w", err)
			}
			if targetSubCode == subCode {
				f.lastSubFP = subFP
				return nil
			}
		} else {
			f.state.TermBlockOrd++
		}
	}
}

// scanToTerm dispatches to the correct scan strategy based on block type.
// Port of SegmentTermsEnumFrame.scanToTerm(BytesRef, boolean).
func (f *segmentTermsEnumFrame) scanToTerm(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.isLeafBlock {
		if f.allEqual {
			return f.binarySearchTermLeaf(target, exactOnly)
		}
		return f.scanToTermLeaf(target, exactOnly)
	}
	return f.scanToTermNonLeaf(target, exactOnly)
}

// scanToTermLeaf performs a linear scan through a leaf block.
// Port of SegmentTermsEnumFrame.scanToTermLeaf(BytesRef, boolean).
func (f *segmentTermsEnumFrame) scanToTermLeaf(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
		}
		return index.SeekStatusEnd, nil
	}
	f.ste.termExists = true
	f.subCode = 0
	for f.nextEnt < f.entCount {
		f.nextEnt++
		suffLen, err := f.suffixLengthsReader.ReadVInt()
		if err != nil {
			return 0, fmt.Errorf("scanToTermLeaf: read suffixLength: %w", err)
		}
		f.suffixLength = int(suffLen)
		f.startBytePos = f.suffixesReader.GetPosition()
		if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
			return 0, fmt.Errorf("scanToTermLeaf: skip suffix: %w", err)
		}
		cmp := bytes.Compare(
			f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength],
			target.Bytes[target.Offset+f.prefixLength:target.Offset+target.Length],
		)
		if cmp < 0 {
			continue
		}
		if cmp > 0 {
			f.fillTerm()
			return index.SeekStatusNotFound, nil
		}
		f.fillTerm()
		return index.SeekStatusFound, nil
	}
	if exactOnly {
		f.fillTerm()
	}
	return index.SeekStatusEnd, nil
}

// binarySearchTermLeaf performs binary search in a leaf block with uniform
// suffix lengths (allEqual == true).
// Port of SegmentTermsEnumFrame.binarySearchTermLeaf(BytesRef, boolean).
func (f *segmentTermsEnumFrame) binarySearchTermLeaf(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
		}
		return index.SeekStatusEnd, nil
	}
	f.ste.termExists = true
	f.subCode = 0

	suffLen, err := f.suffixLengthsReader.ReadVInt()
	if err != nil {
		return 0, fmt.Errorf("binarySearchTermLeaf: read suffixLength: %w", err)
	}
	f.suffixLength = int(suffLen)

	start := f.nextEnt
	end := f.entCount - 1
	cmp := 0
	for start <= end {
		mid := (start + end) >> 1
		f.nextEnt = mid + 1
		f.startBytePos = mid * f.suffixLength
		cmp = bytes.Compare(
			f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength],
			target.Bytes[target.Offset+f.prefixLength:target.Offset+target.Length],
		)
		if cmp < 0 {
			start = mid + 1
		} else if cmp > 0 {
			end = mid - 1
		} else {
			if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
				return 0, fmt.Errorf("binarySearchTermLeaf: set position: %w", err)
			}
			f.fillTerm()
			return index.SeekStatusFound, nil
		}
	}
	// Binary search ended without exact match.
	if end < f.entCount-1 {
		if cmp < 0 {
			f.startBytePos += f.suffixLength
			f.nextEnt++
		}
		if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
			return 0, fmt.Errorf("binarySearchTermLeaf: set position after not-found: %w", err)
		}
		f.fillTerm()
		return index.SeekStatusNotFound, nil
	}
	if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
		return 0, fmt.Errorf("binarySearchTermLeaf: set position at end: %w", err)
	}
	if exactOnly {
		f.fillTerm()
	}
	return index.SeekStatusEnd, nil
}

// scanToTermNonLeaf performs a linear scan through a non-leaf block.
// Port of SegmentTermsEnumFrame.scanToTermNonLeaf(BytesRef, boolean).
func (f *segmentTermsEnumFrame) scanToTermNonLeaf(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
			f.ste.termExists = f.subCode == 0
		}
		return index.SeekStatusEnd, nil
	}
	for f.nextEnt < f.entCount {
		f.nextEnt++
		code, err := f.suffixLengthsReader.ReadVInt()
		if err != nil {
			return 0, fmt.Errorf("scanToTermNonLeaf: read code: %w", err)
		}
		f.suffixLength = int(uint32(code) >> 1)
		f.startBytePos = f.suffixesReader.GetPosition()
		if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
			return 0, fmt.Errorf("scanToTermNonLeaf: skip suffix: %w", err)
		}
		f.ste.termExists = (code & 1) == 0
		if f.ste.termExists {
			f.state.TermBlockOrd++
			f.subCode = 0
		} else {
			subCode, err := f.suffixLengthsReader.ReadVLong()
			if err != nil {
				return 0, fmt.Errorf("scanToTermNonLeaf: read subCode: %w", err)
			}
			f.subCode = subCode
			f.lastSubFP = f.fp - f.subCode
		}
		cmp := bytes.Compare(
			f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength],
			target.Bytes[target.Offset+f.prefixLength:target.Offset+target.Length],
		)
		if cmp < 0 {
			continue
		}
		if cmp > 0 {
			f.fillTerm()
			if !exactOnly && !f.ste.termExists {
				// On a sub-block: descend to find the next term.
				cf, err := f.ste.pushFrameFromSubFP(nil, f.ste.currentFrame.lastSubFP, f.prefixLength+f.suffixLength)
				if err != nil {
					return 0, err
				}
				f.ste.currentFrame = cf
				if err := cf.loadBlock(); err != nil {
					return 0, err
				}
				for {
					isSub, err := f.ste.currentFrame.next()
					if err != nil {
						return 0, err
					}
					if !isSub {
						break
					}
					innerCF, err := f.ste.pushFrameFromSubFP(nil, f.ste.currentFrame.lastSubFP, f.ste.term.Length())
					if err != nil {
						return 0, err
					}
					f.ste.currentFrame = innerCF
					if err := innerCF.loadBlock(); err != nil {
						return 0, err
					}
				}
			}
			return index.SeekStatusNotFound, nil
		}
		// Exact match.
		f.fillTerm()
		return index.SeekStatusFound, nil
	}
	if exactOnly {
		f.fillTerm()
	}
	return index.SeekStatusEnd, nil
}

// fillTerm copies the suffix from the internal buffer into the owning enum's
// term builder at the correct position.
// Port of SegmentTermsEnumFrame.fillTerm().
func (f *segmentTermsEnumFrame) fillTerm() {
	termLen := f.prefixLength + f.suffixLength
	f.ste.growTerm(termLen)
	copy(f.ste.term.Bytes()[f.prefixLength:termLen], f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength])
}
