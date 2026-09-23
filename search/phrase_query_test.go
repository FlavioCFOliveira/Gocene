// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestPhraseQuery.java
// (Apache Lucene 10.5.0): tests PhraseQuery (see TestPositionIncrement).
//
// The @BeforeClass index is rebuilt for every test (phraseQueryBeforeClass);
// every test reads it only, so the data each test sees is the same.

package search_test

import (
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// phraseScoreCompThresh renders SCORE_COMP_THRESH: threshold for comparing floats.
const phraseScoreCompThresh = float32(1e-6)

// phraseGapAnalyzer renders the anonymous Analyzer of beforeClass: a
// MockTokenizer(WHITESPACE, false) with a position increment gap of 100.
type phraseGapAnalyzer struct {
	*analysis.BaseAnalyzer
}

func (a *phraseGapAnalyzer) GetPositionIncrementGap(fieldName string) int { return 100 }

func newPhraseGapAnalyzer() *phraseGapAnalyzer {
	base := analysis.NewAnalyzer(nil)
	base.CreateComponents = func(fieldName string) *analysis.TokenStreamComponents {
		src := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, false, testanalysis.DefaultMaxTokenLength)
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				src.SetReader(r)
				return nil
			},
			Sink: src,
		}
	}
	return &phraseGapAnalyzer{BaseAnalyzer: base}
}

// phraseQueryClass holds the static fields of TestPhraseQuery.
type phraseQueryClass struct {
	searcher *search.IndexSearcher
	reader   *index.DirectoryReader
}

// phraseQueryBeforeClass renders beforeClass(); afterClass is registered with
// t.Cleanup.
func phraseQueryBeforeClass(t *testing.T) *phraseQueryClass {
	t.Helper()
	c := &phraseQueryClass{}
	directory := newDirectory()
	analyzer := newPhraseGapAnalyzer()
	writer := newRandomIndexWriterWithAnalyzer(t, directory, analyzer)

	doc := document.NewDocument()
	doc.Add(newTextField(t, "field", "one two three four five", true))
	doc.Add(newTextField(t, "repeated", "this is a repeated field - first part", true))
	repeatedField := newTextField(t, "repeated", "second part of a repeated field", true)
	doc.Add(repeatedField)
	doc.Add(newTextField(t, "palindrome", "one two three two one", true))
	mustAddDocument(t, writer, doc)

	doc = document.NewDocument()
	doc.Add(newTextField(t, "nonexist", "phrase exist notexist exist found", true))
	mustAddDocument(t, writer, doc)

	doc = document.NewDocument()
	doc.Add(newTextField(t, "nonexist", "phrase exist notexist exist found", true))
	mustAddDocument(t, writer, doc)

	c.reader = mustGetReader(t, writer)
	mustClose(t, writer)

	c.searcher = search.NewIndexSearcher(c.reader)
	t.Cleanup(func() {
		c.searcher = nil
		if err := c.reader.Close(); err != nil {
			t.Errorf("close reader: %v", err)
		}
		c.reader = nil
		if err := directory.Close(); err != nil {
			t.Errorf("close directory: %v", err)
		}
	})
	return c
}

func assertPhraseHitCount(t *testing.T, msg string, hits []*search.ScoreDoc, want int) {
	t.Helper()
	if len(hits) != want {
		t.Fatalf("%s: expected %d hits, got %d", msg, want, len(hits))
	}
}

func TestPhraseQueryNotCloseEnough(t *testing.T) {
	c := phraseQueryBeforeClass(t)
	query := search.NewPhraseQuery(2, "field", "one", "five")
	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 0)
	queryUtilsCheckSearcher(t, query, c.searcher)
}

func TestPhraseQueryBarelyCloseEnough(t *testing.T) {
	c := phraseQueryBeforeClass(t)
	query := search.NewPhraseQuery(3, "field", "one", "five")
	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)
}

// Ensures slop of 0 works for exact matches, but not reversed.
func TestPhraseQueryExact(t *testing.T) {
	c := phraseQueryBeforeClass(t)
	// slop is zero by default
	query := search.NewPhraseQuery(0, "field", "four", "five")
	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "exact match", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	query = search.NewPhraseQuery(0, "field", "two", "one")
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "reverse not exact", hits, 0)
	queryUtilsCheckSearcher(t, query, c.searcher)
}

func TestPhraseQuerySlop1(t *testing.T) {
	c := phraseQueryBeforeClass(t)
	// Ensures slop of 1 works with terms in order.
	query := search.NewPhraseQuery(1, "field", "one", "two")
	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "in order", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// Ensures slop of 1 does not work for phrases out of order;
	// must be at least 2.
	query = search.NewPhraseQuery(1, "field", "two", "one")
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "reversed, slop not 2 or more", hits, 0)
	queryUtilsCheckSearcher(t, query, c.searcher)
}

// As long as slop is at least 2, terms can be reversed.
func TestPhraseQueryOrderDoesntMatter(t *testing.T) {
	c := phraseQueryBeforeClass(t)
	// must be at least two for reverse order match
	query := search.NewPhraseQuery(2, "field", "two", "one")
	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "just sloppy enough", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	query = search.NewPhraseQuery(2, "field", "three", "one")
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "not sloppy enough", hits, 0)
	queryUtilsCheckSearcher(t, query, c.searcher)
}

// slop is the total number of positional moves allowed to line up a phrase.
func TestPhraseQueryMultipleTerms(t *testing.T) {
	c := phraseQueryBeforeClass(t)
	query := search.NewPhraseQuery(2, "field", "one", "three", "five")
	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "two total moves", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// it takes six moves to match this phrase
	query = search.NewPhraseQuery(5, "field", "five", "three", "one")
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "slop of 5 not close enough", hits, 0)
	queryUtilsCheckSearcher(t, query, c.searcher)

	query = search.NewPhraseQuery(6, "field", "five", "three", "one")
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "slop of 6 just right", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)
}

func TestPhraseQueryPhraseQueryWithStopAnalyzer(t *testing.T) {
	directory := newDirectory()
	stopAnalyzer := testanalysis.NewMockAnalyzer(testanalysis.SIMPLE, true, 0, testanalysis.ENGLISH_STOPSET, true)
	writer := newRandomIndexWriterWithConfig(t, directory, newIndexWriterConfigWithAnalyzer(stopAnalyzer))
	doc := newTestDocument(newTextField(t, "field", "the stop words are here", true))
	mustAddDocument(t, writer, doc)
	reader := mustGetReader(t, writer)
	mustClose(t, writer)

	searcher := newSearcher(t, reader)

	// valid exact phrase query
	query := search.NewPhraseQuery(0, "field", "stop", "words")
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 1)
	queryUtilsCheckSearcher(t, query, searcher)

	mustClose(t, reader, directory)
}

func TestPhraseQueryPhraseQueryInConjunctionScorer(t *testing.T) {
	directory := newDirectory()
	writer := newRandomIndexWriter(t, directory)

	doc := newTestDocument(newTextField(t, "source", "marketing info", true))
	mustAddDocument(t, writer, doc)

	doc = newTestDocument(
		newTextField(t, "contents", "foobar", true),
		newTextField(t, "source", "marketing info", true))
	mustAddDocument(t, writer, doc)

	reader := mustGetReader(t, writer)
	mustClose(t, writer)

	searcher := newSearcher(t, reader)

	phraseQuery := search.NewPhraseQuery(0, "source", "marketing", "info")
	hits := mustSearch(t, searcher, phraseQuery, 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 2)
	queryUtilsCheckSearcher(t, phraseQuery, searcher)

	termQuery := search.NewTermQuery(index.NewTerm("contents", "foobar"))
	booleanQuery := search.NewBooleanQueryBuilder()
	booleanQuery.Add(termQuery, search.MUST)
	booleanQuery.Add(phraseQuery, search.MUST)
	hits = mustSearch(t, searcher, booleanQuery.Build(), 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 1)
	queryUtilsCheckSearcher(t, termQuery, searcher)

	mustClose(t, reader)

	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer()).SetOpenMode(index.Create)
	writer = newRandomIndexWriterWithConfig(t, directory, iwc)
	doc = newTestDocument(newTextField(t, "contents", "map entry woo", true))
	mustAddDocument(t, writer, doc)

	doc = newTestDocument(newTextField(t, "contents", "woo map entry", true))
	mustAddDocument(t, writer, doc)

	doc = newTestDocument(newTextField(t, "contents", "map foobarword entry woo", true))
	mustAddDocument(t, writer, doc)

	reader = mustGetReader(t, writer)
	mustClose(t, writer)

	searcher = newSearcher(t, reader)

	termQuery = search.NewTermQuery(index.NewTerm("contents", "woo"))
	phraseQuery = search.NewPhraseQuery(0, "contents", "map", "entry")

	hits = mustSearch(t, searcher, termQuery, 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 3)
	hits = mustSearch(t, searcher, phraseQuery, 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 2)

	booleanQuery = search.NewBooleanQueryBuilder()
	booleanQuery.Add(termQuery, search.MUST)
	booleanQuery.Add(phraseQuery, search.MUST)
	hits = mustSearch(t, searcher, booleanQuery.Build(), 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 2)

	booleanQuery = search.NewBooleanQueryBuilder()
	booleanQuery.Add(phraseQuery, search.MUST)
	booleanQuery.Add(termQuery, search.MUST)
	hits = mustSearch(t, searcher, booleanQuery.Build(), 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 2)
	queryUtilsCheckSearcher(t, booleanQuery.Build(), searcher)

	mustClose(t, reader, directory)
}

func TestPhraseQuerySlopScoring(t *testing.T) {
	directory := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	iwc.SetSimilarity(search.NewLuceneBM25Similarity())
	writer := newRandomIndexWriterWithConfig(t, directory, iwc)

	doc := newTestDocument(newTextField(t, "field", "foo firstname lastname foo", true))
	mustAddDocument(t, writer, doc)

	doc2 := newTestDocument(newTextField(t, "field", "foo firstname zzz lastname foo", true))
	mustAddDocument(t, writer, doc2)

	doc3 := newTestDocument(newTextField(t, "field", "foo firstname zzz yyy lastname foo", true))
	mustAddDocument(t, writer, doc3)

	reader := mustGetReader(t, writer)
	mustClose(t, writer)

	searcher := newSearcher(t, reader)
	searcher.SetSimilarity(search.NewClassicSimilarity())
	query := search.NewPhraseQuery(math.MaxInt32, "field", "firstname", "lastname")
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "", hits, 3)
	// Make sure that those matches where the terms appear closer to
	// each other get a higher score:
	for i, c := range []struct {
		score float64
		doc   int
	}{{1.0, 0}, {0.63, 1}, {0.47, 2}} {
		if math.Abs(float64(hits[i].Score)-c.score) > 0.01 {
			t.Fatalf("hit %d: expected score %v, got %v", i, c.score, hits[i].Score)
		}
		if hits[i].Doc != c.doc {
			t.Fatalf("hit %d: expected doc %d, got %d", i, c.doc, hits[i].Doc)
		}
	}
	queryUtilsCheckSearcher(t, query, searcher)
	mustClose(t, reader, directory)
}

func TestPhraseQueryToString(t *testing.T) {
	q := search.NewPhraseQuery(0, "field")
	if got := q.ToString(""); got != "\"\"" {
		t.Fatalf("expected %q, got %q", "\"\"", got)
	}

	builder := search.NewPhraseQueryBuilder()
	builder.AddWithPosition(index.NewTerm("field", "hi"), 1)
	q = builder.Build()
	if got := q.ToString(""); got != "field:\"? hi\"" {
		t.Fatalf("expected %q, got %q", "field:\"? hi\"", got)
	}

	builder = search.NewPhraseQueryBuilder()
	builder.AddWithPosition(index.NewTerm("field", "hi"), 1)
	builder.AddWithPosition(index.NewTerm("field", "test"), 5)
	q = builder.Build() // Query "this hi this is a test is"

	if got := q.ToString(""); got != "field:\"? hi ? ? ? test\"" {
		t.Fatalf("expected %q, got %q", "field:\"? hi ? ? ? test\"", got)
	}

	builder = search.NewPhraseQueryBuilder()
	builder.AddWithPosition(index.NewTerm("field", "hi"), 1)
	builder.AddWithPosition(index.NewTerm("field", "hello"), 1)
	builder.AddWithPosition(index.NewTerm("field", "test"), 5)
	q = builder.Build()
	if got := q.ToString(""); got != "field:\"? hi|hello ? ? ? test\"" {
		t.Fatalf("expected %q, got %q", "field:\"? hi|hello ? ? ? test\"", got)
	}

	builder = search.NewPhraseQueryBuilder()
	builder.AddWithPosition(index.NewTerm("field", "hi"), 1)
	builder.AddWithPosition(index.NewTerm("field", "hello"), 1)
	builder.AddWithPosition(index.NewTerm("field", "test"), 5)
	builder.SetSlop(5)
	q = builder.Build()
	if got := q.ToString(""); got != "field:\"? hi|hello ? ? ? test\"~5" {
		t.Fatalf("expected %q, got %q", "field:\"? hi|hello ? ? ? test\"~5", got)
	}
}

func TestPhraseQueryWrappedPhrase(t *testing.T) {
	c := phraseQueryBeforeClass(t)
	query := search.NewPhraseQuery(100, "repeated", "first", "part", "second", "part")

	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "slop of 100 just right", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	query = search.NewPhraseQuery(99, "repeated", "first", "part", "second", "part")

	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "slop of 99 not enough", hits, 0)
	queryUtilsCheckSearcher(t, query, c.searcher)
}

// work on two docs like this: "phrase exist notexist exist found"
func TestPhraseQueryNonExistingPhrase(t *testing.T) {
	c := phraseQueryBeforeClass(t)
	// phrase without repetitions that exists in 2 docs
	query := search.NewPhraseQuery(2, "nonexist", "phrase", "notexist", "found")

	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "phrase without repetitions exists in 2 docs", hits, 2)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// phrase with repetitions that exists in 2 docs
	query = search.NewPhraseQuery(1, "nonexist", "phrase", "exist", "exist")

	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "phrase with repetitions exists in two docs", hits, 2)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// phrase I with repetitions that does not exist in any doc
	query = search.NewPhraseQuery(1000, "nonexist", "phrase", "notexist", "phrase")

	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "nonexisting phrase with repetitions does not exist in any doc", hits, 0)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// phrase II with repetitions that does not exist in any doc
	query = search.NewPhraseQuery(1000, "nonexist", "phrase", "exist", "exist", "exist")

	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "nonexisting phrase with repetitions does not exist in any doc", hits, 0)
	queryUtilsCheckSearcher(t, query, c.searcher)
}

func assertPhraseScoresEqual(t *testing.T, msg string, a, b float32) {
	t.Helper()
	if diff := a - b; diff > phraseScoreCompThresh || diff < -phraseScoreCompThresh {
		t.Fatalf("%s: %v vs %v", msg, a, b)
	}
}

// Working on a 2 fields like this: Field("field", "one two three four five")
// Field("palindrome", "one two three two one") Phrase of size 2 occuriong
// twice, once in order and once in reverse, because doc is a palyndrome, is
// counted twice. Also, in this case order in query does not matter. Also,
// when an exact match is found, both sloppy scorer and exact scorer scores
// the same.
func TestPhraseQueryPalyndrome2(t *testing.T) {
	c := phraseQueryBeforeClass(t)

	// search on non palyndrome, find phrase with no slop, using exact phrase scorer
	query := search.NewPhraseQuery(0, "field", "two", "three") // to use exact phrase scorer
	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "phrase found with exact phrase scorer", hits, 1)
	score0 := hits[0].Score
	queryUtilsCheckSearcher(t, query, c.searcher)

	// search on non palyndrome, find phrase with slop 2, though no slop required here.
	query = search.NewPhraseQuery(0, "field", "two", "three") // to use sloppy scorer
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "just sloppy enough", hits, 1)
	score1 := hits[0].Score
	assertPhraseScoresEqual(t, "exact scorer and sloppy scorer score the same when slop does not matter", score0, score1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// search ordered in palyndrome, find it twice
	query = search.NewPhraseQuery(
		2,
		"palindrome",
		"two",
		"three") // must be at least two for both ordered and reversed to match
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "just sloppy enough", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// commented out for sloppy-phrase efficiency (issue 736) - see SloppyPhraseScorer.phraseFreq().
	// assertTrue("ordered scores higher in palindrome",score1+SCORE_COMP_THRESH<score2);

	// search reveresed in palyndrome, find it twice
	query = search.NewPhraseQuery(
		2,
		"palindrome",
		"three",
		"two") // must be at least two for both ordered and reversed to match
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "just sloppy enough", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// commented out for sloppy-phrase efficiency (issue 736) - see SloppyPhraseScorer.phraseFreq().
	// assertTrue("reversed scores higher in palindrome",score1+SCORE_COMP_THRESH<score3);
	// assertEquals("ordered or reversed does not matter",score2, score3, SCORE_COMP_THRESH);
}

// Working on a 2 fields like this: Field("field", "one two three four five")
// Field("palindrome", "one two three two one") Phrase of size 3 occuriong
// twice, once in order and once in reverse, because doc is a palyndrome, is
// counted twice. Also, in this case order in query does not matter. Also,
// when an exact match is found, both sloppy scorer and exact scorer scores
// the same.
func TestPhraseQueryPalyndrome3(t *testing.T) {
	c := phraseQueryBeforeClass(t)

	// search on non palyndrome, find phrase with no slop, using exact phrase scorer
	// slop=0 to use exact phrase scorer
	query := search.NewPhraseQuery(0, "field", "one", "two", "three")
	hits := mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "phrase found with exact phrase scorer", hits, 1)
	score0 := hits[0].Score
	queryUtilsCheckSearcher(t, query, c.searcher)

	// just make sure no exc:
	if _, err := c.searcher.Explain(query, 0); err != nil {
		t.Fatalf("explain: %v", err)
	}

	// search on non palyndrome, find phrase with slop 3, though no slop required here.
	// slop=4 to use sloppy scorer
	query = search.NewPhraseQuery(4, "field", "one", "two", "three")
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "just sloppy enough", hits, 1)
	score1 := hits[0].Score
	assertPhraseScoresEqual(t, "exact scorer and sloppy scorer score the same when slop does not matter", score0, score1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// search ordered in palyndrome, find it twice
	// slop must be at least four for both ordered and reversed to match
	query = search.NewPhraseQuery(4, "palindrome", "one", "two", "three")
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs

	// just make sure no exc:
	if _, err := c.searcher.Explain(query, 0); err != nil {
		t.Fatalf("explain: %v", err)
	}

	assertPhraseHitCount(t, "just sloppy enough", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// commented out for sloppy-phrase efficiency (issue 736) - see SloppyPhraseScorer.phraseFreq().
	// assertTrue("ordered scores higher in palindrome",score1+SCORE_COMP_THRESH<score2);

	// search reveresed in palyndrome, find it twice
	// must be at least four for both ordered and reversed to match
	query = search.NewPhraseQuery(4, "palindrome", "three", "two", "one")
	hits = mustSearch(t, c.searcher, query, 1000).ScoreDocs
	assertPhraseHitCount(t, "just sloppy enough", hits, 1)
	queryUtilsCheckSearcher(t, query, c.searcher)

	// commented out for sloppy-phrase efficiency (issue 736) - see SloppyPhraseScorer.phraseFreq().
	// assertTrue("reversed scores higher in palindrome",score1+SCORE_COMP_THRESH<score3);
	// assertEquals("ordered or reversed does not matter",score2, score3, SCORE_COMP_THRESH);
}

// LUCENE-1280
func TestPhraseQueryEmptyPhraseQuery(t *testing.T) {
	q2 := search.NewBooleanQueryBuilder()
	q2.Add(search.NewPhraseQuery(0, "field"), search.MUST)
	_ = q2.Build().ToString("")
}

// test that a single term is rewritten to a term query
func TestPhraseQueryRewrite(t *testing.T) {
	c := phraseQueryBeforeClass(t)
	pq := search.NewPhraseQuery(0, "foo", "bar")
	rewritten, err := pq.Rewrite(c.searcher)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if _, ok := rewritten.(*search.TermQuery); !ok {
		t.Fatalf("expected TermQuery, got %T", rewritten)
	}
}

// Tests PhraseQuery with terms at the same position in the query.
func TestPhraseQueryZeroPosIncr(t *testing.T) {
	dir := newDirectory()
	tokens := make([]testanalysis.Token, 3)
	tokens[0] = testanalysis.NewTokenWithPosInc("a", 1, 0, 0)
	tokens[1] = testanalysis.NewTokenWithPosInc("aa", 0, 0, 0)
	tokens[2] = testanalysis.NewTokenWithPosInc("b", 1, 0, 0)

	writer := newRandomIndexWriter(t, dir)
	tf, err := document.NewTextFieldFromTokenStream("field", testanalysis.NewCannedTokenStream(tokens...))
	if err != nil {
		t.Fatalf("new TextField: %v", err)
	}
	doc := newTestDocument(tf)
	mustAddDocument(t, writer, doc)
	r := mustGetReader(t, writer)
	mustClose(t, writer)
	searcher := newSearcher(t, r)

	// Sanity check; simple "a b" phrase:
	pqBuilder := search.NewPhraseQueryBuilder()
	pqBuilder.AddWithPosition(index.NewTerm("field", "a"), 0)
	pqBuilder.AddWithPosition(index.NewTerm("field", "b"), 1)
	if got := mustCount(t, searcher, pqBuilder.Build()); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}

	// Now with "a|aa b"
	pqBuilder = search.NewPhraseQueryBuilder()
	pqBuilder.AddWithPosition(index.NewTerm("field", "a"), 0)
	pqBuilder.AddWithPosition(index.NewTerm("field", "aa"), 0)
	pqBuilder.AddWithPosition(index.NewTerm("field", "b"), 1)
	if got := mustCount(t, searcher, pqBuilder.Build()); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}

	// Now with "a|z b" which should not match; this isn't a MultiPhraseQuery
	pqBuilder = search.NewPhraseQueryBuilder()
	pqBuilder.AddWithPosition(index.NewTerm("field", "a"), 0)
	pqBuilder.AddWithPosition(index.NewTerm("field", "z"), 0)
	pqBuilder.AddWithPosition(index.NewTerm("field", "b"), 1)
	if got := mustCount(t, searcher, pqBuilder.Build()); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}

	mustClose(t, r, dir)
}

func TestPhraseQueryRandomPhrases(t *testing.T) {
	dir := newDirectory()
	analyzer := newMockAnalyzer()

	iwc := newIndexWriterConfigWithAnalyzer(analyzer)
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	var docs [][]string
	d := document.NewDocument()
	f := newTextField(t, "f", "", false)
	d.Add(f)

	r := random()

	numDocs := atLeast(10)
	for i := 0; i < numDocs; i++ {
		// at night, must be > 4096 so it spans multiple chunks
		termCount := atLeast(200) // TEST_NIGHTLY is false

		var doc []string

		var sb strings.Builder
		for len(doc) < termCount {
			if r.Intn(5) == 1 || len(docs) == 0 {
				// make new non-empty-string term
				var term string
				for {
					term = randomUnicodeString(r)
					if len(term) > 0 {
						break
					}
				}
				func() {
					ts, err := analyzer.TokenStream("ignore", strings.NewReader(term))
					if err != nil {
						t.Fatalf("tokenStream: %v", err)
					}
					defer mustClose(t, ts)
					termAttr := ts.GetAttributeSource().AddAttribute(tokenattributes.CharTermAttributeType).(analysis.CharTermAttribute)
					if err := ts.Reset(); err != nil {
						t.Fatalf("reset: %v", err)
					}
					for {
						ok, err := ts.IncrementToken()
						if err != nil {
							t.Fatalf("incrementToken: %v", err)
						}
						if !ok {
							break
						}
						text := termAttr.String()
						doc = append(doc, text)
						sb.WriteString(text)
						sb.WriteByte(' ')
					}
					if err := ts.End(); err != nil {
						t.Fatalf("end: %v", err)
					}
				}()
			} else {
				// pick existing sub-phrase
				lastDoc := docs[r.Intn(len(docs))]
				length := nextInt(1, 10)
				start := r.Intn(len(lastDoc) - length)
				for k := start; k < start+length; k++ {
					tk := lastDoc[k]
					doc = append(doc, tk)
					sb.WriteString(tk)
					sb.WriteByte(' ')
				}
			}
		}
		docs = append(docs, doc)
		f.SetStringValue(sb.String())
		mustAddDocument(t, w, d)
	}

	reader := mustGetReader(t, w)
	s := newSearcher(t, reader)
	mustClose(t, w)

	// now search
	num := atLeast(3)
	for i := 0; i < num; i++ {
		docID := r.Intn(len(docs))
		doc := docs[docID]

		numTerm := nextInt(2, 20)
		start := r.Intn(len(doc) - numTerm)
		builder := search.NewPhraseQueryBuilder()
		var sb strings.Builder
		for tpos := start; tpos < start+numTerm; tpos++ {
			builder.AddWithPosition(index.NewTerm("f", doc[tpos]), tpos)
			sb.WriteString(doc[tpos])
			sb.WriteByte(' ')
		}
		pq := builder.Build()

		hits := mustSearch(t, s, pq, numDocs)
		found := false
		for j := 0; j < len(hits.ScoreDocs); j++ {
			if hits.ScoreDocs[j].Doc == docID {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf("phrase '%s' not found; start=%d, it=%d, expected doc %d", sb.String(), start, i, docID)
		}
	}

	mustClose(t, reader, dir)
}

func TestPhraseQueryNegativeSlop(t *testing.T) {
	expectThrowsPanic(t, func() {
		search.NewPhraseQuery(-2, "field", "two", "one")
	})
}

func TestPhraseQueryNegativePosition(t *testing.T) {
	builder := search.NewPhraseQueryBuilder()
	expectThrowsPanic(t, func() {
		builder.AddWithPosition(index.NewTerm("field", "two"), -42)
	})
}

func TestPhraseQueryBackwardPositions(t *testing.T) {
	builder := search.NewPhraseQueryBuilder()
	builder.AddWithPosition(index.NewTerm("field", "one"), 1)
	builder.AddWithPosition(index.NewTerm("field", "two"), 5)
	expectThrowsPanic(t, func() {
		builder.AddWithPosition(index.NewTerm("field", "three"), 4)
	})
}

func TestPhraseQueryPhraseQueryMaxTerms(t *testing.T) {
	builder := search.NewPhraseQueryBuilder()
	termThreshold := 5
	builder.SetMaxTerms(termThreshold)
	for i := 0; i < termThreshold; i++ {
		builder.AddWithPosition(index.NewTerm("field", "one"+strconv.Itoa(i)), i)
	}
	expectThrowsPanic(t, func() {
		builder.AddWithPosition(index.NewTerm("field", "three"), termThreshold)
	})
}

// phraseDOCS renders the private static DOCS.
var phraseDOCS = []string{
	"a b c d e f g h",
	"b c b",
	"c d d d e f g b",
	"c b a b c",
	"a a b b c c d d",
	"a b c d a b c d a b c d",
}

func TestPhraseQueryTopPhrases(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	docs := append([]string(nil), phraseDOCS...)
	random().Shuffle(len(docs), func(i, j int) { docs[i], docs[j] = docs[j], docs[i] })
	for _, value := range phraseDOCS {
		tf, err := document.NewTextField("f", value, false)
		if err != nil {
			t.Fatalf("new TextField: %v", err)
		}
		mustAddDocument(t, w, newTestDocument(tf))
	}
	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	mustClose(t, w)
	searcher := newSearcher(t, r)
	for _, query := range []search.Query{
		search.NewPhraseQuery(0, "f", "b", "c"), // common phrase
		search.NewPhraseQuery(0, "f", "e", "f"), // always appear next to each other
		search.NewPhraseQuery(0, "f", "d", "d"), // repeated term
	} {
		for topN := 1; topN <= 2; topN++ {
			collectorManager := mustTopScoreDocCollectorManager(t, topN, math.MaxInt32)
			hits1 := mustSearchWithManager(t, searcher, query, collectorManager).ScoreDocs

			collectorManager = mustTopScoreDocCollectorManager(t, topN, 1)
			topDocs2 := mustSearchWithManager(t, searcher, query, collectorManager)
			hits2 := topDocs2.ScoreDocs

			if !(len(hits1) > 0) {
				t.Fatalf("%v", query)
			}
			testsearch.CheckEqual(t, query, hits1, hits2)
		}
	}
	mustClose(t, r, dir)
}

// mustTopScoreDocCollectorManager renders
// new TopScoreDocCollectorManager(int numHits, int totalHitsThreshold), whose
// body is this(numHits, null, totalHitsThreshold).
func mustTopScoreDocCollectorManager(t *testing.T, numHits, totalHitsThreshold int) *search.TopScoreDocCollectorManager {
	t.Helper()
	m, err := search.NewTopScoreDocCollectorManager(numHits, nil, totalHitsThreshold)
	if err != nil {
		t.Fatalf("new TopScoreDocCollectorManager: %v", err)
	}
	return m
}

// mustSearchWithManager renders IndexSearcher.search(Query, CollectorManager).
func mustSearchWithManager(t *testing.T, s *search.IndexSearcher, q search.Query, m *search.TopScoreDocCollectorManager) *search.TopDocs {
	t.Helper()
	td, err := search.SearchWithCollectorManager[*search.TopScoreDocCollector, *search.TopDocs](s, q, m)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

func TestPhraseQueryMergeImpacts(t *testing.T) {
	impacts1 := newDummyImpactsEnum(1000)
	impacts2 := newDummyImpactsEnum(2000)
	mergedImpacts := search.ExactPhraseMatcherMergeImpacts([]index.ImpactsEnum{impacts1, impacts2})

	imp := codecs.NewImpact
	impacts1.reset(
		[][]codecs.Impact{
			{imp(3, 10), imp(5, 12), imp(8, 13)},
			{imp(3, 10), imp(5, 11), imp(8, 13), imp(12, 14)},
		},
		[]int{110, 945})

	// Merge with empty impacts
	impacts2.reset([][]codecs.Impact{}, []int{})
	assertPhraseImpacts(t,
		[][]codecs.Impact{
			{imp(3, 10), imp(5, 12), imp(8, 13)},
			{imp(3, 10), imp(5, 11), imp(8, 13), imp(12, 14)},
		},
		[]int{110, 945},
		mustGetImpacts(t, mergedImpacts))

	// Merge with dummy impacts
	impacts2.reset([][]codecs.Impact{{imp(math.MaxInt32, 1)}}, []int{5000})
	assertPhraseImpacts(t,
		[][]codecs.Impact{
			{imp(3, 10), imp(5, 12), imp(8, 13)},
			{imp(3, 10), imp(5, 11), imp(8, 13), imp(12, 14)},
		},
		[]int{110, 945},
		mustGetImpacts(t, mergedImpacts))

	// Merge with dummy impacts that we don't special case
	impacts2.reset([][]codecs.Impact{{imp(math.MaxInt32, 2)}}, []int{5000})
	assertPhraseImpacts(t,
		[][]codecs.Impact{
			{imp(3, 10), imp(5, 12), imp(8, 13)},
			{imp(3, 10), imp(5, 11), imp(8, 13), imp(12, 14)},
		},
		[]int{110, 945},
		mustGetImpacts(t, mergedImpacts))

	// First level of impacts2 doesn't cover the first level of impacts1
	impacts2.reset(
		[][]codecs.Impact{
			{imp(2, 10), imp(6, 13)},
			{imp(3, 9), imp(5, 11), imp(7, 13)},
		},
		[]int{90, 1000})
	assertPhraseImpacts(t,
		[][]codecs.Impact{
			{imp(3, 10), imp(5, 12), imp(7, 13)},
			{imp(3, 10), imp(5, 11), imp(7, 13)},
		},
		[]int{110, 945},
		mustGetImpacts(t, mergedImpacts))

	// Second level of impacts2 doesn't cover the first level of impacts1
	impacts2.reset(
		[][]codecs.Impact{
			{imp(2, 10), imp(6, 11)},
			{imp(3, 9), imp(5, 11), imp(7, 13)},
		},
		[]int{150, 900})
	assertPhraseImpacts(t,
		[][]codecs.Impact{
			{imp(2, 10), imp(3, 11), imp(5, 12), imp(6, 13)},
			{imp(3, 10), imp(5, 11), imp(8, 13), imp(12, 14)}, // same as impacts1
		},
		[]int{110, 945},
		mustGetImpacts(t, mergedImpacts))

	impacts2.reset(
		[][]codecs.Impact{
			{imp(4, 10), imp(9, 13)},
			{imp(1, 1), imp(4, 10), imp(5, 11), imp(8, 13), imp(12, 14), imp(13, 15)},
		},
		[]int{113, 950})
	assertPhraseImpacts(t,
		[][]codecs.Impact{
			{imp(3, 10), imp(4, 12), imp(8, 13)},
			{imp(3, 10), imp(5, 11), imp(8, 13), imp(12, 14)},
		},
		[]int{110, 945},
		mustGetImpacts(t, mergedImpacts))

	// Make sure negative norms are treated as unsigned
	impacts1.reset(
		[][]codecs.Impact{
			{imp(3, 10), imp(5, -10), imp(8, -5)},
			{imp(3, 10), imp(5, -15), imp(8, -5), imp(12, -3)},
		},
		[]int{110, 945})
	impacts2.reset(
		[][]codecs.Impact{
			{imp(2, 10), imp(12, -4)},
			{imp(3, 9), imp(12, -4), imp(20, -1)},
		},
		[]int{150, 960})
	assertPhraseImpacts(t,
		[][]codecs.Impact{
			{imp(2, 10), imp(8, -4)},
			{imp(3, 10), imp(8, -4), imp(12, -3)},
		},
		[]int{110, 945},
		mustGetImpacts(t, mergedImpacts))
}

func mustGetImpacts(t *testing.T, source index.ImpactsSource) index.Impacts {
	t.Helper()
	impacts, err := source.GetImpacts()
	if err != nil {
		t.Fatalf("getImpacts: %v", err)
	}
	return impacts
}

// assertPhraseImpacts renders the private assertEquals(Impact[][], int[], Impacts).
func assertPhraseImpacts(t *testing.T, impacts [][]codecs.Impact, docIDUpTo []int, actual index.Impacts) {
	t.Helper()
	if len(impacts) != actual.NumLevels() {
		t.Fatalf("numLevels: expected %d, got %d", len(impacts), actual.NumLevels())
	}
	for i := 0; i < len(impacts); i++ {
		if docIDUpTo[i] != actual.GetDocIDUpTo(i) {
			t.Fatalf("level %d docIdUpTo: expected %d, got %d", i, docIDUpTo[i], actual.GetDocIDUpTo(i))
		}
		got := copyOfImpacts(actual.GetImpacts(i))
		if len(got) != len(impacts[i]) {
			t.Fatalf("level %d: expected %v, got %v", i, impacts[i], got)
		}
		for j := range got {
			if got[j] != impacts[i][j] {
				t.Fatalf("level %d: expected %v, got %v", i, impacts[i], got)
			}
		}
	}
}

// copyOfImpacts renders the private copyOf(FreqAndNormBuffer).
func copyOfImpacts(buffer *util.FreqAndNormBuffer) []codecs.Impact {
	var impactsCopy []codecs.Impact
	for i := 0; i < buffer.Size; i++ {
		impactsCopy = append(impactsCopy, codecs.NewImpact(buffer.Freqs[i], buffer.Norms[i]))
	}
	return impactsCopy
}

// errPhraseUnsupported renders the UnsupportedOperationException the private
// DummyImpactsEnum throws.
var errPhraseUnsupported = errors.New("UnsupportedOperationException")

// dummyImpactsEnum renders the private DummyImpactsEnum.
type dummyImpactsEnum struct {
	cost      int64
	impacts   [][]codecs.Impact
	docIDUpTo []int
}

func newDummyImpactsEnum(cost int64) *dummyImpactsEnum {
	return &dummyImpactsEnum{cost: cost}
}

func (e *dummyImpactsEnum) reset(impacts [][]codecs.Impact, docIDUpTo []int) {
	e.impacts = impacts
	e.docIDUpTo = docIDUpTo
}

func (e *dummyImpactsEnum) AdvanceShallow(target int) error { return errPhraseUnsupported }

// GetImpacts returns the anonymous Impacts view over the current impacts.
func (e *dummyImpactsEnum) GetImpacts() (index.Impacts, error) {
	return &dummyImpacts{e: e}, nil
}

func (e *dummyImpactsEnum) Freq() (int, error)          { return 0, errPhraseUnsupported }
func (e *dummyImpactsEnum) NextPosition() (int, error)  { return 0, errPhraseUnsupported }
func (e *dummyImpactsEnum) StartOffset() (int, error)   { return 0, errPhraseUnsupported }
func (e *dummyImpactsEnum) EndOffset() (int, error)     { return 0, errPhraseUnsupported }
func (e *dummyImpactsEnum) GetPayload() ([]byte, error) { return nil, errPhraseUnsupported }
func (e *dummyImpactsEnum) DocID() int                  { panic(errPhraseUnsupported) }
func (e *dummyImpactsEnum) NextDoc() (int, error)       { return 0, errPhraseUnsupported }
func (e *dummyImpactsEnum) Advance(target int) (int, error) {
	return 0, errPhraseUnsupported
}
func (e *dummyImpactsEnum) Cost() int64 { return e.cost }

// IntoBitSet carries the inherited DocIdSetIterator.intoBitSet body.
func (e *dummyImpactsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(e, upTo, bitSet, offset)
}

// DocIDRunEnd carries the inherited DocIdSetIterator.docIDRunEnd body.
func (e *dummyImpactsEnum) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(e)
}

type dummyImpacts struct {
	e *dummyImpactsEnum
}

func (d *dummyImpacts) NumLevels() int { return len(d.e.impacts) }

func (d *dummyImpacts) GetDocIDUpTo(level int) int { return d.e.docIDUpTo[level] }

func (d *dummyImpacts) GetImpacts(level int) *util.FreqAndNormBuffer {
	buffer := util.NewFreqAndNormBuffer()
	for _, impact := range d.e.impacts[level] {
		buffer.Add(impact.Freq, impact.Norm)
	}
	return buffer
}

func TestPhraseQueryRandomTopDocs(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	numDocs := atLeast(100) // TEST_NIGHTLY is false; at night, make sure some terms have skip data
	for i := 0; i < numDocs; i++ {
		numTerms := random().Intn(1 << random().Intn(5))
		terms := make([]string, numTerms)
		for k := range terms {
			if random().Intn(2) == 0 {
				terms[k] = "a"
			} else if random().Intn(2) == 0 {
				terms[k] = "b"
			} else {
				terms[k] = "c"
			}
		}
		text := strings.Join(terms, " ")
		tf, err := document.NewTextField("foo", text, false)
		if err != nil {
			t.Fatalf("new TextField: %v", err)
		}
		mustAddDocument(t, w, newTestDocument(tf))
	}
	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	mustClose(t, w)
	searcher := newSearcher(t, reader)

	for _, firstTerm := range []string{"a", "b", "c"} {
		for _, secondTerm := range []string{"a", "b", "c"} {
			query := search.NewPhraseQueryWithBytes(0, "foo", []byte(firstTerm), []byte(secondTerm))

			completeManager := mustTopScoreDocCollectorManager(t, 10, math.MaxInt32) // COMPLETE
			topScoresManager := mustTopScoreDocCollectorManager(t, 10, 10)           // TOP_SCORES

			complete := mustSearchWithManager(t, searcher, query, completeManager)
			topScores := mustSearchWithManager(t, searcher, query, topScoresManager)
			testsearch.CheckEqual(t, query, complete.ScoreDocs, topScores.ScoreDocs)

			filteredQuery := search.NewBooleanQueryBuilder().
				Add(query, search.MUST).
				Add(search.NewTermQuery(index.NewTerm("foo", "b")), search.FILTER).
				Build()

			completeManager = mustTopScoreDocCollectorManager(t, 10, math.MaxInt32) // COMPLETE
			topScoresManager = mustTopScoreDocCollectorManager(t, 10, 10)           // TOP_SCORES

			complete = mustSearchWithManager(t, searcher, filteredQuery, completeManager)
			topScores = mustSearchWithManager(t, searcher, filteredQuery, topScoresManager)
			testsearch.CheckEqual(t, query, complete.ScoreDocs, topScores.ScoreDocs)
		}
	}
	mustClose(t, reader, dir)
}

func TestPhraseQueryNullTerm(t *testing.T) {
	e := expectThrowsPanic(t, func() { search.NewPhraseQueryBuilder().Add(nil) })
	if e != "Cannot add a null term to PhraseQuery" {
		t.Fatalf("unexpected message %q", e)
	}

	e = expectThrowsPanic(t, func() { search.NewPhraseQueryWithBytes(0, "field", nil) })
	if e != "Cannot add a null term to PhraseQuery" {
		t.Fatalf("unexpected message %q", e)
	}

	// e = expectThrows(NullPointerException.class, () -> new PhraseQuery("field", (String) null));
	// NewPhraseQuery(int, String, String...) takes Go strings, which cannot be
	// null, so this Java case has no Go input to render.
}
