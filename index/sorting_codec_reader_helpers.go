//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortingBits wraps a bitset and a doc map to provide a sorted view of live docs.
type SortingBits struct {
	in     util.Bits
	docMap *SorterDocMap
}

func NewSortingBits(in util.Bits, docMap *SorterDocMap) util.Bits {
	return &SortingBits{in: in, docMap: docMap}
}

func (b *SortingBits) Get(index int) bool {
	return b.in.Get(b.docMap.NewToOld(index))
}

func (b *SortingBits) Length() int {
	return b.in.Length()
}

// SortingPointValues wraps PointValues to provide a sorted view.
type SortingPointValues struct {
	in     PointValues
	docMap *SorterDocMap
}

func NewSortingPointValues(in PointValues, docMap *SorterDocMap) PointValues {
	return &SortingPointValues{in: in, docMap: docMap}
}

func (p *SortingPointValues) GetPointTree() (PointTree, error) {
	return NewSortingPointTree(p.in.GetPointTree(), p.docMap), nil
}

func (p *SortingPointValues) GetMinPackedValue() ([]byte, error) {
	return p.in.GetMinPackedValue()
}

func (p *SortingPointValues) GetMaxPackedValue() ([]byte, error) {
	return p.in.GetMaxPackedValue()
}

func (p *SortingPointValues) GetNumDimensions() int {
	return p.in.GetNumDimensions()
}

func (p *SortingPointValues) GetBytesPerDimension() int {
	return p.in.GetBytesPerDimension()
}

func (p *SortingPointValues) Size() int64 {
	return p.in.Size()
}

func (p *SortingPointValues) GetDocCount() int {
	return p.in.GetDocCount()
}

// SortingPointTree wraps a PointTree to provide a sorted view of visited docs.
type SortingPointTree struct {
	indexTree PointTree
	docMap    *SorterDocMap
	visitor   *sortingIntersectVisitor
}

func NewSortingPointTree(indexTree PointTree, docMap *SorterDocMap) PointTree {
	return &SortingPointTree{
		indexTree: indexTree,
		docMap:    docMap,
		visitor:   &sortingIntersectVisitor{docMap: docMap},
	}
}

func (t *SortingPointTree) Clone() PointTree {
	return NewSortingPointTree(t.indexTree.Clone(), t.docMap)
}

func (t *SortingPointTree) MoveToChild() bool {
	return t.indexTree.MoveToChild()
}

func (t *SortingPointTree) MoveToSibling() bool {
	return t.indexTree.MoveToSibling()
}

func (t *SortingPointTree) MoveToParent() bool {
	return t.indexTree.MoveToParent()
}

func (t *SortingPointTree) GetMinPackedValue() []byte {
	return t.indexTree.GetMinPackedValue()
}

func (t *SortingPointTree) GetMaxPackedValue() []byte {
	return t.indexTree.GetMaxPackedValue()
}

func (t *SortingPointTree) Size() int64 {
	return t.indexTree.Size()
}

func (t *SortingPointTree) VisitDocIDs(visitor PointTreeIntersectVisitor) error {
	t.visitor.setIntersectVisitor(visitor)
	return t.indexTree.VisitDocIDs(t.visitor)
}

func (t *SortingPointTree) VisitDocValues(visitor PointTreeIntersectVisitor) error {
	t.visitor.setIntersectVisitor(visitor)
	return t.indexTree.VisitDocValues(t.visitor)
}

type sortingIntersectVisitor struct {
	docMap   *SorterDocMap
	visitor PointTreeIntersectVisitor
}

func (v *sortingIntersectVisitor) setIntersectVisitor(visitor PointTreeIntersectVisitor) {
	v.visitor = visitor
}

func (v *sortingIntersectVisitor) Visit(docID int) error {
	return v.visitor.Visit(v.docMap.OldToNew(docID))
}

func (v *sortingIntersectVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	return v.visitor.VisitByPackedValue(v.docMap.OldToNew(docID), packedValue)
}

func (v *sortingIntersectVisitor) Compare(minPackedValue, maxPackedValue []byte) int {
	return v.visitor.Compare(minPackedValue, maxPackedValue)
}

// SortingIteratorSupplier caches the mapping from sorted doc IDs to original ords.
type SortingIteratorSupplier struct {
	docBits   *util.FixedBitSet
	docToOrd  []int
	size      int
}

func NewSortingIteratorSupplier(values KnnVectorValues, docMap *SorterDocMap) (*SortingIteratorSupplier, error) {
	docToOrd := make([]int, docMap.Size())
	docBits, err := util.NewFixedBitSet(docMap.Size())
	if err != nil {
		return nil, err
	}
	count := 0
	iter := values.Iterator()
	for doc := iter.NextDoc(); doc != -1; doc = iter.NextDoc() {
		newDocID := docMap.OldToNew(doc)
		if newDocID != -1 {
			docToOrd[newDocID] = iter.Index()
			docBits.Set(newDocID)
			count++
		}
	}
	return &SortingIteratorSupplier{
		docBits:   docBits,
		docToOrd:  docToOrd,
		size:      count,
	}, nil
}

func (s *SortingIteratorSupplier) Get() *SortingValuesIterator {
	return &SortingValuesIterator{
		docBits:   s.docBits,
		docToOrd:  s.docToOrd,
		docsWithValues: &bitSetIterator{
			bits: s.docBits,
			size: s.size,
		},
	}
}

func (s *SortingIteratorSupplier) Size() int {
	return s.size
}

// SortingValuesIterator iterates over vectors accepting a mapping to differently-sorted docs.
type SortingValuesIterator struct {
	docBits        *util.FixedBitSet
	docToOrd       []int
	docsWithValues *bitSetIterator
	doc            int
}

func (it *SortingValuesIterator) DocID() int {
	return it.doc
}

func (it *SortingValuesIterator) Index() int {
	return it.docToOrd[it.doc]
}

func (it *SortingValuesIterator) NextDoc() int {
	if it.doc != -1 {
		it.doc = it.docsWithValues.NextDoc()
	}
	return it.doc
}

func (it *SortingValuesIterator) Cost() int64 {
	return int64(it.docBits.Cardinality())
}

type bitSetIterator struct {
	bits *util.FixedBitSet
	size int
}

func (it *bitSetIterator) NextDoc() int {
	// Simplified implementation of Lucene's BitSetIterator
	// In real use, this would use the BitSet's internal representation for speed
	// For now we'll assume the underlying util.FixedBitSet has a way to find the next set bit
	// or we'll implement a simple scan.
	// Since util.FixedBitSet might not have NextSetBit, we'll just iterate for now.
	// (In a real port, we'd add NextSetBit to util.FixedBitSet)
	return -1 // Placeholder: needs implementation in util.FixedBitSet
}

// SortingFloatVectorValues wraps FloatVectorValues to provide a sorted view.
type SortingFloatVectorValues struct {
	delegate         FloatVectorValues
	iteratorSupplier *SortingIteratorSupplier
}

func NewSortingFloatVectorValues(delegate FloatVectorValues, sortMap *SorterDocMap) (*SortingFloatVectorValues, error) {
	supplier, err := NewSortingIteratorSupplier(delegate, sortMap)
	if err != nil {
		return nil, err
	}
	return &SortingFloatVectorValues{
		delegate:         delegate,
		iteratorSupplier: supplier,
	}, nil
}

func (v *SortingFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	return v.delegate.VectorValue(ord)
}

func (v *SortingFloatVectorValues) Dimension() int {
	return v.delegate.Dimension()
}

func (v *SortingFloatVectorValues) Size() int {
	return v.iteratorSupplier.Size()
}

func (v *SortingFloatVectorValues) Iterator() KnnVectorValues.DocIndexIterator {
	return v.iteratorSupplier.Get()
}

// SortingByteVectorValues wraps ByteVectorValues to provide a sorted view.
type SortingByteVectorValues struct {
	delegate         ByteVectorValues
	iteratorSupplier *SortingIteratorSupplier
}

func NewSortingByteVectorValues(delegate ByteVectorValues, sortMap *SorterDocMap) (*SortingByteVectorValues, error) {
	supplier, err := NewSortingIteratorSupplier(delegate, sortMap)
	if err != nil {
		return nil, err
	}
	return &SortingByteVectorValues{
		delegate:         delegate,
		iteratorSupplier: supplier,
	}, nil
}

func (v *SortingByteVectorValues) VectorValue(ord int) ([]byte, error) {
	return v.delegate.VectorValue(ord)
}

func (v *SortingByteVectorValues) Iterator() KnnVectorValues.DocIndexIterator {
	return v.iteratorSupplier.Get()
}

func (v *SortingByteVectorValues) Dimension() int {
	return v.delegate.Dimension()
}

func (v *SortingByteVectorValues) Size() int {
	return v.iteratorSupplier.Size()
}
