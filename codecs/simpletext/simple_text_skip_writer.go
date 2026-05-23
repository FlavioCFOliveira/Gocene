// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SimpleTextSkipWriter writes multi-level skip lists in plain-text format.
//
// This is a standalone implementation (not delegating to
// codecs.MultiLevelSkipListWriter) because the text format requires
// overriding writeLevelLength and writeChildPointer, which the Go port of
// MultiLevelSkipListWriter does not expose as hooks.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextSkipWriter
// (Lucene 10.4.0).
type SimpleTextSkipWriter struct {
	// skipInterval is the number of postings between skip entries at level 0.
	// Mirrors BLOCK_SIZE from Java.
	skipInterval int
	// skipMultiplier is the fan-out between adjacent levels.
	skipMul int
	// maxLevels caps the height of the skip list.
	maxLevels int

	// maxDoc is the maximum document ID in the segment; used to compute
	// numberOfSkipLevels per term.
	maxDoc int

	// numberOfSkipLevels is recomputed for each term in resetSkip.
	numberOfSkipLevels int

	// skipBuffer holds the per-level text payloads.
	skipBuffer []*store.ByteArrayDataOutput

	// wroteHeaderPerLevel tracks whether the "level N" line has been emitted
	// for each level in the current term.
	wroteHeaderPerLevel []bool

	// curDoc is the last document passed to bufferSkip.
	curDoc int

	// curDocFilePointer is the doc-file offset passed to bufferSkip.
	curDocFilePointer int64

	// curCompetitiveFreqNorms accumulates impacts per level.
	curCompetitiveFreqNorms []*codecs.CompetitiveImpactAccumulator

	// scratch is reused for ASCII conversions.
	scratch *util.BytesRefBuilder
}

// NewSimpleTextSkipWriter creates a skip writer for a segment whose maximum
// doc count is maxDoc. The writer must be reset with resetSkip before each
// new term.
//
// Port of SimpleTextSkipWriter(SegmentWriteState).
func NewSimpleTextSkipWriter(maxDoc int) *SimpleTextSkipWriter {
	w := &SimpleTextSkipWriter{
		skipInterval:            skipBlockSize,
		skipMul:                 skipMultiplier,
		maxLevels:               maxSkipLevels,
		maxDoc:                  maxDoc,
		skipBuffer:              make([]*store.ByteArrayDataOutput, maxSkipLevels),
		wroteHeaderPerLevel:     make([]bool, maxSkipLevels),
		curCompetitiveFreqNorms: make([]*codecs.CompetitiveImpactAccumulator, maxSkipLevels),
		scratch:                 util.NewBytesRefBuilder(),
	}
	for i := 0; i < maxSkipLevels; i++ {
		w.skipBuffer[i] = store.NewByteArrayDataOutput(0)
		w.curCompetitiveFreqNorms[i] = codecs.NewCompetitiveImpactAccumulator()
	}
	w.resetSkip()
	return w
}

// resetSkip resets all per-term state. Must be called before writing each term.
//
// Port of SimpleTextSkipWriter.resetSkip().
func (w *SimpleTextSkipWriter) resetSkip() {
	for i := 0; i < w.maxLevels; i++ {
		w.wroteHeaderPerLevel[i] = false
		w.skipBuffer[i] = store.NewByteArrayDataOutput(0)
		w.curCompetitiveFreqNorms[i].Clear()
	}
	w.curDoc = -1
	w.curDocFilePointer = -1
	// Compute numberOfSkipLevels based on maxDoc, mirroring Java base ctor.
	w.numberOfSkipLevels = computeSkipLevels(w.maxDoc, w.skipInterval, w.skipMul, w.maxLevels)
}

// computeSkipLevels mirrors MultiLevelSkipListWriter's constructor formula.
func computeSkipLevels(df, skipInterval, skipMul, maxLevels int) int {
	if df <= skipInterval {
		return 1
	}
	n := 1 + log10Int(df/skipInterval, skipMul)
	if n > maxLevels {
		return maxLevels
	}
	if n < 1 {
		return 1
	}
	return n
}

// log10Int computes floor(log_base(v)).
func log10Int(v, base int) int {
	n := 0
	for v >= base {
		v /= base
		n++
	}
	return n
}

// bufferSkip buffers a skip entry for the given document.
//
// Port of SimpleTextSkipWriter.bufferSkip(int, long, int, CompetitiveImpactAccumulator).
func (w *SimpleTextSkipWriter) bufferSkip(
	doc int,
	docFilePointer int64,
	numDocs int,
	accumulator *codecs.CompetitiveImpactAccumulator,
) error {
	w.curDoc = doc
	w.curDocFilePointer = docFilePointer
	w.curCompetitiveFreqNorms[0].AddAll(accumulator)

	// Determine how many levels this skip entry propagates to.
	numLevels := 1
	windowLen := w.skipInterval * w.skipMul
	if numDocs%windowLen == 0 {
		numLevels++
		df := numDocs / windowLen
		for df%w.skipMul == 0 && numLevels < w.numberOfSkipLevels {
			numLevels++
			df /= w.skipMul
		}
	}

	var childPointer int64

	for level := 0; level < numLevels; level++ {
		if err := w.writeSkipData(level, w.skipBuffer[level]); err != nil {
			return fmt.Errorf("SimpleTextSkipWriter.bufferSkip: level %d: %w", level, err)
		}

		newChildPointer := int64(w.skipBuffer[level].Length())

		if level != 0 {
			// Append child pointer to the level's buffer (text format).
			if err := w.writeChildPointer(childPointer, w.skipBuffer[level]); err != nil {
				return fmt.Errorf("SimpleTextSkipWriter.bufferSkip: childPointer level %d: %w", level, err)
			}
		}

		// Propagate impacts upward.
		if level+1 < w.numberOfSkipLevels {
			w.curCompetitiveFreqNorms[level+1].AddAll(w.curCompetitiveFreqNorms[level])
		}
		w.curCompetitiveFreqNorms[level].Clear()

		childPointer = newChildPointer
	}
	return nil
}

// writeSkipData encodes one plain-text skip entry for the given level into buf.
//
// Port of SimpleTextSkipWriter.writeSkipData(int, DataOutput).
func (w *SimpleTextSkipWriter) writeSkipData(level int, buf *store.ByteArrayDataOutput) error {
	if !w.wroteHeaderPerLevel[level] {
		if err := stWrite(buf, stLevel, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(buf, strconv.Itoa(level), w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(buf); err != nil {
			return err
		}
		w.wroteHeaderPerLevel[level] = true
	}

	if err := stWrite(buf, stSkipDoc, w.scratch); err != nil {
		return err
	}
	if err := stWriteStr(buf, strconv.Itoa(w.curDoc), w.scratch); err != nil {
		return err
	}
	if err := stWriteNewline(buf); err != nil {
		return err
	}

	if err := stWrite(buf, stSkipDocFP, w.scratch); err != nil {
		return err
	}
	if err := stWriteStr(buf, strconv.FormatInt(w.curDocFilePointer, 10), w.scratch); err != nil {
		return err
	}
	if err := stWriteNewline(buf); err != nil {
		return err
	}

	acc := w.curCompetitiveFreqNorms[level]
	impacts := acc.GetCompetitiveFreqNormPairs()

	if err := stWrite(buf, stImpacts, w.scratch); err != nil {
		return err
	}
	if err := stWriteNewline(buf); err != nil {
		return err
	}
	for _, imp := range impacts {
		if err := stWrite(buf, stImpact, w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(buf); err != nil {
			return err
		}
		if err := stWrite(buf, stFreq, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(buf, strconv.Itoa(imp.Freq), w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(buf); err != nil {
			return err
		}
		if err := stWrite(buf, stNorm, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(buf, strconv.FormatInt(imp.Norm, 10), w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(buf); err != nil {
			return err
		}
	}
	if err := stWrite(buf, stImpactsEnd, w.scratch); err != nil {
		return err
	}
	return stWriteNewline(buf)
}

// writeChildPointer appends a text-encoded child pointer to buf.
//
// Port of SimpleTextSkipWriter.writeChildPointer(long, DataOutput).
func (w *SimpleTextSkipWriter) writeChildPointer(childPointer int64, buf *store.ByteArrayDataOutput) error {
	if err := stWrite(buf, stChildPtr, w.scratch); err != nil {
		return err
	}
	if err := stWriteStr(buf, strconv.FormatInt(childPointer, 10), w.scratch); err != nil {
		return err
	}
	return stWriteNewline(buf)
}

// writeLevelLength writes a text-encoded level-length record to output.
//
// Port of SimpleTextSkipWriter.writeLevelLength(long, IndexOutput).
func (w *SimpleTextSkipWriter) writeLevelLength(levelLength int64, output store.IndexOutput) error {
	if err := stWrite(output, stLevelLength, w.scratch); err != nil {
		return err
	}
	if err := stWriteStr(output, strconv.FormatInt(levelLength, 10), w.scratch); err != nil {
		return err
	}
	return stWriteNewline(output)
}

// WriteSkip flushes the buffered skip data to output and returns the file
// pointer where the skip list starts.
//
// Port of SimpleTextSkipWriter.writeSkip(IndexOutput).
func (w *SimpleTextSkipWriter) WriteSkip(output store.IndexOutput) (int64, error) {
	skipOffset := output.GetFilePointer()

	// Write the "skipList " header line.
	if err := stWrite(output, stSkipList, w.scratch); err != nil {
		return 0, err
	}
	if err := stWriteNewline(output); err != nil {
		return 0, err
	}

	// Emit levels top-down (level numberOfSkipLevels-1 first, level 0 last).
	// For each level above 0, emit a text-encoded level length then that
	// level's bytes. Level 0 is emitted without a length prefix.
	for level := w.numberOfSkipLevels - 1; level > 0; level-- {
		levelBytes := w.skipBuffer[level].GetBytes()
		length := int64(len(levelBytes))
		if length > 0 {
			if err := w.writeLevelLength(length, output); err != nil {
				return 0, fmt.Errorf("SimpleTextSkipWriter.WriteSkip: level %d length: %w", level, err)
			}
			if err := output.WriteBytes(levelBytes); err != nil {
				return 0, fmt.Errorf("SimpleTextSkipWriter.WriteSkip: level %d bytes: %w", level, err)
			}
		}
	}
	level0 := w.skipBuffer[0].GetBytes()
	if len(level0) > 0 {
		if err := output.WriteBytes(level0); err != nil {
			return 0, fmt.Errorf("SimpleTextSkipWriter.WriteSkip: level 0 bytes: %w", err)
		}
	}

	return skipOffset, nil
}
