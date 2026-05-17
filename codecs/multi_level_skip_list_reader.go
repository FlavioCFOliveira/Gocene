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
}

// ReadSkipDataFunc is the codec hook invoked once per level for each
// downward step. The implementation reads its per-level skip payload from
// skipStream and returns the document delta gained by this skip.
type ReadSkipDataFunc func(level int, skipStream store.IndexInput) (int, error)

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
		// Read length VLong of the child level.
		childLen, err := store.ReadVLong(r.skipStream[0])
		if err != nil {
			return fmt.Errorf("MultiLevelSkipListReader: readVLong(level=%d): %w", level, err)
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
			delta, err := r.readSkipData(level, r.skipStream[level])
			if err != nil {
				return numSkipped, fmt.Errorf("MultiLevelSkipListReader: readSkipData(level=%d): %w", level, err)
			}
			if delta == 0 {
				// No more skip entries on this level.
				break
			}
			r.skipDoc[level] += delta
			if level == 0 {
				numSkipped += r.skipInterval
				r.lastDoc = r.skipDoc[0]
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

