// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.TestExpressionSorts.
package expressions_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions/js"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestExpressionSorts verifies that SortFields derived from compiled expressions
// integrate correctly with the search.Sort type.
func TestExpressionSorts(t *testing.T) {
	// Create two expressions with different variables
	expr1, err := js.JavascriptCompiler{}.Compile("a * 2")
	if err != nil {
		t.Fatal(err)
	}
	expr2, err := js.JavascriptCompiler{}.Compile("b + 10")
	if err != nil {
		t.Fatal(err)
	}

	// Create bindings covering both variables
	bindings := newTestBindings()
	bindings.Add("a", &testDoubleValuesSource{value: 3})
	bindings.Add("b", &testDoubleValuesSource{value: 7})

	// Get SortFields: non-reversed for expr1, reversed for expr2
	sf1, err := expr1.GetSortField(bindings, false)
	if err != nil {
		t.Fatal(err)
	}
	sf2, err := expr2.GetSortField(bindings, true)
	if err != nil {
		t.Fatal(err)
	}

	// Assemble into a Sort
	sortObj := search.NewSort(sf1, sf2)
	if sortObj == nil {
		t.Fatal("NewSort returned nil")
	}
	if len(sortObj.Fields) != 2 {
		t.Errorf("Sort has %d fields, want 2", len(sortObj.Fields))
	}

	// Verify field-level properties
	if sortObj.Fields[0].GetField() != "a * 2" {
		t.Errorf("Sort field[0] name = %q, want %q", sortObj.Fields[0].GetField(), "a * 2")
	}
	if sortObj.Fields[0].GetReverse() {
		t.Error("Sort field[0] should not be reversed")
	}
	if sortObj.Fields[1].GetField() != "b + 10" {
		t.Errorf("Sort field[1] name = %q, want %q", sortObj.Fields[1].GetField(), "b + 10")
	}
	if !sortObj.Fields[1].GetReverse() {
		t.Error("Sort field[1] should be reversed")
	}

	// Score-type sort fields should cause NeedsScores() to return true
	if !sortObj.NeedsScores() {
		t.Error("Sort built from Score-type SortFields should need scores")
	}
}

// TestExpressionSorts_Builtin verifies that the built-in Sort constructors
// (SortByScore, SortByDoc) work correctly alongside expression-derived sorts.
func TestExpressionSorts_Builtin(t *testing.T) {
	// Built-in sorts
	sortByScore := search.NewSortByScore()
	if len(sortByScore.Fields) != 1 {
		t.Errorf("SortByScore has %d fields, want 1", len(sortByScore.Fields))
	}
	if !sortByScore.NeedsScores() {
		t.Error("SortByScore should need scores")
	}

	sortByDoc := search.NewSortByDoc()
	if len(sortByDoc.Fields) != 1 {
		t.Errorf("SortByDoc has %d fields, want 1", len(sortByDoc.Fields))
	}
	if sortByDoc.NeedsScores() {
		t.Error("SortByDoc should not need scores")
	}

	// Mixed sort: expression-derived + built-in
	expr, err := js.JavascriptCompiler{}.Compile("score + 1")
	if err != nil {
		t.Fatal(err)
	}
	bindings := newTestBindings()
	bindings.Add("score", &testDoubleValuesSource{value: 0.5})

	sf, err := expr.GetSortField(bindings, true)
	if err != nil {
		t.Fatal(err)
	}

	combined := search.NewSort(sf, sortByDoc.Fields[0])
	if len(combined.Fields) != 2 {
		t.Errorf("combined Sort has %d fields, want 2", len(combined.Fields))
	}
}

// TestExpressionSorts_Reversed verifies that creating Sort with non-reversed
// expression-derived SortField works correctly.
func TestExpressionSorts_NotReversed(t *testing.T) {
	expr, err := js.JavascriptCompiler{}.Compile("popularity")
	if err != nil {
		t.Fatal(err)
	}
	bindings := newTestBindings()
	bindings.Add("popularity", &testDoubleValuesSource{value: 100})

	sf, err := expr.GetSortField(bindings, true)
	if err != nil {
		t.Fatal(err)
	}

	sortObj := search.NewSort(sf)
	if !sortObj.NeedsScores() {
		t.Error("expression-derived SortField should need scores")
	}
	if !sortObj.Fields[0].GetReverse() {
		t.Error("GetSortField(..., true) should produce reversed SortField")
	}
}
