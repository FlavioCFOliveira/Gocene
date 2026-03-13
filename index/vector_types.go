// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

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
