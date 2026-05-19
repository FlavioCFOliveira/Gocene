// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file collects skeleton ports for the remaining index-package types
// surfaced in Sprint 22 Phase 8. Each type's full Lucene behaviour is
// deferred to backlog #2707 (merge pipeline) or #2710 (multi-reader
// aggregation). The skeletons expose the construction API and the field
// shape so call-sites can declare typed dependencies today.

// --- MappedMultiFields ------------------------------------------------------

// MappedMultiFields wraps a MultiFields and applies a MergeState.DocMap
// chain so that consumers see merge-time docIDs. Mirrors
// org.apache.lucene.index.MappedMultiFields from Apache Lucene 10.4.0.
//
// Gocene skeleton: stores the inputs only — Iterator/Terms forwarding is
// deferred to backlog #2710.
type MappedMultiFields struct {
	MergeState *MergeState
	Multi      *MultiFields
}

// NewMappedMultiFields builds a MappedMultiFields wrapper.
func NewMappedMultiFields(ms *MergeState, multi *MultiFields) *MappedMultiFields {
	return &MappedMultiFields{MergeState: ms, Multi: multi}
}

// --- MultiLeafReader ---------------------------------------------------------

// MultiLeafReader is the alias type Lucene uses internally to describe a
// LeafReader that aggregates several other LeafReaders (e.g. via
// ParallelLeafReader composition). Gocene uses a marker interface so callers
// can express the constraint without re-implementing LeafReader.
type MultiLeafReader interface {
	LeafReaderInterface

	// GetParallelReaders returns the underlying LeafReaders.
	GetParallelReaders() []*LeafReader
}

// --- MultiPostingsEnum -------------------------------------------------------

// MultiPostingsEnum is the priority-queue-based merge of several PostingsEnum
// instances. Mirrors org.apache.lucene.index.MultiPostingsEnum from Apache
// Lucene 10.4.0.
//
// Gocene skeleton: stores the sub-enumerators only — the actual round-robin
// advance is deferred to backlog #2710 alongside MultiTermsEnum. The
// EnumWithSlice payload is wired so MappingMultiPostingsEnum.Reset can
// rebuild its per-sub-reader index already.
type MultiPostingsEnum struct {
	Subs          []PostingsEnum
	SubsWithSlice []EnumWithSlice
	NumSubs       int
}

// NewMultiPostingsEnum constructs a MultiPostingsEnum over subs.
func NewMultiPostingsEnum(subs []PostingsEnum) *MultiPostingsEnum {
	return &MultiPostingsEnum{Subs: subs}
}

// EnumWithSlice pairs a PostingsEnum with the ReaderSlice describing how
// the sub-reader fits into the composite reader. Mirrors
// org.apache.lucene.index.MultiPostingsEnum.EnumWithSlice from Apache
// Lucene 10.4.0.
type EnumWithSlice struct {
	PostingsEnum PostingsEnum
	Slice        ReaderSlice
}

// GetSubs returns the per-sub-reader (PostingsEnum, ReaderSlice) pairs
// that back this MultiPostingsEnum. Mirrors getSubs() in Lucene.
func (m *MultiPostingsEnum) GetSubs() []EnumWithSlice { return m.SubsWithSlice }

// GetNumSubs returns the number of active sub-enumerators. Mirrors
// getNumSubs() in Lucene.
func (m *MultiPostingsEnum) GetNumSubs() int { return m.NumSubs }

// --- ReaderManager -----------------------------------------------------------

// ReaderManager refreshes a single shared DirectoryReader, exposing the
// current view via Acquire/Release. Mirrors
// org.apache.lucene.index.ReaderManager from Apache Lucene 10.4.0.
//
// Gocene skeleton: stores the underlying reader; concurrency-safe Acquire/
// Release (with reference counting) is deferred to backlog #2707.
type ReaderManager struct {
	Current *DirectoryReader
}

// NewReaderManager wraps a DirectoryReader.
func NewReaderManager(reader *DirectoryReader) *ReaderManager {
	return &ReaderManager{Current: reader}
}

// --- SimpleMergedSegmentWarmer ------------------------------------------------

// SimpleMergedSegmentWarmer is a no-op MergedSegmentWarmer that simply
// touches the merged segment so its files are paged in. Mirrors
// org.apache.lucene.index.SimpleMergedSegmentWarmer from Apache Lucene 10.4.0.
type SimpleMergedSegmentWarmer struct{}

// NewSimpleMergedSegmentWarmer returns the canonical no-op warmer.
func NewSimpleMergedSegmentWarmer() *SimpleMergedSegmentWarmer {
	return &SimpleMergedSegmentWarmer{}
}

// Warm is a no-op (full implementation deferred to backlog #2707).
func (SimpleMergedSegmentWarmer) Warm(_ *LeafReader) error { return nil }

// --- SlowCodecReaderWrapper ---------------------------------------------------

// SlowCodecReaderWrapper adapts an arbitrary LeafReader to the CodecReader
// surface by copying values through. Mirrors
// org.apache.lucene.index.SlowCodecReaderWrapper from Apache Lucene 10.4.0.
//
// Gocene skeleton: wraps a LeafReader; full per-field codec-reader exposure
// (PostingsReader/StoredFieldsReader/TermVectorsReader/NormsProducer/...)
// lands once codec ports are integrated.
type SlowCodecReaderWrapper struct {
	*LeafReader
}

// NewSlowCodecReaderWrapper wraps a LeafReader.
func NewSlowCodecReaderWrapper(in *LeafReader) *SlowCodecReaderWrapper {
	return &SlowCodecReaderWrapper{LeafReader: in}
}

// --- SlowImpactsEnum ----------------------------------------------------------

// SlowImpactsEnum wraps a PostingsEnum and returns trivial Impacts (constant
// freq=1, norm=1) for every doc range. Mirrors
// org.apache.lucene.index.SlowImpactsEnum from Apache Lucene 10.4.0.
//
// Gocene skeleton: full implementation lives next to a concrete ImpactsEnum
// once SimScorer integration lands; until then a degenerate stub is exposed.
type SlowImpactsEnum struct {
	PostingsEnum
}

// NewSlowImpactsEnum wraps a PostingsEnum.
func NewSlowImpactsEnum(in PostingsEnum) *SlowImpactsEnum {
	return &SlowImpactsEnum{PostingsEnum: in}
}

// AdvanceShallow is a no-op for the slow degenerate variant.
func (SlowImpactsEnum) AdvanceShallow(_ int) error { return nil }

// GetImpacts returns a single-level Impacts with freq=1, norm=1 spanning
// the whole remaining doc range.
func (SlowImpactsEnum) GetImpacts() (Impacts, error) {
	return slowSingleImpact{}, nil
}

type slowSingleImpact struct{}

func (slowSingleImpact) NumLevels() int         { return 1 }
func (slowSingleImpact) GetDocIDUpTo(_ int) int { return NO_MORE_DOCS }
func (slowSingleImpact) GetImpacts(_ int) *FreqAndNormBuffer {
	b := NewFreqAndNormBuffer()
	b.Add(1, 1)
	return b
}

// --- SortingCodecReader -------------------------------------------------------

// SortingCodecReader wraps a CodecReader and re-orders its docIDs according
// to a SortField list. Mirrors
// org.apache.lucene.index.SortingCodecReader from Apache Lucene 10.4.0.
//
// Gocene skeleton: stores the inputs only; the actual re-order logic lands
// alongside SorterPolicy in backlog #2708.
type SortingCodecReader struct {
	*CodecReader
}

// NewSortingCodecReader wraps a CodecReader.
func NewSortingCodecReader(in *CodecReader) *SortingCodecReader {
	return &SortingCodecReader{CodecReader: in}
}
