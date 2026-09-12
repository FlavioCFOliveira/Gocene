// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ByteVectorValues provides access to per-document vector values indexed as bytes.
// This is the Go port of Lucene's org.apache.lucene.index.ByteVectorValues.
type ByteVectorValues interface {
	KnnVectorValues
	// VectorValue returns the vector value for the given vector ordinal.
	// It is illegal to call this method if ord is not in [0, Size() - 1].
	VectorValue(ord int) ([]byte, error)

	// CopyByteVectorValues creates a new copy of this ByteVectorValues.
	CopyByteVectorValues() (ByteVectorValues, error)

	// Scorer returns a VectorScorer for the given query vector and the current ByteVectorValues.
	Scorer(target []byte) (util.VectorScorer, error)

	// Rescorer rescores using the given query vector and the current ByteVectorValues.
	Rescorer(target []byte) (util.VectorScorer, error)
}

// CheckByteVectorField checks the Vector Encoding of a field.
// This is the Go port of ByteVectorValues.checkField.
func CheckByteVectorField(in LeafReader, field string) error {
	fi := in.GetFieldInfos().FieldInfoByName(field)
	if fi != nil && fi.VectorDimension() != 0 && fi.VectorEncoding() != VectorEncodingByte {
		return fmt.Errorf("unexpected vector encoding (%s) for field %s (expected=%s)",
			fi.VectorEncoding(), field, VectorEncodingByte)
	}
	return nil
}

// FromBytes creates a ByteVectorValues from a list of byte arrays.
// This is the Go port of ByteVectorValues.fromBytes.
func FromBytes(vectors [][]byte, dim int) ByteVectorValues {
	return &byteVectorValuesFromBytes{
		vectors: vectors,
		dim:     dim,
	}
}

type byteVectorValuesFromBytes struct {
	vectors [][]byte
	dim     int
}

func (b *byteVectorValuesFromBytes) Dimension() int {
	return b.dim
}

func (b *byteVectorValuesFromBytes) Size() int {
	return len(b.vectors)
}

func (b *byteVectorValuesFromBytes) OrdToDoc(ord int) int {
	return ord
}

func (b *byteVectorValuesFromBytes) Prefetch(ordsToPrefetch []int, numOrds int) error {
	return nil
}

func (b *byteVectorValuesFromBytes) Copy() (KnnVectorValues, error) {
	return b, nil
}

func (b *byteVectorValuesFromBytes) GetEncoding() VectorEncoding {
	return VectorEncodingByte
}

func (b *byteVectorValuesFromBytes) GetVectorByteLength() int {
	return b.Dimension() * VectorEncodingByteSize(b.GetEncoding())
}

func (b *byteVectorValuesFromBytes) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &acceptOrdsBitSet{
		acceptDocs: acceptDocs,
		size:       b.Size(),
	}
}

func (b *byteVectorValuesFromBytes) Iterator() util.DocIndexIterator {
	return util.NewDenseDocIndexIterator(b.Size())
}

func (b *byteVectorValuesFromBytes) VectorValue(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(b.vectors) {
		return nil, errors.New("index out of bounds")
	}
	return b.vectors[ord], nil
}

func (b *byteVectorValuesFromBytes) CopyByteVectorValues() (ByteVectorValues, error) {
	return b, nil
}

func (b *byteVectorValuesFromBytes) Scorer(target []byte) (util.VectorScorer, error) {
	return nil, errors.New("not implemented")
}

func (b *byteVectorValuesFromBytes) Rescorer(target []byte) (util.VectorScorer, error) {
	return b.Scorer(target)
}

type acceptOrdsBitSet struct {
	acceptDocs util.Bits
	size       int
}

func (b *acceptOrdsBitSet) Get(index int) bool {
	return b.acceptDocs.Get(b.OrdToDoc(index))
}

func (b *acceptOrdsBitSet) Length() int {
	return b.size
}

func (b *acceptOrdsBitSet) Cardinality() int {
	count := 0
	for i := 0; i < b.size; i++ {
		if b.Get(i) {
			count++
		}
	}
	return count
}

func (b *acceptOrdsBitSet) OrdToDoc(index int) int {
	return index
}
