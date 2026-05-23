// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of relevant portions of
// org.apache.lucene.expressions.TestExpressionValueSource.
package expressions_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions"
)

// --- helpers ---

// constantSource is a DoubleValuesSource that always returns a fixed value.
type constantSource struct {
	v float64
}

func (c *constantSource) GetValues(_ expressions.DoubleValues) (expressions.DoubleValues, error) {
	return expressions.NewConstantDoubleValues(c.v), nil
}
func (c *constantSource) NeedsScores() bool  { return false }
func (c *constantSource) IsCacheable() bool  { return true }

// mapBindings implements DoubleValuesBindings over a map.
type mapBindings map[string]expressions.DoubleValuesSource

func (m mapBindings) GetDoubleValuesSource(name string) (expressions.DoubleValuesSource, bool) {
	s, ok := m[name]
	return s, ok
}

// sumEvaluator returns the sum of all function values.
func sumEvaluator(fvs []expressions.DoubleValues) (float64, error) {
	sum := 0.0
	for _, fv := range fvs {
		v, err := fv.DoubleValue()
		if err != nil {
			return 0, err
		}
		sum += v
	}
	return sum, nil
}

// --- tests ---

// TestExpressionValueSource_GetValues verifies that GetValues returns an
// ExpressionFunctionValues that evaluates the expression per document using
// the per-variable DoubleValuesSource instances.
func TestExpressionValueSource_GetValues(t *testing.T) {
	// expr = a + b  (sum of two constant sources)
	expr := expressions.NewExpressionWithDoubleValues(
		"a + b",
		[]string{"a", "b"},
		sumEvaluator,
	)

	bindings := mapBindings{
		"a": &constantSource{3.0},
		"b": &constantSource{7.0},
	}

	evs, err := expressions.NewExpressionValueSource(bindings, expr)
	if err != nil {
		t.Fatalf("NewExpressionValueSource: %v", err)
	}

	dv, err := evs.GetValues(nil)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}

	ok, err := dv.AdvanceExact(0)
	if err != nil {
		t.Fatalf("AdvanceExact(0): %v", err)
	}
	if !ok {
		t.Fatal("AdvanceExact(0): expected true")
	}

	got, err := dv.DoubleValue()
	if err != nil {
		t.Fatalf("DoubleValue: %v", err)
	}
	if got != 10.0 {
		t.Errorf("expected 10.0, got %v", got)
	}
}

// TestExpressionValueSource_MissingVariable verifies that construction returns
// an error when a variable is not registered in the bindings.
func TestExpressionValueSource_MissingVariable(t *testing.T) {
	expr := expressions.NewExpressionWithDoubleValues(
		"missing",
		[]string{"missing"},
		sumEvaluator,
	)

	bindings := mapBindings{}
	_, err := expressions.NewExpressionValueSource(bindings, expr)
	if err == nil {
		t.Fatal("expected error for missing variable, got nil")
	}
}

// TestExpressionValueSource_NeedsScores verifies that NeedsScores aggregates
// correctly across variables.
func TestExpressionValueSource_NeedsScores(t *testing.T) {
	scoreSource := &scoreNeedingSource{}

	expr := expressions.NewExpressionWithDoubleValues(
		"a + score",
		[]string{"a", "score"},
		sumEvaluator,
	)

	bindings := mapBindings{
		"a":     &constantSource{1.0},
		"score": scoreSource,
	}

	evs, err := expressions.NewExpressionValueSource(bindings, expr)
	if err != nil {
		t.Fatalf("NewExpressionValueSource: %v", err)
	}
	if !evs.NeedsScores() {
		t.Error("expected NeedsScores() == true when a variable source needs scores")
	}

	// Without the score-needing source:
	bindings2 := mapBindings{
		"a":     &constantSource{1.0},
		"score": &constantSource{0.0},
	}
	evs2, _ := expressions.NewExpressionValueSource(bindings2, expr)
	if evs2.NeedsScores() {
		t.Error("expected NeedsScores() == false when no variable source needs scores")
	}
}

// TestExpressionValueSource_ZeroWhenUnpositioned verifies the lazy-zero
// semantics: when the underlying source returns false from AdvanceExact the
// value defaults to 0.
func TestExpressionValueSource_ZeroWhenUnpositioned(t *testing.T) {
	// A source that returns (false, nil) for AdvanceExact to simulate missing values.
	noValueSrc := &neverPositionedSource{}

	expr := expressions.NewExpressionWithDoubleValues(
		"x",
		[]string{"x"},
		func(fvs []expressions.DoubleValues) (float64, error) {
			return fvs[0].DoubleValue()
		},
	)

	bindings := mapBindings{"x": noValueSrc}
	evs, err := expressions.NewExpressionValueSource(bindings, expr)
	if err != nil {
		t.Fatalf("NewExpressionValueSource: %v", err)
	}

	dv, err := evs.GetValues(nil)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}

	if _, err := dv.AdvanceExact(0); err != nil {
		t.Fatalf("AdvanceExact: %v", err)
	}
	got, err := dv.DoubleValue()
	if err != nil {
		t.Fatalf("DoubleValue: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0 when source has no value for doc, got %v", got)
	}
}

// TestExpressionValueSource_NilBindings verifies that nil bindings returns
// an error rather than panicking.
func TestExpressionValueSource_NilBindings(t *testing.T) {
	expr := expressions.NewExpressionWithDoubleValues("a", []string{"a"}, sumEvaluator)
	_, err := expressions.NewExpressionValueSource(nil, expr)
	if err == nil {
		t.Fatal("expected error for nil bindings")
	}
}

// TestExpressionValueSource_String checks that String() returns the expected
// "expr(…)" representation.
func TestExpressionValueSource_String(t *testing.T) {
	expr := expressions.NewExpressionWithDoubleValues("a+b", []string{"a", "b"}, sumEvaluator)
	bindings := mapBindings{"a": &constantSource{1}, "b": &constantSource{2}}
	evs, _ := expressions.NewExpressionValueSource(bindings, expr)
	if got := evs.String(); got != "expr(a+b)" {
		t.Errorf("String(): expected \"expr(a+b)\", got %q", got)
	}
}

// --- helper types ---

type scoreNeedingSource struct{}

func (s *scoreNeedingSource) GetValues(_ expressions.DoubleValues) (expressions.DoubleValues, error) {
	return expressions.NewConstantDoubleValues(1.0), nil
}
func (s *scoreNeedingSource) NeedsScores() bool { return true }
func (s *scoreNeedingSource) IsCacheable() bool { return false }

// neverPositionedSource returns (false, nil) from AdvanceExact to simulate a
// document with no value for a given field.
type neverPositionedSource struct{}

func (n *neverPositionedSource) GetValues(_ expressions.DoubleValues) (expressions.DoubleValues, error) {
	return &neverPositionedValues{}, nil
}
func (n *neverPositionedSource) NeedsScores() bool { return false }
func (n *neverPositionedSource) IsCacheable() bool { return true }

type neverPositionedValues struct{}

func (v *neverPositionedValues) AdvanceExact(_ int) (bool, error) { return false, nil }
func (v *neverPositionedValues) DoubleValue() (float64, error) {
	return 0, errors.New("should not be called when unpositioned")
}
