// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestMaxClauseLimit.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// mclExpectTooManyClauses renders expectThrows(IndexSearcher.TooManyClauses.class, ...):
// TooManyNestedClauses extends TooManyClauses, so both satisfy it.
func mclExpectTooManyClauses(t *testing.T, err error) error {
	t.Helper()
	var tmc *search.TooManyClauses
	var tmnc *search.TooManyNestedClauses
	if !errors.As(err, &tmc) && !errors.As(err, &tmnc) {
		t.Fatalf("expected IndexSearcher.TooManyClauses, got %v", err)
	}
	return err
}

// mclExpectTooManyNestedClauses renders
// expectThrows(IndexSearcher.TooManyNestedClauses.class, ...).
func mclExpectTooManyNestedClauses(t *testing.T, err error) {
	t.Helper()
	var tmnc *search.TooManyNestedClauses
	if !errors.As(err, &tmnc) {
		t.Fatalf("expected IndexSearcher.TooManyNestedClauses, got %v", err)
	}
}

func TestMaxClauseLimitIllegalArgumentExceptionOnZero(t *testing.T) {
	current := search.GetMaxClauseCount()
	expectThrowsPanic(t, func() {
		search.SetMaxClauseCount(0)
	})
	if got := search.GetMaxClauseCount(); got != current {
		t.Fatalf("attempt to change to 0 should have failed w/o modifying: expected %d, got %d", current, got)
	}
}

func TestMaxClauseLimitFlattenInnerDisjunctionsWithMoreThan1024Terms(t *testing.T) {
	searcher := newSearcher(t, newMultiReader(t))

	builder1024 := search.NewBooleanQueryBuilder()
	for i := 0; i < 1024; i++ {
		builder1024.Add(search.NewTermQuery(index.NewTerm("foo", "bar-"+strconv.Itoa(i))), search.SHOULD)
	}
	inner := builder1024.Build()
	query := search.NewBooleanQueryBuilder().
		Add(inner, search.SHOULD).
		Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.SHOULD).
		Build()

	_, err := searcher.Rewrite(query)
	e := mclExpectTooManyClauses(t, err)
	var tmnc *search.TooManyNestedClauses
	if errors.As(e, &tmnc) {
		t.Fatal("Should have been caught during flattening and not required full nested walk")
	}
}

func TestMaxClauseLimitLargeTermsNestedFirst(t *testing.T) {
	searcher := newSearcher(t, newMultiReader(t))
	nestedBuilder := search.NewBooleanQueryBuilder()

	nestedBuilder.SetMinimumNumberShouldMatch(5)
	for i := 0; i < 600; i++ {
		nestedBuilder.Add(search.NewTermQuery(index.NewTerm("foo", "bar-"+strconv.Itoa(i))), search.SHOULD)
	}
	inner := nestedBuilder.Build()
	builderMixed := search.NewBooleanQueryBuilder().Add(inner, search.SHOULD)
	builderMixed.SetMinimumNumberShouldMatch(5)
	for i := 0; i < 600; i++ {
		builderMixed.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
	}
	query := builderMixed.Build()

	// Can't be flattened, but high clause count should still be cause during nested walk...
	_, err := searcher.Rewrite(query)
	mclExpectTooManyNestedClauses(t, err)
}

func TestMaxClauseLimitLargeTermsNestedLast(t *testing.T) {
	searcher := newSearcher(t, newMultiReader(t))
	nestedBuilder := search.NewBooleanQueryBuilder()

	nestedBuilder.SetMinimumNumberShouldMatch(5)
	for i := 0; i < 600; i++ {
		nestedBuilder.Add(search.NewTermQuery(index.NewTerm("foo", "bar-"+strconv.Itoa(i))), search.SHOULD)
	}
	inner := nestedBuilder.Build()
	builderMixed := search.NewBooleanQueryBuilder()
	builderMixed.SetMinimumNumberShouldMatch(5)
	for i := 0; i < 600; i++ {
		builderMixed.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
	}
	builderMixed.Add(inner, search.SHOULD)
	query := builderMixed.Build()

	// Can't be flattened, but high clause count should still be cause during nested walk...
	_, err := searcher.Rewrite(query)
	mclExpectTooManyNestedClauses(t, err)
}

func TestMaxClauseLimitLargeDisjunctionMaxQuery(t *testing.T) {
	searcher := newSearcher(t, newMultiReader(t))
	clausesQueryArray := make([]search.Query, 1050)

	for i := 0; i < 1049; i++ {
		clausesQueryArray[i] = search.NewTermQuery(index.NewTerm("field", "a"))
	}

	pq := search.NewPhraseQuery(0, "field")

	clausesQueryArray[1049] = pq

	dmq := search.NewDisjunctionMaxQuery(clausesQueryArray, 0.5)

	// Can't be flattened, but high clause count should still be cause during nested walk...
	_, err := searcher.Rewrite(dmq)
	mclExpectTooManyNestedClauses(t, err)
}

func TestMaxClauseLimitMultiExactWithRepeats(t *testing.T) {
	searcher := newSearcher(t, newMultiReader(t))
	qb := search.NewMultiPhraseQueryBuilder()

	for i := 0; i < 1050; i++ {
		qb.AddTermsAtPosition([]*index.Term{index.NewTerm("foo", "bar-"+strconv.Itoa(i)), index.NewTerm("foo", "bar+"+strconv.Itoa(i))}, 0)
	}

	// Can't be flattened, but high clause count should still be cause during nested walk...
	_, err := searcher.Rewrite(qb.Build())
	mclExpectTooManyNestedClauses(t, err)
}
