// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"container/list"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of org.apache.lucene.index.DocumentsWriterFlushControl from Apache
// Lucene 10.4.0 (commit 9983b7c). This class tracks per-thread memory
// consumption and uses a configured flush policy to decide when a
// DocumentsWriterPerThread (DWPT) must flush. It also protects the writer
// against address-space exhaustion by force-flushing any DWPT that exceeds
// the configured per-thread RAM hard limit.
//
// The Java original is package-private and tightly coupled with several
// other package-private types: DocumentsWriterStallControl,
// DocumentsWriterPerThreadPool, DocumentsWriterDeleteQueue,
// LiveIndexWriterConfig, FlushPolicy and DocumentsWriterPerThread. Many of
// those types have not yet been ported in their Lucene-compatible shape.
// To keep this port self-contained and independently testable while the
// other ports are still in flight (Sprint 55 option (c) where gaps), this
// file declares minimal local interfaces (flushControlDWPT, flushControlPool,
// flushControlPolicy, flushControlDeleteQueue, flushControlConfig) that
// capture exactly the contract DocumentsWriterFlushControl needs. When the
// underlying types catch up, an adapter layer can wire them in without
// touching this file.
//
// The stall controller is small (~115 LOC in Lucene) and only used by
// DocumentsWriterFlushControl; it is inlined here as the unexported
// documentsWriterStallControl helper to mirror the upstream cohesion.

// ErrFlushControlClosed mirrors AlreadyClosedException thrown by
// obtainAndLock when the flush control has been closed.
var ErrFlushControlClosed = errors.New("flush control is closed")

// flushControlDWPT is the minimal DWPT-shaped contract this file relies on.
// It mirrors the package-private methods used by Lucene's
// DocumentsWriterFlushControl.
type flushControlDWPT interface {
	// getCommitLastBytesUsedDelta returns the delta between the current
	// bytesUsed and the last committed value. It does not commit.
	GetCommitLastBytesUsedDelta() int64
	// commitLastBytesUsed atomically advances the last-committed cursor.
	CommitLastBytesUsed(delta int64)
	// getLastCommittedBytesUsed returns the last committed bytesUsed value.
	GetLastCommittedBytesUsed() int64
	// ramBytesUsed returns the current bytesUsed estimate (uncommitted).
	RAMBytesUsed() int64
	// getNumDocsInRAM returns the number of documents buffered by this DWPT.
	GetNumDocsInRAMLocked() int
	// isFlushPending reports whether the DWPT has already been marked pending.
	IsFlushPending() bool
	// setFlushPending marks this DWPT as pending; called under the flush
	// control monitor.
	SetFlushPending()
	// abort discards in-memory state. Used when aborting pending flushes.
	Abort() error
	// tryLock attempts to acquire the DWPT lock without blocking.
	TryLock() bool
	// lock acquires the DWPT lock, blocking.
	Lock()
	// unlock releases the DWPT lock.
	Unlock()
	// isHeldByCurrentThread reports whether the current goroutine holds the
	// DWPT lock. Used only for assertions.
	IsHeldByCurrentThread() bool
	// getDeleteQueue returns the delete queue this DWPT is bound to. Used to
	// detect stale DWPTs during a full flush.
	GetDeleteQueue() flushControlDeleteQueue
}

// flushControlPool is the per-thread pool surface used by this file. It
// matches DocumentsWriterPerThreadPool's package-private API.
type flushControlPool interface {
	// Size returns the current number of registered DWPTs.
	Size() int
	// IsRegistered reports whether the given DWPT is currently in the pool.
	IsRegistered(perThread flushControlDWPT) bool
	// Checkout removes the DWPT from the pool. Returns true on success.
	Checkout(perThread flushControlDWPT) bool
	// GetAndLock acquires a DWPT from the pool with its lock already held.
	GetAndLock() flushControlDWPT
	// LockNewWriters blocks creation of new DWPTs.
	LockNewWriters()
	// UnlockNewWriters lifts the LockNewWriters block.
	UnlockNewWriters()
	// Iter visits every registered DWPT. The iteration order is unspecified.
	Iter(visit func(flushControlDWPT) bool)
	// FilterAndLock returns every DWPT matching predicate with its lock held.
	// The caller is responsible for releasing each lock.
	FilterAndLock(predicate func(flushControlDWPT) bool) []flushControlDWPT
}

// flushControlPolicy mirrors the package-private FlushPolicy.onChange hook.
// It intentionally does NOT collide with the exported index.FlushPolicy
// interface (which has a different, simpler shape) — this is the upstream
// contract that DocumentsWriterFlushControl was designed against.
type flushControlPolicy interface {
	// OnChange is called whenever the flush control's accounting changes for
	// a perThread. When perThread is nil, the change was a delete-only event.
	OnChange(control *DocumentsWriterFlushControl, perThread flushControlDWPT)
}

// flushControlDeleteQueue is the minimal delete-queue surface used by this
// file: identity comparison and a ram-bytes accessor.
type flushControlDeleteQueue interface {
	RAMBytesUsed() int64
	GetLastSequenceNumber() int64
	GetMaxSeqNo() int64
}

// flushControlConfig is the LiveIndexWriterConfig surface this file needs.
type flushControlConfig interface {
	GetRAMBufferSizeMB() float64
	GetMaxBufferedDocs() int
	GetRAMPerThreadHardLimitMB() int
	GetFlushPolicy() flushControlPolicy
	GetInfoStream() util.InfoStream
}

// flushControlOwner is the minimal DocumentsWriter surface used by this
// file: the bound delete queue, a way to swap it during full flushes, and
// a way to roll back the flushed-doc counter on abort.
type flushControlOwner interface {
	GetDeleteQueue() flushControlDeleteQueue
	ResetDeleteQueue(poolSize int) int64
	SubtractFlushedNumDocs(numDocs int)
	GetPerThreadPool() flushControlPool
}

// DocumentsWriterFlushControl tracks DWPT memory consumption and decides
// when a DWPT must flush. See the file-level comment for porting notes.
type DocumentsWriterFlushControl struct {
	// mu protects all fields below except those marked atomic/volatile.
	mu sync.Mutex
	// cond signals waiters in WaitForFlush and obtainAndLock.
	cond *sync.Cond

	hardMaxBytesPerDWPT int64

	activeBytes int64
	// flushBytes is read without holding mu (volatile in Lucene).
	flushBytes atomic.Int64
	// numPending is read without holding mu (volatile in Lucene).
	numPending atomic.Int32

	numDocsSinceStalled int // assert-only

	flushDeletes atomic.Bool

	fullFlush         bool
	fullFlushMarkDone bool

	// flushQueue is a FIFO of DWPTs ready to be flushed.
	flushQueue *list.List // of flushControlDWPT
	// blockedFlushes is a FIFO of DWPTs pending while a full flush is active.
	blockedFlushes *list.List // of flushControlDWPT
	// flushingWriters tracks every DWPT currently in some flushing state.
	flushingWriters []flushControlDWPT

	// assert-only peak tracking
	maxConfiguredRAMBuffer float64
	peakActiveBytes        int64
	peakFlushBytes         int64
	peakNetBytes           int64
	peakDelta              int64
	flushByRAMWasDisabled  bool

	stallControl  *documentsWriterStallControl
	perThreadPool flushControlPool
	flushPolicy   flushControlPolicy
	closed        bool
	owner         flushControlOwner
	config        flushControlConfig
	infoStream    util.InfoStream

	stallStartNS int64
}

// NewDocumentsWriterFlushControl constructs a flush control for the given
// DocumentsWriter and live config. The DocumentsWriter must provide its
// per-thread pool via owner.GetPerThreadPool.
func NewDocumentsWriterFlushControl(
	owner flushControlOwner,
	config flushControlConfig,
) *DocumentsWriterFlushControl {
	infoStream := config.GetInfoStream()
	if infoStream == nil {
		infoStream = util.NoOpInfoStream
	}
	c := &DocumentsWriterFlushControl{
		hardMaxBytesPerDWPT: int64(config.GetRAMPerThreadHardLimitMB()) * 1024 * 1024,
		flushQueue:          list.New(),
		blockedFlushes:      list.New(),
		flushingWriters:     make([]flushControlDWPT, 0, 8),
		stallControl:        newDocumentsWriterStallControl(),
		perThreadPool:       owner.GetPerThreadPool(),
		flushPolicy:         config.GetFlushPolicy(),
		owner:               owner,
		config:              config,
		infoStream:          infoStream,
	}
	c.cond = sync.NewCond(&c.mu)
	return c
}

// ActiveBytes returns the sum of bytesUsed across all active (not yet
// pending) DWPTs.
func (c *DocumentsWriterFlushControl) ActiveBytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activeBytes
}

// GetFlushingBytes returns the sum of bytesUsed across all flushing DWPTs.
// Mirrors the volatile read of flushBytes in Lucene.
func (c *DocumentsWriterFlushControl) GetFlushingBytes() int64 {
	return c.flushBytes.Load()
}

// NetBytes returns activeBytes + flushBytes.
func (c *DocumentsWriterFlushControl) NetBytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.flushBytes.Load() + c.activeBytes
}

// stallLimitBytes returns the byte threshold above which incoming indexing
// threads are stalled. Caller need not hold mu.
func (c *DocumentsWriterFlushControl) stallLimitBytes() int64 {
	maxRAMMB := c.config.GetRAMBufferSizeMB()
	if maxRAMMB == float64(DISABLE_AUTO_FLUSH) {
		return 1<<63 - 1
	}
	return int64(2 * maxRAMMB * 1024 * 1024)
}

// assertMemory mirrors Lucene's package-private assert. Always returns true.
func (c *DocumentsWriterFlushControl) assertMemory() bool {
	maxRAMMB := c.config.GetRAMBufferSizeMB()
	if maxRAMMB != float64(DISABLE_AUTO_FLUSH) && !c.flushByRAMWasDisabled {
		if maxRAMMB > c.maxConfiguredRAMBuffer {
			c.maxConfiguredRAMBuffer = maxRAMMB
		}
		ram := c.flushBytes.Load() + c.activeBytes
		ramBufferBytes := int64(c.maxConfiguredRAMBuffer * 1024 * 1024)
		expected := (2 * ramBufferBytes) +
			(int64(c.numPending.Load())+int64(c.numFlushingDWPTLocked())+int64(c.numBlockedFlushesLocked()))*c.peakDelta +
			int64(c.numDocsSinceStalled)*c.peakDelta
		if c.peakDelta < (ramBufferBytes >> 1) {
			if ram > expected {
				panic(fmt.Sprintf("actual mem: %d byte, expected mem: %d byte", ram, expected))
			}
		}
	} else {
		c.flushByRAMWasDisabled = true
	}
	return true
}

// updatePeaks updates the assert-only peak counters.
func (c *DocumentsWriterFlushControl) updatePeaks(delta int64) bool {
	if c.activeBytes > c.peakActiveBytes {
		c.peakActiveBytes = c.activeBytes
	}
	fb := c.flushBytes.Load()
	if fb > c.peakFlushBytes {
		c.peakFlushBytes = fb
	}
	net := fb + c.activeBytes
	if net > c.peakNetBytes {
		c.peakNetBytes = net
	}
	if delta > c.peakDelta {
		c.peakDelta = delta
	}
	return true
}

// ramBufferGranularity returns the smallest delta that must be flushed to
// the global RAM accounting, to avoid contention on tiny per-doc updates.
func (c *DocumentsWriterFlushControl) ramBufferGranularity() int64 {
	ramBufferSizeMB := c.config.GetRAMBufferSizeMB()
	if ramBufferSizeMB == float64(DISABLE_AUTO_FLUSH) {
		ramBufferSizeMB = float64(c.config.GetRAMPerThreadHardLimitMB())
	}
	// No more than ~0.1% of the RAM buffer size, capped at 16kB.
	granularity := int64(ramBufferSizeMB * 1024.0)
	if granularity > 16*1024 {
		granularity = 16 * 1024
	}
	return granularity
}

// DoAfterDocument is called after a document has been processed by a DWPT.
// If the DWPT needs to be checked out for flushing, it is returned;
// otherwise nil is returned.
func (c *DocumentsWriterFlushControl) DoAfterDocument(perThread flushControlDWPT) flushControlDWPT {
	delta := perThread.GetCommitLastBytesUsedDelta()
	// Skip global accounting on small deltas to reduce contention.
	if c.config.GetMaxBufferedDocs() == DISABLE_AUTO_FLUSH && delta < c.ramBufferGranularity() {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	perThread.CommitLastBytesUsed(delta)
	var result flushControlDWPT
	func() {
		defer func() {
			stalled := c.updateStallStateLocked()
			c.assertNumDocsSinceStalledLocked(stalled)
			_ = c.assertMemory()
		}()
		if perThread.IsFlushPending() {
			c.flushBytes.Add(delta)
			_ = c.updatePeaks(delta)
		} else {
			c.activeBytes += delta
			_ = c.updatePeaks(delta)
			c.flushPolicy.OnChange(c, perThread)
			if !perThread.IsFlushPending() && perThread.RAMBytesUsed() > c.hardMaxBytesPerDWPT {
				c.setFlushPendingLocked(perThread)
			}
		}
		result = c.checkoutLocked(perThread, false)
	}()
	return result
}

// checkoutLocked decides whether the given perThread should be removed from
// the pool right now. Caller must hold c.mu.
func (c *DocumentsWriterFlushControl) checkoutLocked(perThread flushControlDWPT, markPending bool) flushControlDWPT {
	if c.fullFlush {
		if perThread.IsFlushPending() {
			c.checkoutAndBlockLocked(perThread)
			return c.nextPendingFlushInternal()
		}
	} else {
		if markPending {
			if perThread.IsFlushPending() {
				panic("perThread already flush pending in markPending path")
			}
			c.setFlushPendingLocked(perThread)
		}
		if perThread.IsFlushPending() {
			return c.checkOutForFlushLocked(perThread)
		}
	}
	return nil
}

// assertNumDocsSinceStalledLocked tracks docs finished while stalled. Used
// only by assertMemory to bound the expected RAM consumption.
func (c *DocumentsWriterFlushControl) assertNumDocsSinceStalledLocked(stalled bool) bool {
	if stalled {
		c.numDocsSinceStalled++
	} else {
		c.numDocsSinceStalled = 0
	}
	return true
}

// DoAfterFlush is called by the flushing thread once a DWPT flush completes.
func (c *DocumentsWriterFlushControl) DoAfterFlush(dwpt flushControlDWPT) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.containsFlushingLocked(dwpt) {
		panic("DoAfterFlush called for DWPT not in flushingWriters")
	}
	defer c.cond.Broadcast()
	defer c.updateStallStateLocked()
	c.removeFlushingLocked(dwpt)
	c.flushBytes.Add(-dwpt.GetLastCommittedBytesUsed())
	_ = c.assertMemory()
}

// updateStallStateLocked recomputes whether indexing should be stalled and
// pushes the result to the stall controller. Caller must hold c.mu.
func (c *DocumentsWriterFlushControl) updateStallStateLocked() bool {
	limit := c.stallLimitBytes()
	stall := (c.activeBytes+c.flushBytes.Load()) > limit &&
		c.activeBytes < limit &&
		!c.closed
	if c.infoStream.IsEnabled("DWFC") {
		if stall != c.stallControl.AnyStalledThreads() {
			if stall {
				c.infoStream.Message("DW", fmt.Sprintf(
					"now stalling flushes: netBytes: %.1f MB flushBytes: %.1f MB fullFlush: %v",
					float64(c.flushBytes.Load()+c.activeBytes)/1024.0/1024.0,
					float64(c.flushBytes.Load())/1024.0/1024.0,
					c.fullFlush,
				))
				c.stallStartNS = time.Now().UnixNano()
			} else {
				c.infoStream.Message("DW", fmt.Sprintf(
					"done stalling flushes for %.1f msec: netBytes: %.1f MB flushBytes: %.1f MB fullFlush: %v",
					float64(time.Now().UnixNano()-c.stallStartNS)/float64(time.Millisecond.Nanoseconds()),
					float64(c.flushBytes.Load()+c.activeBytes)/1024.0/1024.0,
					float64(c.flushBytes.Load())/1024.0/1024.0,
					c.fullFlush,
				))
			}
		}
	}
	c.stallControl.UpdateStalled(stall)
	return stall
}

// WaitForFlush blocks until no DWPTs are currently flushing.
func (c *DocumentsWriterFlushControl) WaitForFlush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.flushingWriters) != 0 {
		c.cond.Wait()
	}
}

// SetFlushPending marks the given DWPT as pending. The DWPT must have at
// least one buffered document and must not already be pending.
func (c *DocumentsWriterFlushControl) SetFlushPending(perThread flushControlDWPT) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setFlushPendingLocked(perThread)
}

// setFlushPendingLocked is the unlocked variant of SetFlushPending.
func (c *DocumentsWriterFlushControl) setFlushPendingLocked(perThread flushControlDWPT) {
	if perThread.IsFlushPending() {
		panic("DWPT is already flush pending")
	}
	if perThread.GetNumDocsInRAMLocked() > 0 {
		perThread.SetFlushPending()
		bytes := perThread.GetLastCommittedBytesUsed()
		c.flushBytes.Add(bytes)
		c.activeBytes -= bytes
		c.numPending.Add(1)
		_ = c.assertMemory()
	}
}

// DoOnAbort releases accounting bytes for an aborted DWPT and removes it
// from the pool.
func (c *DocumentsWriterFlushControl) DoOnAbort(perThread flushControlDWPT) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.perThreadPool.IsRegistered(perThread) {
		panic("DoOnAbort called for unregistered DWPT")
	}
	if !perThread.IsHeldByCurrentThread() {
		panic("DoOnAbort called without holding DWPT lock")
	}
	defer func() {
		c.updateStallStateLocked()
		if !c.perThreadPool.Checkout(perThread) {
			panic("perThreadPool.Checkout failed in DoOnAbort")
		}
	}()
	if perThread.IsFlushPending() {
		c.flushBytes.Add(-perThread.GetLastCommittedBytesUsed())
	} else {
		c.activeBytes -= perThread.GetLastCommittedBytesUsed()
	}
	_ = c.assertMemory()
}

// checkoutAndBlockLocked moves a pending DWPT to the blockedFlushes queue.
// Used during a full flush when a stray pending DWPT shows up.
func (c *DocumentsWriterFlushControl) checkoutAndBlockLocked(perThread flushControlDWPT) {
	if !c.perThreadPool.IsRegistered(perThread) {
		panic("checkoutAndBlock: DWPT not registered")
	}
	if !perThread.IsHeldByCurrentThread() {
		panic("checkoutAndBlock: DWPT lock not held")
	}
	if !perThread.IsFlushPending() {
		panic("can not block non-pending threadstate")
	}
	if !c.fullFlush {
		panic("can not block if fullFlush == false")
	}
	c.numPending.Add(-1)
	c.blockedFlushes.PushBack(perThread)
	if !c.perThreadPool.Checkout(perThread) {
		panic("perThreadPool.Checkout failed in checkoutAndBlock")
	}
}

// checkOutForFlushLocked moves a pending DWPT into flushingWriters and
// returns it to the caller for actual flushing.
func (c *DocumentsWriterFlushControl) checkOutForFlushLocked(perThread flushControlDWPT) flushControlDWPT {
	if !perThread.IsFlushPending() {
		panic("checkOutForFlush: DWPT not flush pending")
	}
	if !perThread.IsHeldByCurrentThread() {
		panic("checkOutForFlush: DWPT lock not held")
	}
	if !c.perThreadPool.IsRegistered(perThread) {
		panic("checkOutForFlush: DWPT not registered")
	}
	defer c.updateStallStateLocked()
	c.addFlushingDWPTLocked(perThread)
	c.numPending.Add(-1)
	if !c.perThreadPool.Checkout(perThread) {
		panic("perThreadPool.Checkout failed in checkOutForFlush")
	}
	return perThread
}

// addFlushingDWPTLocked records a DWPT in flushingWriters.
func (c *DocumentsWriterFlushControl) addFlushingDWPTLocked(perThread flushControlDWPT) {
	if c.containsFlushingLocked(perThread) {
		panic("DWPT is already flushing")
	}
	c.flushingWriters = append(c.flushingWriters, perThread)
}

// containsFlushingLocked reports whether perThread is in flushingWriters.
func (c *DocumentsWriterFlushControl) containsFlushingLocked(perThread flushControlDWPT) bool {
	for _, d := range c.flushingWriters {
		if d == perThread {
			return true
		}
	}
	return false
}

// removeFlushingLocked removes perThread from flushingWriters.
func (c *DocumentsWriterFlushControl) removeFlushingLocked(perThread flushControlDWPT) {
	for i, d := range c.flushingWriters {
		if d == perThread {
			c.flushingWriters = append(c.flushingWriters[:i], c.flushingWriters[i+1:]...)
			return
		}
	}
}

// String renders a short diagnostic. Mirrors Lucene's toString.
func (c *DocumentsWriterFlushControl) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Sprintf(
		"DocumentsWriterFlushControl [activeBytes=%d, flushBytes=%d]",
		c.activeBytes, c.flushBytes.Load(),
	)
}

// NextPendingFlush returns the next DWPT ready to be flushed, or nil if
// none is immediately available. If no DWPT is in the flushQueue and we are
// not in a full flush, it scans the pool for any pending DWPT it can lock
// and check out.
func (c *DocumentsWriterFlushControl) NextPendingFlush() flushControlDWPT {
	var (
		numPending int32
		fullFlush  bool
	)
	c.mu.Lock()
	if e := c.flushQueue.Front(); e != nil {
		c.flushQueue.Remove(e)
		c.updateStallStateLocked()
		c.mu.Unlock()
		return e.Value.(flushControlDWPT)
	}
	fullFlush = c.fullFlush
	numPending = c.numPending.Load()
	c.mu.Unlock()

	if numPending > 0 && !fullFlush {
		var picked flushControlDWPT
		c.perThreadPool.Iter(func(next flushControlDWPT) bool {
			if !next.IsFlushPending() {
				return true
			}
			if !next.TryLock() {
				return true
			}
			released := false
			defer func() {
				if !released {
					next.Unlock()
				}
			}()
			if !c.perThreadPool.IsRegistered(next) {
				return true
			}
			c.mu.Lock()
			out := c.checkOutForFlushLocked(next)
			c.mu.Unlock()
			picked = out
			released = true // checkout removed it from the pool
			return false    // stop iteration
		})
		return picked
	}
	return nil
}

// nextPendingFlushInternal is the queue-only variant used while holding c.mu
// inside checkoutLocked.
func (c *DocumentsWriterFlushControl) nextPendingFlushInternal() flushControlDWPT {
	if e := c.flushQueue.Front(); e != nil {
		c.flushQueue.Remove(e)
		c.updateStallStateLocked()
		return e.Value.(flushControlDWPT)
	}
	return nil
}

// Close signals that the writer is closing. Stalled threads are released.
func (c *DocumentsWriterFlushControl) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.stallControl.UpdateStalled(false)
	c.cond.Broadcast()
	return nil
}

// AllActiveWriters visits every DWPT currently in the pool.
func (c *DocumentsWriterFlushControl) AllActiveWriters(visit func(flushControlDWPT) bool) {
	c.perThreadPool.Iter(visit)
}

// DoOnDelete is called when a global delete has been applied. It feeds an
// onChange notification to the flush policy with a nil DWPT.
func (c *DocumentsWriterFlushControl) DoOnDelete() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flushPolicy.OnChange(c, nil)
}

// GetDeleteBytesUsed returns the heap bytes consumed by buffered deletes
// that would be freed by pushing them.
func (c *DocumentsWriterFlushControl) GetDeleteBytesUsed() int64 {
	if dq := c.owner.GetDeleteQueue(); dq != nil {
		return dq.RAMBytesUsed()
	}
	return 0
}

// RAMBytesUsed returns the total heap bytes accounted for by this control.
func (c *DocumentsWriterFlushControl) RAMBytesUsed() int64 {
	return c.GetDeleteBytesUsed() + c.NetBytes()
}

// NumFlushingDWPT returns the count of DWPTs currently in some flushing
// state (in-queue or actively flushing).
func (c *DocumentsWriterFlushControl) NumFlushingDWPT() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.numFlushingDWPTLocked()
}

// numFlushingDWPTLocked is the unlocked variant of NumFlushingDWPT.
func (c *DocumentsWriterFlushControl) numFlushingDWPTLocked() int {
	return len(c.flushingWriters)
}

// GetAndResetApplyAllDeletes atomically clears and returns the
// flush-deletes flag.
func (c *DocumentsWriterFlushControl) GetAndResetApplyAllDeletes() bool {
	return c.flushDeletes.Swap(false)
}

// GetApplyAllDeletes reads the flush-deletes flag without resetting it.
func (c *DocumentsWriterFlushControl) GetApplyAllDeletes() bool {
	return c.flushDeletes.Load()
}

// SetApplyAllDeletes raises the flush-deletes flag.
func (c *DocumentsWriterFlushControl) SetApplyAllDeletes() {
	c.flushDeletes.Store(true)
}

// ObtainAndLock returns a DWPT (with its lock held) that is bound to the
// current delete queue. If the control has been closed, returns
// ErrFlushControlClosed.
func (c *DocumentsWriterFlushControl) ObtainAndLock() (flushControlDWPT, error) {
	for {
		c.mu.Lock()
		closed := c.closed
		fullFlush := c.fullFlush
		fullFlushMarkDone := c.fullFlushMarkDone
		c.mu.Unlock()
		if closed {
			return nil, store.NewAlreadyClosedException(ErrFlushControlClosed.Error(), nil)
		}
		perThread := c.perThreadPool.GetAndLock()
		if perThread == nil {
			continue
		}
		if perThread.GetDeleteQueue() == c.owner.GetDeleteQueue() {
			return perThread, nil
		}
		// Stale DWPT: must be the result of a full flush in progress.
		if !fullFlush || fullFlushMarkDone {
			perThread.Unlock()
			panic(fmt.Sprintf(
				"found a stale DWPT but full flush mark phase is already done fullFlush: %v markDone: %v",
				fullFlush, fullFlushMarkDone,
			))
		}
		perThread.Unlock()
	}
}

// MarkForFullFlush marks every DWPT bound to the current delete queue for
// flush, swaps in a new delete queue and returns the sequence number gap
// the next DWPT may use.
func (c *DocumentsWriterFlushControl) MarkForFullFlush() int64 {
	var (
		flushingQueue flushControlDeleteQueue
		seqNo         int64
	)
	c.mu.Lock()
	if c.fullFlush {
		c.mu.Unlock()
		panic("MarkForFullFlush called while full flush is still running")
	}
	if c.fullFlushMarkDone {
		c.mu.Unlock()
		panic("full flush collection marker is still set to true")
	}
	c.fullFlush = true
	flushingQueue = c.owner.GetDeleteQueue()
	c.perThreadPool.LockNewWriters()
	func() {
		defer c.perThreadPool.UnlockNewWriters()
		seqNo = c.owner.ResetDeleteQueue(c.perThreadPool.Size())
	}()
	c.mu.Unlock()

	fullFlushBuffer := make([]flushControlDWPT, 0, 8)
	dwpts := c.perThreadPool.FilterAndLock(func(d flushControlDWPT) bool {
		return d.GetDeleteQueue() == flushingQueue
	})
	for _, next := range dwpts {
		func() {
			defer next.Unlock()
			if next.GetNumDocsInRAMLocked() > 0 {
				c.mu.Lock()
				if !next.IsFlushPending() {
					c.setFlushPendingLocked(next)
				}
				flushing := c.checkOutForFlushLocked(next)
				c.mu.Unlock()
				if flushing == nil {
					panic("DWPT must never be nil here")
				}
				if flushing != next {
					panic("flushControl returned different DWPT")
				}
				fullFlushBuffer = append(fullFlushBuffer, flushing)
			} else {
				if !c.perThreadPool.Checkout(next) {
					panic("perThreadPool.Checkout failed in MarkForFullFlush")
				}
			}
		}()
	}

	c.mu.Lock()
	c.pruneBlockedQueueLocked(flushingQueue)
	c.assertBlockedFlushesLocked(c.owner.GetDeleteQueue())
	for _, d := range fullFlushBuffer {
		c.flushQueue.PushBack(d)
	}
	c.updateStallStateLocked()
	c.fullFlushMarkDone = true
	c.mu.Unlock()

	if flushingQueue.GetLastSequenceNumber() > flushingQueue.GetMaxSeqNo() {
		panic("flushingQueue.GetLastSequenceNumber() > GetMaxSeqNo()")
	}
	return seqNo
}

// pruneBlockedQueueLocked moves any DWPT bound to flushingQueue from
// blockedFlushes into flushQueue.
func (c *DocumentsWriterFlushControl) pruneBlockedQueueLocked(flushingQueue flushControlDeleteQueue) {
	var next *list.Element
	for e := c.blockedFlushes.Front(); e != nil; e = next {
		next = e.Next()
		blockedFlush := e.Value.(flushControlDWPT)
		if blockedFlush.GetDeleteQueue() == flushingQueue {
			c.blockedFlushes.Remove(e)
			c.addFlushingDWPTLocked(blockedFlush)
			// don't decr pending here - it's already done when DWPT was blocked
			c.flushQueue.PushBack(blockedFlush)
		}
	}
}

// FinishFullFlush concludes a full flush. flushQueue and flushingWriters
// must already be empty.
func (c *DocumentsWriterFlushControl) FinishFullFlush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.fullFlush {
		panic("FinishFullFlush called when not in full flush")
	}
	if c.flushQueue.Len() != 0 {
		panic("FinishFullFlush: flushQueue not empty")
	}
	if len(c.flushingWriters) != 0 {
		panic("FinishFullFlush: flushingWriters not empty")
	}
	defer func() {
		c.fullFlushMarkDone = false
		c.fullFlush = false
		c.updateStallStateLocked()
	}()
	if c.blockedFlushes.Len() != 0 {
		c.assertBlockedFlushesLocked(c.owner.GetDeleteQueue())
		c.pruneBlockedQueueLocked(c.owner.GetDeleteQueue())
		if c.blockedFlushes.Len() != 0 {
			panic("blocked flushes not drained")
		}
	}
}

// assertBlockedFlushesLocked panics if any blocked DWPT is bound to a
// different delete queue. Used as a defensive invariant check.
func (c *DocumentsWriterFlushControl) assertBlockedFlushesLocked(flushingQueue flushControlDeleteQueue) bool {
	for e := c.blockedFlushes.Front(); e != nil; e = e.Next() {
		blockedFlush := e.Value.(flushControlDWPT)
		if blockedFlush.GetDeleteQueue() != flushingQueue {
			panic("blocked DWPT bound to wrong delete queue")
		}
	}
	return true
}

// AbortFullFlushes aborts every DWPT in flushQueue/blockedFlushes and
// clears the full-flush state.
func (c *DocumentsWriterFlushControl) AbortFullFlushes() {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer func() {
		c.fullFlushMarkDone = false
		c.fullFlush = false
	}()
	c.abortPendingFlushesLocked()
}

// AbortPendingFlushes aborts every DWPT in flushQueue/blockedFlushes
// without touching the full-flush state.
func (c *DocumentsWriterFlushControl) AbortPendingFlushes() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.abortPendingFlushesLocked()
}

// abortPendingFlushesLocked is the unlocked variant used by both Abort* paths.
func (c *DocumentsWriterFlushControl) abortPendingFlushesLocked() {
	defer func() {
		c.flushQueue.Init()
		c.blockedFlushes.Init()
		c.updateStallStateLocked()
	}()
	for e := c.flushQueue.Front(); e != nil; e = e.Next() {
		dwpt := e.Value.(flushControlDWPT)
		c.abortOneLocked(dwpt)
	}
	for e := c.blockedFlushes.Front(); e != nil; e = e.Next() {
		blockedFlush := e.Value.(flushControlDWPT)
		// Add to flushingWriters so doAfterFlush accounting matches.
		c.addFlushingDWPTLocked(blockedFlush)
		c.abortOneLocked(blockedFlush)
	}
}

// abortOneLocked aborts a single DWPT and runs the doAfterFlush bookkeeping.
func (c *DocumentsWriterFlushControl) abortOneLocked(dwpt flushControlDWPT) {
	defer func() {
		// Inline DoAfterFlush bookkeeping without re-acquiring mu.
		if c.containsFlushingLocked(dwpt) {
			c.removeFlushingLocked(dwpt)
			c.flushBytes.Add(-dwpt.GetLastCommittedBytesUsed())
			_ = c.assertMemory()
			c.updateStallStateLocked()
			c.cond.Broadcast()
		}
	}()
	c.owner.SubtractFlushedNumDocs(dwpt.GetNumDocsInRAMLocked())
	_ = dwpt.Abort() // best-effort, matches Lucene's swallow
}

// IsFullFlush reports whether a full flush is currently in progress.
func (c *DocumentsWriterFlushControl) IsFullFlush() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fullFlush
}

// NumQueuedFlushes returns the number of DWPTs already checked out and
// waiting in the flush queue.
func (c *DocumentsWriterFlushControl) NumQueuedFlushes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.flushQueue.Len()
}

// NumBlockedFlushes returns the number of DWPTs in the blocked queue.
func (c *DocumentsWriterFlushControl) NumBlockedFlushes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.numBlockedFlushesLocked()
}

// numBlockedFlushesLocked is the unlocked variant of NumBlockedFlushes.
func (c *DocumentsWriterFlushControl) numBlockedFlushesLocked() int {
	return c.blockedFlushes.Len()
}

// WaitIfStalled blocks if too many DWPTs are flushing and no checked-out
// DWPT is available.
func (c *DocumentsWriterFlushControl) WaitIfStalled() {
	c.stallControl.WaitIfStalled()
}

// AnyStalledThreads reports whether any thread is currently stalled.
func (c *DocumentsWriterFlushControl) AnyStalledThreads() bool {
	return c.stallControl.AnyStalledThreads()
}

// GetInfoStream returns the InfoStream this control logs to.
func (c *DocumentsWriterFlushControl) GetInfoStream() util.InfoStream {
	return c.infoStream
}

// FindLargestNonPendingWriter scans the pool for the DWPT with the highest
// committed bytesUsed that is not already pending.
func (c *DocumentsWriterFlushControl) FindLargestNonPendingWriter() flushControlDWPT {
	c.mu.Lock()
	defer c.mu.Unlock()
	var maxRAMUsingWriter flushControlDWPT
	maxRAMSoFar := int64(-1)
	count := 0
	c.perThreadPool.Iter(func(next flushControlDWPT) bool {
		if !next.IsFlushPending() && next.GetNumDocsInRAMLocked() > 0 {
			nextRAM := next.GetLastCommittedBytesUsed()
			if c.infoStream.IsEnabled("FP") {
				c.infoStream.Message("FP", fmt.Sprintf(
					"thread state has %d bytes; docInRAM=%d", nextRAM, next.GetNumDocsInRAMLocked(),
				))
			}
			count++
			if nextRAM > maxRAMSoFar {
				maxRAMSoFar = nextRAM
				maxRAMUsingWriter = next
			}
		}
		return true
	})
	if c.infoStream.IsEnabled("FP") {
		c.infoStream.Message("FP", fmt.Sprintf("%d in-use non-flushing threads states", count))
	}
	return maxRAMUsingWriter
}

// CheckoutLargestNonPendingWriter returns the largest non-pending DWPT
// after locking and checking it out of the pool, or nil if none exists.
func (c *DocumentsWriterFlushControl) CheckoutLargestNonPendingWriter() flushControlDWPT {
	largest := c.FindLargestNonPendingWriter()
	if largest == nil {
		return nil
	}
	largest.Lock()
	defer largest.Unlock()
	if !c.perThreadPool.IsRegistered(largest) {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.updateStallStateLocked()
	return c.checkoutLocked(largest, !largest.IsFlushPending())
}

// GetPeakActiveBytes returns the highest activeBytes ever observed.
func (c *DocumentsWriterFlushControl) GetPeakActiveBytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.peakActiveBytes
}

// GetPeakNetBytes returns the highest activeBytes+flushBytes ever observed.
func (c *DocumentsWriterFlushControl) GetPeakNetBytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.peakNetBytes
}

// ----------------------------------------------------------------------------
// documentsWriterStallControl is the Go port of
// org.apache.lucene.index.DocumentsWriterStallControl. The original is small
// (~115 LOC), package-private and used only by DocumentsWriterFlushControl,
// so it is inlined here to mirror the upstream cohesion.
// ----------------------------------------------------------------------------

// documentsWriterStallControl coordinates indexing threads with flushing.
// When flushing significantly lags indexing, incoming indexers are blocked.
type documentsWriterStallControl struct {
	mu      sync.Mutex
	cond    *sync.Cond
	stalled atomic.Bool

	// assert-only fields
	numWaiting int
	wasStalled bool
}

// newDocumentsWriterStallControl constructs a fresh stall controller.
func newDocumentsWriterStallControl() *documentsWriterStallControl {
	s := &documentsWriterStallControl{}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// UpdateStalled toggles the stalled state. When transitioning to unstalled,
// all blocked waiters are released.
func (s *documentsWriterStallControl) UpdateStalled(stalled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stalled.Load() == stalled {
		return
	}
	s.stalled.Store(stalled)
	if stalled {
		s.wasStalled = true
	}
	s.cond.Broadcast()
}

// WaitIfStalled blocks until the controller becomes unstalled or up to 1s.
func (s *documentsWriterStallControl) WaitIfStalled() {
	if !s.stalled.Load() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stalled.Load() {
		return
	}
	s.numWaiting++
	// Mirror Lucene: bounded wait with a 1s safety to recover from any
	// missed broadcast. We use a timer that broadcasts the cond on expiry.
	done := make(chan struct{})
	t := time.AfterFunc(time.Second, func() {
		s.mu.Lock()
		s.cond.Broadcast()
		s.mu.Unlock()
		close(done)
	})
	s.cond.Wait()
	t.Stop()
	select {
	case <-done:
	default:
	}
	s.numWaiting--
}

// AnyStalledThreads reports whether the controller is currently stalled.
func (s *documentsWriterStallControl) AnyStalledThreads() bool {
	return s.stalled.Load()
}

// HasBlocked reports whether any thread is currently blocked. Test-only.
func (s *documentsWriterStallControl) HasBlocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.numWaiting > 0
}

// GetNumWaiting returns the number of currently blocked threads. Test-only.
func (s *documentsWriterStallControl) GetNumWaiting() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.numWaiting
}

// IsHealthy reports the inverse of AnyStalledThreads. Test-only.
func (s *documentsWriterStallControl) IsHealthy() bool {
	return !s.stalled.Load()
}

// WasStalled reports whether the controller has ever been stalled. Test-only.
func (s *documentsWriterStallControl) WasStalled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wasStalled
}
