// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// segTicket is a tiny helper that builds a segment-bearing ticket and
// attaches a FlushedSegment to it, mirroring the typical Lucene flow.
func segTicket(t *testing.T) *FlushQueueTicket {
	t.Helper()
	return NewFlushQueueTicket(nil, true)
}

func TestFlushQueue_AddTicket_EnqueuesSegmentTicket(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()

	ticket, err := q.AddTicket(func() (*FlushQueueTicket, error) {
		return segTicket(t), nil
	})
	if err != nil {
		t.Fatalf("AddTicket returned error: %v", err)
	}
	if ticket == nil {
		t.Fatal("AddTicket returned nil ticket for non-nil supplier")
	}
	if got := q.GetTicketCount(); got != 1 {
		t.Fatalf("GetTicketCount = %d, want 1", got)
	}
	if !q.HasTickets() {
		t.Fatal("HasTickets = false, want true after AddTicket")
	}
}

func TestFlushQueue_AddTicket_NilSupplierResultRollsBackCounter(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()

	ticket, err := q.AddTicket(func() (*FlushQueueTicket, error) { return nil, nil })
	if err != nil {
		t.Fatalf("AddTicket returned error: %v", err)
	}
	if ticket != nil {
		t.Fatalf("AddTicket returned %v, want nil", ticket)
	}
	if got := q.GetTicketCount(); got != 0 {
		t.Fatalf("GetTicketCount = %d, want 0 after nil supplier", got)
	}
	if q.HasTickets() {
		t.Fatal("HasTickets = true, want false after nil supplier")
	}
}

func TestFlushQueue_AddTicket_SupplierErrorRollsBackCounter(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	boom := errors.New("supplier failed")

	_, err := q.AddTicket(func() (*FlushQueueTicket, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Fatalf("AddTicket err = %v, want wrapping %v", err, boom)
	}
	if got := q.GetTicketCount(); got != 0 {
		t.Fatalf("GetTicketCount = %d, want 0 after supplier error", got)
	}
}

func TestFlushQueue_AddTicket_NilSupplierReturnsError(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	if _, err := q.AddTicket(nil); err == nil {
		t.Fatal("AddTicket(nil) returned no error")
	}
}

func TestFlushQueueTicket_CanPublish_StateMatrix(t *testing.T) {
	tests := []struct {
		name       string
		hasSegment bool
		attach     func(*FlushQueueTicket)
		want       bool
	}{
		{"global-deletes-no-segment", false, nil, true},
		{"segment-pending", true, nil, false},
		{"segment-attached", true, func(t *FlushQueueTicket) { t.setSegment(&FlushedSegment{}) }, true},
		{"segment-failed", true, func(t *FlushQueueTicket) { t.setFailed() }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ticket := NewFlushQueueTicket(nil, tc.hasSegment)
			if tc.attach != nil {
				tc.attach(ticket)
			}
			if got := ticket.CanPublish(); got != tc.want {
				t.Fatalf("CanPublish = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFlushQueueTicket_MarkPublished_Once(t *testing.T) {
	ticket := NewFlushQueueTicket(nil, false)
	if err := ticket.MarkPublished(); err != nil {
		t.Fatalf("first MarkPublished returned %v", err)
	}
	if err := ticket.MarkPublished(); !errors.Is(err, ErrFlushQueueAlreadyPublished) {
		t.Fatalf("second MarkPublished err = %v, want %v", err, ErrFlushQueueAlreadyPublished)
	}
}

func TestFlushQueue_AddSegment_AttachesAndAllowsPublish(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	ticket, err := q.AddTicket(func() (*FlushQueueTicket, error) { return segTicket(t), nil })
	if err != nil {
		t.Fatalf("AddTicket: %v", err)
	}
	segment := &FlushedSegment{}
	if err := q.AddSegment(ticket, segment); err != nil {
		t.Fatalf("AddSegment: %v", err)
	}
	if ticket.GetFlushedSegment() != segment {
		t.Fatal("GetFlushedSegment did not return the attached segment")
	}
	if !ticket.CanPublish() {
		t.Fatal("CanPublish = false after AddSegment")
	}
}

func TestFlushQueue_AddSegment_GlobalDeletesTicketRejected(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	ticket := NewFlushQueueTicket(nil, false)
	if err := q.AddSegment(ticket, &FlushedSegment{}); err == nil {
		t.Fatal("AddSegment on global-deletes ticket returned no error")
	}
}

func TestFlushQueue_MarkTicketFailed_AllowsPublish(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	ticket, _ := q.AddTicket(func() (*FlushQueueTicket, error) { return segTicket(t), nil })
	if err := q.MarkTicketFailed(ticket); err != nil {
		t.Fatalf("MarkTicketFailed: %v", err)
	}
	if !ticket.CanPublish() {
		t.Fatal("CanPublish = false after MarkTicketFailed")
	}
}

func TestFlushQueue_ForcePurge_DrainsInFIFOOrder(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()

	tickets := make([]*FlushQueueTicket, 3)
	for i := range tickets {
		tk, err := q.AddTicket(func() (*FlushQueueTicket, error) { return NewFlushQueueTicket(nil, false), nil })
		if err != nil {
			t.Fatalf("AddTicket[%d]: %v", i, err)
		}
		tickets[i] = tk
	}

	var seen []*FlushQueueTicket
	consumer := func(tk *FlushQueueTicket) error {
		seen = append(seen, tk)
		return nil
	}
	if err := q.ForcePurge(consumer); err != nil {
		t.Fatalf("ForcePurge: %v", err)
	}
	if got := q.GetTicketCount(); got != 0 {
		t.Fatalf("GetTicketCount after purge = %d, want 0", got)
	}
	if len(seen) != len(tickets) {
		t.Fatalf("len(seen) = %d, want %d", len(seen), len(tickets))
	}
	for i := range tickets {
		if seen[i] != tickets[i] {
			t.Fatalf("FIFO violated at index %d: got %p, want %p", i, seen[i], tickets[i])
		}
	}
}

func TestFlushQueue_ForcePurge_StopsAtNonPublishableHead(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	// head: segment-pending (not publishable yet)
	_, _ = q.AddTicket(func() (*FlushQueueTicket, error) { return segTicket(t), nil })
	// tail: global deletes (always publishable)
	_, _ = q.AddTicket(func() (*FlushQueueTicket, error) { return NewFlushQueueTicket(nil, false), nil })

	var called int
	if err := q.ForcePurge(func(*FlushQueueTicket) error {
		called++
		return nil
	}); err != nil {
		t.Fatalf("ForcePurge: %v", err)
	}
	if called != 0 {
		t.Fatalf("consumer called %d times, want 0 (head not publishable)", called)
	}
	if got := q.GetTicketCount(); got != 2 {
		t.Fatalf("GetTicketCount = %d, want 2 (no tickets drained)", got)
	}
}

func TestFlushQueue_ForcePurge_PropagatesConsumerError(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	_, _ = q.AddTicket(func() (*FlushQueueTicket, error) { return NewFlushQueueTicket(nil, false), nil })
	_, _ = q.AddTicket(func() (*FlushQueueTicket, error) { return NewFlushQueueTicket(nil, false), nil })

	boom := errors.New("publish failed")
	calls := 0
	err := q.ForcePurge(func(*FlushQueueTicket) error {
		calls++
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("ForcePurge err = %v, want %v", err, boom)
	}
	if calls != 1 {
		t.Fatalf("consumer calls = %d, want 1 (purge must short-circuit)", calls)
	}
	// Lucene drains the polled ticket before propagating the error, so the
	// counter drops by one for the ticket that was consumed.
	if got := q.GetTicketCount(); got != 1 {
		t.Fatalf("GetTicketCount after error = %d, want 1", got)
	}
}

func TestFlushQueue_TryPurge_NonBlockingWhenPurgeHeld(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	_, _ = q.AddTicket(func() (*FlushQueueTicket, error) { return NewFlushQueueTicket(nil, false), nil })

	// Manually grab purgeLock to simulate another goroutine purging.
	q.purgeLock.Lock()
	defer q.purgeLock.Unlock()

	var called atomic.Int32
	if err := q.TryPurge(func(*FlushQueueTicket) error {
		called.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("TryPurge: %v", err)
	}
	if got := called.Load(); got != 0 {
		t.Fatalf("consumer called %d times, want 0 while purgeLock is held", got)
	}
}

func TestFlushQueue_TryPurge_DrainsWhenLockAvailable(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	_, _ = q.AddTicket(func() (*FlushQueueTicket, error) { return NewFlushQueueTicket(nil, false), nil })
	var called int
	if err := q.TryPurge(func(*FlushQueueTicket) error { called++; return nil }); err != nil {
		t.Fatalf("TryPurge: %v", err)
	}
	if called != 1 {
		t.Fatalf("consumer called %d times, want 1", called)
	}
}

func TestFlushQueue_ConcurrentProducersConsumer(t *testing.T) {
	const producers = 8
	const perProducer = 64

	q := NewDocumentsWriterFlushQueue()
	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				_, err := q.AddTicket(func() (*FlushQueueTicket, error) {
					return NewFlushQueueTicket(nil, false), nil
				})
				if err != nil {
					t.Errorf("AddTicket: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	want := producers * perProducer
	if got := q.GetTicketCount(); got != want {
		t.Fatalf("GetTicketCount = %d, want %d", got, want)
	}

	var drained atomic.Int32
	if err := q.ForcePurge(func(*FlushQueueTicket) error {
		drained.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("ForcePurge: %v", err)
	}
	if got := int(drained.Load()); got != want {
		t.Fatalf("drained = %d, want %d", got, want)
	}
	if q.HasTickets() {
		t.Fatal("HasTickets = true after full drain")
	}
}

func TestFlushQueue_AddSegmentAndPurgePreservesOrder(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()

	t1, _ := q.AddTicket(func() (*FlushQueueTicket, error) { return segTicket(t), nil })
	t2, _ := q.AddTicket(func() (*FlushQueueTicket, error) { return segTicket(t), nil })

	// Complete in reverse order; purge must still emit in FIFO.
	if err := q.AddSegment(t2, &FlushedSegment{}); err != nil {
		t.Fatalf("AddSegment t2: %v", err)
	}
	// Head is still pending, so purge drains nothing.
	var calls int
	if err := q.ForcePurge(func(*FlushQueueTicket) error { calls++; return nil }); err != nil {
		t.Fatalf("ForcePurge (head pending): %v", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0 while head pending", calls)
	}

	if err := q.AddSegment(t1, &FlushedSegment{}); err != nil {
		t.Fatalf("AddSegment t1: %v", err)
	}

	var order []*FlushQueueTicket
	if err := q.ForcePurge(func(tk *FlushQueueTicket) error { order = append(order, tk); return nil }); err != nil {
		t.Fatalf("ForcePurge: %v", err)
	}
	if len(order) != 2 || order[0] != t1 || order[1] != t2 {
		t.Fatalf("order = %v, want [t1, t2]", order)
	}
}

func TestFlushQueue_AddSegmentNilTicketRejected(t *testing.T) {
	q := NewDocumentsWriterFlushQueue()
	if err := q.AddSegment(nil, &FlushedSegment{}); err == nil {
		t.Fatal("AddSegment(nil, _) returned no error")
	}
	if err := q.MarkTicketFailed(nil); err == nil {
		t.Fatal("MarkTicketFailed(nil) returned no error")
	}
}
