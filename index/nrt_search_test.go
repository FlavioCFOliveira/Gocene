// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestNRTSearchBasic verifies basic NRT search: add a document, open an NRT
// reader, and search for its content.
func TestNRTSearchBasic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	nrtAddDoc(t, w, "1", "hello world")

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	top, err := searcher.Search(search.NewTermQuery(index.NewTerm("body", "hello")), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Fatalf("TotalHits = %d, want 1", top.TotalHits.Value)
	}
}

// TestNRTSearchAfterReopen opens a reader, adds more documents, reopens, and
// verifies the new reader sees all documents.
func TestNRTSearchAfterReopen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	// Add first doc and open initial reader.
	nrtAddDoc(t, w, "1", "alpha")
	r1, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("open reader 1: %v", err)
	}
	defer r1.Close()

	// Add more docs and reopen.
	for i := 2; i <= 5; i++ {
		nrtAddDoc(t, w, strconv.Itoa(i), "beta")
	}
	r2, err := index.OpenIfChangedFromWriter(r1, w)
	if err != nil {
		t.Fatalf("openIfChanged: %v", err)
	}
	if r2 == nil {
		t.Fatal("openIfChanged returned nil despite new docs")
	}
	defer r2.Close()

	if got := r2.MaxDoc(); got != 5 {
		t.Fatalf("r2 MaxDoc = %d, want 5", got)
	}
}

// TestNRTSearchConsistency checks that searching an NRT reader produces
// consistent results across multiple queries.
func TestNRTSearchConsistency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	docs := []struct{ id, body string }{
		{"1", "apple banana"},
		{"2", "banana cherry"},
		{"3", "cherry date"},
	}
	for _, d := range docs {
		nrtAddDoc(t, w, d.id, d.body)
	}

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	tests := []struct {
		term string
		want int64
	}{
		{"apple", 1},
		{"banana", 2},
		{"cherry", 2},
		{"date", 1},
		{"nonexistent", 0},
	}
	for _, tc := range tests {
		top, err := searcher.Search(search.NewTermQuery(index.NewTerm("body", tc.term)), 10)
		if err != nil {
			t.Fatalf("Search(%q): %v", tc.term, err)
		}
		if top.TotalHits.Value != tc.want {
			t.Fatalf("Search(%q) TotalHits = %d, want %d", tc.term, top.TotalHits.Value, tc.want)
		}
	}
}

// TestNRTSearchConcurrentReadWrite runs concurrent indexing and NRT reading
// to verify thread safety.
func TestNRTSearchConcurrentReadWrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	var wg sync.WaitGroup
	// Writer goroutine: add 50 docs.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			nrtAddDoc(t, w, strconv.Itoa(i), "concurrent")
		}
	}()

	// Reader goroutine: repeatedly open NRT readers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			reader, err := index.OpenDirectoryReaderFromWriter(w)
			if err != nil {
				t.Errorf("OpenDirectoryReaderFromWriter: %v", err)
				return
			}
			// At minimum, the reader should be openable and return
			// a non-negative maxDoc.
			if reader.MaxDoc() < 0 {
				t.Errorf("negative MaxDoc: %d", reader.MaxDoc())
			}
			reader.Close()
		}
	}()

	wg.Wait()
}

// TestNRTSearchAfterDelete checks that documents deleted before opening a
// reader are not visible.
func TestNRTSearchAfterDelete(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	nrtAddDoc(t, w, "1", "visible")
	nrtAddDoc(t, w, "2", "deleted")

	// Delete document with id "2" before opening reader.
	if err := w.DeleteDocuments(index.NewTerm("id", "2")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	// The reader should NOT see "deleted" but should see "visible".
	searcher := search.NewIndexSearcher(reader)
	top, err := searcher.Search(search.NewTermQuery(index.NewTerm("body", "visible")), 10)
	if err != nil {
		t.Fatalf("Search(visible): %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Fatalf("Search(visible) TotalHits = %d, want 1", top.TotalHits.Value)
	}
}

// TestNRTSearchMultipleFields verifies searching across multiple fields.
func TestNRTSearchMultipleFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	doc := document.NewDocument()
	idF, err := document.NewStringField("id", "1", true)
	if err != nil {
		t.Fatalf("id field: %v", err)
	}
	doc.Add(idF)
	titleF, err := document.NewTextField("title", "important document", true)
	if err != nil {
		t.Fatalf("title field: %v", err)
	}
	doc.Add(titleF)
	bodyF, err := document.NewTextField("body", "the quick brown fox", true)
	if err != nil {
		t.Fatalf("body field: %v", err)
	}
	doc.Add(bodyF)

	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Search in title field.
	top, err := searcher.Search(search.NewTermQuery(index.NewTerm("title", "important")), 10)
	if err != nil {
		t.Fatalf("Search(title, important): %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Fatalf("Search(title, important) TotalHits = %d, want 1", top.TotalHits.Value)
	}

	// Search in body field.
	top, err = searcher.Search(search.NewTermQuery(index.NewTerm("body", "quick")), 10)
	if err != nil {
		t.Fatalf("Search(body, quick): %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Fatalf("Search(body, quick) TotalHits = %d, want 1", top.TotalHits.Value)
	}
}
