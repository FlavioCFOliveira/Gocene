// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/spi"

// SegmentCommitInfo is the Go port of
// org.apache.lucene.index.SegmentCommitInfo from Apache Lucene 10.5.0: a
// SegmentInfo plus the per-commit state that changes as documents are deleted
// and doc-values are updated — the deletion generation, the field-infos and
// doc-values generations, and the files each of those generations wrote.
//
// PORT NOTE: the declaration lives in package spi because the canonical
// segments_N reader and writer (SegmentInfosFormat) must name it without
// taking a back-edge into index. index re-exports it here under its Lucene
// name, exactly as it does for SegmentInfos.
type SegmentCommitInfo = spi.SegmentCommitInfo

// NewSegmentCommitInfo builds a SegmentCommitInfo over the given SegmentInfo
// with the supplied deleted-document count and deletion generation.
func NewSegmentCommitInfo(info *SegmentInfo, delCount int, delGen int64) *SegmentCommitInfo {
	return spi.NewSegmentCommitInfo(info, delCount, delGen)
}

// SegmentCommitInfoList is a slice of SegmentCommitInfo pointers, re-exported
// so callers can name the type spi uses for segment listings.
type SegmentCommitInfoList = spi.SegmentCommitInfoList
