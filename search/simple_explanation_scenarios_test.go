// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Shared scenario table for the TestSimpleExplanations query shapes. It is
// consumed by TestSimpleExplanationsOfNonMatches (which re-runs every
// TestSimpleExplanations scenario in non-match mode), mirroring the way the
// Java TestSimpleExplanationsOfNonMatches subclass inherits all of its
// superclass's test methods and only overrides qtest.
//
// Each scenario carries the same query and expected matching-doc set used by
// the corresponding TestSimpleExplanations_* method.

package search_test

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// simpleExplanationScenario is one (name, query, expected docs) tuple from the
// TestSimpleExplanations suite.
type simpleExplanationScenario struct {
	name string
	q    search.Query
	exp  []int
}

// simpleExplanationScenarios returns every query shape exercised by
// TestSimpleExplanations, built against the given harness (needed for the
// matchTheseItems helper). The list is kept in lock-step with the individual
// TestSimpleExplanations_* methods.
func simpleExplanationScenarios(tc *explanationTestCase) []simpleExplanationScenario {
	dmq := func(tie float32, qs ...search.Query) search.Query {
		return search.NewDisjunctionMaxQueryWithTieBreaker(qs, tie)
	}
	mpq := func(slop int, groups ...[]string) search.Query {
		qb := search.NewMultiPhraseQueryBuilder()
		for _, g := range groups {
			qb.AddTerms(explTerms(g))
		}
		if slop > 0 {
			qb.SetSlop(slop)
		}
		return qb.Build()
	}
	bq := func(build func(q *search.BooleanQuery)) search.Query {
		q := search.NewBooleanQuery()
		build(q)
		return q
	}

	return []simpleExplanationScenario{
		{"T1", simpleTerm("w1"), []int{0, 1, 2, 3}},
		{"T2", search.NewBoostQuery(simpleTerm("w1"), 100), []int{0, 1, 2, 3}},
		{"MA1", search.NewMatchAllDocsQuery(), []int{0, 1, 2, 3}},
		{"MA2", search.NewBoostQuery(search.NewMatchAllDocsQuery(), 1000), []int{0, 1, 2, 3}},
		{"P1", search.NewPhraseQueryWithStrings(explField, "w1", "w2"), []int{0}},
		{"P2", search.NewPhraseQueryWithStrings(explField, "w1", "w3"), []int{1, 3}},
		{"P3", phraseSlop(1, explField, "w1", "w2"), []int{0, 1, 2}},
		{"P4", phraseSlop(1, explField, "w2", "w3"), []int{0, 1, 2, 3}},
		{"P5", phraseSlop(1, explField, "w3", "w2"), []int{1, 3}},
		{"P6", phraseSlop(2, explField, "w3", "w2"), []int{0, 1, 3}},
		{"P7", phraseSlop(3, explField, "w3", "w2"), []int{0, 1, 2, 3}},
		{"CSQ1", search.NewConstantScoreQuery(tc.matchTheseItems([]int{0, 1, 2, 3})), []int{0, 1, 2, 3}},
		{"CSQ2", search.NewConstantScoreQuery(tc.matchTheseItems([]int{1, 3})), []int{1, 3}},
		{"CSQ3", search.NewBoostQuery(search.NewConstantScoreQuery(tc.matchTheseItems([]int{0, 2})), 1000), []int{0, 2}},
		{"DMQ1", dmq(0.0, simpleTerm("w1"), simpleTerm("w5")), []int{0, 1, 2, 3}},
		{"DMQ2", dmq(0.5, simpleTerm("w1"), simpleTerm("w5")), []int{0, 1, 2, 3}},
		{"DMQ3", dmq(0.5, simpleTerm("QQ"), simpleTerm("w5")), []int{0}},
		{"DMQ4", dmq(0.5, simpleTerm("QQ"), simpleTerm("xx")), []int{2, 3}},
		{"DMQ5", dmq(0.5, bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.SHOULD)
			q.Add(simpleTerm("QQ"), search.MUST_NOT)
		}), simpleTerm("xx")), []int{2, 3}},
		{"DMQ6", dmq(0.5, bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.MUST_NOT)
			q.Add(simpleTerm("w3"), search.SHOULD)
		}), simpleTerm("xx")), []int{0, 1, 2, 3}},
		{"DMQ7", dmq(0.5, bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.MUST_NOT)
			q.Add(simpleTerm("w3"), search.SHOULD)
		}), simpleTerm("w2")), []int{0, 1, 2, 3}},
		{"DMQ8", dmq(0.5, bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.SHOULD)
			q.Add(search.NewBoostQuery(simpleTerm("w5"), 100), search.SHOULD)
		}), search.NewBoostQuery(simpleTerm("xx"), 100000)), []int{0, 2, 3}},
		{"DMQ9", dmq(0.5, bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.SHOULD)
			q.Add(search.NewBoostQuery(simpleTerm("w5"), 100), search.SHOULD)
		}), search.NewBoostQuery(simpleTerm("xx"), 0)), []int{0, 2, 3}},
		{"MPQ1", mpq(0, []string{"w1"}, []string{"w2", "w3", "xx"}), []int{0, 1, 2, 3}},
		{"MPQ2", mpq(0, []string{"w1"}, []string{"w2", "w3"}), []int{0, 1, 3}},
		{"MPQ3", mpq(0, []string{"w1", "xx"}, []string{"w2", "w3"}), []int{0, 1, 2, 3}},
		{"MPQ4", mpq(0, []string{"w1"}, []string{"w2"}), []int{0}},
		{"MPQ5", mpq(1, []string{"w1"}, []string{"w2"}), []int{0, 1, 2}},
		{"MPQ6", mpq(1, []string{"w1", "w3"}, []string{"w2"}), []int{0, 1, 2, 3}},
		{"BQ1", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.MUST)
			q.Add(simpleTerm("w2"), search.MUST)
		}), []int{0, 1, 2, 3}},
		{"BQ2", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.MUST)
			q.Add(simpleTerm("w3"), search.MUST)
		}), []int{2, 3}},
		{"BQ3", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.SHOULD)
			q.Add(simpleTerm("w3"), search.MUST)
		}), []int{0, 1, 2, 3}},
		{"BQ4", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.SHOULD)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("xx"), search.MUST_NOT)
			inner.Add(simpleTerm("w2"), search.SHOULD)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"BQ5", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.SHOULD)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("qq"), search.MUST)
			inner.Add(simpleTerm("w2"), search.SHOULD)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"BQ6", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.SHOULD)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("qq"), search.MUST_NOT)
			inner.Add(simpleTerm("w5"), search.SHOULD)
			q.Add(inner, search.MUST_NOT)
		}), []int{1, 2, 3}},
		{"BQ7", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.MUST)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("qq"), search.SHOULD)
			cl := search.NewBooleanQuery()
			cl.Add(simpleTerm("xx"), search.SHOULD)
			cl.Add(simpleTerm("w2"), search.MUST_NOT)
			inner.Add(cl, search.SHOULD)
			cr := search.NewBooleanQuery()
			cr.Add(simpleTerm("w3"), search.MUST)
			cr.Add(simpleTerm("w4"), search.MUST)
			inner.Add(cr, search.SHOULD)
			q.Add(inner, search.MUST)
		}), []int{0}},
		{"BQ8", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.MUST)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("qq"), search.SHOULD)
			cl := search.NewBooleanQuery()
			cl.Add(simpleTerm("xx"), search.SHOULD)
			cl.Add(simpleTerm("w2"), search.MUST_NOT)
			inner.Add(cl, search.SHOULD)
			cr := search.NewBooleanQuery()
			cr.Add(simpleTerm("w3"), search.MUST)
			cr.Add(simpleTerm("w4"), search.MUST)
			inner.Add(cr, search.SHOULD)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"BQ9", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.MUST)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("qq"), search.SHOULD)
			cl := search.NewBooleanQuery()
			cl.Add(simpleTerm("xx"), search.MUST_NOT)
			cl.Add(simpleTerm("w2"), search.SHOULD)
			inner.Add(cl, search.SHOULD)
			cr := search.NewBooleanQuery()
			cr.Add(simpleTerm("w3"), search.MUST)
			cr.Add(simpleTerm("w4"), search.MUST)
			inner.Add(cr, search.MUST_NOT)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"BQ10", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.MUST)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("qq"), search.SHOULD)
			cl := search.NewBooleanQuery()
			cl.Add(simpleTerm("xx"), search.MUST_NOT)
			cl.Add(simpleTerm("w2"), search.SHOULD)
			inner.Add(cl, search.SHOULD)
			cr := search.NewBooleanQuery()
			cr.Add(simpleTerm("w3"), search.MUST)
			cr.Add(simpleTerm("w4"), search.MUST)
			inner.Add(cr, search.MUST_NOT)
			q.Add(inner, search.MUST)
		}), []int{1}},
		{"BQ11", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.SHOULD)
			q.Add(search.NewBoostQuery(simpleTerm("w1"), 1000), search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"BQ14", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("QQQQQ"), search.SHOULD)
			q.Add(simpleTerm("w1"), search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"BQ15", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("QQQQQ"), search.MUST_NOT)
			q.Add(simpleTerm("w1"), search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"BQ16", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("QQQQQ"), search.SHOULD)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("w1"), search.SHOULD)
			inner.Add(simpleTerm("xx"), search.MUST_NOT)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1}},
		{"BQ17", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w2"), search.SHOULD)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("w1"), search.SHOULD)
			inner.Add(simpleTerm("xx"), search.MUST_NOT)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"BQ19", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.MUST_NOT)
			q.Add(simpleTerm("w3"), search.SHOULD)
		}), []int{0, 1}},
		{"BQ20", bq(func(q *search.BooleanQuery) {
			q.SetMinimumNumberShouldMatch(2)
			q.Add(simpleTerm("QQQQQ"), search.SHOULD)
			q.Add(simpleTerm("yy"), search.SHOULD)
			q.Add(simpleTerm("zz"), search.SHOULD)
			q.Add(simpleTerm("w5"), search.SHOULD)
			q.Add(simpleTerm("w4"), search.SHOULD)
		}), []int{0, 3}},
		{"BQ21", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.SHOULD)
			q.Add(simpleTerm("zz"), search.SHOULD)
		}), []int{1, 2, 3}},
		{"BQ23", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.FILTER)
			q.Add(simpleTerm("w2"), search.FILTER)
		}), []int{0, 1, 2, 3}},
		{"BQ24", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.FILTER)
			q.Add(simpleTerm("w2"), search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"BQ25", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.FILTER)
			q.Add(simpleTerm("w2"), search.MUST)
		}), []int{0, 1, 2, 3}},
		{"BQ26", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.FILTER)
			q.Add(simpleTerm("xx"), search.MUST_NOT)
		}), []int{0, 1}},
		{"MultiFieldBQ1", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.MUST)
			q.Add(simpleAltTerm("w2"), search.MUST)
		}), []int{0, 1, 2, 3}},
		{"MultiFieldBQ2", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.MUST)
			q.Add(simpleAltTerm("w3"), search.MUST)
		}), []int{2, 3}},
		{"MultiFieldBQ3", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("yy"), search.SHOULD)
			q.Add(simpleAltTerm("w3"), search.MUST)
		}), []int{0, 1, 2, 3}},
		{"MultiFieldBQ4", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.SHOULD)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("xx"), search.MUST_NOT)
			inner.Add(simpleAltTerm("w2"), search.SHOULD)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"MultiFieldBQ5", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.SHOULD)
			inner := search.NewBooleanQuery()
			inner.Add(simpleAltTerm("qq"), search.MUST)
			inner.Add(simpleAltTerm("w2"), search.SHOULD)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"MultiFieldBQ6", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.SHOULD)
			inner := search.NewBooleanQuery()
			inner.Add(simpleAltTerm("qq"), search.MUST_NOT)
			inner.Add(simpleAltTerm("w5"), search.SHOULD)
			q.Add(inner, search.MUST_NOT)
		}), []int{1, 2, 3}},
		{"MultiFieldBQ7", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.MUST)
			inner := search.NewBooleanQuery()
			inner.Add(simpleAltTerm("qq"), search.SHOULD)
			cl := search.NewBooleanQuery()
			cl.Add(simpleAltTerm("xx"), search.SHOULD)
			cl.Add(simpleAltTerm("w2"), search.MUST_NOT)
			inner.Add(cl, search.SHOULD)
			cr := search.NewBooleanQuery()
			cr.Add(simpleAltTerm("w3"), search.MUST)
			cr.Add(simpleAltTerm("w4"), search.MUST)
			inner.Add(cr, search.SHOULD)
			q.Add(inner, search.MUST)
		}), []int{0}},
		{"MultiFieldBQ8", bq(func(q *search.BooleanQuery) {
			q.Add(simpleAltTerm("w1"), search.MUST)
			inner := search.NewBooleanQuery()
			inner.Add(simpleTerm("qq"), search.SHOULD)
			cl := search.NewBooleanQuery()
			cl.Add(simpleAltTerm("xx"), search.SHOULD)
			cl.Add(simpleTerm("w2"), search.MUST_NOT)
			inner.Add(cl, search.SHOULD)
			cr := search.NewBooleanQuery()
			cr.Add(simpleAltTerm("w3"), search.MUST)
			cr.Add(simpleTerm("w4"), search.MUST)
			inner.Add(cr, search.SHOULD)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"MultiFieldBQ9", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.MUST)
			inner := search.NewBooleanQuery()
			inner.Add(simpleAltTerm("qq"), search.SHOULD)
			cl := search.NewBooleanQuery()
			cl.Add(simpleTerm("xx"), search.MUST_NOT)
			cl.Add(simpleTerm("w2"), search.SHOULD)
			inner.Add(cl, search.SHOULD)
			cr := search.NewBooleanQuery()
			cr.Add(simpleAltTerm("w3"), search.MUST)
			cr.Add(simpleTerm("w4"), search.MUST)
			inner.Add(cr, search.MUST_NOT)
			q.Add(inner, search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"MultiFieldBQ10", bq(func(q *search.BooleanQuery) {
			q.Add(simpleTerm("w1"), search.MUST)
			inner := search.NewBooleanQuery()
			inner.Add(simpleAltTerm("qq"), search.SHOULD)
			cl := search.NewBooleanQuery()
			cl.Add(simpleTerm("xx"), search.MUST_NOT)
			cl.Add(simpleAltTerm("w2"), search.SHOULD)
			inner.Add(cl, search.SHOULD)
			cr := search.NewBooleanQuery()
			cr.Add(simpleAltTerm("w3"), search.MUST)
			cr.Add(simpleTerm("w4"), search.MUST)
			inner.Add(cr, search.MUST_NOT)
			q.Add(inner, search.MUST)
		}), []int{1}},
		{"MultiFieldBQofPQ1", bq(func(q *search.BooleanQuery) {
			q.Add(search.NewPhraseQueryWithStrings(explField, "w1", "w2"), search.SHOULD)
			q.Add(search.NewPhraseQueryWithStrings(explAltField, "w1", "w2"), search.SHOULD)
		}), []int{0}},
		{"MultiFieldBQofPQ2", bq(func(q *search.BooleanQuery) {
			q.Add(search.NewPhraseQueryWithStrings(explField, "w1", "w3"), search.SHOULD)
			q.Add(search.NewPhraseQueryWithStrings(explAltField, "w1", "w3"), search.SHOULD)
		}), []int{1, 3}},
		{"MultiFieldBQofPQ3", bq(func(q *search.BooleanQuery) {
			q.Add(phraseSlop(1, explField, "w1", "w2"), search.SHOULD)
			q.Add(phraseSlop(1, explAltField, "w1", "w2"), search.SHOULD)
		}), []int{0, 1, 2}},
		{"MultiFieldBQofPQ4", bq(func(q *search.BooleanQuery) {
			q.Add(phraseSlop(1, explField, "w2", "w3"), search.SHOULD)
			q.Add(phraseSlop(1, explAltField, "w2", "w3"), search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"MultiFieldBQofPQ5", bq(func(q *search.BooleanQuery) {
			q.Add(phraseSlop(1, explField, "w3", "w2"), search.SHOULD)
			q.Add(phraseSlop(1, explAltField, "w3", "w2"), search.SHOULD)
		}), []int{1, 3}},
		{"MultiFieldBQofPQ6", bq(func(q *search.BooleanQuery) {
			q.Add(phraseSlop(2, explField, "w3", "w2"), search.SHOULD)
			q.Add(phraseSlop(2, explAltField, "w3", "w2"), search.SHOULD)
		}), []int{0, 1, 3}},
		{"MultiFieldBQofPQ7", bq(func(q *search.BooleanQuery) {
			q.Add(phraseSlop(3, explField, "w3", "w2"), search.SHOULD)
			q.Add(phraseSlop(1, explAltField, "w3", "w2"), search.SHOULD)
		}), []int{0, 1, 2, 3}},
		{"SynonymQuery", search.NewSynonymQueryBuilder(explField).
			AddTerm(index.NewTerm(explField, "w1")).
			AddTerm(index.NewTerm(explField, "w2")).
			Build(), []int{0, 1, 2, 3}},
	}
}
