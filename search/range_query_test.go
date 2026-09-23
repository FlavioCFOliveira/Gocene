// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestTermRangeQuery.java
// (Apache Lucene 10.5.0).
//
// TermRangeQuery.newStringRange(String, String, String, ...) takes nullable
// strings. Gocene's NewStringRange takes Go strings and renders Java's null as
// the empty string; the Java calls that pass a real empty string ("") are
// therefore rendered with the body of newStringRange itself
// (new TermRangeQuery(field, new BytesRef(""), ...)).

package search_test

import (
	"io"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// termRangeQueryTest renders the per-test state of TestTermRangeQuery: the
// docCount field and the Directory created in setUp.
type termRangeQueryTest struct {
	t        *testing.T
	docCount int
	dir      store.Directory
}

// setUp renders TestTermRangeQuery.setUp(); tearDown is registered with t.Cleanup.
func newTermRangeQueryTest(t *testing.T) *termRangeQueryTest {
	tc := &termRangeQueryTest{t: t, dir: newDirectory()}
	t.Cleanup(func() {
		if err := tc.dir.Close(); err != nil {
			t.Errorf("close dir: %v", err)
		}
	})
	return tc
}

func (tc *termRangeQueryTest) openReader() *index.DirectoryReader {
	return mustOpenDirectoryReader(tc.t, tc.dir)
}

func TestTermRangeQueryExclusive(t *testing.T) {
	tc := newTermRangeQueryTest(t)
	query := search.NewStringRange("content", "A", "C", false, false)
	tc.initializeIndex([]string{"A", "B", "C", "D"})
	reader := tc.openReader()
	searcher := newSearcher(t, reader)
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 1 {
		t.Fatalf("A,B,C,D, only B in range: expected 1, got %d", len(hits))
	}
	mustClose(t, reader)

	tc.initializeIndex([]string{"A", "B", "D"})
	reader = tc.openReader()
	searcher = newSearcher(t, reader)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 1 {
		t.Fatalf("A,B,D, only B in range: expected 1, got %d", len(hits))
	}
	mustClose(t, reader)

	tc.addDoc("C")
	reader = tc.openReader()
	searcher = newSearcher(t, reader)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 1 {
		t.Fatalf("C added, still only B in range: expected 1, got %d", len(hits))
	}
	mustClose(t, reader)
}

func TestTermRangeQueryInclusive(t *testing.T) {
	tc := newTermRangeQueryTest(t)
	query := search.NewStringRange("content", "A", "C", true, true)

	tc.initializeIndex([]string{"A", "B", "C", "D"})
	reader := tc.openReader()
	searcher := newSearcher(t, reader)
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 3 {
		t.Fatalf("A,B,C,D - A,B,C in range: expected 3, got %d", len(hits))
	}
	mustClose(t, reader)

	tc.initializeIndex([]string{"A", "B", "D"})
	reader = tc.openReader()
	searcher = newSearcher(t, reader)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 2 {
		t.Fatalf("A,B,D - A and B in range: expected 2, got %d", len(hits))
	}
	mustClose(t, reader)

	tc.addDoc("C")
	reader = tc.openReader()
	searcher = newSearcher(t, reader)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 3 {
		t.Fatalf("C added - A, B, C in range: expected 3, got %d", len(hits))
	}
	mustClose(t, reader)
}

func TestTermRangeQueryAllDocs(t *testing.T) {
	tc := newTermRangeQueryTest(t)
	tc.initializeIndex([]string{"A", "B", "C", "D"})
	reader := tc.openReader()
	searcher := newSearcher(t, reader)

	query := search.NewTermRangeQuery("content", nil, nil, true, true)
	if got := len(mustSearch(t, searcher, query, 1000).ScoreDocs); got != 4 {
		t.Fatalf("expected 4, got %d", got)
	}

	// TermRangeQuery.newStringRange("content", "", null, true, true)
	query = search.NewTermRangeQuery("content", util.NewBytesRef([]byte("")), nil, true, true)
	if got := len(mustSearch(t, searcher, query, 1000).ScoreDocs); got != 4 {
		t.Fatalf("expected 4, got %d", got)
	}

	// TermRangeQuery.newStringRange("content", "", null, true, false)
	query = search.NewTermRangeQuery("content", util.NewBytesRef([]byte("")), nil, true, false)
	if got := len(mustSearch(t, searcher, query, 1000).ScoreDocs); got != 4 {
		t.Fatalf("expected 4, got %d", got)
	}

	// and now another one
	query = search.NewStringRange("content", "B", "", true, true)
	if got := len(mustSearch(t, searcher, query, 1000).ScoreDocs); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
	mustClose(t, reader)
}

// This test should not be here, but it tests the fuzzy query rewrite mode
// (TOP_TERMS_SCORING_BOOLEAN_REWRITE) with constant score and checks, that
// only the lower end of terms is put into the range.
func TestTermRangeQueryTopTermsRewrite(t *testing.T) {
	tc := newTermRangeQueryTest(t)
	tc.initializeIndex([]string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"})

	reader := tc.openReader()
	searcher := newSearcher(t, reader)
	rewriteMethod := search.NewTopTermsScoringBooleanQueryRewrite(50)
	query := search.NewStringRangeWithRewriteMethod("content", "B", "J", true, true, rewriteMethod)
	checkTermRangeBooleanTerms(t, searcher, query, "B", "C", "D", "E", "F", "G", "H", "I", "J")

	savedClauseCount := search.GetMaxClauseCount()
	func() {
		defer search.SetMaxClauseCount(savedClauseCount)
		search.SetMaxClauseCount(3)
		checkTermRangeBooleanTerms(t, searcher, query, "B", "C", "D")
	}()
	mustClose(t, reader)
}

func checkTermRangeBooleanTerms(t *testing.T, searcher *search.IndexSearcher, query *search.TermRangeQuery, terms ...string) {
	t.Helper()
	bq := mustRewrite(t, searcher, query).(*search.BooleanQuery)
	allowedTerms := map[string]struct{}{}
	for _, term := range terms {
		allowedTerms[term] = struct{}{}
	}
	if len(bq.Clauses()) != len(allowedTerms) {
		t.Fatalf("expected %d clauses, got %d", len(allowedTerms), len(bq.Clauses()))
	}
	for _, c := range bq.Clauses() {
		tq, ok := c.Query().(*search.TermQuery)
		if !ok {
			t.Fatalf("expected a TermQuery, got %T", c.Query())
		}
		term := tq.GetTerm().Text()
		if _, ok := allowedTerms[term]; !ok {
			t.Fatalf("invalid term: %s", term)
		}
		delete(allowedTerms, term) // remove to fail on double terms
	}
	if len(allowedTerms) != 0 {
		t.Fatalf("expected no remaining terms, got %v", allowedTerms)
	}
}

func TestTermRangeQueryEqualsHashcode(t *testing.T) {
	var query search.Query = search.NewStringRange("content", "A", "C", true, true)

	var other search.Query = search.NewStringRange("content", "A", "C", true, true)

	if !query.Equals(query) {
		t.Fatal("query equals itself is true")
	}
	if !query.Equals(other) {
		t.Fatal("equivalent queries are equal")
	}
	if query.HashCode() != other.HashCode() {
		t.Fatal("hashcode must return same value when equals is true")
	}

	other = search.NewStringRange("notcontent", "A", "C", true, true)
	if query.Equals(other) {
		t.Fatal("Different fields are not equal")
	}

	other = search.NewStringRange("content", "X", "C", true, true)
	if query.Equals(other) {
		t.Fatal("Different lower terms are not equal")
	}

	other = search.NewStringRange("content", "A", "Z", true, true)
	if query.Equals(other) {
		t.Fatal("Different upper terms are not equal")
	}

	query = search.NewStringRange("content", "", "C", true, true)
	other = search.NewStringRange("content", "", "C", true, true)
	if !query.Equals(other) {
		t.Fatal("equivalent queries with null lowerterms are equal()")
	}
	if query.HashCode() != other.HashCode() {
		t.Fatal("hashcode must return same value when equals is true")
	}

	query = search.NewStringRange("content", "C", "", true, true)
	other = search.NewStringRange("content", "C", "", true, true)
	if !query.Equals(other) {
		t.Fatal("equivalent queries with null upperterms are equal()")
	}
	if query.HashCode() != other.HashCode() {
		t.Fatal("hashcode returns same value")
	}

	query = search.NewStringRange("content", "", "C", true, true)
	other = search.NewStringRange("content", "C", "", true, true)
	if query.Equals(other) {
		t.Fatal("queries with different upper and lower terms are not equal")
	}

	query = search.NewStringRange("content", "A", "C", false, false)
	other = search.NewStringRange("content", "A", "C", true, true)
	if query.Equals(other) {
		t.Fatal("queries with different inclusive are not equal")
	}
}

// newSingleCharAnalyzer renders the private TestTermRangeQuery.SingleCharAnalyzer.
func newSingleCharAnalyzer() analysis.Analyzer {
	a := analysis.NewAnalyzer(nil)
	a.CreateComponents = func(fieldName string) *analysis.TokenStreamComponents {
		src := newSingleCharTokenizer()
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				src.SetReader(r)
				return nil
			},
			Sink: src,
		}
	}
	return a
}

// singleCharTokenizer renders SingleCharAnalyzer.SingleCharTokenizer: it
// emits exactly one token holding the first character of the input, or an
// empty token when the input is empty.
type singleCharTokenizer struct {
	*analysis.BaseTokenizer
	done    bool
	termAtt analysis.CharTermAttribute
}

func newSingleCharTokenizer() *singleCharTokenizer {
	tk := &singleCharTokenizer{BaseTokenizer: analysis.NewBaseTokenizer()}
	tk.termAtt = tk.AddAttribute(tokenattributes.CharTermAttributeType).(analysis.CharTermAttribute)
	return tk
}

// readChar renders input.read(buffer) with a one-char buffer: it returns the
// first character of the input, or ok=false at end of input.
func (tk *singleCharTokenizer) readChar() (string, bool, error) {
	var buf [utf8.UTFMax]byte
	n := 0
	for n < utf8.UTFMax {
		m, err := tk.GetReader().Read(buf[n : n+1])
		n += m
		if n > 0 && utf8.FullRune(buf[:n]) {
			return string(buf[:n]), true, nil
		}
		if err == io.EOF {
			return "", n > 0, nil
		}
		if err != nil {
			return "", false, err
		}
	}
	return string(buf[:n]), true, nil
}

func (tk *singleCharTokenizer) IncrementToken() (bool, error) {
	if tk.done {
		return false, nil
	}
	c, ok, err := tk.readChar()
	if err != nil {
		return false, err
	}
	tk.ClearAttributes()
	tk.done = true
	if ok {
		tk.termAtt.SetValue(c)
	}
	return true, nil
}

func (tk *singleCharTokenizer) Reset() error {
	if err := tk.BaseTokenizer.Reset(); err != nil {
		return err
	}
	tk.done = false
	return nil
}

var _ analysis.Tokenizer = (*singleCharTokenizer)(nil)

// initializeIndex renders initializeIndex(String[]):
// initializeIndex(values, new MockAnalyzer(random(), MockTokenizer.WHITESPACE, false)).
func (tc *termRangeQueryTest) initializeIndex(values []string) {
	tc.initializeIndexWithAnalyzer(values, testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, false, 0, nil, true))
}

// initializeIndexWithAnalyzer renders initializeIndex(String[], Analyzer).
func (tc *termRangeQueryTest) initializeIndexWithAnalyzer(values []string, analyzer analysis.Analyzer) {
	iwc := newIndexWriterConfigWithAnalyzer(analyzer).SetOpenMode(index.Create)
	writer := mustNewIndexWriter(tc.t, tc.dir, iwc)
	for i := 0; i < len(values); i++ {
		tc.insertDoc(writer, values[i])
	}
	mustClose(tc.t, writer)
}

// addDoc renders addDoc(String). shouldnt create an analyzer for every doc?
func (tc *termRangeQueryTest) addDoc(content string) {
	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, false, 0, nil, true)).
		SetOpenMode(index.Append)
	writer := mustNewIndexWriter(tc.t, tc.dir, iwc)
	tc.insertDoc(writer, content)
	mustClose(tc.t, writer)
}

func (tc *termRangeQueryTest) insertDoc(writer *index.IndexWriter, content string) {
	doc := document.NewDocument()

	doc.Add(newStringField(tc.t, "id", "id"+strconv.Itoa(tc.docCount), true))
	doc.Add(newTextField(tc.t, "content", content, false))

	mustAddDocument(tc.t, writer, doc)
	tc.docCount++
}

// LUCENE-38
func TestTermRangeQueryExclusiveLowerNull(t *testing.T) {
	tc := newTermRangeQueryTest(t)
	analyzer := newSingleCharAnalyzer()
	// http://issues.apache.org/jira/browse/LUCENE-38
	query := search.NewStringRange("content", "", "C", false, false)
	tc.initializeIndexWithAnalyzer([]string{"A", "B", "", "C", "D"}, analyzer)
	reader := tc.openReader()
	searcher := newSearcher(t, reader)
	numHits := mustSearch(t, searcher, query, 1000).TotalHits.Value
	// When Lucene-38 is fixed, use the assert on the next line:
	if numHits != 3 {
		t.Fatalf("A,B,<empty string>,C,D => A, B & <empty string> are in range: expected 3, got %d", numHits)
	}
	mustClose(t, reader)
	tc.initializeIndexWithAnalyzer([]string{"A", "B", "", "D"}, analyzer)
	reader = tc.openReader()
	searcher = newSearcher(t, reader)
	numHits = mustSearch(t, searcher, query, 1000).TotalHits.Value
	if numHits != 3 {
		t.Fatalf("A,B,<empty string>,D => A, B & <empty string> are in range: expected 3, got %d", numHits)
	}
	mustClose(t, reader)
	tc.addDoc("C")
	reader = tc.openReader()
	searcher = newSearcher(t, reader)
	numHits = mustSearch(t, searcher, query, 1000).TotalHits.Value
	if numHits != 3 {
		t.Fatalf("C added, still A, B & <empty string> are in range: expected 3, got %d", numHits)
	}
	mustClose(t, reader)
}

// LUCENE-38
func TestTermRangeQueryInclusiveLowerNull(t *testing.T) {
	tc := newTermRangeQueryTest(t)
	// http://issues.apache.org/jira/browse/LUCENE-38
	analyzer := newSingleCharAnalyzer()
	query := search.NewStringRange("content", "", "C", true, true)
	tc.initializeIndexWithAnalyzer([]string{"A", "B", "", "C", "D"}, analyzer)
	reader := tc.openReader()
	searcher := newSearcher(t, reader)
	numHits := mustSearch(t, searcher, query, 1000).TotalHits.Value
	if numHits != 4 {
		t.Fatalf("A,B,<empty string>,C,D => A,B,<empty string>,C in range: expected 4, got %d", numHits)
	}
	mustClose(t, reader)
	tc.initializeIndexWithAnalyzer([]string{"A", "B", "", "D"}, analyzer)
	reader = tc.openReader()
	searcher = newSearcher(t, reader)
	numHits = mustSearch(t, searcher, query, 1000).TotalHits.Value
	if numHits != 3 {
		t.Fatalf("A,B,<empty string>,D - A, B and <empty string> in range: expected 3, got %d", numHits)
	}
	mustClose(t, reader)
	tc.addDoc("C")
	reader = tc.openReader()
	searcher = newSearcher(t, reader)
	numHits = mustSearch(t, searcher, query, 1000).TotalHits.Value
	if numHits != 4 {
		t.Fatalf("C added => A,B,<empty string>,C in range: expected 4, got %d", numHits)
	}
	mustClose(t, reader)
}
