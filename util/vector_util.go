package util

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"runtime"

	"github.com/FlavioCFOliveira/Gocene/internal/vectorization"
)

// VectorUtil provides utilities for computations with numeric arrays.
//
// Port of org.apache.lucene.util.VectorUtil (Apache Lucene 10.5.0,
// lucene/core/src/java/org/apache/lucene/util/VectorUtil.java). The Java
// static methods are package-level functions; Java overloads over byte[]
// carry the Bytes suffix. Every IllegalArgumentException is rendered as a
// panic carrying the Java message.
type VectorUtil struct{}

// vectorUtilEpsilon mirrors VectorUtil.EPSILON.
const vectorUtilEpsilon = 1e-4

// vectorUtilSupport mirrors VectorUtil.IMPL,
// VectorizationProvider.getInstance().getVectorUtilSupport().
var vectorUtilSupport = vectorization.NewVectorizationProvider().GetVectorUtilSupport()

func panicDimensionsDiffer(a, b int) {
	panic(fmt.Sprintf("vector dimensions differ: %d!=%d", a, b))
}

// ComputeDotProduct returns the vector dot product of the two vectors. It is
// a Gocene name for [DotProduct] kept for existing callers.
func ComputeDotProduct(a, b []float32) float32 {
	return DotProduct(a, b)
}

// DotProduct returns the vector dot product of the two vectors. It panics
// when the dimensions differ. Mirrors VectorUtil.dotProduct(float[], float[]),
// including its finiteness assertion.
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	r := vectorUtilSupport.DotProductFloats(a, b)
	if AssertsEnabled() && !isFiniteFloat32(r) {
		panic(NewAssertionError(fmt.Sprintf("not finite: %s from <%v,%v>", javaFloatString(r), a, b)))
	}
	return r
}

// Euclidean is a Gocene alias of [SquareDistance] kept for existing callers.
func Euclidean(a, b []float32) float32 {
	return SquareDistance(a, b)
}

// MaximumInnerProduct is a Gocene alias of [DotProduct] kept for existing
// callers.
func MaximumInnerProduct(a, b []float32) float32 {
	return DotProduct(a, b)
}

// Cosine returns the cosine similarity between the two vectors. It panics
// when the dimensions differ. Mirrors VectorUtil.cosine(float[], float[]),
// including its finiteness assertion.
func Cosine(a, b []float32) float32 {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	r := vectorUtilSupport.CosineFloats(a, b)
	if AssertsEnabled() && !isFiniteFloat32(r) {
		panic(NewAssertionError(nil))
	}
	return r
}

// CosineBytes returns the cosine similarity between the two signed-byte
// vectors. Mirrors VectorUtil.cosine(byte[], byte[]).
func CosineBytes(a, b []byte) float32 {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	return vectorUtilSupport.CosineBytes(a, b)
}

// SquareDistance returns the sum of squared differences of the two vectors.
// Mirrors VectorUtil.squareDistance(float[], float[]), including its
// finiteness assertion.
func SquareDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	r := vectorUtilSupport.SquareDistanceFloats(a, b)
	if AssertsEnabled() && !isFiniteFloat32(r) {
		panic(NewAssertionError(nil))
	}
	return r
}

// SquareDistanceBytes returns the sum of squared differences of the two
// signed-byte vectors. Mirrors VectorUtil.squareDistance(byte[], byte[]).
func SquareDistanceBytes(a, b []byte) int {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	return vectorUtilSupport.SquareDistanceBytes(a, b)
}

// Int4SquareDistance mirrors VectorUtil.int4SquareDistance(byte[], byte[]).
func Int4SquareDistance(a, b []byte) int {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	return vectorUtilSupport.Int4SquareDistance(a, b)
}

// Int4SquareDistanceSinglePacked mirrors
// VectorUtil.int4SquareDistanceSinglePacked(byte[], byte[]).
func Int4SquareDistanceSinglePacked(unpacked, packed []byte) int {
	if len(packed) != ((len(unpacked) + 1) >> 1) {
		panic(fmt.Sprintf("vector dimensions differ: %d!= 2 * %d", len(unpacked), len(packed)))
	}
	return vectorUtilSupport.Int4SquareDistanceSinglePacked(unpacked, packed)
}

// Int4SquareDistanceBothPacked mirrors
// VectorUtil.int4SquareDistanceBothPacked(byte[], byte[]).
func Int4SquareDistanceBothPacked(a, b []byte) int {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	return vectorUtilSupport.Int4SquareDistanceBothPacked(a, b)
}

// Uint8SquareDistance returns the sum of squared differences of the two
// unsigned-byte vectors. Mirrors VectorUtil.uint8SquareDistance(byte[], byte[]).
func Uint8SquareDistance(a, b []byte) int {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	return vectorUtilSupport.Uint8SquareDistance(a, b)
}

// L2Normalize modifies the argument to be unit length, dividing by its
// l2-norm. It panics when the vector is all zeroes. Mirrors
// VectorUtil.l2normalize(float[]).
func L2Normalize(v []float32) {
	L2NormalizeThrow(v, true)
}

// IsUnitVector reports whether the vector is a unit vector. Mirrors
// VectorUtil.isUnitVector(float[]).
func IsUnitVector(v []float32) bool {
	l1norm := float64(vectorUtilSupport.DotProductFloats(v, v))
	return math.Abs(l1norm-1.0) <= vectorUtilEpsilon
}

// L2NormalizeThrow modifies the argument to be unit length, dividing by its
// l2-norm; an all-zero vector panics when throwOnZero is set and is returned
// unchanged otherwise. Mirrors VectorUtil.l2normalize(float[], boolean).
func L2NormalizeThrow(v []float32, throwOnZero bool) []float32 {
	l1norm := float64(vectorUtilSupport.DotProductFloats(v, v))
	if l1norm == 0 {
		if throwOnZero {
			panic("Cannot normalize a zero-length vector")
		}
		return v
	}
	if math.Abs(l1norm-1.0) <= vectorUtilEpsilon {
		return v
	}
	dim := len(v)
	l2norm := math.Sqrt(l1norm)
	for i := 0; i < dim; i++ {
		v[i] /= float32(l2norm)
	}
	return v
}

// DotProductBytes returns the dot product computed over signed bytes. Mirrors
// VectorUtil.dotProduct(byte[], byte[]).
func DotProductBytes(a, b []byte) int {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	return vectorUtilSupport.DotProductBytes(a, b)
}

// Uint8DotProduct returns the dot product computed over unsigned bytes.
// Mirrors VectorUtil.uint8DotProduct(byte[], byte[]).
func Uint8DotProduct(a, b []byte) int {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	return vectorUtilSupport.Uint8DotProduct(a, b)
}

// Int4DotProduct mirrors VectorUtil.int4DotProduct(byte[], byte[]).
func Int4DotProduct(a, b []byte) int {
	if len(a) != len(b) {
		panicDimensionsDiffer(len(a), len(b))
	}
	return vectorUtilSupport.Int4DotProduct(a, b)
}

// Int4DotProductSinglePacked mirrors
// VectorUtil.int4DotProductSinglePacked(byte[], byte[]).
func Int4DotProductSinglePacked(unpacked, packed []byte) int {
	if len(packed) != ((len(unpacked) + 1) >> 1) {
		panic(fmt.Sprintf("vector dimensions differ: %d != 2 * %d", len(unpacked), len(packed)))
	}
	return vectorUtilSupport.Int4DotProductSinglePacked(unpacked, packed)
}

// Int4DotProductBothPacked mirrors
// VectorUtil.int4DotProductBothPacked(byte[], byte[]).
func Int4DotProductBothPacked(a, b []byte) int {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d != %d", len(a), len(b)))
	}
	return vectorUtilSupport.Int4DotProductBothPacked(a, b)
}

// Int4BitDotProduct computes the dot product between a transposed int4 query
// vector q (4 stripes, one bit plane each) and a single-bit document vector
// d. Mirrors VectorUtil.int4BitDotProduct(byte[], byte[]) of Apache Lucene
// 10.5.0, which throws IllegalArgumentException when q is not four times the
// length of d.
func Int4BitDotProduct(q, d []byte) int64 {
	if len(q) != len(d)*4 {
		panic(fmt.Sprintf("vector dimensions incompatible: %d!= 4 x %d", len(q), len(d)))
	}
	return vectorUtilSupport.Int4BitDotProduct(q, d)
}

// Int4DibitDotProduct mirrors VectorUtil.int4DibitDotProduct(byte[], byte[]).
func Int4DibitDotProduct(q, d []byte) int64 {
	if len(q) != len(d)*2 {
		panic(fmt.Sprintf("vector dimensions incompatible: %d!= 2 x %d", len(q), len(d)))
	}
	return vectorUtilSupport.Int4DibitDotProduct(q, d)
}

// DotProductScore returns the dot product score computed over signed bytes,
// scaled to be in [0, 1]. Mirrors VectorUtil.dotProductScore(byte[], byte[]).
func DotProductScore(a, b []byte) float32 {
	// divide by 2 * 2^14 (maximum absolute value of product of 2 signed bytes) * len
	denom := float32(len(a) * (1 << 15))
	return 0.5 + float32(DotProductBytes(a, b))/denom
}

// ScaleMaxInnerProductScore scales a maximum-inner-product similarity to be
// non-negative. Mirrors VectorUtil.scaleMaxInnerProductScore(float).
func ScaleMaxInnerProductScore(vectorDotProductSimilarity float32) float32 {
	if vectorDotProductSimilarity < 0 {
		return 1 / (1 + -1*vectorDotProductSimilarity)
	}
	return vectorDotProductSimilarity + 1
}

// NormalizeToUnitInterval normalizes a value in [-1, 1] to [0, 1]. Mirrors
// VectorUtil.normalizeToUnitInterval(float): Math.max((1 + value) / 2, 0).
func NormalizeToUnitInterval(value float32) float32 {
	return javaMaxFloat32((1+value)/2, 0)
}

// NormalizeDistanceToUnitInterval normalizes a squared distance to [0, 1].
// Mirrors VectorUtil.normalizeDistanceToUnitInterval(float).
func NormalizeDistanceToUnitInterval(squaredDistance float32) float32 {
	return 1.0 / (1.0 + squaredDistance)
}

// IsZeroVector reports whether every dimension of v is zero. Mirrors
// VectorUtil.isZeroVector(float[]).
func IsZeroVector(v []float32) bool {
	for _, value := range v {
		if value != 0 {
			return false
		}
	}
	return true
}

// IsZeroVectorBytes reports whether every dimension of v is zero. Mirrors
// VectorUtil.isZeroVector(byte[]).
func IsZeroVectorBytes(v []byte) bool {
	for _, value := range v {
		if value != 0 {
			return false
		}
	}
	return true
}

// MinMaxScalarQuantize scalar-quantizes vector into dest and returns the
// corrective offset to apply to the score. Mirrors
// VectorUtil.minMaxScalarQuantize.
func MinMaxScalarQuantize(vector []float32, dest []byte, scale, alpha, minQuantile, maxQuantile float32) float32 {
	if len(vector) != len(dest) {
		panic("source and destination arrays should be the same size")
	}
	return vectorUtilSupport.MinMaxScalarQuantize(vector, dest, scale, alpha, minQuantile, maxQuantile)
}

// RecalculateOffset recalculates the corrective offset of an already
// quantized vector. Mirrors VectorUtil.recalculateOffset.
func RecalculateOffset(vector []byte, oldAlpha, oldMinQuantile, scale, alpha, minQuantile, maxQuantile float32) float32 {
	return vectorUtilSupport.RecalculateScalarQuantizationOffset(
		vector, oldAlpha, oldMinQuantile, scale, alpha, minQuantile, maxQuantile)
}

// FilterByScore keeps, in place, the (doc, score) pairs below upTo whose score
// is at least minScoreInclusive and returns how many are left. Mirrors
// VectorUtil.filterByScore(int[], double[], double, int).
func FilterByScore(docBuffer []int32, scoreBuffer []float64, minScoreInclusive float64, upTo int) int {
	if len(docBuffer) != len(scoreBuffer) || len(docBuffer) < upTo {
		panic("docBuffer and scoreBuffer should keep same length and at least as long as upTo")
	}
	return vectorUtilSupport.FilterByScore(docBuffer, scoreBuffer, minScoreInclusive, upTo)
}

func isFiniteFloat32(f float32) bool {
	return !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0)
}

// javaMaxFloat32 mirrors Math.max(float, float): NaN if either argument is
// NaN, and +0.0 preferred over -0.0.
func javaMaxFloat32(a, b float32) float32 {
	if a != a {
		return a
	}
	if a == 0 && b == 0 && math.Signbit(float64(a)) {
		return b
	}
	if a >= b {
		return a
	}
	return b
}

// AddVec adds the second argument to the first: u[i] += v[i] for every index
// of u. Mirrors VectorUtil.add(float[], float[]) of Apache Lucene 10.5.0; the
// Go name carries the Vec suffix because util.Add already renders
// NumericUtils.add in this package.
func AddVec(u, v []float32) {
	for i := 0; i < len(u); i++ {
		u[i] += v[i]
	}
}

// xorBitCountStrideAsInt mirrors VectorUtil.XOR_BIT_COUNT_STRIDE_AS_INT:
// Constants.OS_ARCH.equals("aarch64"), which Go reports as GOARCH "arm64".
var xorBitCountStrideAsInt = runtime.GOARCH == "arm64"

// XorBitCount returns the XOR bit count computed over signed bytes. It panics
// with an IllegalArgumentException message when the dimensions differ.
// Mirrors VectorUtil.xorBitCount(byte[], byte[]) of Apache Lucene 10.5.0.
func XorBitCount(a, b []byte) int {
	if len(a) != len(b) {
		panic(fmt.Sprintf("vector dimensions differ: %d!=%d", len(a), len(b)))
	}
	if xorBitCountStrideAsInt {
		return xorBitCountInt(a, b)
	}
	return xorBitCountLong(a, b)
}

// xorBitCountInt is the XOR bit count striding over 4 bytes at a time.
// Mirrors the package-private VectorUtil.xorBitCountInt.
func xorBitCountInt(a, b []byte) int {
	distance, i := 0, 0
	for upperBound := len(a) & -4; i < upperBound; i += 4 {
		distance += bits.OnesCount32(binary.NativeEndian.Uint32(a[i:]) ^ binary.NativeEndian.Uint32(b[i:]))
	}
	// tail:
	for ; i < len(a); i++ {
		distance += bits.OnesCount8(a[i] ^ b[i])
	}
	return distance
}

// xorBitCountLong is the XOR bit count striding over 8 bytes at a time.
// Mirrors the package-private VectorUtil.xorBitCountLong.
func xorBitCountLong(a, b []byte) int {
	distance, i := 0, 0
	for upperBound := len(a) & -8; i < upperBound; i += 8 {
		distance += bits.OnesCount64(binary.NativeEndian.Uint64(a[i:]) ^ binary.NativeEndian.Uint64(b[i:]))
	}
	// tail:
	for ; i < len(a); i++ {
		distance += bits.OnesCount8(a[i] ^ b[i])
	}
	return distance
}

// CheckFinite checks that a float vector only has finite components and
// returns it for call-chaining. It panics with an IllegalArgumentException
// message naming the first non-finite component. Mirrors
// VectorUtil.checkFinite(float[]) of Apache Lucene 10.5.0.
func CheckFinite(v []float32) []float32 {
	for i := 0; i < len(v); i++ {
		if !isFiniteFloat32(v[i]) {
			panic(fmt.Sprintf("non-finite value at vector[%d]=%s", i, javaFloatString(v[i])))
		}
	}
	return v
}

// javaFloatString renders the non-finite float values the way Java's
// Float.toString does ("NaN", "Infinity", "-Infinity") and finite values
// with the shortest representation.
func javaFloatString(f float32) string {
	switch {
	case math.IsNaN(float64(f)):
		return "NaN"
	case math.IsInf(float64(f), 1):
		return "Infinity"
	case math.IsInf(float64(f), -1):
		return "-Infinity"
	}
	return fmt.Sprint(f)
}

// FindNextGEQ returns the first index of buffer, at least from, whose value is
// greater than or equal to target, given that buffer is sorted between index
// 0 inclusive and to exclusive; to is returned when there is no such index.
// Mirrors VectorUtil.findNextGEQ(int[], int, int, int) of Apache Lucene
// 10.5.0, including its sortedness assertion.
func FindNextGEQ(buffer []int32, target int32, from, to int) int {
	if AssertsEnabled() {
		for i := 0; i < to-1; i++ {
			if buffer[i] > buffer[i+1] {
				panic(NewAssertionError(nil))
			}
		}
	}
	return vectorUtilSupport.FindNextGEQ(buffer, target, from, to)
}
