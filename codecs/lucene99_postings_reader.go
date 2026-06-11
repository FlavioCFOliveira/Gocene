// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Source: lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/
//
//	lucene99/Lucene99PostingsReader.java
//
// Purpose: read-side decoder for the Lucene 9.9 backward-codecs postings
// format.  Block size is 128 (long-based ForUtil).  The skip data is read
// inline from the .doc stream (as opposed to the Lucene104 format which
// stores skip data in a separate .psm meta file).
//
// Impact encoding: VInt freqDelta with bit-0 norm-change flag.  When bit-0
// is clear, norm advances by 1.  When bit-0 is set, the next VLong is a
// zig-zag-encoded norm delta, and norm advances by 1 + decodedDelta.

package codecs

import (
	"fmt"
	"math"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ─── Constants ────────────────────────────────────────────────────────────────────

const (
	lucene99MetaCodec  = "Lucene99PostingsWriterMeta"
	lucene99DocCodec   = "Lucene99PostingsWriterDoc"
	lucene99PosCodec   = "Lucene99PostingsWriterPos"
	lucene99PayCodec   = "Lucene99PostingsWriterPay"
	lucene99TermsCodec = "Lucene90PostingsWriterTerms"

	lucene99VersionStart   = 0
	lucene99VersionCurrent = 0

	lucene99MaxSkipLevels = 10

	lucene99MaxPostingsSizeForFullPrefetch = 16_384

	// lucene99NoMoreDocs is the internal "no more docs" sentinel
	// (math.MaxInt32, matching Java's Integer.MAX_VALUE). It is used in
	// docBuffer sentinel and skip-level markers. Public boundaries translate
	// this to index.NO_MORE_DOCS (-1).
	lucene99NoMoreDocs = math.MaxInt32
)

// We reuse lucene99BlockSize (128) from lucene99_for_util.go and
// lucene99BlockSizeLog2 (7).

// ─── Lucene99PostingsReader ───────────────────────────────────────────────────────

// Lucene99PostingsReader reads .doc / .pos / .pay files written by
// Lucene99PostingsWriter (backward codecs format, block size = 128).
//
// Mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsReader
// from Apache Lucene 10.4.0.
type Lucene99PostingsReader struct {
	docIn store.IndexInput
	posIn store.IndexInput // nil when segment has no positions
	payIn store.IndexInput // nil when segment has no payloads or offsets

	version int32

	// stateCache maps *BlockTermState handles (allocated by NewTermState) back
	// to the *IntBlockTermState that owns them.  Same bridge pattern as the
	// Lucene104PostingsReader.
	stateCacheMu sync.Mutex
	stateCache   map[*BlockTermState]*IntBlockTermState
}

// lookupOrCreateState returns the IntBlockTermState bridged to termState,
// creating and registering one on demand. It is safe for concurrent use.
func (r *Lucene99PostingsReader) lookupOrCreateState(termState *BlockTermState) *IntBlockTermState {
	r.stateCacheMu.Lock()
	defer r.stateCacheMu.Unlock()
	its := r.stateCache[termState]
	if its == nil {
		its = NewIntBlockTermState()
		its.BlockTermState = termState
		r.stateCache[termState] = its
	}
	return its
}

// NewLucene99PostingsReader opens and validates the .doc file, and
// conditionally .pos / .pay.
//
// Mirrors Lucene99PostingsReader(SegmentReadState).
func NewLucene99PostingsReader(state *SegmentReadState) (*Lucene99PostingsReader, error) {
	var docIn, posIn, payIn store.IndexInput
	success := false
	defer func() {
		if !success {
			for _, in := range []store.IndexInput{docIn, posIn, payIn} {
				if in != nil {
					_ = in.Close()
				}
			}
		}
	}()

	docName := GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, "doc")
	var err error
	docIn, err = state.Directory.OpenInput(docName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene99 postings reader: open doc %q: %w", docName, err)
	}
	version, err := CheckIndexHeader(
		docIn, lucene99DocCodec,
		int32(lucene99VersionStart), int32(lucene99VersionCurrent),
		state.SegmentInfo.GetID(), state.SegmentSuffix,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene99 postings reader: check doc header: %w", err)
	}
	if _, err = RetrieveChecksum(docIn); err != nil {
		return nil, fmt.Errorf("lucene99 postings reader: retrieve doc checksum: %w", err)
	}

	if state.FieldInfos.HasProx() {
		posName := GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, "pos")
		posIn, err = state.Directory.OpenInput(posName, store.IOContext{Context: store.ContextRead})
		if err != nil {
			return nil, fmt.Errorf("lucene99 postings reader: open pos %q: %w", posName, err)
		}
		if _, err = CheckIndexHeader(
			posIn, lucene99PosCodec, version, version,
			state.SegmentInfo.GetID(), state.SegmentSuffix,
		); err != nil {
			return nil, fmt.Errorf("lucene99 postings reader: check pos header: %w", err)
		}
		if _, err = RetrieveChecksum(posIn); err != nil {
			return nil, fmt.Errorf("lucene99 postings reader: retrieve pos checksum: %w", err)
		}

		if state.FieldInfos.HasPayloads() || state.FieldInfos.HasOffsets() {
			payName := GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, "pay")
			payIn, err = state.Directory.OpenInput(payName, store.IOContext{Context: store.ContextRead})
			if err != nil {
				return nil, fmt.Errorf("lucene99 postings reader: open pay %q: %w", payName, err)
			}
			if _, err = CheckIndexHeader(
				payIn, lucene99PayCodec, version, version,
				state.SegmentInfo.GetID(), state.SegmentSuffix,
			); err != nil {
				return nil, fmt.Errorf("lucene99 postings reader: check pay header: %w", err)
			}
			if _, err = RetrieveChecksum(payIn); err != nil {
				return nil, fmt.Errorf("lucene99 postings reader: retrieve pay checksum: %w", err)
			}
		}
	}

	r := &Lucene99PostingsReader{
		docIn:      docIn,
		posIn:      posIn,
		payIn:      payIn,
		version:    version,
		stateCache: make(map[*BlockTermState]*IntBlockTermState),
	}
	success = true
	return r, nil
}

// Init validates the terms-in header and block size.
//
// Mirrors Lucene99PostingsReader.init(IndexInput, SegmentReadState).
func (r *Lucene99PostingsReader) Init(termsIn store.IndexInput, state *SegmentReadState) error {
	if _, err := CheckIndexHeader(
		termsIn,
		lucene99TermsCodec,
		int32(lucene99VersionStart),
		int32(lucene99VersionCurrent),
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	); err != nil {
		return fmt.Errorf("lucene99 postings reader: init terms header: %w", err)
	}
	blockSize, err := store.ReadVInt(termsIn)
	if err != nil {
		return fmt.Errorf("lucene99 postings reader: read block size: %w", err)
	}
	if int(blockSize) != lucene99BlockSize {
		return fmt.Errorf(
			"lucene99 postings reader: index-time BLOCK_SIZE (%d) != read-time BLOCK_SIZE (%d)",
			blockSize, lucene99BlockSize,
		)
	}
	return nil
}

// NewTermState allocates a fresh IntBlockTermState and registers it in the
// stateCache so that DecodeTerm/Postings can retrieve the extended state later.
func (r *Lucene99PostingsReader) NewTermState() *BlockTermState {
	its := NewIntBlockTermState()
	r.stateCacheMu.Lock()
	r.stateCache[its.BlockTermState] = its
	r.stateCacheMu.Unlock()
	return its.BlockTermState
}

// DecodeTerm reads codec-specific metadata from in into termState.
//
// Mirrors Lucene99PostingsReader.decodeTerm(DataInput, FieldInfo,
// BlockTermState, boolean).
func (r *Lucene99PostingsReader) DecodeTerm(
	in store.DataInput,
	fieldInfo *index.FieldInfo,
	termState *BlockTermState,
	absolute bool,
) error {
	its := r.lookupOrCreateState(termState)

	hasPos := fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	hasPayloads := fieldInfo.HasPayloads()

	if absolute {
		its.DocStartFP = 0
		its.PosStartFP = 0
		its.PayStartFP = 0
	}

	l, err := store.ReadVLong(in)
	if err != nil {
		return fmt.Errorf("lucene99 decode term: read vlong: %w", err)
	}

	if l&0x01 == 0 {
		its.DocStartFP += l >> 1
		if termState.DocFreq == 1 {
			v, err2 := store.ReadVInt(in)
			if err2 != nil {
				return fmt.Errorf("lucene99 decode term: read singleton docID: %w", err2)
			}
			its.SingletonDocID = int(v)
		} else {
			its.SingletonDocID = -1
		}
	} else {
		its.SingletonDocID += int(util.ZigZagDecodeInt64(l >> 1))
	}

	if hasPos {
		delta, err2 := store.ReadVLong(in)
		if err2 != nil {
			return fmt.Errorf("lucene99 decode term: read pos fp delta: %w", err2)
		}
		its.PosStartFP += delta

		if hasOffsets || hasPayloads {
			delta2, err3 := store.ReadVLong(in)
			if err3 != nil {
				return fmt.Errorf("lucene99 decode term: read pay fp delta: %w", err3)
			}
			its.PayStartFP += delta2
		}

		if termState.TotalTermFreq > int64(lucene99BlockSize) {
			offset, err4 := store.ReadVLong(in)
			if err4 != nil {
				return fmt.Errorf("lucene99 decode term: read lastPosBlockOffset: %w", err4)
			}
			its.LastPosBlockOffset = offset
		} else {
			its.LastPosBlockOffset = -1
		}
	}

	if termState.DocFreq > lucene99BlockSize {
		skipOffset, err2 := store.ReadVLong(in)
		if err2 != nil {
			return fmt.Errorf("lucene99 decode term: read skipOffset: %w", err2)
		}
		its.SkipOffset = skipOffset
	} else {
		its.SkipOffset = -1
	}

	return nil
}

// Postings returns a BlockDocsEnum or EverythingEnum positioned at the term.
//
// Mirrors Lucene99PostingsReader.postings(FieldInfo, BlockTermState,
// PostingsEnum, int).
func (r *Lucene99PostingsReader) Postings(
	fieldInfo *index.FieldInfo,
	termState *BlockTermState,
	reuse index.PostingsEnum,
	flags int,
) (index.PostingsEnum, error) {
	its := r.lookupOrCreateState(termState)

	hasPos := fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
	needsPos := hasPos && (flags&index.PostingsFlagPositions) != 0

	if !hasPos || !needsPos {
		var docsEnum *blockDocsEnum99
		if prev, ok := reuse.(*blockDocsEnum99); ok && prev.canReuse(r.docIn, fieldInfo) {
			docsEnum = prev
		} else {
			docsEnum = newBlockDocsEnum99(fieldInfo, r)
		}
		return docsEnum.reset(its, flags)
	}

	var everythingEnum *everythingEnum99
	if prev, ok := reuse.(*everythingEnum99); ok && prev.canReuse(r.docIn, fieldInfo) {
		everythingEnum = prev
	} else {
		everythingEnum = newEverythingEnum99(fieldInfo, r)
	}
	return everythingEnum.reset(its, flags)
}

// Impacts returns an ImpactsEnum for impact-aware scoring.
//
// Mirrors Lucene99PostingsReader.impacts(FieldInfo, BlockTermState, int).
func (r *Lucene99PostingsReader) Impacts(
	fieldInfo *index.FieldInfo,
	termState *BlockTermState,
	flags int,
) (index.ImpactsEnum, error) {
	its := r.lookupOrCreateState(termState)

	if its.DocFreq <= lucene99BlockSize {
		// No skip data, wrap in SlowImpactsEnum.
		pe, err := r.Postings(fieldInfo, termState, nil, flags)
		if err != nil {
			return nil, err
		}
		return index.NewSlowImpactsEnum(pe), nil
	}

	hasPos := fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	hasPayloads := fieldInfo.HasPayloads()
	needsPos := hasPos && (flags&index.PostingsFlagPositions) != 0
	needsOffsets := hasOffsets && (flags&index.PostingsFlagOffsets) != 0
	needsPayloads := hasPayloads && (flags&index.PostingsFlagPayloads) != 0

	if !hasPos || !needsPos {
		return newBlockImpactsDocsEnum99(fieldInfo, its, r)
	}

	if (hasOffsets && needsOffsets) || (hasPayloads && needsPayloads) {
		return newBlockImpactsEverythingEnum99(fieldInfo, its, r, flags)
	}

	return newBlockImpactsPostingsEnum99(fieldInfo, its, r)
}

// CheckIntegrity validates CRC footers on all owned files.
//
// Mirrors Lucene99PostingsReader.checkIntegrity().
func (r *Lucene99PostingsReader) CheckIntegrity() error {
	if r.docIn != nil {
		if _, err := ChecksumEntireFile(r.docIn); err != nil {
			return fmt.Errorf("lucene99 postings reader: checksum doc: %w", err)
		}
	}
	if r.posIn != nil {
		if _, err := ChecksumEntireFile(r.posIn); err != nil {
			return fmt.Errorf("lucene99 postings reader: checksum pos: %w", err)
		}
	}
	if r.payIn != nil {
		if _, err := ChecksumEntireFile(r.payIn); err != nil {
			return fmt.Errorf("lucene99 postings reader: checksum pay: %w", err)
		}
	}
	return nil
}

// Close releases file handles owned by the reader.
func (r *Lucene99PostingsReader) Close() error {
	var firstErr error
	for _, in := range []store.IndexInput{r.docIn, r.posIn, r.payIn} {
		if in == nil {
			continue
		}
		if err := in.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// seekAndPrefetchPostings repositions docIn to the start of the term's
// postings data.  Prefetch hints are omitted because the Go IndexInput
// interface does not expose a prefetch method.
//
// Mirrors Lucene99PostingsReader.seekAndPrefetchPostings.
func (r *Lucene99PostingsReader) seekAndPrefetchPostings(docIn store.IndexInput, state *IntBlockTermState) error {
	if docIn.GetFilePointer() != state.DocStartFP {
		return docIn.SetPosition(state.DocStartFP)
	}
	return nil
}

// prefetchSkipData positions docIn near the skip data, but only if
// skipOffset is large enough that it wasn't already covered by the initial
// prefetch.  Prefetch hints are omitted (no prefetch API in Go).
//
// Mirrors Lucene99PostingsReader.prefetchSkipData.
func (r *Lucene99PostingsReader) prefetchSkipData(docIn store.IndexInput, docStartFP, skipOffset int64) error {
	_ = docIn
	_ = docStartFP
	_ = skipOffset
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────────

// prefixSum99 computes the prefix (exclusive) sum over buffer[0:count],
// starting from base.  After this call buffer[i] holds the absolute doc ID.
// Mirrors Lucene99PostingsReader.prefixSum.
func prefixSum99(buffer []int64, count int, base int64) {
	sum := base
	for i := 0; i < count; i++ {
		sum += buffer[i]
		buffer[i] = sum
	}
}

// findFirstGreater99 returns the smallest index i >= from such that
// buffer[i] >= target, or lucene99BlockSize if none found.
// Mirrors Lucene99PostingsReader.findFirstGreater.
func findFirstGreater99(buffer []int64, target int64, from int) int {
	for i := from; i < lucene99BlockSize; i++ {
		if buffer[i] >= target {
			return i
		}
	}
	return lucene99BlockSize
}

// readLucene99VIntBlock decodes a tail (< BLOCK_SIZE docs) block using
// group-varint encoding with optional interleaved freq values.
// Mirrors PostingsUtil.readVIntBlock from backward_codecs.lucene99.
func readLucene99VIntBlock(
	docIn store.DataInput,
	docBuffer []int64,
	freqBuffer []int64,
	num int,
	indexHasFreq bool,
	decodeFreq bool,
) error {
	if err := util.ReadGroupVIntsInt64(docIn, docBuffer, num); err != nil {
		return err
	}
	if indexHasFreq && decodeFreq {
		for i := 0; i < num; i++ {
			freqBuffer[i] = docBuffer[i] & 0x01
			docBuffer[i] >>= 1
			if freqBuffer[i] == 0 {
				v, err := store.ReadVInt(docIn)
				if err != nil {
					return err
				}
				freqBuffer[i] = int64(v)
			}
		}
	} else if indexHasFreq {
		for i := 0; i < num; i++ {
			docBuffer[i] >>= 1
		}
	}
	return nil
}

// ─── lucene99SkipReader ───────────────────────────────────────────────────────────

// lucene99SkipReader wraps MultiLevelSkipListReader for the lucene99
// postings format.  It maintains per-level doc/pos/pay pointers that are
// updated in the readSkipData callback and snapshotted in the
// onSeekChild/onSetLastSkipData hooks.
//
// Mirrors backward_codecs.lucene99.Lucene99SkipReader.
type lucene99SkipReader struct {
	base              *MultiLevelSkipListReader
	docPointer        []int64
	posPointer        []int64 // nil when hasPos is false
	payPointer        []int64 // nil when !(hasOffsets || hasPayloads)
	posBufferUpto     []int   // nil when hasPos is false
	payloadByteUpto   []int   // nil when hasPayloads is false
	hasPos            bool
	hasOffsetsOrPay   bool

	// "Last accepted" fields snapshot by onSetLastSkipData.
	lastDocPointer       int64
	lastPosPointer       int64
	lastPayPointer       int64
	lastPosBufferUpto    int
	lastPayloadByteUpto  int
}

// newLucene99SkipReader creates a skip reader for the lucene99 format.
func newLucene99SkipReader(
	skipStream store.IndexInput,
	maxSkipLevels int,
	hasPos, hasOffsets, hasPayloads bool,
) *lucene99SkipReader {
	r := &lucene99SkipReader{
		docPointer:      make([]int64, maxSkipLevels),
		hasPos:          hasPos,
		hasOffsetsOrPay: hasOffsets || hasPayloads,
	}
	if hasPos {
		r.posPointer = make([]int64, maxSkipLevels)
		r.posBufferUpto = make([]int, maxSkipLevels)
		if hasPayloads {
			r.payloadByteUpto = make([]int, maxSkipLevels)
		}
		if r.hasOffsetsOrPay {
			r.payPointer = make([]int64, maxSkipLevels)
		}
	}

	readSkipData := func(level int, skipStream store.IndexInput) (int, error) {
		delta, err := store.ReadVInt(skipStream)
		if err != nil {
			return 0, err
		}

		docPtrDelta, err := store.ReadVLong(skipStream)
		if err != nil {
			return 0, err
		}
		r.docPointer[level] += docPtrDelta

		if r.posPointer != nil {
			posPtrDelta, err2 := store.ReadVLong(skipStream)
			if err2 != nil {
				return 0, err2
			}
			r.posPointer[level] += posPtrDelta

			posBuf, err3 := store.ReadVInt(skipStream)
			if err3 != nil {
				return 0, err3
			}
			r.posBufferUpto[level] = int(posBuf)

			if r.payloadByteUpto != nil {
				payBuf, err4 := store.ReadVInt(skipStream)
				if err4 != nil {
					return 0, err4
				}
				r.payloadByteUpto[level] = int(payBuf)
			}

			if r.payPointer != nil {
				payPtrDelta, err5 := store.ReadVLong(skipStream)
				if err5 != nil {
					return 0, err5
				}
				r.payPointer[level] += payPtrDelta
			}
		}

		// Read and skip impact data (base class behaviour).
		impactLen, err6 := store.ReadVInt(skipStream)
		if err6 != nil {
			return 0, err6
		}
		if impactLen > 0 {
			if err7 := skipBytesInput(skipStream, int64(impactLen)); err7 != nil {
				return 0, err7
			}
		}

		return int(delta), nil
	}

	r.base = NewMultiLevelSkipListReader(
		skipStream, maxSkipLevels, lucene99BlockSize, 8, readSkipData,
	)

	r.base.SetOnSeekChild(func(childLevel int) {
		r.docPointer[childLevel] = r.lastDocPointer
		if r.posPointer != nil {
			r.posPointer[childLevel] = r.lastPosPointer
			r.posBufferUpto[childLevel] = r.lastPosBufferUpto
			if r.payloadByteUpto != nil {
				r.payloadByteUpto[childLevel] = r.lastPayloadByteUpto
			}
			if r.payPointer != nil {
				r.payPointer[childLevel] = r.lastPayPointer
			}
		}
	})

	r.base.SetOnSetLastSkipData(func(level int) {
		r.lastDocPointer = r.docPointer[level]
		if r.posPointer != nil {
			r.lastPosPointer = r.posPointer[level]
			r.lastPosBufferUpto = r.posBufferUpto[level]
			if r.payPointer != nil {
				r.lastPayPointer = r.payPointer[level]
			}
			if r.payloadByteUpto != nil {
				r.lastPayloadByteUpto = r.payloadByteUpto[level]
			}
		}
	})

	return r
}

// trim adjusts docFreq so that the base skip reader doesn't attempt to
// read past the last valid skip entry.
func (r *lucene99SkipReader) trim(df int) int {
	if df%lucene99BlockSize == 0 {
		return df - 1
	}
	return df
}

// init initialises the skip reader for a new term.
func (r *lucene99SkipReader) init(
	skipPointer, docBasePointer, posBasePointer, payBasePointer int64, df int,
) error {
	if err := r.base.Init(skipPointer, r.trim(df)); err != nil {
		return err
	}
	r.lastDocPointer = docBasePointer
	r.lastPosPointer = posBasePointer
	r.lastPayPointer = payBasePointer

	for i := range r.docPointer {
		r.docPointer[i] = docBasePointer
	}
	if r.posPointer != nil {
		for i := range r.posPointer {
			r.posPointer[i] = posBasePointer
		}
		if r.payPointer != nil {
			for i := range r.payPointer {
				r.payPointer[i] = payBasePointer
			}
		}
	}
	return nil
}

// skipTo advances to the skip entry for target and returns the number
// of docs skipped.
func (r *lucene99SkipReader) skipTo(target int) (int, error) {
	return r.base.SkipTo(target)
}

func (r *lucene99SkipReader) getDocPointer() int64  { return r.lastDocPointer }
func (r *lucene99SkipReader) getPosPointer() int64   { return r.lastPosPointer }
func (r *lucene99SkipReader) getPayPointer() int64   { return r.lastPayPointer }
func (r *lucene99SkipReader) getPosBufferUpto() int  { return r.lastPosBufferUpto }
func (r *lucene99SkipReader) getPayloadByteUpto() int { return r.lastPayloadByteUpto }

// getNextSkipDoc returns the doc id of the next skip entry on level 0.
func (r *lucene99SkipReader) getNextSkipDoc() int {
	return r.base.GetSkipDoc(0)
}

// getDoc returns the doc id of the last skip entry accepted.
func (r *lucene99SkipReader) getDoc() int {
	return r.base.GetDoc()
}

// ─── lucene99ScoreSkipReader ──────────────────────────────────────────────────────

// lucene99ScoreSkipReader extends lucene99SkipReader with impact-data
// storage and decoding.
//
// Mirrors backward_codecs.lucene99.Lucene99ScoreSkipReader.
type lucene99ScoreSkipReader struct {
	*lucene99SkipReader
	impactData       [][]byte
	impactDataLength []int
	perLevelImpacts  []*index.FreqAndNormBuffer
	numLevels        int
	badi             *store.ByteArrayDataInput
}

// newLucene99ScoreSkipReader creates a score-aware skip reader that
// decodes and caches impact data.
func newLucene99ScoreSkipReader(
	skipStream store.IndexInput,
	maxSkipLevels int,
	hasPos, hasOffsets, hasPayloads bool,
) *lucene99ScoreSkipReader {
	// Build the base skip reader first.
	baseR := &lucene99SkipReader{
		docPointer:      make([]int64, maxSkipLevels),
		hasPos:          hasPos,
		hasOffsetsOrPay: hasOffsets || hasPayloads,
	}
	if hasPos {
		baseR.posPointer = make([]int64, maxSkipLevels)
		baseR.posBufferUpto = make([]int, maxSkipLevels)
		if hasPayloads {
			baseR.payloadByteUpto = make([]int, maxSkipLevels)
		}
		if baseR.hasOffsetsOrPay {
			baseR.payPointer = make([]int64, maxSkipLevels)
		}
	}

	r := &lucene99ScoreSkipReader{
		lucene99SkipReader: baseR,
		impactData:         make([][]byte, maxSkipLevels),
		impactDataLength:   make([]int, maxSkipLevels),
		perLevelImpacts:    make([]*index.FreqAndNormBuffer, maxSkipLevels),
		badi:               store.NewByteArrayDataInput(nil),
	}
	for i := 0; i < maxSkipLevels; i++ {
		r.impactData[i] = make([]byte, 0)
		r.perLevelImpacts[i] = index.NewFreqAndNormBuffer()
		r.perLevelImpacts[i].Add(math.MaxInt32, 1)
	}

	// Build the readSkipData callback that stores impacts.
	readSkipData := func(level int, skipStream store.IndexInput) (int, error) {
		delta, err := store.ReadVInt(skipStream)
		if err != nil {
			return 0, err
		}

		docPtrDelta, err := store.ReadVLong(skipStream)
		if err != nil {
			return 0, err
		}
		baseR.docPointer[level] += docPtrDelta

		if baseR.posPointer != nil {
			posPtrDelta, err2 := store.ReadVLong(skipStream)
			if err2 != nil {
				return 0, err2
			}
			baseR.posPointer[level] += posPtrDelta

			posBuf, err3 := store.ReadVInt(skipStream)
			if err3 != nil {
				return 0, err3
			}
			baseR.posBufferUpto[level] = int(posBuf)

			if baseR.payloadByteUpto != nil {
				payBuf, err4 := store.ReadVInt(skipStream)
				if err4 != nil {
					return 0, err4
				}
				baseR.payloadByteUpto[level] = int(payBuf)
			}

			if baseR.payPointer != nil {
				payPtrDelta, err5 := store.ReadVLong(skipStream)
				if err5 != nil {
					return 0, err5
				}
				baseR.payPointer[level] += payPtrDelta
			}
		}

		// Read and store impact data (overridden from base class).
		impactLen, err6 := store.ReadVInt(skipStream)
		if err6 != nil {
			return 0, err6
		}
		if impactLen > 0 {
			if len(r.impactData[level]) < int(impactLen) {
				r.impactData[level] = make([]byte, impactLen)
			}
			r.impactDataLength[level] = int(impactLen)
			if err7 := skipStream.ReadBytes(r.impactData[level][:impactLen]); err7 != nil {
				return 0, err7
			}
		} else {
			r.impactDataLength[level] = 0
		}

		return int(delta), nil
	}

	baseR.base = NewMultiLevelSkipListReader(
		skipStream, maxSkipLevels, lucene99BlockSize, 8, readSkipData,
	)

	baseR.base.SetOnSeekChild(func(childLevel int) {
		baseR.docPointer[childLevel] = baseR.lastDocPointer
		if baseR.posPointer != nil {
			baseR.posPointer[childLevel] = baseR.lastPosPointer
			baseR.posBufferUpto[childLevel] = baseR.lastPosBufferUpto
			if baseR.payloadByteUpto != nil {
				baseR.payloadByteUpto[childLevel] = baseR.lastPayloadByteUpto
			}
			if baseR.payPointer != nil {
				baseR.payPointer[childLevel] = baseR.lastPayPointer
			}
		}
	})

	baseR.base.SetOnSetLastSkipData(func(level int) {
		baseR.lastDocPointer = baseR.docPointer[level]
		if baseR.posPointer != nil {
			baseR.lastPosPointer = baseR.posPointer[level]
			baseR.lastPosBufferUpto = baseR.posBufferUpto[level]
			if baseR.payPointer != nil {
				baseR.lastPayPointer = baseR.payPointer[level]
			}
			if baseR.payloadByteUpto != nil {
				baseR.lastPayloadByteUpto = baseR.payloadByteUpto[level]
			}
		}
	})

	return r
}

// skipTo advances the skip reader and updates numLevels for getImpacts.
func (r *lucene99ScoreSkipReader) skipTo(target int) (int, error) {
	result, err := r.base.SkipTo(target)
	if err != nil {
		return 0, err
	}
	if r.base.NumberOfSkipLevels() > 0 {
		r.numLevels = r.base.NumberOfSkipLevels()
	} else {
		// End of postings; fill with dummy data like SlowImpactsEnum.
		r.numLevels = 1
		if len(r.perLevelImpacts[0].Freqs) < 1 {
			r.perLevelImpacts[0].GrowNoCopy(1)
		}
		r.perLevelImpacts[0].Freqs[0] = math.MaxInt32
		r.perLevelImpacts[0].Norms[0] = 1
		r.perLevelImpacts[0].Size = 1
		r.impactDataLength[0] = 0
	}
	return result, nil
}

// getImpacts returns an Impacts view backed by the stored impact bytes.
func (r *lucene99ScoreSkipReader) getImpacts() index.Impacts {
	return &l99ScoreImpacts{parent: r}
}

// l99ScoreImpacts implements index.Impacts backed by a lucene99ScoreSkipReader.
type l99ScoreImpacts struct {
	parent *lucene99ScoreSkipReader
}

func (i *l99ScoreImpacts) NumLevels() int {
	return i.parent.numLevels
}

func (i *l99ScoreImpacts) GetDocIDUpTo(level int) int {
	if level >= i.parent.numLevels {
		return index.NO_MORE_DOCS
	}
	return i.parent.base.GetSkipDoc(level)
}

func (i *l99ScoreImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	if level >= len(i.parent.impactDataLength) {
		return i.parent.perLevelImpacts[0]
	}
	dl := i.parent.impactDataLength[level]
	if dl > 0 {
		i.parent.badi.Reset(i.parent.impactData[level][:dl])
		i.parent.perLevelImpacts[level] = decodeImpacts99(i.parent.badi, i.parent.perLevelImpacts[level])
		i.parent.impactDataLength[level] = 0
	}
	return i.parent.perLevelImpacts[level]
}

// decodeImpacts99 decodes the compact impact format written by
// Lucene99ScoreSkipReader.  Wire format:
//
//	for each (freq, norm):
//	  freqDelta = VInt; freq += 1 + (freqDelta >> 1)
//	  if freqDelta & 1: norm += 1 + readZLong()
//	  else: norm++
//
// Mirrors Lucene99ScoreSkipReader.readImpacts(ByteArrayDataInput,
// FreqAndNormBuffer).
func decodeImpacts99(in *store.ByteArrayDataInput, reuse *index.FreqAndNormBuffer) *index.FreqAndNormBuffer {
	var freq, size int
	var norm int64
	reuse.GrowNoCopy(4)
	for !in.EOF() {
		raw, _ := store.ReadVInt(in)
		freq += 1 + int(raw>>1)
		if raw&1 != 0 {
			// Norm delta is encoded as ZLong (zig-zag VLong).
			zigzag, _ := store.ReadVLong(in)
			normDelta := int64(zigzag>>1) ^ -(int64(zigzag) & 1)
			norm += 1 + normDelta
		} else {
			norm++
		}
		if size == len(reuse.Freqs) {
			reuse.GrowNoCopy(size + 1)
		}
		reuse.Freqs[size] = freq
		reuse.Norms[size] = norm
		size++
	}
	reuse.Size = size
	return reuse
}

// ─── blockDocsEnum99 ──────────────────────────────────────────────────────────────

// blockDocsEnum99 iterates over doc IDs (and optionally freqs) for a term
// in the lucene99 format.  No positions.
//
// Mirrors Lucene99PostingsReader.BlockDocsEnum.
type blockDocsEnum99 struct {
	reader      *Lucene99PostingsReader
	forUtil     *lucene99ForUtil
	forDeltaUtil *lucene99ForDeltaUtil
	pforUtil    *lucene99PForUtil

	docBuffer  [lucene99BlockSize + 1]int64
	freqBuffer [lucene99BlockSize]int64

	docBufferUpto int

	skipper            *lucene99SkipReader
	skipped            bool
	prefetchedSkipData bool

	startDocIn store.IndexInput
	docIn      store.IndexInput

	indexHasFreq     bool
	indexHasPos      bool
	indexHasOffsets  bool
	indexHasPayloads bool

	docFreq       int
	totalTermFreq int64
	blockUpto     int
	doc           int
	accum         int64

	docTermStartFP int64
	skipOffset     int64
	nextSkipDoc    int

	needsFreq     bool
	isFreqsRead   bool
	singletonDocID int
}

func newBlockDocsEnum99(fieldInfo *index.FieldInfo, reader *Lucene99PostingsReader) *blockDocsEnum99 {
	e := &blockDocsEnum99{
		reader:     reader,
		startDocIn: reader.docIn,
		indexHasFreq:   fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs,
		indexHasPos:    fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions,
		indexHasOffsets: fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
		indexHasPayloads: fieldInfo.HasPayloads(),
	}
	e.docBuffer[lucene99BlockSize] = lucene99NoMoreDocs
	return e
}

func (e *blockDocsEnum99) canReuse(docIn store.IndexInput, fieldInfo *index.FieldInfo) bool {
	return docIn == e.startDocIn &&
		e.indexHasFreq == (fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs) &&
		e.indexHasPos == (fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions) &&
		e.indexHasPayloads == fieldInfo.HasPayloads()
}

func (e *blockDocsEnum99) reset(termState *IntBlockTermState, flags int) (index.PostingsEnum, error) {
	e.docFreq = termState.DocFreq
	e.totalTermFreq = termState.TotalTermFreq
	if !e.indexHasFreq {
		e.totalTermFreq = int64(termState.DocFreq)
	}
	e.docTermStartFP = termState.DocStartFP
	e.skipOffset = termState.SkipOffset
	e.singletonDocID = termState.SingletonDocID

	if e.docFreq > 1 {
		if e.docIn == nil {
			e.docIn = e.startDocIn.Clone()
		}
		if err := e.reader.seekAndPrefetchPostings(e.docIn, termState); err != nil {
			return nil, err
		}
	}

	e.doc = -1
	e.needsFreq = e.indexHasFreq && (flags&index.PostingsFlagFreqs) != 0
	e.isFreqsRead = true
	if !e.indexHasFreq || !e.needsFreq {
		n := lucene99BlockSize
		if e.docFreq < n {
			n = e.docFreq
		}
		for i := 0; i < n; i++ {
			e.freqBuffer[i] = 1
		}
	}
	e.accum = 0
	e.blockUpto = 0
	e.nextSkipDoc = lucene99BlockSize - 1
	e.docBufferUpto = lucene99BlockSize
	e.skipped = false
	e.prefetchedSkipData = false
	return e, nil
}

// ─── PostingsEnum methods for blockDocsEnum99 ─────────────────────────────────────

func (e *blockDocsEnum99) DocID() int { return e.doc }

func (e *blockDocsEnum99) Cost() int64 { return int64(e.docFreq) }

func (e *blockDocsEnum99) Freq() (int, error) {
	if !e.isFreqsRead {
		if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
			return 0, err
		}
		e.isFreqsRead = true
	}
	return int(e.freqBuffer[e.docBufferUpto-1]), nil
}

func (e *blockDocsEnum99) NextPosition() (int, error) { return -1, nil }
func (e *blockDocsEnum99) StartOffset() (int, error)  { return -1, nil }
func (e *blockDocsEnum99) EndOffset() (int, error)    { return -1, nil }
func (e *blockDocsEnum99) GetPayload() ([]byte, error) { return nil, nil }

func (e *blockDocsEnum99) refillDocs() error {
	if !e.isFreqsRead {
		if err := e.pforUtil.skip(e.docIn); err != nil {
			return err
		}
		e.isFreqsRead = true
	}

	left := e.docFreq - e.blockUpto
	if left < 0 {
		left = 0
	}

	if left >= lucene99BlockSize {
		if e.forUtil == nil {
			e.forUtil = newLucene99ForUtil()
		}
		if e.forDeltaUtil == nil {
			e.forDeltaUtil = newLucene99ForDeltaUtil(e.forUtil)
		}
		if e.pforUtil == nil && (e.indexHasFreq && e.needsFreq) {
			e.pforUtil = newLucene99PForUtil(e.forUtil)
		}

		if err := e.forDeltaUtil.decodeAndPrefixSum(e.docIn, e.accum, e.docBuffer[:]); err != nil {
			return err
		}

		if e.indexHasFreq {
			if e.needsFreq {
				e.isFreqsRead = false
			} else {
				if e.pforUtil == nil {
					e.pforUtil = newLucene99PForUtil(e.forUtil)
				}
				if err := e.pforUtil.skip(e.docIn); err != nil {
					return err
				}
			}
		}
		e.blockUpto += lucene99BlockSize
	} else if e.docFreq == 1 {
		e.docBuffer[0] = int64(e.singletonDocID)
		e.freqBuffer[0] = e.totalTermFreq
		e.docBuffer[1] = lucene99NoMoreDocs
		e.blockUpto++
	} else {
		if err := readLucene99VIntBlock(e.docIn, e.docBuffer[:], e.freqBuffer[:], left, e.indexHasFreq, e.needsFreq); err != nil {
			return err
		}
		prefixSum99(e.docBuffer[:], left, e.accum)
		e.docBuffer[left] = lucene99NoMoreDocs
		e.blockUpto += left
	}
	e.accum = e.docBuffer[lucene99BlockSize-1]
	e.docBufferUpto = 0
	return nil
}

func (e *blockDocsEnum99) NextDoc() (int, error) {
	if e.docBufferUpto == lucene99BlockSize {
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
	}
	e.doc = int(e.docBuffer[e.docBufferUpto])
	e.docBufferUpto++
	return e.doc, nil
}

func (e *blockDocsEnum99) Advance(target int) (int, error) {
	if e.docFreq > lucene99BlockSize {
		if target <= e.nextSkipDoc {
			if !e.prefetchedSkipData {
				e.reader.prefetchSkipData(e.docIn, e.docTermStartFP, e.skipOffset)
				e.prefetchedSkipData = true
			}
		} else {
			if e.skipper == nil {
				e.skipper = newLucene99SkipReader(
					e.docIn.Clone(), lucene99MaxSkipLevels,
					e.indexHasPos, e.indexHasOffsets, e.indexHasPayloads,
				)
			}
			if !e.skipped {
				if err := e.skipper.init(
					e.docTermStartFP+e.skipOffset, e.docTermStartFP, 0, 0, e.docFreq,
				); err != nil {
					return 0, err
				}
				e.skipped = true
			}

			newDocUpto, err := e.skipper.skipTo(target)
			if err != nil {
				return 0, err
			}
			newDocUpto++ // always +1 to fix the result

			if newDocUpto >= e.blockUpto {
				e.blockUpto = newDocUpto
				e.docBufferUpto = lucene99BlockSize
				e.accum = int64(e.skipper.getDoc())
				if err := e.docIn.SetPosition(e.skipper.getDocPointer()); err != nil {
					return 0, err
				}
				e.isFreqsRead = true
			}
			e.nextSkipDoc = e.skipper.getNextSkipDoc()
		}
	}

	if e.docBufferUpto == lucene99BlockSize {
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
	}

	for {
		doc := e.docBuffer[e.docBufferUpto]
		if doc >= int64(target) {
			e.docBufferUpto++
			e.doc = int(doc)
			return e.doc, nil
		}
		e.docBufferUpto++
	}
}

// ─── everythingEnum99 ─────────────────────────────────────────────────────────────

// everythingEnum99 iterates over docs with full position/payload/offset
// decoding.  Mirrors Lucene99PostingsReader.EverythingEnum.
type everythingEnum99 struct {
	reader       *Lucene99PostingsReader
	forUtil      *lucene99ForUtil
	forDeltaUtil *lucene99ForDeltaUtil
	pforUtil     *lucene99PForUtil

	docBuffer  [lucene99BlockSize + 1]int64
	freqBuffer [lucene99BlockSize]int64
	posDeltaBuffer [lucene99BlockSize]int64

	payloadLengthBuffer    []int64
	offsetStartDeltaBuffer []int64
	offsetLengthBuffer     []int64

	payloadBytes    []byte
	payloadByteUpto int
	payloadLength   int

	lastStartOffset int
	startOffset     int
	endOffset       int

	docBufferUpto int
	posBufferUpto int

	skipper            *lucene99SkipReader
	skipped            bool
	prefetchedSkipData bool

	startDocIn store.IndexInput
	docIn      store.IndexInput
	posIn      store.IndexInput
	payIn      store.IndexInput
	payload    *util.BytesRef

	indexHasOffsets  bool
	indexHasPayloads bool

	docFreq        int
	totalTermFreq  int64
	blockUpto      int
	doc            int
	accum          int64
	freq           int
	position       int

	posPendingCount int
	posPendingFP    int64
	payPendingFP    int64

	docTermStartFP int64
	posTermStartFP int64
	payTermStartFP int64
	lastPosBlockFP int64
	skipOffset     int64
	nextSkipDoc     int

	needsOffsets  bool
	needsPayloads bool
	singletonDocID int
}

func newEverythingEnum99(fieldInfo *index.FieldInfo, reader *Lucene99PostingsReader) *everythingEnum99 {
	e := &everythingEnum99{
		reader:     reader,
		startDocIn: reader.docIn,
		indexHasOffsets:  fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
		indexHasPayloads: fieldInfo.HasPayloads(),
	}
	e.posIn = reader.posIn.Clone()
	if e.indexHasOffsets || e.indexHasPayloads {
		e.payIn = reader.payIn.Clone()
	}
	if e.indexHasOffsets {
		e.offsetStartDeltaBuffer = make([]int64, lucene99BlockSize)
		e.offsetLengthBuffer = make([]int64, lucene99BlockSize)
		e.startOffset = -1
		e.endOffset = -1
	}
	if e.indexHasPayloads {
		e.payloadLengthBuffer = make([]int64, lucene99BlockSize)
		e.payloadBytes = make([]byte, 128)
		e.payload = util.NewBytesRefEmpty()
	}
	e.docBuffer[lucene99BlockSize] = lucene99NoMoreDocs
	return e
}

func (e *everythingEnum99) canReuse(docIn store.IndexInput, fieldInfo *index.FieldInfo) bool {
	return docIn == e.startDocIn &&
		e.indexHasOffsets == (fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets) &&
		e.indexHasPayloads == fieldInfo.HasPayloads()
}

func (e *everythingEnum99) reset(termState *IntBlockTermState, flags int) (index.PostingsEnum, error) {
	e.docFreq = termState.DocFreq
	e.docTermStartFP = termState.DocStartFP
	e.posTermStartFP = termState.PosStartFP
	e.payTermStartFP = termState.PayStartFP
	e.skipOffset = termState.SkipOffset
	e.totalTermFreq = termState.TotalTermFreq
	e.singletonDocID = termState.SingletonDocID

	if e.docFreq > 1 {
		if e.docIn == nil {
			e.docIn = e.startDocIn.Clone()
		}
		if err := e.reader.seekAndPrefetchPostings(e.docIn, termState); err != nil {
			return nil, err
		}
	}

	e.posPendingFP = e.posTermStartFP
	e.payPendingFP = e.payTermStartFP
	e.posPendingCount = 0

	if e.totalTermFreq < int64(lucene99BlockSize) {
		e.lastPosBlockFP = e.posTermStartFP
	} else if e.totalTermFreq == int64(lucene99BlockSize) {
		e.lastPosBlockFP = -1
	} else {
		e.lastPosBlockFP = e.posTermStartFP + termState.LastPosBlockOffset
	}

	e.needsOffsets = (flags & index.PostingsFlagOffsets) != 0
	e.needsPayloads = (flags & index.PostingsFlagPayloads) != 0

	e.doc = -1
	e.accum = 0
	e.blockUpto = 0
	if e.docFreq > lucene99BlockSize {
		e.nextSkipDoc = lucene99BlockSize - 1
	} else {
		e.nextSkipDoc = lucene99NoMoreDocs
	}
	e.docBufferUpto = lucene99BlockSize
	e.skipped = false
	e.prefetchedSkipData = false
	return e, nil
}

// ─── PostingsEnum methods for everythingEnum99 ────────────────────────────────────

func (e *everythingEnum99) DocID() int { return e.doc }
func (e *everythingEnum99) Cost() int64 { return int64(e.docFreq) }
func (e *everythingEnum99) Freq() (int, error) { return e.freq, nil }

func (e *everythingEnum99) refillDocs() error {
	left := e.docFreq - e.blockUpto
	if left < 0 {
		left = 0
	}

	if left >= lucene99BlockSize {
		if e.forUtil == nil {
			e.forUtil = newLucene99ForUtil()
		}
		if e.forDeltaUtil == nil {
			e.forDeltaUtil = newLucene99ForDeltaUtil(e.forUtil)
		}
		if e.pforUtil == nil {
			e.pforUtil = newLucene99PForUtil(e.forUtil)
		}

		if err := e.forDeltaUtil.decodeAndPrefixSum(e.docIn, e.accum, e.docBuffer[:]); err != nil {
			return err
		}
		if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
			return err
		}
		e.blockUpto += lucene99BlockSize
	} else if e.docFreq == 1 {
		e.docBuffer[0] = int64(e.singletonDocID)
		e.freqBuffer[0] = e.totalTermFreq
		e.docBuffer[1] = lucene99NoMoreDocs
		e.blockUpto++
	} else {
		if err := readLucene99VIntBlock(e.docIn, e.docBuffer[:], e.freqBuffer[:], left, true, true); err != nil {
			return err
		}
		prefixSum99(e.docBuffer[:], left, e.accum)
		e.docBuffer[left] = lucene99NoMoreDocs
		e.blockUpto += left
	}
	e.accum = e.docBuffer[lucene99BlockSize-1]
	e.docBufferUpto = 0
	return nil
}

func (e *everythingEnum99) refillPositions() error {
	if e.posIn.GetFilePointer() == e.lastPosBlockFP {
		count := int(e.totalTermFreq % int64(lucene99BlockSize))
		var payloadLength, offsetLength int
		e.payloadByteUpto = 0
		for i := 0; i < count; i++ {
			code, err := store.ReadVInt(e.posIn)
			if err != nil {
				return err
			}
			if e.indexHasPayloads {
				if code&1 != 0 {
					pl, err2 := store.ReadVInt(e.posIn)
					if err2 != nil {
						return err2
					}
					payloadLength = int(pl)
				}
				e.payloadLengthBuffer[i] = int64(payloadLength)
				e.posDeltaBuffer[i] = int64(int64(code) >> 1)
				if payloadLength != 0 {
					need := e.payloadByteUpto + payloadLength
					if need > len(e.payloadBytes) {
						newBytes := make([]byte, need*2)
						copy(newBytes, e.payloadBytes)
						e.payloadBytes = newBytes
					}
					if err2 := e.posIn.ReadBytes(e.payloadBytes[e.payloadByteUpto : e.payloadByteUpto+payloadLength]); err2 != nil {
						return err2
					}
					e.payloadByteUpto += payloadLength
				}
			} else {
				e.posDeltaBuffer[i] = int64(int64(code))
			}

			if e.indexHasOffsets {
				deltaCode, err2 := store.ReadVInt(e.posIn)
				if err2 != nil {
					return err2
				}
				if deltaCode&1 != 0 {
					ol, err3 := store.ReadVInt(e.posIn)
					if err3 != nil {
						return err3
					}
					offsetLength = int(ol)
				}
				e.offsetStartDeltaBuffer[i] = int64(int64(deltaCode) >> 1)
				e.offsetLengthBuffer[i] = int64(offsetLength)
			}
		}
		e.payloadByteUpto = 0
	} else {
		if err := e.pforUtil.decode(e.posIn, e.posDeltaBuffer[:]); err != nil {
			return err
		}

		if e.indexHasPayloads {
			if e.needsPayloads {
				if err := e.pforUtil.decode(e.payIn, e.payloadLengthBuffer); err != nil {
					return err
				}
				numBytes, err2 := store.ReadVInt(e.payIn)
				if err2 != nil {
					return err2
				}
				n := int(numBytes)
				if n > len(e.payloadBytes) {
					e.payloadBytes = make([]byte, n*2)
				}
				if err3 := e.payIn.ReadBytes(e.payloadBytes[:n]); err3 != nil {
					return err3
				}
			} else {
				if err := e.pforUtil.skip(e.payIn); err != nil {
					return err
				}
				numBytes, err2 := store.ReadVInt(e.payIn)
				if err2 != nil {
					return err2
				}
				if err3 := skipBytesInput(e.payIn, int64(numBytes)); err3 != nil {
					return err3
				}
			}
			e.payloadByteUpto = 0
		}

		if e.indexHasOffsets {
			if e.needsOffsets {
				if err := e.pforUtil.decode(e.payIn, e.offsetStartDeltaBuffer); err != nil {
					return err
				}
				if err2 := e.pforUtil.decode(e.payIn, e.offsetLengthBuffer); err2 != nil {
					return err2
				}
			} else {
				if err := e.pforUtil.skip(e.payIn); err != nil {
					return err
				}
				if err2 := e.pforUtil.skip(e.payIn); err2 != nil {
					return err2
				}
			}
		}
	}
	return nil
}

func (e *everythingEnum99) NextDoc() (int, error) {
	if e.docBufferUpto == lucene99BlockSize {
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
	}
	e.doc = int(e.docBuffer[e.docBufferUpto])
	e.freq = int(e.freqBuffer[e.docBufferUpto])
	e.posPendingCount += e.freq
	e.docBufferUpto++
	e.position = 0
	e.lastStartOffset = 0
	return e.doc, nil
}

func (e *everythingEnum99) Advance(target int) (int, error) {
	if target > e.nextSkipDoc {
		if e.skipper == nil {
			e.skipper = newLucene99SkipReader(
				e.docIn.Clone(), lucene99MaxSkipLevels,
				true, e.indexHasOffsets, e.indexHasPayloads,
			)
		}
		if !e.skipped {
			if err := e.skipper.init(
				e.docTermStartFP+e.skipOffset, e.docTermStartFP,
				e.posTermStartFP, e.payTermStartFP, e.docFreq,
			); err != nil {
				return 0, err
			}
			e.skipped = true
		}

		newDocUpto, err := e.skipper.skipTo(target)
		if err != nil {
			return 0, err
		}
		newDocUpto++

		if newDocUpto > e.blockUpto-lucene99BlockSize+e.docBufferUpto {
			e.blockUpto = newDocUpto
			e.docBufferUpto = lucene99BlockSize
			e.accum = int64(e.skipper.getDoc())
			if err := e.docIn.SetPosition(e.skipper.getDocPointer()); err != nil {
				return 0, err
			}
			e.posPendingFP = e.skipper.getPosPointer()
			e.payPendingFP = e.skipper.getPayPointer()
			e.posPendingCount = e.skipper.getPosBufferUpto()
			e.lastStartOffset = 0
			e.payloadByteUpto = e.skipper.getPayloadByteUpto()
		}
		e.nextSkipDoc = e.skipper.getNextSkipDoc()
	} else {
		if !e.prefetchedSkipData {
			e.reader.prefetchSkipData(e.docIn, e.docTermStartFP, e.skipOffset)
			e.prefetchedSkipData = true
		}
	}

	if e.docBufferUpto == lucene99BlockSize {
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
	}

	for {
		doc := e.docBuffer[e.docBufferUpto]
		e.freq = int(e.freqBuffer[e.docBufferUpto])
		e.posPendingCount += e.freq
		e.docBufferUpto++
		if doc >= int64(target) {
			e.position = 0
			e.lastStartOffset = 0
			e.doc = int(doc)
			return e.doc, nil
		}
	}
}

func (e *everythingEnum99) skipPositions() error {
	toSkip := e.posPendingCount - e.freq
	leftInBlock := lucene99BlockSize - e.posBufferUpto
	if toSkip < leftInBlock {
		end := e.posBufferUpto + toSkip
		for i := e.posBufferUpto; i < end; i++ {
			if e.indexHasPayloads {
				e.payloadByteUpto += int(e.payloadLengthBuffer[i])
			}
		}
		e.posBufferUpto = end
	} else {
		toSkip -= leftInBlock
		for toSkip >= lucene99BlockSize {
			if err := e.pforUtil.skip(e.posIn); err != nil {
				return err
			}
			if e.indexHasPayloads && e.payIn != nil {
				if err := e.pforUtil.skip(e.payIn); err != nil {
					return err
				}
				numBytes, err2 := store.ReadVInt(e.payIn)
				if err2 != nil {
					return err2
				}
				if err3 := skipBytesInput(e.payIn, int64(numBytes)); err3 != nil {
					return err3
				}
			}
			if e.indexHasOffsets && e.payIn != nil {
				if err := e.pforUtil.skip(e.payIn); err != nil {
					return err
				}
				if err2 := e.pforUtil.skip(e.payIn); err2 != nil {
					return err2
				}
			}
			toSkip -= lucene99BlockSize
		}
		if err := e.refillPositions(); err != nil {
			return err
		}
		e.payloadByteUpto = 0
		e.posBufferUpto = 0
		for i := 0; i < toSkip; i++ {
			if e.indexHasPayloads {
				e.payloadByteUpto += int(e.payloadLengthBuffer[i])
			}
		}
		e.posBufferUpto = toSkip
	}
	e.position = 0
	e.lastStartOffset = 0
	return nil
}

func (e *everythingEnum99) NextPosition() (int, error) {
	if e.posPendingFP != -1 {
		if err := e.posIn.SetPosition(e.posPendingFP); err != nil {
			return 0, err
		}
		e.posPendingFP = -1
		if e.payPendingFP != -1 && e.payIn != nil {
			if err := e.payIn.SetPosition(e.payPendingFP); err != nil {
				return 0, err
			}
			e.payPendingFP = -1
		}
		e.posBufferUpto = lucene99BlockSize
	}

	if e.posPendingCount > e.freq {
		if err := e.skipPositions(); err != nil {
			return 0, err
		}
		e.posPendingCount = e.freq
	}

	if e.posBufferUpto == lucene99BlockSize {
		if err := e.refillPositions(); err != nil {
			return 0, err
		}
		e.posBufferUpto = 0
	}

	e.position += int(e.posDeltaBuffer[e.posBufferUpto])

	if e.indexHasPayloads {
		e.payloadLength = int(e.payloadLengthBuffer[e.posBufferUpto])
		e.payload.Bytes = e.payloadBytes
		e.payload.Offset = e.payloadByteUpto
		e.payload.Length = e.payloadLength
		e.payloadByteUpto += e.payloadLength
	}

	if e.indexHasOffsets {
		e.startOffset = e.lastStartOffset + int(e.offsetStartDeltaBuffer[e.posBufferUpto])
		e.endOffset = e.startOffset + int(e.offsetLengthBuffer[e.posBufferUpto])
		e.lastStartOffset = e.startOffset
	}

	e.posBufferUpto++
	e.posPendingCount--
	return e.position, nil
}

func (e *everythingEnum99) StartOffset() (int, error) { return e.startOffset, nil }
func (e *everythingEnum99) EndOffset() (int, error)   { return e.endOffset, nil }

func (e *everythingEnum99) GetPayload() ([]byte, error) {
	if !e.needsPayloads || e.payloadLength == 0 {
		return nil, nil
	}
	start := e.payloadByteUpto - e.payloadLength
	if start < 0 {
		start = 0
	}
	return e.payloadBytes[start : start+e.payloadLength], nil
}

// ─── BlockImpactsDocsEnum99 ──────────────────────────────────────────────────────

// BlockImpactsDocsEnum99 provides impact-aware iteration over docs (no positions).
// Mirrors Lucene99PostingsReader.BlockImpactsDocsEnum.
type blockImpactsDocsEnum99 struct {
	forUtil     *lucene99ForUtil
	forDeltaUtil *lucene99ForDeltaUtil
	pforUtil    *lucene99PForUtil

	docBuffer  [lucene99BlockSize + 1]int64
	freqBuffer [lucene99BlockSize]int64

	docBufferUpto int

	skipper *lucene99ScoreSkipReader

	docIn store.IndexInput

	indexHasFreq bool

	docFreq    int
	blockUpto  int
	doc        int
	accum      int64
	nextSkipDoc int

	isFreqsRead bool
}

func newBlockImpactsDocsEnum99(
	fieldInfo *index.FieldInfo, termState *IntBlockTermState, reader *Lucene99PostingsReader,
) (*blockImpactsDocsEnum99, error) {
	e := &blockImpactsDocsEnum99{
		indexHasFreq: fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs,
	}

	e.docIn = reader.docIn.Clone()
	e.docFreq = termState.DocFreq

	if err := reader.seekAndPrefetchPostings(e.docIn, termState); err != nil {
		return nil, err
	}
	reader.prefetchSkipData(e.docIn, termState.DocStartFP, termState.SkipOffset)

	e.doc = -1
	e.accum = 0
	e.blockUpto = 0
	e.docBufferUpto = lucene99BlockSize

	e.skipper = newLucene99ScoreSkipReader(
		e.docIn.Clone(), lucene99MaxSkipLevels,
		fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions,
		fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
		fieldInfo.HasPayloads(),
	)
	if err := e.skipper.init(
		termState.DocStartFP+termState.SkipOffset,
		termState.DocStartFP, termState.PosStartFP, termState.PayStartFP,
		e.docFreq,
	); err != nil {
		return nil, err
	}

	e.docBuffer[lucene99BlockSize] = lucene99NoMoreDocs
	e.isFreqsRead = true
	if !e.indexHasFreq {
		for i := 0; i < lucene99BlockSize; i++ {
			e.freqBuffer[i] = 1
		}
	}

	return e, nil
}

func (e *blockImpactsDocsEnum99) DocID() int                         { return e.doc }
func (e *blockImpactsDocsEnum99) Cost() int64                        { return int64(e.docFreq) }
func (e *blockImpactsDocsEnum99) NextPosition() (int, error)         { return -1, nil }
func (e *blockImpactsDocsEnum99) StartOffset() (int, error)          { return -1, nil }
func (e *blockImpactsDocsEnum99) EndOffset() (int, error)            { return -1, nil }
func (e *blockImpactsDocsEnum99) GetPayload() ([]byte, error)        { return nil, nil }

func (e *blockImpactsDocsEnum99) Freq() (int, error) {
	if !e.isFreqsRead {
		if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
			return 0, err
		}
		e.isFreqsRead = true
	}
	return int(e.freqBuffer[e.docBufferUpto-1]), nil
}

func (e *blockImpactsDocsEnum99) refillDocs() error {
	if !e.isFreqsRead {
		if err := e.pforUtil.skip(e.docIn); err != nil {
			return err
		}
		e.isFreqsRead = true
	}

	left := e.docFreq - e.blockUpto
	if left < 0 {
		left = 0
	}

	if left >= lucene99BlockSize {
		if e.forUtil == nil {
			e.forUtil = newLucene99ForUtil()
		}
		if e.forDeltaUtil == nil {
			e.forDeltaUtil = newLucene99ForDeltaUtil(e.forUtil)
		}
		if e.pforUtil == nil && e.indexHasFreq {
			e.pforUtil = newLucene99PForUtil(e.forUtil)
		}

		if err := e.forDeltaUtil.decodeAndPrefixSum(e.docIn, e.accum, e.docBuffer[:]); err != nil {
			return err
		}
		if e.indexHasFreq {
			e.isFreqsRead = false
		}
		e.blockUpto += lucene99BlockSize
	} else {
		if err := readLucene99VIntBlock(e.docIn, e.docBuffer[:], e.freqBuffer[:], left, e.indexHasFreq, true); err != nil {
			return err
		}
		prefixSum99(e.docBuffer[:], left, e.accum)
		e.docBuffer[left] = lucene99NoMoreDocs
		e.blockUpto += left
	}
	e.accum = e.docBuffer[lucene99BlockSize-1]
	e.docBufferUpto = 0
	return nil
}

func (e *blockImpactsDocsEnum99) NextDoc() (int, error) {
	if e.docBufferUpto == lucene99BlockSize {
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
	}
	e.doc = int(e.docBuffer[e.docBufferUpto])
	e.docBufferUpto++
	return e.doc, nil
}

func (e *blockImpactsDocsEnum99) Advance(target int) (int, error) {
	if target > e.nextSkipDoc {
		if err := e.advanceShallow(target); err != nil {
			return 0, err
		}
	}
	if e.docBufferUpto == lucene99BlockSize {
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
	}
	next := findFirstGreater99(e.docBuffer[:], int64(target), e.docBufferUpto)
	e.doc = int(e.docBuffer[next])
	e.docBufferUpto = next + 1
	return e.doc, nil
}

func (e *blockImpactsDocsEnum99) advanceShallow(target int) error {
	if target > e.nextSkipDoc {
		newDocUpto, err := e.skipper.skipTo(target)
		if err != nil {
			return err
		}
		newDocUpto++

		if newDocUpto >= e.blockUpto {
			e.blockUpto = newDocUpto
			e.docBufferUpto = lucene99BlockSize
			e.accum = int64(e.skipper.getDoc())
			if err := e.docIn.SetPosition(e.skipper.getDocPointer()); err != nil {
				return err
			}
			e.isFreqsRead = true
		}
		e.nextSkipDoc = e.skipper.getNextSkipDoc()
	}
	return nil
}

func (e *blockImpactsDocsEnum99) AdvanceShallow(target int) error {
	return e.advanceShallow(target)
}

func (e *blockImpactsDocsEnum99) GetImpacts() (index.Impacts, error) {
	if err := e.advanceShallow(e.doc); err != nil {
		return nil, err
	}
	return e.skipper.getImpacts(), nil
}

// ─── BlockImpactsPostingsEnum99 ──────────────────────────────────────────────────

// BlockImpactsPostingsEnum99 provides impact-aware iteration with positions
// (no payloads/offsets). Mirrors Lucene99PostingsReader.BlockImpactsPostingsEnum.
type blockImpactsPostingsEnum99 struct {
	forUtil      *lucene99ForUtil
	forDeltaUtil *lucene99ForDeltaUtil
	pforUtil     *lucene99PForUtil

	docBuffer      [lucene99BlockSize]int64
	freqBuffer     [lucene99BlockSize]int64
	posDeltaBuffer [lucene99BlockSize]int64

	docBufferUpto int
	posBufferUpto int

	skipper *lucene99ScoreSkipReader

	docIn store.IndexInput
	posIn store.IndexInput

	indexHasOffsets  bool
	indexHasPayloads bool

	docFreq       int
	totalTermFreq int64
	docUpto       int
	doc           int
	accum         int64
	freq          int
	position      int

	posPendingCount int
	posPendingFP    int64

	docTermStartFP int64
	posTermStartFP int64
	payTermStartFP int64
	lastPosBlockFP int64
	nextSkipDoc    int
}

func newBlockImpactsPostingsEnum99(
	fieldInfo *index.FieldInfo, termState *IntBlockTermState, reader *Lucene99PostingsReader,
) (*blockImpactsPostingsEnum99, error) {
	e := &blockImpactsPostingsEnum99{
		indexHasOffsets:  fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
		indexHasPayloads: fieldInfo.HasPayloads(),
	}

	e.docIn = reader.docIn.Clone()
	e.posIn = reader.posIn.Clone()

	e.docFreq = termState.DocFreq
	e.docTermStartFP = termState.DocStartFP
	e.posTermStartFP = termState.PosStartFP
	e.payTermStartFP = termState.PayStartFP
	e.totalTermFreq = termState.TotalTermFreq

	if err := reader.seekAndPrefetchPostings(e.docIn, termState); err != nil {
		return nil, err
	}
	reader.prefetchSkipData(e.docIn, termState.DocStartFP, termState.SkipOffset)

	e.posPendingFP = e.posTermStartFP
	e.posPendingCount = 0

	if e.totalTermFreq < int64(lucene99BlockSize) {
		e.lastPosBlockFP = e.posTermStartFP
	} else if e.totalTermFreq == int64(lucene99BlockSize) {
		e.lastPosBlockFP = -1
	} else {
		e.lastPosBlockFP = e.posTermStartFP + termState.LastPosBlockOffset
	}

	e.doc = -1
	e.accum = 0
	e.docUpto = 0
	e.docBufferUpto = lucene99BlockSize

	e.skipper = newLucene99ScoreSkipReader(
		e.docIn.Clone(), lucene99MaxSkipLevels, true,
		e.indexHasOffsets, e.indexHasPayloads,
	)
	if err := e.skipper.init(
		e.docTermStartFP+termState.SkipOffset,
		e.docTermStartFP, e.posTermStartFP, e.payTermStartFP,
		e.docFreq,
	); err != nil {
		return nil, err
	}

	return e, nil
}

func (e *blockImpactsPostingsEnum99) DocID() int                  { return e.doc }
func (e *blockImpactsPostingsEnum99) Cost() int64                 { return int64(e.docFreq) }
func (e *blockImpactsPostingsEnum99) Freq() (int, error)          { return e.freq, nil }
func (e *blockImpactsPostingsEnum99) StartOffset() (int, error)   { return -1, nil }
func (e *blockImpactsPostingsEnum99) EndOffset() (int, error)     { return -1, nil }
func (e *blockImpactsPostingsEnum99) GetPayload() ([]byte, error) { return nil, nil }

func (e *blockImpactsPostingsEnum99) refillDocs() error {
	left := e.docFreq - e.docUpto
	if left < 0 {
		left = 0
	}

	if left >= lucene99BlockSize {
		if e.forUtil == nil {
			e.forUtil = newLucene99ForUtil()
		}
		if e.forDeltaUtil == nil {
			e.forDeltaUtil = newLucene99ForDeltaUtil(e.forUtil)
		}
		if e.pforUtil == nil {
			e.pforUtil = newLucene99PForUtil(e.forUtil)
		}

		if err := e.forDeltaUtil.decodeAndPrefixSum(e.docIn, e.accum, e.docBuffer[:]); err != nil {
			return err
		}
		if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
			return err
		}
	} else {
		if err := readLucene99VIntBlock(e.docIn, e.docBuffer[:], e.freqBuffer[:], left, true, true); err != nil {
			return err
		}
		prefixSum99(e.docBuffer[:], left, e.accum)
		e.docBuffer[left] = lucene99NoMoreDocs
	}
	e.accum = e.docBuffer[lucene99BlockSize-1]
	e.docBufferUpto = 0
	return nil
}

func (e *blockImpactsPostingsEnum99) refillPositions() error {
	if e.posIn.GetFilePointer() == e.lastPosBlockFP {
		count := int(e.totalTermFreq % int64(lucene99BlockSize))
		var payloadLength int
		for i := 0; i < count; i++ {
			code, err := store.ReadVInt(e.posIn)
			if err != nil {
				return err
			}
			if e.indexHasPayloads {
				if code&1 != 0 {
					pl, err2 := store.ReadVInt(e.posIn)
					if err2 != nil {
						return err2
					}
					payloadLength = int(pl)
				}
				e.posDeltaBuffer[i] = int64(int64(code) >> 1)
				if payloadLength != 0 {
					if err2 := skipBytesInput(e.posIn, int64(payloadLength)); err2 != nil {
						return err2
					}
				}
			} else {
				e.posDeltaBuffer[i] = int64(int64(code))
			}
			if e.indexHasOffsets {
				if _, err2 := store.ReadVInt(e.posIn); err2 != nil {
					return err2
				}
				// Offset length change indicator consumed but not needed.
			}
		}
	} else {
		if err := e.pforUtil.decode(e.posIn, e.posDeltaBuffer[:]); err != nil {
			return err
		}
	}
	return nil
}

func (e *blockImpactsPostingsEnum99) NextDoc() (int, error) {
	return e.Advance(e.doc + 1)
}

func (e *blockImpactsPostingsEnum99) Advance(target int) (int, error) {
	if target > e.nextSkipDoc {
		if err := e.advanceShallow(target); err != nil {
			return 0, err
		}
	}
	if e.docBufferUpto == lucene99BlockSize {
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
	}

	next := findFirstGreater99(e.docBuffer[:], int64(target), e.docBufferUpto)
	if next == lucene99BlockSize {
		e.doc = lucene99NoMoreDocs
		return e.doc, nil
	}
	e.doc = int(e.docBuffer[next])
	e.freq = int(e.freqBuffer[next])
	for i := e.docBufferUpto; i <= next; i++ {
		e.posPendingCount += int(e.freqBuffer[i])
	}
	e.docUpto += next - e.docBufferUpto + 1
	e.docBufferUpto = next + 1
	e.position = 0
	return e.doc, nil
}

func (e *blockImpactsPostingsEnum99) skipPositions() error {
	toSkip := e.posPendingCount - e.freq
	leftInBlock := lucene99BlockSize - e.posBufferUpto
	if toSkip < leftInBlock {
		e.posBufferUpto += toSkip
	} else {
		toSkip -= leftInBlock
		for toSkip >= lucene99BlockSize {
			if err := e.pforUtil.skip(e.posIn); err != nil {
				return err
			}
			toSkip -= lucene99BlockSize
		}
		if err := e.refillPositions(); err != nil {
			return err
		}
		e.posBufferUpto = toSkip
	}
	e.position = 0
	return nil
}

func (e *blockImpactsPostingsEnum99) NextPosition() (int, error) {
	if e.posPendingFP != -1 {
		if err := e.posIn.SetPosition(e.posPendingFP); err != nil {
			return 0, err
		}
		e.posPendingFP = -1
		e.posBufferUpto = lucene99BlockSize
	}

	if e.posPendingCount > e.freq {
		if err := e.skipPositions(); err != nil {
			return 0, err
		}
		e.posPendingCount = e.freq
	}

	if e.posBufferUpto == lucene99BlockSize {
		if err := e.refillPositions(); err != nil {
			return 0, err
		}
		e.posBufferUpto = 0
	}

	e.position += int(e.posDeltaBuffer[e.posBufferUpto])
	e.posBufferUpto++
	e.posPendingCount--
	return e.position, nil
}

func (e *blockImpactsPostingsEnum99) advanceShallow(target int) error {
	if target > e.nextSkipDoc {
		newDocUpto, err := e.skipper.skipTo(target)
		if err != nil {
			return err
		}
		newDocUpto++

		if newDocUpto > e.docUpto {
			e.docUpto = newDocUpto
			e.docBufferUpto = lucene99BlockSize
			e.accum = int64(e.skipper.getDoc())
			e.posPendingFP = e.skipper.getPosPointer()
			e.posPendingCount = e.skipper.getPosBufferUpto()
			if err := e.docIn.SetPosition(e.skipper.getDocPointer()); err != nil {
				return err
			}
		}
		e.nextSkipDoc = e.skipper.getNextSkipDoc()
	}
	return nil
}

func (e *blockImpactsPostingsEnum99) AdvanceShallow(target int) error {
	return e.advanceShallow(target)
}

func (e *blockImpactsPostingsEnum99) GetImpacts() (index.Impacts, error) {
	if err := e.advanceShallow(e.doc); err != nil {
		return nil, err
	}
	return e.skipper.getImpacts(), nil
}

// ─── BlockImpactsEverythingEnum99 ────────────────────────────────────────────────

// BlockImpactsEverythingEnum99 provides impact-aware iteration with full
// position/payload/offset support. Mirrors
// Lucene99PostingsReader.BlockImpactsEverythingEnum.
type blockImpactsEverythingEnum99 struct {
	forUtil      *lucene99ForUtil
	forDeltaUtil *lucene99ForDeltaUtil
	pforUtil     *lucene99PForUtil

	docBuffer      [lucene99BlockSize]int64
	freqBuffer     [lucene99BlockSize]int64
	posDeltaBuffer [lucene99BlockSize]int64

	payloadLengthBuffer    []int64
	offsetStartDeltaBuffer []int64
	offsetLengthBuffer     []int64

	payloadBytes    []byte
	payloadByteUpto int
	payloadLength   int

	lastStartOffset int
	startOffset     int
	endOffset       int

	docBufferUpto int
	posBufferUpto int

	skipper *lucene99ScoreSkipReader

	docIn store.IndexInput
	posIn store.IndexInput
	payIn store.IndexInput
	payload *util.BytesRef

	indexHasFreq     bool
	indexHasPos      bool
	indexHasOffsets  bool
	indexHasPayloads bool

	docFreq       int
	totalTermFreq int64
	docUpto       int
	posDocUpTo    int
	doc           int
	accum         int64
	position      int

	posPendingCount int
	posPendingFP    int64
	payPendingFP    int64

	docTermStartFP int64
	posTermStartFP int64
	payTermStartFP int64
	lastPosBlockFP int64
	nextSkipDoc    int

	needsPositions bool
	needsOffsets   bool
	needsPayloads  bool

	isFreqsRead bool
	seekTo      int64
}

func newBlockImpactsEverythingEnum99(
	fieldInfo *index.FieldInfo, termState *IntBlockTermState,
	reader *Lucene99PostingsReader, flags int,
) (*blockImpactsEverythingEnum99, error) {
	e := &blockImpactsEverythingEnum99{
		indexHasFreq:     fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs,
		indexHasPos:      fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions,
		indexHasOffsets:  fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
		indexHasPayloads: fieldInfo.HasPayloads(),
		needsPositions:   (flags & index.PostingsFlagPositions) != 0,
		needsOffsets:     (flags & index.PostingsFlagOffsets) != 0,
		needsPayloads:    (flags & index.PostingsFlagPayloads) != 0,
		seekTo:           -1,
	}

	e.docIn = reader.docIn.Clone()

	if e.indexHasPos && e.needsPositions {
		e.posIn = reader.posIn.Clone()
	}

	if (e.indexHasOffsets && e.needsOffsets) || (e.indexHasPayloads && e.needsPayloads) {
		e.payIn = reader.payIn.Clone()
	}

	if e.indexHasOffsets {
		e.offsetStartDeltaBuffer = make([]int64, lucene99BlockSize)
		e.offsetLengthBuffer = make([]int64, lucene99BlockSize)
		e.startOffset = -1
		e.endOffset = -1
	}

	if e.indexHasPayloads {
		e.payloadLengthBuffer = make([]int64, lucene99BlockSize)
		e.payloadBytes = make([]byte, 128)
		e.payload = util.NewBytesRefEmpty()
	}

	e.docFreq = termState.DocFreq
	e.docTermStartFP = termState.DocStartFP
	e.posTermStartFP = termState.PosStartFP
	e.payTermStartFP = termState.PayStartFP
	e.totalTermFreq = termState.TotalTermFreq

	if err := reader.seekAndPrefetchPostings(e.docIn, termState); err != nil {
		return nil, err
	}
	reader.prefetchSkipData(e.docIn, termState.DocStartFP, termState.SkipOffset)

	e.posPendingFP = e.posTermStartFP
	e.payPendingFP = e.payTermStartFP
	e.posPendingCount = 0

	if e.totalTermFreq < int64(lucene99BlockSize) {
		e.lastPosBlockFP = e.posTermStartFP
	} else if e.totalTermFreq == int64(lucene99BlockSize) {
		e.lastPosBlockFP = -1
	} else {
		e.lastPosBlockFP = e.posTermStartFP + termState.LastPosBlockOffset
	}

	e.doc = -1
	e.accum = 0
	e.docUpto = 0
	e.posDocUpTo = 0
	e.isFreqsRead = true
	e.docBufferUpto = lucene99BlockSize

	e.skipper = newLucene99ScoreSkipReader(
		e.docIn.Clone(), lucene99MaxSkipLevels,
		e.indexHasPos, e.indexHasOffsets, e.indexHasPayloads,
	)
	if err := e.skipper.init(
		e.docTermStartFP+termState.SkipOffset,
		e.docTermStartFP, e.posTermStartFP, e.payTermStartFP,
		e.docFreq,
	); err != nil {
		return nil, err
	}

	if !e.indexHasFreq {
		for i := 0; i < lucene99BlockSize; i++ {
			e.freqBuffer[i] = 1
		}
	}

	return e, nil
}

func (e *blockImpactsEverythingEnum99) DocID() int  { return e.doc }
func (e *blockImpactsEverythingEnum99) Cost() int64 { return int64(e.docFreq) }

func (e *blockImpactsEverythingEnum99) Freq() (int, error) {
	if e.indexHasFreq && !e.isFreqsRead {
		if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
			return 0, err
		}
		e.isFreqsRead = true
	}
	return int(e.freqBuffer[e.docBufferUpto-1]), nil
}

func (e *blockImpactsEverythingEnum99) refillDocs() error {
	if e.indexHasFreq {
		if !e.isFreqsRead {
			if e.indexHasPos && e.needsPositions && e.posDocUpTo < e.docUpto {
				if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
					return err
				}
			} else {
				if err := e.pforUtil.skip(e.docIn); err != nil {
					return err
				}
			}
			e.isFreqsRead = true
		}
		if e.indexHasPos && e.needsPositions {
			for e.posDocUpTo < e.docUpto {
				e.posPendingCount += int(e.freqBuffer[e.docBufferUpto-(e.docUpto-e.posDocUpTo)])
				e.posDocUpTo++
			}
		}
	}

	left := e.docFreq - e.docUpto
	if left < 0 {
		left = 0
	}

	if left >= lucene99BlockSize {
		if e.forUtil == nil {
			e.forUtil = newLucene99ForUtil()
		}
		if e.forDeltaUtil == nil {
			e.forDeltaUtil = newLucene99ForDeltaUtil(e.forUtil)
		}
		if e.pforUtil == nil {
			e.pforUtil = newLucene99PForUtil(e.forUtil)
		}

		if err := e.forDeltaUtil.decodeAndPrefixSum(e.docIn, e.accum, e.docBuffer[:]); err != nil {
			return err
		}
		if e.indexHasFreq {
			e.isFreqsRead = false
		}
	} else {
		if err := readLucene99VIntBlock(e.docIn, e.docBuffer[:], e.freqBuffer[:], left, e.indexHasFreq, true); err != nil {
			return err
		}
		prefixSum99(e.docBuffer[:], left, e.accum)
		e.docBuffer[left] = lucene99NoMoreDocs
	}
	e.accum = e.docBuffer[lucene99BlockSize-1]
	e.docBufferUpto = 0
	return nil
}

func (e *blockImpactsEverythingEnum99) refillPositions() error {
	if e.posIn.GetFilePointer() == e.lastPosBlockFP {
		count := int(e.totalTermFreq % int64(lucene99BlockSize))
		var payloadLength, offsetLength int
		e.payloadByteUpto = 0
		for i := 0; i < count; i++ {
			code, err := store.ReadVInt(e.posIn)
			if err != nil {
				return err
			}
			if e.indexHasPayloads {
				if code&1 != 0 {
					pl, err2 := store.ReadVInt(e.posIn)
					if err2 != nil {
						return err2
					}
					payloadLength = int(pl)
				}
				e.payloadLengthBuffer[i] = int64(payloadLength)
				e.posDeltaBuffer[i] = int64(int64(code) >> 1)
				if payloadLength != 0 {
					need := e.payloadByteUpto + payloadLength
					if need > len(e.payloadBytes) {
						e.payloadBytes = make([]byte, need*2)
					}
					if err2 := e.posIn.ReadBytes(e.payloadBytes[e.payloadByteUpto : e.payloadByteUpto+payloadLength]); err2 != nil {
						return err2
					}
					e.payloadByteUpto += payloadLength
				}
			} else {
				e.posDeltaBuffer[i] = int64(int64(code))
			}
			if e.indexHasOffsets {
				deltaCode, err2 := store.ReadVInt(e.posIn)
				if err2 != nil {
					return err2
				}
				if deltaCode&1 != 0 {
					ol, err3 := store.ReadVInt(e.posIn)
					if err3 != nil {
						return err3
					}
					offsetLength = int(ol)
				}
				e.offsetStartDeltaBuffer[i] = int64(int64(deltaCode) >> 1)
				e.offsetLengthBuffer[i] = int64(offsetLength)
			}
		}
		e.payloadByteUpto = 0
	} else {
		if err := e.pforUtil.decode(e.posIn, e.posDeltaBuffer[:]); err != nil {
			return err
		}

		if e.indexHasPayloads && e.payIn != nil {
			if e.needsPayloads {
				if err := e.pforUtil.decode(e.payIn, e.payloadLengthBuffer); err != nil {
					return err
				}
				numBytes, err2 := store.ReadVInt(e.payIn)
				if err2 != nil {
					return err2
				}
				n := int(numBytes)
				if n > len(e.payloadBytes) {
					e.payloadBytes = make([]byte, n*2)
				}
				if err3 := e.payIn.ReadBytes(e.payloadBytes[:n]); err3 != nil {
					return err3
				}
			} else {
				if err := e.pforUtil.skip(e.payIn); err != nil {
					return err
				}
				numBytes, err2 := store.ReadVInt(e.payIn)
				if err2 != nil {
					return err2
				}
				if err3 := skipBytesInput(e.payIn, int64(numBytes)); err3 != nil {
					return err3
				}
			}
			e.payloadByteUpto = 0
		}

		if e.indexHasOffsets && e.payIn != nil {
			if e.needsOffsets {
				if err := e.pforUtil.decode(e.payIn, e.offsetStartDeltaBuffer); err != nil {
					return err
				}
				if err2 := e.pforUtil.decode(e.payIn, e.offsetLengthBuffer); err2 != nil {
					return err2
				}
			} else {
				if err := e.pforUtil.skip(e.payIn); err != nil {
					return err
				}
				if err2 := e.pforUtil.skip(e.payIn); err2 != nil {
					return err2
				}
			}
		}
	}
	return nil
}

func (e *blockImpactsEverythingEnum99) NextDoc() (int, error) {
	return e.Advance(e.doc + 1)
}

func (e *blockImpactsEverythingEnum99) Advance(target int) (int, error) {
	if target > e.nextSkipDoc {
		if err := e.advanceShallow(target); err != nil {
			return 0, err
		}
	}
	if e.docBufferUpto == lucene99BlockSize {
		if e.seekTo >= 0 {
			if err := e.docIn.SetPosition(e.seekTo); err != nil {
				return 0, err
			}
			e.seekTo = -1
			e.isFreqsRead = true
		}
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
	}

	for {
		doc := e.docBuffer[e.docBufferUpto]
		e.docBufferUpto++
		e.docUpto++
		if doc >= int64(target) {
			e.position = 0
			e.lastStartOffset = 0
			e.doc = int(doc)
			return e.doc, nil
		}
		if e.docBufferUpto == lucene99BlockSize {
			e.doc = lucene99NoMoreDocs
			return e.doc, nil
		}
	}
}

func (e *blockImpactsEverythingEnum99) skipPositions() error {
	toSkip := e.posPendingCount - int(e.freqBuffer[e.docBufferUpto-1])
	leftInBlock := lucene99BlockSize - e.posBufferUpto
	if toSkip < leftInBlock {
		end := e.posBufferUpto + toSkip
		for i := e.posBufferUpto; i < end; i++ {
			if e.indexHasPayloads {
				e.payloadByteUpto += int(e.payloadLengthBuffer[i])
			}
		}
		e.posBufferUpto = end
	} else {
		toSkip -= leftInBlock
		for toSkip >= lucene99BlockSize {
			if err := e.pforUtil.skip(e.posIn); err != nil {
				return err
			}
			if e.indexHasPayloads && e.payIn != nil {
				if err := e.pforUtil.skip(e.payIn); err != nil {
					return err
				}
				numBytes, err2 := store.ReadVInt(e.payIn)
				if err2 != nil {
					return err2
				}
				if err3 := skipBytesInput(e.payIn, int64(numBytes)); err3 != nil {
					return err3
				}
			}
			if e.indexHasOffsets && e.payIn != nil {
				if err := e.pforUtil.skip(e.payIn); err != nil {
					return err
				}
				if err2 := e.pforUtil.skip(e.payIn); err2 != nil {
					return err2
				}
			}
			toSkip -= lucene99BlockSize
		}
		if err := e.refillPositions(); err != nil {
			return err
		}
		e.payloadByteUpto = 0
		e.posBufferUpto = 0
		for i := 0; i < toSkip; i++ {
			if e.indexHasPayloads {
				e.payloadByteUpto += int(e.payloadLengthBuffer[i])
			}
		}
		e.posBufferUpto = toSkip
	}
	e.position = 0
	e.lastStartOffset = 0
	return nil
}

func (e *blockImpactsEverythingEnum99) NextPosition() (int, error) {
	if !e.indexHasPos || !e.needsPositions {
		return -1, nil
	}

	if !e.isFreqsRead {
		if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
			return 0, err
		}
		e.isFreqsRead = true
	}
	for e.posDocUpTo < e.docUpto {
		e.posPendingCount += int(e.freqBuffer[e.docBufferUpto-(e.docUpto-e.posDocUpTo)])
		e.posDocUpTo++
	}

	if e.posPendingFP != -1 {
		if err := e.posIn.SetPosition(e.posPendingFP); err != nil {
			return 0, err
		}
		e.posPendingFP = -1
		if e.payPendingFP != -1 && e.payIn != nil {
			if err := e.payIn.SetPosition(e.payPendingFP); err != nil {
				return 0, err
			}
			e.payPendingFP = -1
		}
		e.posBufferUpto = lucene99BlockSize
	}

	if e.posPendingCount > int(e.freqBuffer[e.docBufferUpto-1]) {
		if err := e.skipPositions(); err != nil {
			return 0, err
		}
		e.posPendingCount = int(e.freqBuffer[e.docBufferUpto-1])
	}

	if e.posBufferUpto == lucene99BlockSize {
		if err := e.refillPositions(); err != nil {
			return 0, err
		}
		e.posBufferUpto = 0
	}

	e.position += int(e.posDeltaBuffer[e.posBufferUpto])

	if e.indexHasPayloads {
		e.payloadLength = int(e.payloadLengthBuffer[e.posBufferUpto])
		e.payload.Bytes = e.payloadBytes
		e.payload.Offset = e.payloadByteUpto
		e.payload.Length = e.payloadLength
		e.payloadByteUpto += e.payloadLength
	}

	if e.indexHasOffsets && e.needsOffsets {
		e.startOffset = e.lastStartOffset + int(e.offsetStartDeltaBuffer[e.posBufferUpto])
		e.endOffset = e.startOffset + int(e.offsetLengthBuffer[e.posBufferUpto])
		e.lastStartOffset = e.startOffset
	}

	e.posBufferUpto++
	e.posPendingCount--
	return e.position, nil
}

func (e *blockImpactsEverythingEnum99) StartOffset() (int, error) { return e.startOffset, nil }
func (e *blockImpactsEverythingEnum99) EndOffset() (int, error)   { return e.endOffset, nil }

func (e *blockImpactsEverythingEnum99) GetPayload() ([]byte, error) {
	if !e.needsPayloads || e.payloadLength == 0 {
		return nil, nil
	}
	start := e.payloadByteUpto - e.payloadLength
	if start < 0 {
		start = 0
	}
	return e.payloadBytes[start : start+e.payloadLength], nil
}

func (e *blockImpactsEverythingEnum99) advanceShallow(target int) error {
	if target > e.nextSkipDoc {
		newDocUpto, err := e.skipper.skipTo(target)
		if err != nil {
			return err
		}
		newDocUpto++

		if newDocUpto > e.docUpto {
			e.docUpto = newDocUpto
			e.posDocUpTo = e.docUpto
			e.docBufferUpto = lucene99BlockSize
			e.accum = int64(e.skipper.getDoc())
			e.posPendingFP = e.skipper.getPosPointer()
			e.payPendingFP = e.skipper.getPayPointer()
			e.posPendingCount = e.skipper.getPosBufferUpto()
			e.lastStartOffset = 0
			e.payloadByteUpto = e.skipper.getPayloadByteUpto()
			e.seekTo = e.skipper.getDocPointer()
		}
		e.nextSkipDoc = e.skipper.getNextSkipDoc()
	}
	return nil
}

func (e *blockImpactsEverythingEnum99) AdvanceShallow(target int) error {
	return e.advanceShallow(target)
}

func (e *blockImpactsEverythingEnum99) GetImpacts() (index.Impacts, error) {
	if err := e.advanceShallow(e.doc); err != nil {
		return nil, err
	}
	return e.skipper.getImpacts(), nil
}

// compile-time interface checks
var _ PostingsReaderBase = (*Lucene99PostingsReader)(nil)
var _ index.PostingsEnum = (*blockDocsEnum99)(nil)
var _ index.PostingsEnum = (*everythingEnum99)(nil)
var _ index.ImpactsEnum = (*blockImpactsDocsEnum99)(nil)
var _ index.ImpactsEnum = (*blockImpactsPostingsEnum99)(nil)
var _ index.ImpactsEnum = (*blockImpactsEverythingEnum99)(nil)
