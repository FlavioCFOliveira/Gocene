// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// MultiLevelSkipListReader walks a multi-level skip list produced by
// MultiLevelSkipListWriter to jump forward over a posting list without
// decoding every entry. It is the Go port of
// org.apache.lucene.codecs.MultiLevelSkipListReader from Apache Lucene 10.4.0.
//
// The reader maintains a per-level cursor: skipDoc[level] is the document
// reached by the cursor on that level and childPointer[level] is the file
// pointer the cursor would jump to in the level below. SkipTo walks up to
// the highest level that has a skip for the target, advances that level until
// it reaches or passes the target, then walks down seeking child levels at
// the last accepted skip entry.
//
// MultiLevelSkipListReader is an abstract base in Java; in Go it is a
// concrete struct whose codec-specific decoding is supplied by an embedded
// callback hook (ReadSkipDataFunc). Concrete codec readers wire this hook
// to decode their per-level skip payload and return the doc delta gained.
type MultiLevelSkipListReader struct {
	// skipInterval is the number of postings between skip entries at level 0.
	skipInterval int
	// skipIntervalPerLevel caches skipInterval * skipMultiplier^level.
	skipIntervalPerLevel []int
	// skipMultiplier is the fan-out between adjacent levels.
	skipMultiplier int
	// maxNumberOfSkipLevels caps the height of the skip list.
	maxNumberOfSkipLevels int
	// numberOfSkipLevels is the actual height as computed from df.
	numberOfSkipLevels int

	// docCount is the document frequency of the term whose skip list we walk.
	docCount int

	// skipStream is the underlying index input for each level. Level 0 reuses
	// the master stream supplied at construction; higher levels point into
	// independent IndexInputs the reader created via skipStream[0].Clone().
	skipStream []store.IndexInput

	// skipPointer[level] is the file pointer where the level's payload starts.
	skipPointer []int64
	// childPointer[level] is the file pointer in the level below that the
	// current skip block would jump to. Level 0 has no children.
	childPointer []int64

	// skipDoc[level] is the document id reached by the cursor on that level.
	skipDoc []int

	// numSkipped[level] is the number of postings skipped up to the current
	// cursor on that level. It is updated when the cursor advances or when a
	// child cursor is reset to a parent position.
	numSkipped []int

	// lastDoc is the document id of the last accepted skip entry (the one
	// whose docId <= target). It is returned by GetDoc.
	lastDoc int

	// lastChildPointer is the child pointer of the last accepted skip entry.
	lastChildPointer int64

	// haveSkipped indicates whether SkipTo has advanced the cursor at all.
	haveSkipped bool

	// readSkipData decodes the codec-specific skip payload for the given
	// level. Returns the doc delta (new_doc - skipDoc[level]) and any error.
	readSkipData ReadSkipDataFunc

	// onSetLastSkipData, when non-nil, is called immediately before the reader
	// commits a new skip entry (i.e. when the current entry at level is about to
	// be advanced). This mirrors the Java MultiLevelSkipListReader.setLastSkipData
	// override hook. Concrete readers use it to snapshot per-level state as the
	// "last accepted" skip data.
	onSetLastSkipData func(level int)

	// onSeekChild, when non-nil, is called when the reader resets a child level
	// to the position indicated by the last accepted skip entry. This mirrors the
	// Java MultiLevelSkipListReader.seekChild override hook.
	onSeekChild func(level int)

	// readLevelLength, when non-nil, overrides the default VLong-based level-
	// length reader used in loadSkipLevels. Set via SetReadLevelLength.
	readLevelLength ReadLevelLengthFunc

	// readChildPointer, when non-nil, overrides the default VLong-based child-
	// pointer reader used after each non-leaf skip entry. Set via SetReadChildPointer.
	readChildPointer ReadChildPointerFunc
}

// ReadSkipDataFunc is the codec hook invoked once per level for each
// downward step. The implementation reads its per-level skip payload from
// skipStream and returns the document delta gained by this skip.
type ReadSkipDataFunc func(level int, skipStream store.IndexInput) (int, error)

// ReadLevelLengthFunc is the codec hook that reads one level-length record
// from the skip stream. The default implementation reads a VLong. Override
// to support text-format codecs (e.g. SimpleText).
type ReadLevelLengthFunc func(skipStream store.IndexInput) (int64, error)

// ReadChildPointerFunc is the codec hook that reads one child-pointer record
// from the skip stream. The default implementation reads a VLong. Override
// to support text-format codecs (e.g. SimpleText).
type ReadChildPointerFunc func(skipStream store.IndexInput) (int64, error)

// defaultReadChildPointer is the VLong reader used when the codec does not
// supply its own hook.
func defaultReadChildPointer(skipStream store.IndexInput) (int64, error) {
	return store.ReadVLong(skipStream)
}

// defaultReadLevelLength is the VLong reader used when the codec does not
// supply its own hook.
func defaultReadLevelLength(skipStream store.IndexInput) (int64, error) {
	return store.ReadVLong(skipStream)
}

// SetOnSetLastSkipData registers an optional callback invoked immediately
// before the reader advances past the current skip entry at the given level.
// This mirrors the Java MultiLevelSkipListReader.setLastSkipData override.
func (r *MultiLevelSkipListReader) SetOnSetLastSkipData(fn func(level int)) {
	r.onSetLastSkipData = fn
}

// SetOnSeekChild registers an optional callback invoked when the reader
// resets a child level to the position indicated by the last accepted skip
// entry. This mirrors the Java MultiLevelSkipListReader.seekChild override.
func (r *MultiLevelSkipListReader) SetOnSeekChild(fn func(level int)) {
	r.onSeekChild = fn
}

// SetReadLevelLength overrides the default VLong reader for level-length
// records. Must be set before SkipTo is first called.
func (r *MultiLevelSkipListReader) SetReadLevelLength(fn ReadLevelLengthFunc) {
	r.readLevelLength = fn
}

// SetReadChildPointer overrides the default VLong reader for child-pointer
// records. Must be set before SkipTo is first called.
func (r *MultiLevelSkipListReader) SetReadChildPointer(fn ReadChildPointerFunc) {
	r.readChildPointer = fn
}

// NewMultiLevelSkipListReader constructs a reader that will walk a skip list
// stored in skipStream. skipInterval and skipMultiplier must match the
// writer-side values; maxSkipLevels caps the height. readSkipData is the
// codec hook used to decode per-level skip payloads.
func NewMultiLevelSkipListReader(skipStream store.IndexInput, maxSkipLevels, skipInterval, skipMultiplier int, readSkipData ReadSkipDataFunc) *MultiLevelSkipListReader {
	r := &MultiLevelSkipListReader{
		skipInterval:          skipInterval,
		skipMultiplier:        skipMultiplier,
		maxNumberOfSkipLevels: maxSkipLevels,
		readSkipData:          readSkipData,
		skipStream:            make([]store.IndexInput, maxSkipLevels),
		skipPointer:           make([]int64, maxSkipLevels),
		childPointer:          make([]int64, maxSkipLevels),
		skipDoc:               make([]int, maxSkipLevels),
		numSkipped:            make([]int, maxSkipLevels),
		skipIntervalPerLevel:  make([]int, maxSkipLevels),
	}
	r.skipStream[0] = skipStream
	r.skipIntervalPerLevel[0] = skipInterval
	for i := 1; i < maxSkipLevels; i++ {
		r.skipIntervalPerLevel[i] = r.skipIntervalPerLevel[i-1] * skipMultiplier
	}
	return r
}

// Init prepares the reader to walk a fresh posting list. skipPointer is the
// file pointer where the skip list starts (the return value of
// MultiLevelSkipListWriter.WriteSkip on the same term). df is the document
// frequency, used to compute the actual number of skip levels for this term.
func (r *MultiLevelSkipListReader) Init(skipPointer int64, df int) error {
	r.docCount = df
	r.numberOfSkipLevels = computeNumberOfSkipLevels(df, r.skipInterval, r.skipMultiplier, r.maxNumberOfSkipLevels)
	for i := range r.skipDoc {
		r.skipDoc[i] = 0
	}
	for i := range r.numSkipped {
		r.numSkipped[i] = 0
	}
	for i := range r.childPointer {
		r.childPointer[i] = 0
	}
	r.lastDoc = 0
	r.lastChildPointer = 0
	r.haveSkipped = false

	if r.numberOfSkipLevels == 0 {
		return nil
	}
	r.skipPointer[0] = skipPointer
	return nil
}

// loadSkipLevels reads the top-down level headers (the per-level lengths)
// and clones the master skipStream so each level can be advanced
// independently. Returns the first error encountered.
func (r *MultiLevelSkipListReader) loadSkipLevels() error {
	if r.numberOfSkipLevels == 0 {
		return nil
	}
	// Position the master stream at the top level.
	if err := r.skipStream[0].SetPosition(r.skipPointer[0]); err != nil {
		return fmt.Errorf("MultiLevelSkipListReader: seek top level: %w", err)
	}

	// Walk top-down. For each level above 0, read the length of that level's
	// payload and reserve a clone positioned at the level's start. Then
	// advance the master stream past the level's payload.
	readLevelLength := r.readLevelLength
	if readLevelLength == nil {
		readLevelLength = defaultReadLevelLength
	}

	for level := r.numberOfSkipLevels - 1; level > 0; level-- {
		// Read length of the current level.
		length, err := readLevelLength(r.skipStream[0])
		if err != nil {
			return fmt.Errorf("MultiLevelSkipListReader: readLevelLength(level=%d): %w", level, err)
		}

		// Current position is the start of this level's payload.
		levelStart := r.skipStream[0].GetFilePointer()
		r.skipPointer[level] = levelStart

		// Clone the stream for independent advancement on this level.
		clone := r.skipStream[0].Clone()
		if err := clone.SetPosition(levelStart); err != nil {
			return fmt.Errorf("MultiLevelSkipListReader: clone seek(level=%d): %w", level, err)
		}
		r.skipStream[level] = clone

		// Move base stream beyond this level's payload.
		if err := r.skipStream[0].SetPosition(levelStart + length); err != nil {
			return fmt.Errorf("MultiLevelSkipListReader: skip past level(level=%d): %w", level, err)
		}
	}

	// Level 0 starts wherever the master stream is now.
	r.skipPointer[0] = r.skipStream[0].GetFilePointer()
	return nil
}

// SkipTo advances the cursor to the largest skip entry whose document number
// is less than or equal to target. Returns the number of postings skipped
// (not docs) — matches Java's int return value.
func (r *MultiLevelSkipListReader) SkipTo(target int) (int, error) {
	if !r.haveSkipped {
		if err := r.loadSkipLevels(); err != nil {
			return 0, err
		}
		r.haveSkipped = true
	}

	if r.numberOfSkipLevels == 0 {
		return 0, nil
	}

	// Walk up the levels until we find the highest level that has a skip for
	// this target. A level "has a skip" when its current skipDoc is still
	// < target; higher levels may already be positioned beyond target from a
	// previous SkipTo call.
	level := 0
	for level < r.numberOfSkipLevels-1 && target > r.skipDoc[level+1] {
		level++
	}

	for level >= 0 {
		if target > r.skipDoc[level] {
			more, err := r.loadNextSkip(level)
			if err != nil {
				return r.numSkipped[0] - r.skipIntervalPerLevel[0] - 1, err
			}
			if !more {
				// This level is exhausted; continue to descend (or exit if level 0).
				continue
			}
		} else {
			// No more skips needed on this level; descend to the child level,
			// seeking it to the last accepted skip entry if necessary.
			if level > 0 && r.lastChildPointer > r.skipStream[level-1].GetFilePointer() {
				if err := r.seekChild(level - 1); err != nil {
					return r.numSkipped[0] - r.skipIntervalPerLevel[0] - 1, err
				}
			}
			level--
		}
	}

	return r.numSkipped[0] - r.skipIntervalPerLevel[0] - 1, nil
}

// loadNextSkip advances the cursor on the given level by one skip entry.
// Returns false if the level is exhausted (and shrinks numberOfSkipLevels).
func (r *MultiLevelSkipListReader) loadNextSkip(level int) (bool, error) {
	// Snapshot the current entry as the "last accepted" before advancing.
	r.setLastSkipData(level)

	r.numSkipped[level] += r.skipIntervalPerLevel[level]

	// numSkipped may overflow a signed int; compare as unsigned. If we've
	// passed the posting list length, this level is exhausted.
	if compareUnsigned(r.numSkipped[level], r.docCount) > 0 {
		r.skipDoc[level] = math.MaxInt32
		if r.numberOfSkipLevels > level {
			r.numberOfSkipLevels = level
		}
		return false, nil
	}

	delta, err := r.readSkipData(level, r.skipStream[level])
	if err != nil {
		return false, fmt.Errorf("MultiLevelSkipListReader: readSkipData(level=%d): %w", level, err)
	}
	if delta <= 0 {
		// A non-positive delta marks the end of this skip level. (This also
		// catches test hooks that return (0, nil) on EOF.)
		r.skipDoc[level] = math.MaxInt32
		if r.numberOfSkipLevels > level {
			r.numberOfSkipLevels = level
		}
		return false, nil
	}
	r.skipDoc[level] += delta

	if level != 0 {
		readChildPtr := r.readChildPointer
		if readChildPtr == nil {
			readChildPtr = defaultReadChildPointer
		}
		childPtr, cpErr := readChildPtr(r.skipStream[level])
		if cpErr != nil {
			return false, fmt.Errorf("MultiLevelSkipListReader: readChildPointer(level=%d): %w", level, cpErr)
		}
		r.childPointer[level] = childPtr + r.skipPointer[level-1]
	}

	return true, nil
}

// setLastSkipData snapshots the current skip entry as the last accepted one.
// This is invoked immediately before advancing the cursor.
func (r *MultiLevelSkipListReader) setLastSkipData(level int) {
	if r.onSetLastSkipData != nil {
		r.onSetLastSkipData(level)
	}
	r.lastDoc = r.skipDoc[level]
	r.lastChildPointer = r.childPointer[level]
}

// seekChild repositions the child level at the last accepted skip entry.
func (r *MultiLevelSkipListReader) seekChild(level int) error {
	if r.onSeekChild != nil {
		r.onSeekChild(level)
	}
	if err := r.skipStream[level].SetPosition(r.lastChildPointer); err != nil {
		return fmt.Errorf("MultiLevelSkipListReader: seekChild(level=%d): %w", level, err)
	}
	r.numSkipped[level] = r.numSkipped[level+1] - r.skipIntervalPerLevel[level+1]
	r.skipDoc[level] = r.lastDoc
	if level > 0 {
		readChildPtr := r.readChildPointer
		if readChildPtr == nil {
			readChildPtr = defaultReadChildPointer
		}
		childPtr, err := readChildPtr(r.skipStream[level])
		if err != nil {
			return fmt.Errorf("MultiLevelSkipListReader: seekChild readChildPointer(level=%d): %w", level, err)
		}
		r.childPointer[level] = childPtr + r.skipPointer[level-1]
	}
	return nil
}

// compareUnsigned compares two int values as unsigned 32-bit integers.
// Returns -1, 0, or 1.
func compareUnsigned(a, b int) int {
	ua := uint32(a)
	ub := uint32(b)
	if ua < ub {
		return -1
	}
	if ua > ub {
		return 1
	}
	return 0
}

// GetDoc returns the last document id reached by the bottom-level cursor.
// Concrete codec readers use this to position their posting iterator after
// a SkipTo call.
func (r *MultiLevelSkipListReader) GetDoc() int {
	return r.lastDoc
}

// NumberOfSkipLevels returns the actual height for the current term. Exposed
// for tests and diagnostics.
func (r *MultiLevelSkipListReader) NumberOfSkipLevels() int {
	return r.numberOfSkipLevels
}

// GetSkipDoc returns the doc id of the current skip entry at the given level.
// Exposed so that concrete skip readers (e.g. Lucene50ScoreSkipReader) can
// implement the Impacts.GetDocIDUpTo contract without embedding unexported
// state.
func (r *MultiLevelSkipListReader) GetSkipDoc(level int) int {
	return r.skipDoc[level]
}

// SetChildPointer sets the child pointer for the given level. This is an
// escape hatch for codecs (e.g. SimpleText) whose readSkipData implementation
// parses the child pointer inline, rather than using the default hook.
func (r *MultiLevelSkipListReader) SetChildPointer(level int, ptr int64) {
	if level >= 0 && level < len(r.childPointer) {
		r.childPointer[level] = ptr
	}
}

// GetChildPointer returns the child pointer for the given level.
// Exposed for the onSetLastSkipData callback.
func (r *MultiLevelSkipListReader) GetChildPointer(level int) int64 {
	if level >= 0 && level < len(r.childPointer) {
		return r.childPointer[level]
	}
	return 0
}

// GetSkipPointer returns the skip pointer (start of skip data) for the given level.
func (r *MultiLevelSkipListReader) GetSkipPointer(level int) int64 {
	if level >= 0 && level < len(r.skipPointer) {
		return r.skipPointer[level]
	}
	return 0
}

// MaxNumberOfSkipLevels returns the configured maximum number of skip levels.
func (r *MultiLevelSkipListReader) MaxNumberOfSkipLevels() int {
	return r.maxNumberOfSkipLevels
}

// Close releases the cloned per-level streams. The master stream supplied at
// construction is the caller's responsibility.
func (r *MultiLevelSkipListReader) Close() error {
	var firstErr error
	for level := 1; level < len(r.skipStream); level++ {
		if r.skipStream[level] == nil {
			continue
		}
		if err := r.skipStream[level].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		r.skipStream[level] = nil
	}
	return firstErr
}
