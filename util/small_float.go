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

package util

import (
	"fmt"
	"math"
	"math/bits"
)

// SmallFloat: floating point numbers smaller than 32 bits.
//
// This is the Go port of org.apache.lucene.util.SmallFloat. The 8-bit
// encoders/decoders are bit-exact with Lucene 10.4.0: for every
// b in [0, 255], FloatToByte(ByteToFloat(b)) == b.
//
// Values less than zero are mapped to byte 0. Underflow (positive but
// smaller than the smallest representable value) is rounded up to the
// smallest non-zero byte (1). Overflow saturates at byte 255 (Java
// returned -1 from a signed byte; that bit pattern is 0xFF on the
// wire).

// FloatToByte converts a float32 to an 8-bit float using the generic
// (numMantissaBits, zeroExp) encoding. Mirrors
// SmallFloat.floatToByte exactly: rounding is truncation toward
// negative infinity (right-shift of the raw IEEE-754 bits).
func FloatToByte(f float32, numMantissaBits, zeroExp int) byte {
	fzero := (63 - zeroExp) << numMantissaBits
	bits := int32(math.Float32bits(f))
	smallfloat := int(bits >> (24 - numMantissaBits))
	switch {
	case smallfloat <= fzero:
		if bits <= 0 {
			return 0
		}
		return 1
	case smallfloat >= fzero+0x100:
		return 0xFF
	default:
		return byte(smallfloat - fzero)
	}
}

// ByteToFloat converts an 8-bit float (produced by FloatToByte) back
// to float32. Mirrors SmallFloat.byteToFloat exactly.
func ByteToFloat(b byte, numMantissaBits, zeroExp int) float32 {
	if b == 0 {
		return 0
	}
	x := uint32(b) << (24 - numMantissaBits)
	x += uint32(63-zeroExp) << 24
	return math.Float32frombits(x)
}

// FloatToByte315 is the specialisation of FloatToByte with
// numMantissaBits=3 and zeroExp=15. Smallest non-zero value
// 5.820766E-10; largest 7.5161928E9; epsilon 0.125.
func FloatToByte315(f float32) byte {
	bits := int32(math.Float32bits(f))
	smallfloat := int(bits >> (24 - 3))
	const fzero = (63 - 15) << 3
	switch {
	case smallfloat <= fzero:
		if bits <= 0 {
			return 0
		}
		return 1
	case smallfloat >= fzero+0x100:
		return 0xFF
	default:
		return byte(smallfloat - fzero)
	}
}

// Byte315ToFloat is the specialisation of ByteToFloat with
// numMantissaBits=3 and zeroExp=15.
func Byte315ToFloat(b byte) float32 {
	if b == 0 {
		return 0
	}
	x := uint32(b) << (24 - 3)
	x += uint32(63-15) << 24
	return math.Float32frombits(x)
}

// ByteToFloat315 is an alias of Byte315ToFloat kept for backwards
// compatibility with earlier Gocene call sites.
func ByteToFloat315(b byte) float32 { return Byte315ToFloat(b) }

// FloatToByte52 is the historical (5 mantissa, 2 exponent)
// specialisation provided for backwards compatibility with earlier
// Gocene call sites. Lucene 10.4.0 no longer ships this variant; the
// encoding follows the same generic formula as FloatToByte with
// numMantissaBits=5 and zeroExp=2.
func FloatToByte52(f float32) byte { return FloatToByte(f, 5, 2) }

// ByteToFloat52 is the inverse of FloatToByte52.
func ByteToFloat52(b byte) float32 { return ByteToFloat(b, 5, 2) }

// LongToInt4 encodes a non-negative int64 in an order-preserving int
// with 4 significant bits.
func LongToInt4(i int64) (int, error) {
	if i < 0 {
		return 0, fmt.Errorf("LongToInt4: only positive values supported, got %d", i)
	}
	numBits := 64 - bits.LeadingZeros64(uint64(i))
	if numBits < 4 {
		return int(i), nil
	}
	shift := numBits - 4
	encoded := int(uint64(i) >> uint(shift))
	encoded &= 0x07
	encoded |= (shift + 1) << 3
	return encoded, nil
}

// Int4ToLong decodes a value produced by LongToInt4.
func Int4ToLong(i int) int64 {
	value := int64(i & 0x07)
	shift := (i >> 3) - 1
	if shift == -1 {
		return value
	}
	return (value | 0x08) << uint(shift)
}

// maxInt4 caches LongToInt4(math.MaxInt32). It is initialised once at
// package init so subsequent lookups are free.
var (
	maxInt4       int
	numFreeValues int
)

func init() {
	v, err := LongToInt4(int64(math.MaxInt32))
	if err != nil {
		panic(fmt.Errorf("LongToInt4(MaxInt32) failed during init: %w", err))
	}
	maxInt4 = v
	numFreeValues = 255 - maxInt4
}

// IntToByte4 encodes a non-negative int into a byte. Built on top of
// LongToInt4; the first NUM_FREE_VALUES values are passed through
// verbatim to keep low values byte-accurate.
func IntToByte4(i int) (byte, error) {
	if i < 0 {
		return 0, fmt.Errorf("IntToByte4: only positive values supported, got %d", i)
	}
	if i < numFreeValues {
		return byte(i), nil
	}
	v, err := LongToInt4(int64(i - numFreeValues))
	if err != nil {
		return 0, err
	}
	return byte(numFreeValues + v), nil
}

// Byte4ToInt decodes a value produced by IntToByte4.
func Byte4ToInt(b byte) int {
	i := int(b)
	if i < numFreeValues {
		return i
	}
	return numFreeValues + int(Int4ToLong(i-numFreeValues))
}

// MaxInt4 returns the cached LongToInt4(MaxInt32). Exposed for tests.
func MaxInt4() int { return maxInt4 }

// NumFreeValues returns the number of byte values that IntToByte4
// passes through verbatim. Exposed for tests.
func NumFreeValues() int { return numFreeValues }

// SmallFloat is a marker type retained for Java parity. The actual
// API consists of package-level functions; this type is left as a
// receiverless namespace anchor.
type SmallFloat struct{}
