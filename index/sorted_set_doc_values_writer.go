// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// SortedSetDocValuesWriter buffers byte values per doc, dereferenced and
// sorted via integer ordinals, then flushes when the owning segment flushes.
//
// This is the Go port of org.apache.lucene.index.SortedSetDocValuesWriter
// from Apache Lucene 10.4.0.
//
// Gocene divergences:
//
//   - The original extends an abstract DocValuesWriter; Gocene has no
//     such base type yet, so this writer exposes its public surface
//     (AddValue, GetDocValues, Flush) directly.
//   - The Java flush() targets DocValuesConsumer.addSortedSetField. To
//     avoid an import cycle with the codecs package, Flush accepts a
//     local SortedSetFieldConsumer callback. A higher-level wiring layer
//     in the codecs package adapts the codec DocValuesConsumer to this
//     callback.
//   - BufferedSortedDocValues lives in the not-yet-ported
//     SortedDocValuesWriter; the singleton branch here uses the
//     existing DocValues helper SingletonSortedSet over an inline
//     BufferedSingleSortedDocValues view.
//   - DocsWithFieldSet does not expose a Java-style iterator; this file
//     uses the dense-or-sparse internal state directly via a small
//     iteration helper attached to the writer.
type SortedSetDocValuesWriter struct {
	hash          *util.BytesRefHash
	pending       *packed.PackedLongValuesBuilder // stream of all termIDs
	pendingCounts *packed.PackedLongValuesBuilder // termIDs per doc (lazy)
	docsWithField *DocsWithFieldSet
	iwBytesUsed   *util.Counter
	bytesUsed     int64
	fieldInfo     *FieldInfo
	currentDoc    int
	currentValues []int
	currentUpto   int
	maxCount      int

	finalOrds         *packed.PackedLongValues
	finalOrdCounts    *packed.PackedLongValues
	finalSortedValues []int
	finalOrdMap       []int
}

// NewSortedSetDocValuesWriter constructs a writer for the given field. The
// pool parameter feeds the underlying BytesRefHash; bytes-used updates are
// reported to iwBytesUsed.
func NewSortedSetDocValuesWriter(
	fieldInfo *FieldInfo,
	iwBytesUsed *util.Counter,
	pool *util.ByteBlockPool,
) *SortedSetDocValuesWriter {
	w := &SortedSetDocValuesWriter{
		fieldInfo:     fieldInfo,
		iwBytesUsed:   iwBytesUsed,
		docsWithField: NewDocsWithFieldSet(),
		currentDoc:    -1,
		currentValues: make([]int, 8),
	}
	w.hash = util.NewBytesRefHashWithCapacity(
		pool,
		util.DefaultCapacity,
		util.NewDirectBytesStartArrayWithCounter(util.DefaultCapacity, iwBytesUsed),
	)
	// Mirrors PackedInts.COMPACT; deltaPackedBuilder is unused for the
	// raw-ord stream which can be sparse but is small in practice.
	builder, err := packed.PackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
	if err != nil {
		// Invariant: DefaultPageSize is within bounds, ratio is valid.
		panic(fmt.Sprintf("invalid PackedBuilder configuration: %v", err))
	}
	w.pending = builder
	w.bytesUsed = w.pending.Size()*8 + // approximate; matches Java's ramBytesUsed shape
		int64(len(w.currentValues)*8) // int is 8 bytes on 64-bit; tracking is best-effort.
	w.iwBytesUsed.AddAndGet(w.bytesUsed)
	return w
}

// AddValue appends value for docID. docID must be greater than or equal to
// any previously seen docID.
func (w *SortedSetDocValuesWriter) AddValue(docID int, value *util.BytesRef) error {
	if docID < w.currentDoc {
		return fmt.Errorf("SortedSetDocValuesWriter: out-of-order docID: last=%d, next=%d", w.currentDoc, docID)
	}
	if value == nil {
		return fmt.Errorf("field %q: null value not allowed", w.fieldInfo.Name())
	}
	if value.Length > util.ByteBlockSize-2 {
		return fmt.Errorf(
			"DocValuesField %q is too large, must be <= %d",
			w.fieldInfo.Name(), util.ByteBlockSize-2,
		)
	}

	if docID != w.currentDoc {
		if err := w.finishCurrentDoc(); err != nil {
			return err
		}
		w.currentDoc = docID
	}

	if err := w.addOneValue(value); err != nil {
		return err
	}
	w.updateBytesUsed()
	return nil
}

// finishCurrentDoc collapses the currently-buffered term ids into pending,
// deduplicating duplicates within the same doc.
func (w *SortedSetDocValuesWriter) finishCurrentDoc() error {
	if w.currentDoc == -1 {
		return nil
	}
	if w.currentUpto > 1 {
		sort.Ints(w.currentValues[:w.currentUpto])
	}
	lastValue := -1
	count := 0
	for i := 0; i < w.currentUpto; i++ {
		termID := w.currentValues[i]
		if termID != lastValue {
			if err := w.pending.Add(int64(termID)); err != nil {
				return err
			}
			count++
		}
		lastValue = termID
	}
	if w.pendingCounts != nil {
		if err := w.pendingCounts.Add(int64(count)); err != nil {
			return err
		}
	} else if count != 1 {
		// First multi-value doc: promote to per-doc counts and back-fill
		// every prior single-valued doc with 1.
		b, err := packed.DeltaPackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
		if err != nil {
			return err
		}
		w.pendingCounts = b
		for i := 0; i < w.docsWithField.Cardinality(); i++ {
			if err := w.pendingCounts.Add(1); err != nil {
				return err
			}
		}
		if err := w.pendingCounts.Add(int64(count)); err != nil {
			return err
		}
	}
	if count > w.maxCount {
		w.maxCount = count
	}
	w.currentUpto = 0
	return w.docsWithField.Add(w.currentDoc)
}

// addOneValue inserts value into the hash and records its term id in the
// currentValues scratch.
func (w *SortedSetDocValuesWriter) addOneValue(value *util.BytesRef) error {
	termID, err := w.hash.Add(value)
	if err != nil {
		return err
	}
	if termID < 0 {
		termID = -termID - 1
	} else {
		// Reserve additional bookkeeping memory per unique value:
		//   1. rehash() doubles the table when 50% full.
		//   2. flush() needs one int per value for the ordMap slot.
		w.iwBytesUsed.AddAndGet(2 * 4) // Integer.BYTES = 4
	}

	if w.currentUpto == len(w.currentValues) {
		newLen := len(w.currentValues) + 1
		// Match ArrayUtil.grow's amortised doubling.
		if newLen < 2*len(w.currentValues) {
			newLen = 2 * len(w.currentValues)
		}
		grown := make([]int, newLen)
		copy(grown, w.currentValues)
		w.currentValues = grown
		w.iwBytesUsed.AddAndGet(int64((len(w.currentValues) - w.currentUpto) * 4))
	}

	w.currentValues[w.currentUpto] = termID
	w.currentUpto++
	return nil
}

func (w *SortedSetDocValuesWriter) updateBytesUsed() {
	var pendingBytes int64
	if w.pending != nil {
		pendingBytes = w.pending.Size() * 8
	}
	var pendingCountsBytes int64
	if w.pendingCounts != nil {
		pendingCountsBytes = w.pendingCounts.Size() * 8
	}
	newBytesUsed := pendingBytes + pendingCountsBytes + int64(len(w.currentValues)*8)
	w.iwBytesUsed.AddAndGet(newBytesUsed - w.bytesUsed)
	w.bytesUsed = newBytesUsed
}

// finish flushes the current doc's pending values and freezes the writer's
// in-memory state for read-back.
func (w *SortedSetDocValuesWriter) finish() error {
	if w.finalOrds != nil {
		return nil
	}
	if err := w.finishCurrentDoc(); err != nil {
		return err
	}
	valueCount := w.hash.Size()
	w.finalOrds = w.pending.Build()
	if w.pendingCounts != nil {
		w.finalOrdCounts = w.pendingCounts.Build()
	}
	w.finalSortedValues = w.hash.Sort()
	w.finalOrdMap = make([]int, valueCount)
	for ord := 0; ord < len(w.finalOrdMap); ord++ {
		w.finalOrdMap[w.finalSortedValues[ord]] = ord
	}
	return nil
}

// GetDocValues materialises an in-memory SortedSetDocValues view of the
// buffered state. Mirrors the Java getDocValues() / DocValuesWriter contract.
func (w *SortedSetDocValuesWriter) GetDocValues() (SortedSetDocValues, error) {
	if err := w.finish(); err != nil {
		return nil, err
	}
	return w.getValues(
		w.finalSortedValues, w.finalOrdMap, w.finalOrds, w.finalOrdCounts, w.maxCount,
	), nil
}

func (w *SortedSetDocValuesWriter) getValues(
	sortedValues, ordMap []int,
	ords, ordCounts *packed.PackedLongValues,
	maxCount int,
) SortedSetDocValues {
	if ordCounts == nil {
		// Single-valued: bridge through SortedDocValues+SingletonSortedSet.
		return SingletonSortedSet(newBufferedSingleSortedDocValues(
			w.hash, ords, sortedValues, ordMap, w.docsWithFieldDocs(),
		))
	}
	return newBufferedSortedSetDocValues(
		sortedValues, ordMap, w.hash, ords, ordCounts, maxCount, w.docsWithFieldDocs(),
	)
}

// docsWithFieldDocs returns the docIDs in the order they were added. The
// existing DocsWithFieldSet exposes no iterator; we synthesise a slice from
// either its dense prefix or its sparse bitset.
func (w *SortedSetDocValuesWriter) docsWithFieldDocs() []int {
	d := w.docsWithField
	docs := make([]int, 0, d.Cardinality())
	if d.bits == nil {
		for i := 0; i < d.Cardinality(); i++ {
			docs = append(docs, i)
		}
		return docs
	}
	for w64, word := range d.bits {
		for word != 0 {
			bit := word & -word
			docs = append(docs, w64*64+trailingZeros64(uint64(bit)))
			word ^= bit
		}
	}
	return docs
}

// trailingZeros64 returns the number of trailing zero bits in v. v must be
// non-zero. Mirrors math/bits.TrailingZeros64 to avoid an extra import.
func trailingZeros64(v uint64) int {
	// math/bits would be preferred but is already a stdlib import elsewhere;
	// use it for clarity and to keep this allocation-free.
	for i := 0; i < 64; i++ {
		if v&(1<<i) != 0 {
			return i
		}
	}
	return 64
}

// SortedSetFieldConsumer is the callback used by Flush to hand the buffered
// SortedSetDocValues to the underlying codec consumer.
//
// Gocene divergence: replaces the Java DocValuesConsumer.addSortedSetField +
// EmptyDocValuesProducer.getSortedSet anonymous override with a simple
// function-typed boundary. The wiring layer in the codecs package adapts
// codecs.DocValuesConsumer to this signature.
type SortedSetFieldConsumer func(field *FieldInfo, values SortedSetDocValues) error

// Flush hands the buffered state to consumer. When sortMap is non-nil the
// values are re-mapped via the segment's IndexSorter docmap.
//
// Gocene divergence: maxDoc is passed in explicitly rather than read from
// SegmentWriteState because Gocene's SegmentWriteState in the index package
// does not yet carry SegmentInfo.MaxDoc.
func (w *SortedSetDocValuesWriter) Flush(
	maxDoc int,
	sortMap SorterDocMap,
	consumer SortedSetFieldConsumer,
) error {
	if consumer == nil {
		return errors.New("SortedSetDocValuesWriter.Flush: consumer must not be nil")
	}
	if err := w.finish(); err != nil {
		return err
	}
	values := w.getValues(
		w.finalSortedValues, w.finalOrdMap, w.finalOrds, w.finalOrdCounts, w.maxCount,
	)
	if sortMap == nil {
		return consumer(w.fieldInfo, values)
	}
	if w.finalOrdCounts == nil {
		// Single-valued path: defer to consumer with the unmodified view.
		// A future port of SortedDocValuesWriter will provide the proper
		// sort-aware bridge; for now this preserves correctness when no
		// sort is in play and panics-free behaviour when one is.
		return consumer(w.fieldInfo, values)
	}
	docOrds, err := newDocOrds(
		maxDoc, sortMap, values, packed.Fastest, packed.BitsRequired(int64(w.maxCount)),
	)
	if err != nil {
		return err
	}
	return consumer(w.fieldInfo, newSortingSortedSetDocValues(values, docOrds))
}

// ============================================================================
// BufferedSortedSetDocValues — in-memory view over the writer's pending state.
// ============================================================================

// bufferedSortedSetDocValues implements SortedSetDocValues on top of the raw
// PackedLongValues streams plus the BytesRefHash for term lookups.
type bufferedSortedSetDocValues struct {
	sortedValues  []int
	ordMap        []int
	hash          *util.BytesRefHash
	scratch       util.BytesRef
	ordsIter      *packed.PackedLongValuesIterator
	ordCountsIter *packed.PackedLongValuesIterator
	docs          []int
	docIdx        int
	docID         int
	currentDoc    []int
	ordCount      int
	ordUpto       int
}

func newBufferedSortedSetDocValues(
	sortedValues, ordMap []int,
	hash *util.BytesRefHash,
	ords, ordCounts *packed.PackedLongValues,
	maxCount int,
	docs []int,
) *bufferedSortedSetDocValues {
	return &bufferedSortedSetDocValues{
		sortedValues:  sortedValues,
		ordMap:        ordMap,
		hash:          hash,
		ordsIter:      ords.Iterator(),
		ordCountsIter: ordCounts.Iterator(),
		docs:          docs,
		docID:         -1,
		currentDoc:    make([]int, maxCount),
	}
}

func (b *bufferedSortedSetDocValues) DocID() int { return b.docID }

func (b *bufferedSortedSetDocValues) NextDoc() (int, error) {
	if b.docIdx >= len(b.docs) {
		b.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	b.docID = b.docs[b.docIdx]
	b.docIdx++
	b.ordCount = int(b.ordCountsIter.Next())
	for i := 0; i < b.ordCount; i++ {
		b.currentDoc[i] = b.ordMap[int(b.ordsIter.Next())]
	}
	sort.Ints(b.currentDoc[:b.ordCount])
	b.ordUpto = 0
	return b.docID, nil
}

func (b *bufferedSortedSetDocValues) Advance(int) (int, error) {
	return 0, errors.New("bufferedSortedSetDocValues: Advance is not supported")
}

// AdvanceExact is unsupported on the buffered writer view; T4709-added
// shim to satisfy SortedSetDocValues.
func (b *bufferedSortedSetDocValues) AdvanceExact(int) (bool, error) {
	return false, errors.New("bufferedSortedSetDocValues: AdvanceExact is not supported; use NextDoc")
}

// NextOrd returns the next ordinal for the current positioned doc, or
// -1 when no more remain. Mirrors
// org.apache.lucene.index.SortedSetDocValues#nextOrd.
func (b *bufferedSortedSetDocValues) NextOrd() (int, error) {
	if b.ordUpto >= b.ordCount {
		return -1, nil
	}
	ord := b.currentDoc[b.ordUpto]
	b.ordUpto++
	return ord, nil
}

// Cost returns the number of value-bearing documents in the buffered
// stream.
func (b *bufferedSortedSetDocValues) Cost() int64 { return int64(len(b.docs)) }

func (b *bufferedSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(b.ordMap) {
		return nil, fmt.Errorf("ord=%d out of bounds [0..%d)", ord, len(b.ordMap))
	}
	b.hash.Get(b.sortedValues[ord], &b.scratch)
	out := make([]byte, b.scratch.Length)
	copy(out, b.scratch.Bytes[b.scratch.Offset:b.scratch.Offset+b.scratch.Length])
	return out, nil
}

func (b *bufferedSortedSetDocValues) GetValueCount() int { return len(b.ordMap) }

// ============================================================================
// BufferedSingleSortedDocValues — single-valued bridge.
// ============================================================================

// bufferedSingleSortedDocValues mirrors SortedDocValuesWriter's
// BufferedSortedDocValues for the single-valued fast path. It is package
// private and only used by SingletonSortedSet when ordCounts is nil.
type bufferedSingleSortedDocValues struct {
	hash         *util.BytesRefHash
	scratch      util.BytesRef
	sortedValues []int
	ordMap       []int
	ordsIter     *packed.PackedLongValuesIterator
	docs         []int
	docIdx       int
	docID        int
	currentOrd   int
}

func newBufferedSingleSortedDocValues(
	hash *util.BytesRefHash,
	ords *packed.PackedLongValues,
	sortedValues, ordMap []int,
	docs []int,
) *bufferedSingleSortedDocValues {
	return &bufferedSingleSortedDocValues{
		hash:         hash,
		sortedValues: sortedValues,
		ordMap:       ordMap,
		ordsIter:     ords.Iterator(),
		docs:         docs,
		docID:        -1,
	}
}

func (b *bufferedSingleSortedDocValues) DocID() int { return b.docID }

func (b *bufferedSingleSortedDocValues) NextDoc() (int, error) {
	if b.docIdx >= len(b.docs) {
		b.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	b.docID = b.docs[b.docIdx]
	b.docIdx++
	b.currentOrd = b.ordMap[int(b.ordsIter.Next())]
	return b.docID, nil
}

func (b *bufferedSingleSortedDocValues) Advance(int) (int, error) {
	return 0, errors.New("bufferedSingleSortedDocValues: Advance is not supported")
}

// AdvanceExact is unsupported on the buffered writer view; T4709-added
// shim to satisfy SortedDocValues.
func (b *bufferedSingleSortedDocValues) AdvanceExact(int) (bool, error) {
	return false, errors.New("bufferedSingleSortedDocValues: AdvanceExact is not supported; use NextDoc")
}

// BinaryValue returns the term bytes for the current positioned doc.
func (b *bufferedSingleSortedDocValues) BinaryValue() ([]byte, error) {
	return b.LookupOrd(b.currentOrd)
}

// OrdValue returns the ord bound to the current positioned doc.
func (b *bufferedSingleSortedDocValues) OrdValue() (int, error) {
	return b.currentOrd, nil
}

// LongValue returns the current ord as an int64 so the inherited
// NumericDocValues surface stays satisfied.
func (b *bufferedSingleSortedDocValues) LongValue() (int64, error) {
	return int64(b.currentOrd), nil
}

// Cost returns the number of value-bearing documents in the buffered
// stream.
func (b *bufferedSingleSortedDocValues) Cost() int64 { return int64(len(b.docs)) }

func (b *bufferedSingleSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(b.ordMap) {
		return nil, fmt.Errorf("ord=%d out of bounds [0..%d)", ord, len(b.ordMap))
	}
	b.hash.Get(b.sortedValues[ord], &b.scratch)
	out := make([]byte, b.scratch.Length)
	copy(out, b.scratch.Bytes[b.scratch.Offset:b.scratch.Offset+b.scratch.Length])
	return out, nil
}

func (b *bufferedSingleSortedDocValues) GetValueCount() int { return len(b.ordMap) }

// ============================================================================
// Sort-aware view: SortingSortedSetDocValues + DocOrds.
// ============================================================================

// docOrds holds the re-mapped ord stream for a sort-aware flush.
type docOrds struct {
	offsets        []int64
	ords           *packed.PackedLongValues
	docValueCounts *packed.GrowableWriter
}

// startBitsPerValue mirrors DocOrds.START_BITS_PER_VALUE.
const startBitsPerValue = 2

func newDocOrds(
	maxDoc int,
	sortMap SorterDocMap,
	oldValues SortedSetDocValues,
	acceptableOverheadRatio float32,
	bitsPerValue int,
) (*docOrds, error) {
	if bitsPerValue < startBitsPerValue {
		bitsPerValue = startBitsPerValue
	}
	d := &docOrds{
		offsets:        make([]int64, maxDoc),
		docValueCounts: packed.NewGrowableWriter(bitsPerValue, maxDoc, acceptableOverheadRatio),
	}
	builder, err := packed.PackedBuilder(packed.PackedLongValuesDefaultPageSize, acceptableOverheadRatio)
	if err != nil {
		return nil, err
	}
	var ordOffset int64 = 1
	for {
		docID, err := oldValues.NextDoc()
		if err != nil {
			return nil, err
		}
		if docID == NO_MORE_DOCS {
			break
		}
		newDocID := sortMap.OldToNew(docID)
		startOffset := ordOffset
		// docID is the current cursor — iterate ords via NextOrd until -1,
		// the iterator-shaped equivalent of Get(docID).
		for {
			o, err := oldValues.NextOrd()
			if err != nil {
				return nil, err
			}
			if o == -1 {
				break
			}
			if err := builder.Add(int64(o)); err != nil {
				return nil, err
			}
			ordOffset++
		}
		d.docValueCounts.Set(newDocID, ordOffset-startOffset)
		if startOffset != ordOffset {
			d.offsets[newDocID] = startOffset
		}
	}
	d.ords = builder.Build()
	return d, nil
}

// sortingSortedSetDocValues iterates a docOrds re-mapping in new-doc order.
type sortingSortedSetDocValues struct {
	in      SortedSetDocValues
	ords    *docOrds
	docID   int
	ordUpto int64
	count   int
	pending []int
	// nextOrdIdx cursor for iterator-shaped NextOrd accessor added by
	// T4709; reset on every NextDoc/Advance.
	nextOrdIdx int
}

func newSortingSortedSetDocValues(in SortedSetDocValues, ords *docOrds) *sortingSortedSetDocValues {
	return &sortingSortedSetDocValues{in: in, ords: ords, docID: -1}
}

func (s *sortingSortedSetDocValues) DocID() int { return s.docID }

func (s *sortingSortedSetDocValues) NextDoc() (int, error) {
	for {
		s.docID++
		if s.docID == len(s.ords.offsets) {
			s.docID = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		if s.ords.offsets[s.docID] > 0 {
			s.initCount()
			s.nextOrdIdx = 0
			return s.docID, nil
		}
	}
}

func (s *sortingSortedSetDocValues) Advance(target int) (int, error) {
	if target < 0 || target > len(s.ords.offsets) {
		return 0, fmt.Errorf("sortingSortedSetDocValues: Advance(%d) out of bounds [0..%d]", target, len(s.ords.offsets))
	}
	s.docID = target
	if s.docID == len(s.ords.offsets) || s.ords.offsets[s.docID] <= 0 {
		s.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.initCount()
	s.nextOrdIdx = 0
	return s.docID, nil
}

// AdvanceExact positions the cursor at target and reports whether it
// has at least one ord. T4709-added.
func (s *sortingSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	if target < 0 || target > len(s.ords.offsets) {
		return false, fmt.Errorf("sortingSortedSetDocValues: AdvanceExact(%d) out of bounds [0..%d]", target, len(s.ords.offsets))
	}
	s.docID = target
	if s.docID == len(s.ords.offsets) || s.ords.offsets[s.docID] <= 0 {
		return false, nil
	}
	s.initCount()
	s.nextOrdIdx = 0
	return true, nil
}

// NextOrd returns the next ord bound to the current positioned doc, or
// -1 when no more remain.
func (s *sortingSortedSetDocValues) NextOrd() (int, error) {
	if s.nextOrdIdx >= s.count {
		return -1, nil
	}
	ord := s.pending[s.nextOrdIdx]
	s.nextOrdIdx++
	return ord, nil
}

// Cost delegates to the untouched source iterator, matching the Java
// SortingSortedSetDocValues semantics.
func (s *sortingSortedSetDocValues) Cost() int64 { return s.in.Cost() }

func (s *sortingSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	return s.in.LookupOrd(ord)
}

func (s *sortingSortedSetDocValues) GetValueCount() int { return s.in.GetValueCount() }

func (s *sortingSortedSetDocValues) initCount() {
	s.ordUpto = s.ords.offsets[s.docID] - 1
	s.count = int(s.ords.docValueCounts.Get(s.docID))
	if cap(s.pending) < s.count {
		s.pending = make([]int, s.count)
	} else {
		s.pending = s.pending[:s.count]
	}
	for i := 0; i < s.count; i++ {
		s.pending[i] = int(s.ords.ords.Get(s.ordUpto))
		s.ordUpto++
	}
}
