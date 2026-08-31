// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilterCodecReader contains another CodecReader, which it uses as its basic source
// of data, possibly transforming the data along the way or providing additional functionality.
//
// This is the Go port of Lucene's org.apache.lucene.index.FilterCodecReader.
type FilterCodecReader struct {
	in *CodecReader

	// Overrides for WrapLiveDocs
	liveDocsOverride util.Bits
	numDocsOverride  *int
}

// NewFilterCodecReader creates a new FilterCodecReader wrapping the given reader.
func NewFilterCodecReader(in *CodecReader) *FilterCodecReader {
	return &FilterCodecReader{
		in: in,
	}
}

// Unwrap returns the wrapped instance by reader as long as this reader is an instance of FilterCodecReader.
func Unwrap(reader LeafReaderInterface) LeafReaderInterface {
	for {
		if delegate, ok := reader.(interface{ GetDelegate() LeafReaderInterface }); ok {
			reader = delegate.GetDelegate()
		} else {
			break
		}
	}
	return reader
}

// GetDelegate returns the wrapped CodecReader.
func (r *FilterCodecReader) GetDelegate() LeafReaderInterface {
	return r.in
}

// WrapLiveDocs returns a filtered codec reader with the given live docs and numDocs.
func WrapLiveDocs(reader *CodecReader, liveDocs util.Bits, numDocs int) *FilterCodecReader {
	return &FilterCodecReader{
		in:               reader,
		liveDocsOverride: liveDocs,
		numDocsOverride:  &numDocs,
	}
}

// --- Delegation Methods ---

func (r *FilterCodecReader) Close() error {
	return r.in.Close()
}

func (r *FilterCodecReader) DocCount() int {
	return r.in.DocCount()
}

func (r *FilterCodecReader) NumDocs() int {
	if r.numDocsOverride != nil {
		return *r.numDocsOverride
	}
	return r.in.NumDocs()
}

func (r *FilterCodecReader) MaxDoc() int {
	return r.in.MaxDoc()
}

func (r *FilterCodecReader) HasDeletions() bool {
	return r.in.HasDeletions()
}

func (r *FilterCodecReader) NumDeletedDocs() int {
	return r.in.NumDeletedDocs()
}

func (r *FilterCodecReader) GetLiveDocs() util.Bits {
	if r.liveDocsOverride != nil {
		return r.liveDocsOverride
	}
	return r.in.GetLiveDocs()
}

func (r *FilterCodecReader) GetFieldInfos() *FieldInfos {
	return r.in.GetFieldInfos()
}

func (r *FilterCodecReader) Terms(field string) (Terms, error) {
	return r.in.Terms(field)
}

func (r *FilterCodecReader) GetTermVectors(docID int) (Fields, error) {
	return r.in.GetTermVectors(docID)
}

func (r *FilterCodecReader) StoredFields() (StoredFields, error) {
	return r.in.StoredFields()
}

func (r *FilterCodecReader) TermVectors() (TermVectors, error) {
	return r.in.TermVectors()
}

func (r *FilterCodecReader) GetCoreReaders() *SegmentCoreReaders {
	return r.in.GetCoreReaders()
}

func (r *FilterCodecReader) GetCoreCacheKey() interface{} {
	return r.in.GetCoreCacheKey()
}

func (r *FilterCodecReader) GetTermVectorsReader() TermVectorsReader {
	return r.in.GetTermVectorsReader()
}

func (r *FilterCodecReader) GetStoredFieldsReader() StoredFieldsReader {
	return r.in.GetStoredFieldsReader()
}

func (r *FilterCodecReader) GetFieldsReader() FieldsProducer {
	return r.in.GetFieldsReader()
}

func (r *FilterCodecReader) GetPostingsReader() FieldsProducer {
	return r.in.GetPostingsReader()
}

func (r *FilterCodecReader) GetDocValuesReader() interface{} {
	return r.in.GetDocValuesReader()
}

func (r *FilterCodecReader) GetNormsReader() interface{} {
	return r.in.GetNormsReader()
}

func (r *FilterCodecReader) GetPointsReader() interface{} {
	return r.in.GetPointsReader()
}

func (r *FilterCodecReader) GetVectorReader() interface{} {
	return r.in.GetVectorReader()
}

func (r *FilterCodecReader) IncRef() error {
	return r.in.IncRef()
}

func (r *FilterCodecReader) DecRef() error {
	return r.in.DecRef()
}

func (r *FilterCodecReader) TryIncRef() bool {
	return r.in.TryIncRef()
}

func (r *FilterCodecReader) GetRefCount() int32 {
	return r.in.GetRefCount()
}

func (r *FilterCodecReader) EnsureOpen() error {
	return r.in.EnsureOpen()
}

func (r *FilterCodecReader) GetContext() (IndexReaderContext, error) {
	return r.in.GetContext()
}

func (r *FilterCodecReader) Leaves() ([]*LeafReaderContext, error) {
	return r.in.Leaves()
}

func (r *FilterCodecReader) CheckIntegrity() error {
	return r.in.CheckIntegrity()
}

func (r *FilterCodecReader) GetMetaData() *IndexReaderMetaData {
	return r.in.GetMetaData()
}

func (r *FilterCodecReader) GetSegmentInfo() *SegmentInfo {
	return r.in.GetSegmentInfo()
}
