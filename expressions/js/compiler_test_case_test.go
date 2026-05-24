// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.CompilerTestCase.
// CompilerTestCase is a base class used by TestJavascriptOperations,
// TestCustomFunctions, TestJavascriptFunction, and TestAPISanity. It provides
// a compile() helper that invokes JavascriptCompiler. In Gocene the equivalent
// is JavascriptCompiler{}.Compile(source), available in the current package.
//
// The Java version also accepts custom functions via MethodHandles; that
// mechanism is Java-specific and has no equivalent in Go — custom functions
// are added directly to the expression_math.go switch table instead.
package js_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions/js"
)

// TestCompilerTestCase verifies that the Gocene JavascriptCompiler handles
// the subset of the grammar implemented in the hand-written parser.
// The tests that require the full ANTLR grammar (bitwise operators,
// comparison operators, conditional expressions, unary negation, etc.) are
// skipped; they are covered by TestJavascriptOperations which delegates
// to the same skip explanation.
func TestCompilerTestCase_BasicCompile(t *testing.T) {
	cases := []struct {
		src      string
		vars     map[string]float64
		expected float64
	}{
		{"1", nil, 1},
		{"1+1", nil, 2},
		{"5*10", nil, 50},
		{"10/5", nil, 2},
		{"1+2*3", nil, 7},
		{"(1+2)*3", nil, 9},
		{"sqrt(4)", nil, 2},
		{"abs(0.0-5)", nil, 5}, // unary negation not yet supported; use subtraction
		{"a+b", map[string]float64{"a": 3, "b": 7}, 10},
	}

	c := js.JavascriptCompiler{}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.src, func(t *testing.T) {
			expr, err := c.Compile(tc.src)
			if err != nil {
				t.Fatalf("Compile(%q): %v", tc.src, err)
			}
			got, err := expr.Evaluate(tc.vars)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if got != tc.expected {
				t.Errorf("Compile(%q): expected %v, got %v", tc.src, tc.expected, got)
			}
		})
	}
}
