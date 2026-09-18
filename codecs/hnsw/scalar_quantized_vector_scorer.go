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

package hnsw

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// ScalarQuantizedVectorScorer is the Go port of
// org.apache.lucene.codecs.hnsw.ScalarQuantizedVectorScorer (Lucene
// 10.5.0), the default scalar quantized implementation of
// [FlatVectorsScorer]. Scoring requests against
// [quantization.LegacyQuantizedByteVectorValues] are routed through the
// scalar-quantized similarity; requests against any other vector values are
// delegated to the wrapped non-quantized scorer ("it is possible to get to
// this branch during initial indexing and flush").
type ScalarQuantizedVectorScorer struct {
	nonQuantizedDelegate FlatVectorsScorer
}

// NewScalarQuantizedVectorScorer mirrors
// ScalarQuantizedVectorScorer(FlatVectorsScorer).
func NewScalarQuantizedVectorScorer(flatVectorsScorer FlatVectorsScorer) *ScalarQuantizedVectorScorer {
	return &ScalarQuantizedVectorScorer{nonQuantizedDelegate: flatVectorsScorer}
}

// String returns the canonical Java toString() output.
func (s *ScalarQuantizedVectorScorer) String() string {
	return fmt.Sprintf("ScalarQuantizedVectorScorer(nonQuantizedDelegate=%v)", s.nonQuantizedDelegate)
}

// GetRandomVectorScorerSupplier mirrors
// getRandomVectorScorerSupplier(VectorSimilarityFunction, KnnVectorValues).
func (s *ScalarQuantizedVectorScorer) GetRandomVectorScorerSupplier(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues hnsw.KnnVectorValues,
) (hnsw.RandomVectorScorerSupplier, error) {
	if quantizedByteVectorValues, ok := vectorValues.(quantization.LegacyQuantizedByteVectorValues); ok {
		return NewScalarQuantizedRandomVectorScorerSupplier(
			similarityFunction,
			quantizedByteVectorValues.GetScalarQuantizer(),
			quantizedByteVectorValues,
		)
	}
	// It is possible to get to this branch during initial indexing and flush
	return s.nonQuantizedDelegate.GetRandomVectorScorerSupplier(similarityFunction, vectorValues)
}

// GetRandomVectorScorer mirrors the float[] overload of
// getRandomVectorScorer: the target is quantized with the values' scalar
// quantizer and scored against the stored quantized vectors.
func (s *ScalarQuantizedVectorScorer) GetRandomVectorScorer(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues hnsw.KnnVectorValues,
	target []float32,
) (hnsw.RandomVectorScorer, error) {
	if quantizedByteVectorValues, ok := vectorValues.(quantization.LegacyQuantizedByteVectorValues); ok {
		scalarQuantizer := quantizedByteVectorValues.GetScalarQuantizer()
		targetBytes := make([]byte, len(target))
		offsetCorrection, err := QuantizeQuery(target, targetBytes, similarityFunction, scalarQuantizer)
		if err != nil {
			return nil, err
		}
		scalarQuantizedVectorSimilarity, err := quantization.FromVectorSimilarity(
			similarityFunction,
			scalarQuantizer.GetConstantMultiplier(),
			scalarQuantizer.GetBits(),
		)
		if err != nil {
			return nil, err
		}
		return &scalarQuantizedFloatScorer{
			AbstractRandomVectorScorer: hnsw.NewAbstractRandomVectorScorer(quantizedByteVectorValues),
			values:                     quantizedByteVectorValues,
			similarity:                 scalarQuantizedVectorSimilarity,
			targetBytes:                targetBytes,
			offsetCorrection:           offsetCorrection,
		}, nil
	}
	// It is possible to get to this branch during initial indexing and flush
	return s.nonQuantizedDelegate.GetRandomVectorScorer(similarityFunction, vectorValues, target)
}

// GetRandomVectorScorerByte mirrors the byte[] overload of
// getRandomVectorScorer, which always delegates to the non-quantized scorer.
func (s *ScalarQuantizedVectorScorer) GetRandomVectorScorerByte(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues hnsw.KnnVectorValues,
	target []byte,
) (hnsw.RandomVectorScorer, error) {
	return s.nonQuantizedDelegate.GetRandomVectorScorerByte(similarityFunction, vectorValues, target)
}

// QuantizeQuery is the Go port of the static
// ScalarQuantizedVectorScorer.quantizeQuery. For COSINE the query is copied
// (ArrayUtil.copyArray) and l2-normalized before quantization; the other
// similarities quantize the query as is. Returns the offset correction the
// caller adds when scoring against quantized vectors.
func QuantizeQuery(
	query []float32,
	quantizedQuery []byte,
	similarityFunction index.VectorSimilarityFunction,
	scalarQuantizer *quantization.ScalarQuantizer,
) (float32, error) {
	if len(query) != len(quantizedQuery) {
		return 0, fmt.Errorf(
			"QuantizeQuery: query/quantizedQuery length mismatch: %d != %d",
			len(query), len(quantizedQuery),
		)
	}
	processed := query
	if similarityFunction == index.VectorSimilarityFunctionCosine {
		copied := make([]float32, len(query))
		copy(copied, query)
		util.L2Normalize(copied)
		processed = copied
	}
	return scalarQuantizer.Quantize(processed, quantizedQuery, similarityFunction), nil
}

// scalarQuantizedFloatScorer is the anonymous
// RandomVectorScorer.AbstractRandomVectorScorer returned by the float[]
// overload of getRandomVectorScorer.
type scalarQuantizedFloatScorer struct {
	*hnsw.AbstractRandomVectorScorer
	values           quantization.LegacyQuantizedByteVectorValues
	similarity       quantization.ScalarQuantizedVectorSimilarity
	targetBytes      []byte
	offsetCorrection float32
}

// Score scores the quantized target against the stored vector at node,
// applying the stored score correction constant.
func (s *scalarQuantizedFloatScorer) Score(node int) (float32, error) {
	nodeVector, err := s.values.VectorValue(node)
	if err != nil {
		return 0, err
	}
	nodeOffset, err := s.values.GetScoreCorrectionConstant(node)
	if err != nil {
		return 0, err
	}
	return s.similarity.Score(s.targetBytes, s.offsetCorrection, nodeVector, nodeOffset), nil
}

// BulkScore carries the RandomVectorScorer.bulkScore default.
func (s *scalarQuantizedFloatScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return hnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// ScalarQuantizedRandomVectorScorerSupplier is the Go port of the public
// nested class
// ScalarQuantizedVectorScorer.ScalarQuantizedRandomVectorScorerSupplier.
type ScalarQuantizedRandomVectorScorerSupplier struct {
	values                   quantization.LegacyQuantizedByteVectorValues
	similarity               quantization.ScalarQuantizedVectorSimilarity
	vectorSimilarityFunction index.VectorSimilarityFunction
}

// NewScalarQuantizedRandomVectorScorerSupplier mirrors the public
// constructor ScalarQuantizedRandomVectorScorerSupplier(
// VectorSimilarityFunction, ScalarQuantizer, LegacyQuantizedByteVectorValues).
func NewScalarQuantizedRandomVectorScorerSupplier(
	similarityFunction index.VectorSimilarityFunction,
	scalarQuantizer *quantization.ScalarQuantizer,
	values quantization.LegacyQuantizedByteVectorValues,
) (*ScalarQuantizedRandomVectorScorerSupplier, error) {
	similarity, err := quantization.FromVectorSimilarity(
		similarityFunction,
		scalarQuantizer.GetConstantMultiplier(),
		scalarQuantizer.GetBits(),
	)
	if err != nil {
		return nil, err
	}
	return &ScalarQuantizedRandomVectorScorerSupplier{
		values:                   values,
		similarity:               similarity,
		vectorSimilarityFunction: similarityFunction,
	}, nil
}

// newScalarQuantizedRandomVectorScorerSupplierShallow mirrors the private
// constructor used by copy(), which reuses the similarity.
func newScalarQuantizedRandomVectorScorerSupplierShallow(
	similarity quantization.ScalarQuantizedVectorSimilarity,
	vectorSimilarityFunction index.VectorSimilarityFunction,
	values quantization.LegacyQuantizedByteVectorValues,
) *ScalarQuantizedRandomVectorScorerSupplier {
	return &ScalarQuantizedRandomVectorScorerSupplier{
		values:                   values,
		similarity:               similarity,
		vectorSimilarityFunction: vectorSimilarityFunction,
	}
}

// Scorer mirrors scorer(): the scorer reads a copy of the values and owns
// its query vector buffer.
func (s *ScalarQuantizedRandomVectorScorerSupplier) Scorer() (hnsw.UpdateableRandomVectorScorer, error) {
	vectorsCopy, err := s.values.CopyLegacyQuantizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	queryVector := make([]byte, s.values.Dimension())
	return &scalarQuantizedSupplierScorer{
		AbstractUpdateableRandomVectorScorer: hnsw.NewAbstractUpdateableRandomVectorScorer(vectorsCopy),
		vectorsCopy:                          vectorsCopy,
		similarity:                           s.similarity,
		queryVector:                          queryVector,
	}, nil
}

// Copy mirrors copy().
func (s *ScalarQuantizedRandomVectorScorerSupplier) Copy() (hnsw.RandomVectorScorerSupplier, error) {
	valuesCopy, err := s.values.CopyLegacyQuantizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	return newScalarQuantizedRandomVectorScorerSupplierShallow(s.similarity, s.vectorSimilarityFunction, valuesCopy), nil
}

// String returns the canonical Java toString() output.
func (s *ScalarQuantizedRandomVectorScorerSupplier) String() string {
	return fmt.Sprintf("ScalarQuantizedRandomVectorScorerSupplier(vectorSimilarityFunction=%s)", s.vectorSimilarityFunction.ID().String())
}

// scalarQuantizedSupplierScorer is the anonymous
// UpdateableRandomVectorScorer.AbstractUpdateableRandomVectorScorer returned
// by ScalarQuantizedRandomVectorScorerSupplier.scorer().
type scalarQuantizedSupplierScorer struct {
	*hnsw.AbstractUpdateableRandomVectorScorer
	vectorsCopy quantization.LegacyQuantizedByteVectorValues
	similarity  quantization.ScalarQuantizedVectorSimilarity
	queryVector []byte
	queryOffset float32
}

// SetScoringOrdinal copies the vector at node into the query buffer and
// records its score correction constant.
func (s *scalarQuantizedSupplierScorer) SetScoringOrdinal(node int) error {
	v, err := s.vectorsCopy.VectorValue(node)
	if err != nil {
		return err
	}
	copy(s.queryVector, v)
	offset, err := s.vectorsCopy.GetScoreCorrectionConstant(node)
	if err != nil {
		return err
	}
	s.queryOffset = offset
	return nil
}

// Score scores the buffered query against the stored vector at node.
func (s *scalarQuantizedSupplierScorer) Score(node int) (float32, error) {
	nodeVector, err := s.vectorsCopy.VectorValue(node)
	if err != nil {
		return 0, err
	}
	nodeOffset, err := s.vectorsCopy.GetScoreCorrectionConstant(node)
	if err != nil {
		return 0, err
	}
	return s.similarity.Score(s.queryVector, s.queryOffset, nodeVector, nodeOffset), nil
}

// BulkScore carries the RandomVectorScorer.bulkScore default.
func (s *scalarQuantizedSupplierScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return hnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}
