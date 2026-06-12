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

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

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

// lucene103RWPostingsFormat is the read-write impersonation of
// Lucene103PostingsFormat used only by tests, mirroring the test-only Java
// Lucene103RWPostingsFormat. Its FieldsConsumer wires the test-only writer
// through the production Lucene103BlockTreeTermsWriter; FieldsProducer is
// inherited from the production read-only format.
type lucene103RWPostingsFormat struct {
	*Lucene103PostingsFormat
}

// NewLucene103RWPostingsFormat creates the read-write impersonation of
// Lucene103PostingsFormat used only by tests. The returned format delegates
// FieldsProducer to the production read-only implementation and overrides
// FieldsConsumer to wire the test-only Lucene103 postings writer.
func NewLucene103RWPostingsFormat() *lucene103RWPostingsFormat {
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
