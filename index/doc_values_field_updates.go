// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Port note:
//
// This file is the Go port of
// lucene/core/src/java/org/apache/lucene/index/DocValuesFieldUpdates.java
// (Apache Lucene 10.4.0). The original is a package-private abstract
// class; the Go port exposes the contract as the
// [DocValuesFieldUpdatesIterator] interface plus the
// [BaseDocValuesFieldUpdates] struct that holds the shared state and
// drives finish/sort. Concrete subtypes (currently
// [BinaryDocValuesFieldUpdates]) embed the base and supply the
// subtype-specific Swap/Grow/Resize hooks via the
// [BaseDocValuesFieldUpdates.HookSwap], HookGrow and HookResize
// fields, mirroring Java's virtual dispatch.
//
// Naming: the Lucene "DocValuesFieldUpdates.Iterator" is renamed
// [DocValuesFieldUpdatesIterator] in Go to avoid colliding with the
// existing per-package "Iterator" names.

// docValuesFieldUpdatesPageSize is the page size used for the docs
// PagedMutable. Mirrors the Java PAGE_SIZE constant (1024).
const docValuesFieldUpdatesPageSize = 1024

// docValuesFieldUpdatesShift is the number of low bits in each docs
// entry reserved for the hasValue mask. Java uses 1 (SHIFT).
const docValuesFieldUpdatesShift = 1

// docValuesFieldUpdatesHasValueMask marks an entry as carrying a real
// value (Java HAS_VALUE_MASK = 1).
const docValuesFieldUpdatesHasValueMask uint64 = 1

// docValuesFieldUpdatesHasNoValueMask marks a reset entry that
// explicitly clears the doc (Java HAS_NO_VALUE_MASK = 0).
const docValuesFieldUpdatesHasNoValueMask uint64 = 0

// DocValuesFieldUpdatesIterator iterates over updated documents and
// their values within a single packet, in increasing doc-id order.
// Mirrors the package-private nested class
// {@code DocValuesFieldUpdates.Iterator} in Lucene; only documents
// with updates (or explicit resets) are visited.
//
// NextDoc, DocID, LongValue, BinaryValue, DelGen and HasValue follow
// the Java contract exactly. The Advance/AdvanceExact/Cost trio is
// intentionally omitted because the Java parent throws
// UnsupportedOperationException for them.
type DocValuesFieldUpdatesIterator interface {
	// NextDoc returns the next doc id with an update or
	// util.NO_MORE_DOCS when the packet is exhausted.
	NextDoc() int

	// DocID returns the current doc id, or -1 before NextDoc has
	// been called, or util.NO_MORE_DOCS once exhausted.
	DocID() int

	// LongValue returns the long value for the current doc when the
	// underlying packet stores numeric updates.
	LongValue() int64

	// BinaryValue returns the binary value for the current doc when
	// the underlying packet stores binary updates. The returned ref
	// is owned by the iterator and must not be retained across
	// NextDoc calls.
	BinaryValue() *util.BytesRef

	// DelGen returns the delete generation that identifies the
	// packet this iterator belongs to.
	DelGen() int64

	// HasValue reports whether the current entry carries a value
	// (true) or is a reset (false).
	HasValue() bool
}

// BaseDocValuesFieldUpdates holds the shared state and behaviour of a
// per-(field, delGen) update packet. Concrete subtypes embed it and
// wire the HookSwap / HookGrow / HookResize callbacks to extend the
// in-place storage maintenance with their own auxiliary arrays.
//
// The struct is safe for concurrent use through its own mutex, which
// guards Add, Reset, Size, Any and Finish, mirroring the Java
// {@code synchronized} qualifiers. Read paths reached through an
// [DocValuesFieldUpdatesIterator] are not synchronised: callers must
// have called [BaseDocValuesFieldUpdates.Finish] before iterating, as
// in Lucene.
type BaseDocValuesFieldUpdates struct {
	// mu guards every field below it. Acquired by Add, Reset, Size,
	// Any and Finish.
	mu sync.Mutex

	maxDoc       int
	delGen       int64
	field        string
	dvType       DocValuesType
	bitsPerValue int
	finished     bool

	// Docs packs (docID<<1)|hasValueMask entries. Exported via the
	// Docs accessor so subtypes (which embed this struct) can iterate
	// the same backing storage from their own hooks.
	Docs *packed.AbstractPagedMutable

	// Size counts the entries currently stored. Updated by Add and
	// shrunk by Finish through the Resize hook.
	Size int

	// HookSwap, HookGrow, HookResize allow the embedding type to
	// extend the in-place maintenance. The base implementation
	// (baseSwap, baseGrow, baseResize) is always called either by
	// the hook (when the subtype overrides it) or directly when the
	// hook is nil. Set them from the concrete type's constructor.
	HookSwap   func(i, j int)
	HookGrow   func(size int)
	HookResize func(size int)
}

// InitBaseDocValuesFieldUpdates initialises an already-allocated
// BaseDocValuesFieldUpdates with the validated parameters. It is
// called by concrete subtype constructors right after the embedding
// struct has been allocated; the subtype then installs its hooks.
func InitBaseDocValuesFieldUpdates(b *BaseDocValuesFieldUpdates, maxDoc int, delGen int64, field string, dvType DocValuesType) error {
	if b == nil {
		return fmt.Errorf("doc values field updates: nil base")
	}
	if dvType == DocValuesTypeNone {
		return fmt.Errorf("doc values field updates: DocValuesType must not be DocValuesTypeNone")
	}
	bitsPerValue := packed.BitsRequired(int64(maxDoc-1)) + docValuesFieldUpdatesShift
	docs, err := packed.NewPagedMutable(1, docValuesFieldUpdatesPageSize, bitsPerValue, packed.Default)
	if err != nil {
		return fmt.Errorf("doc values field updates: init docs: %w", err)
	}
	b.maxDoc = maxDoc
	b.delGen = delGen
	b.field = field
	b.dvType = dvType
	b.bitsPerValue = bitsPerValue
	b.finished = false
	b.Docs = docs.AbstractPagedMutable
	b.Size = 0
	return nil
}

// Field returns the field name this packet targets.
func (b *BaseDocValuesFieldUpdates) Field() string { return b.field }

// Type returns the doc values type this packet updates.
func (b *BaseDocValuesFieldUpdates) Type() DocValuesType { return b.dvType }

// DelGen returns the delete generation this packet is bound to.
func (b *BaseDocValuesFieldUpdates) DelGen() int64 { return b.delGen }

// MaxDoc returns the segment-wide maxDoc this packet was created for.
func (b *BaseDocValuesFieldUpdates) MaxDoc() int { return b.maxDoc }

// GetFinished reports whether [BaseDocValuesFieldUpdates.Finish] has
// been called.
func (b *BaseDocValuesFieldUpdates) GetFinished() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.finished
}

// SizeLocked returns the number of stored entries. Mirrors Lucene's
// {@code DocValuesFieldUpdates#size()}.
func (b *BaseDocValuesFieldUpdates) SizeLocked() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Size
}

// Any reports whether the packet has any updates. Mirrors
// {@code DocValuesFieldUpdates#any()}.
func (b *BaseDocValuesFieldUpdates) Any() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Size > 0
}

// Reset records an explicit "clear this doc" update. Mirrors
// {@code DocValuesFieldUpdates#reset(int)}.
func (b *BaseDocValuesFieldUpdates) Reset(doc int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.addInternalLocked(doc, docValuesFieldUpdatesHasNoValueMask)
	return err
}

// AddDoc reserves a new slot for the given doc id and returns the
// slot index. Subtypes call this from their own Add(...) methods
// before recording the per-entry value in their auxiliary arrays.
// Mirrors {@code DocValuesFieldUpdates#add(int)}.
func (b *BaseDocValuesFieldUpdates) AddDoc(doc int) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.addInternalLocked(doc, docValuesFieldUpdatesHasValueMask)
}

// addInternalLocked is the workhorse for both AddDoc and Reset. The
// caller must hold b.mu.
func (b *BaseDocValuesFieldUpdates) addInternalLocked(doc int, hasValueMask uint64) (int, error) {
	if b.finished {
		return 0, fmt.Errorf("doc values field updates: already finished")
	}
	if doc < 0 || doc >= b.maxDoc {
		return 0, fmt.Errorf("doc values field updates: doc %d out of range [0,%d)", doc, b.maxDoc)
	}
	// TODO: if the Sorter interface changes to take long indexes, we
	// can remove that limitation.
	const maxInt32 = int(^uint32(0) >> 1)
	if b.Size == maxInt32 {
		return 0, fmt.Errorf("doc values field updates: cannot support more than %d doc/value entries", maxInt32)
	}
	if int64(b.Size) == b.Docs.Size() {
		b.growLocked(b.Size + 1)
	}
	packed := (uint64(doc) << docValuesFieldUpdatesShift) | hasValueMask
	b.Docs.Set(int64(b.Size), int64(packed))
	b.Size++
	return b.Size - 1, nil
}

// growLocked dispatches to HookGrow when set, otherwise calls the
// base implementation directly. The caller must hold b.mu.
func (b *BaseDocValuesFieldUpdates) growLocked(size int) {
	if b.HookGrow != nil {
		b.HookGrow(size)
		return
	}
	b.GrowBase(size)
}

// GrowBase grows the docs storage to fit at least size entries. The
// concrete type's HookGrow MUST call GrowBase (in addition to growing
// its own auxiliary arrays) so the base storage stays in sync.
func (b *BaseDocValuesFieldUpdates) GrowBase(size int) {
	b.Docs = b.Docs.Grow(int64(size))
}

// ResizeBase trims (or expands) the docs storage to exactly size
// entries. The concrete type's HookResize MUST call ResizeBase.
func (b *BaseDocValuesFieldUpdates) ResizeBase(size int) {
	b.Docs = b.Docs.Resize(int64(size))
}

// SwapBase exchanges the docs entries at i and j. The concrete
// type's HookSwap MUST call SwapBase first, then swap its own
// auxiliary arrays.
func (b *BaseDocValuesFieldUpdates) SwapBase(i, j int) {
	tmp := b.Docs.Get(int64(j))
	b.Docs.Set(int64(j), b.Docs.Get(int64(i)))
	b.Docs.Set(int64(i), tmp)
}

// swapLocked dispatches to HookSwap when set, otherwise calls the
// base implementation directly. The caller must hold b.mu through
// the surrounding sort.
func (b *BaseDocValuesFieldUpdates) swapLocked(i, j int) {
	if b.HookSwap != nil {
		b.HookSwap(i, j)
		return
	}
	b.SwapBase(i, j)
}

// EnsureFinished panics with a stable message when the packet has
// not yet been finished. Subtypes call this from Iterator() to mirror
// Java's {@code ensureFinished()} contract.
func (b *BaseDocValuesFieldUpdates) EnsureFinished() {
	b.mu.Lock()
	finished := b.finished
	b.mu.Unlock()
	if !finished {
		panic("doc values field updates: call finish first")
	}
}

// Finish freezes the packet and sorts entries by doc id (stable,
// last-write-wins for ties). Mirrors {@code DocValuesFieldUpdates#finish()}.
//
// The sort is delegated to an IntroSorter driven by a side
// "ords" PackedInts.Mutable used to stabilise it, matching the Java
// implementation note inside finish().
func (b *BaseDocValuesFieldUpdates) Finish() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.finished {
		return fmt.Errorf("doc values field updates: already finished")
	}
	b.finished = true
	if int64(b.Size) < b.Docs.Size() {
		if b.HookResize != nil {
			b.HookResize(b.Size)
		} else {
			b.ResizeBase(b.Size)
		}
	}
	if b.Size == 0 {
		return nil
	}
	// We need a stable sort but InPlaceMergeSorter performs lots of
	// swaps which hurts performance due to all the packed ints we are
	// using. Another option would be TimSorter, but it needs
	// additional API (copy to temp storage, compare with item in temp
	// storage, etc.) so we instead use quicksort and record ords of
	// each update to guarantee stability.
	ordsBits := packed.BitsRequired(int64(b.Size - 1))
	if ordsBits < 1 {
		ordsBits = 1
	}
	ords := packed.GetMutable(b.Size, ordsBits, packed.Default)
	for i := 0; i < b.Size; i++ {
		ords.Set(i, int64(i))
	}
	adapter := &docValuesFieldUpdatesSort{
		base: b,
		ords: ords,
	}
	util.NewIntroSorter(adapter).Sort(0, b.Size)
	return nil
}

// RamBytesUsedBase reports the shallow accounting common to every
// subtype. Concrete types add their own auxiliary footprint on top.
//
// The Java reference adds NUM_BYTES_OBJECT_HEADER + 2*Integer.BYTES
// + 2 + Long.BYTES + NUM_BYTES_OBJECT_REF. The Go port keeps the
// same shape with NumBytesObjectRef from util/array_util.go and a
// fixed 16-byte header proxy; the result is a best-effort number
// consistent with Lucene "approximate" semantics, not a byte-perfect
// match (see util/ram_usage_estimator.go for the rationale).
func (b *BaseDocValuesFieldUpdates) RamBytesUsedBase() int64 {
	const objectHeader = 16
	const intBytes = 4
	const longBytes = 8
	return b.Docs.RamBytesUsed() +
		int64(objectHeader) +
		2*int64(intBytes) +
		2 +
		int64(longBytes) +
		int64(util.NumBytesObjectRef)
}

// DocsAt returns the packed docs entry at index i. Subtypes use it
// from their iterators when iterating the base storage.
func (b *BaseDocValuesFieldUpdates) DocsAt(i int64) int64 { return b.Docs.Get(i) }

// docValuesFieldUpdatesSort drives the IntroSorter that finish() runs
// over the docs+ords pair.
type docValuesFieldUpdatesSort struct {
	base     *BaseDocValuesFieldUpdates
	ords     packed.Mutable
	pivotDoc int64
	pivotOrd int
}

// Compare orders by (docID >>> 1, ord) so equal doc ids keep their
// insertion order (last-write-wins).
func (s *docValuesFieldUpdatesSort) Compare(i, j int) int {
	a := uint64(s.base.Docs.Get(int64(i))) >> docValuesFieldUpdatesShift
	b := uint64(s.base.Docs.Get(int64(j))) >> docValuesFieldUpdatesShift
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return int(s.ords.Get(i) - s.ords.Get(j))
}

// Swap swaps both the docs entries (via the subtype-aware hook) and
// the ords entries that stabilise the sort.
func (s *docValuesFieldUpdatesSort) Swap(i, j int) {
	tmpOrd := s.ords.Get(i)
	s.ords.Set(i, s.ords.Get(j))
	s.ords.Set(j, tmpOrd)
	s.base.swapLocked(i, j)
}

// Sort is unused: IntroSorter.Sort drives the algorithm.
func (s *docValuesFieldUpdatesSort) Sort(from, to int) {}

// SetPivot snapshots the current entry as the pivot.
func (s *docValuesFieldUpdatesSort) SetPivot(i int) {
	s.pivotDoc = int64(uint64(s.base.Docs.Get(int64(i))) >> docValuesFieldUpdatesShift)
	s.pivotOrd = int(s.ords.Get(i))
}

// ComparePivot compares the snapshotted pivot to the entry at j.
func (s *docValuesFieldUpdatesSort) ComparePivot(j int) int {
	other := int64(uint64(s.base.Docs.Get(int64(j))) >> docValuesFieldUpdatesShift)
	switch {
	case s.pivotDoc < other:
		return -1
	case s.pivotDoc > other:
		return 1
	}
	return s.pivotOrd - int(s.ords.Get(j))
}

// BaseDocValuesFieldUpdatesIterator is the shared portion of the
// concrete iterators returned by binary / numeric subtypes. It walks
// the packed docs entries in order, collapses consecutive entries
// that share the same doc id (so the last update wins), and calls
// the subtype-supplied SetIdx hook before exposing the value via
// DocID/HasValue/DelGen.
//
// Mirrors the Java {@code DocValuesFieldUpdates.AbstractIterator}.
type BaseDocValuesFieldUpdatesIterator struct {
	size     int
	docs     *packed.AbstractPagedMutable
	idx      int64
	doc      int
	delGen   int64
	hasValue bool

	// SetIdx is invoked when NextDoc has identified the entry whose
	// value should be exposed. It receives the docs-array index
	// (idx-1 in Java parlance) so the subtype can refresh its
	// per-entry offset/length/long cache.
	SetIdx func(idx int64)
}

// InitBaseDocValuesFieldUpdatesIterator wires the iterator state.
// Subtypes call it from their constructors and then assign SetIdx.
func InitBaseDocValuesFieldUpdatesIterator(it *BaseDocValuesFieldUpdatesIterator, size int, docs *packed.AbstractPagedMutable, delGen int64) {
	it.size = size
	it.docs = docs
	it.idx = 0
	it.doc = -1
	it.delGen = delGen
	it.hasValue = false
}

// NextDoc advances to the next doc id with an update, collapsing
// consecutive entries that share the same doc id (last write wins).
// Mirrors {@code AbstractIterator#nextDoc()}.
func (it *BaseDocValuesFieldUpdatesIterator) NextDoc() int {
	if it.idx >= int64(it.size) {
		it.doc = util.NO_MORE_DOCS
		return it.doc
	}
	longDoc := uint64(it.docs.Get(it.idx))
	it.idx++
	for it.idx < int64(it.size) {
		nextLongDoc := uint64(it.docs.Get(it.idx))
		if (longDoc >> docValuesFieldUpdatesShift) != (nextLongDoc >> docValuesFieldUpdatesShift) {
			break
		}
		longDoc = nextLongDoc
		it.idx++
	}
	it.hasValue = (longDoc & docValuesFieldUpdatesHasValueMask) > 0
	if it.hasValue && it.SetIdx != nil {
		it.SetIdx(it.idx - 1)
	}
	it.doc = int(longDoc >> docValuesFieldUpdatesShift)
	return it.doc
}

// DocID returns the current doc id.
func (it *BaseDocValuesFieldUpdatesIterator) DocID() int { return it.doc }

// DelGen returns the packet's delete generation.
func (it *BaseDocValuesFieldUpdatesIterator) DelGen() int64 { return it.delGen }

// HasValue reports whether the current entry carries a value.
func (it *BaseDocValuesFieldUpdatesIterator) HasValue() bool { return it.hasValue }

// MergedDocValuesFieldUpdatesIterator merge-sorts multiple iterators
// (one per delGen) and, when several iterators expose the same doc
// id, surfaces the value from the iterator with the largest delGen.
// Mirrors {@code DocValuesFieldUpdates#mergedIterator(Iterator[])}.
//
// Returns nil when no input iterator has any documents.
func MergedDocValuesFieldUpdatesIterator(subs []DocValuesFieldUpdatesIterator) DocValuesFieldUpdatesIterator {
	if len(subs) == 0 {
		return nil
	}
	if len(subs) == 1 {
		return subs[0]
	}
	pq, err := util.NewPriorityQueue(len(subs), func(a, b DocValuesFieldUpdatesIterator) bool {
		// Sort by smaller doc id; on ties, by larger delGen. The
		// Java assertion that delGens are unique across subs is
		// preserved as a sanity check in dev builds via a panic
		// here would be too strict; we rely on the caller honouring
		// the contract instead.
		ai, bi := a.DocID(), b.DocID()
		if ai != bi {
			return ai < bi
		}
		return b.DelGen() < a.DelGen()
	})
	if err != nil {
		// Only fails on negative size, which we ruled out above.
		panic(fmt.Sprintf("doc values field updates: priority queue: %v", err))
	}
	for _, sub := range subs {
		if sub.NextDoc() != util.NO_MORE_DOCS {
			pq.Add(sub)
		}
	}
	if pq.Size() == 0 {
		return nil
	}
	return &mergedDocValuesFieldUpdatesIterator{queue: pq, doc: -1}
}

// mergedDocValuesFieldUpdatesIterator implements the merged view
// produced by MergedDocValuesFieldUpdatesIterator.
type mergedDocValuesFieldUpdatesIterator struct {
	queue *util.PriorityQueue[DocValuesFieldUpdatesIterator]
	doc   int
}

// NextDoc advances every sub-iterator past the current doc id and
// returns the smallest doc id still pending across all subs.
func (m *mergedDocValuesFieldUpdatesIterator) NextDoc() int {
	for {
		if m.queue.Size() == 0 {
			m.doc = util.NO_MORE_DOCS
			return m.doc
		}
		newDoc := m.queue.Top().DocID()
		if newDoc != m.doc {
			if newDoc < m.doc {
				panic(fmt.Sprintf("doc values field updates: merged iterator went backwards (doc=%d newDoc=%d)", m.doc, newDoc))
			}
			m.doc = newDoc
			return m.doc
		}
		if m.queue.Top().NextDoc() == util.NO_MORE_DOCS {
			m.queue.Pop()
		} else {
			m.queue.UpdateTop()
		}
	}
}

// DocID returns the current doc id.
func (m *mergedDocValuesFieldUpdatesIterator) DocID() int { return m.doc }

// LongValue forwards to the current top iterator.
func (m *mergedDocValuesFieldUpdatesIterator) LongValue() int64 {
	return m.queue.Top().LongValue()
}

// BinaryValue forwards to the current top iterator.
func (m *mergedDocValuesFieldUpdatesIterator) BinaryValue() *util.BytesRef {
	return m.queue.Top().BinaryValue()
}

// DelGen panics: callers must not query the merged iterator's
// delGen. Mirrors the Java behaviour which throws
// UnsupportedOperationException for the same reason.
func (m *mergedDocValuesFieldUpdatesIterator) DelGen() int64 {
	panic("doc values field updates: merged iterator has no delGen")
}

// HasValue forwards to the current top iterator.
func (m *mergedDocValuesFieldUpdatesIterator) HasValue() bool {
	return m.queue.Top().HasValue()
}

// AsBinaryDocValues adapts a DocValuesFieldUpdatesIterator into the
// segment-side [BinaryDocValues] interface. Mirrors the static
// {@code Iterator#asBinaryDocValues}.
func AsBinaryDocValues(it DocValuesFieldUpdatesIterator) BinaryDocValues {
	return &updatesAsBinaryDV{it: it}
}

// AsNumericDocValues adapts a DocValuesFieldUpdatesIterator into the
// segment-side [NumericDocValues] interface. Mirrors the static
// {@code Iterator#asNumericDocValues}.
func AsNumericDocValues(it DocValuesFieldUpdatesIterator) NumericDocValues {
	return &updatesAsNumericDV{it: it}
}

// updatesAsBinaryDV adapts the iterator to BinaryDocValues. Get is
// best-effort: only the most recently visited doc can be served,
// matching the limited surface Lucene exposes via this wrapper.
type updatesAsBinaryDV struct {
	it DocValuesFieldUpdatesIterator
}

func (a *updatesAsBinaryDV) Get(docID int) ([]byte, error) {
	if a.it.DocID() == docID {
		v := a.it.BinaryValue()
		if v == nil {
			return nil, nil
		}
		return v.Bytes[v.Offset : v.Offset+v.Length], nil
	}
	return nil, fmt.Errorf("doc values field updates: binary doc values wrapper does not support random Get; docID=%d", docID)
}

func (a *updatesAsBinaryDV) Advance(target int) (int, error) {
	for {
		next := a.it.NextDoc()
		if next == util.NO_MORE_DOCS || next >= target {
			return next, nil
		}
	}
}

func (a *updatesAsBinaryDV) NextDoc() (int, error) {
	return a.it.NextDoc(), nil
}

func (a *updatesAsBinaryDV) DocID() int { return a.it.DocID() }

// updatesAsNumericDV adapts the iterator to NumericDocValues, with
// the same caveats as updatesAsBinaryDV.
type updatesAsNumericDV struct {
	it DocValuesFieldUpdatesIterator
}

func (a *updatesAsNumericDV) Get(docID int) (int64, error) {
	if a.it.DocID() == docID {
		return a.it.LongValue(), nil
	}
	return 0, fmt.Errorf("doc values field updates: numeric doc values wrapper does not support random Get; docID=%d", docID)
}

func (a *updatesAsNumericDV) Advance(target int) (int, error) {
	for {
		next := a.it.NextDoc()
		if next == util.NO_MORE_DOCS || next >= target {
			return next, nil
		}
	}
}

func (a *updatesAsNumericDV) NextDoc() (int, error) {
	return a.it.NextDoc(), nil
}

func (a *updatesAsNumericDV) DocID() int { return a.it.DocID() }
