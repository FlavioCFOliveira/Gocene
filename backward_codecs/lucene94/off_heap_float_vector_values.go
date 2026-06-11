// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

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
//     with a [offHeap94FloatVariant] strategy interface — same approach as
//     the lucene99 port.
//   - There is no readFloats method on store.IndexInput; VectorValue reads
//     raw bytes and composes float32 values via math.Float32frombits.
//   - For BYTE encoding, VectorValue reads raw bytes and casts each byte to
//     float32. The Java reference calls readFloats unconditionally which
//     would over-read for BYTE vectors; this deviation is documented in
//     the port.
//   - Java's randomAccessSlice is replaced by reading the addresses data
//     into a byte slice and wrapping it in store.ByteArrayRandomAccessInput.

// offHeap94FloatOrdToDocReader mirrors DirectMonotonicReader.
type offHeap94FloatOrdToDocReader interface {
	Get(index int64) (int64, error)
}

// noMoreDocs94 mirrors DocIdSetIterator.NO_MORE_DOCS.
const noMoreDocs94 = 2147483647

// OffHeapFloatVectorValues reads float32 vector values from the index.
// Lucene 9.4 supports both FLOAT32 and BYTE encoding; for BYTE encoded
// fields the raw bytes are cast to float32.
//
// Port of
// org.apache.lucene.backward_codecs.lucene94.OffHeapFloatVectorValues
// (Lucene 10.4.0).
//
// Instances are not safe for concurrent use; use Copy to obtain an independent
// iterator.
type OffHeapFloatVectorValues struct {
	dimension          int
	size               int
	byteSize           int
	encoding           index.VectorEncoding
	slice              store.IndexInput
	floatValue         []float32
	curOrd             int
	similarityFunction index.VectorSimilarityFunction

	variant offHeap94FloatVariant
}

// offHeap94FloatVariant captures layout-specific behaviour.
type offHeap94FloatVariant interface {
	iterator(parent *OffHeapFloatVectorValues) index.DocIndexIterator
	ordToDoc(parent *OffHeapFloatVectorValues, ord int) int
	getAcceptOrds(parent *OffHeapFloatVectorValues, acceptDocs util.Bits) util.Bits
	copy(parent *OffHeapFloatVectorValues) (*OffHeapFloatVectorValues, error)
	scorer(parent *OffHeapFloatVectorValues, target []float32) (codec94VectorScorerView, error)
}

func newOffHeap94FloatVectorValues(
	dimension, size int,
	encoding index.VectorEncoding,
	similarityFunction index.VectorSimilarityFunction,
	slice store.IndexInput,
	variant offHeap94FloatVariant,
) *OffHeapFloatVectorValues {
	byteSize := dimension * 4 // Float.BYTES
	if encoding == index.VectorEncodingByte {
		byteSize = dimension
	}
	return &OffHeapFloatVectorValues{
		dimension:          dimension,
		size:               size,
		byteSize:           byteSize,
		encoding:           encoding,
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

// GetEncoding returns the vector encoding.
func (v *OffHeapFloatVectorValues) GetEncoding() index.VectorEncoding { return v.encoding }

// VectorValue reads and returns the float vector for the given ordinal.
//
// Port of OffHeapFloatVectorValues.vectorValue(int).
func (v *OffHeapFloatVectorValues) VectorValue(targetOrd int) ([]float32, error) {
	if v.curOrd == targetOrd {
		return v.floatValue, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, fmt.Errorf("lucene94 off-heap float: seek to ord %d: %w", targetOrd, err)
	}
	switch v.encoding {
	case index.VectorEncodingFloat32:
		buf := make([]byte, v.byteSize)
		if err := v.slice.ReadBytes(buf); err != nil {
			return nil, fmt.Errorf("lucene94 off-heap float: read float bytes: %w", err)
		}
		for i := range v.floatValue {
			bits := uint32(buf[i*4]) | uint32(buf[i*4+1])<<8 |
				uint32(buf[i*4+2])<<16 | uint32(buf[i*4+3])<<24
			v.floatValue[i] = math.Float32frombits(bits)
		}
	case index.VectorEncodingByte:
		buf := make([]byte, v.byteSize)
		if err := v.slice.ReadBytes(buf); err != nil {
			return nil, fmt.Errorf("lucene94 off-heap float: read byte data: %w", err)
		}
		for i, b := range buf {
			v.floatValue[i] = float32(b)
		}
	default:
		return nil, fmt.Errorf("lucene94 off-heap float: unsupported encoding %v", v.encoding)
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
func (v *OffHeapFloatVectorValues) Scorer(target []float32) (codec94VectorScorerView, error) {
	return v.variant.scorer(v, target)
}

// LoadFloat constructs the appropriate variant (dense, sparse or empty)
// based on the field entry's docsWithField sentinel and encoding.
//
// Port of OffHeapFloatVectorValues.load (Lucene 10.4.0).
func LoadFloat(
	fieldEntry *lucene94FieldEntry,
	vectorData store.IndexInput,
) (*OffHeapFloatVectorValues, error) {
	if fieldEntry == nil {
		return nil, errors.New("lucene94: LoadFloat: fieldEntry is nil")
	}
	if fieldEntry.docsWithFieldOffset == -2 {
		return newEmptyOffHeap94Float(fieldEntry.dimension), nil
	}
	bytesSlice, err := vectorData.Slice(
		"float-vector-data",
		fieldEntry.vectorDataOffset, fieldEntry.vectorDataLength)
	if err != nil {
		return nil, fmt.Errorf("lucene94: LoadFloat: slice vector data: %w", err)
	}
	if fieldEntry.docsWithFieldOffset == -1 {
		return newOffHeap94FloatVectorValues(
			fieldEntry.dimension, fieldEntry.size,
			fieldEntry.vectorEncoding, fieldEntry.similarityFunction,
			bytesSlice, denseOffHeap94FloatVariant{},
		), nil
	}
	return newSparseOffHeap94Float(
		fieldEntry, vectorData, bytesSlice,
	)
}

// ---------------------------------------------------------------------------
// Dense variant
// ---------------------------------------------------------------------------

type denseOffHeap94FloatVariant struct{}

func (denseOffHeap94FloatVariant) iterator(parent *OffHeapFloatVectorValues) index.DocIndexIterator {
	return newDenseDocIter94(parent.size)
}

func (denseOffHeap94FloatVariant) ordToDoc(_ *OffHeapFloatVectorValues, ord int) int {
	return ord
}

func (denseOffHeap94FloatVariant) getAcceptOrds(_ *OffHeapFloatVectorValues, acceptDocs util.Bits) util.Bits {
	return acceptDocs
}

func (denseOffHeap94FloatVariant) copy(parent *OffHeapFloatVectorValues) (*OffHeapFloatVectorValues, error) {
	cloned := parent.slice.Clone()
	return newOffHeap94FloatVectorValues(
		parent.dimension, parent.size,
		parent.encoding, parent.similarityFunction, cloned,
		denseOffHeap94FloatVariant{},
	), nil
}

func (denseOffHeap94FloatVariant) scorer(parent *OffHeapFloatVectorValues, target []float32) (codec94VectorScorerView, error) {
	cp, err := denseOffHeap94FloatVariant{}.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cp.Iterator()
	return &float94ScorerView{
		it:     it,
		fvv:    cp,
		target: target,
	}, nil
}

// ---------------------------------------------------------------------------
// Sparse variant
// ---------------------------------------------------------------------------

type sparseOffHeap94FloatVariant struct {
	ordToDocReader offHeap94FloatOrdToDocReader
	disi           *codecs_lucene90.IndexedDISI
	dataIn         store.IndexInput
}

func newSparseOffHeap94Float(
	fieldEntry *lucene94FieldEntry,
	dataIn store.IndexInput,
	slice store.IndexInput,
) (*OffHeapFloatVectorValues, error) {
	addrData, err := readBytesFromInput94(dataIn, fieldEntry.addressesOffset, fieldEntry.addressesLength)
	if err != nil {
		return nil, fmt.Errorf("lucene94: sparse off-heap float: read addresses: %w", err)
	}
	addrRA := store.NewByteArrayRandomAccessInput(addrData)
	otr, err := packed.NewDirectMonotonicReader(fieldEntry.meta, addrRA)
	if err != nil {
		return nil, fmt.Errorf("lucene94: sparse off-heap float: create DirectMonotonicReader: %w", err)
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
		return nil, fmt.Errorf("lucene94: sparse off-heap float: create IndexedDISI: %w", err)
	}
	v := &sparseOffHeap94FloatVariant{
		ordToDocReader: otr,
		disi:           disi,
		dataIn:         dataIn,
	}
	return newOffHeap94FloatVectorValues(
		fieldEntry.dimension, fieldEntry.size,
		fieldEntry.vectorEncoding, fieldEntry.similarityFunction,
		slice, v,
	), nil
}

func (s *sparseOffHeap94FloatVariant) iterator(_ *OffHeapFloatVectorValues) index.DocIndexIterator {
	return &indexedDISIIter94{disi: s.disi}
}

func (s *sparseOffHeap94FloatVariant) ordToDoc(_ *OffHeapFloatVectorValues, ord int) int {
	doc, err := s.ordToDocReader.Get(int64(ord))
	if err != nil {
		return 0
	}
	return int(doc)
}

func (s *sparseOffHeap94FloatVariant) getAcceptOrds(parent *OffHeapFloatVectorValues, acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &ordinalBits94{accept: acceptDocs, v: parent}
}

func (s *sparseOffHeap94FloatVariant) copy(parent *OffHeapFloatVectorValues) (*OffHeapFloatVectorValues, error) {
	cloned := parent.slice.Clone()
	return newOffHeap94FloatVectorValues(
		parent.dimension, parent.size,
		parent.encoding, parent.similarityFunction, cloned,
		&sparseOffHeap94FloatVariant{
			ordToDocReader: s.ordToDocReader,
			disi:           s.disi,
			dataIn:         s.dataIn,
		},
	), nil
}

func (s *sparseOffHeap94FloatVariant) scorer(parent *OffHeapFloatVectorValues, target []float32) (codec94VectorScorerView, error) {
	cp, err := s.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cp.Iterator()
	return &float94ScorerView{
		it:     it,
		fvv:    cp,
		target: target,
	}, nil
}

// ---------------------------------------------------------------------------
// Empty variant
// ---------------------------------------------------------------------------

type emptyOffHeap94FloatVariant struct{}

func newEmptyOffHeap94Float(dimension int) *OffHeapFloatVectorValues {
	return newOffHeap94FloatVectorValues(
		dimension, 0,
		index.VectorEncodingFloat32, index.VectorSimilarityFunctionCosine, nil,
		emptyOffHeap94FloatVariant{},
	)
}

func (emptyOffHeap94FloatVariant) iterator(_ *OffHeapFloatVectorValues) index.DocIndexIterator {
	return newDenseDocIter94(0)
}

func (emptyOffHeap94FloatVariant) ordToDoc(_ *OffHeapFloatVectorValues, _ int) int {
	panic("lucene94: emptyOffHeap94FloatVariant.ordToDoc not supported")
}

func (emptyOffHeap94FloatVariant) getAcceptOrds(_ *OffHeapFloatVectorValues, _ util.Bits) util.Bits {
	return nil
}

func (emptyOffHeap94FloatVariant) copy(_ *OffHeapFloatVectorValues) (*OffHeapFloatVectorValues, error) {
	return nil, errors.New("lucene94: emptyOffHeap94FloatVariant.copy not supported")
}

func (emptyOffHeap94FloatVariant) scorer(_ *OffHeapFloatVectorValues, _ []float32) (codec94VectorScorerView, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helper utilities
// ---------------------------------------------------------------------------

// readBytesFromInput94 reads exactly length bytes at [offset, offset+length)
// from the given IndexInput.
func readBytesFromInput94(in store.IndexInput, offset, length int64) ([]byte, error) {
	if err := in.SetPosition(offset); err != nil {
		return nil, err
	}
	return in.ReadBytesN(int(length))
}

// ---------------------------------------------------------------------------
// Dense DocIndexIterator
// ---------------------------------------------------------------------------

type denseDocIter94 struct {
	doc  int
	size int
}

func newDenseDocIter94(size int) *denseDocIter94 {
	return &denseDocIter94{doc: -1, size: size}
}

func (d *denseDocIter94) DocID() int { return d.doc }

func (d *denseDocIter94) NextDoc() (int, error) { return d.Advance(d.doc + 1) }

func (d *denseDocIter94) Advance(target int) (int, error) {
	if target >= d.size {
		d.doc = noMoreDocs94
		return d.doc, nil
	}
	d.doc = target
	return d.doc, nil
}

func (d *denseDocIter94) Cost() int64 { return int64(d.size) }

func (d *denseDocIter94) Index() int { return d.doc }

// ---------------------------------------------------------------------------
// IndexedDISI DocIndexIterator wrapper
// ---------------------------------------------------------------------------

type indexedDISIIter94 struct {
	disi *codecs_lucene90.IndexedDISI
}

func (i *indexedDISIIter94) DocID() int { return i.disi.DocID() }

func (i *indexedDISIIter94) NextDoc() (int, error) { return i.disi.NextDoc() }

func (i *indexedDISIIter94) Advance(target int) (int, error) { return i.disi.Advance(target) }

func (i *indexedDISIIter94) Cost() int64 { return i.disi.Cost() }

func (i *indexedDISIIter94) Index() int { return i.disi.Index() }

// ---------------------------------------------------------------------------
// Ordinal-keyed Bits (sparse variant's getAcceptOrds)
// ---------------------------------------------------------------------------

type ordinalBits94 struct {
	accept util.Bits
	v      *OffHeapFloatVectorValues
}

func (b *ordinalBits94) Get(index int) bool {
	return b.accept.Get(b.v.OrdToDoc(index))
}

func (b *ordinalBits94) Length() int { return b.v.size }

// ---------------------------------------------------------------------------
// VectorScorerView
// ---------------------------------------------------------------------------

// codec94VectorScorerView mirrors codecs.VectorScorerView at the
// backward_codecs/lucene94 boundary.
type codec94VectorScorerView interface {
	Score() (float32, error)
	Iterator() codec94DocIDSetIteratorView
	Bulk() codec94VectorScorerBulkView
}

// codec94VectorScorerBulkView mirrors codecs.VectorScorerBulkView.
type codec94VectorScorerBulkView interface{}

// codec94DocIDSetIteratorView mirrors codecs.DocIDSetIteratorView.
type codec94DocIDSetIteratorView interface {
	DocID() int
	NextDoc() (int, error)
	Advance(target int) (int, error)
	Cost() int64
	DocIDRunEnd() int
}

type float94ScorerView struct {
	it     index.DocIndexIterator
	fvv    *OffHeapFloatVectorValues
	target []float32
}

func (s *float94ScorerView) Score() (float32, error) {
	vec, err := s.fvv.VectorValue(s.it.Index())
	if err != nil {
		return 0, err
	}
	return similarityCompare94(s.fvv.similarityFunction, vec, s.target), nil
}

func (s *float94ScorerView) Iterator() codec94DocIDSetIteratorView {
	return &docIndexIterToView94{it: s.it}
}

func (s *float94ScorerView) Bulk() codec94VectorScorerBulkView { return nil }

// docIndexIterToView94 adapts index.DocIndexIterator to codec94DocIDSetIteratorView.
type docIndexIterToView94 struct{ it index.DocIndexIterator }

func (d *docIndexIterToView94) DocID() int                 { return d.it.DocID() }
func (d *docIndexIterToView94) NextDoc() (int, error)      { return d.it.NextDoc() }
func (d *docIndexIterToView94) Advance(t int) (int, error) { return d.it.Advance(t) }
func (d *docIndexIterToView94) Cost() int64                { return d.it.Cost() }
func (d *docIndexIterToView94) DocIDRunEnd() int           { return noMoreDocs94 }

// similarityCompare94 mirrors VectorSimilarityFunction.compare.
func similarityCompare94(sim index.VectorSimilarityFunction, v1, v2 []float32) float32 {
	switch sim {
	case index.VectorSimilarityFunctionEuclidean:
		return euclideanSimilarity94(v1, v2)
	case index.VectorSimilarityFunctionDotProduct:
		return dotProductSimilarity94(v1, v2)
	case index.VectorSimilarityFunctionCosine:
		return cosineSimilarity94(v1, v2)
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		return maxInnerProductSimilarity94(v1, v2)
	default:
		return 0
	}
}

func euclideanSimilarity94(v1, v2 []float32) float32 {
	var sum float32
	for i := range v1 {
		diff := v1[i] - v2[i]
		sum += diff * diff
	}
	return 1.0 / (1.0 + sum)
}

func dotProductSimilarity94(v1, v2 []float32) float32 {
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

func cosineSimilarity94(v1, v2 []float32) float32 {
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

func maxInnerProductSimilarity94(v1, v2 []float32) float32 {
	var dot float32
	for i := range v1 {
		dot += v1[i] * v2[i]
	}
	if dot < 0 {
		return 1.0 / (1.0 - dot)
	}
	return dot + 1.0
}
