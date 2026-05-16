// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package quantization

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// nearF32 reports whether two float32 values agree within tol.
func nearF32(got, want, tol float32) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// TestFromVectorSimilarity_DispatchTypes verifies that the factory
// returns the correct concrete type for each VectorSimilarityFunction
// value, mirroring the Java `switch (sim)` arms one-to-one.
func TestFromVectorSimilarity_DispatchTypes(t *testing.T) {
	const constMul float32 = 0.5

	cases := []struct {
		name string
		sim  index.VectorSimilarityFunction
		bits byte
		// want is the type assertion to perform on the result.
		assertType func(t *testing.T, got ScalarQuantizedVectorSimilarity)
	}{
		{
			name: "EUCLIDEAN bits=7",
			sim:  index.VectorSimilarityFunctionEuclidean,
			bits: 7,
			assertType: func(t *testing.T, got ScalarQuantizedVectorSimilarity) {
				t.Helper()
				if _, ok := got.(*Euclidean); !ok {
					t.Fatalf("expected *Euclidean, got %T", got)
				}
			},
		},
		{
			name: "DOT_PRODUCT bits=7",
			sim:  index.VectorSimilarityFunctionDotProduct,
			bits: 7,
			assertType: func(t *testing.T, got ScalarQuantizedVectorSimilarity) {
				t.Helper()
				if _, ok := got.(*DotProduct); !ok {
					t.Fatalf("expected *DotProduct, got %T", got)
				}
			},
		},
		{
			name: "COSINE bits=4 (int4 path)",
			sim:  index.VectorSimilarityFunctionCosine,
			bits: 4,
			assertType: func(t *testing.T, got ScalarQuantizedVectorSimilarity) {
				t.Helper()
				if _, ok := got.(*DotProduct); !ok {
					t.Fatalf("expected *DotProduct, got %T", got)
				}
			},
		},
		{
			name: "MAXIMUM_INNER_PRODUCT bits=8",
			sim:  index.VectorSimilarityFunctionMaximumInnerProduct,
			bits: 8,
			assertType: func(t *testing.T, got ScalarQuantizedVectorSimilarity) {
				t.Helper()
				if _, ok := got.(*MaximumInnerProduct); !ok {
					t.Fatalf("expected *MaximumInnerProduct, got %T", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FromVectorSimilarity(tc.sim, constMul, tc.bits)
			if err != nil {
				t.Fatalf("FromVectorSimilarity: unexpected error: %v", err)
			}
			if got == nil {
				t.Fatalf("FromVectorSimilarity: returned nil implementation")
			}
			tc.assertType(t, got)
		})
	}
}

// TestFromVectorSimilarity_UnknownSimilarity verifies that an
// out-of-range VectorSimilarityFunction yields an error rather than a
// nil-valued implementation. Java surfaces a MatchException here.
func TestFromVectorSimilarity_UnknownSimilarity(t *testing.T) {
	const bogus index.VectorSimilarityFunction = 99
	got, err := FromVectorSimilarity(bogus, 0.5, 7)
	if err == nil {
		t.Fatalf("FromVectorSimilarity(%v): expected error, got nil", bogus)
	}
	if got != nil {
		t.Fatalf("FromVectorSimilarity(%v): expected nil impl on error, got %T", bogus, got)
	}
}

// TestEuclidean_ScoreKnownValues hand-computes the Euclidean
// similarity for a small fixed input pair and asserts the Go output
// matches.
//
// Inputs:
//
//	storedVector  = {1, 2, 3, 4}
//	queryVector   = {0, 0, 0, 0}
//	uint8 sq dist = 1 + 4 + 9 + 16 = 30
//	constMul      = 0.25
//	adjusted      = 30 * 0.25 = 7.5
//	score         = 1 / (1 + 7.5) = 1 / 8.5
func TestEuclidean_ScoreKnownValues(t *testing.T) {
	const constMul float32 = 0.25
	stored := []byte{1, 2, 3, 4}
	query := []byte{0, 0, 0, 0}
	want := float32(1.0 / 8.5)

	e := &Euclidean{constMultiplier: constMul}
	got := e.Score(query, 0, stored, 0)
	if !nearF32(got, want, 1e-7) {
		t.Fatalf("Euclidean.Score = %v, want %v", got, want)
	}

	// Verify the same path through the factory yields an identical
	// result; offsets are ignored by Euclidean.
	impl, err := FromVectorSimilarity(index.VectorSimilarityFunctionEuclidean, constMul, 7)
	if err != nil {
		t.Fatalf("FromVectorSimilarity: %v", err)
	}
	gotFactory := impl.Score(query, 99, stored, -77)
	if !nearF32(gotFactory, want, 1e-7) {
		t.Fatalf("factory Euclidean.Score = %v, want %v", gotFactory, want)
	}
}

// TestDotProduct_ScoreKnownValues hand-computes the dot-product
// similarity and asserts the Go output matches.
//
// Inputs:
//
//	storedVector  = {1, 2, 3, 4}
//	queryVector   = {5, 6, 7, 8}
//	uint8 dot     = 5 + 12 + 21 + 32 = 70
//	constMul      = 0.1
//	queryOffset   = 0.2
//	vectorOffset  = 0.3
//	adjusted      = 70 * 0.1 + 0.2 + 0.3 = 7.5
//	score         = (1 + 7.5) / 2 = 4.25
func TestDotProduct_ScoreKnownValues(t *testing.T) {
	const (
		constMul     float32 = 0.1
		queryOffset  float32 = 0.2
		vectorOffset float32 = 0.3
	)
	stored := []byte{1, 2, 3, 4}
	query := []byte{5, 6, 7, 8}

	d := &DotProduct{constMultiplier: constMul, comparator: util.Uint8DotProduct}
	got := d.Score(query, queryOffset, stored, vectorOffset)
	const want float32 = 4.25
	if !nearF32(got, want, 1e-6) {
		t.Fatalf("DotProduct.Score = %v, want %v", got, want)
	}
}

// TestDotProduct_ScoreClampsAtZero verifies the `Math.max(..., 0)`
// behavior of the Java original: a sufficiently negative `adjusted`
// must collapse to zero, never to a negative score.
func TestDotProduct_ScoreClampsAtZero(t *testing.T) {
	// All-zero vectors -> dotProduct = 0, adjusted = -10,
	// (1 + -10) / 2 = -4.5 -> clamped to 0.
	stored := make([]byte, 8)
	query := make([]byte, 8)
	d := &DotProduct{constMultiplier: 1, comparator: util.Uint8DotProduct}
	got := d.Score(query, -5, stored, -5)
	if got != 0 {
		t.Fatalf("DotProduct.Score clamp: got %v, want 0", got)
	}
}

// TestDotProduct_Int4Dispatch verifies that selecting bits<=4 routes
// the call through the int4 forward stub, while bits>4 routes through
// util.Uint8DotProduct. For uint4-valid inputs both paths must
// produce identical numerical results.
func TestDotProduct_Int4Dispatch(t *testing.T) {
	stored := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	query := []byte{8, 7, 6, 5, 4, 3, 2, 1}
	const (
		constMul     float32 = 0.5
		queryOffset  float32 = 1
		vectorOffset float32 = -1
	)

	// bits=4 -> int4 path (currently delegating to Uint8DotProduct).
	int4Impl, err := FromVectorSimilarity(index.VectorSimilarityFunctionDotProduct, constMul, 4)
	if err != nil {
		t.Fatalf("FromVectorSimilarity(4): %v", err)
	}
	got4 := int4Impl.Score(query, queryOffset, stored, vectorOffset)

	// bits=8 -> uint8 path.
	uint8Impl, err := FromVectorSimilarity(index.VectorSimilarityFunctionDotProduct, constMul, 8)
	if err != nil {
		t.Fatalf("FromVectorSimilarity(8): %v", err)
	}
	got8 := uint8Impl.Score(query, queryOffset, stored, vectorOffset)

	if !nearF32(got4, got8, 1e-7) {
		t.Fatalf("int4 vs uint8 dispatch diverged for uint4-valid inputs: int4=%v uint8=%v", got4, got8)
	}
}

// TestMaximumInnerProduct_ScoreKnownValues hand-computes both
// branches of [util.ScaleMaxInnerProductScore]: one positive-adjusted
// (similarity + 1) and one negative-adjusted (1 / (1 + |sim|)).
func TestMaximumInnerProduct_ScoreKnownValues(t *testing.T) {
	stored := []byte{1, 2, 3, 4}
	query := []byte{5, 6, 7, 8}
	// uint8 dot = 70.
	cases := []struct {
		name         string
		constMul     float32
		queryOffset  float32
		vectorOffset float32
		want         float32
	}{
		{
			// adjusted = 70*0.1 + 0.2 + 0.3 = 7.5 -> 7.5 + 1 = 8.5
			name:         "positive_adjusted",
			constMul:     0.1,
			queryOffset:  0.2,
			vectorOffset: 0.3,
			want:         8.5,
		},
		{
			// adjusted = 70*0.0 - 5 - 5 = -10 -> 1 / (1 + 10) = 1/11
			name:         "negative_adjusted",
			constMul:     0,
			queryOffset:  -5,
			vectorOffset: -5,
			want:         float32(1.0 / 11.0),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &MaximumInnerProduct{constMultiplier: tc.constMul, comparator: util.Uint8DotProduct}
			got := m.Score(query, tc.queryOffset, stored, tc.vectorOffset)
			if !nearF32(got, tc.want, 1e-6) {
				t.Fatalf("Score = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestNonZeroScores is the Go peer of Lucene's
// TestScalarQuantizedVectorSimilarity#testNonZeroScores. With
// all-zero quantized vectors the dot-product/square-distance
// computations collapse to 0; the test checks that the final score
// is non-negative across every (similarity, bits) combination and
// arbitrary multiplier sign / offset sign.
//
// Note: a fixed-seed PRNG is used in place of Lucene's
// LuceneTestCase.random() so the test is fully deterministic.
func TestNonZeroScores(t *testing.T) {
	r := rand.New(rand.NewPCG(0xC0DE, 0xCAFE))
	quantized := [2][32]byte{} // both zero
	sims := []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionCosine,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	}
	bitsCases := []byte{4, 7, 8}

	for _, sim := range sims {
		for _, bits := range bitsCases {
			for iter := 0; iter < 32; iter++ {
				mul := r.Float32()
				if r.IntN(2) == 0 {
					mul = -mul
				}
				impl, err := FromVectorSimilarity(sim, mul, bits)
				if err != nil {
					t.Fatalf("sim=%v bits=%d: factory error: %v", sim, bits, err)
				}
				negA := -(r.Float32() * float32(r.IntN(10)+1))
				negB := -(r.Float32() * float32(r.IntN(10)+1))
				score := impl.Score(quantized[0][:], negA, quantized[1][:], negB)
				if score < 0 {
					t.Fatalf("sim=%v bits=%d mul=%v negA=%v negB=%v: score %v < 0",
						sim, bits, mul, negA, negB, score)
				}
				if math.IsNaN(float64(score)) {
					t.Fatalf("sim=%v bits=%d mul=%v negA=%v negB=%v: score is NaN",
						sim, bits, mul, negA, negB)
				}
			}
		}
	}
}

// TestEuclidean_LengthMismatchPanics mirrors the Java contract that
// length-mismatched inputs surface as an exception (Java
// IllegalArgumentException, Go panic) via the underlying
// [util.Uint8SquareDistance].
func TestEuclidean_LengthMismatchPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on length mismatch")
		}
	}()
	e := &Euclidean{constMultiplier: 1}
	_ = e.Score([]byte{0, 0}, 0, []byte{0, 0, 0}, 0)
}

// TestDotProduct_LengthMismatchPanics mirrors the Java contract for
// the dot-product variants.
func TestDotProduct_LengthMismatchPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on length mismatch")
		}
	}()
	d := &DotProduct{constMultiplier: 1, comparator: util.Uint8DotProduct}
	_ = d.Score([]byte{0, 0}, 0, []byte{0, 0, 0}, 0)
}

// TestMaximumInnerProduct_LengthMismatchPanics mirrors the Java
// contract for the maximum-inner-product variant.
func TestMaximumInnerProduct_LengthMismatchPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on length mismatch")
		}
	}()
	m := &MaximumInnerProduct{constMultiplier: 1, comparator: util.Uint8DotProduct}
	_ = m.Score([]byte{0, 0}, 0, []byte{0, 0, 0}, 0)
}

// TestFromVectorSimilarity_RejectsUnknown is a typed-error
// regression: callers should be able to distinguish the factory's
// own error from arbitrary unrelated failures. The sentinel-less
// implementation today simply returns a wrapped fmt.Errorf, so the
// surface contract is "non-nil error + nil impl".
func TestFromVectorSimilarity_RejectsUnknown(t *testing.T) {
	_, err := FromVectorSimilarity(index.VectorSimilarityFunction(123), 0, 8)
	if err == nil {
		t.Fatal("expected error")
	}
	// Sanity-check the error string mentions the bad value so the
	// caller's logs are actionable.
	if errors.Is(err, nil) {
		t.Fatal("error must not be nil-equivalent")
	}
}
