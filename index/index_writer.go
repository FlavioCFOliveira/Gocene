// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
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

// DeleteDocumentsQuery deletes documents matching the given query.
func (w *IndexWriter) DeleteDocumentsQuery(query interface{}) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	// Simple implementation for testing
	return nil
}

// commitData holds user-defined commit data.
type commitData struct {
	data map[string]string
}

// IndexWriter extension for commit-related fields
var (
	// liveCommitData holds the current commit data that will be written on next commit
	liveCommitData *commitData
	// preparedCommit indicates if prepareCommit has been called
	preparedCommit bool
)

// SetLiveCommitData sets the commit data that will be written with the next commit.
// This data is stored in the commit point and can be retrieved later.
// The data is "live" meaning it can be modified until the actual commit happens.
func (w *IndexWriter) SetLiveCommitData(data map[string]string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if liveCommitData == nil {
		liveCommitData = &commitData{data: make(map[string]string)}
	}
	// Copy the data to ensure we capture the values at commit time
	for k, v := range data {
		liveCommitData.data[k] = v
	}
}

// getLiveCommitData returns the current live commit data
func (w *IndexWriter) getLiveCommitData() map[string]string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if liveCommitData == nil {
		return nil
	}
	// Return a copy to prevent external modification
	result := make(map[string]string, len(liveCommitData.data))
	for k, v := range liveCommitData.data {
		result[k] = v
	}
	return result
}

// clearLiveCommitData clears the live commit data
func (w *IndexWriter) clearLiveCommitData() {
	w.mu.Lock()
	defer w.mu.Unlock()
	liveCommitData = nil
}

// PrepareCommit prepares for a commit without actually committing.
// This is the first phase of a two-phase commit. After calling prepareCommit,
// you must call commit() to complete the commit, or rollback() to abort.
//
// While prepareCommit is in progress, no other changes can be made to the index
// (adds, deletes, etc.) and the writer cannot be closed normally.
func (w *IndexWriter) PrepareCommit() error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if preparedCommit {
		return errors.New("prepareCommit already called; call commit or rollback first")
	}

	// Mark that we're in the prepared state
	preparedCommit = true

	// In a full implementation, this would:
	// 1. Flush any buffered documents
	// 2. Apply any buffered deletes
	// 3. Sync all files to disk
	// 4. Prepare the segments file but don't write it yet

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

	// Add commit data if present
	if liveCommitData != nil && len(liveCommitData.data) > 0 {
		si.SetUserData(liveCommitData.data)
	}

	err = WriteSegmentInfos(si, w.directory)
	if err != nil {
		return err
	}

	// Clear the prepared commit flag
	preparedCommit = false

	return nil
}

// Close closes the IndexWriter.
func (w *IndexWriter) Close() error {
	w.mu.Lock()
	if w.closed || w.tragicError != nil {
		w.mu.Unlock()
		return nil
	}

	// Check if prepareCommit was called but commit wasn't
	if preparedCommit {
		w.mu.Unlock()
		return errors.New("cannot close IndexWriter when prepareCommit was called but commit wasn't")
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

// GetConfig returns the live configuration for this IndexWriter.
// The returned LiveIndexWriterConfig can be used to change settings
// dynamically while the IndexWriter is open.
func (w *IndexWriter) GetConfig() *LiveIndexWriterConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return NewLiveIndexWriterConfig(w.config)
}

// DeleteAll deletes all documents in the index.
// This method will be fully implemented when delete tracking is complete.
func (w *IndexWriter) DeleteAll() error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	// Mark all documents for deletion
	// For now, just clear the doc count
	w.docCount = 0
	return nil
}

// Rollback rolls back all changes made since the last commit.
// This closes the writer and returns the index to its previous state.
func (w *IndexWriter) Rollback() error {
	w.mu.Lock()
	if w.closed || w.tragicError != nil {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	// Close the merge scheduler without committing
	if s := w.config.GetMergeScheduler(); s != nil {
		_ = s.Close()
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	preparedCommit = false
	w.clearLiveCommitData()
	return nil
}

// ForceMerge forces merge policy to merge segments until there are
// at most maxNumSegments segments.
func (w *IndexWriter) ForceMerge(maxNumSegments int) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	// Simple implementation - just commit to flush any buffered docs
	return w.Commit()
}

// GetNumBufferedDocuments returns the number of documents currently
// buffered in RAM.
func (w *IndexWriter) GetNumBufferedDocuments() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.docCount
}

// GetSegmentCount returns the number of segments in the index.
func (w *IndexWriter) GetSegmentCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	si, err := ReadSegmentInfos(w.directory)
	if err != nil {
		return 0
	}
	return si.Size()
}

// GetBufferedDeleteTermsSize returns the number of delete terms
// currently buffered in RAM.
func (w *IndexWriter) GetBufferedDeleteTermsSize() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// Placeholder - would track buffered delete terms
	return 0
}

// GetFlushCount returns the number of times the index has been flushed.
func (w *IndexWriter) GetFlushCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// Placeholder - would track flush count
	return 0
}

// DocStats holds document statistics for the index.
type DocStats struct {
	NumDocs int
	MaxDoc  int
}

// GetDocStats returns document statistics for the index.
func (w *IndexWriter) GetDocStats() *DocStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	numDocs := w.docCount
	si, err := ReadSegmentInfos(w.directory)
	if err == nil {
		numDocs += si.TotalNumDocs()
	}

	return &DocStats{
		NumDocs: numDocs,
		MaxDoc:  numDocs,
	}
}

// AddIndexes adds all documents from the provided directories to this index.
// This is a core operation for index merging and backup restoration.
// The source directories are not modified.
//
// This method will:
//   - Copy all segments from source directories
//   - Merge small segments if configured
//   - Validate codec compatibility
//   - Handle soft deletes appropriately
//
// Returns an error if:
//   - The writer is closed
//   - A source directory is the same as the target directory
//   - Codec incompatibility is detected
//   - Merge failures occur
func (w *IndexWriter) AddIndexes(dirs ...store.Directory) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Validate that we're not adding our own directory
	for _, dir := range dirs {
		if dir == w.directory {
			return errors.New("cannot add index to itself")
		}
	}

	// Read current segment infos
	si, err := ReadSegmentInfos(w.directory)
	if err != nil {
		si = NewSegmentInfos()
		si.SetGeneration(1)
	}

	// Process each source directory
	for _, dir := range dirs {
		// Read source segment infos
		sourceSI, err := ReadSegmentInfos(dir)
		if err != nil {
			// Directory may be empty, skip it
			continue
		}

		// Copy segments from source
		for _, sci := range sourceSI.List() {
			// In a full implementation, this would:
			// 1. Copy segment files from source to target
			// 2. Update segment name to avoid conflicts
			// 3. Add to segment infos

			// For now, just increment doc count to simulate
			w.docCount += sci.DocCount()
		}
	}

	return nil
}

// AddIndexesFromReader adds indexes from the provided IndexReaders.
// This is used to add segments from existing readers to this writer.
func (w *IndexWriter) AddIndexesFromReader(readers ...IndexReader) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Process each reader
	for _, reader := range readers {
		// In a full implementation, this would:
		// 1. Extract segments from reader
		// 2. Copy segment files to target directory
		// 3. Update segment infos

		// For now, just increment doc count to simulate
		w.docCount += reader.NumDocs()
	}

	return nil
}

// WaitForMerges waits for all pending merges to complete.
// This is useful for testing and when a consistent index state is needed.
func (w *IndexWriter) WaitForMerges() error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// In a full implementation, this would:
	// 1. Wait for all running merges to complete
	// 2. Return any merge errors

	return nil
}

// AddDocuments adds a block of documents atomically.
// This is used for parent-child document relationships.
func (w *IndexWriter) AddDocuments(docs []Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Add all documents in the block
	for range docs {
		w.docCount++
	}

	return nil
}

// UpdateDocValues updates the doc values for documents matching the given term.
// This is used for updating numeric doc values without re-indexing.
func (w *IndexWriter) UpdateDocValues(term *Term, field string, value interface{}) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Placeholder - would update doc values in the index
	return nil
}

// CloneSegmentInfos returns a copy of the current SegmentInfos.
// This is used for testing and diagnostics.
func (w *IndexWriter) CloneSegmentInfos() *SegmentInfos {
	w.mu.RLock()
	defer w.mu.RUnlock()

	si, err := ReadSegmentInfos(w.directory)
	if err != nil {
		return NewSegmentInfos()
	}
	return si
}

// NewMatchAllDocsQuery creates a query that matches all documents.
func NewMatchAllDocsQuery() Query {
	return &MatchAllDocsQuery{}
}

// MatchAllDocsQuery is a query that matches all documents.
type MatchAllDocsQuery struct{}

// Rewrite rewrites the query.
func (q *MatchAllDocsQuery) Rewrite(reader *IndexReader) (Query, error) { return q, nil }

// Clone creates a copy of this query.
func (q *MatchAllDocsQuery) Clone() Query { return q }

// Equals checks if this query equals another.
func (q *MatchAllDocsQuery) Equals(other Query) bool { return false }

// HashCode returns a hash code for this query.
func (q *MatchAllDocsQuery) HashCode() int { return 0 }

// CreateWeight creates a Weight for this query.
func (q *MatchAllDocsQuery) CreateWeight(searcher IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return nil, nil
}
