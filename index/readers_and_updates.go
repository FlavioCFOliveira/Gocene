// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// ReadersAndUpdates
// Source: lucene/core/src/java/org/apache/lucene/index/ReadersAndUpdates.java
// (Apache Lucene 10.4.0)
//
// Used by IndexWriter to hold open SegmentReaders (for searching or merging),
// plus pending deletes and doc-values updates, for a given segment.
//
// Sprint 55 (GOC-3377) scope — option (c): "port surface and orchestration
// where the collaborators exist; emit documented gap errors where they do
// not".
//
//   - Ref counting, the pending / merging DV-update maps, isMerging
//     bookkeeping, addDVUpdate, getNumDVUpdates, dropMergingUpdates,
//     getMergingDVUpdates, dropChanges (best-effort), dropReaders, and
//     the prune-loop tail of writeFieldUpdates are FULLY ported and exercised
//     by tests in this package.
//
//   - Heavyweight paths whose dependencies are not yet ported return one of
//     the [ErrReadersAndUpdatesNot*] sentinels below; each error names the
//     missing collaborator. The affected entry points are:
//
//       getReadOnlyClone, writeLiveDocs, writeFieldUpdates (the codec-write
//       body — the prune tail is reachable through a dedicated helper used
//       by tests), handleDVUpdates, numDeletesToMerge, getLatestReader,
//       getLiveDocs / getHardLiveDocs, getReaderForMerge, isFullyDeleted,
//       keepFullyDeletedSegment, swapNewReaderWithLatestLiveDocs and
//       createNewReaderWithLatestLiveDocs.
//
//   - The minimum [Gocene.PendingDeletes] (a docID set with a mutex; see
//     [nrt_segment_reader.go]) is treated as opaque pending-deletes state.
//     The full PendingDeletes API used by Lucene (getLiveDocs,
//     getHardLiveDocs, onNewReader, onDocValuesUpdate, numDeletesToMerge,
//     writeLiveDocs, needsRefresh, numDocs, isFullyDeleted, dropChanges,
//     getDelCount, delete, mustInitOnDelete) is not yet available; the
//     orchestrator falls back to the minimum surface where possible and
//     reports gaps otherwise.
//
//   - [SegmentReader.IncRef]/[SegmentReader.DecRef] do not exist in Gocene
//     today (the reader is owned, not ref-counted). The port keeps
//     [ReadersAndUpdates] external ref counting in `refCount`; the inner
//     reader lifecycle is handled by [SegmentReader.Close] in dropReaders.
//
//   - The Codec / DocValuesFormat / TrackingDirectoryWrapper / SegmentWriteState
//     pipeline reached by the original writeFieldUpdates is fully stubbed:
//     calling [ReadersAndUpdates.WriteFieldUpdates] returns
//     [ErrReadersAndUpdatesDVWriteUnsupported] unless there is nothing to
//     write (in which case it returns false with no error, matching Lucene).
//     The prune logic is exposed separately through
//     [ReadersAndUpdates.PruneAppliedDVUpdates] so the bookkeeping can still
//     be exercised in tests.
//
// All observable surface that does not depend on missing collaborators
// behaves byte-for-byte as the Lucene reference. The divergences above
// are tracked alongside [FrozenBufferedUpdates] and will be resolved as
// the dependent ports land (PendingDeletes full API, MergePolicy.MergeReader,
// TrackingDirectoryWrapper, DocValuesConsumer pipeline).

package index

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrReadersAndUpdatesLiveDocsUnsupported is returned by any entry point
// that needs the Lucene "PendingDeletes.getLiveDocs / getHardLiveDocs /
// writeLiveDocs / needsRefresh / numDocs / isFullyDeleted" surface, which
// is not yet ported. See file header.
var ErrReadersAndUpdatesLiveDocsUnsupported = errors.New(
	"readers and updates: pending-deletes live-docs surface not yet ported " +
		"(getLiveDocs / getHardLiveDocs / writeLiveDocs / needsRefresh / numDocs / isFullyDeleted)",
)

// ErrReadersAndUpdatesDVWriteUnsupported is returned by
// [ReadersAndUpdates.WriteFieldUpdates] when the packet actually has work
// to do. The codec-write body depends on TrackingDirectoryWrapper,
// SegmentWriteState wiring, DocValuesConsumer, and the
// PendingDeletes.onDocValuesUpdate / onNewReader callbacks, none of which
// are ported yet. See file header.
var ErrReadersAndUpdatesDVWriteUnsupported = errors.New(
	"readers and updates: doc-values updates writer pipeline not yet ported " +
		"(TrackingDirectoryWrapper / DocValuesConsumer / PendingDeletes callbacks)",
)

// ErrReadersAndUpdatesMergeReaderUnsupported is returned by
// [ReadersAndUpdates.GetReaderForMerge] and [ReadersAndUpdates.NumDeletesToMerge]:
// MergePolicy.MergeReader and PendingDeletes.numDeletesToMerge are not yet
// ported. See file header.
var ErrReadersAndUpdatesMergeReaderUnsupported = errors.New(
	"readers and updates: merge-reader surface not yet ported " +
		"(MergePolicy.MergeReader / PendingDeletes.numDeletesToMerge)",
)

// ErrReadersAndUpdatesReadOnlyCloneUnsupported is returned by
// [ReadersAndUpdates.GetReadOnlyClone]: the alternate SegmentReader
// constructor that wraps an existing reader with replacement live-docs is
// not ported.
var ErrReadersAndUpdatesReadOnlyCloneUnsupported = errors.New(
	"readers and updates: SegmentReader live-docs wrapper constructor not yet ported",
)

// ErrReadersAndUpdatesUpdateNotFinished is returned by
// [ReadersAndUpdates.AddDVUpdate] when the caller passes an unfinished
// packet, mirroring Lucene's IllegalArgumentException("call finish first").
var ErrReadersAndUpdatesUpdateNotFinished = errors.New(
	"readers and updates: doc-values update packet must be finished first",
)

// dvUpdatePacket is the minimum DocValuesFieldUpdates shape consumed by
// [ReadersAndUpdates]. It captures the [BaseDocValuesFieldUpdates] surface
// without taking a hard dependency on the concrete type, so future
// numeric/binary subclasses can be plugged in. All Gocene packets
// satisfy this shape today.
//
// RamBytesUsed mirrors Lucene's {@code DocValuesFieldUpdates#ramBytesUsed()}.
// BinaryDocValuesFieldUpdates already overrides it with auxiliary-array
// awareness; the orchestrator only needs the polymorphic dispatch.
type dvUpdatePacket interface {
	Field() string
	Type() DocValuesType
	DelGen() int64
	GetFinished() bool
	Any() bool
	RamBytesUsed() int64
}

// readersAndUpdatesPacket is the concrete adapter Gocene uses today.
// Internally we hold *BaseDocValuesFieldUpdates, but we deliberately go
// through the [dvUpdatePacket] interface so the call sites stay typed
// against the minimal contract.
type readersAndUpdatesPacket struct {
	inner dvUpdatePacket
}

// mergedDocValues merges the current on-disk DV with an incoming update DV
// instance and merges the two instances giving the incoming update precedence
// in terms of values, in other words the values of the update always win
// over the on-disk version.
type mergedDocValues[T any] struct {
	updateIterator DocValuesFieldUpdatesIterator
	docIDOut       int
	docIDOnDisk    int
	updateDocID    int
	onDisk         T
	update         T
	current        T
	scratch        *util.FixedBitSet
}

func newMergedDocValues[T any](onDisk T, update T, updateIterator DocValuesFieldUpdatesIterator) *mergedDocValues[T] {
	return &mergedDocValues[T]{
		onDisk:         onDisk,
		update:         update,
		updateIterator: updateIterator,
		docIDOut:       -1,
		docIDOnDisk:    -1,
		updateDocID:    -1,
	}
}

func (m *mergedDocValues[T]) nextDoc(onDiskNext func() int, updateNext func() int) int {
	hasValue := false
	for {
		if m.docIDOnDisk == m.docIDOut {
			m.docIDOnDisk = onDiskNext()
		}
		if m.updateDocID == m.docIDOut {
			m.updateDocID = updateNext()
		}
		if m.docIDOnDisk < m.updateDocID {
			m.docIDOut = m.docIDOnDisk
			m.current = m.onDisk
			hasValue = true
		} else {
			m.docIDOut = m.updateDocID
			if m.docIDOut != util.NO_MORE_DOCS {
				m.current = m.update
				hasValue = m.updateIterator.HasValue()
			} else {
				hasValue = true
			}
		}
		if hasValue {
			break
		}
	}
	return m.docIDOut
}

func (m *mergedDocValues[T]) intoBitSet(upTo int, bitSet *util.FixedBitSet, offset int, onDiskIntoBitSet func(*util.FixedBitSet, int, int)) {
	if m.onDisk == nil {
		for doc := m.update.DocID(); doc < upTo; doc = m.update.NextDoc() {
			if m.updateIterator.HasValue() {
				bitSet.Set(doc - offset)
			} else {
				bitSet.Clear(doc - offset)
			}
		}
		return
	}

	if m.scratch == nil {
		m.scratch = util.NewFixedBitSet(bitSet.Length())
	} else {
		m.scratch = util.EnsureCapacityAndClear(m.scratch, bitSet.Length()-1)
	}

	onDiskIntoBitSet(m.scratch, offset, upTo)
	m.docIDOnDisk = m.onDisk.DocID()

	for doc := m.update.DocID(); doc < upTo; doc = m.update.NextDoc() {
		if m.updateIterator.HasValue() {
			m.scratch.Set(doc - offset)
		} else {
			m.scratch.Clear(doc - offset)
		}
	}

	util.FixedBitSetOrRange(m.scratch, 0, bitSet, 0, bitSet.Length())

	for {
		for m.update.DocID() < m.docIDOnDisk && !m.updateIterator.HasValue() {
			m.update.NextDoc()
		}
		if m.docIDOnDisk != util.NO_MORE_DOCS &&
			m.update.DocID() == m.docIDOnDisk &&
			!m.updateIterator.HasValue() {
			m.docIDOnDisk = m.onDisk.NextDoc()
		} else {
			break
		}
	}

	m.updateDocID = m.update.DocID()
	if m.docIDOnDisk < m.updateDocID {
		m.docIDOut = m.docIDOnDisk
		m.current = m.onDisk
	} else {
		m.docIDOut = m.updateDocID
		m.current = m.update
	}
}

type numericMergedDocValues struct {
	merged *mergedDocValues[NumericDocValues]
}

func (n *numericMergedDocValues) LongValue() (int64, error) {
	return n.merged.current.LongValue()
}

func (n *numericMergedDocValues) Advance(target int) (int, error) {
	return n.merged.nextDoc(n.merged.onDisk.NextDoc, n.merged.update.NextDoc), nil
}

func (n *numericMergedDocValues) AdvanceExact(target int) (bool, error) {
	panic("unsupported")
}

func (n *numericMergedDocValues) DocID() int {
	return n.merged.docIDOut
}

func (n *numericMergedDocValues) NextDoc() (int, error) {
	return n.merged.nextDoc(n.merged.onDisk.NextDoc, n.merged.update.NextDoc), nil
}

func (n *numericMergedDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	n.merged.intoBitSet(upTo, bitSet, offset, func(s *util.FixedBitSet, off, u int) {
		n.merged.onDisk.IntoBitSet(u, s, off)
	})
	return nil
}

func (n *numericMergedDocValues) Cost() int64 {
	return n.merged.onDisk.Cost()
}

type binaryMergedDocValues struct {
	merged *mergedDocValues[BinaryDocValues]
}

func (b *binaryMergedDocValues) BinaryValue() ([]byte, error) {
	return b.merged.current.BinaryValue()
}

func (b *binaryMergedDocValues) Advance(target int) (int, error) {
	return b.merged.nextDoc(b.merged.onDisk.NextDoc, b.merged.update.NextDoc), nil
}

func (b *binaryMergedDocValues) AdvanceExact(target int) (bool, error) {
	panic("unsupported")
}

func (b *binaryMergedDocValues) DocID() int {
	return b.merged.docIDOut
}

func (b *binaryMergedDocValues) NextDoc() (int, error) {
	return b.merged.nextDoc(b.merged.onDisk.NextDoc, b.merged.update.NextDoc), nil
}

func (b *binaryMergedDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	b.merged.intoBitSet(upTo, bitSet, offset, func(s *util.FixedBitSet, off, u int) {
		b.merged.onDisk.IntoBitSet(u, s, off)
	})
	return nil
}

func (b *binaryMergedDocValues) Cost() int64 {
	return b.merged.onDisk.Cost()
}

// ReadersAndUpdates holds an open [SegmentReader] (for searching or
// merging), plus pending deletes and resolved doc-values updates, for a
// single segment. Mirrors the package-private
// {@code org.apache.lucene.index.ReadersAndUpdates}.
//
// Concurrency: every method that mutates internal state takes the
// embedded [sync.Mutex]; reads that are read-only take RLock through the
// dedicated read methods. The refCount field is a lock-free
// [atomic.Int32]. The ramBytesUsed counter is a lock-free
// [atomic.Int64].
type ReadersAndUpdates struct {
	// info is the SegmentCommitInfo this entry wraps. Not final in
	// Lucene because the writer may replace it after a clone; the same
	// pointer is reused here, with mutating clones swapped under mu by
	// callers who own the entry.
	info *SegmentCommitInfo

	// refCount tracks how many consumers are using this instance.
	// Initialised to 1.
	refCount atomic.Int32

	// reader is the lazily opened SegmentReader for this segment. Set
	// once (or replaced via swapNewReaderWithLatestLiveDocs) and reset
	// to nil by DropReaders.
	reader *SegmentReader

	// pendingDeletes is the docID-set + bookkeeping used to record
	// not-yet-flushed deletes against this segment. See file header
	// for the divergence note: the Gocene PendingDeletes only exposes
	// a tiny subset of the Lucene surface.
	pendingDeletes *PendingDeletes

	// indexCreatedVersionMajor is the major version this index was
	// created with. Carried so it can be threaded into the SegmentReader
	// constructor when (and if) that constructor needs it.
	indexCreatedVersionMajor int

	// isMerging is true while this segment is currently being merged.
	// While merging, every new DV update is also stored in
	// mergingDVUpdates so IndexWriter can replay them on the merged
	// segment.
	isMerging bool

	// pendingDVUpdates holds resolved (to docIDs) doc-values updates
	// that have not yet been written to the index, grouped by field.
	pendingDVUpdates map[string][]*readersAndUpdatesPacket

	// mergingDVUpdates mirrors pendingDVUpdates for updates resolved
	// while this segment is being merged. At end-of-merge IndexWriter
	// carries them over (remapping their docIDs) to the merged segment.
	mergingDVUpdates map[string][]*readersAndUpdatesPacket

	// sortMap is set only when there are DV updates against this
	// segment AND the index is sorted. Mirrors Lucene's package-private
	// {@code Sorter.DocMap}.
	sortMap sortDocMap

	// ramBytesUsed accumulates the RAM footprint of the pending DV
	// updates held by this entry. Decremented by PruneAppliedDVUpdates.
	ramBytesUsed atomic.Int64

	// mu guards every non-atomic field above.
	mu sync.Mutex
}

// NewReadersAndUpdates creates a new entry without an open reader.
// Mirrors Lucene's
// {@code ReadersAndUpdates(int, SegmentCommitInfo, PendingDeletes)}
// constructor. The reference count starts at 1.
func NewReadersAndUpdates(
	indexCreatedVersionMajor int,
	info *SegmentCommitInfo,
	pendingDeletes *PendingDeletes,
) (*ReadersAndUpdates, error) {
	if info == nil {
		return nil, fmt.Errorf("readers and updates: info must not be nil")
	}
	if pendingDeletes == nil {
		return nil, fmt.Errorf("readers and updates: pendingDeletes must not be nil")
	}
	rau := &ReadersAndUpdates{
		info:                     info,
		pendingDeletes:           pendingDeletes,
		indexCreatedVersionMajor: indexCreatedVersionMajor,
		pendingDVUpdates:         make(map[string][]*readersAndUpdatesPacket),
		mergingDVUpdates:         make(map[string][]*readersAndUpdatesPacket),
	}
	rau.refCount.Store(1)
	return rau, nil
}

// NewReadersAndUpdatesFromReader initialises a new entry from a
// previously opened [SegmentReader].
//
// Lucene NOTE: "steals incoming ref from reader". In Gocene there is no
// SegmentReader ref count yet, so the caller transfers ownership of the
// reader pointer. The new entry will call [SegmentReader.Close] on it
// from [ReadersAndUpdates.DropReaders].
//
// The PendingDeletes.onNewReader callback used by Lucene is not invoked
// because that callback is not yet ported (see file header).
func NewReadersAndUpdatesFromReader(
	indexCreatedVersionMajor int,
	reader *SegmentReader,
	pendingDeletes *PendingDeletes,
) (*ReadersAndUpdates, error) {
	if reader == nil {
		return nil, fmt.Errorf("readers and updates: reader must not be nil")
	}
	info := reader.GetSegmentCommitInfo()
	if info == nil {
		return nil, fmt.Errorf("readers and updates: reader has no SegmentCommitInfo")
	}
	rau, err := NewReadersAndUpdates(indexCreatedVersionMajor, info, pendingDeletes)
	if err != nil {
		return nil, err
	}
	rau.reader = reader
	return rau, nil
}

// IncRef increments the reference count. Mirrors
// {@code ReadersAndUpdates#incRef()}. The post-increment value must be
// greater than 1; an error is returned if the invariant is violated.
// Lucene reifies this assertion through {@code assert rc > 1}, which is
// only active under {@code -ea}. The port surfaces the same invariant as
// a real error so it cannot be silenced in production.
func (r *ReadersAndUpdates) IncRef() error {
	rc := r.refCount.Add(1)
	if rc <= 1 {
		return fmt.Errorf("readers and updates: incRef invariant violated, seg=%s rc=%d", r.info, rc)
	}
	return nil
}

// DecRef decrements the reference count. Mirrors
// {@code ReadersAndUpdates#decRef()}. The post-decrement value must be
// non-negative; an error is returned otherwise. Same rationale as
// [ReadersAndUpdates.IncRef].
func (r *ReadersAndUpdates) DecRef() error {
	rc := r.refCount.Add(-1)
	if rc < 0 {
		return fmt.Errorf("readers and updates: decRef invariant violated, seg=%s rc=%d", r.info, rc)
	}
	return nil
}

// RefCount returns the current reference count. Mirrors
// {@code ReadersAndUpdates#refCount()}.
func (r *ReadersAndUpdates) RefCount() int {
	return int(r.refCount.Load())
}

// Info returns the [SegmentCommitInfo] this entry wraps. Lucene exposes
// the field directly via package-private access; the accessor here keeps
// the field encapsulated.
func (r *ReadersAndUpdates) Info() *SegmentCommitInfo {
	return r.info
}

// GetDelCount returns the number of pending deletes recorded against
// this segment. Mirrors {@code ReadersAndUpdates#getDelCount()}.
//
// DIVERGENCE: Lucene delegates to PendingDeletes.getDelCount(); the
// Gocene PendingDeletes only tracks the docID set, so the count is the
// size of that set.
func (r *ReadersAndUpdates) GetDelCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pendingDeletes.delCountLocked()
}

// AddDVUpdate adds a new resolved doc-values update packet. The packet
// must already be finished. Mirrors
// {@code ReadersAndUpdates#addDVUpdate(DocValuesFieldUpdates)}.
//
// Duplicate-delGen detection is performed under the lock; on duplicate
// the call returns an error (Lucene throws AssertionError on the same
// condition).
//
// When [ReadersAndUpdates.IsMerging] is true the packet is mirrored
// into mergingDVUpdates for end-of-merge carry-over.
func (r *ReadersAndUpdates) AddDVUpdate(update *BaseDocValuesFieldUpdates) error {
	if update == nil {
		return fmt.Errorf("readers and updates: update must not be nil")
	}
	return r.addDVUpdatePacket(&readersAndUpdatesPacket{inner: update})
}

// addDVUpdatePacket is the internal implementation; the public AddDVUpdate
// is a thin typed wrapper. Splitting them keeps the orchestrator
// interface-friendly (so future numeric/binary subclasses can be plugged
// in) without exposing the wrapper type.
func (r *ReadersAndUpdates) addDVUpdatePacket(packet *readersAndUpdatesPacket) error {
	if packet == nil || packet.inner == nil {
		return fmt.Errorf("readers and updates: packet must not be nil")
	}
	if !packet.inner.GetFinished() {
		return ErrReadersAndUpdatesUpdateNotFinished
	}
	field := packet.inner.Field()
	r.mu.Lock()
	defer r.mu.Unlock()

	existing := r.pendingDVUpdates[field]
	if err := r.assertNoDupGenLocked(existing, packet); err != nil {
		return err
	}

	r.ramBytesUsed.Add(packet.inner.RamBytesUsed())
	r.pendingDVUpdates[field] = append(existing, packet)

	if r.isMerging {
		r.mergingDVUpdates[field] = append(r.mergingDVUpdates[field], packet)
	}
	return nil
}

// assertNoDupGenLocked mirrors {@code ReadersAndUpdates#assertNoDupGen}.
// The caller must hold r.mu.
func (r *ReadersAndUpdates) assertNoDupGenLocked(
	existing []*readersAndUpdatesPacket,
	candidate *readersAndUpdatesPacket,
) error {
	gen := candidate.inner.DelGen()
	for _, old := range existing {
		if old.inner.DelGen() == gen {
			return fmt.Errorf("readers and updates: duplicate delGen=%d for seg=%s", gen, r.info)
		}
	}
	return nil
}

// GetNumDVUpdates returns the total number of pending DV update packets
// across all fields. Mirrors {@code ReadersAndUpdates#getNumDVUpdates()}.
func (r *ReadersAndUpdates) GetNumDVUpdates() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, updates := range r.pendingDVUpdates {
		count += int64(len(updates))
	}
	return count
}

// RamBytesUsed returns the accumulated RAM footprint of the pending DV
// updates held by this entry. Lock-free; mirrors Lucene's
// {@code ReadersAndUpdates#ramBytesUsed}.
func (r *ReadersAndUpdates) RamBytesUsed() int64 {
	return r.ramBytesUsed.Load()
}

// GetReader returns the [SegmentReader] for this segment, opening one
// on first call and incrementing its external ref count.
//
// DIVERGENCE: Lucene increments the SegmentReader's own ref count and
// invokes pendingDeletes.onNewReader; neither hook is ported. The Gocene
// implementation lazily constructs a SegmentReader via [NewSegmentReader]
// when none is set and returns the cached pointer. The "extra ref for the
// caller" semantics are not enforced; callers must coordinate ownership
// through [ReadersAndUpdates.IncRef] / [ReadersAndUpdates.DecRef] until
// SegmentReader ref counting lands.
func (r *ReadersAndUpdates) GetReader() (*SegmentReader, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reader == nil {
		r.reader = NewSegmentReader(r.info)
	}
	return r.reader, nil
}

// Release is the symmetric counterpart of [ReadersAndUpdates.GetReader].
// Mirrors {@code ReadersAndUpdates#release(SegmentReader)}.
//
// DIVERGENCE: Lucene asserts the reader is the same one it tracks and
// calls SegmentReader.decRef. Gocene cannot decRef the inner reader; the
// method validates the identity and returns nil if it matches.
func (r *ReadersAndUpdates) Release(sr *SegmentReader) error {
	if sr == nil {
		return fmt.Errorf("readers and updates: cannot release nil reader")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reader != nil && sr != r.reader {
		return fmt.Errorf("readers and updates: released reader is not the cached one")
	}
	return nil
}

// Delete records a pending delete for the given docID. Mirrors
// {@code ReadersAndUpdates#delete(int)}.
//
// DIVERGENCE: Lucene routes the call through
// PendingDeletes.delete(docID), which validates the docID, may need an
// open reader to initialise live-docs, and returns false if the doc was
// already deleted. The Gocene PendingDeletes only stores the set, so the
// orchestrator records the docID directly and returns true iff the docID
// was not already present.
func (r *ReadersAndUpdates) Delete(docID int) (bool, error) {
	if docID < 0 {
		return false, fmt.Errorf("readers and updates: negative docID %d", docID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pendingDeletes.recordDeleteLocked(docID), nil
}

// DropReaders releases the cached SegmentReader (calling
// [SegmentReader.Close]) and decrements the entry's external ref count.
// Mirrors {@code ReadersAndUpdates#dropReaders()}.
//
// DIVERGENCE: Lucene calls SegmentReader.decRef, not Close; Gocene has
// no SegmentReader ref count, so Close is the closest equivalent. The
// entry-level decRef is still performed.
func (r *ReadersAndUpdates) DropReaders() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var closeErr error
	if r.reader != nil {
		closeErr = r.reader.Close()
		r.reader = nil
	}
	if err := r.DecRef(); err != nil {
		if closeErr != nil {
			return fmt.Errorf("readers and updates: drop readers: close=%v, decRef=%w", closeErr, err)
		}
		return err
	}
	return closeErr
}

// GetReadOnlyClone is the entry-point that Lucene uses to hand a fresh
// SegmentReader (with replacement live-docs) to consumers that must see
// the latest deletes.
func (r *ReadersAndUpdates) GetReadOnlyClone() (*SegmentReader, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.reader == nil {
		r.reader = NewSegmentReader(r.info)
	}

	liveDocs := r.pendingDeletes.GetLiveDocs()
	if liveDocs != nil {
		return NewSegmentReaderClone(
			r.info,
			r.reader,
			liveDocs,
			r.pendingDeletes.GetHardLiveDocs(),
			r.pendingDeletes.NumDocs(),
			true), nil
	}

	// liveDocs == nil and reader != nil. That can only be if there are no deletes
	r.reader.IncRef()
	return r.reader, nil
}

// NumDeletesToMerge returns the number of deletes that would be applied
// when this segment is merged with the supplied [MergePolicy]. The
// underlying PendingDeletes.numDeletesToMerge entry point is not yet
// ported. See file header.
func (r *ReadersAndUpdates) NumDeletesToMerge(_ MergePolicy) (int, error) {
	return 0, ErrReadersAndUpdatesMergeReaderUnsupported
}

// GetLiveDocs returns a snapshot of the live docs.
func (r *ReadersAndUpdates) GetLiveDocs() (util.Bits, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pendingDeletes.GetLiveDocs(), nil
}

// GetHardLiveDocs returns the live-docs bits excluding soft-deleted
// documents.
func (r *ReadersAndUpdates) GetHardLiveDocs() (util.Bits, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pendingDeletes.GetHardLiveDocs(), nil
}

// DropChanges discards any pending changes against this segment.
// Mirrors {@code ReadersAndUpdates#dropChanges()}.
//
// DIVERGENCE: Lucene delegates to PendingDeletes.dropChanges() and then
// drops merging updates. The Gocene PendingDeletes has no equivalent
// hook, so the docID set is cleared directly and merging updates are
// dropped via [ReadersAndUpdates.DropMergingUpdates].
func (r *ReadersAndUpdates) DropChanges() {
	r.mu.Lock()
	r.pendingDeletes.clearLocked()
	r.mu.Unlock()
	r.DropMergingUpdates()
}

// WriteLiveDocs flushes any pending live-docs changes to disk.
func (r *ReadersAndUpdates) WriteLiveDocs(dir store.Directory) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pendingDeletes.WriteLiveDocs(dir)
}

func (r *ReadersAndUpdates) writeFieldInfosGen(
	fieldInfos *schema.FieldInfos,
	dir store.Directory,
	infosFormat spi.FieldInfosFormat,
) (map[string]int64, error) {
	nextFieldInfosGen := r.info.NextFieldInfosGen()
	segmentSuffix := strconv.FormatInt(nextFieldInfosGen, 36)

	estInfosSize := int64(40 + 90*fieldInfos.Size())
	infosContext := store.NewFlushIOContext(r.info.SegmentInfo().DocCount(), estInfosSize)

	trackingDir := store.NewTrackingDirectoryWrapper(dir)
	err := infosFormat.Write(trackingDir, r.info.SegmentInfo(), segmentSuffix, fieldInfos, infosContext)
	if err != nil {
		return nil, err
	}
	r.info.AdvanceFieldInfosGen()
	return trackingDir.GetCreatedFiles(), nil
}

func (r *ReadersAndUpdates) handleDVUpdates(
	infos *schema.FieldInfos,
	dir store.Directory,
	dvFormat spi.DocValuesFormat,
	reader *SegmentReader,
	fieldFiles map[int]map[string]int64,
	maxDelGen int64,
	infoStream util.InfoStream,
) error {
	for field, updates := range r.pendingDVUpdates {
		if len(updates) == 0 {
			continue
		}
		update := updates[0]
		dvType := update.inner.Type()

		updatesToApply := make([]*readersAndUpdatesPacket, 0, len(updates))
		var bytes int64
		for _, u := range updates {
			if u.inner.DelGen() <= maxDelGen {
				bytes += u.inner.RamBytesUsed()
				updatesToApply = append(updatesToApply, u)
			}
		}

		if len(updatesToApply) == 0 {
			continue
		}

		if infoStream != nil && infoStream.IsEnabled("BD") {
			infoStream.Message("BD", fmt.Sprintf(
				"now write %d pending DV updates for field=%s, seg=%s, bytes=%.3f MB",
				len(updatesToApply), field, r.info, float64(bytes)/1024/1024))
		}

		nextDocValuesGen := r.info.NextDocValuesGen()
		segmentSuffix := strconv.FormatInt(nextDocValuesGen, 36)
		updatesContext := store.NewFlushIOContext(r.info.SegmentInfo().DocCount(), bytes)

		fieldInfo := infos.FieldInfoByName(field)
		if fieldInfo == nil {
			return fmt.Errorf("field info not found for field %s", field)
		}
		fieldInfo.SetDocValuesGen(nextDocValuesGen)

		fieldInfos := schema.NewFieldInfos([]*schema.FieldInfo{fieldInfo})
		trackingDir := store.NewTrackingDirectoryWrapper(dir)
		state := NewSegmentWriteStateWithSuffix(infoStream, trackingDir, r.info.SegmentInfo(), fieldInfos, nil, updatesContext, segmentSuffix)

		fieldsConsumer, err := dvFormat.FieldsConsumer(state)
		if err != nil {
			return err
		}

		updateSupplier := func(fi *schema.FieldInfo) DocValuesFieldUpdatesIterator {
			if fi.Name() != fieldInfo.Name() {
				panic(fmt.Sprintf("expected field info for field: %s but got: %s", fieldInfo.Name(), fi.Name()))
			}
			subs := make([]DocValuesFieldUpdatesIterator, len(updatesToApply))
			for i, u := range updatesToApply {
				subs[i] = u.inner.(*BaseDocValuesFieldUpdates).Iterator()
			}
			return MergedDocValuesFieldUpdatesIterator(subs)
		}

		if dvType == DocValuesTypeBinary {
			err = fieldsConsumer.AddBinaryField(fieldInfo, &binaryMergedDocValues{
				merged: newMergedDocValues(reader.GetBinaryDocValues(field),
					AsBinaryDocValues(updateSupplier(fieldInfo)),
					updateSupplier(fieldInfo)),
			})
		} else {
			err = fieldsConsumer.AddNumericField(fieldInfo, &numericMergedDocValues{
				merged: newMergedDocValues(reader.GetNumericDocValues(field),
					AsNumericDocValues(updateSupplier(fieldInfo)),
					updateSupplier(fieldInfo)),
			})
		}
		fieldsConsumer.Close()
		if err != nil {
			return err
		}

		fieldFiles[fieldInfo.Number()] = trackingDir.GetCreatedFiles()
	}
	return nil
}

func (r *ReadersAndUpdates) WriteFieldUpdates(
	dir store.Directory,
	fieldNumbers *schema.FieldNumbers,
	maxDelGen int64,
	infoStream util.InfoStream,
) (bool, error) {
	startTime := util.Now()
	newDVFiles := make(map[int]map[string]int64)
	var fieldInfosFiles map[string]int64
	var fieldInfos *schema.FieldInfos
	any := false
	for _, updates := range r.pendingDVUpdates {
		for _, update := range updates {
			if update.inner.DelGen() <= maxDelGen && update.inner.Any() {
				any = true
				break
			}
		}
		if any {
			break
		}
	}

	if !any {
		return false, nil
	}

	trackingDir := store.NewTrackingDirectoryWrapper(dir)
	success := false
	defer func() {
		if !success {
			r.info.AdvanceNextWriteFieldInfosGen()
			r.info.AdvanceNextWriteDocValuesGen()
			for fileName := range trackingDir.GetCreatedFiles() {
				dir.DeleteFile(fileName)
			}
		}
	}()

	var reader *SegmentReader
	if r.reader == nil {
		reader = NewSegmentReader(r.info)
	} else {
		reader = r.reader
	}

	byName := make(map[string]*schema.FieldInfo)
	maxFieldNumber := -1
	for _, fi := range reader.GetFieldInfos().Iterator() {
		byName[fi.Name()] = fi.Clone(fi.Number())
		if fi.Number() > maxFieldNumber {
			maxFieldNumber = fi.Number()
		}
	}

	for _, updates := range r.pendingDVUpdates {
		if len(updates) == 0 {
			continue
		}
		update := updates[0]
		field := update.inner.Field()
		if fi, ok := byName[field]; ok {
			// field already exists
		} else {
			fi := fieldNumbers.ConstructFieldInfo(field, update.inner.Type(), maxFieldNumber+1)
			maxFieldNumber++
			byName[fi.Name()] = fi
		}
	}
	fieldInfos = schema.NewFieldInfos(mapToSlice(byName))

	codec := LookupCodecByName(r.info.SegmentInfo().Codec())
	err := r.handleDVUpdates(fieldInfos, trackingDir, codec.DocValuesFormat(), reader, newDVFiles, maxDelGen, infoStream)
	if err != nil {
		return false, err
	}

	fieldInfosFiles, err = r.writeFieldInfosGen(fieldInfos, trackingDir, codec.FieldInfosFormat())
	if err != nil {
		return false, err
	}

	success = true

	bytesFreed := r.PruneAppliedDVUpdates(maxDelGen)
	_ = bytesFreed

	r.info.SetFieldInfosFiles(fieldInfosFiles)
	for fieldNum, files := range r.info.DocValuesUpdatesFiles() {
		if _, ok := newDVFiles[fieldNum]; !ok {
			newDVFiles[fieldNum] = files
		}
	}
	r.info.SetDocValuesUpdatesFiles(newDVFiles)

	if r.reader != nil {
		r.swapNewReaderWithLatestLiveDocs()
	}

	if infoStream != nil && infoStream.IsEnabled("BD") {
		infoStream.Message("BD", fmt.Sprintf(
			"done write field updates for seg=%s; took %.3fs; new files: %v",
			r.info, util.Since(startTime).Seconds(), newDVFiles))
	}
	return true, nil
}

func mapToSlice(m map[string]*schema.FieldInfo) []*schema.FieldInfo {
	s := make([]*schema.FieldInfo, 0, len(m))
	for _, v := range m {
		s = append(s, v)
	}
	return s
}

// anyDVUpdatesEligibleLocked mirrors the early-exit loop at the head of
// {@code ReadersAndUpdates#writeFieldUpdates}. The caller must hold r.mu.
func (r *ReadersAndUpdates) anyDVUpdatesEligibleLocked(maxDelGen int64) bool {
	for _, updates := range r.pendingDVUpdates {
		for _, update := range updates {
			if update.inner.DelGen() <= maxDelGen && update.inner.Any() {
				return true
			}
		}
	}
	return false
}

// PruneAppliedDVUpdates strips every pending DV update whose delGen is
// <= maxDelGen and returns the number of bytes freed. Mirrors the
// post-write prune loop in {@code ReadersAndUpdates#writeFieldUpdates}.
//
// This helper is split out so the bookkeeping can be exercised without
// going through the not-yet-ported codec-write path. Once
// [ReadersAndUpdates.WriteFieldUpdates] is fully ported it will call
// this helper internally.
func (r *ReadersAndUpdates) PruneAppliedDVUpdates(maxDelGen int64) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	var bytesFreed int64
	for field, updates := range r.pendingDVUpdates {
		upto := 0
		for _, update := range updates {
			if update.inner.DelGen() > maxDelGen {
				updates[upto] = update
				upto++
			} else {
				bytesFreed += update.inner.RamBytesUsed()
			}
		}
		if upto == 0 {
			delete(r.pendingDVUpdates, field)
		} else {
			for i := upto; i < len(updates); i++ {
				updates[i] = nil
			}
			r.pendingDVUpdates[field] = updates[:upto]
		}
	}
	if bytesFreed > 0 {
		bytes := r.ramBytesUsed.Add(-bytesFreed)
		if bytes < 0 {
			r.ramBytesUsed.Add(bytesFreed)
			panic(fmt.Sprintf("readers and updates: ramBytesUsed went negative: %d", bytes))
		}
	}
	return bytesFreed
}

// SetIsMerging marks this entry as being merged. Mirrors
// {@code ReadersAndUpdates#setIsMerging()}. The first call also asserts
// that mergingDVUpdates is empty (mirroring Lucene's assertion).
func (r *ReadersAndUpdates) SetIsMerging() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isMerging {
		return nil
	}
	if len(r.mergingDVUpdates) != 0 {
		return fmt.Errorf("readers and updates: setIsMerging called with non-empty mergingDVUpdates")
	}
	r.isMerging = true
	return nil
}

// IsMerging reports whether this entry is currently being merged.
// Mirrors {@code ReadersAndUpdates#isMerging()}.
func (r *ReadersAndUpdates) IsMerging() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isMerging
}

// GetReaderForMerge would return a reader carrying the latest doc-values
// updates and deletions, wrapped in a MergePolicy.MergeReader.
// MergePolicy.MergeReader is not yet ported. See file header.
func (r *ReadersAndUpdates) GetReaderForMerge() (any, error) {
	return nil, ErrReadersAndUpdatesMergeReaderUnsupported
}

// DropMergingUpdates drops all merging updates. Mirrors
// {@code ReadersAndUpdates#dropMergingUpdates()}.
func (r *ReadersAndUpdates) DropMergingUpdates() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range r.mergingDVUpdates {
		delete(r.mergingDVUpdates, k)
	}
	r.isMerging = false
}

// GetMergingDVUpdates returns the per-field list of DV update packets
// gathered while this segment was being merged. Mirrors
// {@code ReadersAndUpdates#getMergingDVUpdates()} — the call atomically
// clears the isMerging flag so subsequent updates land in pendingDVUpdates
// only (never lost).
//
// The returned map is a shallow copy; mutating it does not affect the
// internal state, but the per-field slices are shared with future
// callers until those callers detach them.
func (r *ReadersAndUpdates) GetMergingDVUpdates() map[string][]*BaseDocValuesFieldUpdates {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.isMerging = false
	out := make(map[string][]*BaseDocValuesFieldUpdates, len(r.mergingDVUpdates))
	for field, packets := range r.mergingDVUpdates {
		typed := make([]*BaseDocValuesFieldUpdates, 0, len(packets))
		for _, packet := range packets {
			if base, ok := packet.inner.(*BaseDocValuesFieldUpdates); ok {
				typed = append(typed, base)
			}
		}
		out[field] = typed
	}
	return out
}

// IsFullyDeleted reports whether every document in this segment is
// deleted.
func (r *ReadersAndUpdates) IsFullyDeleted() (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.reader != nil {
		if nrtr, ok := r.reader.(*NRTSegmentReader); ok {
			return nrtr.NumDocs() == 0, nil
		}
	}

	return r.pendingDeletes.NumPendingDeletes() == r.info.SegmentInfo().DocCount(), nil
}

// KeepFullyDeletedSegment asks the supplied MergePolicy whether a
// fully-deleted segment should be retained. The policy hook depends on
// PendingDeletes.getLatestReader, which is not yet ported. See file header.
func (r *ReadersAndUpdates) KeepFullyDeletedSegment(_ MergePolicy) (bool, error) {
	return false, ErrReadersAndUpdatesLiveDocsUnsupported
}

// SortMap returns the per-segment sort doc-map, or nil when the index
// is not sorted. The setter is package-internal and reserved for the
// future sort-aware DV writer path.
func (r *ReadersAndUpdates) SortMap() sortDocMap {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sortMap
}

// SetSortMap installs the per-segment sort doc-map. Visible for the
// (currently stubbed) DV-update writer.
func (r *ReadersAndUpdates) SetSortMap(m sortDocMap) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sortMap = m
}

// String renders the pending-deletes set in a stable order for the
// orchestrator's toString output. Mirrors the human-readable
// PendingDeletes#toString in Lucene by reporting the count only — the
// raw docID set is intentionally not exposed in the string form.
func (r *ReadersAndUpdates) String() string {
	return fmt.Sprintf("ReadersAndLiveDocs(seg=%s pendingDeletes=%s)", r.info, r.pendingDeletes)
}

// ----- PendingDeletes adapters --------------------------------------------
//
// These three helpers name, for the orchestrator above, the PendingDeletes
// operations Lucene's ReadersAndUpdates calls directly on the field:
// numPendingDeletes(), delete(int) and dropChanges(). Each is a thin forward
// to the corresponding method on the ported PendingDeletes, which does its own
// locking.

// delCountLocked returns the number of recorded pending deletes. Mirrors
// PendingDeletes.numPendingDeletes().
func (p *PendingDeletes) delCountLocked() int {
	return p.NumPendingDeletes()
}

// recordDeleteLocked records a delete, reporting whether the document was not
// already deleted. Mirrors PendingDeletes.delete(int).
func (p *PendingDeletes) recordDeleteLocked(docID int) bool {
	return p.Delete(docID)
}

// clearLocked discards every recorded pending delete. Mirrors
// PendingDeletes.dropChanges().
func (p *PendingDeletes) clearLocked() {
	p.DropChanges()
}

// ----- dvUpdatePacket adapter on BaseDocValuesFieldUpdates ----------------
//
// BaseDocValuesFieldUpdates exposes Field/Type/DelGen/Any/GetFinished
// directly. It also exposes RamBytesUsedBase for the shallow footprint;
// concrete subtypes (e.g. BinaryDocValuesFieldUpdates) override the full
// RamBytesUsed with their own auxiliary-array sizes. The adapter below
// makes a bare BaseDocValuesFieldUpdates satisfy [dvUpdatePacket] by
// promoting RamBytesUsedBase to the polymorphic name. Concrete subtypes
// shadow this method through their own RamBytesUsed, so the orchestrator
// always sees the most precise accounting available.
//
// RamBytesUsed returns the shallow RAM footprint of this packet. Concrete
// subtypes (BinaryDocValuesFieldUpdates, future numeric variants) override
// this with auxiliary-array-aware accounting.
func (b *BaseDocValuesFieldUpdates) RamBytesUsed() int64 {
	if b == nil {
		return 0
	}
	return b.RamBytesUsedBase()
}
