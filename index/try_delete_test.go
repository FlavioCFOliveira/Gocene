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
// Infrastructure gaps that drive the t.Skip calls in this file:
//   - Delete application is not implemented: IndexWriter.DeleteDocuments and
//     DeleteDocumentsQuery are no-op stubs and IndexWriter.TryDeleteDocument
//     only returns a sequence number without clearing live docs, so a delete
//     never reduces the on-disk document count.
//   - No IndexWriter.HasDeletions: there is no way to assert that a writer
//     observed a pending or applied deletion.
//   - No near-real-time reader: SearcherManager is constructed from an
//     *IndexSearcher, and OpenDirectoryReader only accepts a store.Directory,
//     so SearcherManager(writer, ...) and DirectoryReader.open(writer) have
//     no equivalent.
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

// TestTryDeleteDocument ports TestTryDelete.testTryDeleteDocument.
func TestTryDeleteDocument(t *testing.T) {
	dir := createTryDeleteIndex(t)
	writer := newTryDeleteWriter(t, dir)
	defer writer.Close()

	t.Fatal("tryDeleteDocument does not clear live docs and IndexWriter has " +
		"no HasDeletions/NRT SearcherManager: a delete cannot be observed " +
		"until the buffered-updates / live-docs pipeline is ported")
}

// TestTryDeleteDocumentCloseAndReopen ports
// TestTryDelete.testTryDeleteDocumentCloseAndReopen.
func TestTryDeleteDocumentCloseAndReopen(t *testing.T) {
	dir := createTryDeleteIndex(t)
	writer := newTryDeleteWriter(t, dir)
	defer writer.Close()

	t.Fatal("requires DirectoryReader.open(writer) (no NRT reader) and a " +
		"tryDeleteDocument that clears live docs so the deletion survives " +
		"close and reopen")
}

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
