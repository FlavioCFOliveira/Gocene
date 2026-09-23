// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// Port of lucene/core/src/test/org/apache/lucene/util/TestVectorUtil.java
// (Apache Lucene 10.5.0). Every Java test method has a Go test of the same
// name. Notes:
//   - expectThrows(IllegalArgumentException.class, ...) is rendered with
//     expectPanic: Gocene raises the unchecked Java exceptions as panics.
//   - testFilterByScore fills two org.apache.lucene.search.DocAndScoreAccBuffer
//     instances. Package util cannot import package search (search imports
//     util), and the Go DocAndScoreAccBuffer stores its docs as []int where
//     Java stores int[]; the port therefore holds the same docs/scores/size
//     triple in local int32/float64 slices grown like growNoCopy(128+padding).
//   - defaultedProvider and defOrPanamaProvider render
//     BaseVectorizationTestCase.defaultProvider() and maybePanamaProvider():
//     Gocene has no Panama provider, so the latter is the provider returned by
//     VectorizationProvider.getInstance().

import (
	"bytes"
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/internal/vectorization"
)

const vectorUtilTestDelta = 1e-4

func assertFloatEquals(t *testing.T, expected, actual, delta float64) {
	t.Helper()
	if math.Abs(expected-actual) > delta || math.IsNaN(actual) != math.IsNaN(expected) {
		t.Fatalf("expected:<%v> but was:<%v> (delta %v)", expected, actual, delta)
	}
}

func TestVectorUtil_testBasicDotProduct(t *testing.T) {
	assertFloatEquals(t, 5, float64(DotProduct([]float32{1, 2, 3}, []float32{-10, 0, 5})), 0)
}

func TestVectorUtil_testSelfDotProduct(t *testing.T) {
	// the dot product of a vector with itself is equal to the sum of the squares of its components
	r := newTestRandom(t)
	v := vectorUtilRandomVector(r)
	assertFloatEquals(t, float64(vectorUtilL2(v)), float64(DotProduct(v, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testOrthogonalDotProduct(t *testing.T) {
	// the dot product of two perpendicular vectors is 0
	r := newTestRandom(t)
	v := make([]float32, 2)
	v[0] = float32(r.Intn(100))
	v[1] = float32(r.Intn(100))
	u := make([]float32, 2)
	u[0] = v[1]
	u[1] = -v[0]
	assertFloatEquals(t, 0, float64(DotProduct(u, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testDotProductThrowsForDimensionMismatch(t *testing.T) {
	v, u := []float32{1, 0, 0}, []float32{0, 1}
	expectPanic(t, func() { DotProduct(u, v) })
}

func TestVectorUtil_testSelfSquareDistance(t *testing.T) {
	// the l2 distance of a vector with itself is zero
	r := newTestRandom(t)
	v := vectorUtilRandomVector(r)
	assertFloatEquals(t, 0, float64(SquareDistance(v, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testBasicSquareDistance(t *testing.T) {
	assertFloatEquals(t, 12, float64(SquareDistance([]float32{1, 2, 3}, []float32{-1, 0, 5})), 0)
}

func TestVectorUtil_testSquareDistanceThrowsForDimensionMismatch(t *testing.T) {
	v, u := []float32{1, 0, 0}, []float32{0, 1}
	expectPanic(t, func() { SquareDistance(u, v) })
}

func TestVectorUtil_testRandomSquareDistance(t *testing.T) {
	// the square distance of a vector with its inverse is equal to four times the sum of squares of
	// its components
	r := newTestRandom(t)
	v := vectorUtilRandomVector(r)
	u := vectorUtilNegative(v)
	assertFloatEquals(t, float64(4*vectorUtilL2(v)), float64(SquareDistance(u, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testBasicCosine(t *testing.T) {
	assertFloatEquals(t, float64(float32(0.11952)),
		float64(Cosine([]float32{1, 2, 3}, []float32{-10, 0, 5})), vectorUtilTestDelta)
}

func TestVectorUtil_testSelfCosine(t *testing.T) {
	// the dot product of a vector with itself is always equal to 1
	r := newTestRandom(t)
	v := vectorUtilRandomVector(r)
	// ensure the vector is non-zero so that cosine is defined
	v[0] = r.Float32() + 0.01
	assertFloatEquals(t, 1.0, float64(Cosine(v, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testOrthogonalCosine(t *testing.T) {
	// the cosine of two perpendicular vectors is 0
	r := newTestRandom(t)
	v := make([]float32, 2)
	v[0] = float32(r.Intn(100))
	// ensure the vector is non-zero so that cosine is defined
	v[1] = float32(1 + r.Intn(99))
	u := make([]float32, 2)
	u[0] = v[1]
	u[1] = -v[0]
	assertFloatEquals(t, 0, float64(Cosine(u, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testCosineThrowsForDimensionMismatch(t *testing.T) {
	v, u := []float32{1, 0, 0}, []float32{0, 1}
	expectPanic(t, func() { Cosine(u, v) })
}

func TestVectorUtil_testNormalize(t *testing.T) {
	r := newTestRandom(t)
	v := vectorUtilRandomVector(r)
	v[r.Intn(len(v))] = 1 // ensure vector is not all zeroes
	L2Normalize(v)
	assertFloatEquals(t, 1, float64(vectorUtilL2(v)), vectorUtilTestDelta)
}

func TestVectorUtil_testNormalizeZeroThrows(t *testing.T) {
	v := []float32{0, 0, 0}
	expectPanic(t, func() { L2Normalize(v) })
}

func TestVectorUtil_testNormalizeToUnitInterval(t *testing.T) {
	r := newTestRandom(t)
	for i := 0; i < 100; i++ {
		// Generates a float in the range [-1.0, 1.0)
		f := r.Float32()*2 - 1
		v := NormalizeToUnitInterval(f)
		if !(v >= 0) {
			t.Fatalf("v=%v < 0", v)
		}
		if !(v <= 1) {
			t.Fatalf("v=%v > 1", v)
		}
		assertFloatEquals(t, float64(javaMaxFloat32((1+f)/2, 0)), float64(v), 0)
	}
}

func TestVectorUtil_testExtremeNumerics(t *testing.T) {
	v1 := make([]float32, 1536)
	v2 := make([]float32, 1536)
	for i := 0; i < 1536; i++ {
		v1[i] = 0.888888
		v2[i] = -0.777777
	}
	for _, vectorSimilarityFunction := range []VectorSimilarityFunction{
		EuclideanSim, DotProductSim, CosineSim, MaximumInnerProductSim,
	} {
		v := vectorSimilarityFunction.CompareFloat(v1, v2)
		if !(v >= 0) {
			t.Fatalf("%v expected >=0 got:%v", vectorSimilarityFunction.ID(), v)
		}
	}
}

func vectorUtilL2(v []float32) float32 {
	var l2 float32
	for _, x := range v {
		l2 += x * x
	}
	return l2
}

func vectorUtilNegative(v []float32) []float32 {
	u := make([]float32, len(v))
	for i := range v {
		u[i] = -v[i]
	}
	return u
}

func vectorUtilNegativeBytes(v []byte) []byte {
	u := make([]byte, len(v))
	for i := range v {
		// what is (byte) -(-128)? 127?
		u[i] = byte(-int8(v[i]))
	}
	return u
}

func vectorUtilL2Bytes(v []byte) float32 {
	var l2 float32
	for i := range v {
		l2 += float32(int(int8(v[i])) * int(int8(v[i])))
	}
	return l2
}

func vectorUtilUint8L2(v []byte) float32 {
	var l2 float32
	for i := range v {
		l2 += float32(int(v[i]) * int(v[i]))
	}
	return l2
}

func vectorUtilRandomVector(r *rand.Rand) []float32 {
	return VectorUtilRandomVectorDim(r, r.Intn(100)+1)
}

// VectorUtilRandomVectorDim renders the public static TestVectorUtil.randomVector(int).
func VectorUtilRandomVectorDim(r *rand.Rand, dim int) []float32 {
	v := make([]float32, dim)
	for i := 0; i < dim; i++ {
		v[i] = r.Float32()
	}
	return v
}

func vectorUtilRandomVectorBytes(r *rand.Rand) []byte {
	v := randomBinaryTerm(r, nextInt(r, 1, 100))
	// clip at -127 to avoid overflow
	for i := v.Offset; i < v.Offset+v.Length; i++ {
		if int8(v.Bytes[i]) == -128 {
			v.Bytes[i] = byte(0x81) // -127
		}
	}
	if AssertsEnabled() && !(v.Offset == 0) {
		panic(NewAssertionError(nil))
	}
	return v.Bytes
}

// VectorUtilRandomVectorBytesDim renders the public static
// TestVectorUtil.randomVectorBytes(int).
func VectorUtilRandomVectorBytesDim(r *rand.Rand, dim int) []byte {
	v := randomBinaryTerm(r, dim)
	// clip at -127 to avoid overflow
	for i := v.Offset; i < v.Offset+v.Length; i++ {
		if int8(v.Bytes[i]) == -128 {
			v.Bytes[i] = byte(0x81) // -127
		}
	}
	return v.Bytes
}

func signedBytes(values ...int) []byte {
	b := make([]byte, len(values))
	for i, v := range values {
		b[i] = byte(int8(v))
	}
	return b
}

func TestVectorUtil_testBasicDotProductBytes(t *testing.T) {
	a := signedBytes(1, 2, 3)
	b := signedBytes(-10, 0, 5)
	assertFloatEquals(t, 5, float64(DotProductBytes(a, b)), 0)
	denom := float32(len(a) * (1 << 15))
	assertFloatEquals(t, float64(0.5+5/denom), float64(DotProductScore(a, b)), vectorUtilTestDelta)

	// dot product 0 maps to dotProductScore 0.5
	zero := signedBytes(0, 0, 0)
	assertFloatEquals(t, 0.5, float64(DotProductScore(a, zero)), vectorUtilTestDelta)

	minV := signedBytes(-128, -128)
	maxV := signedBytes(127, 127)
	// minimum dot product score is not quite zero because 127 < 128
	assertFloatEquals(t, 0.0039, float64(DotProductScore(minV, maxV)), vectorUtilTestDelta)

	// maximum dot product score
	assertFloatEquals(t, 1, float64(DotProductScore(minV, minV)), vectorUtilTestDelta)
}

func TestVectorUtil_testSelfDotProductBytes(t *testing.T) {
	// the dot product of a vector with itself is equal to the sum of the squares of its components
	r := newTestRandom(t)
	v := vectorUtilRandomVectorBytes(r)
	assertFloatEquals(t, float64(vectorUtilL2Bytes(v)), float64(DotProductBytes(v, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testOrthogonalDotProductBytes(t *testing.T) {
	// the dot product of two perpendicular vectors is 0
	r := newTestRandom(t)
	a := make([]byte, 2)
	a[0] = byte(r.Intn(100))
	a[1] = byte(r.Intn(100))
	b := make([]byte, 2)
	b[0] = a[1]
	b[1] = byte(-int8(a[0]))
	assertFloatEquals(t, 0, float64(DotProductBytes(a, b)), vectorUtilTestDelta)
}

func TestVectorUtil_testSelfSquareDistanceBytes(t *testing.T) {
	// the l2 distance of a vector with itself is zero
	r := newTestRandom(t)
	v := vectorUtilRandomVectorBytes(r)
	assertFloatEquals(t, 0, float64(SquareDistanceBytes(v, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testBasicSquareDistanceBytes(t *testing.T) {
	assertFloatEquals(t, 12, float64(SquareDistanceBytes(signedBytes(1, 2, 3), signedBytes(-1, 0, 5))), 0)
}

func TestVectorUtil_testRandomSquareDistanceBytes(t *testing.T) {
	// the square distance of a vector with its inverse is equal to four times the sum of squares of
	// its components
	r := newTestRandom(t)
	v := vectorUtilRandomVectorBytes(r)
	u := vectorUtilNegativeBytes(v)
	assertFloatEquals(t, float64(4*vectorUtilL2Bytes(v)), float64(SquareDistanceBytes(u, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testBasicDotProductUint8(t *testing.T) {
	a := signedBytes(1, 2, 3)
	b := signedBytes(-10, 0, 5)
	assertFloatEquals(t, 261, float64(Uint8DotProduct(a, b)), 0)

	minV := signedBytes(-128, -128)
	maxV := signedBytes(127, 127)
	assertFloatEquals(t, 32512, float64(Uint8DotProduct(minV, maxV)), vectorUtilTestDelta)
}

func TestVectorUtil_testSelfDotProductUint8(t *testing.T) {
	// the dot product of a vector with itself is equal to the sum of the squares of its components
	r := newTestRandom(t)
	v := vectorUtilRandomVectorBytes(r)
	assertFloatEquals(t, float64(vectorUtilUint8L2(v)), float64(Uint8DotProduct(v, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testSelfSquareDistanceUint8(t *testing.T) {
	// the l2 distance of a vector with itself is zero
	r := newTestRandom(t)
	v := vectorUtilRandomVectorBytes(r)
	assertFloatEquals(t, 0, float64(Uint8SquareDistance(v, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testBasicSquareDistanceUint8(t *testing.T) {
	assertFloatEquals(t, 64524, float64(Uint8SquareDistance(signedBytes(1, 2, 3), signedBytes(-1, 0, 5))), 0)
}

func TestVectorUtil_testBasicCosineBytes(t *testing.T) {
	assertFloatEquals(t, float64(float32(0.11952)),
		float64(CosineBytes(signedBytes(1, 2, 3), signedBytes(-10, 0, 5))), vectorUtilTestDelta)
}

func TestVectorUtil_testSelfCosineBytes(t *testing.T) {
	// the dot product of a vector with itself is always equal to 1
	r := newTestRandom(t)
	v := vectorUtilRandomVectorBytes(r)
	// ensure the vector is non-zero so that cosine is defined
	v[0] = byte(r.Intn(126) + 1)
	assertFloatEquals(t, 1.0, float64(CosineBytes(v, v)), vectorUtilTestDelta)
}

func TestVectorUtil_testOrthogonalCosineBytes(t *testing.T) {
	// the cosine of two perpendicular vectors is 0
	r := newTestRandom(t)
	v := make([]float32, 2)
	v[0] = float32(r.Intn(100))
	// ensure the vector is non-zero so that cosine is defined
	v[1] = float32(1 + r.Intn(99))
	u := make([]float32, 2)
	u[0] = v[1]
	u[1] = -v[0]
	assertFloatEquals(t, 0, float64(Cosine(u, v)), vectorUtilTestDelta)
}

// toIntBiFunction renders the TestVectorUtil.ToIntBiFunction interface.
type toIntBiFunction func(a, b []byte) int

func TestVectorUtil_testBasicXorBitCount(t *testing.T) {
	testBasicXorBitCountImpl(t, XorBitCount)
	testBasicXorBitCountImpl(t, xorBitCountInt)
	testBasicXorBitCountImpl(t, xorBitCountLong)
	// test sanity
	testBasicXorBitCountImpl(t, vectorUtilTestXorBitCount)
}

func testBasicXorBitCountImpl(t *testing.T, xorBitCount toIntBiFunction) {
	t.Helper()
	check := func(expected int, a, b []byte) {
		t.Helper()
		if got := xorBitCount(a, b); got != expected {
			t.Fatalf("xorBitCount(%v, %v) = %d, want %d", a, b, got, expected)
		}
	}
	check(0, []byte{1}, []byte{1})
	check(0, []byte{1, 2, 3}, []byte{1, 2, 3})
	check(1, []byte{1, 2, 3}, []byte{0, 2, 3})
	check(2, []byte{1, 2, 3}, []byte{0, 6, 3})
	check(3, []byte{1, 2, 3}, []byte{0, 6, 7})
	check(4, []byte{1, 2, 3}, []byte{2, 6, 7})

	// 32-bit / int boundary
	check(0, []byte{1, 2, 3, 4}, []byte{1, 2, 3, 4})
	check(1, []byte{1, 2, 3, 4}, []byte{0, 2, 3, 4})
	check(0, []byte{1, 2, 3, 4, 5}, []byte{1, 2, 3, 4, 5})
	check(1, []byte{1, 2, 3, 4, 5}, []byte{0, 2, 3, 4, 5})

	// 64-bit / long boundary
	check(0, []byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	check(1, []byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte{0, 2, 3, 4, 5, 6, 7, 8})

	check(0, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	check(1, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}, []byte{0, 2, 3, 4, 5, 6, 7, 8, 9})
}

func TestVectorUtil_testXorBitCount(t *testing.T) {
	r := newTestRandom(t)
	iterations := atLeast(r, 100)
	for i := 0; i < iterations; i++ {
		size := r.Intn(1024)
		a := make([]byte, size)
		b := make([]byte, size)
		r.Read(a)
		r.Read(b)

		expected := vectorUtilTestXorBitCount(a, b)
		if got := XorBitCount(a, b); got != expected {
			t.Fatalf("XorBitCount = %d, want %d", got, expected)
		}
		if got := xorBitCountInt(a, b); got != expected {
			t.Fatalf("xorBitCountInt = %d, want %d", got, expected)
		}
		if got := xorBitCountLong(a, b); got != expected {
			t.Fatalf("xorBitCountLong = %d, want %d", got, expected)
		}
	}
}

func vectorUtilTestXorBitCount(a, b []byte) int {
	res := 0
	for i := 0; i < len(a); i++ {
		x := a[i]
		y := b[i]
		for j := 0; j < 8; j++ {
			if x == y {
				break
			}
			if (x & 0x01) != (y & 0x01) {
				res++
			}
			x = x >> 1
			y = y >> 1
		}
	}
	return res
}

func TestVectorUtil_testFindNextGEQ(t *testing.T) {
	r := newTestRandom(t)
	padding := nextInt(r, 0, 5)
	values := make([]int32, 128+padding)
	v := 0
	for i := 0; i < 128; i++ {
		v += nextInt(r, 1, 1000)
		values[i] = int32(v)
	}

	// Now duel with slowFindFirstGreater
	for iter := 0; iter < 1_000; iter++ {
		from := nextInt(r, 0, 127)
		target := nextInt(r, int(values[from]), max(int(values[from]), int(values[127]))) +
			r.Intn(10) - 5
		expected := vectorUtilSlowFindNextGEQ(values, 128, int32(target), from)
		if got := FindNextGEQ(values, int32(target), from, 128); got != expected {
			t.Fatalf("FindNextGEQ(target=%d, from=%d) = %d, want %d", target, from, got, expected)
		}
	}
}

func vectorUtilSlowFindNextGEQ(buffer []int32, length int, target int32, from int) int {
	for i := from; i < length; i++ {
		if buffer[i] >= target {
			return i
		}
	}
	return length
}

func TestVectorUtil_testFilterByScore(t *testing.T) {
	r := newTestRandom(t)
	for iter := 0; iter < 1_000; iter++ {
		padding := nextInt(r, 0, 5)
		// b1 and b2 render DocAndScoreAccBuffer after growNoCopy(128 + padding).
		b1Docs, b1Scores := make([]int32, 128+padding), make([]float64, 128+padding)
		b2Docs, b2Scores := make([]int32, 128+padding), make([]float64, 128+padding)

		doc := 0
		for i := 0; i < 128+padding; i++ {
			doc += nextInt(r, 1, 1000)
			b1Docs[i], b2Docs[i] = int32(doc), int32(doc)
			score := r.Float64()
			b1Scores[i], b2Scores[i] = score, score
		}

		minScoreInclusive := r.Float64()
		upTo := nextInt(r, 0, 127)
		b1Size := vectorUtilSlowFilterByScore(b1Docs, b1Scores, minScoreInclusive, upTo)
		b2Size := FilterByScore(b2Docs, b2Scores, minScoreInclusive, upTo)
		if b1Size != b2Size {
			t.Fatalf("size: %d != %d", b1Size, b2Size)
		}
		if !slices.Equal(b1Docs[:b1Size], b2Docs[:b2Size]) {
			t.Fatalf("docs differ")
		}
		// two double array should be exactly the same, so just use simple Arrays.equals
		if !slices.Equal(b1Scores[:b1Size], b2Scores[:b2Size]) {
			t.Fatalf("scores differ")
		}
	}
}

func vectorUtilSlowFilterByScore(docBuffer []int32, scoreBuffer []float64, minScoreInclusive float64, upTo int) int {
	newSize := 0
	for i := 0; i < upTo; i++ {
		if scoreBuffer[i] >= minScoreInclusive {
			docBuffer[newSize] = docBuffer[i]
			scoreBuffer[newSize] = scoreBuffer[i]
			newSize++
		}
	}
	return newSize
}

func TestVectorUtil_testInt4BitDotProductInvariants(t *testing.T) {
	r := newTestRandom(t)
	iterations := atLeast(r, 10)
	for i := 0; i < iterations; i++ {
		size := nextInt(r, 1, 10)
		d := make([]byte, size)
		q := make([]byte, size*4-1)
		expectPanic(t, func() { Int4BitDotProduct(q, d) })
	}
}

var (
	vectorUtilDefaultedProvider   = vectorization.NewDefaultVectorizationProvider()
	vectorUtilDefOrPanamaProvider = vectorization.NewVectorizationProvider()
)

// int4BitDotProductFunc renders the TestVectorUtil.Int4BitDotProduct interface.
type int4BitDotProductFunc func(q, d []byte) int64

func TestVectorUtil_testBasicInt4BitDotProduct(t *testing.T) {
	testBasicInt4BitDotProductImpl(t, Int4BitDotProduct)
	testBasicInt4BitDotProductImpl(t, vectorUtilDefaultedProvider.GetVectorUtilSupport().Int4BitDotProduct)
	testBasicInt4BitDotProductImpl(t, vectorUtilDefOrPanamaProvider.GetVectorUtilSupport().Int4BitDotProduct)
}

func testBasicInt4BitDotProductImpl(t *testing.T, fn int4BitDotProductFunc) {
	t.Helper()
	check := func(expected int64, q, d []byte) {
		t.Helper()
		if AssertsEnabled() && !(int64(scalarInt4BitDotProduct(q, d)) == expected) {
			panic(NewAssertionError(nil))
		}
		if got := fn(q, d); got != expected {
			t.Fatalf("int4BitDotProduct(%v, %v) = %d, want %d", q, d, got, expected)
		}
	}
	if got := fn([]byte{1, 1, 1, 1}, []byte{1}); got != 15 {
		t.Fatalf("got %d, want 15", got)
	}
	if got := fn([]byte{1, 2, 1, 2, 1, 2, 1, 2}, []byte{1, 2}); got != 30 {
		t.Fatalf("got %d, want 30", got)
	}

	check(60, []byte{1, 2, 3, 1, 2, 3, 1, 2, 3, 1, 2, 3}, []byte{1, 2, 3})                // 4 + 8 + 16 + 32
	check(75, []byte{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4}, []byte{1, 2, 3, 4}) // 5 + 10 + 20 + 40
	check(105, []byte{1, 2, 3, 4, 5, 1, 2, 3, 4, 5, 1, 2, 3, 4, 5, 1, 2, 3, 4, 5},
		[]byte{1, 2, 3, 4, 5}) // 7 + 14 + 28 + 56
	check(135, []byte{1, 2, 3, 4, 5, 6, 1, 2, 3, 4, 5, 6, 1, 2, 3, 4, 5, 6, 1, 2, 3, 4, 5, 6},
		[]byte{1, 2, 3, 4, 5, 6}) // 9 + 18 + 36 + 72
	check(180, []byte{
		1, 2, 3, 4, 5, 6, 7, 1, 2, 3, 4, 5, 6, 7, 1, 2, 3, 4, 5, 6, 7, 1, 2, 3, 4, 5, 6, 7,
	}, []byte{1, 2, 3, 4, 5, 6, 7}) // 12 + 24 + 48 + 96
	check(195, []byte{
		1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6,
		7, 8,
	}, []byte{1, 2, 3, 4, 5, 6, 7, 8}) // 13 + 26 + 52 + 104
	check(225, []byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3,
		4, 5, 6, 7, 8, 9,
	}, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}) // 15 + 30 + 60 + 120
}

func TestVectorUtil_testInt4BitDotProduct(t *testing.T) {
	r := newTestRandom(t)
	testInt4BitDotProductImpl(t, r, Int4BitDotProduct)
	testInt4BitDotProductImpl(t, r, vectorUtilDefaultedProvider.GetVectorUtilSupport().Int4BitDotProduct)
	testInt4BitDotProductImpl(t, r, vectorUtilDefOrPanamaProvider.GetVectorUtilSupport().Int4BitDotProduct)
}

func testInt4BitDotProductImpl(t *testing.T, r *rand.Rand, fn int4BitDotProductFunc) {
	t.Helper()
	iterations := atLeast(r, 50)
	for i := 0; i < iterations; i++ {
		size := r.Intn(5000)
		d := make([]byte, size)
		q := make([]byte, size*4)
		r.Read(d)
		r.Read(q)
		if want, got := int64(scalarInt4BitDotProduct(q, d)), fn(q, d); got != want {
			t.Fatalf("random: got %d, want %d", got, want)
		}

		copy(d, bytes.Repeat([]byte{0x7F}, len(d)))
		copy(q, bytes.Repeat([]byte{0x7F}, len(q)))
		if want, got := int64(scalarInt4BitDotProduct(q, d)), fn(q, d); got != want {
			t.Fatalf("MAX_VALUE: got %d, want %d", got, want)
		}

		copy(d, bytes.Repeat([]byte{0x80}, len(d)))
		copy(q, bytes.Repeat([]byte{0x80}, len(q)))
		if want, got := int64(scalarInt4BitDotProduct(q, d)), fn(q, d); got != want {
			t.Fatalf("MIN_VALUE: got %d, want %d", got, want)
		}
	}
}

func scalarInt4BitDotProduct(q, d []byte) int {
	res := 0
	for i := 0; i < 4; i++ {
		res += vectorUtilPopcount(q, i*len(d), d, len(d)) << i
	}
	return res
}

// vectorUtilPopcount renders the public static TestVectorUtil.popcount.
func vectorUtilPopcount(a []byte, aOffset int, b []byte, length int) int {
	res := 0
	for j := 0; j < length; j++ {
		value := int(a[aOffset+j]&b[j]) & 0xFF
		for k := 0; k < 8; k++ {
			if (value & (1 << k)) != 0 {
				res++
			}
		}
	}
	return res
}
