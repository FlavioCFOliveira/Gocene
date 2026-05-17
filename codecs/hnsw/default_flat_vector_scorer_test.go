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

package hnsw_test

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	hnswutil "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// fakeFloatVectorValues is the minimal FloatVectorValues fixture used by
// the DefaultFlatVectorScorer tests. It mirrors the in-memory test peer
// `OffsetFloatVectorValues` in TestFlatVectorScorer.java by exposing an
// ordinal-keyed float slice and reporting FLOAT32 encoding.
type fakeFloatVectorValues struct {
	dim     int
	vectors [][]float32
}

func (v *fakeFloatVectorValues) Dimension() int       { return v.dim }
func (v *fakeFloatVectorValues) Size() int            { return len(v.vectors) }
func (v *fakeFloatVectorValues) OrdToDoc(ord int) int { return ord }
func (v *fakeFloatVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// GetAcceptOrds is a no-op for the fixture: returns the input
// untouched, matching the dense-values default.
func (v *fakeFloatVectorValues) GetAcceptOrds(b util.Bits) util.Bits { return b }

// Iterator returns nil because the scorer tests never walk the
// iterator; SetScoringOrdinal and Score are ordinal-keyed and bypass
// the iterator entirely.
func (v *fakeFloatVectorValues) Iterator() hnswutil.DocIndexIterator { return nil }

func (v *fakeFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	return v.vectors[ord], nil
}

func (v *fakeFloatVectorValues) CopyFloat() (hnsw.FloatVectorValues, error) {
	cp := make([][]float32, len(v.vectors))
	for i, src := range v.vectors {
		cp[i] = append([]float32(nil), src...)
	}
	return &fakeFloatVectorValues{dim: v.dim, vectors: cp}, nil
}

// fakeByteVectorValues is the byte-encoded counterpart of
// [fakeFloatVectorValues].
type fakeByteVectorValues struct {
	dim     int
	vectors [][]byte
}

func (v *fakeByteVectorValues) Dimension() int                      { return v.dim }
func (v *fakeByteVectorValues) Size() int                           { return len(v.vectors) }
func (v *fakeByteVectorValues) OrdToDoc(ord int) int                { return ord }
func (v *fakeByteVectorValues) GetEncoding() index.VectorEncoding   { return index.VectorEncodingByte }
func (v *fakeByteVectorValues) GetAcceptOrds(b util.Bits) util.Bits { return b }
func (v *fakeByteVectorValues) Iterator() hnswutil.DocIndexIterator { return nil }
func (v *fakeByteVectorValues) VectorValue(ord int) ([]byte, error) { return v.vectors[ord], nil }

func (v *fakeByteVectorValues) CopyByte() (hnsw.ByteVectorValues, error) {
	cp := make([][]byte, len(v.vectors))
	for i, src := range v.vectors {
		cp[i] = append([]byte(nil), src...)
	}
	return &fakeByteVectorValues{dim: v.dim, vectors: cp}, nil
}

// TestDefaultFlatVectorScorer_String verifies the canonical toString().
// Indirect coverage of TestFlatVectorScorer#testDefaultOrMemSegScorer.
func TestDefaultFlatVectorScorer_String(t *testing.T) {
	if got, want := hnsw.NewDefaultFlatVectorScorer().String(), "DefaultFlatVectorScorer()"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// TestDefaultFlatVectorScorer_FloatDimensionMismatch checks the float
// path dimension guard.
// Indirect coverage of TestFlatVectorScorer#testCheckFloatDimensions.
func TestDefaultFlatVectorScorer_FloatDimensionMismatch(t *testing.T) {
	vv := &fakeFloatVectorValues{dim: 4, vectors: [][]float32{make([]float32, 4)}}
	scorer := hnsw.NewDefaultFlatVectorScorer()
	for _, sim := range []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionCosine,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	} {
		_, err := scorer.GetRandomVectorScorer(sim, vv, make([]float32, 5))
		if err == nil {
			t.Fatalf("expected dimension mismatch error for %s", sim)
		}
		if !strings.Contains(err.Error(), "vector query dimension") {
			t.Fatalf("error %q does not mention dimension mismatch", err)
		}
	}
}

// TestDefaultFlatVectorScorer_ByteDimensionMismatch checks the byte
// path dimension guard.
// Indirect coverage of TestFlatVectorScorer#testCheckByteDimensions.
func TestDefaultFlatVectorScorer_ByteDimensionMismatch(t *testing.T) {
	vv := &fakeByteVectorValues{dim: 4, vectors: [][]byte{make([]byte, 4)}}
	scorer := hnsw.NewDefaultFlatVectorScorer()
	for _, sim := range []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionCosine,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	} {
		_, err := scorer.GetRandomVectorScorerByte(sim, vv, make([]byte, 5))
		if err == nil {
			t.Fatalf("expected dimension mismatch error for %s", sim)
		}
	}
}

// TestDefaultFlatVectorScorer_FloatSelfMatchIsPerfect verifies that
// scoring a vector against itself under EUCLIDEAN returns 1.0 (the
// normalised maximum). This exercises the GetRandomVectorScorer +
// Score path end-to-end through the AbstractRandomVectorScorer.
func TestDefaultFlatVectorScorer_FloatSelfMatchIsPerfect(t *testing.T) {
	vv := &fakeFloatVectorValues{
		dim: 3,
		vectors: [][]float32{
			{1, 0, 0},
			{0, 1, 0},
		},
	}
	scorer := hnsw.NewDefaultFlatVectorScorer()
	s, err := scorer.GetRandomVectorScorer(
		index.VectorSimilarityFunctionEuclidean, vv, []float32{1, 0, 0},
	)
	if err != nil {
		t.Fatalf("GetRandomVectorScorer: %v", err)
	}
	score, err := s.Score(0)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if math.Abs(float64(score-1.0)) > 1e-6 {
		t.Fatalf("Score(0) = %v, want 1.0", score)
	}
}

// TestDefaultFlatVectorScorer_SupplierIndependence verifies that
// creating multiple scorers from the same supplier does not perturb
// previously-set scoring ordinals — this is the core invariant of
// TestFlatVectorScorer#testMultipleFloatScorers and
// testMultipleByteScorers.
func TestDefaultFlatVectorScorer_SupplierIndependence(t *testing.T) {
	vv := &fakeFloatVectorValues{
		dim: 4,
		vectors: [][]float32{
			{0, 0, 0, 0},
			{1, 1, 1, 1},
			{15, 15, 15, 15},
		},
	}
	scorer := hnsw.NewDefaultFlatVectorScorer()
	supplier, err := scorer.GetRandomVectorScorerSupplier(
		index.VectorSimilarityFunctionEuclidean, vv,
	)
	if err != nil {
		t.Fatalf("GetRandomVectorScorerSupplier: %v", err)
	}

	first, err := supplier.Scorer()
	if err != nil {
		t.Fatalf("Scorer #1: %v", err)
	}
	if err := first.SetScoringOrdinal(0); err != nil {
		t.Fatalf("SetScoringOrdinal #1: %v", err)
	}
	firstScore, err := first.Score(1)
	if err != nil {
		t.Fatalf("Score #1: %v", err)
	}

	second, err := supplier.Scorer()
	if err != nil {
		t.Fatalf("Scorer #2: %v", err)
	}
	if err := second.SetScoringOrdinal(2); err != nil {
		t.Fatalf("SetScoringOrdinal #2: %v", err)
	}

	rerun, err := first.Score(1)
	if err != nil {
		t.Fatalf("Score #1 rerun: %v", err)
	}
	if firstScore != rerun {
		t.Fatalf("first scorer perturbed: before=%v after=%v", firstScore, rerun)
	}
}

// TestDefaultFlatVectorScorer_UnknownEncodingErrors verifies the
// fallback error path mirroring Java's IllegalArgumentException for an
// unexpected KnnVectorValues subtype.
func TestDefaultFlatVectorScorer_UnknownEncodingErrors(t *testing.T) {
	rawNoEncoding := &noEncodingValues{dim: 3, size: 0}
	scorer := hnsw.NewDefaultFlatVectorScorer()
	_, err := scorer.GetRandomVectorScorerSupplier(
		index.VectorSimilarityFunctionEuclidean, rawNoEncoding,
	)
	if err == nil {
		t.Fatalf("expected error for unknown encoding, got nil")
	}
	if !strings.Contains(err.Error(), "FloatVectorValues or ByteVectorValues") {
		t.Fatalf("error %q does not mention the expected fallback path", err)
	}
}

// noEncodingValues implements hnswutil.KnnVectorValues but not
// hnsw.HasEncoding, exercising the fallback branch.
type noEncodingValues struct {
	dim  int
	size int
}

func (v *noEncodingValues) Dimension() int                      { return v.dim }
func (v *noEncodingValues) Size() int                           { return v.size }
func (v *noEncodingValues) OrdToDoc(ord int) int                { return ord }
func (v *noEncodingValues) GetAcceptOrds(b util.Bits) util.Bits { return b }
func (v *noEncodingValues) Iterator() hnswutil.DocIndexIterator { return nil }

// TestCheckDimensions_ErrorMessage verifies the byte-for-byte text of
// the dimension-mismatch error matches the Java reference.
func TestCheckDimensions_ErrorMessage(t *testing.T) {
	err := hnsw.CheckDimensions(5, 4)
	if err == nil {
		t.Fatal("expected error for mismatched dimensions")
	}
	want := "vector query dimension: 5 differs from field dimension: 4"
	if err.Error() != want {
		t.Fatalf("CheckDimensions error = %q, want %q", err.Error(), want)
	}
	if err := hnsw.CheckDimensions(3, 3); err != nil {
		t.Fatalf("CheckDimensions(3,3) = %v, want nil", err)
	}
}

// TestCheckDimensions_Equality verifies CheckDimensions returns nil
// for equal dimensions (the no-op path).
func TestCheckDimensions_Equality(t *testing.T) {
	if err := hnsw.CheckDimensions(7, 7); err != nil {
		t.Fatalf("CheckDimensions(7,7) = %v, want nil", err)
	}
}

// TestFlatVectorScorerUtil_DefaultFallback verifies the
// GetLucene99FlatVectorsScorer factory returns the singleton default.
func TestFlatVectorScorerUtil_DefaultFallback(t *testing.T) {
	got := hnsw.GetLucene99FlatVectorsScorer()
	if got != hnsw.DefaultFlatVectorScorerInstance {
		t.Fatalf("GetLucene99FlatVectorsScorer() returned a different instance from the default singleton")
	}
}

// TestFlatVectorScorerUtil_ScalarQuantizedWrapper verifies the
// GetLucene99ScalarQuantizedVectorsScorer factory returns a
// ScalarQuantizedVectorScorer wrapping the default.
func TestFlatVectorScorerUtil_ScalarQuantizedWrapper(t *testing.T) {
	got := hnsw.GetLucene99ScalarQuantizedVectorsScorer()
	if got == nil {
		t.Fatal("GetLucene99ScalarQuantizedVectorsScorer returned nil")
	}
	if _, ok := got.(*hnsw.ScalarQuantizedVectorScorer); !ok {
		t.Fatalf("expected *ScalarQuantizedVectorScorer, got %T", got)
	}
}

// TestScalarQuantizedVectorScorer_String verifies the canonical
// toString() output of the wrapper, mirroring Java's exact textual
// format.
func TestScalarQuantizedVectorScorer_String(t *testing.T) {
	s := hnsw.NewScalarQuantizedVectorScorer(hnsw.NewDefaultFlatVectorScorer())
	got := s.String()
	if !strings.HasPrefix(got, "ScalarQuantizedVectorScorer(nonQuantizedDelegate=") {
		t.Fatalf("String() = %q, missing expected prefix", got)
	}
	if !strings.HasSuffix(got, ")") {
		t.Fatalf("String() = %q, missing closing paren", got)
	}
}

// TestScalarQuantizedVectorScorer_NonQuantizedFallback verifies that
// the wrapper delegates to its inner scorer when the input is not
// quantized — the indexing/flush fallback path documented in the Java
// reference.
func TestScalarQuantizedVectorScorer_NonQuantizedFallback(t *testing.T) {
	vv := &fakeFloatVectorValues{dim: 3, vectors: [][]float32{{1, 2, 3}, {4, 5, 6}}}
	scorer := hnsw.NewScalarQuantizedVectorScorer(hnsw.NewDefaultFlatVectorScorer())

	supplier, err := scorer.GetRandomVectorScorerSupplier(
		index.VectorSimilarityFunctionEuclidean, vv,
	)
	if err != nil {
		t.Fatalf("GetRandomVectorScorerSupplier (fallback): %v", err)
	}
	if supplier == nil {
		t.Fatal("expected non-nil supplier from fallback path")
	}
}

// TestQuantizeQuery_LengthMismatch checks the explicit length guard
// added to QuantizeQuery, which is implicit in the Java reference via
// ScalarQuantizer.quantize's assertion.
func TestQuantizeQuery_LengthMismatch(t *testing.T) {
	// Pass a nil quantizer because the length check runs before the
	// quantizer is consulted.
	_, err := hnsw.QuantizeQuery([]float32{1, 2, 3}, make([]byte, 2), index.VectorSimilarityFunctionEuclidean, nil)
	if err == nil {
		t.Fatal("expected length-mismatch error")
	}
	if !strings.Contains(err.Error(), "length mismatch") {
		t.Fatalf("error %q does not mention length mismatch", err)
	}
}
