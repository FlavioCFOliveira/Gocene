// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.TestAPISanity.
// The Java original checks that ANTLR-compiled Expression subclasses declare
// the same exception types as the abstract base class. In Go, Expression is a
// concrete struct; the equivalent sanity check verifies that EvaluateDoubleValues
// correctly propagates errors returned by the evaluator func.
package js_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions"
	"github.com/FlavioCFOliveira/Gocene/expressions/js"
)

// TestAPISanity_EvaluatorErrorPropagation verifies that errors returned by
// the evaluator func are propagated by EvaluateDoubleValues, mirroring the
// Java check that evaluate() declares IOException.
func TestAPISanity_EvaluatorErrorPropagation(t *testing.T) {
	sentinel := errors.New("evaluator error")
	expr := expressions.NewExpressionWithDoubleValues(
		"x",
		[]string{"x"},
		func(_ []expressions.DoubleValues) (float64, error) {
			return 0, sentinel
		},
	)
	dv := expressions.NewConstantDoubleValues(0)
	_, err := expr.EvaluateDoubleValues([]expressions.DoubleValues{dv})
	if !errors.Is(err, sentinel) {
		t.Errorf("EvaluateDoubleValues: expected sentinel error, got %v", err)
	}
}

// TestAPISanity_CompileProducesWorkingExpression verifies that a compiled
// expression (via JavascriptCompiler) evaluates correctly without errors.
func TestAPISanity_CompileProducesWorkingExpression(t *testing.T) {
	c := js.JavascriptCompiler{}
	expr, err := c.Compile("1")
	if err != nil {
		t.Fatalf("Compile(\"1\"): %v", err)
	}
	got, err := expr.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got != 1 {
		t.Errorf("expected 1, got %v", got)
	}
}
