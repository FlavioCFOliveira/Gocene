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

// SortedDocValuesWriter buffers up to one byte[] value per doc, dereferences
// them through a BytesRefHash, and on flush sorts the unique values to assign
// stable ordinals.
//
// This is the Go port of org.apache.lucene.index.SortedDocValuesWriter
// from Apache Lucene 10.4.0.
//
// Gocene divergences:
//
//   - The Java original extends an abstract DocValuesWriter<SortedDocValues>;
//     Gocene has no such base type, so the public surface (AddValue,
//     GetDocValues, Flush) is exposed directly on the writer.
//   - DocsWithFieldSet does not expose a Java-style iterator; the writer
//     reuses the dense-or-sparse traversal helper materialised by the
//     sibling SortedSetDocValuesWriter (docsWithFieldDocs / trailingZeros64).
type SortedDocValuesWriter struct {
	hash          *util.BytesRefHash
	pending       *packed.PackedLongValuesBuilder
	docsWithField *DocsWithFieldSet
	iwBytesUsed   *util.Counter
	bytesUsed     int64
	fieldInfo     *FieldInfo
	lastDocID     int

	finalOrds         *packed.PackedLongValues
	finalSortedValues []int
	finalOrdMap       []int
}

// NewSortedDocValuesWriter constructs a writer for the given field. The pool
// parameter feeds the underlying BytesRefHash; bytes-used updates are
// reported to iwBytesUsed.
func NewSortedDocValuesWriter(
	fieldInfo *FieldInfo,
	iwBytesUsed *util.Counter,
	pool *util.ByteBlockPool,
) *SortedDocValuesWriter {
	w := &SortedDocValuesWriter{
		fieldInfo:     fieldInfo,
		iwBytesUsed:   iwBytesUsed,
		docsWithField: NewDocsWithFieldSet(),
		lastDocID:     -1,
	}
	w.hash = util.NewBytesRefHashWithCapacity(
		pool,
		util.DefaultCapacity,
		util.NewDirectBytesStartArrayWithCounter(util.DefaultCapacity, iwBytesUsed),
	)
	// Mirrors PackedInts.COMPACT (deltaPackedBuilder); the pending stream is
	// monotonically grown per added doc.
	builder, err := packed.DeltaPackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
	if err != nil {
		// Invariant: DefaultPageSize is within bounds, ratio is valid.
		panic(fmt.Sprintf("invalid DeltaPackedBuilder configuration: %v", err))
	}
	w.pending = builder
	w.bytesUsed = w.pending.Size() * 8
	w.iwBytesUsed.AddAndGet(w.bytesUsed)
	return w
}

// AddValue appends value for docID. docID must be strictly greater than any
// previously seen docID; only one value per doc is allowed.
func (w *SortedDocValuesWriter) AddValue(docID int, value *util.BytesRef) error {
	if docID <= w.lastDocID {
		return fmt.Errorf(
			"DocValuesField %q appears more than once in this document (only one value is allowed per field)",
			w.fieldInfo.Name(),
		)
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

	if err := w.addOneValue(value); err != nil {
		return err
	}
	if err := w.docsWithField.Add(docID); err != nil {
		return err
	}
	w.lastDocID = docID
	return nil
}

// addOneValue inserts value into the hash and records its term id in
// pending.
func (w *SortedDocValuesWriter) addOneValue(value *util.BytesRef) error {
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
	if err := w.pending.Add(int64(termID)); err != nil {
		return err
	}
	w.updateBytesUsed()
	return nil
}

func (w *SortedDocValuesWriter) updateBytesUsed() {
	newBytesUsed := w.pending.Size() * 8
	w.iwBytesUsed.AddAndGet(newBytesUsed - w.bytesUsed)
	w.bytesUsed = newBytesUsed
}

// finish freezes the in-memory state for read-back; idempotent.
func (w *SortedDocValuesWriter) finish() error {
	if w.finalSortedValues != nil {
		return nil
	}
	valueCount := w.hash.Size()
	w.updateBytesUsed()
	w.finalSortedValues = w.hash.Sort()
	w.finalOrds = w.pending.Build()
	w.finalOrdMap = make([]int, valueCount)
	for ord := 0; ord < valueCount; ord++ {
		w.finalOrdMap[w.finalSortedValues[ord]] = ord
	}
	return nil
}

// GetDocValues materialises an in-memory SortedDocValues view of the
// buffered state. Mirrors the Java getDocValues() / DocValuesWriter
// contract.
func (w *SortedDocValuesWriter) GetDocValues() (SortedDocValues, error) {
	if err := w.finish(); err != nil {
		return nil, err
	}
	return newBufferedSingleSortedDocValues(
		w.hash, w.finalOrds, w.finalSortedValues, w.finalOrdMap, w.docsWithFieldDocs(),
	), nil
}

// docsWithFieldDocs materialises the docIDs in addition order via the same
// dense-or-sparse traversal helper used by SortedSetDocValuesWriter.
func (w *SortedDocValuesWriter) docsWithFieldDocs() []int {
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

// Flush hands the buffered values to dvConsumer.AddSortedField through the
// producer built by getDocValuesProducer. When sortMap is non-nil the values
// are re-mapped via the segment's IndexSorter docmap. Mirrors
// SortedDocValuesWriter.flush(SegmentWriteState, Sorter.DocMap,
// DocValuesConsumer).
func (w *SortedDocValuesWriter) Flush(
	state *SegmentWriteState,
	sortMap SorterDocMap,
	dvConsumer DocValuesConsumer,
) error {
	if dvConsumer == nil {
		return errors.New("SortedDocValuesWriter.Flush: consumer must not be nil")
	}
	if err := w.finish(); err != nil {
		return err
	}
	producer, err := sortedDocValuesWriterGetDocValuesProducer(
		w.fieldInfo, w.hash, w.finalOrds, w.finalSortedValues, w.finalOrdMap, w.docsWithField, sortMap)
	if err != nil {
		return err
	}
	return dvConsumer.AddSortedField(w.fieldInfo, producer)
}

// sortedDocValuesWriterGetDocValuesProducer mirrors the static
// SortedDocValuesWriter.getDocValuesProducer(FieldInfo, BytesRefHash,
// PackedLongValues, int[], int[], DocsWithFieldSet, Sorter.DocMap): when
// sortMap is set the ordinals are sorted once, and the returned producer hands
// out a fresh view on every call.
func sortedDocValuesWriterGetDocValuesProducer(
	writerFieldInfo *FieldInfo,
	hash *util.BytesRefHash,
	ords *packed.PackedLongValues,
	sortedValues []int,
	ordMap []int,
	docsWithField *DocsWithFieldSet,
	sortMap SorterDocMap,
) (DocValuesProducer, error) {
	var sorted []int
	if sortMap != nil {
		s, err := sortDocValues(
			sortMap.Size(),
			sortMap,
			newBufferedSingleSortedDocValues(hash, ords, sortedValues, ordMap, docsWithFieldSetDocs(docsWithField)),
		)
		if err != nil {
			return nil, err
		}
		sorted = s
	}
	return &sortedDocValuesWriterDocValuesProducer{
		writerFieldInfo: writerFieldInfo,
		hash:            hash,
		ords:            ords,
		sortedValues:    sortedValues,
		ordMap:          ordMap,
		docsWithField:   docsWithField,
		sorted:          sorted,
	}, nil
}

// sortedDocValuesWriterDocValuesProducer is the anonymous
// EmptyDocValuesProducer subclass returned by getDocValuesProducer.
type sortedDocValuesWriterDocValuesProducer struct {
	EmptyDocValuesProducer
	writerFieldInfo *FieldInfo
	hash            *util.BytesRefHash
	ords            *packed.PackedLongValues
	sortedValues    []int
	ordMap          []int
	docsWithField   *DocsWithFieldSet
	sorted          []int
}

// GetSorted returns the buffered values, or their sorted view.
func (p *sortedDocValuesWriterDocValuesProducer) GetSorted(fieldInfoIn *FieldInfo) (SortedDocValues, error) {
	if fieldInfoIn != p.writerFieldInfo {
		return nil, errors.New("wrong fieldInfo")
	}
	buf := newBufferedSingleSortedDocValues(
		p.hash, p.ords, p.sortedValues, p.ordMap, docsWithFieldSetDocs(p.docsWithField))
	if p.sorted == nil {
		return buf, nil
	}
	return newSortingSortedDocValues(buf, p.sorted), nil
}

// GetMergeInstance returns the receiver (DocValuesProducer default).
func (p *sortedDocValuesWriterDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

// sortDocValues mirrors SortedDocValuesWriter.sortDocValues in the Java
// source: it walks the unsorted view and builds an ord-per-newDocID slice,
// filling unset slots with -1.
func sortDocValues(maxDoc int, sortMap SorterDocMap, oldValues SortedDocValues) ([]int, error) {
	ords := make([]int, maxDoc)
	for i := range ords {
		ords[i] = -1
	}
	for {
		docID, err := oldValues.NextDoc()
		if err != nil {
			return nil, err
		}
		if docID == NO_MORE_DOCS {
			break
		}
		// docID is the current cursor — OrdValue is equivalent to GetOrd(docID).
		ord, err := oldValues.OrdValue()
		if err != nil {
			return nil, err
		}
		newDocID := sortMap.OldToNew(docID)
		if newDocID < 0 || newDocID >= maxDoc {
			return nil, fmt.Errorf("sortDocValues: sortMap.OldToNew(%d)=%d outside [0..%d)", docID, newDocID, maxDoc)
		}
		ords[newDocID] = ord
	}
	return ords, nil
}

// ============================================================================
// SortingSortedDocValues — sort-aware view used by Flush when sortMap != nil.
// ============================================================================

// sortingSortedDocValues iterates a sortDocValues result in new-doc order and
// resolves bytes via the underlying buffered view.
type sortingSortedDocValues struct {
	in    SortedDocValues
	ords  []int
	docID int
}

func newSortingSortedDocValues(in SortedDocValues, ords []int) *sortingSortedDocValues {
	return &sortingSortedDocValues{in: in, ords: ords, docID: -1}
}

func (s *sortingSortedDocValues) DocID() int { return s.docID }

func (s *sortingSortedDocValues) NextDoc() (int, error) {
	for {
		s.docID++
		if s.docID == len(s.ords) {
			s.docID = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		if s.ords[s.docID] != -1 {
			return s.docID, nil
		}
	}
}

// Advance returns the docID if it has a value, otherwise NO_MORE_DOCS.
// Mirrors the Java SortingSortedDocValues.advanceExact contract; the Java
// advance(int) intentionally throws UnsupportedOperationException — Gocene
// returns an error in that case.
func (s *sortingSortedDocValues) Advance(target int) (int, error) {
	return 0, errors.New("sortingSortedDocValues: Advance is not supported; use NextDoc")
}

// AdvanceExact positions the cursor at target and reports whether the
// target has an ord. Mirrors the Java
// SortingSortedDocValues#advanceExact, which the IndexSorter callers
// rely on. T4709-added.
func (s *sortingSortedDocValues) AdvanceExact(target int) (bool, error) {
	if target < 0 || target >= len(s.ords) {
		return false, fmt.Errorf("sortingSortedDocValues: AdvanceExact(%d) out of bounds", target)
	}
	s.docID = target
	return s.ords[target] != -1, nil
}

// BinaryValue returns the term bytes for the current cursor position.
// Mirrors the Java reference's advanceExact + lookupOrd(ordValue()).
func (s *sortingSortedDocValues) BinaryValue() ([]byte, error) {
	if s.docID < 0 || s.docID >= len(s.ords) || s.ords[s.docID] == -1 {
		return nil, fmt.Errorf("sortingSortedDocValues: BinaryValue at invalid position %d", s.docID)
	}
	return s.in.LookupOrd(s.ords[s.docID])
}

// OrdValue returns the ord bound to the current cursor position.
// Mirrors org.apache.lucene.index.SortedDocValues#ordValue.
func (s *sortingSortedDocValues) OrdValue() (int, error) {
	if s.docID < 0 || s.docID >= len(s.ords) {
		return -1, fmt.Errorf("sortingSortedDocValues: OrdValue at invalid position %d", s.docID)
	}
	return s.ords[s.docID], nil
}

// LongValue is unsupported on SortedDocValues — the inherited
// NumericDocValues surface satisfies the interface but ordinals are
// surfaced through OrdValue.
func (s *sortingSortedDocValues) LongValue() (int64, error) {
	ord, err := s.OrdValue()
	if err != nil {
		return 0, err
	}
	return int64(ord), nil
}

// Cost delegates to the underlying buffered view.
func (s *sortingSortedDocValues) Cost() int64 { return s.in.Cost() }

func (s *sortingSortedDocValues) LookupOrd(ord int) ([]byte, error) { return s.in.LookupOrd(ord) }

func (s *sortingSortedDocValues) GetValueCount() int { return s.in.GetValueCount() }

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which the Java counterpart of this type does not override.
func (s *sortingSortedDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0 — docID() + 1 — which the Java counterpart of this type
// does not override.
func (s *sortingSortedDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(s)
}
