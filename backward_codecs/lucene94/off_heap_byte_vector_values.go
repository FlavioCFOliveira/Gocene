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
//   - Java's abstract class hierarchy (OffHeapByteVectorValues,
//     DenseOffHeapVectorValues, SparseOffHeapVectorValues,
//     EmptyOffHeapVectorValues) is replaced by a single concrete struct
//     with a [offHeap94ByteVariant] strategy interface — same approach as
//     the lucene99 port.
//   - Java's randomAccessSlice is replaced by reading the addresses data
//     into a byte slice and wrapping it in store.ByteArrayRandomAccessInput.
//   - Java uses a ByteBuffer for zero-copy; Gocene reads directly into a
//     byte slice.

// offHeap94ByteOrdToDocReader mirrors DirectMonotonicReader.
type offHeap94ByteOrdToDocReader interface {
	Get(index int64) (int64, error)
}

// OffHeapByteVectorValues reads byte vector values from the index.
// Lucene 9.4 stores byte vectors as raw bytes (1 byte per dimension).
//
// Port of
// org.apache.lucene.backward_codecs.lucene94.OffHeapByteVectorValues
// (Lucene 10.4.0).
//
// Instances are not safe for concurrent use; use Copy to obtain an independent
// iterator.
type OffHeapByteVectorValues struct {
	dimension          int
	size               int
	byteSize           int
	slice              store.IndexInput
	binaryValue        []byte
	curOrd             int
	similarityFunction index.VectorSimilarityFunction

	variant offHeap94ByteVariant
}

// offHeap94ByteVariant captures layout-specific behaviour.
type offHeap94ByteVariant interface {
	iterator(parent *OffHeapByteVectorValues) index.DocIndexIterator
	ordToDoc(parent *OffHeapByteVectorValues, ord int) int
	getAcceptOrds(parent *OffHeapByteVectorValues, acceptDocs util.Bits) util.Bits
	copy(parent *OffHeapByteVectorValues) (*OffHeapByteVectorValues, error)
	scorer(parent *OffHeapByteVectorValues, target []byte) (codec94ByteVectorScorerView, error)
}

func newOffHeap94ByteVectorValues(
	dimension, size int,
	similarityFunction index.VectorSimilarityFunction,
	slice store.IndexInput,
	variant offHeap94ByteVariant,
) *OffHeapByteVectorValues {
	return &OffHeapByteVectorValues{
		dimension:          dimension,
		size:               size,
		byteSize:           dimension,
		slice:              slice,
		binaryValue:        make([]byte, dimension),
		curOrd:             -1,
		similarityFunction: similarityFunction,
		variant:            variant,
	}
}

// Dimension returns the vector dimension.
func (v *OffHeapByteVectorValues) Dimension() int { return v.dimension }

// Size returns the number of vectors.
func (v *OffHeapByteVectorValues) Size() int { return v.size }

// GetEncoding returns BYTE.
func (v *OffHeapByteVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingByte
}

// VectorValue reads and returns the byte vector for the given ordinal.
//
// Port of OffHeapByteVectorValues.vectorValue(int).
func (v *OffHeapByteVectorValues) VectorValue(targetOrd int) ([]byte, error) {
	if v.curOrd == targetOrd {
		return v.binaryValue, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, fmt.Errorf("lucene94 off-heap byte: seek to ord %d: %w", targetOrd, err)
	}
	if err := v.slice.ReadBytes(v.binaryValue); err != nil {
		return nil, fmt.Errorf("lucene94 off-heap byte: read bytes: %w", err)
	}
	v.curOrd = targetOrd
	return v.binaryValue, nil
}

// Iterator returns a DocIndexIterator over this vector set.
func (v *OffHeapByteVectorValues) Iterator() index.DocIndexIterator {
	return v.variant.iterator(v)
}

// OrdToDoc maps a vector ordinal to its document ID.
func (v *OffHeapByteVectorValues) OrdToDoc(ord int) int {
	return v.variant.ordToDoc(v, ord)
}

// GetAcceptOrds wraps acceptDocs for ordinal-based access.
func (v *OffHeapByteVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return v.variant.getAcceptOrds(v, acceptDocs)
}

// Copy returns an independent iterator over the same data.
func (v *OffHeapByteVectorValues) Copy() (*OffHeapByteVectorValues, error) {
	return v.variant.copy(v)
}

// Scorer returns a VectorScorerView for the given target, or nil for the
// empty variant.
func (v *OffHeapByteVectorValues) Scorer(target []byte) (codec94ByteVectorScorerView, error) {
	return v.variant.scorer(v, target)
}

// LoadByte constructs the appropriate variant (dense, sparse or empty)
// based on the field entry's docsWithField sentinel.
//
// Port of OffHeapByteVectorValues.load (Lucene 10.4.0).
func LoadByte(
	fieldEntry *lucene94FieldEntry,
	vectorData store.IndexInput,
) (*OffHeapByteVectorValues, error) {
	if fieldEntry == nil {
		return nil, errors.New("lucene94: LoadByte: fieldEntry is nil")
	}
	if fieldEntry.docsWithFieldOffset == -2 {
		return newEmptyOffHeap94Byte(fieldEntry.dimension), nil
	}
	bytesSlice, err := vectorData.Slice(
		"byte-vector-data",
		fieldEntry.vectorDataOffset, fieldEntry.vectorDataLength)
	if err != nil {
		return nil, fmt.Errorf("lucene94: LoadByte: slice vector data: %w", err)
	}
	if fieldEntry.docsWithFieldOffset == -1 {
		return newOffHeap94ByteVectorValues(
			fieldEntry.dimension, fieldEntry.size,
			fieldEntry.similarityFunction, bytesSlice,
			denseOffHeap94ByteVariant{},
		), nil
	}
	return newSparseOffHeap94Byte(
		fieldEntry, vectorData, bytesSlice,
	)
}

// ---------------------------------------------------------------------------
// Dense variant
// ---------------------------------------------------------------------------

type denseOffHeap94ByteVariant struct{}

func (denseOffHeap94ByteVariant) iterator(parent *OffHeapByteVectorValues) index.DocIndexIterator {
	return newDenseDocIter94(parent.size)
}

func (denseOffHeap94ByteVariant) ordToDoc(_ *OffHeapByteVectorValues, ord int) int {
	return ord
}

func (denseOffHeap94ByteVariant) getAcceptOrds(_ *OffHeapByteVectorValues, acceptDocs util.Bits) util.Bits {
	return acceptDocs
}

func (denseOffHeap94ByteVariant) copy(parent *OffHeapByteVectorValues) (*OffHeapByteVectorValues, error) {
	cloned := parent.slice.Clone()
	return newOffHeap94ByteVectorValues(
		parent.dimension, parent.size,
		parent.similarityFunction, cloned,
		denseOffHeap94ByteVariant{},
	), nil
}

func (denseOffHeap94ByteVariant) scorer(parent *OffHeapByteVectorValues, target []byte) (codec94ByteVectorScorerView, error) {
	cp, err := denseOffHeap94ByteVariant{}.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cp.Iterator()
	return &byte94ScorerView{
		it:     it,
		bvv:    cp,
		target: target,
	}, nil
}

// ---------------------------------------------------------------------------
// Sparse variant
// ---------------------------------------------------------------------------

type sparseOffHeap94ByteVariant struct {
	ordToDocReader offHeap94ByteOrdToDocReader
	disi           *codecs_lucene90.IndexedDISI
	dataIn         store.IndexInput
}

func newSparseOffHeap94Byte(
	fieldEntry *lucene94FieldEntry,
	dataIn store.IndexInput,
	slice store.IndexInput,
) (*OffHeapByteVectorValues, error) {
	addrData, err := readBytesFromInput94(dataIn, fieldEntry.addressesOffset, fieldEntry.addressesLength)
	if err != nil {
		return nil, fmt.Errorf("lucene94: sparse off-heap byte: read addresses: %w", err)
	}
	addrRA := store.NewByteArrayRandomAccessInput(addrData)
	otr, err := packed.NewDirectMonotonicReader(fieldEntry.meta, addrRA)
	if err != nil {
		return nil, fmt.Errorf("lucene94: sparse off-heap byte: create DirectMonotonicReader: %w", err)
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
		return nil, fmt.Errorf("lucene94: sparse off-heap byte: create IndexedDISI: %w", err)
	}
	v := &sparseOffHeap94ByteVariant{
		ordToDocReader: otr,
		disi:           disi,
		dataIn:         dataIn,
	}
	return newOffHeap94ByteVectorValues(
		fieldEntry.dimension, fieldEntry.size,
		fieldEntry.similarityFunction, slice, v,
	), nil
}

func (s *sparseOffHeap94ByteVariant) iterator(_ *OffHeapByteVectorValues) index.DocIndexIterator {
	return &indexedDISIIter94{disi: s.disi}
}

func (s *sparseOffHeap94ByteVariant) ordToDoc(_ *OffHeapByteVectorValues, ord int) int {
	doc, err := s.ordToDocReader.Get(int64(ord))
	if err != nil {
		return 0
	}
	return int(doc)
}

func (s *sparseOffHeap94ByteVariant) getAcceptOrds(parent *OffHeapByteVectorValues, acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &byteOrdinalBits94{accept: acceptDocs, v: parent}
}

func (s *sparseOffHeap94ByteVariant) copy(parent *OffHeapByteVectorValues) (*OffHeapByteVectorValues, error) {
	cloned := parent.slice.Clone()
	return newOffHeap94ByteVectorValues(
		parent.dimension, parent.size,
		parent.similarityFunction, cloned,
		&sparseOffHeap94ByteVariant{
			ordToDocReader: s.ordToDocReader,
			disi:           s.disi,
			dataIn:         s.dataIn,
		},
	), nil
}

func (s *sparseOffHeap94ByteVariant) scorer(parent *OffHeapByteVectorValues, target []byte) (codec94ByteVectorScorerView, error) {
	cp, err := s.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cp.Iterator()
	return &byte94ScorerView{
		it:     it,
		bvv:    cp,
		target: target,
	}, nil
}

// ---------------------------------------------------------------------------
// Empty variant
// ---------------------------------------------------------------------------

type emptyOffHeap94ByteVariant struct{}

func newEmptyOffHeap94Byte(dimension int) *OffHeapByteVectorValues {
	return newOffHeap94ByteVectorValues(
		dimension, 0,
		index.VectorSimilarityFunctionCosine, nil,
		emptyOffHeap94ByteVariant{},
	)
}

func (emptyOffHeap94ByteVariant) iterator(_ *OffHeapByteVectorValues) index.DocIndexIterator {
	return newDenseDocIter94(0)
}

func (emptyOffHeap94ByteVariant) ordToDoc(_ *OffHeapByteVectorValues, _ int) int {
	panic("lucene94: emptyOffHeap94ByteVariant.ordToDoc not supported")
}

func (emptyOffHeap94ByteVariant) getAcceptOrds(_ *OffHeapByteVectorValues, _ util.Bits) util.Bits {
	return nil
}

func (emptyOffHeap94ByteVariant) copy(_ *OffHeapByteVectorValues) (*OffHeapByteVectorValues, error) {
	return nil, errors.New("lucene94: emptyOffHeap94ByteVariant.copy not supported")
}

func (emptyOffHeap94ByteVariant) scorer(_ *OffHeapByteVectorValues, _ []byte) (codec94ByteVectorScorerView, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Ordinal-keyed Bits (sparse variant's getAcceptOrds)
// ---------------------------------------------------------------------------

type byteOrdinalBits94 struct {
	accept util.Bits
	v      *OffHeapByteVectorValues
}

func (b *byteOrdinalBits94) Get(index int) bool {
	return b.accept.Get(b.v.OrdToDoc(index))
}

func (b *byteOrdinalBits94) Length() int { return b.v.size }

// ---------------------------------------------------------------------------
// VectorScorerView (byte vectors)
// ---------------------------------------------------------------------------

// codec94ByteVectorScorerView mirrors codecs.VectorScorerView at the
// backward_codecs/lucene94 boundary.
type codec94ByteVectorScorerView interface {
	Score() (float32, error)
	Iterator() codec94DocIDSetIteratorView
	Bulk() codec94VectorScorerBulkView
}

// byte94ScorerView is the VectorScorerView for byte vector values.
type byte94ScorerView struct {
	it     index.DocIndexIterator
	bvv    *OffHeapByteVectorValues
	target []byte
}

func (s *byte94ScorerView) Score() (float32, error) {
	vec, err := s.bvv.VectorValue(s.it.Index())
	if err != nil {
		return 0, err
	}
	return similarityCompareByte94(s.bvv.similarityFunction, vec, s.target), nil
}

func (s *byte94ScorerView) Iterator() codec94DocIDSetIteratorView {
	return &docIndexIterToView94{it: s.it}
}

func (s *byte94ScorerView) Bulk() codec94VectorScorerBulkView { return nil }

// similarityCompareByte94 mirrors VectorSimilarityFunction.compare for byte vectors.
func similarityCompareByte94(sim index.VectorSimilarityFunction, v1, v2 []byte) float32 {
	switch sim {
	case index.VectorSimilarityFunctionEuclidean:
		return euclideanSimilarityByte94(v1, v2)
	case index.VectorSimilarityFunctionDotProduct:
		return dotProductSimilarityByte94(v1, v2)
	case index.VectorSimilarityFunctionCosine:
		return cosineSimilarityByte94(v1, v2)
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		return maxInnerProductSimilarityByte94(v1, v2)
	default:
		return 0
	}
}

func euclideanSimilarityByte94(v1, v2 []byte) float32 {
	var sum int32
	for i := range v1 {
		diff := int32(v1[i]) - int32(v2[i])
		sum += diff * diff
	}
	return 1.0 / (1.0 + float32(sum))
}

func dotProductSimilarityByte94(v1, v2 []byte) float32 {
	var dot int32
	for i := range v1 {
		dot += int32(v1[i]) * int32(v2[i])
	}
	maxDot := float32(127 * 127 * len(v1))
	return (float32(dot) + maxDot) / (2.0 * maxDot)
}

func cosineSimilarityByte94(v1, v2 []byte) float32 {
	var dot, norm1, norm2 int32
	for i := range v1 {
		dot += int32(v1[i]) * int32(v2[i])
		norm1 += int32(v1[i]) * int32(v1[i])
		norm2 += int32(v2[i]) * int32(v2[i])
	}
	if norm1 == 0 || norm2 == 0 {
		return 0.5
	}
	cosine := float32(dot) / (float32Sqrt94(float32(norm1)) * float32Sqrt94(float32(norm2)))
	return (cosine + 1.0) / 2.0
}

func maxInnerProductSimilarityByte94(v1, v2 []byte) float32 {
	var dot int32
	for i := range v1 {
		dot += int32(v1[i]) * int32(v2[i])
	}
	if dot < 0 {
		return 1.0 / (1.0 - float32(dot)/1000.0)
	}
	return float32(dot)/1000.0 + 1.0
}

func float32Sqrt94(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}
