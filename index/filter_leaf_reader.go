// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// errFilterLeafReaderUnsupported mirrors the UnsupportedOperationException the
// Lucene base classes raise for the optional parts of the Terms / TermsEnum
// contracts that a wrapped delegate does not provide.
var errFilterLeafReaderUnsupported = errors.New("operation not supported by the wrapped reader")

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
		cacheHelper: spi.NewReaderCacheHelper(),
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

// GetCoreCacheHelper delegates to the wrapped reader, mirroring the guidance in
// FilterLeafReader's javadoc: a wrapper that does not change the content of the
// wrapped reader delegates getCoreCacheHelper().
func (r *FilterLeafReader) GetCoreCacheHelper() CacheHelper {
	if r.cacheHelper != nil {
		return r.cacheHelper
	}
	return r.in.GetCoreCacheHelper()
}

// GetReaderCacheHelper delegates to the wrapped reader, for the same reason as
// GetCoreCacheHelper.
func (r *FilterLeafReader) GetReaderCacheHelper() CacheHelper {
	if r.cacheHelper != nil {
		return r.cacheHelper
	}
	return r.in.GetReaderCacheHelper()
}

// GetCoreCacheKey returns the cache key of the core cache helper, or nil when
// this reader exposes no core helper.
func (r *FilterLeafReader) GetCoreCacheKey() interface{} {
	helper := r.GetCoreCacheHelper()
	if helper == nil {
		return nil
	}
	return helper.CacheKey()
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
		if h, ok := r.cacheHelper.(*spi.ReaderCacheHelper); ok {
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

// UnwrapFilterLeafReader unwraps chains of FilterLeafReader, mirroring the
// static FilterLeafReader.unwrap(LeafReader).
func UnwrapFilterLeafReader(reader LeafReader) LeafReader {
	for {
		filtered, ok := reader.(*FilterLeafReader)
		if !ok {
			return reader
		}
		reader = filtered.GetDelegate()
	}
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

// DocID returns the first document ID in this segment.
func (r *FilterLeafReader) DocID() int {
	return r.in.DocID()
}

// HasDeletions returns true if this reader has deleted documents.
func (r *FilterLeafReader) HasDeletions() bool {
	return r.in.HasDeletions()
}

// NumDeletedDocs returns the number of deleted documents.
func (r *FilterLeafReader) NumDeletedDocs() int {
	return r.in.NumDeletedDocs()
}

// EnsureOpen fails when the wrapped reader is closed.
func (r *FilterLeafReader) EnsureOpen() error {
	return r.in.EnsureOpen()
}

// GetFieldInfos returns the FieldInfos of the wrapped reader.
func (r *FilterLeafReader) GetFieldInfos() *spi.FieldInfos {
	return r.in.GetFieldInfos()
}

// GetLiveDocs returns the live docs of the wrapped reader.
func (r *FilterLeafReader) GetLiveDocs() util.Bits {
	return r.in.GetLiveDocs()
}

// GetTermVectors returns the term vectors of a document as a Fields view.
// Lucene 10 replaced IndexReader.getTermVectors(int) with
// termVectors().get(int); this accessor is the same call expressed on the
// reader.
func (r *FilterLeafReader) GetTermVectors(docID int) (Fields, error) {
	tv, err := r.in.TermVectors()
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, nil
	}
	return tv.Get(docID)
}

// Terms returns the Terms for a field.
func (r *FilterLeafReader) Terms(field string) (Terms, error) {
	return r.in.Terms(field)
}

// DocFreq returns the number of documents containing the term.
func (r *FilterLeafReader) DocFreq(term Term) (int, error) {
	return r.in.DocFreq(term)
}

// TotalTermFreq returns the total number of occurrences of the term.
func (r *FilterLeafReader) TotalTermFreq(term Term) (int64, error) {
	return r.in.TotalTermFreq(term)
}

// Postings returns the postings for a term with the requested flags.
func (r *FilterLeafReader) Postings(term Term, flags int) (PostingsEnum, error) {
	return r.in.Postings(term, flags)
}

// PostingsWithFreqPositions returns the postings for a term with specific flags.
// It is the historical spelling of Postings(term, flags) and forwards to it.
func (r *FilterLeafReader) PostingsWithFreqPositions(term Term, flags int) (PostingsEnum, error) {
	return r.in.Postings(term, flags)
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
func (r *FilterLeafReader) GetFloatVectorValues(field string) (spi.FloatVectorValues, error) {
	return r.in.GetFloatVectorValues(field)
}

// GetByteVectorValues returns ByteVectorValues for the given field.
func (r *FilterLeafReader) GetByteVectorValues(field string) (spi.ByteVectorValues, error) {
	return r.in.GetByteVectorValues(field)
}

// SearchNearestVectors searches for the k nearest vectors to the target.
func (r *FilterLeafReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (spi.TopDocs, error) {
	return r.in.SearchNearestVectors(field, target, k, acceptDocs, visitedLimit)
}

// SearchNearestVectorsCollector ports
// FilterLeafReader.searchNearestVectors(String, float[], KnnCollector, AcceptDocs),
// whose body is a straight delegation to the wrapped reader.
func (r *FilterLeafReader) SearchNearestVectorsCollector(field string, target []float32, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	return r.in.SearchNearestVectorsCollector(field, target, knnCollector, acceptDocs)
}

// SearchNearestVectorsByteCollector ports
// FilterLeafReader.searchNearestVectors(String, byte[], KnnCollector, AcceptDocs),
// whose body is a straight delegation to the wrapped reader.
func (r *FilterLeafReader) SearchNearestVectorsByteCollector(field string, target []byte, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	return r.in.SearchNearestVectorsByteCollector(field, target, knnCollector, acceptDocs)
}

// GetDocValuesSkipper returns a DocValuesSkipper for efficient skipping.
func (r *FilterLeafReader) GetDocValuesSkipper(field string) (spi.DocValuesSkipper, error) {
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

// GetSegmentInfo returns the SegmentInfo of the wrapped reader, or nil when the
// wrapped reader is not segment-backed. Lucene reaches the segment info by
// testing the unwrapped reader for SegmentReader; the equivalent here is to
// probe the delegate for the accessor.
func (r *FilterLeafReader) GetSegmentInfo() *SegmentInfo {
	if withInfo, ok := interface{}(r.in).(interface{ GetSegmentInfo() *SegmentInfo }); ok {
		return withInfo.GetSegmentInfo()
	}
	return nil
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

// String mirrors FilterLeafReader.toString: "FilterLeafReader(" + in + ')'.
func (f *FilterLeafReader) String() string {
	return fmt.Sprintf("FilterLeafReader(%v)", f.in)
}

// --- Inner filter classes as separate types ---

// FilterFields is a base class for filtering Fields implementations.
type FilterFields struct {
	in Fields
}

// NewFilterFields wraps in. It panics when in is nil, mirroring the Java
// constructor's NullPointerException.
func NewFilterFields(in Fields) *FilterFields {
	if in == nil {
		panic("incoming Fields must not be null")
	}
	return &FilterFields{in: in}
}

// Iterator returns the wrapped Fields' field-name iterator.
func (f *FilterFields) Iterator() (FieldIterator, error) {
	return f.in.Iterator()
}

// Terms returns the wrapped Fields' Terms for the given field.
func (f *FilterFields) Terms(field string) (Terms, error) {
	return f.in.Terms(field)
}

// Size returns the number of fields in the wrapped Fields.
func (f *FilterFields) Size() int {
	return f.in.Size()
}

// FilterTerms is a base class for filtering Terms implementations.
type FilterTerms struct {
	in Terms
}

// NewFilterTerms wraps in. It panics when in is nil, mirroring the Java
// constructor's NullPointerException.
func NewFilterTerms(in Terms) *FilterTerms {
	if in == nil {
		panic("incoming Terms must not be null")
	}
	return &FilterTerms{in: in}
}

// Field returns the wrapped Terms' field name.
func (f *FilterTerms) Field() string {
	return f.in.Field()
}

// Iterator returns the wrapped Terms' iterator.
func (f *FilterTerms) Iterator() (TermsEnum, error) {
	return f.in.Iterator()
}

// GetIteratorWithSeek returns the wrapped Terms' iterator positioned at or
// after seekTerm.
func (f *FilterTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	return f.in.GetIteratorWithSeek(seekTerm)
}

// Intersect delegates to the wrapped Terms.
//
// NOTE (from the Java javadoc): if the order of terms and documents is not
// changed, and if these terms are going to be intersected with automata,
// subclasses should consider overriding this for better performance.
func (f *FilterTerms) Intersect(compiled *automaton.CompiledAutomaton, startTerm *Term) (TermsEnum, error) {
	return f.in.Intersect(compiled, startTerm)
}

// GetPostingsReader delegates to the wrapped Terms.
func (f *FilterTerms) GetPostingsReader(termText string, flags int) (PostingsEnum, error) {
	return f.in.GetPostingsReader(termText, flags)
}

// Size returns the number of unique terms in the wrapped Terms.
func (f *FilterTerms) Size() int64 {
	return f.in.Size()
}

// GetSumTotalTermFreq delegates to the wrapped Terms.
func (f *FilterTerms) GetSumTotalTermFreq() (int64, error) {
	return f.in.GetSumTotalTermFreq()
}

// GetSumDocFreq delegates to the wrapped Terms.
func (f *FilterTerms) GetSumDocFreq() (int64, error) {
	return f.in.GetSumDocFreq()
}

// GetDocCount delegates to the wrapped Terms.
func (f *FilterTerms) GetDocCount() (int, error) {
	return f.in.GetDocCount()
}

// HasFreqs delegates to the wrapped Terms.
func (f *FilterTerms) HasFreqs() bool {
	return f.in.HasFreqs()
}

// HasOffsets delegates to the wrapped Terms.
func (f *FilterTerms) HasOffsets() bool {
	return f.in.HasOffsets()
}

// HasPositions delegates to the wrapped Terms.
func (f *FilterTerms) HasPositions() bool {
	return f.in.HasPositions()
}

// HasPayloads delegates to the wrapped Terms.
func (f *FilterTerms) HasPayloads() bool {
	return f.in.HasPayloads()
}

// GetMin delegates to the wrapped Terms.
func (f *FilterTerms) GetMin() (*Term, error) {
	return f.in.GetMin()
}

// GetMax delegates to the wrapped Terms.
func (f *FilterTerms) GetMax() (*Term, error) {
	return f.in.GetMax()
}

// GetStats returns the term statistics summary.
//
// Lucene's FilterTerms delegates to in.getStats(); Gocene's Terms contract does
// not carry getStats(), so this runs the concrete implementation Lucene
// declares on the Terms base class over the wrapped Terms.
func (f *FilterTerms) GetStats() (interface{}, error) {
	docCount, err := f.GetDocCount()
	if err != nil {
		return nil, err
	}
	sumTotalTermFreq, err := f.GetSumTotalTermFreq()
	if err != nil {
		return nil, err
	}
	sumDocFreq, err := f.GetSumDocFreq()
	if err != nil {
		return nil, err
	}
	return fmt.Sprintf("impl=%T,size=%d,docCount=%d,sumTotalTermFreq=%d,sumDocFreq=%d",
		f.in, f.Size(), docCount, sumTotalTermFreq, sumDocFreq), nil
}

// FilterTermsEnum is a base class for filtering TermsEnum implementations.
type FilterTermsEnum struct {
	in TermsEnum

	// atts holds the lazily created AttributeSource that BaseTermsEnum keeps
	// when the wrapped enumerator exposes none of its own.
	atts *util.AttributeSource
}

// NewFilterTermsEnum wraps in. It panics when in is nil, mirroring the Java
// constructor's NullPointerException.
func NewFilterTermsEnum(in TermsEnum) *FilterTermsEnum {
	if in == nil {
		panic("incoming TermsEnum must not be null")
	}
	return &FilterTermsEnum{in: in}
}

// Attributes returns the wrapped enumerator's attributes when it exposes them,
// and otherwise the lazily created AttributeSource of BaseTermsEnum.attributes().
func (f *FilterTermsEnum) Attributes() *util.AttributeSource {
	if withAtts, ok := f.in.(interface{ Attributes() *util.AttributeSource }); ok {
		return withAtts.Attributes()
	}
	if f.atts == nil {
		f.atts = util.NewAttributeSource()
	}
	return f.atts
}

// SeekCeil delegates to the wrapped enumerator.
func (f *FilterTermsEnum) SeekCeil(text *Term) (*Term, error) {
	return f.in.SeekCeil(text)
}

// SeekExact delegates to the wrapped enumerator.
func (f *FilterTermsEnum) SeekExact(text *Term) (bool, error) {
	return f.in.SeekExact(text)
}

// SeekExactOrd seeks by ordinal. Lucene's FilterTermsEnum delegates to
// in.seekExact(long); when the wrapped enumerator does not support ordinal
// seeking the TermsEnum contract is an UnsupportedOperationException, which is
// reported here as an error.
func (f *FilterTermsEnum) SeekExactOrd(ord int64) error {
	if seeker, ok := f.in.(interface{ SeekExactOrd(ord int64) error }); ok {
		return seeker.SeekExactOrd(ord)
	}
	return fmt.Errorf("FilterTermsEnum.SeekExactOrd: %w", errFilterLeafReaderUnsupported)
}

// Next delegates to the wrapped enumerator.
func (f *FilterTermsEnum) Next() (*Term, error) {
	return f.in.Next()
}

// Term delegates to the wrapped enumerator.
func (f *FilterTermsEnum) Term() *Term {
	return f.in.Term()
}

// Ord delegates to the wrapped enumerator.
func (f *FilterTermsEnum) Ord() int64 {
	return f.in.Ord()
}

// DocFreq delegates to the wrapped enumerator.
func (f *FilterTermsEnum) DocFreq() (int, error) {
	return f.in.DocFreq()
}

// TotalTermFreq delegates to the wrapped enumerator.
func (f *FilterTermsEnum) TotalTermFreq() (int64, error) {
	return f.in.TotalTermFreq()
}

// Postings delegates to the wrapped enumerator.
func (f *FilterTermsEnum) Postings(flags int) (PostingsEnum, error) {
	return f.in.Postings(flags)
}

// PostingsWithLiveDocs delegates to the wrapped enumerator.
func (f *FilterTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return f.in.PostingsWithLiveDocs(liveDocs, flags)
}

// Impacts delegates to the wrapped enumerator.
func (f *FilterTermsEnum) Impacts(flags int) (spi.ImpactsEnum, error) {
	return f.in.Impacts(flags)
}

// SeekExactWithState seeks by TermState. Lucene's FilterTermsEnum delegates to
// in.seekExact(BytesRef, TermState); when the wrapped enumerator does not
// expose that entry point, the BaseTermsEnum default applies: seek exactly and
// fail when the term does not exist.
func (f *FilterTermsEnum) SeekExactWithState(term *Term, state TermState) error {
	if seeker, ok := f.in.(interface {
		SeekExactWithState(term *Term, state TermState) error
	}); ok {
		return seeker.SeekExactWithState(term, state)
	}
	return SeekExactWithState(f.in, term, state)
}

// PrepareSeekExact returns the two-phase seekExact supplier. Lucene's
// FilterTermsEnum delegates to in.prepareSeekExact; when the wrapped
// enumerator does not expose it, the BaseTermsEnum default applies: a supplier
// that performs the plain seekExact.
func (f *FilterTermsEnum) PrepareSeekExact(text *Term) (util.IOBooleanSupplier, error) {
	if preparer, ok := f.in.(interface {
		PrepareSeekExact(text *Term) (util.IOBooleanSupplier, error)
	}); ok {
		return preparer.PrepareSeekExact(text)
	}
	return func() (bool, error) { return f.in.SeekExact(text) }, nil
}

// TermState returns the wrapped enumerator's TermState when it exposes one,
// and otherwise the BaseTermsEnum default: a TermState whose CopyFrom is
// unsupported.
func (f *FilterTermsEnum) TermState() (TermState, error) {
	if stateful, ok := f.in.(interface{ TermState() (TermState, error) }); ok {
		return stateful.TermState()
	}
	return filterTermState{}, nil
}

// filterTermState mirrors the anonymous TermState returned by
// BaseTermsEnum.termState(), whose copyFrom throws UnsupportedOperationException.
type filterTermState struct{}

// CopyFrom is unsupported.
func (filterTermState) CopyFrom(TermState) error {
	return fmt.Errorf("FilterTermsEnum.TermState.CopyFrom: %w", errFilterLeafReaderUnsupported)
}

// FilterPostingsEnum is a base class for filtering PostingsEnum implementations.
type FilterPostingsEnum struct {
	in PostingsEnum
}

// NewFilterPostingsEnum wraps in. It panics when in is nil, mirroring the Java
// constructor's NullPointerException.
func NewFilterPostingsEnum(in PostingsEnum) *FilterPostingsEnum {
	if in == nil {
		panic("incoming PostingsEnum must not be null")
	}
	return &FilterPostingsEnum{in: in}
}

// DocID delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) DocID() int {
	return f.in.DocID()
}

// DocIDRunEnd delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) DocIDRunEnd() (int, error) {
	return f.in.DocIDRunEnd()
}

// Freq delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) Freq() (int, error) {
	return f.in.Freq()
}

// NextDoc delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) NextDoc() (int, error) {
	return f.in.NextDoc()
}

// Advance delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) Advance(target int) (int, error) {
	return f.in.Advance(target)
}

// NextPosition delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) NextPosition() (int, error) {
	return f.in.NextPosition()
}

// StartOffset delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) StartOffset() (int, error) {
	return f.in.StartOffset()
}

// EndOffset delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) EndOffset() (int, error) {
	return f.in.EndOffset()
}

// GetPayload delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) GetPayload() ([]byte, error) {
	return f.in.GetPayload()
}

// Cost delegates to the wrapped enumerator.
func (f *FilterPostingsEnum) Cost() int64 {
	return f.in.Cost()
}

// Unwrap returns the wrapped enumerator, mirroring Unwrappable<PostingsEnum>.
func (f *FilterPostingsEnum) Unwrap() PostingsEnum {
	return f.in
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (f *FilterPostingsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(f, upTo, bitSet, offset)
}
