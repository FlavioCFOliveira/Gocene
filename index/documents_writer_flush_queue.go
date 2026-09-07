// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"
	"sync/atomic"
)

// FlushTicket represents a pending flush operation in the DocumentsWriterFlushQueue.
type FlushTicket struct {
	mu            sync.Mutex
	frozenUpdates *FrozenBufferedUpdates
	hasSegment    bool
	segment       *FlushedSegment
	failed        bool
	published     bool
}

// NewFlushTicket creates a new FlushTicket.
func NewFlushTicket(frozenUpdates *FrozenBufferedUpdates, hasSegment bool) *FlushTicket {
	return &FlushTicket{
		frozenUpdates: frozenUpdates,
		hasSegment:    hasSegment,
	}
}

// CanPublish returns true if the ticket is ready to be published.
func (t *FlushTicket) CanPublish() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.hasSegment || t.segment != nil || t.failed
}

// MarkPublished marks the ticket as published.
func (t *FlushTicket) MarkPublished() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.published {
		panic("ticket was already published - can not publish twice")
	}
	t.published = true
}

func (t *FlushTicket) setSegment(segment *FlushedSegment) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.failed {
		panic("cannot set segment on failed ticket")
	}
	t.segment = segment
}

func (t *FlushTicket) setFailed() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.segment != nil {
		panic("cannot mark ticket as failed if it has a segment")
	}
	t.failed = true
}

// GetFlushedSegment returns the flushed segment or nil if this flush ticket doesn't have a segment.
func (t *FlushTicket) GetFlushedSegment() *FlushedSegment {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.segment
}

// GetFrozenUpdates returns the frozen global deletes package.
func (t *FlushTicket) GetFrozenUpdates() *FrozenBufferedUpdates {
	return t.frozenUpdates
}

// DocumentsWriterFlushQueue manages the queue of pending flushes for the DocumentsWriter.
type DocumentsWriterFlushQueue struct {
	mu          sync.Mutex
	queue       []*FlushTicket
	ticketCount atomic.Int32
	purgeMu     sync.Mutex
}

// NewDocumentsWriterFlushQueue creates a new DocumentsWriterFlushQueue.
func NewDocumentsWriterFlushQueue() *DocumentsWriterFlushQueue {
	return &DocumentsWriterFlushQueue{
		queue: make([]*FlushTicket, 0),
	}
}

// AddTicket adds a new ticket to the flush queue.
func (q *DocumentsWriterFlushQueue) AddTicket(ticketSupplier func() (*FlushTicket, error)) (*FlushTicket, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.incTickets()

	ticket, err := ticketSupplier()
	if err != nil {
		q.decTickets()
		return nil, err
	}

	if ticket != nil {
		q.queue = append(q.queue, ticket)
		return ticket, nil
	}

	q.decTickets()
	return nil, nil
}

func (q *DocumentsWriterFlushQueue) incTickets() {
	q.ticketCount.Add(1)
}

func (q *DocumentsWriterFlushQueue) decTickets() {
	q.ticketCount.Add(-1)
}

// AddSegment associates a flushed segment with a ticket.
func (q *DocumentsWriterFlushQueue) AddSegment(ticket *FlushTicket, segment *FlushedSegment) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !ticket.hasSegment {
		panic("ticket must have segment")
	}
	ticket.setSegment(segment)
}

// MarkTicketFailed marks a ticket as failed to allow the queue to be cleared.
func (q *DocumentsWriterFlushQueue) MarkTicketFailed(ticket *FlushTicket) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !ticket.hasSegment {
		panic("ticket must have segment")
	}
	ticket.setFailed()
}

// HasTickets returns true if there are any tickets in the queue.
func (q *DocumentsWriterFlushQueue) HasTickets() bool {
	return q.ticketCount.Load() != 0
}

// GetTicketCount returns the number of tickets currently in the queue.
func (q *DocumentsWriterFlushQueue) GetTicketCount() int {
	return int(q.ticketCount.Load())
}

func (q *DocumentsWriterFlushQueue) innerPurge(consumer func(*FlushTicket) error) error {
	for {
		var head *FlushTicket
		var canPublish bool

		q.mu.Lock()
		if len(q.queue) > 0 {
			head = q.queue[0]
			canPublish = head.CanPublish()
		}
		q.mu.Unlock()

		if canPublish {
			if err := consumer(head); err != nil {
				return err
			}

			q.mu.Lock()
			if len(q.queue) > 0 && q.queue[0] == head {
				q.queue = q.queue[1:]
				q.decTickets()
			}
			q.mu.Unlock()
		} else {
			break
		}
	}
	return nil
}

// ForcePurge forces a purge of the flush queue.
func (q *DocumentsWriterFlushQueue) ForcePurge(consumer func(*FlushTicket) error) error {
	q.purgeMu.Lock()
	defer q.purgeMu.Unlock()
	return q.innerPurge(consumer)
}

// TryPurge attempts to purge the flush queue if the purge lock can be acquired.
func (q *DocumentsWriterFlushQueue) TryPurge(consumer func(*FlushTicket) error) error {
	if q.purgeMu.TryLock() {
		defer q.purgeMu.Unlock()
		return q.innerPurge(consumer)
	}
	return nil
}
