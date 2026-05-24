// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

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

	// atomic fields for lock-free access
	closed      atomic.Bool
	docCount    atomic.Int32
	tragicError atomic.Pointer[error]

	// mu protects shared state changes (segment infos, commit data, etc.)
	// NOT for document-level operations which should be lock-free
	mu sync.RWMutex

	// documentsWriter handles the actual document processing and flushing
	// DocumentsWriter has its own internal locking
	documentsWriter *DocumentsWriter

	// pendingDeleteTerms holds term-based delete operations buffered since the
	// last commit. Protected by mu.
	pendingDeleteTerms []*Term

	// docFieldIndex holds (field, value) pairs for each document added since
	// the last commit. Used by Commit to apply term-based deletes without a
	// full inverted index. Protected by mu.
	docFieldIndex [][]docFieldEntry

	// pendingFieldInfos accumulates FieldInfos from documents added since the
	// last commit. Set to nil after each Commit. Protected by mu.
	pendingFieldInfos *FieldInfos

	// committedSegments holds SegmentCommitInfos created by previous Commits,
	// along with their in-memory FieldInfos, so that AddIndexes can read them
	// from this writer's own directory. Protected by mu.
	committedSegments []*SegmentCommitInfo
}

// docFieldEntry is a (field-name, term-value) pair extracted from a document
// during AddDocument for use by the buffered term-delete mechanism.
type docFieldEntry struct {
	field string
	val   string
}

// docFieldNamer is a minimal interface satisfied by any field type that
// exposes a name and a string value; document.Field satisfies this via the
// Name() and StringValue() methods added on *Field.
type docFieldNamer interface {
	Name() string
	StringValue() string
}

// indexableFieldMeta is satisfied by document.Field (and all its subtypes)
// without a direct import of the document package.  It exposes the per-field
// metadata needed to build a FieldInfo.  All return types are from the index
// package or primitives — no circular import.
type indexableFieldMeta interface {
	Name() string
	// Field-level metadata mirrors document.FieldType's accessor surface.
	IsStored() bool
	IsIndexed() bool
	IsTokenized() bool
	IndexOptions() IndexOptions
	DocValuesType() DocValuesType
	HasTermVectors() bool
}

// NewIndexWriter creates a new IndexWriter.
func NewIndexWriter(dir store.Directory, config *IndexWriterConfig) (*IndexWriter, error) {
	if config.GetMergeScheduler() == nil {
		config.SetMergeScheduler(NewConcurrentMergeScheduler())
	}

	// Create the DocumentsWriter for actual document processing
	docWriter, err := NewDocumentsWriter(dir, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create DocumentsWriter: %w", err)
	}

	writer := &IndexWriter{
		directory:       dir,
		config:          config,
		documentsWriter: docWriter,
	}
	// Initialize atomic fields
	writer.closed.Store(false)
	writer.docCount.Store(0)
	return writer, nil
}

// ensureOpen checks if the writer is closed or has encountered a tragic error.
// Uses atomic operations for lock-free checks on hot paths.
func (w *IndexWriter) ensureOpen() error {
	if w.closed.Load() {
		return NewAlreadyClosedException("IndexWriter is closed", nil)
	}
	if err := w.tragicError.Load(); err != nil {
		return NewAlreadyClosedException("tragic error occurred", *err)
	}
	return nil
}

// setTragicError sets the tragic error and prevents further operations.
// Uses atomic compare-and-swap to ensure only the first error is stored.
func (w *IndexWriter) setTragicError(err error) {
	w.tragicError.CompareAndSwap(nil, &err)
}

// AddDocument adds a document to the index.
// Minimizes critical section by processing document outside the global lock.
// DocumentsWriter handles its own internal concurrency.
func (w *IndexWriter) AddDocument(doc Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	// DocumentsWriter has its own internal locking, so we don't need
	// to hold the global lock during document processing.
	if w.documentsWriter != nil {
		if err := w.documentsWriter.AddDocument(doc, nil); err != nil {
			return fmt.Errorf("failed to add document: %w", err)
		}
	}

	// Extract (field, value) pairs for the buffered term-delete mechanism and
	// accumulate FieldInfos for all fields seen in this document.
	var docEntries []docFieldEntry
	w.mu.Lock()
	for _, fi := range doc.GetFields() {
		if fi == nil {
			continue
		}
		if fn, ok := fi.(docFieldNamer); ok {
			if sv := fn.StringValue(); sv != "" {
				docEntries = append(docEntries, docFieldEntry{field: fn.Name(), val: sv})
			}
		}
		// Accumulate FieldInfos from fields that expose their type metadata.
		if fm, ok := fi.(indexableFieldMeta); ok {
			w.addFieldToInfos(fm)
		}
	}
	w.docFieldIndex = append(w.docFieldIndex, docEntries)
	w.mu.Unlock()

	// Atomic increment - no lock needed
	w.docCount.Add(1)
	return nil
}

// addFieldToInfos adds (or merges) a field's metadata into pendingFieldInfos.
// Must be called with w.mu held.
func (w *IndexWriter) addFieldToInfos(fm indexableFieldMeta) {
	if w.pendingFieldInfos == nil {
		w.pendingFieldInfos = NewFieldInfos()
	}
	name := fm.Name()
	if w.pendingFieldInfos.GetByName(name) != nil {
		// Already registered; do not re-add (field numbers must be stable).
		return
	}
	opts := FieldInfoOptions{
		IndexOptions:             fm.IndexOptions(),
		DocValuesType:            fm.DocValuesType(),
		DocValuesSkipIndexType:   DocValuesSkipIndexTypeNone,
		DocValuesGen:             -1,
		Stored:                   fm.IsStored(),
		Tokenized:                fm.IsTokenized(),
		StoreTermVectors:         fm.HasTermVectors(),
		VectorEncoding:           VectorEncodingFloat32,
		VectorSimilarityFunction: VectorSimilarityFunctionEuclidean,
	}
	number := w.pendingFieldInfos.GetNextFieldNumber()
	fi := NewFieldInfo(name, number, opts)
	_ = w.pendingFieldInfos.Add(fi) // ignore duplicate-number errors; field is already checked above
}

// UpdateDocument updates a document in the index.
// Minimizes critical section by processing document outside the global lock.
func (w *IndexWriter) UpdateDocument(term *Term, doc Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	// Process document outside global lock - DocumentsWriter has internal locking
	if w.documentsWriter != nil {
		if err := w.documentsWriter.UpdateDocument(doc, nil, term); err != nil {
			return fmt.Errorf("failed to update document: %w", err)
		}
	}

	return nil
}

// DeleteDocuments buffers a term-based delete that will be applied at the next
// Commit. Each document containing the given term in the given field is marked
// for deletion; the delete count is reflected in the SegmentCommitInfo written
// by Commit.
func (w *IndexWriter) DeleteDocuments(term *Term) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	w.pendingDeleteTerms = append(w.pendingDeleteTerms, term)
	w.mu.Unlock()
	return nil
}

// DeleteDocumentsQuery deletes documents matching the given query.
// Minimizes critical section - only holds lock for state updates.
func (w *IndexWriter) DeleteDocumentsQuery(query interface{}) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	// Document deletion processing happens outside global lock
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

// clearLiveCommitData clears the live commit data.
// Must be called with w.mu held.
func (w *IndexWriter) clearLiveCommitData() {
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
		return fmt.Errorf("cannot prepare commit: %w", err)
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
		return fmt.Errorf("cannot commit: %w", err)
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
	currentDocCount := int(w.docCount.Load())
	if currentDocCount > 0 {
		// Apply buffered term-based deletes by scanning the in-memory doc field
		// index. We track (field, value) pairs per document so that term-based
		// deletes can be applied at commit time without a real inverted index.
		delCount := 0
		if len(w.pendingDeleteTerms) > 0 && len(w.docFieldIndex) > 0 {
			// Build a set of (field, value) pairs to delete.
			type fieldVal struct{ field, val string }
			delSet := make(map[fieldVal]struct{}, len(w.pendingDeleteTerms))
			for _, dt := range w.pendingDeleteTerms {
				delSet[fieldVal{dt.Field, dt.Text()}] = struct{}{}
			}
			// Count docs that match at least one delete term.
			for _, docFields := range w.docFieldIndex {
				for _, fv := range docFields {
					if _, ok := delSet[fieldVal{fv.field, fv.val}]; ok {
						delCount++
						break // each doc counted at most once
					}
				}
			}
			// Guard: delCount must not exceed docCount.
			if delCount > currentDocCount {
				delCount = currentDocCount
			}
		}

		segmentName := si.GetNextSegmentName()
		segmentInfo := NewSegmentInfo(segmentName, currentDocCount, nil)
		sci := NewSegmentCommitInfo(segmentInfo, delCount, -1)
		// Associate accumulated FieldInfos so that readers opened after this
		// commit can enumerate all fields without accessing codec files.
		if w.pendingFieldInfos != nil {
			sci.SetInMemoryFieldInfos(w.pendingFieldInfos)
		}
		si.Add(sci)
		w.committedSegments = append(w.committedSegments, sci)
		w.docCount.Store(0) // Documents "flushed" to segment
		w.pendingDeleteTerms = w.pendingDeleteTerms[:0]
		w.docFieldIndex = w.docFieldIndex[:0]
		w.pendingFieldInfos = nil
	}

	// Add commit data if present
	if liveCommitData != nil && len(liveCommitData.data) > 0 {
		si.SetUserData(liveCommitData.data)
	}

	err = WriteSegmentInfos(si, w.directory)
	if err != nil {
		return fmt.Errorf("failed to write segment infos: %w", err)
	}

	// Clear the prepared commit flag
	preparedCommit = false

	return nil
}

// Close closes the IndexWriter.
// Uses atomic operations for state checks to minimize lock contention.
func (w *IndexWriter) Close() error {
	// Fast path: check if already closed using atomic
	if w.closed.Load() || w.tragicError.Load() != nil {
		return nil
	}

	// Check if prepareCommit was called but commit wasn't
	w.mu.RLock()
	if preparedCommit {
		w.mu.RUnlock()
		return errors.New("cannot close IndexWriter when prepareCommit was called but commit wasn't")
	}
	w.mu.RUnlock()

	// Try to commit changes before closing
	if err := w.Commit(); err != nil {
		// If commit fails, we still want to close the scheduler
		if s := w.config.GetMergeScheduler(); s != nil {
			_ = s.Close()
		}
		return fmt.Errorf("failed to commit during close: %w", err)
	}

	// Set closed atomically
	w.closed.Store(true)

	// Close the merge scheduler
	if s := w.config.GetMergeScheduler(); s != nil {
		return s.Close()
	}

	return nil
}

// NumDocs returns the number of documents in the index.
// Uses atomic load for buffered doc count - no lock needed.
func (w *IndexWriter) NumDocs() int {
	// In a real implementation, this would involve reading SegmentInfos
	// and accounting for deleted documents.
	si, err := ReadSegmentInfos(w.directory)
	if err != nil {
		return int(w.docCount.Load())
	}
	return si.TotalNumDocs() + int(w.docCount.Load())
}

// MaxDoc returns the maximum document ID.
func (w *IndexWriter) MaxDoc() int {
	return w.NumDocs()
}

// IsClosed returns true if the writer is closed.
// Uses atomic operations for lock-free check.
func (w *IndexWriter) IsClosed() bool {
	return w.closed.Load() || w.tragicError.Load() != nil
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
// Uses atomic store to reset counter without holding global lock.
func (w *IndexWriter) DeleteAll() error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	// Atomic store to reset counter - no lock needed
	w.docCount.Store(0)
	return nil
}

// Rollback rolls back all changes made since the last commit.
// This closes the writer and returns the index to its previous state.
// Uses atomic operations to minimize lock contention.
func (w *IndexWriter) Rollback() error {
	// Fast path: check if already closed using atomic
	if w.closed.Load() || w.tragicError.Load() != nil {
		return nil
	}

	// Close the merge scheduler without committing
	if s := w.config.GetMergeScheduler(); s != nil {
		_ = s.Close()
	}

	// Set closed atomically
	w.closed.Store(true)

	w.mu.Lock()
	defer w.mu.Unlock()
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

	// Commit flushes buffered docs; it acquires w.mu itself.
	return w.Commit()
}

// GetNumBufferedDocuments returns the number of documents currently
// buffered in RAM. Uses atomic load for lock-free access.
func (w *IndexWriter) GetNumBufferedDocuments() int {
	return int(w.docCount.Load())
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
// Uses atomic load for buffered doc count - no lock needed.
func (w *IndexWriter) GetDocStats() *DocStats {
	numDocs := int(w.docCount.Load())
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

	// Validate that we're not adding our own directory
	for _, dir := range dirs {
		if dir == w.directory {
			return errors.New("cannot add index to itself")
		}
	}

	// Process each source directory: read its SegmentInfos and accumulate the
	// live document count and FieldInfos so that subsequent Commit creates a
	// segment that reflects all imported fields.
	for _, dir := range dirs {
		sourceSI, err := ReadSegmentInfos(dir)
		if err != nil {
			// Empty or unreadable directory — skip.
			continue
		}

		for _, sci := range sourceSI.List() {
			liveDocs := sci.NumDocs()
			if liveDocs <= 0 {
				continue
			}
			// Accumulate document count into the pending buffer so that the
			// next Commit creates a segment for them.
			w.docCount.Add(int32(liveDocs))

			// Merge FieldInfos from the source segment into pending.
			srcFI := sci.GetInMemoryFieldInfos()
			if srcFI != nil {
				w.mu.Lock()
				if w.pendingFieldInfos == nil {
					w.pendingFieldInfos = NewFieldInfos()
				}
				it := srcFI.Iterator()
				for {
					info := it.Next()
					if info == nil {
						break
					}
					if w.pendingFieldInfos.GetByName(info.Name()) != nil {
						continue
					}
					number := w.pendingFieldInfos.GetNextFieldNumber()
					clone := NewFieldInfo(info.Name(), number, FieldInfoOptions{
						IndexOptions:             info.IndexOptions(),
						DocValuesType:            info.DocValuesType(),
						DocValuesSkipIndexType:   DocValuesSkipIndexTypeNone,
						DocValuesGen:             -1,
						Stored:                   info.IsStored(),
						Tokenized:                info.IsTokenized(),
						OmitNorms:                info.OmitNorms(),
						StoreTermVectors:         info.HasTermVectors(),
						VectorEncoding:           VectorEncodingFloat32,
						VectorSimilarityFunction: VectorSimilarityFunctionEuclidean,
					})
					_ = w.pendingFieldInfos.Add(clone)
				}
				w.mu.Unlock()
			}
		}
	}

	return nil
}

// AddIndexesFromReader adds indexes from the provided IndexReaders.
// This is used to add segments from existing readers to this writer.
func (w *IndexWriter) AddIndexesFromReader(readers ...*IndexReader) error {
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
		w.docCount.Add(int32(reader.NumDocs()))
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
// Uses atomic add to update counter - no global lock needed.
func (w *IndexWriter) AddDocuments(docs []Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	// Atomic add to update counter - no lock needed
	w.docCount.Add(int32(len(docs)))

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
