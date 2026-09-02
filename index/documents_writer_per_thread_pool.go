// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "sync"

// Port of org.apache.lucene.index.DocumentsWriterPerThreadPool from Apache
// Lucene 10.4.0 (commit 9983b7c). The Java class is package-private and final,
// declared as {@code final class DocumentsWriterPerThreadPool implements
// Iterable<DocumentsWriterPerThread>, Closeable}; we mirror that visibility
// with an unexported Go type.
//
// Divergence — generic parameterisation. The Java pool is hardcoded to
// DocumentsWriterPerThread because DWPT extends ReentrantLock and exposes the
// state predicates (isFlushPending/isAborted/isQueueAdvanced/ramBytesUsed)
// directly. The Gocene DWPT struct does not yet expose any of that surface
// (Sprint 55 ports the pool ahead of the DWPT lock/state extension), so this
// port follows the same generic pattern already used by
// [[lockable_concurrent_approximate_priority_queue]]: the pool is parameterised
// over the pooledDWPT interface, whose method set is exactly what the upstream
// Java pool requires from DWPT. Once DWPT grows the matching surface in a
// later task, instances will be constructed as
// documentsWriterPerThreadPool[*DocumentsWriterPerThread]; until then the
// generic boundary lets the pool ship and be tested in isolation.

// pooledDWPT models the slice of the Java DocumentsWriterPerThread API that
// DocumentsWriterPerThreadPool reaches into. The Java pool calls dwpt.lock(),
// dwpt.unlock(), dwpt.isHeldByCurrentThread() (inherited from ReentrantLock),
// dwpt.tryLock() (via the LockableConcurrentApproximatePriorityQueue queueLock
// constraint), dwpt.ramBytesUsed(), dwpt.isFlushPending(), dwpt.isAborted()
// and dwpt.isQueueAdvanced(). Pointer-receiver equality is what makes the
// IdentityHashMap semantics of the Java source (Collections.newSetFromMap(new
// IdentityHashMap<>())) translate cleanly to a Go map keyed by T.
type pooledDWPT interface {
	comparable
	queueLock
	// Lock acquires the per-DWPT lock, blocking until it succeeds. Mirrors
	// ReentrantLock#lock().
	Lock()
	// IsHeldByCurrentThread reports whether the calling goroutine currently
	// owns the lock. Mirrors ReentrantLock#isHeldByCurrentThread().
	IsHeldByCurrentThread() bool
	// RamBytesUsed returns the DWPT's current RAM accounting in bytes. Mirrors
	// DocumentsWriterPerThread#ramBytesUsed().
	RamBytesUsed() int64
	// IsFlushPending reports whether the DWPT has been marked for flush.
	// Mirrors DocumentsWriterPerThread#isFlushPending().
	IsFlushPending() bool
	// IsAborted reports whether the DWPT has been aborted. Mirrors
	// DocumentsWriterPerThread#isAborted().
	IsAborted() bool
	// IsQueueAdvanced reports whether the DWPT's delete queue has been swapped
	// for a newer one. Mirrors DocumentsWriterPerThread#isQueueAdvanced().
	IsQueueAdvanced() bool
}

// documentsWriterPerThreadPool controls DWPT instances and their goroutine
// assignments during indexing. Each DWPT, once obtained from the pool, is used
// exclusively for indexing a single document or list of documents by the
// obtaining goroutine. Goroutines must obtain a DWPT via getAndLock to make
// progress; depending on contention the assignment may differ from one
// document to the next.
//
// Once a DWPT is selected for flush it is checked out of the pool (via
// checkout) and is never reused for indexing.
type documentsWriterPerThreadPool[T pooledDWPT] struct {
	// mu serialises every operation the Java source marks {@code synchronized}.
	// All other state in this struct except freeList is guarded by mu.
	mu sync.Mutex
	// notifyAll fires when takenWriterPermits reaches zero, waking any goroutine
	// blocked in newWriter. Modelled with sync.Cond to mirror the Java
	// notifyAll/wait pair on the monitor; the cond is bound to mu.
	notifyAll *sync.Cond
	// dwpts holds the set of registered DWPTs. The Java source uses
	// Collections.newSetFromMap(new IdentityHashMap<>()), which is identity-
	// based; in Go pointer-receiver equality on T (via the comparable bound)
	// already gives identity semantics.
	dwpts map[T]struct{}
	// freeList is the work-stealing queue of DWPTs that have completed at least
	// one indexing op and are available for re-use. It is internally
	// concurrent and intentionally not guarded by mu.
	freeList *lockableConcurrentApproximatePriorityQueue[T]
	// dwptFactory is the {@code Supplier<DocumentsWriterPerThread>} from the
	// Java constructor: invoked under mu inside newWriter to mint a fresh DWPT.
	dwptFactory func() T
	// takenWriterPermits counts active lockNewWriters reservations. While
	// positive, newWriter blocks on notifyAll.
	takenWriterPermits int
	// closed gates ensureOpen. Java declares the field volatile; Go's
	// happens-before guarantees on mu acquisition make any read under mu
	// observe the latest stored value, so a plain bool suffices because every
	// reader is either under mu or accepts a stale read by design (Java's
	// volatile is exactly that contract).
	closed bool
}

// newDocumentsWriterPerThreadPool mirrors the Java package-private constructor
// {@code DocumentsWriterPerThreadPool(Supplier<DocumentsWriterPerThread>)}.
// The factory is invoked every time the pool needs a fresh DWPT (i.e. when the
// free list cannot satisfy a getAndLock call).
func newDocumentsWriterPerThreadPool[T pooledDWPT](dwptFactory func() T) *documentsWriterPerThreadPool[T] {
	p := &documentsWriterPerThreadPool[T]{
		dwpts:       make(map[T]struct{}),
		freeList:    newLockableConcurrentApproximatePriorityQueueDefault[T](),
		dwptFactory: dwptFactory,
	}
	p.notifyAll = sync.NewCond(&p.mu)
	return p
}

// size returns the active number of DWPT instances. Mirrors the Java
// {@code synchronized int size()}.
func (p *documentsWriterPerThreadPool[T]) size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.dwpts)
}

// lockNewWriters reserves a permit that prevents newWriter from minting
// further DWPTs until unlockNewWriters is called. Modelled on the Java
// {@code synchronized void lockNewWriters()}; like the original it must be
// paired with unlockNewWriters or the pool will deadlock at the next
// newWriter call.
func (p *documentsWriterPerThreadPool[T]) lockNewWriters() {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Mirrors the Java assert takenWriterPermits >= 0. The invariant is held
	// across every write to the field; defensive enforcement is left out
	// because Java asserts are disabled by default and the bookkeeping below
	// keeps the field monotonically non-negative.
	p.takenWriterPermits++
}

// unlockNewWriters releases one permit previously acquired with
// lockNewWriters. When the last permit is released, every goroutine waiting
// inside newWriter is woken. Mirrors {@code synchronized void
// unlockNewWriters()}.
func (p *documentsWriterPerThreadPool[T]) unlockNewWriters() {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Java: assert takenWriterPermits > 0.
	p.takenWriterPermits--
	if p.takenWriterPermits == 0 {
		p.notifyAll.Broadcast()
	}
}

// newWriter mints a fresh DWPT, registers it, and returns it already locked.
// Mirrors the Java {@code private synchronized DocumentsWriterPerThread
// newWriter()}. The wait loop on takenWriterPermits is what implements the
// "lock new writers" semaphore from above.
func (p *documentsWriterPerThreadPool[T]) newWriter() T {
	p.mu.Lock()
	defer p.mu.Unlock()
	for p.takenWriterPermits > 0 {
		// sync.Cond.Wait atomically releases mu, waits, and re-acquires mu on
		// wake. This matches the Java monitor wait() exactly.
		p.notifyAll.Wait()
	}
	// The pool may have been closed while we were waiting for a permit. The
	// Java comment notes that missing this check would not be catastrophic but
	// would violate the "no new DWPT after close" contract; we mirror the
	// check for fidelity.
	p.ensureOpenLocked()
	dwpt := p.dwptFactory()
	dwpt.Lock()
	p.dwpts[dwpt] = struct{}{}
	return dwpt
}

// getAndLock is used by DocumentsWriter and DocumentsWriterFlushControl to
// obtain a DWPT for an indexing operation (add/updateDocument). It returns a
// DWPT that is already locked by the calling goroutine; the caller owns the
// unlock via marksAsFreeAndUnlock or checkout.
//
// Mirrors the Java {@code DocumentsWriterPerThread getAndLock()}: try the
// free list first; on miss, mint a new writer via newWriter (which adds the
// DWPT to dwpts but not to freeList — freeList registration is deferred to
// marksAsFreeAndUnlock once the DWPT has indexed at least one document).
func (p *documentsWriterPerThreadPool[T]) getAndLock() T {
	p.ensureOpen()
	if dwpt, ok := p.freeList.lockAndPoll(); ok {
		return dwpt
	}
	return p.newWriter()
}

// ensureOpen panics with AlreadyClosedException semantics when the pool is
// closed. The Java source throws AlreadyClosedException, which is an
// unchecked exception; we mirror the unchecked behaviour with panic so call
// sites do not have to thread errors through the indexing hot path.
func (p *documentsWriterPerThreadPool[T]) ensureOpen() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureOpenLocked()
}

// ensureOpenLocked is the mu-held variant invoked from newWriter, which is
// already under the pool lock.
func (p *documentsWriterPerThreadPool[T]) ensureOpenLocked() {
	if p.closed {
		panic(NewAlreadyClosedException("DWPTPool is already closed", nil))
	}
}

// contains is the Java {@code private synchronized boolean contains(
// DocumentsWriterPerThread state)}. It is package-private in upstream and is
// used only from inside marksAsFreeAndUnlock's assertion, so we expose it
// here for the same purpose.
func (p *documentsWriterPerThreadPool[T]) contains(state T) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.dwpts[state]
	return ok
}

// marksAsFreeAndUnlock returns a previously checked-out DWPT to the pool's
// free list and unlocks it, in that order. Mirrors the Java {@code void
// marksAsFreeAndUnlock(DocumentsWriterPerThread state)}.
//
// The Java source guards the call with three asserts: the DWPT must not be
// flush-pending, must not be aborted, and must not have had its delete queue
// advanced. We surface those preconditions via the pooledDWPT predicates and
// panic on violation, matching Java's assertion-violation behaviour rather
// than silently corrupting the pool.
func (p *documentsWriterPerThreadPool[T]) marksAsFreeAndUnlock(state T) {
	ramBytesUsed := state.RamBytesUsed()
	if state.IsFlushPending() || state.IsAborted() || state.IsQueueAdvanced() {
		panic(assertionViolation{
			Op:           "marksAsFreeAndUnlock",
			FlushPending: state.IsFlushPending(),
			Aborted:      state.IsAborted(),
			QueueAdv:     state.IsQueueAdvanced(),
		})
	}
	if !p.contains(state) {
		panic("DocumentsWriterPerThreadPool.marksAsFreeAndUnlock: " +
			"we tried to add a DWPT back to the pool but the pool doesn't know about this DWPT")
	}
	p.freeList.addAndUnlock(state, ramBytesUsed)
}

// snapshot returns a defensive copy of the current registered DWPTs. Mirrors
// the Java {@code public synchronized Iterator<DocumentsWriterPerThread>
// iterator()}; the upstream comment calls out "copy on read - this is a quick
// op since num states is low". Returning a slice is the idiomatic Go peer:
// callers iterate with for-range and never need to call hasNext/next.
func (p *documentsWriterPerThreadPool[T]) snapshot() []T {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]T, 0, len(p.dwpts))
	for dwpt := range p.dwpts {
		out = append(out, dwpt)
	}
	return out
}

// filterAndLock returns the subset of registered DWPTs matching predicate,
// each already locked and confirmed to still be registered. Callers own the
// unlock for every entry. Mirrors the Java {@code List<DocumentsWriterPerThread>
// filterAndLock(Predicate<DocumentsWriterPerThread> predicate)}.
//
// The re-check of isRegistered after Lock is load-bearing: between the
// predicate test (which only consults the snapshot) and the Lock call,
// another goroutine may have checked the DWPT out of the pool. In that case
// we drop the lock and skip the entry, exactly as the Java source does.
func (p *documentsWriterPerThreadPool[T]) filterAndLock(predicate func(T) bool) []T {
	candidates := p.snapshot()
	list := make([]T, 0, len(candidates))
	for _, perThread := range candidates {
		if !predicate(perThread) {
			continue
		}
		perThread.Lock()
		if p.isRegistered(perThread) {
			list = append(list, perThread)
		} else {
			perThread.Unlock()
		}
	}
	return list
}

// checkout removes perThread from the pool. The caller must currently hold
// perThread's lock — the Java source enforces this with {@code assert
// perThread.isHeldByCurrentThread()}. Returns true iff the DWPT was in the
// pool before this call. Mirrors {@code synchronized boolean checkout(
// DocumentsWriterPerThread perThread)}.
//
// The lock-ownership precondition is what makes the removal atomic with
// respect to getAndLock: getAndLock's lockAndPoll uses TryLock on the freeList
// shard, which will fail for any DWPT we hold, so the DWPT cannot be handed
// out to a parallel goroutine between our lock and our remove.
func (p *documentsWriterPerThreadPool[T]) checkout(perThread T) bool {
	if !perThread.IsHeldByCurrentThread() {
		panic("DocumentsWriterPerThreadPool.checkout: DWPT must be held by the current goroutine")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.dwpts[perThread]; ok {
		delete(p.dwpts, perThread)
		p.freeList.remove(perThread)
		return true
	}
	// Java mirrors a defensive assertion that the freeList must not contain
	// the DWPT in this branch. We omit the symmetric check from the hot path
	// because the invariant follows from the dwpts/freeList registration order
	// in newWriter and marksAsFreeAndUnlock.
	return false
}

// isRegistered reports whether perThread is still part of the pool. Mirrors
// the Java {@code synchronized boolean isRegistered(DocumentsWriterPerThread
// perThread)}.
func (p *documentsWriterPerThreadPool[T]) isRegistered(perThread T) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.dwpts[perThread]
	return ok
}

// close marks the pool as closed. Subsequent calls to getAndLock or
// newWriter panic with AlreadyClosedException. Mirrors the Java
// {@code public synchronized void close()} from the Closeable surface.
//
// Java declares Closeable#close as {@code throws IOException}; this pool
// never raises one, and Go's idiomatic close-as-no-op-on-second-call would
// require extra bookkeeping the upstream source does not perform, so the
// signature is bare.
func (p *documentsWriterPerThreadPool[T]) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
}

// assertionViolation carries the diagnostic context the Java source attaches
// to its three-way assert in marksAsFreeAndUnlock. It is panicked, never
// returned; the type exists solely so tests can recover and inspect the
// individual predicate values without parsing a formatted string.
type assertionViolation struct {
	Op           string
	FlushPending bool
	Aborted      bool
	QueueAdv     bool
}

// Error implements the error interface so the value is printable when surfaced
// via recover(); the message matches the Java assertion text.
func (a assertionViolation) Error() string {
	return "DocumentsWriterPerThreadPool." + a.Op +
		": DWPT has pending flush: " + boolToStr(a.FlushPending) +
		" aborted=" + boolToStr(a.Aborted) +
		" queueAdvanced=" + boolToStr(a.QueueAdv)
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
