// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// BitUtil provides high efficiency bit twiddling routines and encoders for primitives.
// This is a port of Apache Lucene's BitUtil class.

// Magic numbers for bit interleaving
const (
	magic0 uint64 = 0x5555555555555555
	magic1 uint64 = 0x3333333333333333
	magic2 uint64 = 0x0F0F0F0F0F0F0F0F
	magic3 uint64 = 0x00FF00FF00FF00FF
	magic4 uint64 = 0x0000FFFF0000FFFF
	magic5 uint64 = 0x00000000FFFFFFFF
	magic6 uint64 = 0xAAAAAAAAAAAAAAAA

	shift0 uint64 = 1
	shift1 uint64 = 2
	shift2 uint64 = 4
	shift3 uint64 = 8
	shift4 uint64 = 16
)

// NextHighestPowerOfTwo returns the next highest power of two, or the current value
// if it's already a power of two or zero.
func NextHighestPowerOfTwo(v int) int {
	if v <= 0 {
		return v
	}
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}

// NextHighestPowerOfTwoInt64 returns the next highest power of two, or the current value
// if it's already a power of two or zero.
func NextHighestPowerOfTwoInt64(v int64) int64 {
	if v <= 0 {
		return v
	}
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}

// Interleave interleaves the first 32 bits of each int value.
// Adapted from: http://graphics.stanford.edu/~seander/bithacks.html#InterleaveBMN
func Interleave(even, odd int) int64 {
	v1 := uint64(uint32(even))
	v2 := uint64(uint32(odd))

	v1 = (v1 | (v1 << shift4)) & magic4
	v1 = (v1 | (v1 << shift3)) & magic3
	v1 = (v1 | (v1 << shift2)) & magic2
	v1 = (v1 | (v1 << shift1)) & magic1
	v1 = (v1 | (v1 << shift0)) & magic0

	v2 = (v2 | (v2 << shift4)) & magic4
	v2 = (v2 | (v2 << shift3)) & magic3
	v2 = (v2 | (v2 << shift2)) & magic2
	v2 = (v2 | (v2 << shift1)) & magic1
	v2 = (v2 | (v2 << shift0)) & magic0

	return int64((v2 << 1) | v1)
}

// Deinterleave extracts just the even-bits value as a long from the bit-interleaved value.
func Deinterleave(b int64) int64 {
	v := uint64(b)
	v &= magic0
	v = (v ^ (v >> shift0)) & magic1
	v = (v ^ (v >> shift1)) & magic2
	v = (v ^ (v >> shift2)) & magic3
	v = (v ^ (v >> shift3)) & magic4
	v = (v ^ (v >> shift4)) & magic5
	return int64(v)
}

// FlipFlop flip flops odd with even bits.
func FlipFlop(b int64) int64 {
	v := uint64(b)
	return int64(((v & magic6) >> 1) | ((v & magic0) << 1))
}

// ZigZagEncodeInt encodes the provided int using zig-zag encoding.
// This is useful for signed integers where small absolute values
// should have small encoded values.
// See: https://developers.google.com/protocol-buffers/docs/encoding#types
func ZigZagEncodeInt(i int) int {
	return (i >> 31) ^ (i << 1)
}

// ZigZagEncodeInt64 encodes the provided int64 using zig-zag encoding.
// This is useful for signed integers where small absolute values
// should have small encoded values.
// See: https://developers.google.com/protocol-buffers/docs/encoding#types
func ZigZagEncodeInt64(l int64) int64 {
	return (l >> 63) ^ (l << 1)
}

// ZigZagDecodeInt decodes an int previously encoded with ZigZagEncodeInt.
func ZigZagDecodeInt(i int) int {
	return ((i >> 1) ^ -(i & 1))
}

// ZigZagDecodeInt64 decodes an int64 previously encoded with ZigZagEncodeInt64.
func ZigZagDecodeInt64(l int64) int64 {
	return ((l >> 1) ^ -(l & 1))
}

// IsZeroOrPowerOfTwo returns true if, and only if, the provided integer - treated as
// an unsigned integer - is either 0 or a power of two.
func IsZeroOrPowerOfTwo(x int) bool {
	return (x & (x - 1)) == 0
}

// IsZeroOrPowerOfTwoInt64 returns true if, and only if, the provided int64 - treated as
// an unsigned int64 - is either 0 or a power of two.
func IsZeroOrPowerOfTwoInt64(x int64) bool {
	return (x & (x - 1)) == 0
}
