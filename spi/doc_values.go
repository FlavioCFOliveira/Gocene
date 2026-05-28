// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// This file declares the canonical doc-values value-type contracts as
// they appear on the codecs-facing surface of Apache Lucene 10.4.0.
//
// Lifted onto the SPI by rmp #4708 (Sprint 118 phase 2d) together with
// DocValuesFormat / DocValuesProducer / DocValuesConsumer. The shapes
// mirror the iterator surface returned by codec producers: each value
// type advances through the documents that hold a value via
// NextDoc/Advance and reports the current value via LongValue,
// BinaryValue, OrdValue, NextOrd, etc.
//
// The index/ package still keeps a divergent random-access "Get(docID)"
// projection of the same five value types. That divergence is tracked
// by rmp #4709 and is out of scope for this lift; the index-side bodies
// carry a TODO(T4709) marker at their declaration site until that task
// migrates index callers onto this iterator-shaped surface.

// NumericDocValues provides per-document numeric values.
//
// Mirrors org.apache.lucene.index.NumericDocValues in Apache
// Lucene 10.4.0. Callers walk the value-bearing documents with
// NextDoc/Advance and read the current value via LongValue.
type NumericDocValues interface {
	// DocID returns the current document ID, or -1 before NextDoc /
	// Advance has been called, or NO_MORE_DOCS once the iterator is
	// exhausted.
	DocID() int

	// NextDoc advances to the next document that has a value and
	// returns its ID, or NO_MORE_DOCS when no document follows.
	NextDoc() (int, error)

	// Advance positions the iterator on the first document with a value
	// whose ID is >= target and returns that ID, or NO_MORE_DOCS when
	// no such document exists.
	Advance(target int) (int, error)

	// LongValue returns the numeric value for the current document.
	LongValue() (int64, error)

	// Cost is an estimate of the cost of iterating over the entire
	// value-bearing document set.
	Cost() int64
}

// BinaryDocValues provides per-document binary values.
//
// Mirrors org.apache.lucene.index.BinaryDocValues in Apache
// Lucene 10.4.0.
type BinaryDocValues interface {
	// DocID returns the current document ID, or -1 before NextDoc /
	// Advance has been called, or NO_MORE_DOCS once the iterator is
	// exhausted.
	DocID() int

	// NextDoc advances to the next document that has a value and
	// returns its ID, or NO_MORE_DOCS when no document follows.
	NextDoc() (int, error)

	// Advance positions the iterator on the first document with a value
	// whose ID is >= target and returns that ID, or NO_MORE_DOCS when
	// no such document exists.
	Advance(target int) (int, error)

	// BinaryValue returns the binary value bound to the current
	// document. The returned slice is only valid until the next call
	// that mutates the iterator state.
	BinaryValue() ([]byte, error)

	// Cost is an estimate of the cost of iterating over the entire
	// value-bearing document set.
	Cost() int64
}

// SortedDocValues provides per-document sorted binary values addressed
// by ordinal.
//
// Mirrors org.apache.lucene.index.SortedDocValues in Apache
// Lucene 10.4.0. The Lucene class embeds the NumericDocValues iterator
// shape so that ordinal-bearing documents can be walked the same way
// numeric ones are; this port preserves that embedding.
type SortedDocValues interface {
	NumericDocValues

	// OrdValue returns the ordinal of the current document's value.
	OrdValue() (int, error)

	// LookupOrd returns the value bound to the given ordinal. The
	// returned slice is only valid until the next call that mutates the
	// underlying ord-to-bytes lookup buffer.
	LookupOrd(ord int) ([]byte, error)

	// GetValueCount returns the number of unique ordinals stored for
	// this field.
	GetValueCount() int
}

// SortedSetDocValues provides per-document multi-valued ordinals.
//
// Mirrors org.apache.lucene.index.SortedSetDocValues in Apache
// Lucene 10.4.0. Each value-bearing document exposes one or more
// ordinals through NextOrd; -1 signals the end of the document's
// ordinal stream.
type SortedSetDocValues interface {
	// DocID returns the current document ID, or -1 before NextDoc /
	// Advance has been called, or NO_MORE_DOCS once the iterator is
	// exhausted.
	DocID() int

	// NextDoc advances to the next document that has at least one
	// value and returns its ID, or NO_MORE_DOCS when no document
	// follows.
	NextDoc() (int, error)

	// Advance positions the iterator on the first document with at
	// least one value whose ID is >= target and returns that ID, or
	// NO_MORE_DOCS when no such document exists.
	Advance(target int) (int, error)

	// NextOrd returns the next ordinal for the current document, or -1
	// when the document has no more ordinals.
	NextOrd() (int, error)

	// LookupOrd returns the value bound to the given ordinal.
	LookupOrd(ord int) ([]byte, error)

	// GetValueCount returns the number of unique ordinals stored for
	// this field.
	GetValueCount() int

	// Cost is an estimate of the cost of iterating over the entire
	// value-bearing document set.
	Cost() int64
}

// SortedNumericDocValues provides per-document multi-valued numeric
// values.
//
// Mirrors org.apache.lucene.index.SortedNumericDocValues in Apache
// Lucene 10.4.0. Each value-bearing document exposes one or more
// values through NextValue; DocValueCount reports how many values the
// current document holds.
type SortedNumericDocValues interface {
	NumericDocValues

	// NextValue returns the next numeric value for the current
	// document.
	NextValue() (int64, error)

	// DocValueCount returns the number of values bound to the current
	// document.
	DocValueCount() (int, error)
}

// DocValuesSkipper allows codecs to skip over runs of documents that
// cannot match a query based on the per-block min/max value summary
// the doc-values format writes alongside the values themselves.
//
// Mirrors org.apache.lucene.index.DocValuesSkipper in Apache
// Lucene 10.4.0. The Gocene port keeps the surface minimal: the
// concrete sparse-skipper implementations live in the codecs package
// and may extend this contract with format-specific helpers.
type DocValuesSkipper interface {
	// SkipTo advances the skipper so the current block covers a
	// document ID >= target, and returns the first such ID, or
	// NO_MORE_DOCS once no further block applies.
	SkipTo(target int) (int, error)

	// GetDocID returns the current document ID, or -1 before SkipTo
	// has been called, or NO_MORE_DOCS once the skipper is exhausted.
	GetDocID() int
}
