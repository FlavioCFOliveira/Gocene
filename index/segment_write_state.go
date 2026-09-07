package index

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentWriteState is a holder class for common parameters used during write.
// This is the Go port of Lucene's org.apache.lucene.index.SegmentWriteState.
type SegmentWriteState struct {
	// InfoStream is used for debugging messages.
	InfoStream util.InfoStream
	// Directory is the directory where this segment will be written to.
	Directory store.Directory
	// SegmentInfo is the SegmentInfo describing this segment.
	SegmentInfo *SegmentInfo
	// FieldInfos describes all fields in this segment.
	FieldInfos *FieldInfos
	// DelCountOnFlush is the number of deleted documents set while flushing the segment.
	DelCountOnFlush int
	// SoftDelCountOnFlush is the number of only soft deleted documents set while flushing the segment.
	SoftDelCountOnFlush int
	// SegUpdates is the BufferedUpdates to apply while we are flushing the segment.
	SegUpdates *BufferedUpdates
	// LiveDocs records live documents; this is only set if there is one or more deleted documents.
	LiveDocs *util.FixedBitSet
	// SegmentSuffix is a unique suffix for any postings files written for this segment.
	SegmentSuffix string
	// Context is the IOContext for all writes.
	Context store.IOContext
}

// NewSegmentWriteState creates a new SegmentWriteState.
func NewSegmentWriteState(
	infoStream util.InfoStream,
	directory store.Directory,
	segmentInfo *SegmentInfo,
	fieldInfos *FieldInfos,
	segUpdates *BufferedUpdates,
	context store.IOContext,
) *SegmentWriteState {
	return NewSegmentWriteStateWithSuffix(infoStream, directory, segmentInfo, fieldInfos, segUpdates, context, "")
}

// NewSegmentWriteStateWithSuffix creates a new SegmentWriteState with a segment suffix.
func NewSegmentWriteStateWithSuffix(
	infoStream util.InfoStream,
	directory store.Directory,
	segmentInfo *SegmentInfo,
	fieldInfos *FieldInfos,
	segUpdates *BufferedUpdates,
	context store.IOContext,
	segmentSuffix string,
) *SegmentWriteState {
	// In Java, this is an assert. In Go, we can perform a check if needed,
	// but for a faithful port of a constructor, we just assign.
	return &SegmentWriteState{
		InfoStream:     infoStream,
		Directory:      directory,
		SegmentInfo:    segmentInfo,
		FieldInfos:     fieldInfos,
		SegUpdates:     segUpdates,
		Context:        context,
		SegmentSuffix:  segmentSuffix,
	}
}

// NewSegmentWriteStateFromOther creates a shallow copy of SegmentWriteState with a new segment suffix.
func NewSegmentWriteStateFromOther(state *SegmentWriteState, segmentSuffix string) *SegmentWriteState {
	return &SegmentWriteState{
		InfoStream:     state.InfoStream,
		Directory:      state.Directory,
		SegmentInfo:    state.SegmentInfo,
		FieldInfos:     state.FieldInfos,
		Context:        state.Context,
		SegmentSuffix:  segmentSuffix,
		SegUpdates:     state.SegUpdates,
		DelCountOnFlush: state.DelCountOnFlush,
		LiveDocs:       state.LiveDocs,
	}
}
