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

// SortedNumericDocValuesWriter buffers up to N long values per doc, sorts
// them ascending within each doc, then flushes when the owning segment
// flushes.
//
// This is the Go port of org.apache.lucene.index.SortedNumericDocValuesWriter
// from Apache Lucene 10.4.0.
//
// Gocene divergences:
//
//   - The Java original extends an abstract DocValuesWriter<SortedNumericDocValues>;
//     Gocene has no such base type yet, so this writer exposes its public
//     surface (AddValue, GetDocValues, Flush) directly. Same pattern as
//     [SortedDocValuesWriter] / [SortedSetDocValuesWriter].
//   - Java's flush() targets DocValuesConsumer.addSortedNumericField via an
//     EmptyDocValuesProducer anonymous subclass. To avoid an import cycle
//     with the codecs package this writer takes a local
//     SortedNumericFieldConsumer callback; the codec wiring layer adapts
//     codecs.DocValuesConsumer to this signature.
//   - The sort-aware singleton branch in Java reaches into the not-yet-ported
//     NumericDocValuesWriter.getDocValuesProducer helper to wrap the single
//     long stream as a NumericDocValues, then re-wraps that as a
//     SortedNumericDocValues via DocValues.singleton. Until
//     NumericDocValuesWriter lands, Gocene returns the unsorted buffered
//     view through DocValues.Singleton in this branch as well (same
//     compromise made by SortedSetDocValuesWriter.Flush for its singleton
//     sort path); the docmap is honoured for the multi-valued branch.
//   - DocsWithFieldSet does not expose a Java-style iterator; the writer
//     reuses the dense-or-sparse traversal helper shared with
//     SortedSetDocValuesWriter (docsWithFieldDocs / trailingZeros64).
//   - Gocene's SortedNumericDocValues uses slice-based Get(docID) instead of
//     the Java docValueCount + nextValue iterator pair, so the buffered view
//     materialises each doc's values into a small reusable scratch slice.
type SortedNumericDocValuesWriter struct {
	pending       *packed.PackedLongValuesBuilder // stream of all values
	pendingCounts *packed.PackedLongValuesBuilder // count of values per doc (lazy)
	docsWithField *DocsWithFieldSet
	iwBytesUsed   *util.Counter
	bytesUsed     int64
	fieldInfo     *FieldInfo
	currentDoc    int
	currentValues []int64
	currentUpto   int

	finalValues      *packed.PackedLongValues
	finalValuesCount *packed.PackedLongValues
}

// NewSortedNumericDocValuesWriter constructs a writer for the given field.
// bytes-used updates are reported to iwBytesUsed.
func NewSortedNumericDocValuesWriter(
	fieldInfo *FieldInfo,
	iwBytesUsed *util.Counter,
) *SortedNumericDocValuesWriter {
	w := &SortedNumericDocValuesWriter{
		fieldInfo:     fieldInfo,
		iwBytesUsed:   iwBytesUsed,
		docsWithField: NewDocsWithFieldSet(),
		currentDoc:    -1,
		currentValues: make([]int64, 8),
	}
	// Mirrors PackedInts.COMPACT (deltaPackedBuilder).
	builder, err := packed.DeltaPackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
	if err != nil {
		// Invariant: DefaultPageSize is within bounds, ratio is valid.
		panic(fmt.Sprintf("invalid DeltaPackedBuilder configuration: %v", err))
	}
	w.pending = builder
	// Best-effort byte accounting; Java uses RamUsageEstimator.sizeOf(currentValues)
	// + pending.ramBytesUsed() + docsWithField.ramBytesUsed(). We approximate
	// to keep the counter monotonic during AddValue without pulling in the
	// full RamUsageEstimator port.
	w.bytesUsed = w.pending.Size()*8 + int64(len(w.currentValues)*8)
	w.iwBytesUsed.AddAndGet(w.bytesUsed)
	return w
}

// AddValue appends value for docID. docID must be greater than or equal to
// any previously seen docID; multiple values for the same docID are allowed
// and will be sorted ascending on doc finish (preserving duplicates).
func (w *SortedNumericDocValuesWriter) AddValue(docID int, value int64) error {
	if docID < w.currentDoc {
		return fmt.Errorf("SortedNumericDocValuesWriter: out-of-order docID: last=%d, next=%d", w.currentDoc, docID)
	}
	if docID != w.currentDoc {
		if err := w.finishCurrentDoc(); err != nil {
			return err
		}
		w.currentDoc = docID
	}
	w.addOneValue(value)
	return w.updateBytesUsed()
}

// finishCurrentDoc sorts the values for the current doc and flushes them
// into pending, recording the per-doc count when more than one doc has
// been observed with a count != 1.
func (w *SortedNumericDocValuesWriter) finishCurrentDoc() error {
	if w.currentDoc == -1 {
		return nil
	}
	if w.currentUpto > 1 {
		// Java uses Arrays.sort(currentValues, 0, currentUpto); the in-doc
		// slice is small and the values are unconstrained int64s, so the
		// stdlib slices/sort algorithm is fine here.
		sort.Slice(w.currentValues[:w.currentUpto], func(i, j int) bool {
			return w.currentValues[i] < w.currentValues[j]
		})
	}
	for i := 0; i < w.currentUpto; i++ {
		if err := w.pending.Add(w.currentValues[i]); err != nil {
			return err
		}
	}
	// Record the number of values for this doc.
	if w.pendingCounts != nil {
		if err := w.pendingCounts.Add(int64(w.currentUpto)); err != nil {
			return err
		}
	} else if w.currentUpto != 1 {
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
		if err := w.pendingCounts.Add(int64(w.currentUpto)); err != nil {
			return err
		}
	}
	w.currentUpto = 0
	return w.docsWithField.Add(w.currentDoc)
}

// addOneValue stores value in the per-doc scratch, growing it as needed.
func (w *SortedNumericDocValuesWriter) addOneValue(value int64) {
	if w.currentUpto == len(w.currentValues) {
		newLen := len(w.currentValues) + 1
		// Match ArrayUtil.grow's amortised doubling shape.
		if newLen < 2*len(w.currentValues) {
			newLen = 2 * len(w.currentValues)
		}
		grown := make([]int64, newLen)
		copy(grown, w.currentValues)
		w.currentValues = grown
	}
	w.currentValues[w.currentUpto] = value
	w.currentUpto++
}

// updateBytesUsed reconciles the iwBytesUsed counter with the writer's
// current memory footprint.
func (w *SortedNumericDocValuesWriter) updateBytesUsed() error {
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
	return nil
}

// finish freezes the writer's in-memory state for read-back. Idempotent.
func (w *SortedNumericDocValuesWriter) finish() error {
	if w.finalValues != nil {
		return nil
	}
	if err := w.finishCurrentDoc(); err != nil {
		return err
	}
	w.finalValues = w.pending.Build()
	if w.pendingCounts != nil {
		w.finalValuesCount = w.pendingCounts.Build()
	}
	return nil
}

// GetDocValues materialises an in-memory SortedNumericDocValues view of
// the buffered state. Mirrors the Java getDocValues() / DocValuesWriter
// contract.
func (w *SortedNumericDocValuesWriter) GetDocValues() (SortedNumericDocValues, error) {
	if err := w.finish(); err != nil {
		return nil, err
	}
	return w.getValues(w.finalValues, w.finalValuesCount), nil
}

// getValues constructs the appropriate buffered view: singleton fast path
// when no multi-valued doc was ever observed, otherwise the explicit
// counts-driven view.
func (w *SortedNumericDocValuesWriter) getValues(
	values *packed.PackedLongValues,
	valueCounts *packed.PackedLongValues,
) SortedNumericDocValues {
	if valueCounts == nil {
		return Singleton(newBufferedNumericDocValues(values, w.docsWithFieldDocs()))
	}
	return newBufferedSortedNumericDocValues(values, valueCounts, w.docsWithFieldDocs())
}

// docsWithFieldDocs returns the docIDs in the order they were added. The
// existing DocsWithFieldSet exposes no iterator; we synthesise a slice from
// either its dense prefix or its sparse bitset, mirroring the helper used
// by SortedSetDocValuesWriter.
func (w *SortedNumericDocValuesWriter) docsWithFieldDocs() []int {
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

// SortedNumericFieldConsumer is the callback used by Flush to hand the
// buffered SortedNumericDocValues to the underlying codec consumer.
//
// Gocene divergence: replaces the Java
// DocValuesConsumer.addSortedNumericField + EmptyDocValuesProducer.getSortedNumeric
// anonymous override with a simple function-typed boundary. The wiring
// layer in the codecs package adapts codecs.DocValuesConsumer to this
// signature.
type SortedNumericFieldConsumer func(field *FieldInfo, values SortedNumericDocValues) error

// Flush hands the buffered state to consumer. When sortMap is non-nil the
// values are re-mapped via the segment's IndexSorter docmap (multi-valued
// branch only; see the type-level divergence note for the singleton
// branch).
//
// Gocene divergence: maxDoc is passed in explicitly rather than read from
// SegmentWriteState because Gocene's SegmentWriteState in the index package
// does not yet carry SegmentInfo.MaxDoc (same convention as
// [SortedSetDocValuesWriter.Flush] and [SortedDocValuesWriter.Flush]).
func (w *SortedNumericDocValuesWriter) Flush(
	maxDoc int,
	sortMap SorterDocMap,
	consumer SortedNumericFieldConsumer,
) error {
	if consumer == nil {
		return errors.New("SortedNumericDocValuesWriter.Flush: consumer must not be nil")
	}
	if err := w.finish(); err != nil {
		return err
	}
	values := w.getValues(w.finalValues, w.finalValuesCount)
	if sortMap == nil || w.finalValuesCount == nil {
		// Singleton branch (no counts) defers to the unsorted view; see
		// the type-level divergence note. The no-sortMap case naturally
		// returns the buffered view as-is.
		return consumer(w.fieldInfo, values)
	}
	sorted, err := newSortedNumericLongValues(
		maxDoc, sortMap, values, packed.Fastest,
	)
	if err != nil {
		return err
	}
	// Rebuild a fresh buffered view: the Java original constructs a second
	// BufferedSortedNumericDocValues so that the SortingSortedNumericDocValues
	// delegate keeps an untouched iterator state to fall back to for cost().
	delegate := newBufferedSortedNumericDocValues(
		w.finalValues, w.finalValuesCount, w.docsWithFieldDocs(),
	)
	return consumer(w.fieldInfo, newSortingSortedNumericDocValues(delegate, sorted))
}

// ============================================================================
// BufferedNumericDocValues — single-valued view used by the singleton path.
// ============================================================================

// bufferedNumericDocValues mirrors the Java NumericDocValuesWriter's
// BufferedNumericDocValues, providing a NumericDocValues view over a
// PackedLongValues stream plus a docID-in-addition-order slice.
type bufferedNumericDocValues struct {
	valuesIter *packed.PackedLongValuesIterator
	docs       []int
	docIdx     int
	docID      int
	value      int64
}

func newBufferedNumericDocValues(
	values *packed.PackedLongValues,
	docs []int,
) *bufferedNumericDocValues {
	return &bufferedNumericDocValues{
		valuesIter: values.Iterator(),
		docs:       docs,
		docID:      -1,
	}
}

// DocID returns the current docID, or NO_MORE_DOCS when exhausted.
func (b *bufferedNumericDocValues) DocID() int { return b.docID }

// NextDoc advances to the next doc with a value.
func (b *bufferedNumericDocValues) NextDoc() (int, error) {
	if b.docIdx >= len(b.docs) {
		b.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	b.docID = b.docs[b.docIdx]
	b.docIdx++
	b.value = b.valuesIter.Next()
	return b.docID, nil
}

// Advance is unsupported on the buffered view (Java raises
// UnsupportedOperationException); callers should iterate via NextDoc.
func (b *bufferedNumericDocValues) Advance(int) (int, error) {
	return 0, errors.New("bufferedNumericDocValues: Advance is not supported; use NextDoc")
}

// Get returns the value for docID. docID must equal the current cursor.
func (b *bufferedNumericDocValues) Get(docID int) (int64, error) {
	if docID != b.docID {
		return 0, fmt.Errorf("bufferedNumericDocValues: Get(%d) requires NextDoc cursor; current=%d", docID, b.docID)
	}
	return b.value, nil
}

// ============================================================================
// BufferedSortedNumericDocValues — multi-valued view.
// ============================================================================

// bufferedSortedNumericDocValues implements SortedNumericDocValues on top of
// the raw PackedLongValues + counts streams plus the docID-in-addition-order
// slice.
type bufferedSortedNumericDocValues struct {
	valuesIter      *packed.PackedLongValuesIterator
	valueCountsIter *packed.PackedLongValuesIterator
	docs            []int
	docIdx          int
	docID           int
	currentDoc      []int64
	currentCount    int
}

func newBufferedSortedNumericDocValues(
	values *packed.PackedLongValues,
	valueCounts *packed.PackedLongValues,
	docs []int,
) *bufferedSortedNumericDocValues {
	return &bufferedSortedNumericDocValues{
		valuesIter:      values.Iterator(),
		valueCountsIter: valueCounts.Iterator(),
		docs:            docs,
		docID:           -1,
	}
}

// DocID returns the current docID, or NO_MORE_DOCS when exhausted.
func (b *bufferedSortedNumericDocValues) DocID() int { return b.docID }

// NextDoc advances to the next doc with values, prefetching that doc's
// values into the per-doc scratch slice in sorted order (already sorted on
// the writer side).
func (b *bufferedSortedNumericDocValues) NextDoc() (int, error) {
	if b.docIdx >= len(b.docs) {
		b.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	b.docID = b.docs[b.docIdx]
	b.docIdx++
	b.currentCount = int(b.valueCountsIter.Next())
	if cap(b.currentDoc) < b.currentCount {
		b.currentDoc = make([]int64, b.currentCount)
	} else {
		b.currentDoc = b.currentDoc[:b.currentCount]
	}
	for i := 0; i < b.currentCount; i++ {
		b.currentDoc[i] = b.valuesIter.Next()
	}
	return b.docID, nil
}

// Advance is unsupported on the buffered view (Java raises
// UnsupportedOperationException); callers should iterate via NextDoc.
func (b *bufferedSortedNumericDocValues) Advance(int) (int, error) {
	return 0, errors.New("bufferedSortedNumericDocValues: Advance is not supported; use NextDoc")
}

// Get returns the values for docID. docID must equal the current cursor.
// A fresh copy is returned so the caller may retain it across NextDoc.
func (b *bufferedSortedNumericDocValues) Get(docID int) ([]int64, error) {
	if docID != b.docID {
		return nil, fmt.Errorf("bufferedSortedNumericDocValues: Get(%d) requires NextDoc cursor; current=%d", docID, b.docID)
	}
	out := make([]int64, b.currentCount)
	copy(out, b.currentDoc[:b.currentCount])
	return out, nil
}

// ============================================================================
// Sort-aware view: SortingSortedNumericDocValues + sortedNumericLongValues.
// ============================================================================

// sortedNumericLongValues mirrors the Java
// SortedNumericDocValuesWriter.LongValues helper. It re-maps a multi-valued
// doc stream through the segment's docmap and stores per-doc start offsets
// plus the flattened (count, value*) tape in a single PackedLongValues.
type sortedNumericLongValues struct {
	offsets []int64
	values  *packed.PackedLongValues
}

func newSortedNumericLongValues(
	maxDoc int,
	sortMap SorterDocMap,
	oldValues SortedNumericDocValues,
	acceptableOverheadRatio float32,
) (*sortedNumericLongValues, error) {
	d := &sortedNumericLongValues{offsets: make([]int64, maxDoc)}
	builder, err := packed.PackedBuilder(packed.PackedLongValuesDefaultPageSize, acceptableOverheadRatio)
	if err != nil {
		return nil, err
	}
	// Java's offsetIndex starts at 1 because 0 marks "no values for this doc".
	var offsetIndex int64 = 1
	for {
		docID, err := oldValues.NextDoc()
		if err != nil {
			return nil, err
		}
		if docID == NO_MORE_DOCS {
			break
		}
		newDocID := sortMap.OldToNew(docID)
		if newDocID < 0 || newDocID >= maxDoc {
			return nil, fmt.Errorf("sortedNumericLongValues: sortMap.OldToNew(%d)=%d outside [0..%d)", docID, newDocID, maxDoc)
		}
		vals, err := oldValues.Get(docID)
		if err != nil {
			return nil, err
		}
		numValues := len(vals)
		// Tape layout per Java: count, v0, v1, ...
		if err := builder.Add(int64(numValues)); err != nil {
			return nil, err
		}
		d.offsets[newDocID] = offsetIndex
		offsetIndex++
		for _, v := range vals {
			if err := builder.Add(v); err != nil {
				return nil, err
			}
			offsetIndex++
		}
	}
	d.values = builder.Build()
	return d, nil
}

// sortingSortedNumericDocValues iterates a sortedNumericLongValues mapping
// in new-doc order; Lucene throws on advance() and delegates cost() to the
// untouched source iterator.
type sortingSortedNumericDocValues struct {
	in        SortedNumericDocValues
	values    *sortedNumericLongValues
	docID     int
	upto      int64
	numValues int
}

func newSortingSortedNumericDocValues(
	in SortedNumericDocValues,
	values *sortedNumericLongValues,
) *sortingSortedNumericDocValues {
	return &sortingSortedNumericDocValues{in: in, values: values, docID: -1}
}

// DocID returns the current docID, or NO_MORE_DOCS when exhausted.
func (s *sortingSortedNumericDocValues) DocID() int { return s.docID }

// NextDoc advances to the next sorted doc that has a value.
func (s *sortingSortedNumericDocValues) NextDoc() (int, error) {
	for {
		s.docID++
		if s.docID >= len(s.values.offsets) {
			s.docID = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		if s.values.offsets[s.docID] > 0 {
			s.upto = s.values.offsets[s.docID]
			s.numValues = int(s.values.values.Get(s.upto - 1))
			return s.docID, nil
		}
	}
}

// Advance is unsupported; the Java original throws
// UnsupportedOperationException("use nextDoc instead"). Gocene returns the
// equivalent error.
func (s *sortingSortedNumericDocValues) Advance(int) (int, error) {
	return 0, errors.New("sortingSortedNumericDocValues: Advance is not supported; use NextDoc")
}

// Get returns the values for docID. docID must equal the current cursor.
// A fresh copy is returned so the caller may retain it across NextDoc.
func (s *sortingSortedNumericDocValues) Get(docID int) ([]int64, error) {
	if docID != s.docID {
		return nil, fmt.Errorf("sortingSortedNumericDocValues: Get(%d) requires NextDoc cursor; current=%d", docID, s.docID)
	}
	out := make([]int64, s.numValues)
	// Java reads via nextValue() which consumes one offset at a time off the
	// shared cursor; Gocene materialises into a fresh slice for the
	// slice-based SortedNumericDocValues.Get contract.
	for i := 0; i < s.numValues; i++ {
		out[i] = s.values.values.Get(s.upto + int64(i))
	}
	return out, nil
}
