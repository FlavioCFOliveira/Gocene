// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentReadState is a holder class for common parameters used during read.
// Mirrors org.apache.lucene.index.SegmentReadState from Apache Lucene 10.5.0.
type SegmentReadState struct {
	// Directory where this segment is read from.
	Directory store.Directory
	// SegmentInfo describing this segment.
	SegmentInfo *SegmentInfo
	// FieldInfos describing all fields in this segment.
	FieldInfos *FieldInfos
	// Context to pass to Directory.OpenInput.
	Context store.IOContext
	// SegmentSuffix is a unique suffix for any postings files read for this segment.
	SegmentSuffix string
}

// NewSegmentReadState constructs a SegmentReadState.
func NewSegmentReadState(dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, context store.IOContext) *SegmentReadState {
	return NewSegmentReadStateWithSuffix(dir, info, fieldInfos, context, "")
}

// NewSegmentReadStateWithSuffix constructs a SegmentReadState with a segment suffix.
func NewSegmentReadStateWithSuffix(dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, context store.IOContext, segmentSuffix string) *SegmentReadState {
	return &SegmentReadState{
		Directory:     dir,
		SegmentInfo:   info,
		FieldInfos:    fieldInfos,
		Context:       context,
		SegmentSuffix: segmentSuffix,
	}
}

// NewSegmentReadStateFromOther creates a SegmentReadState from another, with a new segment suffix.
func NewSegmentReadStateFromOther(other *SegmentReadState, newSegmentSuffix string) *SegmentReadState {
	return &SegmentReadState{
		Directory:     other.Directory,
		SegmentInfo:   other.SegmentInfo,
		FieldInfos:    other.FieldInfos,
		Context:       other.Context,
		SegmentSuffix: newSegmentSuffix,
	}
}
