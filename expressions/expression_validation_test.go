// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.TestExpressionValidation.
package expressions_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions"
	"github.com/FlavioCFOliveira/Gocene/expressions/js"
)

// TestExpressionValidation verifies that NewExpressionValueSource validates
// variable bindings correctly, which is the Gocene equivalent of
// SimpleBindings.validate in the Java original.
func TestExpressionValidation(t *testing.T) {
	expr, err := js.JavascriptCompiler{}.Compile("x + y")
	if err != nil {
		t.Fatal(err)
	}

	// All variables present -> success
	complete := newTestBindings()
	complete.Add("x", &testDoubleValuesSource{value: 1})
	complete.Add("y", &testDoubleValuesSource{value: 2})
	evs, err := expressions.NewExpressionValueSource(complete, expr)
	if err != nil {
		t.Errorf("complete bindings: unexpected error: %v", err)
	}
	if evs == nil {
		t.Fatal("NewExpressionValueSource returned nil for valid bindings")
	}

	// Missing variable -> error
	partial := newTestBindings()
	partial.Add("x", &testDoubleValuesSource{value: 1})
	_, err = expressions.NewExpressionValueSource(partial, expr)
	if err == nil {
		t.Error("expected error for missing variable 'y', got nil")
	}

	// Nil bindings -> error
	_, err = expressions.NewExpressionValueSource(nil, expr)
	if err == nil {
		t.Error("nil bindings: expected error, got nil")
	}

	// Nil expression -> error
	_, err = expressions.NewExpressionValueSource(newTestBindings(), nil)
	if err == nil {
		t.Error("nil expression: expected error, got nil")
	}
}

// TestExpressionValidation_ConstantExpression verifies that expressions without
// variables pass validation even with empty bindings.
func TestExpressionValidation_ConstantExpression(t *testing.T) {
	expr, err := js.JavascriptCompiler{}.Compile("sqrt(100)")
	if err != nil {
		t.Fatal(err)
	}

	empty := newTestBindings()
	evs, err := expressions.NewExpressionValueSource(empty, expr)
	if err != nil {
		t.Errorf("constant expression with empty bindings: unexpected error: %v", err)
	}
	if evs == nil {
		t.Fatal("NewExpressionValueSource returned nil for constant expression")
	}
}

// TestExpressionValidation_NeedsScores verifies that NeedsScores is propagated
// correctly when a variable source requires scores.
func TestExpressionValidation_NeedsScores(t *testing.T) {
	expr, err := js.JavascriptCompiler{}.Compile("x + y")
	if err != nil {
		t.Fatal(err)
	}

	scoring := &scoringTestDoubleValuesSource{}
	bindings := newTestBindings()
	bindings.Add("x", &testDoubleValuesSource{value: 1})
	bindings.Add("y", scoring)

	evs, err := expressions.NewExpressionValueSource(bindings, expr)
	if err != nil {
		t.Fatal(err)
	}
	if !evs.NeedsScores() {
		t.Error("expected NeedsScores()=true when a variable source needs scores")
	}
	if evs.IsCacheable() {
		t.Error("expected IsCacheable()=false when a variable source is not cacheable")
	}
}

// scoringTestDoubleValuesSource is a DoubleValuesSource that reports it needs
// scores and is not cacheable, for testing NeedsScores propagation.
type scoringTestDoubleValuesSource struct{}

func (s *scoringTestDoubleValuesSource) GetValues(_ expressions.DoubleValues) (expressions.DoubleValues, error) {
	return expressions.NewConstantDoubleValues(0), nil
}

func (s *scoringTestDoubleValuesSource) NeedsScores() bool { return true }

func (s *scoringTestDoubleValuesSource) IsCacheable() bool { return false }
