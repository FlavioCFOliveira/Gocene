// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestDisjunctionMaxQuery.java
// (Apache Lucene 10.5.0): test of the DisjunctionMaxQuery.
//
// @LuceneTestCase.SuppressCodecs("SimpleText"): the default codec is used.

package search_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	testutil "github.com/FlavioCFOliveira/Gocene/tests/util"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// dmqScoreCompThresh renders SCORE_COMP_THRESH: threshold for comparing floats.
const dmqScoreCompThresh = float32(0.0000)

// dmqTestSimilarityProvider renders the overrides of the private
// TestSimilarity, a ClassicSimilarity subclass that eliminates tf, idf and
// lengthNorm effects to isolate the test case (same as TestRankingSimilarity
// in TestRanking.zip from http://issues.apache.org/jira/browse/LUCENE-323).
type dmqTestSimilarityProvider struct{}

func (dmqTestSimilarityProvider) Tf(freq float32) float32 {
	if freq > 0.0 {
		return 1.0
	}
	return 0.0
}

// LengthNorm disables length norm.
func (dmqTestSimilarityProvider) LengthNorm(length int) float32 { return 1 }

func (dmqTestSimilarityProvider) Idf(docFreq, docCount int64) float32 { return 1.0 }

// newDMQTestSimilarity renders new TestSimilarity().
func newDMQTestSimilarity() *search.ClassicSimilarity {
	return &search.ClassicSimilarity{TFIDFSimilarity: search.NewTFIDFSimilarity(dmqTestSimilarityProvider{}, true)}
}

// dmqNonAnalyzedType renders the static nonAnalyzedType:
// new FieldType(TextField.TYPE_STORED) with setTokenized(false).
func dmqNonAnalyzedType() *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	ft.SetTokenized(false)
	return ft
}

// dmqFixture holds the fields of TestDisjunctionMaxQuery.
type dmqFixture struct {
	sim   search.Similarity
	index store.Directory
	r     index.LeafReader
	s     *search.IndexSearcher
}

// setUpDMQ renders TestDisjunctionMaxQuery.setUp(); tearDown is registered
// with t.Cleanup.
func setUpDMQ(t *testing.T) *dmqFixture {
	t.Helper()
	f := &dmqFixture{sim: newDMQTestSimilarity()}
	nonAnalyzedType := dmqNonAnalyzedType()

	f.index = newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetSimilarity(f.sim)
	iwc.SetMergePolicy(newLogMergePolicy())
	writer := newRandomIndexWriterWithConfig(t, f.index, iwc)

	// hed is the most important field, dek is secondary

	// d1 is an "ok" match for: albino elephant
	{
		d1 := document.NewDocument()
		d1.Add(newField(t, "id", "d1", nonAnalyzedType)) // Field.Keyword("id", "d1"));
		d1.Add(newTextField(t, "hed", "elephant", true)) // Field.Text("hed", "elephant"));
		d1.Add(newTextField(t, "dek", "elephant", true)) // Field.Text("dek", "elephant"));
		mustAddDocument(t, writer, d1)
	}

	// d2 is a "good" match for: albino elephant
	{
		d2 := document.NewDocument()
		d2.Add(newField(t, "id", "d2", nonAnalyzedType)) // Field.Keyword("id", "d2"));
		d2.Add(newTextField(t, "hed", "elephant", true)) // Field.Text("hed", "elephant"));
		d2.Add(newTextField(t, "dek", "albino", true))   // Field.Text("dek", "albino"));
		d2.Add(newTextField(t, "dek", "elephant", true)) // Field.Text("dek", "elephant"));
		mustAddDocument(t, writer, d2)
	}

	// d3 is a "better" match for: albino elephant
	{
		d3 := document.NewDocument()
		d3.Add(newField(t, "id", "d3", nonAnalyzedType)) // Field.Keyword("id", "d3"));
		d3.Add(newTextField(t, "hed", "albino", true))   // Field.Text("hed", "albino"));
		d3.Add(newTextField(t, "hed", "elephant", true)) // Field.Text("hed", "elephant"));
		mustAddDocument(t, writer, d3)
	}

	// d4 is the "best" match for: albino elephant
	{
		d4 := document.NewDocument()
		d4.Add(newField(t, "id", "d4", nonAnalyzedType))        // Field.Keyword("id", "d4"));
		d4.Add(newTextField(t, "hed", "albino", true))          // Field.Text("hed", "albino"));
		d4.Add(newField(t, "hed", "elephant", nonAnalyzedType)) // Field.Text("hed", "elephant"));
		d4.Add(newTextField(t, "dek", "albino", true))          // Field.Text("dek", "albino"));
		mustAddDocument(t, writer, d4)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	f.r = testutil.GetOnlyLeafReader(mustGetReader(t, writer))
	mustClose(t, writer)
	t.Cleanup(func() {
		if err := f.r.Close(); err != nil {
			t.Errorf("close reader: %v", err)
		}
		if err := f.index.Close(); err != nil {
			t.Errorf("close index: %v", err)
		}
	})
	f.s = search.NewIndexSearcher(f.r)
	f.s.SetSimilarity(f.sim)
	return f
}

// topLeafContext renders (LeafReaderContext) s.getTopReaderContext(), after
// assertTrue(s.getTopReaderContext() instanceof LeafReaderContext).
func (f *dmqFixture) topLeafContext(t *testing.T) *index.LeafReaderContext {
	t.Helper()
	context, ok := f.s.GetTopReaderContext().(*index.LeafReaderContext)
	if !ok {
		t.Fatalf("top reader context is %T, not a LeafReaderContext", f.s.GetTopReaderContext())
	}
	return context
}

// storedID renders storedFields().document(doc).get("id").
func (f *dmqFixture) storedID(t *testing.T, sf index.StoredFields, doc int) string {
	t.Helper()
	return storedDocument(t, sf, doc).GetString("id")
}

func TestDisjunctionMaxQuerySkipToFirsttimeMiss(t *testing.T) {
	f := setUpDMQ(t)
	dq := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("id", "d1"), dmqTq("dek", "DOES_NOT_EXIST")}, 0.0)

	queryUtilsCheckSearcher(t, dq, f.s)
	context := f.topLeafContext(t)
	dw, err := f.s.CreateWeight(mustRewrite(t, f.s, dq), search.COMPLETE, 1)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	ds, err := dw.Scorer(context)
	if err != nil {
		t.Fatalf("scorer: %v", err)
	}
	doc, err := ds.Iterator().Advance(3)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	skipOk := doc != util.NO_MORE_DOCS
	if skipOk {
		t.Fatalf("firsttime skipTo found a match? ... %s", f.storedID(t, mustStoredFields(t, f.r), ds.DocID()))
	}
}

func TestDisjunctionMaxQuerySkipToFirsttimeHit(t *testing.T) {
	f := setUpDMQ(t)
	dq := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("dek", "albino"), dmqTq("dek", "DOES_NOT_EXIST")}, 0.0)

	context := f.topLeafContext(t)
	queryUtilsCheckSearcher(t, dq, f.s)
	dw, err := f.s.CreateWeight(mustRewrite(t, f.s, dq), search.COMPLETE, 1)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	ds, err := dw.Scorer(context)
	if err != nil {
		t.Fatalf("scorer: %v", err)
	}
	doc, err := ds.Iterator().Advance(3)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if doc == util.NO_MORE_DOCS {
		t.Fatal("firsttime skipTo found no match")
	}
	if got := f.storedID(t, mustStoredFields(t, f.r), ds.DocID()); got != "d4" {
		t.Fatalf("found wrong docid: %s", got)
	}
}

// assertDMQEqualScores asserts that every hit scores like the first one.
func assertDMQEqualScores(t *testing.T, h []*search.ScoreDoc, from, to int) {
	t.Helper()
	score := h[0].Score
	for i := from; i < to; i++ {
		if diff := score - h[i].Score; diff > dmqScoreCompThresh || diff < -dmqScoreCompThresh {
			t.Fatalf("score #%d is not the same: expected %v, got %v", i, score, h[i].Score)
		}
	}
}

func TestDisjunctionMaxQuerySimpleEqualScores1(t *testing.T) {
	f := setUpDMQ(t)
	q := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("hed", "albino"), dmqTq("hed", "elephant")}, 0.0)
	queryUtilsCheckSearcher(t, q, f.s)

	h := mustSearch(t, f.s, q, 1000).ScoreDocs

	defer printDMQHitsOnFailure(t, "testSimpleEqualScores1", h, f.s)
	if len(h) != 4 {
		t.Fatalf("all docs should match %s: got %d", q.ToString(""), len(h))
	}
	assertDMQEqualScores(t, h, 1, len(h))
}

func TestDisjunctionMaxQuerySimpleEqualScores2(t *testing.T) {
	f := setUpDMQ(t)
	q := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("dek", "albino"), dmqTq("dek", "elephant")}, 0.0)
	queryUtilsCheckSearcher(t, q, f.s)

	h := mustSearch(t, f.s, q, 1000).ScoreDocs

	defer printDMQHitsOnFailure(t, "testSimpleEqualScores2", h, f.s)
	if len(h) != 3 {
		t.Fatalf("3 docs should match %s: got %d", q.ToString(""), len(h))
	}
	assertDMQEqualScores(t, h, 1, len(h))
}

func TestDisjunctionMaxQuerySimpleEqualScores3(t *testing.T) {
	f := setUpDMQ(t)
	q := search.NewDisjunctionMaxQuery(
		[]search.Query{
			dmqTq("hed", "albino"),
			dmqTq("hed", "elephant"),
			dmqTq("dek", "albino"),
			dmqTq("dek", "elephant"),
		},
		0.0)
	queryUtilsCheckSearcher(t, q, f.s)

	h := mustSearch(t, f.s, q, 1000).ScoreDocs

	defer printDMQHitsOnFailure(t, "testSimpleEqualScores3", h, f.s)
	if len(h) != 4 {
		t.Fatalf("all docs should match %s: got %d", q.ToString(""), len(h))
	}
	assertDMQEqualScores(t, h, 1, len(h))
}

func TestDisjunctionMaxQuerySimpleTiebreaker(t *testing.T) {
	f := setUpDMQ(t)
	q := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("dek", "albino"), dmqTq("dek", "elephant")}, 0.01)
	queryUtilsCheckSearcher(t, q, f.s)

	h := mustSearch(t, f.s, q, 1000).ScoreDocs

	defer printDMQHitsOnFailure(t, "testSimpleTiebreaker", h, f.s)
	if len(h) != 3 {
		t.Fatalf("3 docs should match %s: got %d", q.ToString(""), len(h))
	}
	sf := mustStoredFields(t, f.s)
	if got := f.storedID(t, sf, h[0].Doc); got != "d2" {
		t.Fatalf("wrong first: %s", got)
	}
	score0 := h[0].Score
	score1 := h[1].Score
	score2 := h[2].Score
	if !(score0 > score1) {
		t.Fatalf("d2 does not have better score then others: %v >? %v", score0, score1)
	}
	if diff := score1 - score2; diff > dmqScoreCompThresh || diff < -dmqScoreCompThresh {
		t.Fatalf("d4 and d1 don't have equal scores: %v vs %v", score1, score2)
	}
}

func TestDisjunctionMaxQueryBooleanRequiredEqualScores(t *testing.T) {
	f := setUpDMQ(t)
	q := search.NewBooleanQueryBuilder()
	{
		q1 := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("hed", "albino"), dmqTq("dek", "albino")}, 0.0)
		q.Add(q1, search.MUST) // true,false);
		queryUtilsCheckSearcher(t, q1, f.s)
	}
	{
		q2 := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("hed", "elephant"), dmqTq("dek", "elephant")}, 0.0)
		q.Add(q2, search.MUST) // true,false);
		queryUtilsCheckSearcher(t, q2, f.s)
	}

	queryUtilsCheckSearcher(t, q.Build(), f.s)

	h := mustSearch(t, f.s, q.Build(), 1000).ScoreDocs

	defer printDMQHitsOnFailure(t, "testBooleanRequiredEqualScores1", h, f.s)
	if len(h) != 3 {
		t.Fatalf("3 docs should match: got %d", len(h))
	}
	assertDMQEqualScores(t, h, 1, len(h))
}

func TestDisjunctionMaxQueryBooleanOptionalNoTiebreaker(t *testing.T) {
	f := setUpDMQ(t)
	q := search.NewBooleanQueryBuilder()
	{
		q1 := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("hed", "albino"), dmqTq("dek", "albino")}, 0.0)
		q.Add(q1, search.SHOULD) // false,false);
	}
	{
		q2 := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("hed", "elephant"), dmqTq("dek", "elephant")}, 0.0)
		q.Add(q2, search.SHOULD) // false,false);
	}
	queryUtilsCheckSearcher(t, q.Build(), f.s)

	h := mustSearch(t, f.s, q.Build(), 1000).ScoreDocs

	defer printDMQHitsOnFailure(t, "testBooleanOptionalNoTiebreaker", h, f.s)
	if len(h) != 4 {
		t.Fatalf("4 docs should match: got %d", len(h))
	}
	score := h[0].Score
	assertDMQEqualScores(t, h, 1, len(h)-1) /* note: -1 */
	if got := f.storedID(t, mustStoredFields(t, f.s), h[len(h)-1].Doc); got != "d1" {
		t.Fatalf("wrong last: %s", got)
	}
	score1 := h[len(h)-1].Score
	if !(score > score1) {
		t.Fatalf("d1 does not have worse score then others: %v >? %v", score, score1)
	}
}

func TestDisjunctionMaxQueryBooleanOptionalWithTiebreaker(t *testing.T) {
	f := setUpDMQ(t)
	q := search.NewBooleanQueryBuilder()
	{
		q1 := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("hed", "albino"), dmqTq("dek", "albino")}, 0.01)
		q.Add(q1, search.SHOULD) // false,false);
	}
	{
		q2 := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("hed", "elephant"), dmqTq("dek", "elephant")}, 0.01)
		q.Add(q2, search.SHOULD) // false,false);
	}
	queryUtilsCheckSearcher(t, q.Build(), f.s)

	h := mustSearch(t, f.s, q.Build(), 1000).ScoreDocs

	defer printDMQHitsOnFailure(t, "testBooleanOptionalWithTiebreaker", h, f.s)

	if len(h) != 4 {
		t.Fatalf("4 docs should match: got %d", len(h))
	}

	score0 := h[0].Score
	score1 := h[1].Score
	score2 := h[2].Score
	score3 := h[3].Score

	sf := mustStoredFields(t, f.s)
	doc0 := f.storedID(t, sf, h[0].Doc)
	doc1 := f.storedID(t, sf, h[1].Doc)
	doc2 := f.storedID(t, sf, h[2].Doc)
	doc3 := f.storedID(t, sf, h[3].Doc)

	if !(doc0 == "d2" || doc0 == "d4") {
		t.Fatalf("doc0 should be d2 or d4: %s", doc0)
	}
	if !(doc1 == "d2" || doc1 == "d4") {
		t.Fatalf("doc1 should be d2 or d4: %s", doc0)
	}
	if diff := score0 - score1; diff > dmqScoreCompThresh || diff < -dmqScoreCompThresh {
		t.Fatalf("score0 and score1 should match: %v vs %v", score0, score1)
	}
	if doc2 != "d3" {
		t.Fatalf("wrong third: %s", doc2)
	}
	if !(score1 > score2) {
		t.Fatalf("d3 does not have worse score then d2 and d4: %v >? %v", score1, score2)
	}

	if doc3 != "d1" {
		t.Fatalf("wrong fourth: %s", doc3)
	}
	if !(score2 > score3) {
		t.Fatalf("d1 does not have worse score then d3: %v >? %v", score2, score3)
	}
}

func TestDisjunctionMaxQueryBooleanOptionalWithTiebreakerAndBoost(t *testing.T) {
	f := setUpDMQ(t)
	q := search.NewBooleanQueryBuilder()
	{
		q1 := search.NewDisjunctionMaxQuery(
			[]search.Query{dmqTqBoost("hed", "albino", 1.5), dmqTq("dek", "albino")}, 0.01)
		q.Add(q1, search.SHOULD) // false,false);
	}
	{
		q2 := search.NewDisjunctionMaxQuery(
			[]search.Query{dmqTqBoost("hed", "elephant", 1.5), dmqTq("dek", "elephant")}, 0.01)
		q.Add(q2, search.SHOULD) // false,false);
	}
	queryUtilsCheckSearcher(t, q.Build(), f.s)

	h := mustSearch(t, f.s, q.Build(), 1000).ScoreDocs

	defer printDMQHitsOnFailure(t, "testBooleanOptionalWithTiebreakerAndBoost", h, f.s)

	if len(h) != 4 {
		t.Fatalf("4 docs should match: got %d", len(h))
	}

	score0 := h[0].Score
	score1 := h[1].Score
	score2 := h[2].Score
	score3 := h[3].Score

	sf := mustStoredFields(t, f.s)
	doc0 := f.storedID(t, sf, h[0].Doc)
	doc1 := f.storedID(t, sf, h[1].Doc)
	doc2 := f.storedID(t, sf, h[2].Doc)
	doc3 := f.storedID(t, sf, h[3].Doc)

	for _, c := range []struct{ want, got, msg string }{
		{"d4", doc0, "doc0 should be d4: "},
		{"d3", doc1, "doc1 should be d3: "},
		{"d2", doc2, "doc2 should be d2: "},
		{"d1", doc3, "doc3 should be d1: "},
	} {
		if c.got != c.want {
			t.Fatalf("%s expected %s, got %s", c.msg, c.want, c.got)
		}
	}

	if !(score0 > score1) {
		t.Fatalf("d4 does not have a better score then d3: %v >? %v", score0, score1)
	}
	if !(score1 > score2) {
		t.Fatalf("d3 does not have a better score then d2: %v >? %v", score1, score2)
	}
	if !(score2 > score3) {
		t.Fatalf("d3 does not have a better score then d1: %v >? %v", score2, score3)
	}
}

func TestDisjunctionMaxQueryRewriteBoolean(t *testing.T) {
	f := setUpDMQ(t)
	sub1 := dmqTq("hed", "albino")
	sub2 := dmqTq("hed", "elephant")
	q := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2}, 1.0)
	rewritten := mustRewrite(t, f.s, q)
	expected := search.NewBooleanQueryBuilder().
		Add(sub1, search.SHOULD).
		Add(sub2, search.SHOULD).
		Build()
	if !expected.Equals(rewritten) {
		t.Fatalf("expected %v, got %v", expected, rewritten)
	}
}

func TestDisjunctionMaxQueryRewriteEmpty(t *testing.T) {
	f := setUpDMQ(t)
	q := search.NewDisjunctionMaxQuery([]search.Query{}, 0.0)
	rewritten := mustRewrite(t, f.s, q)
	expected := search.MatchNoDocsQueryInstance
	if !expected.Equals(rewritten) {
		t.Fatalf("expected %v, got %v", expected, rewritten)
	}
}

func TestDisjunctionMaxQueryRewriteAllMatchNoDocs(t *testing.T) {
	f := setUpDMQ(t)
	// All disjuncts rewrite to MatchNoDocsQuery -> should return MatchNoDocsQuery
	sub1 := search.NewMatchNoDocsQuery("test1")
	sub2 := search.NewMatchNoDocsQuery("test2")
	q := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2}, 0.0)
	rewritten := mustRewrite(t, f.s, q)
	if !search.MatchNoDocsQueryInstance.Equals(rewritten) {
		t.Fatalf("expected MatchNoDocsQuery.INSTANCE, got %v", rewritten)
	}
}

func TestDisjunctionMaxQueryRewriteSingleSurvivor(t *testing.T) {
	f := setUpDMQ(t)
	// One disjunct rewrites to MatchNoDocsQuery, other matches -> should return the matching one
	sub := dmqTq("hed", "albino")
	q := search.NewDisjunctionMaxQuery([]search.Query{sub, search.NewMatchNoDocsQuery("test")}, 0.0)
	rewritten := mustRewrite(t, f.s, q)
	if !sub.Equals(rewritten) {
		t.Fatalf("expected %v, got %v", sub, rewritten)
	}
}

func TestDisjunctionMaxQueryRewriteFilterMatchNoDocs(t *testing.T) {
	f := setUpDMQ(t)
	// Mixed: some MatchNoDocsQuery, some real queries -> should filter out MatchNoDocsQuery
	sub1 := dmqTq("hed", "albino")
	sub2 := dmqTq("hed", "elephant")
	q := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2, search.NewMatchNoDocsQuery("test")}, 0.0)
	rewritten := mustRewrite(t, f.s, q)
	expected := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2}, 0.0)
	if !expected.Equals(rewritten) {
		t.Fatalf("expected %v, got %v", expected, rewritten)
	}
}

func TestDisjunctionMaxQueryRewriteFilterMatchNoDocsWithTieBreaker(t *testing.T) {
	f := setUpDMQ(t)
	// Verify tie breaker multiplier is preserved when filtering MatchNoDocsQuery
	sub1 := dmqTq("hed", "albino")
	sub2 := dmqTq("hed", "elephant")
	q := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2, search.NewMatchNoDocsQuery("test")}, 0.1)
	rewritten := mustRewrite(t, f.s, q)
	expected := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2}, 0.1)
	if !expected.Equals(rewritten) {
		t.Fatalf("expected %v, got %v", expected, rewritten)
	}
}

func TestDisjunctionMaxQueryDisjunctOrderAndEquals(t *testing.T) {
	setUpDMQ(t)
	// the order that disjuncts are provided in should not matter for equals() comparisons
	sub1 := dmqTq("hed", "albino")
	sub2 := dmqTq("hed", "elephant")
	q1 := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2}, 1.0)
	q2 := search.NewDisjunctionMaxQuery([]search.Query{sub2, sub1}, 1.0)
	if !q1.Equals(q2) {
		t.Fatalf("expected %v to equal %v", q1, q2)
	}
}

// Inspired from TestIntervals.testIntervalDisjunctionToStringStability.
func TestDisjunctionMaxQueryCasesWhenDisjunctOrderMatters(t *testing.T) {
	setUpDMQ(t)
	clauseNbr := random().Intn(22) + 4 // ensure a reasonably large minimum number of clauses
	terms := make([]string, clauseNbr)
	for i := 0; i < clauseNbr; i++ {
		terms[i] = string(rune('a' + i))
	}

	parts := make([]string, len(terms))
	for i, term := range terms {
		parts[i] = "test:" + term
	}
	expected := "(" + strings.Join(parts, " | ") + ")~1.0"

	clauses := make([]search.Query, len(terms))
	for i, term := range terms {
		clauses[i] = dmqTq("test", term)
	}
	source := search.NewDisjunctionMaxQuery(clauses, 1.0)

	if got := source.ToString(""); got != expected {
		t.Fatalf("toString: expected %q, got %q", expected, got)
	}
	disjuncts := source.Disjuncts()
	if len(disjuncts) != len(terms) {
		t.Fatalf("expected %d disjuncts, got %d", len(terms), len(disjuncts))
	}
	i := 0
	for _, query := range disjuncts {
		if got := query.(*search.TermQuery).GetTerm().Text(); got != terms[i] {
			t.Fatalf("disjunct %d: expected %s, got %s", i, terms[i], got)
		}
		i++
	}
}

func TestDisjunctionMaxQueryRandomTopDocs(t *testing.T) {
	setUpDMQ(t)
	doTestDMQRandomTopDocs(t, 2, 0.05, 0.05)
	doTestDMQRandomTopDocs(t, 2, 1.0, 0.05)
	doTestDMQRandomTopDocs(t, 3, 1.0, 0.5, 0.05)
	doTestDMQRandomTopDocs(t, 4, 1.0, 0.5, 0.05, 0)
	doTestDMQRandomTopDocs(t, 4, 1.0, 0.5, 0.05, 0)
}

func dmqExplain(t *testing.T, f *dmqFixture, dq search.Query) search.Explanation {
	t.Helper()
	dw, err := f.s.CreateWeight(mustRewrite(t, f.s, dq), search.COMPLETE, 1)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	context := f.s.GetTopReaderContext().(*index.LeafReaderContext)
	explanation, err := dw.Explain(context, 1)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	return explanation
}

func TestDisjunctionMaxQueryExplainMatch(t *testing.T) {
	f := setUpDMQ(t)
	// Both match
	sub1 := dmqTq("hed", "elephant")
	sub2 := dmqTq("dek", "elephant")

	dq := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2}, 0.0)

	explanation := dmqExplain(t, f, dq)

	if got := explanation.GetDescription(); got != "max of:" {
		t.Fatalf("description: expected %q, got %q", "max of:", got)
	}
	// Two matching sub queries should be included in the explanation details
	if got := len(explanation.GetDetails()); got != 2 {
		t.Fatalf("details: expected 2, got %d", got)
	}
}

func TestDisjunctionMaxQueryExplainNoMatch(t *testing.T) {
	f := setUpDMQ(t)
	// No match
	sub1 := dmqTq("abc", "elephant")
	sub2 := dmqTq("def", "elephant")

	dq := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2}, 0.0)

	explanation := dmqExplain(t, f, dq)

	if got := explanation.GetDescription(); got != "No matching clause" {
		t.Fatalf("description: expected %q, got %q", "No matching clause", got)
	}
	// Two non-matching sub queries should be included in the explanation details
	if got := len(explanation.GetDetails()); got != 2 {
		t.Fatalf("details: expected 2, got %d", got)
	}
}

func TestDisjunctionMaxQueryExplainMatch_OneNonMatchingSubQuery_NotIncludedInExplanation(t *testing.T) {
	f := setUpDMQ(t)
	// Matches
	sub1 := dmqTq("hed", "elephant")

	// Doesn't match
	sub2 := dmqTq("def", "elephant")

	dq := search.NewDisjunctionMaxQuery([]search.Query{sub1, sub2}, 0.0)

	explanation := dmqExplain(t, f, dq)

	if got := explanation.GetDescription(); got != "max of:" {
		t.Fatalf("description: expected %q, got %q", "max of:", got)
	}
	// Only the matching sub query (sub1) should be included in the explanation details
	if got := len(explanation.GetDetails()); got != 1 {
		t.Fatalf("details: expected 1, got %d", got)
	}
}

// doTestDMQRandomTopDocs renders the private doTestRandomTopDocs(int, double...).
func doTestDMQRandomTopDocs(t *testing.T, numFields int, freqs ...float64) {
	t.Helper()
	if util.AssertsEnabled() && !(numFields == len(freqs)) {
		panic(util.NewAssertionError(nil))
	}
	dir := newDirectory()
	config := index.NewIndexWriterConfigWithAnalyzer(analysis.NewStandardAnalyzer())
	w := mustNewIndexWriter(t, dir, config)

	numDocs := atLeast(100) // TEST_NIGHTLY is false; at night, make sure some terms have skip data
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		for j := 0; j < numFields; j++ {
			var builder strings.Builder
			numAs := 0
			if !(random().Float64() < freqs[j]) {
				numAs = 1 + random().Intn(5)
			}
			for k := 0; k < numAs; k++ {
				if builder.Len() > 0 {
					builder.WriteByte(' ')
				}
				builder.WriteByte('a')
			}
			if random().Intn(2) == 0 {
				doc.Add(mustStringField(t, "field", "c", false))
			}
			numOthers := 0
			if random().Intn(2) != 0 {
				numOthers = 1 + random().Intn(5)
			}
			for k := 0; k < numOthers; k++ {
				if builder.Len() > 0 {
					builder.WriteByte(' ')
				}
				builder.WriteString(strconv.Itoa(int(int32(random().Uint32())))) // Integer.toString(random().nextInt())
			}
			tf, err := document.NewTextFieldFromReader(strconv.Itoa(j), strings.NewReader(builder.String()))
			if err != nil {
				t.Fatalf("new TextField: %v", err)
			}
			doc.Add(tf)
		}
		mustAddDocument(t, w, doc)
	}
	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	mustClose(t, w)
	searcher := newSearcher(t, reader)
	for i := 0; i < 4; i++ {
		clauses := []search.Query{}
		for j := 0; j < numFields; j++ {
			if i%2 == 1 {
				clauses = append(clauses, dmqTq(strconv.Itoa(j), "a"))
			} else {
				var boost float32
				if random().Intn(2) != 0 {
					boost = random().Float32()
				}
				if boost > 0 {
					clauses = append(clauses, dmqTqBoost(strconv.Itoa(j), "a", boost))
				} else {
					clauses = append(clauses, dmqTq(strconv.Itoa(j), "a"))
				}
			}
		}
		tieBreaker := random().Float32()
		var query search.Query = search.NewDisjunctionMaxQuery(clauses, tieBreaker)
		testsearch.CheckTopScores(t, random(), query, searcher)

		query = search.NewBooleanQueryBuilder().
			Add(search.NewDisjunctionMaxQuery(clauses, tieBreaker), search.MUST).
			Add(dmqTq("field", "c"), search.FILTER).
			Build()
		testsearch.CheckTopScores(t, random(), query, searcher)
	}
	mustClose(t, reader, dir)
}

// Ensure generics and type inference play nicely together.
func TestDisjunctionMaxQueryGenerics(t *testing.T) {
	setUpDMQ(t)
	query := search.NewDisjunctionMaxQuery([]search.Query{dmqTq("test", "term")}, 1.0)
	if got := len(query.Disjuncts()); got != 1 {
		t.Fatalf("expected 1 disjunct, got %d", got)
	}

	disjuncts := []search.Query{
		search.NewRegexpQuery(index.NewTerm("field", "foobar")),
		search.NewWildcardQuery(index.NewTerm("field", "foobar")),
	}
	query = search.NewDisjunctionMaxQuery(disjuncts, 1.0)
	if got := len(query.Disjuncts()); got != 2 {
		t.Fatalf("expected 2 disjuncts, got %d", got)
	}
}

// dmqTq renders the macro tq(String, String).
func dmqTq(f, t string) *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(f, t))
}

// dmqTqBoost renders the macro tq(String, String, float).
func dmqTqBoost(f, t string, b float32) *search.BoostQuery {
	q := dmqTq(f, t)
	return search.NewBoostQuery(q, b)
}

// printDMQHitsOnFailure renders the catch (Error e) { printHits(...); throw e; }
// blocks: when the test has failed, the hits are printed to stderr.
func printDMQHitsOnFailure(t *testing.T, test string, h []*search.ScoreDoc, searcher *search.IndexSearcher) {
	if !t.Failed() {
		return
	}
	fmt.Fprintln(os.Stderr, "------- "+test+" -------")

	storedFields, err := searcher.StoredFields()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	for i := 0; i < len(h); i++ {
		visitor := document.NewDocumentStoredFieldVisitor()
		if err := storedFields.Document(h[i].Doc, visitor); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		d := visitor.GetDocument()
		score := h[i].Score
		fmt.Fprintf(os.Stderr, "#%d: %.9f - %s\n", i, score, d.GetString("id"))
	}
}
