// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// newTestPacket builds a small frozen packet carrying a single term
// delete so it satisfies [FrozenBufferedUpdates.Any] and can therefore
// be pushed onto a stream.
func newTestPacket(t *testing.T, seg, field, term string) *FrozenBufferedUpdates {
	t.Helper()
	bu := NewBufferedUpdates(seg)
	bu.AddTerm(NewTerm(field, term), 1)
	p, err := NewFrozenBufferedUpdates(util.NoOpInfoStream, bu, nil)
	if err != nil {
		t.Fatalf("NewFrozenBufferedUpdates: %v", err)
	}
	return p
}

// recordingApplier captures every TryApply / ForceApply invocation so
// tests can assert the dispatch order and force partial-success paths.
type recordingApplier struct {
	mu sync.Mutex

	// tryReturn controls the return value of TryApply per packet. A
	// missing entry defaults to (true, nil).
	tryReturn map[*FrozenBufferedUpdates]bool
	// tryErr forces TryApply to return an error for the given packet.
	tryErr map[*FrozenBufferedUpdates]error
	// forceErr forces ForceApply to return an error for the given packet.
	forceErr map[*FrozenBufferedUpdates]error

	tryCalls   []*FrozenBufferedUpdates
	forceCalls []*FrozenBufferedUpdates
}

func newRecordingApplier() *recordingApplier {
	return &recordingApplier{
		tryReturn: make(map[*FrozenBufferedUpdates]bool),
		tryErr:    make(map[*FrozenBufferedUpdates]error),
		forceErr:  make(map[*FrozenBufferedUpdates]error),
	}
}

func (r *recordingApplier) TryApply(
	_ context.Context, p *FrozenBufferedUpdates,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tryCalls = append(r.tryCalls, p)
	if err, ok := r.tryErr[p]; ok {
		return false, err
	}
	if v, ok := r.tryReturn[p]; ok {
		return v, nil
	}
	return true, nil
}

func (r *recordingApplier) ForceApply(
	_ context.Context, p *FrozenBufferedUpdates,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.forceCalls = append(r.forceCalls, p)
	if err, ok := r.forceErr[p]; ok {
		return err
	}
	return nil
}

// fakeMergeInfo adapts a delGen to BufferedDeletesGenAccessor.
type fakeMergeInfo struct{ gen int64 }

func (f fakeMergeInfo) BufferedDeletesGen() int64 { return f.gen }

func TestBufferedUpdatesStream_PushAssignsGen(t *testing.T) {
	s := NewBufferedUpdatesStream(util.NoOpInfoStream)

	p1 := newTestPacket(t, "seg-1", "body", "alpha")
	p2 := newTestPacket(t, "seg-2", "body", "beta")

	g1, err := s.Push(p1)
	if err != nil {
		t.Fatalf("Push p1: %v", err)
	}
	g2, err := s.Push(p2)
	if err != nil {
		t.Fatalf("Push p2: %v", err)
	}

	if g1 != 1 || g2 != 2 {
		t.Fatalf("delGens=%d,%d want 1,2", g1, g2)
	}
	if p1.DelGen() != 1 || p2.DelGen() != 2 {
		t.Fatalf("packet delGens=%d,%d want 1,2", p1.DelGen(), p2.DelGen())
	}
	if got := s.PendingUpdatesCount(); got != 2 {
		t.Fatalf("PendingUpdatesCount=%d want 2", got)
	}
	if !s.Any() || s.RamBytesUsed() == 0 {
		t.Fatalf("expected non-empty stream after push")
	}
}

func TestBufferedUpdatesStream_PushRejectsEmpty(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	bu := NewBufferedUpdates("seg")
	p, err := NewFrozenBufferedUpdates(util.NoOpInfoStream, bu, nil)
	if err != nil {
		t.Fatalf("NewFrozenBufferedUpdates: %v", err)
	}
	if _, err := s.Push(p); err == nil {
		t.Fatalf("expected error pushing empty packet")
	}
}

func TestBufferedUpdatesStream_PushNilRejected(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	if _, err := s.Push(nil); err == nil {
		t.Fatalf("expected error pushing nil packet")
	}
}

func TestBufferedUpdatesStream_FinishedRemovesAndAdvances(t *testing.T) {
	s := NewBufferedUpdatesStream(util.NoOpInfoStream)
	p1 := newTestPacket(t, "seg-1", "body", "alpha")
	p2 := newTestPacket(t, "seg-2", "body", "beta")

	if _, err := s.Push(p1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Push(p2); err != nil {
		t.Fatal(err)
	}
	bytesBefore := s.RamBytesUsed()

	if err := s.Finished(p1); err != nil {
		t.Fatalf("Finished p1: %v", err)
	}
	if got := s.PendingUpdatesCount(); got != 1 {
		t.Fatalf("PendingUpdatesCount after p1=%d want 1", got)
	}
	if s.CompletedDelGen() != 1 {
		t.Fatalf("CompletedDelGen=%d want 1", s.CompletedDelGen())
	}
	if s.RamBytesUsed() >= bytesBefore {
		t.Fatalf("RAM did not shrink: before=%d after=%d", bytesBefore, s.RamBytesUsed())
	}
	select {
	case <-p1.Applied():
	default:
		t.Fatalf("p1 applied latch not closed")
	}

	if err := s.Finished(p2); err != nil {
		t.Fatalf("Finished p2: %v", err)
	}
	if s.Any() {
		t.Fatalf("stream still reports work after both finished")
	}
}

func TestBufferedUpdatesStream_FinishedOutOfOrderHoldsCompleted(t *testing.T) {
	s := NewBufferedUpdatesStream(util.NoOpInfoStream)
	p1 := newTestPacket(t, "seg-1", "body", "a")
	p2 := newTestPacket(t, "seg-2", "body", "b")
	p3 := newTestPacket(t, "seg-3", "body", "c")
	for _, p := range []*FrozenBufferedUpdates{p1, p2, p3} {
		if _, err := s.Push(p); err != nil {
			t.Fatal(err)
		}
	}

	// Finish p2 first: frontier must stay at 0 (p1 still outstanding).
	if err := s.Finished(p2); err != nil {
		t.Fatal(err)
	}
	if s.CompletedDelGen() != 0 {
		t.Fatalf("CompletedDelGen=%d want 0 (p1 still pending)", s.CompletedDelGen())
	}
	if !s.StillRunning(p1.DelGen()) {
		t.Fatalf("p1 should still be running")
	}
	if s.StillRunning(p2.DelGen()) {
		t.Fatalf("p2 should not be running")
	}

	// Finish p1: frontier jumps to 2 (p2 was already in the gap set).
	if err := s.Finished(p1); err != nil {
		t.Fatal(err)
	}
	if s.CompletedDelGen() != 2 {
		t.Fatalf("CompletedDelGen=%d want 2 after closing the gap", s.CompletedDelGen())
	}

	// Finish p3: frontier reaches 3.
	if err := s.Finished(p3); err != nil {
		t.Fatal(err)
	}
	if s.CompletedDelGen() != 3 {
		t.Fatalf("CompletedDelGen=%d want 3", s.CompletedDelGen())
	}
}

func TestBufferedUpdatesStream_FinishedTwiceRejected(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	p := newTestPacket(t, "seg-1", "body", "a")
	if _, err := s.Push(p); err != nil {
		t.Fatal(err)
	}
	if err := s.Finished(p); err != nil {
		t.Fatal(err)
	}
	if err := s.Finished(p); err == nil {
		t.Fatalf("expected error on second Finished")
	}
}

func TestBufferedUpdatesStream_ClearResetsState(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	p := newTestPacket(t, "seg-1", "body", "a")
	if _, err := s.Push(p); err != nil {
		t.Fatal(err)
	}
	s.Clear()

	if s.PendingUpdatesCount() != 0 || s.RamBytesUsed() != 0 || s.Any() {
		t.Fatalf("stream not cleared: count=%d ram=%d any=%v",
			s.PendingUpdatesCount(), s.RamBytesUsed(), s.Any())
	}
	if s.CompletedDelGen() != 0 {
		t.Fatalf("CompletedDelGen=%d want 0", s.CompletedDelGen())
	}

	// nextGen must restart at 1 (fresh push gets gen 1).
	p2 := newTestPacket(t, "seg-2", "body", "b")
	g, err := s.Push(p2)
	if err != nil {
		t.Fatal(err)
	}
	if g != 1 {
		t.Fatalf("post-Clear gen=%d want 1", g)
	}
}

func TestBufferedUpdatesStream_NextGenAdvances(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	if g := s.NextGen(); g != 1 {
		t.Fatalf("NextGen=%d want 1", g)
	}
	if g := s.NextGen(); g != 2 {
		t.Fatalf("NextGen=%d want 2", g)
	}
	// A subsequent Push must continue from 3.
	p := newTestPacket(t, "seg-1", "body", "a")
	g, err := s.Push(p)
	if err != nil {
		t.Fatal(err)
	}
	if g != 3 {
		t.Fatalf("post-NextGen Push=%d want 3", g)
	}
}

func TestBufferedUpdatesStream_WaitApplyAllNoPackets(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	r := newRecordingApplier()
	if err := s.WaitApplyAll(context.Background(), r); err != nil {
		t.Fatalf("WaitApplyAll: %v", err)
	}
	if len(r.tryCalls)+len(r.forceCalls) != 0 {
		t.Fatalf("applier should not be invoked when no packets are pending")
	}
}

func TestBufferedUpdatesStream_WaitApplyAllInvokesApplier(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	p1 := newTestPacket(t, "seg-1", "body", "a")
	p2 := newTestPacket(t, "seg-2", "body", "b")
	for _, p := range []*FrozenBufferedUpdates{p1, p2} {
		if _, err := s.Push(p); err != nil {
			t.Fatal(err)
		}
	}

	r := newRecordingApplier()
	if err := s.WaitApplyAll(context.Background(), r); err != nil {
		t.Fatalf("WaitApplyAll: %v", err)
	}
	if len(r.tryCalls) != 2 {
		t.Fatalf("TryApply calls=%d want 2", len(r.tryCalls))
	}
	if len(r.forceCalls) != 0 {
		t.Fatalf("ForceApply calls=%d want 0", len(r.forceCalls))
	}
}

func TestBufferedUpdatesStream_WaitApplyFallsBackToForce(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	p1 := newTestPacket(t, "seg-1", "body", "a")
	p2 := newTestPacket(t, "seg-2", "body", "b")
	for _, p := range []*FrozenBufferedUpdates{p1, p2} {
		if _, err := s.Push(p); err != nil {
			t.Fatal(err)
		}
	}

	r := newRecordingApplier()
	// p1 is already being resolved elsewhere — stream must fall back to
	// ForceApply for it but TryApply alone is enough for p2.
	r.tryReturn[p1] = false

	if err := s.WaitApplyAll(context.Background(), r); err != nil {
		t.Fatalf("WaitApplyAll: %v", err)
	}
	if len(r.forceCalls) != 1 || r.forceCalls[0] != p1 {
		t.Fatalf("expected ForceApply on p1, got %v", r.forceCalls)
	}
}

func TestBufferedUpdatesStream_WaitApplyTryError(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	p := newTestPacket(t, "seg-1", "body", "a")
	if _, err := s.Push(p); err != nil {
		t.Fatal(err)
	}
	r := newRecordingApplier()
	sentinel := errors.New("try boom")
	r.tryErr[p] = sentinel

	if err := s.WaitApplyAll(context.Background(), r); !errors.Is(err, sentinel) {
		t.Fatalf("WaitApplyAll err=%v want %v", err, sentinel)
	}
}

func TestBufferedUpdatesStream_WaitApplyContextCanceled(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	p := newTestPacket(t, "seg-1", "body", "a")
	if _, err := s.Push(p); err != nil {
		t.Fatal(err)
	}
	r := newRecordingApplier()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.WaitApplyAll(ctx, r)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitApplyAll err=%v want Canceled", err)
	}
}

func TestBufferedUpdatesStream_WaitApplyAllNilApplier(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	if err := s.WaitApplyAll(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil applier")
	}
}

func TestBufferedUpdatesStream_WaitApplyForMergeFiltersByGen(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	p1 := newTestPacket(t, "seg-1", "body", "a") // delGen=1
	p2 := newTestPacket(t, "seg-2", "body", "b") // delGen=2
	p3 := newTestPacket(t, "seg-3", "body", "c") // delGen=3
	for _, p := range []*FrozenBufferedUpdates{p1, p2, p3} {
		if _, err := s.Push(p); err != nil {
			t.Fatal(err)
		}
	}

	r := newRecordingApplier()
	mergeInfos := []BufferedDeletesGenAccessor{
		fakeMergeInfo{gen: 1},
		fakeMergeInfo{gen: 2},
	}
	if err := s.WaitApplyForMerge(context.Background(), mergeInfos, r); err != nil {
		t.Fatalf("WaitApplyForMerge: %v", err)
	}
	if len(r.tryCalls) != 2 {
		t.Fatalf("TryApply calls=%d want 2 (only delGen<=2)", len(r.tryCalls))
	}
	// Order is guaranteed: ascending delGen.
	if r.tryCalls[0] != p1 || r.tryCalls[1] != p2 {
		t.Fatalf("apply order=%v want [p1 p2]", r.tryCalls)
	}
}

func TestBufferedUpdatesStream_WaitApplyForMergeNilInfo(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	r := newRecordingApplier()
	err := s.WaitApplyForMerge(
		context.Background(),
		[]BufferedDeletesGenAccessor{nil},
		r,
	)
	if err == nil {
		t.Fatalf("expected error for nil merge info")
	}
}

func TestBufferedUpdatesStream_FinishedNonExistentPacketRejected(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	p := newTestPacket(t, "seg-1", "body", "a")
	// Never push p; SetDelGen to simulate a packet that escaped.
	if err := p.SetDelGen(42); err != nil {
		t.Fatal(err)
	}
	if err := s.Finished(p); err == nil {
		t.Fatalf("expected error finishing unknown packet")
	}
}

func TestBufferedUpdatesStream_FinishedNeverPushedRejected(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	p := newTestPacket(t, "seg-1", "body", "a")
	if err := s.Finished(p); err == nil {
		t.Fatalf("expected error finishing un-pushed packet")
	}
}

func TestBufferedUpdatesStream_ConcurrentPushAssignsUniqueGens(t *testing.T) {
	s := NewBufferedUpdatesStream(nil)
	const n = 64
	gens := make([]int64, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			p := newTestPacket(t, "seg", "body", string(rune('a'+i%26)))
			g, err := s.Push(p)
			if err != nil {
				t.Errorf("Push %d: %v", i, err)
				return
			}
			gens[i] = g
		}()
	}
	wg.Wait()

	seen := make(map[int64]struct{}, n)
	for _, g := range gens {
		if _, dup := seen[g]; dup {
			t.Fatalf("duplicate gen %d assigned", g)
		}
		seen[g] = struct{}{}
	}
	if len(seen) != n {
		t.Fatalf("unique gens=%d want %d", len(seen), n)
	}
	if got := s.PendingUpdatesCount(); got != n {
		t.Fatalf("PendingUpdatesCount=%d want %d", got, n)
	}
}

func TestBufferedUpdatesStream_AccountableSurface(t *testing.T) {
	var _ util.Accountable = (*BufferedUpdatesStream)(nil)
	s := NewBufferedUpdatesStream(nil)
	if got := s.GetChildResources(); got != nil {
		t.Fatalf("GetChildResources=%v want nil", got)
	}
}
