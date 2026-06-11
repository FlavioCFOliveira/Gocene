// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.TestDemoExpressions.
package expressions_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions/js"
)

// TestDemoExpressions exercises compiled expressions via the JavascriptCompiler
// and verifies they evaluate correctly with sample data.
func TestDemoExpressions(t *testing.T) {
	// Test basic arithmetic: a + b
	expr, err := js.JavascriptCompiler{}.Compile("a + b")
	if err != nil {
		t.Fatal(err)
	}
	result, err := expr.Evaluate(map[string]float64{"a": 3, "b": 4})
	if err != nil {
		t.Fatal(err)
	}
	if result != 7 {
		t.Errorf("a + b = %v, want 7", result)
	}

	// Test expression with built-in functions and variables
	expr2, err := js.JavascriptCompiler{}.Compile("sqrt(x) * 2")
	if err != nil {
		t.Fatal(err)
	}
	result2, err2 := expr2.Evaluate(map[string]float64{"x": 9})
	if err2 != nil {
		t.Fatal(err2)
	}
	if result2 != 6 {
		t.Errorf("sqrt(9) * 2 = %v, want 6", result2)
	}

	// Test expression with multiple variables and functions
	expr3, err := js.JavascriptCompiler{}.Compile("max(a, b) + min(c, d)")
	if err != nil {
		t.Fatal(err)
	}
	result3, err3 := expr3.Evaluate(map[string]float64{"a": 10, "b": 5, "c": 3, "d": 8})
	if err3 != nil {
		t.Fatal(err3)
	}
	if result3 != 13 {
		t.Errorf("max(10,5) + min(3,8) = %v, want 13", result3)
	}

	// Test expression with only constants and built-in functions (no variables)
	expr4, err := js.JavascriptCompiler{}.Compile("sqrt(16) + abs(-5) + ceil(3.7)")
	if err != nil {
		t.Fatal(err)
	}
	result4, err4 := expr4.Evaluate(nil)
	if err4 != nil {
		t.Fatal(err4)
	}
	if result4 != 13 {
		t.Errorf("sqrt(16) + abs(-5) + ceil(3.7) = %v, want 13", result4)
	}

	// Test expression metadata
	if len(expr.Variables) != 2 {
		t.Errorf("'a + b' should report 2 variables, got %d: %v", len(expr.Variables), expr.Variables)
	}
	if len(expr.GetVariables()) != 2 {
		t.Errorf("GetVariables should return 2, got %d", len(expr.GetVariables()))
	}
	if expr.GetSourceText() != "a + b" {
		t.Errorf("GetSourceText = %q, want %q", expr.GetSourceText(), "a + b")
	}
}

// TestDemoExpressions_WithComparisonOperators tests expressions with comparison
// and boolean operators.
func TestDemoExpressions_WithComparisonOperators(t *testing.T) {
	// Equality comparison returns 1 if true, 0 if false
	expr, err := js.JavascriptCompiler{}.Compile("(a == b) * 2 + 1")
	if err != nil {
		t.Fatal(err)
	}
	result, err := expr.Evaluate(map[string]float64{"a": 5, "b": 5})
	if err != nil {
		t.Fatal(err)
	}
	if result != 3 {
		t.Errorf("(5==5)*2+1 = %v, want 3", result)
	}

	// Inequality comparison
	expr2, err := js.JavascriptCompiler{}.Compile("(a != b) * 3")
	if err != nil {
		t.Fatal(err)
	}
	result2, err2 := expr2.Evaluate(map[string]float64{"a": 5, "b": 3})
	if err2 != nil {
		t.Fatal(err2)
	}
	if result2 != 3 {
		t.Errorf("(5!=3)*3 = %v, want 3", result2)
	}

	// Greater-or-equal comparison
	expr3, err := js.JavascriptCompiler{}.Compile("(a >= b) * 5")
	if err != nil {
		t.Fatal(err)
	}
	result3, err3 := expr3.Evaluate(map[string]float64{"a": 5, "b": 3})
	if err3 != nil {
		t.Fatal(err3)
	}
	if result3 != 5 {
		t.Errorf("(5>=3)*5 = %v, want 5", result3)
	}
}

// TestDemoExpressions_WithConditional tests the ternary conditional operator.
func TestDemoExpressions_WithConditional(t *testing.T) {
	// Ternary condition
	expr, err := js.JavascriptCompiler{}.Compile("a > 0 ? b : c")
	if err != nil {
		t.Fatal(err)
	}

	// a > 0 is true, so picks b=10
	result, err := expr.Evaluate(map[string]float64{"a": 1, "b": 10, "c": 20})
	if err != nil {
		t.Fatal(err)
	}
	if result != 10 {
		t.Errorf("a>0 ? b : c with a=1 = %v, want 10", result)
	}

	// a > 0 is false, so picks c=20
	result2, err := expr.Evaluate(map[string]float64{"a": -1, "b": 10, "c": 20})
	if err != nil {
		t.Fatal(err)
	}
	if result2 != 20 {
		t.Errorf("a>0 ? b : c with a=-1 = %v, want 20", result2)
	}
}
