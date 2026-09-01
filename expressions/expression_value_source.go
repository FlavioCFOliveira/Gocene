// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package expressions

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ExpressionValueSource is a value source that evaluates an expression.
//
// This is the Go port of Lucene's org.apache.lucene.expressions.ExpressionValueSource.
type ExpressionValueSource struct {
	expression Expression
	bindings   Bindings
}

func NewExpressionValueSource(expression Expression, bindings Bindings) *ExpressionValueSource {
	return &ExpressionValueSource{
		expression: expression,
		bindings:   bindings,
	}
}

func (evs *ExpressionValueSource) GetValue(context index.LeafReaderContext, doc int) (float64, error) {
	return evs.expression.Evaluate(evs.bindings, doc)
}

// CachingExpressionValueSource is an ExpressionValueSource that caches its values.
//
// This is the Go port of Lucene's org.apache.lucene.expressions.CachingExpressionValueSource.
type CachingExpressionValueSource struct {
	ExpressionValueSource
	cache []float64
}

func NewCachingExpressionValueSource(evs *ExpressionValueSource, maxDoc int) *CachingExpressionValueSource {
	return &CachingExpressionValueSource{
		ExpressionValueSource: *evs,
		cache:                 make([]float64, maxDoc),
	}
}

func (cevs *CachingExpressionValueSource) GetValue(context index.LeafReaderContext, doc int) (float64, error) {
	// Simplified caching logic.
	if doc >= 0 && doc < len(cevs.cache) {
		return cevs.cache[doc], nil
	}
	val, err := cevs.ExpressionValueSource.GetValue(context, doc)
	if err == nil && doc >= 0 && doc < len(cevs.cache) {
		cevs.cache[doc] = val
	}
	return val, err
}

// ExpressionFunctionValues implements DoubleValuesSource using an expression.
//
// This is the Go port of Lucene's org.apache.lucene.expressions.ExpressionFunctionValues.
type ExpressionFunctionValues struct {
	expression Expression
	bindings   Bindings
}

func NewExpressionFunctionValues(expression Expression, bindings Bindings) *ExpressionFunctionValues {
	return &ExpressionFunctionValues{
		expression: expression,
		bindings:   bindings,
	}
}

func (efv *ExpressionFunctionValues) GetValues(context index.LeafReaderContext) (map[int]float64, error) {
	// In Lucene, this returns a DoubleValues implementation.
	// In Gocene, we use a map for simplicity in this early stage.
	results := make(map[int]float64)
	for doc := 0; doc < context.Reader().MaxDoc(); doc++ {
		val, err := efv.expression.Evaluate(efv.bindings, doc)
		if err == nil {
			results[doc] = val
		}
	}
	return results, nil
}
