// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package expressions_test

// Port of
// lucene/expressions/src/test/org/apache/lucene/expressions/TestExpressionSortField.java
// (Apache Lucene 10.5.0).
//
// Blocker: every test method binds variables with
// SimpleBindings.add(String, org.apache.lucene.search.DoubleValuesSource) or
// SimpleBindings.add(String, Expression) and reads the SortField built by
// Expression.getSortField over those bindings. Gocene's
// expressions.SimpleBindings takes a Gocene-only ValueSource and
// Expression.GetSortField returns a plain SCORE sort field, so each test
// compiles its expressions and then fails naming the missing Lucene classes.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions/js"
)

const expressionSortFieldBlocker = "requires org.apache.lucene.expressions.SimpleBindings.add(String, " +
	"org.apache.lucene.search.DoubleValuesSource / Expression) and the expression-backed " +
	"SortField of Expression.getSortField (not ported)"

func mustCompileExpression(t *testing.T, source string) {
	t.Helper()
	if _, err := (js.JavascriptCompiler{}).Compile(source); err != nil {
		t.Fatalf("compile %q: %v", source, err)
	}
}

func TestExpressionSortField_testToString(t *testing.T) {
	mustCompileExpression(t, "sqrt(_score) + ln(popularity)")
	t.Fatal(expressionSortFieldBlocker)
}

func TestExpressionSortField_testEquals(t *testing.T) {
	mustCompileExpression(t, "sqrt(_score) + ln(popularity)")
	t.Fatal(expressionSortFieldBlocker)
}

func TestExpressionSortField_testNeedsScores(t *testing.T) {
	for _, source := range []string{
		"_score", "0", "intfield", "_score + 0", "intfield + 0", "a + 0", "e + 0",
		"b / c + e * g - sqrt(f)", "b / c + e * g",
	} {
		mustCompileExpression(t, source)
	}
	t.Fatal(expressionSortFieldBlocker)
}
