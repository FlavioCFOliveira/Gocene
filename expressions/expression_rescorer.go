// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package expressions

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ExpressionRescorer is a rescorer that uses an expression to compute scores.
//
// This is the Go port of Lucene's org.apache.lucene.expressions.ExpressionRescorer.
type ExpressionRescorer struct {
	search.SortRescorer
	expression Expression
	bindings   Bindings
}

// NewExpressionRescorer constructs an ExpressionRescorer.
// It mirrors the Java constructor: super(new Sort(expression.getSortField(bindings, true))).
func NewExpressionRescorer(expression Expression, bindings Bindings) *ExpressionRescorer {
	// In Gocene, we need to translate the expression's sort field to a search.SortField.
	// Assuming expression.getSortField is implemented as GetSortField.
	sortField := expression.GetSortField(bindings, true)
	sort := search.NewSort([]*search.SortField{sortField})

	return &ExpressionRescorer{
		SortRescorer: search.NewSortRescorer(sort),
		expression:   expression,
		bindings:     bindings,
	}
}

// Rescore re-scores the top documents. Since it embeds SortRescorer,
// it uses SortRescorer.Rescore by default.
func (er *ExpressionRescorer) Rescore(searcher *search.IndexSearcher, topDocs *search.TopDocs) (*search.TopDocs, error) {
	return er.SortRescorer.Rescore(searcher, topDocs)
}

// Explain provides a detailed explanation of the rescoring step, including
// the value of each variable in the expression.
func (er *ExpressionRescorer) Explain(searcher *search.IndexSearcher, firstPass search.Explanation, docID int) (search.Explanation, error) {
	superExpl, err := er.SortRescorer.Explain(searcher, firstPass, docID)
	if err != nil {
		return nil, err
	}

	leaves := searcher.GetIndexReader().Leaves()
	subReader := search.ReaderUtilSubIndex(docID, leaves)
	readerContext := leaves[subReader]
	docIDInSegment := docID - readerContext.DocBase

	// Delegate to the double value source for detailed variable explanation.
	return er.expression.GetDoubleValuesSource(er.bindings).Explain(readerContext, docIDInSegment, superExpl)
}
