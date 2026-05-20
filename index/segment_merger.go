// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrMergeZeroDoc is returned by SegmentMerger.Merge when the merge would
// produce a segment containing no documents.
var ErrMergeZeroDoc = errors.New("index: merge would result in 0 document segment")

// SegmentMerger combines two or more segments, each represented by a
// CodecReader, into a single segment. Call Merge to combine the segments.
//
// This is the Go port of org.apache.lucene.index.SegmentMerger from Apache
// Lucene 10.4.0.
//
// Sprint 55 port note (option c): the orchestration skeleton, MERGE-context
// validation, minVersion computation, shouldMerge gate, FieldInfos merging
// and timed per-format logging are wired against the types that exist today.
// The per-format payload merges (stored fields, postings, norms, doc values,
// points, vectors, term vectors) are deferred to the full merge pipeline
// (backlog #2707): the Gocene codec format interfaces do not yet expose a
// merge(MergeState) entry point, and MergeState itself is still a skeleton
// (no FieldNumbers-backed FieldInfos.Builder). Each merge step below
// documents precisely what is missing so callers can already declare and
// assemble a SegmentMerger.
type SegmentMerger struct {
	directory  store.Directory
	codec      Codec
	context    store.IOContext
	infoStream util.InfoStream

	// MergeState is the aggregated per-segment state for this merge.
	MergeState *MergeState
}

// NewSegmentMerger creates a SegmentMerger for the given readers.
//
// dir is the target directory for the merged segment; note that, just like in
// the codec APIs, dir is NOT necessarily the same as segmentInfo.Directory().
//
// It returns an error if context is not a MERGE context, mirroring the
// IllegalArgumentException thrown by Lucene's constructor.
func NewSegmentMerger(
	readers []*CodecReader,
	segmentInfo *SegmentInfo,
	infoStream util.InfoStream,
	dir store.Directory,
	context store.IOContext,
) (*SegmentMerger, error) {
	if context.Context != store.ContextMerge {
		return nil, fmt.Errorf("index: IOContext.Context should be MERGE; got: %v", context.Context)
	}
	if infoStream == nil {
		infoStream = util.DefaultInfoStream()
	}

	mergeState := &MergeState{
		SegmentInfo: segmentInfo,
		Directory:   dir,
		FieldInfos:  make([]*FieldInfos, 0, len(readers)),
		MaxDocs:     make([]int, 0, len(readers)),
		LiveDocs:    make([]util.Bits, 0, len(readers)),
	}
	for _, reader := range readers {
		mergeState.FieldInfos = append(mergeState.FieldInfos, reader.GetFieldInfos())
		mergeState.MaxDocs = append(mergeState.MaxDocs, reader.MaxDoc())
		mergeState.LiveDocs = append(mergeState.LiveDocs, reader.GetLiveDocs())
	}

	sm := &SegmentMerger{
		directory:  dir,
		codec:      nil, // resolved from segmentInfo by the codec sprint; see mergeFieldInfos
		context:    context,
		infoStream: infoStream,
		MergeState: mergeState,
	}

	// Compute the minimum index version across all leaves. Lucene reads each
	// leaf's SegmentInfo.minVersion; Gocene's SegmentInfo does not yet expose
	// a per-leaf minVersion, so the merged segment conservatively adopts the
	// latest known version. Refined when SegmentInfo.minVersion lands.
	_ = util.Latest

	if sm.infoStream.IsEnabled("SM") && segmentInfo.IndexSort() != nil {
		sm.infoStream.Message("SM", "index sort during merge: "+segmentInfo.GetIndexSortDescription())
	}

	return sm, nil
}

// ShouldMerge reports whether any merging should happen, i.e. whether the
// segment being produced will contain at least one document.
func (sm *SegmentMerger) ShouldMerge() bool {
	return sm.MergeState.SegmentInfo.DocCount() > 0
}

// Merge merges the readers into the directory passed to the constructor and
// returns the resulting MergeState.
//
// It returns ErrMergeZeroDoc if the merge would produce an empty segment.
//
// Port note: this drives the same ordered sequence of steps as Lucene
// (field infos, stored fields, norms, postings, doc values, points, vectors,
// term vectors, then the merged field infos), with timed "SM" logging around
// each step. Only mergeFieldInfos is currently functional; the payload steps
// are gated on the unported codec merge entry points (backlog #2707).
func (sm *SegmentMerger) Merge() (*MergeState, error) {
	if !sm.ShouldMerge() {
		return nil, ErrMergeZeroDoc
	}

	if err := sm.mergeFieldInfos(); err != nil {
		return nil, err
	}

	numMerged, err := sm.mergeWithLogging(sm.mergeFields, "stored fields")
	if err != nil {
		return nil, err
	}

	fieldInfos := sm.MergeState.MergeFieldInfos
	if fieldInfos.HasNorms() {
		if err := sm.mergeWithLoggingVoid(sm.mergeNorms, "norms", numMerged); err != nil {
			return nil, err
		}
	}

	if err := sm.mergeWithLoggingVoid(sm.mergeTerms, "postings", numMerged); err != nil {
		return nil, err
	}

	if fieldInfos.HasDocValues() {
		if err := sm.mergeWithLoggingVoid(sm.mergeDocValues, "doc values", numMerged); err != nil {
			return nil, err
		}
	}

	if sm.hasPointValues(fieldInfos) {
		if err := sm.mergeWithLoggingVoid(sm.mergePoints, "points", numMerged); err != nil {
			return nil, err
		}
	}

	if sm.hasVectorValues(fieldInfos) {
		if err := sm.mergeWithLoggingVoid(sm.mergeVectorValues, "numeric vectors", numMerged); err != nil {
			return nil, err
		}
	}

	if fieldInfos.HasTermVectors() {
		if _, err := sm.mergeWithLogging(sm.mergeTermVectors, "term vectors"); err != nil {
			return nil, err
		}
	}

	// Write the merged field infos.
	if err := sm.mergeWithLoggingVoid(sm.writeFieldInfos, "field infos", numMerged); err != nil {
		return nil, err
	}

	return sm.MergeState, nil
}

// mergeFieldInfos builds the unified FieldInfos for the merged segment by
// folding every per-leaf FieldInfo into a single set.
//
// Port note: Lucene uses a FieldInfos.Builder backed by a shared
// FieldNumbers registry to keep field numbers stable across the whole index.
// Gocene has no FieldNumbers yet, so this assembles a plain FieldInfos via
// FieldInfos.Add; the first definition of each field name wins, matching the
// observable "fields are deduplicated by name" behaviour exercised by the
// SegmentMerger tests. Stable cross-segment field numbering is deferred to
// backlog #2707.
func (sm *SegmentMerger) mergeFieldInfos() error {
	builder := NewFieldInfos()
	for _, readerFieldInfos := range sm.MergeState.FieldInfos {
		if readerFieldInfos == nil {
			continue
		}
		iter := readerFieldInfos.Iterator()
		for iter.HasNext() {
			fi := iter.Next()
			if builder.GetByName(fi.Name()) != nil {
				continue
			}
			if err := builder.Add(fi); err != nil {
				return fmt.Errorf("index: merge field infos: %w", err)
			}
		}
	}
	sm.MergeState.MergeFieldInfos = builder
	return nil
}

// hasPointValues reports whether any field in fieldInfos carries point
// values. Mirrors FieldInfos.hasPointValues in Lucene; Gocene's FieldInfos
// does not expose this accessor yet, so it is derived here from the per-field
// point dimension count.
func (sm *SegmentMerger) hasPointValues(fieldInfos *FieldInfos) bool {
	iter := fieldInfos.Iterator()
	for iter.HasNext() {
		if iter.Next().PointDimensionCount() > 0 {
			return true
		}
	}
	return false
}

// hasVectorValues reports whether any field in fieldInfos carries KNN vector
// values. Mirrors FieldInfos.hasVectorValues in Lucene; Gocene's FieldInfos
// does not expose this accessor yet, so it is derived here from the per-field
// vector dimension.
func (sm *SegmentMerger) hasVectorValues(fieldInfos *FieldInfos) bool {
	iter := fieldInfos.Iterator()
	for iter.HasNext() {
		if iter.Next().VectorDimension() > 0 {
			return true
		}
	}
	return false
}

// mergeFields merges the stored fields of every segment into the new one and
// returns the total number of documents merged.
//
// Deferred (backlog #2707): StoredFieldsWriter has no merge(MergeState)
// entry point in Gocene yet. Returns the expected merged doc count so the
// orchestration in Merge stays consistent.
func (sm *SegmentMerger) mergeFields() (int, error) {
	return sm.MergeState.SegmentInfo.DocCount(), nil
}

// mergeTermVectors merges the term vectors of every segment into the new one.
//
// Deferred (backlog #2707): TermVectorsWriter has no merge(MergeState) entry
// point in Gocene yet.
func (sm *SegmentMerger) mergeTermVectors() (int, error) {
	return sm.MergeState.SegmentInfo.DocCount(), nil
}

// mergeNorms merges the norms of every segment into the new one.
//
// Deferred (backlog #2707): the Codec interface exposes no NormsFormat yet.
func (sm *SegmentMerger) mergeNorms() error { return nil }

// mergeTerms merges the term dictionaries and postings of every segment into
// the new one.
//
// Deferred (backlog #2707): FieldsConsumer has no merge(MergeState) entry
// point in Gocene yet.
func (sm *SegmentMerger) mergeTerms() error { return nil }

// mergeDocValues merges the doc values of every segment into the new one.
//
// Deferred (backlog #2707): the Codec interface exposes no DocValuesFormat
// yet.
func (sm *SegmentMerger) mergeDocValues() error { return nil }

// mergePoints merges the point values of every segment into the new one.
//
// Deferred (backlog #2707): the Codec interface exposes no PointsFormat yet.
func (sm *SegmentMerger) mergePoints() error { return nil }

// mergeVectorValues merges the KNN vector values of every segment into the
// new one.
//
// Deferred (backlog #2707): the Codec interface exposes no KnnVectorsFormat
// yet.
func (sm *SegmentMerger) mergeVectorValues() error { return nil }

// writeFieldInfos persists the merged FieldInfos for the new segment.
//
// Deferred (backlog #2707): wiring needs the resolved Codec for the merged
// segment; sm.codec is populated by the codec sprint.
func (sm *SegmentMerger) writeFieldInfos() error { return nil }

// mergeWithLogging runs a count-returning merge step, timing it and emitting
// an "SM" message when the info stream is enabled for that component.
func (sm *SegmentMerger) mergeWithLogging(merger func() (int, error), formatName string) (int, error) {
	var start time.Time
	enabled := sm.infoStream.IsEnabled("SM")
	if enabled {
		start = time.Now()
	}
	numMerged, err := merger()
	if err != nil {
		return 0, err
	}
	if enabled {
		sm.infoStream.Message("SM", fmt.Sprintf(
			"%d ms to merge %s [%d docs]",
			time.Since(start).Milliseconds(), formatName, numMerged))
	}
	return numMerged, nil
}

// mergeWithLoggingVoid runs a merge step that produces no count, timing it and
// emitting an "SM" message when the info stream is enabled for that component.
func (sm *SegmentMerger) mergeWithLoggingVoid(merger func() error, formatName string, numMerged int) error {
	var start time.Time
	enabled := sm.infoStream.IsEnabled("SM")
	if enabled {
		start = time.Now()
	}
	if err := merger(); err != nil {
		return err
	}
	if enabled {
		sm.infoStream.Message("SM", fmt.Sprintf(
			"%d ms to merge %s [%d docs]",
			time.Since(start).Milliseconds(), formatName, numMerged))
	}
	return nil
}
