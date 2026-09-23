// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestMultiPhraseQuery.java
// (Apache Lucene 10.5.0): tests the MultiPhraseQuery class.

package search_test

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

func TestMultiPhraseQueryPhrasePrefix(t *testing.T) {
	indexStore := newDirectory()
	writer := newRandomIndexWriter(t, indexStore)
	addMultiPhraseDoc(t, "blueberry pie", writer)
	addMultiPhraseDoc(t, "blueberry strudel", writer)
	addMultiPhraseDoc(t, "blueberry pizza", writer)
	addMultiPhraseDoc(t, "blueberry chewing gum", writer)
	addMultiPhraseDoc(t, "bluebird pizza", writer)
	addMultiPhraseDoc(t, "bluebird foobar pizza", writer)
	addMultiPhraseDoc(t, "piccadilly circus", writer)

	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)

	// search for "blueberry pi*":
	query1builder := search.NewMultiPhraseQueryBuilder()
	// search for "strawberry pi*":
	query2builder := search.NewMultiPhraseQueryBuilder()
	query1builder.Add(index.NewTerm("body", "blueberry"))
	query2builder.Add(index.NewTerm("body", "strawberry"))

	var termsWithPrefix []*index.Term

	// this TermEnum gives "piccadilly", "pie" and "pizza".
	prefix := "pi"
	terms, err := index.MultiTermsGetTerms(reader, "body")
	if err != nil {
		t.Fatalf("MultiTerms.getTerms: %v", err)
	}
	te, err := terms.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	if _, err := te.SeekCeil(index.NewTerm("body", prefix)); err != nil {
		t.Fatalf("seekCeil: %v", err)
	}
	for {
		s := te.Term().Text()
		if strings.HasPrefix(s, prefix) {
			termsWithPrefix = append(termsWithPrefix, index.NewTerm("body", s))
		} else {
			break
		}
		next, err := te.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if next == nil {
			break
		}
	}

	query1builder.AddTerms(termsWithPrefix)
	query1 := query1builder.Build()
	if got := query1.String(); got != "body:\"blueberry (piccadilly pie pizza)\"" {
		t.Fatalf("unexpected toString %q", got)
	}

	query2builder.AddTerms(termsWithPrefix)
	query2 := query2builder.Build()
	if got := query2.String(); got != "body:\"strawberry (piccadilly pie pizza)\"" {
		t.Fatalf("unexpected toString %q", got)
	}

	result := mustSearch(t, searcher, query1, 1000).ScoreDocs
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	result = mustSearch(t, searcher, query2, 1000).ScoreDocs
	if len(result) != 0 {
		t.Fatalf("expected 0, got %d", len(result))
	}

	// search for "blue* pizza":
	query3builder := search.NewMultiPhraseQueryBuilder()
	termsWithPrefix = nil
	prefix = "blue"
	if _, err := te.SeekCeil(index.NewTerm("body", prefix)); err != nil {
		t.Fatalf("seekCeil: %v", err)
	}

	for {
		if strings.HasPrefix(te.Term().Text(), prefix) {
			termsWithPrefix = append(termsWithPrefix, index.NewTerm("body", te.Term().Text()))
		}
		next, err := te.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if next == nil {
			break
		}
	}

	query3builder.AddTerms(termsWithPrefix)
	query3builder.Add(index.NewTerm("body", "pizza"))

	query3 := query3builder.Build()

	result = mustSearch(t, searcher, query3, 1000).ScoreDocs
	if len(result) != 2 { // blueberry pizza, bluebird pizza
		t.Fatalf("expected 2, got %d", len(result))
	}
	if got := query3.String(); got != "body:\"(blueberry bluebird) pizza\"" {
		t.Fatalf("unexpected toString %q", got)
	}

	// test slop:
	query3builder.SetSlop(1)
	query3 = query3builder.Build()
	result = mustSearch(t, searcher, query3, 1000).ScoreDocs

	// just make sure no exc:
	if _, err := searcher.Explain(query3, 0); err != nil {
		t.Fatalf("explain: %v", err)
	}

	if len(result) != 3 { // blueberry pizza, bluebird pizza, bluebird foobar pizza
		t.Fatalf("expected 3, got %d", len(result))
	}

	query4builder := search.NewMultiPhraseQueryBuilder()
	expectThrowsPanic(t, func() {
		query4builder.Add(index.NewTerm("field1", "foo"))
		query4builder.Add(index.NewTerm("field2", "foobar"))
	})

	mustClose(t, writer, reader, indexStore)
}

// LUCENE-2580
func TestMultiPhraseQueryTall(t *testing.T) {
	indexStore := newDirectory()
	writer := newRandomIndexWriter(t, indexStore)
	addMultiPhraseDoc(t, "blueberry chocolate pie", writer)
	addMultiPhraseDoc(t, "blueberry chocolate tart", writer)
	r := mustGetReader(t, writer)
	mustClose(t, writer)

	searcher := newSearcher(t, r)
	qb := search.NewMultiPhraseQueryBuilder()
	qb.Add(index.NewTerm("body", "blueberry"))
	qb.Add(index.NewTerm("body", "chocolate"))
	qb.AddTerms([]*index.Term{index.NewTerm("body", "pie"), index.NewTerm("body", "tart")})
	if got := mustCount(t, searcher, qb.Build()); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
	mustClose(t, r, indexStore)
}

// testMultiSloppyWithRepeats is @Ignore'd in Lucene 10.5.0 (LUCENE-3821 fixes
// sloppy phrase scoring, except for this known problem): JUnit never runs it,
// so it is rendered as a function the test runner does not collect.
func testMultiPhraseQueryMultiSloppyWithRepeats(t *testing.T) {
	indexStore := newDirectory()
	writer := newRandomIndexWriter(t, indexStore)
	addMultiPhraseDoc(t, "a b c d e f g h i k", writer)
	r := mustGetReader(t, writer)
	mustClose(t, writer)

	searcher := newSearcher(t, r)

	qb := search.NewMultiPhraseQueryBuilder()
	// this will fail, when the scorer would propagate [a] rather than [a,b],
	qb.AddTerms([]*index.Term{index.NewTerm("body", "a"), index.NewTerm("body", "b")})
	qb.AddTerms([]*index.Term{index.NewTerm("body", "a")})
	qb.SetSlop(6)
	if got := mustCount(t, searcher, qb.Build()); got != 1 { // should match on "a b"
		t.Fatalf("expected 1, got %d", got)
	}

	mustClose(t, r, indexStore)
}

var _ = testMultiPhraseQueryMultiSloppyWithRepeats

func TestMultiPhraseQueryMultiExactWithRepeats(t *testing.T) {
	indexStore := newDirectory()
	writer := newRandomIndexWriter(t, indexStore)
	addMultiPhraseDoc(t, "a b c d e f g h i k", writer)
	r := mustGetReader(t, writer)
	mustClose(t, writer)

	searcher := newSearcher(t, r)
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTermsAtPosition([]*index.Term{index.NewTerm("body", "a"), index.NewTerm("body", "d")}, 0)
	qb.AddTermsAtPosition([]*index.Term{index.NewTerm("body", "a"), index.NewTerm("body", "f")}, 2)
	if got := mustCount(t, searcher, qb.Build()); got != 1 { // should match on "a b"
		t.Fatalf("expected 1, got %d", got)
	}
	mustClose(t, r, indexStore)
}

// addMultiPhraseDoc renders the private add(String, RandomIndexWriter).
func addMultiPhraseDoc(t *testing.T, s string, writer *testindex.RandomIndexWriter) {
	t.Helper()
	doc := newTestDocument(newTextField(t, "body", s, true))
	mustAddDocument(t, writer, doc)
}

func TestMultiPhraseQueryBooleanQueryContainingSingleTermPrefixQuery(t *testing.T) {
	// this tests against bug 33161 (now fixed)
	// In order to cause the bug, the outer query must have more than one term
	// and all terms required.
	// The contained PhraseMultiQuery must contain exactly one term array.
	indexStore := newDirectory()
	writer := newRandomIndexWriter(t, indexStore)
	addMultiPhraseDoc(t, "blueberry pie", writer)
	addMultiPhraseDoc(t, "blueberry chewing gum", writer)
	addMultiPhraseDoc(t, "blue raspberry pie", writer)

	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)
	// This query will be equivalent to +body:pie +body:"blue*"
	q := search.NewBooleanQueryBuilder()
	q.Add(search.NewTermQuery(index.NewTerm("body", "pie")), search.MUST)

	troubleBuilder := search.NewMultiPhraseQueryBuilder()
	troubleBuilder.AddTerms([]*index.Term{index.NewTerm("body", "blueberry"), index.NewTerm("body", "blue")})
	q.Add(troubleBuilder.Build(), search.MUST)

	// exception will be thrown here without fix
	hits := mustSearch(t, searcher, q.Build(), 1000).ScoreDocs

	if len(hits) != 2 {
		t.Fatalf("Wrong number of hits: expected 2, got %d", len(hits))
	}

	// just make sure no exc:
	if _, err := searcher.Explain(q.Build(), 0); err != nil {
		t.Fatalf("explain: %v", err)
	}

	mustClose(t, writer, reader, indexStore)
}

func TestMultiPhraseQueryPhrasePrefixWithBooleanQuery(t *testing.T) {
	indexStore := newDirectory()
	writer := newRandomIndexWriter(t, indexStore)
	addMultiPhraseTypedDoc(t, "This is a test", "object", writer)
	addMultiPhraseTypedDoc(t, "a note", "note", writer)

	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)

	// This query will be equivalent to +type:note +body:"a t*"
	q := search.NewBooleanQueryBuilder()
	q.Add(search.NewTermQuery(index.NewTerm("type", "note")), search.MUST)

	troubleBuilder := search.NewMultiPhraseQueryBuilder()
	troubleBuilder.Add(index.NewTerm("body", "a"))
	troubleBuilder.AddTerms([]*index.Term{index.NewTerm("body", "test"), index.NewTerm("body", "this")})
	q.Add(troubleBuilder.Build(), search.MUST)

	// exception will be thrown here without fix for #35626:
	hits := mustSearch(t, searcher, q.Build(), 1000).ScoreDocs
	if len(hits) != 0 {
		t.Fatalf("Wrong number of hits: expected 0, got %d", len(hits))
	}
	mustClose(t, writer, reader, indexStore)
}

func TestMultiPhraseQueryNoDocs(t *testing.T) {
	indexStore := newDirectory()
	writer := newRandomIndexWriter(t, indexStore)
	addMultiPhraseTypedDoc(t, "a note", "note", writer)

	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)

	qb := search.NewMultiPhraseQueryBuilder()
	qb.Add(index.NewTerm("body", "a"))
	qb.AddTerms([]*index.Term{index.NewTerm("body", "nope"), index.NewTerm("body", "nope")})
	q := qb.Build()
	if got := mustCount(t, searcher, q); got != 0 {
		t.Fatalf("Wrong number of hits: expected 0, got %d", got)
	}

	// just make sure no exc:
	if _, err := searcher.Explain(q, 0); err != nil {
		t.Fatalf("explain: %v", err)
	}

	mustClose(t, writer, reader, indexStore)
}

func TestMultiPhraseQueryHashCodeAndEquals(t *testing.T) {
	query1builder := search.NewMultiPhraseQueryBuilder()
	query1 := query1builder.Build()

	query2builder := search.NewMultiPhraseQueryBuilder()
	query2 := query2builder.Build()

	assertMultiPhraseEqual(t, query1, query2)

	term1 := index.NewTerm("someField", "someText")

	query1builder.Add(term1)
	query1 = query1builder.Build()

	query2builder.Add(term1)
	query2 = query2builder.Build()

	assertMultiPhraseEqual(t, query1, query2)

	term2 := index.NewTerm("someField", "someMoreText")

	query1builder.Add(term2)
	query1 = query1builder.Build()

	if query1.HashCode() == query2.HashCode() {
		t.Fatal("expected different hash codes")
	}
	if query1.Equals(query2) {
		t.Fatal("expected unequal queries")
	}

	query2builder.Add(term2)
	query2 = query2builder.Build()

	assertMultiPhraseEqual(t, query1, query2)
}

func assertMultiPhraseEqual(t *testing.T, query1, query2 *search.MultiPhraseQuery) {
	t.Helper()
	if query1.HashCode() != query2.HashCode() {
		t.Fatalf("expected equal hash codes, got %d and %d", query1.HashCode(), query2.HashCode())
	}
	if !query1.Equals(query2) {
		t.Fatal("expected equal queries")
	}
}

// addMultiPhraseTypedDoc renders the private add(String, String, RandomIndexWriter).
func addMultiPhraseTypedDoc(t *testing.T, s, typ string, writer *testindex.RandomIndexWriter) {
	t.Helper()
	doc := newTestDocument(
		newTextField(t, "body", s, true),
		newStringField(t, "type", typ, false))
	mustAddDocument(t, writer, doc)
}

// LUCENE-2526
func TestMultiPhraseQueryEmptyToString(t *testing.T) {
	_ = search.NewMultiPhraseQueryBuilder().Build().String()
}

func TestMultiPhraseQueryZeroPosIncr(t *testing.T) {
	var dir store.Directory = store.NewByteBuffersDirectory()
	tokens := []testanalysis.Token{
		testanalysis.NewTokenWithPosInc("a", 1, 0, 0),
		testanalysis.NewTokenWithPosInc("b", 0, 0, 0),
		testanalysis.NewTokenWithPosInc("c", 0, 0, 0),
	}

	writer := newRandomIndexWriter(t, dir)
	doc := newTestDocument(mustTextFieldFromTokenStream(t, "field", testanalysis.NewCannedTokenStream(tokens...)))
	mustAddDocument(t, writer, doc)
	doc = newTestDocument(mustTextFieldFromTokenStream(t, "field", testanalysis.NewCannedTokenStream(tokens...)))
	mustAddDocument(t, writer, doc)
	r := mustGetReader(t, writer)
	mustClose(t, writer)
	s := newSearcher(t, r)
	mpqb := search.NewMultiPhraseQueryBuilder()
	// mpq.setSlop(1);

	// NOTE: not great that if we do the else clause here we
	// get different scores!  MultiPhraseQuery counts that
	// phrase as occurring twice per doc (it should be 1, I
	// think?).  This is because MultipleTermPositions is able to
	// return the same position more than once (0, in this
	// case):
	mpqb.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "b"), index.NewTerm("field", "c")}, 0)
	mpqb.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "a")}, 0)
	hits := mustSearch(t, s, mpqb.Build(), 2)
	if hits.TotalHits.Value != 2 {
		t.Fatalf("expected 2, got %d", hits.TotalHits.Value)
	}
	if math.Abs(float64(hits.ScoreDocs[0].Score-hits.ScoreDocs[1].Score)) > 1e-5 {
		t.Fatalf("scores differ: %v vs %v", hits.ScoreDocs[0].Score, hits.ScoreDocs[1].Score)
	}
	mustClose(t, r, dir)
}

// mustTextFieldFromTokenStream renders new TextField(String, TokenStream).
func mustTextFieldFromTokenStream(t *testing.T, name string, stream *testanalysis.CannedTokenStream) *document.TextField {
	t.Helper()
	f, err := document.NewTextFieldFromTokenStream(name, stream)
	if err != nil {
		t.Fatalf("new TextField: %v", err)
	}
	return f
}

// makeMultiPhraseToken renders the private makeToken(String, int).
func makeMultiPhraseToken(text string, posIncr int) testanalysis.Token {
	return testanalysis.NewTokenWithPosInc(text, posIncr, 0, 0)
}

var incr0DocTokens = []testanalysis.Token{
	makeMultiPhraseToken("x", 1),
	makeMultiPhraseToken("a", 1),
	makeMultiPhraseToken("1", 0),
	makeMultiPhraseToken("m", 1), // not existing, relying on slop=2
	makeMultiPhraseToken("b", 1),
	makeMultiPhraseToken("1", 0),
	makeMultiPhraseToken("n", 1), // not existing, relying on slop=2
	makeMultiPhraseToken("c", 1),
	makeMultiPhraseToken("y", 1),
}

var incr0QueryTokensAnd = []testanalysis.Token{
	makeMultiPhraseToken("a", 1),
	makeMultiPhraseToken("1", 0),
	makeMultiPhraseToken("b", 1),
	makeMultiPhraseToken("1", 0),
	makeMultiPhraseToken("c", 1),
}

var incr0QueryTokensAndOrMatch = [][]testanalysis.Token{
	{makeMultiPhraseToken("a", 1)},
	{makeMultiPhraseToken("x", 1), makeMultiPhraseToken("1", 0)},
	{makeMultiPhraseToken("b", 2)},
	{makeMultiPhraseToken("x", 2), makeMultiPhraseToken("1", 0)},
	{makeMultiPhraseToken("c", 3)},
}

var incr0QueryTokensAndOrNoMatchn = [][]testanalysis.Token{
	{makeMultiPhraseToken("x", 1)},
	{makeMultiPhraseToken("a", 1), makeMultiPhraseToken("1", 0)},
	{makeMultiPhraseToken("x", 2)},
	{makeMultiPhraseToken("b", 2), makeMultiPhraseToken("1", 0)},
	{makeMultiPhraseToken("c", 3)},
}

// using query parser, MPQ will be created, and will not be strict about
// having all query terms in each position - one of each position is
// sufficient (OR logic).
func TestMultiPhraseQueryZeroPosIncrSloppyParsedAnd(t *testing.T) {
	qb := search.NewMultiPhraseQueryBuilder()
	qb.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "a"), index.NewTerm("field", "1")}, -1)
	qb.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "b"), index.NewTerm("field", "1")}, 0)
	qb.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "c")}, 1)
	doTestZeroPosIncrSloppy(t, qb.Build(), 0)
	qb.SetSlop(1)
	doTestZeroPosIncrSloppy(t, qb.Build(), 0)
	qb.SetSlop(2)
	doTestZeroPosIncrSloppy(t, qb.Build(), 1)
}

// doTestZeroPosIncrSloppy renders the private doTestZeroPosIncrSloppy(Query, int).
func doTestZeroPosIncrSloppy(t *testing.T, q search.Query, nExpected int) {
	t.Helper()
	dir := newDirectory() // random dir
	cfg := newIndexWriterConfigWithAnalyzer(nil)
	writer := mustNewIndexWriter(t, dir, cfg)
	doc := newTestDocument(mustTextFieldFromTokenStream(t, "field", testanalysis.NewCannedTokenStream(incr0DocTokens...)))
	mustAddDocument(t, writer, doc)
	r, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("DirectoryReader.open(writer): %v", err)
	}
	mustClose(t, writer)
	s := newSearcher(t, r)

	hits := mustSearch(t, s, q, 1)
	if hits.TotalHits.Value != int64(nExpected) {
		t.Fatalf("wrong number of results: expected %d, got %d", nExpected, hits.TotalHits.Value)
	}

	mustClose(t, r, dir)
}

// PQ AND Mode - Manually creating a phrase query.
func TestMultiPhraseQueryZeroPosIncrSloppyPqAnd(t *testing.T) {
	builder := search.NewPhraseQueryBuilder()
	pos := -1
	for _, tap := range incr0QueryTokensAnd {
		pos += tap.PositionIncrement
		builder.AddWithPosition(index.NewTerm("field", tap.Text), pos)
	}
	builder.SetSlop(0)
	doTestZeroPosIncrSloppy(t, builder.Build(), 0)
	builder.SetSlop(1)
	doTestZeroPosIncrSloppy(t, builder.Build(), 0)
	builder.SetSlop(2)
	doTestZeroPosIncrSloppy(t, builder.Build(), 1)
}

// MPQ AND Mode - Manually creating a multiple phrase query.
func TestMultiPhraseQueryZeroPosIncrSloppyMpqAnd(t *testing.T) {
	mpqb := search.NewMultiPhraseQueryBuilder()
	pos := -1
	for _, tap := range incr0QueryTokensAnd {
		pos += tap.PositionIncrement
		mpqb.AddTermsAtPosition([]*index.Term{index.NewTerm("field", tap.Text)}, pos) // AND logic
	}
	doTestZeroPosIncrSloppy(t, mpqb.Build(), 0)
	mpqb.SetSlop(1)
	doTestZeroPosIncrSloppy(t, mpqb.Build(), 0)
	mpqb.SetSlop(2)
	doTestZeroPosIncrSloppy(t, mpqb.Build(), 1)
}

// MPQ Combined AND OR Mode - Manually creating a multiple phrase query.
func TestMultiPhraseQueryZeroPosIncrSloppyMpqAndOrMatch(t *testing.T) {
	mpqb := search.NewMultiPhraseQueryBuilder()
	for _, tap := range incr0QueryTokensAndOrMatch {
		terms := tapTerms(tap)
		pos := tap[0].PositionIncrement - 1
		mpqb.AddTermsAtPosition(terms, pos) // AND logic in pos, OR across lines
	}
	doTestZeroPosIncrSloppy(t, mpqb.Build(), 0)
	mpqb.SetSlop(1)
	doTestZeroPosIncrSloppy(t, mpqb.Build(), 0)
	mpqb.SetSlop(2)
	doTestZeroPosIncrSloppy(t, mpqb.Build(), 1)
}

// MPQ Combined AND OR Mode - Manually creating a multiple phrase query - with no match.
func TestMultiPhraseQueryZeroPosIncrSloppyMpqAndOrNoMatch(t *testing.T) {
	mpqb := search.NewMultiPhraseQueryBuilder()
	for _, tap := range incr0QueryTokensAndOrNoMatchn {
		terms := tapTerms(tap)
		pos := tap[0].PositionIncrement - 1
		mpqb.AddTermsAtPosition(terms, pos) // AND logic in pos, OR across lines
	}
	doTestZeroPosIncrSloppy(t, mpqb.Build(), 0)
	mpqb.SetSlop(2)
	doTestZeroPosIncrSloppy(t, mpqb.Build(), 0)
}

// tapTerms renders the private tapTerms(Token[]).
func tapTerms(tap []testanalysis.Token) []*index.Term {
	terms := make([]*index.Term, len(tap))
	for i := range terms {
		terms[i] = index.NewTerm("field", tap[i].Text)
	}
	return terms
}

func TestMultiPhraseQueryNegativeSlop(t *testing.T) {
	queryBuilder := search.NewMultiPhraseQueryBuilder()
	queryBuilder.Add(index.NewTerm("field", "two"))
	queryBuilder.Add(index.NewTerm("field", "one"))
	expectThrowsPanic(t, func() {
		queryBuilder.SetSlop(-2)
	})
}
