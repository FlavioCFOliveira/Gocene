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
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// fourBitScale mirrors FOUR_BIT_SCALE, the scaling factor for 4-bit
// quantization: 1f / ((1 << 4) - 1).
const fourBitScale = float32(1) / float32((1<<4)-1)

// Lucene102BinaryFlatVectorsScorer is the Go port of
// org.apache.lucene.backward_codecs.lucene102.Lucene102BinaryFlatVectorsScorer
// (Apache Lucene 10.5.0): a vector scorer over binarized vector values.
type Lucene102BinaryFlatVectorsScorer struct {
	// nonQuantizedDelegate is the delegate scorer for non-quantized vectors.
	nonQuantizedDelegate hnsw.FlatVectorsScorer
}

// NewLucene102BinaryFlatVectorsScorer constructs a new scorer. Mirrors
// Lucene102BinaryFlatVectorsScorer(FlatVectorsScorer).
func NewLucene102BinaryFlatVectorsScorer(nonQuantizedDelegate hnsw.FlatVectorsScorer) *Lucene102BinaryFlatVectorsScorer {
	return &Lucene102BinaryFlatVectorsScorer{nonQuantizedDelegate: nonQuantizedDelegate}
}

// GetRandomVectorScorerSupplier mirrors
// getRandomVectorScorerSupplier(VectorSimilarityFunction, KnnVectorValues),
// which delegates to the non-quantized scorer.
func (s *Lucene102BinaryFlatVectorsScorer) GetRandomVectorScorerSupplier(
	similarityFunction index.VectorSimilarityFunction, vectorValues utilhnsw.KnnVectorValues,
) (utilhnsw.RandomVectorScorerSupplier, error) {
	return s.nonQuantizedDelegate.GetRandomVectorScorerSupplier(similarityFunction, vectorValues)
}

// GetRandomVectorScorer mirrors getRandomVectorScorer(VectorSimilarityFunction,
// KnnVectorValues, float[]): a query against binarized vectors is quantized
// to four bits against the centroid and transposed; any other values are
// scored by the non-quantized scorer.
func (s *Lucene102BinaryFlatVectorsScorer) GetRandomVectorScorer(
	similarityFunction index.VectorSimilarityFunction, vectorValues utilhnsw.KnnVectorValues, target []float32,
) (utilhnsw.RandomVectorScorer, error) {
	if binarizedVectors, ok := vectorValues.(BinarizedByteVectorValues); ok {
		quantizer := binarizedVectors.GetQuantizer()
		centroid, err := binarizedVectors.GetCentroid()
		if err != nil {
			return nil, err
		}
		// We make a copy as the quantization process mutates the input
		copied := util.CopyOfSubArrayGeneric(target, 0, len(target))
		if similarityFunction == index.VectorSimilarityFunctionCosine {
			util.L2Normalize(copied)
		}
		target = copied
		initial := make([]byte, len(target))
		quantized := make([]byte, int(lucene102QueryBits)*binarizedVectors.DiscretizedDimensions()/8)
		queryCorrections := quantizer.ScalarQuantize(target, initial, 4, centroid)
		quantization.TransposeHalfByte(initial, quantized)
		return &binarizedQueryVectorScorer{
			AbstractRandomVectorScorer: utilhnsw.NewAbstractRandomVectorScorer(binarizedVectors),
			quantized:                  quantized,
			queryCorrections:           queryCorrections,
			binarizedVectors:           binarizedVectors,
			similarityFunction:         similarityFunction,
		}, nil
	}
	return s.nonQuantizedDelegate.GetRandomVectorScorer(similarityFunction, vectorValues, target)
}

// GetRandomVectorScorerByte mirrors getRandomVectorScorer(VectorSimilarityFunction,
// KnnVectorValues, byte[]), which delegates to the non-quantized scorer.
func (s *Lucene102BinaryFlatVectorsScorer) GetRandomVectorScorerByte(
	similarityFunction index.VectorSimilarityFunction, vectorValues utilhnsw.KnnVectorValues, target []byte,
) (utilhnsw.RandomVectorScorer, error) {
	return s.nonQuantizedDelegate.GetRandomVectorScorerByte(similarityFunction, vectorValues, target)
}

// String mirrors toString().
func (s *Lucene102BinaryFlatVectorsScorer) String() string {
	return fmt.Sprintf("Lucene102BinaryFlatVectorsScorer(nonQuantizedDelegate=%v)", s.nonQuantizedDelegate)
}

// getBinarizedRandomVectorScorerSupplier mirrors the package-private overload
// getRandomVectorScorerSupplier(VectorSimilarityFunction,
// OffHeapBinarizedQueryVectorValues, BinarizedByteVectorValues).
func (s *Lucene102BinaryFlatVectorsScorer) getBinarizedRandomVectorScorerSupplier(
	similarityFunction index.VectorSimilarityFunction,
	scoringVectors *offHeapBinarizedQueryVectorValues,
	targetVectors BinarizedByteVectorValues,
) utilhnsw.RandomVectorScorerSupplier {
	return &binarizedRandomVectorScorerSupplier{
		queryVectors:       scoringVectors,
		targetVectors:      targetVectors,
		similarityFunction: similarityFunction,
	}
}

// binarizedQueryVectorScorer is the anonymous
// RandomVectorScorer.AbstractRandomVectorScorer returned by
// getRandomVectorScorer(VectorSimilarityFunction, KnnVectorValues, float[]).
type binarizedQueryVectorScorer struct {
	*utilhnsw.AbstractRandomVectorScorer
	quantized          []byte
	queryCorrections   quantization.QuantizationResult
	binarizedVectors   BinarizedByteVectorValues
	similarityFunction index.VectorSimilarityFunction
}

// Score scores the quantized query against the vector at node.
func (s *binarizedQueryVectorScorer) Score(node int) (float32, error) {
	return quantizedScore(s.quantized, s.queryCorrections, s.binarizedVectors, node, s.similarityFunction)
}

// BulkScore carries the RandomVectorScorer.bulkScore default.
func (s *binarizedQueryVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// binarizedRandomVectorScorerSupplier is the Go port of the package-private
// static nested class BinarizedRandomVectorScorerSupplier: a vector scorer
// supplier over binarized vector values.
type binarizedRandomVectorScorerSupplier struct {
	queryVectors       *offHeapBinarizedQueryVectorValues
	targetVectors      BinarizedByteVectorValues
	similarityFunction index.VectorSimilarityFunction
}

// Scorer mirrors BinarizedRandomVectorScorerSupplier.scorer().
func (s *binarizedRandomVectorScorerSupplier) Scorer() (utilhnsw.UpdateableRandomVectorScorer, error) {
	targetVectors, err := s.targetVectors.CopyBinarizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	queryVectors := s.queryVectors.copy()
	return &binarizedUpdateableVectorScorer{
		AbstractUpdateableRandomVectorScorer: utilhnsw.NewAbstractUpdateableRandomVectorScorer(targetVectors),
		targetVectors:                        targetVectors,
		queryVectors:                         queryVectors,
		similarityFunction:                   s.similarityFunction,
	}, nil
}

// Copy mirrors BinarizedRandomVectorScorerSupplier.copy().
func (s *binarizedRandomVectorScorerSupplier) Copy() (utilhnsw.RandomVectorScorerSupplier, error) {
	queryVectors := s.queryVectors.copy()
	targetVectors, err := s.targetVectors.CopyBinarizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	return &binarizedRandomVectorScorerSupplier{
		queryVectors:       queryVectors,
		targetVectors:      targetVectors,
		similarityFunction: s.similarityFunction,
	}, nil
}

// binarizedUpdateableVectorScorer is the anonymous
// UpdateableRandomVectorScorer.AbstractUpdateableRandomVectorScorer returned
// by BinarizedRandomVectorScorerSupplier.scorer().
type binarizedUpdateableVectorScorer struct {
	*utilhnsw.AbstractUpdateableRandomVectorScorer
	targetVectors      BinarizedByteVectorValues
	queryVectors       *offHeapBinarizedQueryVectorValues
	similarityFunction index.VectorSimilarityFunction

	queryCorrections *quantization.QuantizationResult
	vector           []byte
}

// SetScoringOrdinal loads the corrective terms and the quantized query of
// node.
func (s *binarizedUpdateableVectorScorer) SetScoringOrdinal(node int) error {
	queryCorrections, err := s.queryVectors.getCorrectiveTerms(node)
	if err != nil {
		return err
	}
	s.queryCorrections = &queryCorrections
	vector, err := s.queryVectors.vectorValue(node)
	if err != nil {
		return err
	}
	s.vector = vector
	return nil
}

// Score scores the loaded query against the target vector at node. The Java
// IllegalStateException for a scorer whose ordinal was never set is returned
// as an error.
func (s *binarizedUpdateableVectorScorer) Score(node int) (float32, error) {
	if s.vector == nil || s.queryCorrections == nil {
		return 0, errors.New("setScoringOrdinal was not called")
	}
	return quantizedScore(s.vector, *s.queryCorrections, s.targetVectors, node, s.similarityFunction)
}

// BulkScore carries the RandomVectorScorer.bulkScore default.
func (s *binarizedUpdateableVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// quantizedScore mirrors the package-private static quantizedScore: the
// asymmetric estimate of the similarity between a 4-bit query and a single-bit
// target vector, corrected by both vectors' corrective terms. The products
// are converted to float32 explicitly so that no fused multiply-add changes
// the IEEE float arithmetic Java performs.
func quantizedScore(
	quantizedQuery []byte,
	queryCorrections quantization.QuantizationResult,
	targetVectors BinarizedByteVectorValues,
	targetOrd int,
	similarityFunction index.VectorSimilarityFunction,
) (float32, error) {
	binaryCode, err := targetVectors.VectorValue(targetOrd)
	if err != nil {
		return 0, err
	}
	qcDist := float32(util.Int4BitDotProduct(quantizedQuery, binaryCode))
	indexCorrections, err := targetVectors.GetCorrectiveTerms(targetOrd)
	if err != nil {
		return 0, err
	}
	x1 := float32(indexCorrections.QuantizedComponentSum)
	ax := indexCorrections.LowerInterval
	// Here we assume `lx` is simply bit vectors, so the scaling isn't necessary
	lx := indexCorrections.UpperInterval - ax
	ay := queryCorrections.LowerInterval
	ly := float32((queryCorrections.UpperInterval - ay) * fourBitScale)
	y1 := float32(queryCorrections.QuantizedComponentSum)
	score := float32(ax*ay*float32(targetVectors.Dimension())) +
		float32(ay*lx*x1) +
		float32(ax*ly*y1) +
		float32(lx*ly*qcDist)
	// For euclidean, we need to invert the score and apply the additional correction, which is
	// assumed to be the squared l2norm of the centroid centered vectors.
	if similarityFunction == index.VectorSimilarityFunctionEuclidean {
		score = queryCorrections.AdditionalCorrection +
			indexCorrections.AdditionalCorrection -
			float32(2*score)
		return max(1/(1+score), 0), nil
	}
	// For cosine and max inner product, we need to apply the additional correction, which is
	// assumed to be the non-centered dot-product between the vector and the centroid
	centroidDP, err := targetVectors.GetCentroidDP()
	if err != nil {
		return 0, err
	}
	score += queryCorrections.AdditionalCorrection + indexCorrections.AdditionalCorrection - centroidDP
	if similarityFunction == index.VectorSimilarityFunctionMaximumInnerProduct {
		return util.ScaleMaxInnerProductScore(score), nil
	}
	return max((1+score)/2, 0), nil
}

var _ hnsw.FlatVectorsScorer = (*Lucene102BinaryFlatVectorsScorer)(nil)
