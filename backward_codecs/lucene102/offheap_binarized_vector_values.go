// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version 2.0
//	(the "License"); you may not use this file except in compliance with
//	the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0

package lucene102

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene95"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

const (
	// floatBytes is Float.BYTES.
	floatBytes = 4
	// shortBytes is Short.BYTES.
	shortBytes = 2
)

// OffHeapBinarizedVectorValues is the Go port of the public abstract class
// org.apache.lucene.backward_codecs.lucene102.OffHeapBinarizedVectorValues
// (Apache Lucene 10.5.0): binarized vector values loaded from off-heap.
//
// The struct carries the fields and concrete members of the abstract class;
// the dense, sparse and empty subclasses embed it and add copy, iterator,
// getAcceptOrds and scorer(float[]).
type OffHeapBinarizedVectorValues struct {
	dimension          int
	size               int
	numBytes           int
	similarityFunction index.VectorSimilarityFunction
	vectorsScorer      hnsw.FlatVectorsScorer

	slice                 store.IndexInput
	binaryValue           []byte
	byteSize              int
	lastOrd               int
	correctiveValues      []float32
	quantizedComponentSum int
	binaryQuantizer       *quantization.OptimizedScalarQuantizer
	centroid              []float32
	centroidDp            float32
	discretizedDimensions int
}

// newOffHeapBinarizedVectorValues mirrors the OffHeapBinarizedVectorValues
// constructor.
func newOffHeapBinarizedVectorValues(
	dimension int,
	size int,
	centroid []float32,
	centroidDp float32,
	quantizer *quantization.OptimizedScalarQuantizer,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) *OffHeapBinarizedVectorValues {
	numBytes := quantization.Discretize(dimension, 64) / 8
	return &OffHeapBinarizedVectorValues{
		dimension:             dimension,
		size:                  size,
		similarityFunction:    similarityFunction,
		vectorsScorer:         vectorsScorer,
		slice:                 slice,
		centroid:              centroid,
		centroidDp:            centroidDp,
		numBytes:              numBytes,
		correctiveValues:      make([]float32, 3),
		byteSize:              numBytes + (floatBytes * 3) + shortBytes,
		binaryValue:           make([]byte, numBytes),
		lastOrd:               -1,
		binaryQuantizer:       quantizer,
		discretizedDimensions: quantization.Discretize(dimension, 64),
	}
}

// Dimension mirrors dimension().
func (v *OffHeapBinarizedVectorValues) Dimension() int { return v.dimension }

// Size mirrors size().
func (v *OffHeapBinarizedVectorValues) Size() int { return v.size }

// VectorValue mirrors vectorValue(int): it reads the binary code, the three
// corrective values and the unsigned quantized component sum of targetOrd.
func (v *OffHeapBinarizedVectorValues) VectorValue(targetOrd int) ([]byte, error) {
	if v.lastOrd == targetOrd {
		return v.binaryValue, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, err
	}
	if err := v.slice.ReadBytes(v.binaryValue, 0, v.numBytes); err != nil {
		return nil, err
	}
	if err := v.slice.ReadFloats(v.correctiveValues, 0, 3); err != nil {
		return nil, err
	}
	sum, err := v.slice.ReadShort()
	if err != nil {
		return nil, err
	}
	v.quantizedComponentSum = int(uint16(sum))
	v.lastOrd = targetOrd
	return v.binaryValue, nil
}

// DiscretizedDimensions mirrors the discretizedDimensions() override.
func (v *OffHeapBinarizedVectorValues) DiscretizedDimensions() int {
	return v.discretizedDimensions
}

// GetCentroidDP mirrors the getCentroidDP() override.
func (v *OffHeapBinarizedVectorValues) GetCentroidDP() (float32, error) {
	return v.centroidDp, nil
}

// GetCorrectiveTerms mirrors getCorrectiveTerms(int).
func (v *OffHeapBinarizedVectorValues) GetCorrectiveTerms(targetOrd int) (quantization.QuantizationResult, error) {
	if v.lastOrd == targetOrd {
		return quantization.QuantizationResult{
			LowerInterval:         v.correctiveValues[0],
			UpperInterval:         v.correctiveValues[1],
			AdditionalCorrection:  v.correctiveValues[2],
			QuantizedComponentSum: v.quantizedComponentSum,
		}, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd)*int64(v.byteSize) + int64(v.numBytes)); err != nil {
		return quantization.QuantizationResult{}, err
	}
	if err := v.slice.ReadFloats(v.correctiveValues, 0, 3); err != nil {
		return quantization.QuantizationResult{}, err
	}
	sum, err := v.slice.ReadShort()
	if err != nil {
		return quantization.QuantizationResult{}, err
	}
	v.quantizedComponentSum = int(uint16(sum))
	return quantization.QuantizationResult{
		LowerInterval:         v.correctiveValues[0],
		UpperInterval:         v.correctiveValues[1],
		AdditionalCorrection:  v.correctiveValues[2],
		QuantizedComponentSum: v.quantizedComponentSum,
	}, nil
}

// GetQuantizer mirrors getQuantizer().
func (v *OffHeapBinarizedVectorValues) GetQuantizer() *quantization.OptimizedScalarQuantizer {
	return v.binaryQuantizer
}

// GetCentroid mirrors getCentroid().
func (v *OffHeapBinarizedVectorValues) GetCentroid() ([]float32, error) {
	return v.centroid, nil
}

// GetVectorByteLength mirrors the getVectorByteLength() override.
func (v *OffHeapBinarizedVectorValues) GetVectorByteLength() int {
	return v.numBytes
}

// OrdToDoc carries the KnnVectorValues.ordToDoc default.
func (v *OffHeapBinarizedVectorValues) OrdToDoc(ord int) int { return ord }

// Prefetch carries the KnnVectorValues.prefetch default, which does nothing.
func (v *OffHeapBinarizedVectorValues) Prefetch(_ []int, _ int) error { return nil }

// GetEncoding carries the ByteVectorValues.getEncoding override.
func (v *OffHeapBinarizedVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingByte
}

// Scorer carries the ByteVectorValues.scorer(byte[]) default, which throws
// UnsupportedOperationException.
func (v *OffHeapBinarizedVectorValues) Scorer(_ []byte) (util.VectorScorer, error) {
	return nil, quantization.ErrUnsupportedOperation
}

// Rescorer carries the ByteVectorValues.rescorer default, which returns
// scorer(target).
func (v *OffHeapBinarizedVectorValues) Rescorer(target []byte) (util.VectorScorer, error) {
	return v.Scorer(target)
}

// loadOffHeapBinarizedVectorValues mirrors the static
// OffHeapBinarizedVectorValues.load: an empty, dense or sparse view depending
// on the OrdToDoc configuration.
func loadOffHeapBinarizedVectorValues(
	configuration *lucene95.OrdToDocDISIReaderConfiguration,
	dimension int,
	size int,
	binaryQuantizer *quantization.OptimizedScalarQuantizer,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	centroid []float32,
	centroidDp float32,
	quantizedVectorDataOffset int64,
	quantizedVectorDataLength int64,
	vectorData store.IndexInput,
) (BinarizedByteVectorValues, error) {
	if configuration.IsEmpty() {
		return newEmptyOffHeapVectorValues(dimension, similarityFunction, vectorsScorer), nil
	}
	bytesSlice, err := vectorData.Slice("quantized-vector-data", quantizedVectorDataOffset, quantizedVectorDataLength)
	if err != nil {
		return nil, err
	}
	if configuration.IsDense() {
		return newDenseOffHeapVectorValues(
			dimension,
			size,
			centroid,
			centroidDp,
			binaryQuantizer,
			similarityFunction,
			vectorsScorer,
			bytesSlice), nil
	}
	sparse, err := newSparseOffHeapVectorValues(
		configuration,
		dimension,
		size,
		centroid,
		centroidDp,
		binaryQuantizer,
		vectorData,
		similarityFunction,
		vectorsScorer,
		bytesSlice)
	if err != nil {
		return nil, err
	}
	return sparse, nil
}

// offHeapBinarizedVectorScorer is the anonymous VectorScorer returned by
// scorer(float[]) of the dense and sparse views: it scores the iterator's
// current index.
type offHeapBinarizedVectorScorer struct {
	scorer   utilhnsw.RandomVectorScorer
	iterator index.DocIndexIterator
}

// Score scores the iterator's current index.
func (s *offHeapBinarizedVectorScorer) Score() (float32, error) {
	return s.scorer.Score(s.iterator.Index())
}

// Iterator returns the iterator over the scored copy.
func (s *offHeapBinarizedVectorScorer) Iterator() util.DocIdSetIterator { return s.iterator }

// ---------------------------------------------------------------------------
// DenseOffHeapVectorValues
// ---------------------------------------------------------------------------

// DenseOffHeapVectorValues is the Go port of the public static nested class
// OffHeapBinarizedVectorValues.DenseOffHeapVectorValues: dense off-heap
// binarized vector values.
type DenseOffHeapVectorValues struct {
	*OffHeapBinarizedVectorValues
}

// newDenseOffHeapVectorValues mirrors the DenseOffHeapVectorValues constructor.
func newDenseOffHeapVectorValues(
	dimension int,
	size int,
	centroid []float32,
	centroidDp float32,
	binaryQuantizer *quantization.OptimizedScalarQuantizer,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) *DenseOffHeapVectorValues {
	return &DenseOffHeapVectorValues{
		OffHeapBinarizedVectorValues: newOffHeapBinarizedVectorValues(
			dimension, size, centroid, centroidDp, binaryQuantizer, similarityFunction, vectorsScorer, slice),
	}
}

// copyDense mirrors the covariant DenseOffHeapVectorValues.copy().
func (v *DenseOffHeapVectorValues) copyDense() *DenseOffHeapVectorValues {
	return newDenseOffHeapVectorValues(
		v.dimension,
		v.size,
		v.centroid,
		v.centroidDp,
		v.binaryQuantizer,
		v.similarityFunction,
		v.vectorsScorer,
		v.slice.Clone())
}

// Copy mirrors copy().
func (v *DenseOffHeapVectorValues) Copy() (index.KnnVectorValues, error) { return v.copyDense(), nil }

// CopyByteVectorValues mirrors copy() typed as ByteVectorValues.
func (v *DenseOffHeapVectorValues) CopyByteVectorValues() (index.ByteVectorValues, error) {
	return v.copyDense(), nil
}

// CopyBinarizedByteVectorValues mirrors copy() typed as
// BinarizedByteVectorValues.
func (v *DenseOffHeapVectorValues) CopyBinarizedByteVectorValues() (BinarizedByteVectorValues, error) {
	return v.copyDense(), nil
}

// GetAcceptOrds mirrors DenseOffHeapVectorValues.getAcceptOrds, which returns
// acceptDocs.
func (v *DenseOffHeapVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return acceptDocs
}

// ScorerFloat mirrors DenseOffHeapVectorValues.scorer(float[]).
func (v *DenseOffHeapVectorValues) ScorerFloat(target []float32) (util.VectorScorer, error) {
	copied := v.copyDense()
	iterator := copied.Iterator()
	scorer, err := v.vectorsScorer.GetRandomVectorScorer(v.similarityFunction, copied, target)
	if err != nil {
		return nil, err
	}
	return &offHeapBinarizedVectorScorer{scorer: scorer, iterator: iterator}, nil
}

// Iterator mirrors DenseOffHeapVectorValues.iterator(): createDenseIterator().
func (v *DenseOffHeapVectorValues) Iterator() index.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// ---------------------------------------------------------------------------
// sparseOffHeapVectorValues
// ---------------------------------------------------------------------------

// sparseOffHeapVectorValues is the Go port of the private static nested class
// OffHeapBinarizedVectorValues.SparseOffHeapVectorValues: sparse off-heap
// binarized vector values.
type sparseOffHeapVectorValues struct {
	*OffHeapBinarizedVectorValues

	ordToDoc *packed.DirectMonotonicReader
	disi     *lucene90.IndexedDISI
	// dataIn was used to init a new IndexedDIS for #randomAccess()
	dataIn        store.IndexInput
	configuration *lucene95.OrdToDocDISIReaderConfiguration
}

// newSparseOffHeapVectorValues mirrors the SparseOffHeapVectorValues
// constructor.
func newSparseOffHeapVectorValues(
	configuration *lucene95.OrdToDocDISIReaderConfiguration,
	dimension int,
	size int,
	centroid []float32,
	centroidDp float32,
	binaryQuantizer *quantization.OptimizedScalarQuantizer,
	dataIn store.IndexInput,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) (*sparseOffHeapVectorValues, error) {
	base := newOffHeapBinarizedVectorValues(
		dimension, size, centroid, centroidDp, binaryQuantizer, similarityFunction, vectorsScorer, slice)
	ordToDoc, err := configuration.GetDirectMonotonicReader(dataIn)
	if err != nil {
		return nil, err
	}
	disi, err := configuration.GetIndexedDISI(dataIn)
	if err != nil {
		return nil, err
	}
	return &sparseOffHeapVectorValues{
		OffHeapBinarizedVectorValues: base,
		configuration:                configuration,
		dataIn:                       dataIn,
		ordToDoc:                     ordToDoc,
		disi:                         disi,
	}, nil
}

// copySparse mirrors the covariant SparseOffHeapVectorValues.copy().
func (v *sparseOffHeapVectorValues) copySparse() (*sparseOffHeapVectorValues, error) {
	return newSparseOffHeapVectorValues(
		v.configuration,
		v.dimension,
		v.size,
		v.centroid,
		v.centroidDp,
		v.binaryQuantizer,
		v.dataIn,
		v.similarityFunction,
		v.vectorsScorer,
		v.slice.Clone())
}

// Copy mirrors copy().
func (v *sparseOffHeapVectorValues) Copy() (index.KnnVectorValues, error) {
	copied, err := v.copySparse()
	if err != nil {
		return nil, err
	}
	return copied, nil
}

// CopyByteVectorValues mirrors copy() typed as ByteVectorValues.
func (v *sparseOffHeapVectorValues) CopyByteVectorValues() (index.ByteVectorValues, error) {
	copied, err := v.copySparse()
	if err != nil {
		return nil, err
	}
	return copied, nil
}

// CopyBinarizedByteVectorValues mirrors copy() typed as
// BinarizedByteVectorValues.
func (v *sparseOffHeapVectorValues) CopyBinarizedByteVectorValues() (BinarizedByteVectorValues, error) {
	copied, err := v.copySparse()
	if err != nil {
		return nil, err
	}
	return copied, nil
}

// OrdToDoc mirrors SparseOffHeapVectorValues.ordToDoc: (int) ordToDoc.get(ord).
// Java's DirectReader wraps an I/O failure in a RuntimeException, rendered
// here as a panic.
func (v *sparseOffHeapVectorValues) OrdToDoc(ord int) int {
	doc, err := v.ordToDoc.Get(int64(ord))
	if err != nil {
		panic(fmt.Errorf("lucene102: read ordToDoc(%d): %w", ord, err))
	}
	return int(doc)
}

// GetAcceptOrds mirrors SparseOffHeapVectorValues.getAcceptOrds.
func (v *sparseOffHeapVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &sparseOffHeapAcceptOrds{acceptDocs: acceptDocs, values: v}
}

// sparseOffHeapAcceptOrds is the anonymous Bits returned by
// SparseOffHeapVectorValues.getAcceptOrds.
type sparseOffHeapAcceptOrds struct {
	acceptDocs util.Bits
	values     *sparseOffHeapVectorValues
}

// Get accepts the ordinal whose document acceptDocs accepts.
func (b *sparseOffHeapAcceptOrds) Get(index int) bool {
	return b.acceptDocs.Get(b.values.OrdToDoc(index))
}

// Length returns the number of vectors.
func (b *sparseOffHeapAcceptOrds) Length() int { return b.values.size }

// Iterator mirrors SparseOffHeapVectorValues.iterator():
// IndexedDISI.asDocIndexIterator(disi).
func (v *sparseOffHeapVectorValues) Iterator() index.DocIndexIterator {
	return lucene90.AsDocIndexIterator(v.disi)
}

// ScorerFloat mirrors SparseOffHeapVectorValues.scorer(float[]).
func (v *sparseOffHeapVectorValues) ScorerFloat(target []float32) (util.VectorScorer, error) {
	copied, err := v.copySparse()
	if err != nil {
		return nil, err
	}
	iterator := copied.Iterator()
	scorer, err := v.vectorsScorer.GetRandomVectorScorer(v.similarityFunction, copied, target)
	if err != nil {
		return nil, err
	}
	return &offHeapBinarizedVectorScorer{scorer: scorer, iterator: iterator}, nil
}

// ---------------------------------------------------------------------------
// emptyOffHeapVectorValues
// ---------------------------------------------------------------------------

// emptyOffHeapVectorValues is the Go port of the private static nested class
// OffHeapBinarizedVectorValues.EmptyOffHeapVectorValues.
type emptyOffHeapVectorValues struct {
	*OffHeapBinarizedVectorValues
}

// newEmptyOffHeapVectorValues mirrors the EmptyOffHeapVectorValues
// constructor: super(dimension, 0, null, Float.NaN, null, similarityFunction,
// vectorsScorer, null).
func newEmptyOffHeapVectorValues(
	dimension int, similarityFunction index.VectorSimilarityFunction, vectorsScorer hnsw.FlatVectorsScorer,
) *emptyOffHeapVectorValues {
	return &emptyOffHeapVectorValues{
		OffHeapBinarizedVectorValues: newOffHeapBinarizedVectorValues(
			dimension, 0, nil, float32(math.NaN()), nil, similarityFunction, vectorsScorer, nil),
	}
}

// Iterator mirrors EmptyOffHeapVectorValues.iterator(): createDenseIterator().
func (v *emptyOffHeapVectorValues) Iterator() index.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// Copy mirrors EmptyOffHeapVectorValues.copy(), which throws
// UnsupportedOperationException.
func (v *emptyOffHeapVectorValues) Copy() (index.KnnVectorValues, error) {
	return nil, quantization.ErrUnsupportedOperation
}

// CopyByteVectorValues mirrors copy() typed as ByteVectorValues.
func (v *emptyOffHeapVectorValues) CopyByteVectorValues() (index.ByteVectorValues, error) {
	return nil, quantization.ErrUnsupportedOperation
}

// CopyBinarizedByteVectorValues mirrors copy() typed as
// BinarizedByteVectorValues.
func (v *emptyOffHeapVectorValues) CopyBinarizedByteVectorValues() (BinarizedByteVectorValues, error) {
	return nil, quantization.ErrUnsupportedOperation
}

// GetAcceptOrds mirrors EmptyOffHeapVectorValues.getAcceptOrds, which returns
// null.
func (v *emptyOffHeapVectorValues) GetAcceptOrds(_ util.Bits) util.Bits {
	return nil
}

// ScorerFloat mirrors EmptyOffHeapVectorValues.scorer(float[]), which returns
// null.
func (v *emptyOffHeapVectorValues) ScorerFloat(_ []float32) (util.VectorScorer, error) {
	return nil, nil
}

// Compile-time guards.
var (
	_ BinarizedByteVectorValues = (*DenseOffHeapVectorValues)(nil)
	_ BinarizedByteVectorValues = (*sparseOffHeapVectorValues)(nil)
	_ BinarizedByteVectorValues = (*emptyOffHeapVectorValues)(nil)
	_ util.VectorScorer         = (*offHeapBinarizedVectorScorer)(nil)
	_ util.Bits                 = (*sparseOffHeapAcceptOrds)(nil)
)
