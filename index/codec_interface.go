// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// This file is the index-side facade for the codec SPI after the
// SPI unification (rmp #4669 / Sprint 118 phase 2 / rmp #4693 /
// rmp #4706 / rmp #4707). The canonical declaration site lives in
// spi/; index/ re-exports the types as Go aliases so callers that
// historically reached for index.Codec, index.PostingsFormat,
// index.SegmentWriteState, etc. keep compiling without churn.
//
// Aliasing an interface with `type X = spi.X` makes the index-package
// identifier indistinguishable from its SPI counterpart at the type-
// system level: implementations in codecs/ satisfy index/ interfaces
// without any adapter, and the codecbridge adapters collapse to
// identity wrappers.
//
// DocValuesFormat is a codecs-only accessor on Lucene104Codec and is
// NOT part of the index-side Codec surface; see rmp #4708 for the
// remaining lift.

// Codec is an alias of [spi.Codec]. After rmp #4707 the SPI surface
// covers every method that the index-side Codec contract requires,
// including KnnVectorsFormat.
type Codec = spi.Codec

// KnnVectorsFormat is an alias of [spi.KnnVectorsFormat] — the wide
// canonical KNN vectors format the codec exposes via Codec.KnnVectorsFormat().
type KnnVectorsFormat = spi.KnnVectorsFormat

// KnnVectorsWriter is an alias of [spi.KnnVectorsWriter].
type KnnVectorsWriter = spi.KnnVectorsWriter

// KnnFieldVectorsWriter is an alias of [spi.KnnFieldVectorsWriter] —
// the wide, non-generic per-field write-side contract returned by
// [KnnVectorsWriter.AddField].
type KnnFieldVectorsWriter = spi.KnnFieldVectorsWriter

// PostingsFormat is an alias of spi.PostingsFormat.
type PostingsFormat = spi.PostingsFormat

// FieldsConsumer is an alias of spi.FieldsConsumer.
type FieldsConsumer = spi.FieldsConsumer

// FieldsProducer is an alias of spi.FieldsProducer.
type FieldsProducer = spi.FieldsProducer

// StoredFieldsFormat is an alias of spi.StoredFieldsFormat.
type StoredFieldsFormat = spi.StoredFieldsFormat

// StoredFieldsReader is an alias of spi.StoredFieldsReader.
type StoredFieldsReader = spi.StoredFieldsReader

// StoredFieldsWriter is an alias of spi.StoredFieldsWriter.
type StoredFieldsWriter = spi.StoredFieldsWriter

// StoredFieldVisitor is an alias of spi.StoredFieldVisitor.
type StoredFieldVisitor = spi.StoredFieldVisitor

// FieldInfosFormat is an alias of spi.FieldInfosFormat. The Read/Write
// signatures carry a segmentSuffix string parameter to match the
// codecs-side Lucene-faithful shape.
type FieldInfosFormat = spi.FieldInfosFormat

// SegmentInfoFormat is an alias of spi.SegmentInfoFormat.
type SegmentInfoFormat = spi.SegmentInfoFormat

// SegmentInfosFormat is an alias of spi.SegmentInfosFormat. The plural
// segments_N format was lifted onto the SPI by rmp #4706.
type SegmentInfosFormat = spi.SegmentInfosFormat

// TermVectorsFormat is an alias of spi.TermVectorsFormat.
type TermVectorsFormat = spi.TermVectorsFormat

// TermVectorsWriter is an alias of spi.TermVectorsWriter.
type TermVectorsWriter = spi.TermVectorsWriter

// TermVectorsReader is an alias of spi.TermVectorsReader.
type TermVectorsReader = spi.TermVectorsReader

// CompoundFormat is an alias of spi.CompoundFormat.
type CompoundFormat = spi.CompoundFormat

// CompoundDirectory is an alias of spi.CompoundDirectory.
type CompoundDirectory = spi.CompoundDirectory

// IndexableField is an alias of spi.IndexableField — the narrow,
// codec-facing contract that the stored-fields write path consumes.
//
// The document-facing IndexableField (with the wider Lucene 10.4.0 API
// including FieldType / ReaderValue / TokenStream) lives in package
// document and is a structural superset of this interface.
type IndexableField = spi.IndexableField

// SegmentWriteState is an alias of spi.SegmentWriteState. The
// SegUpdates field carries the spi.BufferedUpdatesRef marker
// interface; callers in this package type-assert to *BufferedUpdates
// when they need the structured data.
type SegmentWriteState = spi.SegmentWriteState

// SegmentReadState is an alias of spi.SegmentReadState.
type SegmentReadState = spi.SegmentReadState

// -----------------------------------------------------------------------------
// Legacy index-only interfaces that the SPI does not yet cover.
// -----------------------------------------------------------------------------

// FieldTypeInterface is the index-package projection of the document
// FieldType properties that legacy index-side stored-fields paths still
// consume. It pre-dates the SPI unification and is retained for any
// remaining callers that reach for FieldType() on an index.IndexableField
// implementation defined here (rather than via the document package).
type FieldTypeInterface interface {
	// IsIndexed returns whether the field is indexed.
	IsIndexed() bool

	// IsStored returns whether the field is stored.
	IsStored() bool

	// IsTokenized returns whether the field is tokenized.
	IsTokenized() bool

	// GetIndexOptions returns the indexing options.
	GetIndexOptions() IndexOptions

	// GetDocValuesType returns the doc values type.
	GetDocValuesType() DocValuesType

	// StoreTermVectors returns whether term vectors are stored.
	StoreTermVectors() bool

	// StoreTermVectorPositions returns whether term vector positions are stored.
	StoreTermVectorPositions() bool

	// StoreTermVectorOffsets returns whether term vector offsets are stored.
	StoreTermVectorOffsets() bool
}
