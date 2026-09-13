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

// BaseKnnVectorsWriter provides a default implementation of the merge logic
// for KNN vector writers. It is the Go port of org.apache.lucene.codecs.KnnVectorsWriter
// from Apache Lucene 10.5.0.
//
// Concrete codec writers embed this struct to inherit the standard two-phase
// merge strategy.
type BaseKnnVectorsWriter struct {
	// writer is a reference to the concrete implementation of spi.KnnVectorsWriter.
	// It is used to call AddField during merge.
	writer spi.KnnVectorsWriter
}

// NewBaseKnnVectorsWriter constructs a BaseKnnVectorsWriter.
func NewBaseKnnVectorsWriter(writer spi.KnnVectorsWriter) *BaseKnnVectorsWriter {
	return &BaseKnnVectorsWriter{
		writer: writer,
	}
}

// MergeOneField merges vectors for a single field.
//
// This is the Go port of KnnVectorsWriter.mergeOneField.
// It implements a naive merge by default, returning nil for deferred work.
// Subclasses (via embedding) can override this to implement a two-phase merge strategy
// (e.g., for HNSW graph construction).
func (b *BaseKnnVectorsWriter) MergeOneField(fieldInfo *spi.FieldInfo, mergeState *index.MergeState) (func() error, error) {
	switch fieldInfo.VectorEncoding() {
	case index.VectorEncodingByte:
		fieldWriter, err := b.writer.AddField(fieldInfo)
		if err != nil {
			return nil, err
		}
		byteWriter, ok := fieldWriter.(TypedKnnFieldVectorsWriter[byte])
		if !ok {
			return nil, fmt.Errorf("expected TypedKnnFieldVectorsWriter[byte] for field %s", fieldInfo.Name())
		}
		mergedBytes := mergeByteVectorValues(fieldInfo, mergeState)
		iter := mergedBytes.Iterator()
		for doc, err := iter.NextDoc(); err == nil && doc != util.NO_MORE_DOCS; doc, err = iter.NextDoc() {
			val, err := mergedBytes.VectorValue(iter.Index())
			if err != nil {
				return nil, err
			}
			if err := byteWriter.AddValue(doc, val); err != nil {
				return nil, err
			}
		}
	case index.VectorEncodingFloat32:
		fieldWriter, err := b.writer.AddField(fieldInfo)
		if err != nil {
			return nil, err
		}
		floatWriter, ok := fieldWriter.(TypedKnnFieldVectorsWriter[float32])
		if !ok {
			return nil, fmt.Errorf("expected TypedKnnFieldVectorsWriter[float32] for field %s", fieldInfo.Name())
		}
		mergedFloats := mergeFloatVectorValues(fieldInfo, mergeState)
		iter := mergedFloats.Iterator()
		for doc, err := iter.NextDoc(); err == nil && doc != util.NO_MORE_DOCS; doc, err = iter.NextDoc() {
			val, err := mergedFloats.VectorValue(iter.Index())
			if err != nil {
				return nil, err
			}
			if err := floatWriter.AddValue(doc, val); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

// Merge merges the segment vectors for all fields using a two-phase strategy.
//
// This is the Go port of KnnVectorsWriter.merge.
//
// Phase 1: Merge flat vectors for all fields by calling MergeOneField,
// collecting deferred work (closures) for each field.
//
// Phase 2: Execute the deferred closures (e.g., HNSW graph construction)
// using the flat vector data written in phase 1.
func (b *BaseKnnVectorsWriter) Merge(mergeState *index.MergeState) error {
	for i := 0; i < len(mergeState.Readers); i++ {
		reader, ok := mergeState.Readers[i].(KnnVectorsReader)
		if !ok {
			if mergeState.FieldInfos[i] != nil && mergeState.FieldInfos[i].HasVectorValues() {
				return fmt.Errorf("reader at index %d is not a KnnVectorsReader but field has vector values", i)
			}
			continue
		}
		if err := mergeState.CheckAborted(); err != nil {
			return err
		}
		if err := reader.CheckIntegrity(); err != nil {
			return err
		}
	}

	// Phase 1: merge flat vectors for all fields, collecting deferred work
	var deferredWork []func() error
	for _, fieldInfo := range mergeState.MergeFieldInfos.Iterator() {
		if fieldInfo.HasVectorValues() {
			deferred, err := b.MergeOneField(fieldInfo, mergeState)
			if err != nil {
				return err
			}
			if deferred != nil {
				deferredWork = append(deferredWork, deferred)
			}
		}
	}

	// Phase 2: execute deferred work (e.g., graph construction using the written flat vectors)
	for _, runnable := range deferredWork {
		if err := runnable(); err != nil {
			return err
		}
	}

	return b.writer.Finish()
}

// MapOldOrdToNewOrd maps old ordinals to new ordinals given old doc IDs and an ID mapping.
//
// This is the Go port of KnnVectorsWriter.mapOldOrdToNewOrd.
func MapOldOrdToNewOrd(
	oldDocIds util.Bits,
	sortMap index.DocMap,
	old2NewOrd []int,
	new2OldOrd []int,
	newDocsWithField util.Bits,
) error {
	if oldDocIds == nil {
		return fmt.Errorf("oldDocIds must not be nil")
	}
	if sortMap == nil {
		return fmt.Errorf("sortMap must not be nil")
	}

	newIdToOldOrd := make(map[int]int)
	newDocIds := make([]int, 0, oldDocIds.Cardinality())

	iter := oldDocIds.Iterator()
	oldOrd := 0
	for docID, err := iter.NextDoc(); err == nil && docID != util.NO_MORE_DOCS; docID, err = iter.NextDoc() {
		newID := sortMap.Get(docID)
		newIdToOldOrd[newID] = oldOrd
		newDocIds = append(newDocIds, newID)
		oldOrd++
	}

	util.SortInts(newDocIds)

	newOrd := 0
	for _, newDocID := range newDocIds {
		currOldOrd, ok := newIdToOldOrd[newDocID]
		if !ok {
			return fmt.Errorf("mapping failed for newDocID %d", newDocID)
		}
		if old2NewOrd != nil {
			old2NewOrd[currOldOrd] = newOrd
		}
		if new2OldOrd != nil {
			new2OldOrd[newOrd] = currOldOrd
		}
		if newDocsWithField != nil {
			newDocsWithField.Set(newDocID)
		}
		newOrd++
	}

	return nil
}

// --- Merged Vector Values Implementation ---

func mergeFloatVectorValues(fieldInfo *spi.FieldInfo, mergeState *index.MergeState) index.FloatVectorValues {
	if fieldInfo.VectorEncoding() != index.VectorEncodingFloat32 {
		panic(fmt.Sprintf("cannot merge vectors encoded as [%s] as FLOAT32", fieldInfo.VectorEncoding()))
	}

	var subs []*floatVectorValuesSub
	for i, knnReader := range mergeState.Readers {
		reader, ok := knnReader.(KnnVectorsReader)
		if !ok {
			continue
		}
		sourceFieldInfos := mergeState.FieldInfos[i]
		if sourceFieldInfos == nil || !hasVectorValues(sourceFieldInfos, fieldInfo.Name()) {
			continue
		}

		values, err := reader.GetFloatVectorValues(fieldInfo.Name())
		if err != nil || values == nil {
			continue
		}
		subs = append(subs, &floatVectorValuesSub{
			docMap: mergeState.DocMaps[i],
			values: values,
			iter:   values.Iterator(),
		})
	}

	var mergerSubs []index.DocIDMergerSub
	for _, sub := range subs {
		mergerSubs = append(mergerSubs, sub)
	}
	merger, err := index.NewDocIDMerger(mergerSubs, 0, mergeState.NeedsIndexSort)
	if err != nil {
		panic(err)
	}

	return &mergedFloat32VectorValues{
		subs:        subs,
		docIdMerger: merger,
		size:        calculateTotalSize(subs, func(s *floatVectorValuesSub) int { return s.values.Size() }),
	}
}

func mergeByteVectorValues(fieldInfo *spi.FieldInfo, mergeState *index.MergeState) index.ByteVectorValues {
	if fieldInfo.VectorEncoding() != index.VectorEncodingByte {
		panic(fmt.Sprintf("cannot merge vectors encoded as [%s] as BYTE", fieldInfo.VectorEncoding()))
	}

	var subs []*byteVectorValuesSub
	for i, knnReader := range mergeState.Readers {
		reader, ok := knnReader.(KnnVectorsReader)
		if !ok {
			continue
		}
		sourceFieldInfos := mergeState.FieldInfos[i]
		if sourceFieldInfos == nil || !hasVectorValues(sourceFieldInfos, fieldInfo.Name()) {
			continue
		}

		values, err := reader.GetByteVectorValues(fieldInfo.Name())
		if err != nil || values == nil {
			continue
		}
		subs = append(subs, &byteVectorValuesSub{
			docMap: mergeState.DocMaps[i],
			values: values,
			iter:   values.Iterator(),
		})
	}

	var mergerSubs []index.DocIDMergerSub
	for _, sub := range subs {
		mergerSubs = append(mergerSubs, sub)
	}
	merger, err := index.NewDocIDMerger(mergerSubs, 0, mergeState.NeedsIndexSort)
	if err != nil {
		panic(err)
	}

	return &mergedByteVectorValues{
		subs:        subs,
		docIdMerger: merger,
		size:        calculateTotalSize(subs, func(s *byteVectorValuesSub) int { return s.values.Size() }),
	}
}

func hasVectorValues(fieldInfos *index.FieldInfos, fieldName string) bool {
	if fieldInfos == nil || !fieldInfos.HasVectorValues() {
		return false
	}
	info := fieldInfos.FieldInfo(fieldName)
	return info != nil && info.HasVectorValues()
}

func calculateTotalSize[T any](subs []*T, sizeFn func(*T) int) int {
	total := 0
	for _, sub := range subs {
		total += sizeFn(sub)
	}
	return total
}

type floatVectorValuesSub struct {
	docMap index.DocMap
	values index.FloatVectorValues
	iter   util.DocIndexIterator
}

func (s *floatVectorValuesSub) MappedDocID() int {
	return s.docMap.Get(s.iter.DocID())
}

func (s *floatVectorValuesSub) NextDoc() (int, error) {
	return s.iter.NextDoc()
}

func (s *floatVectorValuesSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.iter.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == util.NO_MORE_DOCS {
			return util.NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(doc)
		if mapped != -1 {
			return mapped, nil
		}
	}
}

type byteVectorValuesSub struct {
	docMap index.DocMap
	values index.ByteVectorValues
	iter   util.DocIndexIterator
}

func (s *byteVectorValuesSub) MappedDocID() int {
	return s.docMap.Get(s.iter.DocID())
}

func (s *byteVectorValuesSub) NextDoc() (int, error) {
	return s.iter.NextDoc()
}

func (s *byteVectorValuesSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.iter.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == util.NO_MORE_DOCS {
			return util.NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(doc)
		if mapped != -1 {
			return mapped, nil
		}
	}
}

type mergedFloat32VectorValues struct {
	subs        []*floatVectorValuesSub
	docIdMerger index.DocIDMerger
	size        int
	docId       int
	lastOrd     int
	current     *floatVectorValuesSub
}

func (m *mergedFloat32VectorValues) Dimension() int {
	if len(m.subs) == 0 {
		return 0
	}
	return m.subs[0].values.Dimension()
}

func (m *mergedFloat32VectorValues) Size() int {
	return m.size
}

func (m *mergedFloat32VectorValues) OrdToDoc(ord int) int {
	panic("not implemented")
}

func (m *mergedFloat32VectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error {
	panic("not implemented")
}

func (m *mergedFloat32VectorValues) Copy() (index.KnnVectorValues, error) {
	panic("not implemented")
}

func (m *mergedFloat32VectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

func (m *mergedFloat32VectorValues) GetVectorByteLength() int {
	return m.Dimension() * index.VectorEncodingByteSize(m.GetEncoding())
}

func (m *mergedFloat32VectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	panic("not implemented")
}

func (m *mergedFloat32VectorValues) Iterator() util.DocIndexIterator {
	return &mergedVectorIterator{
		parent: m,
	}
}

func (m *mergedFloat32VectorValues) VectorValue(ord int) ([]float32, error) {
	if ord != m.lastOrd {
		return nil, fmt.Errorf("only supports forward iteration: ord=%d, lastOrd=%d", ord, m.lastOrd)
	}
	return m.current.values.VectorValue(m.current.iter.Index())
}

func (m *mergedFloat32VectorValues) CopyFloatVectorValues() (index.FloatVectorValues, error) {
	panic("not implemented")
}

func (m *mergedFloat32VectorValues) Scorer(target []float32) (interface{}, error) {
	panic("not implemented")
}

func (m *mergedFloat32VectorValues) Rescorer(target []float32) (interface{}, error) {
	panic("not implemented")
}

type mergedByteVectorValues struct {
	subs        []*byteVectorValuesSub
	docIdMerger index.DocIDMerger
	size        int
	docId       int
	lastOrd     int
	current     *byteVectorValuesSub
}

func (m *mergedByteVectorValues) Dimension() int {
	if len(m.subs) == 0 {
		return 0
	}
	return m.subs[0].values.Dimension()
}

func (m *mergedByteVectorValues) Size() int {
	return m.size
}

func (m *mergedByteVectorValues) OrdToDoc(ord int) int {
	panic("not implemented")
}

func (m *mergedByteVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error {
	panic("not implemented")
}

func (m *mergedByteVectorValues) Copy() (index.KnnVectorValues, error) {
	panic("not implemented")
}

func (m *mergedByteVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingByte
}

func (m *mergedByteVectorValues) GetVectorByteLength() int {
	return m.Dimension() * index.VectorEncodingByteSize(m.GetEncoding())
}

func (m *mergedByteVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	panic("not implemented")
}

func (m *mergedByteVectorValues) Iterator() util.DocIndexIterator {
	return &mergedVectorIterator{
		parent: m,
	}
}

func (m *mergedByteVectorValues) VectorValue(ord int) ([]byte, error) {
	if ord != m.lastOrd {
		return nil, fmt.Errorf("only supports forward iteration: ord=%d, lastOrd=%d", ord, m.lastOrd)
	}
	return m.current.values.VectorValue(m.current.iter.Index())
}

func (m *mergedByteVectorValues) CopyByteVectorValues() (index.ByteVectorValues, error) {
	panic("not implemented")
}

func (m *mergedByteVectorValues) Scorer(target []byte) (util.VectorScorer, error) {
	panic("not implemented")
}

func (m *mergedByteVectorValues) Rescorer(target []byte) (util.VectorScorer, error) {
	panic("not implemented")
}

type mergedVectorIterator struct {
	parent any
}

func (it *mergedVectorIterator) DocID() int {
	if m, ok := it.parent.(*mergedFloat32VectorValues); ok {
		return m.docId
	}
	if m, ok := it.parent.(*mergedByteVectorValues); ok {
		return m.docId
	}
	return util.NO_MORE_DOCS
}

func (it *mergedVectorIterator) Index() int {
	if m, ok := it.parent.(*mergedFloat32VectorValues); ok {
		return m.lastOrd
	}
	if m, ok := it.parent.(*mergedByteVectorValues); ok {
		return m.lastOrd
	}
	return util.NO_MORE_DOCS
}

func (it *mergedVectorIterator) NextDoc() (int, error) {
	if m, ok := it.parent.(*mergedFloat32VectorValues); ok {
		sub := m.docIdMerger.Next()
		if sub == nil {
			m.docId = util.NO_MORE_DOCS
			m.lastOrd = util.NO_MORE_DOCS
		} else {
			m.current = sub.(*floatVectorValuesSub)
			m.docId = m.current.MappedDocID()
			m.lastOrd++
		}
		return m.docId, nil
	}
	if m, ok := it.parent.(*mergedByteVectorValues); ok {
		sub := m.docIdMerger.Next()
		if sub == nil {
			m.docId = util.NO_MORE_DOCS
			m.lastOrd = util.NO_MORE_DOCS
		} else {
			m.current = sub.(*byteVectorValuesSub)
			m.docId = m.current.MappedDocID()
			m.lastOrd++
		}
		return m.docId, nil
	}
	return util.NO_MORE_DOCS, nil
}

func (it *mergedVectorIterator) Advance(target int) (int, error) {
	panic("not implemented")
}

func (it *mergedVectorIterator) Cost() int64 {
	if m, ok := it.parent.(*mergedFloat32VectorValues); ok {
		return int64(m.size)
	}
	if m, ok := it.parent.(*mergedByteVectorValues); ok {
		return int64(m.size)
	}
	return 0
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0 — docID() + 1 — which the Java counterpart of this type
// does not override.
func (it *mergedVectorIterator) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(it)
}
