// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilterLeafReader contains another LeafReader, which it uses as its basic source of
// data, possibly transforming the data along the way or providing additional functionality.
// This is the Go port of Lucene's org.apache.lucene.index.FilterLeafReader.
type FilterLeafReader struct {
	in LeafReader

	// cacheHelper, when non-nil, overrides the cache helper exposed by this
	// wrapper. A custom key lets callers prove that listeners registered on
	// the wrapper are isolated from listeners on the wrapped reader.
	cacheHelper CacheHelper
}

// NewFilterLeafReader constructs a FilterLeafReader based on the specified base reader.
func NewFilterLeafReader(in LeafReader) *FilterLeafReader {
	if in == nil {
		panic("incoming LeafReader must not be null")
	}
	return &FilterLeafReader{in: in}
}

// NewFilterLeafReaderWithCacheKey creates a FilterLeafReader that exposes a
// fresh cache helper/key instead of delegating to the wrapped reader. This is
// the Go equivalent of a Lucene FilterLeafReader subclass that overrides
// getCoreCacheHelper() to return its own key.
func NewFilterLeafReaderWithCacheKey(in LeafReader) *FilterLeafReader {
	if in == nil {
		panic("incoming LeafReader must not be null")
	}
	return &FilterLeafReader{
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
		// Note: FilterLeafReader itself doesn't have an IndexReader to close,
		// but it might be part of one. Since it's a wrapper, we just return nil.
		return nil
	}

	// Default case: share the wrapped reader's lifecycle, so close it first.
	var lastErr error
	if closer, ok := interface{}(r.in).(io.Closer); ok {
		if err := closer.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// GetDelegate returns the wrapped LeafReader.
func (r *FilterLeafReader) GetDelegate() LeafReader {
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

func (f *FilterLeafReader) String() string {
	return fmt.Sprintf("FilterLeafReader(%s)", f.in.String())
}

// --- Inner filter classes as separate types ---

// FilterFields is a base class for filtering Fields implementations.
type FilterFields struct {
	in Fields
}

func NewFilterFields(in Fields) *FilterFields {
	if in == nil {
		panic("incoming Fields must not be null")
	}
	return &FilterFields{in: in}
}

func (f *FilterFields) Iterator() []string {
	return f.in.Iterator()
}

func (f *FilterFields) Terms(field string) (Terms, error) {
	return f.in.Terms(field)
}

func (f *FilterFields) Size() int {
	return f.in.Size()
}

// FilterTerms is a base class for filtering Terms implementations.
type FilterTerms struct {
	in Terms
}

func NewFilterTerms(in Terms) *FilterTerms {
	if in == nil {
		panic("incoming Terms must not be null")
	}
	return &FilterTerms{in: in}
}

func (f *FilterTerms) Iterator() (TermsEnum, error) {
	return f.in.Iterator()
}

func (f *FilterTerms) Size() (int64, error) {
	return f.in.Size()
}

func (f *FilterTerms) GetSumTotalTermFreq() (int64, error) {
	return f.in.GetSumTotalTermFreq()
}

func (f *FilterTerms) GetSumDocFreq() (int64, error) {
	return f.in.GetSumDocFreq()
}

func (f *FilterTerms) GetDocCount() (int, error) {
	return f.in.GetDocCount()
}

func (f *FilterTerms) HasFreqs() bool {
	return f.in.HasFreqs()
}

func (f *FilterTerms) HasOffsets() bool {
	return f.in.HasOffsets()
}

func (f *FilterTerms) HasPositions() bool {
	return f.in.HasPositions()
}

func (f *FilterTerms) HasPayloads() bool {
	return f.in.HasPayloads()
}

func (f *FilterTerms) GetStats() (interface{}, error) {
	return f.in.GetStats()
}

// FilterTermsEnum is a base class for filtering TermsEnum implementations.
type FilterTermsEnum struct {
	in TermsEnum
}

func NewFilterTermsEnum(in TermsEnum) *FilterTermsEnum {
	if in == nil {
		panic("incoming TermsEnum must not be null")
	}
	return &FilterTermsEnum{in: in}
}

func (f *FilterTermsEnum) Attributes() util.AttributeSource {
	return f.in.Attributes()
}

func (f *FilterTermsEnum) SeekCeil(text *util.BytesRef) (SeekStatus, error) {
	return f.in.SeekCeil(text)
}

func (f *FilterTermsEnum) SeekExact(text *util.BytesRef) (bool, error) {
	return f.in.SeekExact(text)
}

func (f *FilterTermsEnum) SeekExactOrd(ord int64) error {
	return f.in.SeekExactOrd(ord)
}

func (f *FilterTermsEnum) Next() (*util.BytesRef, error) {
	return f.in.Next()
}

func (f *FilterTermsEnum) Term() (*util.BytesRef, error) {
	return f.in.Term()
}

func (f *FilterTermsEnum) Ord() (int64, error) {
	return f.in.Ord()
}

func (f *FilterTermsEnum) DocFreq() (int, error) {
	return f.in.DocFreq()
}

func (f *FilterTermsEnum) TotalTermFreq() (int64, error) {
	return f.in.TotalTermFreq()
}

func (f *FilterTermsEnum) Postings(reuse PostingsEnum, flags int) (PostingsEnum, error) {
	return f.in.Postings(reuse, flags)
}

func (f *FilterTermsEnum) Impacts(flags int) (ImpactsEnum, error) {
	return f.in.Impacts(flags)
}

func (f *FilterTermsEnum) SeekExactWithState(term *util.BytesRef, state *TermState) error {
	return f.in.SeekExactWithState(term, state)
}

func (f *FilterTermsEnum) PrepareSeekExact(text *util.BytesRef) (util.IOBooleanSupplier, error) {
	return f.in.PrepareSeekExact(text)
}

func (f *FilterTermsEnum) TermState() (*TermState, error) {
	return f.in.TermState()
}

// FilterPostingsEnum is a base class for filtering PostingsEnum implementations.
type FilterPostingsEnum struct {
	in PostingsEnum
}

func NewFilterPostingsEnum(in PostingsEnum) *FilterPostingsEnum {
	if in == nil {
		panic("incoming PostingsEnum must not be null")
	}
	return &FilterPostingsEnum{in: in}
}

func (f *FilterPostingsEnum) DocID() int {
	return f.in.DocID()
}

func (f *FilterPostingsEnum) Freq() (int, error) {
	return f.in.Freq()
}

func (f *FilterPostingsEnum) NextDoc() (int, error) {
	return f.in.NextDoc()
}

func (f *FilterPostingsEnum) Advance(target int) (int, error) {
	return f.in.Advance(target)
}

func (f *FilterPostingsEnum) NextPosition() (int, error) {
	return f.in.NextPosition()
}

func (f *FilterPostingsEnum) StartOffset() (int, error) {
	return f.in.StartOffset()
}

func (f *FilterPostingsEnum) EndOffset() (int, error) {
	return f.in.EndOffset()
}

func (f *FilterPostingsEnum) GetPayload() (*util.BytesRef, error) {
	return f.in.GetPayload()
}

func (f *FilterPostingsEnum) Cost() int64 {
	return f.in.Cost()
}

func (f *FilterPostingsEnum) Unwrap() PostingsEnum {
	return f.in
}
