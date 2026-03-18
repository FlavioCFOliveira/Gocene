// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Codec is an interface for index encoding/decoding.
// This is defined in the index package to avoid import cycles.
// The codecs package provides implementations of this interface.
type Codec interface {
	// Name returns the name of this codec.
	Name() string

	// PostingsFormat returns the postings format for encoding/decoding term postings.
	PostingsFormat() PostingsFormat

	// StoredFieldsFormat returns the stored fields format.
	StoredFieldsFormat() StoredFieldsFormat

	// FieldInfosFormat returns the field infos format.
	FieldInfosFormat() FieldInfosFormat

	// SegmentInfosFormat returns the segment infos format.
	SegmentInfosFormat() SegmentInfosFormat

	// TermVectorsFormat returns the term vectors format.
	TermVectorsFormat() TermVectorsFormat
}

// PostingsFormat is an interface for encoding/decoding term postings.
type PostingsFormat interface {
	// Name returns the name of this format.
	Name() string

	// FieldsConsumer returns a consumer for writing postings.
	FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error)

	// FieldsProducer returns a producer for reading postings.
	FieldsProducer(state *SegmentReadState) (FieldsProducer, error)
}

// FieldsConsumer is an interface for writing postings.
type FieldsConsumer interface {
	// Write writes a field's postings.
	Write(field string, terms Terms) error

	// Close releases resources.
	Close() error
}

// FieldsProducer is an interface for reading postings.
type FieldsProducer interface {
	// Terms returns the terms for a field.
	Terms(field string) (Terms, error)

	// Close releases resources.
	Close() error
}

// StoredFieldsFormat is an interface for encoding/decoding stored fields.
type StoredFieldsFormat interface {
	// Name returns the name of this format.
	Name() string

	// FieldsReader returns a reader for stored fields.
	FieldsReader(dir store.Directory, segmentInfo *SegmentInfo, fieldInfos *FieldInfos, context store.IOContext) (StoredFieldsReader, error)

	// FieldsWriter returns a writer for stored fields.
	FieldsWriter(dir store.Directory, segmentInfo *SegmentInfo, context store.IOContext) (StoredFieldsWriter, error)
}

// StoredFieldsReader is an interface for reading stored fields.
type StoredFieldsReader interface {
	// VisitDocument visits the stored fields for a document.
	VisitDocument(docID int, visitor StoredFieldVisitor) error

	// Close releases resources.
	Close() error
}

// StoredFieldsWriter is an interface for writing stored fields.
type StoredFieldsWriter interface {
	// StartDocument starts writing a document.
	StartDocument() error

	// FinishDocument finishes writing the current document.
	FinishDocument() error

	// WriteField writes a field.
	WriteField(field IndexableField) error

	// Close releases resources.
	Close() error
}

// StoredFieldVisitor is called for each stored field when visiting a document.
type StoredFieldVisitor interface {
	// StringField is called for a stored string field.
	StringField(field string, value string)

	// BinaryField is called for a stored binary field.
	BinaryField(field string, value []byte)

	// IntField is called for a stored int field.
	IntField(field string, value int)

	// LongField is called for a stored long field.
	LongField(field string, value int64)

	// FloatField is called for a stored float field.
	FloatField(field string, value float32)

	// DoubleField is called for a stored double field.
	DoubleField(field string, value float64)
}

// FieldInfosFormat is an interface for encoding/decoding field infos.
type FieldInfosFormat interface {
	// Name returns the name of this format.
	Name() string

	// Read reads field infos from a directory.
	Read(dir store.Directory, segmentInfo *SegmentInfo, context store.IOContext) (*FieldInfos, error)

	// Write writes field infos to a directory.
	Write(dir store.Directory, segmentInfo *SegmentInfo, fieldInfos *FieldInfos, context store.IOContext) error
}

// SegmentInfosFormat is an interface for encoding/decoding segment infos.
type SegmentInfosFormat interface {
	// Name returns the name of this format.
	Name() string

	// Read reads segment infos from a directory.
	Read(dir store.Directory, context store.IOContext) (*SegmentInfos, error)

	// Write writes segment infos to a directory.
	Write(dir store.Directory, segmentInfos *SegmentInfos, context store.IOContext) error
}

// TermVectorsFormat is an interface for encoding/decoding term vectors.
type TermVectorsFormat interface {
	// Name returns the name of this format.
	Name() string

	// VectorsWriter returns a writer for term vectors.
	VectorsWriter(state *SegmentWriteState) (TermVectorsWriter, error)

	// VectorsReader returns a reader for term vectors.
	VectorsReader(dir store.Directory, segmentInfo *SegmentInfo, fieldInfos *FieldInfos, context store.IOContext) (TermVectorsReader, error)
}

// TermVectorsWriter is an interface for writing term vectors.
type TermVectorsWriter interface {
	// StartDocument starts writing term vectors for a document.
	StartDocument(numFields int) error

	// StartField starts writing a term vector for a field.
	StartField(fieldInfo *FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error

	// StartTerm starts a new term in the current field.
	StartTerm(term []byte) error

	// AddPosition adds a position for the current term.
	AddPosition(position int, startOffset, endOffset int, payload []byte) error

	// FinishTerm finishes the current term.
	FinishTerm() error

	// FinishField finishes the current field.
	FinishField() error

	// FinishDocument finishes the current document.
	FinishDocument() error

	// Close releases resources.
	Close() error
}

// TermVectorsReader is an interface for reading term vectors.
type TermVectorsReader interface {
	// Get retrieves term vectors for the given document ID.
	// Returns a Fields object containing the term vectors.
	Get(docID int) (Fields, error)

	// GetField retrieves the term vector for a specific field in a document.
	GetField(docID int, field string) (Terms, error)

	// Close releases resources.
	Close() error
}

// IndexableField is an interface for fields that can be indexed.
// This is defined here to avoid import cycles.
type IndexableField interface {
	// Name returns the name of the field.
	Name() string

	// FieldType returns the field type.
	FieldType() FieldTypeInterface

	// StringValue returns the string value of the field.
	StringValue() string

	// BinaryValue returns the binary value of the field.
	BinaryValue() []byte

	// NumericValue returns the numeric value of the field.
	NumericValue() interface{}
}

// FieldTypeInterface is an interface for field type properties.
// This is defined to avoid import cycles.
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

// SegmentWriteState holds the state for writing a segment.
type SegmentWriteState struct {
	// Directory is where the segment files are written.
	Directory store.Directory

	// SegmentInfo contains metadata about the segment.
	SegmentInfo *SegmentInfo

	// FieldInfos contains metadata about all fields.
	FieldInfos *FieldInfos

	// SegmentSuffix is an optional suffix for segment files.
	SegmentSuffix string
}

// SegmentReadState holds the state for reading a segment.
type SegmentReadState struct {
	// Directory is where the segment files are read from.
	Directory store.Directory

	// SegmentInfo contains metadata about the segment.
	SegmentInfo *SegmentInfo

	// FieldInfos contains metadata about all fields.
	FieldInfos *FieldInfos

	// SegmentSuffix is an optional suffix for segment files.
	SegmentSuffix string
}
