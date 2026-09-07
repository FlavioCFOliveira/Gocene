// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/util"

// Compare calculates a similarity score between the two float32 vectors with a specified function.
// Higher similarity scores correspond to closer vectors.
//
// PORT NOTE: Mirrors Lucene's VectorSimilarityFunction.compare(float[], float[]).
func Compare(fn VectorSimilarityFunction, v1, v2 []float32) float32 {
	switch fn {
	case VectorSimilarityFunctionEuclidean:
		return util.NormalizeDistanceToUnitInterval(util.SquareDistance(v1, v2))
	case VectorSimilarityFunctionDotProduct:
		return util.NormalizeToUnitInterval(util.DotProduct(v1, v2))
	case VectorSimilarityFunctionCosine:
		return util.NormalizeToUnitInterval(util.Cosine(v1, v2))
	case VectorSimilarityFunctionMaximumInnerProduct:
		return util.ScaleMaxInnerProductScore(util.DotProduct(v1, v2))
	default:
		panic("unknown vector similarity function")
	}
}

// CompareBytes calculates a similarity score between the two signed-byte vectors with a specified function.
// Higher similarity scores correspond to closer vectors.
//
// PORT NOTE: Mirrors Lucene's VectorSimilarityFunction.compare(byte[], byte[]).
func CompareBytes(fn VectorSimilarityFunction, v1, v2 []byte) float32 {
	switch fn {
	case VectorSimilarityFunctionEuclidean:
		return util.NormalizeDistanceToUnitInterval(util.SquareDistanceBytes(v1, v2))
	case VectorSimilarityFunctionDotProduct:
		return util.DotProductScore(v1, v2)
	case VectorSimilarityFunctionCosine:
		return util.NormalizeToUnitInterval(util.CosineBytes(v1, v2))
	case VectorSimilarityFunctionMaximumInnerProduct:
		return util.ScaleMaxInnerProductScore(util.DotProductBytes(v1, v2))
	default:
		panic("unknown vector similarity function")
	}
}
