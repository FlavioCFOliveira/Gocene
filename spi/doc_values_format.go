// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// No import needed for schema as it is now part of spi

// DocValuesFormat encodes and decodes per-document column-stride values
// (the .dvd / .dvm pair in the on-disk codec).
//
// Mirrors org.apache.lucene.codecs.DocValuesFormat in Apache
// Lucene 10.4.0. Lifted onto the SPI by rmp #4708 (Sprint 118 phase
// 2d) so that index/ and codecs/ can both reach the canonical
// interface through a single declaration site.
type DocValuesFormat interface {
	// Name returns the format name persisted in segment metadata.
	Name() string

	// FieldsConsumer returns the per-field write side of the doc-values
	// pipeline for the segment described by state. The caller is
	// responsible for closing the returned consumer.
	FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error)

	// FieldsProducer returns the per-field read side of the doc-values
	// pipeline for the segment described by state. The caller is
	// responsible for closing the returned producer.
	FieldsProducer(state *SegmentReadState) (DocValuesProducer, error)
}

// DocValuesConsumer is the abstract API that consumes numeric, binary and
// sorted doc values. Mirrors the abstract members of
// org.apache.lucene.codecs.DocValuesConsumer in Apache Lucene 10.5.0.
//
// The lifecycle is: the consumer is created by
// DocValuesFormat.FieldsConsumer; AddNumericField, AddBinaryField,
// AddSortedField, AddSortedSetField or AddSortedNumericField is called for
// each Numeric, Binary, Sorted, SortedSet or SortedNumeric doc-values field;
// after all fields are added, the consumer is closed. The API is a "pull"
// rather than a "push": every Add*Field receives a DocValuesProducer and the
// implementation is free to obtain the values from it more than once.
//
// The concrete members of the Java abstract class (merge, mergeNumericField,
// mergeBinaryField, mergeSortedField, mergeSortedSetField,
// mergeSortedNumericField and their helpers) take a MergeState, which lives
// in package index; they are carried by codecs.BaseDocValuesConsumer, which
// every concrete consumer embeds.
type DocValuesConsumer interface {
	// AddNumericField writes numeric doc values for a field.
	AddNumericField(field *FieldInfo, valuesProducer DocValuesProducer) error

	// AddBinaryField writes binary doc values for a field.
	AddBinaryField(field *FieldInfo, valuesProducer DocValuesProducer) error

	// AddSortedField writes pre-sorted binary doc values for a field.
	AddSortedField(field *FieldInfo, valuesProducer DocValuesProducer) error

	// AddSortedNumericField writes pre-sorted numeric doc values for a
	// field.
	AddSortedNumericField(field *FieldInfo, valuesProducer DocValuesProducer) error

	// AddSortedSetField writes pre-sorted set doc values for a field.
	AddSortedSetField(field *FieldInfo, valuesProducer DocValuesProducer) error

	// Close releases the consumer's resources (Closeable.close()).
	Close() error
}

// DocValuesProducer is the per-segment read side of the doc-values
// pipeline. Mirrors org.apache.lucene.codecs.DocValuesProducer in
// Apache Lucene 10.4.0.
//
// Each Get* method returns the iterator-shaped value type for the
// requested field, or nil when the field has no values of that kind.
// GetSkipper returns the optional block-skipper companion when the
// codec writes a sparse index alongside the values.
type DocValuesProducer interface {
	// GetNumeric returns a NumericDocValues iterator for the given
	// field, or nil when the field has no numeric values.
	GetNumeric(field *FieldInfo) (NumericDocValues, error)

	// GetBinary returns a BinaryDocValues iterator for the given
	// field, or nil when the field has no binary values.
	GetBinary(field *FieldInfo) (BinaryDocValues, error)

	// GetSorted returns a SortedDocValues iterator for the given
	// field, or nil when the field has no sorted values.
	GetSorted(field *FieldInfo) (SortedDocValues, error)

	// GetSortedSet returns a SortedSetDocValues iterator for the given
	// field, or nil when the field has no sorted-set values.
	GetSortedSet(field *FieldInfo) (SortedSetDocValues, error)

	// GetSortedNumeric returns a SortedNumericDocValues iterator for
	// the given field, or nil when the field has no sorted-numeric
	// values.
	GetSortedNumeric(field *FieldInfo) (SortedNumericDocValues, error)

	// GetSkipper returns the DocValuesSkipper for the given field, or
	// nil when the codec did not write a skipper companion for that
	// field. Mirrors the GetSkipper(FieldInfo) addition in Apache
	// Lucene 10.4.0's DocValuesProducer.
	GetSkipper(field *FieldInfo) (DocValuesSkipper, error)

	// CheckIntegrity walks the per-field data and validates the
	// checksum framing.
	CheckIntegrity() error

	// GetMergeInstance returns an instance optimized for merging. This
	// instance may only be consumed in the thread that called
	// GetMergeInstance.
	//
	// The default implementation returns the receiver itself.
	//
	// Mirrors DocValuesProducer.getMergeInstance() of Apache Lucene 10.5.0.
	GetMergeInstance() DocValuesProducer

	// Close releases the producer's resources.
	Close() error
}
