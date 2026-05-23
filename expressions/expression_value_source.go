// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.ExpressionValueSource.
package expressions

import (
	"fmt"
)

// DoubleValuesBindings resolves variable names in an expression to
// DoubleValuesSource instances. It is the Lucene-API counterpart of
// Bindings that returns DoubleValuesSource rather than the legacy ValueSource.
//
// Mirrors the role of org.apache.lucene.expressions.Bindings when used with
// the DoubleValuesSource-based API.
type DoubleValuesBindings interface {
	// GetDoubleValuesSource returns the DoubleValuesSource for the given variable
	// name. The second return value is false when the name is not registered.
	GetDoubleValuesSource(name string) (DoubleValuesSource, bool)
}

// ExpressionValueSource is a DoubleValuesSource that evaluates an Expression
// given a set of per-variable DoubleValuesSource instances resolved from a
// DoubleValuesBindings.
//
// Mirrors org.apache.lucene.expressions.ExpressionValueSource.
type ExpressionValueSource struct {
	expression  *Expression
	variables   []DoubleValuesSource
	needsScores bool
}

// NewExpressionValueSource constructs an ExpressionValueSource by resolving
// each variable in expression against bindings.
// Returns an error if any variable is not registered in bindings.
func NewExpressionValueSource(bindings DoubleValuesBindings, expression *Expression) (*ExpressionValueSource, error) {
	if bindings == nil {
		return nil, fmt.Errorf("bindings must not be nil")
	}
	if expression == nil {
		return nil, fmt.Errorf("expression must not be nil")
	}
	variables := make([]DoubleValuesSource, len(expression.Variables))
	needsScores := false
	for i, name := range expression.Variables {
		src, ok := bindings.GetDoubleValuesSource(name)
		if !ok {
			return nil, fmt.Errorf("variable (%s) does not exist", name)
		}
		if src.NeedsScores() {
			needsScores = true
		}
		variables[i] = src
	}
	return &ExpressionValueSource{
		expression:  expression,
		variables:   variables,
		needsScores: needsScores,
	}, nil
}

// newExpressionValueSourceRaw builds an ExpressionValueSource directly from
// pre-resolved variables. Used internally by Rewrite.
func newExpressionValueSourceRaw(variables []DoubleValuesSource, expression *Expression, needsScores bool) *ExpressionValueSource {
	return &ExpressionValueSource{
		expression:  expression,
		variables:   variables,
		needsScores: needsScores,
	}
}

// GetValues returns a DoubleValues that evaluates expression for each document,
// using scores as the scores source for any variable that needs scores.
// scores may be nil when no variable needs scores.
func (s *ExpressionValueSource) GetValues(scores DoubleValues) (DoubleValues, error) {
	valuesCache := make(map[string]DoubleValues, len(s.variables))
	externalValues := make([]DoubleValues, len(s.expression.Variables))

	for i, name := range s.expression.Variables {
		dv, cached := valuesCache[name]
		if !cached {
			var err error
			dv, err = s.variables[i].GetValues(scores)
			if err != nil {
				return nil, fmt.Errorf("ExpressionValueSource.GetValues: variable %q: %w", name, err)
			}
			if dv == nil {
				return nil, fmt.Errorf("unrecognized variable (%s) referenced in expression (%s)",
					name, s.expression.SourceText)
			}
			valuesCache[name] = dv
		}
		externalValues[i] = zeroWhenUnpositioned(dv)
	}

	return NewExpressionFunctionValues(s.expression, externalValues), nil
}

// NeedsScores reports whether any of the variable sources depends on scores.
func (s *ExpressionValueSource) NeedsScores() bool { return s.needsScores }

// IsCacheable reports whether all variable sources are cacheable.
func (s *ExpressionValueSource) IsCacheable() bool {
	for _, v := range s.variables {
		if !v.IsCacheable() {
			return false
		}
	}
	return true
}

// String returns a human-readable representation.
func (s *ExpressionValueSource) String() string {
	return "expr(" + s.expression.SourceText + ")"
}

// zeroWhenUnpositioned wraps a DoubleValues so that:
//  1. It always returns true from AdvanceExact (never skips a doc).
//  2. It lazily advances the underlying source only when DoubleValue is called,
//     defaulting to 0 if the source has no value for the current doc.
//
// This mirrors the anonymous class returned by ExpressionValueSource.zeroWhenUnpositioned
// in org.apache.lucene.expressions.ExpressionValueSource.
func zeroWhenUnpositioned(in DoubleValues) DoubleValues {
	return &lazyZeroDoubleValues{in: in, currentDoc: -1}
}

type lazyZeroDoubleValues struct {
	in         DoubleValues
	currentDoc int
	value      float64
	computed   bool
}

func (z *lazyZeroDoubleValues) AdvanceExact(doc int) (bool, error) {
	if z.currentDoc == doc {
		return true, nil
	}
	z.currentDoc = doc
	z.computed = false
	return true, nil
}

func (z *lazyZeroDoubleValues) DoubleValue() (float64, error) {
	if !z.computed {
		ok, err := z.in.AdvanceExact(z.currentDoc)
		if err != nil {
			return 0, err
		}
		if ok {
			v, err := z.in.DoubleValue()
			if err != nil {
				return 0, err
			}
			z.value = v
		} else {
			z.value = 0
		}
		z.computed = true
	}
	return z.value, nil
}

var _ DoubleValues = (*lazyZeroDoubleValues)(nil)
var _ DoubleValuesSource = (*ExpressionValueSource)(nil)
