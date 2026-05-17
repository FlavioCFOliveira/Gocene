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
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// FloatVectorValues is the read-side counterpart consumed by
// [DefaultFlatVectorScorer] for float32-encoded fields. It mirrors
// org.apache.lucene.index.FloatVectorValues by extending
// [hnsw.KnnVectorValues] with a typed VectorValue accessor and a
// covariant Copy.
//
// Concrete implementations live in codec-specific packages. The
// interface is declared here (rather than in index) because Gocene has
// not yet ported the canonical org.apache.lucene.index.FloatVectorValues
// abstract class; once that port lands the interface will be relocated
// and this declaration retained as an alias.
type FloatVectorValues interface {
	hnsw.KnnVectorValues

	// VectorValue returns the float32 vector at the given ordinal.
	// The returned slice may be the implementation's own storage; the
	// caller must copy if it intends to retain it past the next call.
	VectorValue(ord int) ([]float32, error)

	// CopyFloat returns a copy with independent iterator state and the
	// concrete FloatVectorValues type. Mirrors Java's covariant Copy
	// override.
	CopyFloat() (FloatVectorValues, error)
}

// ByteVectorValues is the byte-encoded counterpart of
// [FloatVectorValues]. Mirrors org.apache.lucene.index.ByteVectorValues
// for the byte path consumed by [DefaultFlatVectorScorer].
type ByteVectorValues interface {
	hnsw.KnnVectorValues

	// VectorValue returns the byte vector at the given ordinal. The
	// returned slice may be the implementation's own storage.
	VectorValue(ord int) ([]byte, error)

	// CopyByte returns a copy with independent iterator state and the
	// concrete ByteVectorValues type.
	CopyByte() (ByteVectorValues, error)
}

// HasEncoding is implemented by [hnsw.KnnVectorValues] views that
// expose their element encoding. The Java reference reads the encoding
// directly off KnnVectorValues via `getEncoding()`; Gocene's
// [hnsw.KnnVectorValues] interface does not carry that accessor yet,
// so DefaultFlatVectorScorer narrows to HasEncoding when it needs to
// branch on FLOAT32 vs BYTE.
//
// Implementations of [FloatVectorValues] / [ByteVectorValues] are
// expected to also implement HasEncoding so the supplier factory can
// dispatch correctly. The Java contract is enforced at the type-system
// level (enum on the abstract class); the Go contract is enforced at
// runtime via the type assertion below.
type HasEncoding interface {
	GetEncoding() index.VectorEncoding
}

// DefaultFlatVectorScorer is the Go port of
// org.apache.lucene.codecs.hnsw.DefaultFlatVectorScorer (Lucene 10.4.0).
// It is the default [FlatVectorsScorer] implementation: a software
// fallback that wires the per-similarity scoring closures by
// dispatching to [index.VectorSimilarityFunction]-aware helpers.
//
// The Java reference exposes a singleton INSTANCE static field; Gocene
// preserves that singleton via [DefaultFlatVectorScorerInstance] and
// the [NewDefaultFlatVectorScorer] constructor, both pointing at the
// same zero-state value.
type DefaultFlatVectorScorer struct{}

// DefaultFlatVectorScorerInstance is the package-level singleton
// equivalent to Java's
// `DefaultFlatVectorScorer.INSTANCE = new DefaultFlatVectorScorer()`.
// Callers that depend on identity comparison should reference this
// rather than constructing fresh instances.
var DefaultFlatVectorScorerInstance = &DefaultFlatVectorScorer{}

// NewDefaultFlatVectorScorer returns the singleton scorer.
// Constructing a fresh DefaultFlatVectorScorer is permitted but
// idiomatic Gocene code should prefer the singleton to mirror the
// Java reference's static-final INSTANCE pattern.
func NewDefaultFlatVectorScorer() *DefaultFlatVectorScorer {
	return DefaultFlatVectorScorerInstance
}

// String returns the canonical Java toString() output.
func (*DefaultFlatVectorScorer) String() string {
	return "DefaultFlatVectorScorer()"
}

// GetRandomVectorScorerSupplier returns a supplier that branches on
// the encoding reported by vectorValues. Float32 fields receive a
// FloatScoringSupplier; byte fields receive a ByteScoringSupplier; any
// other encoding (or a vectorValues that doesn't expose encoding)
// surfaces an error matching the Java IllegalArgumentException
// message.
func (s *DefaultFlatVectorScorer) GetRandomVectorScorerSupplier(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues hnsw.KnnVectorValues,
) (hnsw.RandomVectorScorerSupplier, error) {
	enc, ok := vectorValues.(HasEncoding)
	if !ok {
		return nil, fmt.Errorf(
			"vectorValues must be an instance of FloatVectorValues or ByteVectorValues, got a %T",
			vectorValues,
		)
	}
	switch enc.GetEncoding() {
	case index.VectorEncodingFloat32:
		fv, ok := vectorValues.(FloatVectorValues)
		if !ok {
			return nil, fmt.Errorf(
				"vectorValues reports FLOAT32 encoding but does not implement FloatVectorValues; got %T",
				vectorValues,
			)
		}
		return newFloatScoringSupplier(fv, similarityFunction)
	case index.VectorEncodingByte:
		bv, ok := vectorValues.(ByteVectorValues)
		if !ok {
			return nil, fmt.Errorf(
				"vectorValues reports BYTE encoding but does not implement ByteVectorValues; got %T",
				vectorValues,
			)
		}
		return newByteScoringSupplier(bv, similarityFunction)
	default:
		return nil, fmt.Errorf(
			"vectorValues must be an instance of FloatVectorValues or ByteVectorValues, got a %T",
			vectorValues,
		)
	}
}

// GetRandomVectorScorer returns a scorer over float32 vectors against
// the supplied target. Mirrors the float[] overload in the Java
// reference, including the dimension check.
func (s *DefaultFlatVectorScorer) GetRandomVectorScorer(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues hnsw.KnnVectorValues,
	target []float32,
) (hnsw.RandomVectorScorer, error) {
	fv, ok := vectorValues.(FloatVectorValues)
	if !ok {
		return nil, fmt.Errorf("vectorValues must be an instance of FloatVectorValues; got %T", vectorValues)
	}
	if err := CheckDimensions(len(target), fv.Dimension()); err != nil {
		return nil, err
	}
	return newFloatVectorScorer(fv, target, similarityFunction), nil
}

// GetRandomVectorScorerByte returns a scorer over byte vectors against
// the supplied byte target. Mirrors the byte[] overload in the Java
// reference, including the dimension check.
func (s *DefaultFlatVectorScorer) GetRandomVectorScorerByte(
	similarityFunction index.VectorSimilarityFunction,
	vectorValues hnsw.KnnVectorValues,
	target []byte,
) (hnsw.RandomVectorScorer, error) {
	bv, ok := vectorValues.(ByteVectorValues)
	if !ok {
		return nil, fmt.Errorf("vectorValues must be an instance of ByteVectorValues; got %T", vectorValues)
	}
	if err := CheckDimensions(len(target), bv.Dimension()); err != nil {
		return nil, err
	}
	return newByteVectorScorer(bv, target, similarityFunction), nil
}

// computeFloatSimilarity dispatches to the per-similarity scoring
// closure mirroring Java's VectorSimilarityFunction#compare overloads
// for float32 vectors. The output is the normalised similarity in
// [0, 1] for the bounded functions and the scaled inner product for
// MAXIMUM_INNER_PRODUCT.
//
// The arithmetic intentionally mirrors the legacy codecs-root
// implementations so DefaultFlatVectorScorer remains numerically
// equivalent to the existing test fixtures while the canonical
// index.VectorSimilarityFunction port matures. When the canonical
// VectorSimilarityFunction#compare arrives (post sprint 22), this
// helper becomes a one-liner forwarding to it.
func computeFloatSimilarity(sim index.VectorSimilarityFunction, a, b []float32) float32 {
	switch sim {
	case index.VectorSimilarityFunctionEuclidean:
		var sum float32
		for i := range a {
			d := a[i] - b[i]
			sum += d * d
		}
		return 1.0 / (1.0 + sum)
	case index.VectorSimilarityFunctionDotProduct:
		var dot float32
		for i := range a {
			dot += a[i] * b[i]
		}
		return (dot + 1.0) / 2.0
	case index.VectorSimilarityFunctionCosine:
		var dot, na, nb float32
		for i := range a {
			dot += a[i] * b[i]
			na += a[i] * a[i]
			nb += b[i] * b[i]
		}
		if na == 0 || nb == 0 {
			return 0
		}
		return (dot/(sqrt32(na)*sqrt32(nb)) + 1.0) / 2.0
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		var dot float32
		for i := range a {
			dot += a[i] * b[i]
		}
		if dot < 0 {
			return 1.0 / (1.0 - dot)
		}
		return dot + 1.0
	default:
		return 0
	}
}

// computeByteSimilarity dispatches to the per-similarity scoring
// closure for byte vectors. The arithmetic mirrors the existing
// codecs-root helpers for the same reason documented on
// [computeFloatSimilarity]: numerical parity with the legacy fixtures.
func computeByteSimilarity(sim index.VectorSimilarityFunction, a, b []byte) float32 {
	switch sim {
	case index.VectorSimilarityFunctionEuclidean:
		var sum int32
		for i := range a {
			d := int32(a[i]) - int32(b[i])
			sum += d * d
		}
		return 1.0 / (1.0 + float32(sum))
	case index.VectorSimilarityFunctionDotProduct:
		var dot int32
		for i := range a {
			dot += int32(a[i]) * int32(b[i])
		}
		maxDot := float32(127 * 127 * len(a))
		if maxDot == 0 {
			return 0
		}
		return (float32(dot) + maxDot) / (2.0 * maxDot)
	case index.VectorSimilarityFunctionCosine:
		var dot, na, nb int32
		for i := range a {
			dot += int32(a[i]) * int32(b[i])
			na += int32(a[i]) * int32(a[i])
			nb += int32(b[i]) * int32(b[i])
		}
		if na == 0 || nb == 0 {
			return 0.5
		}
		cos := float32(dot) / (sqrt32(float32(na)) * sqrt32(float32(nb)))
		return (cos + 1.0) / 2.0
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		var dot int32
		for i := range a {
			dot += int32(a[i]) * int32(b[i])
		}
		if dot < 0 {
			return 1.0 / (1.0 - float32(dot)/1000.0)
		}
		return float32(dot)/1000.0 + 1.0
	default:
		return 0
	}
}

// sqrt32 is a float32 wrapper around math.Sqrt; kept private to the
// package because Gocene has no float32-specific sqrt helper yet.
func sqrt32(x float32) float32 {
	if x <= 0 {
		return 0
	}
	return float32(math.Sqrt(float64(x)))
}
