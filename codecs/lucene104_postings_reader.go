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

package codecs

import (
	"fmt"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Lucene104PostingsReader reads .doc / .pos / .pay / .psm files written by
// Lucene104PostingsWriter.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene104.Lucene104PostingsReader from Apache
// Lucene 10.4.0.
type Lucene104PostingsReader struct {
	docIn store.IndexInput
	posIn store.IndexInput // nil when segment has no positions
	payIn store.IndexInput // nil when segment has no payloads or offsets

	maxNumImpactsAtLevel0     int
	maxImpactNumBytesAtLevel0 int
	maxNumImpactsAtLevel1     int
	maxImpactNumBytesAtLevel1 int

	// stateCache maps *BlockTermState handles (allocated by NewTermState) back
	// to the *IntBlockTermState that owns them.  Same bridge pattern as writer.
	stateCache map[*BlockTermState]*IntBlockTermState
}

// NewLucene104PostingsReader opens and validates the .psm meta file, then
// opens .doc, and conditionally .pos / .pay.
//
// Mirrors org.apache.lucene.codecs.lucene104.Lucene104PostingsReader(SegmentReadState).
func NewLucene104PostingsReader(state *SegmentReadState) (*Lucene104PostingsReader, error) {
	metaName := GetSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene104MetaExtension)

	rawMeta, err := state.Directory.OpenInput(metaName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene104 postings reader: open meta %q: %w", metaName, err)
	}
	metaIn := store.NewChecksumIndexInput(rawMeta)

	var version int32
	version, err = CheckIndexHeader(
		metaIn,
		lucene104MetaCodec,
		int32(lucene104VersionStart),
		int32(lucene104VersionCurrent),
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene104 postings reader: check meta header: %w", err)
	}

	r := &Lucene104PostingsReader{
		stateCache: make(map[*BlockTermState]*IntBlockTermState),
	}

	var v int32
	var readErr error
	if v, readErr = metaIn.ReadInt(); readErr == nil {
		r.maxNumImpactsAtLevel0 = int(v)
	}
	if readErr == nil {
		v, readErr = metaIn.ReadInt()
		if readErr == nil {
			r.maxImpactNumBytesAtLevel0 = int(v)
		}
	}
	if readErr == nil {
		v, readErr = metaIn.ReadInt()
		if readErr == nil {
			r.maxNumImpactsAtLevel1 = int(v)
		}
	}
	if readErr == nil {
		v, readErr = metaIn.ReadInt()
		if readErr == nil {
			r.maxImpactNumBytesAtLevel1 = int(v)
		}
	}

	var expectedDocLen int64
	if readErr == nil {
		expectedDocLen, readErr = metaIn.ReadLong()
	}
	if readErr != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene104 postings reader: read meta ints: %w", readErr)
	}
	// expectedDocLen is preserved to match Java's logic; Gocene's
	// RetrieveChecksum does not take an expected-length argument.
	_ = expectedDocLen

	var expectedPosLen, expectedPayLen int64 = -1, -1
	if state.FieldInfos.HasProx() {
		expectedPosLen, readErr = metaIn.ReadLong()
		if readErr != nil {
			_ = metaIn.Close()
			return nil, fmt.Errorf("lucene104 postings reader: read meta pos len: %w", readErr)
		}
		if state.FieldInfos.HasPayloads() || state.FieldInfos.HasOffsets() {
			expectedPayLen, readErr = metaIn.ReadLong()
			if readErr != nil {
				_ = metaIn.Close()
				return nil, fmt.Errorf("lucene104 postings reader: read meta pay len: %w", readErr)
			}
		}
	}
	_ = expectedPosLen
	_ = expectedPayLen

	if _, err = CheckFooter(metaIn); err != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene104 postings reader: check meta footer: %w", err)
	}
	if err = metaIn.Close(); err != nil {
		return nil, fmt.Errorf("lucene104 postings reader: close meta: %w", err)
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
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene104DocExtension)
	docIn, err = state.Directory.OpenInput(docName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene104 postings reader: open doc %q: %w", docName, err)
	}
	if _, err = CheckIndexHeader(
		docIn, lucene104DocCodec, version, version,
		state.SegmentInfo.GetID(), state.SegmentSuffix,
	); err != nil {
		return nil, fmt.Errorf("lucene104 postings reader: check doc header: %w", err)
	}
	if _, err = RetrieveChecksum(docIn); err != nil {
		return nil, fmt.Errorf("lucene104 postings reader: retrieve doc checksum: %w", err)
	}

	if state.FieldInfos.HasProx() {
		posName := GetSegmentFileName(
			state.SegmentInfo.Name(), state.SegmentSuffix, lucene104PosExtension)
		posIn, err = state.Directory.OpenInput(posName, store.IOContext{Context: store.ContextRead})
		if err != nil {
			return nil, fmt.Errorf("lucene104 postings reader: open pos %q: %w", posName, err)
		}
		if _, err = CheckIndexHeader(
			posIn, lucene104PosCodec, version, version,
			state.SegmentInfo.GetID(), state.SegmentSuffix,
		); err != nil {
			return nil, fmt.Errorf("lucene104 postings reader: check pos header: %w", err)
		}
		if _, err = RetrieveChecksum(posIn); err != nil {
			return nil, fmt.Errorf("lucene104 postings reader: retrieve pos checksum: %w", err)
		}
		r.posIn = posIn

		if state.FieldInfos.HasPayloads() || state.FieldInfos.HasOffsets() {
			payName := GetSegmentFileName(
				state.SegmentInfo.Name(), state.SegmentSuffix, lucene104PayExtension)
			payIn, err = state.Directory.OpenInput(payName, store.IOContext{Context: store.ContextRead})
			if err != nil {
				return nil, fmt.Errorf("lucene104 postings reader: open pay %q: %w", payName, err)
			}
			if _, err = CheckIndexHeader(
				payIn, lucene104PayCodec, version, version,
				state.SegmentInfo.GetID(), state.SegmentSuffix,
			); err != nil {
				return nil, fmt.Errorf("lucene104 postings reader: check pay header: %w", err)
			}
			if _, err = RetrieveChecksum(payIn); err != nil {
				return nil, fmt.Errorf("lucene104 postings reader: retrieve pay checksum: %w", err)
			}
			r.payIn = payIn
		}
	}

	r.docIn = docIn
	success = true
	return r, nil
}

// Init validates the terms-in header and block size written by the writer.
//
// Mirrors Lucene104PostingsReader.init(IndexInput, SegmentReadState).
func (r *Lucene104PostingsReader) Init(termsIn store.IndexInput, state *SegmentReadState) error {
	if _, err := CheckIndexHeader(
		termsIn,
		lucene104TermsCodec,
		int32(lucene104VersionStart),
		int32(lucene104VersionCurrent),
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	); err != nil {
		return fmt.Errorf("lucene104 postings reader: init terms header: %w", err)
	}
	blockSize, err := store.ReadVInt(termsIn)
	if err != nil {
		return fmt.Errorf("lucene104 postings reader: read block size: %w", err)
	}
	if int(blockSize) != lucene104BlockSize {
		return fmt.Errorf(
			"lucene104 postings reader: index-time BLOCK_SIZE (%d) != read-time BLOCK_SIZE (%d)",
			blockSize, lucene104BlockSize,
		)
	}
	return nil
}

// NewTermState allocates a fresh IntBlockTermState and registers it in the
// stateCache so that DecodeTerm/Postings can retrieve the extended state later.
func (r *Lucene104PostingsReader) NewTermState() *BlockTermState {
	its := NewIntBlockTermState()
	r.stateCache[its.BlockTermState] = its
	return its.BlockTermState
}

// DecodeTerm reads codec-specific metadata from in into termState.
//
// Mirrors Lucene104PostingsReader.decodeTerm(DataInput, FieldInfo,
// BlockTermState, boolean).
func (r *Lucene104PostingsReader) DecodeTerm(
	in store.IndexInput,
	fieldInfo *index.FieldInfo,
	termState *BlockTermState,
	absolute bool,
) error {
	its := r.stateCache[termState]
	if its == nil {
		// Defensive: allocate on demand (should not happen in normal usage).
		its = NewIntBlockTermState()
		its.BlockTermState = termState
		r.stateCache[termState] = its
	}

	if absolute {
		its.DocStartFP = 0
		its.PosStartFP = 0
		its.PayStartFP = 0
	}

	l, err := store.ReadVLong(in)
	if err != nil {
		return fmt.Errorf("lucene104 decode term: read vlong l: %w", err)
	}

	if l&0x01 == 0 {
		its.DocStartFP += l >> 1
		if termState.DocFreq == 1 {
			v, err2 := store.ReadVInt(in)
			if err2 != nil {
				return fmt.Errorf("lucene104 decode term: read singleton docID: %w", err2)
			}
			its.SingletonDocID = int(v)
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
			return fmt.Errorf("lucene104 decode term: read pos fp delta: %w", err2)
		}
		its.PosStartFP += delta

		if opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets ||
			fieldInfo.HasPayloads() {
			delta2, err3 := store.ReadVLong(in)
			if err3 != nil {
				return fmt.Errorf("lucene104 decode term: read pay fp delta: %w", err3)
			}
			its.PayStartFP += delta2
		}

		if termState.TotalTermFreq > int64(lucene104BlockSize) {
			offset, err4 := store.ReadVLong(in)
			if err4 != nil {
				return fmt.Errorf("lucene104 decode term: read lastPosBlockOffset: %w", err4)
			}
			its.LastPosBlockOffset = offset
		} else {
			its.LastPosBlockOffset = -1
		}
	}
	return nil
}

// Postings returns a BlockPostingsEnum positioned at the term described by
// termState.
//
// Mirrors Lucene104PostingsReader.postings(FieldInfo, BlockTermState,
// PostingsEnum, int).
func (r *Lucene104PostingsReader) Postings(
	fieldInfo *index.FieldInfo,
	termState *BlockTermState,
	reuse index.PostingsEnum,
	flags int,
) (index.PostingsEnum, error) {
	its := r.stateCache[termState]
	if its == nil {
		its = NewIntBlockTermState()
		its.BlockTermState = termState
		r.stateCache[termState] = its
	}

	var bpe *blockPostingsEnum
	if prev, ok := reuse.(*blockPostingsEnum); ok && prev.canReuse(r.docIn, fieldInfo, flags) {
		bpe = prev
	} else {
		var err error
		bpe, err = newBlockPostingsEnum(r, fieldInfo, flags)
		if err != nil {
			return nil, err
		}
	}
	return bpe.reset(its, flags)
}

// Impacts returns a *blockPostingsEnum that also tracks impact data.
//
// Mirrors Lucene104PostingsReader.impacts(FieldInfo, BlockTermState, int).
func (r *Lucene104PostingsReader) Impacts(
	fieldInfo *index.FieldInfo,
	termState *BlockTermState,
	flags int,
) (any, error) {
	its := r.stateCache[termState]
	if its == nil {
		its = NewIntBlockTermState()
		its.BlockTermState = termState
		r.stateCache[termState] = its
	}
	bpe, err := newBlockPostingsEnum(r, fieldInfo, flags)
	if err != nil {
		return nil, err
	}
	bpe.needsImpacts = true
	return bpe.reset(its, flags)
}

// CheckIntegrity validates CRC footers on all owned files.
func (r *Lucene104PostingsReader) CheckIntegrity() error {
	if r.docIn != nil {
		if _, err := ChecksumEntireFile(r.docIn); err != nil {
			return fmt.Errorf("lucene104 postings reader: checksum doc: %w", err)
		}
	}
	if r.posIn != nil {
		if _, err := ChecksumEntireFile(r.posIn); err != nil {
			return fmt.Errorf("lucene104 postings reader: checksum pos: %w", err)
		}
	}
	if r.payIn != nil {
		if _, err := ChecksumEntireFile(r.payIn); err != nil {
			return fmt.Errorf("lucene104 postings reader: checksum pay: %w", err)
		}
	}
	return nil
}

// Close releases file handles owned by this reader.
func (r *Lucene104PostingsReader) Close() error {
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

// ─── readVInt15 / readVLong15 ─────────────────────────────────────────────────

// readVInt15 reads an integer encoded with writeVInt15.
// Mirror of Lucene104PostingsReader.readVInt15(DataInput).
func readVInt15(in store.IndexInput) (int, error) {
	s, err := in.ReadShort()
	if err != nil {
		return 0, err
	}
	if s >= 0 {
		return int(s), nil
	}
	v, err := store.ReadVInt(in)
	if err != nil {
		return 0, err
	}
	return (int(s) & 0x7FFF) | (int(v) << 15), nil
}

// readVLong15 reads a long encoded with writeVLong15.
// Mirror of Lucene104PostingsReader.readVLong15(DataInput).
func readVLong15(in store.IndexInput) (int64, error) {
	s, err := in.ReadShort()
	if err != nil {
		return 0, err
	}
	if s >= 0 {
		return int64(s), nil
	}
	v, err := store.ReadVLong(in)
	if err != nil {
		return 0, err
	}
	return (int64(s) & 0x7FFF) | (v << 15), nil
}

// ─── prefixSum ────────────────────────────────────────────────────────────────

// prefixSum64 computes the prefix (exclusive scan) sum over buffer[0:count],
// starting from base.  After this call buffer[i] holds the absolute doc ID.
// Mirrors Lucene104PostingsReader.prefixSum(int[], int, int).
func prefixSum64(buffer []int64, count int, base int64) {
	sum := base
	for i := range count {
		sum += buffer[i]
		buffer[i] = sum
	}
}

// sumOverRange returns the sum of arr[start:end].
func sumOverRange64(arr []int64, start, end int) int64 {
	var res int64
	for i := start; i < end; i++ {
		res += arr[i]
	}
	return res
}

// ─── blockPostingsEnum ────────────────────────────────────────────────────────

// deltaEncoding identifies which delta encoding a full doc block uses.
type deltaEncoding int

const (
	deltaEncodingPacked deltaEncoding = iota // Frame-Of-Reference packed integers
	deltaEncodingUnary                       // bit-set (unary) encoding
)

// blockPostingsEnum implements index.PostingsEnum for the Lucene104 format.
// It handles PACKED and UNARY doc-delta blocks with two-level skip navigation
// and optional position/payload/offset data.
//
// Mirrors Lucene104PostingsReader.BlockPostingsEnum.
type blockPostingsEnum struct {
	// owning reader (for docIn/posIn/payIn access)
	reader *Lucene104PostingsReader

	forUtil  *ForUtil
	pforUtil *PForUtil

	// ── doc-block state ──────────────────────────────────────────────────────

	encoding  deltaEncoding
	doc       int // current doc ID
	prevDocID int // last doc of the previous block

	// PACKED encoding buffers
	docBuffer  [lucene104BlockSize]int64
	freqBuffer [lucene104BlockSize]int64

	// UNARY encoding: bit-set over BLOCK_SIZE*32 bits
	docBitSet     *util.FixedBitSet
	docBitSetBase int
	// cumulative pop-counts of each word in docBitSet; reuses docBuffer memory
	// logically (separate slice backed by docBuffer for zero-alloc)
	docCumulativeWordPopCounts [lucene104BlockSize]int64

	// ── skip state ───────────────────────────────────────────────────────────

	level0LastDocID int
	level0DocEndFP  int64

	level1LastDocID    int
	level1DocEndFP     int64
	level1DocCountUpto int

	// ── term state ───────────────────────────────────────────────────────────

	docFreq        int
	totalTermFreq  int64
	singletonDocID int

	docCountLeft  int // remaining docs
	docBufferSize int
	docBufferUpto int

	// ── DocInput ─────────────────────────────────────────────────────────────

	docIn store.IndexInput // cloned lazily

	// ── freq state ───────────────────────────────────────────────────────────

	freqFP int64 // -1 when not pending

	// ── position state ───────────────────────────────────────────────────────

	posIn store.IndexInput // nil if not needed, cloned in constructor

	posDeltaBuffer [lucene104BlockSize]int64
	posBufferUpto  int

	// position within current document iteration
	posPendingCount  int
	posDocBufferUpto int

	lastPosBlockFP int64
	position       int

	// ── payload/offset state ─────────────────────────────────────────────────

	payIn store.IndexInput // nil if not needed, cloned in constructor

	payloadLengthBuffer    []int64
	offsetStartDeltaBuffer []int64
	offsetLengthBuffer     []int64

	payloadBytes    []byte
	payloadByteUpto int
	payloadLength   int

	lastStartOffset int
	startOffset     int
	endOffset       int

	// ── level-0 skip data for pos/pay ────────────────────────────────────────

	level0PosEndFP     int64
	level0BlockPosUpto int
	level0PayEndFP     int64
	level0BlockPayUpto int

	// serialised impact bytes for scoring
	level0SerializedImpacts []byte
	level0ImpactLen         int

	// ── level-1 skip data for pos/pay ────────────────────────────────────────

	level1PosEndFP     int64
	level1BlockPosUpto int
	level1PayEndFP     int64
	level1BlockPayUpto int

	level1SerializedImpacts []byte
	level1ImpactLen         int

	// ── field characteristics ─────────────────────────────────────────────────

	options                   index.IndexOptions
	indexHasFreq              bool
	indexHasPos               bool
	indexHasOffsets           bool
	indexHasPayloads          bool
	indexHasOffsetsOrPayloads bool

	flags                  int
	needsFreq              bool
	needsPos               bool
	needsOffsets           bool
	needsPayloads          bool
	needsOffsetsOrPayloads bool
	needsImpacts           bool
	needsDocsAndFreqsOnly  bool

	needsRefilling bool
}

// newBlockPostingsEnum constructs a new blockPostingsEnum. Clones of posIn/payIn
// are taken here if the field/flags require them.
func newBlockPostingsEnum(
	r *Lucene104PostingsReader,
	fieldInfo *index.FieldInfo,
	flags int,
) (*blockPostingsEnum, error) {
	e := &blockPostingsEnum{
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
			return nil, fmt.Errorf("lucene104 postings reader: needsPos but posIn is nil")
		}
		e.posIn = r.posIn.Clone()
	}

	if e.needsOffsets || e.needsPayloads {
		if r.payIn == nil {
			return nil, fmt.Errorf("lucene104 postings reader: needsOffsets/Payloads but payIn is nil")
		}
		e.payIn = r.payIn.Clone()
	}

	if e.needsOffsets {
		e.offsetStartDeltaBuffer = make([]int64, lucene104BlockSize)
		e.offsetLengthBuffer = make([]int64, lucene104BlockSize)
	}

	if e.indexHasPayloads {
		e.payloadLengthBuffer = make([]int64, lucene104BlockSize)
		e.payloadBytes = make([]byte, 128)
	}

	// Impact scratch buffers: allocated only when impacts are tracked.
	if e.needsFreq {
		e.level0SerializedImpacts = make([]byte, r.maxImpactNumBytesAtLevel0)
		e.level1SerializedImpacts = make([]byte, r.maxImpactNumBytesAtLevel1)
	}

	// docBitSet: BLOCK_SIZE * Integer.SIZE bits = 256 * 32 = 8192 bits.
	var err error
	e.docBitSet, err = util.NewFixedBitSet(lucene104BlockSize * 32)
	if err != nil {
		return nil, fmt.Errorf("lucene104 postings reader: allocate docBitSet: %w", err)
	}

	return e, nil
}

// featureRequested reports whether a PostingsEnum flag is requested.
func featureRequested(flags, feature int) bool {
	return flags&feature != 0
}

// canReuse returns true if this enum can be reset for the same docIn/field/flags.
func (e *blockPostingsEnum) canReuse(docIn store.IndexInput, fi *index.FieldInfo, flags int) bool {
	return docIn == e.reader.docIn &&
		e.options == fi.IndexOptions() &&
		e.indexHasPayloads == fi.HasPayloads() &&
		e.flags == flags
}

// reset repositions the enum at the start of the given term state.
// Returns the enum itself (as index.PostingsEnum) for chaining.
func (e *blockPostingsEnum) reset(termState *IntBlockTermState, flags int) (index.PostingsEnum, error) {
	e.docFreq = termState.DocFreq
	e.singletonDocID = termState.SingletonDocID
	e.totalTermFreq = termState.TotalTermFreq
	if !e.indexHasFreq {
		e.totalTermFreq = int64(termState.DocFreq)
	}

	// Lazy-init ForUtil/PForUtil only when needed.
	if e.forUtil == nil && e.docFreq >= lucene104BlockSize {
		e.forUtil = NewForUtil()
	}
	if e.needsFreq && e.pforUtil == nil && e.totalTermFreq >= int64(lucene104BlockSize) {
		if e.forUtil == nil {
			e.forUtil = NewForUtil()
		}
		e.pforUtil = NewPForUtil(e.forUtil)
	}

	if e.docFreq > 1 {
		// Lazy-init docIn clone.
		if e.docIn == nil {
			e.docIn = e.reader.docIn.Clone()
		}
	}

	posTermStartFP := termState.PosStartFP
	payTermStartFP := termState.PayStartFP
	if e.posIn != nil {
		if err := e.posIn.SetPosition(posTermStartFP); err != nil {
			return nil, fmt.Errorf("lucene104 postings enum reset: seek posIn: %w", err)
		}
		if e.payIn != nil {
			if err := e.payIn.SetPosition(payTermStartFP); err != nil {
				return nil, fmt.Errorf("lucene104 postings enum reset: seek payIn: %w", err)
			}
		}
	}

	e.level1PosEndFP = posTermStartFP
	e.level1PayEndFP = payTermStartFP
	e.level0PosEndFP = posTermStartFP
	e.level0PayEndFP = payTermStartFP
	e.posPendingCount = 0
	e.payloadByteUpto = 0

	if termState.TotalTermFreq < int64(lucene104BlockSize) {
		e.lastPosBlockFP = posTermStartFP
	} else if termState.TotalTermFreq == int64(lucene104BlockSize) {
		e.lastPosBlockFP = -1
	} else {
		e.lastPosBlockFP = posTermStartFP + termState.LastPosBlockOffset
	}

	e.level1BlockPosUpto = 0
	e.level1BlockPayUpto = 0
	e.level0BlockPosUpto = 0
	e.level0BlockPayUpto = 0
	e.posBufferUpto = lucene104BlockSize

	e.doc = -1
	e.prevDocID = -1
	e.docCountLeft = e.docFreq
	e.freqFP = -1
	e.level0LastDocID = -1

	if e.docFreq < lucene104Level1NumDocs {
		e.level1LastDocID = index.NO_MORE_DOCS
		if e.docFreq > 1 {
			if err := e.docIn.SetPosition(termState.DocStartFP); err != nil {
				return nil, fmt.Errorf("lucene104 postings enum reset: seek docIn: %w", err)
			}
		}
	} else {
		e.level1LastDocID = -1
		e.level1DocEndFP = termState.DocStartFP
	}
	e.level1DocCountUpto = 0
	e.docBufferSize = lucene104BlockSize
	e.docBufferUpto = lucene104BlockSize
	e.posDocBufferUpto = lucene104BlockSize
	e.needsRefilling = false

	return e, nil
}

// ─── PostingsEnum interface ───────────────────────────────────────────────────

// DocID returns the current doc ID.
func (e *blockPostingsEnum) DocID() int {
	return e.doc
}

// Cost returns the estimated iteration cost (docFreq).
func (e *blockPostingsEnum) Cost() int64 {
	return int64(e.docFreq)
}

// Freq returns the frequency for the current document.
func (e *blockPostingsEnum) Freq() (int, error) {
	if e.freqFP != -1 {
		if err := e.docIn.SetPosition(e.freqFP); err != nil {
			return 0, err
		}
		if err := e.pforUtil.Decode(e.docIn, e.freqBuffer[:]); err != nil {
			return 0, err
		}
		e.freqFP = -1
	}
	return int(e.freqBuffer[e.docBufferUpto-1]), nil
}

// StartOffset returns the start character offset of the current occurrence, or
// -1 if offsets were not indexed or not requested.
func (e *blockPostingsEnum) StartOffset() (int, error) {
	return e.startOffset, nil
}

// EndOffset returns the end character offset of the current occurrence, or -1
// if offsets were not indexed or not requested.
func (e *blockPostingsEnum) EndOffset() (int, error) {
	return e.endOffset, nil
}

// GetPayload returns the payload bytes for the current occurrence, or nil.
func (e *blockPostingsEnum) GetPayload() ([]byte, error) {
	if !e.needsPayloads || e.payloadLength == 0 {
		return nil, nil
	}
	// Return the slice of payloadBytes for the current position.
	// Note: this points into the shared byte slice; caller must not retain.
	start := e.payloadByteUpto - e.payloadLength
	if start < 0 {
		start = 0
	}
	return e.payloadBytes[start : start+e.payloadLength], nil
}

// ─── doc-block decoding ───────────────────────────────────────────────────────

// refillFullBlock decodes the next full block (BLOCK_SIZE docs) from docIn.
// Mirrors BlockPostingsEnum.refillFullBlock().
func (e *blockPostingsEnum) refillFullBlock() error {
	bpv, err := e.docIn.ReadByte()
	if err != nil {
		return fmt.Errorf("lucene104 refillFullBlock: read bpv: %w", err)
	}

	intBPV := int(int8(bpv)) // interpret as signed byte (negative = bit-set)

	if intBPV > 0 {
		// PACKED encoding: bpv bits per value via FOR.
		if err := e.forUtil.Decode(intBPV, e.docIn, e.docBuffer[:]); err != nil {
			return fmt.Errorf("lucene104 refillFullBlock: FOR decode: %w", err)
		}
		prefixSum64(e.docBuffer[:], lucene104BlockSize, int64(e.prevDocID))
		e.encoding = deltaEncodingPacked
	} else {
		// UNARY / bit-set encoding.
		e.docBitSetBase = e.prevDocID + 1
		var numLongs int
		if intBPV == 0 {
			// All 256 docs are consecutive: dense block.
			numLongs = lucene104BlockSize / 64 // = 4
			e.docBitSet.SetAll()
			// Clear words beyond numLongs (SetAll fills the whole bit-set).
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
					return fmt.Errorf("lucene104 refillFullBlock: read bit-set word %d: %w", i, err2)
				}
				words[i] = uint64(raw)
			}
		}
		if e.needsFreq {
			words := e.docBitSet.GetBits()
			for i := 0; i < numLongs-1; i++ {
				e.docCumulativeWordPopCounts[i] = int64(bits.OnesCount64(words[i]))
			}
			// prefix-sum the pop counts
			var acc int64
			for i := 0; i < numLongs-1; i++ {
				acc += e.docCumulativeWordPopCounts[i]
				e.docCumulativeWordPopCounts[i] = acc
			}
			e.docCumulativeWordPopCounts[numLongs-1] = lucene104BlockSize
		}
		e.encoding = deltaEncodingUnary
	}

	if e.indexHasFreq {
		if e.needsFreq {
			e.freqFP = e.docIn.GetFilePointer()
		}
		if err2 := PForUtilSkip(e.docIn); err2 != nil {
			return fmt.Errorf("lucene104 refillFullBlock: skip freq block: %w", err2)
		}
	}

	e.docCountLeft -= lucene104BlockSize
	if e.encoding == deltaEncodingPacked {
		e.prevDocID = int(e.docBuffer[lucene104BlockSize-1])
	} else {
		// For UNARY the last doc in this block is docBitSetBase + (lastSetBit).
		e.prevDocID = e.docBitSetBase + e.docBitSet.PrevSetBit(lucene104BlockSize*32-1)
	}
	e.docBufferUpto = 0
	e.posDocBufferUpto = 0
	return nil
}

// refillRemainder decodes a tail (< BLOCK_SIZE docs) block using VInt/GroupVInt.
// Mirrors BlockPostingsEnum.refillRemainder().
func (e *blockPostingsEnum) refillRemainder() error {
	if e.docFreq == 1 {
		e.docBuffer[0] = int64(e.singletonDocID)
		e.freqBuffer[0] = e.totalTermFreq
		e.docBuffer[1] = index.NO_MORE_DOCS
		e.freqFP = -1
		e.docCountLeft = 0
		e.docBufferSize = 1
	} else {
		// Read VInt/GroupVInt-encoded tail block.
		if err := readVIntBlock104(e.docIn, e.docBuffer[:], e.freqBuffer[:],
			e.docCountLeft, e.indexHasFreq, e.needsFreq); err != nil {
			return fmt.Errorf("lucene104 refillRemainder: %w", err)
		}
		prefixSum64(e.docBuffer[:], e.docCountLeft, int64(e.prevDocID))
		e.docBuffer[e.docCountLeft] = index.NO_MORE_DOCS
		e.freqFP = -1
		e.docBufferSize = e.docCountLeft
		e.docCountLeft = 0
	}
	e.prevDocID = int(e.docBuffer[lucene104BlockSize-1])
	e.docBufferUpto = 0
	e.posDocBufferUpto = 0
	e.encoding = deltaEncodingPacked
	return nil
}

// refillDocs dispatches to refillFullBlock or refillRemainder.
func (e *blockPostingsEnum) refillDocs() error {
	if e.docCountLeft >= lucene104BlockSize {
		return e.refillFullBlock()
	}
	return e.refillRemainder()
}

// ─── skip navigation ──────────────────────────────────────────────────────────

// skipLevel1To advances level-1 skip data until level1LastDocID >= target.
// Mirrors BlockPostingsEnum.skipLevel1To(int).
func (e *blockPostingsEnum) skipLevel1To(target int) error {
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
		e.level1DocCountUpto += lucene104Level1NumDocs

		if e.docCountLeft < lucene104Level1NumDocs {
			e.level1LastDocID = index.NO_MORE_DOCS
			break
		}

		delta1, err := store.ReadVInt(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene104 skipLevel1To: read level1DocDelta: %w", err)
		}
		e.level1LastDocID += int(delta1)

		delta2, err := store.ReadVLong(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene104 skipLevel1To: read level1DocEndFP delta: %w", err)
		}
		e.level1DocEndFP = delta2 + e.docIn.GetFilePointer()

		if e.indexHasFreq {
			skip1EndFPDelta, err2 := e.docIn.ReadShort()
			if err2 != nil {
				return fmt.Errorf("lucene104 skipLevel1To: read skip1EndFP: %w", err2)
			}
			skip1EndFP := int64(skip1EndFPDelta) + e.docIn.GetFilePointer()

			numImpactBytes, err3 := e.docIn.ReadShort()
			if err3 != nil {
				return fmt.Errorf("lucene104 skipLevel1To: read numImpactBytes: %w", err3)
			}

			if e.needsImpacts && e.level1LastDocID >= target {
				if err4 := e.docIn.ReadBytes(e.level1SerializedImpacts[:numImpactBytes]); err4 != nil {
					return fmt.Errorf("lucene104 skipLevel1To: read impact bytes: %w", err4)
				}
				e.level1ImpactLen = int(numImpactBytes)
			} else {
				if err4 := skipBytesInput(e.docIn, int64(numImpactBytes)); err4 != nil {
					return fmt.Errorf("lucene104 skipLevel1To: skip impact bytes: %w", err4)
				}
			}

			if e.indexHasPos {
				posEndFPDelta, err5 := store.ReadVLong(e.docIn)
				if err5 != nil {
					return fmt.Errorf("lucene104 skipLevel1To: read posEndFP delta: %w", err5)
				}
				e.level1PosEndFP += posEndFPDelta

				posUpto, err6 := e.docIn.ReadByte()
				if err6 != nil {
					return fmt.Errorf("lucene104 skipLevel1To: read posUpto: %w", err6)
				}
				e.level1BlockPosUpto = int(posUpto) & 0xFF

				if e.indexHasOffsetsOrPayloads {
					payEndFPDelta, err7 := store.ReadVLong(e.docIn)
					if err7 != nil {
						return fmt.Errorf("lucene104 skipLevel1To: read payEndFP delta: %w", err7)
					}
					e.level1PayEndFP += payEndFPDelta

					payUpto, err8 := store.ReadVInt(e.docIn)
					if err8 != nil {
						return fmt.Errorf("lucene104 skipLevel1To: read payUpto: %w", err8)
					}
					e.level1BlockPayUpto = int(payUpto)
				}
			}
			if err9 := e.docIn.SetPosition(skip1EndFP); err9 != nil {
				return fmt.Errorf("lucene104 skipLevel1To: seek to skip1End: %w", err9)
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
func (e *blockPostingsEnum) readLevel0PosData() error {
	posEndFPDelta, err := store.ReadVLong(e.docIn)
	if err != nil {
		return err
	}
	e.level0PosEndFP += posEndFPDelta

	posUpto, err2 := e.docIn.ReadByte()
	if err2 != nil {
		return err2
	}
	e.level0BlockPosUpto = int(posUpto) & 0xFF

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

// seekPosData repositions posIn/payIn to the given file pointers, or
// accumulates pending positions when the block is already past the target.
// Mirrors BlockPostingsEnum.seekPosData(long, int, long, int).
func (e *blockPostingsEnum) seekPosData(posFP int64, posUpto int, payFP int64, payUpto int) error {
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
		e.posBufferUpto = lucene104BlockSize
	} else {
		e.posPendingCount += int(sumOverRange64(e.freqBuffer[:], e.posDocBufferUpto, lucene104BlockSize))
	}
	return nil
}

// skipLevel0To advances level-0 skip data until level0LastDocID >= target.
// Mirrors BlockPostingsEnum.skipLevel0To(int).
func (e *blockPostingsEnum) skipLevel0To(target int) error {
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

		if e.docCountLeft < lucene104BlockSize {
			e.level0LastDocID = index.NO_MORE_DOCS
			break
		}

		numSkipBytes, err := store.ReadVLong(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene104 skipLevel0To: read numSkipBytes: %w", err)
		}
		skip0End := e.docIn.GetFilePointer() + numSkipBytes

		docDelta, err2 := readVInt15(e.docIn)
		if err2 != nil {
			return fmt.Errorf("lucene104 skipLevel0To: read docDelta: %w", err2)
		}
		e.level0LastDocID += docDelta
		found := target <= e.level0LastDocID

		blockLen, err3 := readVLong15(e.docIn)
		if err3 != nil {
			return fmt.Errorf("lucene104 skipLevel0To: read blockLen: %w", err3)
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
					return fmt.Errorf("lucene104 skipLevel0To: read numImpactBytes: %w", err5)
				}
				if e.needsImpacts && found {
					if err6 := e.docIn.ReadBytes(e.level0SerializedImpacts[:numImpactBytes]); err6 != nil {
						return fmt.Errorf("lucene104 skipLevel0To: read impact bytes: %w", err6)
					}
					e.level0ImpactLen = int(numImpactBytes)
				} else {
					if err6 := skipBytesInput(e.docIn, int64(numImpactBytes)); err6 != nil {
						return fmt.Errorf("lucene104 skipLevel0To: skip impact bytes: %w", err6)
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
		e.docCountLeft -= lucene104BlockSize
	}

	return e.seekPosData(posFP, posUpto, payFP, payUpto)
}

// doAdvanceShallow advances both level-1 and level-0 skip data.
// Mirrors BlockPostingsEnum.doAdvanceShallow(int).
func (e *blockPostingsEnum) doAdvanceShallow(target int) error {
	if target > e.level1LastDocID {
		if err := e.skipLevel1To(target); err != nil {
			return err
		}
	} else if e.needsRefilling {
		if err := e.docIn.SetPosition(e.level0DocEndFP); err != nil {
			return err
		}
		e.docCountLeft -= lucene104BlockSize
	}
	return e.skipLevel0To(target)
}

// doMoveToNextLevel0Block decodes skip data and refills the doc block for the
// next level-0 block.
// Mirrors BlockPostingsEnum.doMoveToNextLevel0Block().
func (e *blockPostingsEnum) doMoveToNextLevel0Block() error {
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
			e.posBufferUpto = lucene104BlockSize
		} else {
			e.posPendingCount += int(sumOverRange64(e.freqBuffer[:], e.posDocBufferUpto, lucene104BlockSize))
		}
	}

	if e.docCountLeft >= lucene104BlockSize {
		// Read level-0 skip header.
		level0NumBytes, err := store.ReadVLong(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene104 doMoveToNextLevel0Block: read level0NumBytes: %w", err)
		}
		_ = level0NumBytes

		docDelta, err2 := readVInt15(e.docIn)
		if err2 != nil {
			return fmt.Errorf("lucene104 doMoveToNextLevel0Block: read docDelta: %w", err2)
		}
		e.level0LastDocID += docDelta

		blockLen, err3 := readVLong15(e.docIn)
		if err3 != nil {
			return fmt.Errorf("lucene104 doMoveToNextLevel0Block: read blockLen: %w", err3)
		}
		e.level0DocEndFP = e.docIn.GetFilePointer() + blockLen

		if e.indexHasFreq {
			numImpactBytes, err4 := store.ReadVInt(e.docIn)
			if err4 != nil {
				return fmt.Errorf("lucene104 doMoveToNextLevel0Block: read numImpactBytes: %w", err4)
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

	e.level0LastDocID = index.NO_MORE_DOCS
	return e.refillRemainder()
}

// moveToNextLevel0Block moves to the next level-0 block, possibly upgrading
// level-1 skip data first.
// Mirrors BlockPostingsEnum.moveToNextLevel0Block().
func (e *blockPostingsEnum) moveToNextLevel0Block() error {
	if e.doc == e.level1LastDocID {
		if err := e.skipLevel1To(e.doc + 1); err != nil {
			return err
		}
	}

	e.prevDocID = e.level0LastDocID

	if e.needsDocsAndFreqsOnly && e.docCountLeft >= lucene104BlockSize {
		level0NumBytes, err := store.ReadVLong(e.docIn)
		if err != nil {
			return fmt.Errorf("lucene104 moveToNextLevel0Block: read level0NumBytes: %w", err)
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

// NextDoc advances to the next document.
// Mirrors BlockPostingsEnum.nextDoc().
func (e *blockPostingsEnum) NextDoc() (int, error) {
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
func (e *blockPostingsEnum) Advance(target int) (int, error) {
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
func (e *blockPostingsEnum) skipPositions(freq int) error {
	toSkip := e.posPendingCount - freq
	leftInBlock := lucene104BlockSize - e.posBufferUpto
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
	for toSkip >= lucene104BlockSize {
		if err := PForUtilSkip(e.posIn); err != nil {
			return err
		}
		if e.payIn != nil {
			if e.indexHasPayloads {
				if err := PForUtilSkip(e.payIn); err != nil {
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
				if err := PForUtilSkip(e.payIn); err != nil {
					return err
				}
				if err2 := PForUtilSkip(e.payIn); err2 != nil {
					return err2
				}
			}
		}
		toSkip -= lucene104BlockSize
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
func (e *blockPostingsEnum) refillLastPositionBlock() error {
	count := int(e.totalTermFreq % int64(lucene104BlockSize))
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
				_ = skipBytesInput(e.posIn, int64(payloadLength)) // skip unwanted payload bytes
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
func (e *blockPostingsEnum) refillOffsetsOrPayloads() error {
	if e.indexHasPayloads {
		if e.needsPayloads {
			if err := e.pforUtil.Decode(e.payIn, e.payloadLengthBuffer); err != nil {
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
			// needsOffsets only: skip payload blocks
			if err := PForUtilSkip(e.payIn); err != nil {
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
			if err := e.pforUtil.Decode(e.payIn, e.offsetStartDeltaBuffer); err != nil {
				return err
			}
			if err2 := e.pforUtil.Decode(e.payIn, e.offsetLengthBuffer); err2 != nil {
				return err2
			}
		} else if e.payIn != nil {
			// needsPayloads only: skip offset blocks
			if err := PForUtilSkip(e.payIn); err != nil {
				return err
			}
			if err2 := PForUtilSkip(e.payIn); err2 != nil {
				return err2
			}
		}
	}
	return nil
}

// refillPositions decodes the next 256-position block from posIn.
// Mirrors BlockPostingsEnum.refillPositions().
func (e *blockPostingsEnum) refillPositions() error {
	if e.posIn.GetFilePointer() == e.lastPosBlockFP {
		return e.refillLastPositionBlock()
	}
	if err := e.pforUtil.Decode(e.posIn, e.posDeltaBuffer[:]); err != nil {
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
func (e *blockPostingsEnum) accumulatePendingPositions() error {
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
func (e *blockPostingsEnum) NextPosition() (int, error) {
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

	if e.posBufferUpto == lucene104BlockSize {
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

// ─── helpers ─────────────────────────────────────────────────────────────────

// findNextGEQ64 returns the smallest index i in buf[from:to] such that
// buf[i] >= target, or to if none found.
// Mirrors Lucene's VectorUtil.findNextGEQ for the int64 variant.
func findNextGEQ64(buf []int64, target, from, to int) int {
	for i := from; i < to; i++ {
		if buf[i] >= int64(target) {
			return i
		}
	}
	return to
}

// readVIntBlock104 decodes a tail block of <BLOCK_SIZE docs using
// GroupVInt + optional VInt freqs.  Mirrors PostingsUtil.readVIntBlock.
func readVIntBlock104(
	docIn store.IndexInput,
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

// skipBytesInput skips n bytes on an IndexInput by seeking forward.
// IndexInput.SkipBytes is on BaseIndexInput but not on the interface;
// this helper uses SetPosition instead.
func skipBytesInput(in store.IndexInput, n int64) error {
	if n == 0 {
		return nil
	}
	return in.SetPosition(in.GetFilePointer() + n)
}

// Compile-time check: blockPostingsEnum implements index.PostingsEnum.
var _ index.PostingsEnum = (*blockPostingsEnum)(nil)

// Compile-time check: Lucene104PostingsReader implements PostingsReaderBase.
var _ PostingsReaderBase = (*Lucene104PostingsReader)(nil)
