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
	in *LeafReader
}

// NewFilterLeafReader creates a new FilterLeafReader wrapping the given reader.
func NewFilterLeafReader(in *LeafReader) *FilterLeafReader {
	return &FilterLeafReader{
		LeafReader: in,
		in:         in,
	}
}

// Close closes the wrapped reader.
func (r *FilterLeafReader) Close() error {
	if closer, ok := interface{}(r.in).(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// GetDelegate returns the wrapped LeafReader.
func (r *FilterLeafReader) GetDelegate() *LeafReader {
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

// EnsureOpen throws an error if the reader is closed.
func (r *FilterLeafReader) EnsureOpen() error {
	return r.in.EnsureOpen()
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
type FilterCodecReader struct {
	*CodecReader
	in *CodecReader
}

// NewFilterCodecReader creates a new FilterCodecReader wrapping the given reader.
func NewFilterCodecReader(in *CodecReader) *FilterCodecReader {
	return &FilterCodecReader{
		CodecReader: in,
		in:          in,
	}
}

// GetDelegate returns the wrapped CodecReader.
func (r *FilterCodecReader) GetDelegate() *CodecReader {
	return r.in
}

// Close closes the wrapped reader.
func (r *FilterCodecReader) Close() error {
	return r.in.Close()
}

// DocCount returns the total number of documents.
func (r *FilterCodecReader) DocCount() int {
	return r.in.DocCount()
}

// NumDocs returns the number of live documents.
func (r *FilterCodecReader) NumDocs() int {
	return r.in.NumDocs()
}

// MaxDoc returns the maximum document ID plus one.
func (r *FilterCodecReader) MaxDoc() int {
	return r.in.MaxDoc()
}

// HasDeletions returns true if this reader has deleted documents.
func (r *FilterCodecReader) HasDeletions() bool {
	return r.in.HasDeletions()
}

// NumDeletedDocs returns the number of deleted documents.
func (r *FilterCodecReader) NumDeletedDocs() int {
	return r.in.NumDeletedDocs()
}

// GetLiveDocs returns the live docs Bits, or nil if all docs are live.
func (r *FilterCodecReader) GetLiveDocs() util.Bits {
	return r.in.GetLiveDocs()
}

// GetFieldInfos returns the FieldInfos for this reader.
func (r *FilterCodecReader) GetFieldInfos() *FieldInfos {
	return r.in.GetFieldInfos()
}

// Terms returns the Terms for a field.
func (r *FilterCodecReader) Terms(field string) (Terms, error) {
	return r.in.Terms(field)
}

// GetTermVectors returns the term vectors for a document.
func (r *FilterCodecReader) GetTermVectors(docID int) (Fields, error) {
	return r.in.GetTermVectors(docID)
}

// StoredFields returns a StoredFields instance for accessing stored fields.
func (r *FilterCodecReader) StoredFields() (StoredFields, error) {
	return r.in.StoredFields()
}

// TermVectors returns a TermVectors instance for accessing term vectors.
func (r *FilterCodecReader) TermVectors() (TermVectors, error) {
	return r.in.TermVectors()
}

// GetCoreReaders returns the SegmentCoreReaders.
func (r *FilterCodecReader) GetCoreReaders() *SegmentCoreReaders {
	return r.in.GetCoreReaders()
}

// GetCoreCacheKey returns the core cache key for this reader.
func (r *FilterCodecReader) GetCoreCacheKey() interface{} {
	return r.in.GetCoreCacheKey()
}

// GetTermVectorsReader returns the TermVectorsReader for this segment.
func (r *FilterCodecReader) GetTermVectorsReader() TermVectorsReader {
	return r.in.GetTermVectorsReader()
}

// GetStoredFieldsReader returns the StoredFieldsReader for this segment.
func (r *FilterCodecReader) GetStoredFieldsReader() StoredFieldsReader {
	return r.in.GetStoredFieldsReader()
}

// GetFieldsReader returns the FieldsProducer for this segment.
func (r *FilterCodecReader) GetFieldsReader() FieldsProducer {
	return r.in.GetFieldsReader()
}

// GetPostingsReader returns the FieldsProducer for this segment.
func (r *FilterCodecReader) GetPostingsReader() FieldsProducer {
	return r.in.GetPostingsReader()
}

// GetDocValuesReader returns the DocValuesProducer for this segment.
func (r *FilterCodecReader) GetDocValuesReader() interface{} {
	return r.in.GetDocValuesReader()
}

// GetNormsReader returns the NormsProducer for this segment.
func (r *FilterCodecReader) GetNormsReader() interface{} {
	return r.in.GetNormsReader()
}

// GetPointsReader returns the PointsReader for this segment.
func (r *FilterCodecReader) GetPointsReader() interface{} {
	return r.in.GetPointsReader()
}

// GetVectorReader returns the KnnVectorsReader for this segment.
func (r *FilterCodecReader) GetVectorReader() interface{} {
	return r.in.GetVectorReader()
}

// IncRef increments the reference count on the core readers.
func (r *FilterCodecReader) IncRef() error {
	return r.in.IncRef()
}

// DecRef decrements the reference count on the core readers.
func (r *FilterCodecReader) DecRef() error {
	return r.in.DecRef()
}

// TryIncRef tries to increment the reference count.
func (r *FilterCodecReader) TryIncRef() bool {
	return r.in.TryIncRef()
}

// GetRefCount returns the current reference count.
func (r *FilterCodecReader) GetRefCount() int32 {
	return r.in.GetRefCount()
}

// EnsureOpen throws an error if the reader is closed.
func (r *FilterCodecReader) EnsureOpen() error {
	return r.in.EnsureOpen()
}

// GetContext returns the reader context for this leaf reader.
func (r *FilterCodecReader) GetContext() (IndexReaderContext, error) {
	return r.in.GetContext()
}

// Leaves returns all leaf reader contexts (just this one for a leaf).
func (r *FilterCodecReader) Leaves() ([]*LeafReaderContext, error) {
	return r.in.Leaves()
}
