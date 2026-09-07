// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/schema"

// VectorEncoding is an alias of schema.VectorEncoding.
// It defines the numeric datatype of the vector values.
type VectorEncoding = schema.VectorEncoding

const (
	// VectorEncodingByte encodes vector using 8 bits of precision per sample.
	// Values provided with higher precision (eg: queries provided as float)
	// *must* be in the range [-128, 127].
	VectorEncodingByte = schema.VectorEncodingByte

	// VectorEncodingFloat32 encodes vector using 32 bits of precision per sample
	// in IEEE floating point format.
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
