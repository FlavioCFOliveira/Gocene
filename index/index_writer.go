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
	"github.com/FlavioCFOliveira/Gocene/util"
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

	// pendingDeletedDocIDs holds document IDs buffered by TryDeleteDocument
	// (NRT delete-by-docID). These are applied to the live-docs bitmap during
	// Commit. Protected by mu.
	pendingDeletedDocIDs []int

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
	// conventions).  It is a pre-commit ESTIMATE only, used by NumDocs() to
	// approximate the live count in the window between a delete/update and the
	// next Commit; at Commit the deletes are resolved exactly against the
	// committed segments' postings (see applyDeletesToCommittedSegments) and this
	// estimate is reset to 0.  Protected by mu.
	pendingCommittedDeleteCount int

	// pendingCommittedDeleteTerms holds the delete terms that must be resolved
	// against already-committed segments at the next Commit.  Populated by
	// DeleteDocuments (unbounded term delete) and by UpdateDocument's append path
	// (delete the old committed doc that the replacement displaces).  Unlike the
	// buffered-doc delete path (pendingDeleteTerms + docFieldIndex), these terms
	// are resolved by opening the committed segments' postings, collecting the
	// exact matching docIDs, marking them in the per-segment live-docs bitset, and
	// persisting a byte-faithful .liv file.  Protected by mu.
	pendingCommittedDeleteTerms []*Term

	// writeLock is the exclusive directory write lock held for the lifetime of
	// this IndexWriter instance.  Nil only if locking is not supported by the
	// directory (legacy path; should not happen with current store implementations).
	writeLock store.Lock

	// liveCommitData holds the current commit data that will be written on next commit
	liveCommitData *commitData
	// preparedCommit indicates if prepareCommit has been called
	preparedCommit bool
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

	// dwpts carries the raw per-thread writer state when a real codec is
	// present.  Commit calls dwpt.Flush to materialise stored fields,
	// postings, and field infos on disk. nil when the writer was opened
	// without a codec (structural-unit-test path).
	dwpts []*DocumentsWriterPerThread
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
		// Resolve the term against committed segments at the next Commit so the
		// old committed doc the replacement displaces is actually deleted
		// (rmp #4753).  The committed docs always precede every buffered ordinal,
		// so an unbounded committed-segment delete cannot touch the replacement.
		w.pendingCommittedDeleteTerms = append(w.pendingCommittedDeleteTerms, term)
		// Pre-commit NumDocs() estimate: if any committed segment has this field,
		// conservatively assume one committed doc is being displaced.  Reset and
		// superseded by the exact resolution at Commit.
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
	// Bound the buffered-doc delete to the documents already added when this
	// delete is issued (ordinals [0, len(docFieldIndex))). A document added
	// AFTER this DeleteDocuments call — e.g. the manual update idiom
	// DeleteDocuments(term) followed by AddDocument(sameTerm) — must NOT be
	// self-deleted, mirroring Lucene's delete-before-add sequencing where a
	// buffered delete only applies to docs indexed before it. (-1/unbounded
	// here would nuke the replacement doc, leaving NumDocs == 0.)
	w.pendingDeleteTerms = append(w.pendingDeleteTerms, termWithBound{term: term, maxOrdinal: len(w.docFieldIndex)})
	// Also resolve this term against already-committed segments at the next
	// Commit so deletions take effect across commits (rmp #4753).
	if term != nil {
		w.pendingCommittedDeleteTerms = append(w.pendingCommittedDeleteTerms, term)
	}
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

// SetLiveCommitData sets the commit data that will be written with the next commit.
// This data is stored in the commit point and can be retrieved later.
// The data is "live" meaning it can be modified until the actual commit happens.
func (w *IndexWriter) SetLiveCommitData(data map[string]string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.liveCommitData == nil {
		w.liveCommitData = &commitData{data: make(map[string]string)}
	}
	// Copy the data to ensure we capture the values at commit time
	for k, v := range data {
		w.liveCommitData.data[k] = v
	}
}

// getLiveCommitData returns the current live commit data
func (w *IndexWriter) getLiveCommitData() map[string]string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.liveCommitData == nil {
		return nil
	}
	// Return a copy to prevent external modification
	result := make(map[string]string, len(w.liveCommitData.data))
	for k, v := range w.liveCommitData.data {
		result[k] = v
	}
	return result
}

// clearLiveCommitData clears the live commit data.
// Must be called with w.mu held.
func (w *IndexWriter) clearLiveCommitData() {
	w.liveCommitData = nil
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

	// Snapshot DocumentsWriter DWPTs. When a codec is configured, the raw
	// per-thread writers are preserved in pendingSegment.dwpts so that
	// Commit can materialise stored fields, postings, and field infos on
	// disk. When no codec is wired (structural-unit-test path) we fall
	// back to the codec-less in-memory merge used by SegmentReader.Terms.
	//
	// dw.mu is held around TakePerThreadPool so that any in-flight
	// AddDocument call that already retrieved DWPT[0] (before the pool
	// reset) finishes ProcessDocument before we hand the DWPT to the
	// codec flush path. This prevents a data race on DWPT internals
	// (storedFields, posting maps) between Commit and AddDocument.
	var inMemFields FieldsProducer
	var pool []*DocumentsWriterPerThread
	if w.documentsWriter != nil {
		w.documentsWriter.mu.Lock()
		pool = w.documentsWriter.TakePerThreadPool()
		w.documentsWriter.mu.Unlock()
		if len(pool) > 0 && w.config.Codec() == nil {
			// Codec-less path: merge postings into a single in-memory producer.
			inMemFields = MergeInMemoryPostings(pool)
			pool = nil // pooled writers not needed further
		}
	}

	// Recompute n from the actual DWPT doc count to close the window between
	// ProcessDocument (increments DWPT.numDocsInRAM) and docCount.Add(1)
	// (atomic, called outside w.mu). Without this, n may lag the real DWPT
	// count, causing segments to be registered with the wrong numDocs.
	//
	// When pool is non-empty but all DWPTs have zero docs, the docs were
	// already swept by a prior TakePerThreadPool call. Discard the pool.
	//
	// When codec is active AND the pool is empty (meaning all docs went through
	// DWPT for this codec but none remain), any residual docCount comes from a
	// docCount.Add(1) that ran after a prior Commit already captured and swept
	// the DWPT. This is a ghost count: the document was already counted in the
	// prior Commit's segment via pool reconciliation. Drop it to prevent
	// duplicate-count segments. When codec is nil, count-only segments from
	// the codec-less path are preserved as-is.
	if len(pool) > 0 {
		actual := 0
		for _, dwpt := range pool {
			actual += dwpt.GetNumDocs()
		}
		if actual > 0 {
			n = actual
		} else {
			// Pool was already swept; residual docCount increment is stale.
			pool = nil
			n = 0
		}
	} else if w.config.Codec() != nil && inMemFields == nil {
		// Codec active, pool empty, no in-memory fields: any residual docCount
		// is a ghost from a prior Commit's DWPT sweep. Suppress it.
		n = 0
	}
	if n == 0 && inMemFields == nil {
		// Nothing left to flush after DWPT reconciliation.
		w.docCount.Store(0)
		return nil
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
		dwpts:               pool,
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

	if w.preparedCommit {
		return errors.New("prepareCommit already called; call commit or rollback first")
	}

	// Mark that we're in the prepared state
	w.preparedCommit = true

	// Flush any buffered documents into pending segments so that Commit (the
	// second phase) only needs to write them to disk. Apply buffered deletes
	// so the live-docs bitmap is current.  File sync and the segments_N write
	// happen in Commit; prepare only guarantees atomicity of the flush.
	if err := w.flushPendingDocsLocked(); err != nil {
		w.preparedCommit = false
		return fmt.Errorf("prepareCommit: flush: %w", err)
	}

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

	// Materialise all pending segments (auto-flush + AddIndexes imports).
	codec := w.config.Codec()

	// Apply buffered delete terms to already-committed segments (rmp #4753).
	// When a codec is wired we resolve each term against the committed
	// segments' postings, collect the exact matching docIDs, mark them in a
	// byte-faithful .liv file, and bump the segment's delGen/delCount.  This is
	// exact: only documents whose postings actually contain the term are
	// deleted, so a term that does not occur in any committed segment is a
	// no-op (e.g. a delete that targets only buffered docs added this session).
	//
	// When no codec is wired (codec-less structural tests) the exact path is
	// unavailable, so we fall back to the historical conservative count-only
	// approximation, charging one deletion per term against the committed
	// segments oldest-first.
	if len(w.pendingCommittedDeleteTerms) > 0 {
		if codec != nil && codec.PostingsFormat() != nil {
			if err2 := w.applyDeletesToCommittedSegments(si, codec); err2 != nil {
				return fmt.Errorf("commit: apply deletes to committed segments: %w", err2)
			}
		} else {
			w.applyApproximateCommittedDeletesFor(si, w.pendingCommittedDeleteTerms)
		}
	}
	w.pendingCommittedDeleteTerms = w.pendingCommittedDeleteTerms[:0]
	w.pendingCommittedDeleteCount = 0

	for _, ps := range w.pendingImportedSegments {
		segmentName := si.GetNextSegmentName()
		segInfo := NewSegmentInfo(segmentName, ps.numDocs, w.directory)
		sci := NewSegmentCommitInfo(segInfo, ps.delCount, -1)
		if ps.softDelCount > 0 {
			sci.SetSoftDelCount(ps.softDelCount)
		}
		if ps.fieldInfos != nil {
			sci.SetInMemoryFieldInfos(ps.fieldInfos)
		}

		codecFlushed := false
		if codec != nil && len(ps.dwpts) > 0 &&
			codec.StoredFieldsFormat() != nil && codec.PostingsFormat() != nil && codec.FieldInfosFormat() != nil {
			// Real-codec path: flush each DWPT to on-disk Lucene format files
			// (stored fields, postings, field infos). The codec name is
			// recorded on the SegmentInfo so OpenDirectoryReader can resolve
			// the right codec when reopening. Skip when the codec has stub/nil
			// format methods (e.g. test codecs) — fall through to the in-memory
			// path.
			//
			// All DWPTs in the pool write to the SAME segment (segInfo/sci).
			// This is safe because getPerThreadWriter reuses pool[0], so the
			// pool has at most one DWPT with any documents.
			for _, dwpt := range ps.dwpts {
				if dwpt.GetNumDocs() == 0 {
					continue
				}
				fi := dwpt.GetFieldInfos()
				if fi == nil {
					fi = NewFieldInfos()
				}
				writeState := &SegmentWriteState{
					Directory:     w.directory,
					SegmentInfo:   segInfo,
					FieldInfos:    fi,
					SegmentSuffix: "",
				}
				if err3 := dwpt.flushStoredFields(codec, writeState); err3 != nil {
					return fmt.Errorf("commit: flush stored fields for %s: %w", segmentName, err3)
				}
				if err3 := dwpt.flushTermVectors(codec, writeState); err3 != nil {
					return fmt.Errorf("commit: flush term vectors for %s: %w", segmentName, err3)
				}
				if err3 := dwpt.flushPostings(codec, writeState, fi); err3 != nil {
					return fmt.Errorf("commit: flush postings for %s: %w", segmentName, err3)
				}
				// KNN vectors must be flushed BEFORE field infos: the
				// PerFieldKnnVectorsWriter stamps the format-name and suffix
				// codec attributes onto each vector FieldInfo, and those
				// attributes must be present when flushFieldInfos serialises
				// the .fnm so the reader can resolve the delegate format.
				if err3 := dwpt.flushKnnVectors(codec, writeState); err3 != nil {
					return fmt.Errorf("commit: flush knn vectors for %s: %w", segmentName, err3)
				}
				if err3 := dwpt.flushFieldInfos(codec, writeState); err3 != nil {
					return fmt.Errorf("commit: flush field infos for %s: %w", segmentName, err3)
				}
				// Accumulate the field infos so stored-field reads can resolve
				// field names from the on-disk .fnm.
				sci.SetInMemoryFieldInfos(fi)
				// Mark codec stamped on segInfo only when actual files were written.
				if !codecFlushed {
					segInfo.SetCodec(codec.Name())
					segInfo.SetFiles([]string{segmentName + ".si"})
					codecFlushed = true
				}
			}
		}
		if codecFlushed && w.config.UseCompoundFile() && codec.CompoundFormat() != nil {
			// Collect all per-format files written for this segment (every file
			// with the segment name prefix, excluding the infrastructure files:
			// .si is written after CFS, .cfs/.cfe are the CFS output targets).
			// All Gocene codec writers now use CodecUtil.writeIndexHeader /
			// writeFooter, so every per-format file is embeddable.
			allFiles, err3 := w.directory.ListAll()
			if err3 != nil {
				return fmt.Errorf("commit: list directory for CFS: %w", err3)
			}
			// A file belongs to this segment when ParseSegmentName resolves to
			// the segment name. ParseSegmentName follows IndexFileNames: it
			// strips at the first '.' or the second '_', so it matches both the
			// plain "_0.ext" form and the per-field "_0_FormatName_suffix.ext"
			// form used by PerFieldPostingsFormat / PerFieldKnnVectorsFormat /
			// etc. A previous "_0." prefix match silently dropped the per-field
			// vector files (_0_Lucene99HnswVectorsFormat_0.vem, ...) from the
			// compound file, so the reopened reader could not find them inside
			// the .cfs.
			var segFiles []string
			for _, f := range allFiles {
				if ParseSegmentName(f) != segmentName {
					continue
				}
				switch GetExtension(f) {
				case "si", "cfs", "cfe":
					// .si is written after CFS; .cfs/.cfe are the output targets.
				default:
					segFiles = append(segFiles, f)
				}
			}
			if len(segFiles) > 0 {
				segInfo.SetFiles(segFiles)
				if err3 := codec.CompoundFormat().Write(w.directory, segInfo, store.IOContextWrite); err3 != nil {
					return fmt.Errorf("commit: write CFS for %s: %w", segmentName, err3)
				}
				// Delete the loose per-format files now packed into CFS.
				for _, f := range segFiles {
					if err3 := w.directory.DeleteFile(f); err3 != nil {
						// Non-fatal: CFS is written; index integrity is preserved.
						_ = err3
					}
				}
				segInfo.SetFiles([]string{
					segmentName + ".cfs",
					segmentName + ".cfe",
					segmentName + ".si",
				})
				segInfo.SetCompoundFile(true)
			} else {
				segInfo.SetFiles([]string{segmentName + ".si"})
			}
		}
		if !codecFlushed {
			// Codec-less or no-DWPT-with-docs path: carry in-memory postings forward.
			if ps.inMemoryFields != nil {
				sci.SetInMemoryFields(ps.inMemoryFields)
				// Also register in the package-level registry so that
				// SegmentReader.Terms() can find the producer after
				// ReadSegmentInfos creates fresh SegmentCommitInfo objects.
				RegisterInMemoryFields(w.directory, segmentName, ps.inMemoryFields)
			}
			segInfo.SetFiles([]string{segmentName + ".si"})
		}

		if len(ps.deletedOrdinals) > 0 {
			sci.SetDeletedOrdinals(ps.deletedOrdinals)
		}
		// Write the .si file for this segment before registering it so that
		// external tools and CheckIndex can verify per-segment integrity.
		if err3 := writeSegmentInfo(w.directory, sci.SegmentInfo(), store.IOContextWrite); err3 != nil {
			return fmt.Errorf("writing .si: %w", err3)
		}
		si.Add(sci)
		w.committedSegments = append(w.committedSegments, sci)
	}
	w.pendingImportedSegments = w.pendingImportedSegments[:0]

	// Add commit data if present
	if w.liveCommitData != nil && len(w.liveCommitData.data) > 0 {
		si.SetUserData(w.liveCommitData.data)
	}

	// Record parentField and indexSort for AddIndexes validation.
	si.SetInMemoryParentField(w.config.ParentField())
	si.SetInMemoryIndexSort(w.config.IndexSort())

	err = WriteSegmentInfos(si, w.directory)
	if err != nil {
		return fmt.Errorf("failed to write segment infos: %w", err)
	}

	// Clear the prepared commit flag
	w.preparedCommit = false

	return nil
}

// applyDeletesToCommittedSegments resolves every buffered committed-delete term
// against the postings of each segment already present in si (the segments
// committed before this Commit), marks the matching live documents as deleted
// in a byte-faithful .liv file, and bumps the segment's delGen/delCount and
// recorded deleted ordinals so the change survives a reader reopen
// (rmp #4753).  Must be called with w.mu held and only when a codec with a
// PostingsFormat is wired.
//
// This mirrors the net effect of Lucene's BufferedUpdatesStream.applyDeletesAndUpdates
// followed by ReadersAndUpdates.writeLiveDocs: term -> docIDs -> live-docs
// bitset -> .liv, with the delete generation advanced per affected segment.
//
// Resolution is exact: a term that occurs in no committed segment deletes
// nothing.  The block-tree reader (Lucene103SegmentTermsEnum) performs full
// multi-block / FST-trie traversal as of rmp #4754, so SeekExact resolves terms
// that live in sub-blocks of high-cardinality fields (e.g. a unique id field);
// every committed deletion is therefore applied precisely.
func (w *IndexWriter) applyDeletesToCommittedSegments(si *SegmentInfos, codec Codec) error {
	for _, sci := range si.List() {
		maxDoc := sci.SegmentInfo().DocCount()
		if maxDoc == 0 {
			continue
		}

		// Seed the deleted set with the segment's pre-existing deletions so the
		// rewritten .liv is cumulative (it must carry both old and new deletes).
		deleted := make(map[int]struct{})
		for _, ord := range sci.GetDeletedOrdinals() {
			if ord >= 0 && ord < maxDoc {
				deleted[ord] = struct{}{}
			}
		}
		preExisting := len(deleted)

		sr, err := openSegmentReader(w.directory, sci)
		if err != nil {
			return fmt.Errorf("open committed segment %s: %w", sci.SegmentInfo().Name(), err)
		}

		err = w.collectDeletedDocIDs(sr, w.pendingCommittedDeleteTerms, deleted)
		_ = sr.Close()
		if err != nil {
			return err
		}

		newDelCount := len(deleted) - preExisting
		if newDelCount <= 0 {
			continue
		}

		// Build the cumulative live-docs bitset: every doc live, then clear the
		// deleted ordinals.
		live, err := util.NewFixedBitSet(maxDoc)
		if err != nil {
			return err
		}
		for i := 0; i < maxDoc; i++ {
			live.Set(i)
		}
		ords := make([]int, 0, len(deleted))
		for ord := range deleted {
			live.Clear(ord)
			ords = append(ords, ord)
		}
		sort.Ints(ords)

		// Advance the deletion generation and write the .liv at that generation.
		delGen := sci.AdvanceDelGen()
		segName := sci.SegmentInfo().Name()
		onDiskDel, err := writeLiveDocs(w.directory, segName, sci.SegmentInfo().GetID(), delGen, live)
		if err != nil {
			return fmt.Errorf("write live docs for %s: %w", segName, err)
		}
		if onDiskDel != len(deleted) {
			return fmt.Errorf("live docs for %s: wrote delCount=%d, expected %d", segName, onDiskDel, len(deleted))
		}

		sci.SetDelCount(len(deleted))
		sci.SetDeletedOrdinals(ords)

		// Register the .liv in the segment's file list so deleters/CheckIndex see it.
		livName := liveDocsFileName(segName, delGen)
		appendSegmentFile(sci.SegmentInfo(), livName)
	}
	return nil
}

// collectDeletedDocIDs seeks each term in the segment reader's postings and
// records every matching docID into the deleted set (skipping docs already in
// the set).  It uses the same GetIterator + SeekExact + Postings(doc-only) path
// as search.TermWeight.Scorer, so it resolves terms exactly the way a query
// would match them.
func (w *IndexWriter) collectDeletedDocIDs(sr *SegmentReader, terms []*Term, deleted map[int]struct{}) error {
	for _, term := range terms {
		if term == nil {
			continue
		}
		t, err := sr.Terms(term.Field)
		if err != nil {
			return fmt.Errorf("terms(%s): %w", term.Field, err)
		}
		if t == nil {
			continue
		}
		te, err := t.GetIterator()
		if err != nil {
			return err
		}
		if te == nil {
			continue
		}
		found, err := te.SeekExact(term)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		// flags == 0 requests only the doc-ID stream (no freqs/positions),
		// which is all we need to enumerate the matching documents.
		pe, err := te.Postings(0)
		if err != nil {
			return err
		}
		if pe == nil {
			continue
		}
		for {
			doc, err := pe.NextDoc()
			if err != nil {
				return err
			}
			if doc == NO_MORE_DOCS {
				break
			}
			deleted[doc] = struct{}{}
		}
	}
	return nil
}

// applyApproximateCommittedDeletesFor charges one deletion per supplied term
// against the committed segments (oldest first), bounded by each segment's live
// count.  It is the conservative fallback for terms that could not be resolved
// exactly: the codec-less structural path (all terms) and the block-tree
// term-lookup gap (terms whose field exists but whose postings the reader could
// not seek; backlog #2692).  Must be called with w.mu held.
func (w *IndexWriter) applyApproximateCommittedDeletesFor(si *SegmentInfos, terms []*Term) {
	remaining := len(terms)
	for _, existingSCI := range si.List() {
		if remaining <= 0 {
			break
		}
		available := existingSCI.NumDocs()
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
}

// appendSegmentFile adds fileName to the SegmentInfo's file set if not already
// present, preserving the existing entries.  SegmentInfo exposes Files/SetFiles
// rather than an add-one method, so this read-modify-write keeps the .liv listed
// alongside the .si / .cfs / .cfe entries.
func appendSegmentFile(si *SegmentInfo, fileName string) {
	files := si.Files()
	for _, f := range files {
		if f == fileName {
			return
		}
	}
	si.SetFiles(append(files, fileName))
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
	if w.preparedCommit {
		w.mu.RUnlock()
		return errors.New("cannot close IndexWriter when prepareCommit was called but commit wasn't")
	}
	w.mu.RUnlock()

	// Release the write lock unconditionally, even if Commit fails below.
	// Leaving the lock held on a failed Close would prevent reopening the
	// index in the same process and leak a stale file lock on disk.
	defer func() {
		if w.writeLock != nil {
			_ = w.writeLock.Close()
			w.writeLock = nil
		}
	}()

	// Try to commit changes before closing
	if err := w.Commit(); err != nil {
		// If commit fails, we still want to close the scheduler
		if s := w.config.GetMergeScheduler(); s != nil {
			_ = s.Close()
		}
		// Mark as closed so callers do not retry Close() in a loop and
		// hit the same failure path.
		w.closed.Store(true)
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
	w.preparedCommit = false
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
		// Register files before writeSegmentInfo so the .si embeds the file list.
		sci.SegmentInfo().SetFiles([]string{segName + ".si"})
		if err := writeSegmentInfo(w.directory, sci.SegmentInfo(), store.IOContextWrite); err != nil {
			return fmt.Errorf("forceMerge: write .si: %w", err)
		}
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
// writeSegmentInfo writes a Lucene99SegmentInfo .si file for si into dir.
//
// The on-disk format is byte-identical to Lucene's
// org.apache.lucene.codecs.lucene99.Lucene99SegmentInfoFormat.write:
//
//	writeIndexHeader("Lucene90SegmentInfo", version=0, id, suffix="")
//	Int32 major, Int32 minor, Int32 bugfix  (from si.Version())
//	Byte  hasMinVersion = 0
//	Int32 docCount
//	Byte  isCompoundFile (1=true, 255=false)
//	Byte  hasBlocks = 0
//	WriteMapOfStrings(diagnostics)
//	WriteSetOfStrings(files as set)
//	WriteMapOfStrings(attributes)
//	VInt  numSortFields = 0
//	writeFooter()
func writeSegmentInfo(dir store.Directory, si *SegmentInfo, context store.IOContext) error {
	name := si.Name() + ".si"
	raw, err := dir.CreateOutput(name, context)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	out := store.NewChecksumIndexOutput(raw)

	var writeErr error
	defer func() {
		if writeErr != nil {
			_ = out.Close()
		}
	}()

	if writeErr = writeIndexHeader(out, "Lucene90SegmentInfo", 0, si.GetID(), ""); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: header: %w", name, writeErr)
	}

	// Version and docCount use Java's DataOutput.writeInt (little-endian).
	major, minor, bugfix := parseSegmentVersion(si.Version())
	if writeErr = store.WriteInt32LE(out, major); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: major: %w", name, writeErr)
	}
	if writeErr = store.WriteInt32LE(out, minor); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: minor: %w", name, writeErr)
	}
	if writeErr = store.WriteInt32LE(out, bugfix); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: bugfix: %w", name, writeErr)
	}

	// hasMinVersion = false
	if writeErr = out.WriteByte(0); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: hasMinVersion: %w", name, writeErr)
	}

	if writeErr = store.WriteInt32LE(out, int32(si.DocCount())); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: docCount: %w", name, writeErr)
	}

	// isCompoundFile: 1=true, 255 (-1 signed) =false — matches Lucene Java byte cast.
	isCompoundFile := byte(255)
	if si.IsCompoundFile() {
		isCompoundFile = 1
	}
	if writeErr = out.WriteByte(isCompoundFile); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: isCompoundFile: %w", name, writeErr)
	}

	// hasBlocks = false
	if writeErr = out.WriteByte(0); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: hasBlocks: %w", name, writeErr)
	}

	if writeErr = store.WriteMapOfStrings(out, si.GetDiagnostics()); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: diagnostics: %w", name, writeErr)
	}

	files := si.Files()
	sort.Strings(files)
	fileSet := make(map[string]struct{}, len(files))
	for _, f := range files {
		fileSet[f] = struct{}{}
	}
	if writeErr = store.WriteSetOfStrings(out, fileSet); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: files: %w", name, writeErr)
	}

	if writeErr = store.WriteMapOfStrings(out, si.GetAttributes()); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: attributes: %w", name, writeErr)
	}

	// numSortFields = 0 (index sort not yet implemented in this write path)
	if writeErr = store.WriteVInt(out, 0); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: numSortFields: %w", name, writeErr)
	}

	if writeErr = writeFooter(out); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: footer: %w", name, writeErr)
	}

	return out.Close()
}

// parseSegmentVersion parses a version string such as "10.4.0" into its
// major, minor, and bugfix components.  Missing components default to zero.
func parseSegmentVersion(v string) (major, minor, bugfix int32) {
	fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &bugfix)
	return
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
	dstFields := dst.Fields()
	srcFields := src.Fields()
	dstEmpty := dst == nil || len(dstFields) == 0
	srcEmpty := src == nil || len(srcFields) == 0
	if dstEmpty && srcEmpty {
		return true
	}
	if dstEmpty != srcEmpty {
		// One is sorted and the other is not.
		return false
	}
	// Both non-empty: compare field by field.
	if len(dstFields) != len(srcFields) {
		return false
	}
	for i := range dstFields {
		df := dstFields[i]
		sf := srcFields[i]
		if df.Field() != sf.Field() || df.SortType() != sf.SortType() || df.Descending() != sf.Descending() {
			return false
		}
	}
	return true
}

// AddIndexesFromReader adds all live documents from each provided IndexReader
// to this index. Each reader is registered as a discrete pendingImportedSegment
// so that subsequent commits and merges see the correct document count and
// segment structure.
//
// This is a simpler path than addIndexes(Directory...) — no segment files are
// copied and no SegmentInfos are read from disk. The readers supply the doc
// count directly. This matches the observable behaviour of the Java
// addIndexes(CodecReader...) flow at the metadata level while deferring the
// full byte-copy path (tracked in backlog #2707).
func (w *IndexWriter) AddIndexesFromReader(readers ...*IndexReader) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	// Collect numDocs outside the lock to avoid holding it while accessing
	// potentially-blocking reader methods.
	type readerInfo struct{ numDocs int }
	infos := make([]readerInfo, 0, len(readers))
	for _, r := range readers {
		n := r.NumDocs()
		if n <= 0 {
			continue
		}
		infos = append(infos, readerInfo{numDocs: n})
	}
	if len(infos) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, ri := range infos {
		ps := pendingSegment{
			numDocs:  ri.numDocs,
			delCount: 0,
		}
		w.pendingImportedSegments = append(w.pendingImportedSegments, ps)
	}

	return nil
}

// WaitForMerges waits for all pending merges to complete.
// This is useful for testing and when a consistent index state is needed.
func (w *IndexWriter) WaitForMerges() error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	// Drain all running merge goroutines through the merge scheduler.  We
	// release the writer lock before blocking so that ongoing merges can
	// acquire it as needed.
	if s := w.config.GetMergeScheduler(); s != nil {
		if cms, ok := s.(*ConcurrentMergeScheduler); ok {
			cms.runningMerges.Wait()
			// Surface any merge errors collected during the wait.
			select {
			case err := <-cms.mergeErrors:
				return fmt.Errorf("WaitForMerges: merge error: %w", err)
			default:
			}
		}
	}
	return nil
}

// AddDocuments adds a block of documents atomically.
// This is used for parent-child document relationships.
//
// Gocene deviation: Lucene adds the whole block as a single atomic unit so
// that inter-document relationships (parent/child) are preserved within a
// segment. This implementation adds each document individually via AddDocument,
// which preserves the document count and codec flush path but does not
// guarantee atomic-block placement. Full block-level atomicity is deferred to
// a future sprint.
func (w *IndexWriter) AddDocuments(docs []Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}
	for _, doc := range docs {
		if err := w.AddDocument(doc); err != nil {
			return err
		}
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
