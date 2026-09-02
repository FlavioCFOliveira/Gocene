//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentReadState is a holder class for common parameters used during read.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentReadState.
type SegmentReadState struct {
	// Directory where this segment is read from.
	Directory util.Directory

	// SegmentInfo describing this segment.
	SegmentInfo *SegmentInfo

	// FieldInfos describing all fields in this segment.
	FieldInfos *FieldInfos

	// Context to pass to Directory.OpenInput.
	Context util.IOContext

	// SegmentSuffix is a unique suffix for any postings files read for this segment.
	SegmentSuffix string
}

func NewSegmentReadState(dir util.Directory, info *SegmentInfo, fieldInfos *FieldInfos, context util.IOContext) *SegmentReadState {
	return &SegmentReadState{
		Directory:     dir,
		SegmentInfo:   info,
		FieldInfos:    fieldInfos,
		Context:       context,
		SegmentSuffix: "",
	}
}

func NewSegmentReadStateWithSuffix(dir util.Directory, info *SegmentInfo, fieldInfos *FieldInfos, context util.IOContext, suffix string) *SegmentReadState {
	return &SegmentReadState{
		Directory:     dir,
		SegmentInfo:   info,
		FieldInfos:    fieldInfos,
		Context:       context,
		SegmentSuffix: suffix,
	}
}
