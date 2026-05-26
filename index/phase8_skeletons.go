// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file collects skeleton ports for the remaining index-package types
// surfaced in Sprint 22 Phase 8. The real implementations for MappedMultiFields,
// MultiPostingsEnum, ReaderManager, SimpleMergedSegmentWarmer, SlowCodecReaderWrapper,
// and SlowImpactsEnum have been promoted to their own files. Only SortingCodecReader
// remains here as a skeleton, deferred to backlog #2708 (SorterPolicy).

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

// --- SortingCodecReader -------------------------------------------------------

// SortingCodecReader wraps a CodecReader and re-orders its docIDs according
// to a SortField list. Mirrors
// org.apache.lucene.index.SortingCodecReader from Apache Lucene 10.4.0.
//
// Gocene skeleton: stores the inputs only; the actual re-order logic lands
// alongside SorterPolicy in backlog #2708 (851-line port requiring SortField,
// SortedDocValues, and the full merge DocMap infrastructure).
type SortingCodecReader struct {
	*CodecReader
}

// NewSortingCodecReader wraps a CodecReader.
func NewSortingCodecReader(in *CodecReader) *SortingCodecReader {
	return &SortingCodecReader{CodecReader: in}
}
