// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene90SkipWriter writes the multi-level skip list used by the Lucene 9.0
// posting format.
//
// Skip data is recorded once per block (blockSize documents). Each skip entry
// encodes:
//  1. docID delta from the previous skip point.
//  2. doc file-pointer delta.
//  3. (if positions are indexed) pos file-pointer delta + posBufferUpto.
//  4. (if offsets or payloads are indexed) payload byte upto + pay
//     file-pointer delta.
//  5. The competitive (freq, norm) pairs for impact-optimised WAND queries,
//     length-prefixed and serialised with WriteImpacts.
//
// Port of org.apache.lucene.backward_codecs.lucene90.Lucene90SkipWriter
// (Lucene 10.4.0).
type Lucene90SkipWriter struct {
	*codecs.MultiLevelSkipListWriter

	lastSkipDoc        []int
	lastSkipDocPointer []int64
	lastSkipPosPointer []int64
	lastSkipPayPointer []int64

	docOut store.IndexOutput
	posOut store.IndexOutput
	payOut store.IndexOutput

	curDoc             int
	curDocPointer      int64
	curPosPointer      int64
	curPayPointer      int64
	curPosBufferUpto   int
	curPayloadByteUpto int
	curCompFreqNorms   []*codecs.CompetitiveImpactAccumulator

	fieldHasPositions bool
	fieldHasOffsets   bool
	fieldHasPayloads  bool

	initialized bool

	// lastDocFP / lastPosFP / lastPayFP hold file pointers snapped at
	// ResetSkip; used to lazy-initialise the per-level arrays only when skip
	// data is actually needed for a term.
	lastDocFP int64
	lastPosFP int64
	lastPayFP int64

	// freqNormOut is a reusable scratch buffer for serialising the per-level
	// competitive impact pairs before they are length-prefixed and appended to
	// the level skip buffer.
	freqNormOut *store.ByteBuffersDataOutput
}

// NewLucene90SkipWriter creates a skip writer for a field with docCount
// documents. docOut, posOut, and payOut are the IndexOutput streams being
// written by the parent postings writer; posOut and payOut may be nil when the
// field does not index positions or payloads/offsets.
//
// Port of Lucene90SkipWriter(int, int, int, IndexOutput, IndexOutput, IndexOutput).
func NewLucene90SkipWriter(
	maxSkipLevels int,
	blockSize int,
	docCount int,
	docOut store.IndexOutput,
	posOut store.IndexOutput,
	payOut store.IndexOutput,
) *Lucene90SkipWriter {
	w := &Lucene90SkipWriter{
		docOut:      docOut,
		posOut:      posOut,
		payOut:      payOut,
		freqNormOut: store.NewByteBuffersDataOutput(),
	}

	w.curCompFreqNorms = make([]*codecs.CompetitiveImpactAccumulator, maxSkipLevels)
	for i := range w.curCompFreqNorms {
		w.curCompFreqNorms[i] = codecs.NewCompetitiveImpactAccumulator()
	}

	w.lastSkipDoc = make([]int, maxSkipLevels)
	w.lastSkipDocPointer = make([]int64, maxSkipLevels)
	if posOut != nil {
		w.lastSkipPosPointer = make([]int64, maxSkipLevels)
		if payOut != nil {
			w.lastSkipPayPointer = make([]int64, maxSkipLevels)
		}
	}

	w.MultiLevelSkipListWriter = codecs.NewMultiLevelSkipListWriter(
		blockSize, 8, maxSkipLevels, docCount, w.writeSkipData,
	)
	return w
}

// SetField configures which optional posting components are present for the
// current field. Must be called before any BufferSkip for the field.
//
// Port of Lucene90SkipWriter.setField(boolean, boolean, boolean).
func (w *Lucene90SkipWriter) SetField(
	fieldHasPositions bool,
	fieldHasOffsets bool,
	fieldHasPayloads bool,
) {
	w.fieldHasPositions = fieldHasPositions
	w.fieldHasOffsets = fieldHasOffsets
	w.fieldHasPayloads = fieldHasPayloads
}

// ResetSkip snaps the current file pointers and clears the competitive-impact
// accumulators. Actual per-level buffer reset is deferred to the first
// BufferSkip call (lazy init).
//
// Port of Lucene90SkipWriter.resetSkip().
func (w *Lucene90SkipWriter) ResetSkip() {
	w.lastDocFP = w.docOut.GetFilePointer()
	if w.fieldHasPositions {
		w.lastPosFP = w.posOut.GetFilePointer()
		if w.fieldHasOffsets || w.fieldHasPayloads {
			w.lastPayFP = w.payOut.GetFilePointer()
		}
	}
	if w.initialized {
		for _, acc := range w.curCompFreqNorms {
			acc.Clear()
		}
	}
	w.initialized = false
}

// initSkip performs the deferred per-level buffer reset on the first
// BufferSkip call for a term. This avoids the O(log docCount) init cost for
// rare terms that never accumulate enough documents to trigger a skip entry.
func (w *Lucene90SkipWriter) initSkip() {
	if !w.initialized {
		w.MultiLevelSkipListWriter.Init()
		for i := range w.lastSkipDoc {
			w.lastSkipDoc[i] = 0
		}
		for i := range w.lastSkipDocPointer {
			w.lastSkipDocPointer[i] = w.lastDocFP
		}
		if w.fieldHasPositions {
			for i := range w.lastSkipPosPointer {
				w.lastSkipPosPointer[i] = w.lastPosFP
			}
			if w.fieldHasOffsets || w.fieldHasPayloads {
				for i := range w.lastSkipPayPointer {
					w.lastSkipPayPointer[i] = w.lastPayFP
				}
			}
		}
		w.initialized = true
	}
}

// BufferSkip records a skip entry for the doc at position numDocs within the
// posting list. competitiveFreqNorms carries the competitive impacts for the
// current block. posFP, payFP, posBufferUpto, and payloadByteUpto are the
// corresponding stream positions; they are ignored when the field does not
// index positions/payloads.
//
// Port of Lucene90SkipWriter.bufferSkip(...).
func (w *Lucene90SkipWriter) BufferSkip(
	doc int,
	competitiveFreqNorms *codecs.CompetitiveImpactAccumulator,
	numDocs int,
	posFP int64,
	payFP int64,
	posBufferUpto int,
	payloadByteUpto int,
) error {
	w.initSkip()
	w.curDoc = doc
	w.curDocPointer = w.docOut.GetFilePointer()
	w.curPosPointer = posFP
	w.curPayPointer = payFP
	w.curPosBufferUpto = posBufferUpto
	w.curPayloadByteUpto = payloadByteUpto
	w.curCompFreqNorms[0].AddAll(competitiveFreqNorms)
	return w.MultiLevelSkipListWriter.BufferSkip(numDocs)
}

// writeSkipData is the codec hook invoked by MultiLevelSkipListWriter.BufferSkip
// for each level. It appends the per-level skip payload to skipBuffer.
//
// Port of Lucene90SkipWriter.writeSkipData(int, DataOutput).
func (w *Lucene90SkipWriter) writeSkipData(level int, skipBuffer *store.ByteArrayDataOutput) error {
	delta := int32(w.curDoc - w.lastSkipDoc[level])
	if err := skipBuffer.WriteVInt(delta); err != nil {
		return fmt.Errorf("lucene90 skip: level %d: writeVInt(docDelta): %w", level, err)
	}
	w.lastSkipDoc[level] = w.curDoc

	docPtrDelta := w.curDocPointer - w.lastSkipDocPointer[level]
	if err := skipBuffer.WriteVLong(docPtrDelta); err != nil {
		return fmt.Errorf("lucene90 skip: level %d: writeVLong(docPtrDelta): %w", level, err)
	}
	w.lastSkipDocPointer[level] = w.curDocPointer

	if w.fieldHasPositions {
		posPtrDelta := w.curPosPointer - w.lastSkipPosPointer[level]
		if err := skipBuffer.WriteVLong(posPtrDelta); err != nil {
			return fmt.Errorf("lucene90 skip: level %d: writeVLong(posPtrDelta): %w", level, err)
		}
		w.lastSkipPosPointer[level] = w.curPosPointer

		if err := skipBuffer.WriteVInt(int32(w.curPosBufferUpto)); err != nil {
			return fmt.Errorf("lucene90 skip: level %d: writeVInt(posBufferUpto): %w", level, err)
		}

		if w.fieldHasPayloads {
			if err := skipBuffer.WriteVInt(int32(w.curPayloadByteUpto)); err != nil {
				return fmt.Errorf("lucene90 skip: level %d: writeVInt(payloadByteUpto): %w", level, err)
			}
		}

		if w.fieldHasOffsets || w.fieldHasPayloads {
			payPtrDelta := w.curPayPointer - w.lastSkipPayPointer[level]
			if err := skipBuffer.WriteVLong(payPtrDelta); err != nil {
				return fmt.Errorf("lucene90 skip: level %d: writeVLong(payPtrDelta): %w", level, err)
			}
			w.lastSkipPayPointer[level] = w.curPayPointer
		}
	}

	// Propagate competitive impacts up to the next level.
	acc := w.curCompFreqNorms[level]
	numLevels := w.MultiLevelSkipListWriter.NumberOfSkipLevels()
	if level+1 < numLevels {
		w.curCompFreqNorms[level+1].AddAll(acc)
	}

	// Serialise competitive impacts into the scratch buffer, then
	// length-prefix and copy to skipBuffer.
	w.freqNormOut.Reset()
	if err := WriteImpacts(acc, w.freqNormOut); err != nil {
		return fmt.Errorf("lucene90 skip: level %d: writeImpacts: %w", level, err)
	}
	sz := int32(w.freqNormOut.Size())
	if err := skipBuffer.WriteVInt(sz); err != nil {
		return fmt.Errorf("lucene90 skip: level %d: writeVInt(impactLen): %w", level, err)
	}
	impactBytes := w.freqNormOut.ToArrayCopy()
	if err := skipBuffer.WriteBytes(impactBytes); err != nil {
		return fmt.Errorf("lucene90 skip: level %d: writeBytes(impacts): %w", level, err)
	}
	acc.Clear()

	return nil
}

// WriteImpacts serialises acc's competitive (freq, norm) pairs to out using
// the delta-encoding scheme defined by Lucene 9.0:
//
//   - freqDelta = impact.freq − previous.freq − 1
//   - normDelta = impact.norm − previous.norm − 1
//   - If normDelta == 0: writeVInt(freqDelta << 1)
//   - Otherwise:         writeVInt((freqDelta << 1) | 1), writeZLong(normDelta)
//
// Port of Lucene90SkipWriter.writeImpacts(CompetitiveImpactAccumulator, DataOutput).
func WriteImpacts(acc *codecs.CompetitiveImpactAccumulator, out *store.ByteBuffersDataOutput) error {
	impacts := acc.GetCompetitiveFreqNormPairs()
	var prevFreq int
	var prevNorm int64
	first := true
	for _, impact := range impacts {
		var freqDelta int
		var normDelta int64
		if first {
			freqDelta = impact.Freq - 1
			normDelta = impact.Norm - 1
			first = false
		} else {
			freqDelta = impact.Freq - prevFreq - 1
			normDelta = impact.Norm - prevNorm - 1
		}
		if normDelta == 0 {
			if err := out.WriteVInt(int32(freqDelta << 1)); err != nil {
				return fmt.Errorf("writeImpacts: writeVInt: %w", err)
			}
		} else {
			if err := out.WriteVInt(int32((freqDelta << 1) | 1)); err != nil {
				return fmt.Errorf("writeImpacts: writeVInt: %w", err)
			}
			if err := out.WriteZLong(normDelta); err != nil {
				return fmt.Errorf("writeImpacts: writeZLong: %w", err)
			}
		}
		prevFreq = impact.Freq
		prevNorm = impact.Norm
	}
	return nil
}
