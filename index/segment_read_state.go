package index

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentReadState is a holder class for common parameters used during read.
// This is the Go port of Lucene's org.apache.lucene.index.SegmentReadState.
type SegmentReadState struct {
	// Directory is the directory where this segment is read from.
	Directory store.Directory
	// SegmentInfo is the SegmentInfo describing this segment.
	SegmentInfo *SegmentInfo
	// FieldInfos describes all fields in this segment.
	FieldInfos *FieldInfos
	// Context is the IOContext to pass to Directory.OpenInput(string, IOContext).
	Context store.IOContext
	// SegmentSuffix is a unique suffix for any postings files read for this segment.
	SegmentSuffix string
}

// NewSegmentReadState creates a new SegmentReadState.
func NewSegmentReadState(dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, context store.IOContext) *SegmentReadState {
	return NewSegmentReadStateWithSuffix(dir, info, fieldInfos, context, "")
}

// NewSegmentReadStateWithSuffix creates a new SegmentReadState with a specific segment suffix.
func NewSegmentReadStateWithSuffix(dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, context store.IOContext, segmentSuffix string) *SegmentReadState {
	return &SegmentReadState{
		Directory:     dir,
		SegmentInfo:   info,
		FieldInfos:    fieldInfos,
		Context:       context,
		SegmentSuffix: segmentSuffix,
	}
}

// NewSegmentReadStateFromOther creates a new SegmentReadState based on another, with a new segment suffix.
func NewSegmentReadStateFromOther(other *SegmentReadState, newSegmentSuffix string) *SegmentReadState {
	return &SegmentReadState{
		Directory:     other.Directory,
		SegmentInfo:   other.SegmentInfo,
		FieldInfos:    other.FieldInfos,
		Context:       other.Context,
		SegmentSuffix: newSegmentSuffix,
	}
}
