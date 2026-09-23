// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package expressions_test

// Port of
// lucene/expressions/src/test/org/apache/lucene/expressions/TestExpressionValidation.java
// (Apache Lucene 10.5.0).
//
// Blocker: every test method adds DoubleValuesSource and Expression bindings
// to an org.apache.lucene.expressions.SimpleBindings and calls
// SimpleBindings.validate(). Gocene's expressions.SimpleBindings takes a
// Gocene-only ValueSource and has no validate(), so each test compiles the
// expressions it binds and then fails naming the missing Lucene members.

import "testing"

const expressionValidationBlocker = "requires org.apache.lucene.expressions.SimpleBindings.add(String, " +
	"org.apache.lucene.search.DoubleValuesSource / Expression) and SimpleBindings.validate() (not ported)"

func TestExpressionValidation_testValidExternals(t *testing.T) {
	mustCompileExpression(t, "valid0 - valid1 + valid2 + _score")
	mustCompileExpression(t, "valide0 + valid0")
	mustCompileExpression(t, "valide0 * valide1")
	t.Fatal(expressionValidationBlocker)
}

func TestExpressionValidation_testInvalidExternal(t *testing.T) {
	mustCompileExpression(t, "badreference")
	t.Fatal(expressionValidationBlocker)
}

func TestExpressionValidation_testInvalidExternal2(t *testing.T) {
	mustCompileExpression(t, "valid + badreference")
	t.Fatal(expressionValidationBlocker)
}

func TestExpressionValidation_testSelfRecursion(t *testing.T) {
	mustCompileExpression(t, "cycle0")
	t.Fatal(expressionValidationBlocker)
}

func TestExpressionValidation_testCoRecursion(t *testing.T) {
	mustCompileExpression(t, "cycle1")
	mustCompileExpression(t, "cycle0")
	t.Fatal(expressionValidationBlocker)
}

func TestExpressionValidation_testCoRecursion2(t *testing.T) {
	mustCompileExpression(t, "cycle1")
	mustCompileExpression(t, "cycle2")
	mustCompileExpression(t, "cycle0")
	t.Fatal(expressionValidationBlocker)
}

func TestExpressionValidation_testCoRecursion3(t *testing.T) {
	mustCompileExpression(t, "100")
	mustCompileExpression(t, "cycle0 + cycle2")
	mustCompileExpression(t, "cycle0 + cycle1")
	t.Fatal(expressionValidationBlocker)
}

func TestExpressionValidation_testCoRecursion4(t *testing.T) {
	mustCompileExpression(t, "100")
	mustCompileExpression(t, "cycle1 + cycle0 + cycle3")
	mustCompileExpression(t, "cycle0 + cycle1 + cycle2")
	t.Fatal(expressionValidationBlocker)
}
