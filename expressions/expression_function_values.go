// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.ExpressionFunctionValues.
package expressions

import "fmt"

// ExpressionFunctionValues is a DoubleValues that evaluates an expression for
// each document. It is created by ExpressionValueSource.GetValues.
//
// Mirrors org.apache.lucene.expressions.ExpressionFunctionValues.
type ExpressionFunctionValues struct {
	expression     *Expression
	functionValues []DoubleValues
	currentValue   float64
	currentDoc     int
	computed       bool
}

// NewExpressionFunctionValues creates an ExpressionFunctionValues for the
// given expression and its resolved per-variable DoubleValues instances.
// Panics if expression or functionValues is nil.
func NewExpressionFunctionValues(expression *Expression, functionValues []DoubleValues) *ExpressionFunctionValues {
	if expression == nil {
		panic("expression must not be nil")
	}
	if functionValues == nil {
		panic("functionValues must not be nil")
	}
	return &ExpressionFunctionValues{
		expression:     expression,
		functionValues: functionValues,
		currentDoc:     -1,
	}
}

// AdvanceExact positions on docID. Returns true (expression always produces a
// value). Matches the Java implementation that never returns false.
func (v *ExpressionFunctionValues) AdvanceExact(docID int) (bool, error) {
	if v.currentDoc == docID {
		return true, nil
	}
	v.currentDoc = docID
	v.computed = false
	return true, nil
}

// DoubleValue evaluates the expression for the current document. The result
// is cached so that repeated calls for the same document cost only one
// evaluation.
func (v *ExpressionFunctionValues) DoubleValue() (float64, error) {
	if !v.computed {
		for _, fv := range v.functionValues {
			if _, err := fv.AdvanceExact(v.currentDoc); err != nil {
				return 0, fmt.Errorf("ExpressionFunctionValues.DoubleValue: advance: %w", err)
			}
		}
		val, err := v.expression.EvaluateDoubleValues(v.functionValues)
		if err != nil {
			return 0, err
		}
		v.currentValue = val
		v.computed = true
	}
	return v.currentValue, nil
}

var _ DoubleValues = (*ExpressionFunctionValues)(nil)
