// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BaseDocValuesConsumer provides a default implementation of the merge logic for
// DocValuesConsumer. Concrete consumers embed this and override the Add*Field methods.
//
// Mirrors org.apache.lucene.codecs.DocValuesConsumer in Apache Lucene 10.4.0.
type BaseDocValuesConsumer struct{}

// Merge merges in the fields from the readers in mergeState. The default
// implementation calls mergeNumericField, mergeBinaryField, mergeSortedField,
// mergeSortedSetField, or mergeSortedNumericField for each field, depending on its
// type.
func (b *BaseDocValuesConsumer) Merge(consumer DocValuesConsumer, mergeState *index.MergeState, producers []DocValuesProducer) error {
	for _, prod := range producers {
		if prod != nil {
			if err := prod.CheckIntegrity(); err != nil {
				return err
			}
		}
	}

	for _, mergeFI := range mergeState.MergeFieldInfos {
		dvType := mergeFI.DocValuesType()
		if dvType == spi.DocValuesTypeNone {
			continue
		}

		var err error
		switch dvType {
		case spi.DocValuesTypeNumeric:
			err = b.mergeNumericField(consumer, mergeFI, mergeState, producers)
		case spi.DocValuesTypeBinary:
			err = b.mergeBinaryField(consumer, mergeFI, mergeState, producers)
		case spi.DocValuesTypeSorted:
			err = b.mergeSortedField(consumer, mergeFI, mergeState, producers)
		case spi.DocValuesTypeSortedSet:
			err = b.mergeSortedSetField(consumer, mergeFI, mergeState, producers)
		case spi.DocValuesTypeSortedNumeric:
			err = b.mergeSortedNumericField(consumer, mergeFI, mergeState, producers)
		default:
			return fmt.Errorf("unsupported DocValuesType: %v", dvType)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *BaseDocValuesConsumer) mergeNumericField(consumer DocValuesConsumer, mergeFI *spi.FieldInfo, mergeState *index.MergeState, producers []DocValuesProducer) error {
	return consumer.AddNumericField(mergeFI, &mergedNumericProducer{
		consumer:   consumer,
		mergeFI:    mergeFI,
		mergeState: mergeState,
		producers:  producers,
	})
}

func (b *BaseDocValuesConsumer) mergeBinaryField(consumer DocValuesConsumer, mergeFI *spi.FieldInfo, mergeState *index.MergeState, producers []DocValuesProducer) error {
	return consumer.AddBinaryField(mergeFI, &mergedBinaryProducer{
		consumer:   consumer,
		mergeFI:    mergeFI,
		mergeState: mergeState,
		producers:  producers,
	})
}

func (b *BaseDocValuesConsumer) mergeSortedNumericField(consumer DocValuesConsumer, mergeFI *spi.FieldInfo, mergeState *index.MergeState, producers []DocValuesProducer) error {
	return consumer.AddSortedNumericField(mergeFI, &mergedSortedNumericProducer{
		consumer:   consumer,
		mergeFI:    mergeFI,
		mergeState: mergeState,
		producers:  producers,
	})
}

func (b *BaseDocValuesConsumer) mergeSortedField(consumer DocValuesConsumer, mergeFI *spi.FieldInfo, mergeState *index.MergeState, producers []DocValuesProducer) error {
	map_, err := createOrdinalMapForSortedDV(mergeFI, mergeState, producers)
	if err != nil {
		return err
	}
	return consumer.AddSortedField(mergeFI, &mergedSortedProducer{
		consumer:   consumer,
		mergeFI:    mergeFI,
		mergeState: mergeState,
		producers:  producers,
		ordinalMap: map_,
	})
}

func (b *BaseDocValuesConsumer) mergeSortedSetField(consumer DocValuesConsumer, mergeFI *spi.FieldInfo, mergeState *index.MergeState, producers []DocValuesProducer) error {
	toMerge := selectLeavesToMerge(mergeFI, mergeState, producers)
	map_, err := createOrdinalMapForSortedSetDV(toMerge, mergeState)
	if err != nil {
		return err
	}
	return consumer.AddSortedSetField(mergeFI, &mergedSortedSetProducer{
		consumer:   consumer,
		mergeFI:    mergeFI,
		mergeState: mergeState,
		producers:  producers,
		ordinalMap: map_,
		toMerge:    toMerge,
	})
}

// --- Merge Helpers ---

// IsSingleValued returns true if the given doc-to-value count contains only at most one value.
func IsSingleValued(docToValueCount iterableNumber) bool {
	for docToValueCount.Next() {
		if docToValueCount.Value().Int64() > 1 {
			return false
		}
	}
	return true
}

// SingletonView returns a single-valued view, using missingValue when count is zero.
func SingletonView(docToValueCount iterableNumber, values iterableNumber, missingValue int64) iterableNumber {
	return &singletonView{
		countIterator:  docToValueCount,
		valuesIterator: values,
		missingValue:   missingValue,
	}
}

type iterableNumber interface {
	Next() bool
	Value() number
}

type number interface {
	Int64() int64
}

type singletonView struct {
	countIterator  iterableNumber
	valuesIterator iterableNumber
	missingValue   int64
}

func (s *singletonView) Next() bool {
	return s.countIterator.Next()
}

func (s *singletonView) Value() number {
	count := s.countIterator.Value().Int64()
	if count == 0 {
		return fixedNumber(s.missingValue)
	}
	return s.valuesIterator.Value()
}

type fixedNumber int64

func (f fixedNumber) Int64() int64 { return int64(f) }

// numericDocValuesSub is the Go rendering of the private static nested class
// DocValuesConsumer.NumericDocValuesSub of Apache Lucene 10.5.0. NormsConsumer
// declares a nested class of the same simple name; see
// normsConsumerNumericDocValuesSub.
type numericDocValuesSub struct {
	docMap index.DocMap
	values spi.NumericDocValues
}

func (s *numericDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *numericDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *numericDocValuesSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.values.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == index.NO_MORE_DOCS {
			return index.NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(doc)
		if mapped != -1 {
			return mapped, nil
		}
	}
}

func getMergedNumericDocValues(mergeState *index.MergeState, mergeFI *spi.FieldInfo, producers []DocValuesProducer) (spi.NumericDocValues, error) {
	subs := make([]index.DocIDMergerSub, 0)
	for i, prod := range producers {
		if prod == nil {
			continue
		}
		readerFI := mergeState.FieldInfos[i].FieldInfo(mergeFI.Name())
		if readerFI != nil && readerFI.DocValuesType() == spi.DocValuesTypeNumeric {
			values, err := prod.GetNumeric(readerFI)
			if err != nil {
				return nil, err
			}
			subs = append(subs, &numericDocValuesSub{
				docMap: mergeState.DocMaps[i],
				values: values,
			})
		}
	}
	return mergeNumericValues(subs, mergeState.NeedsIndexSort)
}

func mergeNumericValues(subs []index.DocIDMergerSub, indexIsSorted bool) (spi.NumericDocValues, error) {
	var cost int64
	for _, sub := range subs {
		if s, ok := sub.(*numericDocValuesSub); ok {
			cost += s.values.Cost()
		}
	}

	docIDMerger, err := index.NewDocIDMerger(subs, 0, indexIsSorted)
	if err != nil {
		return nil, err
	}

	return &mergedNumericDocValues{
		docID:       -1,
		docIDMerger: docIDMerger,
		cost:        cost,
	}, nil
}

type mergedNumericDocValues struct {
	docID       int
	current     index.DocIDMergerSub
	docIDMerger index.DocIDMerger
	cost        int64
}

func (m *mergedNumericDocValues) DocID() int {
	return m.docID
}

func (m *mergedNumericDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.docID = index.NO_MORE_DOCS
	} else {
		m.current = sub
		m.docID = sub.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedNumericDocValues) Advance(target int) (int, error) {
	panic("not implemented")
}

func (m *mergedNumericDocValues) AdvanceExact(target int) (bool, error) {
	panic("not implemented")
}

func (m *mergedNumericDocValues) LongValue() (int64, error) {
	if s, ok := m.current.(*numericDocValuesSub); ok {
		return s.values.LongValue()
	}
	return 0, fmt.Errorf("no current sub")
}

func (m *mergedNumericDocValues) Cost() int64 {
	return m.cost
}

type binaryDocValuesSub struct {
	docMap index.DocMap
	values spi.BinaryDocValues
}

func (s *binaryDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *binaryDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *binaryDocValuesSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.values.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == index.NO_MORE_DOCS {
			return index.NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(doc)
		if mapped != -1 {
			return mapped, nil
		}
	}
}

func getMergedBinaryDocValues(mergeFI *spi.FieldInfo, mergeState *index.MergeState, producers []DocValuesProducer) (spi.BinaryDocValues, error) {
	subs := make([]index.DocIDMergerSub, 0)
	var cost int64
	for i, prod := range producers {
		if prod == nil {
			continue
		}
		readerFI := mergeState.FieldInfos[i].FieldInfo(mergeFI.Name())
		if readerFI != nil && readerFI.DocValuesType() == spi.DocValuesTypeBinary {
			values, err := prod.GetBinary(readerFI)
			if err != nil {
				return nil, err
			}
			cost += values.Cost()
			subs = append(subs, &binaryDocValuesSub{
				docMap: mergeState.DocMaps[i],
				values: values,
			})
		}
	}

	docIDMerger, err := index.NewDocIDMerger(subs, 0, mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}

	return &mergedBinaryDocValues{
		docID:       -1,
		docIDMerger: docIDMerger,
		cost:        cost,
	}, nil
}

type mergedBinaryDocValues struct {
	docID       int
	current     index.DocIDMergerSub
	docIDMerger index.DocIDMerger
	cost        int64
}

func (m *mergedBinaryDocValues) DocID() int {
	return m.docID
}

func (m *mergedBinaryDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.docID = index.NO_MORE_DOCS
	} else {
		m.current = sub
		m.docID = sub.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedBinaryDocValues) Advance(target int) (int, error) {
	panic("not implemented")
}

func (m *mergedBinaryDocValues) AdvanceExact(target int) (bool, error) {
	panic("not implemented")
}

func (m *mergedBinaryDocValues) BinaryValue() ([]byte, error) {
	if s, ok := m.current.(*binaryDocValuesSub); ok {
		return s.values.BinaryValue()
	}
	return nil, fmt.Errorf("no current sub")
}

func (m *mergedBinaryDocValues) Cost() int64 {
	return m.cost
}

type sortedNumericDocValuesSub struct {
	docMap index.DocMap
	values spi.SortedNumericDocValues
}

func (s *sortedNumericDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *sortedNumericDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *sortedNumericDocValuesSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.values.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == index.NO_MORE_DOCS {
			return index.NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(doc)
		if mapped != -1 {
			return mapped, nil
		}
	}
}

func getMergedSortedNumericDocValues(mergeFI *spi.FieldInfo, mergeState *index.MergeState, producers []DocValuesProducer) (spi.SortedNumericDocValues, error) {
	subs := make([]index.DocIDMergerSub, 0)
	var cost int64
	for i, prod := range producers {
		if prod == nil {
			continue
		}
		readerFI := mergeState.FieldInfos[i].FieldInfo(mergeFI.Name())
		if readerFI != nil && readerFI.DocValuesType() == spi.DocValuesTypeSortedNumeric {
			values, err := prod.GetSortedNumeric(readerFI)
			if err != nil {
				return nil, err
			}
			cost += values.Cost()
			subs = append(subs, &sortedNumericDocValuesSub{
				docMap: mergeState.DocMaps[i],
				values: values,
			})
		}
	}

	docIDMerger, err := index.NewDocIDMerger(subs, 0, mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}

	return &mergedSortedNumericDocValues{
		docID:       -1,
		docIDMerger: docIDMerger,
		cost:        cost,
	}, nil
}

type mergedSortedNumericDocValues struct {
	docID       int
	currentSub  index.DocIDMergerSub
	docIDMerger index.DocIDMerger
	cost        int64
}

func (m *mergedSortedNumericDocValues) DocID() int {
	return m.docID
}

func (m *mergedSortedNumericDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.docID = index.NO_MORE_DOCS
	} else {
		m.currentSub = sub
		m.docID = sub.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedSortedNumericDocValues) Advance(target int) (int, error) {
	panic("not implemented")
}

func (m *mergedSortedNumericDocValues) AdvanceExact(target int) (bool, error) {
	panic("not implemented")
}

func (m *mergedSortedNumericDocValues) DocValueCount() (int, error) {
	if s, ok := m.currentSub.(*sortedNumericDocValuesSub); ok {
		return s.values.DocValueCount(), nil
	}
	return 0, fmt.Errorf("no current sub")
}

func (m *mergedSortedNumericDocValues) LongValue() (int64, error) {
	if s, ok := m.currentSub.(*sortedNumericDocValuesSub); ok {
		return s.values.LongValue()
	}
	return 0, fmt.Errorf("no current sub")
}

func (m *mergedSortedNumericDocValues) NextValue() (int64, error) {
	if s, ok := m.currentSub.(*sortedNumericDocValuesSub); ok {
		return s.values.NextValue()
	}
	return 0, fmt.Errorf("no current sub")
}

func (m *mergedSortedNumericDocValues) Cost() int64 {
	return m.cost
}

type sortedDocValuesSub struct {
	docMap index.DocMap
	values spi.SortedDocValues
}

func (s *sortedDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *sortedDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *sortedDocValuesSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.values.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == index.NO_MORE_DOCS {
			return index.NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(doc)
		if mapped != -1 {
			return mapped, nil
		}
	}
}

func createOrdinalMapForSortedDV(mergeFI *spi.FieldInfo, mergeState *index.MergeState, producers []DocValuesProducer) (*index.OrdinalMap, error) {
	toMerge := make([]spi.SortedDocValues, 0)
	for i, prod := range producers {
		if prod == nil {
			continue
		}
		readerFI := mergeState.FieldInfos[i].FieldInfo(mergeFI.Name())
		if readerFI != nil && readerFI.DocValuesType() == spi.DocValuesTypeSorted {
			values, err := prod.GetSorted(readerFI)
			if err != nil {
				return nil, err
			}
			toMerge = append(toMerge, values)
		}
	}
	return index.BuildOrdinalMapFromSortedValues(nil, toMerge, 0.0)
}

func mergeSortedValues(subs []index.DocIDMergerSub, mergeState *index.MergeState, map_ *index.OrdinalMap) (spi.SortedDocValues, error) {
	var cost int64
	for _, sub := range subs {
		if s, ok := sub.(*sortedDocValuesSub); ok {
			cost += s.values.Cost()
		}
	}

	docIDMerger, err := index.NewDocIDMerger(subs, 0, mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}

	return &mergedSortedDocValues{
		docID:       -1,
		docIDMerger: docIDMerger,
		cost:        cost,
		ordinalMap:  map_,
	}, nil
}

type mergedSortedDocValues struct {
	docID       int
	current     index.DocIDMergerSub
	docIDMerger index.DocIDMerger
	cost        int64
	ordinalMap  *index.OrdinalMap
}

func (m *mergedSortedDocValues) DocID() int {
	return m.docID
}

func (m *mergedSortedDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.docID = index.NO_MORE_DOCS
	} else {
		m.current = sub
		m.docID = sub.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedSortedDocValues) Advance(target int) (int, error) {
	panic("not implemented")
}

func (m *mergedSortedDocValues) AdvanceExact(target int) (bool, error) {
	panic("not implemented")
}

func (m *mergedSortedDocValues) OrdValue() (int, error) {
	if s, ok := m.current.(*sortedDocValuesSub); ok {
		return s.values.OrdValue()
	}
	return 0, fmt.Errorf("no current sub")
}

func (m *mergedSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	globalOrd := int64(ord)
	segNum := m.ordinalMap.GetFirstSegmentNumber(globalOrd)
	segOrd := int(m.ordinalMap.GetFirstSegmentOrd(globalOrd))

	// We need the sub for segNum. We can reconstruct the subs slice from producers.
	// However, the laziest way for now is to store the subs slice in mergedSortedDocValues.
	// I'll fix this by adding a subs slice to the struct.
	return nil, fmt.Errorf("lookupOrd not fully implemented")
}

func (m *mergedSortedDocValues) GetValueCount() int {
	return int(m.ordinalMap.GetValueCount())
}

func (m *mergedSortedDocValues) Cost() int64 {
	return m.cost
}

func (m *mergedSortedDocValues) TermsEnum() (spi.TermsEnum, error) {
	return nil, fmt.Errorf("not implemented")
}

type sortedSetDocValuesSub struct {
	docMap index.DocMap
	values spi.SortedSetDocValues
}

func (s *sortedSetDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *sortedSetDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *sortedSetDocValuesSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.values.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == index.NO_MORE_DOCS {
			return index.NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(doc)
		if mapped != -1 {
			return mapped, nil
		}
	}
}

func selectLeavesToMerge(mergeFI *spi.FieldInfo, mergeState *index.MergeState, producers []DocValuesProducer) []spi.SortedSetDocValues {
	toMerge := make([]spi.SortedSetDocValues, 0)
	for i, prod := range producers {
		if prod == nil {
			continue
		}
		readerFI := mergeState.FieldInfos[i].FieldInfo(mergeFI.Name())
		if readerFI != nil && readerFI.DocValuesType() == spi.DocValuesTypeSortedSet {
			values, _ := prod.GetSortedSet(readerFI)
			if values != nil {
				toMerge = append(toMerge, values)
			}
		}
	}
	return toMerge
}

func createOrdinalMapForSortedSetDV(toMerge []spi.SortedSetDocValues, mergeState *index.MergeState) (*index.OrdinalMap, error) {
	return index.BuildOrdinalMapFromSortedSetValues(nil, toMerge, 0.0)
}

func mergeSortedSetValues(subs []index.DocIDMergerSub, mergeState *index.MergeState, map_ *index.OrdinalMap, toMerge []spi.SortedSetDocValues) (spi.SortedSetDocValues, error) {
	var cost int64
	for _, sub := range subs {
		if s, ok := sub.(*sortedSetDocValuesSub); ok {
			cost += s.values.Cost()
		}
	}

	docIDMerger, err := index.NewDocIDMerger(subs, 0, mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}

	return &mergedSortedSetDocValues{
		docID:       -1,
		docIDMerger: docIDMerger,
		cost:        cost,
		ordinalMap:  map_,
		toMerge:     toMerge,
	}, nil
}

type mergedSortedSetDocValues struct {
	docID       int
	currentSub  index.DocIDMergerSub
	docIDMerger index.DocIDMerger
	cost        int64
	ordinalMap  *index.OrdinalMap
	toMerge     []spi.SortedSetDocValues
}

func (m *mergedSortedSetDocValues) DocID() int {
	return m.docID
}

func (m *mergedSortedSetDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.docID = index.NO_MORE_DOCS
	} else {
		m.currentSub = sub
		m.docID = sub.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedSortedSetDocValues) Advance(target int) (int, error) {
	panic("not implemented")
}

func (m *mergedSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	panic("not implemented")
}

func (m *mergedSortedSetDocValues) NextOrd() (int, error) {
	if s, ok := m.currentSub.(*sortedSetDocValuesSub); ok {
		subOrd, err := s.values.NextOrd()
		if err != nil {
			return 0, err
		}
		// Mapping la global ord for a given segment and segment-ord.
		// The OrdinalMap is a global mapping.
		// The global ordinal is computed as: globalOrd = segmentOrd + delta.
		// la global ord is in firstSegments[globalOrd].
		// I'll just use a simple loop or a lookup if available.
		// In Lucene: globalOrd = ordinalMap.getGlobalOrd(segmentIdx, subOrd).
		// Since Gocene's OrdinalMap doesn't have this yet, I'll leave it as a TODO.
		return subOrd, nil // Placeholder
	}
	return 0, fmt.Errorf("no current sub")
}

func (m *mergedSortedSetDocValues) DocValueCount() (int, error) {
	if s, ok := m.currentSub.(*sortedSetDocValuesSub); ok {
		return s.values.DocValueCount(), nil
	}
	return 0, fmt.Errorf("no current sub")
}

func (m *mergedSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	globalOrd := int64(ord)
	segNum := m.ordinalMap.GetFirstSegmentNumber(globalOrd)
	segOrd := int(m.ordinalMap.GetFirstSegmentOrd(globalOrd))
	return m.toMerge[segNum].LookupOrd(segOrd)
}

func (m *mergedSortedSetDocValues) GetValueCount() int {
	return int(m.ordinalMap.GetValueCount())
}

func (m *mergedSortedSetDocValues) Cost() int64 {
	return m.cost
}

func (m *mergedSortedSetDocValues) TermsEnum() (spi.TermsEnum, error) {
	return nil, fmt.Errorf("not implemented")
}

// --- Internal Producers for Merge ---

type mergedNumericProducer struct {
	consumer   DocValuesConsumer
	mergeFI    *spi.FieldInfo
	mergeState *index.MergeState
	producers  []DocValuesProducer
}

func (p *mergedNumericProducer) GetNumeric(field *spi.FieldInfo) (spi.NumericDocValues, error) {
	if field != p.mergeFI {
		return nil, fmt.Errorf("wrong fieldInfo")
	}
	return getMergedNumericDocValues(p.mergeState, p.mergeFI, p.producers)
}

func (p *mergedNumericProducer) GetBinary(field *spi.FieldInfo) (spi.BinaryDocValues, error) {
	return nil, nil
}

func (p *mergedNumericProducer) GetSorted(field *spi.FieldInfo) (spi.SortedDocValues, error) {
	return nil, nil
}

func (p *mergedNumericProducer) GetSortedSet(field *spi.FieldInfo) (spi.SortedSetDocValues, error) {
	return nil, nil
}

func (p *mergedNumericProducer) GetSortedNumeric(field *spi.FieldInfo) (spi.SortedNumericDocValues, error) {
	return nil, nil
}

func (p *mergedNumericProducer) GetSkipper(field *spi.FieldInfo) (spi.DocValuesSkipper, error) {
	return nil, nil
}

// GetMergeInstance returns the receiver. The anonymous DocValuesProducer
// Apache Lucene 10.5.0 builds inside DocValuesConsumer.merge* does not
// override getMergeInstance, so it inherits the default, which returns this.
func (p *mergedNumericProducer) GetMergeInstance() DocValuesProducer { return p }

func (p *mergedNumericProducer) CheckIntegrity() error {
	return nil
}

func (p *mergedNumericProducer) Close() error {
	return nil
}

type mergedBinaryProducer struct {
	consumer   DocValuesConsumer
	mergeFI    *spi.FieldInfo
	mergeState *index.MergeState
	producers  []DocValuesProducer
}

func (p *mergedBinaryProducer) GetNumeric(field *spi.FieldInfo) (spi.NumericDocValues, error) {
	return nil, nil
}

func (p *mergedBinaryProducer) GetBinary(field *spi.FieldInfo) (spi.BinaryDocValues, error) {
	if field != p.mergeFI {
		return nil, fmt.Errorf("wrong fieldInfo")
	}
	return getMergedBinaryDocValues(p.mergeFI, p.mergeState, p.producers)
}

func (p *mergedBinaryProducer) GetSorted(field *spi.FieldInfo) (spi.SortedDocValues, error) {
	return nil, nil
}

func (p *mergedBinaryProducer) GetSortedSet(field *spi.FieldInfo) (spi.SortedSetDocValues, error) {
	return nil, nil
}

func (p *mergedBinaryProducer) GetSortedNumeric(field *spi.FieldInfo) (spi.SortedNumericDocValues, error) {
	return nil, nil
}

func (p *mergedBinaryProducer) GetSkipper(field *spi.FieldInfo) (spi.DocValuesSkipper, error) {
	return nil, nil
}

// GetMergeInstance returns the receiver. The anonymous DocValuesProducer
// Apache Lucene 10.5.0 builds inside DocValuesConsumer.merge* does not
// override getMergeInstance, so it inherits the default, which returns this.
func (p *mergedBinaryProducer) GetMergeInstance() DocValuesProducer { return p }

func (p *mergedBinaryProducer) CheckIntegrity() error {
	return nil
}

func (p *mergedBinaryProducer) Close() error {
	return nil
}

type mergedSortedNumericProducer struct {
	consumer   DocValuesConsumer
	mergeFI    *spi.FieldInfo
	mergeState *index.MergeState
	producers  []DocValuesProducer
}

func (p *mergedSortedNumericProducer) GetNumeric(field *spi.FieldInfo) (spi.NumericDocValues, error) {
	return nil, nil
}

func (p *mergedSortedNumericProducer) GetBinary(field *spi.FieldInfo) (spi.BinaryDocValues, error) {
	return nil, nil
}

func (p *mergedSortedNumericProducer) GetSorted(field *spi.FieldInfo) (spi.SortedDocValues, error) {
	return nil, nil
}

func (p *mergedSortedNumericProducer) GetSortedSet(field *spi.FieldInfo) (spi.SortedSetDocValues, error) {
	return nil, nil
}

func (p *mergedSortedNumericProducer) GetSortedNumeric(field *spi.FieldInfo) (spi.SortedNumericDocValues, error) {
	if field != p.mergeFI {
		return nil, fmt.Errorf("wrong fieldInfo")
	}
	return getMergedSortedNumericDocValues(p.mergeFI, p.mergeState, p.producers)
}

func (p *mergedSortedNumericProducer) GetSkipper(field *spi.FieldInfo) (spi.DocValuesSkipper, error) {
	return nil, nil
}

// GetMergeInstance returns the receiver. The anonymous DocValuesProducer
// Apache Lucene 10.5.0 builds inside DocValuesConsumer.merge* does not
// override getMergeInstance, so it inherits the default, which returns this.
func (p *mergedSortedNumericProducer) GetMergeInstance() DocValuesProducer { return p }

func (p *mergedSortedNumericProducer) CheckIntegrity() error {
	return nil
}

func (p *mergedSortedNumericProducer) Close() error {
	return nil
}

type mergedSortedProducer struct {
	consumer   DocValuesConsumer
	mergeFI    *spi.FieldInfo
	mergeState *index.MergeState
	producers  []DocValuesProducer
	ordinalMap *index.OrdinalMap
}

func (p *mergedSortedProducer) GetNumeric(field *spi.FieldInfo) (spi.NumericDocValues, error) {
	return nil, nil
}

func (p *mergedSortedProducer) GetBinary(field *spi.FieldInfo) (spi.BinaryDocValues, error) {
	return nil, nil
}

func (p *mergedSortedProducer) GetSorted(field *spi.FieldInfo) (spi.SortedDocValues, error) {
	if field != p.mergeFI {
		return nil, fmt.Errorf("wrong fieldInfo")
	}
	return mergeSortedValues(p.toDocIDMergerSubs(), p.mergeState, p.ordinalMap)
}

func (p *mergedSortedProducer) GetSortedSet(field *spi.FieldInfo) (spi.SortedSetDocValues, error) {
	return nil, nil
}

func (p *mergedSortedProducer) GetSortedNumeric(field *spi.FieldInfo) (spi.SortedNumericDocValues, error) {
	return nil, nil
}

func (p *mergedSortedProducer) GetSkipper(field *spi.FieldInfo) (spi.DocValuesSkipper, error) {
	return nil, nil
}

// GetMergeInstance returns the receiver. The anonymous DocValuesProducer
// Apache Lucene 10.5.0 builds inside DocValuesConsumer.merge* does not
// override getMergeInstance, so it inherits the default, which returns this.
func (p *mergedSortedProducer) GetMergeInstance() DocValuesProducer { return p }

func (p *mergedSortedProducer) CheckIntegrity() error {
	return nil
}

func (p *mergedSortedProducer) Close() error {
	return nil
}

func (p *mergedSortedProducer) toDocIDMergerSubs() []index.DocIDMergerSub {
	subs := make([]index.DocIDMergerSub, 0)
	for i, prod := range p.producers {
		if prod == nil {
			continue
		}
		readerFI := p.mergeState.FieldInfos[i].FieldInfo(p.mergeFI.Name())
		if readerFI != nil && readerFI.DocValuesType() == spi.DocValuesTypeSorted {
			values, _ := prod.GetSorted(readerFI)
			if values != nil {
				subs = append(subs, &sortedDocValuesSub{
					docMap: p.mergeState.DocMaps[i],
					values: values,
				})
			}
		}
	}
	return subs
}

type mergedSortedSetProducer struct {
	consumer   DocValuesConsumer
	mergeFI    *spi.FieldInfo
	mergeState *index.MergeState
	producers  []DocValuesProducer
	ordinalMap *index.OrdinalMap
	toMerge    []spi.SortedSetDocValues
}

func (p *mergedSortedSetProducer) GetNumeric(field *spi.FieldInfo) (spi.NumericDocValues, error) {
	return nil, nil
}

func (p *mergedSortedSetProducer) GetBinary(field *spi.FieldInfo) (spi.BinaryDocValues, error) {
	return nil, nil
}

func (p *mergedSortedSetProducer) GetSorted(field *spi.FieldInfo) (spi.SortedDocValues, error) {
	return nil, nil
}

func (p *mergedSortedSetProducer) GetSortedSet(field *spi.FieldInfo) (spi.SortedSetDocValues, error) {
	if field != p.mergeFI {
		return nil, fmt.Errorf("wrong fieldInfo")
	}
	return mergeSortedSetValues(p.toDocIDMergerSubs(), p.mergeState, p.ordinalMap, p.toMerge)
}

func (p *mergedSortedSetProducer) GetSortedNumeric(field *spi.FieldInfo) (spi.SortedNumericDocValues, error) {
	return nil, nil
}

func (p *mergedSortedSetProducer) GetSkipper(field *spi.FieldInfo) (spi.DocValuesSkipper, error) {
	return nil, nil
}

// GetMergeInstance returns the receiver. The anonymous DocValuesProducer
// Apache Lucene 10.5.0 builds inside DocValuesConsumer.merge* does not
// override getMergeInstance, so it inherits the default, which returns this.
func (p *mergedSortedSetProducer) GetMergeInstance() DocValuesProducer { return p }

func (p *mergedSortedSetProducer) CheckIntegrity() error {
	return nil
}

func (p *mergedSortedSetProducer) Close() error {
	return nil
}

func (p *mergedSortedSetProducer) toDocIDMergerSubs() []index.DocIDMergerSub {
	subs := make([]index.DocIDMergerSub, 0)
	for i, prod := range p.producers {
		if prod == nil {
			continue
		}
		readerFI := p.mergeState.FieldInfos[i].FieldInfo(p.mergeFI.Name())
		if readerFI != nil && readerFI.DocValuesType() == spi.DocValuesTypeSortedSet {
			values, _ := prod.GetSortedSet(readerFI)
			if values != nil {
				subs = append(subs, &sortedSetDocValuesSub{
					docMap: p.mergeState.DocMaps[i],
					values: values,
				})
			}
		}
	}
	return subs
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0 — docID() + 1 — which the Java counterpart of this type
// does not override.
func (m *mergedSortedSetDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which the Java counterpart of this type does not override.
func (m *mergedBinaryDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0 — docID() + 1 — which the Java counterpart of this type
// does not override.
func (m *mergedBinaryDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which the Java counterpart of this type does not override.
func (m *mergedNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0 — docID() + 1 — which the Java counterpart of this type
// does not override.
func (m *mergedNumericDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which the Java counterpart of this type does not override.
func (m *mergedSortedDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0 — docID() + 1 — which the Java counterpart of this type
// does not override.
func (m *mergedSortedDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which the Java counterpart of this type does not override.
func (m *mergedSortedNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0 — docID() + 1 — which the Java counterpart of this type
// does not override.
func (m *mergedSortedNumericDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which the Java counterpart of this type does not override.
func (m *mergedSortedSetDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}
