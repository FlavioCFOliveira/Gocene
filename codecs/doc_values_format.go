// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// DocValuesFormat handles encoding/decoding of per-document values.
// This is the Go port of Lucene's org.apache.lucene.codecs.DocValuesFormat.
//
// DocValues are stored in a columnar format for efficient sorting, faceting,
// and value retrieval. They are stored separately from the inverted index
// and are used for operations that need to access field values for all
// documents in a segment.
type DocValuesFormat interface {
	// Name returns the name of this format.
	Name() string

	// FieldsConsumer returns a consumer for writing doc values.
	// The caller should close the returned consumer when done.
	FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error)

	// FieldsProducer returns a producer for reading doc values.
	// The caller should close the returned producer when done.
	FieldsProducer(state *SegmentReadState) (DocValuesProducer, error)
}

// BaseDocValuesFormat provides common functionality for DocValuesFormat implementations.
type BaseDocValuesFormat struct {
	name string
}

// NewBaseDocValuesFormat creates a new BaseDocValuesFormat.
func NewBaseDocValuesFormat(name string) *BaseDocValuesFormat {
	return &BaseDocValuesFormat{name: name}
}

// Name returns the format name.
func (f *BaseDocValuesFormat) Name() string {
	return f.name
}

// FieldsConsumer returns a fields consumer (must be implemented by subclasses).
func (f *BaseDocValuesFormat) FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error) {
	return nil, fmt.Errorf("FieldsConsumer not implemented")
}

// FieldsProducer returns a fields producer (must be implemented by subclasses).
func (f *BaseDocValuesFormat) FieldsProducer(state *SegmentReadState) (DocValuesProducer, error) {
	return nil, fmt.Errorf("FieldsProducer not implemented")
}

// DocValuesConsumer is a consumer for writing doc values.
// This is the Go port of Lucene's org.apache.lucene.codecs.DocValuesConsumer.
type DocValuesConsumer interface {
	// AddNumericField writes a numeric doc values field.
	// The values are provided through the iterator.
	AddNumericField(field *index.FieldInfo, values NumericDocValuesIterator) error

	// AddBinaryField writes a binary doc values field.
	// The values are provided through the iterator.
	AddBinaryField(field *index.FieldInfo, values BinaryDocValuesIterator) error

	// AddSortedField writes a sorted doc values field.
	// The values are provided through the iterator.
	AddSortedField(field *index.FieldInfo, values SortedDocValuesIterator) error

	// AddSortedSetField writes a sorted set doc values field.
	// The values are provided through the iterator.
	AddSortedSetField(field *index.FieldInfo, values SortedSetDocValuesIterator) error

	// AddSortedNumericField writes a sorted numeric doc values field.
	// The values are provided through the iterator.
	AddSortedNumericField(field *index.FieldInfo, values SortedNumericDocValuesIterator) error

	// Close releases resources.
	Close() error
}

// DocValuesProducer is a producer for reading doc values.
// This is the Go port of Lucene's org.apache.lucene.codecs.DocValuesProducer.
type DocValuesProducer interface {
	// GetNumeric returns a NumericDocValues for the given field.
	// Returns nil if the field has no numeric doc values.
	GetNumeric(field *index.FieldInfo) (NumericDocValues, error)

	// GetBinary returns a BinaryDocValues for the given field.
	// Returns nil if the field has no binary doc values.
	GetBinary(field *index.FieldInfo) (BinaryDocValues, error)

	// GetSorted returns a SortedDocValues for the given field.
	// Returns nil if the field has no sorted doc values.
	GetSorted(field *index.FieldInfo) (SortedDocValues, error)

	// GetSortedSet returns a SortedSetDocValues for the given field.
	// Returns nil if the field has no sorted set doc values.
	GetSortedSet(field *index.FieldInfo) (SortedSetDocValues, error)

	// GetSortedNumeric returns a SortedNumericDocValues for the given field.
	// Returns nil if the field has no sorted numeric doc values.
	GetSortedNumeric(field *index.FieldInfo) (SortedNumericDocValues, error)

	// CheckIntegrity checks the integrity of the doc values.
	CheckIntegrity() error

	// Close releases resources.
	Close() error
}

// NumericDocValues provides per-document numeric values.
// This is the Go port of Lucene's org.apache.lucene.index.NumericDocValues.
type NumericDocValues interface {
	// DocID returns the current document ID.
	DocID() int

	// NextDoc advances to the next document that has a value.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// Advance advances to the first document >= target that has a value.
	// Returns NO_MORE_DOCS if there are no more documents.
	Advance(target int) (int, error)

	// LongValue returns the current document's value.
	LongValue() (int64, error)

	// Cost returns an estimate of the cost of iterating through all documents.
	Cost() int64
}

// BinaryDocValues provides per-document binary values.
// This is the Go port of Lucene's org.apache.lucene.index.BinaryDocValues.
type BinaryDocValues interface {
	// DocID returns the current document ID.
	DocID() int

	// NextDoc advances to the next document that has a value.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// Advance advances to the first document >= target that has a value.
	// Returns NO_MORE_DOCS if there are no more documents.
	Advance(target int) (int, error)

	// BinaryValue returns the current document's value.
	BinaryValue() ([]byte, error)

	// Cost returns an estimate of the cost of iterating through all documents.
	Cost() int64
}

// SortedDocValues provides per-document sorted binary values.
// This is the Go port of Lucene's org.apache.lucene.index.SortedDocValues.
type SortedDocValues interface {
	NumericDocValues

	// OrdValue returns the ordinal of the current document's value.
	OrdValue() (int, error)

	// LookupOrd returns the value for the given ordinal.
	LookupOrd(ord int) ([]byte, error)

	// GetValueCount returns the number of unique values.
	GetValueCount() int
}

// SortedSetDocValues provides per-document sorted set binary values.
// This is the Go port of Lucene's org.apache.lucene.index.SortedSetDocValues.
type SortedSetDocValues interface {
	// DocID returns the current document ID.
	DocID() int

	// NextDoc advances to the next document that has values.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// Advance advances to the first document >= target that has values.
	// Returns NO_MORE_DOCS if there are no more documents.
	Advance(target int) (int, error)

	// NextOrd advances to the next ordinal for the current document.
	// Returns -1 if there are no more ordinals for this document.
	NextOrd() (int, error)

	// LookupOrd returns the value for the given ordinal.
	LookupOrd(ord int) ([]byte, error)

	// GetValueCount returns the number of unique values.
	GetValueCount() int

	// Cost returns an estimate of the cost of iterating through all documents.
	Cost() int64
}

// SortedNumericDocValues provides per-document sorted numeric values.
// This is the Go port of Lucene's org.apache.lucene.index.SortedNumericDocValues.
type SortedNumericDocValues interface {
	NumericDocValues

	// NextValue advances to the next value for the current document.
	// Returns the value or an error.
	NextValue() (int64, error)

	// DocValueCount returns the number of values for the current document.
	DocValueCount() (int, error)
}

// NumericDocValuesIterator is an iterator over numeric doc values for writing.
type NumericDocValuesIterator interface {
	// Next advances to the next document.
	// Returns true if there is a next document.
	Next() bool

	// DocID returns the current document ID.
	DocID() int

	// Value returns the current document's value.
	Value() int64
}

// BinaryDocValuesIterator is an iterator over binary doc values for writing.
type BinaryDocValuesIterator interface {
	// Next advances to the next document.
	// Returns true if there is a next document.
	Next() bool

	// DocID returns the current document ID.
	DocID() int

	// Value returns the current document's value.
	Value() []byte
}

// SortedDocValuesIterator is an iterator over sorted doc values for writing.
type SortedDocValuesIterator interface {
	// Next advances to the next document.
	// Returns true if there is a next document.
	Next() bool

	// DocID returns the current document ID.
	DocID() int

	// Ord returns the current document's ordinal value.
	Ord() int
}

// SortedSetDocValuesIterator is an iterator over sorted set doc values for writing.
type SortedSetDocValuesIterator interface {
	// NextDoc advances to the next document.
	// Returns true if there is a next document.
	NextDoc() bool

	// DocID returns the current document ID.
	DocID() int

	// NextOrd advances to the next ordinal for the current document.
	// Returns -1 if there are no more ordinals for this document.
	NextOrd() int
}

// SortedNumericDocValuesIterator is an iterator over sorted numeric doc values for writing.
type SortedNumericDocValuesIterator interface {
	// NextDoc advances to the next document.
	// Returns true if there is a next document.
	NextDoc() bool

	// DocID returns the current document ID.
	DocID() int

	// NextValue advances to the next value for the current document.
	// Returns the value.
	NextValue() int64

	// DocValueCount returns the number of values for the current document.
	DocValueCount() int
}

// DocValuesWriter is a helper for writing doc values.
type DocValuesWriter struct {
	out    store.IndexOutput
	closed bool
}

// NewDocValuesWriter creates a new DocValuesWriter.
func NewDocValuesWriter(out store.IndexOutput) *DocValuesWriter {
	return &DocValuesWriter{out: out}
}

// WriteHeader writes the doc values file header.
func (w *DocValuesWriter) WriteHeader() error {
	// Write magic number
	if err := store.WriteUint32(w.out, 0x44564C00); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(w.out, 1); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// Close closes the writer.
func (w *DocValuesWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.out.Close()
}

// DocValuesReader is a helper for reading doc values.
type DocValuesReader struct {
	in     store.IndexInput
	closed bool
}

// NewDocValuesReader creates a new DocValuesReader.
func NewDocValuesReader(in store.IndexInput) *DocValuesReader {
	return &DocValuesReader{in: in}
}

// ReadHeader reads and validates the doc values file header.
func (r *DocValuesReader) ReadHeader() error {
	// Read magic number
	magic, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x44564C00 {
		return fmt.Errorf("invalid magic number: expected 0x44564C00, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// Close closes the reader.
func (r *DocValuesReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.in.Close()
}
