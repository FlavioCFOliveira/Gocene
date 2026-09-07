// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FlushNotifications is used to notify the IndexWriter about internal events
// occurring within the DocumentsWriter.
type FlushNotifications interface {
	// DeleteUnusedFiles is called when files were written to disk that are not used anymore.
	DeleteUnusedFiles(files []string)
	// FlushFailed is called when a segment failed to flush.
	FlushFailed(info *SegmentInfo)
	// AfterSegmentsFlushed is called after one or more segments were flushed to disk.
	AfterSegmentsFlushed() error
	// OnTragicEvent is called if a flush or an indexing operation caused a tragic / unrecoverable event.
	OnTragicEvent(event error, message string)
	// OnDeletesApplied is called once deletes have been applied either after a flush or on a deletes call.
	OnDeletesApplied()
	// OnTicketBacklog is called once the DocumentsWriter ticket queue has a backlog.
	OnTicketBacklog()
}

// DocumentsWriter accepts multiple added documents and directly writes segment files.
//
// Each added document is passed to the indexing chain, which in turn processes the document into
// the different codec formats. Some formats write bytes to files immediately, e.g. stored fields
// and term vectors, while others are buffered by the indexing chain and written only on flush.
//
// Once we have used our allowed RAM buffer, or the number of added docs is large enough (in the
// case we are flushing by doc count instead of RAM usage), we create a real segment and flush it to
// the Directory.
//
// Threads:
//
// Multiple threads are allowed into UpdateDocuments at once. There is an initial synchronized call
// to DocumentsWriterFlushControl.ObtainAndLock() which allocates a DWPT for this indexing
// thread. The same thread will not necessarily get the same DWPT over time. Then updateDocuments is
// called on that DWPT without synchronization (most of the "heavy lifting" is in this call). Once a
// DWPT fills up enough RAM or hold enough documents in memory the DWPT is checked out for flush and
// all changes are written to the directory. Each DWPT corresponds to one segment being written.
//
// When flush is called by IndexWriter we check out all DWPTs that are associated with the
// current DocumentsWriterDeleteQueue out of the DocumentsWriterPerThreadPool and
// write them to disk. The flush process can piggyback on incoming indexing threads or even block
// them from adding documents if flushing can't keep up with new documents being added. Unless the
// stall control kicks in to block indexing threads flushes are happening concurrently to actual
// index requests.
//
// Exceptions:
//
// Because this class directly updates in-memory posting lists, and flushes stored fields and
// term vectors directly to files in the directory, there are certain limited times when an
// exception can corrupt this state. For example, a disk full while flushing stored fields leaves
// this file in a corrupt state. Or, an OOM exception while appending to the in-memory posting lists
// can corrupt that posting list. We call such exceptions "aborting exceptions". In these cases we
// must call Abort() to discard all docs added since the last flush.
//
// All other exceptions ("non-aborting exceptions") can still partially update the index
// structures. These updates are consistent, but, they represent only a part of the document seen up
// until the exception was hit. When this happens, we immediately mark the document as deleted so
// that the document is always atomically ("all or none") added to the index.
type DocumentsWriter struct {
	mu sync.Mutex

	pendingNumDocs *atomic.Int64
	flushNotifications FlushNotifications
	closed atomic.Bool
	infoStream util.InfoStream
	config *LiveIndexWriterConfig
	numDocsInRAM *atomic.Int32

	deleteQueue *DocumentsWriterDeleteQueue
	ticketQueue *DocumentsWriterFlushQueue
	pendingChangesInCurrentFullFlush atomic.Bool

	perThreadPool *DocumentsWriterPerThreadPool
	flushControl *DocumentsWriterFlushControl
}

func NewDocumentsWriter(
	flushNotifications FlushNotifications,
	indexCreatedVersionMajor int,
	pendingNumDocs *atomic.Int64,
	enableTestPoints bool,
	segmentNameSupplier func() string,
	config *LiveIndexWriterConfig,
	directoryOrig store.Directory,
	directory store.Directory,
	globalFieldNumberMap *FieldInfos,
) *DocumentsWriter {
	dw := &DocumentsWriter{
		config: config,
		infoStream: config.GetInfoStream(),
		pendingNumDocs: pendingNumDocs,
		flushNotifications: flushNotifications,
		numDocsInRAM: &atomic.Int32{},
	}
	dw.deleteQueue = NewDocumentsWriterDeleteQueue(dw.infoStream)
	dw.perThreadPool = NewDocumentsWriterPerThreadPool(func() *DocumentsWriterPerThread {
		infos := NewFieldInfosBuilder(globalFieldNumberMap)
		return NewDocumentsWriterPerThread(
			dw,
			segmentNameSupplier(),
			directoryOrig,
			directory,
			config,
			dw.deleteQueue,
			infos,
			pendingNumDocs,
			enableTestPoints,
		)
	})
	dw.ticketQueue = NewDocumentsWriterFlushQueue()
	dw.flushControl = NewDocumentsWriterFlushControl(dw, config)
	return dw
}

func (dw *DocumentsWriter) DeleteQueries(queries ...Query) (int64, error) {
	return dw.applyDeleteOrUpdate(func(dq *DocumentsWriterDeleteQueue) int64 {
		return dq.AddDeleteQueries(queries)
	})
}

func (dw *DocumentsWriter) DeleteTerms(terms ...Term) (int64, error) {
	return dw.applyDeleteOrUpdate(func(dq *DocumentsWriterDeleteQueue) int64 {
		var lastSeq int64
		for _, t := range terms {
			lastSeq = dq.Add(NewTermNode(t))
		}
		return lastSeq
	})
}

func (dw *DocumentsWriter) UpdateDocValues(updates ...DocValuesUpdate) (int64, error) {
	return dw.applyDeleteOrUpdate(func(dq *DocumentsWriterDeleteQueue) int64 {
		return dq.Add(NewDocValuesUpdatesNode(updates))
	})
}

func (dw *DocumentsWriter) applyDeleteOrUpdate(function func(*DocumentsWriterDeleteQueue) int64) (int64, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	dq := dw.deleteQueue
	seqNo := function(dq)
	dw.flushControl.DoOnDelete()
	if applied, err := dw.applyAllDeletes(); err != nil {
		return 0, err
	} else if applied {
		seqNo = -seqNo
	}
	return seqNo, nil
}

func (dw *DocumentsWriter) applyAllDeletes() (bool, error) {
	dq := dw.deleteQueue

	if dw.flushControl.GetApplyAllDeletes() &&
		!dw.flushControl.IsFullFlush() &&
		dq.isOpen() &&
		dw.flushControl.GetAndResetApplyAllDeletes() {

		ticket, err := dw.ticketQueue.AddTicket(func() (*FlushQueueTicket, error) {
			frozen := dq.MaybeFreezeGlobalBuffer()
			if frozen == nil {
				return nil, nil
			}
			return NewFlushQueueTicket(frozen, false), nil
		})
		if err != nil {
			return false, err
		}
		if ticket != nil {
			dw.flushNotifications.OnDeletesApplied()
			return true, nil
		}
	}
	return false, nil
}

func (dw *DocumentsWriter) PurgeFlushTickets(forced bool, consumer func(*FlushQueueTicket) error) error {
	if forced {
		return dw.ticketQueue.ForcePurge(consumer)
	}
	return dw.ticketQueue.TryPurge(consumer)
}

func (dw *DocumentsWriter) GetNumDocs() int {
	return int(dw.numDocsInRAM.Load())
}

func (dw *DocumentsWriter) ensureOpen() error {
	if dw.closed.Load() {
		return store.NewAlreadyClosedException("this DocumentsWriter is closed", nil)
	}
	return nil
}

func (dw *DocumentsWriter) Abort() error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	success := false
	defer func() {
		if dw.infoStream.IsEnabled("DW") {
			dw.infoStream.Message("DW", fmt.Sprintf("done abort success=%v", success))
		}
	}()

	dw.deleteQueue.Clear()
	if dw.infoStream.IsEnabled("DW") {
		dw.infoStream.Message("DW", "abort")
	}

	for _, perThread := range dw.perThreadPool.FilterAndLock(func(x *DocumentsWriterPerThread) bool { return true }) {
		err := func() error {
			defer perThread.Unlock()
			return dw.abortDocumentsWriterPerThread(perThread)
		}()
		if err != nil {
			return err
		}
	}

	dw.flushControl.AbortPendingFlushes()
	dw.flushControl.WaitForFlush()

	success = true
	return nil
}

func (dw *DocumentsWriter) FlushOneDWPT() (bool, error) {
	if dw.infoStream.IsEnabled("DW") {
		dw.infoStream.Message("DW", "startFlushOneDWPT")
	}
	if flushed, err := dw.maybeFlush(); err != nil {
		return false, err
	} else if flushed {
		return true, nil
	}

	perThread := dw.flushControl.CheckoutLargestNonPendingWriter()
	if perThread != nil {
		if err := dw.doFlush(perThread); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (dw *DocumentsWriter) LockAndAbortAll() (io.Closer, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	if dw.infoStream.IsEnabled("DW") {
		dw.infoStream.Message("DW", "lockAndAbortAll")
	}

	dw.ticketQueue.ForcePurge(func(ticket *FlushQueueTicket) error {
		if seg := ticket.GetFlushedSegment(); seg != nil {
			dw.pendingNumDocs.Add(-int64(seg.SegmentInfo.Info.MaxDoc()))
		}
		return nil
	})

	var writers []*DocumentsWriterPerThread
	released := atomic.Bool{}

	release := func() error {
		if released.CompareAndSwap(false, true) {
			if dw.infoStream.IsEnabled("DW") {
				dw.infoStream.Message("DW", "unlockAllAbortedThread")
			}
			dw.perThreadPool.UnlockNewWriters()
			for _, writer := range writers {
				writer.Unlock()
			}
		}
		return nil
	}

	closer := &abortReleaseCloser{release: release}

	defer func() {
		if r := recover(); r != nil {
			_ = release()
			panic(r)
		}
	}()

	dw.deleteQueue.Clear()
	dw.perThreadPool.LockNewWriters()
	writers = dw.perThreadPool.FilterAndLock(func(x *DocumentsWriterPerThread) bool { return true })
	for _, perThread := range writers {
		if err := dw.abortDocumentsWriterPerThread(perThread); err != nil {
			_ = release()
			return nil, err
		}
	}
	dw.deleteQueue.Clear()
	dw.deleteQueue.SkipSequenceNumbers(len(writers) + 1)

	dw.flushControl.AbortPendingFlushes()
	dw.flushControl.WaitForFlush()

	if dw.infoStream.IsEnabled("DW") {
		dw.infoStream.Message("DW", "finished lockAndAbortAll success=true")
	}
	return closer, nil
}

type abortReleaseCloser struct {
	release func() error
}

func (a *abortReleaseCloser) Close() error {
	return a.release()
}

func (dw *DocumentsWriter) abortDocumentsWriterPerThread(perThread *DocumentsWriterPerThread) error {
	defer dw.flushControl.DoOnAbort(perThread)
	dw.SubtractFlushedNumDocs(perThread.GetNumDocs())
	return perThread.Abort()
}

func (dw *DocumentsWriter) GetMaxCompletedSequenceNumber() int64 {
	return dw.deleteQueue.GetMaxCompletedSeqNo()
}

func (dw *DocumentsWriter) AnyChanges() bool {
	anyChanges := dw.numDocsInRAM.Load() != 0 ||
		dw.AnyDeletions() ||
		dw.ticketQueue.HasTickets() ||
		dw.pendingChangesInCurrentFullFlush.Load()

	if dw.infoStream.IsEnabled("DW") && anyChanges {
		dw.infoStream.Message("DW", fmt.Sprintf(
			"anyChanges? numDocsInRam=%d deletes=%v hasTickets:%v pendingChangesInFullFlush: %v",
			dw.numDocsInRAM.Load(), dw.AnyDeletions(), dw.ticketQueue.HasTickets(), dw.pendingChangesInCurrentFullFlush.Load()))
	}
	return anyChanges
}

func (dw *DocumentsWriter) GetBufferedDeleteTermsSize() int {
	return dw.deleteQueue.GetBufferedUpdatesTermsSize()
}

func (dw *DocumentsWriter) AnyDeletions() bool {
	return dw.deleteQueue.AnyChanges()
}

func (dw *DocumentsWriter) Close() error {
	dw.closed.Store(true)
	_ = dw.flushControl.Close()
	dw.perThreadPool.Close()
	return nil
}

func (dw *DocumentsWriter) preUpdate() (bool, error) {
	if err := dw.ensureOpen(); err != nil {
		return false, err
	}
	hasEvents := false
	for dw.flushControl.AnyStalledThreads() ||
		(dw.config.IsCheckPendingFlushOnUpdate() && dw.flushControl.NumQueuedFlushes() > 0) {
		flushed, err := dw.maybeFlush()
		if err != nil {
			return false, err
		}
		hasEvents = hasEvents || flushed
		dw.flushControl.WaitIfStalled()
	}
	return hasEvents, nil
}

func (dw *DocumentsWriter) postUpdate(flushingDWPT *DocumentsWriterPerThread, hasEvents bool) (bool, error) {
	applied, err := dw.applyAllDeletes()
	if err != nil {
		return false, err
	}
	hasEvents = hasEvents || applied

	if flushingDWPT != nil {
		if err := dw.doFlush(flushingDWPT); err != nil {
			return false, err
		}
		hasEvents = true
	} else if dw.config.IsCheckPendingFlushOnUpdate() {
		flushed, err := dw.maybeFlush()
		if err != nil {
			return false, err
		}
		hasEvents = hasEvents || flushed
	}
	return hasEvents, nil
}

func (dw *DocumentsWriter) UpdateDocuments(docs [][]IndexableField, delNode Node) (int64, error) {
	hasEvents, err := dw.preUpdate()
	if err != nil {
		return 0, err
	}

	dwpt, err := dw.flushControl.ObtainAndLock()
	if err != nil {
		return 0, err
	}

	var flushingDWPT *DocumentsWriterPerThread
	var seqNo int64

	func() {
		defer func() {
			dw.mu.Lock()
			if dwpt.IsFlushPending() || dwpt.IsAborted() || dwpt.IsQueueAdvanced() {
				dwpt.Unlock()
			} else {
				dw.perThreadPool.MarksAsFreeAndUnlock(dwpt)
			}
			dw.mu.Unlock()
		}()

		if err := dw.ensureOpen(); err != nil {
			panic(err)
		}

		seqNo, err = dwpt.UpdateDocuments(docs, delNode, dw.flushNotifications, func() {
			dw.numDocsInRAM.Add(1)
		})
		if err != nil {
			panic(err)
		}

		if dwpt.IsAborted() {
			dw.flushControl.DoOnAbort(dwpt)
		}
		flushingDWPT = dw.flushControl.DoAfterDocument(dwpt)
	}()

	if err != nil {
		return 0, err
	}

	if events, err := dw.postUpdate(flushingDWPT, hasEvents); err != nil {
		return 0, err
	} else if events {
		seqNo = -seqNo
	}
	return seqNo, nil
}

func (dw *DocumentsWriter) UpdateBatch(columnBatch *ColumnBatch, delNode Node) (int64, error) {
	hasEvents, err := dw.preUpdate()
	if err != nil {
		return 0, err
	}

	dwpt, err := dw.flushControl.ObtainAndLock()
	if err != nil {
		return 0, err
	}

	var flushingDWPT *DocumentsWriterPerThread
	var seqNo int64

	func() {
		defer func() {
			dw.mu.Lock()
			if dwpt.IsFlushPending() || dwpt.IsAborted() || dwpt.IsQueueAdvanced() {
				dwpt.Unlock()
			} else {
				dw.perThreadPool.MarksAsFreeAndUnlock(dwpt)
			}
			dw.mu.Unlock()
		}()

		if err := dw.ensureOpen(); err != nil {
			panic(err)
		}

		seqNo, err = dwpt.UpdateBatch(columnBatch, delNode, dw.flushNotifications, func(n int) {
			dw.numDocsInRAM.Add(int32(n))
		})
		if err != nil {
			panic(err)
		}

		if dwpt.IsAborted() {
			dw.flushControl.DoOnAbort(dwpt)
		}
		flushingDWPT = dw.flushControl.DoAfterDocument(dwpt)
	}()

	if err != nil {
		return 0, err
	}

	if events, err := dw.postUpdate(flushingDWPT, hasEvents); err != nil {
		return 0, err
	} else if events {
		seqNo = -seqNo
	}
	return seqNo, nil
}

func (dw *DocumentsWriter) maybeFlush() (bool, error) {
	flushingDWPT := dw.flushControl.NextPendingFlush()
	if flushingDWPT != nil {
		if err := dw.doFlush(flushingDWPT); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (dw *DocumentsWriter) doFlush(flushingDWPT *DocumentsWriterPerThread) error {
	if flushingDWPT == nil {
		return fmt.Errorf("Flushing DWPT must not be null")
	}
	for {
		if flushingDWPT.HasFlushed() {
			break
		}
		success := false
		var ticket *FlushQueueTicket

		func() {
			defer func() {
				if !success && ticket != nil {
					dw.ticketQueue.MarkTicketFailed(ticket)
				}
			}()

			ticket, err := dw.ticketQueue.AddTicket(func() (*FlushQueueTicket, error) {
				frozen, err := flushingDWPT.PrepareFlush()
				if err != nil {
					return nil, err
				}
				return NewFlushQueueTicket(frozen, true), nil
			})
			if err != nil {
				panic(err)
			}

			flushingDocsInRam := flushingDWPT.GetNumDocs()
			dwptSuccess := false
			func() {
				defer func() {
					dw.SubtractFlushedNumDocs(flushingDocsInRam)
					if files := flushingDWPT.PendingFilesToDelete(); len(files) > 0 {
						dw.flushNotifications.DeleteUnusedFiles(files)
					}
					if !dwptSuccess {
						dw.flushNotifications.FlushFailed(flushingDWPT.SegmentInfo)
					}
				}()

				seg, err := flushingDWPT.Flush(dw.perThreadPool.Directory, dw.perThreadPool.Codec, flushingDWPT.SegmentInfo.Name())
				if err != nil {
					panic(err)
				}
				if seg != nil {
					_ = dw.ticketQueue.AddSegment(ticket, seg)
					dwptSuccess = true
				}
			}()
			success = true
		}()

		if dw.ticketQueue.GetTicketCount() >= dw.perThreadPool.Size() {
			dw.flushNotifications.OnTicketBacklog()
		}

		dw.flushControl.DoAfterFlush(flushingDWPT)

		flushingDWPT = dw.flushControl.NextPendingFlush()
		if flushingDWPT == nil {
			break
		}
	}
	dw.flushNotifications.AfterSegmentsFlushed()
	return nil
}

func (dw *DocumentsWriter) GetNextSequenceNumber() int64 {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	return dw.deleteQueue.GetNextSequenceNumber()
}

func (dw *DocumentsWriter) ResetDeleteQueue(maxNumPendingOps int) int64 {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	oldMaxSeqNo := dw.deleteQueue.GetMaxSeqNo()
	dw.deleteQueue = dw.deleteQueue.AdvanceQueue(maxNumPendingOps)
	return oldMaxSeqNo
}

func (dw *DocumentsWriter) SubtractFlushedNumDocs(numFlushed int) {
	oldValue := dw.numDocsInRAM.Load()
	for {
		if dw.numDocsInRAM.CompareAndSwap(oldValue, oldValue-int32(numFlushed)) {
			break
		}
		oldValue = dw.numDocsInRAM.Load()
	}
}

func (dw *DocumentsWriter) FlushAllThreads() (int64, error) {
	if dw.infoStream.IsEnabled("DW") {
		dw.infoStream.Message("DW", "startFullFlush")
	}

	var flushingDeleteQueue *DocumentsWriterDeleteQueue
	var seqNo int64

	dw.mu.Lock()
	dw.pendingChangesInCurrentFullFlush.Store(dw.AnyChanges())
	flushingDeleteQueue = dw.deleteQueue
	seqNo = dw.flushControl.MarkForFullFlush()
	dw.mu.Unlock()

	anythingFlushed := false
	func() {
		defer func() {
			flushingDeleteQueue.Close()
		}()

		flushed, err := dw.maybeFlush()
		if err != nil {
			panic(err)
		}
		anythingFlushed = anythingFlushed || flushed

		dw.flushControl.WaitForFlush()

		if !anythingFlushed && flushingDeleteQueue.AnyChanges() {
			if dw.infoStream.IsEnabled("DW") {
				dw.infoStream.Message("DW", "flush naked frozen global deletes")
			}
			_, _ = dw.ticketQueue.AddTicket(func() (*FlushQueueTicket, error) {
				frozen := flushingDeleteQueue.MaybeFreezeGlobalBuffer()
				if frozen == nil {
					return nil, nil
				}
				return NewFlushQueueTicket(frozen, false), nil
			})
		}
	}()

	if anythingFlushed {
		return -seqNo, nil
	}
	return seqNo, nil
}

func (dw *DocumentsWriter) FinishFullFlush(success bool) error {
	defer func() {
		dw.pendingChangesInCurrentFullFlush.Store(false)
		_, _ = dw.applyAllDeletes()
	}()

	if dw.infoStream.IsEnabled("DW") {
		dw.infoStream.Message("DW", fmt.Sprintf("finishFullFlush success=%v", success))
	}

	if success {
		dw.flushControl.FinishFullFlush()
	} else {
		dw.flushControl.AbortFullFlushes()
	}
	return nil
}

func (dw *DocumentsWriter) RAMBytesUsed() int64 {
	return dw.flushControl.RAMBytesUsed()
}

func (dw *DocumentsWriter) GetFlushingBytes() int64 {
	return dw.flushControl.GetFlushingBytes()
}
