// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/schema"

// This file is the index-side facade for the VectorEncoding /
// VectorSimilarityFunction enums after the SPI unification (rmp #4669 /
// Sprint 117 phase 1.2). The canonical declaration site lives in
// schema/; index/ re-exports the types as Go aliases and re-declares
// the constants as values of the aliased types.

// VectorEncoding is an alias of schema.VectorEncoding.
type VectorEncoding = schema.VectorEncoding

// VectorSimilarityFunction is an alias of schema.VectorSimilarityFunction.
type VectorSimilarityFunction = schema.VectorSimilarityFunction

const (
	VectorEncodingByte    = schema.VectorEncodingByte
	VectorEncodingFloat32 = schema.VectorEncodingFloat32
)

const (
	VectorSimilarityFunctionEuclidean           = schema.VectorSimilarityFunctionEuclidean
	VectorSimilarityFunctionDotProduct          = schema.VectorSimilarityFunctionDotProduct
	VectorSimilarityFunctionCosine              = schema.VectorSimilarityFunctionCosine
	VectorSimilarityFunctionMaximumInnerProduct = schema.VectorSimilarityFunctionMaximumInnerProduct
)
