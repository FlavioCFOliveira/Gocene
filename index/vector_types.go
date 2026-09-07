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

// VectorEncodingByteSize returns the number of bytes required to encode a
// scalar in the given format. A vector will nominally require
// dimension * byteSize bytes of storage.
//
// PORT NOTE: Java exposes this as the final field VectorEncoding.byteSize
// (BYTE = 1, FLOAT32 = 4). VectorEncoding is a Go alias of
// schema.VectorEncoding, and Go forbids declaring methods on a non-local
// type, so the accessor is a package-level function here.
func VectorEncodingByteSize(ve VectorEncoding) int {
	switch ve {
	case VectorEncodingByte:
		return 1
	case VectorEncodingFloat32:
		return 4
	default:
		return 0
	}
}

const (
	VectorSimilarityFunctionEuclidean           = schema.VectorSimilarityFunctionEuclidean
	VectorSimilarityFunctionDotProduct          = schema.VectorSimilarityFunctionDotProduct
	VectorSimilarityFunctionCosine              = schema.VectorSimilarityFunctionCosine
	VectorSimilarityFunctionMaximumInnerProduct = schema.VectorSimilarityFunctionMaximumInnerProduct
)
