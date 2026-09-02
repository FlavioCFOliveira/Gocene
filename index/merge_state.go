// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MergeState aggregates the per-segment state that codecs and merge readers
// need to perform a merge. Mirrors org.apache.lucene.index.MergeState from
// Apache Lucene 10.4.0.
//
// Gocene skeleton: this initial port wires the field declarations and the
// DocMap interface; reader composition (mapping old docIDs to new docIDs)
// and live-docs aggregation are deferred to backlog #2707 (full merge
// pipeline). The struct shape is stable so callers can already declare
// MergeState arguments and assemble it field-by-field.
type MergeState struct {
	// SegmentInfo is the metadata for the segment being produced.
	SegmentInfo *SegmentInfo

	// MergeFieldInfos is the unified FieldInfos for the merged segment.
	MergeFieldInfos *FieldInfos

	// FieldInfos is the per-sub-reader FieldInfos snapshot.
	FieldInfos []*FieldInfos

	// MaxDocs is the per-sub-reader maxDoc.
	MaxDocs []int

	// DocMaps maps old doc IDs to new doc IDs (one per sub-reader).
	DocMaps []DocMap

	// LiveDocs is the per-sub-reader live-docs bitset (nil if all live).
	LiveDocs []util.Bits

	// Directory is the target directory for the merged segment.
	Directory store.Directory

	// NeedsIndexSort indicates the merge must honour an index-level sort.
	NeedsIndexSort bool

	// Readers are the per-sub-reader CodecReaders being merged, in the same
	// order as FieldInfos/MaxDocs/LiveDocs. The payload merge steps read their
	// stored fields / postings / doc values from these (rmp #14/#114).
	Readers []*CodecReader
}

// DocMap maps old doc IDs to new doc IDs during a merge. Mirrors
// org.apache.lucene.index.MergeState.DocMap from Lucene 10.4.0. A docID is
// mapped to the sentinel -1 when the corresponding document was deleted in
// the source segment.
type DocMap interface {
	// Get returns the new docID for the given old docID, or -1 if the
	// document was deleted.
	Get(oldDocID int) int
}
