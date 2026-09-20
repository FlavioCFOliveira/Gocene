// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
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
		Directory:      sm.directory,
		SegmentInfo:    sm.MergeState.SegmentInfo,
		FieldInfos:     sm.MergeState.MergeFieldInfos,
		SegmentSuffix:  "",
		NeedsIndexSort: sm.MergeState.NeedsIndexSort,
		IsMerge:        true,
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
func normsProducerOf(reader CodecReader) spi.NormsProducer {
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
	// NormsConsumer.addNormsField is a pull API: the consumer may ask the
	// producer for the NumericDocValues more than once, so the producer hands
	// out a fresh cursor on every call. Mirrors the anonymous NormsProducer
	// NormsConsumer.mergeNormsField builds (NormsConsumer.java:104-176).
	return consumer.AddNormsField(info, &mergedNormsProducer{
		fieldInfo: info,
		docIDs:    docIDs,
		values:    values,
	})
}

// errMergedNormsAdvance renders the UnsupportedOperationException that
// advance / advanceExact throw on the anonymous NumericDocValues built by
// NormsConsumer.mergeNormsField (NormsConsumer.java:152-160).
var errMergedNormsAdvance = errors.New("index: merged norms: Advance/AdvanceExact is not supported")

// mergedNormsProducer is the NormsProducer the merge hands to the codec
// NormsConsumer. Mirrors the anonymous NormsProducer of
// NormsConsumer.mergeNormsField: getNorms rejects a FieldInfo other than the
// one being merged, checkIntegrity and close are no-ops.
type mergedNormsProducer struct {
	fieldInfo *FieldInfo
	docIDs    []int
	values    []int64
}

// GetNorms returns a fresh cursor over the merged values. Mirrors
// NormsConsumer.mergeNormsField's getNorms(FieldInfo), which raises
// IllegalArgumentException("wrong fieldInfo") for any other field.
func (p *mergedNormsProducer) GetNorms(field *FieldInfo) (NumericDocValues, error) {
	if field != p.fieldInfo {
		return nil, errors.New("wrong fieldInfo")
	}
	return &mergedNormsValues{docIDs: p.docIDs, values: p.values, pos: -1, doc: -1}, nil
}

func (p *mergedNormsProducer) CheckIntegrity() error               { return nil }
func (p *mergedNormsProducer) GetMergeInstance() spi.NormsProducer { return p }
func (p *mergedNormsProducer) Close() error                        { return nil }

// mergedNormsValues replays the merged per-document norm values. docIDs is
// strictly increasing in the merged doc space.
type mergedNormsValues struct {
	docIDs []int
	values []int64
	pos    int
	doc    int
}

func (it *mergedNormsValues) DocID() int { return it.doc }

func (it *mergedNormsValues) NextDoc() (int, error) {
	it.pos++
	if it.pos >= len(it.docIDs) {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	it.doc = it.docIDs[it.pos]
	return it.doc, nil
}

func (it *mergedNormsValues) Advance(int) (int, error) {
	return 0, errMergedNormsAdvance
}

func (it *mergedNormsValues) AdvanceExact(int) (bool, error) {
	return false, errMergedNormsAdvance
}

func (it *mergedNormsValues) LongValue() (int64, error) {
	return it.values[it.pos], nil
}

// Cost mirrors the anonymous NumericDocValues of
// NormsConsumer.mergeNormsField, whose cost() returns 0.
func (it *mergedNormsValues) Cost() int64 { return 0 }

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which the Java
// counterpart of this type does not override.
func (it *mergedNormsValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() —
// docID() + 1 — which the Java counterpart of this type does not override.
func (it *mergedNormsValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(it)
}
