// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "math"

// SmallFloat provides methods for encoding floats in a single byte.
//
// This is used for encoding norms (document boost * field norm).
//
// The encoding uses 3 bits of mantissa and 5 bits of exponent.
// This allows values from 0 to about 256 to be encoded in a single byte.
type SmallFloat struct{}

// FloatToByte315 encodes a float32 into a byte using 3 bits mantissa, 5 bits exponent.
func FloatToByte315(f float32) byte {
	bits := math.Float32bits(f)
	if bits == 0 {
		return 0
	}

	// Extract exponent and mantissa
	exponent := int((bits>>23)&0xFF) - 127 + 1
	mantissa := bits & 0x7FFFFF

	// Round to 3 bits of mantissa
	mantissa3 := (mantissa >> 20) & 0x7

	// Shift exponent to fit in 5 bits
	exp5 := exponent + 15
	if exp5 < 0 {
		exp5 = 0
	}
	if exp5 > 31 {
		exp5 = 31
	}

	return byte((uint32(exp5) << 3) | mantissa3)
}

// ByteToFloat315 decodes a byte into a float32 using 3 bits mantissa, 5 bits exponent.
func ByteToFloat315(b byte) float32 {
	if b == 0 {
		return 0
	}

	exp5 := int(b >> 3)
	mantissa3 := uint32(b & 0x7)

	exponent := exp5 - 15 + 127 - 1
	mantissa := mantissa3 << 20
	bits := (uint32(exponent) << 23) | mantissa

	return math.Float32frombits(bits)
}

// FloatToByte52 encodes a float32 into a byte using 5 bits mantissa, 2 bits exponent.
func FloatToByte52(f float32) byte {
	bits := math.Float32bits(f)
	if bits == 0 {
		return 0
	}

	exponent := int((bits>>23)&0xFF) - 127 + 1
	mantissa := bits & 0x7FFFFF

	mantissa5 := (mantissa >> 18) & 0x1F

	exp2 := exponent + 1
	if exp2 < 0 {
		exp2 = 0
	}
	if exp2 > 3 {
		exp2 = 3
	}

	return byte((uint32(exp2) << 5) | mantissa5)
}

// ByteToFloat52 decodes a byte into a float32 using 5 bits mantissa, 2 bits exponent.
func ByteToFloat52(b byte) float32 {
	if b == 0 {
		return 0
	}

	exp2 := int(b >> 5)
	mantissa5 := uint32(b & 0x1F)

	exponent := exp2 - 1 + 127 - 1
	mantissa := mantissa5 << 18

	bits := (uint32(exponent) << 23) | mantissa
	return math.Float32frombits(bits)
}
