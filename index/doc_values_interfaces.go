// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// NumericDocValues provides an iterator over numeric doc values.
// This is the Go port of Lucene's org.apache.lucene.index.NumericDocValues.
type NumericDocValues interface {
	// Get returns the numeric value for the given document.
	// Returns 0 if the document has no value for this field.
	Get(docID int) (int64, error)

	// Advance advances to the given document.
	// Returns true if the document has a value, false otherwise.
	Advance(target int) (int, error)

	// NextDoc returns the next document that has a value.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// DocID returns the current document ID.
	DocID() int
}

// BinaryDocValues provides an iterator over binary doc values.
// This is the Go port of Lucene's org.apache.lucene.index.BinaryDocValues.
type BinaryDocValues interface {
	// Get returns the binary value for the given document.
	// Returns nil if the document has no value for this field.
	Get(docID int) ([]byte, error)

	// Advance advances to the given document.
	// Returns the document ID or NO_MORE_DOCS.
	Advance(target int) (int, error)

	// NextDoc returns the next document that has a value.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// DocID returns the current document ID.
	DocID() int
}

// SortedDocValues provides an iterator over sorted doc values.
// This is the Go port of Lucene's org.apache.lucene.index.SortedDocValues.
type SortedDocValues interface {
	BinaryDocValues

	// GetOrd returns the ordinal for the given document.
	// Returns -1 if the document has no value for this field.
	GetOrd(docID int) (int, error)

	// LookupOrd returns the value for the given ordinal.
	LookupOrd(ord int) ([]byte, error)

	// GetValueCount returns the number of unique values.
	GetValueCount() int
}

// SortedNumericDocValues provides an iterator over sorted numeric doc values.
// This is the Go port of Lucene's org.apache.lucene.index.SortedNumericDocValues.
type SortedNumericDocValues interface {
	// Get returns the numeric values for the given document.
	// Returns an empty slice if the document has no values for this field.
	Get(docID int) ([]int64, error)

	// Advance advances to the given document.
	// Returns the document ID or NO_MORE_DOCS.
	Advance(target int) (int, error)

	// NextDoc returns the next document that has values.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// DocID returns the current document ID.
	DocID() int
}

// SortedSetDocValues provides an iterator over sorted set doc values.
// This is the Go port of Lucene's org.apache.lucene.index.SortedSetDocValues.
type SortedSetDocValues interface {
	// Get returns the ordinals for the given document.
	// Returns an empty slice if the document has no values for this field.
	Get(docID int) ([]int, error)

	// Advance advances to the given document.
	// Returns the document ID or NO_MORE_DOCS.
	Advance(target int) (int, error)

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
