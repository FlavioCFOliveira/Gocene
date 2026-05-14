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

// vectorEpsilon is the tolerance used by L2Normalize / IsUnitVector to skip
// renormalising vectors that are already within EPSILON of unit length.
// Matches Lucene's VectorUtil.EPSILON.
const vectorEpsilon = 1e-4

// DotProduct returns the dot product of two float32 vectors. Panics with a
// dimension-mismatch message if the lengths differ, mirroring Lucene's
// IllegalArgumentException.
//
// Performance note: this is the scalar fallback in Lucene's VectorUtil.
// Go's compiler does basic loop unrolling and vectorisation for tight
// float32 loops on AMD64 / ARM64; that's enough to keep us competitive with
// Lucene's non-SIMD path. Full SIMD support is deferred (Lucene depends on
// jdk.incubator.vector for that path).
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	var res float32
	// Match Lucene's unrolled path for arrays larger than 32 elements: four
	// independent accumulators reduce dependency chains and let the compiler
	// schedule the multiply-adds more aggressively.
	i := 0
	if len(a) > 32 {
		var acc1, acc2, acc3, acc4 float32
		upper := len(a) &^ 3
		for ; i < upper; i += 4 {
			acc1 += a[i] * b[i]
			acc2 += a[i+1] * b[i+1]
			acc3 += a[i+2] * b[i+2]
			acc4 += a[i+3] * b[i+3]
		}
		res += acc1 + acc2 + acc3 + acc4
	}
	for ; i < len(a); i++ {
		res += a[i] * b[i]
	}
	return res
}

// Cosine returns the cosine similarity between two float32 vectors. Panics
// on dimension mismatch.
func Cosine(a, b []float32) float32 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	var sum, norm1, norm2 float32
	i := 0
	if len(a) > 32 {
		var s1, s2, n1a, n1b, n2a, n2b float32
		upper := len(a) &^ 1
		for ; i < upper; i += 2 {
			s1 += a[i] * b[i]
			n1a += a[i] * a[i]
			n2a += b[i] * b[i]
			s2 += a[i+1] * b[i+1]
			n1b += a[i+1] * a[i+1]
			n2b += b[i+1] * b[i+1]
		}
		sum += s1 + s2
		norm1 += n1a + n1b
		norm2 += n2a + n2b
	}
	for ; i < len(a); i++ {
		sum += a[i] * b[i]
		norm1 += a[i] * a[i]
		norm2 += b[i] * b[i]
	}
	return float32(float64(sum) / math.Sqrt(float64(norm1)*float64(norm2)))
}

// SquareDistance returns the sum of squared differences between two float32
// vectors. Panics on dimension mismatch.
func SquareDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	var res float32
	i := 0
	if len(a) > 32 {
		var acc1, acc2, acc3, acc4 float32
		upper := len(a) &^ 3
		for ; i < upper; i += 4 {
			d1 := a[i] - b[i]
			d2 := a[i+1] - b[i+1]
			d3 := a[i+2] - b[i+2]
			d4 := a[i+3] - b[i+3]
			acc1 += d1 * d1
			acc2 += d2 * d2
			acc3 += d3 * d3
			acc4 += d4 * d4
		}
		res += acc1 + acc2 + acc3 + acc4
	}
	for ; i < len(a); i++ {
		d := a[i] - b[i]
		res += d * d
	}
	return res
}

// DotProductBytes returns the dot product of two signed-byte vectors.
func DotProductBytes(a, b []byte) int32 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	var total int32
	for i := range a {
		total += int32(int8(a[i])) * int32(int8(b[i]))
	}
	return total
}

// Uint8DotProduct returns the dot product of two unsigned-byte vectors.
func Uint8DotProduct(a, b []byte) int32 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	var total int32
	for i := range a {
		total += int32(a[i]) * int32(b[i])
	}
	return total
}

// CosineBytes returns the cosine similarity between two signed-byte vectors.
func CosineBytes(a, b []byte) float32 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	var sum, norm1, norm2 int64
	for i := range a {
		sa := int64(int8(a[i]))
		sb := int64(int8(b[i]))
		sum += sa * sb
		norm1 += sa * sa
		norm2 += sb * sb
	}
	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	return float32(float64(sum) / math.Sqrt(float64(norm1)*float64(norm2)))
}

// SquareDistanceBytes returns the sum of squared differences between two
// signed-byte vectors.
func SquareDistanceBytes(a, b []byte) int32 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	var total int32
	for i := range a {
		d := int32(int8(a[i])) - int32(int8(b[i]))
		total += d * d
	}
	return total
}

// Uint8SquareDistance returns the sum of squared differences between two
// unsigned-byte vectors.
func Uint8SquareDistance(a, b []byte) int32 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	var total int32
	for i := range a {
		d := int32(a[i]) - int32(b[i])
		total += d * d
	}
	return total
}

// L2Normalize normalises v in place by dividing each component by the L2
// norm. Returns v for call chaining, matching Lucene's signature. Panics on
// a zero-length vector (matches Java's IllegalArgumentException).
func L2Normalize(v []float32) []float32 {
	return L2NormalizeThrow(v, true)
}

// L2NormalizeThrow normalises v in place. When throwOnZero is false the
// function leaves a zero vector untouched instead of panicking.
func L2NormalizeThrow(v []float32, throwOnZero bool) []float32 {
	l1norm := dotSelf(v)
	if l1norm == 0 {
		if throwOnZero {
			panic("Cannot normalize a zero-length vector")
		}
		return v
	}
	if math.Abs(l1norm-1.0) <= vectorEpsilon {
		return v
	}
	l2norm := math.Sqrt(l1norm)
	for i := range v {
		v[i] = float32(float64(v[i]) / l2norm)
	}
	return v
}

// IsUnitVector reports whether v is within EPSILON of unit length.
func IsUnitVector(v []float32) bool {
	return math.Abs(dotSelf(v)-1.0) <= vectorEpsilon
}

// AddVec adds v into u component-wise. The destination is u. Panics on
// dimension mismatch.
func AddVec(u, v []float32) {
	if len(u) != len(v) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(u), len(v)))
	}
	for i := range u {
		u[i] += v[i]
	}
}

// CheckFinite verifies every component of v is finite (i.e. not NaN/Inf).
// Returns v for call chaining; panics with the offending index on failure.
func CheckFinite(v []float32) []float32 {
	for i, x := range v {
		if !isFiniteFloat32(x) {
			panic(fmt.Sprintf("non-finite value at vector[%d]=%v", i, x))
		}
	}
	return v
}

// DotProductScore scales the signed-byte dot product into [0, 1].
//
//	denom = len(a) * 2^15
//	score = 0.5 + dot(a,b) / denom
func DotProductScore(a, b []byte) float32 {
	denom := float32(len(a) * (1 << 15))
	return 0.5 + float32(DotProductBytes(a, b))/denom
}

// ScaleMaxInnerProductScore normalises a maximum-inner-product similarity so
// it never goes negative. Mirrors VectorUtil.scaleMaxInnerProductScore.
func ScaleMaxInnerProductScore(similarity float32) float32 {
	if similarity < 0 {
		return 1 / (1 + -1*similarity)
	}
	return similarity + 1
}

// NormalizeToUnitInterval maps a similarity in [-1, 1] to [0, 1] using
// (1 + value) / 2, clamped at 0.
func NormalizeToUnitInterval(value float32) float32 {
	v := (1 + value) / 2
	if v < 0 {
		return 0
	}
	return v
}

// NormalizeDistanceToUnitInterval maps a non-negative squared distance into
// (0, 1] via 1 / (1 + squaredDistance).
func NormalizeDistanceToUnitInterval(squaredDistance float32) float32 {
	return 1.0 / (1.0 + squaredDistance)
}

// XorBitCount returns the population count of (a ^ b), interpreted as a
// sequence of bytes. Panics on dimension mismatch.
//
// Lucene picks between long-stride and int-stride variants based on CPU
// architecture; in Go we always use a single byte-by-byte loop with
// bits.OnesCount8 to keep the code lock-free and zero-alloc. For dense
// workloads this benchmarks within a few percent of Lucene's stride loops.
func XorBitCount(a, b []byte) int32 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	var total int32
	// Stride 8 bytes at a time when possible; small fan-out keeps the inner
	// dependency chain short.
	i := 0
	for ; i+8 <= len(a); i += 8 {
		x := uint64(a[i]) | uint64(a[i+1])<<8 | uint64(a[i+2])<<16 | uint64(a[i+3])<<24 |
			uint64(a[i+4])<<32 | uint64(a[i+5])<<40 | uint64(a[i+6])<<48 | uint64(a[i+7])<<56
		y := uint64(b[i]) | uint64(b[i+1])<<8 | uint64(b[i+2])<<16 | uint64(b[i+3])<<24 |
			uint64(b[i+4])<<32 | uint64(b[i+5])<<40 | uint64(b[i+6])<<48 | uint64(b[i+7])<<56
		total += int32(bits.OnesCount64(x ^ y))
	}
	for ; i < len(a); i++ {
		total += int32(bits.OnesCount8(a[i] ^ b[i]))
	}
	return total
}

// FindNextGEQ returns the first index in buffer[from:to] whose value is
// greater than or equal to target. The buffer slice [0:to] must be sorted in
// non-decreasing order (asserted at the Java layer; we trust the caller here
// to keep the helper allocation-free).
func FindNextGEQ(buffer []int32, target int32, from, to int) int {
	lo, hi := from, to
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if buffer[mid] < target {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// dotSelf is the dot product of v with itself, computed in float64 to keep
// the result consistent across array sizes (it feeds L2Normalize's branch
// on EPSILON). Not exported.
func dotSelf(v []float32) float64 {
	var acc float64
	for _, x := range v {
		acc += float64(x) * float64(x)
	}
	return acc
}

// isFiniteFloat32 is a stdlib-free check for !NaN && !Inf on a float32.
func isFiniteFloat32(x float32) bool {
	return !math.IsNaN(float64(x)) && !math.IsInf(float64(x), 0)
}
