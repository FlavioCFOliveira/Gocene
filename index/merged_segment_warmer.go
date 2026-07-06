// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// MergedSegmentWarmer is called on each newly merged segment before it becomes
// visible to readers. Implementations typically touch terms, doc values, stored
// fields, etc., to page the underlying files into the OS cache. Mirrors
// org.apache.lucene.index.IndexWriter.IndexReaderWarmer (merged-segment warmer).
//
// The Warm parameter is a leaf-reader surface that is satisfied by both
// *LeafReader and *SegmentReader so that unit tests can exercise a warmer
// against a minimal stub, while production merges pass the real SegmentReader.
type MergedSegmentWarmer interface {
	// Warm warms the provided leaf reader. Errors are treated as non-fatal by
	// Lucene's merge pipeline, but returning an error lets the caller log it.
	Warm(reader SegmentWarmerLeafReader) error
}

// SegmentWarmerLeafReader is the minimal leaf-reader surface needed by
// MergedSegmentWarmer. It is implemented by *LeafReader and *SegmentReader.
// This interface is kept narrow to avoid coupling warmers to the full
// index.LeafReaderInterface.
type SegmentWarmerLeafReader interface {
	GetFieldInfos() *FieldInfos
	Terms(field string) (Terms, error)
	GetNormValues(field string) (NumericDocValues, error)
	GetNumericDocValues(field string) (NumericDocValues, error)
	GetBinaryDocValues(field string) (BinaryDocValues, error)
	GetSortedDocValues(field string) (SortedDocValues, error)
	GetSortedNumericDocValues(field string) (SortedNumericDocValues, error)
	GetSortedSetDocValues(field string) (SortedSetDocValues, error)
	StoredFields() (StoredFields, error)
	TermVectors() (TermVectors, error)
}
