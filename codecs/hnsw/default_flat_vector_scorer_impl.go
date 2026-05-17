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
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// floatScoringSupplier is the Go counterpart of the private static
// inner class DefaultFlatVectorScorer.FloatScoringSupplier (Lucene
// 10.4.0). It holds the float-vector view together with a
// per-supplier copy used as the scoring target; each Scorer() call
// creates a fresh closure that scores incoming nodes against the
// supplier's target buffer.
type floatScoringSupplier struct {
	vectors            FloatVectorValues
	targetVectors      FloatVectorValues
	similarityFunction index.VectorSimilarityFunction
}

// newFloatScoringSupplier builds a floatScoringSupplier by copying
// the input view (mirroring the Java constructor's `targetVectors =
// vectors.copy()` line).
func newFloatScoringSupplier(
	vectors FloatVectorValues,
	similarityFunction index.VectorSimilarityFunction,
) (hnsw.RandomVectorScorerSupplier, error) {
	targets, err := vectors.CopyFloat()
	if err != nil {
		return nil, fmt.Errorf("FloatScoringSupplier: copy vectors: %w", err)
	}
	return &floatScoringSupplier{
		vectors:            vectors,
		targetVectors:      targets,
		similarityFunction: similarityFunction,
	}, nil
}

// Scorer returns an UpdateableRandomVectorScorer whose target buffer
// is owned per-scorer; SetScoringOrdinal copies the ordinal's vector
// into the buffer, and Score evaluates the similarity against the
// node's stored vector.
func (s *floatScoringSupplier) Scorer() (hnsw.UpdateableRandomVectorScorer, error) {
	target := make([]float32, s.vectors.Dimension())
	base := hnsw.NewAbstractUpdateableRandomVectorScorer(s.vectors)
	return &floatScoringSupplierScorer{
		AbstractUpdateableRandomVectorScorer: base,
		owner:                                s,
		target:                               target,
	}, nil
}

// Copy mirrors the Java reference: produce a new supplier from the
// same vectors view (the constructor will Copy() the view again).
func (s *floatScoringSupplier) Copy() (hnsw.RandomVectorScorerSupplier, error) {
	return newFloatScoringSupplier(s.vectors, s.similarityFunction)
}

// String returns the canonical Java toString() output.
func (s *floatScoringSupplier) String() string {
	return fmt.Sprintf("FloatScoringSupplier(similarityFunction=%s)", s.similarityFunction.String())
}

// floatScoringSupplierScorer is the per-Scorer() closure equivalent.
// It owns a private target buffer (mirroring `byte[] vector = new
// byte[vectors.dimension()]` in the Java inner anonymous class) so
// multiple scorers from the same supplier do not perturb each other.
type floatScoringSupplierScorer struct {
	*hnsw.AbstractUpdateableRandomVectorScorer
	owner  *floatScoringSupplier
	target []float32
}

// SetScoringOrdinal copies the target vector at node into the
// private buffer, mirroring `System.arraycopy(targetVectors.vectorValue(node),
// 0, vector, 0, vector.length)`.
func (s *floatScoringSupplierScorer) SetScoringOrdinal(node int) error {
	v, err := s.owner.targetVectors.VectorValue(node)
	if err != nil {
		return err
	}
	copy(s.target, v)
	return nil
}

// Score evaluates the similarity between the buffered target and the
// stored vector at node.
func (s *floatScoringSupplierScorer) Score(node int) (float32, error) {
	v, err := s.owner.targetVectors.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return computeFloatSimilarity(s.owner.similarityFunction, s.target, v), nil
}

// BulkScore satisfies the [hnsw.RandomVectorScorer] interface via the
// default implementation, mirroring the Java default method.
func (s *floatScoringSupplierScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return hnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// byteScoringSupplier is the byte counterpart of [floatScoringSupplier],
// mirroring DefaultFlatVectorScorer.ByteScoringSupplier in the Java
// reference.
type byteScoringSupplier struct {
	vectors            ByteVectorValues
	targetVectors      ByteVectorValues
	similarityFunction index.VectorSimilarityFunction
}

// newByteScoringSupplier copies the input view and constructs the
// supplier.
func newByteScoringSupplier(
	vectors ByteVectorValues,
	similarityFunction index.VectorSimilarityFunction,
) (hnsw.RandomVectorScorerSupplier, error) {
	targets, err := vectors.CopyByte()
	if err != nil {
		return nil, fmt.Errorf("ByteScoringSupplier: copy vectors: %w", err)
	}
	return &byteScoringSupplier{
		vectors:            vectors,
		targetVectors:      targets,
		similarityFunction: similarityFunction,
	}, nil
}

// Scorer returns an UpdateableRandomVectorScorer with a per-scorer
// byte target buffer.
func (s *byteScoringSupplier) Scorer() (hnsw.UpdateableRandomVectorScorer, error) {
	target := make([]byte, s.vectors.Dimension())
	base := hnsw.NewAbstractUpdateableRandomVectorScorer(s.vectors)
	return &byteScoringSupplierScorer{
		AbstractUpdateableRandomVectorScorer: base,
		owner:                                s,
		target:                               target,
	}, nil
}

// Copy mirrors the Java reference.
func (s *byteScoringSupplier) Copy() (hnsw.RandomVectorScorerSupplier, error) {
	return newByteScoringSupplier(s.vectors, s.similarityFunction)
}

// String returns the canonical Java toString() output.
func (s *byteScoringSupplier) String() string {
	return fmt.Sprintf("ByteScoringSupplier(similarityFunction=%s)", s.similarityFunction.String())
}

// byteScoringSupplierScorer is the per-Scorer() closure for byte
// vectors.
type byteScoringSupplierScorer struct {
	*hnsw.AbstractUpdateableRandomVectorScorer
	owner  *byteScoringSupplier
	target []byte
}

// SetScoringOrdinal copies the target vector at node into the
// private buffer.
func (s *byteScoringSupplierScorer) SetScoringOrdinal(node int) error {
	v, err := s.owner.targetVectors.VectorValue(node)
	if err != nil {
		return err
	}
	copy(s.target, v)
	return nil
}

// Score evaluates the byte similarity between target and node.
func (s *byteScoringSupplierScorer) Score(node int) (float32, error) {
	v, err := s.owner.targetVectors.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return computeByteSimilarity(s.owner.similarityFunction, s.target, v), nil
}

// BulkScore delegates to the package-default implementation.
func (s *byteScoringSupplierScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return hnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// floatVectorScorer is the Go counterpart of the private inner class
// DefaultFlatVectorScorer.FloatVectorScorer in the Java reference. It
// scores stored vectors against an immutable float32 query vector.
type floatVectorScorer struct {
	*hnsw.AbstractRandomVectorScorer
	values             FloatVectorValues
	query              []float32
	similarityFunction index.VectorSimilarityFunction
}

// newFloatVectorScorer builds a floatVectorScorer bound to query.
func newFloatVectorScorer(
	values FloatVectorValues,
	query []float32,
	similarityFunction index.VectorSimilarityFunction,
) hnsw.RandomVectorScorer {
	return &floatVectorScorer{
		AbstractRandomVectorScorer: hnsw.NewAbstractRandomVectorScorer(values),
		values:                     values,
		query:                      query,
		similarityFunction:         similarityFunction,
	}
}

// Score evaluates the similarity between the immutable query and the
// stored vector at node.
func (s *floatVectorScorer) Score(node int) (float32, error) {
	v, err := s.values.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return computeFloatSimilarity(s.similarityFunction, s.query, v), nil
}

// BulkScore delegates to the package-default implementation.
func (s *floatVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return hnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// byteVectorScorer is the byte counterpart of [floatVectorScorer].
type byteVectorScorer struct {
	*hnsw.AbstractRandomVectorScorer
	values             ByteVectorValues
	query              []byte
	similarityFunction index.VectorSimilarityFunction
}

// newByteVectorScorer builds a byteVectorScorer bound to query.
func newByteVectorScorer(
	values ByteVectorValues,
	query []byte,
	similarityFunction index.VectorSimilarityFunction,
) hnsw.RandomVectorScorer {
	return &byteVectorScorer{
		AbstractRandomVectorScorer: hnsw.NewAbstractRandomVectorScorer(values),
		values:                     values,
		query:                      query,
		similarityFunction:         similarityFunction,
	}
}

// Score evaluates the byte similarity between query and the stored
// vector at node.
func (s *byteVectorScorer) Score(node int) (float32, error) {
	v, err := s.values.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return computeByteSimilarity(s.similarityFunction, s.query, v), nil
}

// BulkScore delegates to the package-default implementation.
func (s *byteVectorScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return hnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}
