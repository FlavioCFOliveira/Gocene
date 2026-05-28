// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// SegmentCommitInfo is an alias of spi.SegmentCommitInfo.
//
// Lifted from package index to package spi by rmp #4706 so that the
// canonical SegmentInfosFormat (segments_N reader/writer) can return and
// accept SegmentCommitInfo values without a back-edge into package index.
type SegmentCommitInfo = spi.SegmentCommitInfo

// SegmentCommitInfoList is an alias of spi.SegmentCommitInfoList.
type SegmentCommitInfoList = spi.SegmentCommitInfoList

// NewSegmentCommitInfo is a thin re-export of spi.NewSegmentCommitInfo.
//
// SegmentInfo is itself a schema-leaf type, so this re-export documents the
// post-lift type chain at the index-side call site without forcing callers
// to import spi/ directly.
func NewSegmentCommitInfo(segmentInfo *schema.SegmentInfo, delCount int, delGen int64) *SegmentCommitInfo {
	return spi.NewSegmentCommitInfo(segmentInfo, delCount, delGen)
}
