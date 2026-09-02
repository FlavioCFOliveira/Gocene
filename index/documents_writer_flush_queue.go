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
)

// Port of org.apache.lucene.index.DocumentsWriterFlushQueue from Apache
// Lucene 10.4.0 (commit 9983b7c). This is a package-private helper that
// serializes the publication of flushed segments and frozen global update
// packets to the IndexWriter. Tickets are appended in the exact order in
// which they were enqueued; consumers drain the queue in order via
// ForcePurge / TryPurge so that segment publication remains deterministic
// even when flushes complete out of order on background threads.
//
// Naming deviations from Lucene (see [[project-gocene-store-bc]] for the
// broader extend-don't-rewrite stance):
//   - The Lucene nested type FlushTicket is exposed here as FlushQueueTicket
//     because the existing port DocumentsWriterPerThread.FlushTicket already
//     owns the FlushTicket identifier with different semantics (Sprint 55
//     decision Q1). FlushQueueTicket is the queue-side ticket; FlushTicket
//     is the DWPT-side flush payload.
//   - FlushedSegment is aliased to FlushTicket (decision Q3) since the DWPT
//     port reuses the FlushTicket struct as the in-memory flushed-segment
//     handoff. When a dedicated FlushedSegment port lands, swap the alias.
//   - FrozenBufferedUpdates is now the real port from
//     frozen_buffered_updates.go (Sprint 55 GOC-3360). The queue still
//     treats it as an opaque payload, so only the identity matters here.
//
// Concurrency contract (mirrors Lucene):
//   - addTicket / addSegment / markTicketFailed / hasTickets / getTicketCount
//     execute under the queue monitor (mu). They never call user code, so
//     the lock is always released before any I/O.
//   - innerPurge runs under purgeLock (a separate mutex) and drains
//     publishable tickets from the head of the queue. It releases mu while
//     calling the consumer so that concurrent producers can keep enqueuing.
//   - forcePurge blocks on purgeLock; tryPurge attempts a non-blocking
//     acquisition and returns immediately if another goroutine is already
//     purging. Both refuse to run while the caller already holds mu, to
//     prevent the same lock-ordering deadlock the Java assertion guards
//     against (!Thread.holdsLock(this)).

// ErrFlushQueueAlreadyPublished is returned by FlushQueueTicket.MarkPublished
// when the ticket has already been published. Lucene fires an assertion
// instead; surfacing an error keeps the Go port debuggable in production
// builds where assertions are disabled.
var ErrFlushQueueAlreadyPublished = errors.New("flush queue ticket was already published")

// FlushedSegment is the queue-side view of a successfully flushed DWPT.
// Aliased to FlushTicket per Sprint 55 decision Q3; replace with a
// dedicated type when the FlushedSegment port lands.
type FlushedSegment = FlushTicket

// FrozenBufferedUpdates is defined in frozen_buffered_updates.go and is
// treated as an opaque payload by the flush queue.

// flushQueueTicketSupplier mirrors java.util.function.Supplier<FlushTicket>.
// Returning a nil ticket signals that no ticket should be enqueued (the
// Lucene supplier may return null when there is nothing to freeze); the
// ticket counter is rolled back in that case to keep hasTickets correct.
type flushQueueTicketSupplier func() (*FlushQueueTicket, error)

// flushQueueTicketConsumer mirrors org.apache.lucene.util.IOConsumer.
// Returning an error short-circuits the purge loop; the caller is expected
// to propagate the error to the IndexWriter.
type flushQueueTicketConsumer func(*FlushQueueTicket) error

// DocumentsWriterFlushQueue serializes publication of flushed segments and
// frozen global update packets. It is goroutine-safe; see file header for
// the locking contract.
type DocumentsWriterFlushQueue struct {
	mu          sync.Mutex
	purgeLock   sync.Mutex
	queue       *list.List // *FlushQueueTicket, head = oldest
	ticketCount atomic.Int32
}

// NewDocumentsWriterFlushQueue constructs an empty queue ready for use.
func NewDocumentsWriterFlushQueue() *DocumentsWriterFlushQueue {
	return &DocumentsWriterFlushQueue{queue: list.New()}
}

// AddTicket atomically (1) increments the ticket counter, (2) invokes
// supplier under the queue monitor and (3) enqueues the returned ticket if
// non-nil. If the supplier returns a nil ticket or an error, the counter
// is rolled back to keep HasTickets / GetTicketCount accurate.
//
// The Java original is declared synchronized; the Go port acquires mu for
// the entire call so the supplier observes the same monitor semantics. The
// supplier must therefore not perform blocking I/O.
func (q *DocumentsWriterFlushQueue) AddTicket(supplier flushQueueTicketSupplier) (*FlushQueueTicket, error) {
	if supplier == nil {
		return nil, errors.New("flush queue: supplier must not be nil")
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	// Mirror Lucene: bump the counter first so #anyChanges sees the
	// in-flight ticket even before the supplier runs to completion.
	q.incTicketsLocked()
	success := false
	defer func() {
		if !success {
			q.decTicketsLocked()
		}
	}()

	ticket, err := supplier()
	if err != nil {
		return nil, fmt.Errorf("flush queue: ticket supplier failed: %w", err)
	}
	if ticket == nil {
		// Supplier had nothing to freeze; release the counter and return.
		return nil, nil
	}
	q.queue.PushBack(ticket)
	success = true
	return ticket, nil
}

// AddSegment attaches a freshly flushed segment to an existing ticket. The
// ticket must have been created with hasSegment=true; the queue does not
// reorder, so the segment lands at the ticket's original queue position.
// Returns an error rather than firing an assertion when the ticket is the
// global-deletes flavor.
func (q *DocumentsWriterFlushQueue) AddSegment(ticket *FlushQueueTicket, segment *FlushedSegment) error {
	if ticket == nil {
		return errors.New("flush queue: ticket must not be nil")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if !ticket.hasSegment {
		return errors.New("flush queue: AddSegment called on a global-deletes ticket")
	}
	ticket.setSegment(segment)
	return nil
}

// MarkTicketFailed marks a segment-bearing ticket as failed so the purge
// loop can drop it without producing a published segment. Mirrors the
// Lucene synchronized method.
func (q *DocumentsWriterFlushQueue) MarkTicketFailed(ticket *FlushQueueTicket) error {
	if ticket == nil {
		return errors.New("flush queue: ticket must not be nil")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if !ticket.hasSegment {
		return errors.New("flush queue: MarkTicketFailed called on a global-deletes ticket")
	}
	ticket.setFailed()
	return nil
}

// HasTickets reports whether at least one ticket is currently outstanding
// (either queued or still being constructed by a concurrent AddTicket).
func (q *DocumentsWriterFlushQueue) HasTickets() bool {
	n := q.ticketCount.Load()
	if n < 0 {
		// Java fires an assertion here; in Go we still report no tickets
		// rather than panicking so the writer can keep making progress.
		return false
	}
	return n != 0
}

// GetTicketCount returns the current ticket count, including in-flight
// tickets whose supplier has not yet produced a value.
func (q *DocumentsWriterFlushQueue) GetTicketCount() int {
	return int(q.ticketCount.Load())
}

// ForcePurge drains every publishable head ticket, blocking on purgeLock
// until the goroutine currently purging (if any) finishes. The caller must
// not hold the queue monitor; doing so would invert the lock order with
// purgeLock and deadlock.
func (q *DocumentsWriterFlushQueue) ForcePurge(consumer flushQueueTicketConsumer) error {
	if consumer == nil {
		return errors.New("flush queue: consumer must not be nil")
	}
	q.purgeLock.Lock()
	defer q.purgeLock.Unlock()
	return q.innerPurge(consumer)
}

// TryPurge is the non-blocking counterpart of ForcePurge. It returns nil
// immediately if another goroutine is already purging.
func (q *DocumentsWriterFlushQueue) TryPurge(consumer flushQueueTicketConsumer) error {
	if consumer == nil {
		return errors.New("flush queue: consumer must not be nil")
	}
	if !q.purgeLock.TryLock() {
		return nil
	}
	defer q.purgeLock.Unlock()
	return q.innerPurge(consumer)
}

// innerPurge drains publishable tickets from the head. purgeLock must be
// held; mu is acquired and released around each peek/poll cycle so that
// concurrent producers can keep enqueuing while the consumer runs.
func (q *DocumentsWriterFlushQueue) innerPurge(consumer flushQueueTicketConsumer) error {
	for {
		var head *FlushQueueTicket
		var canPublish bool

		q.mu.Lock()
		if front := q.queue.Front(); front != nil {
			head = front.Value.(*FlushQueueTicket)
			canPublish = head.CanPublish()
		}
		q.mu.Unlock()

		if !canPublish {
			return nil
		}

		consumerErr := consumer(head)

		q.mu.Lock()
		front := q.queue.Front()
		if front == nil {
			q.mu.Unlock()
			return fmt.Errorf("flush queue: queue head vanished during purge")
		}
		polled := q.queue.Remove(front).(*FlushQueueTicket)
		q.decTicketsLocked()
		q.mu.Unlock()

		// Defensive: holding purgeLock should prevent any other goroutine
		// from polling, so polled must match head. If they diverge a bug
		// elsewhere broke the invariant.
		if polled != head {
			return fmt.Errorf("flush queue: concurrent poll detected (head=%p polled=%p)", head, polled)
		}
		if consumerErr != nil {
			return consumerErr
		}
	}
}

// incTicketsLocked / decTicketsLocked must be called with mu held. They
// preserve the Lucene invariants that the counter never goes negative.
func (q *DocumentsWriterFlushQueue) incTicketsLocked() {
	n := q.ticketCount.Add(1)
	if n <= 0 {
		// Should be unreachable; Java asserts > 0. Roll back so the queue
		// stays in a sane state if this ever fires.
		q.ticketCount.Add(-1)
	}
}

func (q *DocumentsWriterFlushQueue) decTicketsLocked() {
	n := q.ticketCount.Add(-1)
	if n < 0 {
		// Match the Lucene "assert numTickets >= 0" guard by clamping.
		q.ticketCount.Store(0)
	}
}

// FlushQueueTicket is the queue-side ticket carried by
// DocumentsWriterFlushQueue. See file header for the rationale behind the
// FlushQueueTicket vs FlushTicket naming split (Sprint 55 decision Q1).
//
// A ticket is publishable once:
//   - it is a global-deletes ticket (hasSegment == false), or
//   - its segment has been attached via AddSegment, or
//   - it has been marked failed via MarkTicketFailed.
//
// Tickets are single-publish: MarkPublished returns
// ErrFlushQueueAlreadyPublished on a second call so callers can detect
// double-publication without losing state.
type FlushQueueTicket struct {
	mu            sync.Mutex
	frozenUpdates *FrozenBufferedUpdates
	hasSegment    bool
	segment       *FlushedSegment
	failed        bool
	published     bool
}

// NewFlushQueueTicket constructs a ticket carrying an optional frozen
// global-deletes packet. hasSegment must be true iff a segment will be
// attached later via DocumentsWriterFlushQueue.AddSegment.
func NewFlushQueueTicket(frozenUpdates *FrozenBufferedUpdates, hasSegment bool) *FlushQueueTicket {
	return &FlushQueueTicket{frozenUpdates: frozenUpdates, hasSegment: hasSegment}
}

// CanPublish reports whether this ticket is ready to be drained by the
// purge loop. It mirrors the Lucene method of the same name.
func (t *FlushQueueTicket) CanPublish() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.hasSegment || t.segment != nil || t.failed
}

// MarkPublished marks the ticket as published exactly once. Returns
// ErrFlushQueueAlreadyPublished on a second call.
func (t *FlushQueueTicket) MarkPublished() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.published {
		return ErrFlushQueueAlreadyPublished
	}
	t.published = true
	return nil
}

// GetFlushedSegment returns the attached segment or nil if this is a
// global-deletes ticket (or the segment has not been attached yet).
func (t *FlushQueueTicket) GetFlushedSegment() *FlushedSegment {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.segment
}

// GetFrozenUpdates returns the frozen global-deletes packet attached at
// construction. May be nil for segment-only tickets.
func (t *FlushQueueTicket) GetFrozenUpdates() *FrozenBufferedUpdates {
	return t.frozenUpdates
}

// HasSegment reports whether this ticket expects a segment to be attached
// before it becomes publishable.
func (t *FlushQueueTicket) HasSegment() bool {
	return t.hasSegment
}

// setSegment attaches a flushed segment. Callers must hold the queue
// monitor; the ticket itself locks internally to keep CanPublish coherent.
func (t *FlushQueueTicket) setSegment(segment *FlushedSegment) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.failed {
		return // mirrors the Java assertion !failed; drop the update.
	}
	t.segment = segment
}

// setFailed marks the ticket as failed. Same locking notes as setSegment.
func (t *FlushQueueTicket) setFailed() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.segment != nil {
		return // mirrors the Java assertion segment == null.
	}
	t.failed = true
}
