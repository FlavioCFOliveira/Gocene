// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// MultiReader is a CompositeReader that reads from multiple indexes.
// This is the Go port of Lucene's org.apache.lucene.index.MultiReader.
//
// MultiReader allows searching across multiple indexes as if they were a single index.
// It is useful for applications that need to search across multiple separate indexes
// without merging them.
type MultiReader struct {
	*BaseCompositeReader

	// readers are the sub-readers
	readers []IndexReaderInterface

	// closed indicates if this reader has been closed
	closed bool
}

// NewMultiReader creates a new MultiReader from the given sub-readers.
//
// The subReaders must all be IndexReader instances from different indexes.
// If closeAllSubReaders is true, closing this MultiReader will close all sub-readers.
func NewMultiReader(subReaders []IndexReaderInterface) (*MultiReader, error) {
	if len(subReaders) == 0 {
		return nil, fmt.Errorf("subReaders array must be non-empty")
	}

	// Create base composite reader
	baseReader, err := NewBaseCompositeReader(subReaders)
	if err != nil {
		return nil, err
	}

	reader := &MultiReader{
		BaseCompositeReader: baseReader,
		readers:             make([]IndexReaderInterface, len(subReaders)),
	}

	// Copy sub-readers
	copy(reader.readers, subReaders)

	return reader, nil
}

// GetSequentialSubReaders returns the sub-readers in sequential order.
func (r *MultiReader) GetSequentialSubReaders() []IndexReaderInterface {
	return r.readers
}

// Close closes this reader and all sub-readers.
func (r *MultiReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true

	// Close all sub-readers
	var lastErr error
	for _, reader := range r.readers {
		if err := reader.Close(); err != nil {
			lastErr = err
		}
	}

	// Close base composite reader
	if err := r.BaseCompositeReader.closeInternal(); err != nil {
		lastErr = err
	}

	return lastErr
}

// IsCurrent returns true if all sub-readers are current.
func (r *MultiReader) IsCurrent() (bool, error) {
	for _, reader := range r.readers {
		if currentReader, ok := reader.(interface{ IsCurrent() (bool, error) }); ok {
			isCurrent, err := currentReader.IsCurrent()
			if err != nil {
				return false, err
			}
			if !isCurrent {
				return false, nil
			}
		}
	}
	return true, nil
}

// GetTermVectors returns the term vectors for a document across all sub-readers.
func (r *MultiReader) GetTermVectors(docID int) (Fields, error) {
	// Find the sub-reader for this document
	readerIndex := r.ReaderIndex(docID)
	if readerIndex < 0 || readerIndex >= len(r.readers) {
		return nil, fmt.Errorf("document ID %d out of range", docID)
	}

	// Calculate local doc ID
	localDocID := docID - r.ReaderBase(readerIndex)

	// Get term vectors from sub-reader
	if leafReader, ok := r.readers[readerIndex].(LeafReaderInterface); ok {
		return leafReader.GetTermVectors(localDocID)
	}

	return nil, fmt.Errorf("sub-reader %d does not support term vectors", readerIndex)
}

// Terms returns the Terms for a field across all sub-readers.
// Note: This returns terms from the first sub-reader that has the field.
func (r *MultiReader) Terms(field string) (Terms, error) {
	for _, reader := range r.readers {
		if leafReader, ok := reader.(LeafReaderInterface); ok {
			terms, err := leafReader.Terms(field)
			if err != nil {
				return nil, err
			}
			if terms != nil {
				return terms, nil
			}
		}
	}
	return nil, nil
}

// StoredFields returns a StoredFields for accessing stored fields.
func (r *MultiReader) StoredFields() (StoredFields, error) {
	return &multiReaderStoredFields{reader: r}, nil
}

// TermVectors returns a TermVectors for accessing term vectors.
func (r *MultiReader) TermVectors() (TermVectors, error) {
	return &multiReaderTermVectors{reader: r}, nil
}

// multiReaderStoredFields wraps MultiReader to provide StoredFields access.
type multiReaderStoredFields struct {
	reader *MultiReader
}

// Prefetch prefetches stored fields for the given document IDs.
func (msf *multiReaderStoredFields) Prefetch(docIDs []int) error {
	// No-op for now
	return nil
}

// Document retrieves the stored fields for a document using the visitor pattern.
func (msf *multiReaderStoredFields) Document(docID int, visitor StoredFieldVisitor) error {
	// Find the sub-reader for this document
	readerIndex := msf.reader.ReaderIndex(docID)
	if readerIndex < 0 || readerIndex >= len(msf.reader.readers) {
		return fmt.Errorf("document ID %d out of range", docID)
	}

	// Calculate local doc ID
	localDocID := docID - msf.reader.ReaderBase(readerIndex)

	// Get stored fields from sub-reader
	subReader := msf.reader.readers[readerIndex]
	if readerWithStoredFields, ok := subReader.(interface {
		StoredFields() (StoredFields, error)
	}); ok {
		sf, err := readerWithStoredFields.StoredFields()
		if err != nil {
			return err
		}
		if sf != nil {
			return sf.Document(localDocID, visitor)
		}
	}

	return nil
}

// multiReaderTermVectors wraps MultiReader to provide TermVectors access.
type multiReaderTermVectors struct {
	reader *MultiReader
}

// Prefetch prefetches term vectors for the given document IDs.
func (mtv *multiReaderTermVectors) Prefetch(docIDs []int) error {
	// No-op for now
	return nil
}

// Get retrieves the term vectors for a document.
func (mtv *multiReaderTermVectors) Get(docID int) (Fields, error) {
	return mtv.reader.GetTermVectors(docID)
}

// GetField retrieves the term vector for a specific field in a document.
func (mtv *multiReaderTermVectors) GetField(docID int, field string) (Terms, error) {
	fields, err := mtv.Get(docID)
	if err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, nil
	}
	return fields.Terms(field)
}

// Ensure MultiReader implements IndexReaderInterface
var _ IndexReaderInterface = (*MultiReader)(nil)

// Ensure MultiReader implements LeafReaderInterface
var _ LeafReaderInterface = (*MultiReader)(nil)
