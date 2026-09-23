// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestConstantScoreQuery.java
// (Apache Lucene 10.5.0).
//
// This class only tests some basic functionality in CSQ, the main parts are
// mostly tested by MultiTermQuery tests, explanations seems to be tested in
// TestExplanations!

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

func TestConstantScoreQueryCSQ(t *testing.T) {
	q1 := search.NewConstantScoreQuery(search.NewTermQuery(index.NewTerm("a", "b")))
	q2 := search.NewConstantScoreQuery(search.NewTermQuery(index.NewTerm("a", "c")))
	q3 := search.NewConstantScoreQuery(search.NewStringRange("a", "b", "c", true, true))
	queryUtilsCheck(t, q1)
	queryUtilsCheck(t, q2)
	queryUtilsCheckEqual(t, q1, q1)
	queryUtilsCheckEqual(t, q2, q2)
	queryUtilsCheckEqual(t, q3, q3)
	queryUtilsCheckUnequal(t, q1, q2)
	queryUtilsCheckUnequal(t, q2, q3)
	queryUtilsCheckUnequal(t, q1, q3)
	queryUtilsCheckUnequal(t, q1, search.NewTermQuery(index.NewTerm("a", "b")))
}

// csqQueryWrapper renders the private TestConstantScoreQuery.QueryWrapper: a
// query for which other queries don't have special rewrite rules.
type csqQueryWrapper struct {
	in search.Query
}

func (q *csqQueryWrapper) ToString(field string) string { return "MockQuery" }

func (q *csqQueryWrapper) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	return q, nil
}

func (q *csqQueryWrapper) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return q.in.CreateWeight(searcher, scoreMode, boost)
}

func (q *csqQueryWrapper) Visit(visitor search.QueryVisitor) { q.in.Visit(visitor) }

func (q *csqQueryWrapper) Equals(other spi.Query) bool {
	o, ok := other.(*csqQueryWrapper)
	return ok && q.in.Equals(o.in)
}

// csqQueryWrapperClassHash stands for classHash() of QueryWrapper.
const csqQueryWrapperClassHash = 0x51575250

func (q *csqQueryWrapper) HashCode() int { return 31*csqQueryWrapperClassHash + q.in.HashCode() }

func TestConstantScoreQueryConstantScoreQueryAndFilter(t *testing.T) {
	d := newDirectory()
	w := newRandomIndexWriter(t, d)
	doc := newTestDocument(newStringField(t, "field", "a", false))
	mustAddDocument(t, w, doc)
	doc = newTestDocument(newStringField(t, "field", "b", false))
	mustAddDocument(t, w, doc)
	r := mustGetReader(t, w)
	mustClose(t, w)

	filterB := &csqQueryWrapper{in: search.NewTermQuery(index.NewTerm("field", "b"))}
	var query search.Query = search.NewConstantScoreQuery(filterB)

	s := newSearcher(t, r)
	var filtered search.Query = search.NewBooleanQueryBuilder().Add(query, search.MUST).Add(filterB, search.FILTER).Build()
	if got := mustCount(t, s, filtered); got != 1 { // Query for field:b, Filter field:b
		t.Fatalf("expected 1, got %d", got)
	}

	filterA := &csqQueryWrapper{in: search.NewTermQuery(index.NewTerm("field", "a"))}
	query = search.NewConstantScoreQuery(filterA)

	filtered = search.NewBooleanQueryBuilder().Add(query, search.MUST).Add(filterB, search.FILTER).Build()
	if got := mustCount(t, s, filtered); got != 0 { // Query field:b, Filter field:a
		t.Fatalf("expected 0, got %d", got)
	}

	mustClose(t, r, d)
}

func TestConstantScoreQueryPropagatesApproximations(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	f := newTextField(t, "field", "a b", false)
	doc := newTestDocument(f)
	mustAddDocument(t, w, doc)
	if _, err := w.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)
	searcher.SetQueryCache(nil) // to still have approximations

	pq := search.NewPhraseQuery(0, "field", "a", "b")

	q := mustRewrite(t, searcher, search.NewConstantScoreQuery(pq))

	weight, err := searcher.CreateWeight(q, search.COMPLETE, 1)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	scorer, err := weight.Scorer(mustLeaves(t, searcher.GetIndexReader())[0])
	if err != nil {
		t.Fatalf("scorer: %v", err)
	}
	if scorer.TwoPhaseIterator() == nil {
		t.Fatal("expected a two-phase iterator")
	}

	mustClose(t, reader, w, dir)
}

func TestConstantScoreQueryRewriteBubblesUpMatchNoDocsQuery(t *testing.T) {
	multiReader, err := index.NewMultiReader(nil)
	if err != nil {
		t.Fatalf("new MultiReader: %v", err)
	}
	searcher := newSearcher(t, multiReader)

	query := search.NewConstantScoreQuery(search.MatchNoDocsQueryInstance)
	if got := mustRewrite(t, searcher, query); !search.MatchNoDocsQueryInstance.Equals(got) {
		t.Fatalf("expected %v, got %v", search.MatchNoDocsQueryInstance, got)
	}
}
