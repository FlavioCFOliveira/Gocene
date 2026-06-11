// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.TestJavascriptOperations.
package js_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions/js"
)

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

func assertEvalNonZero(t *testing.T, src string) {
	t.Helper()
	eval := compile(t, src)
	got, err := eval(nil)
	if err != nil {
		t.Fatalf("eval(%q): %v", src, err)
	}
	if got == 0 {
		t.Errorf("eval(%q) = 0; want non-zero", src)
	}
}

// --- Basic arithmetic (unchanged) ---

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

func TestJavascriptOperations_DivisionOperation(t *testing.T) {
	assertEvalLong(t, "10/5", 2)
	assertEval(t, "10/0.5", nil, 20)
	assertEvalLong(t, "10/5/2", 1)
	assertEvalLong(t, "(27/9)/3", 1)
	assertEvalLong(t, "27/(9/3)", 9)
	// 1/0 returns MaxInt64 matching Lucene bytecode compiler semantics.
	assertEvalLong(t, "1/0", js.MaxInt64)
}

// --- Unary negation (NOW IMPLEMENTED) ---

func TestJavascriptOperations_NegationOperation(t *testing.T) {
	assertEvalLong(t, "-1", -1)
	assertEvalLong(t, "--1", 1)  // double negation
	assertEvalLong(t, "-(1+2)", -3)
	assertEvalLong(t, "1+-2", -1)
	assertEvalLong(t, "-5+10", 5)
}

// --- Bitwise operators (NOW IMPLEMENTED) ---

func TestJavascriptOperations_BitwiseOperations(t *testing.T) {
	assertEvalLong(t, "5&3", 1)   // 101 & 011 = 001
	assertEvalLong(t, "5|3", 7)   // 101 | 011 = 111
	assertEvalLong(t, "5^3", 6)   // 101 ^ 011 = 110
	assertEvalLong(t, "~0", -1)
	// ~~5: bitwise NOT twice returns the original ToInt32 value.
	eval := compile(t, "~~5")
	got, err := eval(nil)
	if err != nil {
		t.Fatalf("eval(~~5): %v", err)
	}
	if got != 5 {
		t.Errorf("~~5 = %v; want 5", got)
	}
}

// --- Comparison operators (NOW IMPLEMENTED) ---

func TestJavascriptOperations_ComparisonOperations(t *testing.T) {
	assertEvalLong(t, "5>3", 1)
	assertEvalLong(t, "3>5", 0)
	assertEvalLong(t, "3<5", 1)
	assertEvalLong(t, "5<3", 0)
	assertEvalLong(t, "5>=5", 1)
	assertEvalLong(t, "5>=6", 0)
	assertEvalLong(t, "5<=5", 1)
	assertEvalLong(t, "5<=4", 0)
	assertEvalLong(t, "5==5", 1)
	assertEvalLong(t, "5==6", 0)
	assertEvalLong(t, "5!=6", 1)
	assertEvalLong(t, "5!=5", 0)
}

// --- Boolean operators (NOW IMPLEMENTED) ---

func TestJavascriptOperations_BooleanOperations(t *testing.T) {
	// && (AND)
	assertEvalLong(t, "1&&1", 1)
	assertEvalLong(t, "1&&0", 0)
	assertEvalLong(t, "0&&1", 0)
	assertEvalLong(t, "5&&3", 1) // both non-zero → true
	// || (OR)
	assertEvalLong(t, "1||0", 1)
	assertEvalLong(t, "0||1", 1)
	assertEvalLong(t, "0||0", 0)
	assertEvalLong(t, "5||0", 1)
	// ! (NOT)
	assertEvalLong(t, "!1", 0)
	assertEvalLong(t, "!0", 1)
	assertEvalLong(t, "!!5", 1) // double negation
}

// --- Conditional operator (NOW IMPLEMENTED) ---

func TestJavascriptOperations_ConditionalOperation(t *testing.T) {
	assertEvalLong(t, "1?10:20", 10)
	assertEvalLong(t, "0?10:20", 20)
	assertEvalLong(t, "(5>3)?1:0", 1)
	assertEvalLong(t, "(3>5)?1:0", 0)
	assertEvalLong(t, "(5==5)?100:200", 100)
}

// --- Shift operators (NOW IMPLEMENTED) ---

func TestJavascriptOperations_ShiftOperations(t *testing.T) {
	assertEvalLong(t, "1<<1", 2)
	assertEvalLong(t, "1<<3", 8)
	assertEvalLong(t, "8>>1", 4)
	assertEvalLong(t, "16>>2", 4)
	assertEvalLong(t, "-1>>1", -1) // arithmetic shift preserves sign
}

// --- Precedence ---

func TestJavascriptOperations_Precedence(t *testing.T) {
	assertEvalLong(t, "2+3*4", 14)
	assertEvalLong(t, "10-2*3", 4)
	assertEvalLong(t, "(2+3)*4", 20)
	assertEvalLong(t, "1+2+3+4", 10)
}

// --- Division by zero ---

func TestJavascriptOperations_DivByZero(t *testing.T) {
	// Integer division by zero returns MaxInt64.
	assertEvalLong(t, "10/0", js.MaxInt64)
	// Floating-point division by zero returns +Inf.
	eval := compile(t, "10.5/0")
	got, err := eval(nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if !math.IsInf(got, 1) {
		t.Errorf("10.5/0 = %v; want +Inf", got)
	}
}
