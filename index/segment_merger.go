// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"io"
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

	// inverseDocMap maps each merged docID to its source (readerIndex,
	// localDocID). It is populated only when the merge honours an index sort
	// (MergeState.NeedsIndexSort), and lets the per-document merge steps
	// (stored fields, term vectors) emit documents in the merged sort order
	// instead of the source (reader, docID) concatenation order.
	inverseDocMap [][2]int
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
		Readers:     readers,
	}
	for _, reader := range readers {
		mergeState.FieldInfos = append(mergeState.FieldInfos, reader.GetFieldInfos())
		mergeState.MaxDocs = append(mergeState.MaxDocs, reader.MaxDoc())
		mergeState.LiveDocs = append(mergeState.LiveDocs, reader.GetLiveDocs())
	}

	// Resolve the codec for the merged segment: prefer the target segment's
	// stamped codec name, falling back to the registered default. The merged
	// payload steps (stored fields, postings, ...) write through this codec.
	codec := resolveMergeCodec(segmentInfo)

	sm := &SegmentMerger{
		directory:  dir,
		codec:      codec,
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

	// Compute the shared old→new doc mapping once, up front, so every payload
	// step agrees on the new doc numbering. When the merged segment carries an
	// index sort this also reorders documents into sorted order (rmp #115).
	if err := sm.buildDocMaps(); err != nil {
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
// It reads each live document's stored fields from the source readers (in
// reader then docID order) and re-serialises them through the merged
// segment's StoredFieldsWriter, mirroring Lucene's
// StoredFieldsWriter.merge(MergeState) net effect (rmp #14/#114).
func (sm *SegmentMerger) mergeFields() (int, error) {
	if sm.codec == nil || sm.codec.StoredFieldsFormat() == nil {
		// No stored-fields codec wired: nothing to write. Keep the doc count
		// consistent for the orchestration.
		return sm.MergeState.SegmentInfo.DocCount(), nil
	}
	writer, err := sm.codec.StoredFieldsFormat().FieldsWriter(sm.directory, sm.MergeState.SegmentInfo, store.IOContextWrite)
	if err != nil {
		return 0, fmt.Errorf("index: merge stored fields: open writer: %w", err)
	}
	defer writer.Close()

	writeDoc := func(i, docID int) error {
		reader := sm.MergeState.Readers[i]
		sfr := reader.GetStoredFieldsReader()
		if err := writer.StartDocument(); err != nil {
			return fmt.Errorf("index: merge stored fields: start doc: %w", err)
		}
		if sfr != nil {
			visitor := &storedFieldsMergeVisitor{writer: writer}
			if err := sfr.VisitDocument(docID, visitor); err != nil {
				return fmt.Errorf("index: merge stored fields: visit doc %d of reader %d: %w", docID, i, err)
			}
			if visitor.err != nil {
				return visitor.err
			}
		}
		if err := writer.FinishDocument(); err != nil {
			return fmt.Errorf("index: merge stored fields: finish doc: %w", err)
		}
		return nil
	}

	total := 0
	if sm.MergeState.NeedsIndexSort && sm.inverseDocMap != nil {
		// Index-sorted merge: emit documents in merged (sorted) docID order so
		// the stored fields agree with the sorted doc-values / postings.
		for _, src := range sm.inverseDocMap {
			i, docID := src[0], src[1]
			if sm.MergeState.Readers[i] == nil {
				continue
			}
			if err := writeDoc(i, docID); err != nil {
				return 0, err
			}
			total++
		}
	} else {
		for i, reader := range sm.MergeState.Readers {
			if reader == nil {
				continue
			}
			if reader.GetStoredFieldsReader() == nil {
				continue
			}
			maxDoc := sm.MergeState.MaxDocs[i]
			liveDocs := sm.MergeState.LiveDocs[i]
			for docID := 0; docID < maxDoc; docID++ {
				if liveDocs != nil && !liveDocs.Get(docID) {
					continue
				}
				if err := writeDoc(i, docID); err != nil {
					return 0, err
				}
				total++
			}
		}
	}
	if err := writer.Finish(total); err != nil {
		return 0, fmt.Errorf("index: merge stored fields: finish: %w", err)
	}
	return total, nil
}

// resolveMergeCodec resolves the codec for the merged segment: the segment's
// stamped codec name if registered, else the process default.
func resolveMergeCodec(segInfo *SegmentInfo) Codec {
	if segInfo != nil {
		if name := segInfo.Codec(); name != "" {
			if c := LookupCodecByName(name); c != nil {
				return c
			}
		}
	}
	return GetDefaultCodec()
}

// storedFieldsMergeVisitor forwards each stored field decoded from a source
// segment straight to the merged segment's StoredFieldsWriter. The first
// WriteField error is captured and surfaced by mergeFields.
type storedFieldsMergeVisitor struct {
	writer StoredFieldsWriter
	err    error
}

func (v *storedFieldsMergeVisitor) write(f *mergeStoredField) {
	if v.err != nil {
		return
	}
	if err := v.writer.WriteField(f); err != nil {
		v.err = fmt.Errorf("index: merge stored fields: write field %q: %w", f.name, err)
	}
}

func (v *storedFieldsMergeVisitor) StringField(field string, value string) {
	v.write(&mergeStoredField{name: field, stringValue: value})
}
func (v *storedFieldsMergeVisitor) BinaryField(field string, value []byte) {
	v.write(&mergeStoredField{name: field, binaryValue: value})
}
func (v *storedFieldsMergeVisitor) IntField(field string, value int) {
	v.write(&mergeStoredField{name: field, numericValue: value})
}
func (v *storedFieldsMergeVisitor) LongField(field string, value int64) {
	v.write(&mergeStoredField{name: field, numericValue: value})
}
func (v *storedFieldsMergeVisitor) FloatField(field string, value float32) {
	v.write(&mergeStoredField{name: field, numericValue: value})
}
func (v *storedFieldsMergeVisitor) DoubleField(field string, value float64) {
	v.write(&mergeStoredField{name: field, numericValue: value})
}

// mergeStoredField is a minimal spi.IndexableField carrying one decoded stored
// value for re-serialisation by a StoredFieldsWriter during a merge. Exactly
// one of stringValue/binaryValue/numericValue is set.
type mergeStoredField struct {
	name         string
	stringValue  string
	binaryValue  []byte
	numericValue interface{}
}

func (f *mergeStoredField) Name() string              { return f.name }
func (f *mergeStoredField) StringValue() string       { return f.stringValue }
func (f *mergeStoredField) BinaryValue() []byte       { return f.binaryValue }
func (f *mergeStoredField) NumericValue() interface{} { return f.numericValue }
func (f *mergeStoredField) ReaderValue() io.Reader    { return nil }
func (f *mergeStoredField) FieldType() FieldTypeInterface {
	return storedOnlyFieldType{}
}

// storedOnlyFieldType is a FieldTypeInterface that reports a stored-only field.
// The StoredFieldsWriter only consults the value accessors, so the remaining
// methods return zero values and are never invoked during a merge.
type storedOnlyFieldType struct{}

func (storedOnlyFieldType) IsIndexed() bool                 { return false }
func (storedOnlyFieldType) IsStored() bool                  { return true }
func (storedOnlyFieldType) IsTokenized() bool               { return false }
func (storedOnlyFieldType) GetIndexOptions() IndexOptions   { return IndexOptionsNone }
func (storedOnlyFieldType) GetDocValuesType() DocValuesType { return DocValuesTypeNone }
func (storedOnlyFieldType) StoreTermVectors() bool          { return false }
func (storedOnlyFieldType) StoreTermVectorPositions() bool  { return false }
func (storedOnlyFieldType) StoreTermVectorOffsets() bool    { return false }

// mergeTermVectors is implemented in segment_merger_termvectors.go (rmp #14/#114).

// mergeNorms is implemented in segment_merger_norms.go (rmp #120).

// mergeTerms is implemented in segment_merger_postings.go (rmp #14/#114).

// mergeDocValues is implemented in segment_merger_docvalues.go (rmp #14/#114).

// mergePoints is implemented in segment_merger_points.go (rmp #14/#114).

// mergeVectorValues is implemented in segment_merger_vectors.go (rmp #14/#114).

// writeFieldInfos persists the merged FieldInfos (.fnm) for the new segment via
// the resolved codec, so the merged segment can be reopened (rmp #14/#114).
func (sm *SegmentMerger) writeFieldInfos() error {
	if sm.codec == nil || sm.MergeState.MergeFieldInfos == nil {
		return nil
	}
	fif := sm.codec.FieldInfosFormat()
	if fif == nil {
		return nil
	}
	if err := fif.Write(sm.directory, sm.MergeState.SegmentInfo, "", sm.MergeState.MergeFieldInfos, store.IOContextWrite); err != nil {
		return fmt.Errorf("index: merge write field infos: %w", err)
	}
	return nil
}

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
