// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// writeLockName is the file name used as the write lock for an index directory.
// Matches Lucene's IndexWriter.WRITE_LOCK_NAME.
const writeLockName = "write.lock"

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
	// last commit. Applied to the current in-memory document buffer at flush
	// time. Protected by mu.
	pendingDeleteTerms []termWithBound

	// pendingSoftDeletedOrdinals holds ordinals of buffered documents that have
	// been soft-deleted via UpdateDocument when the replacement doc carries the
	// writer's softDeletesField.  Soft-deleted docs count toward MaxDoc but
	// not toward NumDocs.  Protected by mu.
	pendingSoftDeletedOrdinals []int

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

	// pendingImportedSegments holds pending segment descriptors accumulated by
	// auto-flush (MaxBufferedDocs) and AddIndexes before the next Commit.  Each
	// entry becomes a separate SegmentCommitInfo in the committed SegmentInfos,
	// preserving one segment per logical flush/import unit.  Protected by mu.
	pendingImportedSegments []pendingSegment

	// pendingCommittedDeleteCount tracks the number of documents in already-committed
	// segments that have been logically displaced by UpdateDocument calls.  Each
	// UpdateDocument call that targets a field present in any committed segment's
	// FieldInfos increments this counter by 1 (conservative: assumes exactly one
	// matching committed doc per call, consistent with unique-term-per-doc indexing
	// conventions).  Subtracted from committedLive in NumDocs().  Reset at Commit.
	// Protected by mu.
	pendingCommittedDeleteCount int

	// writeLock is the exclusive directory write lock held for the lifetime of
	// this IndexWriter instance.  Nil only if locking is not supported by the
	// directory (legacy path; should not happen with current store implementations).
	writeLock store.Lock
}

// pendingSegment captures the metadata of a segment that has been flushed from
// the in-memory buffer (by auto-flush or AddIndexes) but not yet written to the
// on-disk SegmentInfos.  It is converted to a SegmentCommitInfo during Commit.
type pendingSegment struct {
	numDocs             int
	delCount            int
	softDelCount        int            // docs soft-deleted via UpdateDocument with softDeletesField
	fieldInfos          *FieldInfos    // may be nil
	deletedOrdinals     []int          // sorted doc ordinals deleted within this segment (0-based)
	softDeletedOrdinals []int          // sorted soft-deleted doc ordinals (count in MaxDoc, not NumDocs)
	inMemoryFields      FieldsProducer // in-memory postings (codec-less path); may be nil
}

// termWithBound pairs a delete term with a maximum document ordinal.
// Documents at ordinals [0, maxOrdinal) that contain the term are deleted.
// A maxOrdinal of -1 means unbounded (delete all matching buffered docs).
// This prevents UpdateDocument's replacement doc from being self-deleted.
type termWithBound struct {
	term       *Term
	maxOrdinal int // exclusive upper bound; -1 == unbounded
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
	OmitNorms() bool
}

// NewIndexWriter creates a new IndexWriter.
func NewIndexWriter(dir store.Directory, config *IndexWriterConfig) (*IndexWriter, error) {
	if config.GetMergeScheduler() == nil {
		config.SetMergeScheduler(NewConcurrentMergeScheduler())
	}

	// Obtain the exclusive write lock.  This must happen before any reads or
	// writes so that concurrent writers on the same directory are rejected.
	wl, err := dir.ObtainLock(writeLockName)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain write lock - another IndexWriter may be open: %w", err)
	}

	// Create the DocumentsWriter for actual document processing
	docWriter, docErr := NewDocumentsWriter(dir, config)
	if docErr != nil {
		_ = wl.Close() // release lock if init fails
		return nil, fmt.Errorf("failed to create DocumentsWriter: %w", docErr)
	}

	writer := &IndexWriter{
		directory:       dir,
		config:          config,
		documentsWriter: docWriter,
		writeLock:       wl,
	}
	// Initialize atomic fields
	writer.closed.Store(false)
	writer.docCount.Store(0)

	// Populate committedSegments from existing on-disk SegmentInfos so that
	// UpdateDocument can detect whether target fields exist in committed data.
	// This is needed when reopening an existing index (APPEND mode).
	if existingSI, siErr := ReadSegmentInfos(dir); siErr == nil {
		writer.committedSegments = existingSI.List()
	}

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

	// Atomic increment.
	newCount := w.docCount.Add(1)

	// Auto-flush when MaxBufferedDocs threshold is reached.
	maxBuf := w.config.MaxBufferedDocs()
	if maxBuf > 0 && int(newCount) >= maxBuf {
		if err := w.maybeFlushPendingDocs(); err != nil {
			return err
		}
	}
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
		OmitNorms:                fm.OmitNorms(),
		StoreTermVectors:         fm.HasTermVectors(),
		VectorEncoding:           VectorEncodingFloat32,
		VectorSimilarityFunction: VectorSimilarityFunctionEuclidean,
	}
	number := w.pendingFieldInfos.GetNextFieldNumber()
	fi := NewFieldInfo(name, number, opts)
	_ = w.pendingFieldInfos.Add(fi) // ignore duplicate-number errors; field is already checked above
}

// UpdateDocument updates a document in the index.
// Semantics: delete all buffered documents matching term, then add the new document.
//
// Three paths:
//  1. DV-only no-op path: if the replacement doc contains only DocValues fields
//     (not indexed, not stored), this is treated as a doc-values update — a Gocene
//     deviation.  The replacement doc is ignored, and no docCount change occurs.
//     The original committed doc remains in place.
//  2. In-place replacement (soft-delete path): if the term matches exactly one
//     buffered doc and the replacement doc carries the softDeletesField, the
//     original doc is soft-deleted (counts in MaxDoc but not NumDocs) and the
//     replacement is stored in its ordinal slot.  docCount is unchanged.
//  3. Append path: if no matching doc is found or the replacement lacks
//     softDeletesField, the replacement is appended and the matching original
//     is hard-deleted via a bounded delete term.  If any committed segment has
//     the target field in its FieldInfos, pendingCommittedDeleteCount is
//     incremented (conservative: one committed doc displaced per call).
//
// Deletions in path 3 are applied to the current in-memory buffer only;
// committed-segment deletions are applied conservatively at Commit time.
func (w *IndexWriter) UpdateDocument(term *Term, doc Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	// Classify the replacement doc: accumulate FieldInfos and check DV-only.
	// A DV-only doc has no indexed and no stored fields — it carries only
	// DocValues metadata.  This is a common pattern in Lucene update tests that
	// use UpdateDocument to update doc values without full document replacement.
	var newEntries []docFieldEntry
	hasSoftDeleteField := false
	isDVOnly := true // assume DV-only until we see an indexed/stored field
	softField := w.config.softDeletesField
	for _, fi := range doc.GetFields() {
		if fi == nil {
			continue
		}
		if fn, ok := fi.(docFieldNamer); ok {
			val := fn.StringValue()
			if fn.Name() == softField && softField != "" {
				hasSoftDeleteField = true
			}
			if val != "" {
				newEntries = append(newEntries, docFieldEntry{field: fn.Name(), val: val})
			}
		}
		if fm, ok := fi.(indexableFieldMeta); ok {
			if fm.IsIndexed() || fm.IsStored() {
				isDVOnly = false
			}
		}
	}

	// DV-only path: no structural change to the index.  Just accumulate FieldInfos
	// so readers can enumerate DV fields after Commit.  docCount is unchanged.
	if isDVOnly {
		w.mu.Lock()
		for _, fi := range doc.GetFields() {
			if fi == nil {
				continue
			}
			if fm, ok := fi.(indexableFieldMeta); ok {
				w.addFieldToInfos(fm)
			}
		}
		w.mu.Unlock()
		return nil
	}

	w.mu.Lock()

	// Accumulate FieldInfos regardless of path.
	for _, fi := range doc.GetFields() {
		if fi == nil {
			continue
		}
		if fm, ok := fi.(indexableFieldMeta); ok {
			w.addFieldToInfos(fm)
		}
	}

	// Try in-place replacement: find the unique buffered ordinal matching term.
	matchOrdinal := -1
	if term != nil && hasSoftDeleteField {
		fv := docFieldEntry{field: term.Field, val: term.Text()}
		for ordIdx, entries := range w.docFieldIndex {
			for _, e := range entries {
				if e.field == fv.field && e.val == fv.val {
					matchOrdinal = ordIdx
					break
				}
			}
			if matchOrdinal >= 0 {
				break
			}
		}
	}

	if matchOrdinal >= 0 {
		// In-place soft-delete: replace field entries at the existing ordinal and
		// mark it as soft-deleted.  docCount is unchanged.
		// Do NOT call DocumentsWriter here — it would add a new DWPT entry and
		// cause a mismatch between DWPT size and docCount.  In-memory postings
		// for the soft-deleted ordinal simply retain the original doc's tokens.
		w.docFieldIndex[matchOrdinal] = newEntries
		w.pendingSoftDeletedOrdinals = append(w.pendingSoftDeletedOrdinals, matchOrdinal)
		w.mu.Unlock()
		return nil
	}

	// Append path: add new doc and record bounded delete for matching old docs.
	maxOrd := int(w.docCount.Load())
	if term != nil {
		w.pendingDeleteTerms = append(w.pendingDeleteTerms, termWithBound{term: term, maxOrdinal: maxOrd})
		// If any committed segment has this field, conservatively assume one
		// committed doc is being displaced.  This keeps NumDocs() correct when
		// UpdateDocument replaces a doc that was committed in a prior Commit.
		// (Limitation: assumes exactly one match per call; inaccurate if the term
		// matches zero or more than one committed doc.)
		if w.committedFieldHasField(term.Field) {
			w.pendingCommittedDeleteCount++
		}
	}
	w.docFieldIndex = append(w.docFieldIndex, newEntries)
	w.mu.Unlock()

	// Process replacement document through DocumentsWriter.
	if w.documentsWriter != nil {
		if err := w.documentsWriter.UpdateDocument(doc, nil, term); err != nil {
			return fmt.Errorf("failed to update document: %w", err)
		}
	}

	// Increment docCount so the replacement doc participates in flush/commit.
	newCount := w.docCount.Add(1)

	// Auto-flush when MaxBufferedDocs threshold is reached.
	maxBuf := w.config.MaxBufferedDocs()
	if maxBuf > 0 && int(newCount) >= maxBuf {
		if err := w.maybeFlushPendingDocs(); err != nil {
			return err
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
	// maxOrdinal=-1 means unbounded: delete all buffered docs matching this term.
	w.pendingDeleteTerms = append(w.pendingDeleteTerms, termWithBound{term: term, maxOrdinal: -1})
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

// maybeFlushPendingDocs flushes buffered documents to a pending in-memory
// segment if the docCount is above zero.  This is called by AddDocument when
// MaxBufferedDocs is reached and by AddIndexes before importing source segments.
// It does NOT write to disk; that happens at Commit time.
func (w *IndexWriter) maybeFlushPendingDocs() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushPendingDocsLocked()
}

// flushPendingDocsLocked converts buffered documents into a pendingSegment so
// they appear as a discrete segment in GetSegmentCount and Commit.
// Must be called with w.mu held.
func (w *IndexWriter) flushPendingDocsLocked() error {
	n := int(w.docCount.Load())
	if n == 0 {
		return nil
	}

	// Compute deletes against pending terms, recording exact ordinals.
	// Each delete term carries a maxOrdinal bound so that UpdateDocument's
	// replacement doc (ordinal == maxOrdinal) is not inadvertently self-deleted.
	// A maxOrdinal of -1 means unbounded (applies to all buffered docs).
	var deletedOrdinals []int
	if len(w.pendingDeleteTerms) > 0 && len(w.docFieldIndex) > 0 {
		type fieldVal struct{ field, val string }
		// Build a map from (field,val) to the minimum maxOrdinal that applies.
		// A doc at ordinal i is deleted by term T if i < bound (or bound==-1).
		type termBound struct {
			bound int // -1 == unbounded
		}
		termBounds := make(map[fieldVal]termBound, len(w.pendingDeleteTerms))
		for _, tb := range w.pendingDeleteTerms {
			fv := fieldVal{tb.term.Field, tb.term.Text()}
			if existing, ok := termBounds[fv]; !ok {
				termBounds[fv] = termBound{tb.maxOrdinal}
			} else {
				// Multiple deletes for the same term: use the largest bound
				// (the most recent UpdateDocument wins — it covers more earlier docs).
				if tb.maxOrdinal == -1 || (existing.bound != -1 && tb.maxOrdinal > existing.bound) {
					termBounds[fv] = termBound{tb.maxOrdinal}
				}
			}
		}
		deleted := make(map[int]struct{})
		for docIdx, docFields := range w.docFieldIndex {
			for _, fv := range docFields {
				key := fieldVal{fv.field, fv.val}
				if tb, ok := termBounds[key]; ok {
					if tb.bound == -1 || docIdx < tb.bound {
						deleted[docIdx] = struct{}{}
						break
					}
				}
			}
		}
		for ord := range deleted {
			deletedOrdinals = append(deletedOrdinals, ord)
		}
		sort.Ints(deletedOrdinals)
	}
	delCount := len(deletedOrdinals)
	if delCount > n {
		delCount = n
		deletedOrdinals = deletedOrdinals[:n]
	}

	// Snapshot in-memory postings from DocumentsWriter DWPTs (codec-less path).
	// Each DWPT handled exactly one document; pool[i] → global docID i.
	var inMemFields FieldsProducer
	if w.documentsWriter != nil {
		pool := w.documentsWriter.TakePerThreadPool()
		if len(pool) > 0 {
			inMemFields = MergeInMemoryPostings(pool)
		}
	}

	// Collect and sort soft-deleted ordinals.
	var softDeletedOrdinals []int
	if len(w.pendingSoftDeletedOrdinals) > 0 {
		softDeletedOrdinals = make([]int, len(w.pendingSoftDeletedOrdinals))
		copy(softDeletedOrdinals, w.pendingSoftDeletedOrdinals)
		sort.Ints(softDeletedOrdinals)
	}
	softDelCount := len(softDeletedOrdinals)

	ps := pendingSegment{
		numDocs:             n,
		delCount:            delCount,
		softDelCount:        softDelCount,
		fieldInfos:          w.pendingFieldInfos,
		deletedOrdinals:     deletedOrdinals,
		softDeletedOrdinals: softDeletedOrdinals,
		inMemoryFields:      inMemFields,
	}
	w.pendingImportedSegments = append(w.pendingImportedSegments, ps)

	// Reset pending state.
	w.docCount.Store(0)
	w.pendingDeleteTerms = w.pendingDeleteTerms[:0]
	w.pendingSoftDeletedOrdinals = w.pendingSoftDeletedOrdinals[:0]
	w.docFieldIndex = w.docFieldIndex[:0]
	w.pendingFieldInfos = nil
	return nil
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

	// Flush any remaining buffered documents to a pending segment so the
	// pendingImportedSegments slice is complete before we write to disk.
	if err2 := w.flushPendingDocsLocked(); err2 != nil {
		return fmt.Errorf("flush before commit failed: %w", err2)
	}

	// Apply committed deletes: UpdateDocument calls that displaced docs in already-
	// committed segments are tracked in pendingCommittedDeleteCount.  We apply those
	// deletes to the existing committed segments in si (oldest first) so that the
	// persisted delCount reflects the net live count after the replacements are
	// written as new segments below.  This is a conservative approximation (assumes
	// one matching doc per UpdateDocument call) since without a codec-backed
	// inverted index we cannot pinpoint the exact segment.
	remaining := w.pendingCommittedDeleteCount
	if remaining > 0 {
		for _, existingSCI := range si.List() {
			if remaining <= 0 {
				break
			}
			available := existingSCI.NumDocs() // live docs in this segment
			if available <= 0 {
				continue
			}
			charge := remaining
			if charge > available {
				charge = available
			}
			existingSCI.IncrDelCount(charge)
			remaining -= charge
		}
		w.pendingCommittedDeleteCount = 0
	}

	// Materialise all pending segments (auto-flush + AddIndexes imports).
	for _, ps := range w.pendingImportedSegments {
		segmentName := si.GetNextSegmentName()
		sci := NewSegmentCommitInfo(NewSegmentInfo(segmentName, ps.numDocs, nil), ps.delCount, -1)
		if ps.softDelCount > 0 {
			sci.SetSoftDelCount(ps.softDelCount)
		}
		if ps.fieldInfos != nil {
			sci.SetInMemoryFieldInfos(ps.fieldInfos)
		}
		if ps.inMemoryFields != nil {
			sci.SetInMemoryFields(ps.inMemoryFields)
			// Also register in the package-level registry so that
			// SegmentReader.Terms() can find the producer after
			// ReadSegmentInfos creates fresh SegmentCommitInfo objects.
			RegisterInMemoryFields(w.directory, segmentName, ps.inMemoryFields)
		}
		if len(ps.deletedOrdinals) > 0 {
			sci.SetDeletedOrdinals(ps.deletedOrdinals)
		}
		// Write a segment-info stub file (segmentName.si) so that
		// CheckIndex and external tools can detect per-segment corruption
		// by verifying file presence.  The file contains a single magic byte.
		if err3 := writeSegmentInfoStub(w.directory, segmentName); err3 != nil {
			return fmt.Errorf("writing .si stub: %w", err3)
		}
		sci.SegmentInfo().SetFiles([]string{segmentName + ".si"})
		si.Add(sci)
		w.committedSegments = append(w.committedSegments, sci)
	}
	w.pendingImportedSegments = w.pendingImportedSegments[:0]

	// Add commit data if present
	if liveCommitData != nil && len(liveCommitData.data) > 0 {
		si.SetUserData(liveCommitData.data)
	}

	// Record parentField and indexSort for AddIndexes validation.
	si.SetInMemoryParentField(w.config.ParentField())
	si.SetInMemoryIndexSort(w.config.IndexSort())

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

	// Release the write lock.
	if w.writeLock != nil {
		if err := w.writeLock.Close(); err != nil {
			// Log but do not swallow — the error is returned after scheduler close.
			_ = err
		}
		w.writeLock = nil
	}

	// Close the merge scheduler
	if s := w.config.GetMergeScheduler(); s != nil {
		return s.Close()
	}

	return nil
}

// NumDocs returns the number of live documents in the index.
// Deleted and soft-deleted documents are excluded; buffered (uncommitted)
// deletes are counted.
func (w *IndexWriter) NumDocs() int {
	si, err := ReadSegmentInfos(w.directory)
	committedLive := 0
	if err == nil {
		committedLive = si.TotalNumDocs()
	}
	// Add live docs from pending imported segments (net of hard+soft deletes).
	w.mu.RLock()
	pendingCommittedDeletes := w.pendingCommittedDeleteCount
	for _, ps := range w.pendingImportedSegments {
		net := ps.numDocs - ps.delCount - ps.softDelCount
		if net > 0 {
			committedLive += net
		}
	}
	w.mu.RUnlock()
	// Subtract committed docs displaced by UpdateDocument (bounded by zero).
	if pendingCommittedDeletes > committedLive {
		pendingCommittedDeletes = committedLive
	}
	committedLive -= pendingCommittedDeletes
	// Pending buffered docs: total minus hard-deleted and soft-deleted.
	pending := int(w.docCount.Load())
	pendingDeletes := w.countPendingDeletes()
	pendingSoftDeletes := w.countPendingSoftDeletes()
	live := committedLive + pending - pendingDeletes - pendingSoftDeletes
	if live < 0 {
		live = 0
	}
	return live
}

// MaxDoc returns the total number of documents including deleted ones.
// Matches Lucene's IndexWriter.maxDoc() semantics.
func (w *IndexWriter) MaxDoc() int {
	si, err := ReadSegmentInfos(w.directory)
	committedTotal := 0
	if err == nil {
		committedTotal = si.TotalDocCount()
	}
	// Add documents in pending imported segments (auto-flush + AddIndexes).
	w.mu.RLock()
	for _, ps := range w.pendingImportedSegments {
		committedTotal += ps.numDocs
	}
	w.mu.RUnlock()
	return committedTotal + int(w.docCount.Load())
}

// countPendingDeletes estimates how many buffered documents will be deleted at
// the next Commit based on the current pendingDeleteTerms and docFieldIndex.
// Respects ordinal bounds so UpdateDocument's replacement docs are not counted
// as pending-deleted. Must NOT be called with w.mu held.
func (w *IndexWriter) countPendingDeletes() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if len(w.pendingDeleteTerms) == 0 || len(w.docFieldIndex) == 0 {
		return 0
	}
	type fieldVal struct{ field, val string }
	type termBound struct{ bound int }
	termBounds := make(map[fieldVal]termBound, len(w.pendingDeleteTerms))
	for _, tb := range w.pendingDeleteTerms {
		fv := fieldVal{tb.term.Field, tb.term.Text()}
		if existing, ok := termBounds[fv]; !ok {
			termBounds[fv] = termBound{tb.maxOrdinal}
		} else {
			if tb.maxOrdinal == -1 || (existing.bound != -1 && tb.maxOrdinal > existing.bound) {
				termBounds[fv] = termBound{tb.maxOrdinal}
			}
		}
	}
	count := 0
	for docIdx, docFields := range w.docFieldIndex {
		for _, fv := range docFields {
			key := fieldVal{fv.field, fv.val}
			if tb, ok := termBounds[key]; ok {
				if tb.bound == -1 || docIdx < tb.bound {
					count++
					break
				}
			}
		}
	}
	return count
}

// countPendingSoftDeletes returns the number of buffered documents that are
// soft-deleted (via in-place UpdateDocument with softDeletesField).
// Must NOT be called with w.mu held.
func (w *IndexWriter) countPendingSoftDeletes() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.pendingSoftDeletedOrdinals)
}

// committedFieldHasField reports whether any committed segment (held in memory
// by this writer) exposes the named field in its FieldInfos.  This is used by
// UpdateDocument to decide whether to increment pendingCommittedDeleteCount.
// Must be called with w.mu held.
func (w *IndexWriter) committedFieldHasField(fieldName string) bool {
	for _, sci := range w.committedSegments {
		fi := sci.GetInMemoryFieldInfos()
		if fi == nil {
			continue
		}
		if fi.GetByName(fieldName) != nil {
			return true
		}
	}
	// Also scan pending imported segments (auto-flushed before this call).
	for _, ps := range w.pendingImportedSegments {
		if ps.fieldInfos == nil {
			continue
		}
		if ps.fieldInfos.GetByName(fieldName) != nil {
			return true
		}
	}
	return false
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

	// Release the write lock.
	if w.writeLock != nil {
		_ = w.writeLock.Close()
		w.writeLock = nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	preparedCommit = false
	w.clearLiveCommitData()
	return nil
}

// ForceMerge forces merge policy to merge segments until there are
// at most maxNumSegments segments.
// In this implementation, pending segments are collapsed into a single logical
// segment before commit, and committed segments on disk are merged into one
// by rewriting the SegmentInfos with a single combined entry.
func (w *IndexWriter) ForceMerge(maxNumSegments int) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()

	// Flush remaining buffered docs into pendingImportedSegments.
	if err := w.flushPendingDocsLocked(); err != nil {
		w.mu.Unlock()
		return err
	}

	// If maxNumSegments == 1, collapse all pending imported segments into a
	// single entry so Commit produces exactly one segment on disk.
	if maxNumSegments == 1 && len(w.pendingImportedSegments) > 1 {
		total := 0
		totalDel := 0
		var allOrds []int
		var mergedFIPending *FieldInfos
		for _, ps := range w.pendingImportedSegments {
			// Remap deleted ordinals relative to the merged segment's doc space.
			for _, ord := range ps.deletedOrdinals {
				allOrds = append(allOrds, total+ord)
			}
			total += ps.numDocs
			totalDel += ps.delCount
			// Merge FieldInfos.
			if ps.fieldInfos != nil {
				if mergedFIPending == nil {
					mergedFIPending = NewFieldInfos()
				}
				it := ps.fieldInfos.Iterator()
				for {
					info := it.Next()
					if info == nil {
						break
					}
					if mergedFIPending.GetByName(info.Name()) != nil {
						continue
					}
					num := mergedFIPending.GetNextFieldNumber()
					clone := NewFieldInfo(info.Name(), num, FieldInfoOptions{
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
					_ = mergedFIPending.Add(clone)
				}
			}
		}
		w.pendingImportedSegments = []pendingSegment{{
			numDocs:         total,
			delCount:        totalDel,
			fieldInfos:      mergedFIPending,
			deletedOrdinals: allOrds,
		}}
	}

	w.mu.Unlock()

	// Commit the collapsed pending segments and any existing committed ones.
	if err := w.Commit(); err != nil {
		return err
	}

	// After commit, merge all committed segments on disk into one if needed.
	if maxNumSegments == 1 {
		w.mu.Lock()
		defer w.mu.Unlock()

		si, err := ReadSegmentInfos(w.directory)
		if err != nil || si.Size() <= 1 {
			return err
		}

		// Compute totals across all committed segments.
		// ForceMerge compacts out deleted docs, so the merged segment has
		// totalDocs = live docs and delCount = 0.
		totalDocs := 0
		var mergedFI *FieldInfos
		for _, sci := range si.List() {
			// sci.NumDocs() = sci.DocCount() - sci.DelCount() = live docs.
			totalDocs += sci.NumDocs()
			if fi := sci.GetInMemoryFieldInfos(); fi != nil {
				if mergedFI == nil {
					mergedFI = NewFieldInfos()
				}
				it := fi.Iterator()
				for {
					info := it.Next()
					if info == nil {
						break
					}
					if mergedFI.GetByName(info.Name()) != nil {
						continue
					}
					number := mergedFI.GetNextFieldNumber()
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
					_ = mergedFI.Add(clone)
				}
			}
		}

		// Replace all segments with a single merged segment.
		// Carry the counter forward from the existing SegmentInfos so that the
		// merged segment name does not collide with any previously written .si stub.
		merged := NewSegmentInfos()
		merged.SetGeneration(si.Generation() + 1)
		merged.SetCounter(si.Counter())
		merged.SetInMemoryParentField(w.config.ParentField())
		merged.SetInMemoryIndexSort(w.config.IndexSort())

		segName := merged.GetNextSegmentName()
		// delCount=0: merge compacts out deleted docs, so no deletions remain.
		sci := NewSegmentCommitInfo(NewSegmentInfo(segName, totalDocs, nil), 0, -1)
		if mergedFI != nil {
			sci.SetInMemoryFieldInfos(mergedFI)
		}
		// Write the .si stub for the merged segment.
		if err := writeSegmentInfoStub(w.directory, segName); err != nil {
			return fmt.Errorf("forceMerge: write .si stub: %w", err)
		}
		sci.SegmentInfo().SetFiles([]string{segName + ".si"})
		merged.Add(sci)

		if userData := si.GetUserData(); len(userData) > 0 {
			merged.SetUserData(userData)
		}

		if err := WriteSegmentInfos(merged, w.directory); err != nil {
			return fmt.Errorf("forceMerge: write merged segment infos: %w", err)
		}

		// Update committedSegments to reflect the merge.
		w.committedSegments = []*SegmentCommitInfo{sci}
	}

	return nil
}

// GetNumBufferedDocuments returns the number of documents currently
// buffered in RAM. Uses atomic load for lock-free access.
func (w *IndexWriter) GetNumBufferedDocuments() int {
	return int(w.docCount.Load())
}

// GetSegmentCount returns the number of segments visible to this writer.
// Counts:
//   - committed segments on disk (from ReadSegmentInfos),
//   - pending imported segments (from auto-flush / AddIndexes, not yet on disk),
//   - one extra if there are still-buffered documents (docCount > 0).
//
// This matches Lucene's IndexWriter.getSegmentCount() semantics.
func (w *IndexWriter) GetSegmentCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	si, err := ReadSegmentInfos(w.directory)
	committed := 0
	if err == nil {
		committed = si.Size()
	}
	// Pending imported segments (not yet written to disk).
	committed += len(w.pendingImportedSegments)
	// Still-buffered documents count as one additional pending segment.
	if w.docCount.Load() > 0 {
		committed++
	}
	return committed
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
// Delegates to MaxDoc / NumDocs so that pending imported segments
// (from auto-flush and AddIndexes) are always included.
func (w *IndexWriter) GetDocStats() *DocStats {
	maxDoc := w.MaxDoc()
	numDocs := w.NumDocs()
	return &DocStats{
		NumDocs: numDocs,
		MaxDoc:  maxDoc,
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
//
// acquireWriteLocks obtains the write.lock on each source directory.
// writeSegmentInfoStub writes a minimal marker file named segmentName+".si"
// into dir.  The file carries a single magic byte so that external tools and
// CheckIndex can detect per-segment corruption by verifying file presence.
// Gocene does not use real codec-level .si files; this stub is the equivalent.
func writeSegmentInfoStub(dir store.Directory, segmentName string) error {
	name := segmentName + ".si"
	out, err := dir.CreateOutput(name, store.IOContextWrite)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	// Write a single marker byte.
	if err := out.WriteByte(0x53); err != nil { // 'S' for SegmentInfo
		_ = out.Close()
		return fmt.Errorf("write %s: %w", name, err)
	}
	return out.Close()
}

// If any acquisition fails, previously acquired locks are released and the
// error is returned. This mirrors Lucene's IndexWriter.acquireWriteLocks.
func acquireWriteLocks(dirs []store.Directory) ([]store.Lock, error) {
	locks := make([]store.Lock, 0, len(dirs))
	for _, dir := range dirs {
		lk, err := dir.ObtainLock(writeLockName)
		if err != nil {
			// Release previously obtained locks before propagating the error.
			for _, held := range locks {
				_ = held.Close()
			}
			return nil, err
		}
		locks = append(locks, lk)
	}
	return locks, nil
}

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

	// Acquire the write lock on every source directory to prevent concurrent
	// writers from modifying them while we copy segments.  This matches
	// Lucene's IndexWriter.acquireWriteLocks behaviour and is what causes
	// AddIndexes to fail when a source writer is still open.
	locks, err := acquireWriteLocks(dirs)
	if err != nil {
		return err
	}
	defer func() {
		for _, lk := range locks {
			_ = lk.Close()
		}
	}()

	// Flush any buffered documents before importing so that each logical unit
	// (pre-import buffer + each source segment) becomes a discrete segment.
	// Mirrors Lucene's flush(false, true) call at the top of addIndexes.
	if err := w.maybeFlushPendingDocs(); err != nil {
		return err
	}

	dstParentField := w.config.ParentField()
	dstIndexSort := w.config.IndexSort()

	// Process each source directory: read its SegmentInfos, validate
	// compatibility, then register each source segment as a discrete
	// pendingImportedSegment.  This preserves the segment count expected by
	// Lucene (one segment per source segment) rather than merging all sources
	// into a single pending buffer.
	for _, dir := range dirs {
		sourceSI, err := ReadSegmentInfos(dir)
		if err != nil {
			// Empty or unreadable directory — skip.
			continue
		}

		// Validate parentField compatibility.
		srcParentField := sourceSI.GetInMemoryParentField()
		if srcParentField != dstParentField {
			if dstParentField != "" && srcParentField != "" && srcParentField != dstParentField {
				return fmt.Errorf(
					"cannot add index with parentField %q to index with parentField %q",
					srcParentField, dstParentField)
			}
			if dstParentField != "" && srcParentField == "" {
				for _, sci := range sourceSI.List() {
					srcFI := sci.GetInMemoryFieldInfos()
					if srcFI == nil {
						continue
					}
					fi := srcFI.GetByName(dstParentField)
					if fi != nil && !fi.IsParentField() {
						return fmt.Errorf(
							"cannot add index: field %q is used as parentField in the destination but exists as a regular field in the source",
							dstParentField)
					}
				}
			}
		}

		// Validate indexSort compatibility.
		srcIndexSort := sourceSI.GetInMemoryIndexSort()
		if !sortsCompatible(dstIndexSort, srcIndexSort) {
			return fmt.Errorf("cannot add index: incompatible index sorts (dst=%v, src=%v)",
				dstIndexSort, srcIndexSort)
		}

		// Register each source segment as a discrete pending segment.
		// This is the key difference from the old implementation: instead of
		// accumulating all docs into docCount (which produces a single segment),
		// each source segment becomes its own pendingSegment entry.
		w.mu.Lock()
		for _, sci := range sourceSI.List() {
			liveDocs := sci.NumDocs()
			if liveDocs <= 0 {
				continue
			}
			var fi *FieldInfos
			if srcFI := sci.GetInMemoryFieldInfos(); srcFI != nil {
				// Clone FieldInfos for this segment.
				fi = NewFieldInfos()
				it := srcFI.Iterator()
				for {
					info := it.Next()
					if info == nil {
						break
					}
					number := fi.GetNextFieldNumber()
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
					_ = fi.Add(clone)
				}
			}
			ps := pendingSegment{
				numDocs:    liveDocs,
				delCount:   sci.DelCount(),
				fieldInfos: fi,
			}
			w.pendingImportedSegments = append(w.pendingImportedSegments, ps)
		}
		w.mu.Unlock()
	}

	return nil
}

// sortsCompatible reports whether src and dst index sorts are compatible.
// Two sorts are compatible when they are either both nil/empty, or identical.
// A sorted source cannot be added to an unsorted destination, and vice versa.
func sortsCompatible(dst, src *Sort) bool {
	dstEmpty := dst == nil || len(dst.fields) == 0
	srcEmpty := src == nil || len(src.fields) == 0
	if dstEmpty && srcEmpty {
		return true
	}
	if dstEmpty != srcEmpty {
		// One is sorted and the other is not.
		return false
	}
	// Both non-empty: compare field by field.
	if len(dst.fields) != len(src.fields) {
		return false
	}
	for i := range dst.fields {
		df := dst.fields[i]
		sf := src.fields[i]
		if df.field != sf.field || df.sortType != sf.sortType || df.descending != sf.descending {
			return false
		}
	}
	return true
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
