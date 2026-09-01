// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// VectorEncoding describes the numeric datatype of the vector values.
// Mirrors org.apache.lucene.index.VectorEncoding from Apache Lucene 10.5.0.
type VectorEncoding int

const (
	// VectorEncodingByte encodes vector using 8 bits of precision per sample.
	VectorEncodingByte VectorEncoding = iota
	// VectorEncodingFloat32 encodes vector using 32 bits of precision per sample.
	VectorEncodingFloat32
)

// ByteSize returns the number of bytes required to encode a scalar in this format.
func (e VectorEncoding) ByteSize() int {
	switch e {
	case VectorEncodingByte:
		return 1
	case VectorEncodingFloat32:
		return 4
	default:
		panic("unknown VectorEncoding")
	}
}
