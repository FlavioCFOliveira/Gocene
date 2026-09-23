// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package expressions_test

// Port of
// lucene/expressions/src/test/org/apache/lucene/expressions/TestExpressionRescorer.java
// (Apache Lucene 10.5.0).
//
// Blocker: the Java test binds the expression variables with
// SimpleBindings.add(String, org.apache.lucene.search.DoubleValuesSource)
// (DoubleValuesSource.fromIntField and DoubleValuesSource.SCORES) and rescores
// through ExpressionRescorer, which extends SortRescorer with an
// expression-backed SortField. Gocene's expressions.SimpleBindings takes a
// Gocene-only ValueSource and expressions.Expression.GetSortField returns a
// plain SCORE sort field, so the rescoring half of testBasic fails naming the
// missing Lucene classes.

import (
	_ "github.com/FlavioCFOliveira/Gocene/codecs" // registers the default codec (Codec.getDefault())
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/expressions/js"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// expressionRescorerFixture renders the TestExpressionRescorer fields
// searcher, reader and dir, built by setUp().
type expressionRescorerFixture struct {
	searcher *search.IndexSearcher
	reader   *index.DirectoryReader
	dir      store.Directory
}

func newExpressionRescorerFixture(t *testing.T) *expressionRescorerFixture {
	t.Helper()
	f := &expressionRescorerFixture{}
	f.dir = store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	iwc := index.NewIndexWriterConfigWithAnalyzer(
		testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true))
	iwc.SetSimilarity(search.NewClassicSimilarity())
	iw, err := testindex.NewRandomIndexWriterWithConfig(rand.New(rand.NewSource(rand.Int63())), f.dir, iwc)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}

	addDoc := func(id, body string, popularity int64) {
		t.Helper()
		doc := document.NewDocument()
		idField, err := document.NewStringField("id", id, true)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(idField)
		bodyField, err := document.NewTextField("body", body, false)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(bodyField)
		pop, err := document.NewNumericDocValuesField("popularity", popularity)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(pop)
		if _, err := iw.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	addDoc("1", "some contents and more contents", 5)
	addDoc("2", "another document with different contents", 20)
	addDoc("3", "crappy contents", 2)

	f.reader, err = iw.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	f.searcher = search.NewIndexSearcher(f.reader)
	// TODO: fix this test to not be so flaky and use newSearcher
	f.searcher.SetSimilarity(search.NewClassicSimilarity())
	if err := iw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	t.Cleanup(func() {
		// tearDown()
		if err := f.reader.Close(); err != nil {
			t.Errorf("close reader: %v", err)
		}
		if err := f.dir.Close(); err != nil {
			t.Errorf("close dir: %v", err)
		}
	})
	return f
}

func storedID(t *testing.T, r *index.DirectoryReader, docID int) string {
	t.Helper()
	storedFields, err := r.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	visitor := document.NewDocumentStoredFieldVisitor()
	if err := storedFields.Document(docID, visitor); err != nil {
		t.Fatalf("document(%d): %v", docID, err)
	}
	f := visitor.GetDocument().Get("id")
	if f == nil {
		t.Fatalf("document(%d) has no id", docID)
	}
	return f.StringValue()
}

func TestExpressionRescorer_testBasic(t *testing.T) {
	f := newExpressionRescorerFixture(t)

	// create a sort field and sort by it (reverse order)
	query := search.NewTermQuery(index.NewTerm("body", "contents"))
	r := f.reader

	// Just first pass query
	hits, err := f.searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	assertTotalHits(t, hits, 3)
	for i, want := range []string{"3", "1", "2"} {
		if got := storedID(t, r, hits.ScoreDocs[i].Doc); got != want {
			t.Fatalf("first pass hit %d: id %q, want %q", i, got, want)
		}
	}

	// Now, rescore:

	e, err := js.JavascriptCompiler{}.Compile("sqrt(_score) + ln(popularity)")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	_ = e
	t.Fatal("requires org.apache.lucene.expressions.SimpleBindings.add(String, " +
		"org.apache.lucene.search.DoubleValuesSource) with DoubleValuesSource.fromIntField and " +
		"DoubleValuesSource.SCORES, and ExpressionRescorer extending SortRescorer over the " +
		"expression SortField (not ported)")
}

func assertTotalHits(t *testing.T, hits *search.TopDocs, want int64) {
	t.Helper()
	var th *spi.TotalHits = hits.TotalHits
	if th == nil || th.Value != want {
		t.Fatalf("totalHits = %v, want %d", th, want)
	}
}
