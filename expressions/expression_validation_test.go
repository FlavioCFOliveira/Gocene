// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.TestExpressionValidation.
// Tests require JavascriptCompiler.compile (full ANTLR grammar) and
// SimpleBindings.validate (not yet implemented in Gocene). Deferred.
package expressions_test

import "testing"

// TestExpressionValidation skips because it requires a validate() method on
// SimpleBindings and JavascriptCompiler.compile with the full ANTLR grammar.
func TestExpressionValidation(t *testing.T) {
	t.Fatal("requires SimpleBindings.validate and full ANTLR JavascriptCompiler (not yet ported)")
}
