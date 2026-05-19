// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ----------------------------------------------------------------------------
// Test fakes. Mirror just enough of the DWPT/pool/policy/queue contract to
// exercise DocumentsWriterFlushControl in isolation. The shipping
// DocumentsWriterPerThread does not yet satisfy flushControlDWPT (Sprint 55
// option (c) - kept self-contained while the rest of the indexer catches up).
// ----------------------------------------------------------------------------

type fakeDWPT struct {
	mu            sync.Mutex
	bytesUsed     atomic.Int64
	lastCommitted atomic.Int64
	numDocsInRAM  atomic.Int32
	flushPending  atomic.Bool
	deleteQueue   *fakeDeleteQueue
	abortCalled   atomic.Bool
	heldLockOwner atomic.Int64 // poor-man's owner sentinel (1=held, 0=free)
}

func (d *fakeDWPT) GetCommitLastBytesUsedDelta() int64 {
	return d.bytesUsed.Load() - d.lastCommitted.Load()
}
func (d *fakeDWPT) CommitLastBytesUsed(delta int64) {
	d.lastCommitted.Add(delta)
}
func (d *fakeDWPT) GetLastCommittedBytesUsed() int64 { return d.lastCommitted.Load() }
func (d *fakeDWPT) RAMBytesUsed() int64              { return d.bytesUsed.Load() }
func (d *fakeDWPT) GetNumDocsInRAMLocked() int       { return int(d.numDocsInRAM.Load()) }
func (d *fakeDWPT) IsFlushPending() bool             { return d.flushPending.Load() }
func (d *fakeDWPT) SetFlushPending()                 { d.flushPending.Store(true) }
func (d *fakeDWPT) Abort() error                     { d.abortCalled.Store(true); return nil }
func (d *fakeDWPT) TryLock() bool {
	if d.heldLockOwner.CompareAndSwap(0, 1) {
		d.mu.Lock()
		return true
	}
	return false
}
func (d *fakeDWPT) Lock() {
	d.mu.Lock()
	d.heldLockOwner.Store(1)
}
func (d *fakeDWPT) Unlock() {
	d.heldLockOwner.Store(0)
	d.mu.Unlock()
}
func (d *fakeDWPT) IsHeldByCurrentThread() bool { return d.heldLockOwner.Load() == 1 }
func (d *fakeDWPT) GetDeleteQueue() flushControlDeleteQueue {
	if d.deleteQueue == nil {
		return nil
	}
	return d.deleteQueue
}

type fakeDeleteQueue struct {
	ramBytes int64
	lastSeq  int64
	maxSeq   int64
}

func (q *fakeDeleteQueue) RAMBytesUsed() int64          { return q.ramBytes }
func (q *fakeDeleteQueue) GetLastSequenceNumber() int64 { return q.lastSeq }
func (q *fakeDeleteQueue) GetMaxSeqNo() int64           { return q.maxSeq }

type fakePool struct {
	mu       sync.Mutex
	dwpts    []*fakeDWPT
	newLock  sync.Mutex
	newLocks atomic.Int32
}

func (p *fakePool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.dwpts)
}
func (p *fakePool) IsRegistered(perThread flushControlDWPT) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, d := range p.dwpts {
		if any(d) == any(perThread) {
			return true
		}
	}
	return false
}
func (p *fakePool) Checkout(perThread flushControlDWPT) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, d := range p.dwpts {
		if any(d) == any(perThread) {
			p.dwpts = append(p.dwpts[:i], p.dwpts[i+1:]...)
			return true
		}
	}
	return false
}
func (p *fakePool) GetAndLock() flushControlDWPT {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.dwpts) == 0 {
		return nil
	}
	d := p.dwpts[0]
	d.Lock()
	return d
}
func (p *fakePool) LockNewWriters()   { p.newLock.Lock(); p.newLocks.Add(1) }
func (p *fakePool) UnlockNewWriters() { p.newLocks.Add(-1); p.newLock.Unlock() }
func (p *fakePool) Iter(visit func(flushControlDWPT) bool) {
	p.mu.Lock()
	snapshot := append([]*fakeDWPT(nil), p.dwpts...)
	p.mu.Unlock()
	for _, d := range snapshot {
		if !visit(d) {
			return
		}
	}
}
func (p *fakePool) FilterAndLock(predicate func(flushControlDWPT) bool) []flushControlDWPT {
	p.mu.Lock()
	snapshot := append([]*fakeDWPT(nil), p.dwpts...)
	p.mu.Unlock()
	out := make([]flushControlDWPT, 0, len(snapshot))
	for _, d := range snapshot {
		if predicate(d) {
			d.Lock()
			out = append(out, d)
		}
	}
	return out
}

type fakePolicy struct {
	calls atomic.Int32
}

func (p *fakePolicy) OnChange(*DocumentsWriterFlushControl, flushControlDWPT) {
	p.calls.Add(1)
}

type fakeConfig struct {
	ramBufferMB     float64
	maxBufferedDocs int
	hardLimitMB     int
	policy          flushControlPolicy
	infoStream      util.InfoStream
}

func (c *fakeConfig) GetRAMBufferSizeMB() float64        { return c.ramBufferMB }
func (c *fakeConfig) GetMaxBufferedDocs() int            { return c.maxBufferedDocs }
func (c *fakeConfig) GetRAMPerThreadHardLimitMB() int    { return c.hardLimitMB }
func (c *fakeConfig) GetFlushPolicy() flushControlPolicy { return c.policy }
func (c *fakeConfig) GetInfoStream() util.InfoStream     { return c.infoStream }

type fakeOwner struct {
	pool          *fakePool
	deleteQueue   *fakeDeleteQueue
	resetCount    atomic.Int32
	subtractTotal atomic.Int32
}

func (o *fakeOwner) GetDeleteQueue() flushControlDeleteQueue { return o.deleteQueue }
func (o *fakeOwner) ResetDeleteQueue(poolSize int) int64 {
	o.resetCount.Add(1)
	o.deleteQueue = &fakeDeleteQueue{}
	return int64(poolSize)
}
func (o *fakeOwner) SubtractFlushedNumDocs(numDocs int) {
	o.subtractTotal.Add(int32(numDocs))
}
func (o *fakeOwner) GetPerThreadPool() flushControlPool { return o.pool }

// ----------------------------------------------------------------------------
// Builders & assertions
// ----------------------------------------------------------------------------

func newTestControl(t *testing.T, ramBufferMB float64, maxDocs, hardLimitMB int) (*DocumentsWriterFlushControl, *fakeOwner, *fakePool, *fakePolicy) {
	t.Helper()
	policy := &fakePolicy{}
	pool := &fakePool{}
	owner := &fakeOwner{
		pool:        pool,
		deleteQueue: &fakeDeleteQueue{},
	}
	cfg := &fakeConfig{
		ramBufferMB:     ramBufferMB,
		maxBufferedDocs: maxDocs,
		hardLimitMB:     hardLimitMB,
		policy:          policy,
		infoStream:      util.NoOpInfoStream,
	}
	return NewDocumentsWriterFlushControl(owner, cfg), owner, pool, policy
}

func registerDWPT(pool *fakePool, owner *fakeOwner, bytesUsed int64, numDocs int) *fakeDWPT {
	d := &fakeDWPT{deleteQueue: owner.deleteQueue}
	d.bytesUsed.Store(bytesUsed)
	d.numDocsInRAM.Store(int32(numDocs))
	pool.mu.Lock()
	pool.dwpts = append(pool.dwpts, d)
	pool.mu.Unlock()
	return d
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

func TestDocumentsWriterFlushControl_DoAfterDocument_AccountsActive(t *testing.T) {
	c, owner, pool, policy := newTestControl(t, 1.0, DISABLE_AUTO_FLUSH, 1945)
	dwpt := registerDWPT(pool, owner, 4096, 1)

	got := c.DoAfterDocument(dwpt)
	if got != nil {
		t.Fatalf("expected nil checkout, got %v", got)
	}
	if c.ActiveBytes() != 4096 {
		t.Fatalf("activeBytes = %d, want 4096", c.ActiveBytes())
	}
	if policy.calls.Load() != 1 {
		t.Fatalf("policy OnChange calls = %d, want 1", policy.calls.Load())
	}
	if c.GetFlushingBytes() != 0 {
		t.Fatalf("flushingBytes = %d, want 0", c.GetFlushingBytes())
	}
}

func TestDocumentsWriterFlushControl_HardLimit_TriggersFlush(t *testing.T) {
	// hardLimitMB=1 ⇒ 1MB; bytesUsed > limit forces SetFlushPending.
	c, owner, pool, _ := newTestControl(t, 16.0, DISABLE_AUTO_FLUSH, 1)
	dwpt := registerDWPT(pool, owner, 2*1024*1024, 1)
	dwpt.Lock() // emulate Lucene's IsHeldByCurrentThread invariant

	got := c.DoAfterDocument(dwpt)
	if got == nil {
		t.Fatalf("expected DWPT checkout when exceeding hard limit")
	}
	if !dwpt.IsFlushPending() {
		t.Fatalf("DWPT should be flush pending after hard-limit trip")
	}
	if c.GetFlushingBytes() == 0 {
		t.Fatalf("flushingBytes should be > 0 after marking pending")
	}
	dwpt.Unlock()
}

func TestDocumentsWriterFlushControl_SetFlushPending_AccountsBytes(t *testing.T) {
	c, owner, pool, _ := newTestControl(t, 16.0, DISABLE_AUTO_FLUSH, 1945)
	// Use a delta above ramBufferGranularity so DoAfterDocument actually
	// commits to the global accounting (granularity is min(16*1024, ramMB*1024)).
	dwpt := registerDWPT(pool, owner, 64*1024, 1)
	c.DoAfterDocument(dwpt)
	if c.ActiveBytes() == 0 {
		t.Fatalf("precondition: activeBytes must be set")
	}
	before := c.ActiveBytes()
	c.SetFlushPending(dwpt)
	if c.ActiveBytes() != 0 {
		t.Fatalf("activeBytes after pending = %d, want 0", c.ActiveBytes())
	}
	if c.GetFlushingBytes() != before {
		t.Fatalf("flushingBytes = %d, want %d", c.GetFlushingBytes(), before)
	}
}

func TestDocumentsWriterFlushControl_NextPendingFlush_DrainsQueue(t *testing.T) {
	c, owner, pool, _ := newTestControl(t, 16.0, DISABLE_AUTO_FLUSH, 1945)
	dwpt := registerDWPT(pool, owner, 4096, 1)
	dwpt.Lock()
	c.DoAfterDocument(dwpt) // accounts active
	c.SetFlushPending(dwpt) // moves to flushBytes
	// Manually push to flushQueue to simulate a full-flush ordering.
	c.mu.Lock()
	c.flushQueue.PushBack(flushControlDWPT(dwpt))
	c.mu.Unlock()

	if got := c.NextPendingFlush(); got != dwpt {
		t.Fatalf("NextPendingFlush returned %v, want dwpt", got)
	}
	if got := c.NextPendingFlush(); got != nil {
		t.Fatalf("queue should be empty, got %v", got)
	}
	dwpt.Unlock()
}

func TestDocumentsWriterFlushControl_Close_ReleasesObtain(t *testing.T) {
	c, _, _, _ := newTestControl(t, 16.0, DISABLE_AUTO_FLUSH, 1945)
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := c.ObtainAndLock(); err == nil {
		t.Fatalf("ObtainAndLock after Close should fail")
	} else if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected closed error, got %v", err)
	}
}

func TestDocumentsWriterFlushControl_ApplyAllDeletesFlag(t *testing.T) {
	c, _, _, _ := newTestControl(t, 16.0, DISABLE_AUTO_FLUSH, 1945)
	if c.GetApplyAllDeletes() {
		t.Fatalf("initial flag must be false")
	}
	c.SetApplyAllDeletes()
	if !c.GetApplyAllDeletes() {
		t.Fatalf("flag must be true after Set")
	}
	if !c.GetAndResetApplyAllDeletes() {
		t.Fatalf("GetAndReset must return previous true value")
	}
	if c.GetApplyAllDeletes() {
		t.Fatalf("flag must be false after Reset")
	}
}

func TestDocumentsWriterFlushControl_FindLargestNonPending(t *testing.T) {
	c, owner, pool, _ := newTestControl(t, 16.0, DISABLE_AUTO_FLUSH, 1945)
	// Sizes must exceed ramBufferGranularity (16kB) so DoAfterDocument commits.
	small := registerDWPT(pool, owner, 32*1024, 1)
	big := registerDWPT(pool, owner, 128*1024, 2)
	c.DoAfterDocument(small)
	c.DoAfterDocument(big)
	got := c.FindLargestNonPendingWriter()
	if got != big {
		t.Fatalf("FindLargestNonPendingWriter = %v, want big", got)
	}
}

func TestDocumentsWriterFlushControl_DeleteBytesAndRAMBytes(t *testing.T) {
	c, owner, pool, _ := newTestControl(t, 16.0, DISABLE_AUTO_FLUSH, 1945)
	owner.deleteQueue.ramBytes = 512
	d := registerDWPT(pool, owner, 4096, 1)
	c.DoAfterDocument(d)
	if c.GetDeleteBytesUsed() != 512 {
		t.Fatalf("deleteBytesUsed = %d, want 512", c.GetDeleteBytesUsed())
	}
	if c.RAMBytesUsed() != 512+c.NetBytes() {
		t.Fatalf("RAMBytesUsed accounting mismatch")
	}
}

func TestDocumentsWriterFlushControl_String_NoPanic(t *testing.T) {
	c, _, _, _ := newTestControl(t, 16.0, DISABLE_AUTO_FLUSH, 1945)
	if got := c.String(); got == "" {
		t.Fatalf("String() returned empty")
	}
}

// ----------------------------------------------------------------------------
// documentsWriterStallControl tests
// ----------------------------------------------------------------------------

func TestStallControl_StartsHealthy(t *testing.T) {
	s := newDocumentsWriterStallControl()
	if !s.IsHealthy() {
		t.Fatalf("new stall controller must be healthy")
	}
	if s.WasStalled() {
		t.Fatalf("new stall controller must not record wasStalled")
	}
}

func TestStallControl_UpdateStalled_TogglesAndRecords(t *testing.T) {
	s := newDocumentsWriterStallControl()
	s.UpdateStalled(true)
	if s.IsHealthy() {
		t.Fatalf("must be unhealthy after UpdateStalled(true)")
	}
	if !s.AnyStalledThreads() {
		t.Fatalf("AnyStalledThreads must be true")
	}
	if !s.WasStalled() {
		t.Fatalf("WasStalled must be true")
	}
	s.UpdateStalled(false)
	if !s.IsHealthy() {
		t.Fatalf("must be healthy after UpdateStalled(false)")
	}
}

func TestStallControl_WaitIfStalled_NoOpWhenHealthy(t *testing.T) {
	s := newDocumentsWriterStallControl()
	done := make(chan struct{})
	go func() { s.WaitIfStalled(); close(done) }()
	<-done // must return immediately
}

func TestStallControl_WaitIfStalled_ReleasesOnUpdate(t *testing.T) {
	s := newDocumentsWriterStallControl()
	s.UpdateStalled(true)
	done := make(chan struct{})
	go func() { s.WaitIfStalled(); close(done) }()
	// give the goroutine time to park on cond.Wait
	for i := 0; i < 100; i++ {
		if s.GetNumWaiting() > 0 {
			break
		}
	}
	s.UpdateStalled(false)
	<-done
	if s.GetNumWaiting() != 0 {
		t.Fatalf("numWaiting = %d, want 0", s.GetNumWaiting())
	}
}
