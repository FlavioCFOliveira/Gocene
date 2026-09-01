// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"

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

	reader, err := StandardDirectoryReader.Open(w, w.readerPool.GetReaderFactory, w.segmentInfos, applyAllDeletes, true)
	if err != nil {
		return nil, err
	}
	return reader, nil
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

func (w *IndexWriter) maybeProcessEvents(seqNo int64) int64 {
	w.eventQueue.processEvents(w)
	return seqNo
}

func (w *IndexWriter) publishFlushedSegments(applyDeletes bool) {
	// implementation of publishing flushed segments
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
	return nil
}

func (w *IndexWriter) flushFailed(info *SegmentInfo) error {
	return nil
}

func (w *IndexWriter) onTragicEvent(event error, message string) {
	// implementation
}
