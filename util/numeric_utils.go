// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
)

// NumericUtils provides helper APIs to encode numeric values as sortable bytes and vice-versa.
//
// To also index floating point numbers, this class supplies two methods to convert them to
// integer values by changing their bit layout: DoubleToSortableLong, FloatToSortableInt.
// You will have no precision loss by converting floating point numbers to integers and back
// (only that the integer form is not usable). Other data types like dates can be easily
// converted to longs or ints.
//
// This is the Go port of Lucene's org.apache.lucene.util.NumericUtils.
type NumericUtils struct{}

// DoubleToSortableLong converts a double value to a sortable signed long.
// The value is converted by getting their IEEE 754 floating-point "double format" bit layout
// and then some bits are swapped, to be able to compare the result as long. By this the
// precision is not reduced, but the value can easily be used as a long. The sort order
// (including NaN) is defined by Double.Compare; NaN is greater than positive infinity.
//
// See SortableLongToDouble for the reverse conversion.
func DoubleToSortableLong(value float64) int64 {
	return SortableDoubleBits(math.Float64bits(value))
}

// SortableLongToDouble converts a sortable long back to a double.
//
// See DoubleToSortableLong for the reverse conversion.
func SortableLongToDouble(encoded int64) float64 {
	// Apply the same bit transformation to get back original bits
	bits := uint64(encoded)
	// Reverse: bits ^ ((bits >> 63) & 0x7fffffffffffffff)
	// For the reverse, we need to apply the XOR again
	return math.Float64frombits(bits ^ ((bits >> 63) & 0x7fffffffffffffff))
}

// FloatToSortableInt converts a float value to a sortable signed int.
// The value is converted by getting their IEEE 754 floating-point "float format" bit layout
// and then some bits are swapped, to be able to compare the result as int. By this the
// precision is not reduced, but the value can easily be used as an int. The sort order
// (including NaN) is defined by Float.Compare; NaN is greater than positive infinity.
//
// See SortableIntToFloat for the reverse conversion.
func FloatToSortableInt(value float32) int32 {
	return SortableFloatBits(math.Float32bits(value))
}

// SortableIntToFloat converts a sortable int back to a float.
//
// See FloatToSortableInt for the reverse conversion.
func SortableIntToFloat(encoded int32) float32 {
	// Apply the same bit transformation to get back original bits
	bits := uint32(encoded)
	return math.Float32frombits(bits ^ ((bits >> 31) & 0x7fffffff))
}

// SortableDoubleBits converts IEEE 754 representation of a double to sortable order
// (or back to the original). This is a bidirectional transformation.
func SortableDoubleBits(bits uint64) int64 {
	return int64(bits ^ ((bits >> 63) & 0x7fffffffffffffff))
}

// SortableFloatBits converts IEEE 754 representation of a float to sortable order
// (or back to the original). This is a bidirectional transformation.
func SortableFloatBits(bits uint32) int32 {
	return int32(bits ^ ((bits >> 31) & 0x7fffffff))
}

// Subtract computes result = a - b, where a >= b.
// If a < b, an error is returned.
// The dim parameter specifies which dimension to subtract (for multi-dimensional arrays).
func Subtract(bytesPerDim, dim int, a, b, result []byte) error {
	start := dim * bytesPerDim
	end := start + bytesPerDim

	borrow := 0
	var i int

	// Process bytes one at a time for the remainder
	limit := start + (bytesPerDim & ^3)
	for i = end - 1; i >= limit; i-- {
		diff := int(a[i]) - int(b[i]) - borrow
		if diff < 0 {
			borrow = 1
			diff += 256
		} else {
			borrow = 0
		}
		result[i-start] = byte(diff)
	}

	// Process 4 bytes at a time using big-endian int interpretation
	for i -= 3; i >= start; i -= 4 {
		aInt := binary.BigEndian.Uint32(a[i:])
		bInt := binary.BigEndian.Uint32(b[i:])

		diff := uint64(aInt) - uint64(bInt) - uint64(borrow)
		if diff > 0xffffffff {
			borrow = 1
			diff += 0x100000000
		} else {
			borrow = 0
		}

		binary.BigEndian.PutUint32(result[i-start:], uint32(diff))
	}

	if borrow != 0 {
		return errors.New("a < b")
	}
	return nil
}

// Add computes result = a + b, where a and b are unsigned.
// If there is an overflow, an error is returned.
// The dim parameter specifies which dimension to add (for multi-dimensional arrays).
func Add(bytesPerDim, dim int, a, b, result []byte) error {
	start := dim * bytesPerDim
	end := start + bytesPerDim

	carry := 0
	var i int

	// Process bytes one at a time for the remainder
	limit := start + (bytesPerDim & ^3)
	for i = end - 1; i >= limit; i-- {
		digitSum := int(a[i]) + int(b[i]) + carry
		if digitSum >= 256 {
			carry = 1
			digitSum -= 256
		} else {
			carry = 0
		}
		result[i-start] = byte(digitSum)
	}

	// Process 4 bytes at a time using big-endian int interpretation
	for i -= 3; i >= start; i -= 4 {
		aInt := binary.BigEndian.Uint32(a[i:])
		bInt := binary.BigEndian.Uint32(b[i:])

		digitSum := uint64(aInt) + uint64(bInt) + uint64(carry)
		if digitSum >= 0x100000000 {
			carry = 1
			digitSum -= 0x100000000
		} else {
			carry = 0
		}

		binary.BigEndian.PutUint32(result[i-start:], uint32(digitSum))
	}

	if carry != 0 {
		return errors.New("a + b overflows bytesPerDim=" + string(rune(bytesPerDim)))
	}
	return nil
}

// IntToSortableBytes encodes an integer value such that unsigned byte order comparison
// is consistent with Integer.Compare.
//
// See SortableBytesToInt for the reverse conversion.
func IntToSortableBytes(value int32, result []byte, offset int) {
	// Flip the sign bit, so negative ints sort before positive ints correctly
	// Convert to uint32 first to avoid overflow
	binary.BigEndian.PutUint32(result[offset:], uint32(value)^0x80000000)
}

// SortableBytesToInt decodes an integer value previously written with IntToSortableBytes.
//
// See IntToSortableBytes for the reverse conversion.
func SortableBytesToInt(encoded []byte, offset int) int32 {
	x := binary.BigEndian.Uint32(encoded[offset:])
	// Re-flip the sign bit to restore the original value
	return int32(x ^ uint32(0x80000000))
}

// LongToSortableBytes encodes a long value such that unsigned byte order comparison
// is consistent with Long.Compare.
//
// See SortableBytesToLong for the reverse conversion.
func LongToSortableBytes(value int64, result []byte, offset int) {
	// Flip the sign bit so negative longs sort before positive longs
	// Convert to uint64 first to avoid overflow
	binary.BigEndian.PutUint64(result[offset:], uint64(value)^0x8000000000000000)
}

// SortableBytesToLong decodes a long value previously written with LongToSortableBytes.
//
// See LongToSortableBytes for the reverse conversion.
func SortableBytesToLong(encoded []byte, offset int) int64 {
	v := binary.BigEndian.Uint64(encoded[offset:])
	// Flip the sign bit back
	return int64(v ^ uint64(0x8000000000000000))
}

// BigIntToSortableBytes encodes a BigInteger value such that unsigned byte order comparison
// is consistent with BigInteger.CompareTo. This also sign-extends the value to bigIntSize
// bytes if necessary: useful to create a fixed-width size.
//
// See SortableBytesToBigInt for the reverse conversion.
func BigIntToSortableBytes(bigInt *big.Int, bigIntSize int, result []byte, offset int) error {
	bigIntBytes := bigInt.Bytes()
	fullBigIntBytes := make([]byte, bigIntSize)

	if len(bigIntBytes) < bigIntSize {
		// Copy bytes to the end of fullBigIntBytes
		copy(fullBigIntBytes[bigIntSize-len(bigIntBytes):], bigIntBytes)
		// Sign extend if negative
		if len(bigIntBytes) > 0 && (bigIntBytes[0]&0x80) != 0 {
			for i := 0; i < bigIntSize-len(bigIntBytes); i++ {
				fullBigIntBytes[i] = 0xff
			}
		}
	} else if len(bigIntBytes) == bigIntSize {
		copy(fullBigIntBytes, bigIntBytes)
	} else {
		return errors.New("BigInteger requires more than " + string(rune(bigIntSize)) + " bytes storage")
	}

	// Flip the sign bit so negative bigints sort before positive bigints
	fullBigIntBytes[0] ^= 0x80

	copy(result[offset:], fullBigIntBytes)
	return nil
}

// SortableBytesToBigInt decodes a BigInteger value previously written with BigIntToSortableBytes.
//
// See BigIntToSortableBytes for the reverse conversion.
func SortableBytesToBigInt(encoded []byte, offset, length int) *big.Int {
	bigIntBytes := make([]byte, length)
	copy(bigIntBytes, encoded[offset:offset+length])
	// Flip the sign bit back to the original
	bigIntBytes[0] ^= 0x80
	return new(big.Int).SetBytes(bigIntBytes)
}
