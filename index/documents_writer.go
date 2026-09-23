// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"io"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/index/column"
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
	// lock is Java's DocumentsWriter monitor -- the object every
	// `synchronized` method and `synchronized (this)` block on
	// DocumentsWriter locks. Java monitors are reentrant and Lucene relies on
	// it: flushAllThreads() runs `synchronized (this) { ...
	// flushControl.markForFullFlush() ... }` and markForFullFlush() calls back
	// into the `synchronized` resetDeleteQueue(), acquiring the same monitor a
	// second time on the same thread. The holder identity Java reads from
	// Thread.currentThread() is passed explicitly as a util.LockOwner; see
	// util/reentrant_lock.go.
	lock util.ReentrantLock

	pendingNumDocs     *atomic.Int64
	flushNotifications FlushNotifications
	closed             atomic.Bool
	infoStream         util.InfoStream
	config             *LiveIndexWriterConfig
	numDocsInRAM       *atomic.Int32

	deleteQueue                      *DocumentsWriterDeleteQueue
	ticketQueue                      *DocumentsWriterFlushQueue
	pendingChangesInCurrentFullFlush atomic.Bool

	perThreadPool *DocumentsWriterPerThreadPool
	flushControl  *DocumentsWriterFlushControl
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
	globalFieldNumberMap *FieldNumbers,
) *DocumentsWriter {
	dw := &DocumentsWriter{
		config:             config,
		infoStream:         config.GetInfoStream(),
		pendingNumDocs:     pendingNumDocs,
		flushNotifications: flushNotifications,
		numDocsInRAM:       &atomic.Int32{},
	}
	dw.deleteQueue = NewDocumentsWriterDeleteQueue(dw.infoStream)
	dw.perThreadPool = NewDocumentsWriterPerThreadPool(func() *DocumentsWriterPerThread {
		infos := NewFieldInfosBuilderFor(globalFieldNumberMap)
		return NewDocumentsWriterPerThread(
			indexCreatedVersionMajor,
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
	return dw.applyDeleteOrUpdate(func(dq *DocumentsWriterDeleteQueue) (int64, error) {
		return dq.AddDelete(queries...)
	})
}

func (dw *DocumentsWriter) DeleteTerms(terms ...Term) (int64, error) {
	return dw.applyDeleteOrUpdate(func(dq *DocumentsWriterDeleteQueue) (int64, error) {
		return dq.AddDeleteTerms(terms...)
	})
}

func (dw *DocumentsWriter) UpdateDocValues(updates ...DocValuesUpdate) (int64, error) {
	return dw.applyDeleteOrUpdate(func(dq *DocumentsWriterDeleteQueue) (int64, error) {
		return dq.AddDocValuesUpdates(updates...)
	})
}

func (dw *DocumentsWriter) applyDeleteOrUpdate(function func(*DocumentsWriterDeleteQueue) (int64, error)) (int64, error) {
	owner := util.NewLockOwner()
	dw.lock.Lock(owner)
	defer dw.lock.Unlock(owner)

	dq := dw.deleteQueue
	seqNo, err := function(dq)
	if err != nil {
		return 0, err
	}
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
		dq.IsOpen() &&
		dw.flushControl.GetAndResetApplyAllDeletes() {

		ticket, err := dw.ticketQueue.AddTicket(func() (*FlushTicket, error) {
			frozen := dq.MaybeFreezeGlobalBuffer()
			if frozen == nil {
				return nil, nil
			}
			return NewFlushTicket(frozen, false), nil
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

func (dw *DocumentsWriter) PurgeFlushTickets(forced bool, consumer func(*FlushTicket) error) error {
	if forced {
		return dw.ticketQueue.ForcePurge(consumer)
	}
	return dw.ticketQueue.TryPurge(consumer)
}

func (dw *DocumentsWriter) GetNumDocs() int {
	return int(dw.numDocsInRAM.Load())
}

// GetDeleteQueue returns the delete queue this writer is currently bound to.
//
// Mirrors the package-private DocumentsWriter.deleteQueue field, which Java
// reads without synchronization from DocumentsWriterFlushControl. The field is
// only reassigned by ResetDeleteQueue under dw.mu, so the read is left
// unsynchronized here too: taking dw.mu would invert the lock order against
// DocumentsWriterFlushControl.mu.
func (dw *DocumentsWriter) GetDeleteQueue() *DocumentsWriterDeleteQueue {
	return dw.deleteQueue
}

// GetPerThreadPool returns the per-thread pool backing this writer. Mirrors the
// package-private DocumentsWriter.perThreadPool field.
func (dw *DocumentsWriter) GetPerThreadPool() *DocumentsWriterPerThreadPool {
	return dw.perThreadPool
}

func (dw *DocumentsWriter) ensureOpen() error {
	if dw.closed.Load() {
		return store.NewAlreadyClosedException("this DocumentsWriter is closed", nil)
	}
	return nil
}

func (dw *DocumentsWriter) Abort() error {
	owner := util.NewLockOwner()
	dw.lock.Lock(owner)
	defer dw.lock.Unlock(owner)

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

	for _, perThread := range dw.perThreadPool.FilterAndLock(owner, func(x *DocumentsWriterPerThread) bool { return true }) {
		err := func() error {
			defer perThread.Unlock(owner)
			return dw.abortDocumentsWriterPerThread(owner, perThread)
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
	owner := util.NewLockOwner()
	dw.lock.Lock(owner)
	defer dw.lock.Unlock(owner)

	if dw.infoStream.IsEnabled("DW") {
		dw.infoStream.Message("DW", "lockAndAbortAll")
	}

	dw.ticketQueue.ForcePurge(func(ticket *FlushTicket) error {
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
				writer.Unlock(owner)
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
	writers = dw.perThreadPool.FilterAndLock(owner, func(x *DocumentsWriterPerThread) bool { return true })
	for _, perThread := range writers {
		if err := dw.abortDocumentsWriterPerThread(owner, perThread); err != nil {
			_ = release()
			return nil, err
		}
	}
	dw.deleteQueue.Clear()
	dw.deleteQueue.SkipSequenceNumbers(int64(len(writers) + 1))

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

func (dw *DocumentsWriter) abortDocumentsWriterPerThread(owner util.LockOwner, perThread *DocumentsWriterPerThread) error {
	defer dw.flushControl.DoOnAbort(owner, perThread)
	dw.SubtractFlushedNumDocs(perThread.GetNumDocsInRAM())
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

	owner := util.NewLockOwner()
	dwpt, err := dw.flushControl.ObtainAndLock(owner)
	if err != nil {
		return 0, err
	}

	var flushingDWPT *DocumentsWriterPerThread
	var seqNo int64

	func() {
		defer func() {
			dw.lock.Lock(owner)
			if dwpt.IsFlushPending() || dwpt.IsAborted() || dwpt.IsQueueAdvanced() {
				dwpt.Unlock(owner)
			} else {
				dw.perThreadPool.MarksAsFreeAndUnlock(owner, dwpt)
			}
			dw.lock.Unlock(owner)
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
			dw.flushControl.DoOnAbort(owner, dwpt)
		}
		flushingDWPT = dw.flushControl.DoAfterDocument(owner, dwpt)
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

func (dw *DocumentsWriter) UpdateBatch(columnBatch *column.ColumnBatch, delNode Node) (int64, error) {
	hasEvents, err := dw.preUpdate()
	if err != nil {
		return 0, err
	}

	owner := util.NewLockOwner()
	dwpt, err := dw.flushControl.ObtainAndLock(owner)
	if err != nil {
		return 0, err
	}

	var flushingDWPT *DocumentsWriterPerThread
	var seqNo int64

	func() {
		defer func() {
			dw.lock.Lock(owner)
			if dwpt.IsFlushPending() || dwpt.IsAborted() || dwpt.IsQueueAdvanced() {
				dwpt.Unlock(owner)
			} else {
				dw.perThreadPool.MarksAsFreeAndUnlock(owner, dwpt)
			}
			dw.lock.Unlock(owner)
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
			dw.flushControl.DoOnAbort(owner, dwpt)
		}
		flushingDWPT = dw.flushControl.DoAfterDocument(owner, dwpt)
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
		return fmt.Errorf("flushing DWPT must not be null")
	}
	for {
		if err := dw.doFlushOne(flushingDWPT); err != nil {
			return err
		}
		flushingDWPT = dw.flushControl.NextPendingFlush()
		if flushingDWPT == nil {
			break
		}
	}
	return dw.flushNotifications.AfterSegmentsFlushed()
}

// doFlushOne is one iteration of the do/while body of
// DocumentsWriter.doFlush. It is a separate function so the two nested
// try/finally blocks of the Java original map onto deferred calls with the
// same unwinding order: the ticket-failure guard runs first, then
// flushControl.doAfterFlush.
func (dw *DocumentsWriter) doFlushOne(flushingDWPT *DocumentsWriterPerThread) (err error) {
	// Java: finally { flushControl.doAfterFlush(flushingDWPT); }
	defer dw.flushControl.DoAfterFlush(flushingDWPT)

	success := false
	var ticket *FlushTicket
	// Java: finally { if (!success && ticket != null) markTicketFailed(ticket); }
	// In the case of a failure make sure we are making progress and apply all
	// the deletes since the segment flush failed, because the flush ticket
	// could hold global deletes — see FlushTicket#canPublish().
	defer func() {
		if !success && ticket != nil {
			dw.ticketQueue.MarkTicketFailed(ticket)
		}
	}()

	/*
	 * Since with DWPT the flush process is concurrent and several DWPT could
	 * flush at the same time we must maintain the order of the flushes before
	 * we can apply the flushed segment and the frozen global deletes it is
	 * buffering. The reason for this is that the global deletes mark a certain
	 * point in time where we took a DWPT out of rotation and froze the global
	 * deletes.
	 *
	 * Example: a flush 'A' starts and freezes the global deletes, then flush
	 * 'B' starts and freezes all deletes that occurred since 'A' started. If
	 * 'B' finishes before 'A' we need to wait until 'A' is done, otherwise the
	 * deletes frozen by 'B' are not applied to 'A' and we might fail to delete
	 * documents in 'A'.
	 */
	dwpt := flushingDWPT
	// Each flush is assigned a ticket in the order they acquire the ticketQueue lock.
	ticket, err = dw.ticketQueue.AddTicket(func() (*FlushTicket, error) {
		frozen, ferr := dwpt.PrepareFlush()
		if ferr != nil {
			return nil, ferr
		}
		return NewFlushTicket(frozen, true), nil
	})
	if err != nil {
		return err
	}

	if err := dw.flushAndAddSegment(flushingDWPT, ticket); err != nil {
		return err
	}
	// The flush was successful once we reach this point — the new segment has
	// been assigned to the ticket.
	success = true

	if dw.ticketQueue.GetTicketCount() >= dw.perThreadPool.Size() {
		// This means there is a backlog: the one goroutine in innerPurge can't
		// keep up with all the other goroutines flushing segments. In this case
		// we forcefully stall the producers.
		dw.flushNotifications.OnTicketBacklog()
	}
	return nil
}

// flushAndAddSegment performs the concurrent (unlocked) flush of one DWPT and
// hands the resulting segment to its ticket. It carries the inner
// try/finally of DocumentsWriter.doFlush that subtracts the flushed docs,
// reports files to delete and signals a failed flush.
func (dw *DocumentsWriter) flushAndAddSegment(
	flushingDWPT *DocumentsWriterPerThread,
	ticket *FlushTicket,
) (err error) {
	flushingDocsInRAM := flushingDWPT.GetNumDocsInRAM()
	dwptSuccess := false
	defer func() {
		dw.SubtractFlushedNumDocs(flushingDocsInRAM)
		if files := flushingDWPT.PendingFilesToDelete(); len(files) > 0 {
			dw.flushNotifications.DeleteUnusedFiles(files)
		}
		if !dwptSuccess {
			dw.flushNotifications.FlushFailed(flushingDWPT.GetSegmentInfo())
		}
	}()

	// Flush concurrently, without holding the DocumentsWriter monitor.
	newSegment, err := flushingDWPT.Flush(dw.flushNotifications)
	if err != nil {
		return err
	}
	dw.ticketQueue.AddSegment(ticket, newSegment)
	dwptSuccess = true
	return nil
}

func (dw *DocumentsWriter) GetNextSequenceNumber() int64 {
	owner := util.NewLockOwner()
	dw.lock.Lock(owner)
	defer dw.lock.Unlock(owner)
	return dw.deleteQueue.GetNextSequenceNumber()
}

func (dw *DocumentsWriter) ResetDeleteQueue(owner util.LockOwner, maxNumPendingOps int) int64 {
	dw.lock.Lock(owner)
	defer dw.lock.Unlock(owner)

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

	owner := util.NewLockOwner()
	dw.lock.Lock(owner)
	dw.pendingChangesInCurrentFullFlush.Store(dw.AnyChanges())
	flushingDeleteQueue = dw.deleteQueue
	seqNo = dw.flushControl.MarkForFullFlush(owner)
	dw.lock.Unlock(owner)

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
			_, _ = dw.ticketQueue.AddTicket(func() (*FlushTicket, error) {
				frozen := flushingDeleteQueue.MaybeFreezeGlobalBuffer()
				if frozen == nil {
					return nil, nil
				}
				return NewFlushTicket(frozen, false), nil
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
