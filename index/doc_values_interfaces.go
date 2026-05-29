// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/spi"

// This file is the index-side facade for the doc-values value-type
// contracts after rmp #4710 (Sprint 118 phase 2f). The canonical
// declarations live in spi/ (lifted there by rmp #4708) and the
// iterator-shaped methods were added additively by rmp #4709. T4710
// completed the structural collapse: every value-type interface body
// previously declared in this file is now a Go type alias of its
// spi/ counterpart, removing the legacy random-access Get(docID) /
// GetOrd(docID) projection in the process. Implementations that
// historically returned an index.X under the old shape continue to
// satisfy the alias because every Gocene producer (codec-side,
// writer-side, filter wrapper, singleton wrapper, …) was migrated to
// the iterator surface as part of T4710.

// NumericDocValues is an alias of [spi.NumericDocValues] — the iterator-
// shaped per-document numeric value contract.
type NumericDocValues = spi.NumericDocValues

// BinaryDocValues is an alias of [spi.BinaryDocValues].
type BinaryDocValues = spi.BinaryDocValues

// SortedDocValues is an alias of [spi.SortedDocValues].
type SortedDocValues = spi.SortedDocValues

// SortedNumericDocValues is an alias of [spi.SortedNumericDocValues].
type SortedNumericDocValues = spi.SortedNumericDocValues

// SortedSetDocValues is an alias of [spi.SortedSetDocValues].
type SortedSetDocValues = spi.SortedSetDocValues

// DocValuesSkipper is an alias of [spi.DocValuesSkipper].
type DocValuesSkipper = spi.DocValuesSkipper

// FloatVectorValues provides an iterator over float vector values.
// This is the Go port of Lucene's org.apache.lucene.index.FloatVectorValues.
//
// Out of scope for the rmp #4710 doc-values collapse: vector values
// have not yet been lifted onto the SPI iterator surface.
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
//
// Out of scope for the rmp #4710 doc-values collapse: vector values
// have not yet been lifted onto the SPI iterator surface.
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

// PointTreeIntersectVisitor is the canonical visitor surface a point-values
// consumer drives during a BKD intersection. It mirrors the subset of
// org.apache.lucene.index.PointValues.IntersectVisitor that the search-side
// point queries (PointRangeQuery, PointInGeo3DShapeQuery, …) actually invoke.
//
// It is exported from the index package so that consumers in search/ and
// spatial3d/ can alias it (rmp #4769) and an on-disk BKD-backed PointValues —
// constructed by the codec PointsReader — can satisfy the consumers' narrow
// PointValues interfaces without each package owning a distinct,
// structurally-identical-but-incompatible visitor type. Compare returns an
// int (0=CELL_OUTSIDE_QUERY, 1=CELL_INSIDE_QUERY, 2=CELL_CROSSES_QUERY),
// matching the codecs.Relation / index.PointValues.Relation enum order, so the
// search packages do not need to import codecs.
type PointTreeIntersectVisitor interface {
	// Visit is called for each docID that matches the query when the
	// packed value is not needed (CELL_INSIDE_QUERY leaves).
	Visit(docID int) error

	// VisitByPackedValue is called for each (docID, packedValue) pair that
	// must be gated against the query (CELL_CROSSES_QUERY leaves). The
	// packedValue buffer is reused across calls; visitors that retain it
	// must copy.
	VisitByPackedValue(docID int, packedValue []byte) error

	// Compare returns the relation between the cell [minPackedValue,
	// maxPackedValue] and the query: 0=outside, 1=inside, 2=crosses.
	Compare(minPackedValue, maxPackedValue []byte) int

	// Grow is a hint that the visitor will receive at least count more
	// matches in the current sub-walk.
	Grow(count int)
}
