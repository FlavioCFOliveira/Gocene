// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene912

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// noMoreDocs is the sentinel that signals end of enumeration in the docBuffer.
// Matches Java's PostingsEnum.NO_MORE_DOCS = Integer.MAX_VALUE in the Java
// reader; here we use index.NO_MORE_DOCS which is -1 in Gocene. However, the
// Java code stores NO_MORE_DOCS as a *positive* sentinel in the buffer because
// it uses Integer.MAX_VALUE. Gocene's index.NO_MORE_DOCS = -1.
//
// We keep the same trick: docBuffer[BLOCK_SIZE] is always set to math.MaxInt32
// so that findNextGEQ terminates naturally, but we *return* index.NO_MORE_DOCS
// (-1) to the caller when the value equals math.MaxInt32.
const postingsNoMoreDocsBuffer = math.MaxInt32

// prefetchPostings hints to the OS that the postings data starting at
// termState.DocStartFP should be loaded into cache. Falls back to a no-op
// when the IndexInput does not implement the optional Prefetchable interface.
func prefetchPostings(docIn store.IndexInput, its *IntBlockTermState) error {
	// If we are already at the right FP, streaming is likely; skip prefetch.
	if docIn.GetFilePointer() == its.DocStartFP {
		return nil
	}
	type prefetchable interface {
		Prefetch(offset int64, length int64) error
	}
	if p, ok := docIn.(prefetchable); ok {
		return p.Prefetch(its.DocStartFP, 1)
	}
	return nil
}

// prefixSum computes the prefix sum of buffer[0..count-1] with the given base.
func postingsPrefixSum(buffer []int64, count int, base int64) {
	buffer[0] += base
	for i := 1; i < count; i++ {
		buffer[i] += buffer[i-1]
	}
}

// findNextGEQ returns the smallest index i in buffer[from..to-1] such that
// buffer[i] >= target, or to if none exists.
func findNextGEQ(buffer []int64, target int64, from, to int) int {
	for i := from; i < to; i++ {
		if buffer[i] >= target {
			return i
		}
	}
	return to
}

// sumOverRange returns the sum of arr[start..end-1].
func sumOverRange(arr []int64, start, end int) int64 {
	var res int64
	for i := start; i < end; i++ {
		res += arr[i]
	}
	return res
}

// readVInt15 reads a compact variable-length integer that fits in 15 bits.
// If the high bit of the first short is clear (s >= 0), the value is the
// short itself.  Otherwise the remaining bits are in the next VInt.
// Matches Lucene912PostingsReader.readVInt15.
func readVInt15(in store.DataInput) (int, error) {
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

// readVLong15 reads a compact variable-length long that fits in 15 bits.
// Matches Lucene912PostingsReader.readVLong15.
func readVLong15(in store.DataInput) (int64, error) {
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

// readImpacts deserialises (freq, norm) pairs from a ByteArrayDataInput.
// Matches Lucene912PostingsReader.readImpacts.
func readImpacts(in *store.ByteArrayDataInput, buf *index.FreqAndNormBuffer) {
	var freq int
	var norm int64
	size := 0
	for in.GetPosition() < in.Length() {
		freqDelta, _ := store.ReadVInt(in)
		if freqDelta&0x01 != 0 {
			freq += 1 + int(freqDelta>>1)
			z, _ := store.ReadVLong(in) // zigzag-encoded delta
			norm += 1 + zigZagDecodeInt64(z)
		} else {
			freq += 1 + int(freqDelta>>1)
			norm++
		}
		buf.Freqs[size] = freq
		buf.Norms[size] = norm
		size++
	}
	buf.Size = size
}

// zigZagDecodeInt64 decodes a zigzag-encoded int64 (matches Java BitUtil.zigZagDecode).
func zigZagDecodeInt64(v int64) int64 {
	return int64(uint64(v)>>1) ^ -(v & 1)
}

// ─── blockDocsEnum ──────────────────────────────────────────────────────────

// blockDocsEnum decodes doc IDs and optionally term frequencies for a single
// term.  It handles both full-block (bit-packed) and residual (vInt) decoding,
// and implements the two-level skip index (level 0 = one packed block, level 1
// = Level1NumDocs docs).
//
// Mirrors org.apache.lucene.backward_codecs.lucene912.Lucene912PostingsReader.BlockDocsEnum.
type blockDocsEnum struct {
	reader *Lucene912PostingsReader

	pforUtil     *pforUtil
	forDeltaUtil *forDeltaUtil

	docBuffer  [BlockSize + 1]int64
	freqBuffer [BlockSize]int64

	indexHasFreq bool
	needsFreq    bool
	freqFP       int64

	doc            int
	docFreq        int
	totalTermFreq  int64
	singletonDocID int

	docCountUpto  int
	prevDocID     int64
	docBufferSize int
	docBufferUpto int

	// level 0 skip
	level0LastDocID int

	// level 1 skip
	level1LastDocID    int
	level1DocEndFP     int64
	level1DocCountUpto int

	docIn store.IndexInput // cloned per-term
}

func newBlockDocsEnum(r *Lucene912PostingsReader, fieldInfo *index.FieldInfo) *blockDocsEnum {
	opts := fieldInfo.IndexOptions()
	e := &blockDocsEnum{
		reader:       r,
		indexHasFreq: opts >= index.IndexOptionsDocsAndFreqs,
		freqFP:       -1,
	}
	e.docBuffer[BlockSize] = postingsNoMoreDocsBuffer
	return e
}

func (e *blockDocsEnum) canReuse(docIn store.IndexInput, fieldInfo *index.FieldInfo) bool {
	if e.reader == nil {
		return false
	}
	// Check that the docIn is the same object as the reader's docIn (pointer eq).
	// In Go we compare interface values; this is always true when the reader is the same.
	opts := fieldInfo.IndexOptions()
	return e.indexHasFreq == (opts >= index.IndexOptionsDocsAndFreqs)
}

func (e *blockDocsEnum) reset(its *IntBlockTermState, flags int) (index.PostingsEnum, error) {
	e.docFreq = its.BlockTermState.DocFreq
	e.singletonDocID = its.SingletonDocID
	if e.docFreq > 1 {
		if e.docIn == nil {
			e.docIn = e.reader.docIn.Clone()
		}
		if err := prefetchPostings(e.docIn, its); err != nil {
			return nil, err
		}
	}

	if e.pforUtil == nil && e.docFreq >= BlockSize {
		e.pforUtil = &pforUtil{}
		e.forDeltaUtil = &forDeltaUtil{}
	}
	if e.indexHasFreq {
		e.totalTermFreq = its.BlockTermState.TotalTermFreq
	} else {
		e.totalTermFreq = int64(e.docFreq)
	}

	e.needsFreq = (flags & index.PostingsFlagFreqs) != 0
	if !e.indexHasFreq || !e.needsFreq {
		for i := 0; i < min(BlockSize, e.docFreq); i++ {
			e.freqBuffer[i] = 1
		}
	}
	e.freqFP = -1
	e.doc = -1
	e.prevDocID = -1
	e.docCountUpto = 0
	e.level0LastDocID = -1
	if e.docFreq < Level1NumDocs {
		e.level1LastDocID = postingsNoMoreDocsBuffer
		if e.docFreq > 1 {
			if err := e.docIn.SetPosition(its.DocStartFP); err != nil {
				return nil, err
			}
		}
	} else {
		e.level1LastDocID = -1
		e.level1DocEndFP = its.DocStartFP
	}
	e.level1DocCountUpto = 0
	e.docBufferSize = BlockSize
	e.docBufferUpto = BlockSize
	return e, nil
}

func (e *blockDocsEnum) NextDoc() (int, error) {
	if e.docBufferUpto == BlockSize {
		if err := e.moveToNextLevel0Block(); err != nil {
			return 0, err
		}
	}
	e.doc = int(e.docBuffer[e.docBufferUpto])
	e.docBufferUpto++
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS, nil
	}
	return e.doc, nil
}

func (e *blockDocsEnum) Advance(target int) (int, error) {
	if int64(target) > int64(e.level0LastDocID) {
		if int64(target) > int64(e.level1LastDocID) {
			if err := e.skipLevel1To(target); err != nil {
				return 0, err
			}
		}
		if err := e.skipLevel0To(target); err != nil {
			return 0, err
		}
		if e.docFreq-e.docCountUpto >= BlockSize {
			if err := e.refillFullBlock(); err != nil {
				return 0, err
			}
		} else {
			if err := e.refillRemainder(); err != nil {
				return 0, err
			}
		}
	}
	next := findNextGEQ(e.docBuffer[:], int64(target), e.docBufferUpto, e.docBufferSize)
	e.doc = int(e.docBuffer[next])
	e.docBufferUpto = next + 1
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS, nil
	}
	return e.doc, nil
}

func (e *blockDocsEnum) DocID() int {
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS
	}
	return e.doc
}

func (e *blockDocsEnum) Freq() (int, error) {
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

func (e *blockDocsEnum) NextPosition() (int, error)  { return -1, nil }
func (e *blockDocsEnum) StartOffset() (int, error)   { return -1, nil }
func (e *blockDocsEnum) EndOffset() (int, error)     { return -1, nil }
func (e *blockDocsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (e *blockDocsEnum) Cost() int64                 { return int64(e.docFreq) }

func (e *blockDocsEnum) refillFullBlock() error {
	if err := e.forDeltaUtil.decodeAndPrefixSum(e.docIn, e.prevDocID, e.docBuffer[:]); err != nil {
		return err
	}
	if e.indexHasFreq {
		if e.needsFreq {
			e.freqFP = e.docIn.GetFilePointer()
		}
		if err := pforUtilSkip(e.docIn); err != nil {
			return err
		}
	}
	e.docCountUpto += BlockSize
	e.prevDocID = e.docBuffer[BlockSize-1]
	e.docBufferUpto = 0
	return nil
}

func (e *blockDocsEnum) refillRemainder() error {
	left := e.docFreq - e.docCountUpto
	if left == 0 {
		e.docBuffer[0] = postingsNoMoreDocsBuffer
		e.docBufferUpto = 0
		e.docBufferSize = 0
		return nil
	}
	if e.docFreq == 1 {
		e.docBuffer[0] = int64(e.singletonDocID)
		e.freqBuffer[0] = e.totalTermFreq
		e.docBuffer[1] = postingsNoMoreDocsBuffer
		e.docCountUpto++
	} else {
		if err := ReadVIntBlock(e.docIn, e.docBuffer[:], e.freqBuffer[:], left, e.indexHasFreq, e.needsFreq); err != nil {
			return err
		}
		postingsPrefixSum(e.docBuffer[:left], left, e.prevDocID)
		e.docBuffer[left] = postingsNoMoreDocsBuffer
		e.docCountUpto += left
	}
	e.docBufferUpto = 0
	e.docBufferSize = left
	e.freqFP = -1
	return nil
}

func (e *blockDocsEnum) skipLevel1To(target int) error {
	for {
		e.prevDocID = int64(e.level1LastDocID)
		e.level0LastDocID = e.level1LastDocID
		if err := e.docIn.SetPosition(e.level1DocEndFP); err != nil {
			return err
		}
		e.docCountUpto = e.level1DocCountUpto
		e.level1DocCountUpto += Level1NumDocs

		if e.docFreq-e.docCountUpto < Level1NumDocs {
			e.level1LastDocID = postingsNoMoreDocsBuffer
			break
		}

		delta, err := store.ReadVInt(e.docIn)
		if err != nil {
			return err
		}
		e.level1LastDocID += int(delta)

		endFPDelta, err := store.ReadVLong(e.docIn)
		if err != nil {
			return err
		}
		e.level1DocEndFP = endFPDelta + e.docIn.GetFilePointer()

		if e.level1LastDocID >= target {
			if e.indexHasFreq {
				s, err := e.docIn.ReadShort()
				if err != nil {
					return err
				}
				if err := e.docIn.SetPosition(e.docIn.GetFilePointer() + int64(int16(s))); err != nil {
					return err
				}
			}
			break
		}
	}
	return nil
}

func (e *blockDocsEnum) skipLevel0To(target int) error {
	for {
		e.prevDocID = int64(e.level0LastDocID)
		if e.docFreq-e.docCountUpto >= BlockSize {
			skip0NumBytes, err := store.ReadVLong(e.docIn)
			if err != nil {
				return err
			}
			skip0EndFP := e.docIn.GetFilePointer() + skip0NumBytes
			docDelta, err := readVInt15(e.docIn)
			if err != nil {
				return err
			}
			e.level0LastDocID += docDelta

			if target <= e.level0LastDocID {
				if err := e.docIn.SetPosition(skip0EndFP); err != nil {
					return err
				}
				break
			}

			blockLen, err := readVLong15(e.docIn)
			if err != nil {
				return err
			}
			if err := e.docIn.SetPosition(e.docIn.GetFilePointer() + blockLen); err != nil {
				return err
			}
			e.docCountUpto += BlockSize
		} else {
			e.level0LastDocID = postingsNoMoreDocsBuffer
			break
		}
	}
	return nil
}

func (e *blockDocsEnum) moveToNextLevel0Block() error {
	if e.doc == e.level1LastDocID {
		if err := e.skipLevel1To(e.doc + 1); err != nil {
			return err
		}
	}

	e.prevDocID = int64(e.level0LastDocID)
	if e.docFreq-e.docCountUpto >= BlockSize {
		if _, err := store.ReadVLong(e.docIn); err != nil { // skip0 num bytes
			return err
		}
		if err := e.refillFullBlock(); err != nil {
			return err
		}
		e.level0LastDocID = int(e.docBuffer[BlockSize-1])
	} else {
		e.level0LastDocID = postingsNoMoreDocsBuffer
		if err := e.refillRemainder(); err != nil {
			return err
		}
	}
	return nil
}

var _ index.PostingsEnum = (*blockDocsEnum)(nil)

// ─── everythingEnum ──────────────────────────────────────────────────────────

// everythingEnum decodes doc IDs, term frequencies, positions, payloads, and
// offsets.  Mirrors BlockDocsEnum's logic but also handles the .pos/.pay files.
//
// Mirrors Lucene912PostingsReader.EverythingEnum.
type everythingEnum struct {
	reader *Lucene912PostingsReader

	pforUtil     *pforUtil
	forDeltaUtil *forDeltaUtil

	docBuffer      [BlockSize + 1]int64
	freqBuffer     [BlockSize + 1]int64
	posDeltaBuffer [BlockSize]int64

	payloadLengthBuffer    []int64
	offsetStartDeltaBuffer []int64
	offsetLengthBuffer     []int64

	payloadBytes    []byte
	payloadByteUpto int
	payloadLength   int

	lastStartOffset int
	startOffset     int
	endOffset       int

	posBufferUpto int

	posIn store.IndexInput
	payIn store.IndexInput

	indexHasOffsets           bool
	indexHasPayloads          bool
	indexHasOffsetsOrPayloads bool

	freq     int
	position int

	posPendingCount int64
	lastPosBlockFP  int64

	level0PosEndFP     int64
	level0BlockPosUpto int
	level0PayEndFP     int64
	level0BlockPayUpto int

	level1PosEndFP     int64
	level1BlockPosUpto int
	level1PayEndFP     int64
	level1BlockPayUpto int

	needsOffsets  bool
	needsPayloads bool

	// from abstractPostingsEnum
	doc                int
	indexHasFreq       bool
	docFreq            int
	totalTermFreq      int64
	singletonDocID     int
	docCountUpto       int
	prevDocID          int64
	docBufferSize      int
	docBufferUpto      int
	level0LastDocID    int
	level1LastDocID    int
	level1DocEndFP     int64
	level1DocCountUpto int

	docIn store.IndexInput
}

func newEverythingEnum(r *Lucene912PostingsReader, fieldInfo *index.FieldInfo) (*everythingEnum, error) {
	opts := fieldInfo.IndexOptions()
	e := &everythingEnum{
		reader:           r,
		indexHasFreq:     opts >= index.IndexOptionsDocsAndFreqs,
		indexHasOffsets:  opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
		indexHasPayloads: fieldInfo.HasPayloads(),
	}
	e.indexHasOffsetsOrPayloads = e.indexHasOffsets || e.indexHasPayloads
	e.docBuffer[BlockSize] = postingsNoMoreDocsBuffer
	e.freqBuffer[BlockSize] = 1 // ensure freq sentinel

	e.posIn = r.posIn.Clone()
	if e.indexHasOffsetsOrPayloads {
		if r.payIn == nil {
			return nil, fmt.Errorf("everythingEnum: payIn is nil but field has offsets/payloads")
		}
		e.payIn = r.payIn.Clone()
	}
	if e.indexHasOffsets {
		e.offsetStartDeltaBuffer = make([]int64, BlockSize)
		e.offsetLengthBuffer = make([]int64, BlockSize)
	} else {
		e.startOffset = -1
		e.endOffset = -1
	}
	if e.indexHasPayloads {
		e.payloadLengthBuffer = make([]int64, BlockSize)
		e.payloadBytes = make([]byte, 128)
	}
	return e, nil
}

func (e *everythingEnum) canReuse(docIn store.IndexInput, fieldInfo *index.FieldInfo) bool {
	opts := fieldInfo.IndexOptions()
	return e.indexHasOffsets == (opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets) &&
		e.indexHasPayloads == fieldInfo.HasPayloads()
}

func (e *everythingEnum) reset(its *IntBlockTermState, flags int) (index.PostingsEnum, error) {
	e.docFreq = its.BlockTermState.DocFreq
	e.singletonDocID = its.SingletonDocID
	if e.docFreq > 1 {
		if e.docIn == nil {
			e.docIn = e.reader.docIn.Clone()
		}
		if err := prefetchPostings(e.docIn, its); err != nil {
			return nil, err
		}
	}
	if e.forDeltaUtil == nil && e.docFreq >= BlockSize {
		e.forDeltaUtil = &forDeltaUtil{}
	}
	e.totalTermFreq = its.BlockTermState.TotalTermFreq
	if e.pforUtil == nil && e.totalTermFreq >= BlockSize {
		e.pforUtil = &pforUtil{}
	}

	posTermStartFP := its.PosStartFP
	payTermStartFP := its.PayStartFP
	if err := e.posIn.SetPosition(posTermStartFP); err != nil {
		return nil, err
	}
	if e.indexHasOffsetsOrPayloads {
		if err := e.payIn.SetPosition(payTermStartFP); err != nil {
			return nil, err
		}
	}

	e.level1PosEndFP = posTermStartFP
	e.level1PayEndFP = payTermStartFP
	e.level0PosEndFP = posTermStartFP
	e.level0PayEndFP = payTermStartFP
	e.posPendingCount = 0
	e.payloadByteUpto = 0

	if e.totalTermFreq < int64(BlockSize) {
		e.lastPosBlockFP = posTermStartFP
	} else if e.totalTermFreq == int64(BlockSize) {
		e.lastPosBlockFP = -1
	} else {
		e.lastPosBlockFP = posTermStartFP + its.LastPosBlockOffset
	}

	e.needsOffsets = (flags & index.PostingsFlagOffsets) != 0
	e.needsPayloads = (flags & index.PostingsFlagPayloads) != 0

	e.level1BlockPosUpto = 0
	e.level1BlockPayUpto = 0
	e.level0BlockPosUpto = 0
	e.level0BlockPayUpto = 0
	e.posBufferUpto = BlockSize

	// resetIdsAndLevelParams
	e.doc = -1
	e.prevDocID = -1
	e.docCountUpto = 0
	e.level0LastDocID = -1
	if e.docFreq < Level1NumDocs {
		e.level1LastDocID = postingsNoMoreDocsBuffer
		if e.docFreq > 1 {
			if err := e.docIn.SetPosition(its.DocStartFP); err != nil {
				return nil, err
			}
		}
	} else {
		e.level1LastDocID = -1
		e.level1DocEndFP = its.DocStartFP
	}
	e.level1DocCountUpto = 0
	e.docBufferSize = BlockSize
	e.docBufferUpto = BlockSize
	return e, nil
}

func (e *everythingEnum) DocID() int {
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS
	}
	return e.doc
}

func (e *everythingEnum) Freq() (int, error) { return e.freq, nil }

func (e *everythingEnum) NextDoc() (int, error) {
	if e.docBufferUpto == BlockSize {
		if err := e.moveToNextLevel0Block(); err != nil {
			return 0, err
		}
	}
	e.doc = int(e.docBuffer[e.docBufferUpto])
	e.freq = int(e.freqBuffer[e.docBufferUpto])
	e.docBufferUpto++
	e.posPendingCount += int64(e.freq)
	e.position = 0
	e.lastStartOffset = 0
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS, nil
	}
	return e.doc, nil
}

func (e *everythingEnum) Advance(target int) (int, error) {
	if int64(target) > int64(e.level0LastDocID) {
		if int64(target) > int64(e.level1LastDocID) {
			if err := e.skipLevel1To(target); err != nil {
				return 0, err
			}
		}
		if err := e.skipLevel0To(target); err != nil {
			return 0, err
		}
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
	}
	next := findNextGEQ(e.docBuffer[:], int64(target), e.docBufferUpto, e.docBufferSize)
	e.posPendingCount += sumOverRange(e.freqBuffer[:], e.docBufferUpto, next+1)
	e.freq = int(e.freqBuffer[next])
	e.docBufferUpto = next + 1
	e.position = 0
	e.lastStartOffset = 0
	e.doc = int(e.docBuffer[next])
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS, nil
	}
	return e.doc, nil
}

func (e *everythingEnum) Cost() int64 { return int64(e.docFreq) }

func (e *everythingEnum) NextPosition() (int, error) {
	if e.posPendingCount > int64(e.freq) {
		if err := e.skipPositions(); err != nil {
			return 0, err
		}
		e.posPendingCount = int64(e.freq)
	}
	if e.posBufferUpto == BlockSize {
		if err := e.refillPositions(); err != nil {
			return 0, err
		}
		e.posBufferUpto = 0
	}
	e.position += int(e.posDeltaBuffer[e.posBufferUpto])

	if e.indexHasPayloads {
		e.payloadLength = int(e.payloadLengthBuffer[e.posBufferUpto])
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

func (e *everythingEnum) StartOffset() (int, error) {
	if !e.indexHasOffsets {
		return -1, nil
	}
	return e.startOffset, nil
}

func (e *everythingEnum) EndOffset() (int, error) {
	if !e.indexHasOffsets {
		return -1, nil
	}
	return e.endOffset, nil
}

func (e *everythingEnum) GetPayload() ([]byte, error) {
	if e.payloadLength == 0 {
		return nil, nil
	}
	out := make([]byte, e.payloadLength)
	copy(out, e.payloadBytes[e.payloadByteUpto-e.payloadLength:e.payloadByteUpto])
	return out, nil
}

func (e *everythingEnum) refillDocs() error {
	left := e.docFreq - e.docCountUpto
	if left >= BlockSize {
		if err := e.forDeltaUtil.decodeAndPrefixSum(e.docIn, e.prevDocID, e.docBuffer[:]); err != nil {
			return err
		}
		if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
			return err
		}
		e.docCountUpto += BlockSize
	} else if e.docFreq == 1 {
		e.docBuffer[0] = int64(e.singletonDocID)
		e.freqBuffer[0] = e.totalTermFreq
		e.docBuffer[1] = postingsNoMoreDocsBuffer
		e.docCountUpto++
		e.docBufferSize = 1
	} else {
		if err := ReadVIntBlock(e.docIn, e.docBuffer[:], e.freqBuffer[:], left, e.indexHasFreq, true); err != nil {
			return err
		}
		postingsPrefixSum(e.docBuffer[:left], left, e.prevDocID)
		e.docBuffer[left] = postingsNoMoreDocsBuffer
		e.docCountUpto += left
		e.docBufferSize = left
	}
	e.prevDocID = e.docBuffer[BlockSize-1]
	e.docBufferUpto = 0
	return nil
}

func (e *everythingEnum) skipLevel1To(target int) error {
	for {
		e.prevDocID = int64(e.level1LastDocID)
		e.level0LastDocID = e.level1LastDocID
		if err := e.docIn.SetPosition(e.level1DocEndFP); err != nil {
			return err
		}
		e.level0PosEndFP = e.level1PosEndFP
		e.level0BlockPosUpto = e.level1BlockPosUpto
		if e.indexHasOffsetsOrPayloads {
			e.level0PayEndFP = e.level1PayEndFP
			e.level0BlockPayUpto = e.level1BlockPayUpto
		}
		e.docCountUpto = e.level1DocCountUpto
		e.level1DocCountUpto += Level1NumDocs

		if e.docFreq-e.docCountUpto < Level1NumDocs {
			e.level1LastDocID = postingsNoMoreDocsBuffer
			break
		}

		d, err := store.ReadVInt(e.docIn)
		if err != nil {
			return err
		}
		e.level1LastDocID += int(d)

		delta, err := store.ReadVLong(e.docIn)
		if err != nil {
			return err
		}
		e.level1DocEndFP = delta + e.docIn.GetFilePointer()

		skip1EndFPOffset, err := e.docIn.ReadShort()
		if err != nil {
			return err
		}
		skip1EndFP := int64(skip1EndFPOffset) + e.docIn.GetFilePointer()

		if _, err = e.docIn.ReadShort(); err != nil { // impacts size
			return err
		}
		// skip impacts bytes
		posDelta, err := store.ReadVLong(e.docIn)
		if err != nil {
			return err
		}
		e.level1PosEndFP += posDelta
		byt, err := e.docIn.ReadByte()
		if err != nil {
			return err
		}
		e.level1BlockPosUpto = int(byt)
		if e.indexHasOffsetsOrPayloads {
			payDelta, err := store.ReadVLong(e.docIn)
			if err != nil {
				return err
			}
			e.level1PayEndFP += payDelta
			payUpto, err := store.ReadVInt(e.docIn)
			if err != nil {
				return err
			}
			e.level1BlockPayUpto = int(payUpto)
		}

		if e.level1LastDocID >= target {
			if err := e.docIn.SetPosition(skip1EndFP); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (e *everythingEnum) skipLevel0To(target int) error {
	for {
		e.prevDocID = int64(e.level0LastDocID)

		if e.level0PosEndFP >= e.posIn.GetFilePointer() {
			if err := e.posIn.SetPosition(e.level0PosEndFP); err != nil {
				return err
			}
			e.posPendingCount = int64(e.level0BlockPosUpto)
			if e.indexHasOffsetsOrPayloads {
				if err := e.payIn.SetPosition(e.level0PayEndFP); err != nil {
					return err
				}
				e.payloadByteUpto = e.level0BlockPayUpto
			}
			e.posBufferUpto = BlockSize
		} else {
			e.posPendingCount += sumOverRange(e.freqBuffer[:], e.docBufferUpto, BlockSize)
		}

		if e.docFreq-e.docCountUpto >= BlockSize {
			_, err := store.ReadVLong(e.docIn) // skip0 num bytes
			if err != nil {
				return err
			}
			docDelta, err := readVInt15(e.docIn)
			if err != nil {
				return err
			}
			e.level0LastDocID += docDelta

			blockLength, err := readVLong15(e.docIn)
			if err != nil {
				return err
			}
			blockEndFP := e.docIn.GetFilePointer() + blockLength

			impactsLen, err := store.ReadVLong(e.docIn)
			if err != nil {
				return err
			}
			if err := e.docIn.SetPosition(e.docIn.GetFilePointer() + impactsLen); err != nil {
				return err
			}

			posDelta, err := store.ReadVLong(e.docIn)
			if err != nil {
				return err
			}
			e.level0PosEndFP += posDelta
			byt, err := e.docIn.ReadByte()
			if err != nil {
				return err
			}
			e.level0BlockPosUpto = int(byt)

			if e.indexHasOffsetsOrPayloads {
				payDelta, err := store.ReadVLong(e.docIn)
				if err != nil {
					return err
				}
				e.level0PayEndFP += payDelta
				payUpto, err := store.ReadVInt(e.docIn)
				if err != nil {
					return err
				}
				e.level0BlockPayUpto = int(payUpto)
			}

			if int64(target) <= int64(e.level0LastDocID) {
				break
			}

			if err := e.docIn.SetPosition(blockEndFP); err != nil {
				return err
			}
			e.docCountUpto += BlockSize
		} else {
			e.level0LastDocID = postingsNoMoreDocsBuffer
			break
		}
	}
	return nil
}

func (e *everythingEnum) moveToNextLevel0Block() error {
	if e.doc == e.level1LastDocID {
		if err := e.skipLevel1To(e.doc + 1); err != nil {
			return err
		}
	}

	e.prevDocID = int64(e.level0LastDocID)

	if e.level0PosEndFP >= e.posIn.GetFilePointer() {
		if err := e.posIn.SetPosition(e.level0PosEndFP); err != nil {
			return err
		}
		e.posPendingCount = int64(e.level0BlockPosUpto)
		if e.indexHasOffsetsOrPayloads {
			if err := e.payIn.SetPosition(e.level0PayEndFP); err != nil {
				return err
			}
			e.payloadByteUpto = e.level0BlockPayUpto
		}
		e.posBufferUpto = BlockSize
	}

	if e.docFreq-e.docCountUpto >= BlockSize {
		_, err := store.ReadVLong(e.docIn) // skip0 num bytes
		if err != nil {
			return err
		}
		docDelta, err := readVInt15(e.docIn)
		if err != nil {
			return err
		}
		e.level0LastDocID += docDelta

		_, err = readVLong15(e.docIn) // block length
		if err != nil {
			return err
		}

		impactsLen, err := store.ReadVLong(e.docIn)
		if err != nil {
			return err
		}
		if err := e.docIn.SetPosition(e.docIn.GetFilePointer() + impactsLen); err != nil {
			return err
		}

		posDelta, err := store.ReadVLong(e.docIn)
		if err != nil {
			return err
		}
		e.level0PosEndFP += posDelta
		byt, err := e.docIn.ReadByte()
		if err != nil {
			return err
		}
		e.level0BlockPosUpto = int(byt)
		if e.indexHasOffsetsOrPayloads {
			payDelta, err := store.ReadVLong(e.docIn)
			if err != nil {
				return err
			}
			e.level0PayEndFP += payDelta
			payUpto, err := store.ReadVInt(e.docIn)
			if err != nil {
				return err
			}
			e.level0BlockPayUpto = int(payUpto)
		}
		if err := e.refillDocs(); err != nil {
			return err
		}
	} else {
		e.level0LastDocID = postingsNoMoreDocsBuffer
		if err := e.refillDocs(); err != nil {
			return err
		}
	}
	return nil
}

func (e *everythingEnum) skipPositions() error {
	toSkip := e.posPendingCount - int64(e.freq)
	leftInBlock := BlockSize - e.posBufferUpto
	if toSkip < int64(leftInBlock) {
		end := e.posBufferUpto + int(toSkip)
		if e.indexHasPayloads {
			e.payloadByteUpto += int(sumOverRange(e.payloadLengthBuffer, e.posBufferUpto, end))
		}
		e.posBufferUpto = end
	} else {
		toSkip -= int64(leftInBlock)
		for toSkip >= int64(BlockSize) {
			if err := pforUtilSkip(e.posIn); err != nil {
				return err
			}
			if e.indexHasPayloads {
				if err := pforUtilSkip(e.payIn); err != nil {
					return err
				}
				numBytes, err := store.ReadVInt(e.payIn)
				if err != nil {
					return err
				}
				if err := e.payIn.SetPosition(e.payIn.GetFilePointer() + int64(numBytes)); err != nil {
					return err
				}
			}
			if e.indexHasOffsets {
				if err := pforUtilSkip(e.payIn); err != nil {
					return err
				}
				if err := pforUtilSkip(e.payIn); err != nil {
					return err
				}
			}
			toSkip -= int64(BlockSize)
		}
		if err := e.refillPositions(); err != nil {
			return err
		}
		e.payloadByteUpto = 0
		toSkipInt := int(toSkip)
		if e.indexHasPayloads {
			e.payloadByteUpto += int(sumOverRange(e.payloadLengthBuffer, 0, toSkipInt))
		}
		e.posBufferUpto = toSkipInt
	}
	e.position = 0
	e.lastStartOffset = 0
	return nil
}

func (e *everythingEnum) refillPositions() error {
	if e.posIn.GetFilePointer() == e.lastPosBlockFP {
		count := int(e.totalTermFreq % int64(BlockSize))
		var payloadLength int
		var offsetLength int
		e.payloadByteUpto = 0
		for i := 0; i < count; i++ {
			code, err := store.ReadVInt(e.posIn)
			if err != nil {
				return err
			}
			if e.indexHasPayloads {
				if code&1 != 0 {
					pl, err := store.ReadVInt(e.posIn)
					if err != nil {
						return err
					}
					payloadLength = int(pl)
				}
				e.payloadLengthBuffer[i] = int64(payloadLength)
				e.posDeltaBuffer[i] = int64(code >> 1)
				if payloadLength != 0 {
					needed := e.payloadByteUpto + payloadLength
					if needed > len(e.payloadBytes) {
						newLen := needed * 2
						if newLen < 128 {
							newLen = 128
						}
						grown := make([]byte, newLen)
						copy(grown, e.payloadBytes)
						e.payloadBytes = grown
					}
					if err := e.posIn.ReadBytes(e.payloadBytes[e.payloadByteUpto : e.payloadByteUpto+payloadLength]); err != nil {
						return err
					}
					e.payloadByteUpto += payloadLength
				}
			} else {
				e.posDeltaBuffer[i] = int64(code)
			}
			if e.indexHasOffsets {
				deltaCode, err := store.ReadVInt(e.posIn)
				if err != nil {
					return err
				}
				if deltaCode&1 != 0 {
					ol, err := store.ReadVInt(e.posIn)
					if err != nil {
						return err
					}
					offsetLength = int(ol)
				}
				e.offsetStartDeltaBuffer[i] = int64(deltaCode >> 1)
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
				numBytes, err := store.ReadVInt(e.payIn)
				if err != nil {
					return err
				}
				needed := int(numBytes)
				if needed > len(e.payloadBytes) {
					e.payloadBytes = make([]byte, needed*2)
				}
				if err := e.payIn.ReadBytes(e.payloadBytes[:needed]); err != nil {
					return err
				}
			} else {
				if err := pforUtilSkip(e.payIn); err != nil {
					return err
				}
				numBytes, err := store.ReadVInt(e.payIn)
				if err != nil {
					return err
				}
				if err := e.payIn.SetPosition(e.payIn.GetFilePointer() + int64(numBytes)); err != nil {
					return err
				}
			}
			e.payloadByteUpto = 0
		}
		if e.indexHasOffsets {
			if e.needsOffsets {
				if err := e.pforUtil.decode(e.payIn, e.offsetStartDeltaBuffer); err != nil {
					return err
				}
				if err := e.pforUtil.decode(e.payIn, e.offsetLengthBuffer); err != nil {
					return err
				}
			} else {
				if err := pforUtilSkip(e.payIn); err != nil {
					return err
				}
				if err := pforUtilSkip(e.payIn); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

var _ index.PostingsEnum = (*everythingEnum)(nil)

// ─── blockImpactsDocsEnum ────────────────────────────────────────────────────

// blockImpactsDocsEnum mirrors Lucene912PostingsReader.BlockImpactsDocsEnum.
// It supports the ImpactsEnum interface (AdvanceShallow + GetImpacts).
type blockImpactsDocsEnum struct {
	reader *Lucene912PostingsReader

	pforUtil     *pforUtil
	forDeltaUtil *forDeltaUtil

	docBuffer  [BlockSize + 1]int64
	freqBuffer [BlockSize]int64

	indexHasPos bool
	freqFP      int64

	doc           int
	docFreq       int
	prevDocID     int64
	docCountUpto  int
	docBufferSize int
	docBufferUpto int

	level0LastDocID    int
	level0DocEndFP     int64
	level1LastDocID    int
	level1DocEndFP     int64
	level1DocCountUpto int

	needsRefilling bool

	level0SerializedImpacts []byte
	level0ImpactsLen        int
	level1SerializedImpacts []byte
	level1ImpactsLen        int

	impactBuffer *index.FreqAndNormBuffer

	docIn store.IndexInput
}

func newBlockImpactsDocsEnum(r *Lucene912PostingsReader, indexHasPos bool, its *IntBlockTermState) (*blockImpactsDocsEnum, error) {
	e := &blockImpactsDocsEnum{
		reader:       r,
		indexHasPos:  indexHasPos,
		freqFP:       -1,
		pforUtil:     &pforUtil{},
		forDeltaUtil: &forDeltaUtil{},
	}
	e.docBuffer[BlockSize] = postingsNoMoreDocsBuffer
	e.docIn = r.docIn.Clone()
	if err := prefetchPostings(e.docIn, its); err != nil {
		return nil, err
	}

	e.level0SerializedImpacts = make([]byte, r.maxImpactNumBytesAtLevel0)
	e.level1SerializedImpacts = make([]byte, r.maxImpactNumBytesAtLevel1)

	capacity := 1
	if r.maxNumImpactsAtLevel0 > capacity {
		capacity = r.maxNumImpactsAtLevel0
	}
	if r.maxNumImpactsAtLevel1 > capacity {
		capacity = r.maxNumImpactsAtLevel1
	}
	e.impactBuffer = index.NewFreqAndNormBuffer()
	e.impactBuffer.GrowNoCopy(capacity)

	e.docFreq = its.BlockTermState.DocFreq
	if e.docFreq < Level1NumDocs {
		e.level1LastDocID = postingsNoMoreDocsBuffer
		if e.docFreq > 1 {
			if err := e.docIn.SetPosition(its.DocStartFP); err != nil {
				return nil, err
			}
		}
	} else {
		e.level1LastDocID = -1
		e.level1DocEndFP = its.DocStartFP
	}
	e.level0LastDocID = -1
	e.doc = -1
	e.prevDocID = -1
	e.docBufferSize = BlockSize
	e.docBufferUpto = BlockSize
	return e, nil
}

func (e *blockImpactsDocsEnum) DocID() int {
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS
	}
	return e.doc
}

func (e *blockImpactsDocsEnum) Freq() (int, error) {
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

func (e *blockImpactsDocsEnum) NextPosition() (int, error)  { return -1, nil }
func (e *blockImpactsDocsEnum) StartOffset() (int, error)   { return -1, nil }
func (e *blockImpactsDocsEnum) EndOffset() (int, error)     { return -1, nil }
func (e *blockImpactsDocsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (e *blockImpactsDocsEnum) Cost() int64                 { return int64(e.docFreq) }

func (e *blockImpactsDocsEnum) AdvanceShallow(target int) error {
	if int64(target) > int64(e.level0LastDocID) {
		if int64(target) > int64(e.level1LastDocID) {
			if err := e.skipLevel1To(target); err != nil {
				return err
			}
		} else if e.needsRefilling {
			if err := e.docIn.SetPosition(e.level0DocEndFP); err != nil {
				return err
			}
			e.docCountUpto += BlockSize
		}
		if err := e.skipLevel0To(target); err != nil {
			return err
		}
		e.needsRefilling = true
	}
	return nil
}

func (e *blockImpactsDocsEnum) GetImpacts() (index.Impacts, error) {
	return &blockImpacts{e: e}, nil
}

func (e *blockImpactsDocsEnum) NextDoc() (int, error) {
	if e.docBufferUpto == BlockSize {
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
	e.doc = int(e.docBuffer[e.docBufferUpto])
	e.docBufferUpto++
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS, nil
	}
	return e.doc, nil
}

func (e *blockImpactsDocsEnum) Advance(target int) (int, error) {
	if int64(target) > int64(e.level0LastDocID) || e.needsRefilling {
		if err := e.AdvanceShallow(target); err != nil {
			return 0, err
		}
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
		e.needsRefilling = false
	}
	next := findNextGEQ(e.docBuffer[:], int64(target), e.docBufferUpto, e.docBufferSize)
	e.doc = int(e.docBuffer[next])
	e.docBufferUpto = next + 1
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS, nil
	}
	return e.doc, nil
}

func (e *blockImpactsDocsEnum) refillDocs() error {
	left := e.docFreq - e.docCountUpto
	if left >= BlockSize {
		if err := e.forDeltaUtil.decodeAndPrefixSum(e.docIn, e.prevDocID, e.docBuffer[:]); err != nil {
			return err
		}
		e.freqFP = e.docIn.GetFilePointer()
		if err := pforUtilSkip(e.docIn); err != nil {
			return err
		}
		e.docCountUpto += BlockSize
	} else {
		if err := ReadVIntBlock(e.docIn, e.docBuffer[:], e.freqBuffer[:], left, true, true); err != nil {
			return err
		}
		postingsPrefixSum(e.docBuffer[:left], left, e.prevDocID)
		e.docBuffer[left] = postingsNoMoreDocsBuffer
		e.freqFP = -1
		e.docCountUpto += left
		e.docBufferSize = left
	}
	e.prevDocID = e.docBuffer[BlockSize-1]
	e.docBufferUpto = 0
	return nil
}

func (e *blockImpactsDocsEnum) skipLevel1To(target int) error {
	for {
		e.prevDocID = int64(e.level1LastDocID)
		e.level0LastDocID = e.level1LastDocID
		if err := e.docIn.SetPosition(e.level1DocEndFP); err != nil {
			return err
		}
		e.docCountUpto = e.level1DocCountUpto
		e.level1DocCountUpto += Level1NumDocs

		if e.docFreq-e.docCountUpto < Level1NumDocs {
			e.level1LastDocID = postingsNoMoreDocsBuffer
			break
		}

		d, err := store.ReadVInt(e.docIn)
		if err != nil {
			return err
		}
		e.level1LastDocID += int(d)

		endFPDelta, err := store.ReadVLong(e.docIn)
		if err != nil {
			return err
		}
		e.level1DocEndFP = endFPDelta + e.docIn.GetFilePointer()

		if e.level1LastDocID >= target {
			skip1EndFPOffset, err := e.docIn.ReadShort()
			if err != nil {
				return err
			}
			skip1EndFP := int64(skip1EndFPOffset) + e.docIn.GetFilePointer()
			numImpactBytes, err := e.docIn.ReadShort()
			if err != nil {
				return err
			}
			nb := int(int16(numImpactBytes))
			if nb > len(e.level1SerializedImpacts) {
				e.level1SerializedImpacts = make([]byte, nb)
			}
			if err := e.docIn.ReadBytes(e.level1SerializedImpacts[:nb]); err != nil {
				return err
			}
			e.level1ImpactsLen = nb
			if err := e.docIn.SetPosition(skip1EndFP); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (e *blockImpactsDocsEnum) skipLevel0To(target int) error {
	for {
		e.prevDocID = int64(e.level0LastDocID)
		if e.docFreq-e.docCountUpto >= BlockSize {
			skip0NumBytes, err := store.ReadVLong(e.docIn)
			if err != nil {
				return err
			}
			skip0End := e.docIn.GetFilePointer() + skip0NumBytes
			docDelta, err := readVInt15(e.docIn)
			if err != nil {
				return err
			}
			blockLength, err := readVLong15(e.docIn)
			if err != nil {
				return err
			}
			e.level0LastDocID += docDelta

			if target <= e.level0LastDocID {
				e.level0DocEndFP = e.docIn.GetFilePointer() + blockLength
				numImpactBytes, err := store.ReadVInt(e.docIn)
				if err != nil {
					return err
				}
				nb := int(numImpactBytes)
				if nb > len(e.level0SerializedImpacts) {
					e.level0SerializedImpacts = make([]byte, nb)
				}
				if err := e.docIn.ReadBytes(e.level0SerializedImpacts[:nb]); err != nil {
					return err
				}
				e.level0ImpactsLen = nb
				if err := e.docIn.SetPosition(skip0End); err != nil {
					return err
				}
				break
			}

			if err := e.docIn.SetPosition(e.docIn.GetFilePointer() + blockLength); err != nil {
				return err
			}
			e.docCountUpto += BlockSize
		} else {
			e.level0LastDocID = postingsNoMoreDocsBuffer
			break
		}
	}
	return nil
}

func (e *blockImpactsDocsEnum) moveToNextLevel0Block() error {
	if e.doc == e.level1LastDocID {
		if err := e.skipLevel1To(e.doc + 1); err != nil {
			return err
		}
	} else if e.needsRefilling {
		if err := e.docIn.SetPosition(e.level0DocEndFP); err != nil {
			return err
		}
		e.docCountUpto += BlockSize
	}

	e.prevDocID = int64(e.level0LastDocID)
	if e.docFreq-e.docCountUpto >= BlockSize {
		skip0Len, err := store.ReadVLong(e.docIn)
		if err != nil {
			return err
		}
		skip0End := e.docIn.GetFilePointer() + skip0Len
		docDelta, err := readVInt15(e.docIn)
		if err != nil {
			return err
		}
		blockLength, err := readVLong15(e.docIn)
		if err != nil {
			return err
		}
		e.level0LastDocID += docDelta
		e.level0DocEndFP = e.docIn.GetFilePointer() + blockLength

		numImpactBytes, err := store.ReadVInt(e.docIn)
		if err != nil {
			return err
		}
		nb := int(numImpactBytes)
		if nb > len(e.level0SerializedImpacts) {
			e.level0SerializedImpacts = make([]byte, nb)
		}
		if err := e.docIn.ReadBytes(e.level0SerializedImpacts[:nb]); err != nil {
			return err
		}
		e.level0ImpactsLen = nb
		if err := e.docIn.SetPosition(skip0End); err != nil {
			return err
		}
	} else {
		e.level0LastDocID = postingsNoMoreDocsBuffer
	}

	if err := e.refillDocs(); err != nil {
		return err
	}
	e.needsRefilling = false
	return nil
}

// blockImpacts implements index.Impacts for blockImpactsDocsEnum.
type blockImpacts struct {
	e   *blockImpactsDocsEnum
	buf store.ByteArrayDataInput
}

func (bi *blockImpacts) NumLevels() int {
	if bi.e.level1LastDocID == postingsNoMoreDocsBuffer {
		return 1
	}
	return 2
}

func (bi *blockImpacts) GetDocIDUpTo(level int) int {
	if level == 0 {
		return bi.e.level0LastDocID
	}
	if level == 1 {
		if bi.e.level1LastDocID == postingsNoMoreDocsBuffer {
			return index.NO_MORE_DOCS
		}
		return bi.e.level1LastDocID
	}
	return index.NO_MORE_DOCS
}

func (bi *blockImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	buf := bi.e.impactBuffer
	if level == 0 && bi.e.level0LastDocID != postingsNoMoreDocsBuffer {
		bi.buf.Reset(bi.e.level0SerializedImpacts[:bi.e.level0ImpactsLen])
		readImpacts(&bi.buf, buf)
		return buf
	}
	if level == 1 {
		bi.buf.Reset(bi.e.level1SerializedImpacts[:bi.e.level1ImpactsLen])
		readImpacts(&bi.buf, buf)
		return buf
	}
	buf.GrowNoCopy(1)
	buf.Freqs[0] = math.MaxInt32
	buf.Norms[0] = 1
	buf.Size = 1
	return buf
}

var _ index.ImpactsEnum = (*blockImpactsDocsEnum)(nil)

// ─── blockImpactsPostingsEnum ─────────────────────────────────────────────────

// blockImpactsPostingsEnum mirrors Lucene912PostingsReader.BlockImpactsPostingsEnum.
// Decodes doc IDs, frequencies, and positions, while also serving impacts.
type blockImpactsPostingsEnum struct {
	reader *Lucene912PostingsReader

	pforUtil     *pforUtil
	forDeltaUtil *forDeltaUtil

	docBuffer      [BlockSize + 1]int64
	freqBuffer     [BlockSize]int64
	posDeltaBuffer [BlockSize]int64

	posBufferUpto int
	posIn         store.IndexInput

	indexHasFreq              bool
	indexHasOffsets           bool
	indexHasPayloads          bool
	indexHasOffsetsOrPayloads bool

	totalTermFreq   int64
	freq            int
	position        int
	posPendingCount int64
	lastPosBlockFP  int64
	singletonDocID  int

	level0PosEndFP     int64
	level0BlockPosUpto int
	level1PosEndFP     int64
	level1BlockPosUpto int

	doc           int
	docFreq       int
	prevDocID     int64
	docCountUpto  int
	docBufferSize int
	docBufferUpto int

	level0LastDocID    int
	level0DocEndFP     int64
	level1LastDocID    int
	level1DocEndFP     int64
	level1DocCountUpto int

	needsRefilling bool

	level0SerializedImpacts []byte
	level0ImpactsLen        int
	level1SerializedImpacts []byte
	level1ImpactsLen        int

	impactBuffer *index.FreqAndNormBuffer
	docIn        store.IndexInput
}

func newBlockImpactsPostingsEnum(r *Lucene912PostingsReader, fieldInfo *index.FieldInfo, its *IntBlockTermState) (*blockImpactsPostingsEnum, error) {
	opts := fieldInfo.IndexOptions()
	e := &blockImpactsPostingsEnum{
		reader:           r,
		indexHasFreq:     opts >= index.IndexOptionsDocsAndFreqs,
		indexHasOffsets:  opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
		indexHasPayloads: fieldInfo.HasPayloads(),
		pforUtil:         &pforUtil{},
		forDeltaUtil:     &forDeltaUtil{},
	}
	e.indexHasOffsetsOrPayloads = e.indexHasOffsets || e.indexHasPayloads
	e.docBuffer[BlockSize] = postingsNoMoreDocsBuffer

	e.posIn = r.posIn.Clone()
	e.docIn = r.docIn.Clone()
	if err := prefetchPostings(e.docIn, its); err != nil {
		return nil, err
	}

	e.level0SerializedImpacts = make([]byte, r.maxImpactNumBytesAtLevel0)
	e.level1SerializedImpacts = make([]byte, r.maxImpactNumBytesAtLevel1)

	capacity := 1
	if r.maxNumImpactsAtLevel0 > capacity {
		capacity = r.maxNumImpactsAtLevel0
	}
	if r.maxNumImpactsAtLevel1 > capacity {
		capacity = r.maxNumImpactsAtLevel1
	}
	e.impactBuffer = index.NewFreqAndNormBuffer()
	e.impactBuffer.GrowNoCopy(capacity)

	e.docFreq = its.BlockTermState.DocFreq
	e.totalTermFreq = its.BlockTermState.TotalTermFreq
	e.singletonDocID = its.SingletonDocID

	posTermStartFP := its.PosStartFP
	if err := e.posIn.SetPosition(posTermStartFP); err != nil {
		return nil, err
	}
	e.level1PosEndFP = posTermStartFP
	e.level0PosEndFP = posTermStartFP
	e.posPendingCount = 0

	if e.totalTermFreq < int64(BlockSize) {
		e.lastPosBlockFP = posTermStartFP
	} else if e.totalTermFreq == int64(BlockSize) {
		e.lastPosBlockFP = -1
	} else {
		e.lastPosBlockFP = posTermStartFP + its.LastPosBlockOffset
	}

	e.level1BlockPosUpto = 0
	e.posBufferUpto = BlockSize

	e.doc = -1
	e.prevDocID = -1
	e.docCountUpto = 0
	e.level0LastDocID = -1
	if e.docFreq < Level1NumDocs {
		e.level1LastDocID = postingsNoMoreDocsBuffer
		if e.docFreq > 1 {
			if err := e.docIn.SetPosition(its.DocStartFP); err != nil {
				return nil, err
			}
		}
	} else {
		e.level1LastDocID = -1
		e.level1DocEndFP = its.DocStartFP
	}
	e.level1DocCountUpto = 0
	e.docBufferSize = BlockSize
	e.docBufferUpto = BlockSize
	return e, nil
}

func (e *blockImpactsPostingsEnum) DocID() int {
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS
	}
	return e.doc
}

func (e *blockImpactsPostingsEnum) Freq() (int, error) { return e.freq, nil }

func (e *blockImpactsPostingsEnum) NextPosition() (int, error) {
	if e.posPendingCount > int64(e.freq) {
		if err := e.skipPositions(); err != nil {
			return 0, err
		}
		e.posPendingCount = int64(e.freq)
	}
	if e.posBufferUpto == BlockSize {
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

func (e *blockImpactsPostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (e *blockImpactsPostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (e *blockImpactsPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (e *blockImpactsPostingsEnum) Cost() int64                 { return int64(e.docFreq) }

func (e *blockImpactsPostingsEnum) AdvanceShallow(target int) error {
	if int64(target) > int64(e.level0LastDocID) {
		if int64(target) > int64(e.level1LastDocID) {
			if err := e.skipLevel1To(target); err != nil {
				return err
			}
		} else if e.needsRefilling {
			if err := e.docIn.SetPosition(e.level0DocEndFP); err != nil {
				return err
			}
			e.docCountUpto += BlockSize
		}
		if err := e.skipLevel0To(target); err != nil {
			return err
		}
		e.needsRefilling = true
	}
	return nil
}

func (e *blockImpactsPostingsEnum) GetImpacts() (index.Impacts, error) {
	return &blockImpactsPostings{e: e}, nil
}

func (e *blockImpactsPostingsEnum) NextDoc() (int, error) {
	if e.docBufferUpto == BlockSize {
		if err := e.AdvanceShallow(e.doc + 1); err != nil {
			return 0, err
		}
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
		e.needsRefilling = false
	}
	e.doc = int(e.docBuffer[e.docBufferUpto])
	e.freq = int(e.freqBuffer[e.docBufferUpto])
	e.posPendingCount += int64(e.freq)
	e.docBufferUpto++
	e.position = 0
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS, nil
	}
	return e.doc, nil
}

func (e *blockImpactsPostingsEnum) Advance(target int) (int, error) {
	if err := e.AdvanceShallow(target); err != nil {
		return 0, err
	}
	if e.needsRefilling {
		if err := e.refillDocs(); err != nil {
			return 0, err
		}
		e.needsRefilling = false
	}
	next := findNextGEQ(e.docBuffer[:], int64(target), e.docBufferUpto, e.docBufferSize)
	e.posPendingCount += sumOverRange(e.freqBuffer[:], e.docBufferUpto, next+1)
	e.freq = int(e.freqBuffer[next])
	e.docBufferUpto = next + 1
	e.position = 0
	e.doc = int(e.docBuffer[next])
	if e.doc == postingsNoMoreDocsBuffer {
		return index.NO_MORE_DOCS, nil
	}
	return e.doc, nil
}

func (e *blockImpactsPostingsEnum) refillDocs() error {
	left := e.docFreq - e.docCountUpto
	if left >= BlockSize {
		if err := e.forDeltaUtil.decodeAndPrefixSum(e.docIn, e.prevDocID, e.docBuffer[:]); err != nil {
			return err
		}
		if err := e.pforUtil.decode(e.docIn, e.freqBuffer[:]); err != nil {
			return err
		}
		e.docCountUpto += BlockSize
	} else if e.docFreq == 1 {
		e.docBuffer[0] = int64(e.singletonDocID)
		e.freqBuffer[0] = e.totalTermFreq
		e.docBuffer[1] = postingsNoMoreDocsBuffer
		e.docCountUpto++
	} else {
		if err := ReadVIntBlock(e.docIn, e.docBuffer[:], e.freqBuffer[:], left, e.indexHasFreq, true); err != nil {
			return err
		}
		postingsPrefixSum(e.docBuffer[:left], left, e.prevDocID)
		e.docBuffer[left] = postingsNoMoreDocsBuffer
		e.docCountUpto += left
		e.docBufferSize = left
	}
	e.prevDocID = e.docBuffer[BlockSize-1]
	e.docBufferUpto = 0
	return nil
}

func (e *blockImpactsPostingsEnum) skipLevel1To(target int) error {
	for {
		e.prevDocID = int64(e.level1LastDocID)
		e.level0LastDocID = e.level1LastDocID
		if err := e.docIn.SetPosition(e.level1DocEndFP); err != nil {
			return err
		}
		e.level0PosEndFP = e.level1PosEndFP
		e.level0BlockPosUpto = e.level1BlockPosUpto
		e.docCountUpto = e.level1DocCountUpto
		e.level1DocCountUpto += Level1NumDocs

		if e.docFreq-e.docCountUpto < Level1NumDocs {
			e.level1LastDocID = postingsNoMoreDocsBuffer
			break
		}

		d, err := store.ReadVInt(e.docIn)
		if err != nil {
			return err
		}
		e.level1LastDocID += int(d)

		endFPDelta, err := store.ReadVLong(e.docIn)
		if err != nil {
			return err
		}
		e.level1DocEndFP = endFPDelta + e.docIn.GetFilePointer()

		skip1EndFPOffset, err := e.docIn.ReadShort()
		if err != nil {
			return err
		}
		skip1EndFP := int64(skip1EndFPOffset) + e.docIn.GetFilePointer()

		numImpactBytes, err := e.docIn.ReadShort()
		if err != nil {
			return err
		}
		nb := int(int16(numImpactBytes))
		if e.level1LastDocID >= target {
			if nb > len(e.level1SerializedImpacts) {
				e.level1SerializedImpacts = make([]byte, nb)
			}
			if err := e.docIn.ReadBytes(e.level1SerializedImpacts[:nb]); err != nil {
				return err
			}
			e.level1ImpactsLen = nb
		} else {
			if err := e.docIn.SetPosition(e.docIn.GetFilePointer() + int64(nb)); err != nil {
				return err
			}
		}

		posDelta, err := store.ReadVLong(e.docIn)
		if err != nil {
			return err
		}
		e.level1PosEndFP += posDelta
		byt, err := e.docIn.ReadByte()
		if err != nil {
			return err
		}
		e.level1BlockPosUpto = int(byt)

		if e.level1LastDocID >= target {
			if err := e.docIn.SetPosition(skip1EndFP); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (e *blockImpactsPostingsEnum) skipLevel0To(target int) error {
	for {
		e.prevDocID = int64(e.level0LastDocID)

		if e.level0PosEndFP >= e.posIn.GetFilePointer() {
			if err := e.posIn.SetPosition(e.level0PosEndFP); err != nil {
				return err
			}
			e.posPendingCount = int64(e.level0BlockPosUpto)
			e.posBufferUpto = BlockSize
		} else {
			e.posPendingCount += sumOverRange(e.freqBuffer[:], e.docBufferUpto, BlockSize)
		}

		if e.docFreq-e.docCountUpto >= BlockSize {
			_, err := store.ReadVLong(e.docIn) // skip0 num bytes
			if err != nil {
				return err
			}
			docDelta, err := readVInt15(e.docIn)
			if err != nil {
				return err
			}
			blockLength, err := readVLong15(e.docIn)
			if err != nil {
				return err
			}
			e.level0DocEndFP = e.docIn.GetFilePointer() + blockLength
			e.level0LastDocID += docDelta

			if target <= e.level0LastDocID {
				numImpactBytes, err := store.ReadVInt(e.docIn)
				if err != nil {
					return err
				}
				nb := int(numImpactBytes)
				if nb > len(e.level0SerializedImpacts) {
					e.level0SerializedImpacts = make([]byte, nb)
				}
				if err := e.docIn.ReadBytes(e.level0SerializedImpacts[:nb]); err != nil {
					return err
				}
				e.level0ImpactsLen = nb
				posDelta, err := store.ReadVLong(e.docIn)
				if err != nil {
					return err
				}
				e.level0PosEndFP += posDelta
				byt, err := e.docIn.ReadByte()
				if err != nil {
					return err
				}
				e.level0BlockPosUpto = int(byt)
				if e.indexHasOffsetsOrPayloads {
					if _, err := store.ReadVLong(e.docIn); err != nil { // pay fp delta
						return err
					}
					if _, err := store.ReadVInt(e.docIn); err != nil { // pay upto
						return err
					}
				}
				break
			}

			impactsLen, err := store.ReadVLong(e.docIn)
			if err != nil {
				return err
			}
			if err := e.docIn.SetPosition(e.docIn.GetFilePointer() + impactsLen); err != nil {
				return err
			}
			posDelta, err := store.ReadVLong(e.docIn)
			if err != nil {
				return err
			}
			e.level0PosEndFP += posDelta
			posUpto, err := store.ReadVInt(e.docIn)
			if err != nil {
				return err
			}
			e.level0BlockPosUpto = int(posUpto)
			if err := e.docIn.SetPosition(e.level0DocEndFP); err != nil {
				return err
			}
			e.docCountUpto += BlockSize
		} else {
			e.level0LastDocID = postingsNoMoreDocsBuffer
			break
		}
	}
	return nil
}

func (e *blockImpactsPostingsEnum) skipPositions() error {
	toSkip := e.posPendingCount - int64(e.freq)
	leftInBlock := BlockSize - e.posBufferUpto
	if toSkip < int64(leftInBlock) {
		e.posBufferUpto += int(toSkip)
	} else {
		toSkip -= int64(leftInBlock)
		for toSkip >= int64(BlockSize) {
			if err := pforUtilSkip(e.posIn); err != nil {
				return err
			}
			toSkip -= int64(BlockSize)
		}
		if err := e.refillPositions(); err != nil {
			return err
		}
		e.posBufferUpto = int(toSkip)
	}
	e.position = 0
	return nil
}

func (e *blockImpactsPostingsEnum) refillPositions() error {
	if e.posIn.GetFilePointer() == e.lastPosBlockFP {
		count := int(e.totalTermFreq % int64(BlockSize))
		var payloadLength int
		for i := 0; i < count; i++ {
			code, err := store.ReadVInt(e.posIn)
			if err != nil {
				return err
			}
			if e.indexHasPayloads {
				if code&1 != 0 {
					pl, err := store.ReadVInt(e.posIn)
					if err != nil {
						return err
					}
					payloadLength = int(pl)
				}
				e.posDeltaBuffer[i] = int64(code >> 1)
				if payloadLength != 0 {
					if err := e.posIn.SetPosition(e.posIn.GetFilePointer() + int64(payloadLength)); err != nil {
						return err
					}
				}
			} else {
				e.posDeltaBuffer[i] = int64(code)
			}
			if e.indexHasOffsets {
				deltaCode, err := store.ReadVInt(e.posIn)
				if err != nil {
					return err
				}
				if deltaCode&1 != 0 {
					if _, err := store.ReadVInt(e.posIn); err != nil {
						return err
					}
				}
			}
		}
	} else {
		if err := e.pforUtil.decode(e.posIn, e.posDeltaBuffer[:]); err != nil {
			return err
		}
	}
	return nil
}

// blockImpactsPostings implements index.Impacts for blockImpactsPostingsEnum.
type blockImpactsPostings struct {
	e   *blockImpactsPostingsEnum
	buf store.ByteArrayDataInput
}

func (bi *blockImpactsPostings) NumLevels() int {
	if bi.e.level1LastDocID == postingsNoMoreDocsBuffer {
		return 1
	}
	return 2
}

func (bi *blockImpactsPostings) GetDocIDUpTo(level int) int {
	if level == 0 {
		return bi.e.level0LastDocID
	}
	if level == 1 {
		if bi.e.level1LastDocID == postingsNoMoreDocsBuffer {
			return index.NO_MORE_DOCS
		}
		return bi.e.level1LastDocID
	}
	return index.NO_MORE_DOCS
}

func (bi *blockImpactsPostings) GetImpacts(level int) *index.FreqAndNormBuffer {
	buf := bi.e.impactBuffer
	if level == 0 && bi.e.level0LastDocID != postingsNoMoreDocsBuffer {
		bi.buf.Reset(bi.e.level0SerializedImpacts[:bi.e.level0ImpactsLen])
		readImpacts(&bi.buf, buf)
		return buf
	}
	if level == 1 {
		bi.buf.Reset(bi.e.level1SerializedImpacts[:bi.e.level1ImpactsLen])
		readImpacts(&bi.buf, buf)
		return buf
	}
	buf.GrowNoCopy(1)
	buf.Freqs[0] = math.MaxInt32
	buf.Norms[0] = 1
	buf.Size = 1
	return buf
}

var _ index.ImpactsEnum = (*blockImpactsPostingsEnum)(nil)
