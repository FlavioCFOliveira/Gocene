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
//   - Java's flush() targets DocValuesConsumer.addSortedField via an
//     EmptyDocValuesProducer anonymous subclass. To avoid an import cycle
//     with the codecs package this writer takes a local SortedFieldConsumer
//     callback; the codec wiring layer adapts codecs.DocValuesConsumer to
//     this signature (same pattern as [SortedSetDocValuesWriter]).
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

// SortedFieldConsumer is the callback used by Flush to hand the buffered
// SortedDocValues to the underlying codec consumer.
//
// Gocene divergence: replaces the Java DocValuesConsumer.addSortedField +
// EmptyDocValuesProducer.getSorted anonymous override with a simple
// function-typed boundary. The wiring layer in the codecs package adapts
// codecs.DocValuesConsumer to this signature.
type SortedFieldConsumer func(field *FieldInfo, values SortedDocValues) error

// Flush hands the buffered state to consumer. When sortMap is non-nil the
// values are re-mapped via the segment's IndexSorter docmap.
//
// Gocene divergence: maxDoc is passed in explicitly rather than read from
// SegmentWriteState because Gocene's SegmentWriteState in the index package
// does not yet carry SegmentInfo.MaxDoc (same convention as
// [SortedSetDocValuesWriter.Flush]).
func (w *SortedDocValuesWriter) Flush(
	maxDoc int,
	sortMap SorterDocMap,
	consumer SortedFieldConsumer,
) error {
	if consumer == nil {
		return errors.New("SortedDocValuesWriter.Flush: consumer must not be nil")
	}
	if err := w.finish(); err != nil {
		return err
	}
	buf := newBufferedSingleSortedDocValues(
		w.hash, w.finalOrds, w.finalSortedValues, w.finalOrdMap, w.docsWithFieldDocs(),
	)
	if sortMap == nil {
		return consumer(w.fieldInfo, buf)
	}
	sorted, err := sortDocValues(maxDoc, sortMap, buf)
	if err != nil {
		return err
	}
	// Rebuild a fresh buffered view: the Java original constructs a second
	// BufferedSortedDocValues so that the SortingSortedDocValues delegate
	// has an untouched iterator state to fall back to for lookupOrd.
	delegate := newBufferedSingleSortedDocValues(
		w.hash, w.finalOrds, w.finalSortedValues, w.finalOrdMap, w.docsWithFieldDocs(),
	)
	return consumer(w.fieldInfo, newSortingSortedDocValues(delegate, sorted))
}

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
		ord, err := oldValues.GetOrd(docID)
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

// Get returns the value bytes for docID. docID must equal the current
// cursor; the Java original implements advanceExact + lookupOrd(ordValue()),
// which Gocene collapses into a single Get for consistency with the rest of
// the package.
func (s *sortingSortedDocValues) Get(docID int) ([]byte, error) {
	if docID != s.docID {
		return nil, fmt.Errorf("sortingSortedDocValues: Get(%d) requires NextDoc cursor; current=%d", docID, s.docID)
	}
	return s.in.LookupOrd(s.ords[docID])
}

// GetOrd returns the ord assigned to docID. docID must equal the current
// cursor.
func (s *sortingSortedDocValues) GetOrd(docID int) (int, error) {
	if docID != s.docID {
		return -1, fmt.Errorf("sortingSortedDocValues: GetOrd(%d) requires NextDoc cursor; current=%d", docID, s.docID)
	}
	return s.ords[docID], nil
}

func (s *sortingSortedDocValues) LookupOrd(ord int) ([]byte, error) { return s.in.LookupOrd(ord) }

func (s *sortingSortedDocValues) GetValueCount() int { return s.in.GetValueCount() }
