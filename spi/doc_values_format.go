// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import "github.com/FlavioCFOliveira/Gocene/schema"

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

// DocValuesConsumer is the per-segment write side of the doc-values
// pipeline. Mirrors org.apache.lucene.codecs.DocValuesConsumer in
// Apache Lucene 10.4.0.
//
// The flush path feeds each Add*Field call with a writer-side
// iterator over the in-memory accumulator's contents; the consumer
// serializes the values to the segment's .dvd / .dvm files.
type DocValuesConsumer interface {
	// AddNumericField persists a numeric doc-values field.
	AddNumericField(field *schema.FieldInfo, values NumericDocValuesIterator) error

	// AddBinaryField persists a binary doc-values field.
	AddBinaryField(field *schema.FieldInfo, values BinaryDocValuesIterator) error

	// AddSortedField persists a sorted doc-values field.
	AddSortedField(field *schema.FieldInfo, values SortedDocValuesIterator) error

	// AddSortedSetField persists a sorted-set doc-values field.
	AddSortedSetField(field *schema.FieldInfo, values SortedSetDocValuesIterator) error

	// AddSortedNumericField persists a sorted-numeric doc-values field.
	AddSortedNumericField(field *schema.FieldInfo, values SortedNumericDocValuesIterator) error

	// Close flushes any pending bytes and releases the consumer's
	// resources.
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
	GetNumeric(field *schema.FieldInfo) (NumericDocValues, error)

	// GetBinary returns a BinaryDocValues iterator for the given
	// field, or nil when the field has no binary values.
	GetBinary(field *schema.FieldInfo) (BinaryDocValues, error)

	// GetSorted returns a SortedDocValues iterator for the given
	// field, or nil when the field has no sorted values.
	GetSorted(field *schema.FieldInfo) (SortedDocValues, error)

	// GetSortedSet returns a SortedSetDocValues iterator for the given
	// field, or nil when the field has no sorted-set values.
	GetSortedSet(field *schema.FieldInfo) (SortedSetDocValues, error)

	// GetSortedNumeric returns a SortedNumericDocValues iterator for
	// the given field, or nil when the field has no sorted-numeric
	// values.
	GetSortedNumeric(field *schema.FieldInfo) (SortedNumericDocValues, error)

	// GetSkipper returns the DocValuesSkipper for the given field, or
	// nil when the codec did not write a skipper companion for that
	// field. Mirrors the GetSkipper(FieldInfo) addition in Apache
	// Lucene 10.4.0's DocValuesProducer.
	GetSkipper(field *schema.FieldInfo) (DocValuesSkipper, error)

	// CheckIntegrity walks the per-field data and validates the
	// checksum framing.
	CheckIntegrity() error

	// Close releases the producer's resources.
	Close() error
}
