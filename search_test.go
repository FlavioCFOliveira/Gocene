// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/TestSearch.java
// (Apache Lucene 10.5.0): JUnit adaptation of an older test case SearchTest.

package gocene

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

// testSearchNewSearcherBlocker names the class LuceneTestCase.newSearcher builds.
const testSearchNewSearcherBlocker = "requires org.apache.lucene.tests.search.AssertingIndexSearcher " +
	"(built by LuceneTestCase.newSearcher(IndexReader)) (not ported)"

// TestSearch performs a number of searches. It also compares output of
// searches using multi-file index segments with single-file index segments.
//
// TODO: someone should check that the results of the searches are still
// correct by adding assert statements. Right now, the test passes if the
// results are the same between multi-file and single-file formats, even if
// the results are wrong.
func TestSearch(t *testing.T) {
	r := rand.New(rand.NewSource(rand.Int63()))
	var sw strings.Builder
	doTestSearch(t, r, &sw, false)
	multiFileOutput := sw.String()
	// System.out.println(multiFileOutput);

	sw.Reset()
	doTestSearch(t, r, &sw, true)
	singleFileOutput := sw.String()

	if multiFileOutput != singleFileOutput {
		t.Fatalf("assertEquals(multiFileOutput, singleFileOutput):\n%s\n---\n%s", multiFileOutput, singleFileOutput)
	}
}

// doTestSearch renders the private doTestSearch(Random, PrintWriter, boolean).
func doTestSearch(t *testing.T, random *rand.Rand, out *strings.Builder, useCompoundFile bool) {
	t.Helper()
	directory := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory()) // newDirectory()
	analyzer := testanalysis.NewMockAnalyzerRandom(random)
	conf := index.NewIndexWriterConfigWithAnalyzer(analyzer) // newIndexWriterConfig(analyzer)
	mp := conf.GetMergePolicy()
	noCFSRatio := 0.0
	if useCompoundFile {
		noCFSRatio = 1.0
	}
	mp.(interface{ SetNoCFSRatio(float64) }).SetNoCFSRatio(noCFSRatio)
	writer, err := index.NewIndexWriter(directory, conf)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}

	docs := []string{
		"a b c d e",
		"a b c d e a b c d e",
		"a b c d e f g h i j",
		"a c e",
		"e c a",
		"a c e a c e",
		"a c e a b c",
	}
	for j := 0; j < len(docs); j++ {
		d := document.NewDocument()
		contents, err := document.NewTextField("contents", docs[j], true) // newTextField
		if err != nil {
			t.Fatalf("newTextField: %v", err)
		}
		d.Add(contents)
		id, err := document.NewNumericDocValuesField("id", int64(j))
		if err != nil {
			t.Fatalf("new NumericDocValuesField: %v", err)
		}
		d.Add(id)
		if _, err := writer.AddDocument(d); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(directory)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	// IndexSearcher searcher = newSearcher(reader);
	t.Fatal(testSearchNewSearcherBlocker)
	var searcher *search.IndexSearcher

	sort := search.NewSort(search.FieldScore, search.NewSortField("id", spi.SortFieldTypeInt))

	for _, query := range buildQueries() {
		fmt.Fprintf(out, "Query: %s\n", testsearch.QueryString(query, "contents"))
		if testing.Verbose() {
			t.Logf("TEST: query=%v", query)
		}

		topDocs, err := searcher.SearchWithSort(query, 1000, sort, false)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		hits := topDocs.ScoreDocs
		fmt.Fprintf(out, "%d total results\n", len(hits))
		storedFields, err := searcher.StoredFields()
		if err != nil {
			t.Fatalf("storedFields: %v", err)
		}
		for i := 0; i < len(hits) && i < 10; i++ {
			visitor := document.NewDocumentStoredFieldVisitor()
			if err := storedFields.Document(hits[i].Doc, visitor); err != nil {
				t.Fatalf("document(%d): %v", hits[i].Doc, err)
			}
			d := visitor.GetDocument()
			fmt.Fprintf(out, "%d %v %s\n", i, hits[i].Score, d.Get("contents").StringValue())
		}
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	if err := directory.Close(); err != nil {
		t.Fatalf("close directory: %v", err)
	}
}

// buildQueries renders the private buildQueries().
func buildQueries() []search.Query {
	var queries []search.Query

	booleanAB := search.NewBooleanQueryBuilder()
	booleanAB.Add(search.NewTermQuery(index.NewTerm("contents", "a")), search.SHOULD)
	booleanAB.Add(search.NewTermQuery(index.NewTerm("contents", "b")), search.SHOULD)
	queries = append(queries, booleanAB.Build())

	phraseAB := search.NewPhraseQuery(0, "contents", "a", "b")
	queries = append(queries, phraseAB)

	phraseABC := search.NewPhraseQuery(0, "contents", "a", "b", "c")
	queries = append(queries, phraseABC)

	booleanAC := search.NewBooleanQueryBuilder()
	booleanAC.Add(search.NewTermQuery(index.NewTerm("contents", "a")), search.SHOULD)
	booleanAC.Add(search.NewTermQuery(index.NewTerm("contents", "c")), search.SHOULD)
	queries = append(queries, booleanAC.Build())

	phraseAC := search.NewPhraseQuery(0, "contents", "a", "c")
	queries = append(queries, phraseAC)

	phraseACE := search.NewPhraseQuery(0, "contents", "a", "c", "e")
	queries = append(queries, phraseACE)

	return queries
}
