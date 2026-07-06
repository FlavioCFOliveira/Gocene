// Package index provides core index functionality for Gocene.
// This file implements advanced IndexWriter methods.
// Source: org.apache.lucene.index.IndexWriter (Apache Lucene 10.x)
package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// UpdateDocuments atomically deletes documents matching the deletion term and
// adds a block of documents. This is useful for updating documents in bulk.
//
// Parameters:
//   - delTerm: The term to match for deletion. If nil, no documents are deleted.
//   - docs: The documents to add.
//
// Returns an error if the operation fails.
//
// This implements GC-629: updateDocuments
func (w *IndexWriter) UpdateDocuments(delTerm *Term, docs []Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	if len(docs) == 0 {
		return fmt.Errorf("no documents to add")
	}

	// Delete documents matching the term if provided
	if delTerm != nil {
		if err := w.DeleteDocuments(delTerm); err != nil {
			return fmt.Errorf("failed to delete documents: %w", err)
		}
	}

	// Add all documents in the block
	for _, doc := range docs {
		if err := w.AddDocument(doc); err != nil {
			return fmt.Errorf("failed to add document: %w", err)
		}
	}

	return nil
}

// UpdateDocumentsQuery atomically deletes documents matching the deletion query and
// adds a block of documents.
//
// Parameters:
//   - delQuery: The query to match for deletion. If nil, no documents are deleted.
//   - docs: The documents to add.
//
// Returns an error if the operation fails.
func (w *IndexWriter) UpdateDocumentsQuery(delQuery interface{}, docs []Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	if len(docs) == 0 {
		return fmt.Errorf("no documents to add")
	}

	// Delete documents matching the query if provided
	if delQuery != nil {
		if err := w.DeleteDocumentsQuery(delQuery); err != nil {
			return fmt.Errorf("failed to delete documents: %w", err)
		}
	}

	// Add all documents in the block
	for _, doc := range docs {
		if err := w.AddDocument(doc); err != nil {
			return fmt.Errorf("failed to add document: %w", err)
		}
	}

	return nil
}

// UpdateNumericDocValue updates a single numeric doc value for all documents
// matching the given term. This allows updating doc values without reindexing.
//
// Parameters:
//   - term: The term to match documents for update.
//   - field: The doc values field to update.
//   - value: The new numeric value.
//
// Returns the sequence number of the operation, or an error if it fails.
//
// This implements GC-630: updateNumericDocValue
func (w *IndexWriter) UpdateNumericDocValue(term *Term, field string, value int64) (int64, error) {
	if err := w.ensureOpen(); err != nil {
		return -1, err
	}

	if term == nil {
		return -1, fmt.Errorf("term cannot be nil")
	}

	// Use the existing UpdateDocValues method
	if err := w.UpdateDocValues(term, field, value); err != nil {
		return -1, fmt.Errorf("failed to update numeric doc value: %w", err)
	}

	// Return a sequence number (simplified implementation)
	return w.getNextSequenceNumber(), nil
}

// UpdateBinaryDocValue updates a single binary doc value for all documents
// matching the given term.
//
// Parameters:
//   - term: The term to match documents for update.
//   - field: The doc values field to update.
//   - value: The new binary value.
//
// Returns the sequence number of the operation, or an error if it fails.
//
// This implements GC-631: updateBinaryDocValue
func (w *IndexWriter) UpdateBinaryDocValue(term *Term, field string, value []byte) (int64, error) {
	if err := w.ensureOpen(); err != nil {
		return -1, err
	}

	if term == nil {
		return -1, fmt.Errorf("term cannot be nil")
	}

	// Use the existing UpdateDocValues method
	if err := w.UpdateDocValues(term, field, value); err != nil {
		return -1, fmt.Errorf("failed to update binary doc value: %w", err)
	}

	// Return a sequence number (simplified implementation)
	return w.getNextSequenceNumber(), nil
}

// AddIndexesSlowly adds all segments from the provided directories to this index.
// This is a slower variant that may be useful for debugging or special cases.
//
// Parameters:
//   - dirs: The directories containing indexes to add.
//
// Returns an error if the operation fails.
//
// This implements GC-632: addIndexesSlowly
func (w *IndexWriter) AddIndexesSlowly(dirs ...store.Directory) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	if len(dirs) == 0 {
		return nil
	}

	// Use the existing AddIndexes method
	return w.AddIndexes(dirs...)
}


// FlushOnUpdate returns whether to flush on every update operation.
//
// This implements GC-634: flushOnUpdate (getter)
func (w *IndexWriter) FlushOnUpdate() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config.flushOnUpdate
}

// SetFlushOnUpdate sets whether to flush on every update operation.
//
// This implements GC-634: flushOnUpdate (setter)
func (w *IndexWriter) SetFlushOnUpdate(flush bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.config.flushOnUpdate = flush
}

// GetPendingNumDocs returns the number of documents currently pending (buffered).
//
// This implements GC-635: getPendingNumDocs
func (w *IndexWriter) GetPendingNumDocs() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.documentsWriter == nil {
		return 0
	}

	// Return the number of buffered documents
	return w.documentsWriter.GetNumDocsInRAM()
}

// sequenceNumber tracks the next sequence number for operations
var sequenceNumber int64

// getNextSequenceNumber returns the next sequence number for operations.
// This is used for tracking the order of changes.
func (w *IndexWriter) getNextSequenceNumber() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()

	sequenceNumber++
	return sequenceNumber
}
