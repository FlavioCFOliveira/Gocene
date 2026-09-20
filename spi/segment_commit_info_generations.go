// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// This file completes the port of org.apache.lucene.index.SegmentCommitInfo
// from Apache Lucene 10.5.0 with the members the write path needs: the
// next-write generation counters that IndexWriter advances before it writes a
// new live-docs, field-infos or doc-values file, the buffered-deletes
// generation, the cached commit size, and the field-updates predicate.

// nextGen returns the generation that follows gen: 1 when gen does not exist
// (-1), gen+1 otherwise. Mirrors the `gen == -1 ? 1 : gen + 1` expression the
// Java constructor uses for each of the three next-write counters.
func nextGen(gen int64) int64 {
	if gen == -1 {
		return 1
	}
	return gen + 1
}

// GetNextWriteDelGen returns the generation the next live-docs file written
// for this segment will carry. Mirrors getNextWriteDelGen.
func (sci *SegmentCommitInfo) GetNextWriteDelGen() int64 {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.nextWriteDelGen
}

// SetNextWriteDelGen sets the generation the next live-docs file written for
// this segment will carry. Mirrors setNextWriteDelGen.
func (sci *SegmentCommitInfo) SetNextWriteDelGen(v int64) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.nextWriteDelGen = v
}

// AdvanceNextWriteDelGen bumps the next live-docs generation without
// consuming it, so a subsequent write skips the generation that was reserved
// but never written. Mirrors advanceNextWriteDelGen.
func (sci *SegmentCommitInfo) AdvanceNextWriteDelGen() {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.nextWriteDelGen++
}

// NextFieldInfosGen returns the generation the next field-infos file written
// for this segment will carry. Mirrors getNextFieldInfosGen.
func (sci *SegmentCommitInfo) NextFieldInfosGen() int64 {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.nextWriteFieldInfosGen
}

// SetNextWriteFieldInfosGen sets the generation the next field-infos file
// written for this segment will carry. Mirrors setNextWriteFieldInfosGen.
func (sci *SegmentCommitInfo) SetNextWriteFieldInfosGen(v int64) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.nextWriteFieldInfosGen = v
}

// AdvanceNextWriteFieldInfosGen bumps the next field-infos generation without
// consuming it. Mirrors advanceNextWriteFieldInfosGen.
func (sci *SegmentCommitInfo) AdvanceNextWriteFieldInfosGen() {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.nextWriteFieldInfosGen++
}

// NextDocValuesGen returns the generation the next doc-values file written for
// this segment will carry. Mirrors getNextWriteDocValuesGen.
func (sci *SegmentCommitInfo) NextDocValuesGen() int64 {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.nextWriteDocValuesGen
}

// SetNextWriteDocValuesGen sets the generation the next doc-values file
// written for this segment will carry. Mirrors setNextWriteDocValuesGen.
func (sci *SegmentCommitInfo) SetNextWriteDocValuesGen(v int64) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.nextWriteDocValuesGen = v
}

// AdvanceNextWriteDocValuesGen bumps the next doc-values generation without
// consuming it. Mirrors advanceNextWriteDocValuesGen.
func (sci *SegmentCommitInfo) AdvanceNextWriteDocValuesGen() {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.nextWriteDocValuesGen++
}

// GetBufferedDeletesGen returns the sequence number of the buffered-deletes
// packet applied to this segment, or -1 when none has been. Mirrors
// getBufferedDeletesGen.
func (sci *SegmentCommitInfo) GetBufferedDeletesGen() int64 {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.bufferedDeletesGen
}

// SetBufferedDeletesGen records the sequence number of the buffered-deletes
// packet applied to this segment. Only the first call takes effect, matching
// the `if (bufferedDeletesGen == -1)` guard in Java.
func (sci *SegmentCommitInfo) SetBufferedDeletesGen(v int64) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	if sci.bufferedDeletesGen == -1 {
		sci.bufferedDeletesGen = v
		// Java also clears the cached size here, because changing the
		// buffered-deletes generation can change the file set.
		sci.sizeInBytes = -1
	}
}

// HasFieldUpdates reports whether this segment carries doc-values field
// updates, i.e. whether a doc-values generation has been written. Mirrors
// hasFieldUpdates.
func (sci *SegmentCommitInfo) HasFieldUpdates() bool {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.docValuesGen != -1
}

// SizeInBytes returns the total size in bytes of every file this commit
// references, computed once and cached. Mirrors sizeInBytes().
func (sci *SegmentCommitInfo) SizeInBytes() (int64, error) {
	sci.mu.RLock()
	cached := sci.sizeInBytes
	sci.mu.RUnlock()
	if cached != -1 {
		return cached, nil
	}

	// GetFiles takes the read lock itself, so it must be called unlocked.
	files := sci.GetFiles()
	dir := sci.Info.Directory()

	var sum int64
	for _, name := range files {
		length, err := dir.FileLength(name)
		if err != nil {
			return 0, err
		}
		sum += length
	}

	sci.mu.Lock()
	sci.sizeInBytes = sum
	sci.mu.Unlock()
	return sum, nil
}
