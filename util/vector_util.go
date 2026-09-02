package util

import "math"

// VectorUtil provides utilities for computations with numeric arrays.
type VectorUtil struct{}

// ComputeDotProduct returns the vector dot product of the two vectors.
func ComputeDotProduct(a, b []float32) float32 {
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
	dot := ComputeDotProduct(v, v)
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
	dot := ComputeDotProduct(v, v)
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

// SquareDistance calculates the squared Euclidean distance between two vectors.
func SquareDistance(v1, v2 []float32) float32 {
	var res float32
	for i := 0; i < len(v1); i++ {
		diff := v1[i] - v2[i]
		res += diff * diff
	}
	return res
}

// SquareDistanceBytes calculates the squared Euclidean distance between two signed-byte vectors.
func SquareDistanceBytes(v1, v2 []byte) float32 {
	var res float32
	for i := 0; i < len(v1); i++ {
		diff := float32(int8(v1[i])) - float32(int8(v2[i]))
		res += diff * diff
	}
	return res
}

// NormalizeDistanceToUnitInterval maps [0, inf) to (0, 1].
func NormalizeDistanceToUnitInterval(dist float32) float32 {
	return 1 / (1 + dist)
}

// NormalizeToUnitInterval maps [-1, 1] to [0, 1].
func NormalizeToUnitInterval(val float32) float32 {
	return (val + 1) / 2
}

// DotProductScore computes the dot product score for signed-byte vectors.
func DotProductScore(v1, v2 []byte) float32 {
	var sum float32
	for i := 0; i < len(v1); i++ {
		sum += float32(int8(v1[i])) * float32(int8(v2[i]))
	}
	return NormalizeToUnitInterval(sum)
}

// Cosine calculates the cosine similarity between two float32 vectors.
func Cosine(v1, v2 []float32) float32 {
	var sum, norm1, norm2 float32
	for i := 0; i < len(v1); i++ {
		sum += v1[i] * v2[i]
		norm1 += v1[i] * v1[i]
		norm2 += v2[i] * v2[i]
	}
	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	return sum / float32(math.Sqrt(float64(norm1)*float64(norm2)))
}

// CosineBytes calculates the cosine similarity between two signed-byte vectors.
func CosineBytes(v1, v2 []byte) float32 {
	var sum, norm1, norm2 float32
	for i := 0; i < len(v1); i++ {
		e1 := float32(int8(v1[i]))
		e2 := float32(int8(v2[i]))
		sum += e1 * e2
		norm1 += e1 * e1
		norm2 += e2 * e2
	}
	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	return sum / float32(math.Sqrt(float64(norm1)*float64(norm2)))
}

// DotProductBytes computes the dot product of two signed-byte vectors.
func DotProductBytes(v1, v2 []byte) float32 {
	var sum float32
	for i := 0; i < len(v1); i++ {
		sum += float32(int8(v1[i])) * float32(int8(v2[i]))
	}
	return sum
}

// ScaleMaxInnerProductScore scales the inner product to [0, 1].
func ScaleMaxInnerProductScore(val float32) float32 {
	return (val + 1) / 2
}

