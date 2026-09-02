// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// BufferedUpdatesStream
// Source: lucene/core/src/java/org/apache/lucene/index/BufferedUpdatesStream.java
// (Apache Lucene 10.4.0)
//
// Tracks the stream of [FrozenBufferedUpdates]. When a DWPT flushes its
// buffered deletes and updates are appended here and immediately resolved
// (to actual docIDs, per segment) on the indexing thread that triggered
// the flush. Merges, NRT-reader pulls, commit/close, or RAM/count
// thresholds sync against this stream to ensure all resolving packets
// complete.
//
// Each packet is assigned a generation; each flushed or merged segment
// is also assigned a generation, so the apply path can decide which
// packets still apply to any given segment.
//
// Sprint 55 (GOC-3363) — divergence inventory (option c, stub gaps):
//
//   - [SegmentCommitInfo.BufferedDeletesGen] is not yet present on the
//     Gocene type; [waitApplyForMerge] uses
//     [BufferedDeletesGenAccessor] (a narrow Go interface defined in this
//     file) so callers can adapt as soon as the accessor lands.
//   - [IndexWriter.tryApply] / [IndexWriter.forceApply] are not yet ported.
//     The waitApply loop talks to the [PacketApplier] interface defined
//     here; the concrete IndexWriter wiring is deferred to the apply
//     sprint that lands the FieldUpdatesBuffer / SegmentState plumbing
//     (cross-ref: [ErrFrozenBufferedUpdatesNotApplicable]).
//
// Everything else (push, finished, waitApplyAll, finished-segment
// frontier tracking, RAM accounting) is fully ported and exercised by
// the unit tests in [buffered_updates_stream_test.go].

package index

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PacketApplier is the narrow contract the stream uses to resolve a
// packet against the live index. Mirrors the [IndexWriter.tryApply] and
// [IndexWriter.forceApply] pair from the Lucene reference.
//
// TryApply attempts to resolve the packet without blocking; it returns
// true if the caller takes responsibility for the work (the stream then
// stops tracking the packet locally) and false if another resolver is
// already running, in which case the stream falls back to ForceApply.
//
// ForceApply blocks until the packet is fully resolved. Both methods
// observe ctx for cancellation; the Lucene reference is synchronous and
// throws IOException, the Go port surfaces both as a returned error.
//
// The concrete implementation is supplied by IndexWriter once the apply
// pipeline (FieldUpdatesBuffer / SegmentState) lands — see
// [ErrFrozenBufferedUpdatesNotApplicable] for the current divergence
// rationale.
type PacketApplier interface {
	TryApply(ctx context.Context, packet *FrozenBufferedUpdates) (bool, error)
	ForceApply(ctx context.Context, packet *FrozenBufferedUpdates) error
}

// BufferedDeletesGenAccessor is the narrow accessor used by
// [BufferedUpdatesStream.WaitApplyForMerge] to read the per-segment
// buffered-deletes generation. Lucene exposes this directly on
// SegmentCommitInfo; the Gocene type does not carry the field yet, so
// callers adapt their SegmentCommitInfo (or a thin wrapper) to this
// interface in the meantime.
type BufferedDeletesGenAccessor interface {
	BufferedDeletesGen() int64
}

// ApplyDeletesResult mirrors the record returned by the apply pipeline
// once it propagates back through the IndexWriter. AnyDeletes is true
// when at least one delete was resolved; AllDeleted lists segments that
// were fully deleted as a result of the apply.
//
// The current sprint does not yet populate this value (the apply
// pipeline is still stubbed); the type is exported to lock the public
// shape in place for the downstream sprint.
type ApplyDeletesResult struct {
	AnyDeletes bool
	AllDeleted []*SegmentCommitInfo
}

// ErrBufferedUpdatesStreamClosed is returned by mutating operations on
// a stream whose owning IndexWriter has been rolled back. The Lucene
// reference does not throw — it silently clears — but the Go port
// surfaces the situation so callers do not race against a cleared
// stream.
var ErrBufferedUpdatesStreamClosed = errors.New(
	"buffered updates stream: rolled back",
)

// BufferedUpdatesStream tracks pending [FrozenBufferedUpdates] packets,
// assigns delGens, and coordinates wait-for-apply across refresh,
// commit, and merge.
//
// The zero value is not usable; construct instances with
// [NewBufferedUpdatesStream].
//
// Concurrency: every observable method is safe to call from multiple
// goroutines. The stream uses a single Mutex (matching the
// synchronized blocks in the Java reference) on the hot push/finished
// paths and an atomic counter for RAM accounting. Latency-sensitive
// callers should still batch.
type BufferedUpdatesStream struct {
	// mu guards updates and nextGen. RamBytesUsed and Any are served
	// from bytesUsed without taking the lock.
	mu sync.Mutex

	// updates is the live set of pending packets. Ordering is irrelevant
	// — the apply path snapshots before iterating — and Lucene uses a
	// HashSet, so map[*FrozenBufferedUpdates]struct{} matches semantics.
	updates map[*FrozenBufferedUpdates]struct{}

	// nextGen starts at 1 so a SegmentInfo whose bufferedDelGen still
	// defaults to 0 is correctly treated as "no deletes applied yet".
	nextGen int64

	finishedSegments *finishedSegments
	infoStream       util.InfoStream

	// bytesUsed is the RAM accounting (sum of packet.BytesUsed across
	// updates). Kept as int64 atomic so RamBytesUsed and Any avoid the
	// mutex on the read path.
	bytesUsed atomic.Int64
}

// NewBufferedUpdatesStream constructs an empty stream. Passing a nil
// infoStream installs [util.NoOpInfoStream] so the diagnostic calls on
// the hot path stay branch-free.
func NewBufferedUpdatesStream(infoStream util.InfoStream) *BufferedUpdatesStream {
	if infoStream == nil {
		infoStream = util.NoOpInfoStream
	}
	return &BufferedUpdatesStream{
		updates:          make(map[*FrozenBufferedUpdates]struct{}),
		nextGen:          1,
		finishedSegments: newFinishedSegments(infoStream),
		infoStream:       infoStream,
	}
}

// Push appends a packet to the stream, assigning its delGen atomically
// with the insertion so concurrent pushers cannot reorder the stream.
//
// Returns the assigned delGen, or an error if the packet is empty
// (mirrors the Lucene assertion {@code assert packet.any()}) or its
// delGen was already set.
func (s *BufferedUpdatesStream) Push(packet *FrozenBufferedUpdates) (int64, error) {
	if packet == nil {
		return 0, errors.New("buffered updates stream: nil packet")
	}
	if !packet.Any() {
		return 0, errors.New("buffered updates stream: packet has no work")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	gen := s.nextGen
	s.nextGen++

	if err := packet.SetDelGen(gen); err != nil {
		// Roll the generation back so the next caller does not see a hole.
		s.nextGen--
		return 0, fmt.Errorf("buffered updates stream: %w", err)
	}

	s.updates[packet] = struct{}{}
	s.bytesUsed.Add(int64(packet.BytesUsed()))

	if s.infoStream.IsEnabled("BD") {
		s.infoStream.Message("BD", fmt.Sprintf(
			"push new packet (%s), packetCount=%d, bytesUsed=%.3f MB",
			packet, len(s.updates),
			float64(s.bytesUsed.Load())/1024.0/1024.0,
		))
	}

	return gen, nil
}

// PendingUpdatesCount returns the number of packets currently being
// tracked.
func (s *BufferedUpdatesStream) PendingUpdatesCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.updates)
}

// Clear discards every packet and resets the generation counter. Only
// used by [IndexWriter.Rollback].
func (s *BufferedUpdatesStream) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(s.updates)
	s.nextGen = 1
	s.finishedSegments.clear()
	s.bytesUsed.Store(0)
}

// Any reports whether the stream is currently holding any RAM.
func (s *BufferedUpdatesStream) Any() bool {
	return s.bytesUsed.Load() != 0
}

// RamBytesUsed reports the aggregated RAM accounting across pending
// packets. Implements [util.Accountable].
func (s *BufferedUpdatesStream) RamBytesUsed() int64 {
	return s.bytesUsed.Load()
}

// GetChildResources reports no child accountables; the stream exposes
// its packets only opaquely. Implements [util.Accountable].
func (s *BufferedUpdatesStream) GetChildResources() []util.Accountable {
	return nil
}

// Finished marks the packet as fully resolved. Called by indexing
// threads once they finish applying every delete/update in the packet.
//
// Returns an error if the packet was never pushed or has already been
// marked finished, mirroring the Lucene assertion
// {@code assert packet.applied.getCount() == 1}.
func (s *BufferedUpdatesStream) Finished(packet *FrozenBufferedUpdates) error {
	if packet == nil {
		return errors.New("buffered updates stream: nil packet")
	}
	if packet.DelGen() < 0 {
		return errors.New("buffered updates stream: packet was never pushed")
	}

	s.mu.Lock()
	if _, ok := s.updates[packet]; !ok {
		s.mu.Unlock()
		return fmt.Errorf(
			"buffered updates stream: packet %s already finished", packet,
		)
	}
	delete(s.updates, packet)
	s.bytesUsed.Add(-int64(packet.BytesUsed()))
	s.mu.Unlock()

	// fireApplied is idempotent; FinishedSegment is serialized inside
	// finishedSegments.
	packet.fireApplied()
	s.finishedSegments.finishedSegment(packet.DelGen())
	return nil
}

// FinishedSegment records that the indexing thread is done resolving
// the packet identified by delGen. Used internally by [Finished] and
// exposed for callers that resolve packets out-of-band.
func (s *BufferedUpdatesStream) FinishedSegment(delGen int64) {
	s.finishedSegments.finishedSegment(delGen)
}

// StillRunning reports whether the packet at delGen is still being
// resolved.
func (s *BufferedUpdatesStream) StillRunning(delGen int64) bool {
	return s.finishedSegments.stillRunning(delGen)
}

// CompletedDelGen returns the largest delGen for which every prior
// packet, inclusive, is known to be finished.
func (s *BufferedUpdatesStream) CompletedDelGen() int64 {
	return s.finishedSegments.completedDelGen()
}

// NextGen reserves and returns the next delGen without pushing a
// packet. Mirrors Lucene's {@code getNextGen}; used by paths that need
// to claim a gen up-front (private-segment publish).
func (s *BufferedUpdatesStream) NextGen() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	gen := s.nextGen
	s.nextGen++
	return gen
}

// WaitApplyAll blocks until every currently-pending packet has been
// resolved. Called from refresh and commit.
//
// The supplied PacketApplier is the bridge to the IndexWriter (or a
// stand-in in tests). Errors from the applier are propagated verbatim.
func (s *BufferedUpdatesStream) WaitApplyAll(
	ctx context.Context, applier PacketApplier,
) error {
	if applier == nil {
		return errors.New("buffered updates stream: nil applier")
	}

	s.mu.Lock()
	waitFor := make([]*FrozenBufferedUpdates, 0, len(s.updates))
	for p := range s.updates {
		waitFor = append(waitFor, p)
	}
	s.mu.Unlock()

	return s.waitApply(ctx, waitFor, applier)
}

// WaitApplyForMerge blocks until every packet whose delGen is less than
// or equal to the maximum buffered-deletes-gen across mergeInfos has
// been resolved. Called by the merge path before committing the merge.
//
// Each entry in mergeInfos must implement [BufferedDeletesGenAccessor].
// The current Gocene SegmentCommitInfo does not yet expose this
// accessor — callers wrap their SegmentCommitInfo in a thin adapter
// until [GOC-future] lands the field.
func (s *BufferedUpdatesStream) WaitApplyForMerge(
	ctx context.Context,
	mergeInfos []BufferedDeletesGenAccessor,
	applier PacketApplier,
) error {
	if applier == nil {
		return errors.New("buffered updates stream: nil applier")
	}

	maxDelGen := int64(-1 << 63) // math.MinInt64 without the dependency
	for _, info := range mergeInfos {
		if info == nil {
			return errors.New("buffered updates stream: nil merge info")
		}
		if g := info.BufferedDeletesGen(); g > maxDelGen {
			maxDelGen = g
		}
	}

	s.mu.Lock()
	waitFor := make([]*FrozenBufferedUpdates, 0)
	for p := range s.updates {
		if p.DelGen() <= maxDelGen {
			waitFor = append(waitFor, p)
		}
	}
	s.mu.Unlock()

	if s.infoStream.IsEnabled("BD") {
		s.infoStream.Message("BD", fmt.Sprintf(
			"waitApplyForMerge: %d packets, %d merging segments",
			len(waitFor), len(mergeInfos),
		))
	}

	return s.waitApply(ctx, waitFor, applier)
}

// waitApply drives the snapshot taken by [WaitApplyAll] or
// [WaitApplyForMerge] through the supplied applier.
//
// Sort order: snapshots are walked in delGen-ascending order. Lucene
// iterates a HashSet (unordered); the Go port sorts so concurrent
// readers see deterministic per-test traces — the order is irrelevant
// for the apply contract because every packet is independently
// resolvable.
func (s *BufferedUpdatesStream) waitApply(
	ctx context.Context,
	waitFor []*FrozenBufferedUpdates,
	applier PacketApplier,
) error {
	startNS := time.Now()
	packetCount := len(waitFor)

	if packetCount == 0 {
		if s.infoStream.IsEnabled("BD") {
			s.infoStream.Message("BD", "waitApply: no deletes to apply")
		}
		return nil
	}

	sort.Slice(waitFor, func(i, j int) bool {
		return waitFor[i].DelGen() < waitFor[j].DelGen()
	})

	if s.infoStream.IsEnabled("BD") {
		s.infoStream.Message("BD", fmt.Sprintf(
			"waitApply: %d packets: %v", packetCount, waitFor,
		))
	}

	var pending []*FrozenBufferedUpdates
	var totalDelCount int64

	for _, packet := range waitFor {
		if err := ctx.Err(); err != nil {
			return err
		}
		ok, err := applier.TryApply(ctx, packet)
		if err != nil {
			return err
		}
		if !ok {
			// Another resolver took ownership; block on it below.
			pending = append(pending, packet)
		}
		totalDelCount += packet.TotalDelCount()
	}
	for _, packet := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := applier.ForceApply(ctx, packet); err != nil {
			return err
		}
	}

	if s.infoStream.IsEnabled("BD") {
		s.infoStream.Message("BD", fmt.Sprintf(
			"waitApply: done %d packets; totalDelCount=%d; totBytesUsed=%d; took %.2f msec",
			packetCount, totalDelCount, s.bytesUsed.Load(),
			float64(time.Since(startNS))/float64(time.Millisecond),
		))
	}
	return nil
}

// finishedSegments tracks the contiguous range of packets that have
// finished resolving. The structure is needed because packets are
// resolved concurrently and only the contiguous completed prefix can
// be safely written to disk.
//
// Lucene uses a LongHashSet for the gap set; the port uses a native
// map[int64]struct{} because the set is small (bounded by in-flight
// concurrency) and there is no Long-boxing overhead to amortise.
type finishedSegments struct {
	mu sync.Mutex

	// completed is the largest delGen, inclusive, for which every
	// prior packet has finished applying.
	completed int64

	// gaps tracks the "holes" in the current applying frontier;
	// once a hole is filled the frontier advances past it.
	gaps map[int64]struct{}

	infoStream util.InfoStream
}

func newFinishedSegments(infoStream util.InfoStream) *finishedSegments {
	return &finishedSegments{
		gaps:       make(map[int64]struct{}),
		infoStream: infoStream,
	}
}

func (f *finishedSegments) clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	clear(f.gaps)
	f.completed = 0
}

func (f *finishedSegments) stillRunning(delGen int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if delGen <= f.completed {
		return false
	}
	_, present := f.gaps[delGen]
	return !present
}

func (f *finishedSegments) completedDelGen() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.completed
}

func (f *finishedSegments) finishedSegment(delGen int64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.gaps[delGen] = struct{}{}
	for {
		next := f.completed + 1
		if _, ok := f.gaps[next]; !ok {
			break
		}
		delete(f.gaps, next)
		f.completed = next
	}

	if f.infoStream.IsEnabled("BD") {
		f.infoStream.Message("BD", fmt.Sprintf(
			"finished packet delGen=%d now completedDelGen=%d",
			delGen, f.completed,
		))
	}
}
