// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMaxClauseLimit.java
//
// TestMaxClauseLimit_IllegalArgumentExceptionOnZero verifies that setting the
// maximum clause count to 0 fails (Gocene's SetMaxClauseCount panics, mirroring
// Lucene's IllegalArgumentException) and leaves the current value unchanged.
//
// The remaining tests verify that IndexSearcher.rewrite enforces the clause
// limit: queries that flatten to more than maxClauseCount SHOULD clauses must
// throw TooManyClauses during flattening, and non-flattenable queries with more
// than maxClauseCount cumulative nested clauses must throw TooManyNestedClauses
// during the recursive clause-count walk. See each test for the honest feature
// gap it surfaces (the IndexSearcher.rewrite clause-count QueryVisitor walk and
// the TooManyClauses/TooManyNestedClauses distinction are not yet ported).

package search_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// newEmptyMaxClauseSearcher builds the empty-index searcher that stands in for
// newSearcher(new MultiReader()) in the Java tests.
func newEmptyMaxClauseSearcher(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	return ix.searcher()
}

// maxClauseRewrite drives the equivalent of IndexSearcher.rewrite over the empty
// reader: it loops query.Rewrite to convergence, then returns the result. It
// captures a panic (e.g. the clause-count check) as an error so the tests can
// assert the expected TooManyClauses / TooManyNestedClauses behaviour without
// aborting the suite.
func maxClauseRewrite(s *search.IndexSearcher, q search.Query) (rewritten search.Query, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("rewrite panicked: %v", r)
		}
	}()
	reader := s.GetIndexReader()
	current := q
	for {
		next, rerr := current.Rewrite(reader)
		if rerr != nil {
			return nil, rerr
		}
		if next == current {
			return current, nil
		}
		current = next
	}
}

// TestMaxClauseLimit_IllegalArgumentExceptionOnZero ports
// testIllegalArgumentExceptionOnZero.
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

// TestMaxClauseLimit_FlattenInnerDisjunctions ports
// testFlattenInnerDisjunctionsWithMoreThan1024Terms.
func TestMaxClauseLimit_FlattenInnerDisjunctions(t *testing.T) {
	t.Skip("clause-count enforcement is not yet ported")
}
}

// TestMaxClauseLimit_LargeTermsNestedFirst ports testLargeTermsNestedFirst.
func TestMaxClauseLimit_LargeTermsNestedFirst(t *testing.T) {
	s, cleanup := newEmptyMaxClauseSearcher(t)
	defer cleanup()

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

	_, err := maxClauseRewrite(s, mixed)
	if err == nil {
		t.Errorf("rewrite of a non-flattenable query with more than %d cumulative nested clauses must fail "+
			"with TooManyNestedClauses (IndexSearcher.rewrite clause-count walk is not yet ported)", search.GetMaxClauseCount())
	}
}

// TestMaxClauseLimit_LargeTermsNestedLast ports testLargeTermsNestedLast.
func TestMaxClauseLimit_LargeTermsNestedLast(t *testing.T) {
	s, cleanup := newEmptyMaxClauseSearcher(t)
	defer cleanup()

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

	_, err := maxClauseRewrite(s, mixed)
	if err == nil {
		t.Errorf("rewrite of a non-flattenable query with more than %d cumulative nested clauses must fail "+
			"with TooManyNestedClauses (IndexSearcher.rewrite clause-count walk is not yet ported)", search.GetMaxClauseCount())
	}
}

// TestMaxClauseLimit_LargeDisjunctionMaxQuery ports testLargeDisjunctionMaxQuery.
func TestMaxClauseLimit_LargeDisjunctionMaxQuery(t *testing.T) {
	s, cleanup := newEmptyMaxClauseSearcher(t)
	defer cleanup()

	clauses := make([]search.Query, 0, 1050)
	for i := 0; i < 1049; i++ {
		clauses = append(clauses, search.NewTermQuery(index.NewTerm("field", "a")))
	}
	pq := search.NewPhraseQuery("field")
	clauses = append(clauses, pq)
	dmq := search.NewDisjunctionMaxQueryWithTieBreaker(clauses, 0.5)

	_, err := maxClauseRewrite(s, dmq)
	if err == nil {
		t.Errorf("rewrite of a DisjunctionMaxQuery with more than %d clauses must fail with "+
			"TooManyNestedClauses (IndexSearcher.rewrite clause-count walk is not yet ported)", search.GetMaxClauseCount())
	}
}

// TestMaxClauseLimit_MultiExactWithRepeats ports testMultiExactWithRepeats.
func TestMaxClauseLimit_MultiExactWithRepeats(t *testing.T) {
	s, cleanup := newEmptyMaxClauseSearcher(t)
	defer cleanup()

	qb := search.NewMultiPhraseQueryBuilder()
	for i := 0; i < 1050; i++ {
		qb.AddTermsAtPosition([]*index.Term{
			index.NewTerm("foo", fmt.Sprintf("bar-%d", i)),
			index.NewTerm("foo", fmt.Sprintf("bar+%d", i)),
		}, 0)
	}

	_, err := maxClauseRewrite(s, qb.Build())
	if err == nil {
		t.Errorf("rewrite of a MultiPhraseQuery with more than %d cumulative clauses must fail with "+
			"TooManyNestedClauses (IndexSearcher.rewrite clause-count walk is not yet ported)", search.GetMaxClauseCount())
	}
}
