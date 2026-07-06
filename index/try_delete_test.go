// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestTryDelete
// Source: lucene/core/src/test/org/apache/lucene/index/TestTryDelete.java
// Reference tag: releases/lucene/10.4.0 (commit 9983b7c)
//
// GOC-4213 (Sprint 55, option c): every Java test method has a corresponding
// Go test function. Tests whose dependencies are not yet ported to Gocene
// call t.Skip with a precise reason; the index-building helpers are still
// exercised so the port stays compilable and the gaps are explicit.
//
package index_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTryDeleteWriter mirrors TestTryDelete.getWriter: a writer configured with
// a LogByteSizeMergePolicy and OpenMode.CREATE_OR_APPEND. Lucene uses
// MockAnalyzer(random()); Gocene's WhitespaceAnalyzer is the closest faithful
// equivalent.
func newTryDeleteWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()

	conf := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	conf.SetMergePolicy(index.NewLogByteSizeMergePolicy())
	conf.SetOpenMode(index.CREATE_OR_APPEND)

	writer, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return writer
}

// createTryDeleteIndex mirrors TestTryDelete.createIndex: ten documents, each
// with a stored "foo" field holding its ordinal, committed and closed.
func createTryDeleteIndex(t *testing.T) store.Directory {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	writer := newTryDeleteWriter(t, dir)

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		fooField, err := document.NewStringField("foo", fmt.Sprintf("%d", i), true)
		if err != nil {
			t.Fatalf("NewStringField(foo): %v", err)
		}
		doc.Add(fooField)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return dir
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------


// TestDeleteDocuments ports TestTryDelete.testDeleteDocuments.
func TestDeleteDocuments(t *testing.T) {
	dir := createTryDeleteIndex(t)
	writer := newTryDeleteWriter(t, dir)
	defer writer.Close()

	// Delete the document whose "foo" value is "7" via a TermQuery.
	q := search.NewTermQuery(index.NewTerm("foo", "7"))
	if err := writer.DeleteDocumentsQuery(q); err != nil {
		t.Fatalf("DeleteDocumentsQuery: %v", err)
	}
	if !writer.HasDeletions() {
		t.Fatal("expected HasDeletions() to be true after a query delete")
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	topDocs, err := searcher.Search(q, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 0 {
		t.Fatalf("expected 0 hits after delete, got %d", topDocs.TotalHits.Value)
	}
	if reader.NumDocs() != 9 {
		t.Fatalf("expected NumDocs=9 after deleting one of ten, got %d", reader.NumDocs())
	}
}

// TestTryDeleteDocument ports TestTryDelete.testTryDeleteDocument.
func TestTryDeleteDocument(t *testing.T) {
	dir := createTryDeleteIndex(t)
	writer := newTryDeleteWriter(t, dir)
	defer writer.Close()

	reader, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	q := search.NewTermQuery(index.NewTerm("foo", "0"))
	searcher := search.NewIndexSearcher(reader)
	topDocs, err := searcher.Search(q, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Fatalf("expected 1 hit for foo:0, got %d", topDocs.TotalHits.Value)
	}

	if ok, err := writer.TryDeleteDocument(reader, 0); err != nil {
		t.Fatalf("TryDeleteDocument: %v", err)
	} else if !ok {
		t.Fatal("expected TryDeleteDocument to succeed")
	}
	if !writer.HasDeletions() {
		t.Fatal("expected HasDeletions() to be true after TryDeleteDocument")
	}

	reader2, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter after delete: %v", err)
	}
	defer reader2.Close()
	searcher2 := search.NewIndexSearcher(reader2)
	topDocs, err = searcher2.Search(q, 10)
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if topDocs.TotalHits.Value != 0 {
		t.Fatalf("expected 0 hits for foo:0 after TryDeleteDocument, got %d", topDocs.TotalHits.Value)
	}
}

// TestTryDeleteDocumentCloseAndReopen ports
// TestTryDelete.testTryDeleteDocumentCloseAndReopen.
func TestTryDeleteDocumentCloseAndReopen(t *testing.T) {
	dir := createTryDeleteIndex(t)
	writer := newTryDeleteWriter(t, dir)

	reader, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	q := search.NewTermQuery(index.NewTerm("foo", "0"))
	searcher := search.NewIndexSearcher(reader)
	topDocs, err := searcher.Search(q, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Fatalf("expected 1 hit for foo:0, got %d", topDocs.TotalHits.Value)
	}

	if ok, err := writer.TryDeleteDocument(reader, 0); err != nil {
		t.Fatalf("TryDeleteDocument: %v", err)
	} else if !ok {
		t.Fatal("expected TryDeleteDocument to succeed")
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader2.Close()

	searcher2 := search.NewIndexSearcher(reader2)
	topDocs, err = searcher2.Search(q, 10)
	if err != nil {
		t.Fatalf("Search after reopen: %v", err)
	}
	if topDocs.TotalHits.Value != 0 {
		t.Fatalf("expected 0 hits for foo:0 after commit+reopen, got %d", topDocs.TotalHits.Value)
	}
}
