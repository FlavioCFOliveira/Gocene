// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package schema

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// VectorEncoding specifies how vector values are encoded.
type VectorEncoding int

const (
	// VectorEncodingByte stores vector values as signed bytes.
	VectorEncodingByte VectorEncoding = iota

	// VectorEncodingFloat32 stores vector values as IEEE 32-bit floating point.
	VectorEncodingFloat32
)

// String returns the string representation of the VectorEncoding.
func (ve VectorEncoding) String() string {
	switch ve {
	case VectorEncodingByte:
		return "BYTE"
	case VectorEncodingFloat32:
		return "FLOAT32"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", ve)
	}
}

// VectorSimilarityFunction specifies the distance function used for similarity calculation.
type VectorSimilarityFunction int

const (
	// VectorSimilarityFunctionEuclidean uses squared Euclidean distance.
	VectorSimilarityFunctionEuclidean VectorSimilarityFunction = iota

	// VectorSimilarityFunctionDotProduct uses dot product similarity.
	VectorSimilarityFunctionDotProduct

	// VectorSimilarityFunctionCosine uses cosine similarity.
	VectorSimilarityFunctionCosine

	// VectorSimilarityFunctionMaximumInnerProduct uses maximum inner product similarity.
	VectorSimilarityFunctionMaximumInnerProduct
)

// String returns the string representation of the VectorSimilarityFunction.
func (vsf VectorSimilarityFunction) String() string {
	switch vsf {
	case VectorSimilarityFunctionEuclidean:
		return "EUCLIDEAN"
	case VectorSimilarityFunctionDotProduct:
		return "DOT_PRODUCT"
	case VectorSimilarityFunctionCosine:
		return "COSINE"
	case VectorSimilarityFunctionMaximumInnerProduct:
		return "MAXIMUM_INNER_PRODUCT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", vsf)
	}
}

// Compare returns the similarity score between two equal-dimension float
// vectors under this similarity function. Higher scores are more similar; the
// result lies in (0, 1] for EUCLIDEAN, [0, 1] for DOT_PRODUCT and COSINE, and
// [0, +inf) for MAXIMUM_INNER_PRODUCT.
//
// The arithmetic mirrors org.apache.lucene.index.VectorSimilarityFunction.
// compare(float[], float[]) exactly (Lucene 10.4.0), forwarding to the same
// VectorUtil primitives. Panics on a dimension mismatch (as the primitives do).
func (vsf VectorSimilarityFunction) Compare(v1, v2 []float32) float32 {
	switch vsf {
	case VectorSimilarityFunctionEuclidean:
		return util.NormalizeDistanceToUnitInterval(util.SquareDistance(v1, v2))
	case VectorSimilarityFunctionDotProduct:
		return util.NormalizeToUnitInterval(util.DotProduct(v1, v2))
	case VectorSimilarityFunctionCosine:
		return util.NormalizeToUnitInterval(util.Cosine(v1, v2))
	case VectorSimilarityFunctionMaximumInnerProduct:
		return util.ScaleMaxInnerProductScore(util.DotProduct(v1, v2))
	default:
		return 0
	}
}

// CompareBytes returns the similarity score between two equal-dimension
// signed-byte vectors under this similarity function.
//
// The arithmetic mirrors org.apache.lucene.index.VectorSimilarityFunction.
// compare(byte[], byte[]) exactly (Lucene 10.4.0). Panics on a dimension
// mismatch (as the primitives do).
func (vsf VectorSimilarityFunction) CompareBytes(v1, v2 []byte) float32 {
	switch vsf {
	case VectorSimilarityFunctionEuclidean:
		return 1 / (1 + float32(util.SquareDistanceBytes(v1, v2)))
	case VectorSimilarityFunctionDotProduct:
		return util.DotProductScore(v1, v2)
	case VectorSimilarityFunctionCosine:
		return (1 + util.CosineBytes(v1, v2)) / 2
	case VectorSimilarityFunctionMaximumInnerProduct:
		return util.ScaleMaxInnerProductScore(float32(util.DotProductBytes(v1, v2)))
	default:
		return 0
	}
}
