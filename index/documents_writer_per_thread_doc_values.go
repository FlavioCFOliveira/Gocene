// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocValuesBuffer buffers doc-values for a single field across multiple documents.
type DocValuesBuffer struct {
	docIDs             []int
	numericValues      []int64
	binaryValues       [][]byte
	numericValuesMulti [][]int64
	binaryValuesMulti  [][][]byte
}

// flushDocValues writes the buffered doc-values for every doc-values field to
// the codec's DocValuesConsumer, serialising the per-segment .dvd / .dvm
// files.
//
// It mirrors the docValuesConsumer step of Lucene's IndexingChain.flush: a
// single DocValuesConsumer is opened for the segment and each field is added
// once, in field-number order. Every field is replayed into the index-side
// DocValuesWriter of its type, whose flush hands the consumer a
// DocValuesProducer over the buffered values, exactly as
// IndexingChain.writeDocValues does with the per-field writers.
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
		if err := dwpt.flushDocValuesField(state, consumer, df.fieldInfo, df.buf); err != nil {
			return fmt.Errorf("doc values field %q: %w", df.fieldInfo.Name(), err)
		}
	}
	return nil
}

// flushDocValuesField replays a single doc-values field into the
// DocValuesWriter of its type and flushes that writer to consumer.
func (dwpt *DocumentsWriterPerThread) flushDocValuesField(
	state *SegmentWriteState,
	consumer DocValuesConsumer,
	fieldInfo *FieldInfo,
	buf *DocValuesBuffer,
) error {
	switch fieldInfo.DocValuesType() {
	case DocValuesTypeNumeric:
		w := NewNumericDocValuesWriter(fieldInfo, util.NewCounter())
		for i, docID := range buf.docIDs {
			if err := w.AddValue(docID, buf.numericValues[i]); err != nil {
				return err
			}
		}
		return w.Flush(state, nil, consumer)
	case DocValuesTypeBinary:
		w, err := NewBinaryDocValuesWriter(fieldInfo, util.NewCounter())
		if err != nil {
			return err
		}
		for i, docID := range buf.docIDs {
			if err := w.AddValue(docID, &util.BytesRef{
				Bytes:  buf.binaryValues[i],
				Offset: 0,
				Length: len(buf.binaryValues[i]),
			}); err != nil {
				return err
			}
		}
		return w.Flush(state, nil, consumer)
	case DocValuesTypeSortedNumeric:
		// SortedNumericDocValuesWriter sorts each document's values ascending
		// when the document is finished (the SortedNumericDocValues contract).
		w := NewSortedNumericDocValuesWriter(fieldInfo, util.NewCounter())
		for i, docID := range buf.docIDs {
			for _, v := range buf.numericValuesMulti[i] {
				if err := w.AddValue(docID, v); err != nil {
					return err
				}
			}
		}
		return w.Flush(state, nil, consumer)
	case DocValuesTypeSorted:
		w := NewSortedDocValuesWriter(fieldInfo, util.NewCounter(),
			util.NewByteBlockPool(util.NewDirectAllocator()))
		for i, docID := range buf.docIDs {
			if err := w.AddValue(docID, &util.BytesRef{
				Bytes:  buf.binaryValues[i],
				Offset: 0,
				Length: len(buf.binaryValues[i]),
			}); err != nil {
				return err
			}
		}
		return w.Flush(state, nil, consumer)
	case DocValuesTypeSortedSet:
		w := NewSortedSetDocValuesWriter(fieldInfo, util.NewCounter(),
			util.NewByteBlockPool(util.NewDirectAllocator()))
		for i, docID := range buf.docIDs {
			for _, v := range buf.binaryValuesMulti[i] {
				if err := w.AddValue(docID, &util.BytesRef{
					Bytes:  v,
					Offset: 0,
					Length: len(v),
				}); err != nil {
					return err
				}
			}
		}
		return w.Flush(state, nil, consumer)
	default:
		return fmt.Errorf("unsupported doc values type %v", fieldInfo.DocValuesType())
	}
}
