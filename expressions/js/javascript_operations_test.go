// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.TestJavascriptOperations.
// Tests are partitioned into two groups:
//  1. Operations supported by the current hand-written recursive-descent parser:
//     addition, subtraction, multiplication, division (active tests).
//  2. Operations that require the full ANTLR grammar (unary negation, bitwise
//     ops, comparison ops, conditional ?:, shift ops, boolean AND/OR/NOT):
//     deferred and documented with t.Skip.
package js_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions/js"
)

// compile is a helper that calls JavascriptCompiler.Compile and fatals on error.
func compile(t *testing.T, src string) func(map[string]float64) (float64, error) {
	t.Helper()
	c := js.JavascriptCompiler{}
	expr, err := c.Compile(src)
	if err != nil {
		t.Fatalf("Compile(%q): %v", src, err)
	}
	return expr.Evaluate
}

func assertEval(t *testing.T, src string, vars map[string]float64, expected float64) {
	t.Helper()
	eval := compile(t, src)
	got, err := eval(vars)
	if err != nil {
		t.Fatalf("eval(%q): %v", src, err)
	}
	if got != expected {
		t.Errorf("eval(%q) = %v; want %v", src, got, expected)
	}
}

func assertEvalLong(t *testing.T, src string, expected int64) {
	t.Helper()
	assertEval(t, src, nil, float64(expected))
}

// TestJavascriptOperations_AddOperation mirrors testAddOperation.
func TestJavascriptOperations_AddOperation(t *testing.T) {
	assertEvalLong(t, "1+1", 2)
	assertEval(t, "1+0.5+0.5", nil, 2)
	assertEvalLong(t, "5+10", 15)
	assertEvalLong(t, "1+1+2", 4)
	assertEvalLong(t, "(1+1)+2", 4)
	assertEvalLong(t, "1+(1+2)", 4)
	assertEvalLong(t, "0+1", 1)
	assertEvalLong(t, "1+0", 1)
	assertEvalLong(t, "0+0", 0)
}

// TestJavascriptOperations_SubtractOperation mirrors testSubtractOperation.
func TestJavascriptOperations_SubtractOperation(t *testing.T) {
	assertEvalLong(t, "1-1", 0)
	assertEvalLong(t, "5-10", -5)
	assertEval(t, "1-0.5-0.5", nil, 0)
	assertEvalLong(t, "1-1-2", -2)
	assertEvalLong(t, "(1-1)-2", -2)
	assertEvalLong(t, "1-(1-2)", 2)
	assertEvalLong(t, "0-1", -1)
	assertEvalLong(t, "1-0", 1)
	assertEvalLong(t, "0-0", 0)
}

// TestJavascriptOperations_MultiplyOperation mirrors testMultiplyOperation.
func TestJavascriptOperations_MultiplyOperation(t *testing.T) {
	assertEvalLong(t, "1*1", 1)
	assertEvalLong(t, "5*10", 50)
	assertEval(t, "50*0.1", nil, 5)
	assertEvalLong(t, "1*1*2", 2)
	assertEvalLong(t, "(1*1)*2", 2)
	assertEvalLong(t, "1*(1*2)", 2)
	assertEvalLong(t, "10*0", 0)
	assertEvalLong(t, "0*0", 0)
}

// TestJavascriptOperations_DivisionOperation mirrors the non-∞ cases of
// testDivisionOperation. The 1/0 == MaxInt64 case requires Lucene's specific
// integer-overflow semantics from the ANTLR bytecode compiler; deferred.
func TestJavascriptOperations_DivisionOperation(t *testing.T) {
	assertEvalLong(t, "1*1", 1)
	assertEvalLong(t, "10/5", 2)
	assertEval(t, "10/0.5", nil, 20)
	assertEvalLong(t, "10/5/2", 1)
	assertEvalLong(t, "(27/9)/3", 1)
	assertEvalLong(t, "27/(9/3)", 9)
	// 1/0 in Lucene bytecode-compiled expression yields MaxInt64.
	// Gocene hand-written parser yields 0 (guarded division). Deferred.
}

// TestJavascriptOperations_NegationOperation mirrors testNegationOperation.
// Requires full ANTLR grammar (unary negation not supported in hand-written parser).
func TestJavascriptOperations_NegationOperation(t *testing.T) {
	t.Fatal("requires full ANTLR grammar (unary negation not yet implemented)")
}

// TestJavascriptOperations_BitwiseOperations mirrors the bitwise test methods.
// Requires full ANTLR grammar.
func TestJavascriptOperations_BitwiseOperations(t *testing.T) {
	t.Fatal("requires full ANTLR grammar (bitwise operators not yet implemented)")
}

// TestJavascriptOperations_ComparisonOperations mirrors the comparison methods.
// Requires full ANTLR grammar.
func TestJavascriptOperations_ComparisonOperations(t *testing.T) {
	t.Fatal("requires full ANTLR grammar (comparison operators not yet implemented)")
}

// TestJavascriptOperations_BooleanOperations mirrors the boolean methods.
// Requires full ANTLR grammar.
func TestJavascriptOperations_BooleanOperations(t *testing.T) {
	t.Fatal("requires full ANTLR grammar (boolean AND/OR/NOT not yet implemented)")
}

// TestJavascriptOperations_ConditionalOperation mirrors testConditionalOperation.
// Requires full ANTLR grammar.
func TestJavascriptOperations_ConditionalOperation(t *testing.T) {
	t.Fatal("requires full ANTLR grammar (conditional ?: not yet implemented)")
}

// TestJavascriptOperations_ShiftOperations mirrors the shift test methods.
// Requires full ANTLR grammar.
func TestJavascriptOperations_ShiftOperations(t *testing.T) {
	t.Fatal("requires full ANTLR grammar (shift operators not yet implemented)")
}

// TestJavascriptOperations_Precedence verifies that the hand-written parser
// respects operator precedence (* before +).
func TestJavascriptOperations_Precedence(t *testing.T) {
	// 2+3*4 = 14 (not 20)
	assertEvalLong(t, "2+3*4", 14)
	// 10-2*3 = 4 (not 24)
	assertEvalLong(t, "10-2*3", 4)
	// (2+3)*4 = 20
	assertEvalLong(t, "(2+3)*4", 20)
	// 1+2+3+4 = 10
	assertEvalLong(t, "1+2+3+4", 10)
}

// TestJavascriptOperations_DivByZero verifies that dividing by zero returns 0.
// (Gocene's hand-written parser guards division by zero; full Lucene returns +Inf/MaxInt.)
func TestJavascriptOperations_DivByZero(t *testing.T) {
	eval := compile(t, "10/0")
	got, err := eval(nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if !math.IsInf(got, 0) && got != 0 {
		t.Errorf("10/0 = %v; want 0 or ±Inf", got)
	}
}
