// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestDocValuesRewriteMethod.java
// (Apache Lucene 10.5.0): tests the DocValuesRewriteMethod.

package search_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// sortedSetIndexedFieldBlocker names the production member setUp calls.
const sortedSetIndexedFieldBlocker = "requires org.apache.lucene.document.SortedSetDocValuesField.indexedField(" +
	"String, BytesRef) (not ported)"

// docValuesRewriteMethodFixture holds the fields of TestDocValuesRewriteMethod.
type docValuesRewriteMethodFixture struct {
	searcher  *search.IndexSearcher
	fieldName string
}

// setUpDocValuesRewriteMethod renders TestDocValuesRewriteMethod.setUp();
// tearDown is registered with t.Cleanup.
func setUpDocValuesRewriteMethod(t *testing.T) *docValuesRewriteMethodFixture {
	t.Helper()
	f := &docValuesRewriteMethodFixture{}
	dir := newDirectory()
	t.Cleanup(func() {
		if err := dir.Close(); err != nil {
			t.Errorf("close dir: %v", err)
		}
	})
	if random().Intn(2) == 0 {
		f.fieldName = "field"
	} else {
		f.fieldName = "" // sometimes use an empty string as field name
	}
	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.KEYWORD, false, 0, nil, true))
	iwc.SetMaxBufferedDocs(nextInt(50, 1000))
	writer := newRandomIndexWriterWithConfig(t, dir, iwc)
	var terms []string
	num := atLeast(200)
	for i := 0; i < num; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		numTerms := random().Intn(4)
		for j := 0; j < numTerms; j++ {
			s := randomUnicodeString(random())
			doc.Add(newStringField(t, f.fieldName, s, false))
			dv, err := document.NewSortedSetDocValuesField(f.fieldName, [][]byte{[]byte(s)})
			if err != nil {
				t.Fatalf("new SortedSetDocValuesField: %v", err)
			}
			doc.Add(dv)
			// doc.add(SortedSetDocValuesField.indexedField(fieldName + "_with-skip", new BytesRef(s)));
			mustClose(t, writer)
			t.Fatal(sortedSetIndexedFieldBlocker)
			terms = append(terms, s)
		}
		mustAddDocument(t, writer, doc)
	}

	numDeletions := random().Intn(num / 10)
	for i := 0; i < numDeletions; i++ {
		if _, err := writer.DeleteDocuments(index.NewTerm("id", strconv.Itoa(random().Intn(num)))); err != nil {
			t.Fatalf("deleteDocuments: %v", err)
		}
	}

	reader := mustGetReader(t, writer)
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close reader: %v", err)
		}
	})
	f.searcher = newSearcher(t, reader)
	mustClose(t, writer)
	return f
}

// test a bunch of random regular expressions
func TestDocValuesRewriteMethodRegexps(t *testing.T) {
	f := setUpDocValuesRewriteMethod(t)
	num := atLeast(1000)
	for i := 0; i < num; i++ {
		t.Fatal("requires org.apache.lucene.tests.util.automaton.AutomatonTestUtil.randomRegexp(Random) (not ported)")
		f.assertSame(t, "")
	}
}

// assertSame checks that the # of hits is the same as if the query is run
// against the inverted index.
func (f *docValuesRewriteMethodFixture) assertSame(t *testing.T, regexp string) {
	t.Helper()
	docValues := search.NewRegexpQueryFull(
		index.NewTerm(f.fieldName, regexp),
		automaton.RegExpNone,
		0,
		nil,
		automaton.DefaultDeterminizeWorkLimit,
		search.NewDocValuesRewriteMethod(),
		true)
	docValuesWithSkip := search.NewRegexpQueryFull(
		index.NewTerm(f.fieldName+"_with-skip", regexp),
		automaton.RegExpNone,
		0,
		nil,
		automaton.DefaultDeterminizeWorkLimit,
		search.NewDocValuesRewriteMethod(),
		true)
	inverted := search.NewRegexpQueryWithFlags(index.NewTerm(f.fieldName, regexp), automaton.RegExpNone)

	invertedDocs := mustSearch(t, f.searcher, inverted, 25)
	docValuesDocs := mustSearch(t, f.searcher, docValues, 25)
	docValuesWithSkipDocs := mustSearch(t, f.searcher, docValuesWithSkip, 25)

	testsearch.CheckEqual(t, inverted, invertedDocs.ScoreDocs, docValuesDocs.ScoreDocs)
	testsearch.CheckEqual(t, inverted, invertedDocs.ScoreDocs, docValuesWithSkipDocs.ScoreDocs)
}

func TestDocValuesRewriteMethodEquals(t *testing.T) {
	f := setUpDocValuesRewriteMethod(t)
	{
		a1 := search.NewRegexpQueryWithFlags(index.NewTerm(f.fieldName, "[aA]"), automaton.RegExpNone)
		a2 := search.NewRegexpQueryWithFlags(index.NewTerm(f.fieldName, "[aA]"), automaton.RegExpNone)
		b := search.NewRegexpQueryWithFlags(index.NewTerm(f.fieldName, "[bB]"), automaton.RegExpNone)
		if !a1.Equals(a2) {
			t.Fatal("a1 != a2")
		}
		if a1.Equals(b) {
			t.Fatal("a1 == b")
		}
	}

	{
		a1 := search.NewRegexpQueryFull(
			index.NewTerm(f.fieldName, "[aA]"),
			automaton.RegExpNone,
			0,
			nil,
			automaton.DefaultDeterminizeWorkLimit,
			search.NewDocValuesRewriteMethod(),
			true)
		a2 := search.NewRegexpQueryFull(
			index.NewTerm(f.fieldName, "[aA]"),
			automaton.RegExpNone,
			0,
			nil,
			automaton.DefaultDeterminizeWorkLimit,
			search.NewDocValuesRewriteMethod(),
			true)
		b := search.NewRegexpQueryFull(
			index.NewTerm(f.fieldName, "[bB]"),
			automaton.RegExpNone,
			0,
			nil,
			automaton.DefaultDeterminizeWorkLimit,
			search.NewDocValuesRewriteMethod(),
			true)
		if !a1.Equals(a2) {
			t.Fatal("a1 != a2")
		}
		if a1.Equals(b) {
			t.Fatal("a1 == b")
		}
		queryUtilsCheck(t, a1)
	}
}
