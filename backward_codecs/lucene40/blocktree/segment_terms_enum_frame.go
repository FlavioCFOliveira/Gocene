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

// segmentTermsEnumFrame holds the per-level state for one level of the block
// tree during SegmentTermsEnum traversal.
//
// Port of the package-private class
// org.apache.lucene.backward_codecs.lucene40.blocktree.SegmentTermsEnumFrame
// (Lucene 10.4.0).
type segmentTermsEnumFrame struct {
	// ord is the index of this frame in the SegmentTermsEnum.stack slice.
	ord int

	hasTerms     bool
	hasTermsOrig bool
	isFloor      bool

	arc *fst.Arc[*util.BytesRef]

	// fp is the file pointer of the start of this block.
	fp     int64
	fpOrig int64
	fpEnd  int64
	// totalSuffixBytes is used for Stats.
	totalSuffixBytes int64

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

	// nextEnt is the index of the next entry to read, or -1 if not loaded.
	nextEnt int

	isLastInFloor bool
	isLeafBlock   bool

	lastSubFP int64

	nextFloorLabel       int
	numFollowFloorBlocks int

	metaDataUpto int

	// compressionAlg is the algorithm used for suffix compression in this block.
	compressionAlg CompressionAlgorithm

	// state holds decoded per-term metadata (docFreq, totalTermFreq, postings fp).
	// It is initialised from postingsReader.NewTermState() in the constructor.
	state *codecs.BlockTermState

	// bytes holds encoded per-term metadata (lazy-decoded).
	bytes       []byte
	bytesReader *store.ByteArrayDataInput

	// ste is the owning SegmentTermsEnum.
	ste *SegmentTermsEnum

	// working variables used during entry scanning
	startBytePos int
	suffix       int
	subCode      int64

	version int32
}

// newSegmentTermsEnumFrame constructs a frame for the given SegmentTermsEnum at
// stack ordinal ord.
func newSegmentTermsEnumFrame(ste *SegmentTermsEnum, ord int) *segmentTermsEnumFrame {
	var version int32
	if ste.fr != nil && ste.fr.parent != nil {
		version = ste.fr.parent.version
	}

	var suffixLengthsReader *store.ByteArrayDataInput
	var suffixLengthBytes []byte
	if version >= versionCompressedSuffixes {
		suffixLengthBytes = make([]byte, 32)
		suffixLengthsReader = store.NewByteArrayDataInput(nil)
	}

	var state *codecs.BlockTermState
	if ste.fr != nil && ste.fr.parent != nil && ste.fr.parent.postingsReader != nil {
		state = ste.fr.parent.postingsReader.NewTermState()
		if state != nil {
			state.TotalTermFreq = -1
		}
	}

	f := &segmentTermsEnumFrame{
		ste:                 ste,
		ord:                 ord,
		suffixBytes:         make([]byte, 128),
		suffixesReader:      store.NewByteArrayDataInput(nil),
		suffixLengthBytes:   suffixLengthBytes,
		suffixLengthsReader: suffixLengthsReader,
		statBytes:           make([]byte, 64),
		statsReader:         store.NewByteArrayDataInput(nil),
		floorData:           make([]byte, 32),
		floorDataReader:     store.NewByteArrayDataInput(nil),
		bytes:               make([]byte, 32),
		bytesReader:         store.NewByteArrayDataInput(nil),
		nextEnt:             -1,
		state:               state,
		version:             version,
	}
	// If suffix lengths share the same reader as suffixes (old format),
	// point suffixLengthsReader at suffixesReader.
	if version < versionCompressedSuffixes {
		f.suffixLengthsReader = f.suffixesReader
	}
	return f
}

// getTermBlockOrd returns the number of terms decoded so far in this block.
func (f *segmentTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	if f.state == nil {
		return 0
	}
	return f.state.TermBlockOrd
}

// setFloorData copies the remaining floor-entry data from in/source into the
// frame and initialises the floor reader.
//
// Port of SegmentTermsEnumFrame.setFloorData.
func (f *segmentTermsEnumFrame) setFloorData(in *store.ByteArrayDataInput, source *util.BytesRef) {
	numBytes := source.Length - (in.GetPosition() - source.Offset)
	if numBytes > len(f.floorData) {
		if numBytes > len(f.floorData) {
			f.floorData = util.GrowExactByte(f.floorData, util.Oversize(numBytes, 1))
		}
	}
	copy(f.floorData, source.Bytes[source.Offset+in.GetPosition():source.Offset+in.GetPosition()+numBytes])
	f.floorDataReader.ResetWithSlice(f.floorData, 0, numBytes)
	v, _ := store.ReadVInt(f.floorDataReader)
	f.numFollowFloorBlocks = int(v)
	b, _ := f.floorDataReader.ReadByte()
	f.nextFloorLabel = int(b) & 0xFF
}

// loadNextFloorBlock advances to the next sub-block of a floor block.
//
// Port of SegmentTermsEnumFrame.loadNextFloorBlock.
func (f *segmentTermsEnumFrame) loadNextFloorBlock() error {
	f.fp = f.fpEnd
	f.nextEnt = -1
	return f.loadBlock()
}

// loadBlock loads the next block of terms from the terms file.
//
// Port of SegmentTermsEnumFrame.loadBlock.
func (f *segmentTermsEnumFrame) loadBlock() error {
	// Lazily clone the terms file input.
	f.ste.initIndexInput()

	if f.nextEnt != -1 {
		// Already loaded.
		return nil
	}

	if err := f.ste.in.SetPosition(f.fp); err != nil {
		return fmt.Errorf("blocktree loadBlock seek fp=%d: %w", f.fp, err)
	}

	code, err := store.ReadVInt(f.ste.in)
	if err != nil {
		return fmt.Errorf("blocktree loadBlock read entCount: %w", err)
	}
	f.entCount = int(code >> 1)
	f.isLastInFloor = (code & 1) != 0

	startSuffixFP := f.ste.in.GetFilePointer()

	// Read suffix bytes.
	if f.version >= versionCompressedSuffixes {
		codeL, err2 := store.ReadVLong(f.ste.in)
		if err2 != nil {
			return fmt.Errorf("blocktree loadBlock read suffix codeL: %w", err2)
		}
		f.isLeafBlock = (codeL & 0x04) != 0
		numSuffixBytes := int(codeL >> 3)
		if numSuffixBytes > len(f.suffixBytes) {
			f.suffixBytes = util.GrowExactByte(f.suffixBytes, util.Oversize(numSuffixBytes, 1))
		}
		algCode := int(codeL & 0x03)
		alg, err3 := CompressionAlgorithmByCode(algCode)
		if err3 != nil {
			return fmt.Errorf("blocktree loadBlock: %w", err3)
		}
		f.compressionAlg = alg
		if err4 := f.compressionAlg.Decompress(indexInputCompressAdapter{f.ste.in}, f.suffixBytes, numSuffixBytes); err4 != nil {
			return fmt.Errorf("blocktree loadBlock read suffix bytes: %w", err4)
		}
		f.suffixesReader.ResetWithSlice(f.suffixBytes, 0, numSuffixBytes)

		numSuffixLengthsRaw, err5 := store.ReadVInt(f.ste.in)
		if err5 != nil {
			return fmt.Errorf("blocktree loadBlock read suffix length header: %w", err5)
		}
		allEqual := (numSuffixLengthsRaw & 0x01) != 0
		numSuffixLengthBytes := int(numSuffixLengthsRaw >> 1)
		if numSuffixLengthBytes > len(f.suffixLengthBytes) {
			f.suffixLengthBytes = util.GrowExactByte(f.suffixLengthBytes, util.Oversize(numSuffixLengthBytes, 1))
		}
		if allEqual {
			b, err6 := f.ste.in.ReadByte()
			if err6 != nil {
				return fmt.Errorf("blocktree loadBlock read allEqual suffix byte: %w", err6)
			}
			for i := 0; i < numSuffixLengthBytes; i++ {
				f.suffixLengthBytes[i] = b
			}
		} else {
			if err6 := f.ste.in.ReadBytes(f.suffixLengthBytes[:numSuffixLengthBytes]); err6 != nil {
				return fmt.Errorf("blocktree loadBlock read suffix lengths: %w", err6)
			}
		}
		f.suffixLengthsReader.ResetWithSlice(f.suffixLengthBytes, 0, numSuffixLengthBytes)
	} else {
		code2, err2 := store.ReadVInt(f.ste.in)
		if err2 != nil {
			return fmt.Errorf("blocktree loadBlock read old suffix code: %w", err2)
		}
		f.isLeafBlock = (code2 & 1) != 0
		numBytes := int(code2 >> 1)
		if numBytes > len(f.suffixBytes) {
			f.suffixBytes = util.GrowExactByte(f.suffixBytes, util.Oversize(numBytes, 1))
		}
		if err3 := f.ste.in.ReadBytes(f.suffixBytes[:numBytes]); err3 != nil {
			return fmt.Errorf("blocktree loadBlock read suffix bytes (old): %w", err3)
		}
		f.suffixesReader.ResetWithSlice(f.suffixBytes, 0, numBytes)
		// Old format: suffix lengths are interleaved in suffixesReader.
		f.suffixLengthsReader = f.suffixesReader
	}
	f.totalSuffixBytes = f.ste.in.GetFilePointer() - startSuffixFP

	// Read stats bytes.
	statLen, err := store.ReadVInt(f.ste.in)
	if err != nil {
		return fmt.Errorf("blocktree loadBlock read stats len: %w", err)
	}
	numStatBytes := int(statLen)
	if numStatBytes > len(f.statBytes) {
		f.statBytes = util.GrowExactByte(f.statBytes, util.Oversize(numStatBytes, 1))
	}
	if err2 := f.ste.in.ReadBytes(f.statBytes[:numStatBytes]); err2 != nil {
		return fmt.Errorf("blocktree loadBlock read stats: %w", err2)
	}
	f.statsReader.ResetWithSlice(f.statBytes, 0, numStatBytes)
	f.statsSingletonRunLength = 0
	f.metaDataUpto = 0

	if f.state != nil {
		f.state.TermBlockOrd = 0
	}
	f.nextEnt = 0
	f.lastSubFP = -1

	// Read meta bytes.
	metaLen, err := store.ReadVInt(f.ste.in)
	if err != nil {
		return fmt.Errorf("blocktree loadBlock read meta len: %w", err)
	}
	numMetaBytes := int(metaLen)
	if numMetaBytes > len(f.bytes) {
		f.bytes = util.GrowExactByte(f.bytes, util.Oversize(numMetaBytes, 1))
	}
	if err2 := f.ste.in.ReadBytes(f.bytes[:numMetaBytes]); err2 != nil {
		return fmt.Errorf("blocktree loadBlock read meta: %w", err2)
	}
	f.bytesReader.ResetWithSlice(f.bytes, 0, numMetaBytes)

	f.fpEnd = f.ste.in.GetFilePointer()
	return nil
}

// rewind resets the frame to the beginning of its original block.
//
// Port of SegmentTermsEnumFrame.rewind.
func (f *segmentTermsEnumFrame) rewind() {
	f.fp = f.fpOrig
	f.nextEnt = -1
	f.hasTerms = f.hasTermsOrig
	if f.isFloor {
		f.floorDataReader.ResetWithSlice(f.floorData, 0, len(f.floorData))
		v, _ := store.ReadVInt(f.floorDataReader)
		f.numFollowFloorBlocks = int(v)
		b, _ := f.floorDataReader.ReadByte()
		f.nextFloorLabel = int(b) & 0xFF
	}
}

// next advances to the next entry in this block.
// Returns true if the entry is a sub-block, false if it's a term.
//
// Port of SegmentTermsEnumFrame.next.
func (f *segmentTermsEnumFrame) next() (bool, error) {
	if f.isLeafBlock {
		f.nextLeaf()
		return false, nil
	}
	return f.nextNonLeaf()
}

// nextLeaf advances within a leaf block (all entries are terms).
//
// Port of SegmentTermsEnumFrame.nextLeaf.
func (f *segmentTermsEnumFrame) nextLeaf() {
	f.nextEnt++
	v, _ := store.ReadVInt(f.suffixLengthsReader)
	f.suffix = int(v)
	f.startBytePos = f.suffixesReader.GetPosition()
	f.ste.term.SetLength(f.prefix + f.suffix)
	f.ste.term.Grow(f.ste.term.Length())
	_ = f.suffixesReader.ReadBytes(f.ste.term.Bytes()[f.prefix : f.prefix+f.suffix])
	f.ste.termExists = true
}

// nextNonLeaf advances within a non-leaf block (entries are terms or sub-blocks).
//
// Port of SegmentTermsEnumFrame.nextNonLeaf.
func (f *segmentTermsEnumFrame) nextNonLeaf() (bool, error) {
	for {
		if f.nextEnt == f.entCount {
			if err := f.loadNextFloorBlock(); err != nil {
				return false, err
			}
			if f.isLeafBlock {
				f.nextLeaf()
				return false, nil
			}
			continue
		}
		f.nextEnt++
		code, _ := store.ReadVInt(f.suffixLengthsReader)
		f.suffix = int(code >> 1)
		f.startBytePos = f.suffixesReader.GetPosition()
		f.ste.term.SetLength(f.prefix + f.suffix)
		f.ste.term.Grow(f.ste.term.Length())
		_ = f.suffixesReader.ReadBytes(f.ste.term.Bytes()[f.prefix : f.prefix+f.suffix])
		if (code & 1) == 0 {
			// A normal term.
			f.ste.termExists = true
			f.subCode = 0
			if f.state != nil {
				f.state.TermBlockOrd++
			}
			return false, nil
		}
		// A sub-block.
		f.ste.termExists = false
		v, _ := store.ReadVLong(f.suffixLengthsReader)
		f.subCode = int64(v)
		f.lastSubFP = f.fp - f.subCode
		return true, nil
	}
}

// scanToFloorFrame advances the floor pointer to the sub-block that may
// contain target.
//
// Port of SegmentTermsEnumFrame.scanToFloorFrame.
func (f *segmentTermsEnumFrame) scanToFloorFrame(target *util.BytesRef) {
	if !f.isFloor || target.Length <= f.prefix {
		return
	}
	targetLabel := int(target.Bytes[target.Offset+f.prefix]) & 0xFF
	if targetLabel < f.nextFloorLabel {
		return
	}

	var newFP int64 = f.fpOrig
	for {
		code, _ := store.ReadVLong(f.floorDataReader)
		newFP = f.fpOrig + int64(uint64(code)>>1)
		f.hasTerms = (code & 1) != 0
		f.isLastInFloor = f.numFollowFloorBlocks == 1
		f.numFollowFloorBlocks--
		if f.isLastInFloor {
			f.nextFloorLabel = 256
			break
		}
		b, _ := f.floorDataReader.ReadByte()
		f.nextFloorLabel = int(b) & 0xFF
		if targetLabel < f.nextFloorLabel {
			break
		}
	}
	if newFP != f.fp {
		f.nextEnt = -1
		f.fp = newFP
	}
}

// decodeMetaData lazily decodes per-term statistics and postings metadata for
// all terms up to getTermBlockOrd().
//
// Port of SegmentTermsEnumFrame.decodeMetaData.
func (f *segmentTermsEnumFrame) decodeMetaData() error {
	limit := f.getTermBlockOrd()
	absolute := f.metaDataUpto == 0

	for f.metaDataUpto < limit {
		if f.version >= versionCompressedSuffixes {
			if f.statsSingletonRunLength > 0 {
				if f.state != nil {
					f.state.DocFreq = 1
					f.state.TotalTermFreq = 1
				}
				f.statsSingletonRunLength--
			} else {
				token, _ := store.ReadVInt(f.statsReader)
				if (token & 1) == 1 {
					if f.state != nil {
						f.state.DocFreq = 1
						f.state.TotalTermFreq = 1
					}
					f.statsSingletonRunLength = int(token >> 1)
				} else {
					if f.state != nil {
						f.state.DocFreq = int(token >> 1)
					}
					if f.ste.fr.fieldInfo.IndexOptions() == index.IndexOptionsDocs {
						if f.state != nil {
							f.state.TotalTermFreq = int64(f.state.DocFreq)
						}
					} else {
						v, _ := store.ReadVLong(f.statsReader)
						if f.state != nil {
							f.state.TotalTermFreq = int64(f.state.DocFreq) + int64(v)
						}
					}
				}
			}
		} else {
			df, _ := store.ReadVInt(f.statsReader)
			if f.state != nil {
				f.state.DocFreq = int(df)
			}
			if f.ste.fr.fieldInfo.IndexOptions() == index.IndexOptionsDocs {
				if f.state != nil {
					f.state.TotalTermFreq = int64(f.state.DocFreq)
				}
			} else {
				v, _ := store.ReadVLong(f.statsReader)
				if f.state != nil {
					f.state.TotalTermFreq = int64(f.state.DocFreq) + int64(v)
				}
			}
		}

		// Decode postings metadata.
		if f.state != nil && f.ste.fr.parent.postingsReader != nil {
			if err := f.ste.fr.parent.postingsReader.DecodeTerm(
				f.bytesReader,
				f.ste.fr.fieldInfo,
				f.state,
				absolute,
			); err != nil {
				return fmt.Errorf("blocktree decodeMetaData: %w", err)
			}
		}

		f.metaDataUpto++
		absolute = false
	}
	if f.state != nil {
		f.state.TermBlockOrd = f.metaDataUpto
	}
	return nil
}

// scanToSubBlock scans entries in this non-leaf block until the sub-block
// pointer with the given file pointer is found.
//
// Port of SegmentTermsEnumFrame.scanToSubBlock.
//
//lint:ignore U1000 used by seekFloor path, not yet wired for this backward-compat codec.
func (f *segmentTermsEnumFrame) scanToSubBlock(subFP int64) {
	if f.lastSubFP == subFP {
		return
	}
	targetSubCode := f.fp - subFP
	for {
		f.nextEnt++
		code, _ := store.ReadVInt(f.suffixLengthsReader)
		// skip suffix bytes
		f.suffixesReader.SetPosition(f.suffixesReader.GetPosition() + int(code>>1))
		if (code & 1) != 0 {
			subCode, _ := store.ReadVLong(f.suffixLengthsReader)
			if int64(subCode) == targetSubCode {
				f.lastSubFP = subFP
				return
			}
		} else {
			if f.state != nil {
				f.state.TermBlockOrd++
			}
		}
	}
}

// seekStatus is the result of a scan-to-term operation.
type seekStatus int

const (
	seekStatusEnd      seekStatus = iota // exhausted block without finding target
	seekStatusNotFound                   // past target; current entry is after target
	seekStatusFound                      // exact match
)

// scanToTerm scans the current block's entries looking for target.
//
// Port of SegmentTermsEnumFrame.scanToTerm.
func (f *segmentTermsEnumFrame) scanToTerm(target *util.BytesRef, exactOnly bool) (seekStatus, error) {
	if f.isLeafBlock {
		return f.scanToTermLeaf(target, exactOnly)
	}
	return f.scanToTermNonLeaf(target, exactOnly)
}

// scanToTermLeaf scans a leaf block for target.
//
// Port of SegmentTermsEnumFrame.scanToTermLeaf.
func (f *segmentTermsEnumFrame) scanToTermLeaf(target *util.BytesRef, exactOnly bool) (seekStatus, error) {
	f.ste.termExists = true
	f.subCode = 0

	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
		}
		return seekStatusEnd, nil
	}

	for {
		f.nextEnt++
		v, _ := store.ReadVInt(f.suffixLengthsReader)
		f.suffix = int(v)
		f.startBytePos = f.suffixesReader.GetPosition()
		_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffix)

		cmp := bytes.Compare(
			f.suffixBytes[f.startBytePos:f.startBytePos+f.suffix],
			target.Bytes[target.Offset+f.prefix:target.Offset+target.Length],
		)
		if cmp < 0 {
			// Keep scanning.
		} else if cmp > 0 {
			f.fillTerm()
			return seekStatusNotFound, nil
		} else {
			f.fillTerm()
			return seekStatusFound, nil
		}
		if f.nextEnt == f.entCount {
			if exactOnly {
				f.fillTerm()
			}
			return seekStatusEnd, nil
		}
	}
}

// scanToTermNonLeaf scans a non-leaf block for target.
//
// Port of SegmentTermsEnumFrame.scanToTermNonLeaf.
func (f *segmentTermsEnumFrame) scanToTermNonLeaf(target *util.BytesRef, exactOnly bool) (seekStatus, error) {
	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
			f.ste.termExists = f.subCode == 0
		}
		return seekStatusEnd, nil
	}

	for f.nextEnt < f.entCount {
		f.nextEnt++
		code, _ := store.ReadVInt(f.suffixLengthsReader)
		f.suffix = int(code >> 1)
		termLen := f.prefix + f.suffix
		f.startBytePos = f.suffixesReader.GetPosition()
		_ = f.suffixesReader.SetPosition(f.startBytePos + f.suffix)
		f.ste.termExists = (code & 1) == 0
		if f.ste.termExists {
			if f.state != nil {
				f.state.TermBlockOrd++
			}
			f.subCode = 0
		} else {
			v, _ := store.ReadVLong(f.suffixLengthsReader)
			f.subCode = int64(v)
			f.lastSubFP = f.fp - f.subCode
		}

		cmp := bytes.Compare(
			f.suffixBytes[f.startBytePos:f.startBytePos+f.suffix],
			target.Bytes[target.Offset+f.prefix:target.Offset+target.Length],
		)
		if cmp < 0 {
			// Keep scanning.
		} else if cmp > 0 {
			f.fillTerm()
			if !exactOnly && !f.ste.termExists {
				// On a sub-block: recurse to find the next term after target.
				f.ste.currentFrame = f.ste.pushFrameFP(nil, f.ste.currentFrame.lastSubFP, termLen)
				if err := f.ste.currentFrame.loadBlock(); err != nil {
					return 0, err
				}
				for {
					isSubBlock, err := f.ste.currentFrame.next()
					if err != nil {
						return 0, err
					}
					if !isSubBlock {
						break
					}
					f.ste.currentFrame = f.ste.pushFrameFP(nil, f.ste.currentFrame.lastSubFP, f.ste.term.Length())
					if err2 := f.ste.currentFrame.loadBlock(); err2 != nil {
						return 0, err2
					}
				}
			}
			return seekStatusNotFound, nil
		} else {
			f.fillTerm()
			return seekStatusFound, nil
		}
	}

	if exactOnly {
		f.fillTerm()
	}
	return seekStatusEnd, nil
}

// fillTerm copies the current suffix into ste.term.
func (f *segmentTermsEnumFrame) fillTerm() {
	termLen := f.prefix + f.suffix
	f.ste.term.SetLength(termLen)
	f.ste.term.Grow(termLen)
	copy(f.ste.term.Bytes()[f.prefix:], f.suffixBytes[f.startBytePos:f.startBytePos+f.suffix])
}

// indexInputCompressAdapter wraps a store.IndexInput so it satisfies the
// package-private compressInput interface (which requires ReadVInt).
// store.IndexInput embeds DataInput but not VariableLengthInput, so a thin
// adapter that delegates ReadVInt via the package-level function is needed.
type indexInputCompressAdapter struct {
	store.IndexInput
}

func (a indexInputCompressAdapter) ReadVInt() (int32, error) {
	return store.ReadVInt(a.IndexInput)
}
