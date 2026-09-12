// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentReadState and SegmentWriteState are declared as SPI aliases in
// codec_interface.go (type SegmentReadState = spi.SegmentReadState). These
// constructors assemble the codec-facing shape from the index package's
// *SegmentInfo, which is itself an alias of *spi.SegmentInfo.

// NewSegmentReadState constructs a SegmentReadState for reading segment info.
func NewSegmentReadState(dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, _ store.IOContext) *SegmentReadState {
	return NewSegmentReadStateWithSuffix(dir, info, fieldInfos, store.IOContext{}, "")
}

// NewSegmentReadStateWithSuffix constructs a SegmentReadState with a per-format suffix.
func NewSegmentReadStateWithSuffix(dir store.Directory, info *SegmentInfo, fieldInfos *FieldInfos, _ store.IOContext, suffix string) *SegmentReadState {
	return &SegmentReadState{
		Directory:     dir,
		SegmentInfo:   info,
		FieldInfos:    fieldInfos,
		SegmentSuffix: suffix,
	}
}
