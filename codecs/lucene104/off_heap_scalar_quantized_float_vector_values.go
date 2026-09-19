// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/codecs/lucene104/OffHeapScalarQuantizedFloatVectorValues.java

package lucene104

import (
	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene95"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// offHeapScalarQuantizedFloatVectorValues reads quantized vector values from
// the index input and returns float vector values after dequantizing them.
//
// Mirrors {@code abstract class OffHeapScalarQuantizedFloatVectorValues extends
// FloatVectorValues implements HasIndexSlice} (Lucene 10.5.0,
// package-private, @lucene.internal). Go has no abstract classes, so the type
// is the interface the class publishes; its shared state and concrete members
// live on [baseOffHeapScalarQuantizedFloatVectorValues], which each concrete
// subtype embeds.
//
// The class exists for read-only indexes whose full-precision float vectors
// have been dropped to save storage space.
type offHeapScalarQuantizedFloatVectorValues interface {
	spi.FloatVectorValues
	quantization.HasIndexSlice

	// GetCorrectiveTerms carries the public (non-override) member
	// {@code OptimizedScalarQuantizer.QuantizationResult getCorrectiveTerms(int)}.
	GetCorrectiveTerms(targetOrd int) (quantization.QuantizationResult, error)
}

// baseOffHeapScalarQuantizedFloatVectorValues carries the fields and concrete
// members of the abstract class OffHeapScalarQuantizedFloatVectorValues.
//
// The owner field is the Go stand-in for the dispatch Java gets from
// {@code this} on the members the subclasses override (copy, getAcceptOrds,
// scorer, iterator, ordToDoc).
type baseOffHeapScalarQuantizedFloatVectorValues struct {
	owner offHeapScalarQuantizedFloatVectorValues

	dimension          int
	size               int
	similarityFunction index.VectorSimilarityFunction
	vectorsScorer      hnsw.FlatVectorsScorer

	slice                   store.IndexInput
	vectorValue             []float32
	byteValue               []byte
	unpackedByteVectorValue []byte
	byteSize                int
	lastOrd                 int
	correctiveValues        []float32
	quantizedComponentSum   int
	encoding                quantization.ScalarEncoding
	centroid                []float32
}

// init reproduces the package-private constructor
//
//	OffHeapScalarQuantizedFloatVectorValues(int dimension, int size,
//	    float[] centroid, ScalarEncoding encoding,
//	    VectorSimilarityFunction similarityFunction,
//	    FlatVectorsScorer vectorsScorer, IndexInput slice)
func (v *baseOffHeapScalarQuantizedFloatVectorValues) init(
	dimension int,
	size int,
	centroid []float32,
	encoding quantization.ScalarEncoding,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) {
	v.dimension = dimension
	v.size = size
	v.similarityFunction = similarityFunction
	v.vectorsScorer = vectorsScorer
	v.slice = slice
	v.centroid = centroid
	v.correctiveValues = make([]float32, offHeapQuantizedCorrectiveValues)
	v.encoding = encoding
	docPackedLength := encoding.GetDocPackedLength(dimension)
	v.byteSize = docPackedLength + (4 * 3) + 4
	// ByteBuffer.allocate(docPackedLength) and its backing array().
	v.byteValue = make([]byte, docPackedLength)
	v.vectorValue = make([]float32, dimension)
	v.unpackedByteVectorValue = make([]byte, dimension)
	v.lastOrd = -1
}

// Dimension reproduces {@code return dimension;}.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) Dimension() int { return v.dimension }

// Size reproduces {@code return size;}.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) Size() int { return v.size }

// VectorValue reproduces the body of
// {@code public float[] vectorValue(int targetOrd)}: read the quantized byte
// vector plus its corrective values, unpack it according to the encoding, and
// dequantize into the float scratch.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) VectorValue(targetOrd int) ([]float32, error) {
	if v.lastOrd == targetOrd {
		return v.vectorValue, nil
	}

	// read quantized byte vector, correctiveValues and quantizedComponentSum
	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, err
	}
	if err := v.slice.ReadBytes(v.byteValue, 0, len(v.byteValue)); err != nil {
		return nil, err
	}
	if err := v.slice.ReadFloats(v.correctiveValues, 0, offHeapQuantizedCorrectiveValues); err != nil {
		return nil, err
	}
	sum, err := v.slice.ReadInt()
	if err != nil {
		return nil, err
	}
	v.quantizedComponentSum = int(sum)

	// unpack bytes
	switch v.encoding {
	case quantization.ScalarEncodingPackedNibble:
		unpackNibbles(v.byteValue, v.unpackedByteVectorValue)
	case quantization.ScalarEncodingSingleBitQueryNibble:
		quantization.UnpackBinary(v.byteValue, v.unpackedByteVectorValue)
	case quantization.ScalarEncodingDibitQueryNibble:
		quantization.UntransposeDibit(v.byteValue, v.unpackedByteVectorValue)
	case quantization.ScalarEncodingUnsignedByte, quantization.ScalarEncodingSevenBit:
		quantization.DeQuantize(
			v.byteValue,
			v.vectorValue,
			v.encoding.GetBits(),
			v.correctiveValues[0],
			v.correctiveValues[1],
			v.centroid,
		)
		v.lastOrd = targetOrd
		return v.vectorValue, nil
	}

	// dequantize
	quantization.DeQuantize(
		v.unpackedByteVectorValue,
		v.vectorValue,
		v.encoding.GetBits(),
		v.correctiveValues[0],
		v.correctiveValues[1],
		v.centroid,
	)

	v.lastOrd = targetOrd
	return v.vectorValue, nil
}

// GetCorrectiveTerms reproduces
// {@code public OptimizedScalarQuantizer.QuantizationResult getCorrectiveTerms(int targetOrd)}.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) GetCorrectiveTerms(targetOrd int) (quantization.QuantizationResult, error) {
	if v.lastOrd == targetOrd {
		return quantization.QuantizationResult{
			LowerInterval:         v.correctiveValues[0],
			UpperInterval:         v.correctiveValues[1],
			AdditionalCorrection:  v.correctiveValues[2],
			QuantizedComponentSum: v.quantizedComponentSum,
		}, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd)*int64(v.byteSize) + int64(len(v.byteValue))); err != nil {
		return quantization.QuantizationResult{}, err
	}
	if err := v.slice.ReadFloats(v.correctiveValues, 0, offHeapQuantizedCorrectiveValues); err != nil {
		return quantization.QuantizationResult{}, err
	}
	sum, err := v.slice.ReadInt()
	if err != nil {
		return quantization.QuantizationResult{}, err
	}
	v.quantizedComponentSum = int(sum)
	return quantization.QuantizationResult{
		LowerInterval:         v.correctiveValues[0],
		UpperInterval:         v.correctiveValues[1],
		AdditionalCorrection:  v.correctiveValues[2],
		QuantizedComponentSum: v.quantizedComponentSum,
	}, nil
}

// GetVectorByteLength reproduces {@code return dimension;}.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) GetVectorByteLength() int { return v.dimension }

// GetSlice reproduces {@code return slice;}.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) GetSlice() store.IndexInput { return v.slice }

// GetEncoding carries FloatVectorValues.getEncoding(), whose body is
// {@code return VectorEncoding.FLOAT32;}.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) GetEncoding() spi.VectorEncoding {
	return util.VectorEncodingFloat32
}

// OrdToDoc carries the KnnVectorValues default {@code return ord;}. The sparse
// subtype overrides it.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) OrdToDoc(ord int) int { return ord }

// Prefetch carries the KnnVectorValues default, whose body is empty.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) Prefetch(_ []int, _ int) error { return nil }

// Rescorer carries FloatVectorValues.rescorer(float[]), whose default body is
// {@code return scorer(target);}.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) Rescorer(target []float32) (util.VectorScorer, error) {
	return v.owner.Scorer(target)
}

// Copy carries KnnVectorValues.copy() through the owner.
func (v *baseOffHeapScalarQuantizedFloatVectorValues) Copy() (spi.KnnVectorValues, error) {
	return v.owner.Copy()
}

// CopyFloatVectorValues carries the covariant FloatVectorValues.copy().
func (v *baseOffHeapScalarQuantizedFloatVectorValues) CopyFloatVectorValues() (spi.FloatVectorValues, error) {
	return v.owner.CopyFloatVectorValues()
}

// ─── load ───────────────────────────────────────────────────────────────────

// loadOffHeapScalarQuantizedFloatVectorValues reproduces the package-private
// static
//
//	static OffHeapScalarQuantizedFloatVectorValues load(
//	    OrdToDocDISIReaderConfiguration configuration, int dimension, int size,
//	    ScalarEncoding encoding, VectorSimilarityFunction similarityFunction,
//	    FlatVectorsScorer vectorsScorer, float[] centroid,
//	    long quantizedVectorDataOffset, long quantizedVectorDataLength,
//	    IndexInput vectorData)
func loadOffHeapScalarQuantizedFloatVectorValues(
	configuration *lucene95.OrdToDocDISIReaderConfiguration,
	dimension int,
	size int,
	encoding quantization.ScalarEncoding,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	centroid []float32,
	quantizedVectorDataOffset int64,
	quantizedVectorDataLength int64,
	vectorData store.IndexInput,
) (offHeapScalarQuantizedFloatVectorValues, error) {
	if configuration.IsEmpty() {
		return newEmptyOffHeapFloatVectorValues(dimension, similarityFunction, vectorsScorer), nil
	}
	bytesSlice, err := vectorData.Slice(
		"scalar-quantized-float-vector-data", quantizedVectorDataOffset, quantizedVectorDataLength)
	if err != nil {
		return nil, err
	}
	if configuration.IsDense() {
		return newDenseOffHeapFloatVectorValues(
			dimension, size, centroid, encoding, similarityFunction, vectorsScorer, bytesSlice), nil
	}
	return newSparseOffHeapFloatVectorValues(
		configuration, dimension, size, centroid, encoding, vectorData,
		similarityFunction, vectorsScorer, bytesSlice)
}

// ─── DenseOffHeapVectorValues ───────────────────────────────────────────────

// denseOffHeapFloatVectorValues are dense off-heap scalar quantized vector
// values.
//
// Mirrors the private nested class
// {@code static class OffHeapScalarQuantizedFloatVectorValues.DenseOffHeapVectorValues}.
type denseOffHeapFloatVectorValues struct {
	baseOffHeapScalarQuantizedFloatVectorValues
}

// newDenseOffHeapFloatVectorValues reproduces the constructor, whose body is
// the super call.
func newDenseOffHeapFloatVectorValues(
	dimension int,
	size int,
	centroid []float32,
	encoding quantization.ScalarEncoding,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) *denseOffHeapFloatVectorValues {
	v := &denseOffHeapFloatVectorValues{}
	v.init(dimension, size, centroid, encoding, similarityFunction, vectorsScorer, slice)
	v.owner = v
	return v
}

// copyDense reproduces the covariant
// {@code public DenseOffHeapVectorValues copy()}.
func (v *denseOffHeapFloatVectorValues) copyDense() (*denseOffHeapFloatVectorValues, error) {
	var cloned store.IndexInput
	if v.slice != nil {
		cloned = v.slice.Clone()
	}
	return newDenseOffHeapFloatVectorValues(
		v.dimension, v.size, v.centroid, v.encoding, v.similarityFunction, v.vectorsScorer, cloned), nil
}

// Copy carries KnnVectorValues.copy().
func (v *denseOffHeapFloatVectorValues) Copy() (spi.KnnVectorValues, error) { return v.copyDense() }

// CopyFloatVectorValues carries the covariant FloatVectorValues.copy().
func (v *denseOffHeapFloatVectorValues) CopyFloatVectorValues() (spi.FloatVectorValues, error) {
	return v.copyDense()
}

// GetAcceptOrds reproduces {@code return acceptDocs;}.
func (v *denseOffHeapFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return acceptDocs
}

// Iterator reproduces {@code return createDenseIterator();}.
func (v *denseOffHeapFloatVectorValues) Iterator() spi.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// Scorer reproduces DenseOffHeapVectorValues.scorer(float[]).
func (v *denseOffHeapFloatVectorValues) Scorer(target []float32) (util.VectorScorer, error) {
	cp, err := v.copyDense()
	if err != nil {
		return nil, err
	}
	iterator := cp.Iterator()
	scorer, err := v.vectorsScorer.GetRandomVectorScorer(v.similarityFunction, cp, target)
	if err != nil {
		return nil, err
	}
	return &offHeapQuantizedVectorScorer{scorer: scorer, iterator: iterator, sparse: false}, nil
}

// ─── SparseOffHeapVectorValues ──────────────────────────────────────────────

// sparseOffHeapFloatVectorValues are sparse off-heap scalar quantized vector
// values.
//
// Mirrors the private nested class
// {@code static class OffHeapScalarQuantizedFloatVectorValues.SparseOffHeapVectorValues}.
type sparseOffHeapFloatVectorValues struct {
	baseOffHeapScalarQuantizedFloatVectorValues

	ordToDoc *packed.DirectMonotonicReader
	disi     *lucene90.IndexedDISI
	// dataIn was used to init a new IndexedDIS for #randomAccess()
	dataIn        store.IndexInput
	configuration *lucene95.OrdToDocDISIReaderConfiguration
}

// newSparseOffHeapFloatVectorValues reproduces the constructor.
func newSparseOffHeapFloatVectorValues(
	configuration *lucene95.OrdToDocDISIReaderConfiguration,
	dimension int,
	size int,
	centroid []float32,
	encoding quantization.ScalarEncoding,
	dataIn store.IndexInput,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) (*sparseOffHeapFloatVectorValues, error) {
	v := &sparseOffHeapFloatVectorValues{}
	v.init(dimension, size, centroid, encoding, similarityFunction, vectorsScorer, slice)
	v.owner = v
	v.configuration = configuration
	v.dataIn = dataIn
	ordToDoc, err := configuration.GetDirectMonotonicReader(dataIn)
	if err != nil {
		return nil, err
	}
	v.ordToDoc = ordToDoc
	disi, err := configuration.GetIndexedDISI(dataIn)
	if err != nil {
		return nil, err
	}
	v.disi = disi
	return v, nil
}

// copySparse reproduces the covariant
// {@code public SparseOffHeapVectorValues copy()}.
func (v *sparseOffHeapFloatVectorValues) copySparse() (*sparseOffHeapFloatVectorValues, error) {
	var cloned store.IndexInput
	if v.slice != nil {
		cloned = v.slice.Clone()
	}
	return newSparseOffHeapFloatVectorValues(
		v.configuration, v.dimension, v.size, v.centroid, v.encoding, v.dataIn,
		v.similarityFunction, v.vectorsScorer, cloned)
}

// Copy carries KnnVectorValues.copy().
func (v *sparseOffHeapFloatVectorValues) Copy() (spi.KnnVectorValues, error) { return v.copySparse() }

// CopyFloatVectorValues carries the covariant FloatVectorValues.copy().
func (v *sparseOffHeapFloatVectorValues) CopyFloatVectorValues() (spi.FloatVectorValues, error) {
	return v.copySparse()
}

// OrdToDoc reproduces {@code return (int) ordToDoc.get(ord);}. Java's
// DirectMonotonicReader.get does not declare IOException; Gocene's does, and a
// failure is reported as the sentinel -1 because the Java signature has no
// error channel.
func (v *sparseOffHeapFloatVectorValues) OrdToDoc(ord int) int {
	doc, err := v.ordToDoc.Get(int64(ord))
	if err != nil {
		return -1
	}
	return int(doc)
}

// GetAcceptOrds reproduces SparseOffHeapVectorValues.getAcceptOrds(Bits).
func (v *sparseOffHeapFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &sparseOffHeapFloatAcceptOrds{values: v, acceptDocs: acceptDocs}
}

// sparseOffHeapFloatAcceptOrds is the anonymous Bits returned by
// SparseOffHeapVectorValues.getAcceptOrds.
type sparseOffHeapFloatAcceptOrds struct {
	values     *sparseOffHeapFloatVectorValues
	acceptDocs util.Bits
}

// Get reproduces {@code return acceptDocs.get(ordToDoc(index));}.
func (b *sparseOffHeapFloatAcceptOrds) Get(index int) bool {
	return b.acceptDocs.Get(b.values.OrdToDoc(index))
}

// Length reproduces {@code return size;}.
func (b *sparseOffHeapFloatAcceptOrds) Length() int { return b.values.size }

// Iterator reproduces {@code return IndexedDISI.asDocIndexIterator(disi);}.
func (v *sparseOffHeapFloatVectorValues) Iterator() spi.DocIndexIterator {
	return lucene90.AsDocIndexIterator(v.disi)
}

// Scorer reproduces SparseOffHeapVectorValues.scorer(float[]).
func (v *sparseOffHeapFloatVectorValues) Scorer(target []float32) (util.VectorScorer, error) {
	cp, err := v.copySparse()
	if err != nil {
		return nil, err
	}
	iterator := cp.Iterator()
	scorer, err := v.vectorsScorer.GetRandomVectorScorer(v.similarityFunction, cp, target)
	if err != nil {
		return nil, err
	}
	return &offHeapQuantizedVectorScorer{scorer: scorer, iterator: iterator, sparse: true}, nil
}

// ─── EmptyOffHeapVectorValues ───────────────────────────────────────────────

// emptyOffHeapFloatVectorValues are empty vector values.
//
// Mirrors the private nested class
// {@code static class OffHeapScalarQuantizedFloatVectorValues.EmptyOffHeapVectorValues}.
type emptyOffHeapFloatVectorValues struct {
	baseOffHeapScalarQuantizedFloatVectorValues
}

// newEmptyOffHeapFloatVectorValues reproduces the constructor, whose body is
//
//	super(dimension, 0, null, ScalarEncoding.UNSIGNED_BYTE, similarityFunction,
//	      vectorsScorer, null);
func newEmptyOffHeapFloatVectorValues(
	dimension int,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
) *emptyOffHeapFloatVectorValues {
	v := &emptyOffHeapFloatVectorValues{}
	v.init(dimension, 0, nil, quantization.ScalarEncodingUnsignedByte,
		similarityFunction, vectorsScorer, nil)
	v.owner = v
	return v
}

// Iterator reproduces {@code return createDenseIterator();}.
func (v *emptyOffHeapFloatVectorValues) Iterator() spi.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// Copy reproduces {@code throw new UnsupportedOperationException();}.
func (v *emptyOffHeapFloatVectorValues) Copy() (spi.KnnVectorValues, error) {
	return nil, errUnsupportedOperation
}

// CopyFloatVectorValues carries the same UnsupportedOperationException.
func (v *emptyOffHeapFloatVectorValues) CopyFloatVectorValues() (spi.FloatVectorValues, error) {
	return nil, errUnsupportedOperation
}

// GetAcceptOrds reproduces {@code return null;}.
func (v *emptyOffHeapFloatVectorValues) GetAcceptOrds(_ util.Bits) util.Bits { return nil }

// Scorer reproduces {@code return null;}.
func (v *emptyOffHeapFloatVectorValues) Scorer(_ []float32) (util.VectorScorer, error) {
	return nil, nil
}

var (
	_ offHeapScalarQuantizedFloatVectorValues = (*denseOffHeapFloatVectorValues)(nil)
	_ offHeapScalarQuantizedFloatVectorValues = (*sparseOffHeapFloatVectorValues)(nil)
	_ offHeapScalarQuantizedFloatVectorValues = (*emptyOffHeapFloatVectorValues)(nil)
)
