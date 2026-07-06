// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIsCurrent.
// Source: lucene/core/src/test/org/apache/lucene/index/TestIsCurrent.java
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newIsCurrentWriter creates an IndexWriter over a fresh RAM directory using
// a simple WhitespaceAnalyzer-equivalent tokeniser.  This is sufficient for
// the one-field, single-token documents used by TestIsCurrent.
func newIsCurrentWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return w
}

// addIsCurrentDoc indexes a single document with one TextField named "content"
// whose value is the supplied string.
func addIsCurrentDoc(t *testing.T, w *index.IndexWriter, value string) {
	t.Helper()
	doc := document.NewDocument()
	f, err := document.NewTextField("content", value, true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument(%q): %v", value, err)
	}
}

// TestIsCurrent_DeleteByTermIsCurrent ports testDeleteByTermIsCurrent().
//
// Java opens an NRT reader from the writer, asserts it is current, deletes the
// single document by term, commits, and asserts the reader is now stale.
func TestIsCurrent_DeleteByTermIsCurrent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newIsCurrentWriter(t, dir)
	defer writer.Close()

	addIsCurrentDoc(t, writer, "aaa")
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	current, err := reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent: %v", err)
	}
	if !current {
		t.Fatal("fresh NRT reader should be current")
	}

	if err := writer.DeleteDocuments(index.NewTerm("content", "aaa")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit after delete: %v", err)
	}

	current, err = reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent after commit: %v", err)
	}
	if current {
		t.Fatal("NRT reader should be stale after a commit changed the index")
	}
}

// TestIsCurrent_DeleteAllIsCurrent ports testDeleteAllIsCurrent().
//
// Java opens an NRT reader from the writer, asserts it is current, calls
// writer.deleteAll(), commits, and asserts the reader is now stale.
func TestIsCurrent_DeleteAllIsCurrent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newIsCurrentWriter(t, dir)
	defer writer.Close()

	addIsCurrentDoc(t, writer, "aaa")
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	current, err := reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent: %v", err)
	}
	if !current {
		t.Fatal("fresh NRT reader should be current")
	}

	if err := writer.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit after deleteAll: %v", err)
	}

	current, err = reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent after commit: %v", err)
	}
	if current {
		t.Fatal("NRT reader should be stale after deleteAll + commit")
	}
}
