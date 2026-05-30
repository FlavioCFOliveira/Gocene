// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.TestDemoExpressions.
// All tests in this file require a wired IndexSearcher with NumericDocValuesField
// and a fully ANTLR-backed JavascriptCompiler (including all grammar operators
// and math functions). Both are not yet available in Gocene.
// Tests are skipped with diagnostics until those dependencies land.
package expressions_test

import "testing"

// TestDemoExpressions skips because it requires JavascriptCompiler (full ANTLR
// grammar) and IndexSearcher + NumericDocValuesField infrastructure not yet
// present in Gocene.
func TestDemoExpressions(t *testing.T) {
	t.Fatal("requires full ANTLR JavascriptCompiler and IndexSearcher infrastructure (not yet ported)")
}
