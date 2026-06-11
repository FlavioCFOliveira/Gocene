// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene92

import (
	"errors"
	"fmt"
	"math"

	codecs_lucene90 "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Deviations from the Java reference (Lucene 10.4.0):
//
//   - Java's abstract class hierarchy (OffHeapFloatVectorValues,
//     DenseOffHeapVectorValues, SparseOffHeapVectorValues,
//     EmptyOffHeapVectorValues) is replaced by a single concrete struct
//     with a [offHeap92Variant] strategy interface — same approach as the
//     lucene99 port in backward_codecs/lucene99/off_heap_quantized_float_vector_values.go.
//   - There is no readFloats method on store.IndexInput; VectorValue reads
//     raw bytes and composes float32 values via math.Float32frombits.
//   - Java's randomAccessSlice is replaced by reading the addresses data
//     into a byte slice and wrapping it in store.ByteArrayRandomAccessInput.

// offHeap92OrdToDocReader mirrors DirectMonotonicReader — the only method
// consumed by the sparse variant.
type offHeap92OrdToDocReader interface {
	Get(index int64) (int64, error)
}

// noMoreDocs92 mirrors DocIdSetIterator.NO_MORE_DOCS.
const noMoreDocs92 = 2147483647

// OffHeapFloatVectorValues reads float32 vector values from the index.
//
// Port of
// org.apache.lucene.backward_codecs.lucene92.OffHeapFloatVectorValues
// (Lucene 10.4.0).
//
// Lucene 9.2 stores only FLOAT32 vectors; byteSize is always
// dimension * Float.BYTES.
//
// Instances are not safe for concurrent use; use Copy to obtain an independent
// iterator.
type OffHeapFloatVectorValues struct {
	dimension          int
	size               int
	byteSize           int
	slice              store.IndexInput
	floatValue         []float32
	curOrd             int
	similarityFunction index.VectorSimilarityFunction

	variant offHeap92Variant
}

// offHeap92Variant captures layout-specific behaviour.
type offHeap92Variant interface {
	iterator(parent *OffHeapFloatVectorValues) index.DocIndexIterator
	ordToDoc(parent *OffHeapFloatVectorValues, ord int) int
	getAcceptOrds(parent *OffHeapFloatVectorValues, acceptDocs util.Bits) util.Bits
	copy(parent *OffHeapFloatVectorValues) (*OffHeapFloatVectorValues, error)
	scorer(parent *OffHeapFloatVectorValues, target []float32) (codecVectorScorerView, error)
}

func newOffHeapFloatVectorValues(
	dimension, size int,
	similarityFunction index.VectorSimilarityFunction,
	slice store.IndexInput,
	variant offHeap92Variant,
) *OffHeapFloatVectorValues {
	return &OffHeapFloatVectorValues{
		dimension:          dimension,
		size:               size,
		byteSize:           dimension * 4, // Float.BYTES
		slice:              slice,
		floatValue:         make([]float32, dimension),
		curOrd:             -1,
		similarityFunction: similarityFunction,
		variant:            variant,
	}
}

// Dimension returns the vector dimension.
func (v *OffHeapFloatVectorValues) Dimension() int { return v.dimension }

// Size returns the number of vectors.
func (v *OffHeapFloatVectorValues) Size() int { return v.size }

// GetEncoding returns FLOAT32: lucene92 stores only float32 vectors.
func (v *OffHeapFloatVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// VectorValue reads and returns the float vector for the given ordinal.
//
// Port of OffHeapFloatVectorValues.vectorValue(int).
func (v *OffHeapFloatVectorValues) VectorValue(targetOrd int) ([]float32, error) {
	if v.curOrd == targetOrd {
		return v.floatValue, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, fmt.Errorf("lucene92 off-heap float: seek to ord %d: %w", targetOrd, err)
	}
	// Read dimension * 4 bytes and decode as little-endian float32 values.
	buf := make([]byte, v.byteSize)
	if err := v.slice.ReadBytes(buf); err != nil {
		return nil, fmt.Errorf("lucene92 off-heap float: read bytes: %w", err)
	}
	for i := range v.floatValue {
		bits := uint32(buf[i*4]) | uint32(buf[i*4+1])<<8 | uint32(buf[i*4+2])<<16 | uint32(buf[i*4+3])<<24
		v.floatValue[i] = math.Float32frombits(bits)
	}
	v.curOrd = targetOrd
	return v.floatValue, nil
}

// Iterator returns a DocIndexIterator over this vector set.
func (v *OffHeapFloatVectorValues) Iterator() index.DocIndexIterator {
	return v.variant.iterator(v)
}

// OrdToDoc maps a vector ordinal to its document ID.
func (v *OffHeapFloatVectorValues) OrdToDoc(ord int) int {
	return v.variant.ordToDoc(v, ord)
}

// GetAcceptOrds wraps acceptDocs for ordinal-based access.
func (v *OffHeapFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return v.variant.getAcceptOrds(v, acceptDocs)
}

// Copy returns an independent iterator over the same data.
func (v *OffHeapFloatVectorValues) Copy() (*OffHeapFloatVectorValues, error) {
	return v.variant.copy(v)
}

// Scorer returns a VectorScorerView for the given target, or nil for the
// empty variant.
func (v *OffHeapFloatVectorValues) Scorer(target []float32) (codecVectorScorerView, error) {
	return v.variant.scorer(v, target)
}

// LoadFloat constructs the appropriate variant (dense, sparse or empty)
// based on the field entry's docsWithField sentinel.
//
// Port of OffHeapFloatVectorValues.load (Lucene 10.4.0).
func LoadFloat(
	fieldEntry *lucene92FieldEntry,
	vectorData store.IndexInput,
) (*OffHeapFloatVectorValues, error) {
	if fieldEntry == nil {
		return nil, errors.New("lucene92: LoadFloat: fieldEntry is nil")
	}
	if fieldEntry.docsWithFieldOffset == -2 {
		return newEmptyOffHeap92(fieldEntry.dimension), nil
	}
	bytesSlice, err := vectorData.Slice(
		"float-vector-data",
		fieldEntry.vectorDataOffset, fieldEntry.vectorDataLength)
	if err != nil {
		return nil, fmt.Errorf("lucene92: LoadFloat: slice vector data: %w", err)
	}
	if fieldEntry.docsWithFieldOffset == -1 {
		return newOffHeapFloatVectorValues(
			fieldEntry.dimension, fieldEntry.size,
			fieldEntry.similarityFunction, bytesSlice,
			denseOffHeap92Variant{},
		), nil
	}
	return newSparseOffHeap92(
		fieldEntry, vectorData, bytesSlice,
	)
}

// ---------------------------------------------------------------------------
// Dense variant
// ---------------------------------------------------------------------------

type denseOffHeap92Variant struct{}

func (denseOffHeap92Variant) iterator(parent *OffHeapFloatVectorValues) index.DocIndexIterator {
	return newDenseDocIter92(parent.size)
}

func (denseOffHeap92Variant) ordToDoc(_ *OffHeapFloatVectorValues, ord int) int {
	return ord
}

func (denseOffHeap92Variant) getAcceptOrds(_ *OffHeapFloatVectorValues, acceptDocs util.Bits) util.Bits {
	return acceptDocs
}

func (denseOffHeap92Variant) copy(parent *OffHeapFloatVectorValues) (*OffHeapFloatVectorValues, error) {
	cloned := parent.slice.Clone()
	return newOffHeapFloatVectorValues(
		parent.dimension, parent.size,
		parent.similarityFunction, cloned,
		denseOffHeap92Variant{},
	), nil
}

func (denseOffHeap92Variant) scorer(parent *OffHeapFloatVectorValues, target []float32) (codecVectorScorerView, error) {
	cp, err := denseOffHeap92Variant{}.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cp.Iterator()
	return &floatScorerView92{
		it:   it,
		fvv:  cp,
		target: target,
	}, nil
}

// ---------------------------------------------------------------------------
// Sparse variant
// ---------------------------------------------------------------------------

type sparseOffHeap92Variant struct {
	ordToDocReader offHeap92OrdToDocReader
	disi           *codecs_lucene90.IndexedDISI
	dataIn         store.IndexInput
}

func newSparseOffHeap92(
	fieldEntry *lucene92FieldEntry,
	dataIn store.IndexInput,
	slice store.IndexInput,
) (*OffHeapFloatVectorValues, error) {
	// Read addresses data into bytes and wrap as RandomAccessInput.
	addrData, err := readBytesFromInput(dataIn, fieldEntry.addressesOffset, fieldEntry.addressesLength)
	if err != nil {
		return nil, fmt.Errorf("lucene92: sparse off-heap: read addresses: %w", err)
	}
	addrRA := store.NewByteArrayRandomAccessInput(addrData)
	otr, err := packed.NewDirectMonotonicReader(fieldEntry.meta, addrRA)
	if err != nil {
		return nil, fmt.Errorf("lucene92: sparse off-heap: create DirectMonotonicReader: %w", err)
	}
	disi, err := codecs_lucene90.NewIndexedDISI(
		dataIn,
		fieldEntry.docsWithFieldOffset,
		fieldEntry.docsWithFieldLength,
		int(fieldEntry.jumpTableEntryCount),
		byte(fieldEntry.denseRankPower),
		int64(fieldEntry.size),
	)
	if err != nil {
		return nil, fmt.Errorf("lucene92: sparse off-heap: create IndexedDISI: %w", err)
	}
	v := &sparseOffHeap92Variant{
		ordToDocReader: otr,
		disi:           disi,
		dataIn:         dataIn,
	}
	return newOffHeapFloatVectorValues(
		fieldEntry.dimension, fieldEntry.size,
		fieldEntry.similarityFunction, slice, v,
	), nil
}

func (s *sparseOffHeap92Variant) iterator(_ *OffHeapFloatVectorValues) index.DocIndexIterator {
	return &indexedDISIIter92{disi: s.disi}
}

func (s *sparseOffHeap92Variant) ordToDoc(_ *OffHeapFloatVectorValues, ord int) int {
	doc, err := s.ordToDocReader.Get(int64(ord))
	if err != nil {
		return 0
	}
	return int(doc)
}

func (s *sparseOffHeap92Variant) getAcceptOrds(parent *OffHeapFloatVectorValues, acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &ordinalBits92{accept: acceptDocs, v: parent}
}

func (s *sparseOffHeap92Variant) copy(parent *OffHeapFloatVectorValues) (*OffHeapFloatVectorValues, error) {
	cloned := parent.slice.Clone()
	// The dataIn is already the parent field's vector data input; the
	// sparse constructor re-reads addresses and creates a new DirectMonotonic
	// reader from it. We need the fieldEntry, which is referenced only
	// through the variant's persisted state — but we lost the fieldEntry.
	// Instead, reuse the dataIn and disi/ordToDoc from the parent variant.
	// For correctness we need a clone that shares the same underlying
	// file references.
	pe := s
	return newOffHeapFloatVectorValues(
		parent.dimension, parent.size,
		parent.similarityFunction, cloned,
		&sparseOffHeap92Variant{
			ordToDocReader: pe.ordToDocReader,
			disi:           pe.disi,
			dataIn:         pe.dataIn,
		},
	), nil
}

func (s *sparseOffHeap92Variant) scorer(parent *OffHeapFloatVectorValues, target []float32) (codecVectorScorerView, error) {
	cp, err := s.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cp.Iterator()
	return &floatScorerView92{
		it:   it,
		fvv:  cp,
		target: target,
	}, nil
}

// ---------------------------------------------------------------------------
// Empty variant
// ---------------------------------------------------------------------------

type emptyOffHeap92Variant struct{}

func newEmptyOffHeap92(dimension int) *OffHeapFloatVectorValues {
	return newOffHeapFloatVectorValues(
		dimension, 0,
		index.VectorSimilarityFunctionCosine, nil,
		emptyOffHeap92Variant{},
	)
}

func (emptyOffHeap92Variant) iterator(_ *OffHeapFloatVectorValues) index.DocIndexIterator {
	return newDenseDocIter92(0)
}

func (emptyOffHeap92Variant) ordToDoc(_ *OffHeapFloatVectorValues, _ int) int {
	panic("lucene92: emptyOffHeap92Variant.ordToDoc not supported")
}

func (emptyOffHeap92Variant) getAcceptOrds(_ *OffHeapFloatVectorValues, _ util.Bits) util.Bits {
	return nil
}

func (emptyOffHeap92Variant) copy(_ *OffHeapFloatVectorValues) (*OffHeapFloatVectorValues, error) {
	return nil, errors.New("lucene92: emptyOffHeap92Variant.copy not supported")
}

func (emptyOffHeap92Variant) scorer(_ *OffHeapFloatVectorValues, _ []float32) (codecVectorScorerView, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helper utilities
// ---------------------------------------------------------------------------

// readBytesFromInput reads exactly length bytes at [offset, offset+length)
// from the given IndexInput.
func readBytesFromInput(in store.IndexInput, offset, length int64) ([]byte, error) {
	if err := in.SetPosition(offset); err != nil {
		return nil, err
	}
	return in.ReadBytesN(int(length))
}

// ---------------------------------------------------------------------------
// Dense DocIndexIterator
// ---------------------------------------------------------------------------

type denseDocIter92 struct {
	doc  int
	size int
}

func newDenseDocIter92(size int) *denseDocIter92 {
	return &denseDocIter92{doc: -1, size: size}
}

func (d *denseDocIter92) DocID() int { return d.doc }

func (d *denseDocIter92) NextDoc() (int, error) { return d.Advance(d.doc + 1) }

func (d *denseDocIter92) Advance(target int) (int, error) {
	if target >= d.size {
		d.doc = noMoreDocs92
		return d.doc, nil
	}
	d.doc = target
	return d.doc, nil
}

func (d *denseDocIter92) Cost() int64 { return int64(d.size) }

func (d *denseDocIter92) Index() int { return d.doc }

// ---------------------------------------------------------------------------
// IndexedDISI DocIndexIterator wrapper
// ---------------------------------------------------------------------------

type indexedDISIIter92 struct {
	disi *codecs_lucene90.IndexedDISI
}

func (i *indexedDISIIter92) DocID() int { return i.disi.DocID() }

func (i *indexedDISIIter92) NextDoc() (int, error) { return i.disi.NextDoc() }

func (i *indexedDISIIter92) Advance(target int) (int, error) { return i.disi.Advance(target) }

func (i *indexedDISIIter92) Cost() int64 { return i.disi.Cost() }

func (i *indexedDISIIter92) Index() int { return i.disi.Index() }

// ---------------------------------------------------------------------------
// Ordinal-keyed Bits (sparse variant's getAcceptOrds)
// ---------------------------------------------------------------------------

type ordinalBits92 struct {
	accept util.Bits
	v      *OffHeapFloatVectorValues
}

func (b *ordinalBits92) Get(index int) bool {
	return b.accept.Get(b.v.OrdToDoc(index))
}

func (b *ordinalBits92) Length() int { return b.v.size }

// ---------------------------------------------------------------------------
// VectorScorerView
// ---------------------------------------------------------------------------

// codecVectorScorerView mirrors codecs.VectorScorerView at the
// backward_codecs/lucene92 boundary to avoid import cycles.
type codecVectorScorerView interface {
	Score() (float32, error)
	Iterator() codecDocIDSetIteratorView
	Bulk() codecVectorScorerBulkView
}

// codecVectorScorerBulkView mirrors codecs.VectorScorerBulkView.
type codecVectorScorerBulkView interface{}

// codecDocIDSetIteratorView mirrors codecs.DocIDSetIteratorView.
type codecDocIDSetIteratorView interface {
	DocID() int
	NextDoc() (int, error)
	Advance(target int) (int, error)
	Cost() int64
	DocIDRunEnd() int
}

type floatScorerView92 struct {
	it     index.DocIndexIterator
	fvv    *OffHeapFloatVectorValues
	target []float32
}

func (s *floatScorerView92) Score() (float32, error) {
	vec, err := s.fvv.VectorValue(s.it.Index())
	if err != nil {
		return 0, err
	}
	sim := similarityCompare(s.fvv.similarityFunction, vec, s.target)
	return sim, nil
}

func (s *floatScorerView92) Iterator() codecDocIDSetIteratorView {
	return &docIndexIterToView92{it: s.it}
}

func (s *floatScorerView92) Bulk() codecVectorScorerBulkView { return nil }

// docIndexIterToView92 adapts index.DocIndexIterator to codecDocIDSetIteratorView.
type docIndexIterToView92 struct{ it index.DocIndexIterator }

func (d *docIndexIterToView92) DocID() int                 { return d.it.DocID() }
func (d *docIndexIterToView92) NextDoc() (int, error)      { return d.it.NextDoc() }
func (d *docIndexIterToView92) Advance(t int) (int, error) { return d.it.Advance(t) }
func (d *docIndexIterToView92) Cost() int64                { return d.it.Cost() }
func (d *docIndexIterToView92) DocIDRunEnd() int           { return noMoreDocs92 }

// similarityCompare mirrors
// org.apache.lucene.index.VectorSimilarityFunction.compare(float[], float[]).
func similarityCompare(sim index.VectorSimilarityFunction, v1, v2 []float32) float32 {
	switch sim {
	case index.VectorSimilarityFunctionEuclidean:
		return euclideanSimilarity(v1, v2)
	case index.VectorSimilarityFunctionDotProduct:
		return dotProductSimilarity(v1, v2)
	case index.VectorSimilarityFunctionCosine:
		return cosineSimilarity(v1, v2)
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		return maxInnerProductSimilarity(v1, v2)
	default:
		return 0
	}
}

func euclideanSimilarity(v1, v2 []float32) float32 {
	var sum float32
	for i := range v1 {
		diff := v1[i] - v2[i]
		sum += diff * diff
	}
	return 1.0 / (1.0 + sum)
}

func dotProductSimilarity(v1, v2 []float32) float32 {
	var dot float32
	for i := range v1 {
		dot += v1[i] * v2[i]
	}
	score := (dot + 1.0) / 2.0
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func cosineSimilarity(v1, v2 []float32) float32 {
	var dot, norm1, norm2 float32
	for i := range v1 {
		dot += v1[i] * v2[i]
		norm1 += v1[i] * v1[i]
		norm2 += v2[i] * v2[i]
	}
	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	return (dot/(float32(math.Sqrt(float64(norm1)))*float32(math.Sqrt(float64(norm2))))+1.0)/2.0
}

func maxInnerProductSimilarity(v1, v2 []float32) float32 {
	var dot float32
	for i := range v1 {
		dot += v1[i] * v2[i]
	}
	if dot < 0 {
		return 1.0 / (1.0 - dot)
	}
	return dot + 1.0
}
