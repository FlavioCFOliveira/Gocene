// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package expressions

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ExpressionRescorer is a rescorer that uses an expression to compute scores.
//
// This is the Go port of Lucene's org.apache.lucene.expressions.ExpressionRescorer.
type ExpressionRescorer struct {
	expression Expression
	bindings   Bindings
}

func NewExpressionRescorer(expression Expression, bindings Bindings) *ExpressionRescorer {
	return &ExpressionRescorer{
		expression: expression,
		bindings:   bindings,
	}
}

func (er *ExpressionRescorer) Rescore(doc int, score float32) float32 {
	val, err := er.expression.Evaluate(er.bindings, doc)
	if err != nil {
		return score
	}
	return score + float32(val)
}
