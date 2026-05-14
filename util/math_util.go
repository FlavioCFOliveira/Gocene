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
	"math"
	"math/bits"
)

// MathLogIntBase returns floor(log_base(x)) for x > 0, and 0 for x <= 0.
// Mirrors Lucene's MathUtil.log(long x, int base). Base must be > 1.
//
// The base == 2 path is specialised using the standard
// bits.LeadingZeros64 helper to avoid the iterative division loop, just
// like the Lucene reference uses Long.numberOfLeadingZeros.
func MathLogIntBase(x int64, base int) int {
	if base == 2 {
		if x <= 0 {
			return 0
		}
		return 63 - bits.LeadingZeros64(uint64(x))
	}
	if base <= 1 {
		panic("base must be > 1")
	}
	ret := 0
	for x >= int64(base) {
		x /= int64(base)
		ret++
	}
	return ret
}

// MathLog returns log_base(x) for double arguments. Mirrors
// MathUtil.log(double base, double x).
func MathLog(base, x float64) float64 {
	return math.Log(x) / math.Log(base)
}

// MathGCD returns the greatest common divisor of a and b, consistent
// with BigInteger.gcd(BigInteger). Special case: GCD(MinInt64, 0) and
// GCD(MinInt64, MinInt64) return MinInt64 because 2^64 cannot be
// represented as int64 — this matches the Lucene contract exactly.
func MathGCD(a, b int64) int64 {
	a = absInt64(a)
	b = absInt64(b)
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	commonTrailingZeros := bits.TrailingZeros64(uint64(a) | uint64(b))
	a = int64(uint64(a) >> uint(bits.TrailingZeros64(uint64(a))))
	for {
		b = int64(uint64(b) >> uint(bits.TrailingZeros64(uint64(b))))
		if a == b {
			break
		}
		// Treat MinInt64 as 2^64 (largest possible value) per Lucene contract.
		if a > b || a == math.MinInt64 {
			a, b = b, a
		}
		if a == 1 {
			break
		}
		b -= a
	}
	return a << uint(commonTrailingZeros)
}

// absInt64 returns the absolute value of x, mirroring Java's Math.abs.
// Math.abs(Long.MIN_VALUE) is Long.MIN_VALUE (overflow); we replicate
// that wraparound for byte-for-byte compatibility.
func absInt64(x int64) int64 {
	if x < 0 {
		return -x // wraps for MinInt64, same as Java Math.abs
	}
	return x
}

// MathAsinh returns the inverse hyperbolic sine of a.
//
// Special cases: NaN -> NaN; ±0 -> ±0; ±Inf -> ±Inf.
func MathAsinh(a float64) float64 {
	var sign float64
	// Check sign bit on raw representation so that -0 is treated as negative.
	if math.Float64bits(a)&(1<<63) != 0 {
		a = math.Abs(a)
		sign = -1.0
	} else {
		sign = 1.0
	}
	return sign * math.Log(math.Sqrt(a*a+1.0)+a)
}

// MathAcosh returns the inverse hyperbolic cosine of a.
//
// Special cases: NaN -> NaN; +1 -> +0; +Inf -> +Inf; a < 1 -> NaN.
func MathAcosh(a float64) float64 {
	return math.Log(math.Sqrt(a*a-1.0) + a)
}

// MathAtanh returns the inverse hyperbolic tangent of a.
//
// Special cases: NaN -> NaN; ±0 -> ±0; +1 -> +Inf; -1 -> -Inf; |a|>1 -> NaN.
func MathAtanh(a float64) float64 {
	var mult float64
	if math.Float64bits(a)&(1<<63) != 0 {
		a = math.Abs(a)
		mult = -0.5
	} else {
		mult = 0.5
	}
	return mult * math.Log((1.0+a)/(1.0-a))
}

// MathSumRelativeErrorBound returns a relative error bound for the sum
// of numValues POSITIVE doubles computed via recursive summation, based
// on formula 3.5 of Higham (1993). For numValues <= 1 the bound is 0.
// The unit roundoff u is 2^-52 (the IEEE-754 double machine epsilon).
func MathSumRelativeErrorBound(numValues int) float64 {
	if numValues <= 1 {
		return 0
	}
	u := math.Ldexp(1.0, -52) // Math.scalb(1.0, -52)
	return float64(numValues-1) * u
}

// MathSumUpperBound returns the maximum possible sum across numValues
// non-negative doubles, given that one observed sum is `sum`. Returns
// `sum` unchanged when numValues <= 2.
func MathSumUpperBound(sum float64, numValues int) float64 {
	if numValues <= 2 {
		return sum
	}
	b := MathSumRelativeErrorBound(numValues)
	return (1.0 + 2*b) * sum
}

// MathUnsignedMin returns the minimum of two int32 values when both are
// interpreted as unsigned. Mirrors MathUtil.unsignedMin.
func MathUnsignedMin(a, b int32) int32 {
	if uint32(a) < uint32(b) {
		return a
	}
	return b
}
