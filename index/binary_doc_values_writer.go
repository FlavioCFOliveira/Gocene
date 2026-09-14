// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

const (
	// maxBinaryLength is the maximum length for a binary field.
	maxBinaryLength = util.MaxArrayLength
	// blockBits is the block size for PagedBytes storage.
	blockBits = 12
)

// BinaryDocValuesWriter buffers up pending byte[] per doc, then flushes when
// segment flushes. Go port of org.apache.lucene.index.BinaryDocValuesWriter.
type BinaryDocValuesWriter struct {
	bytes       *store.PagedBytes
	bytesOut    *store.PagedBytesDataOutput
	iwBytesUsed *util.Counter

	lengths       *packed.PackedLongValuesBuilder
	docsWithField *DocsWithFieldSet
	fieldInfo     *FieldInfo
	bytesUsed     int64
	lastDocID     int
	maxLength     int
	frozen        bool

	finalLengths *packed.PackedLongValues
}

// NewBinaryDocValuesWriter creates a new BinaryDocValuesWriter.
func NewBinaryDocValuesWriter(fieldInfo *FieldInfo, iwBytesUsed *util.Counter) (*BinaryDocValuesWriter, error) {
	bytes, err := store.NewPagedBytes(blockBits)
	if err != nil {
		return nil, err
	}
	bytesOut := bytes.GetDataOutput()
	lengths, err := packed.DeltaPackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
	if err != nil {
		return nil, fmt.Errorf("failed to create packed long values builder: %w", err)
	}
	docsWithField := NewDocsWithFieldSet()

	// Calculate initial RAM usage.
	bytesUsed := lengths.RamBytesUsed() + docsWithField.RamBytesUsed()
	iwBytesUsed.AddAndGet(bytesUsed)

	return &BinaryDocValuesWriter{
		bytes:         bytes,
		bytesOut:      bytesOut,
		iwBytesUsed:   iwBytesUsed,
		lengths:       lengths,
		docsWithField: docsWithField,
		fieldInfo:     fieldInfo,
		bytesUsed:     bytesUsed,
		lastDocID:     -1,
		maxLength:     0,
	}, nil
}

// AddValue adds a binary value for the given docID.
func (w *BinaryDocValuesWriter) AddValue(docID int, value *util.BytesRef) error {
	if docID <= w.lastDocID {
		return fmt.Errorf("DocValuesField %q appears more than once in this document (only one value is allowed per field)", w.fieldInfo.Name())
	}
	if value == nil {
		return fmt.Errorf("field=%q: null value not allowed", w.fieldInfo.Name())
	}
	if value.Length > maxBinaryLength {
		return fmt.Errorf("DocValuesField %q is too large, must be <= %d", w.fieldInfo.Name(), maxBinaryLength)
	}

	if value.Length > w.maxLength {
		w.maxLength = value.Length
	}
	w.lengths.Add(int64(value.Length))

	if err := w.bytesOut.WriteBytes(value.Bytes, value.Offset, value.Length); err != nil {
		return fmt.Errorf("failed to write bytes to PagedBytes: %w", err)
	}
	w.docsWithField.Add(docID)
	w.updateBytesUsed()

	w.lastDocID = docID
	return nil
}

func (w *BinaryDocValuesWriter) updateBytesUsed() {
	newBytesUsed := w.lengths.RamBytesUsed() + w.bytes.RamBytesUsed() + w.docsWithField.RamBytesUsed()
	w.iwBytesUsed.AddAndGet(newBytesUsed - w.bytesUsed)
	w.bytesUsed = newBytesUsed
}

// freezeBytes freezes the PagedBytes exactly once. GetDocValues may be called
// before Flush (e.g. when index sorting on this field reads the in-RAM
// values), so Flush cannot rely on being the first to freeze.
func (w *BinaryDocValuesWriter) freezeBytes() {
	if !w.frozen {
		w.bytes.Freeze(false)
		w.frozen = true
	}
}

// GetDocValues returns the doc values for the current segment.
func (w *BinaryDocValuesWriter) GetDocValues() BinaryDocValues {
	if w.finalLengths == nil {
		w.finalLengths = w.lengths.Build()
	}
	w.freezeBytes()

	bytesIn, err := w.bytes.GetDataInput()
	if err != nil {
		panic(err) // Should not happen after freeze
	}

	return newBufferedBinaryDocValues(w.finalLengths, w.maxLength, bytesIn, w.docsWithField.Iterator())
}

// Flush hands the buffered values to dvConsumer.AddBinaryField. When sortMap
// is non-nil the values are re-mapped via the segment's IndexSorter docmap.
// Mirrors BinaryDocValuesWriter.flush(SegmentWriteState, Sorter.DocMap,
// DocValuesConsumer).
func (w *BinaryDocValuesWriter) Flush(state *SegmentWriteState, sortMap SorterDocMap, dvConsumer DocValuesConsumer) error {
	if dvConsumer == nil {
		return errors.New("BinaryDocValuesWriter.Flush: consumer must not be nil")
	}
	w.freezeBytes()
	if w.finalLengths == nil {
		w.finalLengths = w.lengths.Build()
	}
	var sorted *binaryDVs
	if sortMap != nil {
		bytesIn, err := w.bytes.GetDataInput()
		if err != nil {
			return err
		}
		s, err := newBinaryDVs(
			state.SegmentInfo.MaxDoc(),
			sortMap,
			newBufferedBinaryDocValues(w.finalLengths, w.maxLength, bytesIn, w.docsWithField.Iterator()),
		)
		if err != nil {
			return err
		}
		sorted = s
	}
	return dvConsumer.AddBinaryField(w.fieldInfo, &binaryDocValuesWriterDocValuesProducer{w: w, sorted: sorted})
}

// binaryDocValuesWriterDocValuesProducer is the anonymous
// EmptyDocValuesProducer subclass built by flush.
type binaryDocValuesWriterDocValuesProducer struct {
	EmptyDocValuesProducer
	w      *BinaryDocValuesWriter
	sorted *binaryDVs
}

// GetBinary returns the buffered values, or their sorted view.
func (p *binaryDocValuesWriterDocValuesProducer) GetBinary(fieldInfoIn *FieldInfo) (BinaryDocValues, error) {
	if fieldInfoIn != p.w.fieldInfo {
		return nil, errors.New("wrong fieldInfo")
	}
	if p.sorted == nil {
		bytesIn, err := p.w.bytes.GetDataInput()
		if err != nil {
			return nil, err
		}
		return newBufferedBinaryDocValues(p.w.finalLengths, p.w.maxLength, bytesIn, p.w.docsWithField.Iterator()), nil
	}
	return newSortingBinaryDocValues(p.sorted), nil
}

// GetMergeInstance returns the receiver (DocValuesProducer default).
func (p *binaryDocValuesWriterDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

// bufferedBinaryDocValues iterates over the values we have in ram. Mirrors
// BinaryDocValuesWriter.BufferedBinaryDocValues.
type bufferedBinaryDocValues struct {
	value           *util.BytesRefBuilder
	lengthsIterator *packed.PackedLongValuesIterator
	docsWithField   util.DocIdSetIterator
	bytesIterator   *store.PagedBytesDataInput
}

func newBufferedBinaryDocValues(
	lengths *packed.PackedLongValues,
	maxLength int,
	bytesIterator *store.PagedBytesDataInput,
	docsWithFields util.DocIdSetIterator,
) *bufferedBinaryDocValues {
	value := util.NewBytesRefBuilder()
	value.Grow(maxLength)
	return &bufferedBinaryDocValues{
		value:           value,
		lengthsIterator: lengths.Iterator(),
		bytesIterator:   bytesIterator,
		docsWithField:   docsWithFields,
	}
}

func (b *bufferedBinaryDocValues) DocID() int {
	return b.docsWithField.DocID()
}

func (b *bufferedBinaryDocValues) NextDoc() (int, error) {
	docID, err := b.docsWithField.NextDoc()
	if err != nil {
		return docID, err
	}
	if docID != util.NO_MORE_DOCS {
		length := int(b.lengthsIterator.Next())
		b.value.SetLength(length)
		if err := b.bytesIterator.ReadBytes(b.value.Bytes(), 0, length); err != nil {
			return docID, err
		}
	}
	return docID, nil
}

func (b *bufferedBinaryDocValues) Advance(target int) (int, error) {
	panic("unsupported")
}

func (b *bufferedBinaryDocValues) AdvanceExact(target int) (bool, error) {
	panic("unsupported")
}

func (b *bufferedBinaryDocValues) Cost() int64 {
	return b.docsWithField.Cost()
}

func (b *bufferedBinaryDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	it := b.docsWithField
	for {
		doc, err := it.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS || doc >= upTo {
			break
		}
		bitSet.Set(doc + offset)
	}
	return nil
}

func (b *bufferedBinaryDocValues) DocIDRunEnd() (int, error) {
	return b.docsWithField.DocIDRunEnd()
}

func (b *bufferedBinaryDocValues) BinaryValue() ([]byte, error) {
	return b.value.Get().ValidBytes(), nil
}

// sortingBinaryDocValues mirrors BinaryDocValuesWriter.SortingBinaryDocValues.
type sortingBinaryDocValues struct {
	dvs   *binaryDVs
	spare *util.BytesRef
	docID int
}

func newSortingBinaryDocValues(dvs *binaryDVs) *sortingBinaryDocValues {
	return &sortingBinaryDocValues{dvs: dvs, spare: util.NewBytesRefEmpty(), docID: -1}
}

func (s *sortingBinaryDocValues) NextDoc() (int, error) {
	for {
		s.docID++
		if s.docID == len(s.dvs.offsets) {
			s.docID = util.NO_MORE_DOCS
			return s.docID, nil
		}
		if s.dvs.offsets[s.docID] > 0 {
			return s.docID, nil
		}
	}
}

func (s *sortingBinaryDocValues) DocID() int {
	return s.docID
}

func (s *sortingBinaryDocValues) Advance(target int) (int, error) {
	panic("use NextDoc instead")
}

func (s *sortingBinaryDocValues) AdvanceExact(target int) (bool, error) {
	panic("use NextDoc instead")
}

func (s *sortingBinaryDocValues) BinaryValue() ([]byte, error) {
	s.dvs.values.Get(s.dvs.offsets[s.docID]-1, s.spare)
	return s.spare.ValidBytes(), nil
}

func (s *sortingBinaryDocValues) Cost() int64 {
	return int64(s.dvs.values.Size())
}

func (s *sortingBinaryDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	panic("unsupported")
}

func (s *sortingBinaryDocValues) DocIDRunEnd() (int, error) {
	panic("unsupported")
}

// binaryDVs mirrors BinaryDocValuesWriter.BinaryDVs: the values re-keyed into
// new-doc order.
type binaryDVs struct {
	offsets []int
	values  *util.BytesRefArray
}

func (b *binaryDVs) Advance(target int) (int, error) {
	panic("unsupported")
}

func (b *binaryDVs) AdvanceExact(target int) (bool, error) {
	panic("unsupported")
}

func newBinaryDVs(maxDoc int, sortMap SorterDocMap, oldValues BinaryDocValues) (*binaryDVs, error) {
	offsets := make([]int, maxDoc)
	values := util.NewBytesRefArray(4096)
	offset := 1 // 0 means no values for this document
	for {
		docID, err := oldValues.NextDoc()
		if err != nil {
			return nil, err
		}
		if docID == util.NO_MORE_DOCS {
			break
		}
		newDocID := sortMap.OldToNew(docID)
		val, err := oldValues.BinaryValue()
		if err != nil {
			return nil, err
		}
		values.AppendBytes(val)
		offsets[newDocID] = offset
		offset++
	}
	return &binaryDVs{offsets: offsets, values: values}, nil
}

// NewBinaryDVs returns a SortingBinaryDocValues over
// new BinaryDVs(maxDoc, sortMap, oldValues), the form SortingCodecReader uses.
func NewBinaryDVs(maxDoc int, sortMap SorterDocMap, oldValues BinaryDocValues) BinaryDocValues {
	dvs, err := newBinaryDVs(maxDoc, sortMap, oldValues)
	if err != nil {
		panic(err)
	}
	return newSortingBinaryDocValues(dvs)
}
