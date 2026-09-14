// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene95"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// offHeapBinarizedVectorValues is the base implementation for binarized vector
// values loaded from off-heap.
//
// Port of org.apache.lucene.backward_codecs.lucene102.OffHeapBinarizedVectorValues.
type offHeapBinarizedVectorValues struct {
	dimension             int
	size                  int
	numBytes              int
	similarityFunction    index.VectorSimilarityFunction
	vectorsScorer         hnsw.FlatVectorsScorer
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

func newOffHeapBinarizedVectorValues(
	dimension int,
	size int,
	centroid []float32,
	centroidDp float32,
	quantizer *quantization.OptimizedScalarQuantizer,
	similarityFunction index.VectorSimilarityFunction,
	vectorsScorer hnsw.FlatVectorsScorer,
	slice store.IndexInput,
) *offHeapBinarizedVectorValues {
	discretized := quantization.Discretize(dimension, 64)
	numBytes := discretized / 8
	byteSize := numBytes + (4 * 3) + 2 // 3 floats + 1 short

	return &offHeapBinarizedVectorValues{
		dimension:             dimension,
		size:                  size,
		similarityFunction:    similarityFunction,
		vectorsScorer:         vectorsScorer,
		slice:                 slice,
		centroid:              centroid,
		centroidDp:            centroidDp,
		numBytes:              numBytes,
		correctiveValues:      make([]float32, 3),
		byteSize:              byteSize,
		binaryValue:           make([]byte, numBytes),
		binaryQuantizer:       quantizer,
		discretizedDimensions: discretized,
		lastOrd:               -1,
	}
}

func (v *offHeapBinarizedVectorValues) Dimension() int {
	return v.dimension
}

func (v *offHeapBinarizedVectorValues) Size() int {
	return v.size
}

func (v *offHeapBinarizedVectorValues) VectorValue(targetOrd int) ([]byte, error) {
	if v.lastOrd == targetOrd {
		return v.binaryValue, nil
	}

	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, err
	}

	if err := v.slice.ReadBytes(v.binaryValue, 0, len(v.binaryValue)); err != nil {
		return nil, err
	}

	for i := 0; i < 3; i++ {
		val, err := v.slice.ReadInt()
		if err != nil {
			return nil, err
		}
		v.correctiveValues[i] = math.Float32frombits(uint32(val))
	}

	short, err := v.slice.ReadShort()
	if err != nil {
		return nil, err
	}
	v.quantizedComponentSum = int(uint16(short))
	v.lastOrd = targetOrd

	return v.binaryValue, nil
}

func (v *offHeapBinarizedVectorValues) GetCorrectiveTerms(targetOrd int) (quantization.QuantizationResult, error) {
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

	for i := 0; i < 3; i++ {
		val, err := v.slice.ReadInt()
		if err != nil {
			return quantization.QuantizationResult{}, err
		}
		v.correctiveValues[i] = math.Float32frombits(uint32(val))
	}

	short, err := v.slice.ReadShort()
	if err != nil {
		return quantization.QuantizationResult{}, err
	}
	v.quantizedComponentSum = int(uint16(short))

	return quantization.QuantizationResult{
		LowerInterval:         v.correctiveValues[0],
		UpperInterval:         v.correctiveValues[1],
		AdditionalCorrection:  v.correctiveValues[2],
		QuantizedComponentSum: v.quantizedComponentSum,
	}, nil
}

func (v *offHeapBinarizedVectorValues) GetQuantizer() *quantization.OptimizedScalarQuantizer {
	return v.binaryQuantizer
}

func (v *offHeapBinarizedVectorValues) GetCentroid() ([]float32, error) {
	return v.centroid, nil
}

func (v *offHeapBinarizedVectorValues) GetVectorByteLength() int {
	return v.numBytes
}

// Load creates an off-heap binarized vector values reader.
func Load(
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
		return &DenseOffHeapVectorValues{
			offHeapBinarizedVectorValues: newOffHeapBinarizedVectorValues(
				dimension,
				size,
				centroid,
				centroidDp,
				binaryQuantizer,
				similarityFunction,
				vectorsScorer,
				bytesSlice,
			),
		}, nil
	}

	// Java: this.ordToDoc = configuration.getDirectMonotonicReader(dataIn);
	//       this.disi = configuration.getIndexedDISI(dataIn);
	ordToDoc, err := configuration.GetDirectMonotonicReader(vectorData)
	if err != nil {
		return nil, err
	}
	disi, err := configuration.GetIndexedDISI(vectorData)
	if err != nil {
		return nil, err
	}
	return &SparseOffHeapVectorValues{
		offHeapBinarizedVectorValues: newOffHeapBinarizedVectorValues(
			dimension,
			size,
			centroid,
			centroidDp,
			binaryQuantizer,
			similarityFunction,
			vectorsScorer,
			bytesSlice,
		),
		configuration: configuration,
		dataIn:        vectorData,
		ordToDoc:      ordToDoc,
		disi:          disi,
	}, nil
}

type DenseOffHeapVectorValues struct {
	*offHeapBinarizedVectorValues
}

func (v *DenseOffHeapVectorValues) Copy() (BinarizedByteVectorValues, error) {
	clonedSlice, err := v.slice.Clone()
	if err != nil {
		return nil, err
	}
	if clonedSlice == nil {
		return nil, fmt.Errorf("failed to clone index input")
	}

	return &DenseOffHeapVectorValues{
		offHeapBinarizedVectorValues: newOffHeapBinarizedVectorValues(
			v.dimension,
			v.size,
			v.centroid,
			v.centroidDp,
			v.binaryQuantizer,
			v.similarityFunction,
			v.vectorsScorer,
			clonedSlice,
		),
	}, nil
}

func (v *DenseOffHeapVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return acceptDocs
}

func (v *DenseOffHeapVectorValues) Scorer(target []float32) (search.VectorScorer, error) {
	copyVal, err := v.Copy()
	if err != nil {
		return nil, err
	}
	denseCopy := copyVal.(*DenseOffHeapVectorValues)
	iterator := denseCopy.Iterator()

	scorer, err := v.vectorsScorer.GetRandomVectorScorer(v.similarityFunction, denseCopy, target)
	if err != nil {
		return nil, err
	}

	return &vectorScorerWrapper{
		scorer:   scorer,
		iterator: iterator,
	}, nil
}

func (v *DenseOffHeapVectorValues) Iterator() search.DocIndexIterator {
	return &denseDocIndexIterator{
		size:  v.size,
		index: -1,
	}
}

type denseDocIndexIterator struct {
	size  int
	index int
}

func (it *denseDocIndexIterator) Next() int {
	it.index++
	if it.index >= it.size {
		return search.NO_MORE_DOCS
	}
	return it.index
}

func (it *denseDocIndexIterator) Index() int {
	return it.index
}

func (it *denseDocIndexIterator) Cost() int64 {
	return 1
}

type vectorScorerWrapper struct {
	scorer   utilhnsw.RandomVectorScorer
	iterator search.DocIndexIterator
}

func (s *vectorScorerWrapper) Score() (float32, error) {
	return s.scorer.Score(s.iterator.Index())
}

func (s *vectorScorerWrapper) Iterator() util.DocIdSetIterator {
	return s.iterator
}

type SparseOffHeapVectorValues struct {
	*offHeapBinarizedVectorValues
	ordToDoc      *packed.DirectMonotonicReader
	disi          *lucene90.IndexedDISI
	dataIn        store.IndexInput
	configuration *lucene95.OrdToDocDISIReaderConfiguration
}

func (v *SparseOffHeapVectorValues) Copy() (BinarizedByteVectorValues, error) {
	clonedSlice, err := v.slice.Clone()
	if err != nil {
		return nil, err
	}

	return &SparseOffHeapVectorValues{
		offHeapBinarizedVectorValues: newOffHeapBinarizedVectorValues(
			v.dimension,
			v.size,
			v.centroid,
			v.centroidDp,
			v.binaryQuantizer,
			v.similarityFunction,
			v.vectorsScorer,
			clonedSlice,
		),
		configuration: v.configuration,
		dataIn:        v.dataIn,
		ordToDoc:      v.ordToDoc,
		disi:          v.disi,
	}, nil
}

func (v *SparseOffHeapVectorValues) OrdToDoc(ord int) int {
	return int(v.ordToDoc.Get(ord))
}

func (v *SparseOffHeapVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &sparseAcceptOrds{
		acceptDocs: acceptDocs,
		v:          v,
	}
}

type sparseAcceptOrds struct {
	acceptDocs util.Bits
	v          *SparseOffHeapVectorValues
}

func (s *sparseAcceptOrds) Get(index int) bool {
	return s.acceptDocs.Get(s.v.OrdToDoc(index))
}

func (s *sparseAcceptOrds) Length() int {
	return s.v.size
}

func (v *SparseOffHeapVectorValues) Iterator() search.DocIndexIterator {
	return &indexedDISIAdapter{disi: v.disi}
}

type indexedDISIAdapter struct {
	disi *lucene90.IndexedDISI
}

func (a *indexedDISIAdapter) Next() int {
	doc, err := a.disi.NextDoc()
	if err != nil {
		return search.NO_MORE_DOCS
	}
	return doc
}

func (a *indexedDISIAdapter) Index() int {
	return a.disi.Index()
}

func (a *indexedDISIAdapter) Cost() int64 {
	return a.disi.Cost()
}

func (v *SparseOffHeapVectorValues) Scorer(target []float32) (search.VectorScorer, error) {
	copyVal, err := v.Copy()
	if err != nil {
		return nil, err
	}
	sparseCopy := copyVal.(*SparseOffHeapVectorValues)
	iterator := sparseCopy.Iterator()

	scorer, err := v.vectorsScorer.GetRandomVectorScorer(v.similarityFunction, sparseCopy, target)
	if err != nil {
		return nil, err
	}

	return &vectorScorerWrapper{
		scorer:   scorer,
		iterator: iterator,
	}, nil
}

type emptyOffHeapVectorValues struct {
	*offHeapBinarizedVectorValues
}

func newEmptyOffHeapVectorValues(dimension int, similarityFunction index.VectorSimilarityFunction, vectorsScorer hnsw.FlatVectorsScorer) *emptyOffHeapVectorValues {
	return &emptyOffHeapVectorValues{
		offHeapBinarizedVectorValues: &offHeapBinarizedVectorValues{
			dimension:          dimension,
			size:               0,
			similarityFunction: similarityFunction,
			vectorsScorer:      vectorsScorer,
			centroidDp:         float32(math.NaN()),
		},
	}
}

func (v *emptyOffHeapVectorValues) Iterator() search.DocIndexIterator {
	return &denseDocIndexIterator{
		size:  0,
		index: -1,
	}
}

func (v *emptyOffHeapVectorValues) Copy() (BinarizedByteVectorValues, error) {
	return nil, fmt.Errorf("unsupported operation")
}

func (v *emptyOffHeapVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return nil
}

func (v *emptyOffHeapVectorValues) Scorer(target []float32) (search.VectorScorer, error) {
	return nil, nil
}
