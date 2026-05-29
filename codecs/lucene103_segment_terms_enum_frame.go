// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// SegmentTermsEnumFrame.java (Apache Lucene 10.4.0).
//
// segmentTermsEnumFrame is the per-block cursor pushed onto
// [Lucene103SegmentTermsEnum].stack as the seek / next walk descends the .tim
// block file. It is the strict port of Lucene's package-private
// SegmentTermsEnumFrame and drives the multi-block, floor-block and
// sub-block recursion the previous Next()-only stub deferred (backlog #2692,
// resolved under rmp #4754).
//
// Note on naming: the exported [SegmentTermsEnumFrame] in lucene103_stats.go is
// a separate, Length()-only struct consumed solely by [Lucene103BlockTreeStats].
// This lowercase type is the production cursor; the two are intentionally
// distinct and must not be merged without updating the Stats contract.
//
// A frame is single-threaded and owned by exactly one
// [Lucene103SegmentTermsEnum]; the back-pointer ste reaches the shared .tim
// IndexInput, the field reader and the postings reader.
type segmentTermsEnumFrame struct {
	// ord is this frame's index in ste.stack (mirrors Java field "ord").
	ord int

	hasTerms     bool
	hasTermsOrig bool
	isFloor      bool

	// node is the trie node this frame was seek'd to, or nil for a "next"
	// frame pushed during sequential iteration.
	node *TrieNode

	// fp is the .tim file pointer this block was loaded from. fpOrig is the
	// parent floor block's file pointer (== fp for non-floor blocks). fpEnd
	// is the file pointer just past this block's data.
	fp               int64
	fpOrig           int64
	fpEnd            int64
	totalSuffixBytes int64 // for stats parity; not consumed by the enum

	// Per-block buffers, reused across loadBlock calls and grown via
	// util.Oversize exactly like Lucene's ArrayUtil.oversize.
	suffixBytes    []byte
	suffixesReader *store.ByteArrayDataInput

	suffixLengthBytes   []byte
	suffixLengthsReader *store.ByteArrayDataInput

	statBytes               []byte
	statsSingletonRunLength int
	statsReader             *store.ByteArrayDataInput

	// rewindPos / floorDataPos / floorDataReader manage the per-frame floor
	// data cursor. floorDataReader is an independent clone of the trie index
	// input so several frames can each track their own floor position without
	// trampling one another (Lucene relies on save/restore of the shared
	// IndexInput; Gocene clones to make the contract explicit and safe).
	rewindPos       int64
	floorDataPos    int64
	floorDataReader store.IndexInput

	// prefixLength is the number of bytes shared by every term in this block
	// (the trie depth at which the block was pushed).
	prefixLength int

	// entCount is the number of entries (term + sub-block) in this block.
	entCount int

	// nextEnt is the index of the next entry to decode, or -1 when the block
	// is not yet loaded.
	nextEnt int

	isLastInFloor bool
	isLeafBlock   bool
	allEqual      bool

	lastSubFP int64

	nextFloorLabel       int
	numFollowFloorBlocks int

	// metaDataUpto is the number of term entries whose stats + postings
	// metadata have been decoded. Metadata decode is lazy.
	metaDataUpto int

	state *BlockTermState

	// bytes / bytesReader hold the postings-side metadata blob decoded by
	// decodeMetaData via PostingsReaderBase.DecodeTerm.
	bytes       []byte
	bytesReader *store.ByteArrayDataInput

	// startBytePos / suffixLength / subCode mirror the Java scratch fields set
	// as a side effect of next()/scanToTerm and consumed by fillTerm.
	startBytePos int
	suffixLength int
	subCode      int64

	compressionAlg CompressionAlgorithm

	ste *Lucene103SegmentTermsEnum
}

// newSegmentTermsEnumFrame allocates a frame for ste at the given stack
// ordinal, mirroring the Java constructor SegmentTermsEnumFrame(ste, ord). The
// baseline buffer sizes match Lucene's defaults (128 / 32 / 64 / 32) and grow
// on demand inside loadBlock.
func newSegmentTermsEnumFrame(ste *Lucene103SegmentTermsEnum, ord int) (*segmentTermsEnumFrame, error) {
	if ste == nil {
		return nil, errors.New("newSegmentTermsEnumFrame: ste must not be nil")
	}
	f := &segmentTermsEnumFrame{
		ste:                 ste,
		ord:                 ord,
		suffixBytes:         make([]byte, 128),
		suffixesReader:      store.NewByteArrayDataInput(nil),
		suffixLengthBytes:   make([]byte, 32),
		suffixLengthsReader: store.NewByteArrayDataInput(nil),
		statBytes:           make([]byte, 64),
		statsReader:         store.NewByteArrayDataInput(nil),
		bytes:               make([]byte, 32),
		bytesReader:         store.NewByteArrayDataInput(nil),
		nextEnt:             -1,
		lastSubFP:           -1,
		compressionAlg:      CompressionNoCompression,
	}
	if ste.fr != nil && ste.fr.parent != nil && ste.fr.parent.postingsReader != nil {
		f.state = ste.fr.parent.postingsReader.NewTermState()
	} else {
		f.state = NewBlockTermState()
	}
	f.state.TotalTermFreq = -1
	return f, nil
}

// setFloorData binds the frame to the floor-data section of its trie node.
// in is positioned at the node's floorDataFp; the frame clones it so its file
// pointer is independent of sibling frames and of the trie walk.
//
// Mirrors SegmentTermsEnumFrame.setFloorData(IndexInput).
func (f *segmentTermsEnumFrame) setFloorData(in store.IndexInput) error {
	// in is positioned by TrieReader.FloorData at the node's floor-data start.
	// Capture that position before cloning: not every IndexInput.Clone()
	// implementation preserves the source file pointer on the clone (e.g.
	// SimpleFS/ByteBuffers slices return a clone positioned at 0), so seek the
	// clone explicitly to keep the read independent of that behaviour.
	pos := in.GetFilePointer()
	clone := in.Clone()
	if err := clone.SetPosition(pos); err != nil {
		return fmt.Errorf("setFloorData: seek clone to floor data: %w", err)
	}
	f.floorDataReader = clone
	f.rewindPos = clone.GetFilePointer()
	numFollow, err := store.ReadVInt(clone)
	if err != nil {
		return fmt.Errorf("setFloorData: read numFollowFloorBlocks: %w", err)
	}
	f.numFollowFloorBlocks = int(numFollow)
	b, err := clone.ReadByte()
	if err != nil {
		return fmt.Errorf("setFloorData: read nextFloorLabel: %w", err)
	}
	f.nextFloorLabel = int(b) & 0xff
	f.floorDataPos = clone.GetFilePointer()
	return nil
}

// getTermBlockOrd returns the term's ordinal inside the block, used as the
// metadata-decode high-water mark. Mirrors SegmentTermsEnumFrame.getTermBlockOrd.
func (f *segmentTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	return f.state.TermBlockOrd
}

// loadNextFloorBlock advances fp to the end of the current block and reloads,
// stepping to the next floor sub-block written inline. Mirrors
// SegmentTermsEnumFrame.loadNextFloorBlock.
func (f *segmentTermsEnumFrame) loadNextFloorBlock() error {
	if f.node != nil && !f.isFloor {
		return fmt.Errorf("loadNextFloorBlock: node=%v isFloor=%v", f.node, f.isFloor)
	}
	f.fp = f.fpEnd
	f.nextEnt = -1
	return f.loadBlock()
}

// loadBlock does the initial decode of the block at fp: it reads the entry
// count, the (possibly compressed) suffix corpus, the suffix-length blob, the
// stats blob and the postings metadata blob into reusable byte slices. Per-term
// metadata is decoded lazily later in decodeMetaData.
//
// Mirrors SegmentTermsEnumFrame.loadBlock.
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

	// Term suffixes: VLong header packs (numSuffixBytes << 3) | (isLeaf << 2) | comprAlg.
	codeL, err := store.ReadVLong(in)
	if err != nil {
		return fmt.Errorf("loadBlock: read suffix code: %w", err)
	}
	f.isLeafBlock = (codeL & 0x04) != 0
	numSuffixBytes := int(uint64(codeL) >> 3)
	if cap(f.suffixBytes) < numSuffixBytes {
		f.suffixBytes = make([]byte, util.Oversize(numSuffixBytes, 1))
	}
	alg, err := CompressionAlgorithmByCode(int(codeL) & 0x03)
	if err != nil {
		return index.NewCorruptIndexExceptionWithCause(err.Error(), "lucene103 terms", err)
	}
	f.compressionAlg = alg
	if err := alg.Read(asCompressionInput(in), f.suffixBytes, numSuffixBytes); err != nil {
		return fmt.Errorf("loadBlock: decompress suffixes: %w", err)
	}
	f.suffixesReader.ResetWithSlice(f.suffixBytes, 0, numSuffixBytes)

	// Suffix-length blob: VInt header packs (length << 1) | allEqual.
	slCode, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("loadBlock: read suffix-length code: %w", err)
	}
	f.allEqual = (slCode & 0x01) != 0
	numSuffixLengthBytes := int(uint32(slCode) >> 1)
	if cap(f.suffixLengthBytes) < numSuffixLengthBytes {
		f.suffixLengthBytes = make([]byte, util.Oversize(numSuffixLengthBytes, 1))
	}
	if f.allEqual {
		b, err := in.ReadByte()
		if err != nil {
			return fmt.Errorf("loadBlock: read constant suffix length: %w", err)
		}
		buf := f.suffixLengthBytes[:numSuffixLengthBytes]
		for i := range buf {
			buf[i] = b
		}
	} else {
		if err := in.ReadBytes(f.suffixLengthBytes[:numSuffixLengthBytes]); err != nil {
			return fmt.Errorf("loadBlock: read suffix lengths: %w", err)
		}
	}
	f.suffixLengthsReader.ResetWithSlice(f.suffixLengthBytes, 0, numSuffixLengthBytes)
	f.totalSuffixBytes = in.GetFilePointer() - startSuffixFP

	// Stats blob.
	numBytes, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("loadBlock: read stats length: %w", err)
	}
	if cap(f.statBytes) < int(numBytes) {
		f.statBytes = make([]byte, util.Oversize(int(numBytes), 1))
	}
	if err := in.ReadBytes(f.statBytes[:numBytes]); err != nil {
		return fmt.Errorf("loadBlock: read stats: %w", err)
	}
	f.statsReader.ResetWithSlice(f.statBytes, 0, int(numBytes))
	f.statsSingletonRunLength = 0
	f.metaDataUpto = 0

	f.state.TermBlockOrd = 0
	f.nextEnt = 0
	f.lastSubFP = -1

	// Postings metadata blob.
	numBytes, err = store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("loadBlock: read metadata length: %w", err)
	}
	if cap(f.bytes) < int(numBytes) {
		f.bytes = make([]byte, util.Oversize(int(numBytes), 1))
	}
	if err := in.ReadBytes(f.bytes[:numBytes]); err != nil {
		return fmt.Errorf("loadBlock: read metadata: %w", err)
	}
	f.bytesReader.ResetWithSlice(f.bytes, 0, int(numBytes))

	f.fpEnd = in.GetFilePointer()
	return nil
}

// rewind resets the frame to its parent floor block's start so the next
// loadBlock re-reads from fpOrig. Mirrors SegmentTermsEnumFrame.rewind.
func (f *segmentTermsEnumFrame) rewind() error {
	f.fp = f.fpOrig
	f.nextEnt = -1
	f.hasTerms = f.hasTermsOrig
	if f.isFloor {
		if f.floorDataReader == nil {
			return errors.New("rewind: floorDataReader is nil on a floor frame")
		}
		if err := f.floorDataReader.SetPosition(f.rewindPos); err != nil {
			return fmt.Errorf("rewind: seek floor data: %w", err)
		}
		numFollow, err := store.ReadVInt(f.floorDataReader)
		if err != nil {
			return fmt.Errorf("rewind: read numFollowFloorBlocks: %w", err)
		}
		f.numFollowFloorBlocks = int(numFollow)
		b, err := f.floorDataReader.ReadByte()
		if err != nil {
			return fmt.Errorf("rewind: read nextFloorLabel: %w", err)
		}
		f.nextFloorLabel = int(b) & 0xff
		f.floorDataPos = f.floorDataReader.GetFilePointer()
	}
	return nil
}

// next decodes the next entry; returns true if it is a sub-block. Mirrors
// SegmentTermsEnumFrame.next.
func (f *segmentTermsEnumFrame) next() (bool, error) {
	if f.isLeafBlock {
		return false, f.nextLeaf()
	}
	return f.nextNonLeaf()
}

// nextLeaf advances to the next term in a leaf block. Mirrors
// SegmentTermsEnumFrame.nextLeaf.
func (f *segmentTermsEnumFrame) nextLeaf() error {
	if f.nextEnt < 0 || f.nextEnt >= f.entCount {
		return fmt.Errorf("nextLeaf: nextEnt=%d entCount=%d fp=%d", f.nextEnt, f.entCount, f.fp)
	}
	f.nextEnt++
	suffix, err := f.suffixLengthsReader.ReadVInt()
	if err != nil {
		return fmt.Errorf("nextLeaf: read suffix length: %w", err)
	}
	f.suffixLength = int(suffix)
	f.startBytePos = f.suffixesReader.GetPosition()
	termLen := f.prefixLength + f.suffixLength
	f.ste.growTerm(termLen)
	if err := f.suffixesReader.ReadBytes(f.ste.term.Bytes()[f.prefixLength:termLen]); err != nil {
		return fmt.Errorf("nextLeaf: read suffix bytes: %w", err)
	}
	f.ste.termExists = true
	return nil
}

// nextNonLeaf advances to the next entry in a non-leaf block, possibly
// crossing into the next floor sub-block. Returns true if the entry is a
// sub-block. Mirrors SegmentTermsEnumFrame.nextNonLeaf.
func (f *segmentTermsEnumFrame) nextNonLeaf() (bool, error) {
	for {
		if f.nextEnt == f.entCount {
			if err := f.loadNextFloorBlock(); err != nil {
				return false, err
			}
			if f.isLeafBlock {
				return false, f.nextLeaf()
			}
			continue
		}
		if f.nextEnt < 0 || f.nextEnt >= f.entCount {
			return false, fmt.Errorf("nextNonLeaf: nextEnt=%d entCount=%d fp=%d", f.nextEnt, f.entCount, f.fp)
		}
		f.nextEnt++
		code, err := f.suffixLengthsReader.ReadVInt()
		if err != nil {
			return false, fmt.Errorf("nextNonLeaf: read suffix code: %w", err)
		}
		f.suffixLength = int(uint32(code) >> 1)
		f.startBytePos = f.suffixesReader.GetPosition()
		termLen := f.prefixLength + f.suffixLength
		f.ste.growTerm(termLen)
		if err := f.suffixesReader.ReadBytes(f.ste.term.Bytes()[f.prefixLength:termLen]); err != nil {
			return false, fmt.Errorf("nextNonLeaf: read suffix bytes: %w", err)
		}
		if code&1 == 0 {
			// A normal term.
			f.ste.termExists = true
			f.subCode = 0
			f.state.TermBlockOrd++
			return false, nil
		}
		// A sub-block; make sub-FP absolute.
		f.ste.termExists = false
		sc, err := f.suffixLengthsReader.ReadVLong()
		if err != nil {
			return false, fmt.Errorf("nextNonLeaf: read sub-block FP delta: %w", err)
		}
		f.subCode = sc
		f.lastSubFP = f.fp - sc
		return true, nil
	}
}

// scanToFloorFrame positions the frame on the floor sub-block that should
// contain target, re-reading the floor data and updating fp / hasTerms /
// isLastInFloor. Mirrors SegmentTermsEnumFrame.scanToFloorFrame.
func (f *segmentTermsEnumFrame) scanToFloorFrame(target *util.BytesRef) error {
	if !f.isFloor || target.Length <= f.prefixLength {
		return nil
	}
	targetLabel := int(target.Bytes[target.Offset+f.prefixLength]) & 0xff
	if targetLabel < f.nextFloorLabel {
		return nil
	}
	if f.numFollowFloorBlocks == 0 {
		return fmt.Errorf("scanToFloorFrame: numFollowFloorBlocks=0 with targetLabel=%d nextFloorLabel=%d", targetLabel, f.nextFloorLabel)
	}

	newFP := f.fpOrig
	if err := f.floorDataReader.SetPosition(f.floorDataPos); err != nil {
		return fmt.Errorf("scanToFloorFrame: seek floor data: %w", err)
	}
	for {
		code, err := store.ReadVLong(f.floorDataReader)
		if err != nil {
			return fmt.Errorf("scanToFloorFrame: read floor code: %w", err)
		}
		newFP = f.fpOrig + int64(uint64(code)>>1)
		f.hasTerms = (code & 1) != 0
		f.isLastInFloor = f.numFollowFloorBlocks == 1
		f.numFollowFloorBlocks--
		if f.isLastInFloor {
			f.nextFloorLabel = 256
			break
		}
		b, err := f.floorDataReader.ReadByte()
		if err != nil {
			return fmt.Errorf("scanToFloorFrame: read next floor label: %w", err)
		}
		f.nextFloorLabel = int(b) & 0xff
		if targetLabel < f.nextFloorLabel {
			break
		}
	}
	f.floorDataPos = f.floorDataReader.GetFilePointer()
	if newFP != f.fp {
		// Force re-load of the block.
		f.nextEnt = -1
		f.fp = newFP
	}
	return nil
}

// decodeMetaData lazily catches up on stats + postings metadata decode for
// every term up to (and including) the current cursor position. Mirrors
// SegmentTermsEnumFrame.decodeMetaData.
func (f *segmentTermsEnumFrame) decodeMetaData() error {
	limit := f.getTermBlockOrd()
	if limit <= 0 {
		return fmt.Errorf("decodeMetaData: limit=%d must be > 0", limit)
	}
	absolute := f.metaDataUpto == 0
	hasFreqs := f.ste.fr.fieldInfo.IndexOptions() != index.IndexOptionsDocs

	for f.metaDataUpto < limit {
		// Stats.
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
				if hasFreqs {
					delta, err := f.statsReader.ReadVLong()
					if err != nil {
						return fmt.Errorf("decodeMetaData: read totalTermFreq delta: %w", err)
					}
					f.state.TotalTermFreq = int64(f.state.DocFreq) + delta
				} else {
					f.state.TotalTermFreq = int64(f.state.DocFreq)
				}
			}
		}

		// Postings metadata.
		if err := f.ste.fr.parent.postingsReader.DecodeTerm(
			f.bytesReader, f.ste.fr.fieldInfo, f.state, absolute,
		); err != nil {
			return fmt.Errorf("decodeMetaData: DecodeTerm: %w", err)
		}

		f.metaDataUpto++
		absolute = false
	}
	f.state.TermBlockOrd = f.metaDataUpto
	return nil
}

// scanToSubBlock advances the cursor (without setting startBytePos/suffix) to
// the sub-block entry whose absolute file pointer equals subFP. Only called by
// next() when popping back into a partially scanned parent frame. Mirrors
// SegmentTermsEnumFrame.scanToSubBlock.
func (f *segmentTermsEnumFrame) scanToSubBlock(subFP int64) error {
	if f.isLeafBlock {
		return errors.New("scanToSubBlock: called on a leaf block")
	}
	if f.lastSubFP == subFP {
		return nil
	}
	if subFP >= f.fp {
		return fmt.Errorf("scanToSubBlock: subFP=%d must be < fp=%d", subFP, f.fp)
	}
	targetSubCode := f.fp - subFP
	for {
		if f.nextEnt >= f.entCount {
			return fmt.Errorf("scanToSubBlock: ran off block end fp=%d subFP=%d", f.fp, subFP)
		}
		f.nextEnt++
		code, err := f.suffixLengthsReader.ReadVInt()
		if err != nil {
			return fmt.Errorf("scanToSubBlock: read suffix code: %w", err)
		}
		if err := f.suffixesReader.SetPosition(f.suffixesReader.GetPosition() + int(uint32(code)>>1)); err != nil {
			return fmt.Errorf("scanToSubBlock: skip suffix bytes: %w", err)
		}
		if code&1 != 0 {
			subCode, err := f.suffixLengthsReader.ReadVLong()
			if err != nil {
				return fmt.Errorf("scanToSubBlock: read sub-block code: %w", err)
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

// scanToTerm scans the loaded block for target, setting startBytePos/suffix as
// a side effect. Mirrors SegmentTermsEnumFrame.scanToTerm.
func (f *segmentTermsEnumFrame) scanToTerm(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.isLeafBlock {
		if f.allEqual {
			return f.binarySearchTermLeaf(target, exactOnly)
		}
		return f.scanToTermLeaf(target, exactOnly)
	}
	return f.scanToTermNonLeaf(target, exactOnly)
}

// scanToTermLeaf linearly scans a leaf block for target. Mirrors
// SegmentTermsEnumFrame.scanToTermLeaf.
func (f *segmentTermsEnumFrame) scanToTermLeaf(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.nextEnt == -1 {
		return 0, errors.New("scanToTermLeaf: block not loaded")
	}
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
		suffix, err := f.suffixLengthsReader.ReadVInt()
		if err != nil {
			return 0, fmt.Errorf("scanToTermLeaf: read suffix length: %w", err)
		}
		f.suffixLength = int(suffix)
		f.startBytePos = f.suffixesReader.GetPosition()
		if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
			return 0, fmt.Errorf("scanToTermLeaf: skip suffix bytes: %w", err)
		}
		cmp := compareSuffixToTarget(f.suffixBytes, f.startBytePos, f.suffixLength, target, f.prefixLength)
		switch {
		case cmp < 0:
			// Still before the target; keep scanning.
		case cmp > 0:
			f.fillTerm()
			return index.SeekStatusNotFound, nil
		default:
			f.fillTerm()
			return index.SeekStatusFound, nil
		}
		if f.nextEnt >= f.entCount {
			break
		}
	}

	if exactOnly {
		f.fillTerm()
	}
	return index.SeekStatusEnd, nil
}

// binarySearchTermLeaf binary-searches a leaf block whose suffixes all share
// the same length. Mirrors SegmentTermsEnumFrame.binarySearchTermLeaf.
func (f *segmentTermsEnumFrame) binarySearchTermLeaf(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.nextEnt == -1 {
		return 0, errors.New("binarySearchTermLeaf: block not loaded")
	}
	f.ste.termExists = true
	f.subCode = 0

	if f.nextEnt == f.entCount {
		if exactOnly {
			f.fillTerm()
		}
		return index.SeekStatusEnd, nil
	}

	suffix, err := f.suffixLengthsReader.ReadVInt()
	if err != nil {
		return 0, fmt.Errorf("binarySearchTermLeaf: read suffix length: %w", err)
	}
	f.suffixLength = int(suffix)
	start := f.nextEnt
	end := f.entCount - 1
	cmp := 0
	for start <= end {
		mid := int(uint(start+end) >> 1)
		f.nextEnt = mid + 1
		f.startBytePos = mid * f.suffixLength
		cmp = compareSuffixToTarget(f.suffixBytes, f.startBytePos, f.suffixLength, target, f.prefixLength)
		switch {
		case cmp < 0:
			start = mid + 1
		case cmp > 0:
			end = mid - 1
		default:
			if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
				return 0, fmt.Errorf("binarySearchTermLeaf: set position: %w", err)
			}
			f.fillTerm()
			return index.SeekStatusFound, nil
		}
	}

	var status index.SeekStatus
	if end < f.entCount-1 {
		status = index.SeekStatusNotFound
		// Binary search ended on the lesser term but a greater term exists;
		// advance to it.
		if cmp < 0 {
			f.startBytePos += f.suffixLength
			f.nextEnt++
		}
		if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
			return 0, fmt.Errorf("binarySearchTermLeaf: set position (not found): %w", err)
		}
		f.fillTerm()
	} else {
		status = index.SeekStatusEnd
		if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
			return 0, fmt.Errorf("binarySearchTermLeaf: set position (end): %w", err)
		}
		if exactOnly {
			f.fillTerm()
		}
	}
	return status, nil
}

// scanToTermNonLeaf linearly scans a non-leaf block for target, recursing into
// sub-blocks when the not-found ceiling lands on one (non-exact mode only).
// Mirrors SegmentTermsEnumFrame.scanToTermNonLeaf.
func (f *segmentTermsEnumFrame) scanToTermNonLeaf(target *util.BytesRef, exactOnly bool) (index.SeekStatus, error) {
	if f.nextEnt == -1 {
		return 0, errors.New("scanToTermNonLeaf: block not loaded")
	}

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
			return 0, fmt.Errorf("scanToTermNonLeaf: read suffix code: %w", err)
		}
		f.suffixLength = int(uint32(code) >> 1)
		f.startBytePos = f.suffixesReader.GetPosition()
		if err := f.suffixesReader.SetPosition(f.startBytePos + f.suffixLength); err != nil {
			return 0, fmt.Errorf("scanToTermNonLeaf: skip suffix bytes: %w", err)
		}
		f.ste.termExists = (code & 1) == 0
		if f.ste.termExists {
			f.state.TermBlockOrd++
			f.subCode = 0
		} else {
			sc, err := f.suffixLengthsReader.ReadVLong()
			if err != nil {
				return 0, fmt.Errorf("scanToTermNonLeaf: read sub-block code: %w", err)
			}
			f.subCode = sc
			f.lastSubFP = f.fp - sc
		}

		cmp := compareSuffixToTarget(f.suffixBytes, f.startBytePos, f.suffixLength, target, f.prefixLength)
		switch {
		case cmp < 0:
			// Still before the target; keep scanning.
		case cmp > 0:
			f.fillTerm()
			if !exactOnly && !f.ste.termExists {
				// Position to the next term after target by recursing into the
				// sub-frame(s).
				next, err := f.ste.pushFrameFP(nil, f.ste.currentFrame.lastSubFP, f.prefixLength+f.suffixLength)
				if err != nil {
					return 0, err
				}
				f.ste.currentFrame = next
				if err := f.ste.currentFrame.loadBlock(); err != nil {
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
					nf, err := f.ste.pushFrameFP(nil, f.ste.currentFrame.lastSubFP, f.ste.term.Length())
					if err != nil {
						return 0, err
					}
					f.ste.currentFrame = nf
					if err := f.ste.currentFrame.loadBlock(); err != nil {
						return 0, err
					}
				}
			}
			return index.SeekStatusNotFound, nil
		default:
			// Exact match: cannot be a sub-block (the index would have routed
			// us straight to it).
			f.fillTerm()
			return index.SeekStatusFound, nil
		}
	}

	if exactOnly {
		f.fillTerm()
	}
	return index.SeekStatusEnd, nil
}

// fillTerm copies the current suffix into the shared term builder at
// prefixLength. Mirrors SegmentTermsEnumFrame.fillTerm.
func (f *segmentTermsEnumFrame) fillTerm() {
	termLen := f.prefixLength + f.suffixLength
	f.ste.growTerm(termLen)
	copy(f.ste.term.Bytes()[f.prefixLength:termLen], f.suffixBytes[f.startBytePos:f.startBytePos+f.suffixLength])
}

// compressionInputAdapter adapts any [store.DataInput] to the [CompressionInput]
// surface ([store.DataInput] + [store.VariableLengthInput]) by computing VInt /
// VLong through the package-level [store.ReadVInt] / [store.ReadVLong] helpers,
// which only require ReadByte. Some IndexInput implementations
// (SimpleFSIndexInput, MMapIndexInput, NIOFSIndexInput) do not declare ReadVInt
// / ReadVLong methods of their own, so a direct type assertion to
// CompressionInput would fail for on-disk directories. The adapter keeps the
// block-tree reader independent of which concrete IndexInput backs the .tim
// file.
type compressionInputAdapter struct {
	store.DataInput
}

func (a compressionInputAdapter) ReadVInt() (int32, error)  { return store.ReadVInt(a.DataInput) }
func (a compressionInputAdapter) ReadVLong() (int64, error) { return store.ReadVLong(a.DataInput) }

// asCompressionInput returns in as a [CompressionInput], wrapping it in the
// adapter only when it does not already satisfy the interface natively (so
// ByteBuffersIndexInput / BufferedIndexInput pay no overhead).
func asCompressionInput(in store.IndexInput) CompressionInput {
	if ci, ok := in.(CompressionInput); ok {
		return ci
	}
	return compressionInputAdapter{DataInput: in}
}

// compareSuffixToTarget compares suffixBytes[start:start+len] with the suffix
// portion of target (target.Bytes[target.Offset+prefixLength : target.Offset+target.Length])
// using unsigned byte ordering, the same comparison Lucene performs with
// Arrays.compareUnsigned.
func compareSuffixToTarget(suffixBytes []byte, start, length int, target *util.BytesRef, prefixLength int) int {
	a := suffixBytes[start : start+length]
	b := target.Bytes[target.Offset+prefixLength : target.Offset+target.Length]
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}
