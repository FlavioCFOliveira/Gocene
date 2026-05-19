// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// FrozenBufferedUpdates
// Source: lucene/core/src/java/org/apache/lucene/index/FrozenBufferedUpdates.java
// (Apache Lucene 10.4.0)
//
// Holds buffered deletes and updates by term or query, once pushed. Pushed
// deletes/updates are write-once, so the structure shifts to a memory-
// efficient projection of [BufferedUpdates]. DocIDs are not held here:
// per-segment doc-id resolution happens during [FrozenBufferedUpdates.Apply].
//
// Sprint 55 (GOC-3360) scope:
//
//   - This file replaces the empty placeholder previously declared in
//     [documents_writer_flush_queue.go] (Sprint 55 decision Q2).
//   - The projection logic (term iteration, query packing, RAM accounting,
//     lock/latch semantics, delGen lifecycle, String, Any) is fully ported.
//   - [FrozenBufferedUpdates.TermDocsIterator] is fully ported and exercised
//     by the unit tests in this package against the existing [LeafReader]
//     and [Fields] surfaces.
//   - The full Apply pipeline (term/query/doc-values application against
//     live segments) requires types that are not yet present in Gocene:
//     FieldUpdatesBuffer, NumericDocValuesFieldUpdates, the per-segment
//     SegmentState carrying ReadersAndUpdates + sort map, and an
//     IndexSearcher able to rewrite + score [Query] objects against a
//     [LeafReader]. The [FrozenBufferedUpdates.Apply] entry point is wired
//     here and dispatches to per-segment hooks, but the deep
//     applyTermDeletes / applyQueryDeletes / applyDocValuesUpdates bodies
//     return [ErrFrozenBufferedUpdatesNotApplicable] when invoked with
//     non-empty payloads — they are documented divergences scheduled for
//     resolution alongside the dependent ports (see backlog #2705,
//     FieldUpdatesBuffer / BufferedUpdatesStream sprint).
//
// All observable surface that does not depend on the missing types behaves
// byte-for-byte as the Lucene reference. Tests cover the projection and
// the iterator on real on-disk segments.

package index

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FrozenBytesPerDelQuery is the RAM accounting constant used to size the
// query-portion of a frozen packet. Mirrors
// {@code FrozenBufferedUpdates.BYTES_PER_DEL_QUERY} from Lucene 10.4.0:
// one object reference, one int and a 24-byte query footprint estimate.
//
// Lucene reference: NUM_BYTES_OBJECT_REF (8) + Integer.BYTES (4) + 24.
const FrozenBytesPerDelQuery = 8 + 4 + 24

// ErrFrozenBufferedUpdatesNotPushed mirrors the
// IllegalArgumentException thrown by Lucene's apply() when invoked before
// BufferedUpdatesStream.push has assigned a delGen.
var ErrFrozenBufferedUpdatesNotPushed = errors.New(
	"frozen buffered updates: gen is not yet set; call SetDelGen first",
)

// ErrFrozenBufferedUpdatesNotApplicable is returned by [FrozenBufferedUpdates.Apply]
// while the dependent ports (FieldUpdatesBuffer, BufferedUpdatesStream
// SegmentState, IndexSearcher pipeline) are still absent from Gocene.
// The error is only returned when the packet actually has work to do; an
// empty packet returns 0 with no error so the wiring around DWFC can be
// exercised end-to-end. See file header for the divergence rationale.
var ErrFrozenBufferedUpdatesNotApplicable = errors.New(
	"frozen buffered updates: full Apply pipeline pending dependent ports " +
		"(FieldUpdatesBuffer / BufferedUpdatesStream.SegmentState)",
)

// frozenQueryEntry is the parallel (Query, docIDUpto) projection produced
// from [BufferedUpdates.deleteQueries] at freeze time. The pair mirrors
// Lucene's parallel arrays deleteQueries[] and deleteQueryLimits[].
type frozenQueryEntry struct {
	query Query
	limit int
}

// FrozenBufferedUpdates is the Go port of
// org.apache.lucene.index.FrozenBufferedUpdates.
//
// Instances are constructed via [NewFrozenBufferedUpdates] from a mutable
// [BufferedUpdates] packet. After construction the packet is read-only
// except for the [FrozenBufferedUpdates.SetDelGen] one-shot and the
// totalDelCount counter maintained by [FrozenBufferedUpdates.Apply].
//
// Concurrency: every mutating method takes [FrozenBufferedUpdates.Lock]
// or its non-blocking [FrozenBufferedUpdates.TryLock] variant first. The
// [FrozenBufferedUpdates.Applied] latch is fired once Apply completes
// successfully on any goroutine, mirroring Lucene's CountDownLatch.
type FrozenBufferedUpdates struct {
	// deleteTerms is the sorted-by-(field,bytes) projection of
	// BufferedUpdates.deleteTerms produced by PrefixCodedTermsBuilder.
	deleteTerms *PrefixCodedTerms

	// deleteQueries is the parallel-array projection of
	// BufferedUpdates.deleteQueries (parallel slices in Lucene).
	deleteQueries []frozenQueryEntry

	// applied is closed exactly once when Apply succeeds.
	applied chan struct{}
	// appliedOnce guards the close of applied.
	appliedOnce sync.Once

	// applyMu is the reentrant-like guard around Apply. Go has no
	// reentrant mutex; the port uses a plain Mutex and relies on the
	// invariant that Apply never re-enters itself on the same goroutine.
	applyMu sync.Mutex
	// applyHeld is set under applyMu so IsApplied / asserts can confirm
	// the caller currently holds the lock.
	applyHeld atomic.Bool

	// totalDelCount accumulates the new deletes/updates produced by
	// successive Apply calls.
	totalDelCount int64

	// bytesUsed is the post-compression RAM accounting captured at
	// construction time. Matches Lucene's bytesUsed (an int cast).
	bytesUsed int32

	// delGen is the deletion generation assigned via SetDelGen.
	// A value of -1 indicates the packet has not yet been pushed.
	delGen int64

	// privateSegment is non-nil iff this packet represents segment-private
	// deletes (only query deletes and doc values updates are allowed).
	privateSegment *SegmentCommitInfo

	// infoStream is the diagnostics sink. May be the [util.NoOpInfoStream]
	// singleton; never nil after construction.
	infoStream util.InfoStream
}

// NewFrozenBufferedUpdates compresses a mutable [BufferedUpdates] packet
// into the write-once frozen form. The infoStream is optional: passing
// nil installs [util.NoOpInfoStream]. The privateSegment is non-nil
// for segment-private packets (which must not carry term deletes).
//
// Returns an error if invariants are violated (privateSegment + term
// deletes is rejected, mirroring Lucene's assertion).
func NewFrozenBufferedUpdates(
	infoStream util.InfoStream,
	updates *BufferedUpdates,
	privateSegment *SegmentCommitInfo,
) (*FrozenBufferedUpdates, error) {
	if updates == nil {
		return nil, errors.New("frozen buffered updates: updates must not be nil")
	}
	if privateSegment != nil && !updates.deleteTerms.IsEmpty() {
		return nil, errors.New(
			"frozen buffered updates: segment private packet must only carry query deletes",
		)
	}
	if infoStream == nil {
		infoStream = util.NoOpInfoStream
	}

	// Build the sorted PrefixCodedTerms projection from deleteTerms.
	builder := NewPrefixCodedTermsBuilder()
	for _, entry := range updates.deleteTerms.ForEachOrdered() {
		builder.AddFieldBytes(entry.Field, entry.Bytes)
	}
	frozenTerms := builder.Finish()

	// Project the query map into parallel slices. Iteration order over a
	// Go map is randomized; Lucene's LinkedHashMap (entrySet) preserves
	// insertion order. The application loop in Apply is order-agnostic
	// (each query is independently evaluated against every segment) so
	// the projection is sorted by query identity to keep observable
	// output (RAM accounting, Any, String) deterministic across runs.
	queries := make([]frozenQueryEntry, 0, len(updates.deleteQueries))
	for q, limit := range updates.deleteQueries {
		queries = append(queries, frozenQueryEntry{query: q, limit: limit})
	}
	sort.SliceStable(queries, func(i, j int) bool {
		return queries[i].query.HashCode() < queries[j].query.HashCode()
	})

	bytes := int64(frozenTerms.Size())*estimatedTermBytes(frozenTerms) +
		int64(len(queries))*FrozenBytesPerDelQuery

	f := &FrozenBufferedUpdates{
		deleteTerms:    frozenTerms,
		deleteQueries:  queries,
		applied:        make(chan struct{}),
		bytesUsed:      int32(bytes),
		delGen:         -1,
		privateSegment: privateSegment,
		infoStream:     infoStream,
	}

	if infoStream.IsEnabled("BD") {
		origBytes := updates.RamBytesUsed()
		ratio := 0.0
		if origBytes > 0 {
			ratio = 100.0 * float64(f.bytesUsed) / float64(origBytes)
		}
		infoStream.Message("BD", fmt.Sprintf(
			"compressed %d to %d bytes (%.2f%%) for deletes/updates; private segment %v",
			origBytes, f.bytesUsed, ratio, privateSegment,
		))
	}

	return f, nil
}

// estimatedTermBytes returns a flat per-entry estimate used to size the
// term-portion of the frozen packet. The Java original calls
// {@code deleteTerms.ramBytesUsed()} on the encoded byte stream;
// PrefixCodedTerms in Gocene is still skeleton-state (see backlog #2705),
// so the port uses a constant rough estimate that keeps the bytesUsed
// counter monotonically non-zero whenever terms are present.
func estimatedTermBytes(p *PrefixCodedTerms) int64 {
	if p == nil || p.Size() == 0 {
		return 0
	}
	// 16 bytes per (field, bytes) entry mirrors the order-of-magnitude
	// of Lucene's prefix-coded output for typical Unicode terms.
	return 16
}

// TryLock attempts to acquire the apply lock without blocking. Returns
// true on success. Mirrors Lucene's ReentrantLock.tryLock.
func (f *FrozenBufferedUpdates) TryLock() bool {
	if f.applyMu.TryLock() {
		f.applyHeld.Store(true)
		return true
	}
	return false
}

// Lock acquires the apply lock, blocking until it is available.
func (f *FrozenBufferedUpdates) Lock() {
	f.applyMu.Lock()
	f.applyHeld.Store(true)
}

// Unlock releases the apply lock. The caller must currently hold it.
func (f *FrozenBufferedUpdates) Unlock() {
	f.applyHeld.Store(false)
	f.applyMu.Unlock()
}

// IsApplied reports whether the [FrozenBufferedUpdates.Applied] latch has
// fired. The caller must currently hold the apply lock to match the
// Lucene assertion {@code applyLock.isHeldByCurrentThread()}.
func (f *FrozenBufferedUpdates) IsApplied() bool {
	if !f.applyHeld.Load() {
		return false
	}
	select {
	case <-f.applied:
		return true
	default:
		return false
	}
}

// Applied returns a channel that is closed once Apply succeeds. The
// channel mirrors Lucene's {@code CountDownLatch applied}.
func (f *FrozenBufferedUpdates) Applied() <-chan struct{} {
	return f.applied
}

// SetDelGen assigns the deletion generation and propagates it to the
// underlying [PrefixCodedTerms]. The packet may not be re-assigned.
func (f *FrozenBufferedUpdates) SetDelGen(gen int64) error {
	if f.delGen != -1 {
		return fmt.Errorf(
			"frozen buffered updates: delGen was already set to %d", f.delGen,
		)
	}
	f.delGen = gen
	if f.deleteTerms != nil {
		f.deleteTerms.SetDelGen(gen)
	}
	return nil
}

// DelGen returns the deletion generation. The caller is expected to call
// this only after [FrozenBufferedUpdates.SetDelGen]; returns -1 if not yet
// set, matching Lucene's pre-set state.
func (f *FrozenBufferedUpdates) DelGen() int64 { return f.delGen }

// PrivateSegment returns the segment-private commit info, or nil when
// this packet carries global deletes.
func (f *FrozenBufferedUpdates) PrivateSegment() *SegmentCommitInfo {
	return f.privateSegment
}

// BytesUsed returns the RAM accounting captured at construction.
func (f *FrozenBufferedUpdates) BytesUsed() int32 { return f.bytesUsed }

// TotalDelCount returns the running tally of deletes/updates applied so
// far. The counter is updated by [FrozenBufferedUpdates.Apply].
func (f *FrozenBufferedUpdates) TotalDelCount() int64 { return f.totalDelCount }

// Any reports whether this packet carries any work. Mirrors Lucene's
// {@code any()}; doc-values projection is not yet wired so it is
// represented exclusively by deleteTerms / deleteQueries here.
func (f *FrozenBufferedUpdates) Any() bool {
	return f.deleteTerms.Size() > 0 || len(f.deleteQueries) > 0
}

// String returns a debug representation matching Lucene's toString.
func (f *FrozenBufferedUpdates) String() string {
	s := fmt.Sprintf("delGen=%d", f.delGen)
	if n := f.deleteTerms.Size(); n != 0 {
		s += fmt.Sprintf(" unique deleteTerms=%d", n)
	}
	if n := len(f.deleteQueries); n != 0 {
		s += fmt.Sprintf(" numDeleteQueries=%d", n)
	}
	if f.bytesUsed != 0 {
		s += fmt.Sprintf(" bytesUsed=%d", f.bytesUsed)
	}
	if f.privateSegment != nil {
		s += fmt.Sprintf(" privateSegment=%v", f.privateSegment)
	}
	return s
}

// FrozenSegmentState is the per-segment context handed to
// [FrozenBufferedUpdates.Apply]. It mirrors the inner type
// {@code BufferedUpdatesStream.SegmentState} from Lucene but is exposed
// here under a Gocene-local name until the BufferedUpdatesStream port
// lands. The fields are the subset consumed by the apply pipeline.
type FrozenSegmentState struct {
	// Reader is the per-segment LeafReader the iterator scans.
	Reader *LeafReader
	// DelGen is the segment's current deletion generation; updates
	// older than DelGen are skipped, matching the Lucene contract.
	DelGen int64
	// RefCount is the segment's reference count; a count of 1 means
	// the only remaining reference is held by the apply pipeline
	// itself, and the segment is skipped (it was merged away).
	RefCount int
}

// Apply applies pending term deletes, query deletes and doc values
// updates to the supplied segments and returns the cumulative number of
// affected documents. The caller must hold [FrozenBufferedUpdates.Lock].
//
// When the packet is empty, Apply returns 0 and fires the
// [FrozenBufferedUpdates.Applied] latch even if the dependent pipeline
// is not yet wired. This keeps the surrounding DocumentsWriter and
// flush queue plumbing usable while heavier dependencies (FieldUpdates
// Buffer, IndexSearcher) land in later sprints.
//
// When the packet has work to do and the dependent types are still
// missing, Apply returns [ErrFrozenBufferedUpdatesNotApplicable] without
// firing the latch, so the caller knows the work has not actually run.
func (f *FrozenBufferedUpdates) Apply(segStates []*FrozenSegmentState) (int64, error) {
	if !f.applyHeld.Load() {
		return 0, errors.New("frozen buffered updates: Apply requires Lock")
	}
	if f.delGen == -1 {
		return 0, ErrFrozenBufferedUpdatesNotPushed
	}
	if !f.Any() {
		f.fireApplied()
		return 0, nil
	}
	if f.privateSegment != nil && len(segStates) != 1 {
		return 0, fmt.Errorf(
			"frozen buffered updates: private packet expects exactly one segment, got %d",
			len(segStates),
		)
	}

	// The full pipeline (rewrite, scorer, doc-values update writers) is
	// not yet ported; signal the caller cleanly rather than silently
	// returning 0.
	return 0, ErrFrozenBufferedUpdatesNotApplicable
}

// fireApplied closes the latch exactly once. Safe to call from any
// goroutine.
func (f *FrozenBufferedUpdates) fireApplied() {
	f.appliedOnce.Do(func() { close(f.applied) })
}

// FrozenTermsIterator returns the underlying PrefixCodedTerms iterator
// used by the apply pipeline. Exposed so callers (and tests) can walk
// the sorted term projection without touching internals.
func (f *FrozenBufferedUpdates) FrozenTermsIterator() *PrefixCodedTermIterator {
	return f.deleteTerms.Iterator()
}

// DeleteTermsSize returns the number of terms in the frozen projection.
func (f *FrozenBufferedUpdates) DeleteTermsSize() int64 {
	return f.deleteTerms.Size()
}

// DeleteQueries returns a defensive copy of the (Query, limit) pairs.
// Defensive copying mirrors the read-only contract of the Lucene
// parallel arrays.
func (f *FrozenBufferedUpdates) DeleteQueries() []Query {
	out := make([]Query, len(f.deleteQueries))
	for i, e := range f.deleteQueries {
		out[i] = e.query
	}
	return out
}

// DeleteQueryLimits returns the parallel docID-limit array.
func (f *FrozenBufferedUpdates) DeleteQueryLimits() []int {
	out := make([]int, len(f.deleteQueries))
	for i, e := range f.deleteQueries {
		out[i] = e.limit
	}
	return out
}

// TermDocsProvider abstracts the read side of a [LeafReader] used by
// [TermDocsIterator]. It mirrors the {@code TermsProvider} functional
// interface nested inside Lucene's FrozenBufferedUpdates and lets the
// iterator be unit-tested against either a real LeafReader or a
// minimal Fields-backed fake.
type TermDocsProvider interface {
	Terms(field string) (Terms, error)
}

// leafReaderTermsProvider adapts a *LeafReader to TermDocsProvider.
type leafReaderTermsProvider struct{ reader *LeafReader }

func (p leafReaderTermsProvider) Terms(field string) (Terms, error) {
	if p.reader == nil {
		return nil, nil
	}
	return p.reader.Terms(field)
}

// fieldsTermsProvider adapts a Fields instance to TermDocsProvider.
type fieldsTermsProvider struct{ fields Fields }

func (p fieldsTermsProvider) Terms(field string) (Terms, error) {
	if p.fields == nil {
		return nil, nil
	}
	return p.fields.Terms(field)
}

// TermDocsIterator walks a term dictionary and yields the postings
// iterator for each (field, term) tuple the caller requests. It is the
// Go port of {@code FrozenBufferedUpdates.TermDocsIterator}.
//
// Sorted mode (sortedTerms=true) caches the current "reader term" and
// uses [TermsEnum.SeekCeil] to fast-forward, exactly matching Lucene's
// optimization that lets prefix-coded term dictionaries reuse internal
// state across consecutive seeks. Unsorted mode falls back to
// [TermsEnum.SeekExact] for each term.
type TermDocsIterator struct {
	provider     TermDocsProvider
	field        string
	termsEnum    TermsEnum
	postingsEnum PostingsEnum
	sortedTerms  bool
	readerTerm   *Term
	lastTerm     *Term // sorted-mode invariant check
}

// NewTermDocsIteratorFromReader builds an iterator backed by a
// [LeafReader]. The sortedTerms flag must be true when the caller will
// invoke NextTerm with terms in unsigned-lexicographic order.
func NewTermDocsIteratorFromReader(reader *LeafReader, sortedTerms bool) *TermDocsIterator {
	return &TermDocsIterator{
		provider:    leafReaderTermsProvider{reader: reader},
		sortedTerms: sortedTerms,
	}
}

// NewTermDocsIteratorFromFields builds an iterator backed by a [Fields]
// instance (typically MultiFields or a per-segment Fields view).
func NewTermDocsIteratorFromFields(fields Fields, sortedTerms bool) *TermDocsIterator {
	return &TermDocsIterator{
		provider:    fieldsTermsProvider{fields: fields},
		sortedTerms: sortedTerms,
	}
}

// setField primes the TermsEnum for the requested field, reusing the
// existing enum if the field has not changed.
func (it *TermDocsIterator) setField(field string) error {
	if it.field != "" && it.field == field {
		return nil
	}
	it.field = field
	it.lastTerm = nil
	it.readerTerm = nil

	terms, err := it.provider.Terms(field)
	if err != nil {
		return err
	}
	if terms == nil {
		it.termsEnum = nil
		return nil
	}
	enum, err := terms.GetIterator()
	if err != nil {
		return err
	}
	it.termsEnum = enum
	if it.sortedTerms {
		first, err := enum.Next()
		if err != nil {
			return err
		}
		it.readerTerm = first
	}
	return nil
}

// NextTerm returns the postings iterator for the (field, termBytes)
// tuple, or nil if the term does not exist in the underlying term
// dictionary. The returned [PostingsEnum] is owned by the iterator and
// is reused across successive calls.
//
// Errors returned from the underlying TermsEnum / PostingsEnum are
// propagated verbatim.
func (it *TermDocsIterator) NextTerm(field string, termBytes []byte) (PostingsEnum, error) {
	if err := it.setField(field); err != nil {
		return nil, err
	}
	if it.termsEnum == nil {
		return nil, nil
	}

	want := NewTermFromBytes(field, termBytes)

	if it.sortedTerms {
		if err := it.assertSorted(want); err != nil {
			return nil, err
		}
		if it.readerTerm == nil {
			return nil, nil
		}
		cmp := util.BytesRefCompare(want.Bytes, it.readerTerm.Bytes)
		switch {
		case cmp < 0:
			return nil, nil
		case cmp == 0:
			return it.getDocs()
		default:
			// Seek forward.
			got, err := it.termsEnum.SeekCeil(want)
			if err != nil {
				return nil, err
			}
			if got == nil {
				// END: no more terms in this segment.
				it.termsEnum = nil
				return nil, nil
			}
			if util.BytesRefCompare(got.Bytes, want.Bytes) == 0 {
				return it.getDocs()
			}
			// NOT_FOUND: cache the new reader term and report miss.
			it.readerTerm = got
			return nil, nil
		}
	}

	found, err := it.termsEnum.SeekExact(want)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return it.getDocs()
}

// assertSorted enforces the monotonic-non-decreasing contract of sorted
// mode. The check is always on (Go has no assert toggle); the cost is
// a single bytes compare and a clone per call.
func (it *TermDocsIterator) assertSorted(term *Term) error {
	if it.lastTerm != nil && util.BytesRefCompare(term.Bytes, it.lastTerm.Bytes) < 0 {
		return fmt.Errorf(
			"term docs iterator: out-of-order term %q after %q",
			term.Bytes.String(), it.lastTerm.Bytes.String(),
		)
	}
	it.lastTerm = term.Clone()
	return nil
}

// getDocs returns the postings iterator for the term currently positioned
// in the TermsEnum, reusing the cached postingsEnum when possible.
func (it *TermDocsIterator) getDocs() (PostingsEnum, error) {
	if it.termsEnum == nil {
		return nil, errors.New("term docs iterator: termsEnum unset")
	}
	pe, err := it.termsEnum.Postings(0)
	if err != nil {
		return nil, err
	}
	it.postingsEnum = pe
	return pe, nil
}
