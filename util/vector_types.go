// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
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

// ParseVectorEncoding resolves the enum constant named name, mirroring the
// implicit java.lang.Enum member
// org.apache.lucene.index.VectorEncoding#valueOf(String), called by
// SimpleTextFieldInfosFormat.vectorEncoding. It is the exact inverse of
// [VectorEncoding.String].
func ParseVectorEncoding(name string) (VectorEncoding, error) {
	switch name {
	case "BYTE":
		return VectorEncodingByte, nil
	case "FLOAT32":
		return VectorEncodingFloat32, nil
	default:
		return 0, fmt.Errorf("no enum constant VectorEncoding.%s", name)
	}
}

// VectorSimilarityID is the integer identifier for a similarity function, used for serialization.
type VectorSimilarityID int

const (
	// VectorSimilarityIDEuclidean uses squared Euclidean distance.
	VectorSimilarityIDEuclidean VectorSimilarityID = iota
	// VectorSimilarityIDDotProduct uses dot product similarity.
	VectorSimilarityIDDotProduct
	// VectorSimilarityIDCosine uses cosine similarity.
	VectorSimilarityIDCosine
	// VectorSimilarityIDMaximumInnerProduct uses maximum inner product similarity.
	VectorSimilarityIDMaximumInnerProduct
)

// String returns the string representation of the VectorSimilarityID.
func (vsid VectorSimilarityID) String() string {
	switch vsid {
	case VectorSimilarityIDEuclidean:
		return "EUCLIDEAN"
	case VectorSimilarityIDDotProduct:
		return "DOT_PRODUCT"
	case VectorSimilarityIDCosine:
		return "COSINE"
	case VectorSimilarityIDMaximumInnerProduct:
		return "MAXIMUM_INNER_PRODUCT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", vsid)
	}
}

// ParseVectorSimilarityID resolves the enum constant named name, mirroring the
// implicit java.lang.Enum member
// org.apache.lucene.index.VectorSimilarityFunction#valueOf(String), called by
// SimpleTextFieldInfosFormat.distanceFunction. Gocene splits Java's enum into
// the [VectorSimilarityFunction] behaviour interface and this ordinal, so the
// name resolves to the ordinal; [GetSimilarityFunction] then yields the
// function itself. It is the exact inverse of [VectorSimilarityID.String].
func ParseVectorSimilarityID(name string) (VectorSimilarityID, error) {
	switch name {
	case "EUCLIDEAN":
		return VectorSimilarityIDEuclidean, nil
	case "DOT_PRODUCT":
		return VectorSimilarityIDDotProduct, nil
	case "COSINE":
		return VectorSimilarityIDCosine, nil
	case "MAXIMUM_INNER_PRODUCT":
		return VectorSimilarityIDMaximumInnerProduct, nil
	default:
		return 0, fmt.Errorf("no enum constant VectorSimilarityFunction.%s", name)
	}
}
