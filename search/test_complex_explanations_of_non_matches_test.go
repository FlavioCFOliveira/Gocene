// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestComplexExplanationsOfNonMatches.java
//
// Subclass of TestComplexExplanations that verifies non-matches: it re-runs
// every TestComplexExplanations scenario but, instead of qtest's matching-doc
// checks, asserts via CheckHits.checkNoMatchExplanations that the explanation
// of every NON-matching document is a non-match.
//
// Java models this by inheriting all of TestComplexExplanations' test methods
// and overriding qtest to call CheckHits.checkNoMatchExplanations. Go has no
// method override, so the same query shapes are rebuilt here and replayed
// through the shared qtest/bqtest helpers with nonMatches enabled; those
// helpers already dispatch to testutil.CheckNoMatchExplanations in that mode
// (see base_explanation_test.go). The similarity is pinned to ClassicSimilarity
// exactly as in TestComplexExplanations.setUp.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// newComplexExplanationNonMatchTestCase reuses the TestComplexExplanations
// corpus (ClassicSimilarity pinned) and switches qtest into the non-matches
// verification mode of TestComplexExplanationsOfNonMatches.
func newComplexExplanationNonMatchTestCase(t *testing.T) *explanationTestCase {
	tc := newComplexExplanationTestCase(t)
	tc.nonMatches = true
	return tc
}

func TestComplexExplanationsOfNonMatches_T3(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	query := search.NewTermQuery(index.NewTerm(explField, "w1"))
	tc.bqtest(search.NewBoostQuery(query, 0), []int{0, 1, 2, 3})
}

func TestComplexExplanationsOfNonMatches_MA3(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	tc.bqtest(search.NewBoostQuery(search.NewMatchAllDocsQuery(), 0), []int{0, 1, 2, 3})
}

func TestComplexExplanationsOfNonMatches_FQ5(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	query := search.NewTermQuery(index.NewTerm(explField, "xx"))
	filtered := search.NewBooleanQuery()
	filtered.Add(search.NewBoostQuery(query, 0), search.MUST)
	filtered.Add(tc.matchTheseItems([]int{1, 3}), search.FILTER)
	tc.bqtest(filtered, []int{3})
}

func TestComplexExplanationsOfNonMatches_CSQ4(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	q := search.NewConstantScoreQuery(tc.matchTheseItems([]int{3}))
	tc.bqtest(search.NewBoostQuery(q, 0), []int{3})
}

func TestComplexExplanationsOfNonMatches_DMQ10(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm(explField, "yy")), search.SHOULD)
	query.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w5")), 100), search.SHOULD)

	xxBoosted := search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "xx")), 0)
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{query, xxBoosted}, 0.5)
	tc.bqtest(search.NewBoostQuery(q, 0), []int{0, 2, 3})
}

func TestComplexExplanationsOfNonMatches_MPQ7(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTerms(explTerms([]string{"w1"}))
	qb.AddTerms(explTerms([]string{"w2"}))
	qb.SetSlop(1)
	tc.bqtest(search.NewBoostQuery(qb.Build(), 0), []int{0, 1, 2})
}

func TestComplexExplanationsOfNonMatches_BQ12(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	// NOTE: using qtest not bqtest
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm(explField, "w1")), search.SHOULD)
	query.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w2")), 0), search.SHOULD)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestComplexExplanationsOfNonMatches_BQ13(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	// NOTE: using qtest not bqtest
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm(explField, "w1")), search.SHOULD)
	query.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w5")), 0), search.MUST_NOT)
	tc.qtest(query, []int{1, 2, 3})
}

func TestComplexExplanationsOfNonMatches_BQ18(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	// NOTE: using qtest not bqtest
	query := search.NewBooleanQuery()
	query.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w1")), 0), search.MUST)
	query.Add(search.NewTermQuery(index.NewTerm(explField, "w2")), search.SHOULD)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestComplexExplanationsOfNonMatches_BQ21(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	builder := search.NewBooleanQuery()
	builder.Add(search.NewTermQuery(index.NewTerm(explField, "w1")), search.MUST)
	builder.Add(search.NewTermQuery(index.NewTerm(explField, "w2")), search.SHOULD)
	tc.bqtest(search.NewBoostQuery(builder, 0), []int{0, 1, 2, 3})
}

func TestComplexExplanationsOfNonMatches_BQ22(t *testing.T) {
	tc := newComplexExplanationNonMatchTestCase(t)
	defer tc.cleanup()
	builder := search.NewBooleanQuery()
	builder.Add(search.NewBoostQuery(search.NewTermQuery(index.NewTerm(explField, "w1")), 0), search.MUST)
	builder.Add(search.NewTermQuery(index.NewTerm(explField, "w2")), search.SHOULD)
	tc.bqtest(search.NewBoostQuery(builder, 0), []int{0, 1, 2, 3})
}
