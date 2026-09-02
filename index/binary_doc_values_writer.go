// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// binaryDVWriterMaxLength is the maximum length of a single binary value.
// Mirrors the Java MAX_LENGTH = ArrayUtil.MAX_ARRAY_LENGTH.
const binaryDVWriterMaxLength = math.MaxInt32 - 8

// BinaryDocValuesWriter buffers up one pending byte[] per doc, then flushes
// when the owning segment flushes.
//
// This is the Go port of org.apache.lucene.index.BinaryDocValuesWriter from
// Apache Lucene 10.4.0.
//
// Gocene divergences:
//
//   - The Java original extends an abstract DocValuesWriter<BinaryDocValues>;
//     Gocene has no such base type yet, so this writer exposes its public
//     surface (AddValue, GetDocValues, Flush) directly. Same pattern as
//     [NumericDocValuesWriter] / [SortedDocValuesWriter].
//   - Internal byte store divergence: the Java original buffers values
//     through a PagedBytes DataOutput plus a parallel PackedLongValues length
//     stream, and decodes them back through a DataInput in BufferedBinary-
//     DocValues. Gocene instead backs the buffered values with a single
//     [util.BytesRefArray] (the same store the Java BinaryDVs sort path
//     already uses). BytesRefArray is append-only random-access and stores a
//     full copy of every value, so it subsumes both the byte payload and the
//     per-doc length: the separate PagedBytes + PackedLongValues.Builder +
//     length-iterator machinery is gone, and value i for the i-th added doc
//     is recovered with a direct GetBytes(i). This is purely an in-RAM
//     buffering choice with no serialized output, so byte-for-byte codec
//     compatibility is unaffected; it removes the PagedBytes block paging and
//     the packed length stream in favour of one contiguous, index-addressable
//     store.
//   - Java's flush() targets DocValuesConsumer.addBinaryField via an
//     EmptyDocValuesProducer anonymous subclass. To avoid an import cycle
//     with the codecs package this writer takes a local BinaryFieldConsumer
//     callback; the codec wiring layer adapts codecs.DocValuesConsumer to
//     this signature (same pattern as the sibling DocValuesWriters).
//   - DocsWithFieldSet does not expose a Java-style iterator; the writer
//     reuses the dense-or-sparse traversal helper materialised by the
//     sibling writers (docsWithFieldDocs / trailingZeros64).
//   - maxDoc is passed to Flush explicitly rather than read from
//     SegmentWriteState (same convention as the sibling writers).
type BinaryDocValuesWriter struct {
	values        *util.BytesRefArray
	docsWithField *DocsWithFieldSet
	iwBytesUsed   *util.Counter
	fieldInfo     *FieldInfo
	bytesUsed     int64
	lastDocID     int
}

// NewBinaryDocValuesWriter constructs a writer for the given field.
// bytes-used updates are reported to iwBytesUsed.
func NewBinaryDocValuesWriter(
	fieldInfo *FieldInfo,
	iwBytesUsed *util.Counter,
) (*BinaryDocValuesWriter, error) {
	w := &BinaryDocValuesWriter{
		values:        util.NewBytesRefArray(util.ByteBlockSize),
		docsWithField: NewDocsWithFieldSet(),
		iwBytesUsed:   iwBytesUsed,
		fieldInfo:     fieldInfo,
		lastDocID:     -1,
	}
	w.bytesUsed = w.values.BytesUsed()
	w.iwBytesUsed.AddAndGet(w.bytesUsed)
	return w, nil
}

// AddValue appends value for docID. docID must be strictly greater than any
// previously seen docID; a field accepts at most one value per document.
func (w *BinaryDocValuesWriter) AddValue(docID int, value *util.BytesRef) error {
	if docID <= w.lastDocID {
		return fmt.Errorf(
			"DocValuesField %q appears more than once in this document (only one value is allowed per field)",
			w.fieldInfo.Name(),
		)
	}
	if value == nil {
		return fmt.Errorf("field %q: null value not allowed", w.fieldInfo.Name())
	}
	if value.Length > binaryDVWriterMaxLength {
		return fmt.Errorf(
			"DocValuesField %q is too large, must be <= %d",
			w.fieldInfo.Name(), binaryDVWriterMaxLength,
		)
	}

	w.values.Append(value)
	if err := w.docsWithField.Add(docID); err != nil {
		return err
	}
	w.updateBytesUsed()
	w.lastDocID = docID
	return nil
}

// updateBytesUsed reconciles the iwBytesUsed counter with the writer's
// current memory footprint.
func (w *BinaryDocValuesWriter) updateBytesUsed() {
	newBytesUsed := w.values.BytesUsed()
	w.iwBytesUsed.AddAndGet(newBytesUsed - w.bytesUsed)
	w.bytesUsed = newBytesUsed
}

// docsWithFieldDocs returns the docIDs in the order they were added. The
// existing DocsWithFieldSet exposes no iterator; we synthesise a slice from
// either its dense prefix or its sparse bitset, mirroring the helper used by
// the sibling DocValuesWriters.
func (w *BinaryDocValuesWriter) docsWithFieldDocs() []int {
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

// newBufferedView builds a fresh in-RAM BinaryDocValues over the buffered
// state. Each call obtains an independent cursor so views are not shared.
func (w *BinaryDocValuesWriter) newBufferedView() *bufferedBinaryDocValues {
	return newBufferedBinaryDocValues(w.values, w.docsWithFieldDocs())
}

// GetDocValues materialises an in-memory BinaryDocValues view of the
// buffered state. Mirrors the Java getDocValues() / DocValuesWriter contract.
func (w *BinaryDocValuesWriter) GetDocValues() (BinaryDocValues, error) {
	return w.newBufferedView(), nil
}

// BinaryFieldConsumer is the callback used by Flush to hand the buffered
// BinaryDocValues to the underlying codec consumer.
//
// Gocene divergence: replaces the Java DocValuesConsumer.addBinaryField +
// EmptyDocValuesProducer.getBinary anonymous override with a simple
// function-typed boundary. The wiring layer in the codecs package adapts
// codecs.DocValuesConsumer to this signature.
type BinaryFieldConsumer func(field *FieldInfo, values BinaryDocValues) error

// Flush hands the buffered state to consumer. When sortMap is non-nil the
// values are re-mapped via the segment's IndexSorter docmap.
//
// Gocene divergence: maxDoc is passed in explicitly rather than read from
// SegmentWriteState because Gocene's SegmentWriteState in the index package
// does not yet carry SegmentInfo.MaxDoc (same convention as the sibling
// DocValuesWriters).
func (w *BinaryDocValuesWriter) Flush(
	maxDoc int,
	sortMap SorterDocMap,
	consumer BinaryFieldConsumer,
) error {
	if consumer == nil {
		return errors.New("BinaryDocValuesWriter.Flush: consumer must not be nil")
	}
	if sortMap == nil {
		return consumer(w.fieldInfo, w.newBufferedView())
	}
	sorted, err := newBinaryDVs(maxDoc, sortMap, w.newBufferedView())
	if err != nil {
		return err
	}
	return consumer(w.fieldInfo, newSortingBinaryDocValues(sorted))
}

// ============================================================================
// bufferedBinaryDocValues — iterates the values held in RAM.
// ============================================================================

// bufferedBinaryDocValues walks the docs-with-field slice and resolves each
// value from the backing BytesRefArray by insertion index. Mirrors the Java
// BufferedBinaryDocValues inner class.
type bufferedBinaryDocValues struct {
	values        *util.BytesRefArray
	docsWithField []int
	value         []byte
	cursor        int
	docID         int
}

// newBufferedBinaryDocValues constructs an in-RAM view over values, iterated
// in the order recorded by docsWithField.
func newBufferedBinaryDocValues(
	values *util.BytesRefArray,
	docsWithField []int,
) *bufferedBinaryDocValues {
	return &bufferedBinaryDocValues{
		values:        values,
		docsWithField: docsWithField,
		cursor:        -1,
		docID:         -1,
	}
}

// DocID returns the current docID, or NO_MORE_DOCS when exhausted.
func (b *bufferedBinaryDocValues) DocID() int { return b.docID }

// NextDoc advances to the next doc that has a value and resolves its bytes.
func (b *bufferedBinaryDocValues) NextDoc() (int, error) {
	b.cursor++
	if b.cursor >= len(b.docsWithField) {
		b.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	b.docID = b.docsWithField[b.cursor]
	b.value = b.values.GetBytes(b.cursor)
	return b.docID, nil
}

// Advance is unsupported; the Java original throws
// UnsupportedOperationException.
func (b *bufferedBinaryDocValues) Advance(int) (int, error) {
	return 0, errors.New("bufferedBinaryDocValues: Advance is not supported; use NextDoc")
}

// AdvanceExact is unsupported on the buffered writer view; the Java
// reference also forbids random access on these consumer-side iterators.
// T4709-added shim to satisfy the BinaryDocValues interface; callers
// must drive iteration via NextDoc.
func (b *bufferedBinaryDocValues) AdvanceExact(int) (bool, error) {
	return false, errors.New("bufferedBinaryDocValues: AdvanceExact is not supported; use NextDoc")
}

// BinaryValue returns the bytes bound to the current cursor position.
// Mirrors org.apache.lucene.index.BinaryDocValues#binaryValue.
func (b *bufferedBinaryDocValues) BinaryValue() ([]byte, error) {
	return b.value, nil
}

// Cost returns the number of value-bearing documents iterated by this
// writer-side view.
func (b *bufferedBinaryDocValues) Cost() int64 {
	return int64(len(b.docsWithField))
}

// ============================================================================
// binaryDVs — the docmap-remapped value store.
// ============================================================================

// binaryDVs holds every buffered value re-keyed by new docID. offsets[d] is a
// one-based index into values for new docID d; 0 means "no value". Mirrors
// the Java BinaryDVs inner class.
type binaryDVs struct {
	offsets []int
	values  *util.BytesRefArray
}

// newBinaryDVs walks oldValues in old-doc order, appends every value to a
// BytesRefArray and records its one-based slot under the new docID produced
// by sortMap. Mirrors the Java BinaryDVs constructor.
func newBinaryDVs(maxDoc int, sortMap SorterDocMap, oldValues BinaryDocValues) (*binaryDVs, error) {
	d := &binaryDVs{
		offsets: make([]int, maxDoc),
		values:  util.NewBytesRefArray(util.ByteBlockSize),
	}
	offset := 1 // 0 means no values for this document
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
			return nil, fmt.Errorf(
				"newBinaryDVs: sortMap.OldToNew(%d)=%d outside [0..%d)", docID, newDocID, maxDoc)
		}
		// docID is the current cursor — BinaryValue is equivalent to Get(docID)
		// without the cursor identity check.
		v, err := oldValues.BinaryValue()
		if err != nil {
			return nil, err
		}
		d.values.AppendBytes(v)
		d.offsets[newDocID] = offset
		offset++
	}
	return d, nil
}

// ============================================================================
// sortingBinaryDocValues — sort-aware view used by Flush when sortMap != nil.
// ============================================================================

// sortingBinaryDocValues iterates a binaryDVs store in new-doc order. Mirrors
// the Java SortingBinaryDocValues inner class: Advance throws, and values are
// resolved lazily through the BytesRefArray.
type sortingBinaryDocValues struct {
	dvs   *binaryDVs
	docID int
}

func newSortingBinaryDocValues(dvs *binaryDVs) *sortingBinaryDocValues {
	return &sortingBinaryDocValues{dvs: dvs, docID: -1}
}

// DocID returns the current docID, or NO_MORE_DOCS when exhausted.
func (s *sortingBinaryDocValues) DocID() int { return s.docID }

// NextDoc advances to the next doc that has a value.
func (s *sortingBinaryDocValues) NextDoc() (int, error) {
	for {
		s.docID++
		if s.docID == len(s.dvs.offsets) {
			s.docID = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		if s.dvs.offsets[s.docID] > 0 {
			return s.docID, nil
		}
	}
}

// Advance is unsupported; the Java original throws
// UnsupportedOperationException("use nextDoc instead").
func (s *sortingBinaryDocValues) Advance(int) (int, error) {
	return 0, errors.New("sortingBinaryDocValues: Advance is not supported; use NextDoc")
}

// AdvanceExact is unsupported on the sort-aware writer view, matching
// the Java reference. T4709-added shim to satisfy BinaryDocValues.
func (s *sortingBinaryDocValues) AdvanceExact(int) (bool, error) {
	return false, errors.New("sortingBinaryDocValues: AdvanceExact is not supported; use NextDoc")
}

// BinaryValue returns the bytes bound to the current cursor position.
// Mirrors org.apache.lucene.index.BinaryDocValues#binaryValue.
func (s *sortingBinaryDocValues) BinaryValue() ([]byte, error) {
	if s.docID < 0 || s.docID == NO_MORE_DOCS || s.docID >= len(s.dvs.offsets) {
		return nil, fmt.Errorf(
			"sortingBinaryDocValues: BinaryValue requires a positioned cursor; current=%d", s.docID)
	}
	return s.dvs.values.GetBytes(s.dvs.offsets[s.docID] - 1), nil
}

// Cost returns the number of value-bearing documents represented by the
// sort-aware writer view.
func (s *sortingBinaryDocValues) Cost() int64 {
	var n int64
	for _, off := range s.dvs.offsets {
		if off > 0 {
			n++
		}
	}
	return n
}
