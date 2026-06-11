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

// Source: lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/
//
//	lucene99/Lucene99PostingsWriter.java
//
// Purpose: write-side encoder for the Lucene 9.9 backward-codecs postings
// format. Block size is 128 (long-based ForUtil). Skip data is written inline
// into the .doc stream using a multi-level skip list. Positions are delta-
// encoded with PFor; trailing positions (< BLOCK_SIZE) use VInt encoding.
// Impacts (competitive freq/norm pairs) are written per block for skip levels.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ─── Extension constants ──────────────────────────────────────────────────────────

const (
	lucene99DocExtension = "doc"
	lucene99PosExtension = "pos"
	lucene99PayExtension = "pay"
)

// emptyIntBlockTermState is defined in lucene104_postings_writer.go (same
// package) and is reused here.

// ─── Lucene99PostingsWriter ────────────────────────────────────────────────────────

// Lucene99PostingsWriter writes .doc / .pos / .pay files using 128-wide
// long-based Frame-of-Reference encoding with a multi-level skip list. It is
// the Go port of
// org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsWriter from
// Apache Lucene 10.4.0.
//
// Lucene99PostingsWriter implements both PostingsWriterBase and
// PushPostingsWriterBase.
type Lucene99PostingsWriter struct {
	docOut store.IndexOutput
	posOut store.IndexOutput
	payOut store.IndexOutput

	// For delta encoding in EncodeTerm
	lastState *IntBlockTermState

	// Per-term file-pointer anchors
	docStartFP int64
	posStartFP int64
	payStartFP int64

	// Doc accumulation buffers (BLOCK_SIZE = 128, int64 = long)
	docDeltaBuffer []int64
	freqBuffer     []int64
	docBufferUpto  int

	// Position accumulation buffers
	posDeltaBuffer         []int64
	payloadLengthBuffer    []int64
	offsetStartDeltaBuffer []int64
	offsetLengthBuffer     []int64
	posBufferUpto          int

	// Payload bytes
	payloadBytes    []byte
	payloadByteUpto int

	// Block tracking for skip points
	lastBlockDocID           int
	lastBlockPosFP           int64
	lastBlockPayFP           int64
	lastBlockPosBufferUpto   int
	lastBlockPayloadByteUpto int

	// Doc-level accumulators
	lastDocID       int
	lastPosition    int
	lastStartOffset int
	docCount        int

	// Encoding utilities
	forDeltaUtil *lucene99ForDeltaUtil
	pforUtil     *lucene99PForUtil

	// Skip writer (multi-level skip list)
	skipWriter *lucene99SkipWriter

	// Field-level flags (set by SetField)
	writePositions bool
	writeFreqs     bool
	writePayloads  bool
	writeOffsets   bool
	fieldHasNorms  bool

	// Norms and competitive impact
	norms                 index.NumericDocValues
	competitiveFreqNormAcc *CompetitiveImpactAccumulator

	// Scratch buffer for GroupVInt encoding
	groupVIntScratch []byte

	// State cache: maps *BlockTermState (the handle returned by NewTermState)
	// back to the owning *IntBlockTermState whose extended fields are populated
	// in FinishTerm and read in EncodeTerm.
	stateCache map[*BlockTermState]*IntBlockTermState
}

// NewLucene99PostingsWriter creates a Lucene99PostingsWriter, opening the .doc
// (and optionally .pos and .pay) output files.
//
// Mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsWriter(SegmentWriteState).
func NewLucene99PostingsWriter(state *SegmentWriteState) (*Lucene99PostingsWriter, error) {
	w := &Lucene99PostingsWriter{
		docDeltaBuffer:        make([]int64, lucene99BlockSize),
		freqBuffer:            make([]int64, lucene99BlockSize),
		competitiveFreqNormAcc: NewCompetitiveImpactAccumulator(),
		stateCache:            make(map[*BlockTermState]*IntBlockTermState),
		lastState:             emptyIntBlockTermState,
		groupVIntScratch:      make([]byte, util.GroupVIntMaxLengthPerGroup),
	}

	// Create ForUtil, ForDeltaUtil, PForUtil
	forUtil := newLucene99ForUtil()
	w.forDeltaUtil = newLucene99ForDeltaUtil(forUtil)
	w.pforUtil = newLucene99PForUtil(forUtil)

	// Compute the doc file name and open it
	docFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene99DocExtension)

	var posOut, payOut store.IndexOutput
	success := false
	defer func() {
		if !success {
			closeOutputs(w.docOut, posOut, payOut)
		}
	}()

	rawDocOut, err := state.Directory.CreateOutput(docFileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("lucene99 postings writer: create %s: %w", docFileName, err)
	}
	w.docOut = store.NewChecksumIndexOutput(rawDocOut)

	if err := WriteIndexHeader(w.docOut, lucene99DocCodec, lucene99VersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("lucene99 postings writer: write doc header: %w", err)
	}

	// Conditionally open .pos and .pay files
	if state.FieldInfos.HasProx() {
		w.posDeltaBuffer = make([]int64, lucene99BlockSize)

		posFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene99PosExtension)
		rawPosOut, posErr := state.Directory.CreateOutput(posFileName, store.IOContext{Context: store.ContextWrite})
		if posErr != nil {
			return nil, fmt.Errorf("lucene99 postings writer: create %s: %w", posFileName, posErr)
		}
		posOut = store.NewChecksumIndexOutput(rawPosOut)
		if err := WriteIndexHeader(posOut, lucene99PosCodec, lucene99VersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
			return nil, fmt.Errorf("lucene99 postings writer: write pos header: %w", err)
		}

		if state.FieldInfos.HasPayloads() {
			w.payloadBytes = make([]byte, 128)
			w.payloadLengthBuffer = make([]int64, lucene99BlockSize)
		}

		if state.FieldInfos.HasOffsets() {
			w.offsetStartDeltaBuffer = make([]int64, lucene99BlockSize)
			w.offsetLengthBuffer = make([]int64, lucene99BlockSize)
		}

		if state.FieldInfos.HasPayloads() || state.FieldInfos.HasOffsets() {
			payFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene99PayExtension)
			rawPayOut, payErr := state.Directory.CreateOutput(payFileName, store.IOContext{Context: store.ContextWrite})
			if payErr != nil {
				return nil, fmt.Errorf("lucene99 postings writer: create %s: %w", payFileName, payErr)
			}
			payOut = store.NewChecksumIndexOutput(rawPayOut)
			if err := WriteIndexHeader(payOut, lucene99PayCodec, lucene99VersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
				return nil, fmt.Errorf("lucene99 postings writer: write pay header: %w", err)
			}
		}
	}

	w.posOut = posOut
	w.payOut = payOut

	// Create the skip writer using the segment's maxDoc for level computation
	w.skipWriter = newLucene99SkipWriter(
		lucene99MaxSkipLevels,
		lucene99BlockSize,
		state.SegmentInfo.DocCount(),
		w.docOut,
		w.posOut,
		w.payOut,
	)

	success = true
	return w, nil
}

// ─── PostingsWriterBase implementation ────────────────────────────────────────────

// Init writes the codec header to the terms-index output and the BLOCK_SIZE
// hint consumed by the reader.
//
// Satisfies PostingsWriterBase.
func (w *Lucene99PostingsWriter) Init(termsOut store.IndexOutput, state *SegmentWriteState) error {
	if err := WriteIndexHeader(termsOut, lucene99TermsCodec, lucene99VersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return fmt.Errorf("lucene99 postings writer Init: write terms header: %w", err)
	}
	if err := store.WriteVInt(termsOut, lucene99BlockSize); err != nil {
		return fmt.Errorf("lucene99 postings writer Init: write block size: %w", err)
	}
	return nil
}

// NewTermState returns a fresh *BlockTermState backed by a new *IntBlockTermState.
// The mapping is stored in w.stateCache so that FinishTerm can recover the full
// extended state.
//
// Satisfies PostingsWriterBase.
func (w *Lucene99PostingsWriter) NewTermState() *BlockTermState {
	its := NewIntBlockTermState()
	w.stateCache[its.BlockTermState] = its
	return its.BlockTermState
}

// SetField caches the field-level index options and resets per-field state.
//
// Satisfies PostingsWriterBase.
func (w *Lucene99PostingsWriter) SetField(fieldInfo *index.FieldInfo) (int, error) {
	opts := fieldInfo.IndexOptions()
	w.writeFreqs = opts.HasFreqs()
	w.writePositions = opts.HasPositions()
	w.writePayloads = fieldInfo.HasPayloads()
	w.writeOffsets = opts.HasOffsets()
	w.fieldHasNorms = fieldInfo.HasNorms()
	w.lastState = emptyIntBlockTermState
	w.skipWriter.setField(w.writePositions, w.writeOffsets, w.writePayloads)
	return 0, nil
}

// StartTerm resets all per-term cursors and records the starting file
// pointers for the current term's postings.
//
// Satisfies PostingsWriterBase.
func (w *Lucene99PostingsWriter) StartTerm(norms index.NumericDocValues) error {
	w.docStartFP = w.docOut.GetFilePointer()
	if w.writePositions {
		w.posStartFP = w.posOut.GetFilePointer()
		if w.writePayloads || w.writeOffsets {
			w.payStartFP = w.payOut.GetFilePointer()
		}
	}
	w.lastDocID = 0
	w.lastBlockDocID = -1
	w.skipWriter.resetSkip()
	w.norms = norms
	w.competitiveFreqNormAcc.Clear()
	return nil
}

// StartDoc records the doc-delta and optional frequency for the current
// document. When the doc buffer is full, the block is FOR-delta encoded.
// When starting a new block after a completed one, skip data is recorded.
//
// Satisfies PushPostingsWriterBase.
func (w *Lucene99PostingsWriter) StartDoc(docID, freq int) error {
	// At the start of a new block (after a full block was just finished),
	// buffer skip data for the previous block.
	if w.lastBlockDocID != -1 && w.docBufferUpto == 0 {
		if err := w.skipWriter.bufferSkip(
			w.lastBlockDocID,
			w.competitiveFreqNormAcc,
			w.docCount,
			w.lastBlockPosFP,
			w.lastBlockPayFP,
			w.lastBlockPosBufferUpto,
			w.lastBlockPayloadByteUpto,
		); err != nil {
			return fmt.Errorf("lucene99 postings writer: bufferSkip: %w", err)
		}
		w.competitiveFreqNormAcc.Clear()
	}

	docDelta := docID - w.lastDocID
	if docID < 0 || (w.docCount > 0 && docDelta <= 0) {
		return fmt.Errorf("lucene99 postings writer: docs out of order (%d <= %d)", docID, w.lastDocID)
	}

	w.docDeltaBuffer[w.docBufferUpto] = int64(docDelta)
	if w.writeFreqs {
		w.freqBuffer[w.docBufferUpto] = int64(freq)
	}

	w.docBufferUpto++
	w.docCount++

	// When the doc buffer is full, encode the block immediately (Java
	// startDoc does this inline, and finishDoc saves the skip data later).
	if w.docBufferUpto == lucene99BlockSize {
		if err := w.forDeltaUtil.encodeDeltas(w.docDeltaBuffer, w.docOut); err != nil {
			return fmt.Errorf("lucene99 postings writer: encode doc deltas: %w", err)
		}
		if w.writeFreqs {
			if err := w.pforUtil.encode(w.freqBuffer, w.docOut); err != nil {
				return fmt.Errorf("lucene99 postings writer: encode freqs: %w", err)
			}
		}
		// NOTE: docBufferUpto is NOT reset here; finishDoc will do so
		// after it records the skip state.
	}

	w.lastDocID = docID
	w.lastPosition = 0
	w.lastStartOffset = 0

	if w.fieldHasNorms {
		var norm int64 = 1
		if w.norms != nil {
			found, err := w.norms.AdvanceExact(docID)
			if err == nil && found {
				if n, err := w.norms.LongValue(); err == nil && n != 0 {
					norm = n
				}
			}
		}
		w.competitiveFreqNormAcc.Add(freq, norm)
	} else {
		w.competitiveFreqNormAcc.Add(1, 1)
	}

	return nil
}

// AddPosition records a position occurrence for the current document.
// When the position buffer is full, it is PFor-encoded.
//
// Satisfies PushPostingsWriterBase.
func (w *Lucene99PostingsWriter) AddPosition(position int, payload []byte, startOffset, endOffset int) error {
	if position > index.MaxPosition {
		return fmt.Errorf("lucene99 postings writer: position=%d > MaxPosition=%d", position, index.MaxPosition)
	}
	if position < 0 {
		return fmt.Errorf("lucene99 postings writer: position=%d < 0", position)
	}

	w.posDeltaBuffer[w.posBufferUpto] = int64(position - w.lastPosition)

	if w.writePayloads {
		if len(payload) == 0 {
			w.payloadLengthBuffer[w.posBufferUpto] = 0
		} else {
			w.payloadLengthBuffer[w.posBufferUpto] = int64(len(payload))
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
		if startOffset < w.lastStartOffset {
			return fmt.Errorf("lucene99 postings writer: offsets out of order: startOffset=%d < lastStartOffset=%d", startOffset, w.lastStartOffset)
		}
		if endOffset < startOffset {
			return fmt.Errorf("lucene99 postings writer: endOffset=%d < startOffset=%d", endOffset, startOffset)
		}
		w.offsetStartDeltaBuffer[w.posBufferUpto] = int64(startOffset - w.lastStartOffset)
		w.offsetLengthBuffer[w.posBufferUpto] = int64(endOffset - startOffset)
		w.lastStartOffset = startOffset
	}

	w.posBufferUpto++
	w.lastPosition = position

	if w.posBufferUpto == lucene99BlockSize {
		if err := w.pforUtil.encode(w.posDeltaBuffer, w.posOut); err != nil {
			return fmt.Errorf("lucene99 postings writer: encode pos block: %w", err)
		}

		if w.writePayloads {
			if err := w.pforUtil.encode(w.payloadLengthBuffer, w.payOut); err != nil {
				return fmt.Errorf("lucene99 postings writer: encode payload lengths: %w", err)
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
			if err := w.pforUtil.encode(w.offsetStartDeltaBuffer, w.payOut); err != nil {
				return fmt.Errorf("lucene99 postings writer: encode offset starts: %w", err)
			}
			if err := w.pforUtil.encode(w.offsetLengthBuffer, w.payOut); err != nil {
				return fmt.Errorf("lucene99 postings writer: encode offset lengths: %w", err)
			}
		}
		w.posBufferUpto = 0
	}
	return nil
}

// FinishDoc finalizes the current document. When a doc block has just been
// filled (docBufferUpto == BLOCK_SIZE), the skip state is saved and the
// buffer is reset for the next block.
//
// Satisfies PushPostingsWriterBase.
func (w *Lucene99PostingsWriter) FinishDoc() error {
	if w.docBufferUpto == lucene99BlockSize {
		w.lastBlockDocID = w.lastDocID
		if w.posOut != nil {
			if w.payOut != nil {
				w.lastBlockPayFP = w.payOut.GetFilePointer()
			}
			w.lastBlockPosFP = w.posOut.GetFilePointer()
			w.lastBlockPosBufferUpto = w.posBufferUpto
			w.lastBlockPayloadByteUpto = w.payloadByteUpto
		}
		w.docBufferUpto = 0
	}
	return nil
}

// FinishTerm finalizes the current term, flushing any partial doc block and
// writing trailing VInt-encoded positions. The supplied *BlockTermState must
// have been created by a prior call to NewTermState.
//
// Satisfies PostingsWriterBase.
func (w *Lucene99PostingsWriter) FinishTerm(base *BlockTermState) error {
	its, ok := w.stateCache[base]
	if !ok {
		return fmt.Errorf("lucene99 postings writer: FinishTerm called with unrecognized BlockTermState")
	}
	if base.DocFreq == 0 {
		return fmt.Errorf("lucene99 postings writer: FinishTerm called with docFreq=0")
	}
	if base.DocFreq != w.docCount {
		return fmt.Errorf("lucene99 postings writer: docFreq mismatch: state.DocFreq=%d docCount=%d", base.DocFreq, w.docCount)
	}

	// docFreq == 1: pulse the singleton doc ID into the term dictionary
	// (no separate file write).
	var singletonDocID int
	if base.DocFreq == 1 {
		singletonDocID = int(w.docDeltaBuffer[0])
	} else {
		singletonDocID = -1
		// Group VInt encode the remaining doc deltas and freqs.
		if err := writeLucene99VIntBlock(
			w.docOut,
			w.docDeltaBuffer,
			w.freqBuffer,
			w.docBufferUpto,
			w.writeFreqs,
			w.groupVIntScratch,
		); err != nil {
			return fmt.Errorf("lucene99 postings writer: write VInt block: %w", err)
		}
	}

	// Compute lastPosBlockOffset for positions
	lastPosBlockOffset := int64(-1)
	if w.writePositions {
		if base.TotalTermFreq > int64(lucene99BlockSize) {
			lastPosBlockOffset = w.posOut.GetFilePointer() - w.posStartFP
		}
		if w.posBufferUpto > 0 {
			if err := w.writeTrailingPositions(); err != nil {
				return fmt.Errorf("lucene99 postings writer: write trailing positions: %w", err)
			}
		}
	}

	// Compute skip offset
	var skipOffset int64
	if w.docCount > lucene99BlockSize {
		skipPointer, err := w.skipWriter.writeSkip(w.docOut)
		if err != nil {
			return fmt.Errorf("lucene99 postings writer: write skip: %w", err)
		}
		skipOffset = skipPointer - w.docStartFP
	} else {
		skipOffset = -1
	}

	// Populate IntBlockTermState
	its.DocStartFP = w.docStartFP
	its.PosStartFP = w.posStartFP
	its.PayStartFP = w.payStartFP
	its.SingletonDocID = singletonDocID
	its.SkipOffset = skipOffset
	its.LastPosBlockOffset = lastPosBlockOffset

	// Reset per-term accumulators
	w.docBufferUpto = 0
	w.posBufferUpto = 0
	w.lastDocID = 0
	w.docCount = 0
	return nil
}

// EncodeTerm serializes the IntBlockTermState into out using delta-encoding
// relative to the previous term (or the empty sentinel when absolute=true).
//
// Satisfies PostingsWriterBase.
func (w *Lucene99PostingsWriter) EncodeTerm(out store.IndexOutput, fieldInfo *index.FieldInfo, base *BlockTermState, absolute bool) error {
	its, ok := w.stateCache[base]
	if !ok {
		return fmt.Errorf("lucene99 postings writer: EncodeTerm called with unrecognized BlockTermState")
	}

	if absolute {
		w.lastState = emptyIntBlockTermState
	}
	last := w.lastState

	// With runs of rare values such as ID fields, the increment of pointers
	// in the docs file is often 0. Furthermore some ID schemes like
	// auto-increment IDs or Flake IDs are monotonic, so we encode the delta
	// between consecutive doc IDs to save space.
	if last.SingletonDocID != -1 &&
		its.SingletonDocID != -1 &&
		its.DocStartFP == last.DocStartFP {
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
	if w.writePositions {
		if its.LastPosBlockOffset != -1 {
			if err := store.WriteVLong(out, its.LastPosBlockOffset); err != nil {
				return err
			}
		}
	}
	if its.SkipOffset != -1 {
		if err := store.WriteVLong(out, its.SkipOffset); err != nil {
			return err
		}
	}

	w.lastState = its
	return nil
}

// Close writes footers to all output files and releases their handles.
//
// Satisfies PostingsWriterBase.
func (w *Lucene99PostingsWriter) Close() error {
	success := false
	defer func() {
		if !success {
			closeOutputs(w.docOut, w.posOut, w.payOut)
		}
	}()

	if w.docOut != nil {
		if err := WriteFooter(w.docOut); err != nil {
			return fmt.Errorf("lucene99 postings writer: write doc footer: %w", err)
		}
	}
	if w.posOut != nil {
		if err := WriteFooter(w.posOut); err != nil {
			return fmt.Errorf("lucene99 postings writer: write pos footer: %w", err)
		}
	}
	if w.payOut != nil {
		if err := WriteFooter(w.payOut); err != nil {
			return fmt.Errorf("lucene99 postings writer: write pay footer: %w", err)
		}
	}

	success = true
	return closeOutputs(w.docOut, w.posOut, w.payOut)
}

// ─── Private: trailing positions ─────────────────────────────────────────────────

// writeTrailingPositions emits the remaining positions (< BLOCK_SIZE) as
// VInt-encoded data. Mirrors the Java finishTerm inner loop.
func (w *Lucene99PostingsWriter) writeTrailingPositions() error {
	lastPayloadLength := int64(-1)
	lastOffsetLength := int64(-1)
	payloadBytesReadUpto := 0

	for i := 0; i < w.posBufferUpto; i++ {
		posDelta := w.posDeltaBuffer[i]
		if w.writePayloads {
			payloadLength := int64(0)
			if w.payloadLengthBuffer != nil {
				payloadLength = w.payloadLengthBuffer[i]
			}
			if payloadLength != lastPayloadLength {
				lastPayloadLength = payloadLength
				if err := store.WriteVInt(w.posOut, int32((posDelta<<1)|1)); err != nil {
					return err
				}
				if err := store.WriteVInt(w.posOut, int32(payloadLength)); err != nil {
					return err
				}
			} else {
				if err := store.WriteVInt(w.posOut, int32(posDelta<<1)); err != nil {
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
			if err := store.WriteVInt(w.posOut, int32(posDelta)); err != nil {
				return err
			}
		}

		if w.writeOffsets {
			delta := w.offsetStartDeltaBuffer[i]
			length := w.offsetLengthBuffer[i]
			if length == lastOffsetLength {
				if err := store.WriteVInt(w.posOut, int32(delta<<1)); err != nil {
					return err
				}
			} else {
				if err := store.WriteVInt(w.posOut, int32(delta<<1|1)); err != nil {
					return err
				}
				if err := store.WriteVInt(w.posOut, int32(length)); err != nil {
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

// ─── Private: VInt block helper ──────────────────────────────────────────────────

// writeLucene99VIntBlock writes doc deltas and optional freqs using
// group-varint encoding. Mirrors PostingsUtil.writeVIntBlock from the
// Lucene99 backward-codecs.
func writeLucene99VIntBlock(
	out store.DataOutput,
	docDeltaBuffer, freqBuffer []int64,
	num int,
	writeFreqs bool,
	scratch []byte,
) error {
	// When freqs are present, fold freq==1 into the doc delta's LSB:
	//   storedValue = (delta << 1) | (freq == 1 ? 1 : 0)
	if writeFreqs {
		for i := 0; i < num; i++ {
			var bit int64
			if freqBuffer[i] == 1 {
				bit = 1
			}
			docDeltaBuffer[i] = (docDeltaBuffer[i] << 1) | bit
		}
	}

	// Write the (possibly combined) doc values as group-varint.
	if err := util.WriteGroupVIntsInt64(out, scratch, docDeltaBuffer, num); err != nil {
		return err
	}

	// When freqs are present, write non-1 freqs as standalone VInts.
	if writeFreqs {
		for i := 0; i < num; i++ {
			freq := int32(freqBuffer[i])
			if freq != 1 {
				if err := store.WriteVInt(out, freq); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ─── Lucene99SkipWriter ───────────────────────────────────────────────────────────

// lucene99SkipWriter implements the Lucene99 skip list format as a multi-level
// skip list. It wraps MultiLevelSkipListWriter and provides the codec-specific
// writeSkipData callback.
type lucene99SkipWriter struct {
	inner *MultiLevelSkipListWriter

	// Per-level tracking arrays
	lastSkipDoc        []int
	lastSkipDocPointer []int64
	lastSkipPosPointer []int64
	lastSkipPayPointer []int64

	// Current skip state (set by bufferSkip, read by writeSkipData)
	curDoc               int
	curDocPointer        int64
	curPosPointer        int64
	curPayPointer        int64
	curPosBufferUpto     int
	curPayloadByteUpto   int

	// Per-level competitive impact accumulators
	curCompetitiveFreqNorms []*CompetitiveImpactAccumulator

	// Output file references
	docOut store.IndexOutput
	posOut store.IndexOutput
	payOut store.IndexOutput

	// Field flags
	fieldHasPositions bool
	fieldHasOffsets   bool
	fieldHasPayloads  bool

	// Lazy initialisation state
	initialized bool
	lastDocFP   int64
	lastPosFP   int64
	lastPayFP   int64

	// Scratch buffer for impacts
	freqNormOut *store.ByteArrayDataOutput
}

// newLucene99SkipWriter creates a skip writer for the Lucene99 postings format.
// maxSkipLevels is the maximum height of the skip list (Lucene99 uses 10).
// blockSize is the skip interval (128, same as the postings block size).
// maxDoc is used to compute the actual number of skip levels needed.
func newLucene99SkipWriter(maxSkipLevels, blockSize, maxDoc int, docOut, posOut, payOut store.IndexOutput) *lucene99SkipWriter {
	sw := &lucene99SkipWriter{
		lastSkipDoc:        make([]int, maxSkipLevels),
		lastSkipDocPointer: make([]int64, maxSkipLevels),
		docOut:             docOut,
		posOut:             posOut,
		payOut:             payOut,
		freqNormOut:        store.NewByteArrayDataOutput(0),
	}
	// Allocate per-level competitive accumulators
	sw.curCompetitiveFreqNorms = make([]*CompetitiveImpactAccumulator, maxSkipLevels)
	for i := 0; i < maxSkipLevels; i++ {
		sw.curCompetitiveFreqNorms[i] = NewCompetitiveImpactAccumulator()
	}
	// Position/pointer arrays are allocated only when positions are present.
	// They are lazily allocated when setField is called.

	// Create the inner MultiLevelSkipListWriter with a closure for writeSkipData.
	// The skipMultiplier is hard-coded to 8 per Lucene99.
	sw.inner = NewMultiLevelSkipListWriter(
		blockSize,
		8,
		maxSkipLevels,
		maxDoc,
		func(level int, skipBuf *store.ByteArrayDataOutput) error {
			return sw.writeSkipData(level, skipBuf)
		},
	)
	return sw
}

// setField caches the field-level flags and lazily allocates the position/payload
// pointer arrays if needed.
func (sw *lucene99SkipWriter) setField(fieldHasPositions, fieldHasOffsets, fieldHasPayloads bool) {
	sw.fieldHasPositions = fieldHasPositions
	sw.fieldHasOffsets = fieldHasOffsets
	sw.fieldHasPayloads = fieldHasPayloads

	// Lazily allocate pointer arrays when positions are present.
	// These are allocated once (preserved across terms) since the skip writer
	// is reused across all terms in a field.
	if fieldHasPositions && sw.lastSkipPosPointer == nil {
		n := len(sw.lastSkipDoc)
		sw.lastSkipPosPointer = make([]int64, n)
		if fieldHasOffsets || fieldHasPayloads {
			sw.lastSkipPayPointer = make([]int64, n)
		}
	}
}

// resetSkip resets the skip writer for a new term. It saves the current file
// pointers for lazy initialisation and clears the competitive accumulators.
// Mirrors Java Lucene99SkipWriter.resetSkip().
func (sw *lucene99SkipWriter) resetSkip() {
	sw.lastDocFP = sw.docOut.GetFilePointer()
	if sw.fieldHasPositions {
		sw.lastPosFP = sw.posOut.GetFilePointer()
		if sw.fieldHasOffsets || sw.fieldHasPayloads {
			sw.lastPayFP = sw.payOut.GetFilePointer()
		}
	}
	if sw.initialized {
		for _, acc := range sw.curCompetitiveFreqNorms {
			acc.Clear()
		}
	}
	sw.initialized = false
}

// initSkip lazily initialises the skip writer (creating/resetting per-level
// buffers and arrays). Mirrors Java Lucene99SkipWriter.initSkip().
func (sw *lucene99SkipWriter) initSkip() {
	if !sw.initialized {
		// Reset the inner MultiLevelSkipListWriter's buffers.
		sw.inner.Init()

		// Reset per-level tracking arrays to zero.
		for i := range sw.lastSkipDoc {
			sw.lastSkipDoc[i] = 0
			sw.lastSkipDocPointer[i] = sw.lastDocFP
		}
		if sw.fieldHasPositions && sw.lastSkipPosPointer != nil {
			for i := range sw.lastSkipPosPointer {
				sw.lastSkipPosPointer[i] = sw.lastPosFP
			}
			if sw.fieldHasOffsets || sw.fieldHasPayloads {
				for i := range sw.lastSkipPayPointer {
					sw.lastSkipPayPointer[i] = sw.lastPayFP
				}
			}
		}
		sw.initialized = true
	}
}

// bufferSkip records a skip point at the current document. Mirrors Java
// Lucene99SkipWriter.bufferSkip(int, CompetitiveImpactAccumulator, int, long,
// long, int, int).
func (sw *lucene99SkipWriter) bufferSkip(
	doc int,
	competitiveFreqNorms *CompetitiveImpactAccumulator,
	numDocs int,
	posFP int64,
	payFP int64,
	posBufferUpto int,
	payloadByteUpto int,
) error {
	sw.initSkip()
	sw.curDoc = doc
	sw.curDocPointer = sw.docOut.GetFilePointer()
	sw.curPosPointer = posFP
	sw.curPayPointer = payFP
	sw.curPosBufferUpto = posBufferUpto
	sw.curPayloadByteUpto = payloadByteUpto
	sw.curCompetitiveFreqNorms[0].Copy(competitiveFreqNorms)
	return sw.inner.BufferSkip(numDocs)
}

// writeSkip flushes the buffered skip data to docOut and returns the file
// pointer where the skip data starts. Mirrors inherited writeSkip(IndexOutput).
func (sw *lucene99SkipWriter) writeSkip(out store.IndexOutput) (int64, error) {
	return sw.inner.WriteSkip(out)
}

// writeSkipData writes the per-level skip payload for the current skip point.
// Mirrors Java Lucene99SkipWriter.writeSkipData(int, DataOutput).
func (sw *lucene99SkipWriter) writeSkipData(level int, skipBuf *store.ByteArrayDataOutput) error {
	// Doc ID delta
	delta := sw.curDoc - sw.lastSkipDoc[level]
	if err := skipBuf.WriteVInt(int32(delta)); err != nil {
		return err
	}
	sw.lastSkipDoc[level] = sw.curDoc

	// Doc pointer delta
	if err := skipBuf.WriteVLong(sw.curDocPointer - sw.lastSkipDocPointer[level]); err != nil {
		return err
	}
	sw.lastSkipDocPointer[level] = sw.curDocPointer

	// Position data
	if sw.fieldHasPositions {
		if err := skipBuf.WriteVLong(sw.curPosPointer - sw.lastSkipPosPointer[level]); err != nil {
			return err
		}
		sw.lastSkipPosPointer[level] = sw.curPosPointer

		if err := skipBuf.WriteVInt(int32(sw.curPosBufferUpto)); err != nil {
			return err
		}

		if sw.fieldHasPayloads {
			if err := skipBuf.WriteVInt(int32(sw.curPayloadByteUpto)); err != nil {
				return err
			}
		}

		if sw.fieldHasOffsets || sw.fieldHasPayloads {
			if err := skipBuf.WriteVLong(sw.curPayPointer - sw.lastSkipPayPointer[level]); err != nil {
				return err
			}
			sw.lastSkipPayPointer[level] = sw.curPayPointer
		}
	}

	// Competitive impact data
	competitiveFreqNorms := sw.curCompetitiveFreqNorms[level]

	// Propagate competitive impacts to the next level
	if level+1 < sw.inner.NumberOfSkipLevels() {
		sw.curCompetitiveFreqNorms[level+1].AddAll(competitiveFreqNorms)
	}

	// Write impacts to scratch, then copy to skipBuf
	sw.freqNormOut.Reset()
	if err := writeLucene99Impacts(competitiveFreqNorms.GetCompetitiveFreqNormPairs(), sw.freqNormOut); err != nil {
		return err
	}
	if err := skipBuf.WriteVInt(int32(sw.freqNormOut.Length())); err != nil {
		return err
	}
	if err := skipBuf.WriteBytes(sw.freqNormOut.GetBytes()); err != nil {
		return err
	}
	competitiveFreqNorms.Clear()
	return nil
}

// ─── Private: impact encoding ────────────────────────────────────────────────────

// writeLucene99Impacts encodes a slice of Impact values into out using
// delta compression. Mirrors Lucene99SkipWriter.writeImpacts.
func writeLucene99Impacts(impacts []Impact, out *store.ByteArrayDataOutput) error {
	prev := Impact{}
	for _, imp := range impacts {
		if imp.Freq <= prev.Freq {
			return fmt.Errorf("lucene99 impacts: freq must increase: %d <= %d", imp.Freq, prev.Freq)
		}
		freqDelta := int32(imp.Freq - prev.Freq - 1)
		normDelta := imp.Norm - prev.Norm - 1
		if normDelta == 0 {
			if err := out.WriteVInt(freqDelta << 1); err != nil {
				return err
			}
		} else {
			if err := out.WriteVInt((freqDelta << 1) | 1); err != nil {
				return err
			}
			// Zig-zag encode normDelta and write as VLong.
			// Equivalent to Java DataOutput.writeZLong.
			zigzag := (normDelta << 1) ^ (normDelta >> 63)
			if err := out.WriteVLong(zigzag); err != nil {
				return err
			}
		}
		prev = imp
	}
	return nil
}

// ─── Compile-time interface assertions ────────────────────────────────────────────

var (
	_ PostingsWriterBase     = (*Lucene99PostingsWriter)(nil)
	_ PushPostingsWriterBase = (*Lucene99PostingsWriter)(nil)
)
