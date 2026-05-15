// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package bkd

import (
	"encoding/binary"
	"math/bits"
)

// Port of org.apache.lucene.util.bkd.BKDUtil (Lucene 10.4.0).
//
// Utility helpers used by the BKD writer to compare leaf-block byte
// runs for equality and to compute the length of the common prefix
// between two byte runs. The Lucene implementation specialises for
// numBytes == 4 and numBytes == 8 (the two most common point widths)
// and falls back to a generic loop for everything else.
//
// The Go port keeps the same dispatch shape: explicit specialisations
// for 4 and 8 bytes that read the two values as little-endian
// machine words (matching Java's VH_LE_INT / VH_LE_LONG), and a
// generic helper for arbitrary widths.

// ByteArrayComparator compares two byte slices starting at the
// supplied offsets and returns the length of their common prefix.
// Mirrors the functional interface
// org.apache.lucene.util.ArrayUtil.ByteArrayComparator.
type ByteArrayComparator func(a []byte, aOffset int, b []byte, bOffset int) int

// ByteArrayPredicate tests two byte slices starting at the supplied
// offsets for equality across a fixed number of bytes. Mirrors
// BKDUtil.ByteArrayPredicate in Lucene.
type ByteArrayPredicate func(a []byte, aOffset int, b []byte, bOffset int) bool

// GetPrefixLengthComparator returns a comparator that computes the
// common prefix length across the next numBytes of the provided
// arrays. Mirrors BKDUtil.getPrefixLengthComparator.
//
// numBytes == 4 and numBytes == 8 are specialised to read the two
// values as little-endian machine words and XOR them, locating the
// first differing byte via the count of leading zero bits.
func GetPrefixLengthComparator(numBytes int) ByteArrayComparator {
	switch numBytes {
	case 8:
		// Used by LongPoint, DoublePoint.
		return CommonPrefixLength8
	case 4:
		// Used by IntPoint, FloatPoint, LatLonPoint, LatLonShape.
		return CommonPrefixLength4
	default:
		n := numBytes
		return func(a []byte, aOffset int, b []byte, bOffset int) int {
			return CommonPrefixLengthN(a, aOffset, b, bOffset, n)
		}
	}
}

// CommonPrefixLength8 returns the length of the common prefix across
// the next 8 bytes of both provided arrays. Reads the values as
// little-endian uint64 to match Lucene 10.4.0's
// BitUtil.VH_LE_LONG.get(...).
func CommonPrefixLength8(a []byte, aOffset int, b []byte, bOffset int) int {
	aLong := binary.LittleEndian.Uint64(a[aOffset : aOffset+8])
	bLong := binary.LittleEndian.Uint64(b[bOffset : bOffset+8])
	// XOR the two values; the position of the first differing byte
	// is given by the number of leading zero bits, divided by 8.
	// Java's commonPrefixInBits uses Long.reverseBytes before
	// numberOfLeadingZeros because Java VarHandle.get returns the
	// value in the requested endianness regardless of platform — the
	// "reverseBytes" call there is a compatibility shim for
	// the macroscopic algorithm originally written assuming BE.
	// In Go we can skip the reverse: TrailingZeros gives us the
	// position of the first differing low byte directly when the
	// inputs are already LE-loaded.
	x := aLong ^ bLong
	if x == 0 {
		return 8
	}
	return bits.TrailingZeros64(x) >> 3
}

// CommonPrefixLength4 returns the length of the common prefix across
// the next 4 bytes of both provided arrays. Reads the values as
// little-endian uint32 to match Lucene 10.4.0's BitUtil.VH_LE_INT.
func CommonPrefixLength4(a []byte, aOffset int, b []byte, bOffset int) int {
	aInt := binary.LittleEndian.Uint32(a[aOffset : aOffset+4])
	bInt := binary.LittleEndian.Uint32(b[bOffset : bOffset+4])
	x := aInt ^ bInt
	if x == 0 {
		return 4
	}
	return bits.TrailingZeros32(x) >> 3
}

// CommonPrefixLengthN returns the length of the common prefix across
// numBytes of the two arrays. Mirrors BKDUtil.commonPrefixLengthN,
// which delegates to Arrays.mismatch.
func CommonPrefixLengthN(a []byte, aOffset int, b []byte, bOffset int, numBytes int) int {
	for i := 0; i < numBytes; i++ {
		if a[aOffset+i] != b[bOffset+i] {
			return i
		}
	}
	return numBytes
}

// GetEqualsPredicate returns a predicate that tells whether the next
// numBytes bytes are equal. Mirrors BKDUtil.getEqualsPredicate.
func GetEqualsPredicate(numBytes int) ByteArrayPredicate {
	switch numBytes {
	case 8:
		return Equals8
	case 4:
		return Equals4
	default:
		n := numBytes
		return func(a []byte, aOffset int, b []byte, bOffset int) bool {
			for i := 0; i < n; i++ {
				if a[aOffset+i] != b[bOffset+i] {
					return false
				}
			}
			return true
		}
	}
}

// Equals8 reports whether the next 8 bytes are exactly the same in
// the provided arrays. Reads the values as little-endian uint64.
func Equals8(a []byte, aOffset int, b []byte, bOffset int) bool {
	aLong := binary.LittleEndian.Uint64(a[aOffset : aOffset+8])
	bLong := binary.LittleEndian.Uint64(b[bOffset : bOffset+8])
	return aLong == bLong
}

// Equals4 reports whether the next 4 bytes are exactly the same in
// the provided arrays. Reads the values as little-endian uint32.
func Equals4(a []byte, aOffset int, b []byte, bOffset int) bool {
	aInt := binary.LittleEndian.Uint32(a[aOffset : aOffset+4])
	bInt := binary.LittleEndian.Uint32(b[bOffset : bOffset+4])
	return aInt == bInt
}
