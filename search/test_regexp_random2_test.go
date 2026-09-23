// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestRegexpRandom2.java
// (Apache Lucene 10.5.0).
//
// Create an index with random unicode terms Generates random regexps, and
// validates against a simple impl.
//
// TestFieldCacheRewriteMethod extends this class and overrides assertSame;
// regexpRandom2TestCase.assertSameFn renders that virtual dispatch.

package search_test

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// automatonTestUtilBlocker names
// org.apache.lucene.tests.util.automaton.AutomatonTestUtil.
const automatonTestUtilBlocker = "requires org.apache.lucene.tests.util.automaton.AutomatonTestUtil (not ported)"

// regexpRandom2TestCase renders the fields of TestRegexpRandom2.
type regexpRandom2TestCase struct {
	searcher1 *search.IndexSearcher
	searcher2 *search.IndexSearcher
	searcher3 *search.IndexSearcher
	reader    *index.DirectoryReader
	fieldName string

	// assertSameFn renders the overridable assertSame(String); nil keeps
	// TestRegexpRandom2's own body.
	assertSameFn func(t *testing.T, regexp string)
}

// rr2SetUp renders setUp(); the returned function renders tearDown().
func rr2SetUp(t *testing.T) (*regexpRandom2TestCase, func()) {
	t.Helper()
	tc := &regexpRandom2TestCase{}
	dir := newDirectory()
	if random().Intn(2) == 0 {
		tc.fieldName = "field"
	} else {
		tc.fieldName = "" // sometimes use an empty string as field name
	}
	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.KEYWORD, false, 0, nil, true))
	iwc.SetMaxBufferedDocs(nextInt(50, 1000))
	writer := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	field := newStringField(t, tc.fieldName, "", false)
	doc.Add(field)
	dvField, err := document.NewSortedDocValuesField(tc.fieldName, []byte{})
	if err != nil {
		t.Fatalf("new SortedDocValuesField: %v", err)
	}
	doc.Add(dvField)
	var terms []string

	num := atLeast(200)
	for i := 0; i < num; i++ {
		s := randomUnicodeString(random())
		field.SetStringValue(s)
		dvField.SetBytesValue([]byte(s))
		terms = append(terms, s)
		mustAddDocument(t, writer, doc)
	}

	if testing.Verbose() {
		// utf16 order
		sort.Slice(terms, func(i, j int) bool { return javaStringLess(terms[i], terms[j]) })
		t.Log("UTF16 order:")
		for _, s := range terms {
			t.Logf("  %s", util.ToHexStringCodePoints(s))
		}
	}

	tc.reader = mustGetReader(t, writer)
	tearDown := func() {
		mustClose(t, tc.reader, dir)
	}
	tc.searcher1 = newSearcher(t, tc.reader)
	tc.searcher2 = newSearcher(t, tc.reader)
	tc.searcher3 = newSearcher(t, tc.reader)
	mustClose(t, writer)
	return tc, tearDown
}

// javaStringLess renders String.compareTo(String) < 0: UTF-16 code unit order.
func javaStringLess(a, b string) bool {
	ua, ub := utf16Units(a), utf16Units(b)
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// dumbRegexpQuery renders the private static class DumbRegexpQuery: a stupid
// regexp query that just blasts thru the terms.
type dumbRegexpQuery struct {
	*search.MultiTermQuery
	automaton *automaton.Automaton
}

// newDumbRegexpQuery renders DumbRegexpQuery(Term, int flags).
func newDumbRegexpQuery(t *testing.T, term *index.Term, flags int) *dumbRegexpQuery {
	t.Helper()
	q := &dumbRegexpQuery{MultiTermQuery: search.NewMultiTermQuery(term.Field, search.ConstantScoreBlendedRewrite)}
	re, err := automaton.NewRegExpSyntax(term.Text(), flags)
	if err != nil {
		t.Fatalf("new RegExp: %v", err)
	}
	a, err := re.ToAutomaton()
	if err != nil {
		t.Fatalf("toAutomaton: %v", err)
	}
	q.automaton, err = automaton.Determinize(a, automaton.DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize: %v", err)
	}
	q.MultiTermQuery.SetOwner(q)
	return q
}

// GetTermsEnumWithAttributes renders getTermsEnum(Terms, AttributeSource).
func (q *dumbRegexpQuery) GetTermsEnumWithAttributes(terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error) {
	it, err := terms.Iterator()
	if err != nil {
		return nil, err
	}
	return q.newSimpleAutomatonTermsEnum(it), nil
}

// simpleAutomatonTermsEnum renders DumbRegexpQuery.SimpleAutomatonTermsEnum.
type simpleAutomatonTermsEnum struct {
	*index.FilteredTermsEnum
	runAutomaton *automaton.CharacterRunAutomaton
	utf16        *util.CharsRefBuilder
}

func (q *dumbRegexpQuery) newSimpleAutomatonTermsEnum(tenum index.TermsEnum) *simpleAutomatonTermsEnum {
	e := &simpleAutomatonTermsEnum{
		runAutomaton: automaton.NewCharacterRunAutomaton(q.automaton),
		utf16:        util.NewCharsRefBuilder(),
	}
	e.FilteredTermsEnum = index.NewFilteredTermsEnum(tenum, e)
	e.SetInitialSeekTerm(index.NewTermFromBytes("", []byte("")))
	return e
}

// Accept renders accept(BytesRef).
func (e *simpleAutomatonTermsEnum) Accept(term *spi.Term) (index.AcceptStatus, error) {
	b := term.BytesValue()
	e.utf16.CopyUTF8Bytes(b.Bytes, b.Offset, b.Length)
	if e.runAutomaton.RunRunes(e.utf16.Chars(), 0, e.utf16.Length()) {
		return index.AcceptYes, nil
	}
	return index.AcceptNo, nil
}

// NextSeekTerm keeps the inherited FilteredTermsEnum.nextSeekTerm.
func (e *simpleAutomatonTermsEnum) NextSeekTerm(current *spi.Term) (*spi.Term, error) {
	return nil, nil
}

// ToString renders toString(String field).
func (q *dumbRegexpQuery) ToString(field string) string {
	return field + q.automaton.String()
}

func (q *dumbRegexpQuery) String() string { return q.ToString("") }

// Visit renders visit(QueryVisitor), which is empty.
func (q *dumbRegexpQuery) Visit(visitor search.QueryVisitor) {}

// Equals renders equals(Object).
func (q *dumbRegexpQuery) Equals(obj spi.Query) bool {
	if !q.MultiTermQuery.Equals(obj) {
		return false
	}
	that := obj.(*dumbRegexpQuery)
	return q.automaton.Equals(that.automaton)
}

// rr2TestRegexps renders testRegexps(): test a bunch of random regular
// expressions.
func (tc *regexpRandom2TestCase) testRegexps(t *testing.T) {
	num := atLeast(200)
	for i := 0; i < num; i++ {
		// String reg = AutomatonTestUtil.randomRegexp(random());
		t.Fatal(automatonTestUtilBlocker)
		var reg string
		if testing.Verbose() {
			t.Logf("TEST: regexp='%s'", reg)
		}
		tc.assertSame(t, reg)
	}
}

// assertSame dispatches the overridable assertSame(String).
func (tc *regexpRandom2TestCase) assertSame(t *testing.T, regexp string) {
	t.Helper()
	if tc.assertSameFn != nil {
		tc.assertSameFn(t, regexp)
		return
	}
	tc.rr2AssertSame(t, regexp)
}

// rr2AssertSame renders TestRegexpRandom2.assertSame(String): check that the #
// of hits is the same as from a very simple regexpquery implementation.
func (tc *regexpRandom2TestCase) rr2AssertSame(t *testing.T, regexp string) {
	t.Helper()
	smart := search.NewRegexpQueryWithFlags(index.NewTerm(tc.fieldName, regexp), automaton.RegExpNone)
	nfaQuery := search.NewRegexpQueryFull(
		index.NewTerm(tc.fieldName, regexp),
		automaton.RegExpNone,
		0,
		search.DefaultAutomatonProvider,
		0,
		// TODO: The NFA query is not able to use rewrite method that will utilize the
		// concurrency
		search.ConstantScoreBooleanRewrite,
		false)
	dumb := newDumbRegexpQuery(t, index.NewTerm(tc.fieldName, regexp), automaton.RegExpNone)

	smartDocs := mustSearch(t, tc.searcher1, smart, 25)
	dumbDocs := mustSearch(t, tc.searcher2, dumb, 25)
	nfaDocs := mustSearch(t, tc.searcher3, nfaQuery, 25)

	testsearch.CheckEqual(t, smart, smartDocs.ScoreDocs, dumbDocs.ScoreDocs)
	testsearch.CheckEqual(t, nfaQuery, nfaDocs.ScoreDocs, dumbDocs.ScoreDocs)
}

func TestRegexpRandom2Regexps(t *testing.T) {
	tc, tearDown := rr2SetUp(t)
	defer tearDown()
	tc.testRegexps(t)
}
