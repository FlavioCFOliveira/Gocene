// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// Focused, deterministic coverage of the near-real-time reader primitives
// added in rmp #1/#2: IndexWriter.GetReader, OpenDirectoryReaderFromWriter,
// and OpenIfChangedFromWriter. These assert the core NRT contract — buffered
// (uncommitted) documents are visible through a writer-opened reader, and a
// reopen observes only genuine changes.

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func nrtAddDoc(t *testing.T, w *index.IndexWriter, id, body string) {
	t.Helper()
	doc := document.NewDocument()
	idF, err := document.NewStringField("id", id, true)
	if err != nil {
		t.Fatalf("id field: %v", err)
	}
	doc.Add(idF)
	bodyF, err := document.NewTextField("body", body, true)
	if err != nil {
		t.Fatalf("body field: %v", err)
	}
	doc.Add(bodyF)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
}

// TestGetReader_SeesBufferedDocs proves that GetReader exposes documents
// that were added but never explicitly committed.
func TestGetReader_SeesBufferedDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	// Add a document WITHOUT calling Commit.
	nrtAddDoc(t, w, "1", "hello world")

	reader, err := w.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer reader.Close()

	if got := reader.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1 (buffered doc must be visible)", got)
	}

	searcher := search.NewIndexSearcher(reader)
	top, err := searcher.Search(search.NewTermQuery(index.NewTerm("body", "hello")), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Fatalf("TotalHits = %d, want 1", top.TotalHits.Value)
	}
}

// TestOpenDirectoryReaderFromWriter_Equivalence checks the package-level
// constructor behaves like the method and surfaces buffered docs.
func TestOpenDirectoryReaderFromWriter_Equivalence(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	nrtAddDoc(t, w, "1", "alpha")
	nrtAddDoc(t, w, "2", "beta")

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()
	if got := reader.MaxDoc(); got != 2 {
		t.Fatalf("MaxDoc = %d, want 2", got)
	}

	if _, err := index.OpenDirectoryReaderFromWriter(nil); err == nil {
		t.Fatalf("OpenDirectoryReaderFromWriter(nil) should error")
	}
}

// TestOpenIfChangedFromWriter_ReopenSemantics checks that a reopen returns
// nil when nothing changed and a fresh reader when documents were added.
func TestOpenIfChangedFromWriter_ReopenSemantics(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	nrtAddDoc(t, w, "1", "one")
	r1, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("open r1: %v", err)
	}
	defer r1.Close()
	if got := r1.MaxDoc(); got != 1 {
		t.Fatalf("r1 MaxDoc = %d, want 1", got)
	}

	// No changes since r1 → reopen must report "unchanged" (nil reader).
	r2, err := index.OpenIfChangedFromWriter(r1, w)
	if err != nil {
		t.Fatalf("openIfChanged (no change): %v", err)
	}
	if r2 != nil {
		r2.Close()
		t.Fatalf("openIfChanged returned a new reader despite no change")
	}

	// Add more docs → reopen must observe them.
	for i := 2; i <= 5; i++ {
		nrtAddDoc(t, w, strconv.Itoa(i), "doc"+strconv.Itoa(i))
	}
	r3, err := index.OpenIfChangedFromWriter(r1, w)
	if err != nil {
		t.Fatalf("openIfChanged (changed): %v", err)
	}
	if r3 == nil {
		t.Fatalf("openIfChanged returned nil despite added docs")
	}
	defer r3.Close()
	if got := r3.MaxDoc(); got != 5 {
		t.Fatalf("r3 MaxDoc = %d, want 5", got)
	}
}
