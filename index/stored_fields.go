// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// StoredFields provides access to stored fields for documents.
// This is the Go port of Lucene's org.apache.lucene.index.StoredFields.
//
// StoredFields allows retrieving stored field values for documents.
// It wraps a StoredFieldsReader and provides thread-safe access.
type StoredFields interface {
	// Prefetch prefetches stored fields for the given document IDs.
	// This is a hint to the implementation that these documents
	// will likely be accessed soon.
	Prefetch(docIDs []int) error

	// Document retrieves the stored fields for a single document.
	// The visitor callback receives each stored field value.
	Document(docID int, visitor StoredFieldVisitor) error
}

// StoredFieldsImpl is an implementation of StoredFields that wraps a StoredFieldsReader.
type StoredFieldsImpl struct {
	reader   StoredFieldsReader
	liveDocs util.Bits
	mu       sync.RWMutex
}

// NewStoredFields creates a new StoredFields from a StoredFieldsReader.
func NewStoredFields(reader StoredFieldsReader, liveDocs util.Bits) *StoredFieldsImpl {
	return &StoredFieldsImpl{
		reader:   reader,
		liveDocs: liveDocs,
	}
}

// Prefetch prefetches stored fields for the given document IDs.
func (sf *StoredFieldsImpl) Prefetch(docIDs []int) error {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	if sf.reader == nil {
		return fmt.Errorf("stored fields reader is closed")
	}

	// Default implementation: no-op
	// Subclasses can override to implement actual prefetching
	return nil
}

// Document retrieves the stored fields for a single document.
func (sf *StoredFieldsImpl) Document(docID int, visitor StoredFieldVisitor) error {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	if sf.reader == nil {
		return fmt.Errorf("stored fields reader is closed")
	}

	// Check if document is live
	if sf.liveDocs != nil {
		live := sf.liveDocs.Get(docID)
		if !live {
			return fmt.Errorf("document %d is deleted", docID)
		}
	}

	// Visit stored fields using the provided visitor
	return sf.reader.VisitDocument(docID, visitor)
}

// EmptyStoredFields is a StoredFields implementation with no stored fields.
type EmptyStoredFields struct{}

// NewEmptyStoredFields creates a new EmptyStoredFields.
func NewEmptyStoredFields() *EmptyStoredFields {
	return &EmptyStoredFields{}
}

// Prefetch does nothing.
func (e *EmptyStoredFields) Prefetch(docIDs []int) error {
	return nil
}

// Document does nothing and returns nil.
func (e *EmptyStoredFields) Document(docID int, visitor StoredFieldVisitor) error {
	return nil
}

// Ensure EmptyStoredFields implements StoredFields
var _ StoredFields = (*EmptyStoredFields)(nil)

// Ensure StoredFieldsImpl implements StoredFields
var _ StoredFields = (*StoredFieldsImpl)(nil)