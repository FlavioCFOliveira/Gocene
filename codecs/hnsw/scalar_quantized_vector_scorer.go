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
// 10.4.0). It is a [FlatVectorsScorer] decorator that intercepts
// scoring requests against [quantization.QuantizedByteVectorValues]
// and routes them through the scalar-quantized similarity pipeline;
// requests against non-quantized vector views are delegated to the
// wrapped non-quantized scorer.
//
// The decorator pattern mirrors the Java reference exactly: the
// constructor takes a FlatVectorsScorer (typically
// [DefaultFlatVectorScorerInstance]) which becomes the fallback when
// the supplied vector values are not quantized. The "during initial
// indexing and flush" path that the Java reference comments out lands
// here too — the delegate's getRandomVectorScorerSupplier is called
// whenever the values are still floats.
type ScalarQuantizedVectorScorer struct {
	nonQuantizedDelegate FlatVectorsScorer
}

// NewScalarQuantizedVectorScorer wraps the supplied FlatVectorsScorer
// with the scalar-quantized scoring decorator.
func NewScalarQuantizedVectorScorer(delegate FlatVectorsScorer) *ScalarQuantizedVectorScorer {
	return &ScalarQuantizedVectorScorer{nonQuantizedDelegate: delegate}
}

// String returns the canonical Java toString() output.
func (s *ScalarQuantizedVectorScorer) String() string {
	return fmt.Sprintf("ScalarQuantizedVectorScorer(nonQuantizedDelegate=%v)", s.nonQuantizedDelegate)
}

// GetRandomVectorScorerSupplier returns a supplier specialised for
// QuantizedByteVectorValues when the supplied vectorValues is
// quantized; otherwise it delegates to the wrapped scorer. The
// quantized branch is taken when vectorValues was produced by
// [AsHnswKnnVectorValues] — see [AsQuantizedByteVectorValues] for the
// recovery rules.
func (s *ScalarQuantizedVectorScorer) GetRandomVectorScorerSupplier(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues hnsw.KnnVectorValues,
) (hnsw.RandomVectorScorerSupplier, error) {
	if q, ok := AsQuantizedByteVectorValues(vectorValues); ok {
		quantizer, err := q.GetScalarQuantizer()
		if err != nil {
			return nil, fmt.Errorf("ScalarQuantizedVectorScorer: GetScalarQuantizer: %w", err)
		}
		return NewScalarQuantizedRandomVectorScorerSupplier(similarityFunction, quantizer, q)
	}
	return s.nonQuantizedDelegate.GetRandomVectorScorerSupplier(similarityFunction, vectorValues)
}

// GetRandomVectorScorer returns a scorer that quantizes the supplied
// float32 target on the fly and then scores against the quantized
// byte vectors. When vectorValues is not quantized the request is
// delegated to the wrapped scorer, matching the Java fallback used
// during initial indexing and flush. The quantized branch is taken
// when vectorValues was produced by [AsHnswKnnVectorValues].
func (s *ScalarQuantizedVectorScorer) GetRandomVectorScorer(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues hnsw.KnnVectorValues,
	target []float32,
) (hnsw.RandomVectorScorer, error) {
	q, ok := AsQuantizedByteVectorValues(vectorValues)
	if !ok {
		return s.nonQuantizedDelegate.GetRandomVectorScorer(similarityFunction, vectorValues, target)
	}
	quantizer, err := q.GetScalarQuantizer()
	if err != nil {
		return nil, fmt.Errorf("ScalarQuantizedVectorScorer: GetScalarQuantizer: %w", err)
	}
	targetBytes := make([]byte, len(target))
	offsetCorrection, err := QuantizeQuery(target, targetBytes, similarityFunction, quantizer)
	if err != nil {
		return nil, fmt.Errorf("ScalarQuantizedVectorScorer: QuantizeQuery: %w", err)
	}
	sim, err := quantization.FromVectorSimilarity(similarityFunction, quantizer.GetConstantMultiplier(), quantizer.GetBits())
	if err != nil {
		return nil, fmt.Errorf("ScalarQuantizedVectorScorer: FromVectorSimilarity: %w", err)
	}
	base := hnsw.NewAbstractRandomVectorScorer(AsHnswKnnVectorValues(q))
	return &scalarQuantizedFloatScorer{
		AbstractRandomVectorScorer: base,
		values:                     q,
		similarity:                 sim,
		targetBytes:                targetBytes,
		offsetCorrection:           offsetCorrection,
	}, nil
}

// GetRandomVectorScorerByte mirrors the Java reference: byte targets
// are always delegated to the non-quantized scorer (the Java method
// implementation is `return nonQuantizedDelegate.getRandomVectorScorer(...)`,
// which dispatches to the byte[] overload through Java's method
// overloading; Go's method-name distinction routes the call to
// [FlatVectorsScorer.GetRandomVectorScorerByte] explicitly).
func (s *ScalarQuantizedVectorScorer) GetRandomVectorScorerByte(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues hnsw.KnnVectorValues,
	target []byte,
) (hnsw.RandomVectorScorer, error) {
	return s.nonQuantizedDelegate.GetRandomVectorScorerByte(similarityFunction, vectorValues, target)
}

// QuantizeQuery is the Go port of
// ScalarQuantizedVectorScorer.quantizeQuery in the Java reference. It
// quantizes the supplied float32 query into quantizedQuery using
// scalarQuantizer, l2-normalizing the query first when the similarity
// function is COSINE (the Java reference makes a defensive copy via
// ArrayUtil.copyArray before normalising; Gocene does the same so the
// caller's slice is never mutated).
//
// Returns the offset correction the caller adds when scoring against
// other quantized vectors, matching the Java return value.
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
		// Defensive copy so the caller's slice is never mutated;
		// mirrors `ArrayUtil.copyArray(query)` in the Java reference.
		copied := make([]float32, len(query))
		copy(copied, query)
		util.L2Normalize(copied)
		processed = copied
	}
	return scalarQuantizer.Quantize(processed, quantizedQuery, similarityFunction), nil
}

// scalarQuantizedFloatScorer is the Go counterpart of the anonymous
// inner RandomVectorScorer.AbstractRandomVectorScorer subclass
// returned by getRandomVectorScorer(float[]) in the Java reference.
type scalarQuantizedFloatScorer struct {
	*hnsw.AbstractRandomVectorScorer
	values           quantization.QuantizedByteVectorValues
	similarity       quantization.ScalarQuantizedVectorSimilarity
	targetBytes      []byte
	offsetCorrection float32
}

// Score evaluates the scalar-quantized similarity between the
// pre-quantized target and the stored byte vector at node, applying
// the per-vector score correction stored alongside the quantized
// bytes.
func (s *scalarQuantizedFloatScorer) Score(node int) (float32, error) {
	nodeVec, err := s.values.VectorValue(node)
	if err != nil {
		return 0, err
	}
	nodeOffset, err := s.values.GetScoreCorrectionConstant(node)
	if err != nil {
		return 0, err
	}
	return s.similarity.Score(s.targetBytes, s.offsetCorrection, nodeVec, nodeOffset), nil
}

// BulkScore delegates to the package-default implementation.
func (s *scalarQuantizedFloatScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return hnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// ScalarQuantizedRandomVectorScorerSupplier is the Go port of the
// public inner class
// ScalarQuantizedVectorScorer.ScalarQuantizedRandomVectorScorerSupplier
// in the Java reference. It supplies UpdateableRandomVectorScorer
// instances that score against a per-scorer copy of the quantized
// vector view.
type ScalarQuantizedRandomVectorScorerSupplier struct {
	values                   quantization.QuantizedByteVectorValues
	similarity               quantization.ScalarQuantizedVectorSimilarity
	vectorSimilarityFunction index.VectorSimilarityFunction
}

// NewScalarQuantizedRandomVectorScorerSupplier mirrors the primary
// public constructor in the Java reference, deriving the
// scalar-quantized similarity from the supplied quantizer.
func NewScalarQuantizedRandomVectorScorerSupplier(
	similarityFunction index.VectorSimilarityFunction,
	scalarQuantizer *quantization.ScalarQuantizer,
	values quantization.QuantizedByteVectorValues,
) (*ScalarQuantizedRandomVectorScorerSupplier, error) {
	sim, err := quantization.FromVectorSimilarity(similarityFunction, scalarQuantizer.GetConstantMultiplier(), scalarQuantizer.GetBits())
	if err != nil {
		return nil, fmt.Errorf("ScalarQuantizedRandomVectorScorerSupplier: FromVectorSimilarity: %w", err)
	}
	return &ScalarQuantizedRandomVectorScorerSupplier{
		values:                   values,
		similarity:               sim,
		vectorSimilarityFunction: similarityFunction,
	}, nil
}

// newScalarQuantizedRandomVectorScorerSupplierShallow mirrors the
// private constructor used by Copy() in the Java reference, which
// reuses an already-built similarity instance instead of rebuilding
// it from the quantizer.
func newScalarQuantizedRandomVectorScorerSupplierShallow(
	similarity quantization.ScalarQuantizedVectorSimilarity,
	vectorSimilarityFunction index.VectorSimilarityFunction,
	values quantization.QuantizedByteVectorValues,
) *ScalarQuantizedRandomVectorScorerSupplier {
	return &ScalarQuantizedRandomVectorScorerSupplier{
		values:                   values,
		similarity:               similarity,
		vectorSimilarityFunction: vectorSimilarityFunction,
	}
}

// Scorer returns an UpdateableRandomVectorScorer whose target buffer
// is owned per-scorer. The Java reference copies the vector view via
// `values.copy()` and allocates a fresh `byte[] queryVector =
// new byte[values.dimension()]`; Gocene mirrors both allocations.
func (s *ScalarQuantizedRandomVectorScorerSupplier) Scorer() (hnsw.UpdateableRandomVectorScorer, error) {
	vectorsCopy, err := s.values.Copy()
	if err != nil {
		return nil, fmt.Errorf("ScalarQuantizedRandomVectorScorerSupplier: copy values: %w", err)
	}
	queryVector := make([]byte, s.values.Dimension())
	base := hnsw.NewAbstractUpdateableRandomVectorScorer(AsHnswKnnVectorValues(vectorsCopy))
	return &scalarQuantizedSupplierScorer{
		AbstractUpdateableRandomVectorScorer: base,
		values:                               vectorsCopy,
		similarity:                           s.similarity,
		queryVector:                          queryVector,
	}, nil
}

// Copy mirrors the Java reference: produce a new supplier sharing the
// similarity instance but with an independent quantized view.
func (s *ScalarQuantizedRandomVectorScorerSupplier) Copy() (hnsw.RandomVectorScorerSupplier, error) {
	vectorsCopy, err := s.values.Copy()
	if err != nil {
		return nil, fmt.Errorf("ScalarQuantizedRandomVectorScorerSupplier: copy values: %w", err)
	}
	return newScalarQuantizedRandomVectorScorerSupplierShallow(s.similarity, s.vectorSimilarityFunction, vectorsCopy), nil
}

// String returns the canonical Java toString() output.
func (s *ScalarQuantizedRandomVectorScorerSupplier) String() string {
	return fmt.Sprintf("ScalarQuantizedRandomVectorScorerSupplier(vectorSimilarityFunction=%s)", s.vectorSimilarityFunction.String())
}

// scalarQuantizedSupplierScorer is the per-Scorer() closure for the
// supplier, mirroring the anonymous inner subclass in the Java
// reference.
type scalarQuantizedSupplierScorer struct {
	*hnsw.AbstractUpdateableRandomVectorScorer
	values      quantization.QuantizedByteVectorValues
	similarity  quantization.ScalarQuantizedVectorSimilarity
	queryVector []byte
	queryOffset float32
}

// SetScoringOrdinal copies the target vector at node and the
// matching score-correction constant.
func (s *scalarQuantizedSupplierScorer) SetScoringOrdinal(node int) error {
	v, err := s.values.VectorValue(node)
	if err != nil {
		return err
	}
	copy(s.queryVector, v)
	offset, err := s.values.GetScoreCorrectionConstant(node)
	if err != nil {
		return err
	}
	s.queryOffset = offset
	return nil
}

// Score evaluates the scalar-quantized similarity between the
// buffered target/offset pair and the stored vector at node.
func (s *scalarQuantizedSupplierScorer) Score(node int) (float32, error) {
	nodeVec, err := s.values.VectorValue(node)
	if err != nil {
		return 0, err
	}
	nodeOffset, err := s.values.GetScoreCorrectionConstant(node)
	if err != nil {
		return 0, err
	}
	return s.similarity.Score(s.queryVector, s.queryOffset, nodeVec, nodeOffset), nil
}

// BulkScore delegates to the package-default implementation.
func (s *scalarQuantizedSupplierScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return hnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}
