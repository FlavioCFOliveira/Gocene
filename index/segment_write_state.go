// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentWriteState is declared as an SPI alias in codec_interface.go
// (type SegmentWriteState = spi.SegmentWriteState). These constructors
// bridge from the index package's own *SegmentInfo and *BufferedUpdates to
// the codec-facing shape.

// NewSegmentWriteState constructs a SegmentWriteState.
func NewSegmentWriteState(dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, segUpdates *BufferedUpdates, context store.IOContext) *SegmentWriteState {
	return NewSegmentWriteStateWithSuffix(nil, dir, info, fieldInfos, segUpdates, context, "")
}

// NewSegmentWriteStateWithSuffix constructs a SegmentWriteState with a segment suffix.
// infoStream is accepted for call-site source compatibility but is not yet
// carried by spi.SegmentWriteState (rmp #4669 follow-up).
func NewSegmentWriteStateWithSuffix(_ *util.InfoStream, dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, segUpdates *BufferedUpdates, _ store.IOContext, segmentSuffix string) *SegmentWriteState {
	if err := validateSegmentSuffix(segmentSuffix); err != nil {
		panic(err) // Lucene uses assertions; we panic here for similar behavior on invalid suffix
	}
	var ref spi.BufferedUpdatesRef
	if segUpdates != nil {
		ref = segUpdates
	}
	return &SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   info.ToSchema(),
		FieldInfos:    fieldInfos,
		SegmentSuffix: segmentSuffix,
		SegUpdates:    ref,
	}
}

// NewSegmentWriteStateFromOther creates a shallow copy of SegmentWriteState with a new segment suffix.
func NewSegmentWriteStateFromOther(state *SegmentWriteState, segmentSuffix string) *SegmentWriteState {
	return &SegmentWriteState{
		Directory:       state.Directory,
		SegmentInfo:     state.SegmentInfo,
		FieldInfos:      state.FieldInfos,
		SegmentSuffix:   segmentSuffix,
		SegUpdates:      state.SegUpdates,
		DelCountOnFlush: state.DelCountOnFlush,
		LiveDocs:        state.LiveDocs,
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
