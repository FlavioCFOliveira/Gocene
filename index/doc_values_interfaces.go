// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// TODO(T4709): The five doc-values value-type interfaces declared in
// this file (NumericDocValues, BinaryDocValues, SortedDocValues,
// SortedSetDocValues, SortedNumericDocValues) plus the DocValuesSkipper
// helper carry the legacy random-access "Get(docID)" projection.
//
// rmp #4708 lifted the canonical iterator-shaped surface
// (NextDoc/Advance/LongValue/...) onto package spi and aliased it from
// codecs/, but left these bodies in place because every index-side
// caller still consumes the random-access shape. rmp #4709 migrates
// the index callers onto the SPI iterator surface and then collapses
// these declarations to aliases of spi.NumericDocValues etc.
//
// Until that task lands, this file is intentionally divergent from
// the codecs / spi value-type surface.

// NumericDocValues provides an iterator over numeric doc values.
// This is the Go port of Lucene's org.apache.lucene.index.NumericDocValues.
//
// TODO(T4710): rmp #4710 will drop Get(docID) and collapse this body to
// a type alias of spi.NumericDocValues. T4709 added AdvanceExact and
// LongValue alongside Get to enable callers to migrate without
// breaking existing call sites.
type NumericDocValues interface {
	// Get returns the numeric value for the given document.
	// Returns 0 if the document has no value for this field.
	Get(docID int) (int64, error)

	// Advance advances to the given document.
	// Returns true if the document has a value, false otherwise.
	Advance(target int) (int, error)

	// AdvanceExact positions the iterator on the given target document
	// and returns true if that document has a value. Callers MUST
	// advance monotonically (target >= previous target).
	//
	// Mirrors org.apache.lucene.index.NumericDocValues#advanceExact.
	AdvanceExact(target int) (bool, error)

	// LongValue returns the numeric value for the current document
	// position. Must be called after a successful AdvanceExact / NextDoc
	// / Advance.
	//
	// Mirrors org.apache.lucene.index.NumericDocValues#longValue.
	LongValue() (int64, error)

	// NextDoc returns the next document that has a value.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// DocID returns the current document ID.
	DocID() int
}

// BinaryDocValues provides an iterator over binary doc values.
// This is the Go port of Lucene's org.apache.lucene.index.BinaryDocValues.
//
// TODO(T4710): rmp #4710 will drop Get(docID) and collapse this body to
// a type alias of spi.BinaryDocValues. T4709 added AdvanceExact and
// BinaryValue alongside Get to enable callers to migrate without
// breaking existing call sites.
type BinaryDocValues interface {
	// Get returns the binary value for the given document.
	// Returns nil if the document has no value for this field.
	Get(docID int) ([]byte, error)

	// Advance advances to the given document.
	// Returns the document ID or NO_MORE_DOCS.
	Advance(target int) (int, error)

	// AdvanceExact positions the iterator on the given target document
	// and returns true if that document has a value. Callers MUST
	// advance monotonically (target >= previous target).
	//
	// Mirrors org.apache.lucene.index.BinaryDocValues#advanceExact.
	AdvanceExact(target int) (bool, error)

	// BinaryValue returns the binary value bound to the current
	// document position. Must be called after a successful AdvanceExact
	// / NextDoc / Advance.
	//
	// Mirrors org.apache.lucene.index.BinaryDocValues#binaryValue.
	BinaryValue() ([]byte, error)

	// NextDoc returns the next document that has a value.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// DocID returns the current document ID.
	DocID() int
}

// SortedDocValues provides an iterator over sorted doc values.
// This is the Go port of Lucene's org.apache.lucene.index.SortedDocValues.
//
// TODO(T4710): rmp #4710 will drop GetOrd(docID) and collapse this body
// to a type alias of spi.SortedDocValues. T4709 added OrdValue (current
// position ord) alongside GetOrd. AdvanceExact and BinaryValue come in
// via the embedded BinaryDocValues.
type SortedDocValues interface {
	BinaryDocValues

	// GetOrd returns the ordinal for the given document.
	// Returns -1 if the document has no value for this field.
	GetOrd(docID int) (int, error)

	// OrdValue returns the ordinal for the current document position.
	// Must be called after a successful AdvanceExact / NextDoc /
	// Advance.
	//
	// Mirrors org.apache.lucene.index.SortedDocValues#ordValue.
	OrdValue() (int, error)

	// LookupOrd returns the value for the given ordinal.
	LookupOrd(ord int) ([]byte, error)

	// GetValueCount returns the number of unique values.
	GetValueCount() int
}

// SortedNumericDocValues provides an iterator over sorted numeric doc values.
// This is the Go port of Lucene's org.apache.lucene.index.SortedNumericDocValues.
//
// TODO(T4710): rmp #4710 will drop Get(docID) and collapse this body to
// a type alias of spi.SortedNumericDocValues. T4709 added AdvanceExact,
// NextValue, and DocValueCount alongside Get.
type SortedNumericDocValues interface {
	// Get returns the numeric values for the given document.
	// Returns an empty slice if the document has no values for this field.
	Get(docID int) ([]int64, error)

	// Advance advances to the given document.
	// Returns the document ID or NO_MORE_DOCS.
	Advance(target int) (int, error)

	// AdvanceExact positions the iterator on the given target document
	// and returns true if that document has at least one value. Callers
	// MUST advance monotonically (target >= previous target).
	//
	// Mirrors org.apache.lucene.index.SortedNumericDocValues#advanceExact.
	AdvanceExact(target int) (bool, error)

	// NextValue returns the next numeric value for the current document
	// position. Iterate up to DocValueCount values per document.
	//
	// Mirrors org.apache.lucene.index.SortedNumericDocValues#nextValue.
	NextValue() (int64, error)

	// DocValueCount returns the number of values bound to the current
	// document position.
	//
	// Mirrors org.apache.lucene.index.SortedNumericDocValues#docValueCount.
	DocValueCount() (int, error)

	// NextDoc returns the next document that has values.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// DocID returns the current document ID.
	DocID() int
}

// SortedSetDocValues provides an iterator over sorted set doc values.
// This is the Go port of Lucene's org.apache.lucene.index.SortedSetDocValues.
//
// TODO(T4710): rmp #4710 will drop Get(docID) and collapse this body to
// a type alias of spi.SortedSetDocValues. T4709 added AdvanceExact and
// NextOrd alongside Get.
type SortedSetDocValues interface {
	// Get returns the ordinals for the given document.
	// Returns an empty slice if the document has no values for this field.
	Get(docID int) ([]int, error)

	// Advance advances to the given document.
	// Returns the document ID or NO_MORE_DOCS.
	Advance(target int) (int, error)

	// AdvanceExact positions the iterator on the given target document
	// and returns true if that document has at least one ordinal.
	// Callers MUST advance monotonically (target >= previous target).
	//
	// Mirrors org.apache.lucene.index.SortedSetDocValues#advanceExact.
	AdvanceExact(target int) (bool, error)

	// NextOrd returns the next ordinal for the current document
	// position, or -1 when the document has no more ordinals.
	//
	// Mirrors org.apache.lucene.index.SortedSetDocValues#nextOrd.
	NextOrd() (int, error)

	// NextDoc returns the next document that has values.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// DocID returns the current document ID.
	DocID() int

	// LookupOrd returns the value for the given ordinal.
	LookupOrd(ord int) ([]byte, error)

	// GetValueCount returns the number of unique values.
	GetValueCount() int
}

// FloatVectorValues provides an iterator over float vector values.
// This is the Go port of Lucene's org.apache.lucene.index.FloatVectorValues.
type FloatVectorValues interface {
	// Get returns the float vector for the given document.
	// Returns nil if the document has no vector for this field.
	Get(docID int) ([]float32, error)

	// Advance advances to the given document.
	// Returns the document ID or NO_MORE_DOCS.
	Advance(target int) (int, error)

	// NextDoc returns the next document that has a vector.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// DocID returns the current document ID.
	DocID() int

	// Dimension returns the dimension of the vectors.
	Dimension() int

	// Size returns the number of documents with vectors.
	Size() int
}

// ByteVectorValues provides an iterator over byte vector values.
// This is the Go port of Lucene's org.apache.lucene.index.ByteVectorValues.
type ByteVectorValues interface {
	// Get returns the byte vector for the given document.
	// Returns nil if the document has no vector for this field.
	Get(docID int) ([]byte, error)

	// Advance advances to the given document.
	// Returns the document ID or NO_MORE_DOCS.
	Advance(target int) (int, error)

	// NextDoc returns the next document that has a vector.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// DocID returns the current document ID.
	DocID() int

	// Dimension returns the dimension of the vectors.
	Dimension() int

	// Size returns the number of documents with vectors.
	Size() int
}

// DocValuesSkipper provides efficient skipping for doc values.
// This is the Go port of Lucene's org.apache.lucene.index.DocValuesSkipIndexReader.
type DocValuesSkipper interface {
	// Skip to a document ID that is >= target and has a value.
	// Returns the document ID or NO_MORE_DOCS.
	SkipTo(target int) (int, error)

	// GetDocID returns the current document ID.
	GetDocID() int
}

// PointValues provides access to point values.
// This is the Go port of Lucene's org.apache.lucene.index.PointValues.
type PointValues interface {
	// GetDocCount returns the number of documents that have point values for this field.
	GetDocCount() int

	// GetDocCountWithValue returns the number of documents with values.
	GetDocCountWithValue() int64

	// GetValueCount returns the total number of values across all documents.
	GetValueCount() int64

	// GetMinPackedValue returns the minimum packed value.
	GetMinPackedValue() ([]byte, error)

	// GetMaxPackedValue returns the maximum packed value.
	GetMaxPackedValue() ([]byte, error)

	// GetNumDimensions returns the number of dimensions.
	GetNumDimensions() int

	// GetBytesPerDimension returns the number of bytes per dimension.
	GetBytesPerDimension() int
}
