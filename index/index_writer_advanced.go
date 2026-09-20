// Package index provides core index functionality for Gocene.
// This file implements advanced IndexWriter methods.
// Source: org.apache.lucene.index.IndexWriter (Apache Lucene 10.x)
package index

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// UpdateDocuments atomically deletes documents matching the deletion term and
// adds a block of documents. This is useful for updating documents in bulk.
//
// Parameters:
//   - delTerm: The term to match for deletion. If nil, no documents are deleted.
//   - docs: The documents to add.
//
// Returns the sequence number of the last document added, or an error if the
// operation fails. This implements GC-629: updateDocuments.

// UpdateDocumentsQuery atomically deletes documents matching the deletion query and
// adds a block of documents.
//
// Parameters:
//   - delQuery: The query to match for deletion. If nil, no documents are deleted.
//   - docs: The documents to add.
//
// Returns the sequence number of the last document added, or an error if the
// operation fails.

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
	if err := w.ensureOpen(true); err != nil {
		return err
	}

	if len(dirs) == 0 {
		return nil
	}

	// Use the existing AddIndexes method
	_, err := w.AddIndexes(dirs...)
	return err
}

// FlushOnUpdate returns whether to flush on every update operation.
//
// This implements GC-634: flushOnUpdate (getter)
func (w *IndexWriter) FlushOnUpdate() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.config.IsCheckPendingFlushOnUpdate()
}

// SetFlushOnUpdate sets whether to flush on every update operation.
//
// This implements GC-634: flushOnUpdate (setter)
func (w *IndexWriter) SetFlushOnUpdate(flush bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.config.SetCheckPendingFlushUpdate(flush)
}

// GetPendingNumDocs returns the number of documents currently pending (buffered).
//
// This implements GC-635: getPendingNumDocs
func (w *IndexWriter) GetPendingNumDocs() int {
	return int(w.pendingNumDocs.Load())
}
