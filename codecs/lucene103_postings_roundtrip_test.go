// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Source (writer + format): the TEST-ONLY read-write impersonation classes
//   lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/lucene103/
//       {Lucene103RWPostingsFormat,Lucene103PostingsWriter}.java
// from Apache Lucene 10.4.0. In Lucene 10.4.0, Lucene103PostingsFormat is a
// read-only backward-compatibility format; its writer lives only under
// src/test/ and is used solely to GENERATE fixtures that the production reader
// must then decode. Gocene mirrors that exactly: this file ports that test-only
// writer (lucene103PostingsTestWriter + lucene103RWPostingsFormat) so that the
// production Lucene103PostingsReader can be exercised by a Gocene-write ->
// Gocene-read round-trip.
//
// THESE WRITER TYPES ARE TEST-ONLY. The production Lucene103PostingsFormat in
// lucene103_postings_format.go returns errLucene103ReadOnly from FieldsConsumer,
// matching Apache Lucene's UnsupportedOperationException.
//
// Byte-parity against real Lucene 10.3 output is harness-gated: there is no
// Lucene 10.3 (v103) golden corpus in tools/lucene-fixtures/manifests/
// baseline.tsv yet (the bwc-lucene103-postings scenario remains a deferral;
// see the report accompanying rmp #24). These round-trip tests therefore prove
// internal self-consistency (write -> read recovers the exact logical input
// across the full 128-wide PFOR-delta layout) and exercise every IndexOptions /
// block / payload / offset combination.

package codecs

import (
	"fmt"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ─── test-only Lucene103 postings writer ──────────────────────────────────────
//
// lucene103PostingsTestWriter is a faithful port of the TEST-ONLY
// org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsWriter. It writes
// the 128-wide PFOR-delta .doc/.pos/.pay/.psm layout that the production
// Lucene103PostingsReader decodes. The only structural differences from the
// production Lucene104PostingsWriter are the 128-wide block size, the level-1
// group span (32*128 = 4096 docs), the FOR-delta doc-block encoder
// (forDeltaUtil.encodeDeltas, fused with the reader's decodeAndPrefixSum), and
// the Lucene103 codec names / extensions.

type lucene103PostingsTestWriter struct {
	version int

	metaOut store.IndexOutput
	docOut  store.IndexOutput
	posOut  store.IndexOutput
	payOut  store.IndexOutput

	docStartFP int64
	posStartFP int64
	payStartFP int64

	docDeltaBuffer []int32
	freqBuffer     []int32
	docBufferUpto  int

	posDeltaBuffer         []int32
	payloadLengthBuffer    []int32
	offsetStartDeltaBuffer []int32
	offsetLengthBuffer     []int32
	posBufferUpto          int

	payloadBytes    []byte
	payloadByteUpto int

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

	pforUtil     *lucene103PForUtil
	forDeltaUtil *lucene103ForDeltaUtil

	writePositions bool
	writeFreqs     bool
	writePayloads  bool
	writeOffsets   bool
	fieldHasNorms  bool

	norms index.NumericDocValues

	level0FreqNormAcc *CompetitiveImpactAccumulator
	level1FreqNormAcc *CompetitiveImpactAccumulator

	maxNumImpactsAtLevel0     int
	maxImpactNumBytesAtLevel0 int
	maxNumImpactsAtLevel1     int
	maxImpactNumBytesAtLevel1 int

	scratchOutput *store.ByteBuffersDataOutput
	level0Output  *store.ByteBuffersDataOutput
	level1Output  *store.ByteBuffersDataOutput

	spareBitSet *util.FixedBitSet

	stateCache map[*BlockTermState]*IntBlockTermState
	lastState  *IntBlockTermState
}

func newLucene103PostingsTestWriter(state *SegmentWriteState) (*lucene103PostingsTestWriter, error) {
	version := lucene103VersionCurrent
	w := &lucene103PostingsTestWriter{
		version:           version,
		docDeltaBuffer:    make([]int32, lucene103PostingsBlockSize),
		freqBuffer:        make([]int32, lucene103PostingsBlockSize),
		level0FreqNormAcc: NewCompetitiveImpactAccumulator(),
		level1FreqNormAcc: NewCompetitiveImpactAccumulator(),
		scratchOutput:     store.NewByteBuffersDataOutput(),
		level0Output:      store.NewByteBuffersDataOutput(),
		level1Output:      store.NewByteBuffersDataOutput(),
		stateCache:        make(map[*BlockTermState]*IntBlockTermState),
		lastState:         emptyIntBlockTermState,
		forDeltaUtil:      newLucene103ForDeltaUtil(),
		pforUtil:          newLucene103PForUtil(),
	}

	// spareBitSet needs BLOCK_SIZE * Integer.SIZE bits = 128*32 = 4096 bits.
	spareBitSet, err := util.NewFixedBitSet(lucene103PostingsBlockSize * 32)
	if err != nil {
		return nil, fmt.Errorf("lucene103 test writer: allocating spare bit set: %w", err)
	}
	w.spareBitSet = spareBitSet

	metaFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene103MetaExtension)
	docFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene103DocExtension)

	var posOut, payOut store.IndexOutput
	success := false
	defer func() {
		if !success {
			closeOutputs(w.metaOut, w.docOut, posOut, payOut)
		}
	}()

	rawMetaOut, err := state.Directory.CreateOutput(metaFileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("lucene103 test writer: create %s: %w", metaFileName, err)
	}
	metaOut := store.NewChecksumIndexOutput(rawMetaOut)
	w.metaOut = metaOut

	rawDocOut, err := state.Directory.CreateOutput(docFileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("lucene103 test writer: create %s: %w", docFileName, err)
	}
	docOut := store.NewChecksumIndexOutput(rawDocOut)
	w.docOut = docOut

	if err := WriteIndexHeader(metaOut, lucene103MetaCodec, int32(version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("lucene103 test writer: write meta header: %w", err)
	}
	if err := WriteIndexHeader(docOut, lucene103DocCodec, int32(version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("lucene103 test writer: write doc header: %w", err)
	}

	if state.FieldInfos.HasProx() {
		w.posDeltaBuffer = make([]int32, lucene103PostingsBlockSize)
		posFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene103PosExtension)
		rawPosOut, posErr := state.Directory.CreateOutput(posFileName, store.IOContext{Context: store.ContextWrite})
		if posErr != nil {
			return nil, fmt.Errorf("lucene103 test writer: create %s: %w", posFileName, posErr)
		}
		posOut = store.NewChecksumIndexOutput(rawPosOut)
		if err := WriteIndexHeader(posOut, lucene103PosCodec, int32(version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
			return nil, fmt.Errorf("lucene103 test writer: write pos header: %w", err)
		}

		if state.FieldInfos.HasPayloads() {
			w.payloadBytes = make([]byte, 128)
			w.payloadLengthBuffer = make([]int32, lucene103PostingsBlockSize)
		}
		if state.FieldInfos.HasOffsets() {
			w.offsetStartDeltaBuffer = make([]int32, lucene103PostingsBlockSize)
			w.offsetLengthBuffer = make([]int32, lucene103PostingsBlockSize)
		}

		if state.FieldInfos.HasPayloads() || state.FieldInfos.HasOffsets() {
			payFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene103PayExtension)
			rawPayOut, payErr := state.Directory.CreateOutput(payFileName, store.IOContext{Context: store.ContextWrite})
			if payErr != nil {
				return nil, fmt.Errorf("lucene103 test writer: create %s: %w", payFileName, payErr)
			}
			payOut = store.NewChecksumIndexOutput(rawPayOut)
			if err := WriteIndexHeader(payOut, lucene103PayCodec, int32(version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
				return nil, fmt.Errorf("lucene103 test writer: write pay header: %w", err)
			}
		}
	}

	w.posOut = posOut
	w.payOut = payOut
	success = true
	return w, nil
}

func (w *lucene103PostingsTestWriter) NewTermState() *BlockTermState {
	its := NewIntBlockTermState()
	w.stateCache[its.BlockTermState] = its
	return its.BlockTermState
}

func (w *lucene103PostingsTestWriter) Init(termsOut store.IndexOutput, state *SegmentWriteState) error {
	if err := WriteIndexHeader(termsOut, lucene103TermsCodec, int32(w.version), state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return fmt.Errorf("lucene103 test writer Init: write terms header: %w", err)
	}
	if err := store.WriteVInt(termsOut, lucene103PostingsBlockSize); err != nil {
		return fmt.Errorf("lucene103 test writer Init: write block size: %w", err)
	}
	return nil
}

func (w *lucene103PostingsTestWriter) SetField(fieldInfo *index.FieldInfo) (int, error) {
	opts := fieldInfo.IndexOptions()
	w.writeFreqs = opts.HasFreqs()
	w.writePositions = opts.HasPositions()
	w.writePayloads = fieldInfo.HasPayloads()
	w.writeOffsets = opts.HasOffsets()
	w.fieldHasNorms = fieldInfo.HasNorms()
	w.lastState = emptyIntBlockTermState
	return 0, nil
}

func (w *lucene103PostingsTestWriter) StartTerm(norms index.NumericDocValues) error {
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

func (w *lucene103PostingsTestWriter) StartDoc(docID, freq int) error {
	if w.docBufferUpto == lucene103PostingsBlockSize {
		if err := w.flushDocBlock(false); err != nil {
			return err
		}
		w.docBufferUpto = 0
	}

	docDelta := docID - w.lastDocID
	if docID < 0 || docDelta <= 0 {
		return fmt.Errorf("lucene103 test writer: docs out of order (%d <= %d)", docID, w.lastDocID)
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

func (w *lucene103PostingsTestWriter) AddPosition(position int, payload []byte, startOffset, endOffset int) error {
	if position > index.MaxPosition {
		return fmt.Errorf("lucene103 test writer: position=%d > MaxPosition=%d", position, index.MaxPosition)
	}
	if position < 0 {
		return fmt.Errorf("lucene103 test writer: position=%d < 0", position)
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
	if w.posBufferUpto == lucene103PostingsBlockSize {
		if err := w.pforUtil.encode(w.posDeltaBuffer, w.posOut); err != nil {
			return fmt.Errorf("lucene103 test writer: encode pos block: %w", err)
		}
		if w.writePayloads {
			if err := w.pforUtil.encode(w.payloadLengthBuffer, w.payOut); err != nil {
				return fmt.Errorf("lucene103 test writer: encode payload lengths: %w", err)
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
				return fmt.Errorf("lucene103 test writer: encode offset starts: %w", err)
			}
			if err := w.pforUtil.encode(w.offsetLengthBuffer, w.payOut); err != nil {
				return fmt.Errorf("lucene103 test writer: encode offset lengths: %w", err)
			}
		}
		w.posBufferUpto = 0
	}
	return nil
}

func (w *lucene103PostingsTestWriter) FinishDoc() error {
	w.docBufferUpto++
	w.docCount++
	w.lastDocID = w.docID
	return nil
}

func (w *lucene103PostingsTestWriter) FinishTerm(base *BlockTermState) error {
	its, ok := w.stateCache[base]
	if !ok {
		its = &IntBlockTermState{BlockTermState: base, LastPosBlockOffset: -1, SingletonDocID: -1}
	}
	if base.DocFreq == 0 {
		return fmt.Errorf("lucene103 test writer: FinishTerm called with docFreq=0")
	}
	if base.DocFreq != w.docCount {
		return fmt.Errorf("lucene103 test writer: docFreq mismatch: state.DocFreq=%d docCount=%d", base.DocFreq, w.docCount)
	}

	var singletonDocID int
	if base.DocFreq == 1 {
		singletonDocID = int(w.docDeltaBuffer[0]) - 1
	} else {
		singletonDocID = -1
		if err := w.flushDocBlock(true); err != nil {
			return err
		}
	}

	lastPosBlockOffset := int64(-1)
	if w.writePositions {
		if base.TotalTermFreq > int64(lucene103PostingsBlockSize) {
			lastPosBlockOffset = w.posOut.GetFilePointer() - w.posStartFP
		}
		if w.posBufferUpto > 0 {
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

	w.docBufferUpto = 0
	w.posBufferUpto = 0
	w.lastDocID = -1
	w.docCount = 0
	return nil
}

func (w *lucene103PostingsTestWriter) writeTrailingPositions() error {
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

func (w *lucene103PostingsTestWriter) EncodeTerm(out store.IndexOutput, fieldInfo *index.FieldInfo, base *BlockTermState, absolute bool) error {
	its, ok := w.stateCache[base]
	if !ok {
		its = &IntBlockTermState{BlockTermState: base, LastPosBlockOffset: -1, SingletonDocID: -1}
	}
	if absolute {
		w.lastState = emptyIntBlockTermState
	}
	last := w.lastState

	if last.SingletonDocID != -1 && its.SingletonDocID != -1 && its.DocStartFP == last.DocStartFP {
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

func (w *lucene103PostingsTestWriter) Close() error {
	success := false
	defer func() {
		if !success {
			closeOutputs(w.metaOut, w.docOut, w.posOut, w.payOut)
		}
	}()

	if w.docOut != nil {
		if err := WriteFooter(w.docOut); err != nil {
			return fmt.Errorf("lucene103 test writer: write doc footer: %w", err)
		}
	}
	if w.posOut != nil {
		if err := WriteFooter(w.posOut); err != nil {
			return fmt.Errorf("lucene103 test writer: write pos footer: %w", err)
		}
	}
	if w.payOut != nil {
		if err := WriteFooter(w.payOut); err != nil {
			return fmt.Errorf("lucene103 test writer: write pay footer: %w", err)
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
			return fmt.Errorf("lucene103 test writer: write meta footer: %w", err)
		}
	}
	success = true
	return closeOutputs(w.metaOut, w.docOut, w.posOut, w.payOut)
}

// flushDocBlock mirrors the test-only Lucene103PostingsWriter.flushDocBlock. The
// key Lucene103 difference from the 104 writer is the use of
// forDeltaUtil.encodeDeltas for the FOR-coded doc-delta block.
func (w *lucene103PostingsTestWriter) flushDocBlock(finishTerm bool) error {
	if w.docBufferUpto == 0 {
		return nil
	}

	if w.docBufferUpto < lucene103PostingsBlockSize {
		if err := writeLucene104VIntBlock(
			w.level0Output, w.docDeltaBuffer, w.freqBuffer, w.docBufferUpto, w.writeFreqs,
		); err != nil {
			return err
		}
	} else {
		if w.writeFreqs {
			impacts := w.level0FreqNormAcc.GetCompetitiveFreqNormPairs()
			if len(impacts) > w.maxNumImpactsAtLevel0 {
				w.maxNumImpactsAtLevel0 = len(impacts)
			}
			if err := writeImpacts(impacts, w.scratchOutput); err != nil {
				return err
			}
			if w.level0Output.Size() != 0 {
				return fmt.Errorf("lucene103: level0Output must be empty before writing impacts")
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

		numSkipBytes := w.level0Output.Size()
		if err := w.encodeDocBlock(); err != nil {
			return err
		}

		if w.writeFreqs {
			if err := w.pforUtil.encode(w.freqBuffer, bbdoIndexOutput{w.level0Output}); err != nil {
				return fmt.Errorf("lucene103 test writer: encode freq block: %w", err)
			}
		}

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

	if err := w.level0Output.CopyTo(w.level1Output); err != nil {
		return err
	}
	w.level0Output.Reset()
	w.level0LastDocID = w.docID

	if w.writeFreqs {
		w.level1FreqNormAcc.AddAll(w.level0FreqNormAcc)
		w.level0FreqNormAcc.Clear()
	}

	if (w.docCount & (lucene103Level1NumDocs - 1)) == 0 {
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

// encodeDocBlock chooses the most compact representation for the 128-entry
// doc-delta block. Lucene103 uses forDeltaUtil.encodeDeltas for the FOR path
// (vs the 104 writer's forUtil.Encode).
func (w *lucene103PostingsTestWriter) encodeDocBlock() error {
	bpv, err := w.forDeltaUtil.bitsRequired(w.docDeltaBuffer)
	if err != nil {
		return err
	}
	docRange := w.docID - w.level0LastDocID
	numBitSetLongs := bitsToWords(docRange)
	numBitsNextBpv := min(32, bpv+1) * lucene103PostingsBlockSize

	if docRange == lucene103PostingsBlockSize {
		return w.level0Output.WriteByte(0)
	}
	if numBitsNextBpv <= docRange {
		if err := w.level0Output.WriteByte(byte(bpv)); err != nil {
			return err
		}
		// encodeDeltas mutates docDeltaBuffer in place (collapse8/16); that is
		// fine because the buffer is reset at the start of the next term.
		return w.forDeltaUtil.encodeDeltas(bpv, w.docDeltaBuffer, bbdoIndexOutput{w.level0Output})
	}
	// Bit-set encoding.
	w.spareBitSet.ClearAll()
	s := -1
	for _, d := range w.docDeltaBuffer {
		s += int(d)
		w.spareBitSet.Set(s)
	}
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

func (w *lucene103PostingsTestWriter) writeLevel1SkipData() error {
	if err := store.WriteVInt(w.docOut, int32(w.docID-w.level1LastDocID)); err != nil {
		return err
	}

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

		level1Len := int64(4) + w.scratchOutput.Size() + w.level1Output.Size()
		if err := store.WriteVLong(w.docOut, level1Len); err != nil {
			return err
		}
		level1End = w.docOut.GetFilePointer() + level1Len

		scratchPlusShort := int16(w.scratchOutput.Size() + 2)
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
		return fmt.Errorf("lucene103 test writer: level1 length mismatch: pos=%d want=%d",
			w.docOut.GetFilePointer(), level1End)
	}
	return nil
}

// Compile-time interface checks for the test-only writer.
var (
	_ PostingsWriterBase     = (*lucene103PostingsTestWriter)(nil)
	_ PushPostingsWriterBase = (*lucene103PostingsTestWriter)(nil)
)

// ─── test-only Lucene103 read-write format ────────────────────────────────────

// lucene103RWPostingsFormat is the read-write impersonation of
// Lucene103PostingsFormat used only by tests, mirroring the test-only Java
// Lucene103RWPostingsFormat. Its FieldsConsumer wires the test-only writer
// through the production Lucene103BlockTreeTermsWriter; FieldsProducer is
// inherited from the production read-only format.
type lucene103RWPostingsFormat struct {
	*Lucene103PostingsFormat
}

func newLucene103RWPostingsFormat() *lucene103RWPostingsFormat {
	return &lucene103RWPostingsFormat{
		Lucene103PostingsFormat: NewLucene103PostingsFormat(),
	}
}

func (f *lucene103RWPostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	pw, err := newLucene103PostingsTestWriter(state)
	if err != nil {
		return nil, err
	}
	btw, err := NewLucene103BlockTreeTermsWriter(
		state, pw, f.MinTermBlockSize(), f.MaxTermBlockSize(),
	)
	if err != nil {
		_ = pw.Close()
		return nil, err
	}
	return btw, nil
}

// ─── payload/offset-carrying mock terms ───────────────────────────────────────
//
// l103Terms is a minimal index.Terms implementation that carries explicit
// per-position payloads and offsets (unlike SeedTerms, whose SeedPostingsEnum
// returns nil payloads). It drives the block-tree writer's Write(field, terms)
// with full control over the postings content.

type l103Doc struct {
	docID     int
	positions []int
	payloads  [][]byte
	offsets   []l103Off
}

type l103Off struct{ start, end int }

type l103Term struct {
	text string
	docs []l103Doc
}

type l103Terms struct {
	field      string
	terms      []*l103Term // sorted by text
	hasFreqs   bool
	hasPos     bool
	hasOffsets bool
	hasPay     bool
}

func (t *l103Terms) GetIterator() (index.TermsEnum, error) {
	return &l103TermsEnum{parent: t, pos: -1}, nil
}
func (t *l103Terms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	te := &l103TermsEnum{parent: t, pos: -1}
	if _, err := te.SeekCeil(seekTerm); err != nil {
		return nil, err
	}
	return te, nil
}
func (t *l103Terms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	for _, tm := range t.terms {
		if tm.text == termText {
			return &l103PostingsEnum{parent: t, term: tm, pos: -1}, nil
		}
	}
	return nil, nil
}
func (t *l103Terms) Size() int64 { return int64(len(t.terms)) }
func (t *l103Terms) GetDocCount() (int, error) {
	set := map[int]struct{}{}
	for _, tm := range t.terms {
		for _, d := range tm.docs {
			set[d.docID] = struct{}{}
		}
	}
	return len(set), nil
}
func (t *l103Terms) GetSumDocFreq() (int64, error) {
	var n int64
	for _, tm := range t.terms {
		n += int64(len(tm.docs))
	}
	return n, nil
}
func (t *l103Terms) GetSumTotalTermFreq() (int64, error) {
	if !t.hasFreqs {
		return -1, nil
	}
	var n int64
	for _, tm := range t.terms {
		for _, d := range tm.docs {
			if t.hasPos {
				n += int64(len(d.positions))
			} else {
				n++
			}
		}
	}
	return n, nil
}
func (t *l103Terms) HasFreqs() bool     { return t.hasFreqs }
func (t *l103Terms) HasOffsets() bool   { return t.hasOffsets }
func (t *l103Terms) HasPositions() bool { return t.hasPos }
func (t *l103Terms) HasPayloads() bool  { return t.hasPay }
func (t *l103Terms) GetMin() (*index.Term, error) {
	if len(t.terms) == 0 {
		return nil, nil
	}
	return index.NewTerm(t.field, t.terms[0].text), nil
}
func (t *l103Terms) GetMax() (*index.Term, error) {
	if len(t.terms) == 0 {
		return nil, nil
	}
	return index.NewTerm(t.field, t.terms[len(t.terms)-1].text), nil
}

type l103TermsEnum struct {
	index.TermsEnumBase
	parent *l103Terms
	pos    int
	curr   *index.Term
}

func (m *l103TermsEnum) Next() (*index.Term, error) {
	m.pos++
	if m.pos >= len(m.parent.terms) {
		m.curr = nil
		return nil, nil
	}
	m.curr = index.NewTerm(m.parent.field, m.parent.terms[m.pos].text)
	return m.curr, nil
}
func (m *l103TermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	for i, tm := range m.parent.terms {
		if tm.text >= term.Text() {
			m.pos = i
			m.curr = index.NewTerm(m.parent.field, tm.text)
			return m.curr, nil
		}
	}
	m.pos = len(m.parent.terms)
	m.curr = nil
	return nil, nil
}
func (m *l103TermsEnum) SeekExact(term *index.Term) (bool, error) {
	got, err := m.SeekCeil(term)
	return err == nil && got != nil && got.Equals(term), err
}
func (m *l103TermsEnum) Term() *index.Term { return m.curr }
func (m *l103TermsEnum) DocFreq() (int, error) {
	if m.curr == nil {
		return 0, nil
	}
	return len(m.parent.terms[m.pos].docs), nil
}
func (m *l103TermsEnum) TotalTermFreq() (int64, error) {
	if m.curr == nil {
		return 0, nil
	}
	var n int64
	for _, d := range m.parent.terms[m.pos].docs {
		if m.parent.hasPos {
			n += int64(len(d.positions))
		} else {
			n++
		}
	}
	return n, nil
}
func (m *l103TermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if m.curr == nil {
		return nil, nil
	}
	return &l103PostingsEnum{parent: m.parent, term: m.parent.terms[m.pos], pos: -1}, nil
}
func (m *l103TermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return m.Postings(flags)
}

type l103PostingsEnum struct {
	index.PostingsEnumBase
	parent  *l103Terms
	term    *l103Term
	pos     int
	posIdx  int
	currDoc int
}

func (p *l103PostingsEnum) NextDoc() (int, error) {
	p.pos++
	p.posIdx = 0
	if p.pos >= len(p.term.docs) {
		p.currDoc = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	p.currDoc = p.term.docs[p.pos].docID
	return p.currDoc, nil
}
func (p *l103PostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := p.NextDoc()
		if err != nil || doc >= target || doc == index.NO_MORE_DOCS {
			return doc, err
		}
	}
}
func (p *l103PostingsEnum) DocID() int {
	if p.pos < 0 {
		return -1
	}
	return p.currDoc
}
func (p *l103PostingsEnum) Freq() (int, error) {
	if p.pos < 0 || p.pos >= len(p.term.docs) {
		return 0, nil
	}
	if p.parent.hasPos {
		return len(p.term.docs[p.pos].positions), nil
	}
	return 1, nil
}
func (p *l103PostingsEnum) NextPosition() (int, error) {
	if p.pos < 0 || p.pos >= len(p.term.docs) {
		return index.NO_MORE_POSITIONS, nil
	}
	positions := p.term.docs[p.pos].positions
	if p.posIdx >= len(positions) {
		return index.NO_MORE_POSITIONS, nil
	}
	pos := positions[p.posIdx]
	p.posIdx++
	return pos, nil
}
func (p *l103PostingsEnum) StartOffset() (int, error) {
	idx := p.posIdx - 1
	if p.pos < 0 || p.pos >= len(p.term.docs) || idx < 0 || idx >= len(p.term.docs[p.pos].offsets) {
		return -1, nil
	}
	return p.term.docs[p.pos].offsets[idx].start, nil
}
func (p *l103PostingsEnum) EndOffset() (int, error) {
	idx := p.posIdx - 1
	if p.pos < 0 || p.pos >= len(p.term.docs) || idx < 0 || idx >= len(p.term.docs[p.pos].offsets) {
		return -1, nil
	}
	return p.term.docs[p.pos].offsets[idx].end, nil
}
func (p *l103PostingsEnum) GetPayload() ([]byte, error) {
	idx := p.posIdx - 1
	if p.pos < 0 || p.pos >= len(p.term.docs) || idx < 0 || idx >= len(p.term.docs[p.pos].payloads) {
		return nil, nil
	}
	return p.term.docs[p.pos].payloads[idx], nil
}
func (p *l103PostingsEnum) Cost() int64 { return int64(len(p.term.docs)) }

// ─── round-trip harness ───────────────────────────────────────────────────────

// l103WriteState builds a write state whose FieldInfos reflect the requested
// options (including stored payloads).
func l103WriteState(t *testing.T, dir store.Directory, name, field string, opts index.IndexOptions, storePayloads bool) *SegmentWriteState {
	t.Helper()
	si := index.NewSegmentInfo(name, 100, dir)
	if err := si.SetID(make([]byte, 16)); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	fis := index.NewFieldInfos()
	fi := index.NewFieldInfo(field, 0, index.FieldInfoOptions{IndexOptions: opts})
	if storePayloads {
		fi.SetStorePayloads()
	}
	if err := fis.Add(fi); err != nil {
		t.Fatalf("FieldInfos.Add: %v", err)
	}
	return &SegmentWriteState{Directory: dir, SegmentInfo: si, FieldInfos: fis}
}

// l103RoundTrip writes terms via the test-only RW format, then reads them back
// via the production read-only format and asserts an exact match.
func l103RoundTrip(t *testing.T, opts index.IndexOptions, storePayloads bool, terms *l103Terms) {
	t.Helper()
	// The block-tree writer requires terms in ascending (BytesRef/UTF-8) order;
	// for our ASCII term texts that is plain string order.
	sort.Slice(terms.terms, func(i, j int) bool { return terms.terms[i].text < terms.terms[j].text })

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	ws := l103WriteState(t, dir, "_0", terms.field, opts, storePayloads)

	rw := newLucene103RWPostingsFormat()
	consumer, err := rw.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	if err := consumer.Write(terms.field, terms); err != nil {
		_ = consumer.Close()
		t.Fatalf("Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	// Read back through the PRODUCTION read-only format + production reader.
	prod := NewLucene103PostingsFormat()
	rs := &SegmentReadState{Directory: ws.Directory, SegmentInfo: ws.SegmentInfo, FieldInfos: ws.FieldInfos}
	producer, err := prod.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close()

	// CheckIntegrity walks the block-tree reader, which delegates to the
	// production Lucene103PostingsReader.CheckIntegrity (CRC footer validation
	// over .doc/.pos/.pay). FieldsProducer (the SPI alias) does not surface
	// CheckIntegrity, so assert to the concrete reader.
	type integrityChecker interface{ CheckIntegrity() error }
	if ic, ok := producer.(integrityChecker); ok {
		if err := ic.CheckIntegrity(); err != nil {
			t.Fatalf("CheckIntegrity: %v", err)
		}
	} else {
		t.Fatalf("FieldsProducer %T does not implement CheckIntegrity", producer)
	}

	l103AssertTerms(t, producer, terms, opts, storePayloads)
}

func l103AssertTerms(t *testing.T, producer FieldsProducer, exp *l103Terms, opts index.IndexOptions, storePayloads bool) {
	t.Helper()
	terms, err := producer.Terms(exp.field)
	if err != nil || terms == nil {
		t.Fatalf("Terms(%q): %v (nil=%v)", exp.field, err, terms == nil)
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}

	hasFreqs := opts >= index.IndexOptionsDocsAndFreqs
	hasPos := opts >= index.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets

	flags := index.PostingsFlagFreqs
	if hasPos {
		flags = index.PostingsFlagPositions
	}
	if hasOffsets {
		flags |= index.PostingsFlagOffsets
	}
	if storePayloads {
		flags |= index.PostingsFlagPayloads
	}

	for _, want := range exp.terms {
		found, err := te.SeekExact(index.NewTerm(exp.field, want.text))
		if err != nil || !found {
			t.Fatalf("SeekExact(%q): found=%v err=%v", want.text, found, err)
		}
		pe, err := te.Postings(flags)
		if err != nil {
			t.Fatalf("Postings(%q): %v", want.text, err)
		}
		for di, wd := range want.docs {
			doc, err := pe.NextDoc()
			if err != nil {
				t.Fatalf("term %q doc[%d] NextDoc: %v", want.text, di, err)
			}
			if doc != wd.docID {
				t.Fatalf("term %q doc[%d]: got %d want %d", want.text, di, doc, wd.docID)
			}
			if hasFreqs {
				wantFreq := 1
				if hasPos {
					wantFreq = len(wd.positions)
				}
				freq, err := pe.Freq()
				if err != nil {
					t.Fatalf("term %q doc[%d] Freq: %v", want.text, di, err)
				}
				if freq != wantFreq {
					t.Fatalf("term %q doc[%d] freq: got %d want %d", want.text, di, freq, wantFreq)
				}
			}
			if hasPos {
				for pi, wpos := range wd.positions {
					pos, err := pe.NextPosition()
					if err != nil {
						t.Fatalf("term %q doc[%d] pos[%d] NextPosition: %v", want.text, di, pi, err)
					}
					if pos != wpos {
						t.Fatalf("term %q doc[%d] pos[%d]: got %d want %d", want.text, di, pi, pos, wpos)
					}
					if hasOffsets {
						so, _ := pe.StartOffset()
						eo, _ := pe.EndOffset()
						if so != wd.offsets[pi].start || eo != wd.offsets[pi].end {
							t.Fatalf("term %q doc[%d] pos[%d] offsets: got (%d,%d) want (%d,%d)",
								want.text, di, pi, so, eo, wd.offsets[pi].start, wd.offsets[pi].end)
						}
					}
					if storePayloads {
						pl, _ := pe.GetPayload()
						wantPl := wd.payloads[pi]
						if len(wantPl) == 0 {
							if len(pl) != 0 {
								t.Fatalf("term %q doc[%d] pos[%d] payload: got %v want empty", want.text, di, pi, pl)
							}
						} else if string(pl) != string(wantPl) {
							t.Fatalf("term %q doc[%d] pos[%d] payload: got %q want %q", want.text, di, pi, pl, wantPl)
						}
					}
				}
			}
		}
		last, err := pe.NextDoc()
		if err != nil {
			t.Fatalf("term %q final NextDoc: %v", want.text, err)
		}
		if last != index.NO_MORE_DOCS {
			t.Fatalf("term %q: expected NO_MORE_DOCS, got %d", want.text, last)
		}
	}
}

// ─── tests ─────────────────────────────────────────────────────────────────

// TestLucene103Postings_RoundTrip_AllIndexOptions covers every IndexOptions
// level (DOCS / DOCS_AND_FREQS / +POSITIONS / +OFFSETS) with multiple terms,
// proving the production reader recovers the exact logical input written by the
// test-only writer.
func TestLucene103Postings_RoundTrip_AllIndexOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts index.IndexOptions
	}{
		{"docs", index.IndexOptionsDocs},
		{"docs_freqs", index.IndexOptionsDocsAndFreqs},
		{"docs_freqs_pos", index.IndexOptionsDocsAndFreqsAndPositions},
		{"docs_freqs_pos_off", index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			hasPos := tc.opts >= index.IndexOptionsDocsAndFreqsAndPositions
			hasOff := tc.opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
			terms := &l103Terms{
				field:      "f",
				hasFreqs:   tc.opts >= index.IndexOptionsDocsAndFreqs,
				hasPos:     hasPos,
				hasOffsets: hasOff,
			}
			// Three terms, each with several docs / varying freqs.
			for ti := 0; ti < 3; ti++ {
				tm := &l103Term{text: fmt.Sprintf("term%d", ti)}
				for d := 0; d < 5; d++ {
					doc := l103Doc{docID: d * 7}
					if hasPos {
						freq := 1 + (ti+d)%3
						pos := 0
						for k := 0; k < freq; k++ {
							pos += 1 + k
							doc.positions = append(doc.positions, pos)
							if hasOff {
								doc.offsets = append(doc.offsets, l103Off{start: k * 6, end: k*6 + 5})
							}
						}
					}
					tm.docs = append(tm.docs, doc)
				}
				terms.terms = append(terms.terms, tm)
			}
			l103RoundTrip(t, tc.opts, false, terms)
		})
	}
}

// TestLucene103Postings_RoundTrip_SingletonDoc exercises the docFreq==1
// singleton optimisation (docID pulsed into the term dictionary).
func TestLucene103Postings_RoundTrip_SingletonDoc(t *testing.T) {
	opts := index.IndexOptionsDocsAndFreqsAndPositions
	terms := &l103Terms{field: "f", hasFreqs: true, hasPos: true}
	// Several singleton terms with distinct doc IDs (also exercises the
	// EncodeTerm singleton-delta path across consecutive ID-like terms).
	for ti, docID := range []int{0, 1, 2, 100, 5000} {
		tm := &l103Term{text: fmt.Sprintf("id%05d", ti)}
		tm.docs = append(tm.docs, l103Doc{docID: docID, positions: []int{0, 3, 9}})
		terms.terms = append(terms.terms, tm)
	}
	l103RoundTrip(t, opts, false, terms)
}

// TestLucene103Postings_RoundTrip_LargeBlocks writes a term with more than
// 2*BLOCK_SIZE docs, forcing multiple full 128-doc FOR-delta + PFOR blocks plus
// a VInt tail, and a term whose totalTermFreq exceeds BLOCK_SIZE (lastPosBlock).
func TestLucene103Postings_RoundTrip_LargeBlocks(t *testing.T) {
	opts := index.IndexOptionsDocsAndFreqsAndPositions
	terms := &l103Terms{field: "f", hasFreqs: true, hasPos: true}

	// 300 docs (> 2*128): two full FOR-delta blocks + a 44-doc VInt tail.
	big := &l103Term{text: "big"}
	for d := 0; d < 300; d++ {
		doc := l103Doc{docID: d * 3} // gaps of 3 keep deltas small (FOR path)
		// One position per doc keeps totalTermFreq == 300 (> BLOCK_SIZE),
		// exercising the lastPosBlockOffset term-state field.
		doc.positions = []int{d % 17}
		big.docs = append(big.docs, doc)
	}
	terms.terms = append(terms.terms, big)

	// A dense, consecutive 128-doc block (docRange == BLOCK_SIZE) exercises the
	// "all deltas == 1" byte-0 fast path of the writer.
	dense := &l103Term{text: "dense"}
	for d := 0; d < lucene103PostingsBlockSize; d++ {
		dense.docs = append(dense.docs, l103Doc{docID: 1000 + d, positions: []int{1}})
	}
	terms.terms = append(terms.terms, dense)

	l103RoundTrip(t, opts, false, terms)
}

// TestLucene103Postings_RoundTrip_Payloads exercises non-empty payloads,
// including a payload-length change within a doc and an empty payload.
func TestLucene103Postings_RoundTrip_Payloads(t *testing.T) {
	opts := index.IndexOptionsDocsAndFreqsAndPositions
	terms := &l103Terms{field: "f", hasFreqs: true, hasPos: true, hasPay: true}

	tm := &l103Term{text: "payterm"}
	tm.docs = append(tm.docs, l103Doc{
		docID:     0,
		positions: []int{0, 5, 9},
		payloads:  [][]byte{[]byte("p0"), []byte("payload-1"), nil},
	})
	tm.docs = append(tm.docs, l103Doc{
		docID:     4,
		positions: []int{2, 8},
		payloads:  [][]byte{[]byte("x"), []byte("yy")},
	})
	terms.terms = append(terms.terms, tm)

	// >128 positions in a single doc to exercise the packed payload block path
	// in .pay (pforUtil.encode of payload lengths + payload bytes).
	bigPay := &l103Term{text: "bigpay"}
	doc := l103Doc{docID: 0}
	for k := 0; k < 200; k++ {
		doc.positions = append(doc.positions, k*2)
		doc.payloads = append(doc.payloads, []byte(fmt.Sprintf("pl%d", k%5)))
	}
	bigPay.docs = append(bigPay.docs, doc)
	terms.terms = append(terms.terms, bigPay)

	l103RoundTrip(t, opts, true, terms)
}

// TestLucene103Postings_RoundTrip_Offsets exercises offsets (with positions),
// including offset-length changes and a >128-position doc (packed offset block).
func TestLucene103Postings_RoundTrip_Offsets(t *testing.T) {
	opts := index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	terms := &l103Terms{field: "f", hasFreqs: true, hasPos: true, hasOffsets: true}

	tm := &l103Term{text: "offterm"}
	doc := l103Doc{docID: 0}
	ch := 0
	for k := 0; k < 200; k++ {
		doc.positions = append(doc.positions, k)
		length := 3 + (k % 4) // varying offset lengths
		doc.offsets = append(doc.offsets, l103Off{start: ch, end: ch + length})
		ch += length + 1
	}
	tm.docs = append(tm.docs, doc)
	terms.terms = append(terms.terms, tm)

	l103RoundTrip(t, opts, false, terms)
}

// TestLucene103PostingsFormat_FieldsConsumerReadOnly asserts that the PRODUCTION
// format rejects writes, mirroring Apache Lucene's UnsupportedOperationException.
func TestLucene103PostingsFormat_FieldsConsumerReadOnly(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	ws := l103WriteState(t, dir, "_0", "f", index.IndexOptionsDocsAndFreqs, false)
	prod := NewLucene103PostingsFormat()
	c, err := prod.FieldsConsumer(ws)
	if err == nil {
		t.Fatalf("expected read-only error from production FieldsConsumer, got consumer=%v", c)
	}
	if err != errLucene103ReadOnly {
		t.Fatalf("expected errLucene103ReadOnly, got %v", err)
	}
}
