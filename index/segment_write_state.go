// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentWriteState is a holder class for common parameters used during write.
// Mirrors org.apache.lucene.index.SegmentWriteState from Apache Lucene 10.5.0.
type SegmentWriteState struct {
	// InfoStream used for debugging messages.
	InfoStream *util.InfoStream
	// Directory where this segment will be written to.
	Directory store.Directory
	// SegmentInfo describing this segment.
	SegmentInfo *SegmentInfo
	// FieldInfos describing all fields in this segment.
	FieldInfos *FieldInfos
	// DelCountOnFlush is the number of deleted documents set while flushing the segment.
	DelCountOnFlush int
	// SoftDelCountOnFlush is the number of only soft deleted documents set while flushing the segment.
	SoftDelCountOnFlush int
	// SegUpdates contains deletes and updates to apply while flushing the segment.
	SegUpdates *BufferedUpdates
	// LiveDocs records live documents; this is only set if there is one or more deleted documents.
	LiveDocs *util.FixedBitSet
	// SegmentSuffix is a unique suffix for any postings files written for this segment.
	SegmentSuffix string
	// Context for all writes.
	Context store.IOContext
}

// NewSegmentWriteState constructs a SegmentWriteState.
func NewSegmentWriteState(infoStream *util.InfoStream, dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, segUpdates *BufferedUpdates, context store.IOContext) *SegmentWriteState {
	return NewSegmentWriteStateWithSuffix(infoStream, dir, info, fieldInfos, segUpdates, context, "")
}

// NewSegmentWriteStateWithSuffix constructs a SegmentWriteState with a segment suffix.
func NewSegmentWriteStateWithSuffix(infoStream *util.InfoStream, dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, segUpdates *BufferedUpdates, context store.IOContext, segmentSuffix string) *SegmentWriteState {
	if err := validateSegmentSuffix(segmentSuffix); err != nil {
		panic(err) // Lucene uses assertions; we panic here for similar behavior on invalid suffix
	}
	return &SegmentWriteState{
		InfoStream:    infoStream,
		SegUpdates:     segUpdates,
		Directory:     dir,
		SegmentInfo:   info,
		FieldInfos:    fieldInfos,
		SegmentSuffix: segmentSuffix,
		Context:       context,
	}
}

// NewSegmentWriteStateFromOther creates a shallow copy of SegmentWriteState with a new segment suffix.
func NewSegmentWriteStateFromOther(state *SegmentWriteState, segmentSuffix string) *SegmentWriteState {
	return &SegmentWriteState{
		InfoStream:    state.InfoStream,
		Directory:     state.Directory,
		SegmentInfo:   state.SegmentInfo,
		FieldInfos:    state.FieldInfos,
		Context:       state.Context,
		SegmentSuffix: segmentSuffix,
		SegUpdates:    state.SegUpdates,
		DelCountOnFlush: state.DelCountOnFlush,
		LiveDocs:      state.LiveDocs,
	}
}

func validateSegmentSuffix(segmentSuffix string) error {
	if segmentSuffix == "" {
		return nil
	}
	// either it's a segment suffix (_X_Y) or it's a parsable generation
	// check if it has exactly two parts separated by underscore
	// we use a simple count for this
	underscoreCount := 0
	for _, char := range segmentSuffix {
		if char == '_' {
			underscoreCount++
		}
	}

	if underscoreCount == 2 {
		return nil
	} else if underscoreCount == 0 {
		// check if it's a base36 generation (digits and letters)
		for _, char := range segmentSuffix {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')) {
				return fmt.Errorf("invalid segment suffix: %s", segmentSuffix)
			}
		}
		return nil
	}
	return fmt.Errorf("invalid segment suffix: %s", segmentSuffix)
}
