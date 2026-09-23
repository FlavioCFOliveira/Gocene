// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestTryDelete.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// tryDeleteDocumentMissing names the IndexWriter overload the Java tests
// call: Gocene's TryDeleteDocument takes no reader and returns no sequence
// number.
const tryDeleteDocumentMissing = "org.apache.lucene.index.IndexWriter#tryDeleteDocument(IndexReader, int) is not ported"

func tryDeleteGetWriter(t *testing.T, directory store.Directory) *index.IndexWriter {
	t.Helper()
	policy := index.NewLogByteSizeMergePolicy()
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(policy)
	conf.SetOpenMode(index.CreateOrAppend)

	return mustNewIndexWriter(t, directory, conf)
}

func tryDeleteCreateIndex(t *testing.T) store.Directory {
	t.Helper()
	directory := store.NewByteBuffersDirectory()

	writer := tryDeleteGetWriter(t, directory)

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("foo", strconv.Itoa(i), true)
		if err != nil {
			t.Fatalf("StringField: %v", err)
		}
		doc.Add(f)
		mustAddDocument(t, writer, doc)
	}

	mustCommit(t, writer)
	mustClose(t, writer)

	return directory
}

func assertFooHits(t *testing.T, searcher *search.IndexSearcher, expected int64) {
	t.Helper()
	topDocs, err := searcher.Search(search.NewTermQuery(index.NewTerm("foo", "0")), 100)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if topDocs.TotalHits.Value != expected {
		t.Fatalf("totalHits.value(): expected %d, got %d", expected, topDocs.TotalHits.Value)
	}
}

func newTryDeleteSearcherManager(t *testing.T, writer *index.IndexWriter) *search.SearcherManager {
	t.Helper()
	mgr, err := search.NewSearcherManager(writer, search.NewSearcherFactory())
	if err != nil {
		t.Fatalf("new SearcherManager: %v", err)
	}
	return mgr
}

func mustAcquire(t *testing.T, mgr *search.SearcherManager) *search.IndexSearcher {
	t.Helper()
	searcher, err := mgr.Acquire()
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	return searcher
}

func TestTryDeleteTryDeleteDocument(t *testing.T) {
	directory := tryDeleteCreateIndex(t)

	writer := tryDeleteGetWriter(t, directory)

	mgr := newTryDeleteSearcherManager(t, writer)

	searcher := mustAcquire(t, mgr)

	assertFooHits(t, searcher, 1)

	t.Fatal(tryDeleteDocumentMissing)
}

func TestTryDeleteTryDeleteDocumentCloseAndReopen(t *testing.T) {
	directory := tryDeleteCreateIndex(t)

	writer := tryDeleteGetWriter(t, directory)

	mgr := newTryDeleteSearcherManager(t, writer)

	searcher := mustAcquire(t, mgr)

	assertFooHits(t, searcher, 1)

	t.Fatal(tryDeleteDocumentMissing)
}

func TestTryDeleteDeleteDocuments(t *testing.T) {
	directory := tryDeleteCreateIndex(t)

	writer := tryDeleteGetWriter(t, directory)

	mgr := newTryDeleteSearcherManager(t, writer)

	searcher := mustAcquire(t, mgr)

	assertFooHits(t, searcher, 1)

	result, err := writer.DeleteDocumentsQuery([]index.Query{search.NewTermQuery(index.NewTerm("foo", "0"))})
	if err != nil {
		t.Fatalf("deleteDocuments(Query): %v", err)
	}

	if !(result != -1) {
		t.Fatal("assertTrue(result != -1)")
	}

	// writer.commit();

	hasDeletions, err := writer.HasDeletions()
	if err != nil {
		t.Fatalf("hasDeletions: %v", err)
	}
	if !hasDeletions {
		t.Fatal("assertTrue(writer.hasDeletions())")
	}

	if _, err := mgr.MaybeRefresh(); err != nil {
		t.Fatalf("maybeRefresh: %v", err)
	}

	searcher = mustAcquire(t, mgr)

	assertFooHits(t, searcher, 0)
}
