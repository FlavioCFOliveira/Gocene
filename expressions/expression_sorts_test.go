// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.TestExpressionSorts.
// Tests require JavascriptCompiler.compile (full ANTLR grammar) and a
// full IndexSearcher with NumericDocValuesField. Deferred.
package expressions_test

import "testing"

// TestExpressionSorts skips because it requires full ANTLR JavascriptCompiler
// and IndexSearcher infrastructure not yet in Gocene.
func TestExpressionSorts(t *testing.T) {
	t.Fatal("requires full ANTLR JavascriptCompiler and IndexSearcher infrastructure (not yet ported)")
}
