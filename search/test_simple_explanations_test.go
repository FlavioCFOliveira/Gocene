// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSimpleExplanations.java
//
// TestExplanations subclass focusing on basic query types. We focus on queries
// that don't rewrite to other queries; if those are covered well, the queries
// that rewrite to primitives are covered too.
//
// Each test builds the BaseExplanationTestCase corpus (newExplanationTestCase)
// and runs qtest against it, asserting both the matching doc set and the
// per-document explanation tree, faithfully matching the Java assertions.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// simpleTerm is a TermQuery on FIELD for word.
func simpleTerm(word string) *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(explField, word))
}

// simpleAltTerm is a TermQuery on ALTFIELD for word.
func simpleAltTerm(word string) *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(explAltField, word))
}

// phraseSlop builds a PhraseQuery with the given slop over field's words.
func phraseSlop(slop int, field string, words ...string) *search.PhraseQuery {
	terms := make([]*index.Term, len(words))
	for i, w := range words {
		terms[i] = index.NewTerm(field, w)
	}
	return search.NewPhraseQueryWithSlop(slop, field, terms...)
}

/* simple term tests */

func TestSimpleExplanations_T1(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(simpleTerm("w1"), []int{0, 1, 2, 3})
}

func TestSimpleExplanations_T2(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(search.NewBoostQuery(simpleTerm("w1"), 100), []int{0, 1, 2, 3})
}

/* MatchAllDocs */

func TestSimpleExplanations_MA1(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(search.NewMatchAllDocsQuery(), []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MA2(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(search.NewBoostQuery(search.NewMatchAllDocsQuery(), 1000), []int{0, 1, 2, 3})
}

/* some simple phrase tests */

func TestSimpleExplanations_P1(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(search.NewPhraseQueryWithStrings(explField, "w1", "w2"), []int{0})
}

func TestSimpleExplanations_P2(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(search.NewPhraseQueryWithStrings(explField, "w1", "w3"), []int{1, 3})
}

func TestSimpleExplanations_P3(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(phraseSlop(1, explField, "w1", "w2"), []int{0, 1, 2})
}

func TestSimpleExplanations_P4(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(phraseSlop(1, explField, "w2", "w3"), []int{0, 1, 2, 3})
}

func TestSimpleExplanations_P5(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(phraseSlop(1, explField, "w3", "w2"), []int{1, 3})
}

func TestSimpleExplanations_P6(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(phraseSlop(2, explField, "w3", "w2"), []int{0, 1, 3})
}

func TestSimpleExplanations_P7(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(phraseSlop(3, explField, "w3", "w2"), []int{0, 1, 2, 3})
}

/* ConstantScoreQueries */

func TestSimpleExplanations_CSQ1(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewConstantScoreQuery(tc.matchTheseItems([]int{0, 1, 2, 3}))
	tc.qtest(q, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_CSQ2(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewConstantScoreQuery(tc.matchTheseItems([]int{1, 3}))
	tc.qtest(q, []int{1, 3})
}

func TestSimpleExplanations_CSQ3(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewConstantScoreQuery(tc.matchTheseItems([]int{0, 2}))
	tc.qtest(search.NewBoostQuery(q, 1000), []int{0, 2})
}

/* DisjunctionMaxQuery */

func TestSimpleExplanations_DMQ1(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{simpleTerm("w1"), simpleTerm("w5")}, 0.0)
	tc.qtest(q, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_DMQ2(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{simpleTerm("w1"), simpleTerm("w5")}, 0.5)
	tc.qtest(q, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_DMQ3(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{simpleTerm("QQ"), simpleTerm("w5")}, 0.5)
	tc.qtest(q, []int{0})
}

func TestSimpleExplanations_DMQ4(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{simpleTerm("QQ"), simpleTerm("xx")}, 0.5)
	tc.qtest(q, []int{2, 3})
}

func TestSimpleExplanations_DMQ5(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	bq := search.NewBooleanQuery()
	bq.Add(simpleTerm("yy"), search.SHOULD)
	bq.Add(simpleTerm("QQ"), search.MUST_NOT)
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{bq, simpleTerm("xx")}, 0.5)
	tc.qtest(q, []int{2, 3})
}

func TestSimpleExplanations_DMQ6(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	bq := search.NewBooleanQuery()
	bq.Add(simpleTerm("yy"), search.MUST_NOT)
	bq.Add(simpleTerm("w3"), search.SHOULD)
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{bq, simpleTerm("xx")}, 0.5)
	tc.qtest(q, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_DMQ7(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	bq := search.NewBooleanQuery()
	bq.Add(simpleTerm("yy"), search.MUST_NOT)
	bq.Add(simpleTerm("w3"), search.SHOULD)
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{bq, simpleTerm("w2")}, 0.5)
	tc.qtest(q, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_DMQ8(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	bq := search.NewBooleanQuery()
	bq.Add(simpleTerm("yy"), search.SHOULD)
	bq.Add(search.NewBoostQuery(simpleTerm("w5"), 100), search.SHOULD)
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{bq, search.NewBoostQuery(simpleTerm("xx"), 100000)}, 0.5)
	tc.qtest(q, []int{0, 2, 3})
}

func TestSimpleExplanations_DMQ9(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	bq := search.NewBooleanQuery()
	bq.Add(simpleTerm("yy"), search.SHOULD)
	bq.Add(search.NewBoostQuery(simpleTerm("w5"), 100), search.SHOULD)
	q := search.NewDisjunctionMaxQueryWithTieBreaker(
		[]search.Query{bq, search.NewBoostQuery(simpleTerm("xx"), 0)}, 0.5)
	tc.qtest(q, []int{0, 2, 3})
}

/* MultiPhraseQuery */

func TestSimpleExplanations_MPQ1(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTerms(explTerms([]string{"w1"}))
	qb.AddTerms(explTerms([]string{"w2", "w3", "xx"}))
	tc.qtest(qb.Build(), []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MPQ2(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTerms(explTerms([]string{"w1"}))
	qb.AddTerms(explTerms([]string{"w2", "w3"}))
	tc.qtest(qb.Build(), []int{0, 1, 3})
}

func TestSimpleExplanations_MPQ3(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTerms(explTerms([]string{"w1", "xx"}))
	qb.AddTerms(explTerms([]string{"w2", "w3"}))
	tc.qtest(qb.Build(), []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MPQ4(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTerms(explTerms([]string{"w1"}))
	qb.AddTerms(explTerms([]string{"w2"}))
	tc.qtest(qb.Build(), []int{0})
}

func TestSimpleExplanations_MPQ5(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTerms(explTerms([]string{"w1"}))
	qb.AddTerms(explTerms([]string{"w2"}))
	qb.SetSlop(1)
	tc.qtest(qb.Build(), []int{0, 1, 2})
}

func TestSimpleExplanations_MPQ6(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTerms(explTerms([]string{"w1", "w3"}))
	qb.AddTerms(explTerms([]string{"w2"}))
	qb.SetSlop(1)
	tc.qtest(qb.Build(), []int{0, 1, 2, 3})
}

/* some simple tests of boolean queries containing term queries */

func TestSimpleExplanations_BQ1(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("w1"), search.MUST)
	query.Add(simpleTerm("w2"), search.MUST)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ2(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("yy"), search.MUST)
	query.Add(simpleTerm("w3"), search.MUST)
	tc.qtest(query, []int{2, 3})
}

func TestSimpleExplanations_BQ3(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("yy"), search.SHOULD)
	query.Add(simpleTerm("w3"), search.MUST)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ4(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.SHOULD)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleTerm("xx"), search.MUST_NOT)
	innerQuery.Add(simpleTerm("w2"), search.SHOULD)
	outerQuery.Add(innerQuery, search.SHOULD)
	tc.qtest(outerQuery, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ5(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.SHOULD)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleTerm("qq"), search.MUST)
	innerQuery.Add(simpleTerm("w2"), search.SHOULD)
	outerQuery.Add(innerQuery, search.SHOULD)
	tc.qtest(outerQuery, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ6(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.SHOULD)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleTerm("qq"), search.MUST_NOT)
	innerQuery.Add(simpleTerm("w5"), search.SHOULD)
	outerQuery.Add(innerQuery, search.MUST_NOT)
	tc.qtest(outerQuery, []int{1, 2, 3})
}

func TestSimpleExplanations_BQ7(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.MUST)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleTerm("qq"), search.SHOULD)
	childLeft := search.NewBooleanQuery()
	childLeft.Add(simpleTerm("xx"), search.SHOULD)
	childLeft.Add(simpleTerm("w2"), search.MUST_NOT)
	innerQuery.Add(childLeft, search.SHOULD)
	childRight := search.NewBooleanQuery()
	childRight.Add(simpleTerm("w3"), search.MUST)
	childRight.Add(simpleTerm("w4"), search.MUST)
	innerQuery.Add(childRight, search.SHOULD)
	outerQuery.Add(innerQuery, search.MUST)
	tc.qtest(outerQuery, []int{0})
}

func TestSimpleExplanations_BQ8(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.MUST)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleTerm("qq"), search.SHOULD)
	childLeft := search.NewBooleanQuery()
	childLeft.Add(simpleTerm("xx"), search.SHOULD)
	childLeft.Add(simpleTerm("w2"), search.MUST_NOT)
	innerQuery.Add(childLeft, search.SHOULD)
	childRight := search.NewBooleanQuery()
	childRight.Add(simpleTerm("w3"), search.MUST)
	childRight.Add(simpleTerm("w4"), search.MUST)
	innerQuery.Add(childRight, search.SHOULD)
	outerQuery.Add(innerQuery, search.SHOULD)
	tc.qtest(outerQuery, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ9(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.MUST)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleTerm("qq"), search.SHOULD)
	childLeft := search.NewBooleanQuery()
	childLeft.Add(simpleTerm("xx"), search.MUST_NOT)
	childLeft.Add(simpleTerm("w2"), search.SHOULD)
	innerQuery.Add(childLeft, search.SHOULD)
	childRight := search.NewBooleanQuery()
	childRight.Add(simpleTerm("w3"), search.MUST)
	childRight.Add(simpleTerm("w4"), search.MUST)
	innerQuery.Add(childRight, search.MUST_NOT)
	outerQuery.Add(innerQuery, search.SHOULD)
	tc.qtest(outerQuery, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ10(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.MUST)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleTerm("qq"), search.SHOULD)
	childLeft := search.NewBooleanQuery()
	childLeft.Add(simpleTerm("xx"), search.MUST_NOT)
	childLeft.Add(simpleTerm("w2"), search.SHOULD)
	innerQuery.Add(childLeft, search.SHOULD)
	childRight := search.NewBooleanQuery()
	childRight.Add(simpleTerm("w3"), search.MUST)
	childRight.Add(simpleTerm("w4"), search.MUST)
	innerQuery.Add(childRight, search.MUST_NOT)
	outerQuery.Add(innerQuery, search.MUST)
	tc.qtest(outerQuery, []int{1})
}

func TestSimpleExplanations_BQ11(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("w1"), search.SHOULD)
	query.Add(search.NewBoostQuery(simpleTerm("w1"), 1000), search.SHOULD)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ14(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewBooleanQuery()
	q.Add(simpleTerm("QQQQQ"), search.SHOULD)
	q.Add(simpleTerm("w1"), search.SHOULD)
	tc.qtest(q, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ15(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewBooleanQuery()
	q.Add(simpleTerm("QQQQQ"), search.MUST_NOT)
	q.Add(simpleTerm("w1"), search.SHOULD)
	tc.qtest(q, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ16(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewBooleanQuery()
	q.Add(simpleTerm("QQQQQ"), search.SHOULD)
	booleanQuery := search.NewBooleanQuery()
	booleanQuery.Add(simpleTerm("w1"), search.SHOULD)
	booleanQuery.Add(simpleTerm("xx"), search.MUST_NOT)
	q.Add(booleanQuery, search.SHOULD)
	tc.qtest(q, []int{0, 1})
}

func TestSimpleExplanations_BQ17(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewBooleanQuery()
	q.Add(simpleTerm("w2"), search.SHOULD)
	booleanQuery := search.NewBooleanQuery()
	booleanQuery.Add(simpleTerm("w1"), search.SHOULD)
	booleanQuery.Add(simpleTerm("xx"), search.MUST_NOT)
	q.Add(booleanQuery, search.SHOULD)
	tc.qtest(q, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ19(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("yy"), search.MUST_NOT)
	query.Add(simpleTerm("w3"), search.SHOULD)
	tc.qtest(query, []int{0, 1})
}

func TestSimpleExplanations_BQ20(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewBooleanQuery()
	q.SetMinimumNumberShouldMatch(2)
	q.Add(simpleTerm("QQQQQ"), search.SHOULD)
	q.Add(simpleTerm("yy"), search.SHOULD)
	q.Add(simpleTerm("zz"), search.SHOULD)
	q.Add(simpleTerm("w5"), search.SHOULD)
	q.Add(simpleTerm("w4"), search.SHOULD)
	tc.qtest(q, []int{0, 3})
}

func TestSimpleExplanations_BQ21(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	q := search.NewBooleanQuery()
	q.Add(simpleTerm("yy"), search.SHOULD)
	q.Add(simpleTerm("zz"), search.SHOULD)
	tc.qtest(q, []int{1, 2, 3})
}

func TestSimpleExplanations_BQ23(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("w1"), search.FILTER)
	query.Add(simpleTerm("w2"), search.FILTER)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ24(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("w1"), search.FILTER)
	query.Add(simpleTerm("w2"), search.SHOULD)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ25(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("w1"), search.FILTER)
	query.Add(simpleTerm("w2"), search.MUST)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_BQ26(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("w1"), search.FILTER)
	query.Add(simpleTerm("xx"), search.MUST_NOT)
	tc.qtest(query, []int{0, 1})
}

/* BQ of TQ: using alt so some fields have zero boost and some don't */

func TestSimpleExplanations_MultiFieldBQ1(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("w1"), search.MUST)
	query.Add(simpleAltTerm("w2"), search.MUST)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MultiFieldBQ2(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("yy"), search.MUST)
	query.Add(simpleAltTerm("w3"), search.MUST)
	tc.qtest(query, []int{2, 3})
}

func TestSimpleExplanations_MultiFieldBQ3(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(simpleTerm("yy"), search.SHOULD)
	query.Add(simpleAltTerm("w3"), search.MUST)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MultiFieldBQ4(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.SHOULD)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleTerm("xx"), search.MUST_NOT)
	innerQuery.Add(simpleAltTerm("w2"), search.SHOULD)
	outerQuery.Add(innerQuery, search.SHOULD)
	tc.qtest(outerQuery, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MultiFieldBQ5(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.SHOULD)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleAltTerm("qq"), search.MUST)
	innerQuery.Add(simpleAltTerm("w2"), search.SHOULD)
	outerQuery.Add(innerQuery, search.SHOULD)
	tc.qtest(outerQuery, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MultiFieldBQ6(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.SHOULD)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleAltTerm("qq"), search.MUST_NOT)
	innerQuery.Add(simpleAltTerm("w5"), search.SHOULD)
	outerQuery.Add(innerQuery, search.MUST_NOT)
	tc.qtest(outerQuery, []int{1, 2, 3})
}

func TestSimpleExplanations_MultiFieldBQ7(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.MUST)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleAltTerm("qq"), search.SHOULD)
	childLeft := search.NewBooleanQuery()
	childLeft.Add(simpleAltTerm("xx"), search.SHOULD)
	childLeft.Add(simpleAltTerm("w2"), search.MUST_NOT)
	innerQuery.Add(childLeft, search.SHOULD)
	childRight := search.NewBooleanQuery()
	childRight.Add(simpleAltTerm("w3"), search.MUST)
	childRight.Add(simpleAltTerm("w4"), search.MUST)
	innerQuery.Add(childRight, search.SHOULD)
	outerQuery.Add(innerQuery, search.MUST)
	tc.qtest(outerQuery, []int{0})
}

func TestSimpleExplanations_MultiFieldBQ8(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleAltTerm("w1"), search.MUST)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleTerm("qq"), search.SHOULD)
	childLeft := search.NewBooleanQuery()
	childLeft.Add(simpleAltTerm("xx"), search.SHOULD)
	childLeft.Add(simpleTerm("w2"), search.MUST_NOT)
	innerQuery.Add(childLeft, search.SHOULD)
	childRight := search.NewBooleanQuery()
	childRight.Add(simpleAltTerm("w3"), search.MUST)
	childRight.Add(simpleTerm("w4"), search.MUST)
	innerQuery.Add(childRight, search.SHOULD)
	outerQuery.Add(innerQuery, search.SHOULD)
	tc.qtest(outerQuery, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MultiFieldBQ9(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.MUST)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleAltTerm("qq"), search.SHOULD)
	childLeft := search.NewBooleanQuery()
	childLeft.Add(simpleTerm("xx"), search.MUST_NOT)
	childLeft.Add(simpleTerm("w2"), search.SHOULD)
	innerQuery.Add(childLeft, search.SHOULD)
	childRight := search.NewBooleanQuery()
	childRight.Add(simpleAltTerm("w3"), search.MUST)
	childRight.Add(simpleTerm("w4"), search.MUST)
	innerQuery.Add(childRight, search.MUST_NOT)
	outerQuery.Add(innerQuery, search.SHOULD)
	tc.qtest(outerQuery, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MultiFieldBQ10(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(simpleTerm("w1"), search.MUST)
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(simpleAltTerm("qq"), search.SHOULD)
	childLeft := search.NewBooleanQuery()
	childLeft.Add(simpleTerm("xx"), search.MUST_NOT)
	childLeft.Add(simpleAltTerm("w2"), search.SHOULD)
	innerQuery.Add(childLeft, search.SHOULD)
	childRight := search.NewBooleanQuery()
	childRight.Add(simpleAltTerm("w3"), search.MUST)
	childRight.Add(simpleTerm("w4"), search.MUST)
	innerQuery.Add(childRight, search.MUST_NOT)
	outerQuery.Add(innerQuery, search.MUST)
	tc.qtest(outerQuery, []int{1})
}

/* BQ of PQ: using alt so some fields have zero boost and some don't */

func TestSimpleExplanations_MultiFieldBQofPQ1(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(search.NewPhraseQueryWithStrings(explField, "w1", "w2"), search.SHOULD)
	query.Add(search.NewPhraseQueryWithStrings(explAltField, "w1", "w2"), search.SHOULD)
	tc.qtest(query, []int{0})
}

func TestSimpleExplanations_MultiFieldBQofPQ2(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(search.NewPhraseQueryWithStrings(explField, "w1", "w3"), search.SHOULD)
	query.Add(search.NewPhraseQueryWithStrings(explAltField, "w1", "w3"), search.SHOULD)
	tc.qtest(query, []int{1, 3})
}

func TestSimpleExplanations_MultiFieldBQofPQ3(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(phraseSlop(1, explField, "w1", "w2"), search.SHOULD)
	query.Add(phraseSlop(1, explAltField, "w1", "w2"), search.SHOULD)
	tc.qtest(query, []int{0, 1, 2})
}

func TestSimpleExplanations_MultiFieldBQofPQ4(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(phraseSlop(1, explField, "w2", "w3"), search.SHOULD)
	query.Add(phraseSlop(1, explAltField, "w2", "w3"), search.SHOULD)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_MultiFieldBQofPQ5(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(phraseSlop(1, explField, "w3", "w2"), search.SHOULD)
	query.Add(phraseSlop(1, explAltField, "w3", "w2"), search.SHOULD)
	tc.qtest(query, []int{1, 3})
}

func TestSimpleExplanations_MultiFieldBQofPQ6(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(phraseSlop(2, explField, "w3", "w2"), search.SHOULD)
	query.Add(phraseSlop(2, explAltField, "w3", "w2"), search.SHOULD)
	tc.qtest(query, []int{0, 1, 3})
}

func TestSimpleExplanations_MultiFieldBQofPQ7(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewBooleanQuery()
	query.Add(phraseSlop(3, explField, "w3", "w2"), search.SHOULD)
	query.Add(phraseSlop(1, explAltField, "w3", "w2"), search.SHOULD)
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_SynonymQuery(t *testing.T) {
	tc := newExplanationTestCase(t)
	defer tc.cleanup()
	query := search.NewSynonymQueryBuilder(explField).
		AddTerm(index.NewTerm(explField, "w1")).
		AddTerm(index.NewTerm(explField, "w2")).
		Build()
	tc.qtest(query, []int{0, 1, 2, 3})
}

func TestSimpleExplanations_Equality(t *testing.T) {
	e1 := search.MatchExplanation(1, "an explanation")
	e2 := search.MatchExplanationWithDetails(1, "an explanation",
		search.MatchExplanation(1, "a subexplanation"))
	e25 := search.MatchExplanationWithDetails(1, "an explanation",
		search.MatchExplanationWithDetails(1, "a subexplanation",
			search.MatchExplanation(1, "a subsubexplanation")))
	e3 := search.MatchExplanation(1, "an explanation")
	e4 := search.MatchExplanation(2, "an explanation")
	e5 := search.NoMatchExplanation("an explanation")
	e6 := search.NoMatchExplanationWithDetails("an explanation",
		search.MatchExplanation(1, "a subexplanation"))
	e7 := search.NoMatchExplanation("an explanation")
	e8 := search.MatchExplanation(1, "another explanation")

	if !e1.Equals(e3) {
		t.Errorf("e1 should equal e3")
	}
	if e1.Equals(e2) {
		t.Errorf("e1 should not equal e2")
	}
	if e2.Equals(e25) {
		t.Errorf("e2 should not equal e25")
	}
	if e1.Equals(e4) {
		t.Errorf("e1 should not equal e4")
	}
	if e1.Equals(e5) {
		t.Errorf("e1 should not equal e5")
	}
	if !e5.Equals(e7) {
		t.Errorf("e5 should equal e7")
	}
	if e5.Equals(e6) {
		t.Errorf("e5 should not equal e6")
	}
	if e1.Equals(e8) {
		t.Errorf("e1 should not equal e8")
	}

	if e1.HashCode() != e3.HashCode() {
		t.Errorf("e1.HashCode() should equal e3.HashCode()")
	}
	if e5.HashCode() != e7.HashCode() {
		t.Errorf("e5.HashCode() should equal e7.HashCode()")
	}
}
