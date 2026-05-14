// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math"
	"testing"
)

// floatNear reports whether a and b are within tolerance.
func floatNear(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// TestVectorUtil_DotProduct_SmallFixture is the load-bearing byte-level
// fixture: [1,2,3] · [4,5,6] = 32 — the same result Lucene's
// VectorUtil.dotProduct must return.
func TestVectorUtil_DotProduct_SmallFixture(t *testing.T) {
	got := DotProduct([]float32{1, 2, 3}, []float32{4, 5, 6})
	if got != 32.0 {
		t.Fatalf("got %v, want 32.0", got)
	}
}

// TestVectorUtil_DotProduct_Unrolled feeds an array > 32 elements so the
// unrolled accumulator path runs.
func TestVectorUtil_DotProduct_Unrolled(t *testing.T) {
	a := make([]float32, 64)
	b := make([]float32, 64)
	for i := range a {
		a[i] = float32(i + 1)
		b[i] = 2.0
	}
	got := DotProduct(a, b)
	// sum_{i=1..64} 2i = 2 * 64*65/2 = 4160
	if got != 4160.0 {
		t.Fatalf("got %v, want 4160.0", got)
	}
}

// TestVectorUtil_DotProduct_MismatchedDims confirms the panic.
func TestVectorUtil_DotProduct_MismatchedDims(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	_ = DotProduct([]float32{1, 2}, []float32{1, 2, 3})
}

// TestVectorUtil_Cosine_KnownValue: cos([1,0,0], [1,0,0]) == 1.
func TestVectorUtil_Cosine_KnownValue(t *testing.T) {
	got := Cosine([]float32{1, 0, 0}, []float32{1, 0, 0})
	if !floatNear(float64(got), 1.0, 1e-6) {
		t.Fatalf("got %v, want 1.0", got)
	}
}

// TestVectorUtil_Cosine_Orthogonal: orthogonal vectors yield 0.
func TestVectorUtil_Cosine_Orthogonal(t *testing.T) {
	got := Cosine([]float32{1, 0}, []float32{0, 1})
	if !floatNear(float64(got), 0.0, 1e-6) {
		t.Fatalf("got %v, want 0", got)
	}
}

// TestVectorUtil_SquareDistance computes the squared L2 distance.
func TestVectorUtil_SquareDistance(t *testing.T) {
	got := SquareDistance([]float32{0, 0, 0}, []float32{1, 2, 3})
	if got != 14.0 {
		t.Fatalf("got %v, want 14", got)
	}
}

// TestVectorUtil_SquareDistance_Unrolled exercises the > 32 unrolled path.
func TestVectorUtil_SquareDistance_Unrolled(t *testing.T) {
	n := 40
	a := make([]float32, n)
	b := make([]float32, n)
	for i := range a {
		a[i] = 0
		b[i] = 1
	}
	got := SquareDistance(a, b)
	if got != float32(n) {
		t.Fatalf("got %v, want %d", got, n)
	}
}

// TestVectorUtil_DotProductBytes verifies the signed-byte path with a
// fixture that exercises the negative-byte arithmetic.
func TestVectorUtil_DotProductBytes(t *testing.T) {
	a := []byte{1, 2, 0xFF}      // signed: 1, 2, -1
	b := []byte{1, 1, 1}         // signed: 1, 1, 1
	got := DotProductBytes(a, b) // = 1 + 2 + (-1) = 2
	if got != 2 {
		t.Fatalf("got %d, want 2", got)
	}
}

// TestVectorUtil_Uint8DotProduct verifies the unsigned-byte path.
func TestVectorUtil_Uint8DotProduct(t *testing.T) {
	a := []byte{1, 2, 0xFF}
	b := []byte{1, 1, 1}
	got := Uint8DotProduct(a, b) // = 1 + 2 + 255 = 258
	if got != 258 {
		t.Fatalf("got %d, want 258", got)
	}
}

// TestVectorUtil_CosineBytes confirms a trivial fixture and the
// degenerate zero-norm branch returns 0 (not NaN).
func TestVectorUtil_CosineBytes(t *testing.T) {
	got := CosineBytes([]byte{1, 0}, []byte{1, 0})
	if !floatNear(float64(got), 1.0, 1e-6) {
		t.Fatalf("got %v, want 1.0", got)
	}
	if got := CosineBytes([]byte{0, 0}, []byte{1, 0}); got != 0 {
		t.Fatalf("zero norm should yield 0, got %v", got)
	}
}

// TestVectorUtil_L2Normalize_KnownInput normalises a non-trivial vector and
// confirms the magnitude is 1.
func TestVectorUtil_L2Normalize_KnownInput(t *testing.T) {
	v := []float32{3, 4}
	L2Normalize(v)
	mag := math.Sqrt(float64(v[0])*float64(v[0]) + float64(v[1])*float64(v[1]))
	if !floatNear(mag, 1.0, 1e-6) {
		t.Fatalf("magnitude = %v, want 1.0 (vector=%v)", mag, v)
	}
}

// TestVectorUtil_L2Normalize_Zero panics with throwOnZero=true (the default).
func TestVectorUtil_L2Normalize_Zero(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	_ = L2Normalize([]float32{0, 0, 0})
}

// TestVectorUtil_L2NormalizeThrow_Zero leaves the vector unchanged when
// throwOnZero is false.
func TestVectorUtil_L2NormalizeThrow_Zero(t *testing.T) {
	v := []float32{0, 0, 0}
	out := L2NormalizeThrow(v, false)
	for i := range out {
		if out[i] != 0 {
			t.Fatalf("expected zero vector to remain zero, got %v", out)
		}
	}
}

// TestVectorUtil_L2Normalize_AlreadyUnit returns the vector untouched.
func TestVectorUtil_L2Normalize_AlreadyUnit(t *testing.T) {
	v := []float32{1, 0, 0}
	L2Normalize(v)
	if v[0] != 1 || v[1] != 0 || v[2] != 0 {
		t.Fatalf("already-unit vector mutated: %v", v)
	}
}

// TestVectorUtil_IsUnitVector confirms the predicate.
func TestVectorUtil_IsUnitVector(t *testing.T) {
	if !IsUnitVector([]float32{1, 0, 0}) {
		t.Fatalf("unit vector reported non-unit")
	}
	if IsUnitVector([]float32{2, 0, 0}) {
		t.Fatalf("non-unit vector reported as unit")
	}
}

// TestVectorUtil_AddVec sums vectors in place.
func TestVectorUtil_AddVec(t *testing.T) {
	u := []float32{1, 2, 3}
	AddVec(u, []float32{4, 5, 6})
	want := []float32{5, 7, 9}
	for i := range want {
		if u[i] != want[i] {
			t.Fatalf("u[%d] = %v, want %v", i, u[i], want[i])
		}
	}
}

// TestVectorUtil_CheckFinite_OK accepts a finite vector.
func TestVectorUtil_CheckFinite_OK(t *testing.T) {
	v := []float32{1.0, -2.5, 0}
	if got := CheckFinite(v); &got[0] != &v[0] {
		t.Fatalf("CheckFinite should return the input slice for chaining")
	}
}

// TestVectorUtil_CheckFinite_Panics on NaN/Inf.
func TestVectorUtil_CheckFinite_Panics(t *testing.T) {
	for _, bad := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic for %v", bad)
				}
			}()
			_ = CheckFinite([]float32{1, bad})
		}()
	}
}

// TestVectorUtil_ScaleMaxInnerProductScore covers both branches.
func TestVectorUtil_ScaleMaxInnerProductScore(t *testing.T) {
	if got := ScaleMaxInnerProductScore(0.5); got != 1.5 {
		t.Fatalf("got %v, want 1.5", got)
	}
	if got := ScaleMaxInnerProductScore(-1.0); !floatNear(float64(got), 0.5, 1e-6) {
		t.Fatalf("got %v, want 0.5", got)
	}
}

// TestVectorUtil_NormalizeToUnitInterval covers the (-∞, -1) clamp.
func TestVectorUtil_NormalizeToUnitInterval(t *testing.T) {
	if got := NormalizeToUnitInterval(0); !floatNear(float64(got), 0.5, 1e-6) {
		t.Fatalf("got %v, want 0.5", got)
	}
	if got := NormalizeToUnitInterval(-2); got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
	if got := NormalizeToUnitInterval(1); !floatNear(float64(got), 1.0, 1e-6) {
		t.Fatalf("got %v, want 1.0", got)
	}
}

// TestVectorUtil_NormalizeDistanceToUnitInterval verifies the formula.
func TestVectorUtil_NormalizeDistanceToUnitInterval(t *testing.T) {
	if got := NormalizeDistanceToUnitInterval(0); got != 1.0 {
		t.Fatalf("got %v, want 1.0", got)
	}
	if got := NormalizeDistanceToUnitInterval(1); !floatNear(float64(got), 0.5, 1e-6) {
		t.Fatalf("got %v, want 0.5", got)
	}
}

// TestVectorUtil_DotProductScore covers the scaling.
func TestVectorUtil_DotProductScore(t *testing.T) {
	// dot([1,1,1], [1,1,1]) = 3, denom = 3 * 32768 = 98304, score = 0.5 + 3/98304.
	got := DotProductScore([]byte{1, 1, 1}, []byte{1, 1, 1})
	want := float32(0.5 + 3.0/float32(3*(1<<15)))
	if math.Abs(float64(got)-float64(want)) > 1e-6 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestVectorUtil_XorBitCount counts bits in (a XOR b).
func TestVectorUtil_XorBitCount(t *testing.T) {
	if got := XorBitCount([]byte{0xFF, 0x00}, []byte{0x00, 0xFF}); got != 16 {
		t.Fatalf("got %d, want 16", got)
	}
	if got := XorBitCount([]byte{}, []byte{}); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
	// Longer input to hit the stride-8 path.
	a := make([]byte, 17)
	b := make([]byte, 17)
	for i := range a {
		a[i] = 0xFF
	}
	if got := XorBitCount(a, b); got != 17*8 {
		t.Fatalf("got %d, want %d", got, 17*8)
	}
}

// TestVectorUtil_FindNextGEQ checks the binary search returns the lowest
// index whose value is >= target.
func TestVectorUtil_FindNextGEQ(t *testing.T) {
	buf := []int32{1, 3, 5, 7, 9}
	cases := []struct {
		target int32
		want   int
	}{
		{0, 0}, {1, 0}, {2, 1}, {3, 1}, {6, 3}, {9, 4}, {10, 5},
	}
	for _, c := range cases {
		if got := FindNextGEQ(buf, c.target, 0, len(buf)); got != c.want {
			t.Fatalf("FindNextGEQ(target=%d) = %d, want %d", c.target, got, c.want)
		}
	}
}

// TestVectorUtil_SquareDistanceBytes verifies the signed-byte path.
func TestVectorUtil_SquareDistanceBytes(t *testing.T) {
	a := []byte{0, 0, 0xFF}  // 0, 0, -1
	b := []byte{1, 2, 1}     // 1, 2, 1
	if got := SquareDistanceBytes(a, b); got != 1+4+4 {
		t.Fatalf("got %d, want %d", got, 1+4+4)
	}
}

// TestVectorUtil_Uint8SquareDistance verifies the unsigned-byte path.
func TestVectorUtil_Uint8SquareDistance(t *testing.T) {
	a := []byte{0, 0, 0xFF}
	b := []byte{1, 2, 1}
	// (0-1)^2 + (0-2)^2 + (255-1)^2 = 1 + 4 + 64516 = 64521
	if got := Uint8SquareDistance(a, b); got != 64521 {
		t.Fatalf("got %d, want 64521", got)
	}
}
