// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// MultiLevelSkipListWriter writes a multi-level skip list that lets readers
// jump forward over a posting list without decoding every entry. It is the
// Go port of org.apache.lucene.codecs.MultiLevelSkipListWriter from
// Apache Lucene 10.4.0.
//
// The skip list maintains numberOfSkipLevels levels. Each level samples its
// child level every skipMultiplier entries, so level 0 has one skip entry
// every skipInterval documents, level 1 every skipInterval*skipMultiplier
// documents, etc. The on-disk layout is a top-down dump of all level
// buffers with each level's own byte length prefixed in front of its bytes
// so the reader can walk the levels with a single sequential pass.
//
// MultiLevelSkipListWriter is an abstract base in Java; in Go it is a
// concrete struct whose codec-specific behavior is supplied by an embedded
// callback hook (WriteSkipDataFunc). Concrete codec writers wire this hook
// to encode their per-level metadata (skip doc delta, freq pointer, etc.).
// The base writer automatically appends a child pointer after the skip data
// on every level above 0, matching Lucene's on-disk format.
//
// Typical lifecycle (mirrors Lucene):
//  1. NewMultiLevelSkipListWriter(skipInterval, skipMultiplier, maxSkipLevels, df, writeFn)
//  2. Init() — called once before any documents are buffered
//  3. BufferSkip(df) — called for every skipInterval-th document
//  4. WriteSkip(output) — flushes the buffered skip data, returns the
//     IndexOutput file pointer where the skip list starts
type MultiLevelSkipListWriter struct {
	// skipInterval is the number of postings between skip entries at level 0.
	skipInterval int
	// skipMultiplier is the fan-out between adjacent levels.
	skipMultiplier int
	// windowLength is skipInterval*skipMultiplier; used to fast-path the
	// common single-level case in BufferSkip.
	windowLength int
	// maxSkipLevels caps the height of the skip list.
	maxSkipLevels int
	// numberOfSkipLevels is the actual computed height based on df.
	numberOfSkipLevels int
	// df is the document frequency of the term whose skip list is being built.
	df int

	// skipBuffer holds the per-level skip data accumulated by BufferSkip.
	// Each entry is a RAMOutputStream-equivalent; here a growing byte slice.
	skipBuffer []*store.ByteArrayDataOutput

	// writeSkipData is the codec-specific hook that writes the per-level skip
	// payload for the most recently buffered document. level is 0..numLevels-1
	// and skipBuffer is the buffer the hook should append to.
	writeSkipData WriteSkipDataFunc
}

// WriteSkipDataFunc is the codec hook invoked once per level for each
// BufferSkip call. The implementation appends its per-level skip payload to
// the provided buffer. Returning an error aborts the skip flush.
type WriteSkipDataFunc func(level int, skipBuffer *store.ByteArrayDataOutput) error

// NewMultiLevelSkipListWriter creates a writer that will eventually emit a
// skip list for a posting list of df documents. skipInterval and
// skipMultiplier govern the sampling step (typical defaults: 128 and 8).
// writeSkipData is the codec hook called once per skip-buffered document for
// each level the document belongs to.
func NewMultiLevelSkipListWriter(skipInterval, skipMultiplier, maxSkipLevels, df int, writeSkipData WriteSkipDataFunc) *MultiLevelSkipListWriter {
	w := &MultiLevelSkipListWriter{
		skipInterval:   skipInterval,
		skipMultiplier: skipMultiplier,
		windowLength:   skipInterval * skipMultiplier,
		maxSkipLevels:  maxSkipLevels,
		df:             df,
		writeSkipData:  writeSkipData,
	}
	w.numberOfSkipLevels = computeNumberOfSkipLevels(df, skipInterval, skipMultiplier, maxSkipLevels)
	return w
}

// computeNumberOfSkipLevels reproduces Lucene's
// MultiLevelSkipListWriter constructor formula:
//
//	numberOfSkipLevels = 1 + floor(log(df / skipInterval) / log(skipMultiplier))
//
// capped at maxSkipLevels.
func computeNumberOfSkipLevels(df, skipInterval, skipMultiplier, maxSkipLevels int) int {
	if df <= skipInterval {
		return 1
	}
	// Integer log: count how many times we can divide (df/skipInterval) by
	// skipMultiplier while staying >= 1.
	n := 1
	bucket := df / skipInterval
	for bucket >= skipMultiplier {
		bucket /= skipMultiplier
		n++
	}
	if n > maxSkipLevels {
		return maxSkipLevels
	}
	if n < 1 {
		return 1
	}
	return n
}

// Init resets the per-level buffers so the writer can be reused across terms.
// Must be called before BufferSkip on a freshly constructed or reused writer.
func (w *MultiLevelSkipListWriter) Init() {
	w.skipBuffer = make([]*store.ByteArrayDataOutput, w.numberOfSkipLevels)
	for i := range w.skipBuffer {
		w.skipBuffer[i] = store.NewByteArrayDataOutput(0)
	}
}

// NumberOfSkipLevels returns the actual height of the skip list. Exposed
// primarily for tests and codec writers that need to size their own metadata.
func (w *MultiLevelSkipListWriter) NumberOfSkipLevels() int {
	return w.numberOfSkipLevels
}

// BufferSkip appends a skip entry for the current document position. df is
// the running document frequency (1-based) of the document the caller just
// finished writing. The writer computes which skip levels the document
// belongs to and invokes the codec hook for each one, then appends the child
// pointer for every level above 0.
func (w *MultiLevelSkipListWriter) BufferSkip(df int) error {
	if w.skipBuffer == nil {
		return fmt.Errorf("MultiLevelSkipListWriter: Init() not called")
	}
	if w.writeSkipData == nil {
		return fmt.Errorf("MultiLevelSkipListWriter: writeSkipData hook is nil")
	}
	if df%w.skipInterval != 0 {
		return fmt.Errorf("MultiLevelSkipListWriter.BufferSkip: df=%d is not a multiple of skipInterval=%d", df, w.skipInterval)
	}

	// Determine how many levels this skip entry belongs to. This mirrors
	// Lucene's optimized computation: the common single-level case is a
	// single modulo check.
	numLevels := 1
	if df%w.windowLength == 0 {
		numLevels++
		remaining := df / w.windowLength
		for remaining%w.skipMultiplier == 0 && numLevels < w.numberOfSkipLevels {
			numLevels++
			remaining /= w.skipMultiplier
		}
	}

	var childPointer int64

	for level := 0; level < numLevels; level++ {
		if err := w.writeSkipData(level, w.skipBuffer[level]); err != nil {
			return fmt.Errorf("MultiLevelSkipListWriter: writeSkipData(level=%d): %w", level, err)
		}

		newChildPointer := int64(w.skipBuffer[level].Length())

		if level != 0 {
			// Append the child pointer that tells the reader where in the
			// child level's buffer this skip entry's corresponding child entry
			// starts.
			if err := writeChildPointer(childPointer, w.skipBuffer[level]); err != nil {
				return fmt.Errorf("MultiLevelSkipListWriter: writeChildPointer(level=%d): %w", level, err)
			}
		}

		childPointer = newChildPointer
	}
	return nil
}

// writeChildPointer writes a child pointer in the default VLong format.
// Codecs that need a different encoding (e.g. text formats) must use their
// own writer rather than the base MultiLevelSkipListWriter.
func writeChildPointer(childPointer int64, buf *store.ByteArrayDataOutput) error {
	return buf.WriteVLong(childPointer)
}

// WriteSkip flushes the buffered skip list to output and returns the file
// pointer where the skip data starts. The on-disk layout is top-down: for
// each level above 0, the writer emits a VLong giving the byte length of that
// level's payload followed by the level's own payload. The bottommost
// level (level 0) is emitted without a length prefix because the reader
// reaches it by skipping over the upper levels.
func (w *MultiLevelSkipListWriter) WriteSkip(output store.IndexOutput) (int64, error) {
	if w.skipBuffer == nil {
		return 0, fmt.Errorf("MultiLevelSkipListWriter: Init() not called")
	}

	skipPointer := output.GetFilePointer()
	if w.numberOfSkipLevels == 0 {
		return skipPointer, nil
	}

	// Walk levels top-down. Each level above 0 is prefixed with its own byte
	// length so the reader can seek past the upper levels in one pass.
	for level := w.numberOfSkipLevels - 1; level > 0; level-- {
		levelBytes := w.skipBuffer[level].GetBytes()
		if err := store.WriteVLong(output, int64(len(levelBytes))); err != nil {
			return 0, fmt.Errorf("MultiLevelSkipListWriter: writeLevelLength(level=%d): %w", level, err)
		}
		if len(levelBytes) > 0 {
			if err := output.WriteBytes(levelBytes); err != nil {
				return 0, fmt.Errorf("MultiLevelSkipListWriter: writeBytes(level=%d): %w", level, err)
			}
		}
	}
	level0 := w.skipBuffer[0].GetBytes()
	if len(level0) > 0 {
		if err := output.WriteBytes(level0); err != nil {
			return 0, fmt.Errorf("MultiLevelSkipListWriter: writeBytes(level=0): %w", err)
		}
	}

	return skipPointer, nil
}
