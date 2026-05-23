// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.ExpressionRescorer.
package expressions

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ExpressionRescorer is a Rescorer that uses an Expression to re-score first
// pass hits. It composes a search.SortRescorer and delegates Rescore to it.
//
// Mirrors org.apache.lucene.expressions.ExpressionRescorer which extends
// SortRescorer and overrides explain(). The explain override requires full
// index infrastructure (LeafReaderContext, per-doc DocValues) that is not yet
// wired in Gocene; Explain therefore delegates to SortRescorer.Explain until
// that infrastructure is available.
type ExpressionRescorer struct {
	sortRescorer *search.SortRescorer
	expression   *Expression
	bindings     DoubleValuesBindings
}

// NewExpressionRescorer creates an ExpressionRescorer that re-scores using the
// given expression and bindings. The Sort used by the underlying SortRescorer
// is built from expression's sort field with reverse=true (descending score),
// matching Java's constructor: `new Sort(expression.getSortField(bindings, true))`.
//
// sortField must be a pre-built search.SortField that represents the expression
// score (typically SCORE type). Callers construct it once and pass it here.
func NewExpressionRescorer(sortField *search.SortField, expression *Expression, bindings DoubleValuesBindings) *ExpressionRescorer {
	s := search.NewSort(sortField)
	return &ExpressionRescorer{
		sortRescorer: search.NewSortRescorer(s),
		expression:   expression,
		bindings:     bindings,
	}
}

// Rescore re-sorts topDocs using the expression-derived sort order by
// delegating to the embedded SortRescorer.
func (r *ExpressionRescorer) Rescore(searcher *search.IndexSearcher, topDocs *search.TopDocs) (*search.TopDocs, error) {
	return r.sortRescorer.Rescore(searcher, topDocs)
}

// Explain returns an explanation for the rescored document. Delegates to
// SortRescorer.Explain; the per-variable breakdown (available in the Java
// original via ExpressionValueSource.explain) requires per-segment DocValues
// access not yet wired in Gocene.
func (r *ExpressionRescorer) Explain(searcher *search.IndexSearcher, firstPass search.Explanation, docID int) (search.Explanation, error) {
	return r.sortRescorer.Explain(searcher, firstPass, docID)
}

// Expression returns the expression used for rescoring.
func (r *ExpressionRescorer) Expression() *Expression { return r.expression }

// Bindings returns the variable bindings used for rescoring.
func (r *ExpressionRescorer) Bindings() DoubleValuesBindings { return r.bindings }

var _ search.Rescorer = (*ExpressionRescorer)(nil)
