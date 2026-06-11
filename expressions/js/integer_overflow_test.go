// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package js_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions/js"
)

// TestToInt32 verifies ECMAScript ToInt32 semantics.
func TestToInt32(t *testing.T) {
	tests := []struct {
		input  float64
		expect int32
	}{
		{0, 0},
		{1, 1},
		{-1, -1},
		{2147483647, 2147483647},          // MaxInt32
		{-2147483648, -2147483648},         // MinInt32
		{2147483648, -2147483648},          // overflow wraps
		{4294967296, 0},                    // 2^32 wraps to 0
		{3.14, 3},                          // truncated
		{-3.94, -3},                        // truncated toward zero
		{math.Pow(2, 33), 0},               // 2^33 wraps to 0 in 32 bits
	}

	for _, tt := range tests {
		result := js.ToInt32(tt.input)
		if result != tt.expect {
			t.Errorf("ToInt32(%v) = %d, want %d", tt.input, result, tt.expect)
		}
	}
	t.Logf("ToInt32 tests passed")
}

// TestToUint32 verifies ECMAScript ToUint32 semantics.
func TestToUint32(t *testing.T) {
	tests := []struct {
		input  float64
		expect uint32
	}{
		{0, 0},
		{1, 1},
		{-1, 4294967295},               // wraps to max uint32
		{4294967295, 4294967295},        // MaxUint32
		{4294967296, 0},                 // wraps
		{-3, 4294967293},                // -3 → 2^32 - 3
	}

	for _, tt := range tests {
		result := js.ToUint32(tt.input)
		if result != tt.expect {
			t.Errorf("ToUint32(%v) = %d, want %d", tt.input, result, tt.expect)
		}
	}
	t.Logf("ToUint32 tests passed")
}

// TestIntDiv verifies integer division with overflow semantics.
func TestIntDiv(t *testing.T) {
	tests := []struct {
		a, b   float64
		expect float64
	}{
		{10, 3, 3},                         // 10/3 = 3 (integer division)
		{10, -3, -3},                        // trunc toward zero
		{1, 0, float64(js.MaxInt64)},        // 1/0 = MaxInt64
		{-1, 0, float64(-js.MaxInt64)},      // -1/0 = -MaxInt64
		{0, 0, 0},                            // 0/0 = 0
		{3.14, 1.0, 3},                       // truncated before division
	}

	for _, tt := range tests {
		result := js.IntDiv(tt.a, tt.b)
		if result != tt.expect {
			t.Errorf("IntDiv(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expect)
		}
	}
	t.Logf("IntDiv tests passed")
}

// TestExpression_DivisionByZero verifies that the compiler applies
// integer overflow semantics for integer division by zero.
func TestExpression_DivisionByZero(t *testing.T) {
	compiler := js.JavascriptCompiler{}

	// Integer division by zero returns MaxInt64.
	expr, err := compiler.Compile("1/0")
	if err != nil {
		t.Fatalf("Compile(1/0): %v", err)
	}
	result, err := expr.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate(1/0): %v", err)
	}
	if result != float64(js.MaxInt64) {
		t.Errorf("1/0 = %v, want %d (MaxInt64)", result, js.MaxInt64)
	}

	// Floating-point division by zero returns +Inf.
	expr2, err := compiler.Compile("1.5/0")
	if err != nil {
		t.Fatalf("Compile(1.5/0): %v", err)
	}
	result2, err := expr2.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate(1.5/0): %v", err)
	}
	if !math.IsInf(result2, 1) {
		t.Errorf("1.5/0 = %v, want +Inf", result2)
	}

	t.Logf("DivisionByZero tests passed")
}
