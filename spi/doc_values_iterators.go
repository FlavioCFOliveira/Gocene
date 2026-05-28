// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// This file declares the writer-side iterator contracts that
// DocValuesConsumer.AddXxxField accepts.
//
// They have no direct Java counterpart in Apache Lucene 10.4.0 — Lucene
// uses the read-side iterator types (NumericDocValues etc.) on both
// sides of the flush path. The Gocene port models the writer side with
// dedicated, allocation-friendly iterators because:
//
//   - the write path knows the data in memory and does not need the
//     full "may return an error" surface a producer iterator carries;
//   - keeping a separate type makes it cheap for the index-side
//     in-memory accumulators (NumericDocValuesWriter,
//     SortedDocValuesWriter, …) to expose a flush iterator without
//     dragging in the codec-facing value-type contract.
//
// Lifted onto the SPI by rmp #4708 so the codec-facing
// DocValuesConsumer surface can also live on the SPI.

// NumericDocValuesIterator is the writer-side iterator that the flush
// path feeds into DocValuesConsumer.AddNumericField.
type NumericDocValuesIterator interface {
	// Next advances to the next document and reports whether one
	// exists.
	Next() bool

	// DocID returns the current document ID.
	DocID() int

	// Value returns the numeric value for the current document.
	Value() int64
}

// BinaryDocValuesIterator is the writer-side iterator that the flush
// path feeds into DocValuesConsumer.AddBinaryField.
type BinaryDocValuesIterator interface {
	// Next advances to the next document and reports whether one
	// exists.
	Next() bool

	// DocID returns the current document ID.
	DocID() int

	// Value returns the binary value for the current document. The
	// returned slice is only valid until the next call that mutates the
	// iterator state.
	Value() []byte
}

// SortedDocValuesIterator is the writer-side iterator that the flush
// path feeds into DocValuesConsumer.AddSortedField.
type SortedDocValuesIterator interface {
	// Next advances to the next document and reports whether one
	// exists.
	Next() bool

	// DocID returns the current document ID.
	DocID() int

	// Ord returns the ordinal of the current document's value.
	Ord() int
}

// SortedSetDocValuesIterator is the writer-side iterator that the
// flush path feeds into DocValuesConsumer.AddSortedSetField.
type SortedSetDocValuesIterator interface {
	// NextDoc advances to the next document and reports whether one
	// exists.
	NextDoc() bool

	// DocID returns the current document ID.
	DocID() int

	// NextOrd returns the next ordinal for the current document, or -1
	// when the document has no more ordinals.
	NextOrd() int
}

// SortedNumericDocValuesIterator is the writer-side iterator that the
// flush path feeds into DocValuesConsumer.AddSortedNumericField.
type SortedNumericDocValuesIterator interface {
	// NextDoc advances to the next document and reports whether one
	// exists.
	NextDoc() bool

	// DocID returns the current document ID.
	DocID() int

	// NextValue returns the next value for the current document.
	NextValue() int64

	// DocValueCount returns the number of values bound to the current
	// document.
	DocValueCount() int
}
