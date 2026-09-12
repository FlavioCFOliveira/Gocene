// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
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

// BinaryDocValuesWriter buffers up pending byte[] per doc, then flushes when segment flushes.
type BinaryDocValuesWriter struct {
	bytes      *store.PagedBytes
	bytesOut   *store.PagedBytesDataOutput
	iwBytesUsed *util.Counter

	lengths      *packed.PackedLongValuesBuilder
	docsWithField *DocsWithFieldSet
	fieldInfo    spi.FieldInfo
	bytesUsed    int64
	lastDocID    int
	maxLength    int
	frozen       bool

	finalLengths *packed.PackedLongValues
}

// NewBinaryDocValuesWriter creates a new BinaryDocValuesWriter.
func NewBinaryDocValuesWriter(fieldInfo spi.FieldInfo, iwBytesUsed *util.Counter) (*BinaryDocValuesWriter, error) {
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
		bytes:        bytes,
		bytesOut:     bytesOut,
		iwBytesUsed:  iwBytesUsed,
		lengths:      lengths,
		docsWithField: docsWithField,
		fieldInfo:    fieldInfo,
		bytesUsed:    bytesUsed,
		lastDocID:    -1,
		maxLength:    0,
	}, nil
}

// AddValue adds a binary value for the given docID.
func (w *BinaryDocValuesWriter) AddValue(docID int, value *util.BytesRef) error {
	if docID <= w.lastDocID {
		return fmt.Errorf("DocValuesField %q appears more than once in this document (only one value is allowed per field)", w.fieldInfo.Name)
	}
	if value == nil {
		return fmt.Errorf("field=%q: null value not allowed", w.fieldInfo.Name)
	}
	if value.Length > maxBinaryLength {
		return fmt.Errorf("DocValuesField %q is too large, must be <= %d", w.fieldInfo.Name, maxBinaryLength)
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

	return &bufferedBinaryDocValues{
		finalLengths:   w.finalLengths,
		maxLength:      w.maxLength,
		bytesIterator:   bytesIn,
		docsWithField:   w.docsWithField.Iterator(),
		value:           util.NewBytesRefBuilder(),
	}
}

// Flush writes the binary doc values to the consumer.
func (w *BinaryDocValuesWriter) Flush(state *spi.SegmentWriteState, sortMap spi.SorterDocMap, consumer spi.DocValuesConsumer) error {
	w.freezeBytes()
	if w.finalLengths == nil {
		w.finalLengths = w.lengths.Build()
	}

	var sorted BinaryDocValues
	if sortMap != nil {
		sorted = NewBinaryDVs(
			state.SegmentInfo.DocCount(),
			sortMap,
			w.GetDocValues(),
		)
	}

	var values BinaryDocValues
	if sorted == nil {
		values = w.GetDocValues()
	} else {
		values = sorted
	}

	return consumer.AddBinaryField(&w.fieldInfo, &binaryDocValuesWriterIterator{dvs: values})

}

type bufferedBinaryDocValues struct {
	value           *util.BytesRefBuilder
	lengthsIterator *packed.PackedLongValuesIterator
	docsWithField   util.DocIdSetIterator
	bytesIterator   *store.PagedBytesDataInput
	maxLength       int
	finalLengths    *packed.PackedLongValues
}

func (b *bufferedBinaryDocValues) init() {
	if b.value == nil {
		b.value = util.NewBytesRefBuilder()
		b.value.Grow(b.maxLength)
	}
	if b.lengthsIterator == nil {
		b.lengthsIterator = b.finalLengths.Iterator()
	}
}

func (b *bufferedBinaryDocValues) DocID() int {
	return b.docsWithField.DocID()
}

func (b *bufferedBinaryDocValues) NextDoc() (int, error) {
	b.init()
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

func (b *bufferedBinaryDocValues) DocIDRunEnd() int {
	return b.docsWithField.DocIDRunEnd()
}

func (b *bufferedBinaryDocValues) BinaryValue() ([]byte, error) {
	return b.value.Get().ValidBytes(), nil
}

type sortingBinaryDocValues struct {
	dvs   *binaryDVs
	spare *util.BytesRef
	docID int
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

func NewBinaryDVs(maxDoc int, sortMap spi.SorterDocMap, oldValues BinaryDocValues) BinaryDocValues {
	offsets := make([]int, maxDoc)
	values := util.NewBytesRefArray(4096)
	offset := 1 // 0 means no values for this document

	docID, err := oldValues.NextDoc()
	for err == nil && docID != util.NO_MORE_DOCS {
		newDocID := sortMap.OldToNew(docID)
		val, errVal := oldValues.BinaryValue()
		if errVal != nil {
			// In-memory buffered values should not fail.
			panic(errVal)
		}
		values.AppendBytes(val)
		offsets[newDocID] = offset
		offset++
		docID, err = oldValues.NextDoc()
	}

	return &sortingBinaryDocValues{
		dvs: &binaryDVs{
			offsets: offsets,
			values:  values,
		},
		spare: util.NewBytesRefEmpty(),
		docID: -1,
	}
}

type binaryDocValuesWriterIterator struct {
	dvs BinaryDocValues
}

func (it *binaryDocValuesWriterIterator) Next() bool {
	doc, err := it.dvs.NextDoc()
	if err != nil || doc == util.NO_MORE_DOCS {
		return false
	}
	return true
}

func (it *binaryDocValuesWriterIterator) DocID() int {
	return it.dvs.DocID()
}

func (it *binaryDocValuesWriterIterator) Value() []byte {
	val, err := it.dvs.BinaryValue()
	if err != nil {
		panic(err)
	}
	return val
}
