// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ByteVectorValues provides access to per-document vector values indexed as
// bytes. It is the Go port of org.apache.lucene.index.ByteVectorValues; the
// declaration lives in spi (see [KnnVectorValues]).
type ByteVectorValues = spi.ByteVectorValues

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

// byteVectorValuesFromBytes is the anonymous ByteVectorValues returned by
// ByteVectorValues.fromBytes.
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

// OrdToDoc carries the KnnVectorValues.ordToDoc default, which returns ord.
func (b *byteVectorValuesFromBytes) OrdToDoc(ord int) int {
	return ord
}

// Prefetch carries the KnnVectorValues.prefetch default, which does nothing.
func (b *byteVectorValuesFromBytes) Prefetch(ordsToPrefetch []int, numOrds int) error {
	return nil
}

func (b *byteVectorValuesFromBytes) Copy() (KnnVectorValues, error) {
	return b, nil
}

// GetEncoding carries the ByteVectorValues.getEncoding override.
func (b *byteVectorValuesFromBytes) GetEncoding() VectorEncoding {
	return VectorEncodingByte
}

// GetVectorByteLength carries the KnnVectorValues.getVectorByteLength default.
func (b *byteVectorValuesFromBytes) GetVectorByteLength() int {
	return b.Dimension() * VectorEncodingByteSize(b.GetEncoding())
}

// GetAcceptOrds carries the KnnVectorValues.getAcceptOrds default.
func (b *byteVectorValuesFromBytes) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(b, acceptDocs)
}

func (b *byteVectorValuesFromBytes) Iterator() DocIndexIterator {
	return spi.CreateDenseIterator(b)
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

// Scorer carries the ByteVectorValues.scorer default, which throws
// UnsupportedOperationException.
func (b *byteVectorValuesFromBytes) Scorer(target []byte) (util.VectorScorer, error) {
	return nil, errors.New("not implemented")
}

// Rescorer carries the ByteVectorValues.rescorer default, which returns
// scorer(target).
func (b *byteVectorValuesFromBytes) Rescorer(target []byte) (util.VectorScorer, error) {
	return b.Scorer(target)
}
