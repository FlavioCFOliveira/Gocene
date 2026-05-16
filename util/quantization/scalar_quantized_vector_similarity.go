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

package quantization

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ScalarQuantizedVectorSimilarity is the Go port of
// org.apache.lucene.util.quantization.ScalarQuantizedVectorSimilarity
// (Lucene 10.4.0). Implementations compute the similarity between two
// scalar-quantized byte vectors, applying the per-pair score-correction
// offsets and the quantization-wide constant multiplier so the result
// approximates the equivalent dequantized similarity.
//
// Concrete implementations are obtained from
// [FromVectorSimilarity]; the three returned types correspond
// one-to-one with the Java inner classes Euclidean, DotProduct, and
// MaximumInnerProduct.
type ScalarQuantizedVectorSimilarity interface {
	// Score computes the quantized similarity between queryVector and
	// storedVector. queryVectorOffset and vectorOffset are the
	// score-correction constants attached to the two vectors at
	// quantization time. The method is pure (no I/O, no error path)
	// and mirrors Java's `float score(byte[], float, byte[], float)`.
	Score(queryVector []byte, queryVectorOffset float32, storedVector []byte, vectorOffset float32) float32
}

// ByteVectorComparator is the Go counterpart of Lucene's
// `ScalarQuantizedVectorSimilarity.ByteVectorComparator` functional
// interface. It computes a scalar metric (here, a dot product) over
// two byte vectors of identical length and returns an int32 result.
//
// Implementations must be deterministic and side-effect free.
type ByteVectorComparator func(v1, v2 []byte) int32

// int4DotProductForward is a local forward-declaration stand-in for
// the not-yet-ported util.Int4DotProduct (the Go counterpart of
// VectorUtil.int4DotProduct in Lucene 10.4.0). The Java dispatch in
// fromVectorSimilarity routes the `bits <= 4` branch to int4DotProduct
// and the `bits > 4` branch to uint8DotProduct; for non-negative byte
// inputs in [0, 15] both produce identical integer dot products, so
// this stub delegates to util.Uint8DotProduct.
//
// TODO(rmp): replace this stub with util.Int4DotProduct once that
// helper is added to util/vector_util.go. The dispatch shape in
// [FromVectorSimilarity] must remain intact so the swap is mechanical.
func int4DotProductForward(a, b []byte) int32 {
	return util.Uint8DotProduct(a, b)
}

// FromVectorSimilarity is the Go counterpart of the static factory
// method
// `ScalarQuantizedVectorSimilarity.fromVectorSimilarity(VectorSimilarityFunction, float, byte)`.
// It returns the concrete implementation that applies the appropriate
// quantization corrections for the supplied similarity function.
//
// The bits parameter selects the dot-product variant for the
// DotProduct/Cosine/MaximumInnerProduct cases: bits <= 4 routes
// through the int4 path, bits > 4 through the uint8 path. The
// EUCLIDEAN case ignores bits, mirroring the Java reference.
//
// Returns an error when sim is not one of the four
// [index.VectorSimilarityFunction] constants; the Java original
// exhaustively switches over the enum and would surface a
// MatchException at runtime for an out-of-range value.
func FromVectorSimilarity(sim index.VectorSimilarityFunction, constMultiplier float32, bits byte) (ScalarQuantizedVectorSimilarity, error) {
	switch sim {
	case index.VectorSimilarityFunctionEuclidean:
		return &Euclidean{constMultiplier: constMultiplier}, nil
	case index.VectorSimilarityFunctionCosine, index.VectorSimilarityFunctionDotProduct:
		return &DotProduct{
			constMultiplier: constMultiplier,
			comparator:      dotProductComparator(bits),
		}, nil
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		return &MaximumInnerProduct{
			constMultiplier: constMultiplier,
			comparator:      dotProductComparator(bits),
		}, nil
	default:
		return nil, fmt.Errorf("quantization: unsupported VectorSimilarityFunction %v", sim)
	}
}

// dotProductComparator selects the byte-vector dot-product variant
// matching the Java
// `bits <= 4 ? VectorUtil::int4DotProduct : VectorUtil::uint8DotProduct`
// expression. It is shared by the DotProduct/Cosine and
// MaximumInnerProduct factory branches.
func dotProductComparator(bits byte) ByteVectorComparator {
	if bits <= 4 {
		return int4DotProductForward
	}
	return util.Uint8DotProduct
}

// Euclidean is the Go counterpart of
// `ScalarQuantizedVectorSimilarity.Euclidean` (Lucene 10.4.0). It
// scores a pair of quantized byte vectors by computing the unsigned
// square distance, scaling it by the quantization constant
// multiplier, and folding it into the standard `1 / (1 + d)` formula.
type Euclidean struct {
	constMultiplier float32
}

// Score returns the quantized Euclidean similarity. The signature and
// semantics mirror the Java override exactly.
func (e *Euclidean) Score(queryVector []byte, _ float32, storedVector []byte, _ float32) float32 {
	squareDistance := util.Uint8SquareDistance(storedVector, queryVector)
	adjustedDistance := float32(squareDistance) * e.constMultiplier
	return 1 / (1 + adjustedDistance)
}

// DotProduct is the Go counterpart of
// `ScalarQuantizedVectorSimilarity.DotProduct` (Lucene 10.4.0). It
// scores a pair of quantized byte vectors by computing an unsigned
// dot product, applying the constant multiplier and per-vector
// offsets, and mapping the result through `(1 + x) / 2` clamped at
// zero.
//
// The Java reference has an `assert dotProduct >= 0` invariant that
// holds under correct scalar quantization. Go production builds
// historically do not retain Java's `-ea` assertions, so this port
// omits the assertion at scoring time and trusts the caller to feed
// validly-quantized vectors; the contract is documented for
// completeness.
type DotProduct struct {
	constMultiplier float32
	comparator      ByteVectorComparator
}

// Score returns the quantized dot-product similarity. The signature
// and semantics mirror the Java override exactly.
func (d *DotProduct) Score(queryVector []byte, queryOffset float32, storedVector []byte, vectorOffset float32) float32 {
	dotProduct := d.comparator(storedVector, queryVector)
	adjustedDistance := float32(dotProduct)*d.constMultiplier + queryOffset + vectorOffset
	v := (1 + adjustedDistance) / 2
	if v < 0 {
		return 0
	}
	return v
}

// MaximumInnerProduct is the Go counterpart of
// `ScalarQuantizedVectorSimilarity.MaximumInnerProduct` (Lucene
// 10.4.0). It scores a pair of quantized byte vectors by computing
// an unsigned dot product, applying the constant multiplier and
// per-vector offsets, and feeding the result through
// [util.ScaleMaxInnerProductScore].
//
// The Java reference has an `assert dotProduct >= 0` invariant that
// holds under correct scalar quantization. This port omits the
// assertion at scoring time for the same reason documented on
// [DotProduct].
type MaximumInnerProduct struct {
	constMultiplier float32
	comparator      ByteVectorComparator
}

// Score returns the quantized maximum-inner-product similarity. The
// signature and semantics mirror the Java override exactly.
func (m *MaximumInnerProduct) Score(queryVector []byte, queryOffset float32, storedVector []byte, vectorOffset float32) float32 {
	dotProduct := m.comparator(storedVector, queryVector)
	adjustedDistance := float32(dotProduct)*m.constMultiplier + queryOffset + vectorOffset
	return util.ScaleMaxInnerProductScore(adjustedDistance)
}
