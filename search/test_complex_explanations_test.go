// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestComplexExplanations.java
//
// TestExplanations subclass that builds up complex queries on the assumption
// that if the explanations work out for them they work for anything. These
// cases specifically exercise queries that match with scores of 0 wrapped in
// other queries (boost 0).
//
// Like the Java original, every test pins the similarity to ClassicSimilarity
// (already Gocene's default) before running, and uses the BaseExplanationTestCase
// corpus and qtest/bqtest helpers.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// newComplexExplanationTestCase builds the explanation corpus and pins the
// similarity to ClassicSimilarity, mirroring TestComplexExplanations.setUp.
func newComplexExplanationTestCase(t *testing.T) *explanationTestCase {
	tc := newExplanationTestCase(t)
	tc.searcher.SetSimilarity(search.NewClassicSimilarity())
	return tc
}

func TestComplexExplanations_T3(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewTermQuery(index.NewTerm(explField, "w1"))
	tc.bqtest(search.NewBoostQuery(query, 0), []int{0, 1, 2, 3})
}

func TestComplexExplanations_MA3(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	tc.bqtest(search.NewBoostQuery(search.NewMatchAllDocsQuery(), 0), []int{0, 1, 2, 3})
}

func TestComplexExplanations_FQ5(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewTermQuery(index.NewTerm(explField, "xx"))
	filtered := search.NewBooleanQuery()
	filtered.Add(search.NewBoostQuery(query, 0), search.MUST)
	filtered.Add(tc.matchTheseItems([]int{1, 3}), search.FILTER)
	tc.bqtest(filtered, []int{3})
}

func TestComplexExplanations_CSQ4(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewConstantScoreQuery(tc.matchTheseItems([]int{3}))
	tc.bqtest(search.NewBoostQuery(q, 0), []int{3})
}

func TestComplexExplanations_DMQ10(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm(explField, "yy")), search.SHOULD)
	query.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w5")), 100), search.SHOULD)

	xxBoosted := search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "xx")), 0)
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{query, xxBoosted}, 0.5)
	tc.bqtest(search.NewBoostQuery(q, 0), []int{0, 2, 3})
}

func TestComplexExplanations_MPQ7(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTerms(explTerms([]string{"w1"}))
	qb.AddTerms(explTerms([]string{"w2"}))
	qb.SetSlop(1)
	tc.bqtest(search.NewBoostQuery(qb.Build(), 0), []int{0, 1, 2})
}

func TestComplexExplanations_BQ12(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	// NOTE: using qtest not bqtest
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm(explField, "w1")), search.SHOULD)
	query.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w2")), 0), search.SHOULD)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestComplexExplanations_BQ13(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	// NOTE: using qtest not bqtest
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm(explField, "w1")), search.SHOULD)
	query.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w5")), 0), search.MUST_NOT)
	tc.qtest(query, []int{1, 2, 3})
}

func TestComplexExplanations_BQ18(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	// NOTE: using qtest not bqtest
	query := search.NewBooleanQuery()
	query.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w1")), 0), search.MUST)
	query.Add(search.NewTermQuery(index.NewTerm(explField, "w2")), search.SHOULD)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestComplexExplanations_BQ21(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	builder := search.NewBooleanQuery()
	builder.Add(search.NewTermQuery(index.NewTerm(explField, "w1")), search.MUST)
	builder.Add(search.NewTermQuery(index.NewTerm(explField, "w2")), search.SHOULD)
	tc.bqtest(search.NewBoostQuery(builder, 0), []int{0, 1, 2, 3})
}

func TestComplexExplanations_BQ22(t *testing.T) {
	tc := newComplexExplanationTestCase(t)
	defer tc.cleanup()
	builder := search.NewBooleanQuery()
	builder.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w1")), 0), search.MUST)
	builder.Add(search.NewTermQuery(index.NewTerm(explField, "w2")), search.SHOULD)
	tc.bqtest(search.NewBoostQuery(builder, 0), []int{0, 1, 2, 3})
}
