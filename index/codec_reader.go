// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// CodecReader is a LeafReader that reads from a codec.
// This is the Go port of Lucene's org.apache.lucene.index.CodecReader.
//
// CodecReader is the bridge between LeafReader and SegmentReader.
// It provides access to the underlying codec readers for postings,
// stored fields, and term vectors.
type CodecReader struct {
	*LeafReader

	// coreReaders holds the shared core readers for this segment
	coreReaders *SegmentCoreReaders

	// liveDocs indicates which documents are live (nil if all docs are live)
	liveDocs util.Bits

	// numDocs is the number of live documents
	numDocs int
}

// NewCodecReader creates a new CodecReader for the given segment.
func NewCodecReader(
	coreReaders *SegmentCoreReaders,
	liveDocs util.Bits,
	numDocs int,
) *CodecReader {
	// Create a minimal SegmentInfo for the LeafReader
	// The actual segment info should come from the core readers or be passed in
	segmentInfo := &SegmentInfo{
		name:     coreReaders.GetSegmentName(),
		docCount: 0, // Will be set properly when needed
	}
	return &CodecReader{
		LeafReader:  NewLeafReaderWithFieldInfos(segmentInfo, coreReaders.GetFieldInfos()),
		coreReaders: coreReaders,
		liveDocs:    liveDocs,
		numDocs:     numDocs,
	}
}

// GetCoreReaders returns the SegmentCoreReaders.
func (r *CodecReader) GetCoreReaders() *SegmentCoreReaders {
	return r.coreReaders
}

// DocCount returns the total number of documents (including deleted).
func (r *CodecReader) DocCount() int {
	if r.coreReaders == nil {
		return 0
	}
	// Return maxDoc from segment info
	return r.LeafReader.DocCount()
}

// NumDocs returns the number of live documents.
func (r *CodecReader) NumDocs() int {
	return r.numDocs
}

// MaxDoc returns the maximum document ID plus one.
func (r *CodecReader) MaxDoc() int {
	return r.LeafReader.MaxDoc()
}

// HasDeletions returns true if this reader has deleted documents.
func (r *CodecReader) HasDeletions() bool {
	return r.liveDocs != nil
}

// NumDeletedDocs returns the number of deleted documents.
func (r *CodecReader) NumDeletedDocs() int {
	return r.MaxDoc() - r.numDocs
}

// GetLiveDocs returns the live docs Bits, or nil if all docs are live.
func (r *CodecReader) GetLiveDocs() util.Bits {
	return r.liveDocs
}

// GetFieldInfos returns the FieldInfos for this reader.
func (r *CodecReader) GetFieldInfos() *FieldInfos {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetFieldInfos()
}

// Terms returns the Terms for a field.
func (r *CodecReader) Terms(field string) (Terms, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("codec reader not initialized")
	}
	fields := r.coreReaders.GetFields()
	if fields == nil {
		return nil, nil
	}
	return fields.Terms(field)
}

// StoredFields returns a StoredFields instance for accessing stored fields.
func (r *CodecReader) StoredFields() (StoredFields, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("codec reader not initialized")
	}
	sfReader := r.coreReaders.GetStoredFieldsReader()
	if sfReader == nil {
		return NewEmptyStoredFields(), nil
	}
	return NewStoredFields(sfReader, r.liveDocs), nil
}

// TermVectors returns a TermVectors instance for accessing term vectors.
func (r *CodecReader) TermVectors() (TermVectors, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("codec reader not initialized")
	}
	tvReader := r.coreReaders.GetTermVectorsReader()
	if tvReader == nil {
		return NewEmptyTermVectors(), nil
	}
	return NewTermVectors(tvReader, r.liveDocs), nil
}

// GetTermVectors returns the term vectors for a document.
func (r *CodecReader) GetTermVectors(docID int) (Fields, error) {
	tv, err := r.TermVectors()
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, nil
	}
	return tv.Get(docID)
}

// IncRef increments the reference count on the core readers.
func (r *CodecReader) IncRef() error {
	if r.coreReaders == nil {
		return fmt.Errorf("codec reader not initialized")
	}
	return r.coreReaders.IncRef()
}

// DecRef decrements the reference count on the core readers.
func (r *CodecReader) DecRef() error {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.DecRef()
}

// TryIncRef tries to increment the reference count.
func (r *CodecReader) TryIncRef() bool {
	if r.coreReaders == nil {
		return false
	}
	return r.coreReaders.GetRefCount() > 0 && r.coreReaders.IncRef() == nil
}

// GetRefCount returns the current reference count.
func (r *CodecReader) GetRefCount() int32 {
	if r.coreReaders == nil {
		return 0
	}
	return r.coreReaders.GetRefCount()
}

// Close closes the codec reader.
func (r *CodecReader) Close() error {
	return r.DecRef()
}

// GetCoreCacheKey returns the core cache key for this reader.
func (r *CodecReader) GetCoreCacheKey() interface{} {
	if r.coreReaders == nil {
		return nil
	}
	return NewCoreCacheKey(r.coreReaders.GetSegmentName())
}

// GetTermVectorsReader returns the TermVectorsReader for this segment.
// Returns nil if term vectors are not available.
func (r *CodecReader) GetTermVectorsReader() TermVectorsReader {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetTermVectorsReader()
}

// GetStoredFieldsReader returns the StoredFieldsReader for this segment.
func (r *CodecReader) GetStoredFieldsReader() StoredFieldsReader {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetStoredFieldsReader()
}

// GetFieldsReader returns the FieldsProducer for this segment.
func (r *CodecReader) GetFieldsReader() FieldsProducer {
	if r.coreReaders == nil {
		return nil
	}
	return r.coreReaders.GetFields()
}

// Ensure CodecReader implements the expected interfaces
var _ LeafReaderInterface = (*CodecReader)(nil)

// LeafReaderInterface defines the interface for a leaf reader.
// This is separated from IndexReaderInterface to allow for type assertions.
type LeafReaderInterface interface {
	IndexReaderInterface
	// LeafReader specific methods
	GetCoreCacheKey() interface{}
	GetTermVectors(docID int) (Fields, error)
	Terms(field string) (Terms, error)
}
