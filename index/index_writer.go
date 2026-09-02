// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexWriter creates and maintains an index.
//
// This is the Go port of Lucene's org.apache.lucene.index.IndexWriter.
type IndexWriter struct {
	config *IndexWriterConfig

	dirOrig util.Directory // original user directory
	dir     util.Directory // wrapped with additional checks

	// increments every time a change is completed
	changeCount atomic.Int64
	// last changeCount that was committed
	lastCommitChangeCount atomic.Int64

	// list of segmentInfo we will fallback to if the commit fails
	rollbackSegments *SegmentInfos

	// set when a commit is pending (after PrepareCommit() & before Commit())
	pendingCommit *SegmentInfos
	pendingSeqNo  int64
	pendingCommitChangeCount int64

	filesToCommit []string

	segmentInfos *SegmentInfos
	globalFieldNumberMap *FieldNumbers

	docWriter *DocumentsWriter
	eventQueue *eventQueue
	mergeSource *indexWriterMergeSource
	addIndexesMergeSource *addIndexesMergeSource

	writeDocValuesLock sync.Mutex

	deleter *IndexFileDeleter

	segmentsToMerge map[*SegmentCommitInfo]bool
	mergeMaxNumSegments int

	writeLock util.Lock

	closed  atomic.Bool
	closing atomic.Bool

	maybeMerge atomic.Bool

	commitUserData []mapEntry[string, string]

	mergingSegments map[*SegmentCommitInfo]struct{}
	mergeScheduler MergeScheduler
	runningAddIndexesMerges map[*SegmentMerger]struct{}
	pendingMerges []MergePolicy.OneMerge
	runningMerges map[MergePolicy.OneMerge]struct{}
	mergeExceptions []error
	merges *merges
	mergeGen int64
	didMessageState bool
	flushCount atomic.Int32
	flushDeletesCount atomic.Int32
	readerPool *ReaderPool
	bufferedUpdatesStream *BufferedUpdatesStream

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
func NewIndexWriter(d util.Directory, conf *IndexWriterConfig) (*IndexWriter, error) {
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
	return "_" + util.LongToString(w.segmentInfos.Counter(), 36)
}

func (w *IndexWriter) changed() {
	w.segmentInfos.mu.Lock()
	defer w.segmentInfos.mu.Unlock()
	w.segmentInfos.Changed()
}

func (w *IndexWriter) ensureOpen() {
	if w.closed.Load() {
		panic("IndexWriter is closed")
	}
}

func (w *IndexWriter) AddDocument(doc []IndexableField) (int64, error) {
	w.ensureOpen()
	seqNo, err := w.docWriter.UpdateDocuments([]([]IndexableField){doc}, nil)
	if err != nil {
		return 0, err
	}
	return w.maybeProcessEvents(seqNo), nil
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
	w.fullFlushLock.Lock()
	defer w.fullFlushLock.Unlock()

	seqNo := w.docWriter.FlushAllThreads()
	anyChanges := false
	if seqNo < 0 {
		anyChanges = true
		seqNo = -seqNo
	}

	w.publishFlushedSegments(true)
	w.eventQueue.processEvents(w)

	w.applyAllDeletesAndUpdates()

	w.writeReaderPool(true)

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

	committedSegmentsFileName, err := w.pendingCommit.FinishCommit(w.dir)
	if err != nil {
		return err
	}

	w.deleter.Checkpoint(w.pendingCommit, true)
	w.segmentInfos.UpdateGeneration(w.pendingCommit)
	w.lastCommitChangeCount.Store(w.pendingCommitChangeCount)
	w.rollbackSegments = w.pendingCommit.Clone()

	w.pendingCommit = nil
	_ = committedSegmentsFileName
	return nil
}

func (w *IndexWriter) GetReader(applyAllDeletes bool) (*DirectoryReader, error) {
	w.ensureOpen()
	w.fullFlushLock.Lock()
	defer w.fullFlushLock.Unlock()

	w.docWriter.FlushAllThreads()
	w.publishFlushedSegments(true)
	w.applyAllDeletesAndUpdates()
	w.writeReaderPool(true)

	w.finishGetReaderMerge()

	reader, err := StandardDirectoryReader.Open(w, w.readerPool.GetReaderFactory, w.segmentInfos, applyAllDeletes, true)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func (w *IndexWriter) finishGetReaderMerge() {
	waitMillis := w.liveConfig.GetMaxFullFlushMergeWaitMillis()
	if waitMillis <= 0 {
		return
	}

	w.mergeScheduler.Merge(w.mergeSource, MergeTriggerGetReader)

	start := time.Now()
	for w.mergeScheduler.GetRunningMergeCount() > 0 {
		if time.Since(start).Milliseconds() >= waitMillis {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (w *IndexWriter) GetConfig() *IndexWriterConfig {
	return w.config
}

func (w *IndexWriter) GetAnalyzer() analysis.Analyzer {
	return w.config.analyzer
}

func (w *IndexWriter) GetDirectory() util.Directory {
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
	w.mergeScheduler.Merge(w.mergeSource, trigger)
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
	codec := si.Info.GetCodec()
	reader := codec.FieldInfosFormat()
	if si.HasFieldUpdates() {
		suffix := util.LongToString(si.GetFieldInfosGen(), 36)
		return reader.Read(si.Info.Dir, si.Info, suffix, util.ReadOnce)
	} else if si.Info.GetUseCompoundFile() {
		cfs := codec.CompoundFormat().GetCompoundReader(si.Info.Dir, si.Info)
		defer cfs.Close()
		return reader.Read(cfs, si.Info, "", util.ReadOnce)
	}
	return reader.Read(si.Info.Dir, si.Info, "", util.ReadOnce)
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

func (w *IndexWriter) onTragicEvent(event error, message string) {
	fmt.Printf("IndexWriter: hit tragic %v inside %s\n", event, message)
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
		fieldInfo = fieldInfos.GetByName(softDeletesField)
	}

	hasInitialSoftDeleted := false
	if fieldInfo != nil && fieldInfo.DocValuesGen() == -1 && fieldInfo.DocValuesType() != DocValuesTypeNone {
		hasInitialSoftDeleted = true
	}
	isFullyHardDeleted := newSegment.GetDelCount() == newSegment.Info.DocCount()

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

func (w *IndexWriter) ForceMergeDeletes() error {
	_, err := w.ForceMergeDeletesWithObserver(true)
	return err
}

// ForceMergeDeletesWithObserver executes a merge to expunge all deletes from the index.
// Returns a MergeObserver to monitor progress.
func (w *IndexWriter) ForceMergeDeletesWithObserver(doWait bool) (*MergePolicy.MergeObserver, error) {
	w.ensureOpen()
	w.commitLock.Lock()
	defer w.commitLock.Unlock()

	spec := w.config.GetMergePolicy().FindForcedDeletesMerges(w.segmentInfos, w.mergeSource)
	if spec == nil {
		return index.NewMergeObserver(nil), nil
	}

	// register merges with the scheduler
	if err := w.mergeScheduler.MergeWithSpec(w.mergeSource, spec, doWait); err != nil {
		return nil, err
	}

	return index.NewMergeObserver(spec), nil
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
	merger := index.NewSegmentMerger(
		readers,
		merge.Info.Info,
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
		merge.Info.Info, 0, 0, -1, -1, -1, util.RandomID(),
	))
	merge.GetMergeInfo().Info.SetFiles(dirWrapper.GetCreatedFiles())
	dirWrapper.ClearCreatedFiles()

	// Write SegmentInfo
	codec := w.liveConfig.GetCodec()
	if err := codec.FieldInfosFormat().Write(w.dir, merge.Info.Info, "", util.ReadOnce); err != nil {
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
	return nil
}

func (w *IndexWriter) getPooledInstance(sci *SegmentCommitInfo, writeDeletes bool) *ReadersAndUpdates {
	rau, err := NewReadersAndUpdates(w.config.GetIndexCreatedVersionMajor(), sci, NewPendingDeletes())
	if err != nil {
		return nil
	}
	return rau
}

func (w *IndexWriter) release(rau *ReadersAndUpdates) {
	rau.DecRef()
}

func (w *IndexWriter) commitMerge(merge *OneMerge, docMaps []DocMap) bool {
	if merge.IsAborted() {
		return false
	}

	w.segmentInfos.ApplyMergeChanges(merge, false)

	// Adjust pendingNumDocs
	delDocCount := merge.TotalMaxDoc - merge.Info.Info.MaxDoc()
	w.adjustPendingNumDocs(-delDocCount)

	return true
}

