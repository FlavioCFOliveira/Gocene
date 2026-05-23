// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of relevant portions of
// org.apache.lucene.expressions.TestExpressionRescorer.
// The Java test requires a fully wired IndexSearcher with numeric DocValues;
// those tests are deferred to task 4310-4318 when the ANTLR compiler and
// index infrastructure are connected. The tests here verify the construction
// contract and delegation behaviour of ExpressionRescorer.
package expressions_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestExpressionRescorer_ImplementsRescorer verifies the compile-time interface
// assertion and that construction succeeds with valid inputs.
func TestExpressionRescorer_ImplementsRescorer(t *testing.T) {
	expr := expressions.NewExpressionWithDoubleValues("score", []string{"score"}, func(fvs []expressions.DoubleValues) (float64, error) {
		return fvs[0].DoubleValue()
	})
	bindings := mapBindings{"score": &constantSource{1.0}}
	sf := search.NewSortField("_score", search.SortFieldTypeScore)

	var _ search.Rescorer = expressions.NewExpressionRescorer(sf, expr, bindings)
}

// TestExpressionRescorer_RescoreEmpty verifies that rescoring an empty TopDocs
// returns an empty result without error.
func TestExpressionRescorer_RescoreEmpty(t *testing.T) {
	expr := expressions.NewExpressionWithDoubleValues("s", []string{"s"}, func(fvs []expressions.DoubleValues) (float64, error) {
		return fvs[0].DoubleValue()
	})
	bindings := mapBindings{"s": &constantSource{1.0}}
	sf := search.NewSortField("_score", search.SortFieldTypeScore)
	r := expressions.NewExpressionRescorer(sf, expr, bindings)

	top := &search.TopDocs{ScoreDocs: []*search.ScoreDoc{}}
	got, err := r.Rescore(nil, top)
	if err != nil {
		t.Fatalf("Rescore: %v", err)
	}
	if len(got.ScoreDocs) != 0 {
		t.Errorf("expected 0 results, got %d", len(got.ScoreDocs))
	}
}

// TestExpressionRescorer_RescoreByScore verifies that Rescore orders documents
// by descending score when sort is SCORE type (SortRescorer delegation).
func TestExpressionRescorer_RescoreByScore(t *testing.T) {
	expr := expressions.NewExpressionWithDoubleValues("s", []string{"s"}, func(fvs []expressions.DoubleValues) (float64, error) {
		return fvs[0].DoubleValue()
	})
	bindings := mapBindings{"s": &constantSource{1.0}}
	// Reverse=true on SCORE means descending (highest score first) in SortRescorer.
	sf := search.NewSortFieldReverse("_score", search.SortFieldTypeScore)
	r := expressions.NewExpressionRescorer(sf, expr, bindings)

	top := &search.TopDocs{
		ScoreDocs: []*search.ScoreDoc{
			{Doc: 0, Score: 3.0},
			{Doc: 1, Score: 5.0},
			{Doc: 2, Score: 1.0},
		},
	}
	got, err := r.Rescore(nil, top)
	if err != nil {
		t.Fatalf("Rescore: %v", err)
	}
	if got.ScoreDocs[0].Doc != 1 {
		t.Errorf("expected doc 1 (score 5.0) first, got doc %d", got.ScoreDocs[0].Doc)
	}
}

// TestExpressionRescorer_Accessors verifies Expression and Bindings accessors.
func TestExpressionRescorer_Accessors(t *testing.T) {
	expr := expressions.NewExpressionWithDoubleValues("x", []string{"x"}, func(fvs []expressions.DoubleValues) (float64, error) {
		return 0, nil
	})
	bindings := mapBindings{"x": &constantSource{1.0}}
	sf := search.NewSortField("_score", search.SortFieldTypeScore)
	r := expressions.NewExpressionRescorer(sf, expr, bindings)

	if r.Expression() != expr {
		t.Error("Expression() returned wrong instance")
	}
	if r.Bindings() == nil {
		t.Error("Bindings() returned nil")
	}
}
