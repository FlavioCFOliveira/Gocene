// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// mergeNorms merges the per-document norms of every field with norms across the
// source segments into the new segment, remapping each value's docID through
// the merge DocMaps. Norms are a single numeric (single-byte) value per
// value-bearing document, so the merge is shaped exactly like the NUMERIC
// doc-values merge (mergeNumericDV): collect the live (mapped-docID, value)
// pairs from every reader and feed them to the codec NormsConsumer in ascending
// merged-docID order.
//
// Mirrors org.apache.lucene.index.SegmentMerger.mergeNorms /
// org.apache.lucene.codecs.NormsConsumer.merge, which iterate the merge field
// infos and, for each field with norms, replay the per-segment NumericDocValues
// remapped through the MergeState DocMap.
func (sm *SegmentMerger) mergeNorms() error {
	if sm.codec == nil || sm.codec.NormsFormat() == nil {
		return nil
	}
	if sm.MergeState.DocMaps == nil {
		if err := sm.buildDocMaps(); err != nil {
			return err
		}
	}

	state := &SegmentWriteState{
		Directory:     sm.directory,
		SegmentInfo:   sm.MergeState.SegmentInfo,
		FieldInfos:    sm.MergeState.MergeFieldInfos,
		SegmentSuffix: "",
	}
	consumer, err := sm.codec.NormsFormat().NormsConsumer(state)
	if err != nil {
		return fmt.Errorf("index: merge norms: open consumer: %w", err)
	}
	defer consumer.Close()

	iter := sm.MergeState.MergeFieldInfos.Iterator()
	for iter.HasNext() {
		info := iter.Next()
		if !info.HasNorms() {
			continue
		}
		if err := sm.mergeNormsField(consumer, info); err != nil {
			return err
		}
	}
	return nil
}

// normsProducerOf returns the segment reader's NormsProducer, or nil.
func normsProducerOf(reader *CodecReader) spi.NormsProducer {
	p := reader.GetNormsReader()
	if p == nil {
		return nil
	}
	np, ok := p.(spi.NormsProducer)
	if !ok {
		return nil
	}
	return np
}

// mergeNormsField materialises the merged norm values (in new-docID order)
// for one field and feeds them to the consumer.
func (sm *SegmentMerger) mergeNormsField(consumer NormsConsumer, info *FieldInfo) error {
	var docIDs []int
	var values []int64
	for i, reader := range sm.MergeState.Readers {
		if reader == nil {
			continue
		}
		prod := normsProducerOf(reader)
		fi := subFieldInfo(reader, info.Name())
		if prod == nil || fi == nil {
			continue
		}
		ndv, err := prod.GetNorms(fi)
		if err != nil {
			return fmt.Errorf("index: merge norms: field %q reader %d: %w", info.Name(), i, err)
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
	if sm.MergeState.NeedsIndexSort {
		// An index-sorted merge renumbers documents out of (reader, docID)
		// order, so re-sort into ascending merged-docID order; the
		// NormsConsumer requires strictly increasing docIDs.
		sort.Stable(numericByDoc{docIDs: docIDs, values: values})
	}
	return consumer.AddNormsField(info, &mergedNormsIter{docIDs: docIDs, values: values, pos: -1})
}

// mergedNormsIter replays the merged per-document norm values to the codec
// NormsConsumer. It satisfies the NormsIterator contract: docIDs is strictly
// increasing in the merged doc space.
type mergedNormsIter struct {
	docIDs []int
	values []int64
	pos    int
}

func (it *mergedNormsIter) Next() bool {
	it.pos++
	return it.pos < len(it.docIDs)
}
func (it *mergedNormsIter) DocID() int       { return it.docIDs[it.pos] }
func (it *mergedNormsIter) LongValue() int64 { return it.values[it.pos] }
