// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// flushDocValues writes the buffered doc-values for every doc-values field to
// the codec's DocValuesConsumer, serialising the per-segment .dvd / .dvm
// files.
//
// It mirrors the docValuesConsumer step of Lucene's IndexingChain.flush: a
// single DocValuesConsumer is opened for the segment and each field is added
// once, in field-number order, replaying the buffered per-document values in
// document order. The NUMERIC / BINARY / SORTED_NUMERIC value types are
// driven through the codec's writer-side iterator surface
// (DocValuesConsumer.AddNumericField / AddBinaryField / AddSortedNumericField).
// SORTED / SORTED_SET require the per-ordinal term bytes, so they are routed
// through the index-side SortedDocValuesWriter / SortedSetDocValuesWriter
// (which build the ord->bytes table) and the codec consumer's read-side
// FromReader entry points (see sortedDVConsumerDelegate).
//
// The FieldInfo objects are taken from state.FieldInfos — the same instances
// flushFieldInfos serialises to the .fnm — so the FieldInfo doc-values type
// reaches disk and FieldInfos.HasDocValues() reports true on reopen, lighting
// up the codec DocValuesProducer.
//
// No-op when the codec has no DocValuesFormat or no doc-values fields were
// buffered.
func (dwpt *DocumentsWriterPerThread) flushDocValues(codec Codec, state *SegmentWriteState) error {
	if codec == nil || codec.DocValuesFormat() == nil {
		return nil
	}
	if len(dwpt.docValues) == 0 {
		return nil
	}

	// Collect the doc-values fields from the on-disk FieldInfos, preserving
	// field-number order so the Add* sequence (and thus the per-field meta
	// records) is deterministic across runs.
	type dvField struct {
		fieldInfo *FieldInfo
		buf       *DocValuesBuffer
	}
	var dvFields []dvField
	it := state.FieldInfos.Iterator()
	for {
		fi := it.Next()
		if fi == nil {
			break
		}
		if !fi.DocValuesType().HasDocValues() {
			continue
		}
		buf, ok := dwpt.docValues[fi.Name()]
		if !ok {
			continue
		}
		dvFields = append(dvFields, dvField{fieldInfo: fi, buf: buf})
	}
	if len(dvFields) == 0 {
		return nil
	}

	consumer, err := codec.DocValuesFormat().FieldsConsumer(state)
	if err != nil {
		return fmt.Errorf("doc values FieldsConsumer: %w", err)
	}
	defer consumer.Close()

	for _, df := range dvFields {
		if err := dwpt.flushDocValuesField(consumer, df.fieldInfo, df.buf); err != nil {
			return fmt.Errorf("doc values field %q: %w", df.fieldInfo.Name(), err)
		}
	}
	return nil
}

// sortedDVConsumerDelegate is the read-side entry point onto the codec's
// doc-values consumer for the SORTED / SORTED_SET value types, satisfied
// structurally by *codecs.Lucene90DocValuesConsumer (which the index package
// cannot name due to the codecs->index import direction). The reset closures
// re-materialise a fresh forward iterator each pass because the consumer reads
// the values more than once. index.FieldInfo / index.SortedDocValues /
// index.SortedSetDocValues are type aliases of their schema/spi counterparts,
// so the method set matches the codecs concrete exactly.
type sortedDVConsumerDelegate interface {
	AddSortedFieldFromReader(field *FieldInfo, reset func() (SortedDocValues, error)) error
	AddSortedSetFieldFromReader(field *FieldInfo, reset func() (SortedSetDocValues, error)) error
}

// flushDocValuesField writes a single doc-values field to consumer, dispatching
// on the field's DocValuesType.
func (dwpt *DocumentsWriterPerThread) flushDocValuesField(
	consumer DocValuesConsumer,
	fieldInfo *FieldInfo,
	buf *DocValuesBuffer,
) error {
	switch fieldInfo.DocValuesType() {
	case DocValuesTypeNumeric:
		return consumer.AddNumericField(fieldInfo, &bufferedNumericDVIter{
			docIDs: buf.docIDs,
			values: buf.numericValues,
			pos:    -1,
		})
	case DocValuesTypeBinary:
		return consumer.AddBinaryField(fieldInfo, &bufferedBinaryDVIter{
			docIDs: buf.docIDs,
			values: buf.binaryValues,
			pos:    -1,
		})
	case DocValuesTypeSortedNumeric:
		return consumer.AddSortedNumericField(fieldInfo, &bufferedSortedNumericDVIter{
			docIDs: buf.docIDs,
			values: buf.numericValuesMulti,
			pos:    -1,
		})
	case DocValuesTypeSorted:
		return dwpt.flushSortedDocValuesField(consumer, fieldInfo, buf)
	case DocValuesTypeSortedSet:
		return dwpt.flushSortedSetDocValuesField(consumer, fieldInfo, buf)
	default:
		return fmt.Errorf("unsupported doc values type %v", fieldInfo.DocValuesType())
	}
}

// flushSortedDocValuesField feeds the buffered single binary values into a
// SortedDocValuesWriter (which builds the ord->bytes table) and writes the
// field through the codec consumer's read-side FromReader entry point.
func (dwpt *DocumentsWriterPerThread) flushSortedDocValuesField(
	consumer DocValuesConsumer,
	fieldInfo *FieldInfo,
	buf *DocValuesBuffer,
) error {
	delegate, ok := consumer.(sortedDVConsumerDelegate)
	if !ok {
		return fmt.Errorf("doc values consumer does not support SORTED fields")
	}
	build := func() (*SortedDocValuesWriter, error) {
		w := NewSortedDocValuesWriter(fieldInfo, util.NewCounter(),
			util.NewByteBlockPool(util.NewDirectAllocator()))
		for i, docID := range buf.docIDs {
			if err := w.AddValue(docID, &util.BytesRef{
				Bytes:  buf.binaryValues[i],
				Offset: 0,
				Length: len(buf.binaryValues[i]),
			}); err != nil {
				return nil, err
			}
		}
		return w, nil
	}
	reset := func() (SortedDocValues, error) {
		w, err := build()
		if err != nil {
			return nil, err
		}
		return w.GetDocValues()
	}
	return delegate.AddSortedFieldFromReader(fieldInfo, reset)
}

// flushSortedSetDocValuesField feeds the buffered multi binary values into a
// SortedSetDocValuesWriter and writes the field through the codec consumer's
// read-side FromReader entry point.
func (dwpt *DocumentsWriterPerThread) flushSortedSetDocValuesField(
	consumer DocValuesConsumer,
	fieldInfo *FieldInfo,
	buf *DocValuesBuffer,
) error {
	delegate, ok := consumer.(sortedDVConsumerDelegate)
	if !ok {
		return fmt.Errorf("doc values consumer does not support SORTED_SET fields")
	}
	build := func() (*SortedSetDocValuesWriter, error) {
		w := NewSortedSetDocValuesWriter(fieldInfo, util.NewCounter(),
			util.NewByteBlockPool(util.NewDirectAllocator()))
		for i, docID := range buf.docIDs {
			for _, v := range buf.binaryValuesMulti[i] {
				if err := w.AddValue(docID, &util.BytesRef{
					Bytes:  v,
					Offset: 0,
					Length: len(v),
				}); err != nil {
					return nil, err
				}
			}
		}
		return w, nil
	}
	reset := func() (SortedSetDocValues, error) {
		w, err := build()
		if err != nil {
			return nil, err
		}
		return w.GetDocValues()
	}
	return delegate.AddSortedSetFieldFromReader(fieldInfo, reset)
}

// ---------------------------------------------------------------------------
// Writer-side iterators backed by the in-memory DocValuesBuffer slices.
// These satisfy the spi writer-side iterator contracts the codec
// DocValuesConsumer.AddNumericField / AddBinaryField / AddSortedNumericField
// accept (NumericDocValuesIterator / BinaryDocValuesIterator /
// SortedNumericDocValuesIterator). docIDs is strictly increasing.
// ---------------------------------------------------------------------------

// bufferedNumericDVIter replays a NUMERIC field's buffered values.
type bufferedNumericDVIter struct {
	docIDs []int
	values []int64
	pos    int
}

func (it *bufferedNumericDVIter) Next() bool {
	it.pos++
	return it.pos < len(it.docIDs)
}
func (it *bufferedNumericDVIter) DocID() int   { return it.docIDs[it.pos] }
func (it *bufferedNumericDVIter) Value() int64 { return it.values[it.pos] }

// bufferedBinaryDVIter replays a BINARY field's buffered values.
type bufferedBinaryDVIter struct {
	docIDs []int
	values [][]byte
	pos    int
}

func (it *bufferedBinaryDVIter) Next() bool {
	it.pos++
	return it.pos < len(it.docIDs)
}
func (it *bufferedBinaryDVIter) DocID() int    { return it.docIDs[it.pos] }
func (it *bufferedBinaryDVIter) Value() []byte { return it.values[it.pos] }

// bufferedSortedNumericDVIter replays a SORTED_NUMERIC field's buffered
// multi-values. Each document exposes DocValueCount() values via repeated
// NextValue() calls, matching the writer-side iterator contract.
type bufferedSortedNumericDVIter struct {
	docIDs   []int
	values   [][]int64
	pos      int
	valueIdx int
}

func (it *bufferedSortedNumericDVIter) NextDoc() bool {
	it.pos++
	it.valueIdx = 0
	return it.pos < len(it.docIDs)
}
func (it *bufferedSortedNumericDVIter) DocID() int         { return it.docIDs[it.pos] }
func (it *bufferedSortedNumericDVIter) DocValueCount() int { return len(it.values[it.pos]) }
func (it *bufferedSortedNumericDVIter) NextValue() int64 {
	v := it.values[it.pos][it.valueIdx]
	it.valueIdx++
	return v
}
