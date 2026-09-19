// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortingBits wraps a bitset and a doc map to provide a sorted view of live docs.
// Mirrors org.apache.lucene.index.SortingCodecReader.SortingBits.
type SortingBits struct {
	in     util.Bits
	docMap SorterDocMap
}

func NewSortingBits(in util.Bits, docMap SorterDocMap) util.Bits {
	return &SortingBits{in: in, docMap: docMap}
}

func (b *SortingBits) Get(index int) bool {
	return b.in.Get(b.docMap.NewToOld(index))
}

func (b *SortingBits) Length() int {
	return b.in.Length()
}

// SortingPointValues wraps PointValues to provide a sorted view.
// Mirrors org.apache.lucene.index.SortingCodecReader.SortingPointValues.
type SortingPointValues struct {
	*spi.BasePointValues
	in     spi.PointValues
	docMap SorterDocMap
}

func NewSortingPointValues(in spi.PointValues, docMap SorterDocMap) spi.PointValues {
	p := &SortingPointValues{in: in, docMap: docMap}
	p.BasePointValues = spi.NewBasePointValues(p)
	return p
}

func (p *SortingPointValues) GetPointTree() (PointTree, error) {
	tree, err := p.in.GetPointTree()
	if err != nil {
		return nil, err
	}
	return NewSortingPointTree(tree, p.docMap), nil
}

func (p *SortingPointValues) GetMinPackedValue() ([]byte, error) {
	return p.in.GetMinPackedValue()
}

func (p *SortingPointValues) GetMaxPackedValue() ([]byte, error) {
	return p.in.GetMaxPackedValue()
}

func (p *SortingPointValues) GetNumDimensions() (int, error) {
	return p.in.GetNumDimensions()
}

func (p *SortingPointValues) GetNumIndexDimensions() (int, error) {
	return p.in.GetNumIndexDimensions()
}

func (p *SortingPointValues) GetBytesPerDimension() (int, error) {
	return p.in.GetBytesPerDimension()
}

// Size returns the total number of indexed point values, mirroring
// PointValues.size().
func (p *SortingPointValues) Size() int64 {
	return p.in.Size()
}

func (p *SortingPointValues) GetDocCount() int {
	return p.in.GetDocCount()
}

// SortingPointTree wraps a PointTree to provide a sorted view of visited docs.
// Mirrors org.apache.lucene.index.SortingCodecReader.SortingPointTree.
type SortingPointTree struct {
	indexTree PointTree
	docMap    SorterDocMap
	visitor   *sortingIntersectVisitor
}

func NewSortingPointTree(indexTree PointTree, docMap SorterDocMap) PointTree {
	return &SortingPointTree{
		indexTree: indexTree,
		docMap:    docMap,
		visitor:   &sortingIntersectVisitor{docMap: docMap},
	}
}

func (t *SortingPointTree) Clone() PointTree {
	return NewSortingPointTree(t.indexTree.Clone(), t.docMap)
}

func (t *SortingPointTree) MoveToChild() (bool, error) {
	return t.indexTree.MoveToChild()
}

func (t *SortingPointTree) MoveToSibling() (bool, error) {
	return t.indexTree.MoveToSibling()
}

func (t *SortingPointTree) MoveToParent() (bool, error) {
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

func (t *SortingPointTree) VisitDocIDs(visitor IntersectVisitor) error {
	t.visitor.setIntersectVisitor(visitor)
	return t.indexTree.VisitDocIDs(t.visitor)
}

func (t *SortingPointTree) VisitDocValues(visitor IntersectVisitor) error {
	t.visitor.setIntersectVisitor(visitor)
	return t.indexTree.VisitDocValues(t.visitor)
}

// sortingIntersectVisitor remaps each visited docID from the source order
// into the sorted order before forwarding it to the caller's visitor.
// Mirrors SortingCodecReader.SortingIntersectVisitor.
type sortingIntersectVisitor struct {
	docMap  SorterDocMap
	visitor IntersectVisitor
}

func (v *sortingIntersectVisitor) setIntersectVisitor(visitor IntersectVisitor) {
	v.visitor = visitor
}

func (v *sortingIntersectVisitor) Visit(docID int) error {
	return v.visitor.Visit(v.docMap.OldToNew(docID))
}

func (v *sortingIntersectVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	return v.visitor.VisitByPackedValue(v.docMap.OldToNew(docID), packedValue)
}

func (v *sortingIntersectVisitor) Compare(minPackedValue, maxPackedValue []byte) geo.Relation {
	return v.visitor.Compare(minPackedValue, maxPackedValue)
}

func (v *sortingIntersectVisitor) Grow(count int) {
	v.visitor.Grow(count)
}

// SortingIteratorSupplier caches the mapping from sorted doc IDs to original ords.
// Mirrors org.apache.lucene.index.SortingCodecReader.SortingIteratorSupplier.
type SortingIteratorSupplier struct {
	docBits  *util.FixedBitSet
	docToOrd []int
	size     int
}

// NewSortingIteratorSupplier consumes values' iterator once and records, per
// sorted docID, the delegate ordinal that carries its vector. Mirrors
// SortingCodecReader.iteratorSupplier(KnnVectorValues, Sorter.DocMap).
func NewSortingIteratorSupplier(values KnnVectorValues, docMap SorterDocMap) (*SortingIteratorSupplier, error) {
	docToOrd := make([]int, docMap.Size())
	docBits, err := util.NewFixedBitSet(docMap.Size())
	if err != nil {
		return nil, err
	}
	count := 0
	iter := values.Iterator()
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return nil, err
		}
		if doc == util.NO_MORE_DOCS || doc < 0 || doc >= docMap.Size() {
			break
		}
		newDocID := docMap.OldToNew(doc)
		if newDocID != -1 {
			docToOrd[newDocID] = iter.Index()
			docBits.Set(newDocID)
			count++
		}
	}
	return &SortingIteratorSupplier{
		docBits:  docBits,
		docToOrd: docToOrd,
		size:     count,
	}, nil
}

func (s *SortingIteratorSupplier) Get() *SortingValuesIterator {
	return &SortingValuesIterator{
		docBits:        s.docBits,
		docToOrd:       s.docToOrd,
		docsWithValues: util.NewBitSetIterator(s.docBits, int64(s.size)),
		doc:            -1,
	}
}

func (s *SortingIteratorSupplier) Size() int {
	return s.size
}

// SortingValuesIterator iterates over vectors accepting a mapping to differently-sorted docs.
// Mirrors org.apache.lucene.index.SortingCodecReader.SortingValuesIterator.
type SortingValuesIterator struct {
	docBits        *util.FixedBitSet
	docToOrd       []int
	docsWithValues *util.BitSetIterator
	doc            int
}

func (it *SortingValuesIterator) DocID() int {
	return it.doc
}

func (it *SortingValuesIterator) Index() int {
	return it.docToOrd[it.doc]
}

func (it *SortingValuesIterator) NextDoc() (int, error) {
	if it.doc != util.NO_MORE_DOCS {
		next, err := it.docsWithValues.NextDoc()
		if err != nil {
			return it.doc, err
		}
		it.doc = next
	}
	return it.doc, nil
}

// Advance is unsupported: the sorted view is consumed strictly in order.
// Mirrors SortingValuesIterator.advance, which throws
// UnsupportedOperationException.
func (it *SortingValuesIterator) Advance(target int) (int, error) {
	return it.doc, fmt.Errorf("index: SortingValuesIterator: Advance is not supported")
}

// DocIDRunEnd assumes runs of a single doc ID and returns DocID()+1, the
// default of org.apache.lucene.search.DocIdSetIterator.docIDRunEnd.
func (it *SortingValuesIterator) DocIDRunEnd() (int, error) {
	return it.doc + 1, nil
}

func (it *SortingValuesIterator) Cost() int64 {
	return int64(it.docBits.Cardinality())
}

// SortingFloatVectorValues wraps FloatVectorValues to provide a sorted view.
// Mirrors org.apache.lucene.index.SortingCodecReader.SortingFloatVectorValues.
type SortingFloatVectorValues struct {
	delegate         FloatVectorValues
	iteratorSupplier *SortingIteratorSupplier
}

func NewSortingFloatVectorValues(delegate FloatVectorValues, sortMap SorterDocMap) (*SortingFloatVectorValues, error) {
	if delegate == nil {
		return nil, fmt.Errorf("index: SortingFloatVectorValues: delegate is nil")
	}
	// SortingValuesIterator consumes the iterator and records the docs and ord mapping.
	supplier, err := NewSortingIteratorSupplier(delegate, sortMap)
	if err != nil {
		return nil, err
	}
	return &SortingFloatVectorValues{
		delegate:         delegate,
		iteratorSupplier: supplier,
	}, nil
}

// VectorValue returns the delegate's vector for ord; ordinals are interpreted
// in the delegate's ord-space.
func (v *SortingFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	return v.delegate.VectorValue(ord)
}

func (v *SortingFloatVectorValues) Dimension() int {
	return v.delegate.Dimension()
}

func (v *SortingFloatVectorValues) Size() int {
	return v.iteratorSupplier.Size()
}

func (v *SortingFloatVectorValues) Iterator() DocIndexIterator {
	return v.iteratorSupplier.Get()
}

// OrdToDoc returns ord: KnnVectorValues.ordToDoc is the identity by default
// and SortingFloatVectorValues does not override it.
func (v *SortingFloatVectorValues) OrdToDoc(ord int) int { return ord }

// Prefetch is a no-op, matching KnnVectorValues.prefetch's empty default.
func (v *SortingFloatVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error { return nil }

// Copy is unsupported, mirroring SortingFloatVectorValues.copy, which throws
// UnsupportedOperationException.
func (v *SortingFloatVectorValues) Copy() (KnnVectorValues, error) {
	return nil, fmt.Errorf("index: SortingFloatVectorValues: Copy is not supported")
}

// CopyFloatVectorValues is unsupported for the same reason as Copy.
func (v *SortingFloatVectorValues) CopyFloatVectorValues() (FloatVectorValues, error) {
	return nil, fmt.Errorf("index: SortingFloatVectorValues: Copy is not supported")
}

func (v *SortingFloatVectorValues) GetEncoding() VectorEncoding {
	return VectorEncodingFloat32
}

func (v *SortingFloatVectorValues) GetVectorByteLength() int {
	return v.Dimension() * VectorEncodingByteSize(v.GetEncoding())
}

// GetAcceptOrds carries the KnnVectorValues.getAcceptOrds default, which
// SortingFloatVectorValues does not override.
func (v *SortingFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(v, acceptDocs)
}

// Scorer is unsupported, matching FloatVectorValues.scorer's default.
func (v *SortingFloatVectorValues) Scorer(target []float32) (util.VectorScorer, error) {
	return nil, fmt.Errorf("index: SortingFloatVectorValues: Scorer is not supported")
}

// Rescorer delegates to Scorer, matching FloatVectorValues.rescorer's default.
func (v *SortingFloatVectorValues) Rescorer(target []float32) (util.VectorScorer, error) {
	return v.Scorer(target)
}

// SortingByteVectorValues wraps ByteVectorValues to provide a sorted view.
// Mirrors org.apache.lucene.index.SortingCodecReader.SortingByteVectorValues.
type SortingByteVectorValues struct {
	delegate         ByteVectorValues
	iteratorSupplier *SortingIteratorSupplier
}

func NewSortingByteVectorValues(delegate ByteVectorValues, sortMap SorterDocMap) (*SortingByteVectorValues, error) {
	if delegate == nil {
		return nil, fmt.Errorf("index: SortingByteVectorValues: delegate is nil")
	}
	supplier, err := NewSortingIteratorSupplier(delegate, sortMap)
	if err != nil {
		return nil, err
	}
	return &SortingByteVectorValues{
		delegate:         delegate,
		iteratorSupplier: supplier,
	}, nil
}

// VectorValue returns the delegate's vector for ord; ordinals are interpreted
// in the delegate's ord-space.
func (v *SortingByteVectorValues) VectorValue(ord int) ([]byte, error) {
	return v.delegate.VectorValue(ord)
}

func (v *SortingByteVectorValues) Iterator() DocIndexIterator {
	return v.iteratorSupplier.Get()
}

func (v *SortingByteVectorValues) Dimension() int {
	return v.delegate.Dimension()
}

func (v *SortingByteVectorValues) Size() int {
	return v.iteratorSupplier.Size()
}

// OrdToDoc returns ord: KnnVectorValues.ordToDoc is the identity by default
// and SortingByteVectorValues does not override it.
func (v *SortingByteVectorValues) OrdToDoc(ord int) int { return ord }

// Prefetch is a no-op, matching KnnVectorValues.prefetch's empty default.
func (v *SortingByteVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error { return nil }

// Copy is unsupported, mirroring SortingByteVectorValues.copy, which throws
// UnsupportedOperationException.
func (v *SortingByteVectorValues) Copy() (KnnVectorValues, error) {
	return nil, fmt.Errorf("index: SortingByteVectorValues: Copy is not supported")
}

// CopyByteVectorValues is unsupported for the same reason as Copy.
func (v *SortingByteVectorValues) CopyByteVectorValues() (ByteVectorValues, error) {
	return nil, fmt.Errorf("index: SortingByteVectorValues: Copy is not supported")
}

func (v *SortingByteVectorValues) GetEncoding() VectorEncoding {
	return VectorEncodingByte
}

func (v *SortingByteVectorValues) GetVectorByteLength() int {
	return v.Dimension() * VectorEncodingByteSize(v.GetEncoding())
}

// GetAcceptOrds carries the KnnVectorValues.getAcceptOrds default, which
// SortingByteVectorValues does not override.
func (v *SortingByteVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(v, acceptDocs)
}

// Scorer is unsupported, matching ByteVectorValues.scorer's default.
func (v *SortingByteVectorValues) Scorer(target []byte) (util.VectorScorer, error) {
	return nil, fmt.Errorf("index: SortingByteVectorValues: Scorer is not supported")
}

// Rescorer delegates to Scorer, matching ByteVectorValues.rescorer's default.
func (v *SortingByteVectorValues) Rescorer(target []byte) (util.VectorScorer, error) {
	return v.Scorer(target)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (it *SortingValuesIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// VisitByDocIDSetIterator renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator), which v does not
// override.
func (v *sortingIntersectVisitor) VisitByDocIDSetIterator(iterator spi.DocIdSetIterator) error {
	return spi.DefaultVisitByDocIDSetIterator(v, iterator)
}

// VisitByIntsRef renders the default body of
// PointValues.IntersectVisitor.visit(IntsRef), which v does not override.
func (v *sortingIntersectVisitor) VisitByIntsRef(ref *util.IntsRef) error {
	return spi.DefaultVisitByIntsRef(v, ref)
}

// VisitByDocIDSetIteratorAndPackedValue renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator, byte[]), which v
// does not override.
func (v *sortingIntersectVisitor) VisitByDocIDSetIteratorAndPackedValue(iterator spi.DocIdSetIterator, packedValue []byte) error {
	return spi.DefaultVisitByDocIDSetIteratorAndPackedValue(v, iterator, packedValue)
}
