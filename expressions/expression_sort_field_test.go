// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.TestExpressionSortField.
// Tests require JavascriptCompiler.compile (full ANTLR grammar) and
// IndexSearcher / NumericDocValuesField. Deferred.
package expressions_test

import "testing"

// TestExpressionSortField skips because it requires full ANTLR
// JavascriptCompiler and IndexSearcher infrastructure not yet in Gocene.
func TestExpressionSortField(t *testing.T) {
	t.Skip("requires full ANTLR JavascriptCompiler and IndexSearcher infrastructure (not yet ported)")
}
