// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package expressions_test

// Port of
// lucene/expressions/src/test/org/apache/lucene/expressions/TestExpressionSorts.java
// (Apache Lucene 10.5.0).
//
// Blockers: setUp() builds its searcher with LuceneTestCase.newSearcher, which
// always wraps an org.apache.lucene.tests.search.AssertingIndexSearcher (not
// ported), and assertQuery(Query, Sort) replaces sort fields by the SortField
// of an Expression bound through
// SimpleBindings.add(String, org.apache.lucene.search.DoubleValuesSource),
// which Gocene's expressions.SimpleBindings does not accept.

import "testing"

func TestExpressionSorts_testQueries(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.search.AssertingIndexSearcher (LuceneTestCase.newSearcher) and " +
		"org.apache.lucene.expressions.SimpleBindings.add(String, org.apache.lucene.search.DoubleValuesSource) " +
		"with the expression-backed SortField of Expression.getSortField (not ported)")
}
