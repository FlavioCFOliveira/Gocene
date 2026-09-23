// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestWildcardQuery.java
// (Apache Lucene 10.5.0): tests the '*' and '?' wildcard characters.

package search_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

func TestWildcardQueryEquals(t *testing.T) {
	wq1 := search.NewWildcardQuery(index.NewTerm("field", "b*a"))
	wq2 := search.NewWildcardQuery(index.NewTerm("field", "b*a"))
	wq3 := search.NewWildcardQuery(index.NewTerm("field", "b*a"))

	// reflexive?
	if !wq1.Equals(wq2) {
		t.Fatal("wq1 != wq2")
	}
	if !wq2.Equals(wq1) {
		t.Fatal("wq2 != wq1")
	}

	// transitive?
	if !wq2.Equals(wq3) {
		t.Fatal("wq2 != wq3")
	}
	if !wq1.Equals(wq3) {
		t.Fatal("wq1 != wq3")
	}

	if wq1.Equals(nil) {
		t.Fatal("wq1.equals(null)")
	}

	fq := search.NewFuzzyQuery(index.NewTerm("field", "b*a"))
	if wq1.Equals(fq) {
		t.Fatal("wq1.equals(fq)")
	}
	if fq.Equals(wq1) {
		t.Fatal("fq.equals(wq1)")
	}
}

// Tests if a WildcardQuery that has no wildcard in the term is rewritten to a
// single TermQuery. The boost should be preserved, and the rewrite should
// return a ConstantScoreQuery if the WildcardQuery had a ConstantScore
// rewriteMethod.
func TestWildcardQueryTermWithoutWildcard(t *testing.T) {
	indexStore := getWildcardIndexStore(t, "field", []string{"nowildcard", "nowildcardx"})
	reader := mustOpenDirectoryReader(t, indexStore)
	searcher := newSearcher(t, reader)

	var wq search.Query = search.NewWildcardQuery(index.NewTerm("field", "nowildcard"))
	assertWildcardMatches(t, searcher, wq, 1)

	q := mustRewrite(t, searcher,
		search.NewWildcardQueryWithRewrite(
			index.NewTerm("field", "nowildcard"),
			automaton.DefaultDeterminizeWorkLimit,
			search.ScoringBooleanRewrite))
	if _, ok := q.(*search.TermQuery); !ok {
		t.Fatalf("expected TermQuery, got %T", q)
	}

	q = mustRewrite(t, searcher,
		search.NewWildcardQueryWithRewrite(
			index.NewTerm("field", "nowildcard"),
			automaton.DefaultDeterminizeWorkLimit,
			search.ConstantScoreRewrite))
	if _, ok := q.(*search.MultiTermQueryConstantScoreWrapper); !ok {
		t.Fatalf("expected MultiTermQueryConstantScoreWrapper, got %T", q)
	}

	q = mustRewrite(t, searcher,
		search.NewWildcardQueryWithRewrite(
			index.NewTerm("field", "nowildcard"),
			automaton.DefaultDeterminizeWorkLimit,
			search.ConstantScoreBlendedRewrite))
	if !search.IsMultiTermQueryConstantScoreBlendedWrapper(q) {
		t.Fatalf("expected MultiTermQueryConstantScoreBlendedWrapper, got %T", q)
	}

	q = mustRewrite(t, searcher,
		search.NewWildcardQueryWithRewrite(
			index.NewTerm("field", "nowildcard"),
			automaton.DefaultDeterminizeWorkLimit,
			search.ConstantScoreBooleanRewrite))
	if _, ok := q.(*search.ConstantScoreQuery); !ok {
		t.Fatalf("expected ConstantScoreQuery, got %T", q)
	}
	mustClose(t, reader, indexStore)
}

// Tests if a WildcardQuery with an empty term is rewritten to an empty
// BooleanQuery.
func TestWildcardQueryEmptyTerm(t *testing.T) {
	indexStore := getWildcardIndexStore(t, "field", []string{"nowildcard", "nowildcardx"})
	reader := mustOpenDirectoryReader(t, indexStore)
	searcher := newSearcher(t, reader)

	wq := search.NewWildcardQueryWithRewrite(
		index.NewTerm("field", ""),
		automaton.DefaultDeterminizeWorkLimit,
		search.ScoringBooleanRewrite)
	assertWildcardMatches(t, searcher, wq, 0)
	q := mustRewrite(t, searcher, wq)
	if _, ok := q.(*search.MatchNoDocsQuery); !ok {
		t.Fatalf("expected MatchNoDocsQuery, got %T", q)
	}
	mustClose(t, reader, indexStore)
}

// Tests if a WildcardQuery that has only a trailing * in the term is
// rewritten to a single PrefixQuery. The boost and rewriteMethod should be
// preserved.
func TestWildcardQueryPrefixTerm(t *testing.T) {
	indexStore := getWildcardIndexStore(t, "field", []string{"prefix", "prefixx"})
	reader := mustOpenDirectoryReader(t, indexStore)
	searcher := newSearcher(t, reader)

	wq := search.NewWildcardQuery(index.NewTerm("field", "prefix*"))
	assertWildcardMatches(t, searcher, wq, 2)

	wq = search.NewWildcardQuery(index.NewTerm("field", "*"))
	assertWildcardMatches(t, searcher, wq, 2)
	terms, err := index.MultiTermsGetTerms(searcher.GetIndexReader(), "field")
	if err != nil {
		t.Fatalf("MultiTerms.getTerms: %v", err)
	}
	te, err := wq.GetTermsEnum(terms)
	if err != nil {
		t.Fatalf("getTermsEnum: %v", err)
	}
	if strings.Contains(fmt.Sprintf("%T", te), "AutomatonTermsEnum") {
		t.Fatalf("unexpected terms enum %T", te)
	}
	mustClose(t, reader, indexStore)
}

// Tests Wildcard queries with an asterisk.
func TestWildcardQueryAsterisk(t *testing.T) {
	indexStore := getWildcardIndexStore(t, "body", []string{"metal", "metals"})
	reader := mustOpenDirectoryReader(t, indexStore)
	searcher := newSearcher(t, reader)
	query1 := search.NewTermQuery(index.NewTerm("body", "metal"))
	query2 := search.NewWildcardQuery(index.NewTerm("body", "metal*"))
	query3 := search.NewWildcardQuery(index.NewTerm("body", "m*tal"))
	query4 := search.NewWildcardQuery(index.NewTerm("body", "m*tal*"))
	query5 := search.NewWildcardQuery(index.NewTerm("body", "m*tals"))

	query6 := search.NewBooleanQueryBuilder()
	query6.Add(query5, search.SHOULD)

	query7 := search.NewBooleanQueryBuilder()
	query7.Add(query3, search.SHOULD)
	query7.Add(query5, search.SHOULD)

	// Queries do not automatically lower-case search terms:
	query8 := search.NewWildcardQuery(index.NewTerm("body", "M*tal*"))

	assertWildcardMatches(t, searcher, query1, 1)
	assertWildcardMatches(t, searcher, query2, 2)
	assertWildcardMatches(t, searcher, query3, 1)
	assertWildcardMatches(t, searcher, query4, 2)
	assertWildcardMatches(t, searcher, query5, 1)
	assertWildcardMatches(t, searcher, query6.Build(), 1)
	assertWildcardMatches(t, searcher, query7.Build(), 2)
	assertWildcardMatches(t, searcher, query8, 0)
	assertWildcardMatches(t, searcher, search.NewWildcardQuery(index.NewTerm("body", "*tall")), 0)
	assertWildcardMatches(t, searcher, search.NewWildcardQuery(index.NewTerm("body", "*tal")), 1)
	assertWildcardMatches(t, searcher, search.NewWildcardQuery(index.NewTerm("body", "*tal*")), 2)
	mustClose(t, reader, indexStore)
}

// Tests Wildcard queries with a question mark.
func TestWildcardQueryQuestionmark(t *testing.T) {
	indexStore := getWildcardIndexStore(t, "body", []string{"metal", "metals", "mXtals", "mXtXls"})
	reader := mustOpenDirectoryReader(t, indexStore)
	searcher := newSearcher(t, reader)
	query1 := search.NewWildcardQuery(index.NewTerm("body", "m?tal"))
	query2 := search.NewWildcardQuery(index.NewTerm("body", "metal?"))
	query3 := search.NewWildcardQuery(index.NewTerm("body", "metals?"))
	query4 := search.NewWildcardQuery(index.NewTerm("body", "m?t?ls"))
	query5 := search.NewWildcardQuery(index.NewTerm("body", "M?t?ls"))
	query6 := search.NewWildcardQuery(index.NewTerm("body", "meta??"))

	assertWildcardMatches(t, searcher, query1, 1)
	assertWildcardMatches(t, searcher, query2, 1)
	assertWildcardMatches(t, searcher, query3, 0)
	assertWildcardMatches(t, searcher, query4, 3)
	assertWildcardMatches(t, searcher, query5, 0)
	assertWildcardMatches(t, searcher, query6, 1) // Query: 'meta??' matches 'metals' not 'metal'
	mustClose(t, reader, indexStore)
}

// Tests if wildcard escaping works.
func TestWildcardQueryEscapes(t *testing.T) {
	indexStore := getWildcardIndexStore(t,
		"field", []string{"foo*bar", "foo??bar", "fooCDbar", "fooSOMETHINGbar", "foo\\"})
	reader := mustOpenDirectoryReader(t, indexStore)
	searcher := newSearcher(t, reader)

	// without escape: matches foo??bar, fooCDbar, foo*bar, and fooSOMETHINGbar
	unescaped := search.NewWildcardQuery(index.NewTerm("field", "foo*bar"))
	assertWildcardMatches(t, searcher, unescaped, 4)

	// with escape: only matches foo*bar
	escaped := search.NewWildcardQuery(index.NewTerm("field", "foo\\*bar"))
	assertWildcardMatches(t, searcher, escaped, 1)

	// without escape: matches foo??bar and fooCDbar
	unescaped = search.NewWildcardQuery(index.NewTerm("field", "foo??bar"))
	assertWildcardMatches(t, searcher, unescaped, 2)

	// with escape: matches foo??bar only
	escaped = search.NewWildcardQuery(index.NewTerm("field", "foo\\?\\?bar"))
	assertWildcardMatches(t, searcher, escaped, 1)

	// check escaping at end: lenient parse yields "foo\"
	atEnd := search.NewWildcardQuery(index.NewTerm("field", "foo\\"))
	assertWildcardMatches(t, searcher, atEnd, 1)

	mustClose(t, reader, indexStore)
}

// getWildcardIndexStore renders the private getIndexStore(String, String[]).
func getWildcardIndexStore(t *testing.T, field string, contents []string) store.Directory {
	t.Helper()
	indexStore := newDirectory()
	writer := newRandomIndexWriter(t, indexStore)
	for i := 0; i < len(contents); i++ {
		doc := newTestDocument(newTextField(t, field, contents[i], true))
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)

	return indexStore
}

// assertWildcardMatches renders the private assertMatches(IndexSearcher, Query, int).
func assertWildcardMatches(t *testing.T, searcher *search.IndexSearcher, q search.Query, expectedMatches int) {
	t.Helper()
	result := mustSearch(t, searcher, q, 1000).ScoreDocs
	if len(result) != expectedMatches {
		t.Fatalf("expected %d matches, got %d", expectedMatches, len(result))
	}
}

// Test that wild card queries are parsed to the correct type and are searched
// correctly. This test looks at both parsing and execution of wildcard
// queries. Although placed here, it also tests prefix queries, verifying that
// prefix queries are not parsed into wild card queries, and vice-versa.
func TestWildcardQueryParsingAndSearching(t *testing.T) {
	field := "content"
	docs := []string{
		"\\ abcdefg1", "\\79 hijklmn1", "\\\\ opqrstu1",
	}

	// queries that should find all docs
	matchAll := []search.Query{
		search.NewWildcardQuery(index.NewTerm(field, "*")),
		search.NewWildcardQuery(index.NewTerm(field, "*1")),
		search.NewWildcardQuery(index.NewTerm(field, "**1")),
		search.NewWildcardQuery(index.NewTerm(field, "*?")),
		search.NewWildcardQuery(index.NewTerm(field, "*?1")),
		search.NewWildcardQuery(index.NewTerm(field, "?*1")),
		search.NewWildcardQuery(index.NewTerm(field, "**")),
		search.NewWildcardQuery(index.NewTerm(field, "***")),
		search.NewWildcardQuery(index.NewTerm(field, "\\\\*")),
	}

	// queries that should find no docs
	matchNone := []search.Query{
		search.NewWildcardQuery(index.NewTerm(field, "a*h")),
		search.NewWildcardQuery(index.NewTerm(field, "a?h")),
		search.NewWildcardQuery(index.NewTerm(field, "*a*h")),
		search.NewWildcardQuery(index.NewTerm(field, "?a")),
		search.NewWildcardQuery(index.NewTerm(field, "a?")),
	}

	matchOneDocPrefix := [][]search.Query{
		{
			search.NewPrefixQuery(index.NewTerm(field, "a")),
			search.NewPrefixQuery(index.NewTerm(field, "ab")),
			search.NewPrefixQuery(index.NewTerm(field, "abc")),
		}, // these should find only doc 0
		{
			search.NewPrefixQuery(index.NewTerm(field, "h")),
			search.NewPrefixQuery(index.NewTerm(field, "hi")),
			search.NewPrefixQuery(index.NewTerm(field, "hij")),
			search.NewPrefixQuery(index.NewTerm(field, "\\7")),
		}, // these should find only doc 1
		{
			search.NewPrefixQuery(index.NewTerm(field, "o")),
			search.NewPrefixQuery(index.NewTerm(field, "op")),
			search.NewPrefixQuery(index.NewTerm(field, "opq")),
			search.NewPrefixQuery(index.NewTerm(field, "\\\\")),
		}, // these should find only doc 2
	}

	matchOneDocWild := [][]search.Query{
		{
			search.NewWildcardQuery(index.NewTerm(field, "*a*")), // these should find only doc 0
			search.NewWildcardQuery(index.NewTerm(field, "*ab*")),
			search.NewWildcardQuery(index.NewTerm(field, "*abc**")),
			search.NewWildcardQuery(index.NewTerm(field, "ab*e*")),
			search.NewWildcardQuery(index.NewTerm(field, "*g?")),
			search.NewWildcardQuery(index.NewTerm(field, "*f?1")),
		},
		{
			search.NewWildcardQuery(index.NewTerm(field, "*h*")), // these should find only doc 1
			search.NewWildcardQuery(index.NewTerm(field, "*hi*")),
			search.NewWildcardQuery(index.NewTerm(field, "*hij**")),
			search.NewWildcardQuery(index.NewTerm(field, "hi*k*")),
			search.NewWildcardQuery(index.NewTerm(field, "*n?")),
			search.NewWildcardQuery(index.NewTerm(field, "*m?1")),
			search.NewWildcardQuery(index.NewTerm(field, "hij**")),
		},
		{
			search.NewWildcardQuery(index.NewTerm(field, "*o*")), // these should find only doc 2
			search.NewWildcardQuery(index.NewTerm(field, "*op*")),
			search.NewWildcardQuery(index.NewTerm(field, "*opq**")),
			search.NewWildcardQuery(index.NewTerm(field, "op*q*")),
			search.NewWildcardQuery(index.NewTerm(field, "*u?")),
			search.NewWildcardQuery(index.NewTerm(field, "*t?1")),
			search.NewWildcardQuery(index.NewTerm(field, "opq**")),
		},
	}

	// prepare the index
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	iw := newRandomIndexWriterWithConfig(t, dir, iwc)
	for i := 0; i < len(docs); i++ {
		doc := newTestDocument(newTextField(t, field, docs[i], false))
		mustAddDocument(t, iw, doc)
	}
	mustClose(t, iw)

	reader := mustOpenDirectoryReader(t, dir)
	searcher := newSearcher(t, reader)

	// test queries that must find all
	for _, q := range matchAll {
		hits := mustSearch(t, searcher, q, 1000).ScoreDocs
		if len(hits) != len(docs) {
			t.Fatalf("matchAll: q=%v: expected %d, got %d", q, len(docs), len(hits))
		}
	}

	// test queries that must find none
	for _, q := range matchNone {
		hits := mustSearch(t, searcher, q, 1000).ScoreDocs
		if len(hits) != 0 {
			t.Fatalf("matchNone: q=%v: expected 0, got %d", q, len(hits))
		}
	}

	// thest the prefi queries find only one doc
	for i := 0; i < len(matchOneDocPrefix); i++ {
		for j := 0; j < len(matchOneDocPrefix[i]); j++ {
			q := matchOneDocPrefix[i][j]
			hits := mustSearch(t, searcher, q, 1000).ScoreDocs
			if len(hits) != 1 {
				t.Fatalf("match 1 prefix: doc=%s q=%v: expected 1, got %d", docs[i], q, len(hits))
			}
			if hits[0].Doc != i {
				t.Fatalf("match 1 prefix: doc=%s q=%v: expected doc %d, got %d", docs[i], q, i, hits[0].Doc)
			}
		}
	}

	// test the wildcard queries find only one doc
	for i := 0; i < len(matchOneDocWild); i++ {
		for j := 0; j < len(matchOneDocWild[i]); j++ {
			q := matchOneDocWild[i][j]
			hits := mustSearch(t, searcher, q, 1000).ScoreDocs
			if len(hits) != 1 {
				t.Fatalf("match 1 wild: doc=%s q=%v: expected 1, got %d", docs[i], q, len(hits))
			}
			if hits[0].Doc != i {
				t.Fatalf("match 1 wild: doc=%s q=%v: expected doc %d, got %d", docs[i], q, i, hits[0].Doc)
			}
		}
	}

	mustClose(t, reader, dir)
}

// Tests large Wildcard queries.
func TestWildcardQueryLarge(t *testing.T) {
	// big string from a user
	big := "{group-bm-http-server-02083.node.dm.reg,group-bm-http-server-02082.node.dm.reg,group-bm-http-server-02081.node.dm.reg,group-bm-http-server-02080.node.dm.reg,group-bm-http-server-02079.node.dm.reg,group-bm-http-server-02078.node.dm.reg,group-bm-http-server-02077.node.dm.reg,group-bm-http-server-02076.node.dm.reg,group-bm-http-server-02073.node.dm.reg,group-bm-http-server-02070.node.dm.reg,group-bm-http-server-02067.node.dm.reg,group-bm-http-server-02064.node.dm.reg,group-bm-http-server-02029.node.dm.reg,group-bm-http-server-02028.node.dm.reg,group-bm-http-server-02027.node.dm.reg,group-bm-http-server-02026.node.dm.reg,group-bm-http-server-02025.node.dm.reg,group-bm-http-server-02023.node.dm.reg,group-bm-http-server-02022.node.dm.reg,group-bm-http-server-02021.node.dm.reg,group-bm-http-server-02020.node.dm.reg,group-bm-http-server-02019.node.dm.reg,group-bm-http-server-02018.node.dm.reg,group-bm-http-server-02016.node.dm.reg,group-bm-http-server-02015.node.dm.reg,group-bm-http-server-02014.node.dm.reg,group-bm-http-server-02009.node.dm.reg,group-bm-http-server-02007.node.dm.reg,group-bm-http-server-02004.node.dm.reg,group-bm-http-server-02003.node.dm.reg,group-bm-http-server-02002.node.dm.reg,group-bm-http-server-01311.node.dm.reg,group-bm-http-server-01309.node.dm.reg,group-bm-http-server-01307.node.dm.reg}"
	dir := newDirectory()
	writer := newRandomIndexWriter(t, dir)
	doc := newTestDocument(newStringField(t, "body", big, true))
	mustAddDocument(t, writer, doc)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	searcher := newSearcher(t, reader)
	query := search.NewWildcardQuery(index.NewTerm("body", big+"*"))
	assertWildcardMatches(t, searcher, query, 1)

	mustClose(t, reader, dir)
}

func TestWildcardQueryCostEstimate(t *testing.T) {
	dir := newDirectory()
	writer := newRandomIndexWriter(t, dir)
	for i := 0; i < 1000; i++ {
		doc := newTestDocument(newStringField(t, "body", "foo bar", false))
		mustAddDocument(t, writer, doc)
		doc = newTestDocument(newStringField(t, "body", "foo wuzzle", false))
		mustAddDocument(t, writer, doc)
		doc = newTestDocument(newStringField(t, "body", "bar "+strconv.Itoa(i), false))
		mustAddDocument(t, writer, doc)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	searcher := newSearcher(t, reader)
	lrc := mustLeaves(t, reader)[0]

	query := search.NewWildcardQuery(index.NewTerm("body", "foo*"))
	rewritten := mustRewrite(t, searcher, query)
	weight, err := rewritten.CreateWeight(searcher, search.COMPLETE_NO_SCORES, 1.0)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	supplier, err := weight.ScorerSupplier(lrc)
	if err != nil {
		t.Fatalf("scorerSupplier: %v", err)
	}
	// Automaton queries have an unknown term count, so term collection is
	// deferred to get() and the cost is the worst-case estimate (sum of doc
	// freqs across all terms) rather than the sum over the matching terms only.
	if got := supplier.Cost(); got != 3000 {
		t.Fatalf("cost: expected 3000, got %d", got)
	}

	query = search.NewWildcardQuery(index.NewTerm("body", "bar*"))
	rewritten = mustRewrite(t, searcher, query)
	weight, err = rewritten.CreateWeight(searcher, search.COMPLETE_NO_SCORES, 1.0)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	supplier, err = weight.ScorerSupplier(lrc)
	if err != nil {
		t.Fatalf("scorerSupplier: %v", err)
	}
	if got := supplier.Cost(); got != 3000 { // Too many terms, assume worst-case all terms match
		t.Fatalf("cost: expected 3000, got %d", got)
	}

	mustClose(t, reader, dir)
}

// A leading wildcard is an automaton MultiTermQuery with an unknown term
// count (getTermsCount() == -1). Building its ScorerSupplier must not scan
// the term dictionary -- that is the cheap "planning" phase, and a leading
// wildcard such as "*foo*" cannot seek, so collecting terms there would walk
// the whole dictionary. The scan must be deferred to ScorerSupplier#get(), so
// a parent conjunction can short-circuit (a sibling clause matching no
// documents) before it runs.
func TestWildcardQueryScorerSupplierDoesNotScanTermsEagerly(t *testing.T) {
	dir := newDirectory()
	writer := newRandomIndexWriter(t, dir)
	for i := 0; i < 1000; i++ {
		doc := newTestDocument(newStringField(t, "body", "foo "+strconv.Itoa(i), false))
		mustAddDocument(t, writer, doc)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	// The private NextCountingReaderWrapper extends FilterDirectoryReader with
	// a SubReaderWrapper that wraps every leaf in a FilterLeafReader counting
	// TermsEnum.next() calls.
	defer mustClose(t, dir)
	t.Fatal(filterDirectoryReaderSubReaderWrapperBlocker)
}
