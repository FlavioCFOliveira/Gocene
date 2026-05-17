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
// buffers with each child level's length prefixed in front of its bytes so
// the reader can walk the levels with a single sequential pass.
//
// MultiLevelSkipListWriter is an abstract base in Java; in Go it is a
// concrete struct whose codec-specific behavior is supplied by an embedded
// callback hook (WriteSkipDataFunc). Concrete codec writers wire this hook
// to encode their per-level metadata (skip doc delta, freq pointer, etc.).
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
// belongs to and invokes the codec hook for each one.
func (w *MultiLevelSkipListWriter) BufferSkip(df int) error {
	if w.skipBuffer == nil {
		return fmt.Errorf("MultiLevelSkipListWriter: Init() not called")
	}
	if w.writeSkipData == nil {
		return fmt.Errorf("MultiLevelSkipListWriter: writeSkipData hook is nil")
	}
	// Lucene's bufferSkip: for each level, if df % (skipInterval * pow(skipMultiplier, level)) == 0,
	// call writeSkipData(level, skipBuffer[level]).
	step := w.skipInterval
	for level := 0; level < w.numberOfSkipLevels; level++ {
		if df%step != 0 {
			break
		}
		if err := w.writeSkipData(level, w.skipBuffer[level]); err != nil {
			return fmt.Errorf("MultiLevelSkipListWriter: writeSkipData(level=%d): %w", level, err)
		}
		step *= w.skipMultiplier
	}
	return nil
}

// WriteSkip flushes the buffered skip list to output and returns the file
// pointer where the skip data starts. The on-disk layout is top-down: for
// each level above 0, the writer emits a VLong giving the length of the
// child level's payload followed by the level's own payload. The bottommost
// level (level 0) is emitted without a length prefix because it is followed
// by the term's posting list end.
func (w *MultiLevelSkipListWriter) WriteSkip(output store.IndexOutput) (int64, error) {
	if w.skipBuffer == nil {
		return 0, fmt.Errorf("MultiLevelSkipListWriter: Init() not called")
	}

	skipPointer := output.GetFilePointer()
	if w.numberOfSkipLevels == 0 {
		return skipPointer, nil
	}

	// Walk levels top-down. Each level above 0 is prefixed with the length
	// of the level below it (the child) so the reader can jump straight to
	// its starting offset.
	for level := w.numberOfSkipLevels - 1; level > 0; level-- {
		childBytes := w.skipBuffer[level-1].GetBytes()
		if err := store.WriteVLong(output, int64(len(childBytes))); err != nil {
			return 0, fmt.Errorf("MultiLevelSkipListWriter: writeVLong(level=%d): %w", level, err)
		}
		levelBytes := w.skipBuffer[level].GetBytes()
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
