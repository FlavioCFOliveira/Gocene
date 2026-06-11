// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.TestExpressionSortField.
package expressions_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions"
	"github.com/FlavioCFOliveira/Gocene/expressions/js"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// testDoubleValuesSource is a DoubleValuesSource that returns a constant value
// for every document. Used by test bindings.
type testDoubleValuesSource struct {
	value float64
}

func (s *testDoubleValuesSource) GetValues(_ expressions.DoubleValues) (expressions.DoubleValues, error) {
	return expressions.NewConstantDoubleValues(s.value), nil
}

func (s *testDoubleValuesSource) NeedsScores() bool { return false }

func (s *testDoubleValuesSource) IsCacheable() bool { return true }

// testDoubleValuesBindings is a map-backed DoubleValuesBindings for use in tests.
type testDoubleValuesBindings struct {
	sources map[string]expressions.DoubleValuesSource
}

func newTestBindings() *testDoubleValuesBindings {
	return &testDoubleValuesBindings{sources: make(map[string]expressions.DoubleValuesSource)}
}

func (b *testDoubleValuesBindings) Add(name string, src expressions.DoubleValuesSource) {
	b.sources[name] = src
}

func (b *testDoubleValuesBindings) GetDoubleValuesSource(name string) (expressions.DoubleValuesSource, bool) {
	src, ok := b.sources[name]
	return src, ok
}

// TestExpressionSortField verifies that Expression.GetSortField produces
// correctly configured SortField instances with and without reversal.
func TestExpressionSortField(t *testing.T) {
	// Compile an expression with a single variable
	expr, err := js.JavascriptCompiler{}.Compile("x * 2")
	if err != nil {
		t.Fatal(err)
	}

	// Create bindings with a constant source for variable x
	bindings := newTestBindings()
	bindings.Add("x", &testDoubleValuesSource{value: 5})

	// Non-reversed sort field
	sf, err := expr.GetSortField(bindings, false)
	if err != nil {
		t.Fatal(err)
	}
	if sf == nil {
		t.Fatal("GetSortField returned nil")
	}
	if sf.GetField() != "x * 2" {
		t.Errorf("sort field name = %q, want %q", sf.GetField(), "x * 2")
	}
	if sf.Type != search.SortFieldTypeScore {
		t.Errorf("sort field type = %v, want SortFieldTypeScore", sf.Type)
	}
	if sf.GetReverse() {
		t.Error("expected non-reversed sort field when reverse=false")
	}

	// Reversed sort field
	sfRev, err := expr.GetSortField(bindings, true)
	if err != nil {
		t.Fatal(err)
	}
	if sfRev == nil {
		t.Fatal("GetSortField(true) returned nil")
	}
	if !sfRev.GetReverse() {
		t.Error("expected reversed sort field when reverse=true")
	}
}

// TestExpressionSortField_ConstantExpression verifies that expressions with no
// variables can still produce SortFields.
func TestExpressionSortField_ConstantExpression(t *testing.T) {
	expr, err := js.JavascriptCompiler{}.Compile("sqrt(100)")
	if err != nil {
		t.Fatal(err)
	}

	// Empty bindings are sufficient since the expression has no variables
	emptyBindings := newTestBindings()
	sf, err := expr.GetSortField(emptyBindings, false)
	if err != nil {
		t.Fatal(err)
	}
	if sf == nil {
		t.Fatal("GetSortField returned nil for constant expression")
	}
	if sf.Type != search.SortFieldTypeScore {
		t.Errorf("constant expression sort field type = %v, want SortFieldTypeScore", sf.Type)
	}
}

// TestExpressionSortField_MissingVariable verifies that GetSortField returns an
// error when an expression variable is not registered in the bindings.
func TestExpressionSortField_MissingVariable(t *testing.T) {
	expr, err := js.JavascriptCompiler{}.Compile("x + y")
	if err != nil {
		t.Fatal(err)
	}

	// Only register x, leave y missing
	partialBindings := newTestBindings()
	partialBindings.Add("x", &testDoubleValuesSource{value: 1})

	_, err = expr.GetSortField(partialBindings, false)
	if err == nil {
		t.Error("expected error for missing variable 'y', got nil")
	}
}
