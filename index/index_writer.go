// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"strconv"
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
	rollbackSegments *SegmentInfos

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

	deleter *IndexFileDeleter

	segmentsToMerge     map[*SegmentCommitInfo]bool
	mergeMaxNumSegments int

	writeLock store.Lock

	closed  atomic.Bool
	closing atomic.Bool

	maybeMerge atomic.Bool

	running        atomic.Bool
	commitUserData []mapEntry[string, string]

	mergingSegments         map[*SegmentCommitInfo]struct{}
	mergeScheduler          MergeScheduler
	runningAddIndexesMerges map[*SegmentMerger]struct{}
	pendingMerges           []MergePolicy.OneMerge
	runningMerges           map[MergePolicy.OneMerge]struct{}
	mergeExceptions         []error
	merges                  *merges
	mergeGen                int64
	didMessageState         bool
	flushCount              atomic.Int32
	flushDeletesCount       atomic.Int32
	readerPool              *ReaderPool
	bufferedUpdatesStream   *BufferedUpdatesStream

	eventListener IndexWriterEventListener

	mergeFinishedGen atomic.Int64

	// The instance that was passed to the constructor.
	liveConfig *LiveIndexWriterConfig

	startCommitTime int64

	pendingNumDocs atomic.Int64

	softDeletesEnabled bool

	flushNotifications *DocumentsWriter.FlushNotifications

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

type indexWriterMergeSource struct {
	writer *IndexWriter
}

func (s *indexWriterMergeSource) GetWriter() *IndexWriter {
	return s.writer
}

func (s *indexWriterMergeSource) Merge(merge *OneMerge) error {
	return s.writer.Merge(merge)
}

type addIndexesMergeSource struct {
	writer *IndexWriter
}

func (s *addIndexesMergeSource) GetWriter() *IndexWriter {
	return s.writer
}

// NewIndexWriter constructs a new IndexWriter per the settings given in conf.
func NewIndexWriter(d store.Directory, conf *IndexWriterConfig) (*IndexWriter, error) {
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
	}

	success := false
	defer func() {
		if !success {
			d.CloseLock(writeLock)
		}
	}()

	writer.dir = util.NewLockValidatingDirectoryWrapper(d, writeLock)
	writer.mergeScheduler = liveConfig.GetMergeScheduler()
	writer.mergeScheduler.Initialize(liveConfig.GetInfoStream(), d)

	mode := liveConfig.GetOpenMode()
	indexExists := util.IndexExists(writer.dir)
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
	var rollbackSegments *SegmentInfos

	if create {
		sis := spi.NewSegmentInfos(liveConfig.GetIndexCreatedVersionMajor())
		if indexExists {
			previous, err := spi.ReadLatestCommit(writer.dir)
			if err != nil {
				return nil, err
			}
			sis.UpdateGenerationVersionAndCounter(previous)
		}
		segmentInfos = sis
		rollbackSegments = sis.Clone()
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
			rollbackSegments = lastCommit.Clone()
		} else {
			lastSegmentsFile := spi.GetLastCommitSegmentsFileName(writer.dir.ListAll())
			if lastSegmentsFile == "" {
				return nil, fmt.Errorf("no segments* file found in %s", writer.dir)
			}
			segmentInfos, err = spi.ReadCommit(writer.dirOrig, lastSegmentsFile)
			if err != nil {
				return nil, err
			}
			rollbackSegments = segmentInfos.Clone()
		}
	}

	writer.segmentInfos = segmentInfos
	writer.rollbackSegments = rollbackSegments
	writer.globalFieldNumberMap = writer.getFieldNumberMap()

	writer.bufferedUpdatesStream = NewBufferedUpdatesStream(liveConfig.GetInfoStream())
	writer.docWriter = NewDocumentsWriter(
		writer.flushNotifications(),
		segmentInfos.GetIndexCreatedVersionMajor(),
		&writer.pendingNumDocs,
		false,
		writer.newSegmentName,
		conf,
		writer.dirOrig,
		writer.dir,
		writer.globalFieldNumberMap,
		writer.liveConfig.GetInfoStream(),
	)

	writer.readerPool = NewReaderPool(
		writer.dir,
		writer.dirOrig,
		segmentInfos,
		writer.globalFieldNumberMap,
		writer.bufferedUpdatesStream.GetCompletedDelGen,
		liveConfig.GetInfoStream(),
		liveConfig.GetSoftDeletesField(),
		nil,
	)
	if liveConfig.GetReaderPooling() {
		writer.readerPool.EnableReaderPooling()
	}

	writer.deleter = NewIndexFileDeleter(
		writer.dir.ListAll(),
		writer.dirOrig,
		writer.dir,
		liveConfig.GetIndexDeletionPolicy(),
		segmentInfos,
		liveConfig.GetInfoStream(),
		writer,
		indexExists,
		false,
	)

	writer.eventQueue = newEventQueue(writer)
	writer.mergeSource = &indexWriterMergeSource{writer: writer}
	writer.addIndexesMergeSource = &addIndexesMergeSource{writer: writer}

	writer.running.Store(true)
	success = true
	return writer, nil
}

func (w *IndexWriter) newSegmentName() string {
	w.segmentInfos.mu.Lock()
	defer w.segmentInfos.mu.Unlock()
	w.changeCount.Add(1)
	w.segmentInfos.Changed()
	return "_" + strconv.FormatInt(w.segmentInfos.Counter(), 36)
}

func (w *IndexWriter) GetMaxCompletedSequenceNumber() int64 {
	return w.publishedSeqNo
}

func (w *IndexWriter) changed() {
	w.segmentInfos.mu.Lock()
	defer w.segmentInfos.mu.Unlock()
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

func (w *IndexWriter) tragicEvent(t error, context string) {
	if w.tragedy.Load() == nil {
		w.tragedy.Store(t)
		infoStream := w.config.LiveIndexWriterConfig.GetInfoStream()
		if infoStream.IsEnabled("IW") {
			infoStream.Message("IW", fmt.Sprintf("tragic event: %s: %v", context, t))
		}
		if w.eventListener != nil {
			w.eventListener.OnTragicEvent(t, context)
		}
		w.flushNotifications().OnTragicEvent(t, context)
	}
}

func (w *IndexWriter) maybeCloseOnTragicEvent() error {
	if t := w.tragedy.Load(); t != nil {
		return fmt.Errorf("tragic event occurred: %v", t)
	}
	return nil
}

func (w *IndexWriter) maybeProcessEvents() error {
	return w.eventQueue.processEvents(w)
}

func (w *IndexWriter) AddDocument(doc *document.Document) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	seqNo, err := w.docWriter.AddDocument(doc, w.config.LiveIndexWriterConfig.GetAnalyzer())
	if err != nil {
		w.tragicEvent(err, "AddDocument")
		return 0, err
	}
	w.changed()
	return seqNo, nil
}

func (w *IndexWriter) UpdateDocument(term *Term, doc *document.Document) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	seqNo, err := w.docWriter.UpdateDocument(doc, w.config.LiveIndexWriterConfig.GetAnalyzer(), term)
	if err != nil {
		w.tragicEvent(err, "UpdateDocument")
		return 0, err
	}
	w.changed()
	return seqNo, nil
}

func (w *IndexWriter) AddDocuments(docs []*document.Document) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	lastSeqNo, err := w.docWriter.UpdateDocuments(docs, w.config.LiveIndexWriterConfig.GetAnalyzer(), nil)
	if err != nil {
		w.tragicEvent(err, "AddDocuments")
		return 0, err
	}
	w.changed()
	return lastSeqNo, nil
}

func (w *IndexWriter) UpdateDocuments(term *Term, docs []*document.Document) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	docsFields := make([][]document.IndexableField, len(docs))
	for i, doc := range docs {
		docsFields[i] = doc.GetAllFields()
	}
	var delNode Node
	if term != nil {
		delNode = NewTermNode(term)
	}
	lastSeqNo, err := w.docWriter.UpdateDocuments(docsFields, delNode)
	if err != nil {
		w.tragicEvent(err, "UpdateDocuments")
		return 0, err
	}
	w.changed()
	return lastSeqNo, nil
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
	seqNo, err := w.docWriter.UpdateDocValues([]DocValuesUpdate{
		NewNumericDocValuesUpdate(term, field, &value),
	})
	if err != nil {
		w.tragicEvent(err, "UpdateNumericDocValue")
		return 0, err
	}
	return w.maybeProcessEvents(seqNo), nil
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
	seqNo, err := w.docWriter.UpdateDocValues([]DocValuesUpdate{
		NewBinaryDocValuesUpdate(term, field, value),
	})
	if err != nil {
		w.tragicEvent(err, "UpdateBinaryDocValue")
		return 0, err
	}
	return w.maybeProcessEvents(seqNo), nil
}

// UpdateDocValues updates documents' DocValues fields to the given values.
// Each field update is applied to the set of documents that are associated with the Term
// to the same value. All updates are atomically applied and flushed together.
func (w *IndexWriter) UpdateDocValues(term *Term, updates []*document.Field) (int64, error) {
	if err := w.ensureOpen(true); err != nil {
		return 0, err
	}
	dvUpdates := w.buildDocValuesUpdate(term, updates)
	seqNo, err := w.docWriter.UpdateDocValues(dvUpdates)
	if err != nil {
		w.tragicEvent(err, "UpdateDocValues")
		return 0, err
	}
	return w.maybeProcessEvents(seqNo), nil
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
	delNode := NewDocValuesUpdatesNode(dvUpdates)

	docsFields := [][]document.IndexableField{doc.GetAllFields()}

	seqNo, err := w.docWriter.UpdateDocuments(docsFields, delNode)
	if err != nil {
		w.tragicEvent(err, "SoftUpdateDocument")
		return 0, err
	}
	return w.maybeProcessEvents(seqNo), nil
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
	delNode := NewDocValuesUpdatesNode(dvUpdates)

	docsFields := make([][]document.IndexableField, len(docs))
	for i, doc := range docs {
		docsFields[i] = doc.GetAllFields()
	}

	seqNo, err := w.docWriter.UpdateDocuments(docsFields, delNode)
	if err != nil {
		w.tragicEvent(err, "SoftUpdateDocuments")
		return 0, err
	}
	return w.maybeProcessEvents(seqNo), nil
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
	w.ensureOpen()
	seqNo, err := w.docWriter.DeleteTerms(terms)
	if err != nil {
		return 0, err
	}
	return w.maybeProcessEvents(seqNo), nil
}

// DeleteAll deletes all documents from the index.
func (w *IndexWriter) DeleteAll() (int64, error) {
	w.ensureOpen()

	// Directly call docWriter to avoid loop with DeleteDocumentsByQuery
	seqNo, err := w.docWriter.DeleteQueries([]Query{&MatchAllDocsQuery{}})
	if err != nil {
		return 0, err
	}
	return w.maybeProcessEvents(seqNo), nil
}

// TryDeleteDocument attempts to delete a document by its global docID.
// This is the Go port of Lucene's org.apache.lucene.index.IndexWriter#tryDeleteDocument.
func (w *IndexWriter) TryDeleteDocument(docID int) (bool, error) {
	w.ensureOpen()

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
	for _, sci := range w.segmentInfos.Iterator() {
		maxDoc := sci.Info.MaxDoc()
		if docID < base+maxDoc {
			return sci, docID - base, nil
		}
		base += maxDoc
	}
	return nil, 0, fmt.Errorf("docID %d out of range [0, %d)", docID, w.segmentInfos.TotalMaxDoc())
}

// DeleteDocumentsQuery deletes documents matching the given queries.
func (w *IndexWriter) DeleteDocumentsQuery(queries []Query) (int64, error) {
	w.ensureOpen()

	for _, q := range queries {
		if _, ok := q.(*MatchAllDocsQuery); ok {
			return w.DeleteAll()
		}
	}

	seqNo, err := w.docWriter.DeleteQueries(queries)
	if err != nil {
		return 0, err
	}
	return w.maybeProcessEvents(seqNo), nil
}

func (w *IndexWriter) doBeforeFlush() {}

func (w *IndexWriter) doFlush(applyAllDeletes bool) (int64, error) {
	if err := w.maybeCloseOnTragicEvent(); err != nil {
		return 0, err
	}

	w.doBeforeFlush()

	var seqNo int64
	var flushSuccess bool

	w.fullFlushLock.Lock()
	seqNo = w.docWriter.FlushAllThreads()
	if seqNo >= 0 {
		w.flushCount.Add(1)
	}
	w.publishFlushedSegments(true)
	flushSuccess = true
	w.docWriter.FinishFullFlush(flushSuccess)
	w.eventQueue.processEvents(w)
	w.fullFlushLock.Unlock()

	if applyAllDeletes {
		if err := w.ApplyAllDeletesAndUpdates(); err != nil {
			return 0, err
		}
	}

	anyChanges := (seqNo < 0) || w.maybeMerge.Swap(false)
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
	w.ensureOpen()
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	seqNo, err := w.prepareCommitInternal()
	if err != nil {
		return 0, err
	}
	w.pendingSeqNo = seqNo

	if w.maybeMerge.Swap(false) {
		w.maybeMerge(w.config.GetMergePolicy(), MergeTriggerFullFlush, -1)
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
	w.deleter.IncRef(toCommit.Files(false))

	w.pendingCommit = toCommit
	return seqNo, nil
}

func (w *IndexWriter) Commit() (int64, error) {
	w.ensureOpen()
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

	if w.maybeMerge.Swap(false) {
		w.maybeMerge(w.config.GetMergePolicy(), MergeTriggerFullFlush, -1)
	}
	return seqNo, nil
}

func (w *IndexWriter) finishCommit() error {
	if w.pendingCommit == nil {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	committedSegmentsFileName, err := w.pendingCommit.FinishCommit(w.dir, w.config.Codec())
	if err != nil {
		return err
	}

	w.deleter.Checkpoint(w.pendingCommit, true)
	w.segmentInfos.UpdateGeneration(w.pendingCommit)
	w.lastCommitChangeCount.Store(w.pendingCommitChangeCount)
	w.rollbackSegments = w.pendingCommit.Clone()

	w.pendingCommit = nil
	w.filesToCommit = nil
	_ = committedSegmentsFileName
	return nil
}

func (w *IndexWriter) updatePendingMerges(policy MergePolicy) *MergeSpecification {
	return policy.FindFullFlushMerges(MergeTriggerFullFlush, w.segmentInfos, w.mergeSource)
}

func (w *IndexWriter) preparePointInTimeMerge(
	mergingSegmentInfos *SegmentInfos,
	stopCollectingMergeResults func() bool,
	trigger MergeTrigger,
	mergeFinished func(*SegmentCommitInfo),
) MergeSpecification {
	// In Java, this uses a wrapped MergePolicy to add custom hooks to OneMerge.
	// In Go, we can just find the merges and then add the hooks to each OneMerge.
	spec := w.updatePendingMerges(w.config.GetMergePolicy())
	if spec == nil {
		return MergeSpecification{}
	}

	for _, m := range spec.Merges {
		m.OnMergeFinished = func(merge *OneMerge, success bool, segmentDropped bool) error {
			if segmentDropped == false && success && stopCollectingMergeResults() == false {
				if trigger == MergeTriggerCommit {
					// If we do this in a getReader call here this is obsolete since we
					// already hold a reader that has incRef'd these files
					w.deleter.IncRef(merge.Info.Files())
				}
				mergedSegmentNames := make(map[string]bool)
				for _, sci := range merge.Segments {
					mergedSegmentNames[sci.Info.Name()] = true
				}
				toCommitMergedAwaySegments := make([]*SegmentCommitInfo, 0)
				for _, sci := range mergingSegmentInfos.Iterator() {
					if mergedSegmentNames[sci.Info.Name()] {
						toCommitMergedAwaySegments = append(toCommitMergedAwaySegments, sci)
						if trigger == MergeTriggerCommit {
							w.deleter.DecRef(sci.Files())
						}
					}
				}
				applicableMerge := NewOneMerge(toCommitMergedAwaySegments)
				applicableMerge.Info = merge.Info
				longVal := 0 // Simplified: should be parsed from merge.Info.Name()
				mergingSegmentInfos.counter = max(mergingSegmentInfos.counter, longVal+1)
				mergingSegmentInfos.ApplyMergeChanges(applicableMerge, false)
			}
			return nil
		}
		m.OnMergeComplete = func(merge *OneMerge) {
			if stopCollectingMergeResults() == false && !w.closed.Load() && merge.Info.SegmentInfo().DocCount() > 0 {
				mergeFinished(merge.Info)
			}
		}
	}
	return *spec
}

func (w *IndexWriter) finishGetReaderMerge(
	stopCollectingMergedReaders *atomic.Bool,
	mergedReaders map[string]*SegmentReader,
	openedReadOnlyClones map[string]*SegmentReader,
	openingSegmentInfos *SegmentInfos,
	applyAllDeletes, writeAllDeletes bool,
	pointInTimeMerges MergeSpecification,
	maxCommitMergeWaitMillis int64,
) *StandardDirectoryReader {
	openingSegmentInfos.mu.Lock()
	defer openingSegmentInfos.mu.Unlock()

	w.mergeScheduler.Merge(w.mergeSource, MergeTriggerGetReader)
	// Await the merges. This is a simplified version of pointInTimeMerges.await().
	for _, m := range pointInTimeMerges.Merges {
		m.Done()
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	stopCollectingMergedReaders.Store(true)

	reader := w.maybeReopenMergedNRTReader(
		mergedReaders,
		openedReadOnlyClones,
		openingSegmentInfos,
		applyAllDeletes,
		writeAllDeletes,
	)
	for _, sr := range mergedReaders {
		sr.Close()
	}
	clear(mergedReaders)

	return reader
}

func (w *IndexWriter) maybeReopenMergedNRTReader(
	mergedReaders map[string]*SegmentReader,
	openedReadOnlyClones map[string]*SegmentReader,
	openingSegmentInfos *SegmentInfos,
	applyAllDeletes, writeAllDeletes bool,
) *StandardDirectoryReader {
	if len(mergedReaders) == 0 {
		return nil
	}

	files := make([]string, 0)
	readerFactory := func(sci *SegmentCommitInfo) (*ReadersAndUpdates, error) {
		if sr, ok := mergedReaders[sci.Info.Name()]; ok {
			delete(mergedReaders, sci.Info.Name())
			files = append(files, sr.SegmentInfo().Files()...)
			return w.getPooledInstance(sci, true), nil
		}
		if sr, ok := openedReadOnlyClones[sci.Info.Name()]; ok {
			delete(openedReadOnlyClones, sci.Info.Name())
			sr.IncRef()
			return w.getPooledInstance(sci, true), nil
		}
		return w.getPooledInstance(sci, true), nil
	}

	r := Open(w, readerFactory, openingSegmentInfos, applyAllDeletes, writeAllDeletes)
	w.deleter.DecRef(files)
	return r
}


func (w *IndexWriter) GetConfig() *IndexWriterConfig {
	return w.config
}

func (w *IndexWriter) GetAnalyzer() analysis.Analyzer {
	return w.config.analyzer
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
func (w *IndexWriter) HasDeletions() bool {
	w.ensureOpen()
	if w.bufferedUpdatesStream.Any() || w.docWriter.anyDeletions() || w.readerPool.anyDeletions() {
		return true
	}
	for _, info := range w.segmentInfos.Iterator() {
		if info.HasDeletions() {
			return true
		}
	}
	return false
}

func (w *IndexWriter) GetDocWriterThreadPoolSize() int {
	return len(w.docWriter.GetPerThreadPool())
}

func (w *IndexWriter) GetSegmentCount() int {
	w.segmentInfos.mu.Lock()
	defer w.segmentInfos.mu.Unlock()
	return len(w.segmentInfos.Iterator())
}

func (w *IndexWriter) adjustPendingNumDocs(delta int) {
	w.pendingNumDocs.Add(int64(delta))
}

func (w *IndexWriter) FlushNextBuffer() bool {
	return w.docWriter.FlushNextBuffer()
}

func (w *IndexWriter) Close() error {
	if !w.closing.Swap(true) {
		return nil
	}

	if w.config.GetCommitOnClose() {
		if err := w.Commit(); err != nil {
			return err
		}
	}

	w.closed.Store(true)
	return nil
}

// Rollback reverts the index to the last committed state.
func (w *IndexWriter) Rollback() error {
	w.ensureOpen()
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
	w.docWriter.Abort()

	// 5. Wait for flushes and publish segments
	w.docWriter.FlushControl().WaitForFlush()
	w.publishFlushedSegments(true)

	// 6. Close event queue
	if err := w.eventQueue.Close(); err != nil {
		return fmt.Errorf("failed to close event queue during rollback: %w", err)
	}

	// 7. Roll back pending commit
	if w.pendingCommit != nil {
		if err := w.pendingCommit.RollbackCommit(w.dir); err != nil {
			return fmt.Errorf("failed to rollback pending commit: %w", err)
		}
		w.deleter.DecRef(w.pendingCommit)
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
		w.readerPool.Close()
	}

	// 12. Finalize writer state
	w.lastCommitChangeCount.Store(w.changeCount.Load())
	w.closed.Store(true)

	// 13. Release write lock
	if w.writeLock != nil {
		w.dirOrig.CloseLock(w.writeLock)
	}

	return nil
}

func (w *IndexWriter) maybeProcessEvents(seqNo int64) int64 {
	w.eventQueue.processEvents(w)
	return seqNo
}

func (w *IndexWriter) publishFlushedSegments(forced bool) {
	w.docWriter.PurgeFlushTickets(
		forced,
		func(ticket *DocumentsWriter.FlushTicket) {
			ticket.MarkPublished()
			newSegment := ticket.GetFlushedSegment()
			bufferedUpdates := ticket.GetFrozenUpdates()

			if newSegment == nil {
				if bufferedUpdates != nil && bufferedUpdates.Any() {
					w.publishFrozenUpdates(bufferedUpdates)
				}
			} else {
				w.publishFlushedSegment(
					newSegment.SegmentInfo,
					newSegment.FieldInfos,
					newSegment.SegmentUpdates,
					bufferedUpdates,
					newSegment.SortMap,
				)
			}
		},
	)
}

func (w *IndexWriter) applyAllDeletesAndUpdates() {
	w.bufferedUpdatesStream.WaitApplyAll(w)
}

func (w *IndexWriter) writeReaderPool(writeDeletes bool) {
	if writeDeletes {
		if w.readerPool.Commit(w.segmentInfos) {
			w.checkpointNoSIS()
		}
	} else {
		if w.readerPool.WriteAllDocValuesUpdates() {
			w.checkpoint()
		}
	}
}

func (w *IndexWriter) checkpoint() {
	w.segmentInfos.Changed()
	w.changeCount.Add(1)
}

func (w *IndexWriter) checkpointNoSIS() {
	w.changeCount.Add(1)
}

func (w *IndexWriter) maybeMerge(policy MergePolicy, trigger MergeTrigger, maxSegments int) {
	if policy == nil {
		return
	}

	// Consult the merge policy to find potential merges for the given trigger.
	spec, err := policy.FindMerges(trigger, w.segmentInfos, w.mergeSource)
	if err != nil {
		// Log merge policy error and continue
		return
	}

	if spec != nil {
		// Submit the found merges to the scheduler.
		w.mergeScheduler.MergeWithSpec(w.mergeSource, spec, false)
	}
}

func (w *IndexWriter) getFieldNumberMap() *FieldNumbers {
	fnm := NewFieldNumbers(w.config.GetSoftDeletesField(), w.config.GetParentField())
	for _, sci := range w.segmentInfos.Iterator() {
		fis := readFieldInfos(sci)
		for _, fi := range fis {
			fnm.AddOrGet(fi)
		}
	}
	return fnm
}

func readFieldInfos(si *SegmentCommitInfo) *FieldInfos {
	codec := LookupCodecByName(si.SegmentInfo().Codec())
	reader := codec.FieldInfosFormat()
	if si.HasFieldUpdates() {
		suffix := strconv.FormatInt(si.FieldInfosGen(), 36)
		return reader.Read(si.SegmentInfo().Directory(), si.SegmentInfo(), suffix, store.IOContextReadOnce)
	} else if si.SegmentInfo().IsCompoundFile() {
		cfs := codec.CompoundFormat().GetCompoundReader(si.SegmentInfo().Directory(), si.SegmentInfo())
		defer cfs.Close()
		return reader.Read(cfs, si.SegmentInfo(), "", store.IOContextReadOnce)
	}
	return reader.Read(si.SegmentInfo().Directory(), si.SegmentInfo(), "", store.IOContextReadOnce)
}

type mapEntry[K, V any] struct {
	Key   K
	Value V
}

func (w *IndexWriter) flushNotifications() *DocumentsWriter.FlushNotifications {
	return &DocumentsWriter.FlushNotifications{
		DeleteUnusedFiles: func(files []string) {
			w.eventQueue.add(func(iw *IndexWriter) error {
				return iw.deleteNewFiles(files)
			})
		},
		FlushFailed: func(info *SegmentInfo) {
			w.eventQueue.add(func(iw *IndexWriter) error {
				return iw.flushFailed(info)
			})
		},
		AfterSegmentsFlushed: func() error {
			w.publishFlushedSegments(false)
			return nil
		},
		OnTragicEvent: func(event error, message string) {
			w.onTragicEvent(event, message)
		},
		OnDeletesApplied: func() {
			w.eventQueue.add(func(iw *IndexWriter) error {
				iw.publishFlushedSegments(true)
				iw.flushCount.Add(1)
				return nil
			})
		},
		OnTicketBacklog: func() {
			w.eventQueue.add(func(iw *IndexWriter) error {
				iw.publishFlushedSegments(true)
				return nil
			})
		},
	}
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
	sortMap interface{},
) {
	w.ensureOpen()

	if globalPacket != nil && globalPacket.Any() {
		w.publishFrozenUpdates(globalPacket)
	}

	var nextGen int64
	if packet != nil && packet.Any() {
		nextGen = w.publishFrozenUpdates(packet)
	} else {
		nextGen = w.bufferedUpdatesStream.GetNextGen()
		w.bufferedUpdatesStream.FinishedSegment(nextGen)
	}

	newSegment.SetBufferedDeletesGen(nextGen)
	w.segmentInfos.Add(newSegment)
	w.checkpoint()

	if packet != nil && packet.Any() && sortMap != nil {
		rau := w.getPooledInstance(newSegment, true)
		if rau != nil {
			rau.SetSortMap(sortMap)
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
			if deleted, err := w.isFullyDeleted(rau); err == nil && deleted {
				w.dropDeletedSegment(newSegment)
				w.checkpoint()
			}
			w.release(rau)
		}
	}

	w.flushCount.Add(1)
	w.doAfterFlush()
}

func (w *IndexWriter) publishFrozenUpdates(packet *FrozenBufferedUpdates) int64 {
	nextGen := w.bufferedUpdatesStream.Push(packet)
	w.eventQueue.Add(func(iw *IndexWriter) error {
		defer iw.flushDeletesCount.Add(1)
		if err := iw.tryApply(packet); err != nil {
			iw.onTragicEvent(err, "applyUpdatesPacket")
			return err
		}
		return nil
	})
	return nextGen
}

func (w *IndexWriter) doAfterFlush() {}

// Merge executes a single merge operation.
func (w *IndexWriter) Merge(merge *OneMerge) error {
	w.ensureOpen()
	success := false
	defer func() {
		w.commitLock.Lock()
		merge.Close(success, false, func(mr *OneMerge) {})
		w.mergeFinish(merge)
		w.commitLock.Unlock()
	}()

	err := w.mergeInternal(merge)
	if err != nil {
		w.handleMergeException(err, merge)
		return err
	}
	success = true
	return nil
}

func (w *IndexWriter) ForceMerge(maxNumSegments int) error {
	_, err := w.ForceMergeWithObserver(maxNumSegments, true)
	return err
}

func (w *IndexWriter) ForceMergeWithObserver(maxNumSegments int, doWait bool) (*MergePolicy.MergeObserver, error) {
	w.ensureOpen()
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	spec := w.config.GetMergePolicy().FindForcedMergeSpec(w.segmentInfos, w.mergeSource, maxNumSegments)
	if spec == nil {
		return index.NewMergeObserver(nil), nil
	}

	if err := w.mergeScheduler.MergeWithSpec(w.mergeSource, spec, doWait); err != nil {
		return nil, err
	}

	return index.NewMergeObserver(spec), nil
}

func (w *IndexWriter) ForceMergeDeletes() error {
	_, err := w.ForceMergeDeletesWithObserver(true)
	return err
}

// ForceMergeDeletesWithObserver executes a merge to expunge all deletes from the index.
// Returns a MergeObserver to monitor progress.
func (w *IndexWriter) ForceMergeDeletesWithObserver(doWait bool) (*MergeObserver, error) {
	w.ensureOpen()
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	spec := w.config.GetMergePolicy().FindForcedDeletesMerges(w.segmentInfos, w.mergeSource)
	if spec == nil {
		return NewMergeObserver(nil), nil
	}

	// register merges with the scheduler
	if err := w.mergeScheduler.MergeWithSpec(w.mergeSource, spec, doWait); err != nil {
		return nil, err
	}

	return NewMergeObserver(spec), nil
}

func (w *IndexWriter) mergeFinish(merge *OneMerge) {
	// minimal implementation
}

func (w *IndexWriter) handleMergeException(err error, merge *OneMerge) {
	merge.SetException(err)
	w.mergeExceptions = append(w.mergeExceptions, err)
}

func (w *IndexWriter) mergeInternal(merge *OneMerge) error {
	merge.InitMerge()
	merge.CheckAborted()

	mergeDir := w.mergeScheduler.WrapForMerge(merge, w.dir)
	context := store.IOContextMerge(merge.GetStoreMergeInfo())
	dirWrapper := store.NewTrackingDirectoryWrapper(mergeDir)

	//- Setup Readers
	readers := make([]spi.CodecReader, 0)
	for _, mr := range merge.GetMergeReader() {
		reader := mr.Reader
		wrappedReader := merge.WrapForMerge(reader)
		readers = append(readers, wrappedReader)
	}

	//- SegmentMerger
	merger := NewSegmentMerger(
		readers,
		merge.Info.SegmentInfo(),
		w.liveConfig.GetInfoStream(),
		dirWrapper,
		w.globalFieldNumberMap,
		context,
		w.mergeScheduler.GetIntraMergeExecutor(merge),
		merge,
	)

	if !merger.ShouldMerge() {
		return nil
	}

	merge.CheckAborted()
	w.commitLock.Lock()
	w.runningMerges[merger] = struct{}{}
	w.commitLock.Unlock()
	merge.MergeStartNS = time.Now().UnixNano()

	err := merger.Merge()
	w.commitLock.Lock()
	delete(w.runningMerges, merger)
	w.commitLock.Unlock()

	if err != nil {
		return err
	}

	merge.SetMergeInfo(spi.NewSegmentCommitInfo(
		merge.Info.SegmentInfo(), 0, 0, -1, -1, -1, util.RandomID(),
	))
	merge.GetMergeInfo().SegmentInfo().SetFiles(dirWrapper.GetCreatedFiles())
	dirWrapper.ClearCreatedFiles()

	// Write SegmentInfo
	codec := LookupCodecByName(w.liveConfig.Codec())
	if err := codec.FieldInfosFormat().Write(w.dir, merge.Info.SegmentInfo(), "", store.IOContextReadOnce); err != nil {
		return err
	}

	// --- WARMING ---
	warmer := w.liveConfig.GetMergedSegmentWarmer()
	if w.liveConfig.GetReaderPooling() && warmer != nil {
		rau := w.getPooledInstance(merge.Info, true)
		sr, err := rau.GetReader()
		if err == nil {
			warmer.Warm(sr)
			rau.Release(sr)
		}
		w.release(rau)
	}
	// ----------------

	if !w.commitMerge(merge, nil) {
		return fmt.Errorf("merge aborted")
	}

	return nil
}

func (w *IndexWriter) dropDeletedSegment(sci *SegmentCommitInfo) {
	w.segmentInfos.Remove(sci)
}

func (w *IndexWriter) isFullyDeleted(rau *ReadersAndUpdates) (bool, error) {
	return rau.IsFullyDeleted()
}

func (w *IndexWriter) tryApply(packet *FrozenBufferedUpdates) error {
	packet.Lock()
	defer packet.Unlock()

	states := make([]*FrozenSegmentState, 0, len(w.segmentInfos.Iterator()))
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

	for _, sci := range w.segmentInfos.Iterator() {
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

	w.segmentInfos.ApplyMergeChanges(merge, false)

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
	w.bufferedUpdatesStream.WaitApplyAll(w)
	return nil
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
		w.writeReaderPool(true)
	}

	stopCollectingMergedReaders := &atomic.Bool{}
	mergedReaders := make(map[string]*SegmentReader)
	openedReadOnlyClones := make(map[string]*SegmentReader)
	openingSegmentInfos := w.segmentInfos.Clone()

	pointInTimeMerges := w.preparePointInTimeMerge(
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

	if len(pointInTimeMerges.Merges) > 0 {
		reader := w.finishGetReaderMerge(
			stopCollectingMergedReaders,
			mergedReaders,
			openedReadOnlyClones,
			openingSegmentInfos,
			applyAllDeletes,
			writeAllDeletes,
			pointInTimeMerges,
			w.liveConfig.GetMaxFullFlushMergeWaitMillis(),
		)
		if reader == nil {
			// Fall back to normal NRT open if merge-based reopen failed
			return w.openNRTReader(openingSegmentInfos, applyAllDeletes, writeAllDeletes)
		}
		return reader, nil
	}

	return w.openNRTReader(openingSegmentInfos, applyAllDeletes, writeAllDeletes)
}

func (w *IndexWriter) openNRTReader(sis *SegmentInfos, applyAllDeletes, writeAllDeletes bool) (*StandardDirectoryReader, error) {
	readerFactory := func(sci *SegmentCommitInfo) (*ReadersAndUpdates, error) {
		return w.getPooledInstance(sci, true), nil
	}

	reader, err := OpenNRT(w, readerFactory, sis, applyAllDeletes, writeAllDeletes)
	if err != nil {
		return nil, fmt.Errorf("failed to open reader: %w", err)
	}

	return reader, nil
}
