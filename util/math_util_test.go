// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math"
	"math/big"
	"math/rand"
	"testing"
)

// gcdSlow is a BigInt-based reference used to validate MathGCD over
// randomised inputs, mirroring TestMathUtil.gcd(long, long).
func gcdSlow(a, b int64) int64 {
	bi := new(big.Int).GCD(nil, nil, big.NewInt(a), big.NewInt(b))
	return bi.Int64()
}

var mathTestPrimes = []int64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29}

func mathRandomLong(rng *rand.Rand) int64 {
	switch rng.Intn(3) {
	case 0:
		l := int64(1)
		if rng.Intn(2) == 0 {
			l *= -1
		}
		for _, p := range mathTestPrimes {
			m := rng.Intn(3)
			for j := 0; j < m; j++ {
				l *= p
			}
		}
		return l
	case 1:
		return rng.Int63() - rng.Int63()
	default:
		pool := []int64{math.MinInt64, math.MaxInt64, 0, -1, 1}
		return pool[rng.Intn(len(pool))]
	}
}

func TestMathUtil_GCD(t *testing.T) {
	rng := rand.New(rand.NewSource(0xABCDEF))
	for iter := 0; iter < 100; iter++ {
		l1 := mathRandomLong(rng)
		l2 := mathRandomLong(rng)

		// Skip cases where the BigInt-based reference overflows int64
		// (e.g. GCD(MinInt64, 0) == 2^64) — Lucene documents these as
		// the special case that returns MinInt64.
		if (l1 == math.MinInt64 || l2 == math.MinInt64) && (l1 == 0 || l2 == 0 || l1 == l2) {
			continue
		}
		gcd := MathGCD(l1, l2)
		actualGcd := gcdSlow(l1, l2)
		if gcd != actualGcd {
			t.Fatalf("GCD(%d, %d) = %d want %d", l1, l2, gcd, actualGcd)
		}
		if gcd != 0 {
			if (l1/gcd)*gcd != l1 || (l2/gcd)*gcd != l2 {
				t.Fatalf("GCD(%d, %d) = %d does not divide both", l1, l2, gcd)
			}
		}
	}
}

func TestMathUtil_GCD2(t *testing.T) {
	const a, b, c = int64(30), int64(50), int64(77)

	check := func(want, got int64) {
		t.Helper()
		if got != want {
			t.Fatalf("got %d want %d", got, want)
		}
	}

	check(0, MathGCD(0, 0))
	check(b, MathGCD(0, b))
	check(a, MathGCD(a, 0))
	check(b, MathGCD(0, -b))
	check(a, MathGCD(-a, 0))

	check(10, MathGCD(a, b))
	check(10, MathGCD(-a, b))
	check(10, MathGCD(a, -b))
	check(10, MathGCD(-a, -b))

	check(1, MathGCD(a, c))
	check(1, MathGCD(-a, c))
	check(1, MathGCD(a, -c))
	check(1, MathGCD(-a, -c))

	check(int64(3)*(1<<45), MathGCD(int64(3)*(1<<50), int64(9)*(1<<45)))
	check(int64(1)<<45, MathGCD(int64(1)<<45, math.MinInt64))

	check(math.MaxInt64, MathGCD(math.MaxInt64, 0))
	check(math.MaxInt64, MathGCD(-math.MaxInt64, 0))
	check(1, MathGCD(60247241209, 153092023))

	check(math.MinInt64, MathGCD(math.MinInt64, 0))
	check(math.MinInt64, MathGCD(0, math.MinInt64))
	check(math.MinInt64, MathGCD(math.MinInt64, math.MinInt64))
}

func TestMathUtil_AcoshMethod(t *testing.T) {
	if !math.IsNaN(MathAcosh(math.NaN())) {
		t.Fatalf("acosh(NaN) should be NaN")
	}
	if MathAcosh(1) != 0 {
		t.Fatalf("acosh(1) should be 0, got %g", MathAcosh(1))
	}
	if !math.IsInf(MathAcosh(math.Inf(1)), 1) {
		t.Fatalf("acosh(+Inf) should be +Inf")
	}
	for _, v := range []float64{0.9, 0, -0.0, -0.9, -1, -10, math.Inf(-1)} {
		if !math.IsNaN(MathAcosh(v)) {
			t.Fatalf("acosh(%g) should be NaN, got %g", v, MathAcosh(v))
		}
	}

	approx := func(got, want, eps float64) {
		t.Helper()
		if math.Abs(got-want) > eps {
			t.Fatalf("got %g want %g (eps=%g)", got, want, eps)
		}
	}
	approx(MathAcosh(2.5), 1.5667992369724109, 1e-6)
	approx(MathAcosh(1234567.89), 14.719378760739708, 1e-6)
}

func TestMathUtil_AsinhMethod(t *testing.T) {
	if !math.IsNaN(MathAsinh(math.NaN())) {
		t.Fatalf("asinh(NaN) should be NaN")
	}
	if MathAsinh(0) != 0 {
		t.Fatalf("asinh(+0) should be +0")
	}
	if math.Float64bits(MathAsinh(math.Copysign(0, -1))) != math.Float64bits(math.Copysign(0, -1)) {
		t.Fatalf("asinh(-0) should be -0")
	}
	if !math.IsInf(MathAsinh(math.Inf(1)), 1) {
		t.Fatalf("asinh(+Inf) should be +Inf")
	}
	if !math.IsInf(MathAsinh(math.Inf(-1)), -1) {
		t.Fatalf("asinh(-Inf) should be -Inf")
	}
	approx := func(got, want, eps float64) {
		t.Helper()
		if math.Abs(got-want) > eps {
			t.Fatalf("got %g want %g (eps=%g)", got, want, eps)
		}
	}
	approx(MathAsinh(-1234567.89), -14.719378760740035, 1e-6)
	approx(MathAsinh(-2.5), -1.6472311463710958, 1e-6)
	approx(MathAsinh(-1), -0.8813735870195429, 1e-6)
	if MathAsinh(0) != 0 {
		t.Fatalf("asinh(0) should be 0")
	}
	approx(MathAsinh(1), 0.8813735870195429, 1e-6)
	approx(MathAsinh(2.5), 1.6472311463710958, 1e-6)
	approx(MathAsinh(1234567.89), 14.719378760740035, 1e-6)
}

func TestMathUtil_AtanhMethod(t *testing.T) {
	if !math.IsNaN(MathAtanh(math.NaN())) {
		t.Fatalf("atanh(NaN) should be NaN")
	}
	if MathAtanh(0) != 0 {
		t.Fatalf("atanh(+0) should be +0")
	}
	if math.Float64bits(MathAtanh(math.Copysign(0, -1))) != math.Float64bits(math.Copysign(0, -1)) {
		t.Fatalf("atanh(-0) should be -0")
	}
	if !math.IsInf(MathAtanh(1), 1) {
		t.Fatalf("atanh(1) should be +Inf")
	}
	if !math.IsInf(MathAtanh(-1), -1) {
		t.Fatalf("atanh(-1) should be -Inf")
	}
	for _, v := range []float64{1.1, math.Inf(1), -1.1, math.Inf(-1)} {
		if !math.IsNaN(MathAtanh(v)) {
			t.Fatalf("atanh(%g) should be NaN", v)
		}
	}

	approx := func(got, want, eps float64) {
		t.Helper()
		if math.Abs(got-want) > eps {
			t.Fatalf("got %g want %g (eps=%g)", got, want, eps)
		}
	}
	approx(MathAtanh(-0.5), -0.5493061443340549, 1e-6)
	approx(MathAtanh(0.5), 0.5493061443340549, 1e-6)
}

func TestMathUtil_UnsignedMin(t *testing.T) {
	type tc struct {
		a, b, want int32
	}
	// math.MaxInt32 + 1 wraps to math.MinInt32 in int32 arithmetic; this
	// is exactly what Java's Integer.MAX_VALUE + 1 does for the
	// "interpret as unsigned" tests.
	cases := []tc{
		{0, 0, 0},
		{0, 3, 0},
		{3, 0, 0},
		{0, math.MaxInt32, 0},
		{math.MaxInt32, 0, 0},
		{math.MaxInt32, math.MinInt32, math.MaxInt32}, // MAX+1 wraps to MIN
		{math.MinInt32, math.MaxInt32, math.MaxInt32},
		{math.MinInt32, -1, math.MinInt32},
		{-1, math.MinInt32, math.MinInt32},
		{math.MinInt32, math.MinInt32, math.MinInt32},
	}
	for _, c := range cases {
		got := MathUnsignedMin(c.a, c.b)
		if got != c.want {
			t.Fatalf("UnsignedMin(%d, %d)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestMathUtil_LogIntBase(t *testing.T) {
	if MathLogIntBase(0, 2) != 0 {
		t.Fatalf("log(0, 2) want 0")
	}
	if MathLogIntBase(-5, 2) != 0 {
		t.Fatalf("log(-5, 2) want 0")
	}
	if MathLogIntBase(1, 2) != 0 {
		t.Fatalf("log(1, 2) want 0")
	}
	if MathLogIntBase(8, 2) != 3 {
		t.Fatalf("log(8, 2) want 3 got %d", MathLogIntBase(8, 2))
	}
	if MathLogIntBase(1023, 2) != 9 {
		t.Fatalf("log(1023, 2) want 9 got %d", MathLogIntBase(1023, 2))
	}
	if MathLogIntBase(81, 3) != 4 {
		t.Fatalf("log(81, 3) want 4 got %d", MathLogIntBase(81, 3))
	}
}

func TestMathUtil_LogIntBase_PanicsOnInvalidBase(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on base <= 1 with non-2 base")
		}
	}()
	MathLogIntBase(100, 1)
}

func TestMathUtil_Log(t *testing.T) {
	if v := MathLog(10, 1000); math.Abs(v-3) > 1e-12 {
		t.Fatalf("log_10(1000)=%g want 3", v)
	}
	if v := MathLog(math.E, math.E*math.E); math.Abs(v-2) > 1e-12 {
		t.Fatalf("ln(e^2)=%g want 2", v)
	}
}

func TestMathUtil_SumRelativeErrorBound(t *testing.T) {
	if MathSumRelativeErrorBound(0) != 0 {
		t.Fatalf("0 case")
	}
	if MathSumRelativeErrorBound(1) != 0 {
		t.Fatalf("1 case")
	}
	u := math.Ldexp(1.0, -52)
	for _, n := range []int{2, 3, 10, 100, 1000} {
		want := float64(n-1) * u
		got := MathSumRelativeErrorBound(n)
		if got != want {
			t.Fatalf("bound(%d)=%g want %g", n, got, want)
		}
	}
}

func TestMathUtil_SumUpperBound(t *testing.T) {
	if MathSumUpperBound(1.5, 0) != 1.5 {
		t.Fatalf("n=0 should return sum unchanged")
	}
	if MathSumUpperBound(1.5, 2) != 1.5 {
		t.Fatalf("n=2 should return sum unchanged")
	}
	got := MathSumUpperBound(1.0, 100)
	b := MathSumRelativeErrorBound(100)
	want := (1.0 + 2*b) * 1.0
	if got != want {
		t.Fatalf("upper(1.0, 100)=%g want %g", got, want)
	}
}
