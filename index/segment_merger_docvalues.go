// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"

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
		if err := sm.buildDocMaps(); err != nil {
			return err
		}
	}

	state := &SegmentWriteState{
		Directory:     sm.directory,
		SegmentInfo:   sm.MergeState.SegmentInfo,
		FieldInfos:    sm.MergeState.MergeFieldInfos,
		SegmentSuffix:  "",
			NeedsIndexSort: sm.MergeState.NeedsIndexSort,
			IsMerge:        true,
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
		case DocValuesTypeSorted:
			if err := sm.mergeSortedDV(consumer, info); err != nil {
				return err
			}
		case DocValuesTypeSortedSet:
			if err := sm.mergeSortedSetDV(consumer, info); err != nil {
				return err
			}
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
	if sm.MergeState.NeedsIndexSort {
		sort.Stable(numericByDoc{docIDs: docIDs, values: values})
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
	if sm.MergeState.NeedsIndexSort {
		sort.Stable(binaryByDoc{docIDs: docIDs, values: values})
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
	if sm.MergeState.NeedsIndexSort {
		sort.Stable(sortedNumericByDoc{docIDs: docIDs, values: values})
	}
	return consumer.AddSortedNumericField(info, &sliceSortedNumericDVIter{docIDs: docIDs, values: values, pos: -1})
}

// sortedFromReaderConsumer is the codec consumer's read-side entry point for
// SORTED / SORTED_SET fields (satisfied by *codecs.Lucene90DocValuesConsumer).
type sortedFromReaderConsumer interface {
	AddSortedFieldFromReader(field *FieldInfo, reset func() (SortedDocValues, error)) error
	AddSortedSetFieldFromReader(field *FieldInfo, reset func() (SortedSetDocValues, error)) error
}

// mergeSortedDV merges a SORTED doc-values field. A global OrdinalMap is built
// across the per-segment value tables (reusing the rmp #6 machinery), and a
// merged SortedDocValues presents each live document's value as a global
// ordinal in the merged doc space, with LookupOrd served from the merged table.
func (sm *SegmentMerger) mergeSortedDV(consumer DocValuesConsumer, info *FieldInfo) error {
	delegate, ok := consumer.(sortedFromReaderConsumer)
	if !ok {
		return fmt.Errorf("index: merge doc values: consumer %T does not support SORTED from reader", consumer)
	}

	openSubs := func() ([]SortedDocValues, []int, error) {
		var subs []SortedDocValues
		var orig []int
		for i, reader := range sm.MergeState.Readers {
			if reader == nil {
				continue
			}
			prod := dvProducerOf(reader)
			fi := subFieldInfo(reader, info.Name())
			if prod == nil || fi == nil {
				continue
			}
			sdv, err := prod.GetSorted(fi)
			if err != nil {
				return nil, nil, err
			}
			if sdv == nil {
				continue
			}
			subs = append(subs, sdv)
			orig = append(orig, i)
		}
		return subs, orig, nil
	}

	omSubs, _, err := openSubs()
	if err != nil {
		return fmt.Errorf("index: merge doc values: sorted %q: %w", info.Name(), err)
	}
	if len(omSubs) == 0 {
		return nil
	}
	om, err := BuildOrdinalMapFromSortedValues(NewCacheKey(), omSubs, 0)
	if err != nil {
		return fmt.Errorf("index: merge doc values: sorted %q ordinal map: %w", info.Name(), err)
	}

	reset := func() (SortedDocValues, error) {
		subs, orig, err := openSubs()
		if err != nil {
			return nil, err
		}
		docMaps := make([]DocMap, len(orig))
		maxDocs := make([]int, len(orig))
		for k, oi := range orig {
			docMaps[k] = sm.MergeState.DocMaps[oi]
			maxDocs[k] = sm.MergeState.MaxDocs[oi]
		}
		if sm.MergeState.NeedsIndexSort {
			// The merged docIDs are not produced in (reader, docID) order, so
			// materialise every live doc's global ordinal and present them in
			// ascending merged-docID order (the DocValuesConsumer requires it).
			return newMaterializedSortedDocValues(subs, docMaps, maxDocs, om)
		}
		return &mergedSortedDocValues{subs: subs, docMaps: docMaps, maxDocs: maxDocs, om: om, doc: -1, ord: -1}, nil
	}
	return delegate.AddSortedFieldFromReader(info, reset)
}

// mergeSortedSetDV merges a SORTED_SET doc-values field, analogous to
// mergeSortedDV but emitting each document's (ascending) set of global ordinals.
func (sm *SegmentMerger) mergeSortedSetDV(consumer DocValuesConsumer, info *FieldInfo) error {
	delegate, ok := consumer.(sortedFromReaderConsumer)
	if !ok {
		return fmt.Errorf("index: merge doc values: consumer %T does not support SORTED_SET from reader", consumer)
	}

	openSubs := func() ([]SortedSetDocValues, []int, error) {
		var subs []SortedSetDocValues
		var orig []int
		for i, reader := range sm.MergeState.Readers {
			if reader == nil {
				continue
			}
			prod := dvProducerOf(reader)
			fi := subFieldInfo(reader, info.Name())
			if prod == nil || fi == nil {
				continue
			}
			ssdv, err := prod.GetSortedSet(fi)
			if err != nil {
				return nil, nil, err
			}
			if ssdv == nil {
				continue
			}
			subs = append(subs, ssdv)
			orig = append(orig, i)
		}
		return subs, orig, nil
	}

	omSubs, _, err := openSubs()
	if err != nil {
		return fmt.Errorf("index: merge doc values: sorted-set %q: %w", info.Name(), err)
	}
	if len(omSubs) == 0 {
		return nil
	}
	om, err := BuildOrdinalMapFromSortedSetValues(NewCacheKey(), omSubs, 0)
	if err != nil {
		return fmt.Errorf("index: merge doc values: sorted-set %q ordinal map: %w", info.Name(), err)
	}

	reset := func() (SortedSetDocValues, error) {
		subs, orig, err := openSubs()
		if err != nil {
			return nil, err
		}
		docMaps := make([]DocMap, len(orig))
		maxDocs := make([]int, len(orig))
		for k, oi := range orig {
			docMaps[k] = sm.MergeState.DocMaps[oi]
			maxDocs[k] = sm.MergeState.MaxDocs[oi]
		}
		if sm.MergeState.NeedsIndexSort {
			return newMaterializedSortedSetDocValues(subs, docMaps, maxDocs, om)
		}
		return &mergedSortedSetDocValues{subs: subs, docMaps: docMaps, maxDocs: maxDocs, om: om, doc: -1}, nil
	}
	return delegate.AddSortedSetFieldFromReader(info, reset)
}

// mergedSortedDocValues presents the per-segment SORTED values as one merged
// stream: live docs in merged-docID order, each carrying a global ordinal.
type mergedSortedDocValues struct {
	subs    []SortedDocValues
	docMaps []DocMap
	maxDocs []int
	om      *OrdinalMap
	si      int
	doc     int
	ord     int
}

func (m *mergedSortedDocValues) NextDoc() (int, error) {
	for m.si < len(m.subs) {
		sub := m.subs[m.si]
		for {
			d, err := sub.NextDoc()
			if err != nil {
				return 0, err
			}
			if dvExhaustedDoc(d, m.maxDocs[m.si]) {
				break
			}
			mapped := m.docMaps[m.si].Get(d)
			if mapped < 0 {
				continue
			}
			so, err := sub.OrdValue()
			if err != nil {
				return 0, err
			}
			globals := m.om.GetGlobalOrds(m.si)
			m.ord = int(globals[so])
			m.doc = mapped
			return mapped, nil
		}
		m.si++
	}
	m.doc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

func (m *mergedSortedDocValues) OrdValue() (int, error) { return m.ord, nil }
func (m *mergedSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	segNum := m.om.GetFirstSegmentNumber(int64(ord))
	segOrd := m.om.GetFirstSegmentOrd(int64(ord))
	if segNum < 0 || segNum >= len(m.subs) {
		return nil, fmt.Errorf("index: merge sorted DV: global ord %d out of range", ord)
	}
	return m.subs[segNum].LookupOrd(int(segOrd))
}
func (m *mergedSortedDocValues) GetValueCount() int { return int(m.om.GetValueCount()) }
func (m *mergedSortedDocValues) DocID() int         { return m.doc }
func (m *mergedSortedDocValues) Advance(target int) (int, error) {
	for {
		d, err := m.NextDoc()
		if err != nil || d == NO_MORE_DOCS || d >= target {
			return d, err
		}
	}
}
func (m *mergedSortedDocValues) AdvanceExact(target int) (bool, error) {
	d, err := m.Advance(target)
	return d == target, err
}
func (m *mergedSortedDocValues) LongValue() (int64, error) {
	o, err := m.OrdValue()
	return int64(o), err
}
func (m *mergedSortedDocValues) Cost() int64 { return 0 }

// mergedSortedSetDocValues presents the per-segment SORTED_SET values as one
// merged stream: live docs in merged-docID order, each emitting its global
// ordinals in ascending order via NextOrd.
type mergedSortedSetDocValues struct {
	subs    []SortedSetDocValues
	docMaps []DocMap
	maxDocs []int
	om      *OrdinalMap
	si      int
	doc     int
}

func (m *mergedSortedSetDocValues) NextDoc() (int, error) {
	for m.si < len(m.subs) {
		sub := m.subs[m.si]
		for {
			d, err := sub.NextDoc()
			if err != nil {
				return 0, err
			}
			if dvExhaustedDoc(d, m.maxDocs[m.si]) {
				break
			}
			mapped := m.docMaps[m.si].Get(d)
			if mapped < 0 {
				continue
			}
			m.doc = mapped
			return mapped, nil
		}
		m.si++
	}
	m.doc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

func (m *mergedSortedSetDocValues) NextOrd() (int, error) {
	if m.si >= len(m.subs) {
		return -1, nil
	}
	so, err := m.subs[m.si].NextOrd()
	if err != nil {
		return 0, err
	}
	if so < 0 {
		return -1, nil
	}
	globals := m.om.GetGlobalOrds(m.si)
	return int(globals[so]), nil
}
func (m *mergedSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	segNum := m.om.GetFirstSegmentNumber(int64(ord))
	segOrd := m.om.GetFirstSegmentOrd(int64(ord))
	if segNum < 0 || segNum >= len(m.subs) {
		return nil, fmt.Errorf("index: merge sorted-set DV: global ord %d out of range", ord)
	}
	return m.subs[segNum].LookupOrd(int(segOrd))
}
func (m *mergedSortedSetDocValues) GetValueCount() int { return int(m.om.GetValueCount()) }
func (m *mergedSortedSetDocValues) DocID() int         { return m.doc }
func (m *mergedSortedSetDocValues) Advance(target int) (int, error) {
	for {
		d, err := m.NextDoc()
		if err != nil || d == NO_MORE_DOCS || d >= target {
			return d, err
		}
	}
}
func (m *mergedSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	d, err := m.Advance(target)
	return d == target, err
}
func (m *mergedSortedSetDocValues) Cost() int64 { return 0 }

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

// --- doc-ID sort helpers (index-sorted merge, rmp #115) --------------------
//
// When the merge honours an index sort the per-reader DocMaps renumber
// documents out of (reader, docID) order, so the collected (docID, value)
// pairs must be re-sorted into ascending merged-docID order before being fed
// to the DocValuesConsumer (which requires monotonic docIDs).

type numericByDoc struct {
	docIDs []int
	values []int64
}

func (s numericByDoc) Len() int           { return len(s.docIDs) }
func (s numericByDoc) Less(i, j int) bool { return s.docIDs[i] < s.docIDs[j] }
func (s numericByDoc) Swap(i, j int) {
	s.docIDs[i], s.docIDs[j] = s.docIDs[j], s.docIDs[i]
	s.values[i], s.values[j] = s.values[j], s.values[i]
}

type binaryByDoc struct {
	docIDs []int
	values [][]byte
}

func (s binaryByDoc) Len() int           { return len(s.docIDs) }
func (s binaryByDoc) Less(i, j int) bool { return s.docIDs[i] < s.docIDs[j] }
func (s binaryByDoc) Swap(i, j int) {
	s.docIDs[i], s.docIDs[j] = s.docIDs[j], s.docIDs[i]
	s.values[i], s.values[j] = s.values[j], s.values[i]
}

type sortedNumericByDoc struct {
	docIDs []int
	values [][]int64
}

func (s sortedNumericByDoc) Len() int           { return len(s.docIDs) }
func (s sortedNumericByDoc) Less(i, j int) bool { return s.docIDs[i] < s.docIDs[j] }
func (s sortedNumericByDoc) Swap(i, j int) {
	s.docIDs[i], s.docIDs[j] = s.docIDs[j], s.docIDs[i]
	s.values[i], s.values[j] = s.values[j], s.values[i]
}

// --- materialized SORTED / SORTED_SET views (index-sorted merge) -----------

// newMaterializedSortedDocValues collects every live document's global ordinal
// across the sub-readers and presents them in ascending merged-docID order.
// LookupOrd / GetValueCount stay served from the shared OrdinalMap.
func newMaterializedSortedDocValues(subs []SortedDocValues, docMaps []DocMap, maxDocs []int, om *OrdinalMap) (SortedDocValues, error) {
	var docIDs []int
	var ords []int
	for si, sub := range subs {
		globals := om.GetGlobalOrds(si)
		for {
			d, err := sub.NextDoc()
			if err != nil {
				return nil, err
			}
			if dvExhaustedDoc(d, maxDocs[si]) {
				break
			}
			mapped := docMaps[si].Get(d)
			if mapped < 0 {
				continue
			}
			so, err := sub.OrdValue()
			if err != nil {
				return nil, err
			}
			docIDs = append(docIDs, mapped)
			ords = append(ords, int(globals[so]))
		}
	}
	sort.Stable(sortedOrdByDoc{docIDs: docIDs, ords: ords})
	return &materializedSortedDocValues{subs: subs, om: om, docIDs: docIDs, ords: ords, pos: -1, doc: -1}, nil
}

type sortedOrdByDoc struct {
	docIDs []int
	ords   []int
}

func (s sortedOrdByDoc) Len() int           { return len(s.docIDs) }
func (s sortedOrdByDoc) Less(i, j int) bool { return s.docIDs[i] < s.docIDs[j] }
func (s sortedOrdByDoc) Swap(i, j int) {
	s.docIDs[i], s.docIDs[j] = s.docIDs[j], s.docIDs[i]
	s.ords[i], s.ords[j] = s.ords[j], s.ords[i]
}

type materializedSortedDocValues struct {
	subs   []SortedDocValues
	om     *OrdinalMap
	docIDs []int
	ords   []int
	pos    int
	doc    int
}

func (m *materializedSortedDocValues) NextDoc() (int, error) {
	m.pos++
	if m.pos >= len(m.docIDs) {
		m.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	m.doc = m.docIDs[m.pos]
	return m.doc, nil
}
func (m *materializedSortedDocValues) OrdValue() (int, error) { return m.ords[m.pos], nil }
func (m *materializedSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	segNum := m.om.GetFirstSegmentNumber(int64(ord))
	segOrd := m.om.GetFirstSegmentOrd(int64(ord))
	if segNum < 0 || segNum >= len(m.subs) {
		return nil, fmt.Errorf("index: merge sorted DV: global ord %d out of range", ord)
	}
	return m.subs[segNum].LookupOrd(int(segOrd))
}
func (m *materializedSortedDocValues) GetValueCount() int { return int(m.om.GetValueCount()) }
func (m *materializedSortedDocValues) DocID() int         { return m.doc }
func (m *materializedSortedDocValues) Advance(target int) (int, error) {
	for {
		d, err := m.NextDoc()
		if err != nil || d == NO_MORE_DOCS || d >= target {
			return d, err
		}
	}
}
func (m *materializedSortedDocValues) AdvanceExact(target int) (bool, error) {
	d, err := m.Advance(target)
	return d == target, err
}
func (m *materializedSortedDocValues) LongValue() (int64, error) {
	return int64(m.ords[m.pos]), nil
}
func (m *materializedSortedDocValues) Cost() int64 { return int64(len(m.docIDs)) }

// newMaterializedSortedSetDocValues collects every live document's ascending
// set of global ordinals and presents them in ascending merged-docID order.
func newMaterializedSortedSetDocValues(subs []SortedSetDocValues, docMaps []DocMap, maxDocs []int, om *OrdinalMap) (SortedSetDocValues, error) {
	var docIDs []int
	var ordSets [][]int
	for si, sub := range subs {
		globals := om.GetGlobalOrds(si)
		for {
			d, err := sub.NextDoc()
			if err != nil {
				return nil, err
			}
			if dvExhaustedDoc(d, maxDocs[si]) {
				break
			}
			mapped := docMaps[si].Get(d)
			if mapped < 0 {
				continue
			}
			var set []int
			for {
				so, err := sub.NextOrd()
				if err != nil {
					return nil, err
				}
				if so < 0 {
					break
				}
				set = append(set, int(globals[so]))
			}
			docIDs = append(docIDs, mapped)
			ordSets = append(ordSets, set)
		}
	}
	sort.Stable(sortedSetByDoc{docIDs: docIDs, ordSets: ordSets})
	return &materializedSortedSetDocValues{subs: subs, om: om, docIDs: docIDs, ordSets: ordSets, pos: -1, doc: -1, ordPos: 0}, nil
}

type sortedSetByDoc struct {
	docIDs  []int
	ordSets [][]int
}

func (s sortedSetByDoc) Len() int           { return len(s.docIDs) }
func (s sortedSetByDoc) Less(i, j int) bool { return s.docIDs[i] < s.docIDs[j] }
func (s sortedSetByDoc) Swap(i, j int) {
	s.docIDs[i], s.docIDs[j] = s.docIDs[j], s.docIDs[i]
	s.ordSets[i], s.ordSets[j] = s.ordSets[j], s.ordSets[i]
}

type materializedSortedSetDocValues struct {
	subs    []SortedSetDocValues
	om      *OrdinalMap
	docIDs  []int
	ordSets [][]int
	pos     int
	doc     int
	ordPos  int
}

func (m *materializedSortedSetDocValues) NextDoc() (int, error) {
	m.pos++
	m.ordPos = 0
	if m.pos >= len(m.docIDs) {
		m.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	m.doc = m.docIDs[m.pos]
	return m.doc, nil
}
func (m *materializedSortedSetDocValues) NextOrd() (int, error) {
	if m.pos < 0 || m.pos >= len(m.ordSets) {
		return -1, nil
	}
	set := m.ordSets[m.pos]
	if m.ordPos >= len(set) {
		return -1, nil
	}
	o := set[m.ordPos]
	m.ordPos++
	return o, nil
}
func (m *materializedSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	segNum := m.om.GetFirstSegmentNumber(int64(ord))
	segOrd := m.om.GetFirstSegmentOrd(int64(ord))
	if segNum < 0 || segNum >= len(m.subs) {
		return nil, fmt.Errorf("index: merge sorted-set DV: global ord %d out of range", ord)
	}
	return m.subs[segNum].LookupOrd(int(segOrd))
}
func (m *materializedSortedSetDocValues) GetValueCount() int { return int(m.om.GetValueCount()) }
func (m *materializedSortedSetDocValues) DocID() int         { return m.doc }
func (m *materializedSortedSetDocValues) Advance(target int) (int, error) {
	for {
		d, err := m.NextDoc()
		if err != nil || d == NO_MORE_DOCS || d >= target {
			return d, err
		}
	}
}
func (m *materializedSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	d, err := m.Advance(target)
	return d == target, err
}
func (m *materializedSortedSetDocValues) Cost() int64 { return int64(len(m.docIDs)) }
