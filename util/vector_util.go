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
