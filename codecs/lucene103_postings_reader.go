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
//	lucene103/Lucene103PostingsReader.java
//
// Purpose: read-side decoder for the Lucene 10.3 postings format. Structurally
// this mirrors Lucene104PostingsReader (two-level skip, .psm/.doc/.pos/.pay
// layout, singletonDocID + lastPosBlockOffset term state). The differences,
// all driven by the Lucene 10.3 on-disk contract, are:
//   - BLOCK_SIZE = 128 (Lucene 10.4 uses 256); LEVEL1_NUM_DOCS = 32*128 = 4096.
//   - doc-delta blocks are decoded with lucene103ForDeltaUtil.decodeAndPrefixSum
//     (a fused FOR-decode + prefix-sum), whereas Lucene 10.4 decodes then
//     prefix-sums separately.
//   - the dense-bitset fast path in DocIDRunEnd checks 2 longs (128 bits)
//     rather than 4.
//
// The package-level helpers that are block-size independent (readVInt15,
// readVLong15, prefixSum64, sumOverRange64, findNextGEQ64, skipBytesInput,
// featureRequested, readVIntBlock104, readImpactsFromBytes, the deltaEncoding
// enum) are reused from lucene104_postings_reader.go.

package codecs

import (
	"fmt"
	"math"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Lucene103 postings-format constants.
const (
	lucene103MetaCodec  = "Lucene103PostingsWriterMeta"
	lucene103DocCodec   = "Lucene103PostingsWriterDoc"
	lucene103PosCodec   = "Lucene103PostingsWriterPos"
	lucene103PayCodec   = "Lucene103PostingsWriterPay"
	lucene103TermsCodec = "Lucene103PostingsWriterTerms"

	lucene103MetaExtension = "psm"
	lucene103DocExtension  = "doc"
	lucene103PosExtension  = "pos"
	lucene103PayExtension  = "pay"

	lucene103VersionStart   = 0
	lucene103VersionCurrent = lucene103VersionStart

	// lucene103PostingsBlockSize is the number of doc-deltas per packed block.
	// Lucene 10.3 uses ForUtil.BLOCK_SIZE = 128.
	lucene103PostingsBlockSize = lucene103BlockSizeConst // 128

	// lucene103Level1Factor controls how many blocks form a level-1 skip group.
	lucene103Level1Factor = 32
	// lucene103Level1NumDocs is the number of docs in one level-1 skip group.
	lucene103Level1NumDocs = lucene103Level1Factor * lucene103PostingsBlockSize // 4096
)

// lucene103Level1NoSkip is the internal "no level-1 skip data" sentinel.
// Java Lucene uses Integer.MAX_VALUE (= DocIdSetIterator.NO_MORE_DOCS). Gocene's
// public index.NO_MORE_DOCS is -1, which would make the level-1 skip check fire
// spuriously at the first NextDoc (doc == -1) and dereference a nil docIn for
// singleton terms. We mirror Java's MAX_VALUE internally and translate it to
// index.NO_MORE_DOCS only at the public Impacts boundary. See the equivalent
// lucene104Level1NoSkip for the full rationale.
const lucene103Level1NoSkip = math.MaxInt32

// lucene103NoMoreDocs is the internal "last (remainder) block" sentinel for the
// level0LastDocID skip field, mirroring Java's DocIdSetIterator.NO_MORE_DOCS.
const lucene103NoMoreDocs = math.MaxInt32

// Lucene103PostingsReader reads .doc / .pos / .pay files written by Apache
// Lucene 10.3's Lucene103PostingsWriter, validated against a .psm meta file.
//
// This is the Go port of
// org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsReader.
type Lucene103PostingsReader struct {
	docIn store.IndexInput
	posIn store.IndexInput // nil when segment has no positions
	payIn store.IndexInput // nil when segment has no payloads or offsets

	maxNumImpactsAtLevel0     int
	maxImpactNumBytesAtLevel0 int
	maxNumImpactsAtLevel1     int
	maxImpactNumBytesAtLevel1 int

	// stateCache bridges *BlockTermState handles (allocated by NewTermState)
	// back to the *IntBlockTermState that owns them. See Lucene104PostingsReader
	// for the same pattern and the concurrency rationale.
	stateCache map[*BlockTermState]*IntBlockTermState
}

// lookupOrCreateState returns the IntBlockTermState bridged to termState,
// creating and registering one on demand.
func (r *Lucene103PostingsReader) lookupOrCreateState(termState *BlockTermState) *IntBlockTermState {
	its := r.stateCache[termState]
	if its == nil {
		its = NewIntBlockTermState()
		its.BlockTermState = termState
		r.stateCache[termState] = its
	}
	return its
}

// NewLucene103PostingsReader opens and validates the .psm meta file, then opens
// .doc, and conditionally .pos / .pay.
//
// Mirrors Lucene103PostingsReader(SegmentReadState).
func NewLucene103PostingsReader(state *SegmentReadState) (*Lucene103PostingsReader, error) {
	metaName := GetSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene103MetaExtension)

	rawMeta, err := state.Directory.OpenInput(metaName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene103 postings reader: open meta %q: %w", metaName, err)
	}
	metaIn := store.NewChecksumIndexInput(rawMeta)

	version, err := CheckIndexHeader(
		metaIn,
		lucene103MetaCodec,
		int32(lucene103VersionStart),
		int32(lucene103VersionCurrent),
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene103 postings reader: check meta header: %w", err)
	}

	r := &Lucene103PostingsReader{
		stateCache: make(map[*BlockTermState]*IntBlockTermState),
	}

	var v int32
	var readErr error
	if v, readErr = metaIn.ReadInt(); readErr == nil {
		r.maxNumImpactsAtLevel0 = int(v)
	}
	if readErr == nil {
		if v, readErr = metaIn.ReadInt(); readErr == nil {
			r.maxImpactNumBytesAtLevel0 = int(v)
		}
	}
	if readErr == nil {
		if v, readErr = metaIn.ReadInt(); readErr == nil {
			r.maxNumImpactsAtLevel1 = int(v)
		}
	}
	if readErr == nil {
		if v, readErr = metaIn.ReadInt(); readErr == nil {
			r.maxImpactNumBytesAtLevel1 = int(v)
		}
	}

	var expectedDocLen int64
	if readErr == nil {
		expectedDocLen, readErr = metaIn.ReadLong()
	}
	if readErr != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene103 postings reader: read meta ints: %w", readErr)
	}
	_ = expectedDocLen // Gocene's RetrieveChecksum takes no expected-length argument.

	var expectedPosLen, expectedPayLen int64 = -1, -1
	if state.FieldInfos.HasProx() {
		expectedPosLen, readErr = metaIn.ReadLong()
		if readErr != nil {
			_ = metaIn.Close()
			return nil, fmt.Errorf("lucene103 postings reader: read meta pos len: %w", readErr)
		}
		if state.FieldInfos.HasPayloads() || state.FieldInfos.HasOffsets() {
			expectedPayLen, readErr = metaIn.ReadLong()
			if readErr != nil {
				_ = metaIn.Close()
				return nil, fmt.Errorf("lucene103 postings reader: read meta pay len: %w", readErr)
			}
		}
	}
	_ = expectedPosLen
	_ = expectedPayLen

	if _, err = CheckFooter(metaIn); err != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene103 postings reader: check meta footer: %w", err)
	}
	if err = metaIn.Close(); err != nil {
		return nil, fmt.Errorf("lucene103 postings reader: close meta: %w", err)
	}

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

	docName := GetSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene103DocExtension)
	docIn, err = state.Directory.OpenInput(docName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene103 postings reader: open doc %q: %w", docName, err)
	}
	if _, err = CheckIndexHeader(
		docIn, lucene103DocCodec, version, version,
		state.SegmentInfo.GetID(), state.SegmentSuffix,
	); err != nil {
		return nil, fmt.Errorf("lucene103 postings reader: check doc header: %w", err)
	}
	if _, err = RetrieveChecksum(docIn); err != nil {
		return nil, fmt.Errorf("lucene103 postings reader: retrieve doc checksum: %w", err)
	}

	if state.FieldInfos.HasProx() {
		posName := GetSegmentFileName(
			state.SegmentInfo.Name(), state.SegmentSuffix, lucene103PosExtension)
		posIn, err = state.Directory.OpenInput(posName, store.IOContext{Context: store.ContextRead})
		if err != nil {
			return nil, fmt.Errorf("lucene103 postings reader: open pos %q: %w", posName, err)
		}
		if _, err = CheckIndexHeader(
			posIn, lucene103PosCodec, version, version,
			state.SegmentInfo.GetID(), state.SegmentSuffix,
		); err != nil {
			return nil, fmt.Errorf("lucene103 postings reader: check pos header: %w", err)
		}
		if _, err = RetrieveChecksum(posIn); err != nil {
			return nil, fmt.Errorf("lucene103 postings reader: retrieve pos checksum: %w", err)
		}
		r.posIn = posIn

		if state.FieldInfos.HasPayloads() || state.FieldInfos.HasOffsets() {
			payName := GetSegmentFileName(
				state.SegmentInfo.Name(), state.SegmentSuffix, lucene103PayExtension)
			payIn, err = state.Directory.OpenInput(payName, store.IOContext{Context: store.ContextRead})
			if err != nil {
				return nil, fmt.Errorf("lucene103 postings reader: open pay %q: %w", payName, err)
			}
			if _, err = CheckIndexHeader(
				payIn, lucene103PayCodec, version, version,
				state.SegmentInfo.GetID(), state.SegmentSuffix,
			); err != nil {
				return nil, fmt.Errorf("lucene103 postings reader: check pay header: %w", err)
			}
			if _, err = RetrieveChecksum(payIn); err != nil {
				return nil, fmt.Errorf("lucene103 postings reader: retrieve pay checksum: %w", err)
			}
			r.payIn = payIn
		}
	}

	r.docIn = docIn
	success = true
	return r, nil
}

// Init validates the terms-in header and block size written by the writer.
// Mirrors Lucene103PostingsReader.init(IndexInput, SegmentReadState).
func (r *Lucene103PostingsReader) Init(termsIn store.IndexInput, state *SegmentReadState) error {
	if _, err := CheckIndexHeader(
		termsIn,
		lucene103TermsCodec,
		int32(lucene103VersionStart),
		int32(lucene103VersionCurrent),
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	); err != nil {
		return fmt.Errorf("lucene103 postings reader: init terms header: %w", err)
	}
	blockSize, err := store.ReadVInt(termsIn)
	if err != nil {
		return fmt.Errorf("lucene103 postings reader: read block size: %w", err)
	}
	if int(blockSize) != lucene103PostingsBlockSize {
		return fmt.Errorf(
			"lucene103 postings reader: index-time BLOCK_SIZE (%d) != read-time BLOCK_SIZE (%d)",
			blockSize, lucene103PostingsBlockSize,
		)
	}
	return nil
}

// NewTermState allocates a fresh IntBlockTermState and registers it.
func (r *Lucene103PostingsReader) NewTermState() *BlockTermState {
	its := NewIntBlockTermState()
	r.stateCache[its.BlockTermState] = its
	return its.BlockTermState
}

// DecodeTerm reads codec-specific metadata from in into termState.
// Mirrors Lucene103PostingsReader.decodeTerm(...).
func (r *Lucene103PostingsReader) DecodeTerm(
	in store.DataInput,
	fieldInfo *index.FieldInfo,
	termState *BlockTermState,
	absolute bool,
) error {
	its := r.lookupOrCreateState(termState)

	if absolute {
		its.DocStartFP = 0
		its.PosStartFP = 0
		its.PayStartFP = 0
	}

	l, err := store.ReadVLong(in)
	if err != nil {
		return fmt.Errorf("lucene103 decode term: read vlong l: %w", err)
	}

	if l&0x01 == 0 {
		its.DocStartFP += l >> 1
		if termState.DocFreq == 1 {
			sv, err2 := store.ReadVInt(in)
			if err2 != nil {
				return fmt.Errorf("lucene103 decode term: read singleton docID: %w", err2)
			}
			its.SingletonDocID = int(sv)
		} else {
			its.SingletonDocID = -1
		}
	} else {
		// Delta-encoded singleton docID (zig-zag).
		its.SingletonDocID += int(util.ZigZagDecodeInt64(l >> 1))
	}

	opts := fieldInfo.IndexOptions()
	if opts >= index.IndexOptionsDocsAndFreqsAndPositions {
		delta, err2 := store.ReadVLong(in)
		if err2 != nil {
			return fmt.Errorf("lucene103 decode term: read pos fp delta: %w", err2)
		}
		its.PosStartFP += delta

		if opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets ||
			fieldInfo.HasPayloads() {
			delta2, err3 := store.ReadVLong(in)
			if err3 != nil {
				return fmt.Errorf("lucene103 decode term: read pay fp delta: %w", err3)
			}
			its.PayStartFP += delta2
		}

		if termState.TotalTermFreq > int64(lucene103PostingsBlockSize) {
			offset, err4 := store.ReadVLong(in)
			if err4 != nil {
				return fmt.Errorf("lucene103 decode term: read lastPosBlockOffset: %w", err4)
			}
			its.LastPosBlockOffset = offset
		} else {
			its.LastPosBlockOffset = -1
		}
	}
	return nil
}

// Postings returns a postings enumerator positioned at the term described by
// termState. Mirrors Lucene103PostingsReader.postings(...).
func (r *Lucene103PostingsReader) Postings(
	fieldInfo *index.FieldInfo,
	termState *BlockTermState,
	reuse index.PostingsEnum,
	flags int,
) (index.PostingsEnum, error) {
	its := r.lookupOrCreateState(termState)

	var bpe *lucene103BlockPostingsEnum
	if prev, ok := reuse.(*lucene103BlockPostingsEnum); ok && prev.canReuse(r.docIn, fieldInfo, flags) {
		bpe = prev
	} else {
		var err error
		bpe, err = newLucene103BlockPostingsEnum(r, fieldInfo, flags)
		if err != nil {
			return nil, err
		}
	}
	return bpe.reset(its, flags)
}

// Impacts returns an ImpactsEnum for the given term, enabling impact-based
// block skipping. Mirrors Lucene103PostingsReader.impacts(...).
func (r *Lucene103PostingsReader) Impacts(
	fieldInfo *index.FieldInfo,
	termState *BlockTermState,
	flags int,
) (index.ImpactsEnum, error) {
	its := r.lookupOrCreateState(termState)
	bpe, err := newLucene103BlockPostingsEnum(r, fieldInfo, flags)
	if err != nil {
		return nil, err
	}
	bpe.needsImpacts = true
	bpe.needsDocsAndFreqsOnly = !bpe.needsPos && !bpe.needsImpacts
	pe, err := bpe.reset(its, flags)
	if err != nil {
		return nil, err
	}
	return pe.(*lucene103BlockPostingsEnum), nil
}

// CheckIntegrity validates CRC footers on all owned files.
func (r *Lucene103PostingsReader) CheckIntegrity() error {
	if r.docIn != nil {
		if _, err := ChecksumEntireFile(r.docIn); err != nil {
			return fmt.Errorf("lucene103 postings reader: checksum doc: %w", err)
		}
	}
	if r.posIn != nil {
		if _, err := ChecksumEntireFile(r.posIn); err != nil {
			return fmt.Errorf("lucene103 postings reader: checksum pos: %w", err)
		}
	}
	if r.payIn != nil {
		if _, err := ChecksumEntireFile(r.payIn); err != nil {
			return fmt.Errorf("lucene103 postings reader: checksum pay: %w", err)
		}
	}
	return nil
}

// Close releases file handles owned by this reader.
func (r *Lucene103PostingsReader) Close() error {
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

// ─── lucene103BlockPostingsEnum ───────────────────────────────────────────────

// lucene103BlockPostingsEnum implements index.PostingsEnum / index.ImpactsEnum
// for the Lucene 10.3 format. Mirrors Lucene103PostingsReader.BlockPostingsEnum.
type lucene103BlockPostingsEnum struct {
	reader *Lucene103PostingsReader

	forDeltaUtil *lucene103ForDeltaUtil
	pforUtil     *lucene103PForUtil

	// ── doc-block state ──
	encoding  deltaEncoding
	doc       int
	prevDocID int

	docBuffer  [lucene103PostingsBlockSize]int64
	freqBuffer [lucene103PostingsBlockSize]int64

	docBitSet                  *util.FixedBitSet
	docBitSetBase              int
	docCumulativeWordPopCounts [lucene103PostingsBlockSize]int64

	// ── skip state ──
	level0LastDocID int
	level0DocEndFP  int64

	level1LastDocID    int
	level1DocEndFP     int64
	level1DocCountUpto int

	// ── term state ──
	docFreq        int
	totalTermFreq  int64
	singletonDocID int

	docCountLeft  int
	docBufferSize int
	docBufferUpto int

	// ── doc input ──
	docIn store.IndexInput

	// ── freq state ──
	freqFP int64

	// ── position state ──
	posIn store.IndexInput

	posDeltaBuffer [lucene103PostingsBlockSize]int64
	posBufferUpto  int

	posPendingCount  int
	posDocBufferUpto int

	lastPosBlockFP int64
	position       int

	// ── payload/offset state ──
	payIn store.IndexInput

	payloadLengthBuffer    []int64
	offsetStartDeltaBuffer []int64
	offsetLengthBuffer     []int64

	payloadBytes    []byte
	payloadByteUpto int
	payloadLength   int

	lastStartOffset int
	startOffset     int
	endOffset       int

	// ── level-0 skip data for pos/pay ──
	level0PosEndFP     int64
	level0BlockPosUpto int
	level0PayEndFP     int64
	level0BlockPayUpto int

	level0SerializedImpacts []byte
	level0ImpactLen         int

	// ── level-1 skip data for pos/pay ──
	level1PosEndFP     int64
	level1BlockPosUpto int
	level1PayEndFP     int64
	level1BlockPayUpto int

	level1SerializedImpacts []byte
	level1ImpactLen         int

	// ── field characteristics ──
	options                   index.IndexOptions
	indexHasFreq              bool
	indexHasPos               bool
	indexHasOffsets           bool
	indexHasPayloads          bool
	indexHasOffsetsOrPayloads bool

	flags int

	// ── impacts support ──
	impactBuffer           *index.FreqAndNormBuffer
	scratch                *store.ByteArrayDataInput
	needsFreq              bool
	needsPos               bool
	needsOffsets           bool
	needsPayloads          bool
	needsOffsetsOrPayloads bool
	needsImpacts           bool
	needsDocsAndFreqsOnly  bool

	needsRefilling bool
}

// newLucene103BlockPostingsEnum constructs a fresh enum, cloning posIn/payIn as
// required by the field/flags. Mirrors BlockPostingsEnum(FieldInfo, int, boolean).
func newLucene103BlockPostingsEnum(
	r *Lucene103PostingsReader,
	fieldInfo *index.FieldInfo,
	flags int,
) (*lucene103BlockPostingsEnum, error) {
	e := &lucene103BlockPostingsEnum{
		reader:      r,
		freqFP:      -1,
		startOffset: -1,
		endOffset:   -1,
	}

	e.options = fieldInfo.IndexOptions()
	e.indexHasFreq = e.options >= index.IndexOptionsDocsAndFreqs
	e.indexHasPos = e.options >= index.IndexOptionsDocsAndFreqsAndPositions
	e.indexHasOffsets = e.options >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	e.indexHasPayloads = fieldInfo.HasPayloads()
	e.indexHasOffsetsOrPayloads = e.indexHasOffsets || e.indexHasPayloads

	e.flags = flags
	e.needsFreq = e.indexHasFreq && featureRequested(flags, index.PostingsFlagFreqs)
	e.needsPos = e.indexHasPos && featureRequested(flags, index.PostingsFlagPositions)
	e.needsOffsets = e.indexHasOffsets && featureRequested(flags, index.PostingsFlagOffsets)
	e.needsPayloads = e.indexHasPayloads && featureRequested(flags, index.PostingsFlagPayloads)
	e.needsOffsetsOrPayloads = e.needsOffsets || e.needsPayloads
	e.needsDocsAndFreqsOnly = !e.needsPos && !e.needsImpacts

	if !e.needsFreq {
		for i := range e.freqBuffer {
			e.freqBuffer[i] = 1
		}
	}

	if e.needsPos {
		if r.posIn == nil {
			return nil, fmt.Errorf("lucene103 postings reader: needsPos but posIn is nil")
		}
		e.posIn = r.posIn.Clone()
	}

	if e.needsOffsets || e.needsPayloads {
		if r.payIn == nil {
			return nil, fmt.Errorf("lucene103 postings reader: needsOffsets/Payloads but payIn is nil")
		}
		e.payIn = r.payIn.Clone()
	}

	if e.needsOffsets {
		e.offsetStartDeltaBuffer = make([]int64, lucene103PostingsBlockSize)
		e.offsetLengthBuffer = make([]int64, lucene103PostingsBlockSize)
	}

	if e.indexHasPayloads {
		e.payloadLengthBuffer = make([]int64, lucene103PostingsBlockSize)
		e.payloadBytes = make([]byte, 128)
	}

	if e.needsFreq {
		e.level0SerializedImpacts = make([]byte, r.maxImpactNumBytesAtLevel0)
		e.level1SerializedImpacts = make([]byte, r.maxImpactNumBytesAtLevel1)
		e.impactBuffer = index.NewFreqAndNormBuffer()
		e.impactBuffer.GrowNoCopy(r.maxNumImpactsAtLevel0)
		e.scratch = store.NewByteArrayDataInput(nil)
	}

	// docBitSet: BLOCK_SIZE * Integer.SIZE bits = 128 * 32 = 4096 bits.
	var err error
	e.docBitSet, err = util.NewFixedBitSet(lucene103PostingsBlockSize * 32)
	if err != nil {
		return nil, fmt.Errorf("lucene103 postings reader: allocate docBitSet: %w", err)
	}

	return e, nil
}

// canReuse returns true if this enum can be reset for the same docIn/field/flags.
func (e *lucene103BlockPostingsEnum) canReuse(docIn store.IndexInput, fi *index.FieldInfo, flags int) bool {
	return docIn == e.reader.docIn &&
		e.options == fi.IndexOptions() &&
		e.indexHasPayloads == fi.HasPayloads() &&
		e.flags == flags
}

// reset repositions the enum at the start of the given term state.
// Mirrors BlockPostingsEnum.reset(IntBlockTermState, int).
func (e *lucene103BlockPostingsEnum) reset(termState *IntBlockTermState, flags int) (index.PostingsEnum, error) {
	e.docFreq = termState.DocFreq
	e.singletonDocID = termState.SingletonDocID

	if e.docFreq > 1 {
		if e.docIn == nil {
			e.docIn = e.reader.docIn.Clone()
		}
	}

	if e.forDeltaUtil == nil && e.docFreq >= lucene103PostingsBlockSize {
		e.forDeltaUtil = newLucene103ForDeltaUtil()
	}
	e.totalTermFreq = termState.TotalTermFreq
	if !e.indexHasFreq {
		e.totalTermFreq = int64(termState.DocFreq)
	}
	if e.needsFreq && e.pforUtil == nil && e.totalTermFreq >= int64(lucene103PostingsBlockSize) {
		e.pforUtil = newLucene103PForUtil()
	}

	posTermStartFP := termState.PosStartFP
	payTermStartFP := termState.PayStartFP
	if e.posIn != nil {
		if err := e.posIn.SetPosition(posTermStartFP); err != nil {
			return nil, fmt.Errorf("lucene103 postings enum reset: seek posIn: %w", err)
		}
		if e.payIn != nil {
			if err := e.payIn.SetPosition(payTermStartFP); err != nil {
				return nil, fmt.Errorf("lucene103 postings enum reset: seek payIn: %w", err)
			}
		}
	}

	e.level1PosEndFP = posTermStartFP
	e.level1PayEndFP = payTermStartFP
	e.level0PosEndFP = posTermStartFP
	e.level0PayEndFP = payTermStartFP
	e.posPendingCount = 0
	e.payloadByteUpto = 0

	if termState.TotalTermFreq < int64(lucene103PostingsBlockSize) {
		e.lastPosBlockFP = posTermStartFP
	} else if termState.TotalTermFreq == int64(lucene103PostingsBlockSize) {
		e.lastPosBlockFP = -1
	} else {
		e.lastPosBlockFP = posTermStartFP + termState.LastPosBlockOffset
	}

	e.level1BlockPosUpto = 0
	e.level1BlockPayUpto = 0
	e.level0BlockPosUpto = 0
	e.level0BlockPayUpto = 0
	e.posBufferUpto = lucene103PostingsBlockSize

	e.doc = -1
	e.prevDocID = -1
	e.docCountLeft = e.docFreq
	e.freqFP = -1
	e.level0LastDocID = -1

	if e.docFreq < lucene103Level1NumDocs {
		e.level1LastDocID = lucene103Level1NoSkip
		e.level1DocEndFP = termState.DocStartFP
		if e.docFreq > 1 {
			if err := e.docIn.SetPosition(termState.DocStartFP); err != nil {
				return nil, fmt.Errorf("lucene103 postings enum reset: seek docIn: %w", err)
			}
		}
	} else {
		e.level1LastDocID = -1
		e.level1DocEndFP = termState.DocStartFP
	}
	e.level1DocCountUpto = 0
	e.docBufferSize = lucene103PostingsBlockSize
	e.docBufferUpto = lucene103PostingsBlockSize
	e.posDocBufferUpto = lucene103PostingsBlockSize
	e.needsRefilling = false

	return e, nil
}

// ─── PostingsEnum interface ───────────────────────────────────────────────────

// DocID returns the current doc ID.
func (e *lucene103BlockPostingsEnum) DocID() int { return e.doc }

// Cost returns the estimated iteration cost (docFreq).
func (e *lucene103BlockPostingsEnum) Cost() int64 { return int64(e.docFreq) }

// Freq returns the frequency for the current document. Mirrors freq().
func (e *lucene103BlockPostingsEnum) Freq() (int, error) {
	if e.freqFP != -1 {
		if err := e.docIn.SetPosition(e.freqFP); err != nil {
			return 0, err
		}
		if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
			return 0, err
		}
		e.freqFP = -1
	}
	return int(e.freqBuffer[e.docBufferUpto-1]), nil
}

// StartOffset returns the start character offset of the current occurrence.
func (e *lucene103BlockPostingsEnum) StartOffset() (int, error) { return e.startOffset, nil }

// EndOffset returns the end character offset of the current occurrence.
func (e *lucene103BlockPostingsEnum) EndOffset() (int, error) { return e.endOffset, nil }

// GetPayload returns the payload bytes for the current occurrence, or nil.
func (e *lucene103BlockPostingsEnum) GetPayload() ([]byte, error) {
	if !e.needsPayloads || e.payloadLength == 0 {
		return nil, nil
	}
	start := e.payloadByteUpto - e.payloadLength
	if start < 0 {
		start = 0
	}
	return e.payloadBytes[start : start+e.payloadLength], nil
}

// ─── doc-block decoding ───────────────────────────────────────────────────────

// refillFullBlock decodes the next full block (BLOCK_SIZE docs) from docIn.
// Mirrors BlockPostingsEnum.refillFullBlock().
func (e *lucene103BlockPostingsEnum) refillFullBlock() error {
	bpv, err := e.docIn.ReadByte()
	if err != nil {
		return fmt.Errorf("lucene103 refillFullBlock: read bpv: %w", err)
	}
	intBPV := int(int8(bpv)) // signed: negative => bit-set encoding

	if intBPV > 0 {
		// PACKED: 128 packed integers that record the delta between doc IDs.
		// Lucene 10.3 fuses the FOR decode with the prefix-sum.
		if err := e.forDeltaUtil.decodeAndPrefixSum(intBPV, e.docIn, int32(e.prevDocID), e.docBuffer[:]); err != nil {
			return fmt.Errorf("lucene103 refillFullBlock: FOR-delta decode: %w", err)
		}
		e.encoding = deltaEncodingPacked
	} else {
		// UNARY / bit-set encoding.
		e.docBitSetBase = e.prevDocID + 1
		var numLongs int
		if intBPV == 0 {
			// 0 records that all 128 docs in the block are consecutive.
			numLongs = lucene103PostingsBlockSize / 64 // = 2
			e.docBitSet.SetAll()
			words := e.docBitSet.GetBits()
			for i := numLongs; i < len(words); i++ {
				words[i] = 0
			}
		} else {
			numLongs = -intBPV
			words := e.docBitSet.GetBits()
			e.docBitSet.ClearAll()
			for i := 0; i < numLongs; i++ {
				raw, err2 := e.docIn.ReadLong()
				if err2 != nil {
					return fmt.Errorf("lucene103 refillFullBlock: read bit-set word %d: %w", i, err2)
				}
				words[i] = uint64(raw)
			}
		}
		if e.needsFreq {
			words := e.docBitSet.GetBits()
			for i := 0; i < numLongs-1; i++ {
				e.docCumulativeWordPopCounts[i] = int64(bits.OnesCount64(words[i]))
			}
			var acc int64
			for i := 0; i < numLongs-1; i++ {
				acc += e.docCumulativeWordPopCounts[i]
				e.docCumulativeWordPopCounts[i] = acc
			}
			e.docCumulativeWordPopCounts[numLongs-1] = lucene103PostingsBlockSize
		}
		e.encoding = deltaEncodingUnary
	}

	if e.indexHasFreq {
		if e.needsFreq {
			e.freqFP = e.docIn.GetFilePointer()
		}
		if err2 := lucene103PForUtilSkip(e.docIn); err2 != nil {
			return fmt.Errorf("lucene103 refillFullBlock: skip freq block: %w", err2)
		}
	}

	e.docCountLeft -= lucene103PostingsBlockSize
	if e.encoding == deltaEncodingPacked {
		e.prevDocID = int(e.docBuffer[lucene103PostingsBlockSize-1])
	} else {
		e.prevDocID = e.docBitSetBase + e.docBitSet.PrevSetBit(lucene103PostingsBlockSize*32-1)
	}
	e.docBufferUpto = 0
	e.posDocBufferUpto = 0
	return nil
}

// refillRemainder decodes a tail (< BLOCK_SIZE docs) block.
// Mirrors BlockPostingsEnum.refillRemainder().
func (e *lucene103BlockPostingsEnum) refillRemainder() error {
	if e.docFreq == 1 {
		e.docBuffer[0] = int64(e.singletonDocID)
		e.freqBuffer[0] = e.totalTermFreq
		e.docBuffer[1] = index.NO_MORE_DOCS
		e.freqFP = -1
		e.docCountLeft = 0
		e.docBufferSize = 1
	} else {
		if err := readVIntBlock104(e.docIn, e.docBuffer[:], e.freqBuffer[:],
			e.docCountLeft, e.indexHasFreq, e.needsFreq); err != nil {
			return fmt.Errorf("lucene103 refillRemainder: %w", err)
		}
		prefixSum64(e.docBuffer[:], e.docCountLeft, int64(e.prevDocID))
		e.docBuffer[e.docCountLeft] = index.NO_MORE_DOCS
		e.freqFP = -1
		e.docBufferSize = e.docCountLeft
		e.docCountLeft = 0
	}
	e.prevDocID = int(e.docBuffer[lucene103PostingsBlockSize-1])
	e.docBufferUpto = 0
	e.posDocBufferUpto = 0
	e.encoding = deltaEncodingPacked
	return nil
}

// refillDocs dispatches to refillFullBlock or refillRemainder.
func (e *lucene103BlockPostingsEnum) refillDocs() error {
	if e.docCountLeft >= lucene103PostingsBlockSize {
		return e.refillFullBlock()
	}
	return e.refillRemainder()
}

// ─── skip navigation ──────────────────────────────────────────────────────────

// skipLevel1To advances level-1 skip data until level1LastDocID >= target.
// Mirrors BlockPostingsEnum.skipLevel1To(int).
func (e *lucene103BlockPostingsEnum) skipLevel1To(target int) error {
	for {
		e.prevDocID = e.level1LastDocID
		e.level0LastDocID = e.level1LastDocID
		if err := e.docIn.SetPosition(e.level1DocEndFP); err != nil {
			return err
		}
		e.level0PosEndFP = e.level1PosEndFP
		e.level0BlockPosUpto = e.level1BlockPosUpto
		e.level0PayEndFP = e.level1PayEndFP
		e.level0BlockPayUpto = e.level1BlockPayUpto
		e.docCountLeft = e.docFreq - e.level1DocCountUpto
		e.level1DocCountUpto += lucene103Level1NumDocs

		if e.docCountLeft < lucene103Level1NumDocs {
			e.level1LastDocID = lucene103Level1NoSkip
			break
		}

		delta1, err := store.ReadVInt(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene103 skipLevel1To: read level1DocDelta: %w", err)
		}
		e.level1LastDocID += int(delta1)

		delta2, err := store.ReadVLong(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene103 skipLevel1To: read level1DocEndFP delta: %w", err)
		}
		e.level1DocEndFP = delta2 + e.docIn.GetFilePointer()

		if e.indexHasFreq {
			skip1EndFPDelta, err2 := e.docIn.ReadShort()
			if err2 != nil {
				return fmt.Errorf("lucene103 skipLevel1To: read skip1EndFP: %w", err2)
			}
			skip1EndFP := int64(skip1EndFPDelta) + e.docIn.GetFilePointer()

			numImpactBytes, err3 := e.docIn.ReadShort()
			if err3 != nil {
				return fmt.Errorf("lucene103 skipLevel1To: read numImpactBytes: %w", err3)
			}

			if e.needsImpacts && e.level1LastDocID >= target {
				if err4 := e.docIn.ReadBytes(e.level1SerializedImpacts[:numImpactBytes]); err4 != nil {
					return fmt.Errorf("lucene103 skipLevel1To: read impact bytes: %w", err4)
				}
				e.level1ImpactLen = int(numImpactBytes)
			} else {
				if err4 := skipBytesInput(e.docIn, int64(numImpactBytes)); err4 != nil {
					return fmt.Errorf("lucene103 skipLevel1To: skip impact bytes: %w", err4)
				}
			}

			if e.indexHasPos {
				posEndFPDelta, err5 := store.ReadVLong(e.docIn)
				if err5 != nil {
					return fmt.Errorf("lucene103 skipLevel1To: read posEndFP delta: %w", err5)
				}
				e.level1PosEndFP += posEndFPDelta

				posUpto, err6 := e.docIn.ReadByte()
				if err6 != nil {
					return fmt.Errorf("lucene103 skipLevel1To: read posUpto: %w", err6)
				}
				e.level1BlockPosUpto = int(posUpto)

				if e.indexHasOffsetsOrPayloads {
					payEndFPDelta, err7 := store.ReadVLong(e.docIn)
					if err7 != nil {
						return fmt.Errorf("lucene103 skipLevel1To: read payEndFP delta: %w", err7)
					}
					e.level1PayEndFP += payEndFPDelta

					payUpto, err8 := store.ReadVInt(e.docIn)
					if err8 != nil {
						return fmt.Errorf("lucene103 skipLevel1To: read payUpto: %w", err8)
					}
					e.level1BlockPayUpto = int(payUpto)
				}
			}
			if err9 := e.docIn.SetPosition(skip1EndFP); err9 != nil {
				return fmt.Errorf("lucene103 skipLevel1To: seek to skip1End: %w", err9)
			}
		}

		if e.level1LastDocID >= target {
			break
		}
	}
	return nil
}

// readLevel0PosData reads pos/pay skip data for a level-0 block.
// Mirrors BlockPostingsEnum.readLevel0PosData().
func (e *lucene103BlockPostingsEnum) readLevel0PosData() error {
	posEndFPDelta, err := store.ReadVLong(e.docIn)
	if err != nil {
		return err
	}
	e.level0PosEndFP += posEndFPDelta

	posUpto, err2 := e.docIn.ReadByte()
	if err2 != nil {
		return err2
	}
	e.level0BlockPosUpto = int(posUpto)

	if e.indexHasOffsetsOrPayloads {
		payEndFPDelta, err3 := store.ReadVLong(e.docIn)
		if err3 != nil {
			return err3
		}
		e.level0PayEndFP += payEndFPDelta

		payUpto, err4 := store.ReadVInt(e.docIn)
		if err4 != nil {
			return err4
		}
		e.level0BlockPayUpto = int(payUpto)
	}
	return nil
}

// seekPosData repositions posIn/payIn or accumulates pending positions.
// Mirrors BlockPostingsEnum.seekPosData(long, int, long, int).
func (e *lucene103BlockPostingsEnum) seekPosData(posFP int64, posUpto int, payFP int64, payUpto int) error {
	if e.posIn == nil {
		return nil
	}
	if posFP >= e.posIn.GetFilePointer() {
		if err := e.posIn.SetPosition(posFP); err != nil {
			return err
		}
		e.posPendingCount = posUpto
		if e.payIn != nil {
			if err := e.payIn.SetPosition(payFP); err != nil {
				return err
			}
			e.payloadByteUpto = payUpto
		}
		e.posBufferUpto = lucene103PostingsBlockSize
	} else {
		e.posPendingCount += int(sumOverRange64(e.freqBuffer[:], e.posDocBufferUpto, lucene103PostingsBlockSize))
	}
	return nil
}

// skipLevel0To advances level-0 skip data until level0LastDocID >= target.
// Mirrors BlockPostingsEnum.skipLevel0To(int).
func (e *lucene103BlockPostingsEnum) skipLevel0To(target int) error {
	var posFP int64
	var posUpto int
	var payFP int64
	var payUpto int

	for {
		e.prevDocID = e.level0LastDocID

		posFP = e.level0PosEndFP
		posUpto = e.level0BlockPosUpto
		payFP = e.level0PayEndFP
		payUpto = e.level0BlockPayUpto

		if e.docCountLeft < lucene103PostingsBlockSize {
			e.level0LastDocID = lucene103NoMoreDocs
			break
		}

		numSkipBytes, err := store.ReadVLong(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene103 skipLevel0To: read numSkipBytes: %w", err)
		}
		skip0End := e.docIn.GetFilePointer() + numSkipBytes

		docDelta, err2 := readVInt15(e.docIn)
		if err2 != nil {
			return fmt.Errorf("lucene103 skipLevel0To: read docDelta: %w", err2)
		}
		e.level0LastDocID += docDelta
		found := target <= e.level0LastDocID

		blockLen, err3 := readVLong15(e.docIn)
		if err3 != nil {
			return fmt.Errorf("lucene103 skipLevel0To: read blockLen: %w", err3)
		}
		e.level0DocEndFP = e.docIn.GetFilePointer() + blockLen

		if e.indexHasFreq {
			if !found && !e.needsPos {
				if err4 := e.docIn.SetPosition(skip0End); err4 != nil {
					return err4
				}
			} else {
				numImpactBytes, err5 := store.ReadVInt(e.docIn)
				if err5 != nil {
					return fmt.Errorf("lucene103 skipLevel0To: read numImpactBytes: %w", err5)
				}
				if e.needsImpacts && found {
					if err6 := e.docIn.ReadBytes(e.level0SerializedImpacts[:numImpactBytes]); err6 != nil {
						return fmt.Errorf("lucene103 skipLevel0To: read impact bytes: %w", err6)
					}
					e.level0ImpactLen = int(numImpactBytes)
				} else {
					if err6 := skipBytesInput(e.docIn, int64(numImpactBytes)); err6 != nil {
						return fmt.Errorf("lucene103 skipLevel0To: skip impact bytes: %w", err6)
					}
				}

				if e.needsPos {
					if err7 := e.readLevel0PosData(); err7 != nil {
						return err7
					}
				} else {
					if err7 := e.docIn.SetPosition(skip0End); err7 != nil {
						return err7
					}
				}
			}
		}

		if found {
			break
		}

		if err8 := e.docIn.SetPosition(e.level0DocEndFP); err8 != nil {
			return err8
		}
		e.docCountLeft -= lucene103PostingsBlockSize
	}

	return e.seekPosData(posFP, posUpto, payFP, payUpto)
}

// doAdvanceShallow advances both level-1 and level-0 skip data.
// Mirrors BlockPostingsEnum.doAdvanceShallow(int).
func (e *lucene103BlockPostingsEnum) doAdvanceShallow(target int) error {
	if target > e.level1LastDocID {
		if err := e.skipLevel1To(target); err != nil {
			return err
		}
	} else if e.needsRefilling {
		if err := e.docIn.SetPosition(e.level0DocEndFP); err != nil {
			return err
		}
		e.docCountLeft -= lucene103PostingsBlockSize
	}
	return e.skipLevel0To(target)
}

// doMoveToNextLevel0Block decodes skip data and refills the next level-0 block.
// Mirrors BlockPostingsEnum.doMoveToNextLevel0Block().
func (e *lucene103BlockPostingsEnum) doMoveToNextLevel0Block() error {
	if e.posIn != nil {
		if e.level0PosEndFP >= e.posIn.GetFilePointer() {
			if err := e.posIn.SetPosition(e.level0PosEndFP); err != nil {
				return err
			}
			e.posPendingCount = e.level0BlockPosUpto
			if e.payIn != nil {
				if err := e.payIn.SetPosition(e.level0PayEndFP); err != nil {
					return err
				}
				e.payloadByteUpto = e.level0BlockPayUpto
			}
			e.posBufferUpto = lucene103PostingsBlockSize
		} else {
			e.posPendingCount += int(sumOverRange64(e.freqBuffer[:], e.posDocBufferUpto, lucene103PostingsBlockSize))
		}
	}

	if e.docCountLeft >= lucene103PostingsBlockSize {
		level0NumBytes, err := store.ReadVLong(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene103 doMoveToNextLevel0Block: read level0NumBytes: %w", err)
		}
		_ = level0NumBytes

		docDelta, err2 := readVInt15(e.docIn)
		if err2 != nil {
			return fmt.Errorf("lucene103 doMoveToNextLevel0Block: read docDelta: %w", err2)
		}
		e.level0LastDocID += docDelta

		blockLen, err3 := readVLong15(e.docIn)
		if err3 != nil {
			return fmt.Errorf("lucene103 doMoveToNextLevel0Block: read blockLen: %w", err3)
		}
		e.level0DocEndFP = e.docIn.GetFilePointer() + blockLen

		if e.indexHasFreq {
			numImpactBytes, err4 := store.ReadVInt(e.docIn)
			if err4 != nil {
				return fmt.Errorf("lucene103 doMoveToNextLevel0Block: read numImpactBytes: %w", err4)
			}
			if e.needsImpacts {
				if err5 := e.docIn.ReadBytes(e.level0SerializedImpacts[:numImpactBytes]); err5 != nil {
					return err5
				}
				e.level0ImpactLen = int(numImpactBytes)
			} else {
				if err5 := skipBytesInput(e.docIn, int64(numImpactBytes)); err5 != nil {
					return err5
				}
			}

			if e.indexHasPos {
				if err6 := e.readLevel0PosData(); err6 != nil {
					return err6
				}
			}
		}
		return e.refillFullBlock()
	}

	e.level0LastDocID = lucene103NoMoreDocs
	return e.refillRemainder()
}

// moveToNextLevel0Block moves to the next level-0 block, upgrading level-1 skip
// data first when needed. Mirrors BlockPostingsEnum.moveToNextLevel0Block().
func (e *lucene103BlockPostingsEnum) moveToNextLevel0Block() error {
	if e.doc == e.level1LastDocID {
		if err := e.skipLevel1To(e.doc + 1); err != nil {
			return err
		}
	}

	e.prevDocID = e.level0LastDocID

	if e.needsDocsAndFreqsOnly && e.docCountLeft >= lucene103PostingsBlockSize {
		level0NumBytes, err := store.ReadVLong(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene103 moveToNextLevel0Block: read level0NumBytes: %w", err)
		}
		level0End := e.docIn.GetFilePointer() + level0NumBytes
		docDelta, err2 := readVInt15(e.docIn)
		if err2 != nil {
			return err2
		}
		e.level0LastDocID += docDelta
		if err3 := e.docIn.SetPosition(level0End); err3 != nil {
			return err3
		}
		return e.refillFullBlock()
	}
	return e.doMoveToNextLevel0Block()
}

// NextDoc advances to the next document. Mirrors BlockPostingsEnum.nextDoc().
func (e *lucene103BlockPostingsEnum) NextDoc() (int, error) {
	// Once exhausted on the final block, stay exhausted. See the equivalent
	// guard in the Lucene104 enum for the full rationale (rmp #4763 follow-on).
	if e.doc == index.NO_MORE_DOCS && e.level0LastDocID == lucene103NoMoreDocs {
		return index.NO_MORE_DOCS, nil
	}
	if e.doc == e.level0LastDocID || e.needsRefilling {
		if e.needsRefilling {
			if err := e.refillDocs(); err != nil {
				return 0, err
			}
			e.needsRefilling = false
		} else {
			if err := e.moveToNextLevel0Block(); err != nil {
				return 0, err
			}
		}
	}

	switch e.encoding {
	case deltaEncodingPacked:
		e.doc = int(e.docBuffer[e.docBufferUpto])
	case deltaEncodingUnary:
		next := e.docBitSet.NextSetBit(e.doc - e.docBitSetBase + 1)
		e.doc = e.docBitSetBase + next
	}
	e.docBufferUpto++
	return e.doc, nil
}

// Advance advances to the first document with docID >= target.
// Mirrors BlockPostingsEnum.advance(int).
func (e *lucene103BlockPostingsEnum) Advance(target int) (int, error) {
	if target > e.level0LastDocID || e.needsRefilling {
		if target > e.level0LastDocID {
			if err := e.doAdvanceShallow(target); err != nil {
				return 0, err
			}
		}
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
		e.needsRefilling = false
	}

	switch e.encoding {
	case deltaEncodingPacked:
		next := findNextGEQ64(e.docBuffer[:], target, e.docBufferUpto, e.docBufferSize)
		e.doc = int(e.docBuffer[next])
		e.docBufferUpto = next + 1
	case deltaEncodingUnary:
		next := e.docBitSet.NextSetBit(target - e.docBitSetBase)
		e.doc = e.docBitSetBase + next
		if e.needsFreq {
			wordIndex := next >> 6
			e.docBufferUpto = 1 + int(e.docCumulativeWordPopCounts[wordIndex]) -
				bits.OnesCount64(e.docBitSet.GetBits()[wordIndex]>>next)
		} else {
			e.docBufferUpto = 1
		}
	}

	return e.doc, nil
}

// ─── positions ───────────────────────────────────────────────────────────────

// skipPositions skips freq positions from the position stream.
// Mirrors BlockPostingsEnum.skipPositions(int).
func (e *lucene103BlockPostingsEnum) skipPositions(freq int) error {
	toSkip := e.posPendingCount - freq
	leftInBlock := lucene103PostingsBlockSize - e.posBufferUpto
	if toSkip < leftInBlock {
		end := e.posBufferUpto + toSkip
		if e.needsPayloads {
			for i := e.posBufferUpto; i < end; i++ {
				e.payloadByteUpto += int(e.payloadLengthBuffer[i])
			}
		}
		e.posBufferUpto = end
		return nil
	}
	toSkip -= leftInBlock
	for toSkip >= lucene103PostingsBlockSize {
		if err := lucene103PForUtilSkip(e.posIn); err != nil {
			return err
		}
		if e.payIn != nil {
			if e.indexHasPayloads {
				if err := lucene103PForUtilSkip(e.payIn); err != nil {
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
			if e.indexHasOffsets {
				if err := lucene103PForUtilSkip(e.payIn); err != nil {
					return err
				}
				if err2 := lucene103PForUtilSkip(e.payIn); err2 != nil {
					return err2
				}
			}
		}
		toSkip -= lucene103PostingsBlockSize
	}
	if err := e.refillPositions(); err != nil {
		return err
	}
	if e.needsPayloads {
		var s int64
		for i := 0; i < toSkip; i++ {
			s += e.payloadLengthBuffer[i]
		}
		e.payloadByteUpto = int(s)
	}
	e.posBufferUpto = toSkip
	return nil
}

// refillLastPositionBlock decodes the final (tail) position block from VInts.
// Mirrors BlockPostingsEnum.refillLastPositionBlock().
func (e *lucene103BlockPostingsEnum) refillLastPositionBlock() error {
	count := int(e.totalTermFreq % int64(lucene103PostingsBlockSize))
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
			if e.payloadLengthBuffer != nil {
				e.payloadLengthBuffer[i] = int64(payloadLength)
				e.posDeltaBuffer[i] = int64(code >> 1)
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
				_ = skipBytesInput(e.posIn, int64(payloadLength))
				e.posDeltaBuffer[i] = int64(code >> 1)
			}
		} else {
			e.posDeltaBuffer[i] = int64(code)
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
			if e.offsetStartDeltaBuffer != nil {
				e.offsetStartDeltaBuffer[i] = int64(deltaCode >> 1)
				e.offsetLengthBuffer[i] = int64(offsetLength)
			}
		}
	}
	e.payloadByteUpto = 0
	return nil
}

// refillOffsetsOrPayloads decodes payload/offset PFOR blocks from payIn.
// Mirrors BlockPostingsEnum.refillOffsetsOrPayloads().
func (e *lucene103BlockPostingsEnum) refillOffsetsOrPayloads() error {
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
		} else if e.payIn != nil {
			if err := lucene103PForUtilSkip(e.payIn); err != nil {
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
		} else if e.payIn != nil {
			if err := lucene103PForUtilSkip(e.payIn); err != nil {
				return err
			}
			if err2 := lucene103PForUtilSkip(e.payIn); err2 != nil {
				return err2
			}
		}
	}
	return nil
}

// refillPositions decodes the next 128-position block from posIn.
// Mirrors BlockPostingsEnum.refillPositions().
func (e *lucene103BlockPostingsEnum) refillPositions() error {
	if e.posIn.GetFilePointer() == e.lastPosBlockFP {
		return e.refillLastPositionBlock()
	}
	if err := e.pforUtil.decode(e.posIn, e.posDeltaBuffer[:]); err != nil {
		return err
	}
	if e.indexHasOffsetsOrPayloads {
		return e.refillOffsetsOrPayloads()
	}
	return nil
}

// accumulatePendingPositions triggers lazy freq decoding and sums pending
// positions for the current document.
// Mirrors BlockPostingsEnum.accumulatePendingPositions().
func (e *lucene103BlockPostingsEnum) accumulatePendingPositions() error {
	freq, err := e.Freq()
	if err != nil {
		return err
	}
	e.posPendingCount += int(sumOverRange64(e.freqBuffer[:], e.posDocBufferUpto, e.docBufferUpto))
	e.posDocBufferUpto = e.docBufferUpto

	if e.posPendingCount > freq {
		if err2 := e.skipPositions(freq); err2 != nil {
			return err2
		}
		e.posPendingCount = freq
	}
	return nil
}

// NextPosition advances to the next position in the current document.
// Mirrors BlockPostingsEnum.nextPosition().
func (e *lucene103BlockPostingsEnum) NextPosition() (int, error) {
	if !e.needsPos {
		return -1, nil
	}

	if e.posDocBufferUpto != e.docBufferUpto {
		if err := e.accumulatePendingPositions(); err != nil {
			return 0, err
		}
		e.position = 0
		e.lastStartOffset = 0
	}

	if e.posBufferUpto == lucene103PostingsBlockSize {
		if err := e.refillPositions(); err != nil {
			return 0, err
		}
		e.posBufferUpto = 0
	}

	e.position += int(e.posDeltaBuffer[e.posBufferUpto])

	if e.needsOffsetsOrPayloads {
		if e.needsPayloads {
			e.payloadLength = int(e.payloadLengthBuffer[e.posBufferUpto])
			e.payloadByteUpto += e.payloadLength
		}
		if e.needsOffsets {
			e.startOffset = e.lastStartOffset + int(e.offsetStartDeltaBuffer[e.posBufferUpto])
			e.endOffset = e.startOffset + int(e.offsetLengthBuffer[e.posBufferUpto])
			e.lastStartOffset = e.startOffset
		}
	}

	e.posBufferUpto++
	e.posPendingCount--
	return e.position, nil
}

// ─── ImpactsEnum support ──────────────────────────────────────────────────────

// AdvanceShallow advances the skip data to cover target without decoding the
// full doc block. Mirrors BlockPostingsEnum.advanceShallow(int).
func (e *lucene103BlockPostingsEnum) AdvanceShallow(target int) error {
	if target > e.level0LastDocID {
		if err := e.doAdvanceShallow(target); err != nil {
			return err
		}
		e.needsRefilling = true
	}
	return nil
}

// GetImpacts returns the impact summary for docs up to the current skip
// boundaries. Mirrors BlockPostingsEnum.getImpacts().
func (e *lucene103BlockPostingsEnum) GetImpacts() (index.Impacts, error) {
	return &lucene103BlockImpacts{enum: e}, nil
}

// lucene103BlockImpacts implements index.Impacts backed by a
// lucene103BlockPostingsEnum.
type lucene103BlockImpacts struct {
	enum *lucene103BlockPostingsEnum
}

// NumLevels mirrors BlockPostingsEnum.Impacts.numLevels().
func (b *lucene103BlockImpacts) NumLevels() int {
	if !b.enum.indexHasFreq || b.enum.level1LastDocID == lucene103Level1NoSkip {
		return 1
	}
	return 2
}

// GetDocIDUpTo mirrors BlockPostingsEnum.Impacts.getDocIdUpTo(int).
func (b *lucene103BlockImpacts) GetDocIDUpTo(level int) int {
	if !b.enum.indexHasFreq {
		return index.NO_MORE_DOCS
	}
	if level == 0 {
		v := b.enum.level0LastDocID
		if v == lucene103NoMoreDocs {
			return index.NO_MORE_DOCS
		}
		return v
	}
	if level == 1 {
		v := b.enum.level1LastDocID
		if v == lucene103Level1NoSkip {
			return index.NO_MORE_DOCS
		}
		return v
	}
	return index.NO_MORE_DOCS
}

// GetImpacts mirrors BlockPostingsEnum.Impacts.getImpacts(int).
func (b *lucene103BlockImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	e := b.enum
	if !e.indexHasFreq {
		e.impactBuffer.GrowNoCopy(1)
		e.impactBuffer.Size = 1
		e.impactBuffer.Freqs[0] = 1
		e.impactBuffer.Norms[0] = 1
		return e.impactBuffer
	}
	if level == 0 && e.level0LastDocID != lucene103NoMoreDocs {
		e.scratch.Reset(e.level0SerializedImpacts[:e.level0ImpactLen])
		readImpactsFromBytes(e.scratch, e.impactBuffer)
		return e.impactBuffer
	}
	if level == 1 {
		e.scratch.Reset(e.level1SerializedImpacts[:e.level1ImpactLen])
		readImpactsFromBytes(e.scratch, e.impactBuffer)
		return e.impactBuffer
	}
	e.impactBuffer.GrowNoCopy(1)
	e.impactBuffer.Size = 1
	e.impactBuffer.Freqs[0] = math.MaxInt32
	e.impactBuffer.Norms[0] = 1
	return e.impactBuffer
}

// Compile-time checks.
var (
	_ PostingsReaderBase = (*Lucene103PostingsReader)(nil)
	_ index.ImpactsEnum  = (*lucene103BlockPostingsEnum)(nil)
	_ index.Impacts      = (*lucene103BlockImpacts)(nil)
)
