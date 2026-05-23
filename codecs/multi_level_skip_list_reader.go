// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// MultiLevelSkipListReader walks a multi-level skip list produced by
// MultiLevelSkipListWriter to jump forward over a posting list without
// decoding every entry. It is the Go port of
// org.apache.lucene.codecs.MultiLevelSkipListReader from Apache Lucene 10.4.0.
//
// The reader maintains a per-level cursor: skipDoc[level] is the document
// reached by the cursor on that level and childPointer[level] is the file
// pointer the cursor would jump to in the level below. SkipTo walks down
// the levels, lazily decoding skip blocks until it finds the largest skip
// entry whose doc < target.
//
// MultiLevelSkipListReader is an abstract base in Java; in Go it is a
// concrete struct whose codec-specific decoding is supplied by an embedded
// callback hook (ReadSkipDataFunc). Concrete codec readers wire this hook
// to decode their per-level skip payload and return the doc delta gained.
type MultiLevelSkipListReader struct {
	// skipInterval is the number of postings between skip entries at level 0.
	skipInterval int
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

	// skipPointer[level] is the file pointer where the level starts.
	skipPointer []int64
	// childPointer[level] is the file pointer in the level below that the
	// current skip block would jump to. Level 0 has no children.
	childPointer []int64

	// skipDoc[level] is the document id reached by the cursor on that level.
	skipDoc []int

	// lastDoc is the document id reached by the bottom-level cursor and is
	// returned by GetDoc.
	lastDoc int

	// lastChildPointer is the child pointer of the most recent skip on level 0.
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
	// pointer reader used in loadSkipLevels. Set via SetReadChildPointer.
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
	}
	r.skipStream[0] = skipStream
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
	r.lastDoc = 0
	r.lastChildPointer = 0
	r.haveSkipped = false

	if r.numberOfSkipLevels == 0 {
		return nil
	}
	r.skipPointer[r.numberOfSkipLevels-1] = skipPointer
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
	if err := r.skipStream[0].SetPosition(r.skipPointer[r.numberOfSkipLevels-1]); err != nil {
		return fmt.Errorf("MultiLevelSkipListReader: seek top level: %w", err)
	}
	// Walk top-down. For each level above 0, read the length of the level
	// below and reserve a clone positioned at the next level's start.
	for level := r.numberOfSkipLevels - 1; level > 0; level-- {
		// Read length of the child level — use hook if provided, else VLong.
		var childLen int64
		var err error
		if r.readLevelLength != nil {
			childLen, err = r.readLevelLength(r.skipStream[0])
		} else {
			childLen, err = store.ReadVLong(r.skipStream[0])
		}
		if err != nil {
			return fmt.Errorf("MultiLevelSkipListReader: readLevelLength(level=%d): %w", level, err)
		}
		// Current position is the start of this level's payload.
		levelStart, err := positionOf(r.skipStream[0])
		if err != nil {
			return fmt.Errorf("MultiLevelSkipListReader: tell(level=%d): %w", level, err)
		}
		r.skipPointer[level] = levelStart
		r.childPointer[level] = levelStart - childLen
		// Clone the stream for independent advancement on this level.
		clone := r.skipStream[0].Clone()
		if err := clone.SetPosition(levelStart); err != nil {
			return fmt.Errorf("MultiLevelSkipListReader: clone seek(level=%d): %w", level, err)
		}
		r.skipStream[level] = clone
	}
	// Level 0 starts wherever the master stream is now (after the top-down
	// header walk).
	level0Start, err := positionOf(r.skipStream[0])
	if err != nil {
		return fmt.Errorf("MultiLevelSkipListReader: tell(level=0): %w", err)
	}
	r.skipPointer[0] = level0Start
	return nil
}

// positionOf reads the current file pointer of in. Centralised here because
// store.IndexInput exposes different cursors in different builds; this
// indirection keeps the reader portable.
func positionOf(in store.IndexInput) (int64, error) {
	return in.GetFilePointer(), nil
}

// SkipTo advances the cursor so that the bottom-level skipDoc is the largest
// document id <= target reachable through the skip list. Returns the number
// of postings skipped (not docs) — matches Java's int return value.
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

	// Walk levels top-down; at each level, advance while the next skip
	// would still keep us strictly below target.
	level := r.numberOfSkipLevels - 1
	numSkipped := 0
	for level >= 0 {
		for r.skipDoc[level] < target {
			// Mirror Java's setLastSkipData(level): snapshot current state before
			// advancing. Concrete readers (e.g. Lucene50SkipReader) use this hook
			// to persist per-level pointers as the "last accepted" position.
			if r.onSetLastSkipData != nil {
				r.onSetLastSkipData(level)
			}
			r.lastDoc = r.skipDoc[level]
			r.lastChildPointer = r.childPointer[level]

			delta, err := r.readSkipData(level, r.skipStream[level])
			if err != nil {
				return numSkipped, fmt.Errorf("MultiLevelSkipListReader: readSkipData(level=%d): %w", level, err)
			}
			if delta == 0 {
				// No more skip entries on this level.
				break
			}
			r.skipDoc[level] += delta
			// For non-leaf levels, read the child pointer if a hook is provided
			// (mirrors Java's loadNextSkip: childPointer[level] = readChildPointer(...) + skipPointer[level-1]).
			if level != 0 && r.readChildPointer != nil {
				childPtr, cpErr := r.readChildPointer(r.skipStream[level])
				if cpErr != nil {
					return numSkipped, fmt.Errorf("MultiLevelSkipListReader: readChildPointer(level=%d): %w", level, cpErr)
				}
				r.childPointer[level] = childPtr + r.skipPointer[level-1]
			}
			if level == 0 {
				numSkipped += r.skipInterval
			}
		}
		// Mirror Java's seekChild(level-1): when descending, reposition the
		// child level if the last accepted child pointer is ahead of the
		// current position.
		if level > 0 && r.skipStream[level-1] != nil &&
			r.lastChildPointer > r.skipStream[level-1].GetFilePointer() {
			if r.onSeekChild != nil {
				r.onSeekChild(level - 1)
			}
			if err := r.skipStream[level-1].SetPosition(r.lastChildPointer); err != nil {
				return numSkipped, fmt.Errorf("MultiLevelSkipListReader: seekChild(level=%d): %w", level-1, err)
			}
		}
		level--
	}

	return numSkipped, nil
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
