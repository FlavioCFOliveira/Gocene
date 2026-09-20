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
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lucene104

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/internal/vectorization"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// Lucene104ScalarQuantizedVectorScorer is a vector scorer over
// OptimizedScalarQuantized vectors. It is the Go port of
// org.apache.lucene.codecs.lucene104.Lucene104ScalarQuantizedVectorScorer
// (Apache Lucene 10.5.0).
type Lucene104ScalarQuantizedVectorScorer struct {
	nonQuantizedDelegate hnsw.FlatVectorsScorer
}

// Lucene104ScalarQuantizedVectorScorer implements the codecs.hnsw contract.
var _ hnsw.FlatVectorsScorer = (*Lucene104ScalarQuantizedVectorScorer)(nil)

// NewLucene104ScalarQuantizedVectorScorer constructs a scorer that falls back
// to nonQuantizedDelegate for values that are not quantized. Mirrors
// Lucene104ScalarQuantizedVectorScorer(FlatVectorsScorer).
func NewLucene104ScalarQuantizedVectorScorer(nonQuantizedDelegate hnsw.FlatVectorsScorer) *Lucene104ScalarQuantizedVectorScorer {
	return &Lucene104ScalarQuantizedVectorScorer{
		nonQuantizedDelegate: nonQuantizedDelegate,
	}
}

// GetRandomVectorScorerSupplier mirrors
// getRandomVectorScorerSupplier(VectorSimilarityFunction, KnnVectorValues).
func (s *Lucene104ScalarQuantizedVectorScorer) GetRandomVectorScorerSupplier(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues spi.KnnVectorValues,
) (utilhnsw.RandomVectorScorerSupplier, error) {
	if qv, ok := vectorValues.(quantization.QuantizedByteVectorValues); ok {
		return newScalarQuantizedVectorScorerSupplier(qv, similarityFunction)
	}
	// It is possible to get to this branch during initial indexing and flush
	return s.nonQuantizedDelegate.GetRandomVectorScorerSupplier(similarityFunction, vectorValues)
}

// GetRandomVectorScorer mirrors the float[] overload of
// getRandomVectorScorer(VectorSimilarityFunction, KnnVectorValues, float[]).
func (s *Lucene104ScalarQuantizedVectorScorer) GetRandomVectorScorer(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues spi.KnnVectorValues,
	target []float32,
) (utilhnsw.RandomVectorScorer, error) {
	qv, ok := vectorValues.(quantization.QuantizedByteVectorValues)
	if !ok {
		// It is possible to get to this branch during initial indexing and flush
		return s.nonQuantizedDelegate.GetRandomVectorScorer(similarityFunction, vectorValues, target)
	}
	if err := hnsw.CheckDimensions(len(target), qv.Dimension()); err != nil {
		return nil, err
	}
	quantizer := qv.GetQuantizer()
	scalarEncoding := qv.GetScalarEncoding()
	scratch := make([]byte, scalarEncoding.GetDiscreteDimensions(qv.Dimension()))
	var targetQuantized []byte
	if !scalarEncoding.IsAsymmetric() {
		targetQuantized = scratch
	} else {
		// This is asymmetric quantization, we will pack the vector
		targetQuantized = make([]byte, scalarEncoding.GetQueryPackedLength(len(scratch)))
	}
	// We make a copy as the quantization process mutates the input
	cp := make([]float32, len(target))
	copy(cp, target)
	if similarityFunction == index.VectorSimilarityFunctionCosine {
		util.L2Normalize(cp)
	}
	target = cp
	centroid, err := qv.GetCentroid()
	if err != nil {
		return nil, err
	}
	targetCorrectiveTerms := quantizer.ScalarQuantize(
		target, scratch, scalarEncoding.GetQueryBits(), centroid)
	// for asymmetric encodings with 4-bit query, we need to transpose the
	// nibbles for fast scoring comparisons
	if scalarEncoding == quantization.ScalarEncodingSingleBitQueryNibble ||
		scalarEncoding == quantization.ScalarEncodingDibitQueryNibble {
		quantization.TransposeHalfByte(scratch, targetQuantized)
	}
	return &quantizedVectorScorer{
		AbstractRandomVectorScorer: utilhnsw.NewAbstractRandomVectorScorer(qv),
		values:                     qv,
		targetQuantized:            targetQuantized,
		targetCorrectiveTerms:      targetCorrectiveTerms,
		similarityFunction:         similarityFunction,
	}, nil
}

// GetRandomVectorScorerByte mirrors the byte[] overload of
// getRandomVectorScorer(VectorSimilarityFunction, KnnVectorValues, byte[]).
func (s *Lucene104ScalarQuantizedVectorScorer) GetRandomVectorScorerByte(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues spi.KnnVectorValues,
	target []byte,
) (utilhnsw.RandomVectorScorer, error) {
	if err := hnsw.CheckDimensions(len(target), vectorValues.Dimension()); err != nil {
		return nil, err
	}
	return s.nonQuantizedDelegate.GetRandomVectorScorerByte(similarityFunction, vectorValues, target)
}

// GetAsymmetricQuantizedRandomVectorScorerSupplier renders the public
// getRandomVectorScorerSupplier(VectorSimilarityFunction,
// QuantizedByteVectorValues, QuantizedByteVectorValues) overload; Go has no
// method overloading, so the quantized overload carries a distinct name.
//
// The Java body asserts targetVectors.getScalarEncoding().isAsymmetric().
func (s *Lucene104ScalarQuantizedVectorScorer) GetAsymmetricQuantizedRandomVectorScorerSupplier(
	similarityFunction index.VectorSimilarityFunction,
	scoringVectors quantization.QuantizedByteVectorValues,
	targetVectors quantization.QuantizedByteVectorValues,
) utilhnsw.RandomVectorScorerSupplier {
	return &asymmetricQuantizedRandomVectorScorerSupplier{
		queryVectors:       scoringVectors,
		targetVectors:      targetVectors,
		similarityFunction: similarityFunction,
	}
}

// String mirrors toString().
func (s *Lucene104ScalarQuantizedVectorScorer) String() string {
	return fmt.Sprintf("Lucene104ScalarQuantizedVectorScorer(nonQuantizedDelegate=%v)", s.nonQuantizedDelegate)
}

// asymmetricQuantizedRandomVectorScorerSupplier is the Go port of the nested
// static class
// Lucene104ScalarQuantizedVectorScorer.AsymmetricQuantizedRandomVectorScorerSupplier.
type asymmetricQuantizedRandomVectorScorerSupplier struct {
	queryVectors       quantization.QuantizedByteVectorValues
	targetVectors      quantization.QuantizedByteVectorValues
	similarityFunction index.VectorSimilarityFunction
}

// Scorer mirrors scorer(): it copies both sides so the returned scorer owns
// independent iterators.
func (s *asymmetricQuantizedRandomVectorScorerSupplier) Scorer() (utilhnsw.UpdateableRandomVectorScorer, error) {
	targetVectors, err := s.targetVectors.CopyQuantizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	queryVectors, err := s.queryVectors.CopyQuantizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	return &updateableQuantizedVectorScorer{
		AbstractUpdateableRandomVectorScorer: utilhnsw.NewAbstractUpdateableRandomVectorScorer(targetVectors),
		targetVectors:                        targetVectors,
		queryVectors:                         queryVectors,
		similarityFunction:                   s.similarityFunction,
	}, nil
}

// Copy mirrors copy().
func (s *asymmetricQuantizedRandomVectorScorerSupplier) Copy() (utilhnsw.RandomVectorScorerSupplier, error) {
	queryVectors, err := s.queryVectors.CopyQuantizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	targetVectors, err := s.targetVectors.CopyQuantizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	return &asymmetricQuantizedRandomVectorScorerSupplier{
		queryVectors:       queryVectors,
		targetVectors:      targetVectors,
		similarityFunction: s.similarityFunction,
	}, nil
}

// scalarQuantizedVectorScorerSupplier is the Go port of the nested private
// static final class
// Lucene104ScalarQuantizedVectorScorer.ScalarQuantizedVectorScorerSupplier.
type scalarQuantizedVectorScorerSupplier struct {
	targetValues quantization.QuantizedByteVectorValues
	values       quantization.QuantizedByteVectorValues
	similarity   index.VectorSimilarityFunction
}

// newScalarQuantizedVectorScorerSupplier mirrors the Java constructor, whose
// first statement asserts values.getScalarEncoding().isAsymmetric() == false.
func newScalarQuantizedVectorScorerSupplier(
	values quantization.QuantizedByteVectorValues,
	similarity index.VectorSimilarityFunction,
) (utilhnsw.RandomVectorScorerSupplier, error) {
	if values.GetScalarEncoding().IsAsymmetric() {
		return nil, fmt.Errorf("quantization: scalar quantized vector scorer supplier requires symmetric encoding")
	}
	targetValues, err := values.CopyQuantizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	return &scalarQuantizedVectorScorerSupplier{
		targetValues: targetValues,
		values:       values,
		similarity:   similarity,
	}, nil
}

// Scorer mirrors scorer().
func (s *scalarQuantizedVectorScorerSupplier) Scorer() (utilhnsw.UpdateableRandomVectorScorer, error) {
	return &updateableSymmetricQuantizedVectorScorer{
		AbstractUpdateableRandomVectorScorer: utilhnsw.NewAbstractUpdateableRandomVectorScorer(s.values),
		targetValues:                         s.targetValues,
		values:                               s.values,
		similarityFunction:                   s.similarity,
	}, nil
}

// Copy mirrors copy().
func (s *scalarQuantizedVectorScorerSupplier) Copy() (utilhnsw.RandomVectorScorerSupplier, error) {
	values, err := s.values.CopyQuantizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	return newScalarQuantizedVectorScorerSupplier(values, s.similarity)
}

// quantizedVectorScorer is the anonymous
// RandomVectorScorer.AbstractRandomVectorScorer returned by the float[]
// overload of getRandomVectorScorer.
type quantizedVectorScorer struct {
	*utilhnsw.AbstractRandomVectorScorer
	values                quantization.QuantizedByteVectorValues
	targetQuantized       []byte
	targetCorrectiveTerms quantization.QuantizationResult
	similarityFunction    index.VectorSimilarityFunction
}

// Score mirrors the anonymous class's score(int).
func (s *quantizedVectorScorer) Score(node int) (float32, error) {
	return quantizedScore(
		s.targetQuantized,
		s.targetCorrectiveTerms,
		s.values,
		node,
		s.similarityFunction,
	)
}

// BulkScore carries the RandomVectorScorer.bulkScore default body.
func (s *quantizedVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// updateableQuantizedVectorScorer is the anonymous
// UpdateableRandomVectorScorer.AbstractUpdateableRandomVectorScorer returned
// by AsymmetricQuantizedRandomVectorScorerSupplier.scorer().
type updateableQuantizedVectorScorer struct {
	*utilhnsw.AbstractUpdateableRandomVectorScorer
	targetVectors      quantization.QuantizedByteVectorValues
	queryVectors       quantization.QuantizedByteVectorValues
	similarityFunction index.VectorSimilarityFunction
	vector             []byte
	queryCorrections   *quantization.QuantizationResult
}

// Score mirrors score(int); it reports the Java IllegalStateException when
// setScoringOrdinal has not been called.
func (s *updateableQuantizedVectorScorer) Score(node int) (float32, error) {
	if s.vector == nil || s.queryCorrections == nil {
		return 0, fmt.Errorf("setScoringOrdinal was not called")
	}
	return quantizedScore(s.vector, *s.queryCorrections, s.targetVectors, node, s.similarityFunction)
}

// BulkScore carries the RandomVectorScorer.bulkScore default body.
func (s *updateableQuantizedVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// SetScoringOrdinal mirrors setScoringOrdinal(int).
func (s *updateableQuantizedVectorScorer) SetScoringOrdinal(node int) error {
	vector, err := s.queryVectors.VectorValue(node)
	if err != nil {
		return err
	}
	queryCorrections, err := s.queryVectors.GetCorrectiveTerms(node)
	if err != nil {
		return err
	}
	s.vector = vector
	s.queryCorrections = &queryCorrections
	return nil
}

// updateableSymmetricQuantizedVectorScorer is the anonymous
// UpdateableRandomVectorScorer.AbstractUpdateableRandomVectorScorer returned
// by ScalarQuantizedVectorScorerSupplier.scorer().
type updateableSymmetricQuantizedVectorScorer struct {
	*utilhnsw.AbstractUpdateableRandomVectorScorer
	targetValues          quantization.QuantizedByteVectorValues
	values                quantization.QuantizedByteVectorValues
	similarityFunction    index.VectorSimilarityFunction
	targetVector          []byte
	targetCorrectiveTerms quantization.QuantizationResult
}

// Score mirrors score(int).
func (s *updateableSymmetricQuantizedVectorScorer) Score(node int) (float32, error) {
	return quantizedScore(s.targetVector, s.targetCorrectiveTerms, s.values, node, s.similarityFunction)
}

// BulkScore carries the RandomVectorScorer.bulkScore default body.
func (s *updateableSymmetricQuantizedVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// SetScoringOrdinal mirrors setScoringOrdinal(int), including the
// IllegalStateException raised for the asymmetric encodings.
func (s *updateableSymmetricQuantizedVectorScorer) SetScoringOrdinal(node int) error {
	rawTargetVector, err := s.targetValues.VectorValue(node)
	if err != nil {
		return err
	}
	switch s.values.GetScalarEncoding() {
	case quantization.ScalarEncodingUnsignedByte, quantization.ScalarEncodingSevenBit:
		s.targetVector = rawTargetVector
	case quantization.ScalarEncodingPackedNibble:
		if s.targetVector == nil {
			s.targetVector = make([]byte, quantization.Discretize(s.values.Dimension(), 2))
		}
		unpackNibbles(rawTargetVector, s.targetVector)
	case quantization.ScalarEncodingSingleBitQueryNibble, quantization.ScalarEncodingDibitQueryNibble:
		return fmt.Errorf("%s encoding is not supported for symmetric quantization",
			s.values.GetScalarEncoding())
	}
	targetCorrectiveTerms, err := s.targetValues.GetCorrectiveTerms(node)
	if err != nil {
		return err
	}
	s.targetCorrectiveTerms = targetCorrectiveTerms
	return nil
}

// scaleLUT mirrors the private static final float[] SCALE_LUT.
var scaleLUT = [8]float32{
	1,
	1.0 / ((1 << 2) - 1),
	1.0 / ((1 << 3) - 1),
	1.0 / ((1 << 4) - 1),
	1.0 / ((1 << 5) - 1),
	1.0 / ((1 << 6) - 1),
	1.0 / ((1 << 7) - 1),
	1.0 / ((1 << 8) - 1),
}

// quantizedScore mirrors the private static
// quantizedScore(byte[], QuantizationResult, QuantizedByteVectorValues, int,
// VectorSimilarityFunction).
func quantizedScore(
	quantizedQuery []byte,
	queryCorrections quantization.QuantizationResult,
	targetVectors quantization.QuantizedByteVectorValues,
	targetOrd int,
	similarityFunction index.VectorSimilarityFunction,
) (float32, error) {
	scalarEncoding := targetVectors.GetScalarEncoding()
	quantizedDoc, err := targetVectors.VectorValue(targetOrd)
	if err != nil {
		return 0, err
	}
	impl := &vectorization.VectorUtilSupport{}
	var qcDist float32
	switch scalarEncoding {
	case quantization.ScalarEncodingUnsignedByte:
		qcDist = float32(impl.Uint8DotProduct(quantizedQuery, quantizedDoc))
	case quantization.ScalarEncodingSevenBit:
		qcDist = float32(impl.DotProductBytes(quantizedQuery, quantizedDoc))
	case quantization.ScalarEncodingPackedNibble:
		qcDist = float32(impl.Int4DotProductSinglePacked(quantizedQuery, quantizedDoc))
	case quantization.ScalarEncodingSingleBitQueryNibble:
		qcDist = float32(impl.Int4BitDotProduct(quantizedQuery, quantizedDoc))
	case quantization.ScalarEncodingDibitQueryNibble:
		qcDist = float32(impl.Int4DibitDotProduct(quantizedQuery, quantizedDoc))
	}
	indexCorrections, err := targetVectors.GetCorrectiveTerms(targetOrd)
	if err != nil {
		return 0, err
	}
	queryScale := scaleLUT[scalarEncoding.GetQueryBits()-1]
	scale := scaleLUT[scalarEncoding.GetBits()-1]
	x1 := float32(indexCorrections.QuantizedComponentSum)
	ax := indexCorrections.LowerInterval
	// Here we must scale according to the bits
	lx := (indexCorrections.UpperInterval - ax) * scale
	ay := queryCorrections.LowerInterval
	ly := (queryCorrections.UpperInterval - ay) * queryScale
	y1 := float32(queryCorrections.QuantizedComponentSum)
	score := ax*ay*float32(targetVectors.Dimension()) + ay*lx*x1 + ax*ly*y1 + lx*ly*qcDist
	// For euclidean, we need to invert the score and apply the additional
	// correction, which is assumed to be the squared l2norm of the centroid
	// centered vectors.
	if similarityFunction == index.VectorSimilarityFunctionEuclidean {
		score = queryCorrections.AdditionalCorrection + indexCorrections.AdditionalCorrection - 2*score
		// Ensure that 'score' (the squared euclidean distance) is
		// non-negative. The computed value may be negative as a result of
		// quantization loss.
		return 1 / (1 + float32(math.Max(float64(score), 0))), nil
	}
	// For cosine and max inner product, we need to apply the additional
	// correction, which is assumed to be the non-centered dot-product between
	// the vector and the centroid
	centroidDP, err := targetVectors.GetCentroidDP()
	if err != nil {
		return 0, err
	}
	score += queryCorrections.AdditionalCorrection + indexCorrections.AdditionalCorrection - centroidDP
	if similarityFunction == index.VectorSimilarityFunctionMaximumInnerProduct {
		return util.ScaleMaxInnerProductScore(score), nil
	}
	// Ensure that 'score' (a normalized dot product) is in [-1,1]. The
	// computed value may be out of bounds as a result of quantization loss.
	score = min(max(score, -1), 1)
	return (1 + score) / 2, nil
}
