// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package schema

import (
	"math"
	"testing"
)

// TestVectorSimilarityFunction_Compare locks the float similarity formulas to
// org.apache.lucene.index.VectorSimilarityFunction.compare(float[], float[])
// (Lucene 10.4.0). The expected values are computed from the Lucene definitions
// independently of the implementation.
func TestVectorSimilarityFunction_Compare(t *testing.T) {
	const eps = 1e-6
	a := []float32{2, 3}
	b := []float32{1, 1}

	tests := []struct {
		name string
		sim  VectorSimilarityFunction
		want float64
	}{
		{
			// 1 / (1 + squareDistance); squareDistance = 1+4 = 5.
			name: "euclidean",
			sim:  VectorSimilarityFunctionEuclidean,
			want: 1.0 / (1.0 + 5.0),
		},
		{
			// (1 + dot) / 2; dot = 2*1 + 3*1 = 5.
			name: "dot_product",
			sim:  VectorSimilarityFunctionDotProduct,
			want: (1.0 + 5.0) / 2.0,
		},
		{
			// (1 + cosine) / 2; cosine = 5 / (sqrt(13) * sqrt(2)).
			name: "cosine",
			sim:  VectorSimilarityFunctionCosine,
			want: (1.0 + 5.0/(math.Sqrt(13)*math.Sqrt(2))) / 2.0,
		},
		{
			// scaleMaxInnerProductScore(dot) with dot = 5 >= 0 -> dot + 1.
			name: "max_inner_product",
			sim:  VectorSimilarityFunctionMaximumInnerProduct,
			want: 5.0 + 1.0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.sim.Compare(a, b)
			if math.Abs(float64(got)-tc.want) > eps {
				t.Errorf("Compare = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestVectorSimilarityFunction_CompareBytes locks the byte similarity formulas
// to VectorSimilarityFunction.compare(byte[], byte[]).
func TestVectorSimilarityFunction_CompareBytes(t *testing.T) {
	const eps = 1e-6
	a := []byte{2, 3}
	b := []byte{1, 1}

	// EUCLIDEAN: 1 / (1 + squareDistanceBytes); squareDistance = 1+4 = 5.
	if got, want := vsfCompareBytes(t, VectorSimilarityFunctionEuclidean, a, b), 1.0/(1.0+5.0); math.Abs(got-want) > eps {
		t.Errorf("euclidean = %v, want %v", got, want)
	}
	// DOT_PRODUCT: dotProductScore = 0.5 + dot / (len * 2^15); dot = 5, len = 2.
	wantDot := 0.5 + 5.0/(2.0*float64(int(1)<<15))
	if got := vsfCompareBytes(t, VectorSimilarityFunctionDotProduct, a, b); math.Abs(got-wantDot) > eps {
		t.Errorf("dot_product = %v, want %v", got, wantDot)
	}
	// COSINE: (1 + cosine) / 2; cosine = 5 / (sqrt(13) * sqrt(2)).
	wantCos := (1.0 + 5.0/(math.Sqrt(13)*math.Sqrt(2))) / 2.0
	if got := vsfCompareBytes(t, VectorSimilarityFunctionCosine, a, b); math.Abs(got-wantCos) > eps {
		t.Errorf("cosine = %v, want %v", got, wantCos)
	}
	// MAXIMUM_INNER_PRODUCT: scaleMaxInnerProductScore(dot); dot = 5 >= 0 -> 6.
	if got, want := vsfCompareBytes(t, VectorSimilarityFunctionMaximumInnerProduct, a, b), 6.0; math.Abs(got-want) > eps {
		t.Errorf("max_inner_product = %v, want %v", got, want)
	}
}

func vsfCompareBytes(t *testing.T, sim VectorSimilarityFunction, a, b []byte) float64 {
	t.Helper()
	return float64(sim.CompareBytes(a, b))
}
