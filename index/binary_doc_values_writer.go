package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
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

	lengths      *util.PackedLongValuesBuilder
	docsWithField *DocsWithFieldSet
	fieldInfo    schema.FieldInfo
	bytesUsed    int64
	lastDocID    int
	maxLength    int
	frozen       bool

	finalLengths *util.PackedLongValues
}

// NewBinaryDocValuesWriter creates a new BinaryDocValuesWriter.
func NewBinaryDocValuesWriter(fieldInfo schema.FieldInfo, iwBytesUsed *util.Counter) *BinaryDocValuesWriter {
	bytes := util.NewPagedBytes(blockBits)
	bytesOut := bytes.GetDataOutput()
	lengths := util.NewPackedLongValuesDeltaBuilder(util.Compact)
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
	}
}

// AddValue adds a binary value for the given docID.
func (w *BinaryDocValuesWriter) AddValue(docID int, value *util.BytesRef) error {
	if docID <= w.lastDocID {
		return fmt.Errorf("DocValuesField %q appears more than once in this document (only one value is allowed per field)", w.fieldInfo.Name)
	}
	if value == nil {
		return fmt.Errorf("field=%q: null value not allowed", w.fieldInfo.Name)
	}
	if value.Length() > maxBinaryLength {
		return fmt.Errorf("DocValuesField %q is too large, must be <= %d", w.fieldInfo.Name, maxBinaryLength)
	}

	if value.Length() > w.maxLength {
		w.maxLength = value.Length()
	}
	w.lengths.Add(int64(value.Length()))

	if err := w.bytesOut.WriteBytes(value.Bytes(), value.Offset(), value.Length()); err != nil {
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
	return &bufferedBinaryDocValues{
		finalLengths:   w.finalLengths,
		maxLength:      w.maxLength,
		bytesIterator:   w.bytes.GetDataInput(),
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

	var sorted *binaryDVs
	if sortMap != nil {
		sorted = NewBinaryDVs(
			state.SegmentInfo.MaxDoc(),
			sortMap,
			w.GetDocValues(),
		)
	}

	err := consumer.AddBinaryField(w.fieldInfo, func(fieldInfoIn schema.FieldInfo) BinaryDocValues {
		if fieldInfoIn != w.fieldInfo {
			panic("wrong fieldInfo")
		}
		if sorted == nil {
			return w.GetDocValues()
		}
		return &sortingBinaryDocValues{
			dvs:   sorted,
			spare: util.NewBytesRefBuilder(),
			docID: -1,
		}
	})
	if err != nil {
		return err
	}

	return nil
}

type bufferedBinaryDocValues struct {
	value           *util.BytesRefBuilder
	lengthsIterator *util.PackedLongValuesIterator
	docsWithField   util.DocIdSetIterator
	bytesIterator   *store.PagedBytesDataInput
	maxLength       int
	finalLengths    *util.PackedLongValues
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
	if docID != util.NoMoreDocs {
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
	return b.docsWithField.IntoBitSet(upTo, bitSet, offset)
}

func (b *bufferedBinaryDocValues) DocIDRunEnd() (int, error) {
	return b.docsWithField.DocIDRunEnd()
}

func (b *bufferedBinaryDocValues) BinaryValue() (*util.BytesRef, error) {
	return b.value.Get(), nil
}

type sortingBinaryDocValues struct {
	dvs   *binaryDVs
	spare *util.BytesRefBuilder
	docID int
}

func (s *sortingBinaryDocValues) NextDoc() (int, error) {
	for {
		s.docID++
		if s.docID == len(s.dvs.offsets) {
			s.docID = util.NoMoreDocs
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

func (s *sortingBinaryDocValues) BinaryValue() (*util.BytesRef, error) {
	s.dvs.values.Get(s.spare, s.dvs.offsets[s.docID]-1)
	return s.spare.Get(), nil
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

func NewBinaryDVs(maxDoc int, sortMap spi.SorterDocMap, oldValues BinaryDocValues) *binaryDVs {
	offsets := make([]int, maxDoc)
	values := util.NewBytesRefArray(util.NewCounter())
	offset := 1 // 0 means no values for this document

	docID, err := oldValues.NextDoc()
	for err == nil && docID != util.NoMoreDocs {
		newDocID := sortMap.OldToNew(docID)
		val, errVal := oldValues.BinaryValue()
		if errVal != nil {
			// In-memory buffered values should not fail.
			panic(errVal)
		}
		values.Append(val)
		offsets[newDocID] = offset
		offset++
		docID, err = oldValues.NextDoc()
	}

	return &binaryDVs{
		offsets: offsets,
		values:  values,
	}
}
