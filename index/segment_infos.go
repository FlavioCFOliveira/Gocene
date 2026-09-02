// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfos is an alias of spi.SegmentInfos.
//
// The canonical declaration lifted into package spi in rmp #4706 alongside
// SegmentCommitInfo and the segments_N reader/writer so the SPI surface no
// longer needs adapter shims for the plural SegmentInfosFormat.
type SegmentInfos = spi.SegmentInfos

// NewSegmentInfos is a thin re-export of spi.NewSegmentInfos.
func NewSegmentInfos() *SegmentInfos {
	return spi.NewSegmentInfos()
}

// GetSegmentFileName is a thin re-export of spi.GetSegmentFileName.
func GetSegmentFileName(generation int64) string {
	return spi.GetSegmentFileName(generation)
}

// WriteSegmentInfos is a thin re-export of spi.WriteSegmentInfos. Retained at
// the index-package level for the test corpus and legacy callers that wrote
// segments_N without going through a codec.
func WriteSegmentInfos(si *SegmentInfos, directory store.Directory) error {
	return spi.WriteSegmentInfos(si, directory)
}

// ReadSegmentInfos is a thin re-export of spi.ReadSegmentInfos.
func ReadSegmentInfos(directory store.Directory) (*SegmentInfos, error) {
	return spi.ReadSegmentInfos(directory)
}
