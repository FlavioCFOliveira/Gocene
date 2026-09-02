//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilterDirectoryReader is a DirectoryReader that wraps another DirectoryReader.
//
// This is the Go port of Lucene's org.apache.lucene.index.FilterDirectoryReader.
type FilterDirectoryReader struct {
	*DirectoryReader
	in *DirectoryReader
}

// NewFilterDirectoryReader creates a new FilterDirectoryReader wrapping the given reader.
func NewFilterDirectoryReader(in *DirectoryReader) *FilterDirectoryReader {
	return &FilterDirectoryReader{
		DirectoryReader: in,
		in:              in,
	}
}

// GetDelegate returns the wrapped DirectoryReader.
func (r *FilterDirectoryReader) GetDelegate() *DirectoryReader {
	return r.in
}

// Close closes the wrapped reader.
func (r *FilterDirectoryReader) Close() error {
	return r.in.Close()
}

// LeafReader is a wrapper for a LeafReader.
type FilterLeafReader struct {
	*LeafReader
	in LeafReaderInterface

	// cacheHelper, when non-nil, overrides the cache helper exposed by this
	// wrapper. A custom key lets callers prove that listeners registered on
	// the wrapper are isolated from listeners on the wrapped reader.
	cacheHelper CacheHelper
}

// NewFilterLeafReader creates a new FilterLeafReader wrapping the given reader.
func NewFilterLeafReader(in LeafReaderInterface) *FilterLeafReader {
	var si *SegmentInfo
	if withSeg, ok := in.(interface{ GetSegmentInfo() *SegmentInfo }); ok {
		si = withSeg.GetSegmentInfo()
	}
	return &FilterLeafReader{
		LeafReader: NewLeafReader(si),
		in:         in,
	}
}

// NewFilterLeafReaderWithCacheKey creates a FilterLeafReader that exposes a
// fresh cache helper/key instead of delegating to the wrapped reader. This is
// the Go equivalent of a Lucene FilterLeafReader subclass that overrides
// getCoreCacheHelper() to return its own key.
func NewFilterLeafReaderWithCacheKey(in LeafReaderInterface) *FilterLeafReader {
	var si *SegmentInfo
	if withSeg, ok := in.(interface{ GetSegmentInfo() *SegmentInfo }); ok {
		si = withSeg.GetSegmentInfo()
	}
	return &FilterLeafReader{
		LeafReader:  NewLeafReader(si),
		in:          in,
		cacheHelper: NewReaderCacheHelper(),
	}
}

// GetCacheHelper returns the cache helper for this wrapper. When a custom helper
// was requested (NewFilterLeafReaderWithCacheKey) it is returned; otherwise the
// wrapped reader's helper is delegated to.
func (r *FilterLeafReader) GetCacheHelper() CacheHelper {
	if r.cacheHelper != nil {
		return r.cacheHelper
	}
	if withHelper, ok := interface{}(r.in).(interface{ GetCacheHelper() CacheHelper }); ok {
		return withHelper.GetCacheHelper()
	}
	return nil
}

// GetCoreCacheKey delegates to the wrapped reader.
func (r *FilterLeafReader) GetCoreCacheKey() interface{} {
	return r.in.GetCoreCacheKey()
}

// Close closes the wrapper and, when no custom cache helper is in use, also
// closes the wrapped reader. A FilterLeafReader created with a custom cache
// key (NewFilterLeafReaderWithCacheKey) owns its own IndexReader lifecycle;
// closing it must not propagate to the wrapped reader, so listeners registered
// on the inner reader's cache helper remain isolated.
func (r *FilterLeafReader) Close() error {
	// Custom cache key: this wrapper has an independent lifecycle. Notify its
	// own closed listeners and mark it closed, but do not touch the wrapped
	// reader.
	if r.cacheHelper != nil {
		if h, ok := r.cacheHelper.(*ReaderCacheHelper); ok {
			h.SetClosed()
			h.NotifyClosedListeners()
		}
		return r.IndexReader.Close()
	}

	// Default case: share the wrapped reader's lifecycle, so close it first.
	var lastErr error
	if closer, ok := interface{}(r.in).(io.Closer); ok {
		if err := closer.Close(); err != nil {
			lastErr = err
		}
	}
	if err := r.IndexReader.Close(); err != nil {
		if lastErr == nil {
			lastErr = err
		}
	}
	return lastErr
}

// GetDelegate returns the wrapped LeafReader.
func (r *FilterLeafReader) GetDelegate() LeafReaderInterface {
	return r.in
}

// DocCount returns the total number of documents.
func (r *FilterLeafReader) DocCount() int {
	return r.in.DocCount()
}

// NumDocs returns the number of live documents.
func (r *FilterLeafReader) NumDocs() int {
	return r.in.NumDocs()
}

// MaxDoc returns the maximum document ID plus one.
func (r *FilterLeafReader) MaxDoc() int {
	return r.in.MaxDoc()
}

// HasDeletions returns true if this reader has deleted documents.
func (r *FilterLeafReader) HasDeletions() bool {
	return r.in.HasDeletions()
}

// NumDeletedDocs returns the number of deleted documents.
func (r *FilterLeafReader) NumDeletedDocs() int {
	return r.in.NumDeletedDocs()
}

// GetTermVectors returns the term vectors for a document.
func (r *FilterLeafReader) GetTermVectors(docID int) (Fields, error) {
	return r.in.GetTermVectors(docID)
}

// Terms returns the Terms for a field.
func (r *FilterLeafReader) Terms(field string) (Terms, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	return r.in.Terms(field)
}

// Postings returns the postings for a term.
func (r *FilterLeafReader) Postings(term Term) (PostingsEnum, error) {
	return r.in.Postings(term)
}

// PostingsWithFreqPositions returns the postings for a term with specific flags.
func (r *FilterLeafReader) PostingsWithFreqPositions(term Term, flags int) (PostingsEnum, error) {
	return r.in.PostingsWithFreqPositions(term, flags)
}

// GetNumericDocValues returns NumericDocValues for the given field.
func (r *FilterLeafReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	return r.in.GetNumericDocValues(field)
}

// GetBinaryDocValues returns BinaryDocValues for the given field.
func (r *FilterLeafReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	return r.in.GetBinaryDocValues(field)
}

// GetSortedDocValues returns SortedDocValues for the given field.
func (r *FilterLeafReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	return r.in.GetSortedDocValues(field)
}

// GetSortedNumericDocValues returns SortedNumericDocValues for the given field.
func (r *FilterLeafReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	return r.in.GetSortedNumericDocValues(field)
}

// GetSortedSetDocValues returns SortedSetDocValues for the given field.
func (r *FilterLeafReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	return r.in.GetSortedSetDocValues(field)
}

// GetNormValues returns NumericDocValues for norms of the given field.
func (r *FilterLeafReader) GetNormValues(field string) (NumericDocValues, error) {
	return r.in.GetNormValues(field)
}

// GetPointValues returns PointValues for the given field.
func (r *FilterLeafReader) GetPointValues(field string) (PointValues, error) {
	return r.in.GetPointValues(field)
}

// GetFloatVectorValues returns FloatVectorValues for the given field.
func (r *FilterLeafReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	return r.in.GetFloatVectorValues(field)
}

// GetByteVectorValues returns ByteVectorValues for the given field.
func (r *FilterLeafReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	return r.in.GetByteVectorValues(field)
}

// SearchNearestVectors searches for the k nearest vectors to the target.
func (r *FilterLeafReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits) (TopDocs, error) {
	return r.in.SearchNearestVectors(field, target, k, acceptDocs)
}

// GetDocValuesSkipper returns a DocValuesSkipper for efficient skipping.
func (r *FilterLeafReader) GetDocValuesSkipper(field string) (DocValuesSkipper, error) {
	return r.in.GetDocValuesSkipper(field)
}

// CheckIntegrity checks that the index is not corrupt.
func (r *FilterLeafReader) CheckIntegrity() error {
	return r.in.CheckIntegrity()
}

// GetMetaData returns the IndexReaderMetaData for this reader.
func (r *FilterLeafReader) GetMetaData() *IndexReaderMetaData {
	return r.in.GetMetaData()
}

// GetSegmentInfo returns the SegmentInfo for this reader.
func (r *FilterLeafReader) GetSegmentInfo() *SegmentInfo {
	return r.in.GetSegmentInfo()
}

// IncRef increments the reference count.
func (r *FilterLeafReader) IncRef() error {
	return r.in.IncRef()
}

// DecRef decrements the reference count.
func (r *FilterLeafReader) DecRef() error {
	return r.in.DecRef()
}

// TryIncRef tries to increment the reference count.
func (r *FilterLeafReader) TryIncRef() bool {
	return r.in.TryIncRef()
}

// GetRefCount returns the current reference count.
func (r *FilterLeafReader) GetRefCount() int32 {
	return r.in.GetRefCount()
}


// StoredFields returns a StoredFields instance for accessing stored fields.
func (r *FilterLeafReader) StoredFields() (StoredFields, error) {
	return r.in.StoredFields()
}

// TermVectors returns a TermVectors instance for accessing term vectors.
func (r *FilterLeafReader) TermVectors() (TermVectors, error) {
	return r.in.TermVectors()
}

// GetContext returns the reader context for this leaf reader.
func (r *FilterLeafReader) GetContext() (IndexReaderContext, error) {
	return r.in.GetContext()
}

// Leaves returns all leaf reader contexts (just this one for a leaf).
func (r *FilterLeafReader) Leaves() ([]*LeafReaderContext, error) {
	return r.in.Leaves()
}

// FilterCodecReader is a CodecReader that wraps another CodecReader.
