// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexWriter creates and maintains an index.
//
// This is the Go port of Lucene's org.apache.lucene.index.IndexWriter.
type IndexWriter struct {
	mu sync.Mutex

	// Tragic exception
	tragedy atomic.Value // stores error or nil
	config  *IndexWriterConfig

	dirOrig store.Directory // original user directory
	dir     store.Directory // wrapped with additional checks

	// increments every time a change is completed
	changeCount atomic.Int64
	// last changeCount that was committed
	lastCommitChangeCount atomic.Int64

	// list of segmentInfo we will fallback to if the commit fails
	rollbackSegments spi.SegmentCommitInfoList

	// set when a commit is pending (after PrepareCommit() & before Commit())
	pendingCommit            *SegmentInfos
	pendingSeqNo             int64
	pendingCommitChangeCount int64

	filesToCommit []string

	segmentInfos         *SegmentInfos
	globalFieldNumberMap *FieldNumbers

	docWriter             *DocumentsWriter
	eventQueue            *eventQueue
	mergeSource           *indexWriterMergeSource
	addIndexesMergeSource *addIndexesMergeSource

	writeDocValuesLock sync.Mutex

	// pendingDVUpdates buffers UpdateDocValues/TryUpdateDocValue requests
	// until the next Commit resolves them against the committed segments.
	pendingDVUpdates []pendingDocValuesUpdate

	deleter *IndexFileDeleter

	segmentsToMerge     map[*SegmentCommitInfo]bool
	mergeMaxNumSegments int

	writeLock store.Lock

	closed  atomic.Bool
	closing atomic.Bool

	mergeNeeded atomic.Bool

	running        atomic.Bool
	commitUserData []mapEntry[string, string]

	mergingSegments         map[*SegmentCommitInfo]bool
	mergeScheduler          MergeScheduler
	runningAddIndexesMerges map[*SegmentMerger]struct{}
	pendingMerges           []*OneMerge
	runningMerges           map[*OneMerge]struct{}
	mergeExceptions         []error
	// merges removed
	mergeGen              int64
	didMessageState       bool
	flushCount            atomic.Int32
	flushDeletesCount     atomic.Int32
	readerPool            *ReaderPool
	bufferedUpdatesStream *BufferedUpdatesStream

	eventListener IndexWriterEventListener

	mergeFinishedGen atomic.Int64

	// The instance that was passed to the constructor.
	liveConfig *LiveIndexWriterConfig

	startCommitTime int64

	pendingNumDocs atomic.Int64

	softDeletesEnabled bool

	flushNotificationsProv FlushNotifications

	fullFlushLock sync.Mutex
	commitLock    sync.Mutex

	// publishedSeqNo tracks the last published sequence number from flushes
	publishedSeqNo int64
}

type eventQueue struct {
	closed bool
	queue  chan func(*IndexWriter) error
	writer *IndexWriter
	mu     sync.Mutex
}

func newEventQueue(writer *IndexWriter) *eventQueue {
	return &eventQueue{
		queue:  make(chan func(*IndexWriter) error, 1024),
		writer: writer,
	}
}

func (eq *eventQueue) add(event func(*IndexWriter) error) bool {
	eq.mu.Lock()
	defer eq.mu.Unlock()
	if eq.closed {
		return false
	}
	eq.queue <- event
	return true
}

func (eq *eventQueue) processEvents(writer *IndexWriter) error {
	for {
		select {
		case event := <-eq.queue:
			if err := event(writer); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (eq *eventQueue) close() error {
	eq.mu.Lock()
	defer eq.mu.Unlock()
	eq.closed = true
	for len(eq.queue) > 0 {
		event := <-eq.queue
		if err := event(eq.writer); err != nil {
			return err
		}
	}
	return nil
}

// unboundedMaxMergeSegments is the maxNumSegments value that means "no
// bound". Mirrors org.apache.lucene.index.IndexWriter.UNBOUNDED_MAX_MERGE_SEGMENTS.
const unboundedMaxMergeSegments = -1

// indexWriterMergeSource is the MergeScheduler.MergeSource an IndexWriter
// hands to its MergeScheduler. Mirrors the
// org.apache.lucene.index.IndexWriter.IndexWriterMergeSource record.
type indexWriterMergeSource struct {
	writer *IndexWriter
}

func (s *indexWriterMergeSource) GetWriter() *IndexWriter {
	return s.writer
}

// GetNextMerge returns the next merge the MergePolicy requested, or nil when
// there is none.
func (s *indexWriterMergeSource) GetNextMerge() *OneMerge {
	return s.writer.getNextMerge()
}

// OnMergeFinished does the finishing bookkeeping for a merge.
func (s *indexWriterMergeSource) OnMergeFinished(merge *OneMerge) {
	s.writer.mergeFinish(merge)
}

// HasPendingMerges reports whether merges are waiting to be scheduled.
func (s *indexWriterMergeSource) HasPendingMerges() bool {
	return s.writer.hasPendingMerges()
}

func (s *indexWriterMergeSource) Merge(merge *OneMerge) error {
	return s.writer.Merge(merge)
}

func (s *indexWriterMergeSource) String() string {
	return s.writer.segString()
}

type addIndexesMergeSource struct {
	writer *IndexWriter
}

func (s *addIndexesMergeSource) GetWriter() *IndexWriter {
	return s.writer
}

// NewIndexWriter constructs a new IndexWriter per the settings given in conf.
func NewIndexWriter(d store.Directory, conf *IndexWriterConfig) (iw *IndexWriter, err error) {
	if conf == nil {
		panic("config must not be null")
	}

	writeLock, err := d.ObtainLock("write.lock")
	if err != nil {
		return nil, err
	}

	liveConfig := conf.LiveIndexWriterConfig
	softDeletesEnabled := liveConfig.GetSoftDeletesField() != ""

	writer := &IndexWriter{
		config:             conf,
		dirOrig:            d,
		liveConfig:         liveConfig,
		softDeletesEnabled: softDeletesEnabled,
		writeLock:          writeLock,

		segmentsToMerge:         make(map[*SegmentCommitInfo]bool),
		mergingSegments:         make(map[*SegmentCommitInfo]bool),
		runningMerges:           make(map[*OneMerge]struct{}),
		runningAddIndexesMerges: make(map[*SegmentMerger]struct{}),
	}

	success := false
	defer func() {
		if success {
			return
		}
		closeErr := writeLock.Close()
		switch {
		case closeErr == nil:
		case err == nil:
			err = closeErr
		default:
			err = fmt.Errorf("%w; releasing the write lock also failed: %v", err, closeErr)
		}
	}()

	writer.dir = store.NewLockValidatingDirectoryWrapper(d, writeLock)
	writer.mergeScheduler = liveConfig.GetMergeScheduler()
	if err := writer.mergeScheduler.Initialize(liveConfig.GetInfoStream(), d); err != nil {
		return nil, err
	}

	mode := liveConfig.GetOpenMode()
	indexExists, err := IndexExists(writer.dir)
	if err != nil {
		return nil, err
	}
	var create bool

	switch mode {
	case Create:
		create = true
	case Append:
		if !indexExists {
			return nil, fmt.Errorf("index not found at path")
		}
		create = false
	case CreateOrAppend:
		create = !indexExists
	}

	var segmentInfos *SegmentInfos
	var rollbackSegments spi.SegmentCommitInfoList

	if create {
		sis := spi.NewSegmentInfosWithCreatedVersionMajor(int32(liveConfig.GetIndexCreatedVersionMajor()))
		if indexExists {
			previous, err := spi.ReadLatestCommit(writer.dir)
			if err != nil {
				return nil, err
			}
			sis.UpdateGenerationVersionAndCounter(previous)
		}
		segmentInfos = sis
		writer.segmentInfos = segmentInfos
		rollbackSegments = segmentInfos.CreateBackupSegmentInfos()
		// Record that we have a change (zero out all segments) pending:
		writer.changed()
	} else {
		commit := conf.GetIndexCommit()
		if commit != nil {
			reader := commit.GetReader()
			if reader == nil {
				return nil, fmt.Errorf("index must already have an initial commit to open from reader")
			}
			segmentInfos = reader.GetSegmentInfos().Clone()
			lastCommit, err := spi.ReadCommit(writer.dirOrig, segmentInfos.GetFileName())
			if err != nil {
				return nil, fmt.Errorf("provided reader is stale: %w", err)
			}
			rollbackSegments = lastCommit.CreateBackupSegmentInfos()
		} else {
			files, err := writer.dir.ListAll()
			if err != nil {
				return nil, err
			}
			lastSegmentsFile := spi.GetLastCommitSegmentsFileName(files)
			if lastSegmentsFile == "" {
				return nil, fmt.Errorf("no segments* file found in %s", writer.dir)
			}
			segmentInfos, err = spi.ReadCommit(writer.dirOrig, lastSegmentsFile)
			if err != nil {
				return nil, err
			}
			rollbackSegments = segmentInfos.CreateBackupSegmentInfos()
		}
	}

	writer.segmentInfos = segmentInfos
	writer.rollbackSegments = rollbackSegments
	writer.globalFieldNumberMap = writer.getFieldNumberMap()

	writer.bufferedUpdatesStream = NewBufferedUpdatesStream(liveConfig.GetInfoStream())
	writer.docWriter = NewDocumentsWriter(
		writer.flushNotifications(),
		int(segmentInfos.GetIndexCreatedVersionMajor()),
		&writer.pendingNumDocs,
		false,
		writer.newSegmentName,
		liveConfig,
		writer.dirOrig,
		writer.dir,
		writer.globalFieldNumberMap,
	)

	writer.readerPool = NewReaderPool()
	if liveConfig.GetReaderPooling() {
		writer.readerPool.EnableReaderPooling()
	}

	dirFiles, err := writer.dir.ListAll()
	if err != nil {
		return nil, err
	}
	writer.deleter, err = NewIndexFileDeleter(
		dirFiles,
		writer.dirOrig,
		writer.dir,
		liveConfig.GetIndexDeletionPolicy(),
		segmentInfos,
		liveConfig.GetInfoStream(),
		writer,
		indexExists,
		false,
	)
	if err != nil {
		return nil, err
	}

	writer.eventQueue = newEventQueue(writer)
	writer.mergeSource = &indexWriterMergeSource{writer: writer}
	writer.addIndexesMergeSource = &addIndexesMergeSource{writer: writer}

	writer.running.Store(true)
	success = true
	return writer, nil
}

func (w *IndexWriter) newSegmentName() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Important to increment changeCount so that the segmentInfos is written
	// on close. Otherwise we could close, re-open and re-return the same
	// segment name that was previously returned, which can cause problems at
	// least with ConcurrentMergeScheduler.
	w.changeCount.Add(1)
	w.segmentInfos.Changed()
	counter := w.segmentInfos.GetCounter()
	w.segmentInfos.SetCounter(counter + 1)
	return "_" + strconv.FormatInt(counter, 36)
}

func (w *IndexWriter) GetMaxCompletedSequenceNumber() int64 {
	return w.publishedSeqNo
}

func (w *IndexWriter) changed() {
	w.changeCount.Add(1)
	w.segmentInfos.Changed()
}

func (w *IndexWriter) ensureOpen(failIfClosing bool) error {
	if w.closed.Load() {
		return fmt.Errorf("IndexWriter has been closed")
	}
	if failIfClosing && w.closing.Load() {
		return fmt.Errorf("IndexWriter is closing")
	}
	return nil
}

// OnTragicEvent records the tragic exception, unless one is already recorded,
// and logs it. It does not close the writer and may be called from any
// location without respecting a lock order.
//
// Mirrors org.apache.lucene.index.IndexWriter#onTragicEvent.
func (w *IndexWriter) OnTragicEvent(tragedy error, location string) {
	if tragedy == nil {
		return
	}
	infoStream := w.config.GetInfoStream()
	if infoStream != nil && infoStream.IsEnabled("IW") {
		infoStream.Message("IW", fmt.Sprintf("hit tragic %T inside %s: %v", tragedy, location, tragedy))
	}
	// Only set it once.
	w.tragedy.CompareAndSwap(nil, tragedy)
}

// tragicEvent sets the tragic exception unless one is already set and closes
// the writer if necessary. It does not re-raise the error passed to it.
//
// Mirrors org.apache.lucene.index.IndexWriter#tragicEvent.
func (w *IndexWriter) tragicEvent(t error, location string) error {
	w.OnTragicEvent(t, location)
	return w.maybeCloseOnTragicEvent()
}

func (w *IndexWriter) maybeCloseOnTragicEvent() error {
	if t, ok := w.tragedy.Load().(error); ok && t != nil {
		return fmt.Errorf("index: this IndexWriter hit a tragic event: %w", t)
	}
	return nil
}

// toIndexableFields adapts the document-level field view to the index-level
// IndexableField the indexing chain consumes. Every concrete field type
// implements both interfaces; a field that does not is a programming error and
// is reported rather than silently dropped.
func toIndexableFields(fields []document.IndexableField) ([]IndexableField, error) {
	out := make([]IndexableField, len(fields))
	for i, f := range fields {
		field, ok := f.(IndexableField)
		if !ok {
			return nil, fmt.Errorf("index: field %q does not implement index.IndexableField", f.Name())
		}
		out[i] = field
	}
	return out, nil
}

// toIndexableFieldsBatch adapts a block of documents to the indexing chain's
// field view, preserving document order.
func toIndexableFieldsBatch(docs []*document.Document) ([][]IndexableField, error) {
	out := make([][]IndexableField, len(docs))
	for i, doc := range docs {
		fields, err := toIndexableFields(doc.GetAllFields())
		if err != nil {
			return nil, err
		}
		out[i] = fields
	}
	return out, nil
}

// AddDocument adds a document to this index.
//
// Mirrors org.apache.lucene.index.IndexWriter#addDocument.
func (w *IndexWriter) AddDocument(doc *document.Document) (int64, error) {
	return w.UpdateDocument(nil, doc)
}

// UpdateDocument updates a document by first deleting the documents containing
// term and then adding the new document.
//
// Mirrors org.apache.lucene.index.IndexWriter#updateDocument.
func (w *IndexWriter) UpdateDocument(term *Term, doc *document.Document) (int64, error) {
	fields, err := toIndexableFields(doc.GetAllFields())
	if err != nil {
		return 0, err
	}
	return w.updateDocumentsInternal(newDeleteTermNode(term), [][]IndexableField{fields})
}

// AddDocuments atomically adds a block of documents with sequentially assigned
// document IDs.
//
// Mirrors org.apache.lucene.index.IndexWriter#addDocuments.
func (w *IndexWriter) AddDocuments(docs []*document.Document) (int64, error) {
	return w.UpdateDocuments(nil, docs)
}

// UpdateDocuments atomically deletes documents matching term and adds a block
// of documents with sequentially assigned document IDs.
//
// Mirrors org.apache.lucene.index.IndexWriter#updateDocuments.
func (w *IndexWriter) UpdateDocuments(term *Term, docs []*document.Document) (int64, error) {
	docsFields, err := toIndexableFieldsBatch(docs)
	if err != nil {
		return 0, err
	}
	return w.updateDocumentsInternal(newDeleteTermNode(term), docsFields)
}

// newDeleteTermNode wraps term in a delete node, or returns nil when there is
// no term to delete by. Mirrors the ternary Lucene applies before calling the
// private updateDocuments overload.
func newDeleteTermNode(term *Term) Node {
	if term == nil {
		return nil
	}
	return NewTermNode(*term)
}

// updateDocumentsInternal is the single entry point every add/update variant
// funnels through, mirroring the private
// org.apache.lucene.index.IndexWriter#updateDocuments(Node, Iterable).
func (w *IndexWriter) updateDocumentsInternal(delNode Node, docsFields [][]IndexableField) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	seqNo, err := w.docWriter.UpdateDocuments(docsFields, delNode)
	if err != nil {
		infoStream := w.config.GetInfoStream()
		if infoStream != nil && infoStream.IsEnabled("IW") {
			infoStream.Message("IW", "hit exception updating document")
		}
		if closeErr := w.maybeCloseOnTragicEvent(); closeErr != nil {
			return 0, closeErr
		}
		return 0, err
	}
	return w.maybeProcessEvents(seqNo)
}

// UpdateNumericDocValue updates a document's NumericDocValues for field to the given value.
// You can only update fields that already exist in the index, not add new fields through
// this method. You can only update fields that were indexed with doc values only.
func (w *IndexWriter) UpdateNumericDocValue(term *Term, field string, value int64) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	w.globalFieldNumberMap.VerifyOrCreateDvOnlyField(field, DocValuesTypeNumeric, true)
	if _, ok := w.liveConfig.indexSortFields[field]; ok {
		return 0, fmt.Errorf("cannot update docvalues field involved in the index sort, field=%s", field)
	}
	seqNo, err := w.docWriter.UpdateDocValues(NewNumericDocValuesUpdate(term, field, &value))
	if err != nil {
		return 0, w.tragicEvent(err, "UpdateNumericDocValue")
	}
	return w.maybeProcessEvents(seqNo)
}

// UpdateBinaryDocValue updates a document's BinaryDocValues for field to the given value.
// You can only update fields that already exist in the index, not add new fields through
// this method. You can only update fields that were indexed only with doc values.
func (w *IndexWriter) UpdateBinaryDocValue(term *Term, field string, value []byte) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	if value == nil {
		return 0, fmt.Errorf("cannot update a field to a null value: %s", field)
	}
	w.globalFieldNumberMap.VerifyOrCreateDvOnlyField(field, DocValuesTypeBinary, true)
	seqNo, err := w.docWriter.UpdateDocValues(NewBinaryDocValuesUpdate(term, field, value))
	if err != nil {
		return 0, w.tragicEvent(err, "UpdateBinaryDocValue")
	}
	return w.maybeProcessEvents(seqNo)
}

// UpdateDocValues updates documents' DocValues fields to the given values.
// Each field update is applied to the set of documents that are associated with the Term
// to the same value. All updates are atomically applied and flushed together.
func (w *IndexWriter) UpdateDocValues(term *Term, updates []*document.Field) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	dvUpdates := w.buildDocValuesUpdate(term, updates)
	seqNo, err := w.docWriter.UpdateDocValues(dvUpdates...)
	if err != nil {
		return 0, w.tragicEvent(err, "UpdateDocValues")
	}
	return w.maybeProcessEvents(seqNo)
}

func (w *IndexWriter) buildDocValuesUpdate(term *Term, updates []*document.Field) []DocValuesUpdate {
	dvUpdates := make([]DocValuesUpdate, len(updates))
	for i, f := range updates {
		dvType := f.FieldType().DocValuesType
		if dvType == DocValuesTypeNone {
			panic(fmt.Sprintf("can only update NUMERIC or BINARY fields! field=%s", f.Name()))
		}
		w.globalFieldNumberMap.VerifyOrCreateDvOnlyField(f.Name(), dvType, false)
		if _, ok := w.liveConfig.indexSortFields[f.Name()]; ok {
			panic(fmt.Sprintf("cannot update docvalues field involved in the index sort, field=%s", f.Name()))
		}

		switch dvType {
		case DocValuesTypeNumeric:
			val := f.NumericValue()
			var longVal int64
			if val != nil {
				switch v := val.(type) {
				case int:
					longVal = int64(v)
				case int32:
					longVal = int64(v)
				case int64:
					longVal = v
				case float32:
					longVal = int64(v)
				case float64:
					longVal = int64(v)
				case byte:
					longVal = int64(v)
				case int16:
					longVal = int64(v)
				default:
					panic(fmt.Sprintf("unsupported numeric type: %T", val))
				}
			} else {
				longVal = -1 // represent null
			}
			var pVal *int64
			if val != nil {
				pVal = &longVal
			}
			dvUpdates[i] = NewNumericDocValuesUpdate(term, f.Name(), pVal)
		case DocValuesTypeBinary:
			dvUpdates[i] = NewBinaryDocValuesUpdate(term, f.Name(), f.BinaryValue())
		default:
			panic(fmt.Sprintf("can only update NUMERIC or BINARY fields: field=%s, type=%v", f.Name(), dvType))
		}
	}
	return dvUpdates
}

// SoftUpdateDocument atomically updates documents matching the provided term with the given
// doc-values fields and adds a block of documents with sequentially assigned document IDs.
func (w *IndexWriter) SoftUpdateDocument(term *Term, doc *document.Document, softDeletes []*document.Field) (int64, error) {
	if term == nil {
		return 0, fmt.Errorf("term must not be null")
	}
	if len(softDeletes) == 0 {
		return 0, fmt.Errorf("at least one soft delete must be present")
	}
	dvUpdates := w.buildDocValuesUpdate(term, softDeletes)
	delNode := NewDocValuesUpdatesNode(dvUpdates...)

	fields, err := toIndexableFields(doc.GetAllFields())
	if err != nil {
		return 0, err
	}

	seqNo, err := w.docWriter.UpdateDocuments([][]IndexableField{fields}, delNode)
	if err != nil {
		return 0, w.tragicEvent(err, "SoftUpdateDocument")
	}
	return w.maybeProcessEvents(seqNo)
}

// SoftUpdateDocuments atomically updates documents matching the provided term with the given
// doc-values fields and adds a block of documents with sequentially assigned document IDs.
func (w *IndexWriter) SoftUpdateDocuments(term *Term, docs []*document.Document, softDeletes []*document.Field) (int64, error) {
	if term == nil {
		return 0, fmt.Errorf("term must not be null")
	}
	if len(softDeletes) == 0 {
		return 0, fmt.Errorf("at least one soft delete must be present")
	}
	dvUpdates := w.buildDocValuesUpdate(term, softDeletes)
	delNode := NewDocValuesUpdatesNode(dvUpdates...)

	docsFields, err := toIndexableFieldsBatch(docs)
	if err != nil {
		return 0, err
	}

	seqNo, err := w.docWriter.UpdateDocuments(docsFields, delNode)
	if err != nil {
		return 0, w.tragicEvent(err, "SoftUpdateDocuments")
	}
	return w.maybeProcessEvents(seqNo)
}

// NrtIsCurrent returns true if the given segment infos are still current for the writer.
func (w *IndexWriter) NrtIsCurrent(sis *SegmentInfos) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.segmentInfos.Generation() == sis.Generation() && !w.hasUncommittedChanges()
}

func (w *IndexWriter) GetNRTGeneration() int64 {
	return w.changeCount.Load()
}

func (w *IndexWriter) hasUncommittedChanges() bool {
	return w.changeCount.Load() != w.lastCommitChangeCount.Load()
}

func (w *IndexWriter) GetAnalyzer() api.Analyzer {
	return w.config.LiveIndexWriterConfig.GetAnalyzer()
}

func (w *IndexWriter) DeleteDocuments(terms []Term) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	seqNo, err := w.docWriter.DeleteTerms(terms...)
	if err != nil {
		return 0, err
	}
	return w.maybeProcessEvents(seqNo)
}

// DeleteAll deletes all documents from the index.
func (w *IndexWriter) DeleteAll() (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}

	// Directly call docWriter to avoid loop with DeleteDocumentsByQuery
	seqNo, err := w.docWriter.DeleteQueries(&MatchAllDocsQuery{})
	if err != nil {
		return 0, err
	}
	return w.maybeProcessEvents(seqNo)
}

// TryDeleteDocument attempts to delete a document by its global docID.
// This is the Go port of Lucene's org.apache.lucene.index.IndexWriter#tryDeleteDocument.
func (w *IndexWriter) TryDeleteDocument(docID int) (bool, error) {
	if err := w.ensureOpen(true); err != nil {
		return false, err
	}

	sci, localDocID, err := w.tryModifyDocument(docID)
	if err != nil {
		return false, err
	}

	rau := w.getPooledInstance(sci, true)
	if rau == nil {
		return false, fmt.Errorf("failed to get pooled instance for segment %s", sci)
	}
	defer w.release(rau)

	deleted, err := rau.Delete(localDocID)
	if err != nil {
		return false, err
	}

	if deleted {
		fullyDeleted, err := rau.IsFullyDeleted()
		if err == nil && fullyDeleted {
			w.dropDeletedSegment(sci)
			w.checkpoint()
		}
	}

	return deleted, nil
}

func (w *IndexWriter) tryModifyDocument(docID int) (*SegmentCommitInfo, int, error) {
	base := 0
	for sci := range w.segmentInfos.Iterator() {
		maxDoc := sci.SegmentInfo().MaxDoc()
		if docID < base+maxDoc {
			return sci, docID - base, nil
		}
		base += maxDoc
	}
	return nil, 0, fmt.Errorf("docID %d out of range [0, %d)", docID, w.segmentInfos.TotalMaxDoc())
}

// DeleteDocumentsQuery deletes documents matching the given queries.
func (w *IndexWriter) DeleteDocumentsQuery(queries []Query) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}

	for _, q := range queries {
		if _, ok := q.(*MatchAllDocsQuery); ok {
			return w.DeleteAll()
		}
	}

	seqNo, err := w.docWriter.DeleteQueries(queries...)
	if err != nil {
		return 0, err
	}
	return w.maybeProcessEvents(seqNo)
}

func (w *IndexWriter) doBeforeFlush() {}

func (w *IndexWriter) doFlush(applyAllDeletes bool) (int64, error) {
	if err := w.maybeCloseOnTragicEvent(); err != nil {
		return 0, err
	}

	w.doBeforeFlush()

	var seqNo int64

	flushErr := func() error {
		w.fullFlushLock.Lock()
		defer w.fullFlushLock.Unlock()

		var err error
		seqNo, err = w.docWriter.FlushAllThreads()
		flushSuccess := err == nil
		if err == nil {
			if seqNo >= 0 {
				w.flushCount.Add(1)
			}
			err = w.publishFlushedSegments(true)
			flushSuccess = err == nil
		}
		if finishErr := w.docWriter.FinishFullFlush(flushSuccess); finishErr != nil && err == nil {
			err = finishErr
		}
		if eventErr := w.eventQueue.processEvents(w); eventErr != nil && err == nil {
			err = eventErr
		}
		return err
	}()
	if flushErr != nil {
		return 0, flushErr
	}

	if applyAllDeletes {
		if err := w.ApplyAllDeletesAndUpdates(); err != nil {
			return 0, err
		}
	}

	anyChanges := (seqNo < 0) || w.mergeNeeded.Swap(false)
	if anyChanges {
		w.maybeMerge(w.config.GetMergePolicy(), MergeTriggerFullFlush, -1)
	}

	w.mu.Lock()
	w.writeReaderPool(applyAllDeletes)
	w.doAfterFlush()
	w.mu.Unlock()

	return seqNo, nil
}

func (w *IndexWriter) PrepareCommit() (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	seqNo, err := func() (int64, error) {
		w.commitLock.Lock()
		defer w.commitLock.Unlock()
		return w.prepareCommitInternal()
	}()
	if err != nil {
		return 0, err
	}
	w.pendingSeqNo = seqNo

	// We must do this outside of the commitLock else we can deadlock:
	if w.mergeNeeded.Swap(false) {
		if err := w.maybeMerge(w.config.GetMergePolicy(), MergeTriggerFullFlush, unboundedMaxMergeSegments); err != nil {
			return 0, err
		}
	}
	return seqNo, nil
}

func (w *IndexWriter) prepareCommitInternal() (int64, error) {
	seqNo, err := w.doFlush(true)
	if err != nil {
		return 0, err
	}

	if w.changeCount.Load() != w.lastCommitChangeCount.Load() {
		w.changeCount.Add(1)
		w.changed()
	}

	toCommit := w.segmentInfos.Clone()
	w.pendingCommitChangeCount = w.changeCount.Load()
	// This protects the segmentInfos we are now going to commit: in case a
	// merge completes while we sync the referenced files it would otherwise
	// remove the files we are syncing.
	if err := w.deleter.IncRef(toCommit, false); err != nil {
		return 0, err
	}

	w.pendingCommit = toCommit
	return seqNo, nil
}

func (w *IndexWriter) Commit() (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	seqNo, err := func() (int64, error) {
		w.commitLock.Lock()
		defer w.commitLock.Unlock()

		var seqNo int64
		if w.pendingCommit == nil {
			var err error
			seqNo, err = w.prepareCommitInternal()
			if err != nil {
				return 0, err
			}
		} else {
			seqNo = w.pendingSeqNo
		}

		if err := w.finishCommit(); err != nil {
			return 0, err
		}
		return seqNo, nil
	}()
	if err != nil {
		return 0, err
	}

	// We must do this outside of the commitLock else we can deadlock:
	if w.mergeNeeded.Swap(false) {
		if err := w.maybeMerge(w.config.GetMergePolicy(), MergeTriggerFullFlush, unboundedMaxMergeSegments); err != nil {
			return 0, err
		}
	}
	return seqNo, nil
}

func (w *IndexWriter) finishCommit() error {
	if w.pendingCommit == nil {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	committedSegmentsFileName, err := w.pendingCommit.FinishCommit(w.dir, w.config.GetCodec())
	if err != nil {
		return err
	}

	if err := w.deleter.Checkpoint(w.pendingCommit, true); err != nil {
		return err
	}
	w.segmentInfos.UpdateGeneration(w.pendingCommit)
	w.lastCommitChangeCount.Store(w.pendingCommitChangeCount)
	w.rollbackSegments = w.pendingCommit.CreateBackupSegmentInfos()

	if infoStream := w.config.GetInfoStream(); infoStream != nil && infoStream.IsEnabled("IW") {
		infoStream.Message("IW", fmt.Sprintf("commit: wrote segments file %q", committedSegmentsFileName))
	}

	w.pendingCommit = nil
	w.filesToCommit = nil
	return nil
}

func (w *IndexWriter) updatePendingMerges(policy MergePolicy) (*MergeSpecification, error) {
	return policy.FindFullFlushMerges(MergeTriggerFullFlush, w.segmentInfos, w)
}

func (w *IndexWriter) preparePointInTimeMerge(
	mergingSegmentInfos *SegmentInfos,
	stopCollectingMergeResults func() bool,
	trigger MergeTrigger,
	mergeFinished func(*SegmentCommitInfo),
) (MergeSpecification, error) {
	// In Java, this uses a wrapped MergePolicy to add custom hooks to OneMerge.
	// In Go, we can just find the merges and then add the hooks to each OneMerge.
	spec, err := w.updatePendingMerges(w.config.GetMergePolicy())
	if err != nil {
		return MergeSpecification{}, err
	}
	if spec == nil {
		return MergeSpecification{}, nil
	}

	for _, m := range spec.Merges {
		m.OnMergeFinished = func(merge *OneMerge, success bool, segmentDropped bool) error {
			if segmentDropped == false && success && stopCollectingMergeResults() == false {
				if trigger == MergeTriggerCommit {
					// If we do this in a getReader call here this is obsolete since we
					// already hold a reader that has incRef'd these files
					w.deleter.IncRefFiles(merge.Info.Files())
				}
				mergedSegmentNames := make(map[string]bool)
				for _, sci := range merge.Segments {
					mergedSegmentNames[sci.Info.Name()] = true
				}
				toCommitMergedAwaySegments := make([]*SegmentCommitInfo, 0)
				for sci := range mergingSegmentInfos.Iterator() {
					if mergedSegmentNames[sci.Info.Name()] {
						toCommitMergedAwaySegments = append(toCommitMergedAwaySegments, sci)
						if trigger == MergeTriggerCommit {
							if err := w.deleter.DecRefFiles(sci.Files()); err != nil {
								return err
							}
						}
					}
				}
				applicableMerge := NewOneMerge(toCommitMergedAwaySegments)
				applicableMerge.Info = merge.Info
				// The segment we merged into is not part of the commit point, so
				// the counter has to be advanced past its name.
				segmentCounter, err := strconv.ParseInt(strings.TrimPrefix(merge.Info.Info.Name(), "_"), 36, 64)
				if err != nil {
					return fmt.Errorf("index: cannot parse merged segment name %q: %w", merge.Info.Info.Name(), err)
				}
				if next := segmentCounter + 1; next > mergingSegmentInfos.GetCounter() {
					mergingSegmentInfos.SetCounter(next)
				}
				mergingSegmentInfos.ApplyMergeChanges(applicableMerge.Segments, applicableMerge.Info, false)
			}
			return nil
		}
		m.OnMergeComplete = func(merge *OneMerge) {
			if stopCollectingMergeResults() == false && !w.closed.Load() && merge.Info.SegmentInfo().DocCount() > 0 {
				mergeFinished(merge.Info)
			}
		}
	}
	return *spec, nil
}

func (w *IndexWriter) finishGetReaderMerge(
	stopCollectingMergedReaders *atomic.Bool,
	mergedReaders map[string]*SegmentReader,
	openedReadOnlyClones map[string]*SegmentReader,
	openingSegmentInfos *SegmentInfos,
	applyAllDeletes, writeAllDeletes bool,
	pointInTimeMerges MergeSpecification,
	maxCommitMergeWaitMillis int64,
) (*StandardDirectoryReader, error) {
	if err := w.mergeScheduler.Merge(w.mergeSource, MergeTriggerGetReader); err != nil {
		return nil, err
	}
	// Await the merges. This is a simplified version of pointInTimeMerges.await().
	for _, m := range pointInTimeMerges.Merges {
		<-m.Done()
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	stopCollectingMergedReaders.Store(true)

	reader, err := w.maybeReopenMergedNRTReader(
		mergedReaders,
		openedReadOnlyClones,
		openingSegmentInfos,
		applyAllDeletes,
		writeAllDeletes,
	)
	for _, sr := range mergedReaders {
		if closeErr := sr.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	clear(mergedReaders)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

func (w *IndexWriter) maybeReopenMergedNRTReader(
	mergedReaders map[string]*SegmentReader,
	openedReadOnlyClones map[string]*SegmentReader,
	openingSegmentInfos *SegmentInfos,
	applyAllDeletes, writeAllDeletes bool,
) (*StandardDirectoryReader, error) {
	if len(mergedReaders) == 0 {
		return nil, nil
	}

	files := make([]string, 0)
	readerFactory := func(sci *SegmentCommitInfo) (*SegmentReader, error) {
		name := sci.Info.Name()
		if sr, ok := mergedReaders[name]; ok {
			delete(mergedReaders, name)
			files = append(files, sr.GetSegmentCommitInfo().Files()...)
			return sr, nil
		}
		if sr, ok := openedReadOnlyClones[name]; ok {
			delete(openedReadOnlyClones, name)
			if err := sr.IncRef(); err != nil {
				return nil, err
			}
			return sr, nil
		}
		rld := w.getPooledInstance(sci, true)
		if rld == nil {
			return nil, fmt.Errorf("index: no pooled reader for segment %s", name)
		}
		return rld.GetReader()
	}

	r, err := OpenNRT(w, readerFactory, openingSegmentInfos, applyAllDeletes, writeAllDeletes)
	if err != nil {
		return nil, err
	}
	if err := w.deleter.DecRefFiles(files); err != nil {
		return nil, err
	}
	return r, nil
}

func (w *IndexWriter) GetConfig() *IndexWriterConfig {
	return w.config
}

func (w *IndexWriter) GetDirectory() store.Directory {
	return w.dirOrig
}

func (w *IndexWriter) IsClosed() bool {
	return w.closed.Load()
}

// HasDeletions reports whether there are any deletions in the index, either as
// part of the on-disk segments or as pending in-memory updates.
//
// This is the Go port of Lucene's org.apache.lucene.index.IndexWriter#hasDeletions.
func (w *IndexWriter) HasDeletions() (bool, error) {
	if err := w.ensureOpen(true); err != nil {
		return false, err
	}
	if w.bufferedUpdatesStream.Any() || w.docWriter.AnyDeletions() {
		return true, nil
	}
	for info := range w.segmentInfos.Iterator() {
		if info.HasDeletions() {
			return true, nil
		}
		// ReaderPool.anyDeletions(): a pooled reader may already hold deletes
		// that have not been written back to the segment.
		if rld := w.getPooledInstance(info, false); rld != nil {
			delCount := rld.GetDelCount()
			w.release(rld)
			if delCount > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (w *IndexWriter) GetDocWriterThreadPoolSize() int {
	return w.docWriter.perThreadPool.Size()
}

func (w *IndexWriter) GetSegmentCount() int {
	return w.segmentInfos.Size()
}

func (w *IndexWriter) adjustPendingNumDocs(delta int) {
	w.pendingNumDocs.Add(int64(delta))
}

// FlushNextBuffer flushes the largest in-memory buffer to disk and reports
// whether a buffer was flushed.
//
// Mirrors org.apache.lucene.index.IndexWriter#flushNextBuffer.
func (w *IndexWriter) FlushNextBuffer() (bool, error) {
	return w.docWriter.FlushOneDWPT()
}

func (w *IndexWriter) Close() error {
	if !w.closing.Swap(true) {
		return nil
	}

	if w.config.GetCommitOnClose() {
		if _, err := w.Commit(); err != nil {
			return err
		}
	}

	w.closed.Store(true)
	return nil
}

// Rollback reverts the index to the last committed state.
func (w *IndexWriter) Rollback() error {
	if err := w.ensureOpen(true); err != nil {
		return err
	}
	w.commitLock.Lock()
	defer w.commitLock.Unlock()
	return w.rollbackInternal()
}

func (w *IndexWriter) rollbackInternal() error {
	return w.rollbackInternalNoCommit()
}

func (w *IndexWriter) rollbackInternalNoCommit() error {
	w.fullFlushLock.Lock()
	defer w.fullFlushLock.Unlock()

	// 1. Abort all merges
	w.mergeScheduler.AbortAll()

	// 2. Close merge scheduler
	if err := w.mergeScheduler.Close(); err != nil {
		return fmt.Errorf("failed to close merge scheduler during rollback: %w", err)
	}

	// 3. Close doc writer
	if err := w.docWriter.Close(); err != nil {
		return fmt.Errorf("failed to close doc writer during rollback: %w", err)
	}

	// 4. Abort doc writer
	if err := w.docWriter.Abort(); err != nil {
		return fmt.Errorf("failed to abort doc writer during rollback: %w", err)
	}

	// 5. Wait for flushes and publish segments
	w.docWriter.flushControl.WaitForFlush()
	if err := w.publishFlushedSegments(true); err != nil {
		return fmt.Errorf("failed to publish flushed segments during rollback: %w", err)
	}

	// 6. Close event queue
	if err := w.eventQueue.close(); err != nil {
		return fmt.Errorf("failed to close event queue during rollback: %w", err)
	}

	// 7. Roll back pending commit
	if w.pendingCommit != nil {
		if err := w.pendingCommit.RollbackCommit(w.dir); err != nil {
			return fmt.Errorf("failed to rollback pending commit: %w", err)
		}
		if err := w.deleter.DecRef(w.pendingCommit); err != nil {
			return fmt.Errorf("failed to release the pending commit during rollback: %w", err)
		}
		w.pendingCommit = nil
	}

	// 8. Roll back segment infos
	totalMaxDoc := w.segmentInfos.TotalMaxDoc()
	w.segmentInfos.RollbackSegmentInfos(w.rollbackSegments)
	rollbackMaxDoc := w.segmentInfos.TotalMaxDoc()

	// 9. Adjust pending num docs
	w.adjustPendingNumDocs(-(totalMaxDoc - rollbackMaxDoc))

	// 10. Cleanup unreferenced files
	if err := w.deleter.Checkpoint(w.segmentInfos, false); err != nil {
		return fmt.Errorf("failed to checkpoint deleter during rollback: %w", err)
	}
	if err := w.deleter.Refresh(); err != nil {
		return fmt.Errorf("failed to refresh deleter during rollback: %w", err)
	}
	if err := w.deleter.Close(); err != nil {
		return fmt.Errorf("failed to close deleter during rollback: %w", err)
	}

	// 11. Close reader pool
	if w.readerPool != nil {
		w.readerPool.Clear()
	}

	// 12. Finalize writer state
	w.lastCommitChangeCount.Store(w.changeCount.Load())
	w.closed.Store(true)

	// 13. Release write lock
	if w.writeLock != nil {
		if err := w.writeLock.Close(); err != nil {
			return fmt.Errorf("failed to release the write lock during rollback: %w", err)
		}
		w.writeLock = nil
	}

	return nil
}

// maybeProcessEvents drains the event queue when the sequence number reports
// that events are pending, and normalises the sign of the sequence number.
//
// Mirrors org.apache.lucene.index.IndexWriter#maybeProcessEvents.
func (w *IndexWriter) maybeProcessEvents(seqNo int64) (int64, error) {
	if seqNo < 0 {
		seqNo = -seqNo
		if err := w.processEvents(true); err != nil {
			return seqNo, err
		}
	}
	return seqNo, nil
}

// processEvents drains the event queue and, when triggerMerge is set, asks the
// merge policy for segment-flush merges.
//
// Mirrors org.apache.lucene.index.IndexWriter#processEvents.
func (w *IndexWriter) processEvents(triggerMerge bool) error {
	if w.tragedy.Load() == nil {
		if err := w.eventQueue.processEvents(w); err != nil {
			return err
		}
	}
	if triggerMerge {
		return w.maybeMerge(w.config.GetMergePolicy(), MergeTriggerSegmentFlush, unboundedMaxMergeSegments)
	}
	return nil
}

func (w *IndexWriter) publishFlushedSegments(forced bool) error {
	return w.docWriter.PurgeFlushTickets(
		forced,
		func(ticket *FlushTicket) error {
			ticket.MarkPublished()
			newSegment := ticket.GetFlushedSegment()
			bufferedUpdates := ticket.GetFrozenUpdates()

			if newSegment == nil {
				if bufferedUpdates != nil && bufferedUpdates.Any() {
					_, err := w.publishFrozenUpdates(bufferedUpdates)
					return err
				}
				return nil
			}
			return w.publishFlushedSegment(
				newSegment.SegmentInfo,
				newSegment.FieldInfos,
				newSegment.SegmentUpdates,
				bufferedUpdates,
				newSegment.SortMap,
			)
		},
	)
}

func (w *IndexWriter) applyAllDeletesAndUpdates() error {
	return w.bufferedUpdatesStream.WaitApplyAll(context.Background(), w)
}

// writeReaderPool persists the pooled readers' pending state. When writeDeletes
// is set the live docs are committed too; otherwise only the doc-values updates
// are written. Segments that turn out to be fully deleted are dropped.
//
// Mirrors org.apache.lucene.index.IndexWriter#writeReaderPool.
func (w *IndexWriter) writeReaderPool(writeDeletes bool) error {
	if err := w.readerPool.WriteReaderPool(writeDeletes); err != nil {
		return err
	}
	if writeDeletes {
		// The state only moved to disk; SegmentInfos.version must not advance.
		if err := w.checkpointNoSIS(); err != nil {
			return err
		}
	} else if err := w.checkpoint(); err != nil {
		return err
	}

	// Now do some best effort to check whether a segment is fully deleted.
	var toDrop []*SegmentCommitInfo
	for info := range w.segmentInfos.Iterator() {
		rld := w.getPooledInstance(info, false)
		if rld == nil {
			continue
		}
		fullyDeleted, err := w.isFullyDeleted(rld)
		w.release(rld)
		if err != nil {
			return err
		}
		if fullyDeleted {
			toDrop = append(toDrop, info)
		}
	}
	for _, info := range toDrop {
		w.dropDeletedSegment(info)
	}
	if len(toDrop) != 0 {
		return w.checkpoint()
	}
	return nil
}

// checkpoint records a change to the segments and asks the deleter to refresh
// its reference counts.
//
// Mirrors org.apache.lucene.index.IndexWriter#checkpoint.
func (w *IndexWriter) checkpoint() error {
	w.changed()
	return w.deleter.Checkpoint(w.segmentInfos, false)
}

// checkpointNoSIS is checkpoint without advancing SegmentInfos.version, used
// when the only change was moving already-recorded state to disk.
//
// Mirrors org.apache.lucene.index.IndexWriter#checkpointNoSIS.
func (w *IndexWriter) checkpointNoSIS() error {
	w.changeCount.Add(1)
	return w.deleter.Checkpoint(w.segmentInfos, false)
}

func (w *IndexWriter) maybeMerge(policy MergePolicy, trigger MergeTrigger, maxSegments int) error {
	if policy == nil {
		return nil
	}
	if err := w.ensureOpen(false); err != nil {
		return err
	}

	// Consult the merge policy to find potential merges for the given trigger.
	spec, err := policy.FindMerges(trigger, w.segmentInfos, w)
	if err != nil {
		return err
	}

	if spec != nil {
		// Submit the found merges to the scheduler.
		return w.mergeScheduler.MergeWithSpec(w.mergeSource, spec, false)
	}
	return nil
}

// getFieldNumberMap builds the index-wide field-number registry from the
// FieldInfos of every committed segment.
//
// Mirrors org.apache.lucene.index.IndexWriter#getFieldNumberMap.
func (w *IndexWriter) getFieldNumberMap() *FieldNumbers {
	fnm := NewFieldNumbers(w.config.GetSoftDeletesField(), w.config.GetParentField())
	for sci := range w.segmentInfos.Iterator() {
		fis, err := readFieldInfos(sci)
		if err != nil {
			continue
		}
		for it := fis.Iterator(); it.HasNext(); {
			fi := it.Next()
			if fi == nil {
				break
			}
			fnm.AddOrGet(fi)
		}
	}
	return fnm
}

// readFieldInfos reads the latest FieldInfos for a commit. It is used on
// IndexWriter init and by AddIndexes to create or update the global field map.
//
// Mirrors org.apache.lucene.index.IndexWriter#readFieldInfos.
func readFieldInfos(si *SegmentCommitInfo) (*FieldInfos, error) {
	codec := si.SegmentInfo().Codec()
	if codec == nil {
		return nil, fmt.Errorf("index: segment %s has no codec", si.SegmentInfo().Name())
	}
	reader := codec.FieldInfosFormat()
	if si.HasFieldUpdates() {
		// There are updates, so read the latest, always outside of the CFS.
		suffix := strconv.FormatInt(si.FieldInfosGen(), 36)
		return reader.Read(si.SegmentInfo().Directory(), si.SegmentInfo(), suffix, store.IOContextReadOnce)
	} else if si.SegmentInfo().IsCompoundFile() {
		// Cannot read from the CFS directly, so open the compound view first.
		cfs, err := codec.CompoundFormat().GetCompoundReader(si.SegmentInfo().Directory(), si.SegmentInfo())
		if err != nil {
			return nil, err
		}
		infos, readErr := reader.Read(cfs, si.SegmentInfo(), "", store.IOContextReadOnce)
		closeErr := cfs.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return infos, nil
	}
	return reader.Read(si.SegmentInfo().Directory(), si.SegmentInfo(), "", store.IOContextReadOnce)
}

type mapEntry[K, V any] struct {
	Key   K
	Value V
}

// indexWriterFlushNotifications is the DocumentsWriter.FlushNotifications an
// IndexWriter hands to its DocumentsWriter. Mirrors the anonymous
// DocumentsWriter.FlushNotifications instance held by
// org.apache.lucene.index.IndexWriter.
type indexWriterFlushNotifications struct {
	writer *IndexWriter
}

// DeleteUnusedFiles queues the deletion of files that are no longer used.
func (n *indexWriterFlushNotifications) DeleteUnusedFiles(files []string) {
	n.writer.eventQueue.add(func(iw *IndexWriter) error {
		return iw.deleteNewFiles(files)
	})
}

// FlushFailed queues the cleanup of the files a failed flush left behind.
func (n *indexWriterFlushNotifications) FlushFailed(info *SegmentInfo) {
	n.writer.eventQueue.add(func(iw *IndexWriter) error {
		return iw.flushFailed(info)
	})
}

// AfterSegmentsFlushed publishes the segments a flush produced.
func (n *indexWriterFlushNotifications) AfterSegmentsFlushed() error {
	return n.writer.publishFlushedSegments(false)
}

// OnTragicEvent records a tragic exception raised inside the DocumentsWriter.
func (n *indexWriterFlushNotifications) OnTragicEvent(event error, message string) {
	n.writer.OnTragicEvent(event, message)
}

// OnDeletesApplied publishes flushed segments once deletes have been applied.
func (n *indexWriterFlushNotifications) OnDeletesApplied() {
	n.writer.eventQueue.add(func(iw *IndexWriter) error {
		defer iw.flushCount.Add(1)
		return iw.publishFlushedSegments(true)
	})
}

// OnTicketBacklog publishes flushed segments when the ticket queue backs up.
func (n *indexWriterFlushNotifications) OnTicketBacklog() {
	n.writer.eventQueue.add(func(iw *IndexWriter) error {
		return iw.publishFlushedSegments(true)
	})
}

func (w *IndexWriter) flushNotifications() FlushNotifications {
	return &indexWriterFlushNotifications{writer: w}
}

func (w *IndexWriter) deleteNewFiles(files []string) error {
	return w.deleter.DeleteNewFiles(files)
}

func (w *IndexWriter) flushFailed(info *SegmentInfo) error {
	files := info.Files()
	if files != nil {
		return w.deleter.DeleteNewFiles(files)
	}
	return nil
}

func SetDiagnostics(info *SegmentInfo, source int) {
	// Diagnostics are used for internal tracking and do not affect binary compatibility.
}

func (w *IndexWriter) publishFlushedSegment(
	newSegment *SegmentCommitInfo,
	fieldInfos *FieldInfos,
	packet *FrozenBufferedUpdates,
	globalPacket *FrozenBufferedUpdates,
	sortMap SorterDocMap,
) error {
	if err := w.ensureOpen(true); err != nil {
		return err
	}

	if globalPacket != nil && globalPacket.Any() {
		if _, err := w.publishFrozenUpdates(globalPacket); err != nil {
			return err
		}
	}

	var nextGen int64
	if packet != nil && packet.Any() {
		var err error
		nextGen, err = w.publishFrozenUpdates(packet)
		if err != nil {
			return err
		}
	} else {
		// Since we don't have a delete packet to apply we can get a new
		// generation right away.
		nextGen = w.bufferedUpdatesStream.NextGen()
		// No deletes to apply, so mark this segment as finished.
		w.bufferedUpdatesStream.FinishedSegment(nextGen)
	}

	newSegment.SetBufferedDeletesGen(nextGen)
	w.segmentInfos.Add(newSegment)
	if err := w.checkpoint(); err != nil {
		return err
	}

	if packet != nil && packet.Any() && sortMap != nil {
		rau := w.getPooledInstance(newSegment, true)
		if rau != nil {
			rau.SetSortMap(sortMap)
			w.release(rau)
		}
	}

	softDeletesField := w.config.GetSoftDeletesField()
	var fieldInfo *FieldInfo
	if softDeletesField != "" {
		fieldInfo = fieldInfos.FieldInfoByName(softDeletesField)
	}

	hasInitialSoftDeleted := false
	if fieldInfo != nil && fieldInfo.DocValuesGen() == -1 && fieldInfo.DocValuesType() != DocValuesTypeNone {
		hasInitialSoftDeleted = true
	}
	isFullyHardDeleted := newSegment.DelCount() == newSegment.SegmentInfo().DocCount()

	if hasInitialSoftDeleted || isFullyHardDeleted {
		rau := w.getPooledInstance(newSegment, true)
		if rau != nil {
			deleted, err := w.isFullyDeleted(rau)
			if err == nil && deleted {
				w.dropDeletedSegment(newSegment)
				if cpErr := w.checkpoint(); cpErr != nil {
					w.release(rau)
					return cpErr
				}
			}
			w.release(rau)
			if err != nil {
				return err
			}
		}
	}

	w.flushCount.Add(1)
	w.doAfterFlush()
	return nil
}

func (w *IndexWriter) publishFrozenUpdates(packet *FrozenBufferedUpdates) (int64, error) {
	nextGen, err := w.bufferedUpdatesStream.Push(packet)
	if err != nil {
		return 0, err
	}
	w.eventQueue.add(func(iw *IndexWriter) error {
		defer iw.flushDeletesCount.Add(1)
		if _, err := iw.TryApply(context.Background(), packet); err != nil {
			iw.OnTragicEvent(err, "applyUpdatesPacket")
			return err
		}
		return nil
	})
	return nextGen, nil
}

func (w *IndexWriter) doAfterFlush() {}

// Merge executes a single merge operation.
func (w *IndexWriter) Merge(merge *OneMerge) error {
	if err := w.ensureOpen(true); err != nil {
		return err
	}

	mergeErr := w.mergeInternal(merge)
	if mergeErr != nil {
		w.handleMergeException(mergeErr, merge)
	}

	// The readers are already closed by commitMerge when no error was hit.
	w.commitLock.Lock()
	closeErr := merge.Close(mergeErr == nil, false, func(mr *MergeReader) error { return nil })
	w.commitLock.Unlock()
	w.mergeFinish(merge)

	if mergeErr != nil {
		return mergeErr
	}
	return closeErr
}

func (w *IndexWriter) ForceMerge(maxNumSegments int) error {
	_, err := w.ForceMergeWithObserver(maxNumSegments, true)
	return err
}

func (w *IndexWriter) ForceMergeWithObserver(maxNumSegments int, doWait bool) (*MergeObserver, error) {
	if err := w.ensureOpen(true); err != nil {
		return nil, err
	}
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	// Mirrors IndexWriter#forceMerge: every segment present now is recorded as
	// an original to-be-merged segment before the policy is consulted.
	w.segmentsToMerge = make(map[*SegmentCommitInfo]bool, w.segmentInfos.Size())
	for info := range w.segmentInfos.Iterator() {
		w.segmentsToMerge[info] = true
	}
	w.mergeMaxNumSegments = maxNumSegments

	spec, err := w.config.GetMergePolicy().FindForcedMerges(w.segmentInfos, maxNumSegments, w.segmentsToMerge, w)
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return NewMergeObserver(nil), nil
	}

	if err := w.mergeScheduler.MergeWithSpec(w.mergeSource, spec, doWait); err != nil {
		return nil, err
	}

	return NewMergeObserver(spec), nil
}

func (w *IndexWriter) ForceMergeDeletes() error {
	_, err := w.ForceMergeDeletesWithObserver(true)
	return err
}

// ForceMergeDeletesWithObserver executes a merge to expunge all deletes from the index.
// Returns a MergeObserver to monitor progress.
func (w *IndexWriter) ForceMergeDeletesWithObserver(doWait bool) (*MergeObserver, error) {
	if err := w.ensureOpen(true); err != nil {
		return nil, err
	}
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	spec, err := w.config.GetMergePolicy().FindForcedDeletesMerges(w.segmentInfos, w)
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return NewMergeObserver(nil), nil
	}

	// register merges with the scheduler
	if err := w.mergeScheduler.MergeWithSpec(w.mergeSource, spec, doWait); err != nil {
		return nil, err
	}

	return NewMergeObserver(spec), nil
}

// mergeFinish does the bookkeeping a finished merge requires: it releases the
// source segments from the merging set and drops the merge from the running
// set.
//
// Mirrors org.apache.lucene.index.IndexWriter#mergeFinish.
func (w *IndexWriter) mergeFinish(merge *OneMerge) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// It is possible we are called twice, e.g. if mergeInit raised an error.
	if merge.RegisterDone {
		for _, info := range merge.Segments {
			delete(w.mergingSegments, info)
		}
		merge.RegisterDone = false
	}
	delete(w.runningMerges, merge)
}

// getNextMerge advances the next pending merge to the running set and returns
// it, or nil when no merge is pending.
//
// Mirrors org.apache.lucene.index.IndexWriter#getNextMerge.
func (w *IndexWriter) getNextMerge() *OneMerge {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.pendingMerges) == 0 {
		return nil
	}
	merge := w.pendingMerges[0]
	w.pendingMerges = w.pendingMerges[1:]
	w.runningMerges[merge] = struct{}{}
	return merge
}

// hasPendingMerges reports whether merges are waiting to be scheduled.
//
// Mirrors org.apache.lucene.index.IndexWriter#hasPendingMerges.
func (w *IndexWriter) hasPendingMerges() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.pendingMerges) != 0
}

// segString returns a readable description of the writer's current segments.
//
// Mirrors org.apache.lucene.index.IndexWriter#segString().
func (w *IndexWriter) segString() string {
	return w.segmentInfos.String()
}

// GetInfoStream returns the info stream this writer logs to. It is part of the
// MergePolicy.MergeContext contract IndexWriter implements.
//
// Mirrors org.apache.lucene.index.IndexWriter#getInfoStream.
func (w *IndexWriter) GetInfoStream() InfoStream {
	return w.config.GetInfoStream()
}

// GetMergingSegments returns the segments currently being merged. The returned
// map is not cloned, so it is only safe to read while IndexWriter's lock is
// held, which is the case when IndexWriter invokes the MergePolicy.
//
// Mirrors org.apache.lucene.index.IndexWriter#getMergingSegments.
func (w *IndexWriter) GetMergingSegments() map[*SegmentCommitInfo]bool {
	return w.mergingSegments
}

// NumDeletesToMerge returns the number of deletes a merge would claim back if
// info were merged. It prefers the pooled reader's view, falling back to the
// hard delete count recorded on the segment.
//
// Mirrors org.apache.lucene.index.IndexWriter#numDeletesToMerge.
func (w *IndexWriter) NumDeletesToMerge(info *SegmentCommitInfo) int {
	mergePolicy := w.config.GetMergePolicy()
	rld := w.getPooledInstance(info, false)
	if rld == nil {
		// Without a pooled instance the hard deletes are the safe answer.
		return info.GetDelCount()
	}
	defer w.release(rld)
	numDeletesToMerge, err := rld.NumDeletesToMerge(mergePolicy)
	if err != nil {
		return info.GetDelCount()
	}
	return numDeletesToMerge
}

// NumDeletedDocs returns the number of deleted documents for a pooled reader,
// falling back to the segment's own delete count when the reader is not
// pooled.
//
// Mirrors org.apache.lucene.index.IndexWriter#numDeletedDocs.
func (w *IndexWriter) NumDeletedDocs(info *SegmentCommitInfo) int {
	rld := w.getPooledInstance(info, false)
	if rld == nil {
		delCount := info.GetDelCount()
		if w.softDeletesEnabled {
			delCount += info.GetSoftDelCount()
		}
		return delCount
	}
	defer w.release(rld)
	// Take the full count from the reader, since the SegmentCommitInfo may
	// change concurrently.
	return rld.GetDelCount()
}

func (w *IndexWriter) handleMergeException(err error, merge *OneMerge) {
	merge.SetException(err)
	w.mergeExceptions = append(w.mergeExceptions, err)
}

func (w *IndexWriter) mergeInternal(merge *OneMerge) error {
	merge.InitMerge()
	if err := merge.CheckAborted(); err != nil {
		return err
	}

	mergeDir := w.mergeScheduler.WrapForMerge(merge, w.dir)
	mergeContext := store.IOContextMerge(merge.GetStoreMergeInfo())
	dirWrapper := store.NewTrackingDirectoryWrapper(mergeDir)

	//- Setup Readers
	readers := make([]CodecReader, 0, len(merge.GetMergeReader()))
	for _, mr := range merge.GetMergeReader() {
		readers = append(readers, merge.WrapForMerge(mr.Reader))
	}

	//- SegmentMerger
	codec := w.liveConfig.GetCodec()
	merger, err := NewSegmentMerger(
		readers,
		merge.Info.SegmentInfo(),
		codec,
		w.liveConfig.GetInfoStream(),
		dirWrapper,
		mergeContext,
	)
	if err != nil {
		return err
	}

	if !merger.ShouldMerge() {
		return nil
	}

	if err := merge.CheckAborted(); err != nil {
		return err
	}
	w.mu.Lock()
	w.runningMerges[merge] = struct{}{}
	w.mu.Unlock()
	merge.MergeStartNS = time.Now().UnixNano()

	_, mergeErr := merger.Merge()
	w.mu.Lock()
	delete(w.runningMerges, merge)
	w.mu.Unlock()

	if mergeErr != nil {
		return mergeErr
	}

	mergedInfo := spi.NewSegmentCommitInfo(merge.Info.SegmentInfo(), 0, -1)
	mergedInfo.SetSoftDelCount(0)
	mergedInfo.SetFieldInfosGen(-1)
	mergedInfo.SetDocValuesGen(-1)
	mergedInfo.SetID(util.RandomId())
	merge.SetMergeInfo(mergedInfo)
	merge.GetMergeInfo().SegmentInfo().SetFiles(dirWrapper.GetCreatedFileNames())
	dirWrapper.Clear()

	// Very important to do this before opening the reader, because the codec
	// must know whether prox was written for this segment.
	if err := codec.FieldInfosFormat().Write(
		w.dir, merge.Info.SegmentInfo(), "", merger.MergeState.MergeFieldInfos, mergeContext,
	); err != nil {
		return err
	}

	// --- WARMING ---
	warmer := w.liveConfig.GetMergedSegmentWarmer()
	if w.liveConfig.GetReaderPooling() && warmer != nil {
		rau := w.getPooledInstance(merge.Info, true)
		if rau != nil {
			sr, readerErr := rau.GetReader()
			if readerErr == nil {
				warmer.Warm(sr)
				if releaseErr := rau.Release(sr); releaseErr != nil {
					w.release(rau)
					return releaseErr
				}
			}
			w.release(rau)
			if readerErr != nil {
				return readerErr
			}
		}
	}
	// ----------------

	if !w.commitMerge(merge, nil) {
		return fmt.Errorf("index: merge aborted")
	}

	return nil
}

// dropDeletedSegment removes a fully deleted segment from the live segment
// list.
//
// Mirrors org.apache.lucene.index.IndexWriter#dropDeletedSegment.
func (w *IndexWriter) dropDeletedSegment(sci *SegmentCommitInfo) {
	if idx := w.segmentInfos.IndexOf(sci); idx >= 0 {
		w.segmentInfos.Remove(idx)
	}
}

func (w *IndexWriter) isFullyDeleted(rau *ReadersAndUpdates) (bool, error) {
	return rau.IsFullyDeleted()
}

// TryApply attempts to apply the packet without blocking: it returns false
// when another goroutine already owns the packet's apply lock.
//
// Mirrors org.apache.lucene.index.FrozenBufferedUpdates#tryApply, whose role
// the Gocene PacketApplier moves onto IndexWriter.
func (w *IndexWriter) TryApply(ctx context.Context, packet *FrozenBufferedUpdates) (bool, error) {
	if !packet.TryLock() {
		return false, nil
	}
	defer packet.Unlock()
	if err := w.applyPacketLocked(ctx, packet); err != nil {
		return false, err
	}
	return true, nil
}

// ForceApply blocks until the packet's apply lock is free and then applies it.
//
// Mirrors org.apache.lucene.index.FrozenBufferedUpdates#forceApply.
func (w *IndexWriter) ForceApply(ctx context.Context, packet *FrozenBufferedUpdates) error {
	packet.Lock()
	defer packet.Unlock()
	return w.applyPacketLocked(ctx, packet)
}

// applyPacketLocked resolves one frozen packet against every live segment. The
// caller must hold the packet's apply lock.
func (w *IndexWriter) applyPacketLocked(ctx context.Context, packet *FrozenBufferedUpdates) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	states := make([]*FrozenSegmentState, 0, w.segmentInfos.Size())
	// We need to keep track of the RAUs we acquired so we can release them
	// after Apply is finished.
	// Note: the SegmentReader inside states also holds a ref to the RAU.
	// ReaderPool.Get() increments the RAU ref count.
	// We must release them to avoid leaks.
	//
	// In Lucene, the ReaderPool handles this.
	// Here, we manually track and release.
	//
	// Wait: if we release them immediately, we might break the Reader used by Apply.
	// Actually, we should hold them until Apply returns.
	var raus []*ReadersAndUpdates

	for sci := range w.segmentInfos.Iterator() {
		rau := w.getPooledInstance(sci, true)
		if rau == nil {
			continue
		}
		raus = append(raus, rau)

		sr, err := rau.GetReader()
		if err != nil {
			w.release(rau)
			continue
		}

		states = append(states, &FrozenSegmentState{
			Reader:   sr,
			RAU:      rau,
			DelGen:   sci.GetBufferedDeletesGen(),
			RefCount: int(rau.RefCount()),
		})
	}

	defer func() {
		for _, rau := range raus {
			w.release(rau)
		}
	}()

	_, err := packet.Apply(states)
	return err
}

func (w *IndexWriter) getPooledInstance(sci *SegmentCommitInfo, create bool) *ReadersAndUpdates {
	return w.readerPool.Get(sci, create, func(info *SegmentCommitInfo) *ReadersAndUpdates {
		rau, err := NewReadersAndUpdates(w.config.GetIndexCreatedVersionMajor(), info, NewPendingDeletes(info, nil, info.HasDeletions() == false))
		if err != nil {
			panic(err)
		}
		return rau
	})
}
func (w *IndexWriter) release(rau *ReadersAndUpdates) {
	w.readerPool.Release(rau, true)
}
func (w *IndexWriter) commitMerge(merge *OneMerge, docMaps []DocMap) bool {
	if merge.IsAborted() {
		return false
	}

	w.segmentInfos.ApplyMergeChanges(merge.Segments, merge.Info, false)

	// Adjust pendingNumDocs
	delDocCount := merge.TotalMaxDoc - merge.Info.SegmentInfo().DocCount()
	w.adjustPendingNumDocs(-delDocCount)

	return true
}

// ApplyAllDeletesAndUpdates applies all buffered deletes and updates to all segments.
// This is called between flushAllThreads() and finishFullFlush() during GetReader().
// It updates the live docs bitsets (.liv) for each segment to reflect pending deletes.
// Ref: Lucene IndexWriter:591.
func (w *IndexWriter) ApplyAllDeletesAndUpdates() error {
	if w.bufferedUpdatesStream == nil {
		return nil
	}

	w.flushDeletesCount.Add(1)
	return w.bufferedUpdatesStream.WaitApplyAll(context.Background(), w)
}

// GetReader returns a DirectoryReader that includes uncommitted (flushed but not committed)
// segments, providing Near-Real-Time (NRT) search capability.
// This implements the core NRT reader functionality from Lucene's IndexWriter:509-750.
// Ref: Lucene IndexWriter.getReader(applyAllDeletes, writeAllDeletes).
func (w *IndexWriter) GetReader(applyAllDeletes, writeAllDeletes bool) (*StandardDirectoryReader, error) {
	w.fullFlushLock.Lock()
	defer w.fullFlushLock.Unlock()

	if w.closed.Load() {
		return nil, fmt.Errorf("index is closed")
	}

	w.readerPool.EnableReaderPooling()

	seqNo, err := w.doFlush(applyAllDeletes)
	if err != nil {
		return nil, err
	}
	w.publishedSeqNo = seqNo

	if writeAllDeletes && !applyAllDeletes {
		if err := w.writeReaderPool(true); err != nil {
			return nil, err
		}
	}

	stopCollectingMergedReaders := &atomic.Bool{}
	mergedReaders := make(map[string]*SegmentReader)
	openedReadOnlyClones := make(map[string]*SegmentReader)
	openingSegmentInfos := w.segmentInfos.Clone()

	pointInTimeMerges, err := w.preparePointInTimeMerge(
		openingSegmentInfos,
		func() bool { return stopCollectingMergedReaders.Load() },
		MergeTriggerGetReader,
		func(sci *SegmentCommitInfo) {
			rau := w.getPooledInstance(sci, true)
			if rau == nil {
				return
			}
			sr, err := rau.GetReader()
			if err != nil {
				w.release(rau)
				return
			}
			mergedReaders[sci.Info.Name()] = sr
		},
	)
	if err != nil {
		return nil, err
	}

	if len(pointInTimeMerges.Merges) > 0 {
		reader, err := w.finishGetReaderMerge(
			stopCollectingMergedReaders,
			mergedReaders,
			openedReadOnlyClones,
			openingSegmentInfos,
			applyAllDeletes,
			writeAllDeletes,
			pointInTimeMerges,
			w.liveConfig.GetMaxFullFlushMergeWaitMillis(),
		)
		if err != nil {
			return nil, err
		}
		if reader == nil {
			// Fall back to normal NRT open if merge-based reopen failed
			return w.openNRTReader(openingSegmentInfos, applyAllDeletes, writeAllDeletes)
		}
		return reader, nil
	}

	return w.openNRTReader(openingSegmentInfos, applyAllDeletes, writeAllDeletes)
}

func (w *IndexWriter) openNRTReader(sis *SegmentInfos, applyAllDeletes, writeAllDeletes bool) (*StandardDirectoryReader, error) {
	readerFactory := func(sci *SegmentCommitInfo) (*SegmentReader, error) {
		rld := w.getPooledInstance(sci, true)
		if rld == nil {
			return nil, fmt.Errorf("index: no pooled reader for segment %s", sci.Info.Name())
		}
		return rld.GetReader()
	}

	reader, err := OpenNRT(w, readerFactory, sis, applyAllDeletes, writeAllDeletes)
	if err != nil {
		return nil, fmt.Errorf("failed to open reader: %w", err)
	}

	return reader, nil
}

// MaxDocs is the hard upper bound on the number of documents a single index
// may hold.
//
// Mirrors org.apache.lucene.index.IndexWriter.MAX_DOCS.
const MaxDocs = math.MaxInt32 - 128

// MaxPosition is the maximum value of a token position in an indexed field.
//
// Mirrors org.apache.lucene.index.IndexWriter.MAX_POSITION.
const MaxPosition = math.MaxInt32 - 128

// actualMaxDocs enforces the document limit through a package-private variable
// so that tests can lower it, exactly as IndexWriter.actualMaxDocs does.
var actualMaxDocs = MaxDocs

// SetMaxDocs lowers the enforced maximum document count. Used only for testing.
//
// Mirrors org.apache.lucene.index.IndexWriter.setMaxDocs.
func SetMaxDocs(maxDocs int) error {
	if maxDocs > MaxDocs {
		// Cannot go higher than the hard max:
		return fmt.Errorf("maxDocs must be <= IndexWriter.MAX_DOCS=%d; got: %d", MaxDocs, maxDocs)
	}
	actualMaxDocs = maxDocs
	return nil
}

// GetActualMaxDocs returns the currently enforced maximum document count.
//
// Mirrors org.apache.lucene.index.IndexWriter.getActualMaxDocs.
func GetActualMaxDocs() int {
	return actualMaxDocs
}

// AddIndexes adds all segments from the given directories into this index.
//
// This may be used to parallelise batch indexing: a large document collection
// is broken into sub-collections, each indexed independently, and the complete
// index is then assembled by merging the sub-collection indexes with this
// method.
//
// NOTE: this method acquires the write lock in each directory, so that no other
// IndexWriter is open, or can be opened, while it runs.
//
// Mirrors org.apache.lucene.index.IndexWriter#addIndexes(Directory...).
func (w *IndexWriter) AddIndexes(dirs ...store.Directory) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}

	if err := w.noDupDirs(dirs...); err != nil {
		return 0, err
	}

	locks, err := w.acquireWriteLocks(dirs...)
	if err != nil {
		return 0, err
	}

	seqNo, addErr := w.addIndexesLocked(dirs...)

	for _, lock := range locks {
		if closeErr := lock.Close(); closeErr != nil && addErr == nil {
			addErr = closeErr
		}
	}
	if addErr != nil {
		return 0, addErr
	}

	if err := w.maybeMerge(w.config.GetMergePolicy(), MergeTriggerExplicit, unboundedMaxMergeSegments); err != nil {
		return 0, err
	}
	return seqNo, nil
}

// addIndexesLocked is the body of AddIndexes, run while the write lock of every
// source directory is held.
func (w *IndexWriter) addIndexesLocked(dirs ...store.Directory) (int64, error) {
	indexSort, _ := w.config.GetIndexSort().(*spi.Sort)

	infoStream := w.config.GetInfoStream()
	if infoStream != nil && infoStream.IsEnabled("IW") {
		infoStream.Message("IW", "flush at addIndexes(Directory...)")
	}
	if _, err := w.doFlush(true); err != nil {
		return 0, err
	}

	var infos []*SegmentCommitInfo

	// int64 so overflow of the int32 document limit is detectable.
	var totalMaxDoc int64
	commits := make([]*SegmentInfos, 0, len(dirs))
	for _, dir := range dirs {
		if infoStream != nil && infoStream.IsEnabled("IW") {
			infoStream.Message("IW", fmt.Sprintf("addIndexes: process directory %v", dir))
		}
		sis, err := spi.ReadLatestCommit(dir)
		if err != nil {
			return 0, err
		}
		if w.segmentInfos.GetIndexCreatedVersionMajor() != sis.GetIndexCreatedVersionMajor() {
			return 0, fmt.Errorf(
				"index: cannot use AddIndexes(Directory) with indexes that have been created by a different Lucene version; "+
					"the current index was generated by Lucene %d while one of the directories contains an index generated with Lucene %d",
				w.segmentInfos.GetIndexCreatedVersionMajor(), sis.GetIndexCreatedVersionMajor())
		}
		totalMaxDoc += int64(sis.TotalMaxDoc())
		commits = append(commits, sis)
	}

	// Best-effort up-front check:
	if err := w.testReserveDocs(totalMaxDoc); err != nil {
		return 0, err
	}

	copyErr := func() error {
		for _, sis := range commits {
			for info := range sis.Iterator() {
				segmentIndexSort := info.SegmentInfo().IndexSort()
				if indexSort != nil && (segmentIndexSort == nil || !isCongruentSort(indexSort, segmentIndexSort)) {
					return fmt.Errorf("index: cannot change index sort from %v to %v", segmentIndexSort, indexSort)
				}

				newSegName := w.newSegmentName()

				if infoStream != nil && infoStream.IsEnabled("IW") {
					infoStream.Message("IW", fmt.Sprintf(
						"addIndexes: process segment origName=%s newName=%s info=%v",
						info.SegmentInfo().Name(), newSegName, info))
				}

				sizeInBytes, err := info.SizeInBytes()
				if err != nil {
					return err
				}
				ctx := store.IOContextFlush(store.NewFlushInfo(info.SegmentInfo().MaxDoc(), sizeInBytes))

				fis, err := readFieldInfos(info)
				if err != nil {
					return err
				}
				for it := fis.Iterator(); it.HasNext(); {
					fi := it.Next()
					if fi == nil {
						break
					}
					// This raises an error if any incoming field carries an
					// illegal schema change.
					w.globalFieldNumberMap.AddOrGet(fi)
				}

				copied, err := w.copySegmentAsIs(info, newSegName, ctx)
				if err != nil {
					return err
				}
				infos = append(infos, copied)
			}
		}
		return nil
	}()
	if copyErr != nil {
		for _, sipc := range infos {
			// Safe: these files must exist.
			if err := w.deleteNewFiles(sipc.Files()); err != nil {
				return 0, err
			}
		}
		return 0, copyErr
	}

	seqNo, reserveErr := func() (int64, error) {
		w.mu.Lock()
		defer w.mu.Unlock()

		if err := w.ensureOpen(true); err != nil {
			return 0, err
		}
		// Now reserve the docs, just before SegmentInfos is updated:
		if err := w.reserveDocs(totalMaxDoc); err != nil {
			return 0, err
		}
		return w.docWriter.GetNextSequenceNumber(), nil
	}()
	if reserveErr != nil {
		for _, sipc := range infos {
			if err := w.deleteNewFiles(sipc.Files()); err != nil {
				return 0, err
			}
		}
		return 0, reserveErr
	}

	for _, sipc := range infos {
		w.segmentInfos.Add(sipc)
	}
	if err := w.checkpoint(); err != nil {
		return 0, err
	}

	return seqNo, nil
}

// noDupDirs reports an error when a directory appears twice in dirs, or when
// one of them is this writer's own directory.
//
// Mirrors org.apache.lucene.index.IndexWriter#noDupDirs.
func (w *IndexWriter) noDupDirs(dirs ...store.Directory) error {
	dups := make(map[store.Directory]struct{}, len(dirs))
	for _, dir := range dirs {
		if _, ok := dups[dir]; ok {
			return fmt.Errorf("index: directory %v appears more than once", dir)
		}
		if dir == w.dirOrig {
			return fmt.Errorf("index: cannot add directory to itself")
		}
		dups[dir] = struct{}{}
	}
	return nil
}

// acquireWriteLocks obtains the write lock of every directory. On failure it
// releases the locks it already holds and reports the original error.
//
// Mirrors org.apache.lucene.index.IndexWriter#acquireWriteLocks.
func (w *IndexWriter) acquireWriteLocks(dirs ...store.Directory) ([]store.Lock, error) {
	locks := make([]store.Lock, 0, len(dirs))
	for _, dir := range dirs {
		lock, err := dir.ObtainLock("write.lock")
		if err != nil {
			for _, held := range locks {
				if closeErr := held.Close(); closeErr != nil {
					err = fmt.Errorf("%w; releasing an already acquired write lock also failed: %v", err, closeErr)
				}
			}
			return nil, err
		}
		locks = append(locks, lock)
	}
	return locks, nil
}

// copySegmentAsIs copies every file of info into this writer's directory under
// segName, and returns the SegmentCommitInfo describing the copy.
//
// Mirrors org.apache.lucene.index.IndexWriter#copySegmentAsIs.
func (w *IndexWriter) copySegmentAsIs(
	info *SegmentCommitInfo, segName string, ctx store.IOContext,
) (*SegmentCommitInfo, error) {
	src := info.SegmentInfo()

	// The same SegmentInfo as before, with a new directory and name.
	newInfo := spi.NewSegmentInfo(segName, src.MaxDoc(), w.dirOrig)
	newInfo.SetVersion(src.Version())
	if minVersion, ok := src.MinVersion(); ok {
		newInfo.SetMinVersion(minVersion)
	}
	newInfo.SetUseCompoundFile(src.GetUseCompoundFile())
	newInfo.SetHasBlocks(src.GetHasBlocks())
	newInfo.SetCodec(src.Codec())
	newInfo.SetDiagnostics(src.GetDiagnostics())
	newInfo.SetAttributes(src.GetAttributes())
	newInfo.SetIndexSort(src.IndexSort())
	if err := newInfo.SetID(src.GetID()); err != nil {
		return nil, err
	}

	newInfoPerCommit := spi.NewSegmentCommitInfo(newInfo, info.GetDelCount(), info.GetDelGen())
	newInfoPerCommit.SetSoftDelCount(info.GetSoftDelCount())
	newInfoPerCommit.SetFieldInfosGen(info.FieldInfosGen())
	newInfoPerCommit.SetDocValuesGen(info.DocValuesGen())
	newInfoPerCommit.SetID(info.GetID())

	// SegmentInfo.addFiles renames every file for the new segment; reproduce
	// that here so the recorded file set matches what is copied below.
	srcFiles := src.Files()
	renamed := make([]string, 0, len(srcFiles))
	for _, f := range srcFiles {
		renamed = append(renamed, segName+StripSegmentName(f))
	}
	newInfo.SetFiles(renamed)
	newInfoPerCommit.SetFieldInfosFiles(info.FieldInfosFiles())
	newInfoPerCommit.SetDocValuesUpdatesFiles(info.DocValuesUpdatesFiles())

	copiedFiles := make([]string, 0, len(srcFiles))
	for _, file := range info.Files() {
		newFileName := segName + StripSegmentName(file)
		if err := copyIndexFile(src.Directory(), w.dir, file, newFileName, ctx); err != nil {
			// Safe: these files must exist.
			if delErr := w.deleteNewFiles(copiedFiles); delErr != nil {
				return nil, fmt.Errorf("%w; cleaning up the partial copy also failed: %v", err, delErr)
			}
			return nil, err
		}
		copiedFiles = append(copiedFiles, newFileName)
	}

	return newInfoPerCommit, nil
}

// copyIndexFile copies one file between directories, byte for byte.
//
// Mirrors the Directory#copyFrom call Lucene's copySegmentAsIs performs.
func copyIndexFile(from, to store.Directory, src, dest string, ctx store.IOContext) error {
	in, err := from.OpenInput(src, ctx)
	if err != nil {
		return err
	}
	out, err := to.CreateOutput(dest, ctx)
	if err != nil {
		if closeErr := in.Close(); closeErr != nil {
			return fmt.Errorf("%w; closing the source file also failed: %v", err, closeErr)
		}
		return err
	}

	copyErr := out.CopyBytes(in, in.Length())
	if closeErr := out.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	if closeErr := in.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	return copyErr
}

// reserveDocs reserves room for addedNumDocs more documents, reporting an error
// when the index would exceed MaxDocs.
//
// Mirrors org.apache.lucene.index.IndexWriter#reserveDocs.
func (w *IndexWriter) reserveDocs(addedNumDocs int64) error {
	if w.pendingNumDocs.Add(addedNumDocs) > MaxDocs {
		// Reserve failed: put the docs back and report.
		w.pendingNumDocs.Add(-addedNumDocs)
		return w.tooManyDocs(addedNumDocs)
	}
	return nil
}

// testReserveDocs does a best-effort check that the index would accept
// addedNumDocs more documents, without reserving them.
//
// Mirrors org.apache.lucene.index.IndexWriter#testReserveDocs.
func (w *IndexWriter) testReserveDocs(addedNumDocs int64) error {
	if w.pendingNumDocs.Load()+addedNumDocs > MaxDocs {
		return w.tooManyDocs(addedNumDocs)
	}
	return nil
}

// tooManyDocs builds the error reported when the document limit is exceeded.
//
// Mirrors org.apache.lucene.index.IndexWriter#tooManyDocs.
func (w *IndexWriter) tooManyDocs(addedNumDocs int64) error {
	return fmt.Errorf(
		"index: number of documents in the index cannot exceed %d (current document count is %d; added numDocs is %d)",
		MaxDocs, w.pendingNumDocs.Load(), addedNumDocs)
}

// isCongruentSort reports whether indexSort is a prefix of otherSort.
//
// Mirrors org.apache.lucene.index.IndexWriter#isCongruentSort.
func isCongruentSort(indexSort, otherSort *spi.Sort) bool {
	fields1 := indexSort.Fields()
	fields2 := otherSort.Fields()
	if len(fields1) > len(fields2) {
		return false
	}
	for i, sf := range fields1 {
		if !sameSortField(sf, fields2[i]) {
			return false
		}
	}
	return true
}

// sameSortField compares the serialised identity of two sort fields: the
// properties SegmentInfo persists and that therefore decide whether two index
// sorts describe the same order.
func sameSortField(a, b *spi.SortField) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Field == b.Field &&
		a.Type == b.Type &&
		a.Reverse == b.Reverse &&
		a.Missing == b.Missing &&
		a.Selector == b.Selector
}
