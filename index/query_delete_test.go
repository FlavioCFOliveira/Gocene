// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Coverage for IndexWriter.DeleteDocumentsQuery wired to the default
// QueryDeleteExecutor registered by the search package (rmp #13): query-based
// deletes are applied to committed segments and become visible after commit,
// and an unsupported query value surfaces a clear error rather than being
// silently dropped.

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Registers the default QueryDeleteExecutor and the production codec.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func qdAddDoc(t *testing.T, w *index.IndexWriter, id string) {
	t.Helper()
	doc := document.NewDocument()
	f, err := document.NewStringField("id", id, true)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(f)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
}

// TestQueryDelete_AppliedAfterCommit verifies that a buffered query delete is
// applied to committed segments and visible after the next commit.
func TestQueryDelete_AppliedAfterCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, id := range []string{"aaa", "bbb", "aaa", "ccc", "aaa"} {
		qdAddDoc(t, w, id)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := w.DeleteDocumentsQuery(search.NewTermQuery(index.NewTerm("id", "aaa"))); err != nil {
		t.Fatalf("DeleteDocumentsQuery: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit (post query-delete): %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if got := reader.NumDocs(); got != 2 {
		t.Fatalf("NumDocs after query-delete = %d, want 2", got)
	}
	if got := reader.NumDeletedDocs(); got != 3 {
		t.Fatalf("NumDeletedDocs = %d, want 3", got)
	}
	s := search.NewIndexSearcher(reader)
	top, err := s.Search(search.NewTermQuery(index.NewTerm("id", "aaa")), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 0 {
		t.Fatalf("id:aaa still matches %d docs after query-delete, want 0", top.TotalHits.Value)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestQueryDelete_UnsupportedTypeErrors verifies that buffering a non-Query
// value yields a clear error at commit instead of a silent no-op.
func TestQueryDelete_UnsupportedTypeErrors(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	qdAddDoc(t, w, "aaa")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// A plain string is not a search.Query.
	if err := w.DeleteDocumentsQuery("not-a-query"); err != nil {
		t.Fatalf("DeleteDocumentsQuery (buffering) should not error: %v", err)
	}
	err = w.Commit()
	if err == nil {
		t.Fatalf("Commit should fail for an unsupported query type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported query type") {
		t.Fatalf("error = %q, want it to mention 'unsupported query type'", err.Error())
	}
}
