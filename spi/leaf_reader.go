// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LeafReader is an interface for accessing a single segment of the index.
// Search of an index is done entirely through this interface.
//
// This is the Go port of Lucene's org.apache.lucene.index.LeafReader.
type LeafReader interface {
	// LeafReader is-a IndexReader: Lucene declares `public abstract class LeafReader
	// extends IndexReader`, so every leaf reader also honours the IndexReader contract.
	IndexReaderInterface

	// DocID returns the first document ID in this segment.
	DocID() int

	// MaxDoc returns the maximum document ID (one past the last doc).
	MaxDoc() int

	// NumDocs returns the number of live documents.
	NumDocs() int

	// DocFreq returns the number of documents containing the term.
	DocFreq(term Term) (int, error)

	// TotalTermFreq returns the total number of occurrences of the term.
	TotalTermFreq(term Term) (int64, error)

	// Terms returns the Terms index for this field, or nil if it has none.
	Terms(field string) (Terms, error)

	// Postings returns PostingsEnum for the specified term.
	Postings(term Term, flags int) (PostingsEnum, error)

	// GetNumericDocValues returns NumericDocValues for this field.
	GetNumericDocValues(field string) (NumericDocValues, error)

	// GetBinaryDocValues returns BinaryDocValues for this field.
	GetBinaryDocValues(field string) (BinaryDocValues, error)

	// GetSortedDocValues returns SortedDocValues for this field.
	GetSortedDocValues(field string) (SortedDocValues, error)

	// GetSortedNumericDocValues returns SortedNumericDocValues for this field.
	GetSortedNumericDocValues(field string) (SortedNumericDocValues, error)

	// GetSortedSetDocValues returns SortedSetDocValues for this field.
	GetSortedSetDocValues(field string) (SortedSetDocValues, error)

	// GetNormValues returns NumericDocValues representing norms for this field.
	GetNormValues(field string) (NumericDocValues, error)

	// GetDocValuesSkipper returns a DocValuesSkipper for efficient skipping.
	GetDocValuesSkipper(field string) (DocValuesSkipper, error)

	// GetFloatVectorValues returns FloatVectorValues for this field.
	GetFloatVectorValues(field string) (FloatVectorValues, error)

	// GetByteVectorValues returns ByteVectorValues for this field.
	GetByteVectorValues(field string) (ByteVectorValues, error)

	// SearchNearestVectors searches for the k nearest float vectors to target.
	//
	// Mirrors the final convenience overload
	// LeafReader.searchNearestVectors(String, float[], int, AcceptDocs, int).
	SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (TopDocs, error)

	// SearchNearestVectorsCollector returns the nearest neighbour documents of
	// target in field, gathering them into knnCollector.
	//
	// Mirrors the abstract overload
	// LeafReader.searchNearestVectors(String, float[], KnnCollector, AcceptDocs).
	// Go has no overloading, so the collector-driven form carries the Collector
	// suffix; acceptDocs is rendered as util.Bits, the spelling this interface
	// already uses for the same Lucene parameter on the convenience overload.
	SearchNearestVectorsCollector(field string, target []float32, knnCollector KnnCollector, acceptDocs util.Bits) error

	// SearchNearestVectorsByteCollector returns the nearest neighbour documents
	// of the byte-valued target in field, gathering them into knnCollector.
	//
	// Mirrors the abstract overload
	// LeafReader.searchNearestVectors(String, byte[], KnnCollector, AcceptDocs).
	SearchNearestVectorsByteCollector(field string, target []byte, knnCollector KnnCollector, acceptDocs util.Bits) error

	// GetFieldInfos returns the FieldInfos describing all fields in this reader.
	GetFieldInfos() *FieldInfos

	// GetLiveDocs returns a bitset of live (not deleted) docs.
	GetLiveDocs() util.Bits

	// GetPointValues returns the PointValues for numeric or spatial searches.
	GetPointValues(field string) (PointValues, error)

	// CheckIntegrity checks that the index is not corrupt.
	CheckIntegrity() error

	// GetMetaData returns metadata about this leaf.
	GetMetaData() *IndexReaderMetaData

	// GetContext returns the reader context for this leaf reader.
	GetContext() (IndexReaderContext, error)

	// Close closes the reader and releases resources.
	Close() error

	// IncRef increments the reference count.
	IncRef() error

	// DecRef decrements the reference count.
	DecRef() error

	// TryIncRef tries to increment the reference count.
	TryIncRef() bool

	// GetRefCount returns the current reference count.
	GetRefCount() int32

	// GetCoreCacheHelper returns a CacheHelper for the core data of this leaf.
	GetCoreCacheHelper() CacheHelper

	// GetReaderCacheHelper returns a CacheHelper for the reader.
	GetReaderCacheHelper() CacheHelper
}

// IndexReaderMetaData provides metadata about a LeafReader.
// This is the Go port of Lucene's org.apache.lucene.index.IndexReader.Metadata.
type IndexReaderMetaData struct {
	// HasDeletions is true if this reader has deletions.
	HasDeletions bool

	// NumDocs is the number of live documents.
	NumDocs int

	// MaxDoc is the maximum document ID plus one.
	MaxDoc int
}
