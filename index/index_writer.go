// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// Document represents a document to be indexed.
// This is a minimal interface to avoid circular imports.
type Document interface {
	GetFields() []interface{}
}

// IndexWriter writes and maintains an index.
type IndexWriter struct {
	directory store.Directory
	config    *IndexWriterConfig
	closed    bool
	docCount  int
	mu        sync.RWMutex

	// tragicError holds any unrecoverable error that occurred during an operation.
	// Once set, the writer is considered closed and all subsequent operations will fail.
	tragicError error
}

// NewIndexWriter creates a new IndexWriter.
func NewIndexWriter(dir store.Directory, config *IndexWriterConfig) (*IndexWriter, error) {
	if config.GetMergeScheduler() == nil {
		config.SetMergeScheduler(NewConcurrentMergeScheduler())
	}

	return &IndexWriter{
		directory: dir,
		config:    config,
		closed:    false,
	}, nil
}

// ensureOpen checks if the writer is closed or has encountered a tragic error.
func (w *IndexWriter) ensureOpen() error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.tragicError != nil {
		return NewAlreadyClosedException("tragic error occurred", w.tragicError)
	}
	if w.closed {
		return NewAlreadyClosedException("IndexWriter is closed", nil)
	}
	return nil
}

// setTragicError sets the tragic error and prevents further operations.
func (w *IndexWriter) setTragicError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.tragicError == nil {
		w.tragicError = err
	}
}

// AddDocument adds a document to the index.
func (w *IndexWriter) AddDocument(doc Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.docCount++

	return nil
}

// UpdateDocument updates a document in the index.
func (w *IndexWriter) UpdateDocument(term *Term, doc Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	// Simple implementation for testing
	return nil
}

// DeleteDocuments deletes documents matching the given term.
func (w *IndexWriter) DeleteDocuments(term *Term) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	// Simple implementation for testing
	return nil
}

// Commit commits all pending changes.
func (w *IndexWriter) Commit() error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Simple implementation for testing: persist segments info
	si, err := ReadSegmentInfos(w.directory)
	if err != nil {
		// No segments file yet, create new one
		si = NewSegmentInfos()
		si.SetGeneration(1)
	} else {
		// Advance generation
		si.NextGeneration()
	}

	// Create a dummy segment if we have documents
	if w.docCount > 0 {
		segmentName := si.GetNextSegmentName()
		segmentInfo := NewSegmentInfo(segmentName, w.docCount, nil)
		sci := NewSegmentCommitInfo(segmentInfo, 0, -1)
		si.Add(sci)
		w.docCount = 0 // Documents "flushed" to segment
	}

	err = WriteSegmentInfos(si, w.directory)
	if err != nil {
		return err
	}

	return nil
}

// Close closes the IndexWriter.
func (w *IndexWriter) Close() error {
	w.mu.Lock()
	if w.closed || w.tragicError != nil {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	// Try to commit changes before closing
	if err := w.Commit(); err != nil {
		// If commit fails, we still want to close the scheduler
		if s := w.config.GetMergeScheduler(); s != nil {
			_ = s.Close()
		}
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true

	// Close the merge scheduler
	if s := w.config.GetMergeScheduler(); s != nil {
		return s.Close()
	}

	return nil
}

// NumDocs returns the number of documents in the index.
func (w *IndexWriter) NumDocs() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// In a real implementation, this would involve reading SegmentInfos
	// and accounting for deleted documents.
	si, err := ReadSegmentInfos(w.directory)
	if err != nil {
		return w.docCount
	}
	return si.TotalNumDocs() + w.docCount
}

// MaxDoc returns the maximum document ID.
func (w *IndexWriter) MaxDoc() int {
	return w.NumDocs()
}

// IsClosed returns true if the writer is closed.
func (w *IndexWriter) IsClosed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.closed || w.tragicError != nil
}
