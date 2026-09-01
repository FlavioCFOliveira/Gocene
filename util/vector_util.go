package util

import "math"

// VectorUtil provides utilities for computations with numeric arrays.
type VectorUtil struct{}

// DotProduct returns the vector dot product of the two vectors.
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		panic("vector dimensions differ")
	}
	var sum float32
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}
	return sum
}

// IsUnitVector returns true if the vector is unit length (within epsilon).
func IsUnitVector(v []float32) bool {
	const epsilon = 1e-4
	dot := DotProduct(v, v)
	return math.Abs(float64(dot)-1.0) <= epsilon
}

// L2Normalize normalises the vector in place according to the L2 norm.
// A zero-length vector triggers a panic to mirror the IllegalArgumentException
// thrown by Lucene.
func L2Normalize(v []float32) {
	L2NormalizeThrow(v, true)
}

// L2NormalizeThrow normalises the vector in place according to the L2 norm.
// If throwOnZero is true, a zero-length vector triggers a panic.
// If throwOnZero is false, the zero vector is returned unchanged.
func L2NormalizeThrow(v []float32, throwOnZero bool) []float32 {
	if len(v) == 0 {
		return v
	}
	dot := DotProduct(v, v)
	if dot == 0 {
		if throwOnZero {
			panic("cannot l2normalize a zero-length vector")
		}
		return v
	}
	if IsUnitVector(v) {
		return v
	}
	invNorm := 1.0 / float32(math.Sqrt(float64(dot)))
	for i := range v {
		v[i] *= invNorm
	}
	return v
}

