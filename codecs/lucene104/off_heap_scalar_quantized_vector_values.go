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
//	lucene/core/src/java/org/apache/lucene/codecs/lucene104/OffHeapScalarQuantizedVectorValues.java

package lucene104

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene95"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// errUnsupportedOperation renders java.lang.UnsupportedOperationException,
// which EmptyOffHeapVectorValues.copy() throws.
var errUnsupportedOperation = errors.New("lucene104: unsupported operation")

// offHeapQuantizedCorrectiveValues is the length of the per-vector corrective
// float run, mirroring {@code this.correctiveValues = new float[3];}.
const offHeapQuantizedCorrectiveValues = 3

// OffHeapScalarQuantizedVectorValues are scalar quantized vector values loaded
// from off-heap.
//
// Mirrors {@code public abstract class OffHeapScalarQuantizedVectorValues
// extends QuantizedByteVectorValues} (Lucene 10.5.0, @lucene.internal). Go has
// no abstract classes, so the type is the interface the abstract class
// publishes; its shared state and concrete members live on
// [baseOffHeapScalarQuantizedVectorValues], which each concrete subtype
// embeds.
type OffHeapScalarQuantizedVectorValues interface {
	quantization.QuantizedByteVectorValues
}

// baseOffHeapScalarQuantizedVectorValues carries the fields and the concrete
// members of the abstract class OffHeapScalarQuantizedVectorValues.
//
// The owner field is the Go stand-in for the dynamic dispatch Java gets from
// {@code this}: the members the subclasses override (copy, getAcceptOrds,
// scorer, iterator, ordToDoc) are reached through it, the same indirection
// search.MultiTermQuery installs with SetOwner.
type baseOffHeapScalarQuantizedVectorValues struct {
	owner OffHeapScalarQuantizedVectorValues

	dimension          int
	size               int
	similarityFunction index.VectorSimilarityFunction
	vectorsScorer      hnsw.FlatVectorsScorer

	slice       store.IndexInput
	vectorValue []byte
	byteSize    int
	lastOrd     int

	correctiveValues      []float32
	quantizedComponentSum int
	quantizer             *quantization.OptimizedScalarQuantizer
	encoding              quantization.ScalarEncoding
	centroid              []float32
	centroidDp            float32
	isQuerySide           bool
}

// initBaseOffHeapScalarQuantizedVectorValues reproduces the package-private
// constructor
//
//	OffHeapScalarQuantizedVectorValues(boolean isQuerySide, int dimension,
//	    int size, float[] centroid, float centroidDp,
//	    OptimizedScalarQuantizer quantizer, ScalarEncoding encoding,
//	    VectorSimilarityFunction similarityFunction,
//	    FlatVectorsScorer vectorsScorer, IndexInput slice)
//
// The five-argument-shorter constructor of the Java class is
// {@code this(false, ...)}; callers spell the false out.
func (v *baseOffHeapScalarQuantizedVectorValues) init(
	isQuerySide bool,
	dimension int,
	size int,
	centroid []float32,
	centroidDp float32,
	quantizer *quantization.OptimizedScalarQuantizer,
	encoding quantization.ScalarEncoding,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) {
	v.isQuerySide = isQuerySide
	v.dimension = dimension
	v.size = size
	v.similarityFunction = similarityFunction
	v.vectorsScorer = vectorsScorer
	v.slice = slice
	v.centroid = centroid
	v.centroidDp = centroidDp
	v.correctiveValues = make([]float32, offHeapQuantizedCorrectiveValues)
	v.encoding = encoding
	docPackedLength := encoding.GetDocPackedLength(dimension)
	if isQuerySide {
		docPackedLength = encoding.GetQueryPackedLength(dimension)
	}
	v.byteSize = docPackedLength + (4 * 3) + 4
	// ByteBuffer.allocate(docPackedLength) and its backing array().
	v.vectorValue = make([]byte, docPackedLength)
	v.quantizer = quantizer
	v.lastOrd = -1
}

// Dimension reproduces {@code return dimension;}.
func (v *baseOffHeapScalarQuantizedVectorValues) Dimension() int { return v.dimension }

// Size reproduces {@code return size;}.
func (v *baseOffHeapScalarQuantizedVectorValues) Size() int { return v.size }

// VectorValue reproduces
//
//	if (lastOrd == targetOrd) return vectorValue;
//	slice.seek((long) targetOrd * byteSize);
//	slice.readBytes(byteBuffer.array(), byteBuffer.arrayOffset(), vectorValue.length);
//	slice.readFloats(correctiveValues, 0, 3);
//	quantizedComponentSum = slice.readInt();
//	lastOrd = targetOrd;
//	return vectorValue;
func (v *baseOffHeapScalarQuantizedVectorValues) VectorValue(targetOrd int) ([]byte, error) {
	if v.lastOrd == targetOrd {
		return v.vectorValue, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, err
	}
	if err := v.slice.ReadBytes(v.vectorValue, 0, len(v.vectorValue)); err != nil {
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
	v.lastOrd = targetOrd
	return v.vectorValue, nil
}

// GetSlice reproduces {@code return slice;}.
func (v *baseOffHeapScalarQuantizedVectorValues) GetSlice() store.IndexInput { return v.slice }

// GetCentroidDP reproduces {@code return centroidDp;}.
func (v *baseOffHeapScalarQuantizedVectorValues) GetCentroidDP() (float32, error) {
	return v.centroidDp, nil
}

// GetCorrectiveTerms reproduces
//
//	if (lastOrd == targetOrd) {
//	  return new OptimizedScalarQuantizer.QuantizationResult(
//	      correctiveValues[0], correctiveValues[1], correctiveValues[2], quantizedComponentSum);
//	}
//	slice.seek(((long) targetOrd * byteSize) + vectorValue.length);
//	slice.readFloats(correctiveValues, 0, 3);
//	quantizedComponentSum = slice.readInt();
//	return new OptimizedScalarQuantizer.QuantizationResult(...);
func (v *baseOffHeapScalarQuantizedVectorValues) GetCorrectiveTerms(targetOrd int) (quantization.QuantizationResult, error) {
	if v.lastOrd == targetOrd {
		return quantization.QuantizationResult{
			LowerInterval:         v.correctiveValues[0],
			UpperInterval:         v.correctiveValues[1],
			AdditionalCorrection:  v.correctiveValues[2],
			QuantizedComponentSum: v.quantizedComponentSum,
		}, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd)*int64(v.byteSize) + int64(len(v.vectorValue))); err != nil {
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

// GetQuantizer reproduces {@code return quantizer;}.
func (v *baseOffHeapScalarQuantizedVectorValues) GetQuantizer() *quantization.OptimizedScalarQuantizer {
	return v.quantizer
}

// GetScalarEncoding reproduces {@code return encoding;}.
func (v *baseOffHeapScalarQuantizedVectorValues) GetScalarEncoding() quantization.ScalarEncoding {
	return v.encoding
}

// GetCentroid reproduces {@code return centroid;}.
func (v *baseOffHeapScalarQuantizedVectorValues) GetCentroid() ([]float32, error) {
	return v.centroid, nil
}

// GetVectorByteLength reproduces {@code return vectorValue.length;}.
func (v *baseOffHeapScalarQuantizedVectorValues) GetVectorByteLength() int {
	return len(v.vectorValue)
}

// GetEncoding carries ByteVectorValues.getEncoding(), whose body is
// {@code return VectorEncoding.BYTE;}.
func (v *baseOffHeapScalarQuantizedVectorValues) GetEncoding() spi.VectorEncoding {
	return util.VectorEncodingByte
}

// OrdToDoc carries the KnnVectorValues default {@code return ord;}. The sparse
// subtype overrides it.
func (v *baseOffHeapScalarQuantizedVectorValues) OrdToDoc(ord int) int { return ord }

// Prefetch carries the KnnVectorValues default, whose body is empty.
func (v *baseOffHeapScalarQuantizedVectorValues) Prefetch(_ []int, _ int) error { return nil }

// Scorer carries ByteVectorValues.scorer(byte[]), whose default body throws
// UnsupportedOperationException.
func (v *baseOffHeapScalarQuantizedVectorValues) Scorer(_ []byte) (util.VectorScorer, error) {
	return nil, errUnsupportedOperation
}

// Rescorer carries ByteVectorValues.rescorer(byte[]), whose default body is
// {@code return scorer(target);}.
func (v *baseOffHeapScalarQuantizedVectorValues) Rescorer(target []byte) (util.VectorScorer, error) {
	return v.owner.Scorer(target)
}

// Copy carries KnnVectorValues.copy() through the owner, which is where the
// covariant override lives.
func (v *baseOffHeapScalarQuantizedVectorValues) Copy() (spi.KnnVectorValues, error) {
	return v.owner.Copy()
}

// CopyByteVectorValues carries the covariant ByteVectorValues.copy().
func (v *baseOffHeapScalarQuantizedVectorValues) CopyByteVectorValues() (spi.ByteVectorValues, error) {
	return v.owner.CopyByteVectorValues()
}

// CopyQuantizedByteVectorValues carries the covariant
// QuantizedByteVectorValues.copy().
func (v *baseOffHeapScalarQuantizedVectorValues) CopyQuantizedByteVectorValues() (quantization.QuantizedByteVectorValues, error) {
	return v.owner.CopyQuantizedByteVectorValues()
}

// ─── packNibbles / unpackNibbles ────────────────────────────────────────────

// packNibbles reproduces the package-private static
//
//	static void packNibbles(byte[] unpacked, byte[] packed) {
//	  assert unpacked.length == packed.length * 2;
//	  for (int i = 0; i < packed.length; i++) {
//	    int x = unpacked[i] << 4 | unpacked[packed.length + i];
//	    packed[i] = (byte) x;
//	  }
//	}
//
// Java's assert is a precondition of the writer's call sites; the Go rendering
// reports it as an error because Gocene has no assertion mechanism.
func packNibbles(unpacked, packed []byte) error {
	if len(unpacked) != len(packed)*2 {
		return fmt.Errorf("lucene104 sq: packNibbles: unpacked len %d != 2*packed len %d",
			len(unpacked), len(packed))
	}
	for i := 0; i < len(packed); i++ {
		x := int(unpacked[i])<<4 | int(unpacked[len(packed)+i])
		packed[i] = byte(x)
	}
	return nil
}

// unpackNibbles reproduces the package-private static
//
//	static void unpackNibbles(byte[] packed, byte[] unpacked) {
//	  assert unpacked.length == packed.length * 2;
//	  for (int i = 0; i < packed.length; i++) {
//	    unpacked[i] = (byte) ((packed[i] >> 4) & 0x0F);
//	    unpacked[packed.length + i] = (byte) (packed[i] & 0x0F);
//	  }
//	}
func unpackNibbles(packed, unpacked []byte) {
	for i := 0; i < len(packed); i++ {
		unpacked[i] = (packed[i] >> 4) & 0x0F
		unpacked[len(packed)+i] = packed[i] & 0x0F
	}
}

// ─── load ───────────────────────────────────────────────────────────────────

// loadOffHeapScalarQuantizedVectorValues reproduces the package-private static
//
//	static OffHeapScalarQuantizedVectorValues load(
//	    OrdToDocDISIReaderConfiguration configuration, int dimension, int size,
//	    OptimizedScalarQuantizer quantizer, ScalarEncoding encoding,
//	    VectorSimilarityFunction similarityFunction, FlatVectorsScorer vectorsScorer,
//	    float[] centroid, float centroidDp, long quantizedVectorDataOffset,
//	    long quantizedVectorDataLength, IndexInput vectorData)
func loadOffHeapScalarQuantizedVectorValues(
	configuration *lucene95.OrdToDocDISIReaderConfiguration,
	dimension int,
	size int,
	quantizer *quantization.OptimizedScalarQuantizer,
	encoding quantization.ScalarEncoding,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	centroid []float32,
	centroidDp float32,
	quantizedVectorDataOffset int64,
	quantizedVectorDataLength int64,
	vectorData store.IndexInput,
) (OffHeapScalarQuantizedVectorValues, error) {
	if configuration.IsEmpty() {
		return newEmptyOffHeapVectorValues(dimension, similarityFunction, vectorsScorer), nil
	}
	bytesSlice, err := vectorData.Slice(
		"quantized-vector-data", quantizedVectorDataOffset, quantizedVectorDataLength)
	if err != nil {
		return nil, err
	}
	if configuration.IsDense() {
		return newDenseOffHeapVectorValues(
			dimension, size, centroid, centroidDp, quantizer, encoding,
			similarityFunction, vectorsScorer, bytesSlice), nil
	}
	return newSparseOffHeapVectorValues(
		configuration, dimension, size, centroid, centroidDp, quantizer, encoding,
		vectorData, similarityFunction, vectorsScorer, bytesSlice)
}

// ─── DenseOffHeapVectorValues ───────────────────────────────────────────────

// denseOffHeapVectorValues are dense off-heap scalar quantized vector values.
//
// Mirrors the package-private nested class
// {@code static class OffHeapScalarQuantizedVectorValues.DenseOffHeapVectorValues}.
type denseOffHeapVectorValues struct {
	baseOffHeapScalarQuantizedVectorValues
}

// newDenseOffHeapVectorValues reproduces the nine-argument constructor, whose
// body is {@code super(dimension, size, ..., slice);} — the isQuerySide=false
// form.
func newDenseOffHeapVectorValues(
	dimension int,
	size int,
	centroid []float32,
	centroidDp float32,
	quantizer *quantization.OptimizedScalarQuantizer,
	encoding quantization.ScalarEncoding,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) *denseOffHeapVectorValues {
	return newDenseOffHeapVectorValuesQuerySide(
		false, dimension, size, centroid, centroidDp, quantizer, encoding,
		similarityFunction, vectorsScorer, slice)
}

// newDenseOffHeapVectorValuesQuerySide reproduces the ten-argument constructor
// that carries the isQuerySide flag.
func newDenseOffHeapVectorValuesQuerySide(
	isQuerySide bool,
	dimension int,
	size int,
	centroid []float32,
	centroidDp float32,
	quantizer *quantization.OptimizedScalarQuantizer,
	encoding quantization.ScalarEncoding,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) *denseOffHeapVectorValues {
	v := &denseOffHeapVectorValues{}
	v.init(isQuerySide, dimension, size, centroid, centroidDp, quantizer, encoding,
		similarityFunction, vectorsScorer, slice)
	v.owner = v
	return v
}

// copyDense reproduces the covariant
// {@code public DenseOffHeapVectorValues copy()}.
func (v *denseOffHeapVectorValues) copyDense() (*denseOffHeapVectorValues, error) {
	var cloned store.IndexInput
	if v.slice != nil {
		cloned = v.slice.Clone()
	}
	return newDenseOffHeapVectorValuesQuerySide(
		v.isQuerySide, v.dimension, v.size, v.centroid, v.centroidDp, v.quantizer,
		v.encoding, v.similarityFunction, v.vectorsScorer, cloned), nil
}

// Copy carries KnnVectorValues.copy().
func (v *denseOffHeapVectorValues) Copy() (spi.KnnVectorValues, error) { return v.copyDense() }

// CopyByteVectorValues carries ByteVectorValues.copy().
func (v *denseOffHeapVectorValues) CopyByteVectorValues() (spi.ByteVectorValues, error) {
	return v.copyDense()
}

// CopyQuantizedByteVectorValues carries QuantizedByteVectorValues.copy().
func (v *denseOffHeapVectorValues) CopyQuantizedByteVectorValues() (quantization.QuantizedByteVectorValues, error) {
	return v.copyDense()
}

// GetAcceptOrds reproduces {@code return acceptDocs;}.
func (v *denseOffHeapVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return acceptDocs
}

// Iterator reproduces {@code return createDenseIterator();}.
func (v *denseOffHeapVectorValues) Iterator() spi.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// ScorerFloat reproduces
//
//	DenseOffHeapVectorValues copy = copy();
//	DocIndexIterator iterator = copy.iterator();
//	RandomVectorScorer scorer = vectorsScorer.getRandomVectorScorer(similarityFunction, copy, target);
//	return new VectorScorer() { score() -> scorer.score(iterator.index());
//	                            iterator() -> iterator;
//	                            bulk(m) -> Bulk.fromRandomScorerDense(scorer, iterator, m); };
//
// It carries QuantizedByteVectorValues.scorer(float[]); the Java assert
// isQuerySide == false is a precondition of the call sites.
func (v *denseOffHeapVectorValues) ScorerFloat(target []float32) (util.VectorScorer, error) {
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

// sparseOffHeapVectorValues are sparse off-heap scalar quantized vector
// values.
//
// Mirrors the private nested class
// {@code static class OffHeapScalarQuantizedVectorValues.SparseOffHeapVectorValues}.
type sparseOffHeapVectorValues struct {
	baseOffHeapScalarQuantizedVectorValues

	ordToDoc *packed.DirectMonotonicReader
	disi     *lucene90.IndexedDISI
	// dataIn was used to init a new IndexedDIS for #randomAccess()
	dataIn        store.IndexInput
	configuration *lucene95.OrdToDocDISIReaderConfiguration
}

// newSparseOffHeapVectorValues reproduces the constructor, whose body is the
// isQuerySide=false super call followed by
//
//	this.configuration = configuration;
//	this.dataIn = dataIn;
//	this.ordToDoc = configuration.getDirectMonotonicReader(dataIn);
//	this.disi = configuration.getIndexedDISI(dataIn);
func newSparseOffHeapVectorValues(
	configuration *lucene95.OrdToDocDISIReaderConfiguration,
	dimension int,
	size int,
	centroid []float32,
	centroidDp float32,
	quantizer *quantization.OptimizedScalarQuantizer,
	encoding quantization.ScalarEncoding,
	dataIn store.IndexInput,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) (*sparseOffHeapVectorValues, error) {
	v := &sparseOffHeapVectorValues{}
	v.init(false, dimension, size, centroid, centroidDp, quantizer, encoding,
		similarityFunction, vectorsScorer, slice)
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
func (v *sparseOffHeapVectorValues) copySparse() (*sparseOffHeapVectorValues, error) {
	var cloned store.IndexInput
	if v.slice != nil {
		cloned = v.slice.Clone()
	}
	return newSparseOffHeapVectorValues(
		v.configuration, v.dimension, v.size, v.centroid, v.centroidDp, v.quantizer,
		v.encoding, v.dataIn, v.similarityFunction, v.vectorsScorer, cloned)
}

// Copy carries KnnVectorValues.copy().
func (v *sparseOffHeapVectorValues) Copy() (spi.KnnVectorValues, error) { return v.copySparse() }

// CopyByteVectorValues carries ByteVectorValues.copy().
func (v *sparseOffHeapVectorValues) CopyByteVectorValues() (spi.ByteVectorValues, error) {
	return v.copySparse()
}

// CopyQuantizedByteVectorValues carries QuantizedByteVectorValues.copy().
func (v *sparseOffHeapVectorValues) CopyQuantizedByteVectorValues() (quantization.QuantizedByteVectorValues, error) {
	return v.copySparse()
}

// OrdToDoc reproduces {@code return (int) ordToDoc.get(ord);}. Java's
// DirectMonotonicReader.get does not declare IOException; Gocene's does, and a
// failure is reported as the sentinel -1 because the Java signature has no
// error channel.
func (v *sparseOffHeapVectorValues) OrdToDoc(ord int) int {
	doc, err := v.ordToDoc.Get(int64(ord))
	if err != nil {
		return -1
	}
	return int(doc)
}

// GetAcceptOrds reproduces
//
//	if (acceptDocs == null) return null;
//	return new Bits() {
//	  public boolean get(int index) { return acceptDocs.get(ordToDoc(index)); }
//	  public int length() { return size; }
//	};
func (v *sparseOffHeapVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &sparseOffHeapAcceptOrds{values: v, acceptDocs: acceptDocs}
}

// sparseOffHeapAcceptOrds is the anonymous Bits returned by
// SparseOffHeapVectorValues.getAcceptOrds.
type sparseOffHeapAcceptOrds struct {
	values     *sparseOffHeapVectorValues
	acceptDocs util.Bits
}

// Get reproduces {@code return acceptDocs.get(ordToDoc(index));}.
func (b *sparseOffHeapAcceptOrds) Get(index int) bool {
	return b.acceptDocs.Get(b.values.OrdToDoc(index))
}

// Length reproduces {@code return size;}.
func (b *sparseOffHeapAcceptOrds) Length() int { return b.values.size }

// Iterator reproduces {@code return IndexedDISI.asDocIndexIterator(disi);}.
func (v *sparseOffHeapVectorValues) Iterator() spi.DocIndexIterator {
	return lucene90.AsDocIndexIterator(v.disi)
}

// ScorerFloat reproduces the SparseOffHeapVectorValues body of
// {@code public VectorScorer scorer(float[] target)}, which differs from the
// dense one only in using Bulk.fromRandomScorerSparse.
func (v *sparseOffHeapVectorValues) ScorerFloat(target []float32) (util.VectorScorer, error) {
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

// emptyOffHeapVectorValues mirrors the private nested class
// {@code static class OffHeapScalarQuantizedVectorValues.EmptyOffHeapVectorValues}.
type emptyOffHeapVectorValues struct {
	baseOffHeapScalarQuantizedVectorValues
}

// newEmptyOffHeapVectorValues reproduces the constructor, whose body is
//
//	super(dimension, 0, null, Float.NaN, null, ScalarEncoding.UNSIGNED_BYTE,
//	      similarityFunction, vectorsScorer, null);
func newEmptyOffHeapVectorValues(
	dimension int,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
) *emptyOffHeapVectorValues {
	v := &emptyOffHeapVectorValues{}
	v.init(false, dimension, 0, nil, float32(math.NaN()), nil,
		quantization.ScalarEncodingUnsignedByte, similarityFunction, vectorsScorer, nil)
	v.owner = v
	return v
}

// Iterator reproduces {@code return createDenseIterator();}.
func (v *emptyOffHeapVectorValues) Iterator() spi.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// Copy reproduces {@code throw new UnsupportedOperationException();}.
func (v *emptyOffHeapVectorValues) Copy() (spi.KnnVectorValues, error) {
	return nil, errUnsupportedOperation
}

// CopyByteVectorValues carries the same UnsupportedOperationException.
func (v *emptyOffHeapVectorValues) CopyByteVectorValues() (spi.ByteVectorValues, error) {
	return nil, errUnsupportedOperation
}

// CopyQuantizedByteVectorValues carries the same UnsupportedOperationException.
func (v *emptyOffHeapVectorValues) CopyQuantizedByteVectorValues() (quantization.QuantizedByteVectorValues, error) {
	return nil, errUnsupportedOperation
}

// GetAcceptOrds reproduces {@code return null;}.
func (v *emptyOffHeapVectorValues) GetAcceptOrds(_ util.Bits) util.Bits { return nil }

// ScorerFloat reproduces {@code return null;}.
func (v *emptyOffHeapVectorValues) ScorerFloat(_ []float32) (util.VectorScorer, error) {
	return nil, nil
}

// ─── the anonymous VectorScorer of scorer(float[]) ──────────────────────────

// offHeapQuantizedVectorScorer is the anonymous VectorScorer returned by
// DenseOffHeapVectorValues.scorer(float[]) and
// SparseOffHeapVectorValues.scorer(float[]); the two differ only in which
// VectorScorer.Bulk factory bulk(DocIdSetIterator) calls.
type offHeapQuantizedVectorScorer struct {
	scorer   utilhnsw.RandomVectorScorer
	iterator spi.DocIndexIterator
	sparse   bool
}

// Score reproduces {@code return scorer.score(iterator.index());}.
func (s *offHeapQuantizedVectorScorer) Score() (float32, error) {
	return s.scorer.Score(s.iterator.Index())
}

// Iterator reproduces {@code return iterator;}.
func (s *offHeapQuantizedVectorScorer) Iterator() util.DocIdSetIterator { return s.iterator }

// Bulk reproduces
//
//	return Bulk.fromRandomScorerDense(scorer, iterator, matchingDocs);
//
// for the dense values, and fromRandomScorerSparse for the sparse ones.
func (s *offHeapQuantizedVectorScorer) Bulk(matchingDocs util.DocIdSetIterator) (util.Bulk, error) {
	if s.sparse {
		return search.BulkFromRandomScorerSparse(s.scorer, s.iterator, matchingDocs), nil
	}
	return search.BulkFromRandomScorerDense(s.scorer, s.iterator, matchingDocs), nil
}

var (
	_ OffHeapScalarQuantizedVectorValues = (*denseOffHeapVectorValues)(nil)
	_ OffHeapScalarQuantizedVectorValues = (*sparseOffHeapVectorValues)(nil)
	_ OffHeapScalarQuantizedVectorValues = (*emptyOffHeapVectorValues)(nil)
	_ util.VectorScorer                  = (*offHeapQuantizedVectorScorer)(nil)
)
