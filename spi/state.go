// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentWriteState bundles the per-segment context that every codec
// writer (postings, stored fields, term vectors, field infos) receives.
//
// Mirrors org.apache.lucene.index.SegmentWriteState in Apache Lucene
// 10.4.0, minus the fields whose Gocene counterparts have been deferred
// to follow-up tasks under rmp #4669.
type SegmentWriteState struct {
	// Directory is where the segment files are written.
	Directory store.Directory

	// SegmentInfo carries the metadata for the segment being flushed.
	SegmentInfo *schema.SegmentInfo

	// FieldInfos carries the metadata for every field in the segment.
	FieldInfos *schema.FieldInfos

	// SegmentSuffix is an optional per-format suffix appended to the
	// segment file names. Empty for the default codec; PerField formats
	// populate it to disambiguate sub-formats that share a directory.
	SegmentSuffix string

	// SegUpdates holds the buffered term deletions that must be applied
	// during flush. May be nil when no pending deletes exist. Carried as
	// the BufferedUpdatesRef marker interface so spi/ does not depend on
	// index/; the index-side flush path type-asserts to the concrete
	// *index.BufferedUpdates value.
	//
	// Mirrors Lucene 10.4.0 SegmentWriteState.segUpdates.
	SegUpdates BufferedUpdatesRef

	// LiveDocs is the per-document live-docs bitset that applyDeletes
	// populates when term deletions are applied during flush. Nil until
	// at least one document is deleted; callers allocate it on first
	// use.
	//
	// Mirrors Lucene 10.4.0 SegmentWriteState.liveDocs.
	LiveDocs *util.FixedBitSet

	// DelCountOnFlush is incremented for every document cleared from
	// LiveDocs during the applyDeletes pass. The final value updates the
	// segment's live-doc count.
	//
	// Mirrors Lucene 10.4.0 SegmentWriteState.delCountOnFlush.
	DelCountOnFlush int

	// NeedsIndexSort indicates that the merge producing this segment
	// reorders documents to honour an index sort. It is set by
	// SegmentMerger.buildDocMaps when a reorder is required (i.e. the
	// source segments were not already globally sorted relative to one
	// another). Codecs may consult it during merge to adapt their write
	// strategy; test codecs (e.g. AssertingNeedsIndexSortCodec) use it to
	// observe the merge-need signal.
	//
	// Nil during flush; only populated on the merge path.
	NeedsIndexSort bool

	// IsMerge is true when this SegmentWriteState is created during a merge
	// (by SegmentMerger) rather than during a flush. Test codecs use it to
	// distinguish merge calls from flush calls.
	IsMerge bool
}

// SegmentReadState bundles the per-segment context that every codec
// reader (postings, stored fields, term vectors, field infos) receives.
//
// Mirrors org.apache.lucene.index.SegmentReadState in Apache Lucene
// 10.4.0.
type SegmentReadState struct {
	// Directory is where the segment files are read from.
	Directory store.Directory

	// SegmentInfo carries the metadata for the segment being read.
	SegmentInfo *schema.SegmentInfo

	// FieldInfos carries the metadata for every field in the segment.
	FieldInfos *schema.FieldInfos

	// SegmentSuffix is an optional per-format suffix used to look up
	// segment files. Empty for the default codec.
	SegmentSuffix string
}
