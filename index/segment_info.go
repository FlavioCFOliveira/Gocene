// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Used as a fallback value for the deletion, field-infos and doc-values
// generations of a segment. Mirrors SegmentInfo.NO and SegmentInfo.YES.
const (
	// No marks a generation that does not exist.
	No = -1

	// Yes marks a generation that exists.
	Yes = 1
)

// SegmentInfo is the Go port of org.apache.lucene.index.SegmentInfo from
// Apache Lucene 10.5.0: the immutable description of a single segment — its
// name, directory, document count, codec, compound-file flag, diagnostics,
// attributes and index sort.
//
// PORT NOTE: the declaration lives in package schema, the shared vocabulary
// layer that codecs, spi and index all sit above, so that a SegmentInfo can be
// named without importing index. index re-exports it here under its Lucene
// name, exactly as it does for FieldInfo, FieldInfos and SegmentInfos.
type SegmentInfo = schema.SegmentInfo

// SegmentInfoList is a slice of SegmentInfo pointers.
type SegmentInfoList = schema.SegmentInfoList

// NewSegmentInfo builds a SegmentInfo for a segment of the given name and
// document count, living in dir.
func NewSegmentInfo(name string, docCount int, dir store.Directory) *SegmentInfo {
	return schema.NewSegmentInfo(name, docCount, dir)
}
