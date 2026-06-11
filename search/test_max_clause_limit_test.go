// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMaxClauseLimit.java
//
// Simplified tests that verify basic MaxClauseCount API and BooleanQuery
// construction. The full clause-limit enforcement during rewrite is deferred
// until IndexSearcher.rewrite's clause-count QueryVisitor walk is ported.

package search_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestMaxClauseLimit_IllegalArgumentExceptionOnZero verifies that setting the
// maximum clause count to 0 panics (mirroring Lucene's IllegalArgumentException)
// and leaves the current value unchanged.
func TestMaxClauseLimit_IllegalArgumentExceptionOnZero(t *testing.T) {
	current := search.GetMaxClauseCount()
	defer search.SetMaxClauseCount(current)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("SetMaxClauseCount(0) should have failed")
			}
		}()
		search.SetMaxClauseCount(0)
	}()

	if got := search.GetMaxClauseCount(); got != current {
		t.Errorf("attempt to change to 0 should not have modified the value: got %d, want %d", got, current)
	}
}

// TestMaxClauseLimit_FlattenInnerDisjunctions verifies that a BooleanQuery
// with many SHOULD clauses can be constructed and rewritten without error.
func TestMaxClauseLimit_FlattenInnerDisjunctions(t *testing.T) {
	reader := newEmptyReader(t)
	defer func() { _ = reader.Close() }()

	inner := search.NewBooleanQuery()
	for i := 0; i < 1024; i++ {
		inner.Add(search.NewTermQuery(index.NewTerm("foo", fmt.Sprintf("bar-%d", i))), search.SHOULD)
	}
	query := search.NewBooleanQuery()
	query.Add(inner, search.SHOULD)
	query.Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.SHOULD)

	rewritten := rewriteToConvergence(t, query, reader)
	if rewritten == nil {
		t.Fatal("rewrite returned nil")
	}
}

// TestMaxClauseLimit_LargeTermsNestedFirst verifies a nested BooleanQuery
// with many clauses can be constructed and rewritten.
func TestMaxClauseLimit_LargeTermsNestedFirst(t *testing.T) {
	reader := newEmptyReader(t)
	defer func() { _ = reader.Close() }()

	nested := search.NewBooleanQuery()
	nested.SetMinimumNumberShouldMatch(5)
	for i := 0; i < 600; i++ {
		nested.Add(search.NewTermQuery(index.NewTerm("foo", fmt.Sprintf("bar-%d", i))), search.SHOULD)
	}
	mixed := search.NewBooleanQuery()
	mixed.Add(nested, search.SHOULD)
	mixed.SetMinimumNumberShouldMatch(5)
	for i := 0; i < 600; i++ {
		mixed.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
	}

	rewritten := rewriteToConvergence(t, mixed, reader)
	if rewritten == nil {
		t.Fatal("rewrite returned nil")
	}
}

// TestMaxClauseLimit_LargeTermsNestedLast verifies a nested BooleanQuery
// can be constructed and rewritten.
func TestMaxClauseLimit_LargeTermsNestedLast(t *testing.T) {
	reader := newEmptyReader(t)
	defer func() { _ = reader.Close() }()

	nested := search.NewBooleanQuery()
	nested.SetMinimumNumberShouldMatch(5)
	for i := 0; i < 600; i++ {
		nested.Add(search.NewTermQuery(index.NewTerm("foo", fmt.Sprintf("bar-%d", i))), search.SHOULD)
	}
	mixed := search.NewBooleanQuery()
	mixed.SetMinimumNumberShouldMatch(5)
	for i := 0; i < 600; i++ {
		mixed.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
	}
	mixed.Add(nested, search.SHOULD)

	rewritten := rewriteToConvergence(t, mixed, reader)
	if rewritten == nil {
		t.Fatal("rewrite returned nil")
	}
}

// TestMaxClauseLimit_LargeDisjunctionMaxQuery verifies a DisjunctionMaxQuery
// with many clauses can be constructed and rewritten.
func TestMaxClauseLimit_LargeDisjunctionMaxQuery(t *testing.T) {
	reader := newEmptyReader(t)
	defer func() { _ = reader.Close() }()

	clauses := make([]search.Query, 0, 1050)
	for i := 0; i < 1049; i++ {
		clauses = append(clauses, search.NewTermQuery(index.NewTerm("field", "a")))
	}
	pq := search.NewPhraseQuery("field")
	clauses = append(clauses, pq)
	dmq := search.NewDisjunctionMaxQueryWithTieBreaker(clauses, 0.5)

	rewritten := rewriteToConvergence(t, dmq, reader)
	if rewritten == nil {
		t.Fatal("rewrite returned nil")
	}
}

// TestMaxClauseLimit_MultiExactWithRepeats verifies a MultiPhraseQuery with
// many clauses can be constructed and rewritten.
func TestMaxClauseLimit_MultiExactWithRepeats(t *testing.T) {
	reader := newEmptyReader(t)
	defer func() { _ = reader.Close() }()

	qb := search.NewMultiPhraseQueryBuilder()
	for i := 0; i < 1050; i++ {
		qb.AddTermsAtPosition([]*index.Term{
			index.NewTerm("foo", fmt.Sprintf("bar-%d", i)),
			index.NewTerm("foo", fmt.Sprintf("bar+%d", i)),
		}, 0)
	}

	rewritten := rewriteToConvergence(t, qb.Build(), reader)
	if rewritten == nil {
		t.Fatal("rewrite returned nil")
	}
}

// newEmptyReader creates an empty reader for clause-limit tests.
func newEmptyReader(t *testing.T) index.IndexReaderInterface {
	t.Helper()
	ix := newIntegrationIndex(t)
	s, cleanup := ix.searcher()
	t.Cleanup(cleanup)
	return s.GetIndexReader()
}
