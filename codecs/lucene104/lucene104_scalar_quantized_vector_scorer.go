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

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/internal/vectorization"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// Lucene104ScalarQuantizedVectorScorer is a vector scorer over OptimizedScalarQuantized vectors.
// This is the Go port of org.apache.lucene.codecs.lucene104.Lucene104ScalarQuantizedVectorScorer
// (Lucene 10.4.0).
type Lucene104ScalarQuantizedVectorScorer struct {
	nonQuantizedDelegate codecs.FlatVectorsScorer
}

// NewLucene104ScalarQuantizedVectorScorer constructs a new Lucene104ScalarQuantizedVectorScorer.
func NewLucene104ScalarQuantizedVectorScorer(nonQuantizedDelegate codecs.FlatVectorsScorer) *Lucene104ScalarQuantizedVectorScorer {
	return &Lucene104ScalarQuantizedVectorScorer{
		nonQuantizedDelegate: nonQuantizedDelegate,
	}
}

// GetRandomVectorScorerSupplier returns a supplier that can create scorers.
func (s *Lucene104ScalarQuantizedVectorScorer) GetRandomVectorScorerSupplier(
	similarityFunction codecs.VectorSimilarityFunction,
	vectorValues codecs.KnnVectorValues,
) (codecs.FlatRandomVectorScorerSupplier, error) {
	if qv, ok := AsQuantizedByteVectorValues(vectorValues); ok {
		return newScalarQuantizedVectorScorerSupplier(qv, similarityFunction)
	}
	// It is possible to get to this branch during initial indexing and flush
	return s.nonQuantizedDelegate.GetRandomVectorScorerSupplier(similarityFunction, vectorValues)
}

// GetRandomVectorScorer returns a scorer for float vectors.
func (s *Lucene104ScalarQuantizedVectorScorer) GetRandomVectorScorer(
	similarityFunction codecs.VectorSimilarityFunction,
	vectorValues codecs.KnnVectorValues,
	target []float32,
) (codecs.FlatRandomVectorScorer, error) {
	if qv, ok := AsQuantizedByteVectorValues(vectorValues); ok {
		if len(target) != qv.Dimension() {
			return nil, fmt.Errorf("vector query dimension: %d differs from field dimension: %d", len(target), qv.Dimension())
		}

		quantizer, err := qv.GetScalarQuantizer()
		if err != nil {
			return nil, err
		}

		encoding := qv.GetEncoding()
		discreteDims := encoding.GetDiscreteDimensions(qv.Dimension())
		scratch := make([]byte, discreteDims)

		var targetQuantized []byte
		if !encoding.IsAsymmetric() {
			targetQuantized = scratch
		} else {
			// This is asymmetric quantization, we will pack the vector
			targetQuantized = make([]byte, encoding.GetQueryPackedLength(discreteDims))
		}

		// We make a copy as the quantization process mutates the input
		copyTarget := make([]float32, len(target))
		copy(copyTarget, target)
		if similarityFunction == codecs.VectorSimilarityFunctionCosine {
			util.L2Normalize(copyTarget)
		}

		targetCorrectiveTerms := quantizer.ScalarQuantize(
			copyTarget, scratch, byte(encoding.GetBits()), qv.GetCentroid(),
		)

		// for asymmetric encodings with 4-bit query, we need to transpose the nibbles for fast
		// scoring comparisons
		if encoding == quantization.ScalarEncodingSingleBitQueryNibble ||
			encoding == quantization.ScalarEncodingDibitQueryNibble {
			if err := quantization.TransposeHalfByte(scratch, targetQuantized); err != nil {
				return nil, err
			}
		}

		return &quantizedVectorScorer{
			qv:                    qv,
			targetQuantized:       targetQuantized,
			targetCorrectiveTerms: targetCorrectiveTerms,
			similarityFunction:    similarityFunction,
		}, nil
	}
	// It is possible to get to this branch during initial indexing and flush
	return s.nonQuantizedDelegate.GetRandomVectorScorer(similarityFunction, vectorValues, target)
}

// GetRandomVectorScorerByte returns a scorer for byte vectors.
func (s *Lucene104ScalarQuantizedVectorScorer) GetRandomVectorScorerByte(
	similarityFunction codecs.VectorSimilarityFunction,
	vectorValues codecs.KnnVectorValues,
	target []byte,
) (codecs.FlatRandomVectorScorer, error) {
	if vectorValues.Dimension() != len(target) {
		return nil, fmt.Errorf("vector query dimension: %d differs from field dimension: %d", len(target), vectorValues.Dimension())
	}
	return s.nonQuantizedDelegate.GetRandomVectorScorerByte(similarityFunction, vectorValues, target)
}

// GetAsymmetricQuantizedRandomVectorScorerSupplier returns a supplier for asymmetric quantization.
func (s *Lucene104ScalarQuantizedVectorScorer) GetAsymmetricQuantizedRandomVectorScorerSupplier(
	similarityFunction codecs.VectorSimilarityFunction,
	scoringVectors quantization.QuantizedByteVectorValues,
	targetVectors quantization.QuantizedByteVectorValues,
) codecs.FlatRandomVectorScorerSupplier {
	return &asymmetricQuantizedRandomVectorScorerSupplier{
		queryVectors:       targetVectors,
		targetVectors:      scoringVectors,
		similarityFunction: similarityFunction,
	}
}

func (s *Lucene104ScalarQuantizedVectorScorer) String() string {
	return fmt.Sprintf("Lucene104ScalarQuantizedVectorScorer(nonQuantizedDelegate=%v)", s.nonQuantizedDelegate)
}

type asymmetricQuantizedRandomVectorScorerSupplier struct {
	queryVectors       quantization.QuantizedByteVectorValues
	targetVectors      quantization.QuantizedByteVectorValues
	similarityFunction codecs.VectorSimilarityFunction
}

func (s *asymmetricQuantizedRandomVectorScorerSupplier) Scorer() (codecs.FlatRandomVectorScorer, error) {
	targetVals, err := s.targetVectors.Copy()
	if err != nil {
		return nil, err
	}
	queryVals, err := s.queryVectors.Copy()
	if err != nil {
		return nil, err
	}

	return &updateableQuantizedVectorScorer{
		targetVectors:      targetVals,
		queryVectors:       queryVals,
		similarityFunction: s.similarityFunction,
	}, nil
}

func (s *asymmetricQuantizedRandomVectorScorerSupplier) Copy() (codecs.FlatRandomVectorScorerSupplier, error) {
	qv, err := s.queryVectors.Copy()
	if err != nil {
		return nil, err
	}
	tv, err := s.targetVectors.Copy()
	if err != nil {
		return nil, err
	}
	return &asymmetricQuantizedRandomVectorScorerSupplier{
		queryVectors:       qv,
		targetVectors:      tv,
		similarityFunction: s.similarityFunction,
	}, nil
}

type scalarQuantizedVectorScorerSupplier struct {
	targetValues quantization.QuantizedByteVectorValues
	values       quantization.QuantizedByteVectorValues
	similarity   codecs.VectorSimilarityFunction
}

func newScalarQuantizedVectorScorerSupplier(values quantization.QuantizedByteVectorValues, similarity codecs.VectorSimilarityFunction) (codecs.FlatRandomVectorScorerSupplier, error) {
	if values.GetEncoding().IsAsymmetric() {
		return nil, fmt.Errorf("quantization: scalar quantized vector scorer supplier requires symmetric encoding")
	}
	targetVals, err := values.Copy()
	if err != nil {
		return nil, err
	}
	return &scalarQuantizedVectorScorerSupplier{
		targetValues: targetVals,
		values:       values,
		similarity:   similarity,
	}, nil
}

func (s *scalarQuantizedVectorScorerSupplier) Scorer() (codecs.FlatRandomVectorScorer, error) {
	return &updateableSymmetricQuantizedVectorScorer{
		targetValues:       s.targetValues,
		values:             s.values,
		similarityFunction: s.similarity,
	}, nil
}

func (s *scalarQuantizedVectorScorerSupplier) Copy() (codecs.FlatRandomVectorScorerSupplier, error) {
	vals, err := s.values.Copy()
	if err != nil {
		return nil, err
	}
	return newScalarQuantizedVectorScorerSupplier(vals, s.similarity)
}

type quantizedVectorScorer struct {
	qv                    quantization.QuantizedByteVectorValues
	targetQuantized       []byte
	targetCorrectiveTerms quantization.QuantizationResult
	similarityFunction    codecs.VectorSimilarityFunction
}

func (s *quantizedVectorScorer) Score(node int) (float32, error) {
	return quantizedScore(
		s.targetQuantized,
		s.targetCorrectiveTerms,
		s.qv,
		node,
		s.similarityFunction,
	)
}

func (s *quantizedVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	if numNodes == 0 {
		return float32(math.Inf(-1)), nil
	}
	maxScore := float32(math.Inf(-1))
	for i := 0; i < numNodes; i++ {
		score, err := s.Score(nodes[i])
		if err != nil {
			return 0, err
		}
		scores[i] = score
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore, nil
}

func (s *quantizedVectorScorer) MaxOrd() int {
	return s.qv.Size()
}

func (s *quantizedVectorScorer) SetScoringOrdinal(node int) error {
	return nil
}

type updateableQuantizedVectorScorer struct {
	targetVectors      quantization.QuantizedByteVectorValues
	queryVectors       quantization.QuantizedByteVectorValues
	similarityFunction codecs.VectorSimilarityFunction
	vector             []byte
	queryCorrections   *quantization.QuantizationResult
}

func (s *updateableQuantizedVectorScorer) Score(node int) (float32, error) {
	if s.vector == nil || s.queryCorrections == nil {
		return 0, fmt.Errorf("setScoringOrdinal was not called")
	}
	return quantizedScore(s.vector, *s.queryCorrections, s.targetVectors, node, s.similarityFunction)
}

func (s *updateableQuantizedVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	if numNodes == 0 {
		return float32(math.Inf(-1)), nil
	}
	maxScore := float32(math.Inf(-1))
	for i := 0; i < numNodes; i++ {
		score, err := s.Score(nodes[i])
		if err != nil {
			return 0, err
		}
		scores[i] = score
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore, nil
}

func (s *updateableQuantizedVectorScorer) MaxOrd() int {
	return s.targetVectors.Size()
}

func (s *updateableQuantizedVectorScorer) SetScoringOrdinal(node int) error {
	vec, err := s.queryVectors.VectorValue(node)
	if err != nil {
		return err
	}
	s.vector = vec
	corr, err := s.queryVectors.GetCorrectiveTerms(node)
	if err != nil {
		return err
	}
	s.queryCorrections = &corr
	return nil
}

type updateableSymmetricQuantizedVectorScorer struct {
	targetValues          quantization.QuantizedByteVectorValues
	values                quantization.QuantizedByteVectorValues
	similarityFunction    codecs.VectorSimilarityFunction
	targetVector          []byte
	targetCorrectiveTerms *quantization.QuantizationResult
}

func (s *updateableSymmetricQuantizedVectorScorer) Score(node int) (float32, error) {
	return quantizedScore(s.targetVector, *s.targetCorrectiveTerms, s.values, node, s.similarityFunction)
}

func (s *updateableSymmetricQuantizedVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	if numNodes == 0 {
		return float32(math.Inf(-1)), nil
	}
	maxScore := float32(math.Inf(-1))
	for i := 0; i < numNodes; i++ {
		score, err := s.Score(nodes[i])
		if err != nil {
			return 0, err
		}
		scores[i] = score
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore, nil
}

func (s *updateableSymmetricQuantizedVectorScorer) MaxOrd() int {
	return s.values.Size()
}

func (s *updateableSymmetricQuantizedVectorScorer) SetScoringOrdinal(node int) error {
	rawTargetVector, err := s.targetValues.VectorValue(node)
	if err != nil {
		return err
	}

	switch s.values.GetEncoding() {
	case quantization.ScalarEncodingUnsignedByte, quantization.ScalarEncodingSevenBit:
		s.targetVector = rawTargetVector
	case quantization.ScalarEncodingPackedNibble:
		if s.targetVector == nil {
			s.targetVector = make([]byte, quantization.Discretize(s.values.Dimension(), 2))
		}
		unpackNibblesPacked(rawTargetVector, s.targetVector)
	case quantization.ScalarEncodingSingleBitQueryNibble, quantization.ScalarEncodingDibitQueryNibble:
		return fmt.Errorf("%s encoding is not supported for symmetric quantization", s.values.GetEncoding())
	}

	corr, err := s.targetValues.GetCorrectiveTerms(node)
	if err != nil {
		return err
	}
	s.targetCorrectiveTerms = &corr
	return nil
}

func unpackNibblesPacked(packed, unpacked []byte) {
	n := len(packed)
	if len(unpacked) < n*2 {
		return
	}
	for i := 0; i < n; i++ {
		unpacked[i] = (packed[i] >> 4) & 0x0F
		unpacked[n+i] = packed[i] & 0x0F
	}
}

var scaleLUT = [8]float32{
	1.0,
	1.0 / ((1 << 2) - 1),
	1.0 / ((1 << 3) - 1),
	1.0 / ((1 << 4) - 1),
	1.0 / ((1 << 5) - 1),
	1.0 / ((1 << 6) - 1),
	1.0 / ((1 << 7) - 1),
	1.0 / ((1 << 8) - 1),
}

var queryBitsLUT = map[quantization.ScalarEncoding]int{
	quantization.ScalarEncodingUnsignedByte:         8,
	quantization.ScalarEncodingPackedNibble:         4,
	quantization.ScalarEncodingSevenBit:             7,
	quantization.ScalarEncodingSingleBitQueryNibble: 4,
	quantization.ScalarEncodingDibitQueryNibble:     4,
}

func quantizedScore(
	quantizedQuery []byte,
	queryCorrections quantization.QuantizationResult,
	targetVectors quantization.QuantizedByteVectorValues,
	targetOrd int,
	similarityFunction codecs.VectorSimilarityFunction,
) (float32, error) {
	scalarEncoding := targetVectors.GetEncoding()
	quantizedDoc, err := targetVectors.VectorValue(targetOrd)
	if err != nil {
		return 0, err
	}

	utilSupport := &vectorization.VectorUtilSupport{}
	var qcDist int32
	switch scalarEncoding {
	case quantization.ScalarEncodingUnsignedByte:
		qcDist = utilSupport.Uint8DotProduct(quantizedQuery, quantizedDoc)
	case quantization.ScalarEncodingSevenBit:
		qcDist = utilSupport.DotProductBytes(quantizedQuery, quantizedDoc)
	case quantization.ScalarEncodingPackedNibble:
		qcDist = utilSupport.Int4DotProductSinglePacked(quantizedQuery, quantizedDoc)
	case quantization.ScalarEncodingSingleBitQueryNibble:
		qcDist = int32(utilSupport.Int4BitDotProduct(quantizedQuery, quantizedDoc))
	case quantization.ScalarEncodingDibitQueryNibble:
		qcDist = int32(utilSupport.Int4DibitDotProduct(quantizedQuery, quantizedDoc))
	}

	indexCorrections, err := targetVectors.GetCorrectiveTerms(targetOrd)
	if err != nil {
		return 0, err
	}

	queryBits := queryBitsLUT[scalarEncoding]
	queryScale := scaleLUT[queryBits-1]
	scale := scaleLUT[scalarEncoding.GetBits()-1]

	ax := indexCorrections.LowerInterval
	lx := (indexCorrections.UpperInterval - ax) * scale
	ay := queryCorrections.LowerInterval
	ly := (queryCorrections.UpperInterval - ay) * queryScale
	y1 := queryCorrections.QuantizedComponentSum

	score := ax*ay*float32(targetVectors.Dimension()) + ay*lx*float32(indexCorrections.QuantizedComponentSum) + ax*ly*float32(y1) + lx*ly*float32(qcDist)

	if similarityFunction == codecs.VectorSimilarityFunctionEuclidean {
		score = queryCorrections.AdditionalCorrection + indexCorrections.AdditionalCorrection - 2*score
		return 1 / (1 + float32(math.Max(float64(score), 0))), nil
	} else {
		centroidDP := targetVectors.GetCentroidDP()
		score += queryCorrections.AdditionalCorrection + indexCorrections.AdditionalCorrection - centroidDP
		if similarityFunction == codecs.VectorSimilarityFunctionMaximumInnerProduct {
			return vectorization.ScaleMaxInnerProductScore(score), nil
		}
		score = float32(math.Max(-1, math.Min(1, float64(score))))
		return (1 + score) / 2, nil
	}
}
