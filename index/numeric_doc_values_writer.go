// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// NumericDocValuesWriter buffers up one pending long per doc, then flushes
// when the owning segment flushes.
//
// This is the Go port of org.apache.lucene.index.NumericDocValuesWriter
// from Apache Lucene 10.4.0.
//
// Gocene divergences:
//
//   - The Java original extends an abstract DocValuesWriter<NumericDocValues>;
//     Gocene has no such base type yet, so this writer exposes its public
//     surface (AddValue, GetDocValues, Flush) directly. Same pattern as
//     [SortedNumericDocValuesWriter] / [SortedDocValuesWriter].
//   - DocsWithFieldSet does not expose a Java-style iterator; the writer
//     reuses the dense-or-sparse traversal helper shared with
//     [SortedNumericDocValuesWriter] (docsWithFieldDocs / trailingZeros64).
//   - The buffered single-valued view ([bufferedNumericDocValues]) is the
//     one already declared in sorted_numeric_doc_values_writer.go and is
//     reused here unchanged.
type NumericDocValuesWriter struct {
	pending       *packed.PackedLongValuesBuilder
	finalValues   *packed.PackedLongValues
	iwBytesUsed   *util.Counter
	bytesUsed     int64
	docsWithField *DocsWithFieldSet
	fieldInfo     *FieldInfo
	lastDocID     int
}

// NewNumericDocValuesWriter constructs a writer for the given field.
// bytes-used updates are reported to iwBytesUsed.
func NewNumericDocValuesWriter(
	fieldInfo *FieldInfo,
	iwBytesUsed *util.Counter,
) *NumericDocValuesWriter {
	w := &NumericDocValuesWriter{
		fieldInfo:     fieldInfo,
		iwBytesUsed:   iwBytesUsed,
		docsWithField: NewDocsWithFieldSet(),
		lastDocID:     -1,
	}
	// Mirrors PackedInts.COMPACT (deltaPackedBuilder).
	builder, err := packed.DeltaPackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
	if err != nil {
		// Invariant: DefaultPageSize is within bounds, ratio is valid.
		panic(fmt.Sprintf("invalid DeltaPackedBuilder configuration: %v", err))
	}
	w.pending = builder
	// Best-effort byte accounting; Java uses pending.ramBytesUsed() +
	// docsWithField.ramBytesUsed(). We approximate to keep the counter
	// monotonic during AddValue without pulling in the full
	// RamUsageEstimator port (same approximation as SortedNumericDocValuesWriter).
	w.bytesUsed = w.pending.Size() * 8
	w.iwBytesUsed.AddAndGet(w.bytesUsed)
	return w
}

// AddValue appends value for docID. docID must be strictly greater than any
// previously seen docID; a field accepts at most one value per document.
func (w *NumericDocValuesWriter) AddValue(docID int, value int64) error {
	if docID <= w.lastDocID {
		return fmt.Errorf(
			"DocValuesField %q appears more than once in this document (only one value is allowed per field)",
			w.fieldInfo.Name(),
		)
	}
	if err := w.pending.Add(value); err != nil {
		return err
	}
	if err := w.docsWithField.Add(docID); err != nil {
		return err
	}
	w.updateBytesUsed()
	w.lastDocID = docID
	return nil
}

// updateBytesUsed reconciles the iwBytesUsed counter with the writer's
// current memory footprint.
func (w *NumericDocValuesWriter) updateBytesUsed() {
	newBytesUsed := w.pending.Size() * 8
	w.iwBytesUsed.AddAndGet(newBytesUsed - w.bytesUsed)
	w.bytesUsed = newBytesUsed
}

// finish freezes the writer's in-memory state for read-back. Idempotent.
func (w *NumericDocValuesWriter) finish() {
	if w.finalValues == nil {
		w.finalValues = w.pending.Build()
	}
}

// GetDocValues materialises an in-memory NumericDocValues view of the
// buffered state. Mirrors the Java getDocValues() / DocValuesWriter contract.
func (w *NumericDocValuesWriter) GetDocValues() NumericDocValues {
	w.finish()
	return newBufferedNumericDocValues(w.finalValues, w.docsWithFieldDocs())
}

// docsWithFieldDocs returns the docIDs in the order they were added.
func (w *NumericDocValuesWriter) docsWithFieldDocs() []int {
	return docsWithFieldSetDocs(w.docsWithField)
}

// docsWithFieldSetDocs materialises the documents of d, in ascending order,
// for the slice-backed buffered views: it stands in for the
// docsWithField.iterator() the Java views consume, read from either the
// dense prefix or the sparse bitset of d.
func docsWithFieldSetDocs(d *DocsWithFieldSet) []int {
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

// Flush hands the buffered values to dvConsumer.AddNumericField through the
// producer built by getDocValuesProducer. When sortMap is non-nil the values
// are re-mapped via the segment's IndexSorter docmap. Mirrors
// NumericDocValuesWriter.flush(SegmentWriteState, Sorter.DocMap,
// DocValuesConsumer).
func (w *NumericDocValuesWriter) Flush(
	state *SegmentWriteState,
	sortMap SorterDocMap,
	dvConsumer DocValuesConsumer,
) error {
	if dvConsumer == nil {
		return errors.New("NumericDocValuesWriter.Flush: consumer must not be nil")
	}
	w.finish()
	producer, err := numericDocValuesWriterGetDocValuesProducer(w.fieldInfo, w.finalValues, w.docsWithField, sortMap)
	if err != nil {
		return err
	}
	return dvConsumer.AddNumericField(w.fieldInfo, producer)
}

// numericDocValuesWriterGetDocValuesProducer mirrors the static
// NumericDocValuesWriter.getDocValuesProducer(FieldInfo, PackedLongValues,
// DocsWithFieldSet, Sorter.DocMap): when sortMap is set the values are sorted
// once, and the returned producer hands out a fresh view on every call.
func numericDocValuesWriterGetDocValuesProducer(
	writerFieldInfo *FieldInfo,
	values *packed.PackedLongValues,
	docsWithField *DocsWithFieldSet,
	sortMap SorterDocMap,
) (DocValuesProducer, error) {
	var sorted *numericDVs
	if sortMap != nil {
		oldValues := newBufferedNumericDocValues(values, docsWithFieldSetDocs(docsWithField))
		s, err := sortNumericDocValues(
			sortMap.Size(), sortMap, oldValues, sortMap.Size() == docsWithField.Cardinality())
		if err != nil {
			return nil, err
		}
		sorted = s
	}
	return &numericDocValuesWriterDocValuesProducer{
		writerFieldInfo: writerFieldInfo,
		values:          values,
		docsWithField:   docsWithField,
		sorted:          sorted,
	}, nil
}

// numericDocValuesWriterDocValuesProducer is the anonymous
// EmptyDocValuesProducer subclass returned by getDocValuesProducer.
type numericDocValuesWriterDocValuesProducer struct {
	EmptyDocValuesProducer
	writerFieldInfo *FieldInfo
	values          *packed.PackedLongValues
	docsWithField   *DocsWithFieldSet
	sorted          *numericDVs
}

// GetNumeric returns the buffered values, or their sorted view.
func (p *numericDocValuesWriterDocValuesProducer) GetNumeric(fieldInfo *FieldInfo) (NumericDocValues, error) {
	if fieldInfo != p.writerFieldInfo {
		return nil, errors.New("wrong fieldInfo")
	}
	if p.sorted == nil {
		return newBufferedNumericDocValues(p.values, docsWithFieldSetDocs(p.docsWithField)), nil
	}
	return newSortingNumericDocValues(p.sorted), nil
}

// GetMergeInstance returns the receiver (DocValuesProducer default).
func (p *numericDocValuesWriterDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

// sortNumericDocValues walks oldDocValues, remaps each docID through sortMap and
// scatters the values into a maxDoc-sized array. When dense is false a
// FixedBitSet records which target slots received a value. Mirrors the
// Java NumericDocValuesWriter.sortNumericDocValues helper.
func sortNumericDocValues(
	maxDoc int,
	sortMap SorterDocMap,
	oldDocValues NumericDocValues,
	dense bool,
) (*numericDVs, error) {
	var docsWithField *util.FixedBitSet
	if !dense {
		bs, err := util.NewFixedBitSet(maxDoc)
		if err != nil {
			return nil, err
		}
		docsWithField = bs
	}
	values := make([]int64, maxDoc)
	for {
		docID, err := oldDocValues.NextDoc()
		if err != nil {
			return nil, err
		}
		if docID == NO_MORE_DOCS {
			break
		}
		newDocID := sortMap.OldToNew(docID)
		if newDocID < 0 || newDocID >= maxDoc {
			return nil, fmt.Errorf(
				"sortNumericDocValues: sortMap.OldToNew(%d)=%d outside [0..%d)", docID, newDocID, maxDoc)
		}
		if docsWithField != nil {
			docsWithField.Set(newDocID)
		}
		// docID is the current cursor — LongValue is equivalent to Get(docID).
		v, err := oldDocValues.LongValue()
		if err != nil {
			return nil, err
		}
		values[newDocID] = v
	}
	return newNumericDVs(values, docsWithField), nil
}

// ============================================================================
// numericDVs — the scattered, docmap-remapped value array.
// ============================================================================

// numericDVs holds the values array produced by sortNumericDocValues plus an
// optional sparse-doc bitset. Mirrors the Java NumericDVs inner class.
type numericDVs struct {
	values        []int64
	docsWithField *util.FixedBitSet // nil => every doc in [0, maxDoc) has a value
	maxDoc        int
}

func newNumericDVs(values []int64, docsWithField *util.FixedBitSet) *numericDVs {
	return &numericDVs{
		values:        values,
		docsWithField: docsWithField,
		maxDoc:        len(values),
	}
}

// advanceExact reports whether target has a value.
func (d *numericDVs) advanceExact(target int) bool {
	if d.docsWithField != nil {
		return d.docsWithField.Get(target)
	}
	return true
}

// advance returns the first doc >= target that has a value. target is only
// ever called with a value < maxDoc.
func (d *numericDVs) advance(target int) int {
	if d.docsWithField != nil {
		return d.docsWithField.NextSetBit(target)
	}
	return target
}

// cost returns the number of docs with a value.
func (d *numericDVs) cost() int64 {
	if d.docsWithField != nil {
		return int64(d.docsWithField.Cardinality())
	}
	return int64(d.maxDoc)
}

// ============================================================================
// sortingNumericDocValues — NumericDocValues view over a numericDVs mapping.
// ============================================================================

// sortingNumericDocValues iterates a numericDVs mapping in new-doc order.
// Mirrors the Java NumericDocValuesWriter.SortingNumericDocValues inner class:
// Advance throws, advanceExact is supported (used by the index sorters), and
// cost is computed lazily.
type sortingNumericDocValues struct {
	dvs   *numericDVs
	docID int
	cost  int64
}

func newSortingNumericDocValues(dvs *numericDVs) *sortingNumericDocValues {
	return &sortingNumericDocValues{dvs: dvs, docID: -1, cost: -1}
}

// DocID returns the current docID, or NO_MORE_DOCS when exhausted.
func (s *sortingNumericDocValues) DocID() int { return s.docID }

// NextDoc advances to the next doc that has a value.
func (s *sortingNumericDocValues) NextDoc() (int, error) {
	if s.docID+1 == s.dvs.maxDoc {
		s.docID = NO_MORE_DOCS
	} else {
		s.docID = s.dvs.advance(s.docID + 1)
	}
	return s.docID, nil
}

// Advance is unsupported; the Java original throws
// UnsupportedOperationException("use nextDoc() instead").
func (s *sortingNumericDocValues) Advance(int) (int, error) {
	return 0, errors.New("sortingNumericDocValues: Advance is not supported; use NextDoc")
}

// AdvanceExact positions the cursor at target and reports whether target has
// a value. Needed by the IndexSorter long/int/double/float sorters.
func (s *sortingNumericDocValues) AdvanceExact(target int) (bool, error) {
	s.docID = target
	return s.dvs.advanceExact(target), nil
}

// LongValue returns the value bound to the current cursor position.
// Mirrors org.apache.lucene.index.NumericDocValues#longValue.
func (s *sortingNumericDocValues) LongValue() (int64, error) {
	if s.docID < 0 || s.docID >= len(s.dvs.values) {
		return 0, fmt.Errorf("sortingNumericDocValues: LongValue at invalid position %d", s.docID)
	}
	return s.dvs.values[s.docID], nil
}

// Cost returns the number of docs with a value, computed once and cached.
func (s *sortingNumericDocValues) Cost() int64 {
	if s.cost == -1 {
		s.cost = s.dvs.cost()
	}
	return s.cost
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which the Java counterpart of this type does not override.
func (s *sortingNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0 — docID() + 1 — which the Java counterpart of this type
// does not override.
func (s *sortingNumericDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(s)
}
