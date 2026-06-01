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

// Lucene104 postings-format constants.
const (
	lucene104MetaCodec  = "Lucene104PostingsWriterMeta"
	lucene104DocCodec   = "Lucene104PostingsWriterDoc"
	lucene104PosCodec   = "Lucene104PostingsWriterPos"
	lucene104PayCodec   = "Lucene104PostingsWriterPay"
	lucene104TermsCodec = "Lucene104PostingsWriterTerms"

	lucene104MetaExtension = "psm"
	lucene104DocExtension  = "doc"
	lucene104PosExtension  = "pos"
	lucene104PayExtension  = "pay"

	lucene104VersionStart   = 0
	lucene104VersionCurrent = lucene104VersionStart

	// lucene104BlockSize is the number of doc-deltas per packed block.
	// Lucene 10.4 uses ForUtil.BLOCK_SIZE = 256.
	lucene104BlockSize = ForUtilBlockSize // 256

	// lucene104Level1Factor controls how many blocks form a level-1 skip group.
	lucene104Level1Factor = 32
	// lucene104Level1NumDocs is the number of docs in one level-1 skip group.
	lucene104Level1NumDocs = lucene104Level1Factor * lucene104BlockSize // 8192
	// lucene104Level1Mask is the bitmask used to detect level-1 skip boundaries.
	lucene104Level1Mask = lucene104Level1NumDocs - 1
)

// emptyIntBlockTermState is a sentinel used to detect the first EncodeTerm call
// in a block. Its zero values cause deltas from it to reproduce the absolute
// file pointers of the first real term.
var emptyIntBlockTermState = NewIntBlockTermState()

// Lucene104PostingsWriter writes .doc / .pos / .pay / .psm files using
// PFor block compression and a two-level skip structure.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene104.Lucene104PostingsWriter from Apache
// Lucene 10.4.0.
//
// Lucene104PostingsWriter implements both PostingsWriterBase and
// PushPostingsWriterBase.
type Lucene104PostingsWriter struct {
	version int

	metaOut store.IndexOutput
	docOut  store.IndexOutput
	posOut  store.IndexOutput
	payOut  store.IndexOutput

	// per-term file-pointer anchors
	docStartFP int64
	posStartFP int64
	payStartFP int64

	// doc accumulation buffers (BLOCK_SIZE entries each)
	docDeltaBuffer []int32
	freqBuffer     []int32
	docBufferUpto  int

	// position accumulation buffers
	posDeltaBuffer         []int32
	payloadLengthBuffer    []int32
	offsetStartDeltaBuffer []int32
	offsetLengthBuffer     []int32
	posBufferUpto          int

	payloadBytes    []byte
	payloadByteUpto int

	// two-level skip tracking
	level0LastDocID int
	level0LastPosFP int64
	level0LastPayFP int64
	level1LastDocID int
	level1LastPosFP int64
	level1LastPayFP int64

	docID           int
	lastDocID       int
	lastPosition    int
	lastStartOffset int
	docCount        int

	pforUtil *PForUtil
	forUtil  *ForUtil

	// field-level flags (set by SetField)
	writePositions bool
	writeFreqs     bool
	writePayloads  bool
	writeOffsets   bool
	fieldHasNorms  bool

	norms index.NumericDocValues

	// competitive-impact accumulators
	level0FreqNormAcc *CompetitiveImpactAccumulator
	level1FreqNormAcc *CompetitiveImpactAccumulator

	maxNumImpactsAtLevel0     int
	maxImpactNumBytesAtLevel0 int
	maxNumImpactsAtLevel1     int
	maxImpactNumBytesAtLevel1 int

	// scratch buffers used to prepend skip data lengths
	scratchOutput *store.ByteBuffersDataOutput
	level0Output  *store.ByteBuffersDataOutput
	level1Output  *store.ByteBuffersDataOutput

	// spareBitSet is reused per block for the bit-set encoding path.
	// Maximum required size: BLOCK_SIZE * 32 bits = 8192 bits = 128 uint64 words.
	spareBitSet *util.FixedBitSet

	// stateCache maps *BlockTermState handles (allocated by NewTermState) back
	// to the *IntBlockTermState that owns them. This is necessary because the
	// PostingsWriterBase interface deals only in *BlockTermState.
	stateCache map[*BlockTermState]*IntBlockTermState

	// lastState is the *IntBlockTermState from the previous EncodeTerm call
	// for delta-encoding.
	lastState *IntBlockTermState
}

// NewLucene104PostingsWriter creates a Lucene104PostingsWriter, opening the
// .psm and .doc (and optionally .pos and .pay) output files.
//
// Mirrors org.apache.lucene.codecs.lucene104.Lucene104PostingsWriter(SegmentWriteState).
func NewLucene104PostingsWriter(state *SegmentWriteState) (*Lucene104PostingsWriter, error) {
	return newLucene104PostingsWriterWithVersion(state, lucene104VersionCurrent)
}

func newLucene104PostingsWriterWithVersion(state *SegmentWriteState, version int) (*Lucene104PostingsWriter, error) {
	w := &Lucene104PostingsWriter{
		version:           version,
		docDeltaBuffer:    make([]int32, lucene104BlockSize),
		freqBuffer:        make([]int32, lucene104BlockSize),
		level0FreqNormAcc: NewCompetitiveImpactAccumulator(),
		level1FreqNormAcc: NewCompetitiveImpactAccumulator(),
		scratchOutput:     store.NewByteBuffersDataOutput(),
		level0Output:      store.NewByteBuffersDataOutput(),
		level1Output:      store.NewByteBuffersDataOutput(),
		stateCache:        make(map[*BlockTermState]*IntBlockTermState),
		lastState:         emptyIntBlockTermState,
	}

	// spareBitSet needs BLOCK_SIZE * Integer.SIZE bits = 256*32 = 8192 bits.
	spareBitSet, err := util.NewFixedBitSet(lucene104BlockSize * 32)
	if err != nil {
		return nil, fmt.Errorf("lucene104 postings writer: allocating spare bit set: %w", err)
	}
	w.spareBitSet = spareBitSet

	metaFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene104MetaExtension)
	docFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene104DocExtension)

	var posOut, payOut store.IndexOutput
	success := false
	defer func() {
		if !success {
			closeOutputs(w.metaOut, w.docOut, posOut, payOut)
		}
	}()

	rawMetaOut, err := state.Directory.CreateOutput(metaFileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("lucene104 postings writer: create %s: %w", metaFileName, err)
	}
	metaOut := store.NewChecksumIndexOutput(rawMetaOut)
	w.metaOut = metaOut

	rawDocOut, err := state.Directory.CreateOutput(docFileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("lucene104 postings writer: create %s: %w", docFileName, err)
	}
	docOut := store.NewChecksumIndexOutput(rawDocOut)
	w.docOut = docOut

	if err := WriteIndexHeader(metaOut, lucene104MetaCodec, int32(version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("lucene104 postings writer: write meta header: %w", err)
	}
	if err := WriteIndexHeader(docOut, lucene104DocCodec, int32(version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("lucene104 postings writer: write doc header: %w", err)
	}

	w.forUtil = NewForUtil()
	w.pforUtil = NewPForUtil(w.forUtil)

	if state.FieldInfos.HasProx() {
		w.posDeltaBuffer = make([]int32, lucene104BlockSize)
		posFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene104PosExtension)
		rawPosOut, posErr := state.Directory.CreateOutput(posFileName, store.IOContext{Context: store.ContextWrite})
		if posErr != nil {
			return nil, fmt.Errorf("lucene104 postings writer: create %s: %w", posFileName, posErr)
		}
		posOut = store.NewChecksumIndexOutput(rawPosOut)
		if err := WriteIndexHeader(posOut, lucene104PosCodec, int32(version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
			return nil, fmt.Errorf("lucene104 postings writer: write pos header: %w", err)
		}

		if state.FieldInfos.HasPayloads() {
			w.payloadBytes = make([]byte, 128)
			w.payloadLengthBuffer = make([]int32, lucene104BlockSize)
		}

		if state.FieldInfos.HasOffsets() {
			w.offsetStartDeltaBuffer = make([]int32, lucene104BlockSize)
			w.offsetLengthBuffer = make([]int32, lucene104BlockSize)
		}

		if state.FieldInfos.HasPayloads() || state.FieldInfos.HasOffsets() {
			payFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene104PayExtension)
			rawPayOut, payErr := state.Directory.CreateOutput(payFileName, store.IOContext{Context: store.ContextWrite})
			if payErr != nil {
				return nil, fmt.Errorf("lucene104 postings writer: create %s: %w", payFileName, payErr)
			}
			payOut = store.NewChecksumIndexOutput(rawPayOut)
			if err := WriteIndexHeader(payOut, lucene104PayCodec, int32(version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
				return nil, fmt.Errorf("lucene104 postings writer: write pay header: %w", err)
			}
		}
	}

	w.posOut = posOut
	w.payOut = payOut
	success = true
	return w, nil
}

// NewTermState returns a fresh *BlockTermState backed by a new *IntBlockTermState.
// The returned pointer is the embedded *BlockTermState of the IntBlockTermState;
// the mapping is stored in w.stateCache so that FinishTerm can recover the full
// extended state.
//
// Satisfies PostingsWriterBase.
func (w *Lucene104PostingsWriter) NewTermState() *BlockTermState {
	its := NewIntBlockTermState()
	w.stateCache[its.BlockTermState] = its
	return its.BlockTermState
}

// Init writes the codec header to the terms-index output and the BLOCK_SIZE
// hint consumed by the reader.
//
// Satisfies PostingsWriterBase.
func (w *Lucene104PostingsWriter) Init(termsOut store.IndexOutput, state *SegmentWriteState) error {
	if err := WriteIndexHeader(termsOut, lucene104TermsCodec, int32(w.version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return fmt.Errorf("lucene104 postings writer Init: write terms header: %w", err)
	}
	if err := store.WriteVInt(termsOut, lucene104BlockSize); err != nil {
		return fmt.Errorf("lucene104 postings writer Init: write block size: %w", err)
	}
	return nil
}

// SetField caches the field-level index options and resets the per-field state.
//
// Satisfies PostingsWriterBase.
func (w *Lucene104PostingsWriter) SetField(fieldInfo *index.FieldInfo) (int, error) {
	opts := fieldInfo.IndexOptions()
	w.writeFreqs = opts.HasFreqs()
	w.writePositions = opts.HasPositions()
	w.writePayloads = fieldInfo.HasPayloads()
	w.writeOffsets = opts.HasOffsets()
	w.fieldHasNorms = fieldInfo.HasNorms()
	w.lastState = emptyIntBlockTermState
	return 0, nil
}

// StartTerm resets all per-term cursors and records the starting file
// pointers for the current term's postings.
//
// Satisfies PostingsWriterBase.
func (w *Lucene104PostingsWriter) StartTerm(norms index.NumericDocValues) error {
	w.docStartFP = w.docOut.GetFilePointer()
	if w.writePositions {
		w.posStartFP = w.posOut.GetFilePointer()
		w.level1LastPosFP = w.posStartFP
		w.level0LastPosFP = w.posStartFP
		if w.writePayloads || w.writeOffsets {
			w.payStartFP = w.payOut.GetFilePointer()
			w.level1LastPayFP = w.payStartFP
			w.level0LastPayFP = w.payStartFP
		}
	}
	w.lastDocID = -1
	w.level0LastDocID = -1
	w.level1LastDocID = -1
	w.norms = norms
	if w.writeFreqs {
		w.level0FreqNormAcc.Clear()
	}
	return nil
}

// StartDoc records the doc-delta and optional frequency for the current
// document. When the doc buffer is full it is flushed as a packed block.
//
// Satisfies PushPostingsWriterBase.
func (w *Lucene104PostingsWriter) StartDoc(docID, freq int) error {
	if w.docBufferUpto == lucene104BlockSize {
		if err := w.flushDocBlock(false); err != nil {
			return err
		}
		w.docBufferUpto = 0
	}

	docDelta := docID - w.lastDocID
	if docID < 0 || docDelta <= 0 {
		return fmt.Errorf("lucene104 postings writer: docs out of order (%d <= %d)", docID, w.lastDocID)
	}

	w.docDeltaBuffer[w.docBufferUpto] = int32(docDelta)
	if w.writeFreqs {
		w.freqBuffer[w.docBufferUpto] = int32(freq)
	}

	w.docID = docID
	w.lastPosition = 0
	w.lastStartOffset = 0

	if w.writeFreqs {
		var norm int64 = 1
		if w.fieldHasNorms && w.norms != nil {
			// NumericDocValues is iterator-shaped after rmp #4710: position
			// the iterator on docID via AdvanceExact and read the value via
			// LongValue. Callers feed postings in strictly increasing
			// document order, satisfying AdvanceExact's monotonic-target
			// contract.
			ok, err := w.norms.AdvanceExact(docID)
			if err == nil && ok {
				if n, err := w.norms.LongValue(); err == nil && n != 0 {
					norm = n
				}
			}
		}
		w.level0FreqNormAcc.Add(freq, norm)
	}
	return nil
}

// AddPosition records a position occurrence for the current document.
//
// Satisfies PushPostingsWriterBase.
func (w *Lucene104PostingsWriter) AddPosition(position int, payload []byte, startOffset, endOffset int) error {
	if position > index.MaxPosition {
		return fmt.Errorf("lucene104 postings writer: position=%d > MaxPosition=%d", position, index.MaxPosition)
	}
	if position < 0 {
		return fmt.Errorf("lucene104 postings writer: position=%d < 0", position)
	}

	w.posDeltaBuffer[w.posBufferUpto] = int32(position - w.lastPosition)

	if w.writePayloads {
		if len(payload) == 0 {
			w.payloadLengthBuffer[w.posBufferUpto] = 0
		} else {
			w.payloadLengthBuffer[w.posBufferUpto] = int32(len(payload))
			needed := w.payloadByteUpto + len(payload)
			if needed > len(w.payloadBytes) {
				grown := make([]byte, needed*2)
				copy(grown, w.payloadBytes)
				w.payloadBytes = grown
			}
			copy(w.payloadBytes[w.payloadByteUpto:], payload)
			w.payloadByteUpto += len(payload)
		}
	}

	if w.writeOffsets {
		w.offsetStartDeltaBuffer[w.posBufferUpto] = int32(startOffset - w.lastStartOffset)
		w.offsetLengthBuffer[w.posBufferUpto] = int32(endOffset - startOffset)
		w.lastStartOffset = startOffset
	}

	w.posBufferUpto++
	w.lastPosition = position
	if w.posBufferUpto == lucene104BlockSize {
		if err := w.pforUtil.Encode(w.posDeltaBuffer, w.posOut); err != nil {
			return fmt.Errorf("lucene104 postings writer: encode pos block: %w", err)
		}
		if w.writePayloads {
			if err := w.pforUtil.Encode(w.payloadLengthBuffer, w.payOut); err != nil {
				return fmt.Errorf("lucene104 postings writer: encode payload lengths: %w", err)
			}
			if err := store.WriteVInt(w.payOut, int32(w.payloadByteUpto)); err != nil {
				return err
			}
			if err := w.payOut.WriteBytes(w.payloadBytes[:w.payloadByteUpto]); err != nil {
				return err
			}
			w.payloadByteUpto = 0
		}
		if w.writeOffsets {
			if err := w.pforUtil.Encode(w.offsetStartDeltaBuffer, w.payOut); err != nil {
				return fmt.Errorf("lucene104 postings writer: encode offset starts: %w", err)
			}
			if err := w.pforUtil.Encode(w.offsetLengthBuffer, w.payOut); err != nil {
				return fmt.Errorf("lucene104 postings writer: encode offset lengths: %w", err)
			}
		}
		w.posBufferUpto = 0
	}
	return nil
}

// FinishDoc finalises the current document. Called after all AddPosition
// calls for the document have been made.
//
// Satisfies PushPostingsWriterBase.
func (w *Lucene104PostingsWriter) FinishDoc() error {
	w.docBufferUpto++
	w.docCount++
	w.lastDocID = w.docID
	return nil
}

// FinishTerm finalises the current term, flushing any partial doc block and
// writing trailing VInt-encoded positions, then populates the IntBlockTermState
// with the file-pointer anchors. The supplied *BlockTermState must have been
// created by a prior call to NewTermState.
//
// Satisfies PostingsWriterBase.
func (w *Lucene104PostingsWriter) FinishTerm(base *BlockTermState) error {
	its, ok := w.stateCache[base]
	if !ok {
		// Fallback: treat it as a plain BlockTermState (legacy callers).
		its = &IntBlockTermState{BlockTermState: base, LastPosBlockOffset: -1, SingletonDocID: -1}
	}

	if base.DocFreq == 0 {
		return fmt.Errorf("lucene104 postings writer: FinishTerm called with docFreq=0")
	}
	if base.DocFreq != w.docCount {
		return fmt.Errorf("lucene104 postings writer: docFreq mismatch: state.DocFreq=%d docCount=%d", base.DocFreq, w.docCount)
	}

	var singletonDocID int
	if base.DocFreq == 1 {
		// Pulse the singleton doc ID into the term dictionary.
		singletonDocID = int(w.docDeltaBuffer[0]) - 1
	} else {
		singletonDocID = -1
		if err := w.flushDocBlock(true); err != nil {
			return err
		}
	}

	lastPosBlockOffset := int64(-1)
	if w.writePositions {
		if base.TotalTermFreq > int64(lucene104BlockSize) {
			lastPosBlockOffset = w.posOut.GetFilePointer() - w.posStartFP
		}
		if w.posBufferUpto > 0 {
			// VInt-encode the remaining trailing positions.
			if err := w.writeTrailingPositions(); err != nil {
				return err
			}
		}
	}

	its.DocStartFP = w.docStartFP
	its.PosStartFP = w.posStartFP
	its.PayStartFP = w.payStartFP
	its.SingletonDocID = singletonDocID
	its.LastPosBlockOffset = lastPosBlockOffset

	// Copy extended fields back into the base so callers see them via
	// the BlockTermState handle (TotalTermFreq is already set by the
	// block-tree writer before FinishTerm is called).
	_ = base // already modified via pointer — its.BlockTermState == base

	w.docBufferUpto = 0
	w.posBufferUpto = 0
	w.lastDocID = -1
	w.docCount = 0
	return nil
}

// writeTrailingPositions emits the remaining positions (< BLOCK_SIZE) as
// VInt-encoded data. Mirrors the Java finishTerm inner loop.
func (w *Lucene104PostingsWriter) writeTrailingPositions() error {
	lastPayloadLength := int32(-1)
	lastOffsetLength := int32(-1)
	payloadBytesReadUpto := 0

	for i := 0; i < w.posBufferUpto; i++ {
		posDelta := w.posDeltaBuffer[i]
		if w.writePayloads {
			payloadLength := int32(0)
			if w.payloadLengthBuffer != nil {
				payloadLength = w.payloadLengthBuffer[i]
			}
			if payloadLength != lastPayloadLength {
				lastPayloadLength = payloadLength
				if err := store.WriteVInt(w.posOut, (posDelta<<1)|1); err != nil {
					return err
				}
				if err := store.WriteVInt(w.posOut, payloadLength); err != nil {
					return err
				}
			} else {
				if err := store.WriteVInt(w.posOut, posDelta<<1); err != nil {
					return err
				}
			}
			if payloadLength != 0 {
				if err := w.posOut.WriteBytes(w.payloadBytes[payloadBytesReadUpto : payloadBytesReadUpto+int(payloadLength)]); err != nil {
					return err
				}
				payloadBytesReadUpto += int(payloadLength)
			}
		} else {
			if err := store.WriteVInt(w.posOut, posDelta); err != nil {
				return err
			}
		}

		if w.writeOffsets {
			delta := w.offsetStartDeltaBuffer[i]
			length := w.offsetLengthBuffer[i]
			if length == lastOffsetLength {
				if err := store.WriteVInt(w.posOut, delta<<1); err != nil {
					return err
				}
			} else {
				if err := store.WriteVInt(w.posOut, delta<<1|1); err != nil {
					return err
				}
				if err := store.WriteVInt(w.posOut, length); err != nil {
					return err
				}
				lastOffsetLength = length
			}
		}
	}

	if w.writePayloads {
		w.payloadByteUpto = 0
	}
	return nil
}

// EncodeTerm serialises the IntBlockTermState into out using delta-encoding
// relative to the previous term (or the empty sentinel when absolute=true).
//
// Satisfies PostingsWriterBase.
func (w *Lucene104PostingsWriter) EncodeTerm(out store.IndexOutput, fieldInfo *index.FieldInfo, base *BlockTermState, absolute bool) error {
	its, ok := w.stateCache[base]
	if !ok {
		its = &IntBlockTermState{BlockTermState: base, LastPosBlockOffset: -1, SingletonDocID: -1}
	}

	if absolute {
		w.lastState = emptyIntBlockTermState
	}
	last := w.lastState

	if last.SingletonDocID != -1 &&
		its.SingletonDocID != -1 &&
		its.DocStartFP == last.DocStartFP {
		// Runs of rare terms (e.g. ID fields) encode as doc-ID deltas.
		delta := int64(its.SingletonDocID) - int64(last.SingletonDocID)
		if err := store.WriteVLong(out, (util.ZigZagEncodeInt64(delta)<<1)|0x01); err != nil {
			return err
		}
	} else {
		if err := store.WriteVLong(out, (its.DocStartFP-last.DocStartFP)<<1); err != nil {
			return err
		}
		if its.SingletonDocID != -1 {
			if err := store.WriteVInt(out, int32(its.SingletonDocID)); err != nil {
				return err
			}
		}
	}

	if w.writePositions {
		if err := store.WriteVLong(out, its.PosStartFP-last.PosStartFP); err != nil {
			return err
		}
		if w.writePayloads || w.writeOffsets {
			if err := store.WriteVLong(out, its.PayStartFP-last.PayStartFP); err != nil {
				return err
			}
		}
	}
	if w.writePositions && its.LastPosBlockOffset != -1 {
		if err := store.WriteVLong(out, its.LastPosBlockOffset); err != nil {
			return err
		}
	}

	w.lastState = its
	return nil
}

// Close writes footers and metadata to all output files and releases their
// handles.
//
// Satisfies PostingsWriterBase.
func (w *Lucene104PostingsWriter) Close() error {
	success := false
	defer func() {
		if !success {
			closeOutputs(w.metaOut, w.docOut, w.posOut, w.payOut)
		}
	}()

	if w.docOut != nil {
		if err := WriteFooter(w.docOut); err != nil {
			return fmt.Errorf("lucene104 postings writer: write doc footer: %w", err)
		}
	}
	if w.posOut != nil {
		if err := WriteFooter(w.posOut); err != nil {
			return fmt.Errorf("lucene104 postings writer: write pos footer: %w", err)
		}
	}
	if w.payOut != nil {
		if err := WriteFooter(w.payOut); err != nil {
			return fmt.Errorf("lucene104 postings writer: write pay footer: %w", err)
		}
	}
	if w.metaOut != nil {
		if err := w.metaOut.WriteInt(int32(w.maxNumImpactsAtLevel0)); err != nil {
			return err
		}
		if err := w.metaOut.WriteInt(int32(w.maxImpactNumBytesAtLevel0)); err != nil {
			return err
		}
		if err := w.metaOut.WriteInt(int32(w.maxNumImpactsAtLevel1)); err != nil {
			return err
		}
		if err := w.metaOut.WriteInt(int32(w.maxImpactNumBytesAtLevel1)); err != nil {
			return err
		}
		if err := w.metaOut.WriteLong(w.docOut.GetFilePointer()); err != nil {
			return err
		}
		if w.posOut != nil {
			if err := w.metaOut.WriteLong(w.posOut.GetFilePointer()); err != nil {
				return err
			}
			if w.payOut != nil {
				if err := w.metaOut.WriteLong(w.payOut.GetFilePointer()); err != nil {
					return err
				}
			}
		}
		if err := WriteFooter(w.metaOut); err != nil {
			return fmt.Errorf("lucene104 postings writer: write meta footer: %w", err)
		}
	}
	success = true
	return closeOutputs(w.metaOut, w.docOut, w.posOut, w.payOut)
}

// ---------- private helpers ----------

// flushDocBlock encodes and writes the accumulated doc buffer.
// When finishTerm is true this is the partial (< BLOCK_SIZE) final block.
func (w *Lucene104PostingsWriter) flushDocBlock(finishTerm bool) error {
	if w.docBufferUpto == 0 {
		return nil
	}

	if w.docBufferUpto < lucene104BlockSize {
		// Terminal partial block: VInt-encode the docs/freqs into level0Output.
		// Mirrors Java: PostingsUtil.writeVIntBlock(level0Output, ...) then
		// falls through to the unconditional level0→level1→docOut flush below.
		if err := writeLucene104VIntBlock(
			w.level0Output,
			w.docDeltaBuffer,
			w.freqBuffer,
			w.docBufferUpto,
			w.writeFreqs,
		); err != nil {
			return err
		}
	} else {
		// Full block: write impacts + optional pos/pay deltas as level-0 skip
		// data, then encode the doc deltas and freqs.
		if w.writeFreqs {
			impacts := w.level0FreqNormAcc.GetCompetitiveFreqNormPairs()
			if len(impacts) > w.maxNumImpactsAtLevel0 {
				w.maxNumImpactsAtLevel0 = len(impacts)
			}
			if err := writeImpacts(impacts, w.scratchOutput); err != nil {
				return err
			}
			if w.level0Output.Size() != 0 {
				return fmt.Errorf("lucene104: level0Output must be empty before writing impacts")
			}
			impactBytes := int(w.scratchOutput.Size())
			if impactBytes > w.maxImpactNumBytesAtLevel0 {
				w.maxImpactNumBytesAtLevel0 = impactBytes
			}
			if err := w.level0Output.WriteVLong(w.scratchOutput.Size()); err != nil {
				return err
			}
			if err := w.scratchOutput.CopyTo(w.level0Output); err != nil {
				return err
			}
			w.scratchOutput.Reset()

			if w.writePositions {
				posFP := w.posOut.GetFilePointer()
				if err := w.level0Output.WriteVLong(posFP - w.level0LastPosFP); err != nil {
					return err
				}
				if err := w.level0Output.WriteByte(byte(w.posBufferUpto)); err != nil {
					return err
				}
				w.level0LastPosFP = posFP

				if w.writeOffsets || w.writePayloads {
					payFP := w.payOut.GetFilePointer()
					if err := w.level0Output.WriteVLong(payFP - w.level0LastPayFP); err != nil {
						return err
					}
					if err := w.level0Output.WriteVInt(int32(w.payloadByteUpto)); err != nil {
						return err
					}
					w.level0LastPayFP = payFP
				}
			}
		}

		// Encode doc deltas: choose between FOR, bit-set, or dense.
		numSkipBytes := w.level0Output.Size()
		if err := w.encodeDocBlock(); err != nil {
			return err
		}

		if w.writeFreqs {
			if err := w.pforUtil.Encode(w.freqBuffer, bbdoIndexOutput{w.level0Output}); err != nil {
				return fmt.Errorf("lucene104 postings writer: encode freq block: %w", err)
			}
		}

		// Write level-0 skip trailer into level1Output.
		if err := writeVInt15(w.scratchOutput, int32(w.docID-w.level0LastDocID)); err != nil {
			return err
		}
		if err := writeVLong15(w.scratchOutput, w.level0Output.Size()); err != nil {
			return err
		}
		numSkipBytes += w.scratchOutput.Size()
		if err := w.level1Output.WriteVLong(numSkipBytes); err != nil {
			return err
		}
		if err := w.scratchOutput.CopyTo(w.level1Output); err != nil {
			return err
		}
		w.scratchOutput.Reset()
	}

	// Unconditional: flush level0Output into level1Output.
	// Mirrors Java: level0Output.copyTo(level1Output) runs for both
	// partial-block and full-block paths.
	if err := w.level0Output.CopyTo(w.level1Output); err != nil {
		return err
	}
	w.level0Output.Reset()
	w.level0LastDocID = w.docID

	if w.writeFreqs {
		w.level1FreqNormAcc.AddAll(w.level0FreqNormAcc)
		w.level0FreqNormAcc.Clear()
	}

	if (w.docCount & lucene104Level1Mask) == 0 {
		if err := w.writeLevel1SkipData(); err != nil {
			return err
		}
		w.level1LastDocID = w.docID
		w.level1FreqNormAcc.Clear()
	} else if finishTerm {
		if err := w.level1Output.CopyTo(w.docOut); err != nil {
			return err
		}
		w.level1Output.Reset()
		w.level1FreqNormAcc.Clear()
	}
	return nil
}

// encodeDocBlock chooses the most compact representation for the current
// 256-entry doc-delta block and appends it to level0Output.
//
// Three cases (mirroring the Java reference):
//  1. docRange == BLOCK_SIZE: write a single 0-byte (all deltas are 1).
//  2. FOR encoding is tighter than the next bit width: write bitsPerValue + FOR.
//  3. Bit-set encoding is tighter: write -numBitSetLongs + raw words.
func (w *Lucene104PostingsWriter) encodeDocBlock() error {
	var orVal int32
	for _, d := range w.docDeltaBuffer {
		orVal |= d
	}
	bpv := bpvRequired(orVal)
	docRange := w.docID - w.level0LastDocID
	numBitSetLongs := bitsToWords(docRange)
	numBitsNextBpv := min(32, bpv+1) * lucene104BlockSize

	level0Out := bbdoIndexOutput{w.level0Output}

	if docRange == lucene104BlockSize {
		// All deltas are 1.
		return w.level0Output.WriteByte(0)
	}
	if numBitsNextBpv <= docRange {
		// FOR is more compact.
		if err := w.level0Output.WriteByte(byte(bpv)); err != nil {
			return err
		}
		return w.forUtil.Encode(w.docDeltaBuffer, bpv, level0Out)
	}
	// Bit-set is more compact.
	w.spareBitSet.ClearAll()
	s := -1
	for _, d := range w.docDeltaBuffer {
		s += int(d)
		w.spareBitSet.Set(s)
	}
	// numBitSetLongs fits in a byte (max 64 when BLOCK_SIZE=256, 32-bit range).
	if err := w.level0Output.WriteByte(byte(int8(-numBitSetLongs))); err != nil {
		return err
	}
	words := w.spareBitSet.GetBits()
	for i := 0; i < numBitSetLongs; i++ {
		if err := w.level0Output.WriteLong(int64(words[i])); err != nil {
			return err
		}
	}
	return nil
}

// writeLevel1SkipData flushes the accumulated level-1 output (one group of 32
// blocks = 8192 docs) to docOut, prefixed by skip metadata.
func (w *Lucene104PostingsWriter) writeLevel1SkipData() error {
	if err := store.WriteVInt(w.docOut, int32(w.docID-w.level1LastDocID)); err != nil {
		return err
	}

	// level1End is the expected docOut file pointer once both the skip metadata
	// and the buffered level-1 block (level1Output) have been written. The
	// invariant is verified after level1Output is copied, exactly as in
	// Lucene104PostingsWriter.writeLevel1SkipData (Lucene 10.4.0): the earlier
	// implementation checked the pointer before copying level1Output, so the
	// check spuriously failed for the writeFreqs && !writePositions case once a
	// full level-1 group (8192 docs) was flushed.
	var level1End int64
	if w.writeFreqs {
		impacts := w.level1FreqNormAcc.GetCompetitiveFreqNormPairs()
		if len(impacts) > w.maxNumImpactsAtLevel1 {
			w.maxNumImpactsAtLevel1 = len(impacts)
		}
		if err := writeImpacts(impacts, w.scratchOutput); err != nil {
			return err
		}
		numImpactBytes := w.scratchOutput.Size()
		if int(numImpactBytes) > w.maxImpactNumBytesAtLevel1 {
			w.maxImpactNumBytesAtLevel1 = int(numImpactBytes)
		}

		if w.writePositions {
			posFP := w.posOut.GetFilePointer()
			if err := w.scratchOutput.WriteVLong(posFP - w.level1LastPosFP); err != nil {
				return err
			}
			if err := w.scratchOutput.WriteByte(byte(w.posBufferUpto)); err != nil {
				return err
			}
			w.level1LastPosFP = posFP
			if w.writeOffsets || w.writePayloads {
				payFP := w.payOut.GetFilePointer()
				if err := w.scratchOutput.WriteVLong(payFP - w.level1LastPayFP); err != nil {
					return err
				}
				if err := w.scratchOutput.WriteVInt(int32(w.payloadByteUpto)); err != nil {
					return err
				}
				w.level1LastPayFP = payFP
			}
		}

		// level1Len = 2*Short.BYTES + scratchOutput.size() + level1Output.size()
		level1Len := int64(4) + w.scratchOutput.Size() + w.level1Output.Size()
		if err := store.WriteVLong(w.docOut, level1Len); err != nil {
			return err
		}
		level1End = w.docOut.GetFilePointer() + level1Len

		// Two shorts: total scratch size including the numImpactBytes short,
		// and the numImpactBytes itself.
		scratchPlusShort := int16(w.scratchOutput.Size() + 2 /*Short.BYTES*/)
		if err := w.docOut.WriteShort(scratchPlusShort); err != nil {
			return err
		}
		if err := w.docOut.WriteShort(int16(numImpactBytes)); err != nil {
			return err
		}
		if err := w.scratchOutput.CopyTo(w.docOut); err != nil {
			return err
		}
		w.scratchOutput.Reset()
	} else {
		if err := store.WriteVLong(w.docOut, w.level1Output.Size()); err != nil {
			return err
		}
		level1End = w.docOut.GetFilePointer() + w.level1Output.Size()
	}
	if err := w.level1Output.CopyTo(w.docOut); err != nil {
		return err
	}
	w.level1Output.Reset()

	if w.docOut.GetFilePointer() != level1End {
		return fmt.Errorf(
			"lucene104 postings writer: level1 length mismatch: pos=%d want=%d",
			w.docOut.GetFilePointer(), level1End,
		)
	}
	return nil
}

// writeVInt15 writes v as a 15-bit-optimised variable-length integer.
// Values that fit in 15 bits are emitted as a single 16-bit short;
// larger values prefix a short with the high bit set and follow with a vlong
// for the upper bits.
//
// Mirrors Lucene104PostingsWriter.writeVInt15.
func writeVInt15(out store.DataOutput, v int32) error {
	return writeVLong15(out, int64(v))
}

// writeVLong15 is the long-width variant of writeVInt15.
//
// Mirrors Lucene104PostingsWriter.writeVLong15.
func writeVLong15(out store.DataOutput, v int64) error {
	if (v & ^int64(0x7FFF)) == 0 {
		return out.WriteShort(int16(v))
	}
	if err := out.WriteShort(int16(0x8000 | (v & 0x7FFF))); err != nil {
		return err
	}
	return store.WriteVLong(out, v>>15)
}

// writeImpacts encodes a sorted slice of Impact values into out using
// delta-compression. Mirrors Lucene104PostingsWriter.writeImpacts.
//
// normDelta is written as a zig-zag encoded VLong (writeZLong) using the
// formula (v << 1) ^ (v >> 63), inlined here because store.DataOutput does
// not expose WriteZLong directly.
func writeImpacts(impacts []Impact, out store.DataOutput) error {
	prev := Impact{}
	for _, imp := range impacts {
		freqDelta := int32(imp.Freq - prev.Freq - 1)
		normDelta := imp.Norm - prev.Norm - 1
		if normDelta == 0 {
			if err := store.WriteVInt(out, freqDelta<<1); err != nil {
				return err
			}
		} else {
			if err := store.WriteVInt(out, (freqDelta<<1)|1); err != nil {
				return err
			}
			// zig-zag encode normDelta then write as VLong
			zigzag := (normDelta << 1) ^ (normDelta >> 63)
			if err := store.WriteVLong(out, zigzag); err != nil {
				return err
			}
		}
		prev = imp
	}
	return nil
}

// closeOutputs closes each non-nil output, collecting errors.
func closeOutputs(outs ...store.IndexOutput) error {
	var last error
	for _, o := range outs {
		if o != nil {
			if err := o.Close(); err != nil {
				last = err
			}
		}
	}
	return last
}

// bpvRequired returns the minimum bits-per-value needed to represent orVal.
func bpvRequired(orVal int32) int {
	if orVal == 0 {
		return 0
	}
	return 32 - bits.LeadingZeros32(uint32(orVal))
}

// bitsToWords returns the number of 64-bit words needed to hold numBits bits.
// Mirrors FixedBitSet.bits2words.
func bitsToWords(numBits int) int {
	if numBits <= 0 {
		return 0
	}
	return (numBits + 63) >> 6
}

// bbdoIndexOutput wraps a *store.ByteBuffersDataOutput to satisfy
// store.IndexOutput. The random-access methods are not used by ForUtil /
// PForUtil (they only write); GetFilePointer returns the size so callers that
// measure FP deltas see the correct byte count.
type bbdoIndexOutput struct {
	*store.ByteBuffersDataOutput
}

func (b bbdoIndexOutput) GetFilePointer() int64 { return b.ByteBuffersDataOutput.Size() }
func (b bbdoIndexOutput) SetPosition(_ int64) error {
	return fmt.Errorf("bbdoIndexOutput: SetPosition not supported")
}
func (b bbdoIndexOutput) Length() int64   { return b.ByteBuffersDataOutput.Size() }
func (b bbdoIndexOutput) GetName() string { return "<bbdo>" }
func (b bbdoIndexOutput) Close() error    { return nil }

// Compile-time interface satisfaction checks.
var (
	_ PostingsWriterBase     = (*Lucene104PostingsWriter)(nil)
	_ PushPostingsWriterBase = (*Lucene104PostingsWriter)(nil)
	_ store.IndexOutput      = bbdoIndexOutput{}
)
