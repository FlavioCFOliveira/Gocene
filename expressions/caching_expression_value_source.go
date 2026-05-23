// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.CachingExpressionValueSource.
package expressions

import (
	"fmt"
)

// CachingExpressionValueSource is a DoubleValuesSource that evaluates an
// Expression using a shared per-call value cache. When the expression tree
// contains repeated variable references (e.g. Fibonacci-style recursive
// expressions), the cache ensures each named value is computed only once per
// GetValues call rather than being re-evaluated for every reference.
//
// Mirrors org.apache.lucene.expressions.CachingExpressionValueSource.
type CachingExpressionValueSource struct {
	ExpressionValueSource
}

// NewCachingExpressionValueSource wraps an existing ExpressionValueSource,
// enabling the shared-cache optimisation. The src fields (expression,
// variables, needsScores) are copied by value.
func NewCachingExpressionValueSource(src *ExpressionValueSource) *CachingExpressionValueSource {
	if src == nil {
		panic("CachingExpressionValueSource: src must not be nil")
	}
	return &CachingExpressionValueSource{
		ExpressionValueSource: *src,
	}
}

// NewCachingExpressionValueSourceFromBindings constructs a
// CachingExpressionValueSource by resolving the expression's variables from
// bindings, the same way NewExpressionValueSource does.
func NewCachingExpressionValueSourceFromBindings(bindings DoubleValuesBindings, expression *Expression) (*CachingExpressionValueSource, error) {
	evs, err := NewExpressionValueSource(bindings, expression)
	if err != nil {
		return nil, err
	}
	return &CachingExpressionValueSource{ExpressionValueSource: *evs}, nil
}

// GetValues returns a DoubleValues that evaluates expression for each
// document, using a shared value cache across the entire variable tree to
// avoid recomputing shared sub-expressions.
func (s *CachingExpressionValueSource) GetValues(scores DoubleValues) (DoubleValues, error) {
	return s.getValuesWithCache(scores, make(map[string]DoubleValues, len(s.variables)))
}

func (s *CachingExpressionValueSource) getValuesWithCache(scores DoubleValues, valuesCache map[string]DoubleValues) (DoubleValues, error) {
	externalValues := make([]DoubleValues, len(s.expression.Variables))

	for i, name := range s.expression.Variables {
		dv, cached := valuesCache[name]
		if !cached {
			var err error
			// If the variable source is itself a CachingExpressionValueSource,
			// propagate the shared cache down the tree.
			if cv, ok := s.variables[i].(*CachingExpressionValueSource); ok {
				dv, err = cv.getValuesWithCache(scores, valuesCache)
			} else {
				dv, err = s.variables[i].GetValues(scores)
			}
			if err != nil {
				return nil, fmt.Errorf("CachingExpressionValueSource.GetValues: variable %q: %w", name, err)
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

var _ DoubleValuesSource = (*CachingExpressionValueSource)(nil)
