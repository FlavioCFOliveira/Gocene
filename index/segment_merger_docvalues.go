// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// mergeDocValues merges the doc-values of every doc-values field across the
// source segments into the new segment, remapping each value's docID through
// the merge DocMaps. Numeric, binary and sorted-numeric fields are merged here;
// the ordinal-mapped SORTED / SORTED_SET types are a separate increment of this
// keystone (rmp #14/#114) and currently surface an explicit error rather than
// silently dropping their values.
func (sm *SegmentMerger) mergeDocValues() error {
	if sm.codec == nil || sm.codec.DocValuesFormat() == nil {
		return nil
	}
	if sm.MergeState.DocMaps == nil {
		sm.buildDocMaps()
	}

	state := &SegmentWriteState{
		Directory:     sm.directory,
		SegmentInfo:   sm.MergeState.SegmentInfo,
		FieldInfos:    sm.MergeState.MergeFieldInfos,
		SegmentSuffix: "",
	}
	consumer, err := sm.codec.DocValuesFormat().FieldsConsumer(state)
	if err != nil {
		return fmt.Errorf("index: merge doc values: open consumer: %w", err)
	}
	defer consumer.Close()

	iter := sm.MergeState.MergeFieldInfos.Iterator()
	for iter.HasNext() {
		info := iter.Next()
		if !info.DocValuesType().HasDocValues() {
			continue
		}
		switch info.DocValuesType() {
		case DocValuesTypeNumeric:
			if err := sm.mergeNumericDV(consumer, info); err != nil {
				return err
			}
		case DocValuesTypeBinary:
			if err := sm.mergeBinaryDV(consumer, info); err != nil {
				return err
			}
		case DocValuesTypeSortedNumeric:
			if err := sm.mergeSortedNumericDV(consumer, info); err != nil {
				return err
			}
		case DocValuesTypeSorted, DocValuesTypeSortedSet:
			return fmt.Errorf("index: merge doc values: field %q has %v which needs ordinal-map merge (not yet implemented, rmp #14/#114)",
				info.Name(), info.DocValuesType())
		default:
			return fmt.Errorf("index: merge doc values: field %q has unsupported type %v", info.Name(), info.DocValuesType())
		}
	}
	return nil
}

// dvProducerOf returns the segment reader's DocValuesProducer, or nil.
func dvProducerOf(reader *CodecReader) spi.DocValuesProducer {
	p := reader.GetDocValuesReader()
	if p == nil {
		return nil
	}
	dp, ok := p.(spi.DocValuesProducer)
	if !ok {
		return nil
	}
	return dp
}

// subFieldInfo returns the source reader's FieldInfo for field, or nil.
func subFieldInfo(reader *CodecReader, field string) *FieldInfo {
	fis := reader.GetFieldInfos()
	if fis == nil {
		return nil
	}
	return fis.GetByName(field)
}

// dvExhaustedDoc reports whether a doc-values iterator docID marks the end. The
// codec producers use DocIdSetIterator.NO_MORE_DOCS (math.MaxInt32) while some
// in-memory iterators use index.NO_MORE_DOCS (-1); treating any out-of-range
// docID as the end is robust against both (matches rmp #6's docValuesNoMoreDocs).
func dvExhaustedDoc(docID, maxDoc int) bool {
	return docID < 0 || docID >= maxDoc
}

// mergeNumericDV materialises the merged numeric values (in new-docID order)
// and feeds them to the consumer.
func (sm *SegmentMerger) mergeNumericDV(consumer DocValuesConsumer, info *FieldInfo) error {
	var docIDs []int
	var values []int64
	for i, reader := range sm.MergeState.Readers {
		if reader == nil {
			continue
		}
		prod := dvProducerOf(reader)
		fi := subFieldInfo(reader, info.Name())
		if prod == nil || fi == nil {
			continue
		}
		ndv, err := prod.GetNumeric(fi)
		if err != nil {
			return fmt.Errorf("index: merge doc values: numeric %q reader %d: %w", info.Name(), i, err)
		}
		if ndv == nil {
			continue
		}
		maxDoc := sm.MergeState.MaxDocs[i]
		docMap := sm.MergeState.DocMaps[i]
		for {
			d, err := ndv.NextDoc()
			if err != nil {
				return err
			}
			if dvExhaustedDoc(d, maxDoc) {
				break
			}
			mapped := docMap.Get(d)
			if mapped < 0 {
				continue
			}
			v, err := ndv.LongValue()
			if err != nil {
				return err
			}
			docIDs = append(docIDs, mapped)
			values = append(values, v)
		}
	}
	return consumer.AddNumericField(info, &sliceNumericDVIter{docIDs: docIDs, values: values, pos: -1})
}

// mergeBinaryDV materialises the merged binary values and feeds the consumer.
func (sm *SegmentMerger) mergeBinaryDV(consumer DocValuesConsumer, info *FieldInfo) error {
	var docIDs []int
	var values [][]byte
	for i, reader := range sm.MergeState.Readers {
		if reader == nil {
			continue
		}
		prod := dvProducerOf(reader)
		fi := subFieldInfo(reader, info.Name())
		if prod == nil || fi == nil {
			continue
		}
		bdv, err := prod.GetBinary(fi)
		if err != nil {
			return fmt.Errorf("index: merge doc values: binary %q reader %d: %w", info.Name(), i, err)
		}
		if bdv == nil {
			continue
		}
		maxDoc := sm.MergeState.MaxDocs[i]
		docMap := sm.MergeState.DocMaps[i]
		for {
			d, err := bdv.NextDoc()
			if err != nil {
				return err
			}
			if dvExhaustedDoc(d, maxDoc) {
				break
			}
			mapped := docMap.Get(d)
			if mapped < 0 {
				continue
			}
			v, err := bdv.BinaryValue()
			if err != nil {
				return err
			}
			dup := make([]byte, len(v))
			copy(dup, v)
			docIDs = append(docIDs, mapped)
			values = append(values, dup)
		}
	}
	return consumer.AddBinaryField(info, &sliceBinaryDVIter{docIDs: docIDs, values: values, pos: -1})
}

// mergeSortedNumericDV materialises the merged sorted-numeric values and feeds
// the consumer.
func (sm *SegmentMerger) mergeSortedNumericDV(consumer DocValuesConsumer, info *FieldInfo) error {
	var docIDs []int
	var values [][]int64
	for i, reader := range sm.MergeState.Readers {
		if reader == nil {
			continue
		}
		prod := dvProducerOf(reader)
		fi := subFieldInfo(reader, info.Name())
		if prod == nil || fi == nil {
			continue
		}
		sdv, err := prod.GetSortedNumeric(fi)
		if err != nil {
			return fmt.Errorf("index: merge doc values: sorted-numeric %q reader %d: %w", info.Name(), i, err)
		}
		if sdv == nil {
			continue
		}
		maxDoc := sm.MergeState.MaxDocs[i]
		docMap := sm.MergeState.DocMaps[i]
		for {
			d, err := sdv.NextDoc()
			if err != nil {
				return err
			}
			if dvExhaustedDoc(d, maxDoc) {
				break
			}
			mapped := docMap.Get(d)
			if mapped < 0 {
				continue
			}
			count, err := sdv.DocValueCount()
			if err != nil {
				return err
			}
			docVals := make([]int64, 0, count)
			for j := 0; j < count; j++ {
				v, err := sdv.NextValue()
				if err != nil {
					return err
				}
				docVals = append(docVals, v)
			}
			docIDs = append(docIDs, mapped)
			values = append(values, docVals)
		}
	}
	return consumer.AddSortedNumericField(info, &sliceSortedNumericDVIter{docIDs: docIDs, values: values, pos: -1})
}

// --- merged in-memory iterators fed to the DocValuesConsumer ---------------

type sliceNumericDVIter struct {
	docIDs []int
	values []int64
	pos    int
}

func (it *sliceNumericDVIter) Next() bool   { it.pos++; return it.pos < len(it.docIDs) }
func (it *sliceNumericDVIter) DocID() int   { return it.docIDs[it.pos] }
func (it *sliceNumericDVIter) Value() int64 { return it.values[it.pos] }

type sliceBinaryDVIter struct {
	docIDs []int
	values [][]byte
	pos    int
}

func (it *sliceBinaryDVIter) Next() bool    { it.pos++; return it.pos < len(it.docIDs) }
func (it *sliceBinaryDVIter) DocID() int    { return it.docIDs[it.pos] }
func (it *sliceBinaryDVIter) Value() []byte { return it.values[it.pos] }

type sliceSortedNumericDVIter struct {
	docIDs []int
	values [][]int64
	pos    int
	vpos   int
}

func (it *sliceSortedNumericDVIter) NextDoc() bool {
	it.pos++
	it.vpos = 0
	return it.pos < len(it.docIDs)
}
func (it *sliceSortedNumericDVIter) DocID() int         { return it.docIDs[it.pos] }
func (it *sliceSortedNumericDVIter) DocValueCount() int { return len(it.values[it.pos]) }
func (it *sliceSortedNumericDVIter) NextValue() int64 {
	v := it.values[it.pos][it.vpos]
	it.vpos++
	return v
}
