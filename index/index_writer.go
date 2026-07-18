// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// writeLockName is the file name used as the write lock for an index directory.
// Matches Lucene's IndexWriter.WRITE_LOCK_NAME.
const writeLockName = "write.lock"

// MAX_TERM_LENGTH is the maximum length (in bytes) a term may have when
// indexed via IndexWriter.  Terms longer than this are refused with an
// IllegalArgumentException mentioning "immense term".  The value mirrors
// Lucene's IndexWriter.MAX_TERM_LENGTH = ByteBlockPool.BYTE_BLOCK_SIZE - 2.
const MAX_TERM_LENGTH = 32766

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

	// addLock serializes the per-document atomic unit performed by AddDocument
	// and UpdateDocument: processing the document into the DocumentsWriter
	// (which increments the DWPT's numDocsInRAM), the matching docCount.Add(1),
	// and the auto-flush trigger check.  These three steps MUST be one
	// indivisible unit per document so that docCount and the DWPT document
	// count never drift.  Without this, a flush — which reconciles its segment
	// numDocs from the live DWPT count and then resets docCount to zero — can
	// run after a concurrent document has already been processed into the DWPT
	// (numDocsInRAM incremented) but before that document's docCount.Add(1) has
	// executed.  The flush captures the DWPT document, resets docCount, and the
	// late docCount.Add(1) then becomes a ghost count, inflating MaxDoc by one
	// per such interleaving (rmp #4772).  addLock closes that window.
	addLock sync.Mutex

	// commitLock serializes Commit, PrepareCommit, GetReader (NRT) and DeleteAll
	// against one another.  Lucene uses a dedicated fullFlushLock/commitLock so
	// that a commit can drain in-flight DWPT flushes and apply buffered deletes
	// atomically without racing concurrent add/update/delete operations.  In
	// Gocene's simplified model, holding commitLock around the whole commit and
	// NRT-reader construction prevents an interleaving where a DeleteDocuments
	// that targets buffered docs is recorded after the commit has already
	// snapshotted and flushed those docs, which would otherwise lose the delete
	// (rmp #4753 follow-up).  Concurrent adds/updates continue to serialize on
	// addLock, and commitLock is acquired after addLock so the lock order is
	// addLock → commitLock → mu.
	commitLock sync.Mutex

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

	// pinnedSegmentInfos holds the SegmentInfos for an IndexWriterConfig.IndexCommit
	// when the writer is opened against a specific past commit. It overrides the
	// on-disk latest SegmentInfos for doc-count accounting and for the next commit.
	pinnedSegmentInfos *SegmentInfos

	// lastCommittedSegmentInfos is a clone of the SegmentInfos that was last
	// successfully written to disk by this writer. It is used by Rollback to
	// restore the directory to the last committed state, discarding any
	// uncommitted segments, merges, or buffered changes.
	lastCommittedSegmentInfos *SegmentInfos

	// hasCommitted is true once this writer has written at least one commit to
	// disk. It prevents the "nothing changed" short-circuit from skipping the
	// very first commit on a newly created empty index, which must still produce
	// a segments_N file so that APPEND-mode writers and readers can open it.
	hasCommitted bool

	// rollbackSegmentInfos holds the SegmentInfos baseline the writer should
	// restore to on Rollback. It is captured at writer construction time from the
	// pinned commit (or latest on-disk commit) and represents the last safe
	// committed state before this writer made any changes.
	rollbackSegmentInfos *SegmentInfos

	// startingFiles records the set of files that existed in the directory when
	// this writer was created. Rollback preserves these files so that commits
	// created before the writer was opened (e.g. under a keep-all deletion policy)
	// are not accidentally removed.
	startingFiles map[string]struct{}

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

	// pendingDVUpdates buffers UpdateDocValues and TryUpdateDocValue requests
	// until the next Commit, when they are resolved against committed segments
	// and written as Lucene-compatible per-generation .dvd/.dvm files and a new
	// .fnm.  Protected by mu.
	pendingDVUpdates []pendingDocValuesUpdate

	// pendingDeleteQueries holds query-based deletes (DeleteDocumentsQuery) that
	// must be resolved against committed segments at the next Commit.  Each query
	// is executed via an IndexSearcher opened over the committed segments; matching
	// docIDs are marked as deleted in .liv files exactly as term-based deletes.
	// Stored as interface{} to avoid a compile-time constraint on the query type
	// (the concrete search.Query type lives in package search, not index).
	// Protected by mu.
	pendingDeleteQueries []interface{}

	// pendingDeleteAll is set by DeleteAll to request that all segments'
	// documents be marked as deleted at the next Commit.  When true, Commit
	// writes .liv files for every committed segment with all docs deleted
	// and discards all pending state.  Protected by mu.
	pendingDeleteAll bool

	// writeLock is the exclusive directory write lock held for the lifetime of
	// this IndexWriter instance.  Nil only if locking is not supported by the
	// directory (legacy path; should not happen with current store implementations).
	writeLock store.Lock

	// liveCommitData holds the current commit data that will be written on next commit
	liveCommitData *commitData
	// preparedCommit indicates if prepareCommit has been called
	preparedCommit bool

	// flushCount tracks the number of times flushPendingDocsLocked has
	// successfully materialised buffered documents into a pendingSegment.
	// Accessed atomically; incremented under mu.
	flushCount atomic.Int32

	// nrtGen is incremented each time GetReader materialises new content
	// (flushed segments).  NRT readers compare their snapshot generation
	// against this counter to cheaply determine whether the index has changed
	// since they were opened (used by OpenIfChangedFromWriter and
	// NRTDirectoryReader.IsCurrent).
	nrtGen atomic.Int64

	// nrtSegmentInfos holds the last NRT snapshot SIS (includes
	// flushed-but-not-committed segments).  Used as the base for
	// subsequent GetReader calls so previously NRT-flushed segments
	// remain visible across multiple GetReader calls within the same
	// writer session.
	nrtSegmentInfos *spi.SegmentInfos

	// deleter tracks reference counts for every segment file and enforces the
	// configured IndexDeletionPolicy on writer init and on every commit.
	deleter *IndexFileDeleter
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

	// segmentName is assigned the first time this pending segment is
	// materialised into an NRT snapshot; subsequent GetReader/Commit calls
	// reuse it so in-memory postings registries stay aligned with the
	// on-disk segment name.
	segmentName string

	// materialized is true once this pending segment has been folded into an
	// NRT SegmentInfos snapshot. GetReader skips materialized segments so they
	// are not duplicated, while Commit still writes them to disk.
	materialized bool
	inMemoryFields      FieldsProducer // in-memory postings (codec-less path); may be nil

	// dwpts carries the raw per-thread writer state when a real codec is
	// present.  Commit calls dwpt.Flush to materialise stored fields,
	// postings, and field infos on disk. nil when the writer was opened
	// without a codec (structural-unit-test path).
	dwpts []*DocumentsWriterPerThread

	// The following fields are populated by AddIndexes (directory path) and
	// describe a source segment whose files must be copied into the main
	// directory when this pending segment is committed.
	srcDir          store.Directory  // source directory (nil for flushed DWPT segments)
	srcSegmentName  string           // source segment name used as the file prefix
	srcFiles        []string         // source file names to copy
	srcCompoundFile bool             // whether the source segment used a compound file
	srcSegmentInfo  *schema.SegmentInfo // source SegmentInfo metadata to preserve binary compatibility (ID, codec, version, ...)
	segmentID       []byte           // 16-byte segment ID assigned when first materialised (NRT/merge)
	files           []string         // copied file names in w.directory (populated after copying)
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

	// Validate IndexWriterConfig.setIndexCommit usage up front, mirroring
	// Lucene's IndexWriter constructor checks (TestIndexWriterFromReader).
	if commit := config.IndexCommit(); commit != nil {
		if config.OpenMode() == CREATE {
			return nil, errors.New(
				"cannot use IndexWriterConfig.setIndexCommit() with OpenMode.CREATE")
		}
		// The index must already carry a commit for the pinned commit to be
		// meaningful; an index with no commit on disk is rejected.
		if _, siErr := ReadSegmentInfos(dir); siErr != nil {
			return nil, fmt.Errorf(
				"cannot use IndexWriterConfig.setIndexCommit() when index has no commit: %w", siErr)
		}
	}

	// Obtain the exclusive write lock.  This must happen before any reads or
	// writes so that concurrent writers on the same directory are rejected.
	wl, err := dir.ObtainLock(writeLockName)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain write lock - another IndexWriter may be open: %w", err)
	}

	// Validate OpenMode constraints.  APPEND requires an existing commit; CREATE
	// and CREATE_OR_APPEND tolerate an empty directory.  If validation fails,
	// release the write lock so callers can retry (LUCENE-715).
	_, siErr := ReadSegmentInfos(dir)
	indexExists := siErr == nil
	switch config.OpenMode() {
	case APPEND:
		if !indexExists {
			_ = wl.Close()
			return nil, errors.New("no existing index found for OpenMode.APPEND")
		}
	case CREATE:
		// OpenMode.CREATE truncates any existing index, mirroring Lucene's
		// IndexWriter constructor. The write lock itself is preserved.
		if indexExists {
			if all, err := dir.ListAll(); err == nil {
				for _, f := range all {
					if f == writeLockName {
						continue
					}
					_ = dir.DeleteFile(f)
				}
			}
			indexExists = false
		}
	}

	// Record the files present at writer creation so Rollback can distinguish
	// pre-existing commits from new uncommitted files. This snapshot must be
	// taken after any CREATE truncation so it reflects the real starting state.
	startingFiles := make(map[string]struct{})
	if starting, err := dir.ListAll(); err == nil {
		for _, f := range starting {
			startingFiles[f] = struct{}{}
		}
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
	writer.startingFiles = startingFiles

	// Determine the initial SegmentInfos and the rollback baseline.
	//
	// Three cases, matching Lucene's IndexWriter constructor:
	//   1. NRT reader commit (IndexCommit carries a DirectoryReader): the
	//      writer resumes from the reader's in-memory SegmentInfos. The
	//      reader's prior commit file must still exist on disk; if it was
	//      removed by a deletion policy the reader is stale and we reject it.
	//      The rollback baseline is the last on-disk commit.
	//   2. Non-NRT pinned commit (IndexCommit with no reader): the writer reads
	//      the latest on-disk SegmentInfos, then replaces its segment list with
	//      the pinned commit's segments while preserving the latest generation
	//      and counter (write-once). The rollback baseline is the pinned state.
	//   3. No pinned commit: the writer uses the latest on-disk SegmentInfos and
	//      the rollback baseline is that same state.
	var initialSegmentInfos *SegmentInfos
	var rollbackSegmentInfos *SegmentInfos

	if commit := config.IndexCommit(); commit != nil {
		if reader := commit.GetReader(); reader != nil {
			// NRT reader path.
			if err := reader.EnsureOpen(); err != nil {
				_ = wl.Close()
				return nil, NewAlreadyClosedException("IndexCommit's reader is already closed", err)
			}
			if reader.GetDirectory() != dir {
				_ = wl.Close()
				return nil, errors.New("IndexCommit's reader must have the same directory passed to IndexWriter")
			}

			readerSegmentInfos := reader.GetSegmentInfos()
			if readerSegmentInfos == nil {
				_ = wl.Close()
				return nil, errors.New("IndexCommit's reader has no SegmentInfos")
			}
			if readerSegmentInfos.GetFileName() == "" {
				_ = wl.Close()
				return nil, errors.New("index must already have an initial commit to open from reader")
			}

			// The reader's commit file must still be present; otherwise the reader
			// is stale. This happens when the writer committed/closed after the
			// NRT reader was opened and the default deletion policy removed the
			// old commit file.
			lastCommit, err := readSegmentInfosByFileName(dir, readerSegmentInfos.GetFileName())
			if err != nil {
				_ = wl.Close()
				return nil, fmt.Errorf(
					"the provided reader is stale: its prior commit file %q is missing from index: %w",
					readerSegmentInfos.GetFileName(), err)
			}

			// Resume from the reader's in-memory snapshot, but keep generation and
			// counter in sync with the directory so future commits remain write-once.
			initialSegmentInfos = readerSegmentInfos.Clone()
			initialSegmentInfos.SetGeneration(lastCommit.Generation())
			initialSegmentInfos.SetCounter(lastCommit.Counter())
			initialSegmentInfos.SetLastGeneration(lastCommit.LastGeneration())

			// Rollback restores to the last on-disk commit, since the in-memory
			// additions above that commit belong to the previous writer's session.
			rollbackSegmentInfos = lastCommit.Clone()
		} else {
			// Non-NRT pinned commit path.
			if commit.GetDirectory() != dir {
				_ = wl.Close()
				return nil, fmt.Errorf(
					"IndexCommit's directory doesn't match my directory, expected=%v, got=%v",
					dir, commit.GetDirectory())
			}

			latestSI, err := ReadSegmentInfos(dir)
			if err != nil {
				_ = wl.Close()
				return nil, fmt.Errorf("reading latest SegmentInfos: %w", err)
			}

			pinnedSI, err := readSegmentInfosByFileName(dir, commit.GetSegmentsFileName())
			if err != nil {
				_ = wl.Close()
				return nil, fmt.Errorf("reading pinned SegmentInfos %q: %w",
					commit.GetSegmentsFileName(), err)
			}

			// Start from the latest on-disk metadata (generation, counter, version)
			// but with the pinned commit's segment list and user data.
			initialSegmentInfos = latestSI.Clone()
			initialSegmentInfos.Replace(pinnedSI)

			// Rollback restores to the pinned state.
			rollbackSegmentInfos = pinnedSI.Clone()
		}
	} else {
		// Normal path: use the latest on-disk commit if it exists.
		if existingSI, err := ReadSegmentInfos(dir); err == nil {
			initialSegmentInfos = existingSI.Clone()
			rollbackSegmentInfos = existingSI.Clone()
		} else {
			initialSegmentInfos = NewSegmentInfos()
			initialSegmentInfos.SetGeneration(1)
			rollbackSegmentInfos = initialSegmentInfos.Clone()
		}
	}

	// Populate committedSegments and lastCommittedSegmentInfos from the initial
	// baseline so that UpdateDocument can detect committed fields and Commit can
	// advance from the correct generation.
	writer.committedSegments = initialSegmentInfos.List()
	writer.lastCommittedSegmentInfos = initialSegmentInfos.Clone()
	writer.pinnedSegmentInfos = initialSegmentInfos.Clone()
	writer.rollbackSegmentInfos = rollbackSegmentInfos.Clone()

	// Validate that the index sort matches the existing commit. Changing the
	// index sort on an existing index is not allowed (Lucene throws
	// IllegalArgumentException). This check is skipped for CREATE mode
	// (openMode == CREATE) where the directory is expected to be empty,
	// but a stale segments_N left behind by a previous crash may still
	// exist; in that case the existing sort must match.
	if initialSort := initialSegmentInfos.GetInMemoryIndexSort(); initialSort != nil {
		configSort := config.IndexSort()
		if !sortsCompatible(initialSort, configSort) {
			_ = wl.Close()
			return nil, fmt.Errorf(
				"cannot change index sort from %v to %v: index sort cannot be changed after the index was created",
				initialSort, configSort)
		}
	}

	// Wire the file deleter so that commit/rollback correctly reference-count
	// segment files and enforce the configured deletion policy.  When the writer
	// is pinned to a specific prior commit (SetIndexCommit), the deleter must
	// start from that commit so it does not treat newer commits as live and then
	// try to delete files that belong to segments already advanced by those
	// newer commits.
	files, _ := dir.ListAll()
	policy := config.GetIndexDeletionPolicy()
	if policy == nil {
		// Lucene's default is KeepOnlyLastCommitDeletionPolicy.
		policy = NewKeepOnlyLastCommitDeletionPolicy()
	}
	// initialIndexExists is true only when initialSegmentInfos is backed by a
	// real on-disk commit. For an empty index created on the fly the current
	// segments_N does not exist yet and the deleter must not try to force-read it.
	initialIndexExists := dir.FileExists(initialSegmentInfos.GetFileName())
	deleter, deleterErr := NewIndexFileDeleter(
		files, dir, dir, policy, initialSegmentInfos.Clone(),
		config.GetInfoStream(), writer, initialIndexExists, false)
	if deleterErr != nil {
		_ = wl.Close()
		return nil, fmt.Errorf("IndexFileDeleter init: %w", deleterErr)
	}
	writer.deleter = deleter
	if startingDeleted := deleter.StartingCommitDeleted(); startingDeleted {
		// The deletion policy removed the commit this writer opened on.
		// For a pinned commit that is a valid user choice; for the latest
		// commit it indicates the policy discarded the head.
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

	// Validate sort field types against the DV fields in this document.
	if sort := w.config.IndexSort(); sort != nil {
		if err := w.validateSortFieldTypes(doc, sort); err != nil {
			return err
		}
	}

	// Serialize the per-document atomic unit (process → docCount.Add → flush
	// check) so docCount and the DWPT document count cannot drift across a
	// concurrent flush (rmp #4772; see the addLock field documentation).
	w.addLock.Lock()
	defer w.addLock.Unlock()

	// Enforce the configured per-writer document limit.  Mirrors Lucene's
	// IndexWriter.reserveDocs / tooManyDocs contract.
	if maxDocs := w.config.MaxDocs(); maxDocs > 0 {
		if current := w.maxDocForLimit(); current >= maxDocs {
			return fmt.Errorf(
				"number of documents in the index cannot exceed %d (current document count is %d)",
				maxDocs, current)
		}
	}

	// DocumentsWriter has its own internal locking, so we don't need
	// to hold the global lock during document processing.
	if w.documentsWriter != nil {
		if err := w.documentsWriter.AddDocument(doc, nil); err != nil {
			return err
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
	// parentMarkerField carries the parent-field bit (rmp #4789): when the field
	// is the synthetic parent marker injected by AddDocuments, set IsParentField
	// so it is serialised to the .fnm and survives a cold reopen.
	isParentField := false
	if pm, ok := fm.(interface{ IsParentMarker() bool }); ok {
		isParentField = pm.IsParentMarker()
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
		IsParentField:            isParentField,
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

	// Serialize the per-document atomic unit (process → docCount.Add → flush
	// check) against concurrent AddDocument/UpdateDocument calls so docCount and
	// the DWPT document count cannot drift across a flush (rmp #4772; see the
	// addLock field documentation).
	w.addLock.Lock()
	defer w.addLock.Unlock()

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

	// Append path: add new doc and record bounded delete for matching old docs.
	// Enforce the per-writer document limit on the replacement document.
	if maxDocs := w.config.MaxDocs(); maxDocs > 0 && w.maxDocForLimit() >= maxDocs {
		return fmt.Errorf(
			"number of documents in the index cannot exceed %d (current document count is %d)",
			maxDocs, w.maxDocForLimit())
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

	maxOrd := int(w.docCount.Load())
	if term != nil {
		w.pendingDeleteTerms = append(w.pendingDeleteTerms, termWithBound{term: term, maxOrdinal: maxOrd})
		// Resolve the term against committed segments at the next Commit so the
		// old committed doc the replacement displaces is actually deleted
		// (rmp #4753).  The committed docs always precede every buffered ordinal,
		// so an unbounded committed-segment delete cannot touch the replacement.
		w.pendingCommittedDeleteTerms = append(w.pendingCommittedDeleteTerms, term)
		// Any in-memory pending segment that already exists at the time of this
		// update predates the replacement doc, so the displacement delete must
		// also reach those flushed-but-not-committed docs (rmp #4753 follow-up).
		if err := w.applyTermDeletesToPendingSegments([]*Term{term}); err != nil {
			w.mu.Unlock()
			return err
		}
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

	// Serialize with the per-document add/update lock so that an in-flight
	// AddDocument either fully completes (and is covered by the delete) or starts
	// after the delete (and is not self-deleted). This gives the buffered delete
	// a clean generation boundary matching Lucene's delete-before-add semantics.
	w.addLock.Lock()
	defer w.addLock.Unlock()

	w.mu.Lock()
	// Bound the buffered-doc delete to the documents already added when this
	// delete is issued (ordinals [0, len(docFieldIndex))). A document added
	// AFTER this DeleteDocuments call must NOT be self-deleted, mirroring
	// Lucene's delete-before-add sequencing where a buffered delete only applies
	// to docs indexed before it. The addLock above guarantees docFieldIndex is
	// consistent with the DWPT state.
	w.pendingDeleteTerms = append(w.pendingDeleteTerms, termWithBound{term: term, maxOrdinal: len(w.docFieldIndex)})
	// Also resolve this term against already-committed segments at the next
	// Commit so deletions take effect across commits (rmp #4753).
	if term != nil {
		w.pendingCommittedDeleteTerms = append(w.pendingCommittedDeleteTerms, term)
		// In-memory pending segments that already exist at the time of this
		// delete predate every doc added after it, so the delete must also
		// reach those flushed-but-not-committed docs.
		if err := w.applyTermDeletesToPendingSegments([]*Term{term}); err != nil {
			w.mu.Unlock()
			return err
		}
	}
	w.mu.Unlock()
	return nil
}

// DeleteDocumentsQuery buffers a query-based delete to be applied at the next
// Commit. Matching documents in committed segments are identified by executing
// the query via IndexSearcher and are marked as deleted in their .liv files.
//
// The query parameter must implement the index.Query interface; queries from
// the search package satisfy this. Non-Query values are silently ignored.
//
// Queries that are semantically equivalent to a term delete (TermDeleteQuery,
// e.g. search.TermQuery) are routed through DeleteDocuments so they obey the
// same buffered-document generation semantics and are applied to in-memory
// pending segments and future flushed segments exactly as term deletes are.
//
// Execution requires a query-delete executor hook registered via
// RegisterQueryDeleteExecutor (installed by the search package's init).
// When no executor is registered the query is silently dropped.
func (w *IndexWriter) DeleteDocumentsQuery(query interface{}) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}
	if query == nil {
		return nil
	}

	// Route term-equivalent queries through the term-delete path.  This gives
	// them the same maxOrdinal generation boundary as DeleteDocuments, ensuring
	// buffered documents added before the delete are removed while documents
	// added after the delete survive (rmp #4753).
	if termDelete, ok := query.(TermDeleteQuery); ok {
		term := termDelete.DeleteTerm()
		if term != nil {
			return w.DeleteDocuments(term)
		}
		return nil
	}

	// Serialize with the per-document add/update lock for the same generation
	// boundary reason as DeleteDocuments.
	w.addLock.Lock()
	defer w.addLock.Unlock()

	w.mu.Lock()
	w.pendingDeleteQueries = append(w.pendingDeleteQueries, query)
	// In-memory pending segments that already exist at the time of this query
	// delete predate every doc added after it, so the query must also reach
	// those flushed-but-not-committed docs.
	if err := w.applyQueryDeletesToPendingSegments([]interface{}{query}); err != nil {
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()
	return nil
}

// QueryDeleteExecutor is a hook function that executes queries against a
// directory and returns the docIDs that match on each segment.
// Installed by the search package to break the index → search cycle.
//
// fn(dir, segmentInfos, queries) → per-segment map[segmentName]→[]docID.
// The queries slice contains concrete search.Query values stored as interface{}.
type QueryDeleteExecutor func(dir store.Directory, si *spi.SegmentInfos, queries []interface{}) (map[string][]int, error)

// TermDeleteQuery is implemented by query types that are semantically
// equivalent to a single term delete (e.g. search.TermQuery).  IndexWriter
// routes such queries through the term-delete path so they obey the same
// buffered-document generation semantics and are applied to both in-memory
// pending segments and future flushed segments.
type TermDeleteQuery interface {
	DeleteTerm() *Term
}

var (
	queryDeleteExecutorMu sync.RWMutex
	queryDeleteExecutor   QueryDeleteExecutor
)

// RegisterQueryDeleteExecutor installs the process-wide query-delete executor.
// Called from the search package init to avoid a circular import.
func RegisterQueryDeleteExecutor(fn QueryDeleteExecutor) {
	queryDeleteExecutorMu.Lock()
	queryDeleteExecutor = fn
	queryDeleteExecutorMu.Unlock()
}

// lookupQueryDeleteExecutor returns the registered query-delete executor, or nil.
func lookupQueryDeleteExecutor() QueryDeleteExecutor {
	queryDeleteExecutorMu.RLock()
	defer queryDeleteExecutorMu.RUnlock()
	return queryDeleteExecutor
}

// commitData holds user-defined commit data.
type commitData struct {
	data map[string]string
}

// SetLiveCommitData sets the commit data that will be written with the next commit.
// This data is stored in the commit point and can be retrieved later.
// The data is "live" meaning it can be modified through the provided map until
// the actual commit happens, mirroring Lucene's late-binding semantics.
func (w *IndexWriter) SetLiveCommitData(data map[string]string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.liveCommitData = &commitData{data: data}
}

// getLiveCommitData returns the current live commit data
func (w *IndexWriter) getLiveCommitData() map[string]string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.getLiveCommitDataLocked()
}

// getLiveCommitDataLocked is the unlocked variant; the caller must hold w.mu.
func (w *IndexWriter) getLiveCommitDataLocked() map[string]string {
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
		// No buffered documents to materialise, but deletes targeting buffered
		// docs have no effect once the doc window is empty.  Discard them so
		// they are not double-applied on a later commit.  Leave
		// pendingDeletedDocIDs (TryDeleteDocument global docIDs) untouched: they
		// target committed segments and must survive until the next Commit/NRT
		// reader applies them.
		w.pendingDeleteTerms = w.pendingDeleteTerms[:0]
		w.pendingSoftDeletedOrdinals = w.pendingSoftDeletedOrdinals[:0]
		w.docFieldIndex = w.docFieldIndex[:0]
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
		if len(pool) > 0 {
			// Always merge in-memory postings so GetReader (NRT) can
			// create in-memory segments without writing codec files.
			// When a codec is active the DWPTs are preserved for
			// Commit to write the real files; the in-memory postings
			// serve as the NRT-reader fallback (SegmentReader.Terms
			// falls through to in-memory FieldsProducer when
			// coreReaders is nil).
			inMemFields = MergeInMemoryPostings(pool)
			if w.config.Codec() == nil {
				pool = nil // codec-less: DWPTs not needed beyond merge
			}
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
	w.flushCount.Add(1)

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

	// Serialize with concurrent add/update/delete operations and with Commit,
	// GetReader and DeleteAll so that the flush snapshot captures a stable
	// point-in-time of buffered documents and pending deletes.
	w.addLock.Lock()
	defer w.addLock.Unlock()
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

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

// GetReader returns a near-real-time (NRT) DirectoryReader that reflects
// every document added to this writer so far, including documents still
// buffered and not yet durably committed. It is the Go analogue of
// Lucene's IndexWriter.getReader() (and the engine behind
// DirectoryReader.open(IndexWriter)).
//
// Implementation note: this port realises the NRT contract by flushing
// the buffered documents through the standard commit write-path and then
// opening a DirectoryReader over the resulting segments. Every buffered
// document therefore becomes visible. The one observable difference from
// Lucene — whose getReader flushes to pooled in-memory segments without
// advancing the commit generation — is that GetReader here advances the
// commit point. The no-commit in-memory NRT pooling path (and the
// rollback/commit-pinning semantics that depend on it) is tracked by
// roadmap #118.
func (w *IndexWriter) GetReader() (*DirectoryReader, error) {
	if err := w.ensureOpen(); err != nil {
		return nil, fmt.Errorf("cannot open NRT reader: %w", err)
	}

	// Serialize with concurrent add/update/delete operations and with
	// Commit/PrepareCommit/DeleteAll so the NRT snapshot is stable.
	w.addLock.Lock()
	defer w.addLock.Unlock()
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Flush buffered documents into pending segments.
	if err := w.flushPendingDocsLocked(); err != nil {
		return nil, fmt.Errorf("GetReader: flush failed: %w", err)
	}

	// Determine the base SIS: use the last NRT snapshot (which includes
	// previously flushed segments) if available, otherwise read committed
	// SIS from disk.
	var baseSI *spi.SegmentInfos
	if w.nrtSegmentInfos != nil {
		baseSI = w.nrtSegmentInfos
	} else {
		var err error
		baseSI, err = ReadSegmentInfos(w.directory)
		if err != nil {
			baseSI = NewSegmentInfos()
			baseSI.SetGeneration(1)
		}
	}

	// Build an in-memory NRT snapshot: clone the committed SIS so we can
	// overlay buffered deletes without mutating the previous snapshot or the
	// on-disk SegmentInfos. Lucene leaves the generation unchanged for an
	// in-memory NRT snapshot; advancing it here would make the NRT reader's
	// IndexCommit reference a non-existent segments_N file, which breaks the
	// writer reopen path that verifies the commit file is still on disk.
	nrtSI := baseSI.Clone()

	// Apply buffered deletes (DeleteAll, term, query, docID) against the
	// committed segments in the snapshot.  This makes an NRT reader reflect
	// deletes issued since the last commit without requiring a disk commit.
	if err := w.applyBufferedDeletesToSegmentCommitInfos(nrtSI); err != nil {
		return nil, err
	}

	// Push any delete ordinals computed for the NRT snapshot back to the
	// pending segment records so that a later Commit writes the same live set.
	for i := range w.pendingImportedSegments {
		ps := &w.pendingImportedSegments[i]
		if !ps.materialized {
			continue
		}
		for _, sci := range nrtSI.List() {
			if sci.SegmentInfo().Name() != ps.segmentName {
				continue
			}
			ps.deletedOrdinals = sci.GetDeletedOrdinals()
			ps.delCount = sci.DelCount()
			ps.softDelCount = sci.SoftDelCount()
			break
		}
	}

	for i := range w.pendingImportedSegments {
		ps := &w.pendingImportedSegments[i]
		if ps.materialized {
			continue
		}
		segmentName := nrtSI.GetNextSegmentName()
		ps.segmentName = segmentName
		ps.materialized = true
		segInfo := NewSegmentInfo(segmentName, ps.numDocs, w.directory)
		segInfo.SetID(generateSegmentID())
		segInfo.SetVersion("10.4.0")
		segInfo.SetMinVersion("10.4.0")
		ps.segmentID = segInfo.GetID()
		sci := NewSegmentCommitInfo(segInfo, ps.delCount, -1)
		if ps.softDelCount > 0 {
			sci.SetSoftDelCount(ps.softDelCount)
		}
		if ps.fieldInfos != nil {
			sci.SetInMemoryFieldInfos(ps.fieldInfos)
		}

		// Wire in-memory postings so the codec-less fallback works.
		// Do not stamp a codec name on an in-memory-only segment: it has no
		// on-disk data files, so openSegmentReader would build empty codec
		// readers that hide the in-memory postings. The segment gets its codec
		// name later, during Commit, once the DWPTs are flushed to real files.
		if ps.inMemoryFields != nil {
			sci.SetInMemoryFields(ps.inMemoryFields)
			RegisterInMemoryFields(w.directory, segmentName, ps.inMemoryFields)
		}

		nrtSI.Add(sci)
		w.committedSegments = append(w.committedSegments, sci)
	}

	// Run natural merges on the NRT snapshot so that merge-on-get-reader
	// policies (e.g. LogDocMergePolicy with a low minMergeDocs) are honoured
	// for the returned reader. The merged segment files are written to the
	// directory but the on-disk segments_N is not updated until the next
	// Commit/Close.
	if err := w.maybeMergeSnapshot(nrtSI, GET_READER); err != nil {
		return nil, fmt.Errorf("GetReader: maybeMergeSnapshot: %w", err)
	}

	// The NRT snapshot is *not* a committed baseline: it merely reflects
	// flushed-but-not-committed segments.  Keeping it separate ensures that
	// a subsequent Commit/Close sees these segments as uncommitted changes
	// and writes a new generation, which is what makes prior NRT readers
	// detectable as stale.  Update the DocumentsWriter segment-name counter
	// so future flushes do not reuse names already assigned to materialised
	// NRT segments.
	if w.documentsWriter != nil {
		w.documentsWriter.SyncSegmentNameCounter()
	}

	// Save the NRT snapshot so subsequent GetReader calls include
	// these flushed-but-not-committed segments.
	w.nrtSegmentInfos = nrtSI

	// Increment NRT generation for change detection.
	w.nrtGen.Add(1)

	reader, err := OpenDirectoryReaderWithInfos(w.directory, nrtSI)
	if err != nil {
		return nil, fmt.Errorf("GetReader: open reader: %w", err)
	}
	reader.nrtGen = w.nrtGen.Load()
	reader.writer = w
	return reader, nil
}

// GetNRTGeneration returns the current NRT generation counter. Every GetReader
// call that materialises new content advances this counter. Used by
// OpenIfChangedFromWriter for cheap change detection.
func (w *IndexWriter) GetNRTGeneration() int64 {
	return w.nrtGen.Load()
}

// hasUncommittedChanges reports whether the writer holds buffered
// documents or pending imported segments that a Commit would materialise.
// It is a conservative check used by OpenIfChangedFromWriter to decide
// whether a reopen could observe anything new.
func (w *IndexWriter) hasUncommittedChanges() bool {
	if w.docCount.Load() > 0 {
		return true
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.hasUncommittedChangesLocked()
}

// hasUncommittedChangesLocked is the unlocked variant; the caller must hold
// w.mu (either read or write lock).
func (w *IndexWriter) hasUncommittedChangesLocked() bool {
	// Any pending imported segments are uncommitted, whether they have been
	// materialised into an NRT snapshot or not.  Materialised segments are still
	// not durably committed and must be written by the next Commit/Close.
	return len(w.pendingImportedSegments) > 0 ||
		w.docCount.Load() > 0 ||
		len(w.pendingDeleteTerms) > 0 ||
		len(w.pendingCommittedDeleteTerms) > 0 ||
		len(w.pendingDeleteQueries) > 0 ||
		len(w.pendingDeletedDocIDs) > 0 ||
		len(w.pendingDVUpdates) > 0 ||
		w.pendingDeleteAll
}

// Commit commits all pending changes.
func (w *IndexWriter) Commit() error {
	return w.commitLocked(false)
}

// commitLocked is the internal commit implementation. When force is true the
// commit is written even if no mutations are pending.  IndexWriter.Commit()
// passes false; the writer's Close() calls Commit() directly so any
// pending live commit data still produces a new generation.
func (w *IndexWriter) commitLocked(force bool) error {
	if err := w.ensureOpen(); err != nil {
		return fmt.Errorf("cannot commit: %w", err)
	}

	// Serialize with concurrent add/update/delete operations and with
	// PrepareCommit, GetReader and DeleteAll to ensure the commit snapshot and
	// the application of buffered deletes happen atomically with respect to
	// concurrent index mutations.
	w.addLock.Lock()
	defer w.addLock.Unlock()
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Use the writer's in-memory SegmentInfos as the base for this commit.
	// Reliance on ReadSegmentInfos here is unsafe because a deletion policy may
	// have deleted the latest on-disk commit after the writer opened (e.g.
	// DeleteLastCommitPolicy).  Lucene's commitInternal always writes from the
	// writer's own segmentInfos, not from a re-read of the directory.
	var si *SegmentInfos
	if w.lastCommittedSegmentInfos != nil {
		si = w.lastCommittedSegmentInfos.Clone()
		si.NextGeneration()
	} else {
		var err error
		si, err = ReadSegmentInfos(w.directory)
		if err != nil {
			// No segments file yet, create new one
			si = NewSegmentInfos()
			si.SetGeneration(1)
		} else {
			si.NextGeneration()
		}
	}

	// Make sure every committed segment carries the live-docs ordinals that
	// were written in prior commits.  ReadSegmentInfos only restores delGen
	// and delCount; without the actual deleted docID set, a subsequent commit
	// would overwrite the .liv file instead of cumulatively extending it.
	for _, sci := range si.List() {
		loadLiveDocsFromDisk(w.directory, sci)
	}

	// If nothing has changed since the last commit, do not write a new
	// segments_N. This avoids generating empty commits on writer.Close and
	// keeps generation stable when no mutation occurred.  The exception is the
	// very first commit: an empty index must still materialise a segments_N so
	// that subsequent APPEND-mode writers can open it (rmp #105.2.5).
	if !force && w.hasCommitted && w.lastCommittedSegmentInfos != nil && !w.hasUncommittedChangesLocked() && !w.pendingDeleteAll {
		currentData := si.GetUserData()
		newData := w.getLiveCommitDataLocked()
		if mapsEqual(currentData, newData) {
			w.clearLiveCommitData()
			w.preparedCommit = false
			w.nrtGen.Add(1)
			return nil
		}
	}

	// Materialise all pending segments (auto-flush + AddIndexes imports).
	codec := w.config.Codec()

	// Handle pendingDeleteAll BEFORE flushing new buffered docs: mark every
	// doc in every committed segment as deleted by writing a .liv with all
	// bits cleared.  This must precede flushPendingDocsLocked so that docs
	// added after DeleteAll() are not themselves deleted.
	if w.pendingDeleteAll {
		if codec != nil {
			for _, sci := range si.List() {
				maxDoc := sci.SegmentInfo().DocCount()
				if maxDoc == 0 {
					continue
				}
				// Carry over pre-existing deletions.
				deleted := make(map[int]struct{})
				for _, ord := range sci.GetDeletedOrdinals() {
					if ord >= 0 && ord < maxDoc {
						deleted[ord] = struct{}{}
					}
				}
				// Mark every doc as deleted.
				for i := 0; i < maxDoc; i++ {
					deleted[i] = struct{}{}
				}
				if len(deleted) == len(sci.GetDeletedOrdinals()) {
					continue
				}
				live, err3 := util.NewFixedBitSet(maxDoc)
				if err3 != nil {
					return fmt.Errorf("commit: deleteAll: new bitset: %w", err3)
				}
				ords := make([]int, 0, len(deleted))
				for ord := range deleted {
					ords = append(ords, ord)
				}
				sort.Ints(ords)
				delGen := sci.AdvanceDelGen()
				segName := sci.SegmentInfo().Name()
				if _, err4 := writeLiveDocs(w.directory, segName, sci.SegmentInfo().GetID(), delGen, live); err4 != nil {
					return fmt.Errorf("commit: deleteAll: write live docs for %s: %w", segName, err4)
				}
				sci.SetDelCount(len(deleted))
				sci.SetDeletedOrdinals(ords)
				livName := liveDocsFileName(segName, delGen)
				appendSegmentFile(sci.SegmentInfo(), livName)
			}
		}
		// Discard any pending imported segments from before the DeleteAll call.
		w.pendingImportedSegments = w.pendingImportedSegments[:0]
		w.pendingDeleteAll = false
	}

	// Flush any remaining buffered documents to a pending segment so the
	// pendingImportedSegments slice is complete before we write to disk.
	if err2 := w.flushPendingDocsLocked(); err2 != nil {
		return fmt.Errorf("flush before commit failed: %w", err2)
	}

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

	// Apply query-based deletes (DeleteDocumentsQuery) against committed
	// segments. A buffered query delete must never be silently dropped
	// (rmp #13): resolving a query to its matching docs needs the postings a
	// codec writes, so a codec is mandatory when any query delete is pending.
	if len(w.pendingDeleteQueries) > 0 {
		if codec == nil {
			return fmt.Errorf("commit: cannot apply %d buffered DeleteDocumentsQuery: no codec is configured to resolve the matching documents", len(w.pendingDeleteQueries))
		}
		if err2 := w.applyQueryDeletesToCommittedSegments(si, codec, w.pendingDeleteQueries); err2 != nil {
			return fmt.Errorf("commit: apply query deletes to committed segments: %w", err2)
		}
	}
	w.pendingDeleteQueries = w.pendingDeleteQueries[:0]

	// Apply TryDeleteDocument docIDs against committed segments so that the
	// deletion survives a writer close / directory reader reopen.
	if len(w.pendingDeletedDocIDs) > 0 {
		if err2 := w.applyTryDeleteDocIDsToSegmentInfos(si); err2 != nil {
			return fmt.Errorf("commit: apply try-delete docIDs: %w", err2)
		}
		w.pendingDeletedDocIDs = w.pendingDeletedDocIDs[:0]
	}

	for i := range w.pendingImportedSegments {
		ps := &w.pendingImportedSegments[i]
		segmentName := ps.segmentName
		if segmentName == "" {
			segmentName = si.GetNextSegmentName()
		}

		// AddIndexes (directory path) imports carry their source segment files.
		// Copy them into the main directory now, before any per-format metadata
		// is rewritten, and remember the renamed file list for the SegmentInfo.
		if ps.srcDir != nil {
			copiedFiles, err2 := w.copyImportedSegmentFiles(ps, segmentName)
			if err2 != nil {
				return fmt.Errorf("commit: copy imported segment files for %s: %w", segmentName, err2)
			}
			ps.files = copiedFiles
		}

		segInfo := NewSegmentInfo(segmentName, ps.numDocs, w.directory)
		if ps.srcSegmentInfo != nil {
			// AddIndexes (directory path) imports must preserve the source segment
			// metadata, especially its 16-byte segment ID, so that copied files
			// whose headers embed that ID (e.g. .cfs/.cfe data files) remain
			// readable. This mirrors Lucene's copySegmentAsIs which reuses
			// info.info.getId() for the renamed SegmentInfo.
			if err2 := segInfo.SetID(ps.srcSegmentInfo.GetID()); err2 != nil {
				return fmt.Errorf("commit: set imported segment ID for %s: %w", segmentName, err2)
			}
			segInfo.SetVersion(ps.srcSegmentInfo.Version())
			if minVer, ok := ps.srcSegmentInfo.MinVersion(); ok {
				segInfo.SetMinVersion(minVer)
			}
			segInfo.SetHasBlocks(ps.srcSegmentInfo.HasBlocks())
			segInfo.SetCodec(ps.srcSegmentInfo.Codec())
			segInfo.SetDiagnostics(ps.srcSegmentInfo.GetDiagnostics())
			segInfo.SetAttributes(ps.srcSegmentInfo.GetAttributes())
			segInfo.SetIndexSort(ps.srcSegmentInfo.IndexSort())
		} else if len(ps.segmentID) == 16 {
			// A materialised DWPT/NRT segment already wrote files (e.g. CFS during
			// an NRT merge) with this ID in their headers. Reuse it so the final
			// .si and any existing compound files stay consistent.
			if err2 := segInfo.SetID(ps.segmentID); err2 != nil {
				return fmt.Errorf("commit: set materialised segment ID for %s: %w", segmentName, err2)
			}
			segInfo.SetVersion("10.4.0")
			segInfo.SetMinVersion("10.4.0")
		} else {
			segInfo.SetID(generateSegmentID())
			segInfo.SetVersion("10.4.0")
			segInfo.SetMinVersion("10.4.0")
		}
		sci := NewSegmentCommitInfo(segInfo, ps.delCount, -1)
		if ps.srcSegmentInfo != nil {
			sci.SetID(ps.srcSegmentInfo.GetID())
		}
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
				// Points (BKD) carry no per-field codec attributes (the BKD
				// writer stamps its own framing), so the order relative to
				// flushFieldInfos is immaterial; flushed here next to the other
				// per-field codec writers for symmetry.
				if err3 := dwpt.flushPoints(codec, writeState); err3 != nil {
					return fmt.Errorf("commit: flush points for %s: %w", segmentName, err3)
				}
				// DocValues carry no per-field codec attributes (the
				// Lucene90DocValuesFormat writes a single .dvd/.dvm with its
				// own framing, not a PerField wrapper), so the order relative
				// to flushFieldInfos is immaterial; flushed here next to the
				// other per-field codec writers for symmetry. The FieldInfo
				// doc-values type is recorded in ProcessDocument, so
				// FieldInfos.HasDocValues() reaches disk via flushFieldInfos.
				if err3 := dwpt.flushDocValues(codec, writeState); err3 != nil {
					return fmt.Errorf("commit: flush doc values for %s: %w", segmentName, err3)
				}
				// Norms carry no per-field codec attributes (the
				// Lucene90NormsFormat writes a single .nvd/.nvm with its own
				// framing, not a PerField wrapper), so the order relative to
				// flushFieldInfos is immaterial; flushed here next to the other
				// per-field codec writers for symmetry. The FieldInfo
				// "indexed with norms" bit is recorded in ProcessDocument, so
				// FieldInfos.HasNorms() reaches disk via flushFieldInfos.
				if err3 := dwpt.flushNorms(codec, writeState); err3 != nil {
					return fmt.Errorf("commit: flush norms for %s: %w", segmentName, err3)
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
		if codecFlushed {
			if w.config.UseCompoundFile() && codec.CompoundFormat() != nil {
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
				var hasCFS bool
				for _, f := range allFiles {
					if ParseSegmentName(f) != segmentName {
						continue
					}
					ext := GetExtension(f)
					switch ext {
					case "si":
						// .si is written after CFS.
					case "cfs", "cfe":
						// A compound file already exists for this segment (e.g. it was
						// materialised and packed by an earlier NRT merge). Reuse it
						// instead of creating a duplicate CFS on commit.
						hasCFS = true
					case "liv":
						// Live-docs files are never packed into a compound file; they
						// are written separately after the CFS is produced.
					default:
						segFiles = append(segFiles, f)
					}
				}
				if hasCFS {
					segInfo.SetFiles([]string{
						segmentName + ".cfs",
						segmentName + ".cfe",
						segmentName + ".si",
					})
					segInfo.SetCompoundFile(true)
				} else if len(segFiles) > 0 {
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
			} else {
				// Non-CFS path: collect all per-format files written for this segment.
				// The codec's per-format writers create files such as _0.fdt, _0.fdx,
				// _0.fnm, etc. They must be listed on the SegmentInfo so the segment
				// info writer records them in the .si footer and so the file deleter
				// treats them as referenced by the new commit.
				allFiles, err3 := w.directory.ListAll()
				if err3 != nil {
					return fmt.Errorf("commit: list directory for segment files: %w", err3)
				}
				var segFiles []string
				for _, f := range allFiles {
					if ParseSegmentName(f) != segmentName {
						continue
					}
					switch GetExtension(f) {
					case "cfs", "cfe":
						// CFS output targets are only relevant in the CFS branch above.
					default:
						segFiles = append(segFiles, f)
					}
				}
				// Ensure the .si file is listed even though it is written later in this
				// commit path; without it the file deleter would consider it unreferenced.
				segFiles = append(segFiles, segmentName+".si")
				segInfo.SetFiles(segFiles)
			}
		}

		// Persist deletions accumulated against this pending segment while it was
		// buffered or auto-flushed. This writes a real .liv file so the live-docs
		// state survives a reader reopen (rmp #4753).
		if ps.delCount > 0 && len(ps.deletedOrdinals) > 0 {
			if err3 := w.persistMergedDeletions(sci, ps.deletedOrdinals); err3 != nil {
				return fmt.Errorf("commit: persist pending segment deletions for %s: %w", segmentName, err3)
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

			segFiles := []string{segmentName + ".si"}

			if ps.srcDir != nil {
				// AddIndexes (directory path): the segment files were already copied
				// into the main directory under the new segment name. Use that
				// list directly, preserving the source's compound-file state, and
				// do not repack into a new CFS.
				segFiles = append(segFiles, ps.files...)
				if ps.srcCompoundFile {
					segInfo.SetCompoundFile(true)
				}
			}

			// Persist the FieldInfos to a real .fnm so they survive a reader
			// reopen without the removed _gocene_fi_ userData key (rmp #4785).
			// This covers AddIndexes-imported segments (no DWPT flush) that
			// carry FieldInfos only in memory; the .fnm is the authoritative
			// on-disk source that openSegmentReader reads back.
			if ps.fieldInfos != nil && ps.fieldInfos.Size() > 0 && codec != nil {
				if fif := codec.FieldInfosFormat(); fif != nil {
					if err3 := fif.Write(w.directory, segInfo, "", ps.fieldInfos, store.IOContextWrite); err3 != nil {
						return fmt.Errorf("commit: write field infos for %s: %w", segmentName, err3)
					}
					// Ensure the .fnm is referenced even if the source did not
					// include it (overwriting a copied .fnm is fine because the
					// name is unchanged).
					fnm := segmentName + ".fnm"
					found := false
					for _, f := range segFiles {
						if f == fnm {
							found = true
							break
						}
					}
					if !found {
						segFiles = append(segFiles, fnm)
					}
					// Stamp the codec name so the reopen path
					// (openSegmentReader) resolves a codec and reads the .fnm
					// back as the authoritative FieldInfos source (rmp #4785).
					segInfo.SetCodec(codec.Name())
				}
			}
			segInfo.SetFiles(segFiles)
		}

		if len(ps.deletedOrdinals) > 0 {
			// Persist the deletions carried through this merge/AddIndexes as a
			// real Lucene90 .liv file and bump the segment's delGen/delCount so
			// the reopen path (loadLiveDocsFromDisk, which reads .liv when
			// delGen >= 0) recovers the exact live set. This replaces the legacy
			// _gocene_del_ segments_N userData round-trip (rmp #4789): the
			// byte-faithful .liv is now the authoritative on-disk source of
			// deletions for merged segments, just as Lucene's IndexWriter merge
			// path writes live docs via Lucene90LiveDocsFormat.
			if err3 := w.persistMergedDeletions(sci, ps.deletedOrdinals); err3 != nil {
				return fmt.Errorf("commit: persist merged deletions for %s: %w", segmentName, err3)
			}
		}
		// Stamp the configured index sort onto the segment so writeSegmentInfo
		// serialises it into the .si numSortFields block (rmp #4789), replacing
		// the segments_N _gocene_sort_* userData keys.
		sci.SegmentInfo().SetIndexSort(w.config.IndexSort())
		// Write the .si file for this segment before registering it so that
		// external tools and CheckIndex can verify per-segment integrity.
		// If an NRT merge or an earlier partial commit already wrote a .si for
		// this segment name, remove it so the final commit can write the definitive
		// version with the correct file list.
		siName := segmentName + ".si"
		if _, statErr := w.directory.FileLength(siName); statErr == nil {
			_ = w.directory.DeleteFile(siName)
		}
		if err3 := writeSegmentInfo(w.directory, sci.SegmentInfo(), store.IOContextWrite, w.config.Codec()); err3 != nil {
			return fmt.Errorf("writing .si: %w", err3)
		}
		si.Add(sci)
		w.committedSegments = append(w.committedSegments, sci)
	}
	w.pendingImportedSegments = w.pendingImportedSegments[:0]

	// Apply buffered doc-values updates to committed segments. This writes a new
	// generation of .dvd/.dvm files and a new .fnm for each affected segment,
	// mirroring Lucene's ReadersAndUpdates.writeFieldUpdates.
	if err := w.applyDocValuesUpdatesLocked(si.List()); err != nil {
		return err
	}
	// Keep the writer's in-memory segment list in sync with the committed state
	// (SegmentCommitInfo objects for freshly imported segments are shared; older
	// segments are clones and may have been advanced to new DV/FieldInfos gens).
	w.committedSegments = si.List()

	// Ensure the segment-name counter is past every segment that now exists,
	// so later merges and flushes never reuse a name already on disk.
	// UpdateCounterFromSegments parses segment names numerically (unlike the
	// lexicographic GetMaxSegmentName), avoiding collisions when segment names
	// cross a power of ten (e.g. _99, _100, _101).
	si.UpdateCounterFromSegments()

	// Add commit data if present
	if w.liveCommitData != nil && len(w.liveCommitData.data) > 0 {
		si.SetUserData(w.liveCommitData.data)
	}

	// Record parentField and indexSort for AddIndexes validation.
	si.SetInMemoryParentField(w.config.ParentField())
	si.SetInMemoryIndexSort(w.config.IndexSort())

	if err := WriteSegmentInfos(si, w.directory); err != nil {
		return fmt.Errorf("failed to write segment infos: %w", err)
	}

	// Hand the new commit to the file deleter: it reference-counts the files,
	// invokes the deletion policy, and removes any unreferenced segment files
	// from prior commits.
	if w.deleter != nil {
		if err := w.deleter.Checkpoint(si, true); err != nil {
			return fmt.Errorf("deleter checkpoint: %w", err)
		}
	}

	// Remember the committed state so Rollback can restore it later.
	w.lastCommittedSegmentInfos = si.Clone()
	w.hasCommitted = true

	// A Commit advances the on-disk generation.  Any NRT reader opened before
	// this point must now report IsCurrent == false, so bump the NRT generation
	// even though no new in-memory segments were flushed.
	w.nrtGen.Add(1)

	// The on-disk SegmentInfos now reflects the committed state.  Future NRT
	// readers must read from disk rather than reuse a stale in-memory snapshot
	// that may reference pre-commit segment names/generations (rmp #105.2.7).
	w.nrtSegmentInfos = nil

	// The DocumentsWriter's segment-name counter must stay ahead of any
	// segment names that now exist on disk (e.g. from a previous merge or
	// addIndexes) so the next flush does not reuse an existing name.
	if w.documentsWriter != nil {
		w.documentsWriter.SyncSegmentNameCounter()
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

// applyQueryDeletesToCommittedSegments executes each buffered query against the
// committed segments (via the registered QueryDeleteExecutor hook) and persists
// the matching docIDs as .liv deletions, exactly as term-based deletes do.
func (w *IndexWriter) applyQueryDeletesToCommittedSegments(si *SegmentInfos, codec Codec, queries []interface{}) error {
	exec := lookupQueryDeleteExecutor()
	if exec == nil {
		// A query delete is pending but nothing can resolve it. Surface an
		// explicit error rather than silently dropping the delete (rmp #13):
		// the default executor is installed by importing the search package.
		return fmt.Errorf("no QueryDeleteExecutor is registered: import the search package to enable DeleteDocumentsQuery, or install one via index.RegisterQueryDeleteExecutor")
	}

	// Execute all pending queries in one batch.
	results, err := exec(w.directory, si, queries)
	if err != nil {
		return fmt.Errorf("query-delete executor: %w", err)
	}

	// Apply results per segment — same .liv write path as term-based deletes.
	for _, sci := range si.List() {
		name := sci.SegmentInfo().Name()
		docIDs, ok := results[name]
		if !ok || len(docIDs) == 0 {
			continue
		}

		maxDoc := sci.SegmentInfo().DocCount()
		if maxDoc == 0 {
			continue
		}

		deleted := make(map[int]struct{})
		for _, ord := range sci.GetDeletedOrdinals() {
			if ord >= 0 && ord < maxDoc {
				deleted[ord] = struct{}{}
			}
		}
		prevCount := len(deleted)
		for _, id := range docIDs {
			if id >= 0 && id < maxDoc {
				deleted[id] = struct{}{}
			}
		}
		if len(deleted) == prevCount {
			continue // nothing new
		}

		live, err := util.NewFixedBitSet(maxDoc)
		if err != nil {
			return fmt.Errorf("new fixed bit set for %s: %w", name, err)
		}
		for i := 0; i < maxDoc; i++ {
			live.Set(i)
		}
		var ords []int
		for ord := range deleted {
			live.Clear(ord)
			ords = append(ords, ord)
		}
		sort.Ints(ords)

		delGen := sci.AdvanceDelGen()
		onDiskDel, err := writeLiveDocs(w.directory, name, sci.SegmentInfo().GetID(), delGen, live)
		if err != nil {
			return fmt.Errorf("write live docs for %s: %w", name, err)
		}
		if onDiskDel != len(deleted) {
			return fmt.Errorf("live docs for %s: wrote delCount=%d, expected %d", name, onDiskDel, len(deleted))
		}
		sci.SetDelCount(len(deleted))
		sci.SetDeletedOrdinals(ords)
		livName := liveDocsFileName(name, delGen)
		appendSegmentFile(sci.SegmentInfo(), livName)
	}
	return nil
}

// persistMergedDeletions writes the deletions carried through a merge /
// ForceMerge / AddIndexes into a byte-faithful Lucene90 .liv file, advances the
// segment's deletion generation, records the delCount and deleted ordinals, and
// registers the .liv in the segment's file list (rmp #4789).
//
// This replaces the legacy _gocene_del_ segments_N userData round-trip: a
// merged segment that inherits deletions now carries them in a real on-disk
// .liv, exactly as Lucene's IndexWriter merge path persists merged live docs via
// Lucene90LiveDocsFormat. The reopen path (loadLiveDocsFromDisk) reads the .liv
// back when delGen >= 0, so NumDocs reflects the deletions after a reader
// reopen without any Gocene-private userData key.
//
// deletedOrdinals is the set of 0-based document ordinals that are deleted in
// this segment's doc space; out-of-range ordinals are ignored defensively.
func (w *IndexWriter) persistMergedDeletions(sci *SegmentCommitInfo, deletedOrdinals []int) error {
	segInfo := sci.SegmentInfo()
	maxDoc := segInfo.DocCount()
	if maxDoc <= 0 {
		return nil
	}

	// Build the live-docs bitset: every doc live, then clear the deleted ords.
	live, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return err
	}
	for i := 0; i < maxDoc; i++ {
		live.Set(i)
	}
	ords := make([]int, 0, len(deletedOrdinals))
	for _, ord := range deletedOrdinals {
		if ord < 0 || ord >= maxDoc || !live.Get(ord) {
			continue
		}
		live.Clear(ord)
		ords = append(ords, ord)
	}
	if len(ords) == 0 {
		return nil
	}
	sort.Ints(ords)

	delGen := sci.AdvanceDelGen()
	segName := segInfo.Name()
	onDiskDel, err := writeLiveDocs(w.directory, segName, segInfo.GetID(), delGen, live)
	if err != nil {
		return fmt.Errorf("write live docs for %s: %w", segName, err)
	}
	if onDiskDel != len(ords) {
		return fmt.Errorf("live docs for %s: wrote delCount=%d, expected %d", segName, onDiskDel, len(ords))
	}

	sci.SetDelCount(len(ords))
	sci.SetDeletedOrdinals(ords)

	// Register the .liv in the segment's file list so deleters/CheckIndex see it.
	appendSegmentFile(segInfo, liveDocsFileName(segName, delGen))
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

// tempSegmentCommitInfoForPending builds a throw-away SegmentCommitInfo for an
// in-memory pending segment so that openSegmentReader can resolve terms via the
// merged in-memory postings. The SegmentInfo is given a synthetic name because
// no real files exist yet; the name is only used for debug / live-docs lookup,
// which is skipped when delGen is -1.
func (w *IndexWriter) tempSegmentCommitInfoForPending(ps *pendingSegment, name string) *SegmentCommitInfo {
	segInfo := NewSegmentInfo(name, ps.numDocs, w.directory)
	sci := NewSegmentCommitInfo(segInfo, ps.delCount, -1)
	if ps.fieldInfos != nil {
		sci.SetInMemoryFieldInfos(ps.fieldInfos)
	}
	if ps.inMemoryFields != nil {
		sci.SetInMemoryFields(ps.inMemoryFields)
	}
	return sci
}

// applyTermDeletesToPendingSegments resolves the supplied terms against the
// in-memory postings of every pending segment that already exists when the
// delete is issued, and records the matching docIDs in the pending segment's
// deletedOrdinals. This mirrors Lucene's delete-on-flush behaviour: a delete
// only reaches segments that existed before the delete, so docs added to a
// later segment are not self-deleted. Must be called with w.mu held.
func (w *IndexWriter) applyTermDeletesToPendingSegments(terms []*Term) error {
	if len(terms) == 0 {
		return nil
	}
	for i := range w.pendingImportedSegments {
		ps := &w.pendingImportedSegments[i]
		if ps.numDocs <= 0 || ps.inMemoryFields == nil {
			continue
		}
		name := ps.segmentName
		if name == "" {
			name = fmt.Sprintf("_pending_term_%d", i)
		}
		sci := w.tempSegmentCommitInfoForPending(ps, name)
		sr, err := openSegmentReader(w.directory, sci)
		if err != nil {
			return fmt.Errorf("apply term deletes to pending segment %s: %w", name, err)
		}
		deleted := make(map[int]struct{}, ps.numDocs)
		for _, ord := range ps.deletedOrdinals {
			deleted[ord] = struct{}{}
		}
		if err := w.collectDeletedDocIDs(sr, terms, deleted); err != nil {
			_ = sr.Close()
			return fmt.Errorf("collect deleted doc IDs for pending segment %s: %w", name, err)
		}
		_ = sr.Close()
		if len(deleted) == ps.delCount {
			continue
		}
		ords := make([]int, 0, len(deleted))
		for ord := range deleted {
			ords = append(ords, ord)
		}
		sort.Ints(ords)
		ps.deletedOrdinals = ords
		ps.delCount = len(ords)
	}
	return nil
}

// applyQueryDeletesToPendingSegments executes the supplied queries against
// the existing in-memory pending segments and marks matching docIDs as deleted.
// Must be called with w.mu held.
func (w *IndexWriter) applyQueryDeletesToPendingSegments(queries []interface{}) error {
	if len(queries) == 0 {
		return nil
	}
	exec := lookupQueryDeleteExecutor()
	if exec == nil {
		return nil
	}
	si := NewSegmentInfos()
	pendingIdx := make([]int, 0, len(w.pendingImportedSegments))
	for i := range w.pendingImportedSegments {
		ps := &w.pendingImportedSegments[i]
		if ps.numDocs <= 0 || ps.inMemoryFields == nil {
			continue
		}
		name := ps.segmentName
		if name == "" {
			name = fmt.Sprintf("_pending_query_%d", i)
		}
		sci := w.tempSegmentCommitInfoForPending(ps, name)
		si.Add(sci)
		pendingIdx = append(pendingIdx, i)
	}
	if si.Size() == 0 {
		return nil
	}
	results, err := exec(w.directory, si, queries)
	if err != nil {
		return fmt.Errorf("apply query deletes to pending segments: %w", err)
	}
	for _, idx := range pendingIdx {
		ps := &w.pendingImportedSegments[idx]
		name := ps.segmentName
		if name == "" {
			name = fmt.Sprintf("_pending_query_%d", idx)
		}
		docIDs, ok := results[name]
		if !ok || len(docIDs) == 0 {
			continue
		}
		deleted := make(map[int]struct{}, ps.numDocs)
		for _, ord := range ps.deletedOrdinals {
			deleted[ord] = struct{}{}
		}
		for _, id := range docIDs {
			if id >= 0 && id < ps.numDocs {
				deleted[id] = struct{}{}
			}
		}
		if len(deleted) == ps.delCount {
			continue
		}
		ords := make([]int, 0, len(deleted))
		for ord := range deleted {
			ords = append(ords, ord)
		}
		sort.Ints(ords)
		ps.deletedOrdinals = ords
		ps.delCount = len(ords)
	}
	return nil
}

// applyBufferedDeletesToSegmentCommitInfos merges all buffered committed
// deletions (DeleteAll, term deletes, query deletes, and TryDeleteDocument
// docIDs) into the supplied SegmentInfos without writing any .liv files.  This
// is used by GetReader to build an NRT snapshot whose live-docs state reflects
// deletes issued since the last commit.  Must be called with w.mu held.
func (w *IndexWriter) applyBufferedDeletesToSegmentCommitInfos(si *SegmentInfos) error {
	if si == nil {
		return nil
	}
	segments := si.List()
	if len(segments) == 0 {
		return nil
	}

	// Resolve query deletes once for the whole snapshot.
	var queryResults map[string][]int
	if len(w.pendingDeleteQueries) > 0 {
		exec := lookupQueryDeleteExecutor()
		if exec == nil {
			return fmt.Errorf("GetReader: no QueryDeleteExecutor registered for %d pending query deletes", len(w.pendingDeleteQueries))
		}
		results, err := exec(w.directory, si, w.pendingDeleteQueries)
		if err != nil {
			return fmt.Errorf("GetReader: execute query deletes: %w", err)
		}
		queryResults = results
	}

	for _, sci := range segments {
		maxDoc := sci.SegmentInfo().DocCount()
		if maxDoc == 0 {
			continue
		}

		deleted := make(map[int]struct{}, maxDoc)
		for _, ord := range sci.GetDeletedOrdinals() {
			if ord >= 0 && ord < maxDoc {
				deleted[ord] = struct{}{}
			}
		}

		// DeleteAll marks every document in every committed segment as deleted.
		if w.pendingDeleteAll {
			for i := 0; i < maxDoc; i++ {
				deleted[i] = struct{}{}
			}
		}

		// Term-based deletes.
		if len(w.pendingCommittedDeleteTerms) > 0 {
			sr, err := openSegmentReader(w.directory, sci)
			if err != nil {
				return fmt.Errorf("GetReader: open segment %s: %w", sci.SegmentInfo().Name(), err)
			}
			err = w.collectDeletedDocIDs(sr, w.pendingCommittedDeleteTerms, deleted)
			_ = sr.Close()
			if err != nil {
				return err
			}
		}

		// Query-based deletes.
		if queryResults != nil {
			if docIDs, ok := queryResults[sci.SegmentInfo().Name()]; ok {
				for _, id := range docIDs {
					if id >= 0 && id < maxDoc {
						deleted[id] = struct{}{}
					}
				}
			}
		}

		if len(deleted) == 0 {
			continue
		}

		ords := make([]int, 0, len(deleted))
		for ord := range deleted {
			ords = append(ords, ord)
		}
		sort.Ints(ords)
		sci.SetDelCount(len(ords))
		sci.SetDeletedOrdinals(ords)
	}

	// Drop fully-deleted segments unless the merge policy wants to keep them.
	// This mirrors Lucene's IndexWriter behavior when building an NRT snapshot.
	mp := w.config.GetMergePolicy()
	pruned := make([]*SegmentCommitInfo, 0, len(segments))
	for _, sci := range segments {
		maxDoc := sci.SegmentInfo().DocCount()
		if maxDoc > 0 && sci.DelCount() >= maxDoc {
			if mp != nil && mp.KeepFullyDeletedSegment(sci) {
				pruned = append(pruned, sci)
			}
			continue
		}
		pruned = append(pruned, sci)
	}
	if len(pruned) != len(segments) {
		si.Clear()
		for _, sci := range pruned {
			si.Add(sci)
		}
	}

	// TryDeleteDocument global docIDs.
	if len(w.pendingDeletedDocIDs) > 0 {
		for _, docID := range w.pendingDeletedDocIDs {
			sci, ord, ok := mapGlobalDocIDToSegment(segments, docID)
			if !ok {
				continue
			}
			maxDoc := sci.SegmentInfo().DocCount()
			ords := sci.GetDeletedOrdinals()
			exists := false
			for _, o := range ords {
				if o == ord {
					exists = true
					break
				}
			}
			if exists {
				continue
			}
			ords = append(ords, ord)
			sort.Ints(ords)
			sci.SetDelCount(len(ords))
			sci.SetDeletedOrdinals(ords)
			_ = maxDoc
		}
	}

	return nil
}

// mapGlobalDocIDToSegment maps an IndexReader-level docID to the segment that
// contains it and the in-segment ordinal, using the segment order in the
// supplied SegmentInfos.  It mirrors the leaf ordering used by CompositeReader.
func mapGlobalDocIDToSegment(segments []*SegmentCommitInfo, docID int) (*SegmentCommitInfo, int, bool) {
	offset := 0
	for _, sci := range segments {
		maxDoc := sci.SegmentInfo().DocCount()
		if docID < offset+maxDoc {
			return sci, docID - offset, true
		}
		offset += maxDoc
	}
	return nil, -1, false
}

// applyTryDeleteDocIDsToSegmentInfos marks the in-segment ordinals corresponding
// to each buffered TryDeleteDocument global docID as deleted and persists the
// resulting live-docs bitmap as a byte-faithful .liv file, just like term- and
// query-based deletes.
func (w *IndexWriter) applyTryDeleteDocIDsToSegmentInfos(si *SegmentInfos) error {
	if si == nil || len(w.pendingDeletedDocIDs) == 0 {
		return nil
	}
	segments := si.List()
	for _, docID := range w.pendingDeletedDocIDs {
		sci, ord, ok := mapGlobalDocIDToSegment(segments, docID)
		if !ok {
			continue
		}
		maxDoc := sci.SegmentInfo().DocCount()
		if ord < 0 || ord >= maxDoc {
			continue
		}
		ords := sci.GetDeletedOrdinals()
		exists := false
		for _, o := range ords {
			if o == ord {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		ords = append(ords, ord)
		sort.Ints(ords)
		sci.SetDelCount(len(ords))
		sci.SetDeletedOrdinals(ords)

		live, err := util.NewFixedBitSet(maxDoc)
		if err != nil {
			return fmt.Errorf("try-delete live bitset for %s: %w", sci.SegmentInfo().Name(), err)
		}
		for i := 0; i < maxDoc; i++ {
			live.Set(i)
		}
		for _, o := range ords {
			live.Clear(o)
		}
		delGen := sci.AdvanceDelGen()
		segName := sci.SegmentInfo().Name()
		if _, err := writeLiveDocs(w.directory, segName, sci.SegmentInfo().GetID(), delGen, live); err != nil {
			return fmt.Errorf("try-delete write live docs for %s: %w", segName, err)
		}
		appendSegmentFile(sci.SegmentInfo(), liveDocsFileName(segName, delGen))
	}
	return nil
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

	// Try to commit changes before closing.
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

	// Run natural merges selected by the merge policy before closing. Lucene
	// calls maybeMerge during close so that a merge-on-close policy (e.g.
	// LogDocMergePolicy.setMinMergeDocs(1)) can collapse small/deleted segments.
	if err := w.maybeMerge(CLOSING); err != nil {
		if s := w.config.GetMergeScheduler(); s != nil {
			_ = s.Close()
		}
		w.closed.Store(true)
		return fmt.Errorf("maybeMerge during close: %w", err)
	}

	// Set closed atomically
	w.closed.Store(true)

	// Close the merge scheduler
	if s := w.config.GetMergeScheduler(); s != nil {
		if err := s.Close(); err != nil {
			return err
		}
	}

	// Close the file deleter, releasing reference counts.
	if w.deleter != nil {
		if err := w.deleter.Close(); err != nil {
			return fmt.Errorf("deleter close: %w", err)
		}
		w.deleter = nil
	}

	return nil
}

// mapsEqual reports whether two string maps have identical key/value pairs.
// A nil map and an empty map are considered equal.
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// filesReferencedBySegmentInfos returns the set of files that belong to the
// given SegmentInfos, including the segments_N file and every segment file
// (core files, live-docs, field-infos generations, doc-values updates).
func filesReferencedBySegmentInfos(si *SegmentInfos) map[string]struct{} {
	files := make(map[string]struct{})
	if si == nil {
		return files
	}
	files[si.GetFileName()] = struct{}{}
	for _, sci := range si.List() {
		for _, f := range sci.GetFiles() {
			files[f] = struct{}{}
		}
	}
	return files
}

// NumDocs returns the number of live documents in the index.
// Deleted and soft-deleted documents are excluded; buffered (uncommitted)
// deletes are counted.
func (w *IndexWriter) NumDocs() int {
	var si *SegmentInfos
	if w.nrtSegmentInfos != nil {
		// An NRT snapshot includes committed segments plus materialised in-memory
		// segments; it is the most up-to-date view of the index while the writer
		// is open.
		si = w.nrtSegmentInfos
	} else if w.pinnedSegmentInfos != nil {
		si = w.pinnedSegmentInfos
	} else {
		var err error
		si, err = ReadSegmentInfos(w.directory)
		if err != nil {
			si = nil
		}
	}
	committedLive := 0
	if si != nil {
		committedLive = si.TotalNumDocs()
	}
	// Add live docs from pending imported segments that are not yet part of the
	// NRT snapshot (net of hard+soft deletes).
	w.mu.RLock()
	pendingCommittedDeletes := w.pendingCommittedDeleteCount
	for _, ps := range w.pendingImportedSegments {
		if ps.materialized {
			continue
		}
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
	var si *SegmentInfos
	if w.nrtSegmentInfos != nil {
		si = w.nrtSegmentInfos
	} else if w.pinnedSegmentInfos != nil {
		si = w.pinnedSegmentInfos
	} else {
		var err error
		si, err = ReadSegmentInfos(w.directory)
		if err != nil {
			si = nil
		}
	}
	committedTotal := 0
	if si != nil {
		committedTotal = si.TotalDocCount()
	}
	// Add documents in pending imported segments that are not yet materialised
	// into the NRT snapshot.
	w.mu.RLock()
	for _, ps := range w.pendingImportedSegments {
		if ps.materialized {
			continue
		}
		committedTotal += ps.numDocs
	}
	w.mu.RUnlock()
	return committedTotal + int(w.docCount.Load())
}

// maxDocForLimit returns the document count used for MaxDocs enforcement.
// After DeleteAll the old committed segments are pending deletion, so they do
// not count toward the limit; otherwise the total including committed segments
// is used, matching Lucene's IndexWriter.reserveDocs semantics.
func (w *IndexWriter) maxDocForLimit() int {
	w.mu.RLock()
	pendingDeleteAll := w.pendingDeleteAll
	w.mu.RUnlock()
	if pendingDeleteAll {
		return int(w.docCount.Load())
	}
	var committedTotal int
	if w.nrtSegmentInfos != nil {
		committedTotal = w.nrtSegmentInfos.TotalDocCount()
	} else {
		si, err := ReadSegmentInfos(w.directory)
		if err == nil {
			committedTotal = si.TotalDocCount()
		}
	}
	w.mu.RLock()
	for _, ps := range w.pendingImportedSegments {
		if ps.materialized {
			continue
		}
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

// fieldDocValuesTypeLocked returns the DocValuesType recorded for the named
// field across the writer's pending and committed FieldInfos.  Returns
// DocValuesTypeNone when the field is not yet known to the writer.  Must be
// called with w.mu held.
func (w *IndexWriter) fieldDocValuesTypeLocked(fieldName string) DocValuesType {
	if w.pendingFieldInfos != nil {
		if fi := w.pendingFieldInfos.GetByName(fieldName); fi != nil {
			return fi.DocValuesType()
		}
	}
	for _, sci := range w.committedSegments {
		fi := sci.GetInMemoryFieldInfos()
		if fi == nil {
			continue
		}
		if f := fi.GetByName(fieldName); f != nil {
			return f.DocValuesType()
		}
	}
	for _, ps := range w.pendingImportedSegments {
		if ps.fieldInfos == nil {
			continue
		}
		if f := ps.fieldInfos.GetByName(fieldName); f != nil {
			return f.DocValuesType()
		}
	}
	return DocValuesTypeNone
}

// IsClosed returns true if the writer is closed.
// Uses atomic operations for lock-free check.
func (w *IndexWriter) IsClosed() bool {
	return w.closed.Load() || w.tragicError.Load() != nil
}

// IsDeleterClosed returns true if the writer's IndexFileDeleter has been
// released. This is exposed primarily for tests that verify the abort path
// after an add-document failure.
func (w *IndexWriter) IsDeleterClosed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.deleter == nil
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

	// Serialize with concurrent add/update/delete operations and with Commit,
	// PrepareCommit and GetReader.  A DeleteAll must observe (and discard) the
	// exact set of buffered docs and pending deletes that exist at its call time,
	// without an interleaved commit rewriting the on-disk SegmentInfos underneath
	// us.
	w.addLock.Lock()
	defer w.addLock.Unlock()
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Clear all pending document state so that a subsequent Commit
	// does not include stale entries from before the DeleteAll call.
	w.docCount.Store(0)
	w.pendingDeleteTerms = nil
	w.pendingSoftDeletedOrdinals = nil
	w.pendingDeletedDocIDs = nil
	w.pendingDeleteQueries = nil
	w.pendingCommittedDeleteTerms = nil
	w.pendingCommittedDeleteCount = 0
	w.pendingImportedSegments = nil
	w.docFieldIndex = nil
	w.pendingFieldInfos = nil
	w.committedSegments = nil

	// Discard the DWPT's buffered documents (the DWPT may have docs that
	// were added before DeleteAll was called).  TakePerThreadPool drains
	// and resets the DWPT pool.
	if w.documentsWriter != nil {
		w.documentsWriter.mu.Lock()
		_ = w.documentsWriter.TakePerThreadPool()
		w.documentsWriter.mu.Unlock()
	}

	// Mark that all committed segments should be deleted at the next Commit.
	w.pendingDeleteAll = true

	return nil
}

// Rollback rolls back all changes made since the writer opened.
// This closes the writer and returns the index to the SegmentInfos baseline
// that existed when this writer was created (the pinned commit, the latest
// on-disk commit, or an empty index).
//
// Any segments flushed or merged since the writer opened are removed, buffered
// documents are discarded, and the directory is restored to the rollback
// baseline. Pre-existing commits (e.g. kept by a keep-all deletion policy) are
// preserved.
func (w *IndexWriter) Rollback() error {
	// Fast path: check if already closed using atomic
	if w.closed.Load() || w.tragicError.Load() != nil {
		return nil
	}

	// Serialize with Commit/PrepareCommit/GetReader/DeleteAll so we do not
	// rollback while another thread is writing a new commit.
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Close the merge scheduler without committing.
	if s := w.config.GetMergeScheduler(); s != nil {
		_ = s.Close()
	}

	// Discard all pending state that was never committed.
	w.docCount.Store(0)
	w.pendingDeleteTerms = nil
	w.pendingSoftDeletedOrdinals = nil
	w.pendingDeletedDocIDs = nil
	w.pendingDeleteQueries = nil
	w.pendingCommittedDeleteTerms = nil
	w.pendingCommittedDeleteCount = 0
	w.pendingImportedSegments = nil
	w.docFieldIndex = nil
	w.pendingFieldInfos = nil
	w.pendingDeleteAll = false
	w.preparedCommit = false
	w.clearLiveCommitData()
	w.nrtSegmentInfos = nil

	if w.documentsWriter != nil {
		w.documentsWriter.mu.Lock()
		_ = w.documentsWriter.TakePerThreadPool()
		w.documentsWriter.mu.Unlock()
	}

	// Determine the set of files that must survive the rollback. Use the
	// rollback baseline captured at writer construction, which is the pinned
	// commit (or latest on-disk commit) rather than the writer's own last
	// committed state.
	var baseline *SegmentInfos
	if w.rollbackSegmentInfos != nil {
		baseline = w.rollbackSegmentInfos.Clone()
	} else if w.lastCommittedSegmentInfos != nil {
		baseline = w.lastCommittedSegmentInfos.Clone()
	}

	var keep map[string]struct{}
	if baseline != nil {
		restored := baseline.Clone()
		// If the directory has moved past the baseline (e.g. this writer committed
		// new segments, or a newer commit exists because the writer opened on a
		// pinned prior commit), write a fresh segments_N beyond the newest on-disk
		// generation so the baseline becomes the live commit again. Otherwise the
		// rollback is pure cleanup: delete uncommitted files and leave the existing
		// latest commit untouched.
		baselineGen := restored.Generation()
		latestOnDiskGen := baselineGen
		if current, err := ReadSegmentInfos(w.directory); err == nil {
			if current.Generation() > latestOnDiskGen {
				latestOnDiskGen = current.Generation()
			}
		}
		if latestOnDiskGen > baselineGen {
			restored.SetGeneration(latestOnDiskGen + 1)
			restored.SetLastGeneration(restored.Generation())
			if err := WriteSegmentInfos(restored, w.directory); err != nil {
				return fmt.Errorf("rollback: write restored segment infos: %w", err)
			}
		}
		w.lastCommittedSegmentInfos = restored.Clone()
		w.pinnedSegmentInfos = restored.Clone()
		w.committedSegments = restored.List()
		keep = filesReferencedBySegmentInfos(restored)
	} else {
		w.lastCommittedSegmentInfos = nil
		w.pinnedSegmentInfos = nil
		w.committedSegments = nil
		keep = make(map[string]struct{})
	}

	// Preserve pre-existing files (e.g. commits kept by a keep-all deletion
	// policy) and the write lock itself.
	for f := range w.startingFiles {
		keep[f] = struct{}{}
	}
	keep[writeLockName] = struct{}{}

	allFiles, err := w.directory.ListAll()
	if err != nil {
		return fmt.Errorf("rollback: list directory: %w", err)
	}
	for _, f := range allFiles {
		if _, ok := keep[f]; !ok {
			_ = w.directory.DeleteFile(f)
		}
	}

	// Release the file deleter's reference counts. Any files that are no longer
	// referenced by the baseline (or by pre-existing commits) are now gone from
	// the directory, so closing the deleter just cleans up internal state.
	if w.deleter != nil {
		_ = w.deleter.Close()
		w.deleter = nil
	}

	// Release the write lock and mark the writer closed.
	if w.writeLock != nil {
		_ = w.writeLock.Close()
		w.writeLock = nil
	}
	w.closed.Store(true)
	return nil
}

// ForceMerge forces merge policy to merge segments until there are
// at most maxNumSegments segments.
//
// Documents are committed first so in-memory state is materialized on disk,
// then merges are run in rounds. Each round calls the merge policy's
// FindForcedMerges and executes the returned specification. This repeats
// until the segment count reaches maxNumSegments or the policy has no more
// merges to offer (e.g. remaining segments are too large per size/doc caps).
//
// Without a configured merge policy, all segments are merged into one.
func (w *IndexWriter) ForceMerge(maxNumSegments int) (err error) {
	if err := w.ensureOpen(); err != nil {
		return err
	}
	if maxNumSegments < 1 {
		maxNumSegments = 1
	}

	// Commit first so every buffered/pending document is materialised into a
	// real committed segment WITH its data (stored fields, postings, doc
	// values, points, vectors, term vectors). The merge then operates on those
	// on-disk segments through SegmentMerger (rmp #14/#114), rather than the
	// old metadata-only collapse that dropped all segment data.
	if err := w.Commit(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	defer func() {
		// After any successful merge operation the on-disk SegmentInfos may have
		// changed.  Invalidate the cached NRT snapshot so subsequent GetReader
		// calls observe the merged state rather than a stale pre-merge snapshot,
		// and resync the DocumentsWriter's segment-name counter so new flushes
		// do not collide with segments produced by the merge.
		if err == nil {
			w.nrtSegmentInfos = nil
			if w.documentsWriter != nil {
				w.documentsWriter.SyncSegmentNameCounter()
			}
		}
	}()

	mp := w.config.GetMergePolicy()

	for {
		si, err := ReadSegmentInfos(w.directory)
		if err != nil {
			return nil // no committed index yet — nothing to merge
		}
		if si.Size() <= maxNumSegments {
			return nil // already at or below the requested segment count
		}

		if mp != nil {
			segsToMerge := make(map[*SegmentCommitInfo]bool, si.Size())
			for _, sci := range si.List() {
				segsToMerge[sci] = true
			}
			spec, err := mp.FindForcedMerges(si, maxNumSegments, segsToMerge, &forceMergeContext{})
			if err != nil {
				return fmt.Errorf("forceMerge: find forced merges: %w", err)
			}
			if spec == nil || spec.Size() == 0 {
				// The policy declined to merge (e.g. NoMergePolicy, or it considers
				// the index already force-merged); respect that decision.
				return nil
			}
			if err := w.executeForcedMerges(si, spec); err != nil {
				return err
			}
			// Loop back — the merge policy may need multiple rounds to converge
			// (e.g. LogMergePolicy with mergeFactor=5 can only reduce the segment
			// count by roughly 80 % per round).
		} else {
			// No merge policy configured: merge everything into a single segment
			// (1 <= maxNumSegments always satisfies the target).
			return w.forceMergeToOneSegment(si)
		}
	}
}

// ForceMergeDeletes forces merging of all segments that have deleted documents.
// The merge policy determines which segments to merge (e.g. TieredMergePolicy
// only picks segments where the deleted-doc percentage exceeds a threshold).
//
// This is a potentially costly operation; it is rarely warranted.
func (w *IndexWriter) ForceMergeDeletes() error {
	return w.forceMergeDeletesWait(true)
}

// forceMergeDeletesWait performs forceMergeDeletes. The doWait parameter is
// accepted for API compatibility with Lucene's overload; in this synchronous
// implementation merges always complete before the call returns.
func (w *IndexWriter) forceMergeDeletesWait(doWait bool) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}
	if err := w.Commit(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	mp := w.config.GetMergePolicy()
	if mp == nil {
		return nil
	}

	for {
		si, err := ReadSegmentInfos(w.directory)
		if err != nil {
			return nil
		}

		ctx := &forceMergeContext{}
		spec, err := mp.FindForcedDeletesMerges(si, ctx)
		if err != nil {
			return fmt.Errorf("forceMergeDeletes: find forced deletes merges: %w", err)
		}
		if spec == nil || spec.Size() == 0 {
			return nil // no segments with deletions to merge
		}
		if err := w.executeForcedMerges(si, spec); err != nil {
			return err
		}
		// Loop back in case a new round of merges is needed after old segments
		// (with deletions) were merged away and new segments with deletions
		// appear (e.g. a large segment with deletions was untouched because
		// it exceeded the size cap).
	}
}

// naturalMergeContext is a minimal MergeContext for natural merges chosen by
// the configured MergePolicy. Deletion counts come from the SegmentCommitInfos
// after pending deletes have been applied.
type naturalMergeContext struct{}

func (naturalMergeContext) NumDeletesToMerge(info *SegmentCommitInfo) int { return info.DelCount() }
func (naturalMergeContext) NumDeletedDocs(info *SegmentCommitInfo) int    { return info.DelCount() }
func (naturalMergeContext) GetInfoStream() InfoStream                     { return nil }
func (naturalMergeContext) GetMergingSegments() map[*SegmentCommitInfo]bool {
	return map[*SegmentCommitInfo]bool{}
}

// maybeMerge runs natural merges selected by the configured merge policy until
// no more merges are found. It reads the latest committed SegmentInfos, finds
// merges, executes them, and writes a new commit if any merges occurred. This is
// a minimal synchronous implementation used by Close and NRT GetReader to honour
// merge-on-close/merge-on-get-reader semantics; it mirrors Lucene's
// IndexWriter.maybeMerge but runs through the merge policy directly rather than
// a scheduler for now.
func (w *IndexWriter) maybeMerge(trigger MergeTrigger) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	return w.maybeMergeLocked(trigger)
}

func (w *IndexWriter) maybeMergeLocked(trigger MergeTrigger) error {
	mp := w.config.GetMergePolicy()
	if mp == nil {
		return nil
	}

	for {
		si := w.lastCommittedSegmentInfos.Clone()
		si.NextGeneration()

		spec, err := mp.FindMerges(trigger, si, naturalMergeContext{})
		if err != nil {
			return fmt.Errorf("maybeMerge: find merges: %w", err)
		}
		if spec == nil || spec.Size() == 0 {
			return nil
		}

		if err := w.executeNaturalMerges(si, spec); err != nil {
			return fmt.Errorf("maybeMerge: execute merges: %w", err)
		}

		if err := WriteSegmentInfos(si, w.directory); err != nil {
			return fmt.Errorf("maybeMerge: write segment infos: %w", err)
		}
		if w.deleter != nil {
			if err := w.deleter.Checkpoint(si, true); err != nil {
				return fmt.Errorf("maybeMerge: deleter checkpoint: %w", err)
			}
		}
		w.lastCommittedSegmentInfos = si.Clone()
		w.committedSegments = si.List()
		w.nrtGen.Add(1)
	}
}

// maybeMergeSnapshot runs natural merges against the supplied in-memory
// SegmentInfos without writing a new segments_N file or checkpointing the
// deleter. It is used by GetReader to compact the NRT snapshot so that a
// merge-on-get-reader policy is reflected in the returned reader. This is a
// deliberately simplified synchronous stand-in for Lucene's background NRT
// merge scheduling; source segment files are deleted eagerly because the
// snapshot is only used while the reader is alive.
func (w *IndexWriter) maybeMergeSnapshot(si *SegmentInfos, trigger MergeTrigger) error {
	mp := w.config.GetMergePolicy()
	if mp == nil {
		return nil
	}

	for {
		spec, err := mp.FindMerges(trigger, si, naturalMergeContext{})
		if err != nil {
			return fmt.Errorf("maybeMergeSnapshot: find merges: %w", err)
		}
		if spec == nil || spec.Size() == 0 {
			return nil
		}

		if err := w.executeNaturalMerges(si, spec); err != nil {
			return fmt.Errorf("maybeMergeSnapshot: execute merges: %w", err)
		}
	}
}

// executeNaturalMerges runs the merges in spec against the supplied SegmentInfos
// without writing a new segments_N. The caller (maybeMerge) writes the updated
// SegmentInfos and checkpoints the deleter. Must be called with w.mu held.
func (w *IndexWriter) executeNaturalMerges(si *SegmentInfos, spec *MergeSpecification) error {
	mergedAway := make(map[*SegmentCommitInfo]bool)
	for _, om := range spec.Merges {
		for _, seg := range om.Segments {
			mergedAway[seg] = true
		}
	}

	result := NewSegmentInfos()
	result.SetGeneration(si.Generation())
	result.SetCounter(si.Counter())
	result.SetInMemoryParentField(w.config.ParentField())
	result.SetInMemoryIndexSort(w.config.IndexSort())
	if userData := si.GetUserData(); len(userData) > 0 {
		result.SetUserData(userData)
	}

	for _, sci := range si.List() {
		if !mergedAway[sci] {
			result.Add(sci)
		}
	}

	for _, om := range spec.Merges {
		if len(om.Segments) == 0 {
			continue
		}
		segName := result.GetNextSegmentName()
		merged, err := w.mergeSegmentGroup(om.Segments, segName)
		if err != nil {
			return err
		}
		if merged != nil {
			result.Add(merged)
		}
	}

	for seg := range mergedAway {
		for _, f := range seg.GetFiles() {
			_ = w.directory.DeleteFile(f)
		}
	}

	// Transfer the updated list back into the caller's SegmentInfos.
	si.Clear()
	for _, sci := range result.List() {
		si.Add(sci)
	}
	// Advance the counter numerically so names like _100 are ordered after _99.
	si.UpdateCounterFromSegments()
	return nil
}

// forceMergeContext is a minimal MergeContext for forced merges: nothing is
// concurrently merging and deletion counts come straight from the
// SegmentCommitInfos; the info stream is disabled.
type forceMergeContext struct{}

func (forceMergeContext) NumDeletesToMerge(info *SegmentCommitInfo) int { return info.DelCount() }
func (forceMergeContext) NumDeletedDocs(info *SegmentCommitInfo) int    { return info.DelCount() }
func (forceMergeContext) GetInfoStream() InfoStream                     { return nil }
func (forceMergeContext) GetMergingSegments() map[*SegmentCommitInfo]bool {
	return map[*SegmentCommitInfo]bool{}
}

// executeForcedMerges runs each OneMerge in spec — merging its segment group
// into one new segment — and writes a new SegmentInfos in which every merged
// group is replaced by its result while untouched segments are kept. Source
// files of merged-away segments are deleted. Must be called with w.mu held.
func (w *IndexWriter) executeForcedMerges(si *SegmentInfos, spec *MergeSpecification) error {
	mergedAway := make(map[*SegmentCommitInfo]bool)
	for _, om := range spec.Merges {
		for _, seg := range om.Segments {
			mergedAway[seg] = true
		}
	}

	result := NewSegmentInfos()
	result.SetGeneration(si.Generation() + 1)
	result.SetCounter(si.Counter())
	result.SetInMemoryParentField(w.config.ParentField())
	result.SetInMemoryIndexSort(w.config.IndexSort())
	if userData := si.GetUserData(); len(userData) > 0 {
		result.SetUserData(userData)
	}

	// Keep the segments no merge touched, in order.
	for _, sci := range si.List() {
		if !mergedAway[sci] {
			result.Add(sci)
		}
	}

	// Execute each merge group into a fresh segment.
	for _, om := range spec.Merges {
		if len(om.Segments) == 0 {
			continue
		}
		segName := result.GetNextSegmentName()
		merged, err := w.mergeSegmentGroup(om.Segments, segName)
		if err != nil {
			return err
		}
		if merged != nil {
			result.Add(merged)
		}
	}

	// Advance the segment-name counter past every segment that now exists on disk,
	// otherwise the next reader/merge will reuse a name already in use.
	result.UpdateCounterFromSegments()

	if err := WriteSegmentInfos(result, w.directory); err != nil {
		return fmt.Errorf("forceMerge: write merged segment infos: %w", err)
	}
	// Update the writer's committed baseline so the next Commit uses the merged
	// SegmentInfos (and its advanced segment-name counter) instead of the pre-merge
	// snapshot (rmp #105.2.7).
	w.lastCommittedSegmentInfos = result.Clone()
	w.committedSegments = result.List()

	// Checkpoint the merged commit so the file deleter can DecRef the source
	// segment files in the correct order when the deletion policy removes the
	// pre-merge commit.
	if w.deleter != nil {
		if err := w.deleter.Checkpoint(result, true); err != nil {
			return fmt.Errorf("forceMerge: deleter checkpoint: %w", err)
		}
	}
	// The merged commit's adoption by the deleter is what will eventually delete
	// the source segment files; do not delete them eagerly here.
	return nil
}

// forceMergeToOneSegment merges every segment in si into one new segment via
// SegmentMerger, registers it as the sole committed segment, and deletes the
// sources. Deleted documents are compacted out. Must be called with w.mu held.
func (w *IndexWriter) forceMergeToOneSegment(si *SegmentInfos) error {
	merged := NewSegmentInfos()
	merged.SetGeneration(si.Generation() + 1)
	merged.SetCounter(si.Counter())
	merged.SetInMemoryParentField(w.config.ParentField())
	merged.SetInMemoryIndexSort(w.config.IndexSort())
	if userData := si.GetUserData(); len(userData) > 0 {
		merged.SetUserData(userData)
	}
	segName := merged.GetNextSegmentName()

	sci, err := w.mergeSegmentGroup(si.List(), segName)
	if err != nil {
		return err
	}
	if sci != nil {
		merged.Add(sci)
		w.committedSegments = []*SegmentCommitInfo{sci}
	} else {
		// Every document was deleted: the merged index has no segments.
		w.committedSegments = nil
	}

	// Advance the segment-name counter past the newly merged segment so later
	// flushes/merges on this index never reuse its name.
	merged.UpdateCounterFromSegments()

	if err := WriteSegmentInfos(merged, w.directory); err != nil {
		return fmt.Errorf("forceMerge: write merged segment infos: %w", err)
	}
	// Update the writer's committed baseline so the next Commit uses the merged
	// SegmentInfos and its segment-name counter.
	w.lastCommittedSegmentInfos = merged.Clone()

	// Checkpoint the merged commit so the file deleter can DecRef the source
	// segment files when the deletion policy removes the pre-merge commit.
	if w.deleter != nil {
		if err := w.deleter.Checkpoint(merged, true); err != nil {
			return fmt.Errorf("forceMerge: deleter checkpoint: %w", err)
		}
	}
	return nil
}

// mergeSegmentGroup merges a group of segments into one new segment named
// segName via the real per-format SegmentMerger, writes the segment data and
// its .si, and returns the resulting SegmentCommitInfo (nil when the group has
// zero live documents). It does NOT write segments_N or delete the sources —
// the caller composes the new SegmentInfos. Must be called with w.mu held.
func (w *IndexWriter) mergeSegmentGroup(segs []*SegmentCommitInfo, segName string) (*SegmentCommitInfo, error) {
	codec := w.config.Codec()
	if codec == nil {
		return nil, errors.New("forceMerge: no codec configured")
	}

	var srs []*SegmentReader
	var readers []*CodecReader
	totalLive := 0
	for _, sci := range segs {
		sr, err := openSegmentReader(w.directory, sci)
		if err != nil {
			return nil, fmt.Errorf("forceMerge: open segment %q: %w", sci.SegmentInfo().Name(), err)
		}
		core := sr.GetCoreReaders()
		if core == nil {
			return nil, fmt.Errorf("forceMerge: segment %q has no codec core readers (cannot merge its data)", sci.SegmentInfo().Name())
		}
		cr := NewCodecReader(core, sr.GetLiveDocs(), sci.NumDocs())
		cr.GetSegmentInfo().SetDocCount(sci.SegmentInfo().DocCount())
		srs = append(srs, sr)
		readers = append(readers, cr)
		totalLive += sci.NumDocs()
	}
	closeReaders := func() {
		for _, sr := range srs {
			_ = sr.Close()
		}
	}
	if totalLive == 0 {
		closeReaders()
		return nil, nil
	}

	mergedSI := NewSegmentInfo(segName, totalLive, w.directory)
		mergedSI.SetID(generateSegmentID())
	mergedSI.SetCodec(codec.Name())
	mergedSI.SetVersion("10.4.0")
	mergedSI.SetMinVersion("10.4.0")
	mergedSI.SetIndexSort(w.config.IndexSort())

	sm, err := NewSegmentMerger(readers, mergedSI, codec, util.NoOpInfoStream, w.directory, store.IOContext{Context: store.ContextMerge})
	if err != nil {
		closeReaders()
		return nil, fmt.Errorf("forceMerge: new segment merger: %w", err)
	}
	ms, err := sm.Merge()
	if err != nil {
		closeReaders()
		return nil, fmt.Errorf("forceMerge: merge: %w", err)
	}

	allFiles, err := w.directory.ListAll()
	if err != nil {
		closeReaders()
		return nil, fmt.Errorf("forceMerge: list directory: %w", err)
	}
	segFiles := []string{segName + ".si"}
	for _, f := range allFiles {
		if ParseSegmentName(f) != segName {
			continue
		}
		switch GetExtension(f) {
		case "si", "cfs", "cfe":
		default:
			segFiles = append(segFiles, f)
		}
	}

	sciMerged := NewSegmentCommitInfo(mergedSI, 0, -1)
	if ms != nil && ms.MergeFieldInfos != nil {
		sciMerged.SetInMemoryFieldInfos(ms.MergeFieldInfos)
	}
	mergedSI.SetFiles(segFiles)
	if err := writeSegmentInfo(w.directory, mergedSI, store.IOContextWrite, w.config.Codec()); err != nil {
		closeReaders()
		return nil, fmt.Errorf("forceMerge: write .si: %w", err)
	}
	closeReaders()

	// Warm the newly merged segment before it becomes visible.
	if warmer := w.config.GetMergedSegmentWarmer(); warmer != nil {
		if sr, warmErr := openSegmentReader(w.directory, sciMerged); warmErr == nil {
			if err := warmer.Warm(sr); err != nil {
				if is := w.config.GetInfoStream(); is.IsEnabled("IW") {
					is.Message("IW", fmt.Sprintf("merged-segment warmer failed for %s: %v", segName, err))
				}
			}
			_ = sr.Close()
		}
	}

	return sciMerged, nil
}

// HasDeletions returns true if this writer has buffered or applied
// deletions. It mirrors Lucene's IndexWriter.hasDeletions().
func (w *IndexWriter) HasDeletions() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.pendingDeleteTerms) > 0 ||
		len(w.pendingDeleteQueries) > 0 ||
		len(w.pendingDeletedDocIDs) > 0 ||
		w.pendingCommittedDeleteCount > 0
}

// TryDeleteDocument deletes the document with the given docID as seen by the
// supplied reader. The docID must be within the reader's MaxDoc range. The
// deletion is buffered and becomes visible to the next NRT reader / Commit,
// exactly like a delete-by-docID issued through the live-docs path.
func (w *IndexWriter) TryDeleteDocument(reader IndexReaderInterface, docID int) (bool, error) {
	if err := w.ensureOpen(); err != nil {
		return false, err
	}
	if reader == nil {
		return false, nil
	}
	if docID < 0 || docID >= reader.MaxDoc() {
		return false, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pendingDeletedDocIDs = append(w.pendingDeletedDocIDs, docID)
	return true, nil
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

// NewestSegment returns the newest committed segment visible to this writer,
// or nil when no segments exist yet.  The newest segment is the one with the
// highest segment-name generation among the currently committed/NRT segment set.
// This mirrors Lucene's IndexWriter.newestSegment().
func (w *IndexWriter) NewestSegment() *SegmentCommitInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var source []*SegmentCommitInfo
	if w.nrtSegmentInfos != nil {
		source = w.nrtSegmentInfos.List()
	} else if w.lastCommittedSegmentInfos != nil {
		source = w.lastCommittedSegmentInfos.List()
	}
	if len(source) == 0 {
		return nil
	}

	newest := source[0]
	maxGen := ParseGeneration(newest.SegmentInfo().Name())
	for _, sci := range source[1:] {
		gen := ParseGeneration(sci.SegmentInfo().Name())
		if gen > maxGen {
			maxGen = gen
			newest = sci
		}
	}
	return newest
}

// GetBufferedDeleteTermsSize returns the number of delete terms
// currently buffered in RAM.
func (w *IndexWriter) GetBufferedDeleteTermsSize() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	seen := make(map[string]struct{}, len(w.pendingDeleteTerms)+len(w.pendingCommittedDeleteTerms))
	for _, tb := range w.pendingDeleteTerms {
		if tb.term == nil {
			continue
		}
		key := tb.term.Field + ":" + tb.term.Text()
		seen[key] = struct{}{}
	}
	for _, t := range w.pendingCommittedDeleteTerms {
		if t == nil {
			continue
		}
		key := t.Field + ":" + t.Text()
		seen[key] = struct{}{}
	}
	return len(seen)
}

// GetFlushCount returns the number of times the index has been flushed.
func (w *IndexWriter) GetFlushCount() int {
	return int(w.flushCount.Load())
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
//	Byte  hasMinVersion (1 + 3 LE ints when si.MinVersion() is set, else 0)
//	Int32 docCount
//	Byte  isCompoundFile (1=true, 255=false)
//	Byte  hasBlocks (1=true, 255=false; matches Lucene's YES/NO byte cast)
//	WriteMapOfStrings(diagnostics)
//	WriteSetOfStrings(files as set)
//	WriteMapOfStrings(attributes)
//	VInt  numSortFields = 0
//	writeFooter()
func writeSegmentInfo(dir store.Directory, si *SegmentInfo, context store.IOContext, codec Codec) error {
	if codec != nil {
		if format := codec.SegmentInfoFormat(); format != nil {
			return format.Write(dir, si, context)
		}
	}
	// Fallback: hard-coded Lucene99 segment-info format used when no codec
	// is configured (structural-unit-test path) or the codec does not expose
	// a SegmentInfoFormat.
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

	// hasMinVersion: writeByte(1) + 3 LE ints when set, otherwise writeByte(0),
	// mirroring Lucene99SegmentInfoFormat.writeSegmentInfo (rmp #4784).
	if minVer, ok := si.MinVersion(); ok {
		if writeErr = out.WriteByte(1); writeErr != nil {
			return fmt.Errorf("writeSegmentInfo %s: hasMinVersion: %w", name, writeErr)
		}
		minMajor, minMinor, minBugfix := parseSegmentVersion(minVer)
		if writeErr = store.WriteInt32LE(out, minMajor); writeErr != nil {
			return fmt.Errorf("writeSegmentInfo %s: minMajor: %w", name, writeErr)
		}
		if writeErr = store.WriteInt32LE(out, minMinor); writeErr != nil {
			return fmt.Errorf("writeSegmentInfo %s: minMinor: %w", name, writeErr)
		}
		if writeErr = store.WriteInt32LE(out, minBugfix); writeErr != nil {
			return fmt.Errorf("writeSegmentInfo %s: minBugfix: %w", name, writeErr)
		}
	} else {
		if writeErr = out.WriteByte(0); writeErr != nil {
			return fmt.Errorf("writeSegmentInfo %s: hasMinVersion: %w", name, writeErr)
		}
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

	// hasBlocks: Lucene writes (byte)(getHasBlocks() ? YES(1) : NO(-1)), so
	// false serialises to 255, not literal 0 (rmp #4784).
	hasBlocks := byte(255)
	if si.HasBlocks() {
		hasBlocks = 1
	}
	if writeErr = out.WriteByte(hasBlocks); writeErr != nil {
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

	// Index sort: numSortFields followed by each SortField, byte-faithful to
	// Lucene90SegmentInfoFormat.write (rmp #4789). This replaces the segments_N
	// _gocene_sort_* userData keys with the authoritative on-disk .si block.
	if writeErr = WriteSegmentInfoSort(out, si.IndexSort()); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: index sort: %w", name, writeErr)
	}

	if writeErr = writeFooter(out); writeErr != nil {
		return fmt.Errorf("writeSegmentInfo %s: footer: %w", name, writeErr)
	}

	return out.Close()
}

// segmentSortProviderName is the SortFieldProvider name Lucene writes before a
// plain SortField in the .si index-sort block (SortField.Provider.NAME).
const segmentSortProviderName = "SortField"

// sortTypeToLuceneName maps a schema.SortType to the Lucene SortField.Type enum
// name (Type.toString()) written into the .si index-sort block. Only the five
// index-sortable plain types are representable here; richer SortField flavours
// (SortedNumericSortField / SortedSetSortField) are not yet modelled by
// schema.SortField and are out of scope for rmp #4789.
func sortTypeToLuceneName(t SortType) (string, error) {
	switch t {
	case SortTypeString:
		return "STRING", nil
	case SortTypeLong:
		return "LONG", nil
	case SortTypeInt:
		return "INT", nil
	case SortTypeFloat:
		return "FLOAT", nil
	case SortTypeDouble:
		return "DOUBLE", nil
	default:
		return "", fmt.Errorf("unsupported sort type %d for .si serialization", int(t))
	}
}

// luceneNameToSortType is the inverse of sortTypeToLuceneName.
func luceneNameToSortType(name string) (SortType, error) {
	switch name {
	case "STRING":
		return SortTypeString, nil
	case "LONG":
		return SortTypeLong, nil
	case "INT":
		return SortTypeInt, nil
	case "FLOAT":
		return SortTypeFloat, nil
	case "DOUBLE":
		return SortTypeDouble, nil
	default:
		return 0, fmt.Errorf("unsupported sort type name %q in .si", name)
	}
}

// WriteSegmentInfoSort serialises the per-segment index sort into the .si body,
// byte-faithful to Lucene90SegmentInfoFormat.write: a VInt field count followed
// by, for each field, the provider name (String) and the SortField payload via
// SortField.serialize:
//
//	writeString(field)
//	writeString(type.toString())
//	writeInt(reverse ? 1 : 0)        // little-endian (DataOutput.writeInt)
//	writeInt(missingValue == null ? 0 : 1)
//	[missing-value payload]          // omitted: schema.SortField carries none
//
// schema.SortField models only the field name, plain type, and reverse flag, so
// the missing value is always serialised as absent (writeInt 0); this matches
// the fidelity of the superseded _gocene_sort_* userData keys. Byte-identity
// against Lucene-produced .si is CI-gated.
//
// Exported so the codec-side .si format (codecs.Lucene99SegmentInfoFormat) can
// emit a byte-identical index-sort block, keeping the two .si writers in
// lock-step (rmp #4789).
func WriteSegmentInfoSort(out store.IndexOutput, sort *Sort) error {
	var fields []SortField
	if sort != nil {
		fields = sort.Fields()
	}
	if err := store.WriteVInt(out, int32(len(fields))); err != nil {
		return err
	}
	for i := range fields {
		sf := fields[i]
		typeName, err := sortTypeToLuceneName(sf.SortType())
		if err != nil {
			return err
		}
		if err := store.WriteString(out, segmentSortProviderName); err != nil {
			return err
		}
		if err := store.WriteString(out, sf.Field()); err != nil {
			return err
		}
		if err := store.WriteString(out, typeName); err != nil {
			return err
		}
		reverse := int32(0)
		if sf.Descending() {
			reverse = 1
		}
		if err := store.WriteInt32LE(out, reverse); err != nil {
			return err
		}
		// missingValue == null: schema.SortField does not model a persisted
		// missing value for the index sort.
		if err := store.WriteInt32LE(out, 0); err != nil {
			return err
		}
	}
	return nil
}

// ReadSegmentInfoSort is the inverse of WriteSegmentInfoSort. It returns nil for
// a zero field count (no index sort). A negative count is a corruption error,
// mirroring Lucene90SegmentInfoFormat.read.
//
// Exported so the codec-side .si reader (codecs.Lucene99SegmentInfoFormat) can
// decode the same index-sort block, keeping the two .si readers in lock-step
// (rmp #4789).
func ReadSegmentInfoSort(in store.IndexInput) (*Sort, error) {
	numSortFields, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	if numSortFields == 0 {
		return nil, nil
	}
	if numSortFields < 0 {
		return nil, fmt.Errorf("invalid index sort field count: %d", numSortFields)
	}
	fields := make([]SortField, 0, numSortFields)
	for i := int32(0); i < numSortFields; i++ {
		provider, err := store.ReadString(in)
		if err != nil {
			return nil, err
		}
		if provider != segmentSortProviderName {
			return nil, fmt.Errorf("unsupported sort field provider %q in .si (only %q is modelled)", provider, segmentSortProviderName)
		}
		field, err := store.ReadString(in)
		if err != nil {
			return nil, err
		}
		typeName, err := store.ReadString(in)
		if err != nil {
			return nil, err
		}
		st, err := luceneNameToSortType(typeName)
		if err != nil {
			return nil, err
		}
		reverse, err := store.ReadInt32LE(in)
		if err != nil {
			return nil, err
		}
		hasMissing, err := store.ReadInt32LE(in)
		if err != nil {
			return nil, err
		}
		if hasMissing == 1 {
			// schema.SortField cannot model a persisted missing value yet; a
			// Lucene-produced .si carrying one is out of scope for rmp #4789.
			return nil, fmt.Errorf("index sort field %q carries a missing value, which is not yet supported", field)
		}
		fields = append(fields, NewSortFieldFull(field, st, reverse == 1))
	}
	return schema.NewSortFromFields(fields), nil
}

// readSegmentInfo reads the Lucene99SegmentInfo .si file for the named segment
// from dir, returning a fully-populated SegmentInfo. It is the exact inverse of
// writeSegmentInfo and mirrors
// org.apache.lucene.codecs.lucene99.Lucene99SegmentInfoFormat.read.
//
// This reader lives in package index (rather than only in package codecs) so
// that the per-segment .si is the authoritative source of docCount and metadata
// even when no concrete codec is blank-imported (codec-less structural tests).
// It is registered as the spi.ReadSegmentInfos .si reader hook in init below,
// replacing the legacy _gocene_dc_ userData round-trip (rmp #4785).
func readSegmentInfo(dir store.Directory, segmentName string, segmentID []byte, context store.IOContext) (*SegmentInfo, error) {
	name := segmentName + ".si"
	raw, err := dir.OpenInput(name, context)
	if err != nil {
		return nil, err
	}
	in := store.NewChecksumIndexInput(raw)
	defer in.Close()

	if _, err := checkIndexHeader(in, "Lucene90SegmentInfo", 0, 0, segmentID, ""); err != nil {
		return nil, fmt.Errorf("readSegmentInfo %s: header: %w", name, err)
	}

	// Version (Java DataOutput.writeInt -> little-endian).
	major, err := store.ReadInt32LE(in)
	if err != nil {
		return nil, err
	}
	minor, err := store.ReadInt32LE(in)
	if err != nil {
		return nil, err
	}
	bugfix, err := store.ReadInt32LE(in)
	if err != nil {
		return nil, err
	}
	luceneVersion := fmt.Sprintf("%d.%d.%d", major, minor, bugfix)

	// hasMinVersion sentinel + optional minVersion ints (rmp #4784).
	hasMinVersion, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	var minVersion string
	switch hasMinVersion {
	case 0:
		// absent
	case 1:
		minMajor, err := store.ReadInt32LE(in)
		if err != nil {
			return nil, err
		}
		minMinor, err := store.ReadInt32LE(in)
		if err != nil {
			return nil, err
		}
		minBugfix, err := store.ReadInt32LE(in)
		if err != nil {
			return nil, err
		}
		minVersion = fmt.Sprintf("%d.%d.%d", minMajor, minMinor, minBugfix)
	default:
		return nil, fmt.Errorf("readSegmentInfo %s: illegal hasMinVersion byte: %d", name, hasMinVersion)
	}

	docCount, err := store.ReadInt32LE(in)
	if err != nil {
		return nil, err
	}

	isCompoundByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	isCompoundFile := isCompoundByte == 1

	hasBlocksByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	hasBlocks := hasBlocksByte == 1

	diagnostics, err := store.ReadMapOfStrings(in)
	if err != nil {
		return nil, err
	}

	files, err := store.ReadSetOfStrings(in)
	if err != nil {
		return nil, err
	}

	attributes, err := store.ReadMapOfStrings(in)
	if err != nil {
		return nil, err
	}

	// Index sort (numSortFields + per-field SortField), the inverse of
	// WriteSegmentInfoSort (rmp #4789).
	indexSort, err := ReadSegmentInfoSort(in)
	if err != nil {
		return nil, fmt.Errorf("readSegmentInfo %s: index sort: %w", name, err)
	}

	if _, err := checkFooter(in); err != nil {
		return nil, fmt.Errorf("readSegmentInfo %s: footer: %w", name, err)
	}

	si := NewSegmentInfo(segmentName, int(docCount), dir)
	si.SetID(segmentID)
	si.SetVersion(luceneVersion)
	if minVersion != "" {
		si.SetMinVersion(minVersion)
	}
	si.SetHasBlocks(hasBlocks)
	si.SetCompoundFile(isCompoundFile)
	si.SetDiagnostics(diagnostics)
	fileList := make([]string, 0, len(files))
	for f := range files {
		fileList = append(fileList, f)
	}
	si.SetFiles(fileList)
	for k, v := range attributes {
		si.SetAttribute(k, v)
	}
	if indexSort != nil {
		si.SetIndexSort(indexSort)
	}
	return si, nil
}

// init registers readSegmentInfo as the spi.ReadSegmentInfos .si reader hook so
// that per-segment docCount and metadata are loaded from the authoritative .si
// file rather than from segments_N userData (rmp #4785). Living in package
// index, this hook is always installed whenever the index reader/writer is
// used, independent of which concrete codec (if any) is blank-imported.
func init() {
	spi.RegisterSegmentInfoReader(func(dir store.Directory, segmentName string, segmentID []byte) (*schema.SegmentInfo, error) {
		return readSegmentInfo(dir, segmentName, segmentID, store.IOContextRead)
	})
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

		// Validate parentField compatibility. The source parent field name is
		// read from the per-segment .fnm parent bit (Lucene94FieldInfosFormat,
		// stamped by the indexing path) as the authoritative source (rmp #4789).
		// It falls back to the _gocene_parent userData key only when no source
		// segment carries the parent bit, which happens for a parentField that
		// was configured but never materialised as a real document field
		// (Gocene's AddDocuments block stub, GOC-4136).
		srcParentField := w.sourceParentFieldFromDisk(dir, sourceSI)
		if srcParentField == "" {
			srcParentField = sourceSI.GetInMemoryParentField()
		}
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
						// FieldInfos are now authoritative on the source .fnm
						// (rmp #4785 removed the _gocene_fi_ userData fallback).
						srcFI = readFieldInfosFromDisk(dir, w.config.Codec(), sci.SegmentInfo())
					}
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
			// Prefer in-memory FieldInfos; otherwise read the authoritative .fnm
			// from the source segment on disk (rmp #4785 removed the _gocene_fi_
			// userData fallback, so AddIndexes must consult the real .fnm).
			srcFI := sci.GetInMemoryFieldInfos()
			if srcFI == nil {
				srcFI = readFieldInfosFromDisk(dir, w.config.Codec(), sci.SegmentInfo())
			}
			if srcFI != nil {
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
						IndexOptions:           info.IndexOptions(),
						DocValuesType:          info.DocValuesType(),
						DocValuesSkipIndexType: DocValuesSkipIndexTypeNone,
						DocValuesGen:           -1,
						Stored:                 info.IsStored(),
						Tokenized:              info.IsTokenized(),
						OmitNorms:              info.OmitNorms(),
						StoreTermVectors:       info.HasTermVectors(),
						// Preserve the parent bit so it round-trips into the
						// imported segment's .fnm (rmp #4789); dropping it would
						// silently demote a block-join parent field to a regular
						// field on AddIndexes.
						IsParentField:            info.IsParentField(),
						VectorEncoding:           VectorEncodingFloat32,
						VectorSimilarityFunction: VectorSimilarityFunctionEuclidean,
					})
					_ = fi.Add(clone)
				}
			}
			ps := pendingSegment{
				numDocs:         sci.SegmentInfo().DocCount(),
				delCount:        sci.DelCount(),
				softDelCount:    sci.SoftDelCount(),
				fieldInfos:      fi,
				deletedOrdinals: sci.GetDeletedOrdinals(),
				srcDir:          dir,
				srcSegmentName:  sci.SegmentInfo().Name(),
				srcFiles:        sci.GetFiles(),
				srcCompoundFile: sci.SegmentInfo().IsCompoundFile(),
				srcSegmentInfo:  sci.SegmentInfo(),
			}
			w.pendingImportedSegments = append(w.pendingImportedSegments, ps)
		}
		w.mu.Unlock()
	}

	return nil
}

// sourceParentFieldFromDisk derives the block-join parent field name of a
// source index from the per-segment .fnm parent bit (rmp #4789).
//
// It scans every source segment's FieldInfos — preferring the in-memory copy,
// otherwise reading the authoritative .fnm via the codec FieldInfosFormat — and
// returns the name of the first field whose IsParentField bit is set. Returns
// the empty string when no segment flags a parent field, matching the semantics
// of the removed _gocene_parent userData key for an index without block joins.
func (w *IndexWriter) sourceParentFieldFromDisk(dir store.Directory, sourceSI *SegmentInfos) string {
	for _, sci := range sourceSI.List() {
		srcFI := sci.GetInMemoryFieldInfos()
		if srcFI == nil {
			srcFI = readFieldInfosFromDisk(dir, w.config.Codec(), sci.SegmentInfo())
		}
		if srcFI == nil {
			continue
		}
		it := srcFI.Iterator()
		for {
			info := it.Next()
			if info == nil {
				break
			}
			if info.IsParentField() {
				return info.Name()
			}
		}
	}
	return ""
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

// copyImportedSegmentFiles copies the on-disk segment files for an
// AddIndexes-imported pending segment from the source directory into this
// writer's directory, renaming them from the source segment name to dstName.
// The .si file is skipped because Commit writes a fresh SegmentInfo. Returns
// the list of copied file names in the destination directory.
func (w *IndexWriter) copyImportedSegmentFiles(ps *pendingSegment, dstName string) ([]string, error) {
	srcFiles := ps.srcFiles
	if len(srcFiles) == 0 {
		all, err := ps.srcDir.ListAll()
		if err != nil {
			return nil, fmt.Errorf("AddIndexes: list source directory: %w", err)
		}
		for _, f := range all {
			if strings.HasPrefix(f, ps.srcSegmentName) {
				srcFiles = append(srcFiles, f)
			}
		}
	}

	copied := make([]string, 0, len(srcFiles))
	for _, srcFile := range srcFiles {
		if GetExtension(srcFile) == "si" {
			// Commit writes a new .si with the imported segment's metadata.
			continue
		}
		if !strings.HasPrefix(srcFile, ps.srcSegmentName) {
			return nil, fmt.Errorf("AddIndexes: source file %q does not start with segment name %q", srcFile, ps.srcSegmentName)
		}
		dstFile := dstName + srcFile[len(ps.srcSegmentName):]
		if err := copyDirectoryFile(ps.srcDir, w.directory, srcFile, dstFile); err != nil {
			return nil, fmt.Errorf("AddIndexes: copy %q -> %q: %w", srcFile, dstFile, err)
		}
		copied = append(copied, dstFile)
	}
	return copied, nil
}

// copyDirectoryFile copies a single file from srcDir to dstDir using the
// standard Directory OpenInput/CreateOutput APIs.
func copyDirectoryFile(srcDir, dstDir store.Directory, srcName, dstName string) error {
	srcIn, err := srcDir.OpenInput(srcName, store.IOContextRead)
	if err != nil {
		return err
	}
	defer srcIn.Close()

	dstOut, err := dstDir.CreateOutput(dstName, store.IOContextWrite)
	if err != nil {
		return err
	}

	length := srcIn.Length()
	const chunk = 8192
	buf := make([]byte, chunk)
	var written int64
	for written < length {
		n := chunk
		if length-written < chunk {
			n = int(length - written)
		}
		if err := srcIn.ReadBytes(buf[:n]); err != nil {
			_ = dstOut.Close()
			return err
		}
		if err := dstOut.WriteBytes(buf[:n]); err != nil {
			_ = dstOut.Close()
			return err
		}
		written += int64(n)
	}
	return dstOut.Close()
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

// parentFieldDoc wraps a Document and appends a synthetic NumericDocValues
// parent field to its field list. This mirrors Lucene's
// DocumentsWriterPerThread.addParentField, which injects a NumericDocValuesField
// with value -1 for the configured parent field name into the last document of
// each block so the parent-field FieldInfo is materialised in the .fnm and
// survives a reopen (rmp #4789).
type parentFieldDoc struct {
	inner  Document
	parent parentMarkerField
}

func (d *parentFieldDoc) GetFields() []interface{} {
	inner := d.inner.GetFields()
	out := make([]interface{}, len(inner)+1)
	copy(out, inner)
	out[len(inner)] = &d.parent
	return out
}

// parentMarkerField is a minimal NumericDocValues field (value -1) that
// implements both indexableFieldMeta and spi.IndexableField so that
// addFieldToInfos registers its FieldInfo with DocValuesType = NUMERIC and
// DWPT's asDwptField sees it as a valid field. The IsParentField flag is set
// via a dedicated accessor checked in addFieldToInfos (rmp #4789).
//
// This mirrors Lucene's DocumentsWriterPerThread.parentField which is a
// ReservedField wrapping a NumericDocValuesField(-1).
type parentMarkerField struct{ name string }

func (f *parentMarkerField) Name() string                 { return f.name }
func (f *parentMarkerField) StringValue() string          { return "" }
func (f *parentMarkerField) BinaryValue() []byte          { return nil }
func (f *parentMarkerField) NumericValue() interface{}    { return int64(-1) }
func (f *parentMarkerField) IsStored() bool               { return false }
func (f *parentMarkerField) IsIndexed() bool              { return false }
func (f *parentMarkerField) IsTokenized() bool            { return false }
func (f *parentMarkerField) IndexOptions() IndexOptions   { return IndexOptionsNone }
func (f *parentMarkerField) DocValuesType() DocValuesType { return DocValuesTypeNumeric }
func (f *parentMarkerField) HasTermVectors() bool         { return false }
func (f *parentMarkerField) OmitNorms() bool              { return false }
func (f *parentMarkerField) IsParentMarker() bool         { return true }

// AddDocuments adds a block of documents atomically.
// This is used for parent-child document relationships.
//
// When a parentField is configured, the last document in the block receives a
// synthetic NumericDocValuesField(parentFieldName, -1), mirroring Lucene's
// DocumentsWriterPerThread.addParentField. This ensures the parent field is
// materialised in the segment's FieldInfos (.fnm) so it survives a reopen and
// AddIndexes can validate parentField compatibility from the on-disk .fnm
// (rmp #4789, replacing the removed _gocene_parent userData fallback).
//
// Gocene deviation: full block-level atomicity (single DWPT flush for the
// whole block) is deferred to GOC-4136; documents are added individually.
func (w *IndexWriter) AddDocuments(docs []Document) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}
	if len(docs) == 0 {
		return nil
	}

	// When index sorting is configured, parent field is required for document
	// blocks (the synthetic parent marker injected into the last document is
	// used by the index sorter to keep block children together).
	parentFieldName := w.config.ParentField()
	indexSort := w.config.IndexSort()
	if indexSort != nil && len(indexSort.Fields()) > 0 && parentFieldName == "" {
		return fmt.Errorf(
			"a parent field must be set in order to use document blocks with index sorting; see IndexWriterConfig#setParentField")
	}

	// No document in the block may contain a field whose name matches the
	// reserved parent field (it will be injected into the last document).
	if parentFieldName != "" {
		for _, doc := range docs {
			for _, f := range doc.GetFields() {
				if fi, ok := f.(interface{ Name() string }); ok && fi.Name() == parentFieldName {
					return fmt.Errorf(
						"%q is a reserved field and should not be added to any document",
						parentFieldName)
				}
			}
		}
	}

	for i, doc := range docs {
		d := doc
		// Inject the synthetic parent marker into the last document of the block,
		// mirroring Lucene's DocumentsWriterPerThread.updateDocuments which wraps
		// the last document with addParentField when parentField != null.
		if parentFieldName != "" && i == len(docs)-1 {
			d = &parentFieldDoc{inner: doc, parent: parentMarkerField{name: parentFieldName}}
		}
		if err := w.AddDocument(d); err != nil {
			return err
		}
	}
	return nil
}

// UpdateDocValues updates the doc values for documents matching the given term.
// This is used for updating numeric doc values without re-indexing.
//
// Fields that participate in the index sort are not updatable via
// UpdateDocValues (mirroring Lucene's IllegalArgumentException).
func (w *IndexWriter) UpdateDocValues(term *Term, field string, value interface{}) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Reject UpdateDocValues on fields that participate in the index sort.
	if sort := w.config.IndexSort(); sort != nil {
		for _, sf := range sort.Fields() {
			if sf.Field() == field {
				return fmt.Errorf(
					"cannot update doc values for field %q because it participates in the index sort",
					field)
			}
		}
	}

	// Reject type-mismatched updates so that, for example, a numeric update
	// against a binary DocValues field is caught early.  A nil value is the
	// "reset" update and is allowed for any existing DocValues field.
	dvt := w.fieldDocValuesTypeLocked(field)
	if !dvt.HasDocValues() {
		return fmt.Errorf(
			"cannot update doc values for field %q: field has no doc values",
			field)
	}
	if value != nil {
		switch value.(type) {
		case int64:
			if dvt != DocValuesTypeNumeric {
				return fmt.Errorf(
					"cannot update doc values for field %q: expected numeric doc values but found %v",
					field, dvt)
			}
		case []byte:
			if dvt != DocValuesTypeBinary {
				return fmt.Errorf(
					"cannot update doc values for field %q: expected binary doc values but found %v",
					field, dvt)
			}
		default:
			return fmt.Errorf(
				"cannot update doc values for field %q: unsupported value type %T",
				field, value)
		}
	}

	w.pendingDVUpdates = append(w.pendingDVUpdates, pendingDocValuesUpdate{
		term:  term,
		field: field,
		value: value,
	})
	return nil
}

// validateSortFieldTypes checks that each DocValues field in the document whose
// name matches an index-sort field carries the expected DocValues type.
// Non-DV fields with the same name (e.g. stored or indexed fields) are
// ignored — only explicit DocValues fields are validated.
// Mismatches produce an error mirroring Lucene's
// "expected field [X] to be ..." message.
func (w *IndexWriter) validateSortFieldTypes(doc Document, sort *Sort) error {
	fields := doc.GetFields()
	for _, sf := range sort.Fields() {
		fieldName := sf.Field()
		for _, f := range fields {
			if fi, ok := f.(interface{ Name() string }); !ok || fi.Name() != fieldName {
				continue
			}
			// Found a field matching the sort field name. Check if it's a DV field.
			dvt, ok := f.(interface{ DocValuesType() DocValuesType })
			if !ok {
				continue // not a DV-capable field
			}
			got := dvt.DocValuesType()
			if got == DocValuesTypeNone {
				continue // field has no doc values (e.g. stored-only); skip
			}
			want := sortFieldDVType(sf)
			if want != DocValuesTypeNone && got != want {
				return fmt.Errorf(
					"expected field [%s] to be %v, but got %v",
					fieldName, want, got)
			}
			break
		}
	}
	return nil
}

// sortFieldDVType maps a SortField to the expected DocValuesType.
// Multi-valued sorts are detected via the Selector() method:
//   - SortedNumericSortField sets selector to "min"
//   - SortedSetSortField sets selector to "min" and sortType to STRING
func sortFieldDVType(sf SortField) DocValuesType {
	sel := sf.Selector()
	st := sf.SortType()
	if sel != "" {
		// Multi-valued sort fields
		switch {
		case st == SortTypeString:
			return DocValuesTypeSortedSet
		case st == SortTypeLong || st == SortTypeInt || st == SortTypeFloat || st == SortTypeDouble:
			return DocValuesTypeSortedNumeric
		}
	}
	// Single-valued sort fields
	switch st {
	case SortTypeString:
		return DocValuesTypeSorted
	case SortTypeLong, SortTypeInt, SortTypeFloat, SortTypeDouble:
		return DocValuesTypeNumeric
	default:
		return DocValuesTypeNone
	}
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
